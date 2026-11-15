package ui

import (
	"bytes"
	_ "embed"
	"image"
	"image/color"
	"image/png"

	"gioui.org/op/paint"
	"gioui.org/unit"
)

// GitHub's logo (the "mark-github" Octicon, MIT licence), drawn black on transparent.
//
//go:embed assets/github-mark.png
var githubMarkPNG []byte

var githubMarks = map[color.NRGBA]paint.ImageOp{} // the logo in each colour drawn so far

// githubMark draws GitHub's logo in colour c, e.g. as a button's leading icon.
func githubMark(gtx C, size unit.Dp, c color.NRGBA) D {
	op, ok := githubMarks[c]
	if !ok {
		src, err := png.Decode(bytes.NewReader(githubMarkPNG))
		if err != nil {
			return D{}
		}
		b := src.Bounds()
		dst := image.NewNRGBA(b)
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				_, _, _, alpha := src.At(x, y).RGBA()
				dst.SetNRGBA(x, y, color.NRGBA{R: c.R, G: c.G, B: c.B, A: uint8(uint32(c.A) * (alpha >> 8) / 255)})
			}
		}
		op = paint.NewImageOp(dst)
		githubMarks[c] = op
	}
	s := gtx.Dp(size)
	return imageBox(gtx, op, image.Pt(s, s), 0)
}
