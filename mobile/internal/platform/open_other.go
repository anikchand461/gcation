//go:build !android

package platform

import (
	"os/exec"
	"runtime"

	"gioui.org/io/event"
)

// Open opens the URL with the system's default handler (used when running the app on a computer).
func Open(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}

// HandleEvent and MoveToBack only do something on Android.
func HandleEvent(event.Event) {}

func MoveToBack() bool { return false }
