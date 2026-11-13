//go:build android

// Package platform opens links in other apps: UPI apps, the phone dialer, WhatsApp, Google Meet
// and the browser.
package platform

import (
	"os"

	"gioui.org/app"
	"gioui.org/io/event"
	"git.wow.st/gmp/jni"
)

const flagActivityNewTask = 0x10000000

// Open hands the URL to Android (Intent.ACTION_VIEW), which picks the right app.
// It returns an error if no installed app can open it.
func Open(url string) error {
	return jni.Do(jni.JVMFor(app.JavaVM()), func(env jni.Env) error {
		uriClass := jni.FindClass(env, "android/net/Uri")
		parse := jni.GetStaticMethodID(env, uriClass, "parse", "(Ljava/lang/String;)Landroid/net/Uri;")
		uri, err := jni.CallStaticObjectMethod(env, uriClass, parse, jni.Value(jni.JavaString(env, url)))
		if err != nil {
			return err
		}
		intentClass := jni.FindClass(env, "android/content/Intent")
		newIntent := jni.GetMethodID(env, intentClass, "<init>", "(Ljava/lang/String;Landroid/net/Uri;)V")
		intent, err := jni.NewObject(env, intentClass, newIntent,
			jni.Value(jni.JavaString(env, "android.intent.action.VIEW")), jni.Value(uri))
		if err != nil {
			return err
		}
		addFlags := jni.GetMethodID(env, intentClass, "addFlags", "(I)Landroid/content/Intent;")
		if _, err := jni.CallObjectMethod(env, intent, addFlags, jni.Value(flagActivityNewTask)); err != nil {
			return err
		}
		ctx := jni.Object(app.AppContext())
		start := jni.GetMethodID(env, jni.GetObjectClass(env, ctx), "startActivity", "(Landroid/content/Intent;)V")
		return jni.CallVoidMethod(env, ctx, start, jni.Value(intent))
	})
}

// view is the Android View currently showing the app (0 when none).
var view uintptr

// HandleEvent tracks the Android view. If Android destroys the view while the process lives on,
// the app exits so the next launch starts cleanly (Gio cannot attach a new view to the old window).
func HandleEvent(e event.Event) {
	ve, ok := e.(app.AndroidViewEvent)
	if !ok {
		return
	}
	if ve.View == 0 && view != 0 {
		os.Exit(0)
	}
	view = ve.View
}

// MoveToBack sends the app to the background (like the home button) instead of closing it,
// which is what Android does for the back button on an app's main screen.
func MoveToBack() bool {
	if view == 0 {
		return false
	}
	err := jni.Do(jni.JVMFor(app.JavaVM()), func(env jni.Env) error {
		v := jni.Object(view)
		getContext := jni.GetMethodID(env, jni.GetObjectClass(env, v), "getContext", "()Landroid/content/Context;")
		activity, err := jni.CallObjectMethod(env, v, getContext)
		if err != nil {
			return err
		}
		moveTaskToBack := jni.GetMethodID(env, jni.GetObjectClass(env, activity), "moveTaskToBack", "(Z)Z")
		_, err = jni.CallBooleanMethod(env, activity, moveTaskToBack, jni.Value(1))
		return err
	})
	return err == nil
}
