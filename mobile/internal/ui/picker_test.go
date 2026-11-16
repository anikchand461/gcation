package ui

import (
	"mime"
	"testing"
)

// The file picker (gioui.org/x/explorer) turns each extension passed to ChooseFile into a file
// type with mime.TypeByExtension, which needs the leading dot (".png"). Without it the Android
// picker is given no types and shows no files.
func TestPickerTypes(t *testing.T) {
	for _, ext := range []string{".png", ".jpg", ".jpeg", ".webp"} {
		if mime.TypeByExtension(ext) == "" {
			t.Errorf("no file type for %s: the picker would hide these files", ext)
		}
	}
}
