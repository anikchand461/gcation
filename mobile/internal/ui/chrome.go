package ui

import (
	"fmt"
	"image"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
)

// ---------- Top app bar ----------

// appBar is the native-style top bar: the logo on the home screen, a large title elsewhere,
// and round action buttons on the right.
func (a *App) appBar(gtx C, title string, logo bool, actions ...layout.FlexChild) D {
	return layout.Inset{Left: 18, Right: 12, Top: 10, Bottom: 10}.Layout(gtx, func(gtx C) D {
		kids := []layout.FlexChild{
			layout.Flexed(1, func(gtx C) D {
				if logo {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx C) D { return imageBox(gtx, a.logo, image.Pt(gtx.Dp(34), gtx.Dp(34)), 0) }),
						layout.Rigid(hspace(8)),
						layout.Rigid(func(gtx C) D { return a.wordmark(gtx, 21) }),
					)
				}
				return a.th.label(gtx, title, textOpt{size: 26, weight: font.Bold, maxLines: 1})
			}),
		}
		for i, ac := range actions {
			if i > 0 {
				kids = append(kids, layout.Rigid(hspace(6)))
			}
			kids = append(kids, ac)
		}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, kids...)
	})
}

// barIcon is a flat round icon button for the top bar (with an optional red count badge).
func (a *App) barIcon(clk *widget.Clickable, ic *widget.Icon, count int) layout.FlexChild {
	return layout.Rigid(func(gtx C) D {
		return clk.Layout(gtx, func(gtx C) D {
			s := gtx.Dp(44)
			gtx.Constraints = layout.Exact(image.Pt(s, s))
			if clk.Pressed() {
				fillRound(gtx, image.Pt(s, s), 22, a.th.P.Lav)
			}
			layout.Center.Layout(gtx, func(gtx C) D { return iconSized(gtx, ic, 24, a.th.P.Ink2) })
			if count > 0 {
				d := gtx.Dp(17)
				st := pushOffset(gtx, image.Pt(s-d-gtx.Dp(3), gtx.Dp(4)))
				fillRound(gtx, image.Pt(d, d), 9, rgb(0xe5484d))
				gtx2 := gtx
				gtx2.Constraints = layout.Exact(image.Pt(d, d))
				n := "9+"
				if count < 10 {
					n = string(rune('0' + count))
				}
				layout.Center.Layout(gtx2, func(gtx C) D {
					return a.th.label(gtx, n, textOpt{size: 9.5, weight: font.Bold, color: white})
				})
				st.Pop()
			}
			return D{Size: image.Pt(s, s)}
		})
	})
}

// subBar is the top bar of a pushed screen: back arrow + title.
func (a *App) subBar(gtx C, back *widget.Clickable, title string) D {
	return layout.Inset{Left: 6, Right: 16, Top: 8, Bottom: 8}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			a.barIcon(back, icBack, 0),
			layout.Rigid(hspace(6)),
			layout.Flexed(1, func(gtx C) D { return a.th.label(gtx, title, textOpt{size: 21, weight: font.Bold, maxLines: 1}) }),
		)
	})
}

// ---------- Bottom navigation ----------

type navItem struct {
	ic    *widget.Icon
	label string
	dot   bool
}

// navBar is the Material-style bottom bar: a tinted pill behind the active icon, label below.
func (a *App) navBar(gtx C, items []navItem, active int, clicks []widget.Clickable) D {
	return withBackground(gtx, func(gtx C, size image.Point) {
		paint.FillShape(gtx.Ops, a.th.P.Nav, rrect(gtx, size, 0).Op(gtx.Ops))
		paint.FillShape(gtx.Ops, a.th.P.Line, rrect(gtx, image.Pt(size.X, gtx.Dp(1)), 0).Op(gtx.Ops))
	}, func(gtx C) D {
		return layout.Inset{Top: 10, Bottom: 12}.Layout(gtx, func(gtx C) D {
			kids := make([]layout.FlexChild, len(items))
			for i, it := range items {
				on := i == active
				kids[i] = layout.Flexed(1, func(gtx C) D {
					return clicks[i].Layout(gtx, func(gtx C) D {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						return layout.N.Layout(gtx, func(gtx C) D {
							gtx.Constraints.Min.X = 0
							return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
								layout.Rigid(func(gtx C) D {
									w, h := gtx.Dp(60), gtx.Dp(32)
									gtx.Constraints = layout.Exact(image.Pt(w, h))
									if on {
										fillRound(gtx, image.Pt(w, h), 16, a.th.P.Lav)
									}
									fg := a.th.P.Muted
									if on {
										fg = a.th.P.Accent
									}
									layout.Center.Layout(gtx, func(gtx C) D { return iconSized(gtx, it.ic, 23, fg) })
									if it.dot && !on {
										d := gtx.Dp(9)
										st := pushOffset(gtx, image.Pt(w/2+gtx.Dp(7), gtx.Dp(3)))
										fillRound(gtx, image.Pt(d, d), 5, rgb(0xe5484d))
										st.Pop()
									}
									return D{Size: image.Pt(w, h)}
								}),
								layout.Rigid(vspace(4)),
								layout.Rigid(func(gtx C) D {
									c, w := a.th.P.Muted, font.Medium
									if on {
										c, w = a.th.P.Ink, font.Bold
									}
									return a.th.label(gtx, it.label, textOpt{size: 12, weight: w, color: c, maxLines: 1})
								}),
							)
						})
					})
				})
			}
			return layout.Flex{}.Layout(gtx, kids...)
		})
	})
}

// ---------- Confirm dialog ----------

type dialogState struct {
	title, message, ok string
	danger             bool
	onOK               func()
	cancelBtn, okBtn   widget.Clickable
	scrim              widget.Clickable
}

// confirm shows a centred dialog; onOK runs only if the user taps the confirm button.
func (a *App) confirm(title, message, ok string, danger bool, onOK func()) {
	a.dialog = &dialogState{title: title, message: message, ok: ok, danger: danger, onOK: onOK}
}

func (a *App) layoutDialog(gtx C) D {
	d := a.dialog
	if d == nil {
		return D{}
	}
	if d.cancelBtn.Clicked(gtx) || d.scrim.Clicked(gtx) {
		a.dialog = nil
		return D{}
	}
	if d.okBtn.Clicked(gtx) {
		a.dialog = nil
		d.onOK()
		return D{}
	}
	d.scrim.Layout(gtx, func(gtx C) D {
		paint.FillShape(gtx.Ops, argb(0x0b0726, 150), rrect(gtx, gtx.Constraints.Max, 0).Op(gtx.Ops))
		return D{Size: gtx.Constraints.Max}
	})
	return layout.Center.Layout(gtx, func(gtx C) D {
		return layout.Inset{Left: 24, Right: 24}.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(420))
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return withBackground(gtx, func(gtx C, size image.Point) {
				fillRound(gtx, size, 26, a.th.P.Surface)
			}, func(gtx C) D {
				return pad(22).Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx C) D { return a.th.title(gtx, d.title, 20) }),
						layout.Rigid(vspace(8)),
						layout.Rigid(func(gtx C) D { return a.th.label(gtx, d.message, textOpt{size: 14.5, color: a.th.P.Muted}) }),
						layout.Rigid(vspace(20)),
						layout.Rigid(func(gtx C) D {
							kind := btnPrimary
							if d.danger {
								kind = btnDanger
							}
							return layout.Flex{}.Layout(gtx,
								layout.Flexed(1, func(gtx C) D { return a.th.button(gtx, &d.cancelBtn, "Cancel", btnOpt{full: true, kind: btnGhost}) }),
								layout.Rigid(hspace(8)),
								layout.Flexed(1, func(gtx C) D { return a.th.button(gtx, &d.okBtn, d.ok, btnOpt{full: true, kind: kind}) }),
							)
						}),
					)
				})
			})
		})
	})
}

// ---------- Shared bits ----------

// chips is a horizontal row of selectable pills (filters, presets, course pickers).
func (a *App) chips(gtx C, key string, labels []string, selected func(i int) bool, clicks func(i int) *widget.Clickable) D {
	l := a.list("chips:" + key)
	l.Axis = layout.Horizontal
	return l.List.Layout(gtx, len(labels), func(gtx C, i int) D {
		on := selected(i)
		return layout.Inset{Right: 8}.Layout(gtx, func(gtx C) D {
			return clicks(i).Layout(gtx, func(gtx C) D {
				return withBackground(gtx, func(gtx C, size image.Point) {
					if on {
						fillRound(gtx, size, 99, a.th.P.Ink)
					} else {
						fillRound(gtx, size, 99, a.th.P.Surface)
						strokeRound(gtx, size, 99, 1, a.th.P.Line2)
					}
				}, func(gtx C) D {
					return layout.Inset{Left: 14, Right: 14, Top: 8, Bottom: 8}.Layout(gtx, func(gtx C) D {
						c := a.th.P.Ink2
						if on {
							c = a.th.P.Bg
						}
						return a.th.label(gtx, labels[i], textOpt{size: 13.5, weight: font.SemiBold, color: c, maxLines: 1})
					})
				})
			})
		})
	})
}

// dropdown is a labelled select box: tapping it opens the options in a list below it.
// selected is the chosen index, or -1 to show hint.
func (a *App) dropdown(gtx C, key, label, hint string, options []string, selected *int, open *bool) D {
	toggle := a.clk("dd:" + key)
	if toggle.Clicked(gtx) {
		*open = !*open
	}
	for i := range options {
		if a.clk(fmt.Sprint("dd:", key, ":", i)).Clicked(gtx) {
			*selected, *open = i, false
		}
	}
	text, color := hint, a.th.P.Faint
	if *selected >= 0 && *selected < len(options) {
		text, color = options[*selected], a.th.P.Ink
	}
	box := func(gtx C) D {
		return toggle.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return withBackground(gtx, func(gtx C, size image.Point) {
				fillRound(gtx, size, 13, a.th.P.Field)
				if *open {
					strokeRound(gtx, size, 13, 2, a.th.P.Violet)
				} else {
					strokeRound(gtx, size, 13, 1, a.th.P.Line2)
				}
			}, func(gtx C) D {
				return layout.Inset{Left: 14, Right: 12, Top: 14, Bottom: 14}.Layout(gtx, func(gtx C) D {
					ic := icExpand
					if *open {
						ic = icCollapse
					}
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx C) D { return a.th.label(gtx, text, textOpt{size: 15.5, color: color, maxLines: 1}) }),
						layout.Rigid(func(gtx C) D { return iconSized(gtx, ic, 22, a.th.P.Muted) }),
					)
				})
			})
		})
	}
	menu := func(gtx C) D {
		if !*open {
			return D{}
		}
		return layout.Inset{Top: 6}.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return withBackground(gtx, func(gtx C, size image.Point) {
				fillRound(gtx, size, 13, a.th.P.Surface)
				strokeRound(gtx, size, 13, 1, a.th.P.Line2)
			}, func(gtx C) D {
				rows := make([]layout.FlexChild, len(options))
				for i, o := range options {
					clk, on := a.clk(fmt.Sprint("dd:", key, ":", i)), i == *selected
					rows[i] = layout.Rigid(func(gtx C) D {
						return clk.Layout(gtx, func(gtx C) D {
							gtx.Constraints.Min.X = gtx.Constraints.Max.X
							return withBackground(gtx, func(gtx C, size image.Point) {
								if on || clk.Pressed() {
									fillRound(gtx, size, 10, a.th.P.Lav)
								}
							}, func(gtx C) D {
								return layout.Inset{Left: 14, Right: 14, Top: 12, Bottom: 12}.Layout(gtx, func(gtx C) D {
									return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
										layout.Flexed(1, func(gtx C) D {
											return a.th.label(gtx, o, textOpt{size: 15, weight: font.Medium, maxLines: 1})
										}),
										layout.Rigid(func(gtx C) D {
											if !on {
												return D{}
											}
											return iconSized(gtx, icTick, 18, a.th.P.Accent)
										}),
									)
								})
							})
						})
					})
				}
				return pad(4).Layout(gtx, func(gtx C) D { return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...) })
			})
		})
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Bottom: 6}.Layout(gtx, func(gtx C) D {
				return a.th.label(gtx, label, textOpt{size: 13, weight: font.SemiBold, color: a.th.P.Ink2})
			})
		}),
		layout.Rigid(box),
		layout.Rigid(menu),
	)
}

// listRow is a tappable full-width row (avatar or icon, two lines of text, trailing widget).
func (a *App) listRow(gtx C, clk *widget.Clickable, lead layout.Widget, title, sub string, trail layout.Widget) D {
	body := func(gtx C) D {
		return layout.Inset{Top: 12, Bottom: 12}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(lead),
				layout.Rigid(hspace(12)),
				layout.Flexed(1, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx C) D { return a.th.label(gtx, title, textOpt{size: 15, weight: font.SemiBold, maxLines: 1}) }),
						layout.Rigid(func(gtx C) D {
							if sub == "" {
								return D{}
							}
							return a.th.label(gtx, sub, textOpt{size: 12.5, color: a.th.P.Muted, maxLines: 2})
						}),
					)
				}),
				layout.Rigid(func(gtx C) D {
					if trail == nil {
						return D{}
					}
					return layout.Inset{Left: 8}.Layout(gtx, trail)
				}),
			)
		})
	}
	if clk == nil {
		return body(gtx)
	}
	return clk.Layout(gtx, body)
}

// group wraps rows in one rounded surface with dividers between them (settings-style).
func (a *App) group(gtx C, rows []layout.Widget) D {
	return a.th.card(gtx, 0, func(gtx C) D {
		kids := make([]layout.FlexChild, 0, len(rows)*2)
		for i, r := range rows {
			if i > 0 {
				kids = append(kids, layout.Rigid(func(gtx C) D {
					return layout.Inset{Left: 16}.Layout(gtx, func(gtx C) D { return a.th.divider(gtx) })
				}))
			}
			kids = append(kids, layout.Rigid(func(gtx C) D { return layout.Inset{Left: 16, Right: 12}.Layout(gtx, r) }))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
	})
}

func (a *App) groupLabel(s string) layout.Widget {
	return func(gtx C) D {
		return layout.Inset{Top: 20, Bottom: 8, Left: 4}.Layout(gtx, func(gtx C) D {
			return a.th.label(gtx, s, textOpt{size: 12.5, weight: font.Bold, color: a.th.P.Muted})
		})
	}
}

func (a *App) iconTile(gtx C, ic *widget.Icon, size unit.Dp) D {
	s := gtx.Dp(size)
	gtx.Constraints = layout.Exact(image.Pt(s, s))
	fillRound(gtx, image.Pt(s, s), size*0.3, a.th.P.Lav)
	return layout.Center.Layout(gtx, func(gtx C) D { return iconSized(gtx, ic, size*0.5, a.th.P.Accent) })
}
