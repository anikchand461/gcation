package ui

import (
	"image"
	"math"
	"time"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

// The opening animation: the logo grows in on the brand gradient with a soft pulsing light, the
// name slides up, and once the first screen has its data the whole layer zooms and fades away.

const (
	introMin  = 1100 * time.Millisecond // shortest time the intro stays on screen
	introFade = 420 * time.Millisecond
)

type introState struct {
	start time.Time // zero when the intro is not showing
	exit  time.Time // when the fade-out began
}

// appReady reports whether the screen under the intro has its data.
func (a *App) appReady() bool {
	switch {
	case a.screen == scrSplash:
		return false
	case a.admin != nil:
		return a.admin.loaded || a.admin.err != ""
	}
	return true
}

// layoutIntro draws the intro on top of the app while it plays.
func (a *App) layoutIntro(gtx C) {
	in := &a.intro
	if in.start.IsZero() {
		return
	}
	now := gtx.Now
	if in.exit.IsZero() && now.Sub(in.start) >= introMin && a.appReady() {
		in.exit = now
	}
	fade := float32(1)
	if !in.exit.IsZero() {
		p := float32(now.Sub(in.exit)) / float32(introFade)
		if p >= 1 {
			*in = introState{}
			return
		}
		fade = 1 - easeOut(p)
	}
	gtx.Execute(op.InvalidateCmd{})

	// Catch touches so nothing underneath is tapped by accident.
	defer clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, in)
	defer paint.PushOpacity(gtx.Ops, fade).Pop()
	a.splashScene(gtx, now.Sub(in.start), 1+0.08*(1-fade))
}

// layoutSplash is the screen shown while the student's data loads after logging in.
func (a *App) layoutSplash(gtx C) D {
	if a.intro.start.IsZero() && a.win != nil {
		a.intro.start = gtx.Now
	}
	return a.splashScene(gtx, 3*time.Second, 1) // fully revealed; the intro layer animates on top
}

// splashScene draws the intro at time t. zoom enlarges the logo during the fade-out.
func (a *App) splashScene(gtx C, t time.Duration, zoom float32) D {
	size := gtx.Constraints.Max
	sec := float32(t.Seconds())
	gradientRound(gtx, size, 0, heroA, heroB)
	glow(gtx, image.Pt(size.X, 0), size.Y*2/3, argb(0xc98cff, 110))
	glow(gtx, image.Pt(0, size.Y), size.Y/2, argb(0xdb2777, 80))

	cx, cy := size.X/2, size.Y*44/100
	logo := gtx.Dp(112)
	center := f32.Pt(float32(cx), float32(cy))

	// Light behind the logo that breathes slowly.
	in := easeOut(clamp01(sec / 0.8))
	breathe := float32(1 + 0.08*math.Sin(float64(sec)*2.6))
	st := op.Affine(f32.Affine2D{}.Scale(center, f32.Pt(in*breathe, in*breathe))).Push(gtx.Ops)
	glow(gtx, image.Pt(cx, cy), logo, argb(0xa78bfa, 150))
	st.Pop()

	// Rings that ripple out from the logo.
	for i := 0; i < 2; i++ {
		p := float32(math.Mod(float64(sec-0.45-float32(i)*0.9), 1.8)) / 1.8
		if sec < 0.45+float32(i)*0.9 || p < 0 {
			continue
		}
		r := float32(logo)*0.5 + float32(logo)*0.32*easeOut(p)
		alpha := uint8(110 * (1 - p))
		ring := clip.Ellipse{Min: image.Pt(cx-int(r), cy-int(r)), Max: image.Pt(cx+int(r), cy+int(r))}
		paint.FillShape(gtx.Ops, argb(0xe9d5ff, alpha), clip.Stroke{Path: ring.Path(gtx.Ops), Width: float32(gtx.Dp(1.5))}.Op())
	}

	// The logo pops in.
	if sec > 0 {
		s := easeOutBack(clamp01(sec/0.7)) * zoom
		st := op.Affine(f32.Affine2D{}.Scale(center, f32.Pt(s, s))).Push(gtx.Ops)
		fade := paint.PushOpacity(gtx.Ops, clamp01(sec/0.3))
		off := op.Offset(image.Pt(cx-logo/2, cy-logo/2)).Push(gtx.Ops)
		imageBox(gtx, a.logo, image.Pt(logo, logo), 0)
		off.Pop()
		fade.Pop()
		st.Pop()
	}

	// Name and tagline slide up underneath.
	y := cy + logo/2 + gtx.Dp(50)
	rise := func(delay, dur float32) (float32, int) {
		p := easeOut(clamp01((sec - delay) / dur))
		return p, int((1 - p) * float32(gtx.Dp(18)))
	}
	p, dy := rise(0.3, 0.5)
	h := drawCentered(gtx, cx, y+dy, p, func(gtx C) D {
		return a.th.label(gtx, "G-CATION", textOpt{size: 32, weight: font.Black, color: white})
	})
	p, dy = rise(0.5, 0.5)
	drawCentered(gtx, cx, y+h+gtx.Dp(4)+dy, p, func(gtx C) D {
		return a.th.label(gtx, "Learn. Grow. Succeed.", textOpt{size: 15, color: rgb(0xd7c6f5)})
	})

	// Loading dots near the bottom.
	if dotsIn := clamp01((sec - 0.9) / 0.4); dotsIn > 0 {
		d, gap := gtx.Dp(8), gtx.Dp(16)
		by := size.Y - gtx.Dp(96)
		for i := 0; i < 3; i++ {
			wave := float32(0.5 + 0.5*math.Sin(float64(sec)*5-float64(i)*0.8))
			alpha := uint8(255 * dotsIn * (0.3 + 0.7*wave))
			x := cx + (i-1)*gap - d/2
			paint.FillShape(gtx.Ops, argb(0xffffff, alpha), clip.Ellipse{Min: image.Pt(x, by), Max: image.Pt(x+d, by+d)}.Op(gtx.Ops))
		}
	}
	return D{Size: size}
}

// drawCentered draws w with its top centre at (x, y) and returns its height.
func drawCentered(gtx C, x, y int, alpha float32, w layout.Widget) int {
	gtx.Constraints.Min = image.Point{}
	m := op.Record(gtx.Ops)
	d := w(gtx)
	call := m.Stop()
	if alpha > 0 {
		defer op.Offset(image.Pt(x-d.Size.X/2, y)).Push(gtx.Ops).Pop()
		defer paint.PushOpacity(gtx.Ops, alpha).Pop()
		call.Add(gtx.Ops)
	}
	return d.Size.Y
}

func clamp01(v float32) float32 { return max(0, min(1, v)) }

func easeOut(p float32) float32 { q := 1 - p; return 1 - q*q*q }

// easeOutBack overshoots a little before settling, for a gentle "pop".
func easeOutBack(p float32) float32 {
	const c1 = 1.70158
	const c3 = c1 + 1
	q := p - 1
	return 1 + c3*q*q*q + c1*q*q
}
