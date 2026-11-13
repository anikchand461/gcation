package ui

import (
	"embed"

	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/font/opentype"
)

// Inter (the website's font, SIL Open Font License) in the weights the app uses.
// It includes the ₹ sign in every weight, which the Go fonts lack.
//
//go:embed assets/fonts/*.ttf
var fontFiles embed.FS

const appTypeface font.Typeface = "Inter"

// fontCollection returns Inter first, with the Go fonts as a fallback.
func fontCollection() []font.FontFace {
	var faces []font.FontFace
	entries, _ := fontFiles.ReadDir("assets/fonts")
	for _, e := range entries {
		data, err := fontFiles.ReadFile("assets/fonts/" + e.Name())
		if err != nil {
			continue
		}
		parsed, err := opentype.ParseCollection(data)
		if err != nil {
			continue
		}
		faces = append(faces, parsed...)
	}
	return append(faces, gofont.Collection()...)
}
