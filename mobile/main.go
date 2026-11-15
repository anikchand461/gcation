// Command mobile is the G-CATION app (students and the admin) for Android, written in Go with Gio.
package main

import (
	"image"
	"image/color"
	"log"
	"os"
	"path/filepath"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"gcation/mobile/internal/platform"
	"gcation/mobile/internal/ui"
)

// Live addresses. When testing, override them with GCATION_API / GCATION_WEB on a computer,
// or at build time: gogio ... -ldflags "-X main.defaultAPI=http://10.0.2.2:8000".
var (
	defaultAPI = "https://gcation.onrender.com"
	// defaultAPI = "http://10.0.2.2:8000"
	defaultWeb = "https://gcation.vercel.app"
)

func main() {
	go func() {
		w := new(app.Window)
		w.Option(app.Title("G-CATION"), app.Size(unit.Dp(420), unit.Dp(860)))
		if err := run(w); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

func run(w *app.Window) error {
	dir, err := app.DataDir()
	if err != nil {
		dir = os.TempDir()
	}
	a := ui.New(w, env("GCATION_API", defaultAPI), env("GCATION_WEB", defaultWeb), filepath.Join(dir, "gcation"))

	var ops op.Ops
	var lastBars [2][4]uint8
	for {
		e := w.Event()
		a.Explorer().ListenEvents(e)
		platform.HandleEvent(e)
		switch e := e.(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			a.Layout(gtx)
			// Keep Android's status and navigation bars in the colours of the screen just drawn.
			top, bottom := a.StatusBarColor()
			paintBars(gtx, e, nrgba(top), nrgba(bottom))
			e.Frame(gtx.Ops)
			if bars := [2][4]uint8{top, bottom}; bars != lastBars {
				lastBars = bars
				w.Option(app.StatusColor(nrgba(top)), app.NavigationColor(nrgba(bottom)))
			}
		}
	}
}

// paintBars colours the areas behind the status and navigation bars. Android 15 draws apps
// edge to edge and ignores the bar colours, so the app paints them itself.
// gtx comes from app.NewContext, which shifts the origin below the status bar.
func paintBars(gtx layout.Context, e app.FrameEvent, top, bottom color.NRGBA) {
	m := e.Metric
	l, r := m.Dp(e.Insets.Left), m.Dp(e.Insets.Right)
	t, b := m.Dp(e.Insets.Top), m.Dp(e.Insets.Bottom)
	w, h := gtx.Constraints.Max.X, gtx.Constraints.Max.Y
	if t > 0 {
		paint.FillShape(gtx.Ops, top, clip.Rect{Min: image.Pt(-l, -t), Max: image.Pt(w+r, 0)}.Op())
	}
	if b > 0 {
		paint.FillShape(gtx.Ops, bottom, clip.Rect{Min: image.Pt(-l, h), Max: image.Pt(w+r, h+b)}.Op())
	}
}

func nrgba(c [4]uint8) color.NRGBA { return color.NRGBA{R: c[0], G: c[1], B: c[2], A: c[3]} }

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
