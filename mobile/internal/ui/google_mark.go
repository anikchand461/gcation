package ui

import (
	"image"
	"image/color"
	"math"

	"gioui.org/op/paint"
	"gioui.org/unit"
)

var googleMarkOp *paint.ImageOp

// googleMark draws Google's four-colour "G" for the "Continue with Google" button. It keeps its own
// colours, so c is ignored.
func googleMark(gtx C, size unit.Dp, _ color.NRGBA) D {
	if googleMarkOp == nil {
		op := paint.NewImageOp(drawGoogleG(96))
		googleMarkOp = &op
	}
	s := gtx.Dp(size)
	return imageBox(gtx, *googleMarkOp, image.Pt(s, s), 0)
}

// drawGoogleG paints the "G" as a ring split into red, yellow, green and blue, with the blue bar,
// using 4×4 samples per pixel for smooth edges.
func drawGoogleG(n int) *image.NRGBA {
	red, yellow, green, blue := rgb(0xea4335), rgb(0xfbbc05), rgb(0x34a853), rgb(0x4285f4)
	img := image.NewNRGBA(image.Rect(0, 0, n, n))
	const ss = 4
	outer, inner := 0.5, 0.3 // radii as a share of the size
	for py := 0; py < n; py++ {
		for px := 0; px < n; px++ {
			var r, g, b, hits float64
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					x := (float64(px)+(float64(sx)+0.5)/ss)/float64(n) - 0.5
					y := 0.5 - (float64(py)+(float64(sy)+0.5)/ss)/float64(n)
					d := math.Hypot(x, y)
					deg := math.Atan2(y, x) * 180 / math.Pi
					if deg < 0 {
						deg += 360
					}
					var c color.NRGBA
					switch {
					case x >= 0 && x <= outer*0.96 && math.Abs(y+0.01) <= 0.095 && d <= outer: // the bar
						c = blue
					case d < inner || d > outer:
						continue
					case deg >= 8 && deg < 42: // the opening on the right
						continue
					case deg >= 42 && deg < 140:
						c = red
					case deg >= 140 && deg < 220:
						c = yellow
					case deg >= 220 && deg < 315:
						c = green
					default:
						c = blue
					}
					r, g, b, hits = r+float64(c.R), g+float64(c.G), b+float64(c.B), hits+1
				}
			}
			if hits > 0 {
				img.SetNRGBA(px, py, color.NRGBA{R: uint8(r / hits), G: uint8(g / hits), B: uint8(b / hits), A: uint8(255 * hits / (ss * ss))})
			}
		}
	}
	return img
}
