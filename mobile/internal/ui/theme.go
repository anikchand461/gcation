package ui

import (
	"image"
	"image/color"
	"strings"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

// Palette mirrors the website's light and dark themes.
type Palette struct {
	Bg, Surface, Field, Ink, Ink2, Muted, Faint, Line, Line2 color.NRGBA
	Lav, Accent, Violet                                      color.NRGBA
	Green, GreenBg, Red, RedBg, Amber, AmberBg, Blue, BlueBg color.NRGBA
	Nav, Shadow                                              color.NRGBA
}

var (
	gradA     = rgb(0x4f46e5) // indigo
	gradB     = rgb(0xdb2777) // pink; the two stops pass through violet
	heroA     = rgb(0x0b0726)
	heroB     = rgb(0x5b21b6)
	white     = rgb(0xffffff)
	brandDeep = rgb(0x5b21b6)
)

var lightPalette = Palette{
	Bg: rgb(0xf6f4fc), Surface: rgb(0xffffff), Field: rgb(0xffffff),
	Ink: rgb(0x140f2e), Ink2: rgb(0x2f2a4a), Muted: rgb(0x67628a), Faint: rgb(0x9a96b8),
	Line: rgb(0xe8e4f7), Line2: rgb(0xd8d1f2),
	Lav: rgb(0xefeafe), Accent: rgb(0x6d28d9), Violet: rgb(0x8b5cf6),
	Green: rgb(0x047857), GreenBg: rgb(0xdcfce7), Red: rgb(0xbe123c), RedBg: rgb(0xffe4e6),
	Amber: rgb(0xb45309), AmberBg: rgb(0xfef3c7), Blue: rgb(0x1d4ed8), BlueBg: rgb(0xdbeafe),
	Nav: rgb(0xffffff), Shadow: argb(0x1e1450, 18),
}

var darkPalette = Palette{
	Bg: rgb(0x08061a), Surface: rgb(0x1b1540), Field: rgb(0x130e30),
	Ink: rgb(0xf2effd), Ink2: rgb(0xd4cdf2), Muted: rgb(0xa49ec9), Faint: rgb(0x7c76a6),
	Line: rgb(0x262050), Line2: rgb(0x332b62),
	Lav: rgb(0x2a1f5c), Accent: rgb(0xc4b5fd), Violet: rgb(0xa78bfa),
	Green: rgb(0x6ee7b7), GreenBg: rgb(0x0f2f2a), Red: rgb(0xfda4af), RedBg: rgb(0x3a1424),
	Amber: rgb(0xfcd34d), AmberBg: rgb(0x3a2a10), Blue: rgb(0x93c5fd), BlueBg: rgb(0x142a4a),
	Nav: rgb(0x110c2a), Shadow: argb(0x000000, 70),
}

// Theme holds colours and the text shaper for the whole app.
type Theme struct {
	Dark bool
	P    Palette
	M    *material.Theme
}

func NewTheme(dark bool) *Theme {
	m := material.NewTheme()
	// Inter (with ₹ in every weight), then the Go fonts; the phone's system fonts still
	// cover anything else, such as Indian scripts in names.
	m.Shaper = text.NewShaper(text.WithCollection(fontCollection()))
	m.Face = appTypeface
	t := &Theme{M: m}
	t.SetDark(dark)
	return t
}

func (t *Theme) SetDark(dark bool) {
	t.Dark = dark
	if dark {
		t.P = darkPalette
	} else {
		t.P = lightPalette
	}
	t.M.Palette.Fg = t.P.Ink
	t.M.Palette.Bg = t.P.Bg
	t.M.Palette.ContrastBg = t.P.Violet
	t.M.Palette.ContrastFg = white
}

// ---------- Colour helpers ----------

func rgb(h uint32) color.NRGBA { return argb(h, 255) }

func argb(h uint32, a uint8) color.NRGBA {
	return color.NRGBA{R: uint8(h >> 16), G: uint8(h >> 8), B: uint8(h), A: a}
}

func withAlpha(c color.NRGBA, a uint8) color.NRGBA { c.A = a; return c }

// ---------- Drawing ----------

func rrect(gtx C, size image.Point, radius unit.Dp) clip.RRect {
	r := gtx.Dp(radius)
	if m := min(size.X, size.Y) / 2; r > m {
		r = m
	}
	return clip.RRect{Rect: image.Rectangle{Max: size}, SE: r, SW: r, NW: r, NE: r}
}

func fillRound(gtx C, size image.Point, radius unit.Dp, c color.NRGBA) {
	paint.FillShape(gtx.Ops, c, rrect(gtx, size, radius).Op(gtx.Ops))
}

func strokeRound(gtx C, size image.Point, radius unit.Dp, width unit.Dp, c color.NRGBA) {
	rr := rrect(gtx, size, radius)
	paint.FillShape(gtx.Ops, c, clip.Stroke{Path: rr.Path(gtx.Ops), Width: float32(gtx.Dp(width))}.Op())
}

// gradientRound fills a rounded rectangle with a diagonal two-colour gradient.
func gradientRound(gtx C, size image.Point, radius unit.Dp, c1, c2 color.NRGBA) {
	defer rrect(gtx, size, radius).Push(gtx.Ops).Pop()
	paint.LinearGradientOp{
		Stop1: f32.Pt(0, 0), Color1: c1,
		Stop2: f32.Pt(float32(size.X), float32(size.Y)), Color2: c2,
	}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
}

// glow paints a soft round light (used on hero cards). The light is a radial-gradient image made
// once per size and colour and reused, so drawing it costs one image per frame.
func glow(gtx C, center image.Point, radius int, c color.NRGBA) {
	if radius <= 0 {
		return
	}
	key := glowKey{radius, c}
	img, ok := glowCache[key]
	if !ok {
		img = paint.NewImageOp(radialImage(radius, c))
		glowCache[key] = img
	}
	size := img.Size()
	scale := float32(2*radius) / float32(size.X)
	defer op.Offset(image.Pt(center.X-radius, center.Y-radius)).Push(gtx.Ops).Pop()
	defer op.Affine(f32.Affine2D{}.Scale(f32.Point{}, f32.Pt(scale, scale))).Push(gtx.Ops).Pop()
	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	img.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
}

type glowKey struct {
	radius int
	c      color.NRGBA
}

var glowCache = map[glowKey]paint.ImageOp{}

func radialImage(radius int, c color.NRGBA) *image.NRGBA {
	// Drawn at quarter resolution; the GPU scales it up smoothly.
	r := max(radius/4, 1)
	img := image.NewNRGBA(image.Rect(0, 0, 2*r, 2*r))
	for y := 0; y < 2*r; y++ {
		for x := 0; x < 2*r; x++ {
			dx, dy := float64(x-r)+0.5, float64(y-r)+0.5
			d := (dx*dx + dy*dy) / float64(r*r)
			if d >= 1 {
				continue
			}
			f := (1 - d) * (1 - d)
			img.SetNRGBA(x, y, color.NRGBA{R: c.R, G: c.G, B: c.B, A: uint8(float64(c.A) * f)})
		}
	}
	return img
}

// withBackground lays out w, then paints bg behind it at the same size.
func withBackground(gtx C, bg func(gtx C, size image.Point), w layout.Widget) D {
	m := op.Record(gtx.Ops)
	dims := w(gtx)
	call := m.Stop()
	bg(gtx, dims.Size)
	call.Add(gtx.Ops)
	return dims
}

// ---------- Text ----------

type textOpt struct {
	size     unit.Sp
	weight   font.Weight
	color    color.NRGBA
	maxLines int
	align    text.Alignment
}

func (t *Theme) label(gtx C, s string, o textOpt) D {
	if o.size == 0 {
		o.size = 15
	}
	if o.color.A == 0 {
		o.color = t.P.Ink
	}
	l := material.Label(t.M, o.size, s)
	l.Color = o.color
	l.Font.Weight = o.weight
	l.MaxLines = o.maxLines
	l.Alignment = o.align
	return l.Layout(gtx)
}

// money draws an amount with the ₹ sign at regular weight and the number in o.weight.
func (t *Theme) money(gtx C, v float64, o textOpt) D {
	num := strings.TrimPrefix(rupees(v), "₹")
	sign := o
	sign.weight = font.Normal
	return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return t.label(gtx, "₹", sign) }),
		layout.Rigid(func(gtx C) D { return t.label(gtx, num, o) }),
	)
}

func (t *Theme) title(gtx C, s string, size unit.Sp) D {
	return t.label(gtx, s, textOpt{size: size, weight: font.Bold})
}

func (t *Theme) body(gtx C, s string) D {
	return t.label(gtx, s, textOpt{size: 14, color: t.P.Ink2})
}

func (t *Theme) muted(gtx C, s string, size unit.Sp) D {
	return t.label(gtx, s, textOpt{size: size, color: t.P.Muted})
}

func (t *Theme) kicker(gtx C, s string, c color.NRGBA) D {
	return t.label(gtx, s, textOpt{size: 11, weight: font.Bold, color: c})
}
