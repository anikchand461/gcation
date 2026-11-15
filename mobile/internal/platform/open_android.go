//go:build android

// Package platform opens links in other apps: UPI apps, the phone dialer, WhatsApp, Google Meet
// and the browser.
package platform

import (
	"net/url"
	"os"
	"strings"

	"gioui.org/app"
	"gioui.org/io/event"
	"git.wow.st/gmp/jni"
)

const flagActivityNewTask = 0x10000000

// Open hands the link to Android, which picks the right app. mailto: links open an email app
// (Gmail etc.) with the address, subject and message filled in; everything else (web pages, tel:,
// upi:, Google Meet) is opened with ACTION_VIEW. It returns an error if no installed app can open it.
func Open(link string) error {
	if strings.HasPrefix(link, "mailto:") {
		err := compose(link)
		if err == nil {
			return nil
		}
		// No email app installed: write it in Gmail in the browser instead.
		return start("android.intent.action.VIEW", gmailWeb(link), nil)
	}
	return start("android.intent.action.VIEW", link, nil)
}

// compose opens an email app (ACTION_SENDTO). Gmail and most mail apps read the address,
// subject and body from the intent extras, so they are sent there as well as in the link.
func compose(link string) error {
	to, q := parseMailto(link)
	return start("android.intent.action.SENDTO", "mailto:"+url.PathEscape(to), func(env jni.Env, intentClass jni.Class, intent jni.Object) error {
		putString := jni.GetMethodID(env, intentClass, "putExtra", "(Ljava/lang/String;Ljava/lang/String;)Landroid/content/Intent;")
		putArray := jni.GetMethodID(env, intentClass, "putExtra", "(Ljava/lang/String;[Ljava/lang/String;)Landroid/content/Intent;")
		addresses := jni.NewObjectArray(env, 1, jni.FindClass(env, "java/lang/String"), jni.Object(jni.JavaString(env, to)))
		if _, err := jni.CallObjectMethod(env, intent, putArray,
			jni.Value(jni.JavaString(env, "android.intent.extra.EMAIL")), jni.Value(addresses)); err != nil {
			return err
		}
		for key, value := range map[string]string{"android.intent.extra.SUBJECT": q.Get("subject"), "android.intent.extra.TEXT": q.Get("body")} {
			if value == "" {
				continue
			}
			if _, err := jni.CallObjectMethod(env, intent, putString,
				jni.Value(jni.JavaString(env, key)), jni.Value(jni.JavaString(env, value))); err != nil {
				return err
			}
		}
		return nil
	})
}

// start builds an Intent with the given action and data link, lets extras add to it, and starts it.
func start(action, data string, extras func(jni.Env, jni.Class, jni.Object) error) error {
	return jni.Do(jni.JVMFor(app.JavaVM()), func(env jni.Env) error {
		uriClass := jni.FindClass(env, "android/net/Uri")
		parse := jni.GetStaticMethodID(env, uriClass, "parse", "(Ljava/lang/String;)Landroid/net/Uri;")
		uri, err := jni.CallStaticObjectMethod(env, uriClass, parse, jni.Value(jni.JavaString(env, data)))
		if err != nil {
			return err
		}
		// The Intent is made empty and filled in with setAction / setData: the jni package's
		// NewObject passes constructor arguments wrongly (crashing the app), but calls are fine.
		intentClass := jni.FindClass(env, "android/content/Intent")
		newIntent := jni.GetMethodID(env, intentClass, "<init>", "()V")
		intent, err := jni.NewObject(env, intentClass, newIntent)
		if err != nil {
			return err
		}
		setAction := jni.GetMethodID(env, intentClass, "setAction", "(Ljava/lang/String;)Landroid/content/Intent;")
		if _, err := jni.CallObjectMethod(env, intent, setAction, jni.Value(jni.JavaString(env, action))); err != nil {
			return err
		}
		setData := jni.GetMethodID(env, intentClass, "setData", "(Landroid/net/Uri;)Landroid/content/Intent;")
		if _, err := jni.CallObjectMethod(env, intent, setData, jni.Value(uri)); err != nil {
			return err
		}
		if extras != nil {
			if err := extras(env, intentClass, intent); err != nil {
				return err
			}
		}
		addFlags := jni.GetMethodID(env, intentClass, "addFlags", "(I)Landroid/content/Intent;")
		if _, err := jni.CallObjectMethod(env, intent, addFlags, jni.Value(flagActivityNewTask)); err != nil {
			return err
		}
		ctx := jni.Object(app.AppContext())
		startActivity := jni.GetMethodID(env, jni.GetObjectClass(env, ctx), "startActivity", "(Landroid/content/Intent;)V")
		return jni.CallVoidMethod(env, ctx, startActivity, jni.Value(intent))
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
