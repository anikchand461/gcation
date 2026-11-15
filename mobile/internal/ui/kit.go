package ui

import (
	"image"
	"image/color"
	"strings"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"golang.org/x/exp/shiny/materialdesign/icons"
)

// ---------- Icons ----------

func mustIcon(data []byte) *widget.Icon {
	ic, err := widget.NewIcon(data)
	if err != nil {
		panic(err)
	}
	return ic
}

var (
	icHome         = mustIcon(icons.ActionHome)
	icSchool       = mustIcon(icons.SocialSchool)
	icMegaphone    = mustIcon(icons.ActionAnnouncement)
	icPerson       = mustIcon(icons.SocialPerson)
	icBell         = mustIcon(icons.SocialNotifications)
	icCopy         = mustIcon(icons.ContentContentCopy)
	icCall         = mustIcon(icons.CommunicationCall)
	icChat         = mustIcon(icons.CommunicationChat)
	icVideo        = mustIcon(icons.AVVideocam)
	icOpen         = mustIcon(icons.ActionOpenInNew)
	icLock         = mustIcon(icons.ActionLockOutline)
	icCheck        = mustIcon(icons.ActionCheckCircle)
	icBack         = mustIcon(icons.NavigationArrowBack)
	icChevron      = mustIcon(icons.NavigationChevronRight)
	icMoon         = mustIcon(icons.ImageBrightness2)
	icSun          = mustIcon(icons.ImageWBSunny)
	icLogout       = mustIcon(icons.ActionExitToApp)
	icDownload     = mustIcon(icons.FileFileDownload)
	icPay          = mustIcon(icons.ActionPayment)
	icHourglass    = mustIcon(icons.ActionHourglassEmpty)
	icEye          = mustIcon(icons.ActionVisibility)
	icEyeOff       = mustIcon(icons.ActionVisibilityOff)
	icSend         = mustIcon(icons.ContentSend)
	icReceipt      = mustIcon(icons.ActionReceipt)
	icCalendar     = mustIcon(icons.ActionEvent)
	icInfo         = mustIcon(icons.ActionInfo)
	icAlert        = mustIcon(icons.AlertErrorOutline)
	icShield       = mustIcon(icons.ActionVerifiedUser)
	icLink         = mustIcon(icons.ContentLink)
	icClock        = mustIcon(icons.ActionQueryBuilder)
	icTick         = mustIcon(icons.NavigationCheck)
	icEdit         = mustIcon(icons.ImageEdit)
	icKey          = mustIcon(icons.CommunicationVPNKey)
	icRefresh      = mustIcon(icons.ActionAutorenew)
	icMail         = mustIcon(icons.CommunicationEmail)
	icClose        = mustIcon(icons.NavigationClose)
	icUpload       = mustIcon(icons.FileFileUpload)
	icAdd          = mustIcon(icons.ContentAdd)
	icSettingsMore = mustIcon(icons.NavigationMoreHoriz)
	icCode         = mustIcon(icons.ActionCode)
	icExpand       = mustIcon(icons.NavigationExpandMore)
	icCollapse     = mustIcon(icons.NavigationExpandLess)
)

func iconSized(gtx C, ic *widget.Icon, size unit.Dp, c color.NRGBA) D {
	s := gtx.Dp(size)
	gtx.Constraints = layout.Exact(image.Pt(s, s))
	return ic.Layout(gtx, c)
}

// ---------- Spacing ----------

func vspace(dp unit.Dp) layout.Widget { return layout.Spacer{Height: dp}.Layout }
func hspace(dp unit.Dp) layout.Widget { return layout.Spacer{Width: dp}.Layout }

func pad(all unit.Dp) layout.Inset { return layout.UniformInset(all) }

// ---------- Buttons ----------

type btnKind int

const (
	btnPrimary btnKind = iota
	btnOutline
	btnGhost
	btnSoft
	btnDanger
	btnSuccess
	btnWhite
	btnOnDark
)

type btnOpt struct {
	kind  btnKind
	icon  *widget.Icon
	full  bool
	small bool
	busy  bool
}

func (t *Theme) button(gtx C, clk *widget.Clickable, label string, o btnOpt) D {
	h, fs, ic := unit.Dp(50), unit.Sp(15.5), unit.Dp(19)
	if o.small {
		h, fs, ic = 38, 13.5, 16
	}
	var fg, border color.NRGBA
	bg := func(gtx C, size image.Point) {}
	switch o.kind {
	case btnPrimary:
		fg = white
		bg = func(gtx C, size image.Point) { gradientRound(gtx, size, 14, gradA, gradB) }
	case btnSuccess:
		fg = white
		bg = func(gtx C, size image.Point) { gradientRound(gtx, size, 14, rgb(0x059669), rgb(0x0d9488)) }
	case btnWhite:
		fg = brandDeep
		bg = func(gtx C, size image.Point) { fillRound(gtx, size, 14, white) }
	case btnOnDark:
		fg = white
		bg = func(gtx C, size image.Point) { fillRound(gtx, size, 14, argb(0xffffff, 34)) }
		border = argb(0xffffff, 70)
	case btnOutline:
		fg = t.P.Accent
		bg = func(gtx C, size image.Point) { fillRound(gtx, size, 14, t.P.Surface) }
		border = t.P.Line2
	case btnSoft:
		fg = t.P.Accent
		bg = func(gtx C, size image.Point) { fillRound(gtx, size, 14, t.P.Lav) }
	case btnDanger:
		fg = t.P.Red
		bg = func(gtx C, size image.Point) { fillRound(gtx, size, 14, t.P.RedBg) }
	case btnGhost:
		fg = t.P.Accent
	}
	if o.busy {
		label = "Please wait…"
	}
	return clk.Layout(gtx, func(gtx C) D {
		if o.full {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
		}
		gtx.Constraints.Min.Y = gtx.Dp(h)
		return withBackground(gtx, func(gtx C, size image.Point) {
			bg(gtx, size)
			if border.A > 0 {
				strokeRound(gtx, size, 14, 1, border)
			}
			if clk.Pressed() || o.busy {
				fillRound(gtx, size, 14, argb(0x000000, 26))
			}
		}, func(gtx C) D {
			side := unit.Dp(16)
			if o.small {
				side = 10
			}
			return layout.Inset{Left: side, Right: side}.Layout(gtx, func(gtx C) D {
				return layout.Center.Layout(gtx, func(gtx C) D {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							if o.icon == nil || o.busy {
								return D{}
							}
							return layout.Inset{Right: 8}.Layout(gtx, func(gtx C) D { return iconSized(gtx, o.icon, ic, fg) })
						}),
						layout.Rigid(func(gtx C) D {
							return t.label(gtx, label, textOpt{size: fs, weight: font.Bold, color: fg, maxLines: 1})
						}),
					)
				})
			})
		})
	})
}

// iconButton is a round 42dp button holding only an icon (with an optional red count badge).
func (t *Theme) iconButton(gtx C, clk *widget.Clickable, ic *widget.Icon, count int) D {
	return clk.Layout(gtx, func(gtx C) D {
		s := gtx.Dp(42)
		gtx.Constraints = layout.Exact(image.Pt(s, s))
		fillRound(gtx, image.Pt(s, s), 13, t.P.Surface)
		strokeRound(gtx, image.Pt(s, s), 13, 1, t.P.Line)
		layout.Center.Layout(gtx, func(gtx C) D { return iconSized(gtx, ic, 21, t.P.Accent) })
		if count > 0 {
			d := gtx.Dp(18)
			dot := image.Rect(s-d+gtx.Dp(4), -gtx.Dp(4), s+gtx.Dp(4), d-gtx.Dp(4))
			paint.FillShape(gtx.Ops, rgb(0xe5484d), clip.Ellipse(dot).Op(gtx.Ops))
			n := "9+"
			if count < 10 {
				n = string(rune('0' + count))
			}
			gtx2 := gtx
			gtx2.Constraints = layout.Exact(dot.Size())
			stack := pushOffset(gtx2, dot.Min)
			layout.Center.Layout(gtx2, func(gtx C) D {
				return t.label(gtx, n, textOpt{size: 10, weight: font.Bold, color: white})
			})
			stack.Pop()
		}
		return D{Size: image.Pt(s, s)}
	})
}

// ---------- Text field ----------

type field struct {
	Ed       widget.Editor
	Label    string
	Hint     string
	Password bool
	eye      widget.Clickable
	shown    bool
}

func newField(label, hint string, password bool, hintKind key.InputHint) *field {
	f := &field{Label: label, Hint: hint, Password: password}
	f.Ed.SingleLine = true
	f.Ed.InputHint = hintKind
	if password {
		f.Ed.Mask = '•'
	}
	return f
}

// newArea is a multi-line text field.
func newArea(label, hint string) *field {
	return &field{Label: label, Hint: hint}
}

func (f *field) Text() string { return strings.TrimSpace(f.Ed.Text()) }

func (t *Theme) field(gtx C, f *field) D {
	if f.eye.Clicked(gtx) {
		f.shown = !f.shown
		if f.shown {
			f.Ed.Mask = 0
		} else {
			f.Ed.Mask = '•'
		}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			if f.Label == "" {
				return D{}
			}
			return layout.Inset{Bottom: 6}.Layout(gtx, func(gtx C) D {
				return t.label(gtx, f.Label, textOpt{size: 13, weight: font.SemiBold, color: t.P.Ink2})
			})
		}),
		layout.Rigid(func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			focused := gtx.Focused(&f.Ed)
			return withBackground(gtx, func(gtx C, size image.Point) {
				fillRound(gtx, size, 13, t.P.Field)
				if focused {
					strokeRound(gtx, size, 13, 2, t.P.Violet)
				} else {
					strokeRound(gtx, size, 13, 1, t.P.Line2)
				}
			}, func(gtx C) D {
				return layout.Inset{Left: 14, Right: 6, Top: 4, Bottom: 4}.Layout(gtx, func(gtx C) D {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx C) D {
							return layout.Inset{Top: 10, Bottom: 10, Right: 8}.Layout(gtx, func(gtx C) D {
								e := material.Editor(t.M, &f.Ed, f.Hint)
								e.Color = t.P.Ink
								e.HintColor = t.P.Faint
								e.SelectionColor = withAlpha(t.P.Violet, 90)
								e.TextSize = 15.5
								return e.Layout(gtx)
							})
						}),
						layout.Rigid(func(gtx C) D {
							if !f.Password {
								return D{}
							}
							ic := icEye
							if f.shown {
								ic = icEyeOff
							}
							return f.eye.Layout(gtx, func(gtx C) D {
								return pad(8).Layout(gtx, func(gtx C) D { return iconSized(gtx, ic, 20, t.P.Muted) })
							})
						}),
					)
				})
			})
		}),
	)
}

// ---------- Containers ----------

// card draws a rounded surface with a soft shadow and border.
func (t *Theme) card(gtx C, inset unit.Dp, w layout.Widget) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return withBackground(gtx, func(gtx C, size image.Point) {
		sh := pushOffset(gtx, image.Pt(0, gtx.Dp(2)))
		fillRound(gtx, size, 18, t.P.Shadow)
		sh.Pop()
		fillRound(gtx, size, 18, t.P.Surface)
		strokeRound(gtx, size, 18, 1, t.P.Line)
	}, func(gtx C) D { return pad(inset).Layout(gtx, w) })
}

// tinted is a flat rounded box (notices, chips, rows).
func tinted(gtx C, bg color.NRGBA, radius, inset unit.Dp, w layout.Widget) D {
	return withBackground(gtx, func(gtx C, size image.Point) { fillRound(gtx, size, radius, bg) },
		func(gtx C) D { return pad(inset).Layout(gtx, w) })
}

// heroCard is the dark gradient banner with a violet glow.
func (t *Theme) heroCard(gtx C, w layout.Widget) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return withBackground(gtx, func(gtx C, size image.Point) {
		gradientRound(gtx, size, 24, heroA, heroB)
		st := rrect(gtx, size, 24).Push(gtx.Ops)
		glow(gtx, image.Pt(size.X, 0), size.Y, argb(0xc98cff, 110))
		glow(gtx, image.Pt(0, size.Y), size.Y*2/3, argb(0xdb2777, 60))
		st.Pop()
	}, func(gtx C) D { return pad(22).Layout(gtx, w) })
}

// ---------- Small pieces ----------

type tone int

const (
	toneNeutral tone = iota
	toneGreen
	toneAmber
	toneRed
	toneViolet
	toneBlue
)

func (t *Theme) toneColors(tn tone) (fg, bg color.NRGBA) {
	switch tn {
	case toneGreen:
		return t.P.Green, t.P.GreenBg
	case toneAmber:
		return t.P.Amber, t.P.AmberBg
	case toneRed:
		return t.P.Red, t.P.RedBg
	case toneViolet:
		return t.P.Accent, t.P.Lav
	case toneBlue:
		return t.P.Blue, t.P.BlueBg
	}
	return t.P.Muted, t.P.Lav
}

func (t *Theme) badge(gtx C, s string, tn tone) D {
	gtx.Constraints.Min.X = 0
	fg, bg := t.toneColors(tn)
	return tinted(gtx, bg, 99, 0, func(gtx C) D {
		return layout.Inset{Left: 10, Right: 10, Top: 4, Bottom: 4}.Layout(gtx, func(gtx C) D {
			return t.label(gtx, s, textOpt{size: 11.5, weight: font.Bold, color: fg, maxLines: 1})
		})
	})
}

func (t *Theme) notice(gtx C, ic *widget.Icon, tn tone, w layout.Widget) D {
	fg, bg := t.toneColors(tn)
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return tinted(gtx, bg, 14, 14, func(gtx C) D {
		return layout.Flex{}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Right: 10, Top: 1}.Layout(gtx, func(gtx C) D { return iconSized(gtx, ic, 19, fg) })
			}),
			layout.Flexed(1, w),
		)
	})
}

func (t *Theme) avatar(gtx C, name string, size unit.Dp) D {
	s := gtx.Dp(size)
	gtx.Constraints = layout.Exact(image.Pt(s, s))
	gradientRound(gtx, image.Pt(s, s), size*0.32, gradA, gradB)
	return layout.Center.Layout(gtx, func(gtx C) D {
		return t.label(gtx, initials(name), textOpt{size: unit.Sp(float32(size) * 0.36), weight: font.Bold, color: white})
	})
}

func (t *Theme) progress(gtx C, frac float32, track, fillC1, fillC2 color.NRGBA) D {
	w, h := gtx.Constraints.Max.X, gtx.Dp(8)
	fillRound(gtx, image.Pt(w, h), 4, track)
	fw := int(float32(w) * max(0.03, min(1, frac)))
	gradientRound(gtx, image.Pt(fw, h), 4, fillC1, fillC2)
	return D{Size: image.Pt(w, h)}
}

func (t *Theme) divider(gtx C) D {
	w := gtx.Constraints.Max.X
	paint.FillShape(gtx.Ops, t.P.Line, clip.Rect{Max: image.Pt(w, gtx.Dp(1))}.Op())
	return D{Size: image.Pt(w, gtx.Dp(1))}
}

// sectionTitle is a heading with an optional right-side action.
func (t *Theme) sectionTitle(gtx C, s string, action layout.Widget) D {
	return layout.Inset{Top: 22, Bottom: 10}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx C) D { return t.title(gtx, s, 18) }),
			layout.Rigid(func(gtx C) D {
				if action == nil {
					return D{}
				}
				return action(gtx)
			}),
		)
	})
}

func (t *Theme) linkText(gtx C, clk *widget.Clickable, s string) D {
	return clk.Layout(gtx, func(gtx C) D {
		return t.label(gtx, s, textOpt{size: 13.5, weight: font.Bold, color: t.P.Accent})
	})
}

// imageBox shows an image cropped to fill the given size with rounded corners.
func imageBox(gtx C, op paint.ImageOp, size image.Point, radius unit.Dp) D {
	gtx.Constraints = layout.Exact(size)
	defer rrect(gtx, size, radius).Push(gtx.Ops).Pop()
	return widget.Image{Src: op, Fit: widget.Cover, Position: layout.Center}.Layout(gtx)
}

func centered(gtx C, s string, t *Theme, o textOpt) D {
	o.align = text.Middle
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return t.label(gtx, s, o)
}
