package ui

import (
	"encoding/base64"
	"fmt"
	"image"
	"strconv"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/widget"

	"gcation/mobile/internal/api"
)

type checkoutState struct {
	courseID int64
	step     int  // 1 pay, 2 share details, 3 call
	resend   bool // re-open the steps after details were already sent
	slot     string
	busy     bool
	err      string

	back, iPaid, prevStep, send, mail, copyMail, home, copyUPI, payApp, again widget.Clickable
	call, copyPhone                                                           [2]widget.Clickable
	seg                                                                       [2]widget.Clickable
	pick                                                                      [2]widget.Clickable
	pickShot                                                                  widget.Clickable
	payer, methodOther, amount, date, ref, age, referredBy, referralID        *field
	method                                                                    int // index into payApps, -1 until chosen
	methodOpen                                                                bool
	shot                                                                      []byte        // payment screenshot to upload
	shotPreview                                                               paint.ImageOp // the same screenshot, shown on the form
}

// payApps fills the "Paid using" dropdown; the last entry asks the student to type it.
var payApps = []string{"Google Pay", "PhonePe", "Paytm", "BHIM UPI", "Amazon Pay", "CRED", "WhatsApp Pay", "MobiKwik",
	"Navi", "super.money", "Bank app (YONO SBI, iMobile, etc.)", "Net banking / IMPS / NEFT", "Other"}

func (s *checkoutState) otherMethod() bool { return s.method == len(payApps)-1 }

func (s *checkoutState) init(a *App, c api.Course) {
	s.step = 1
	s.payer = newField("Name on bank / UPI account", "", false, key.HintAny)
	s.payer.Ed.SetText(a.me.Student.Name)
	s.method = -1
	s.methodOther = newField("Which app or bank?", "e.g. Kotak 811", false, key.HintAny)
	s.age = newField("Your age", "", false, key.HintNumeric)
	s.age.Ed.Filter = "0123456789"
	s.referredBy = newField("Referred by (optional)", "Name of the person who told you about us", false, key.HintAny)
	s.referralID = newField("Referral ID (optional)", "e.g. GCM001", false, key.HintAny)
	s.amount = newField("Amount paid (in rupees)", "", false, key.HintNumeric)
	s.amount.Ed.SetText(strconv.FormatFloat(c.Price, 'f', -1, 64))
	s.amount.Ed.Filter = "0123456789."
	s.date = newField("Date paid (YYYY-MM-DD)", "", false, key.HintAny)
	s.date.Ed.SetText(time.Now().In(ist).Format("2006-01-02"))
	s.ref = newField("UTR / transaction ID", "12-digit number from your UPI app", false, key.HintAny)
	if m := a.me.Payment.Methods; len(m) > 0 {
		s.slot = m[0].Slot
	}
}

func (a *App) currentCourse() (api.Course, bool) {
	for _, c := range a.me.Courses {
		if c.ID == a.checkout.courseID {
			return c, true
		}
	}
	return api.Course{}, false
}

func (a *App) layoutCheckout(gtx C) D {
	s := a.checkout
	c, ok := a.currentCourse()
	if !ok {
		a.checkout = nil
		return D{}
	}
	if s.back.Clicked(gtx) {
		a.checkout = nil
		return D{}
	}
	showStatus := c.PendingClaim != nil && !s.resend && c.Access.Status != "active"

	items := []layout.Widget{vspace(4), func(gtx C) D { return a.checkoutSummary(gtx, c) }}
	if c.Access.Status == "active" {
		items = append(items, vspace(12), func(gtx C) D {
			return a.th.notice(gtx, icInfo, toneBlue, func(gtx C) D {
				return a.th.label(gtx, fmt.Sprintf("You already have access until %s. Paying again adds %s after that date.",
					fmtDate(c.Access.Until), durationLabel(c.DefaultDays)), textOpt{size: 13.5, color: a.th.P.Blue})
			})
		})
	}
	if showStatus {
		items = append(items, a.statusPanel(gtx, c)...)
	} else {
		items = append(items, vspace(16), func(gtx C) D { return a.wizardHead(gtx, s.step) }, vspace(14))
		switch s.step {
		case 2:
			items = append(items, func(gtx C) D { return a.stepDetails(gtx, c) })
		case 3:
			items = append(items, func(gtx C) D { return a.stepCall(gtx, c) })
		default:
			items = append(items, func(gtx C) D { return a.stepPay(gtx, c) })
		}
	}

	return a.safeArea(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Left: 12, Right: 16, Top: 8, Bottom: 8}.Layout(gtx, func(gtx C) D {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx C) D { return a.th.iconButton(gtx, &s.back, icBack, 0) }),
						layout.Rigid(hspace(12)),
						layout.Flexed(1, func(gtx C) D {
							t := "Enroll"
							if showStatus {
								t = "Payment status"
							} else if c.Access.Status != "none" {
								t = "Renew"
							}
							return a.th.title(gtx, t, 20)
						}),
					)
				})
			}),
			layout.Flexed(1, func(gtx C) D { return a.page(gtx, fmt.Sprint("checkout", c.ID), items) }),
		)
	})
}

func (a *App) checkoutSummary(gtx C, c api.Course) D {
	return a.th.card(gtx, 14, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return a.thumb(gtx, c, image.Pt(gtx.Dp(64), gtx.Dp(64)), 16) }),
					layout.Rigid(hspace(14)),
					layout.Flexed(1, func(gtx C) D {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx C) D { return a.th.label(gtx, c.Title, textOpt{size: 16.5, weight: font.Bold, maxLines: 2}) }),
							layout.Rigid(func(gtx C) D { return a.th.muted(gtx, durationLabel(c.DefaultDays)+" access", 13) }),
						)
					}),
					layout.Rigid(func(gtx C) D { return a.th.money(gtx, c.Price, textOpt{size: 19, weight: font.Black}) }),
				)
			}),
			layout.Rigid(func(gtx C) D {
				if len(c.Features) == 0 {
					return D{}
				}
				return layout.Inset{Top: 14}.Layout(gtx, func(gtx C) D { return a.featureList(gtx, c.Features, 0) })
			}),
		)
	})
}

func (a *App) wizardHead(gtx C, step int) D {
	labels := []string{"Pay", "Details", "Confirm"}
	kids := make([]layout.FlexChild, 0, 5)
	for i, l := range labels {
		n := i + 1
		if i > 0 {
			kids = append(kids, layout.Rigid(hspace(6)))
		}
		kids = append(kids, layout.Flexed(1, func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			now, done := n == step, n < step
			return withBackground(gtx, func(gtx C, size image.Point) {
				fillRound(gtx, size, 12, a.th.P.Surface)
				if now {
					strokeRound(gtx, size, 12, 2, a.th.P.Violet)
				} else {
					strokeRound(gtx, size, 12, 1, a.th.P.Line)
				}
			}, func(gtx C) D {
				return layout.Inset{Top: 9, Bottom: 9, Left: 10, Right: 6}.Layout(gtx, func(gtx C) D {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							s := gtx.Dp(24)
							gtx.Constraints = layout.Exact(image.Pt(s, s))
							switch {
							case done:
								fillRound(gtx, image.Pt(s, s), 12, a.th.P.Green)
							case now:
								gradientRound(gtx, image.Pt(s, s), 12, gradA, gradB)
							default:
								fillRound(gtx, image.Pt(s, s), 12, a.th.P.Lav)
							}
							return layout.Center.Layout(gtx, func(gtx C) D {
								if done {
									return iconSized(gtx, icTick, 15, white)
								}
								fg := a.th.P.Accent
								if now {
									fg = white
								}
								return a.th.label(gtx, fmt.Sprint(n), textOpt{size: 12, weight: font.Bold, color: fg})
							})
						}),
						layout.Rigid(hspace(7)),
						layout.Flexed(1, func(gtx C) D {
							fg := a.th.P.Faint
							switch {
							case now:
								fg = a.th.P.Ink
							case done:
								fg = a.th.P.Green
							}
							return a.th.label(gtx, l, textOpt{size: 13, weight: font.Bold, color: fg, maxLines: 1})
						}),
					)
				})
			})
		}))
	}
	return layout.Flex{}.Layout(gtx, kids...)
}

func (a *App) method(slot string) (api.Method, bool) {
	for _, m := range a.me.Payment.Methods {
		if m.Slot == slot {
			return m, true
		}
	}
	return api.Method{}, false
}

func (a *App) stepPay(gtx C, c api.Course) D {
	s := a.checkout
	methods := a.me.Payment.Methods
	if len(methods) == 0 {
		return a.th.notice(gtx, icAlert, toneAmber, func(gtx C) D {
			return a.th.label(gtx, "Payment details are being updated. Please call the G-CATION team to pay.", textOpt{size: 13.5, color: a.th.P.Amber})
		})
	}
	m, ok := a.method(s.slot)
	if !ok {
		m, s.slot = methods[0], methods[0].Slot
	}
	for i := range methods {
		if i < 2 && s.seg[i].Clicked(gtx) {
			s.slot = methods[i].Slot
		}
	}
	if s.copyUPI.Clicked(gtx) {
		a.copyText(gtx, m.UPIID, "UPI ID copied")
	}
	if s.payApp.Clicked(gtx) {
		a.open(upiURL(m.UPIID, m.PayeeName, c.Price, a.upiNote()))
	}
	if s.iPaid.Clicked(gtx) {
		s.step = 2
	}

	return a.th.card(gtx, 16, func(gtx C) D {
		kids := []layout.FlexChild{}
		if len(methods) > 1 {
			kids = append(kids, layout.Rigid(func(gtx C) D { return a.segmented(gtx, methods) }), layout.Rigid(vspace(12)))
		}
		kids = append(kids,
			layout.Rigid(func(gtx C) D { return a.qrFrame(gtx, c, m) }),
			layout.Rigid(func(gtx C) D {
				if len(methods) < 2 {
					return D{}
				}
				return layout.Inset{Top: 8}.Layout(gtx, func(gtx C) D {
					return centered(gtx, "QR not working? Try the other one above.", a.th, textOpt{size: 12.5, color: a.th.P.Muted})
				})
			}),
			layout.Rigid(vspace(16)),
			layout.Rigid(func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx C) D { return a.th.muted(gtx, "Amount to pay", 14) }),
					layout.Rigid(func(gtx C) D { return a.th.money(gtx, c.Price, textOpt{size: 28, weight: font.Black}) }),
				)
			}),
		)
		if m.UPIID != "" {
			kids = append(kids,
				layout.Rigid(vspace(12)),
				layout.Rigid(func(gtx C) D {
					return withBackground(gtx, func(gtx C, size image.Point) {
						fillRound(gtx, size, 12, a.th.P.Lav)
					}, func(gtx C) D {
						return layout.Inset{Left: 14, Right: 8, Top: 8, Bottom: 8}.Layout(gtx, func(gtx C) D {
							return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
								layout.Flexed(1, func(gtx C) D {
									label := "UPI ID"
									if m.PayeeName != "" {
										label += " · " + m.PayeeName
									}
									return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
										layout.Rigid(func(gtx C) D { return a.th.muted(gtx, label, 11.5) }),
										layout.Rigid(func(gtx C) D { return a.th.label(gtx, m.UPIID, textOpt{size: 15, weight: font.Bold}) }),
									)
								}),
								layout.Rigid(func(gtx C) D {
									return a.th.button(gtx, &s.copyUPI, "Copy", btnOpt{kind: btnOutline, small: true, icon: icCopy})
								}),
							)
						})
					})
				}),
				layout.Rigid(vspace(12)),
				layout.Rigid(func(gtx C) D { return a.th.button(gtx, &s.payApp, "Pay with UPI app", btnOpt{full: true, icon: icPay}) }),
			)
		}
		kids = append(kids,
			layout.Rigid(vspace(16)),
			layout.Rigid(func(gtx C) D {
				return a.steps(gtx, []string{"Scan the QR or tap “Pay with UPI app”.", "Pay exactly " + rupees(c.Price) + ".", "Keep the payment screenshot or UTR number."})
			}),
			layout.Rigid(vspace(16)),
			layout.Rigid(func(gtx C) D {
				return a.th.button(gtx, &s.iPaid, "I have paid  →", btnOpt{full: true, kind: btnOutline})
			}),
		)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
	})
}

func (a *App) segmented(gtx C, methods []api.Method) D {
	s := a.checkout
	return tinted(gtx, a.th.P.Lav, 13, 4, func(gtx C) D {
		kids := []layout.FlexChild{}
		for i, m := range methods {
			if i >= 2 {
				break
			}
			if i > 0 {
				kids = append(kids, layout.Rigid(hspace(4)))
			}
			on := m.Slot == s.slot
			label := m.Label
			if label == "" {
				label = map[string]string{"primary": "Primary QR", "secondary": "Secondary QR"}[m.Slot]
			}
			kids = append(kids, layout.Flexed(1, func(gtx C) D {
				return s.seg[i].Layout(gtx, func(gtx C) D {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return withBackground(gtx, func(gtx C, size image.Point) {
						if on {
							fillRound(gtx, size, 10, a.th.P.Surface)
						}
					}, func(gtx C) D {
						return layout.Inset{Top: 10, Bottom: 10, Left: 6, Right: 6}.Layout(gtx, func(gtx C) D {
							fg := a.th.P.Muted
							if on {
								fg = a.th.P.Accent
							}
							return centered(gtx, label, a.th, textOpt{size: 13.5, weight: font.Bold, color: fg, maxLines: 1})
						})
					})
				})
			}))
		}
		return layout.Flex{}.Layout(gtx, kids...)
	})
}

// qrFrame shows the uploaded QR image, or a QR made in the app from the UPI ID (amount included).
func (a *App) qrFrame(gtx C, c api.Course, m api.Method) D {
	side := min(gtx.Constraints.Max.X, gtx.Dp(300))
	return layout.Center.Layout(gtx, func(gtx C) D {
		gtx.Constraints = layout.Exact(image.Pt(side, side))
		fillRound(gtx, image.Pt(side, side), 20, white)
		strokeRound(gtx, image.Pt(side, side), 20, 1, a.th.P.Line)
		inner := side - gtx.Dp(28)
		st := pushOffset(gtx, image.Pt(gtx.Dp(14), gtx.Dp(14)))
		defer st.Pop()
		size := image.Pt(inner, inner)
		if m.HasImage {
			if op, ok := a.image(m.ImagePath()); ok {
				gtx.Constraints = layout.Exact(size)
				widget.Image{Src: op, Fit: widget.Contain, Position: layout.Center}.Layout(gtx)
			} else {
				gtx.Constraints = layout.Exact(size)
				layout.Center.Layout(gtx, func(gtx C) D { return a.th.label(gtx, "Loading QR…", textOpt{size: 13, color: rgb(0x67628a)}) })
			}
		} else if m.UPIID != "" {
			gtx.Constraints = layout.Exact(size)
			widget.Image{Src: a.qr(upiURL(m.UPIID, m.PayeeName, c.Price, a.upiNote())), Fit: widget.Contain, Position: layout.Center}.Layout(gtx)
		}
		return D{Size: image.Pt(side, side)}
	})
}

func (a *App) steps(gtx C, lines []string) D {
	kids := make([]layout.FlexChild, 0, len(lines))
	for i, l := range lines {
		kids = append(kids, layout.Rigid(func(gtx C) D {
			return layout.Inset{Bottom: 8}.Layout(gtx, func(gtx C) D {
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						s := gtx.Dp(22)
						gtx.Constraints = layout.Exact(image.Pt(s, s))
						fillRound(gtx, image.Pt(s, s), 11, a.th.P.Lav)
						return layout.Center.Layout(gtx, func(gtx C) D {
							return a.th.label(gtx, fmt.Sprint(i+1), textOpt{size: 11, weight: font.Bold, color: a.th.P.Accent})
						})
					}),
					layout.Rigid(hspace(10)),
					layout.Flexed(1, func(gtx C) D { return a.th.label(gtx, l, textOpt{size: 14, color: a.th.P.Ink2}) }),
				)
			})
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
}

func (a *App) stepDetails(gtx C, c api.Course) D {
	s := a.checkout
	methods := a.me.Payment.Methods
	for i := range methods {
		if i < 2 && s.pick[i].Clicked(gtx) {
			s.slot = methods[i].Slot
		}
	}
	if s.prevStep.Clicked(gtx) {
		s.step = 1
	}
	if s.send.Clicked(gtx) && !s.busy {
		a.sendClaim(c)
	}
	if s.pickShot.Clicked(gtx) {
		a.pickScreenshot()
	}
	return a.th.card(gtx, 18, func(gtx C) D {
		kids := []layout.FlexChild{
			layout.Rigid(func(gtx C) D { return a.th.title(gtx, "Share your payment details", 18) }),
			layout.Rigid(vspace(4)),
			layout.Rigid(func(gtx C) D { return a.th.muted(gtx, "This helps us find your payment in seconds.", 13.5) }),
		}
		if len(methods) > 1 {
			kids = append(kids, layout.Rigid(vspace(14)),
				layout.Rigid(func(gtx C) D {
					return a.th.label(gtx, "Which QR did you pay to?", textOpt{size: 13, weight: font.SemiBold, color: a.th.P.Ink2})
				}),
				layout.Rigid(vspace(6)),
				layout.Rigid(func(gtx C) D {
					ch := []layout.FlexChild{}
					for i, m := range methods {
						if i >= 2 {
							break
						}
						if i > 0 {
							ch = append(ch, layout.Rigid(hspace(8)))
						}
						on := m.Slot == s.slot
						ch = append(ch, layout.Flexed(1, func(gtx C) D {
							return s.pick[i].Layout(gtx, func(gtx C) D {
								gtx.Constraints.Min.X = gtx.Constraints.Max.X
								return withBackground(gtx, func(gtx C, size image.Point) {
									if on {
										fillRound(gtx, size, 12, a.th.P.Lav)
										strokeRound(gtx, size, 12, 2, a.th.P.Violet)
									} else {
										fillRound(gtx, size, 12, a.th.P.Field)
										strokeRound(gtx, size, 12, 1, a.th.P.Line2)
									}
								}, func(gtx C) D {
									return pad(12).Layout(gtx, func(gtx C) D {
										return centered(gtx, m.Label, a.th, textOpt{size: 13.5, weight: font.Bold, maxLines: 2})
									})
								})
							})
						}))
					}
					return layout.Flex{}.Layout(gtx, ch...)
				}))
		}
		fields := func(fs ...*field) {
			for _, f := range fs {
				kids = append(kids, layout.Rigid(vspace(12)), layout.Rigid(func(gtx C) D { return a.th.field(gtx, f) }))
			}
		}
		fields(s.payer)
		kids = append(kids, layout.Rigid(vspace(12)), layout.Rigid(func(gtx C) D {
			return a.dropdown(gtx, "payapp", "Paid using", "Select the app you paid with", payApps, &s.method, &s.methodOpen)
		}))
		if s.otherMethod() {
			fields(s.methodOther)
		}
		fields(s.amount, s.date, s.ref, s.age, s.referredBy, s.referralID)
		kids = append(kids, layout.Rigid(vspace(14)), layout.Rigid(func(gtx C) D { return a.screenshotPicker(gtx) }))
		kids = append(kids,
			layout.Rigid(func(gtx C) D {
				if s.err == "" {
					return D{}
				}
				return layout.Inset{Top: 12}.Layout(gtx, func(gtx C) D {
					return a.th.notice(gtx, icAlert, toneRed, func(gtx C) D { return a.th.label(gtx, s.err, textOpt{size: 13.5, color: a.th.P.Red}) })
				})
			}),
			layout.Rigid(vspace(18)),
			layout.Rigid(func(gtx C) D {
				return a.th.button(gtx, &s.send, "Send details", btnOpt{full: true, icon: icSend, busy: s.busy})
			}),
			layout.Rigid(vspace(8)),
			layout.Rigid(func(gtx C) D { return a.th.button(gtx, &s.prevStep, "Back", btnOpt{full: true, kind: btnGhost}) }),
		)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
	})
}

func (a *App) sendClaim(c api.Course) {
	s := a.checkout
	in := api.ClaimIn{
		CourseID: c.ID, QRSlot: s.slot, PayerName: s.payer.Text(),
		PaidOn: s.date.Text(), Reference: s.ref.Text(), ReferredBy: s.referredBy.Text(),
		ReferralID: strings.ToUpper(strings.ReplaceAll(s.referralID.Text(), " ", "")),
	}
	switch {
	case s.method < 0:
		s.err = "Choose the app you paid with."
		return
	case s.otherMethod():
		in.Method = s.methodOther.Text()
	default:
		in.Method = payApps[s.method]
	}
	if in.Method == "" {
		s.err = "Type the app or bank you paid with."
		return
	}
	if len(in.Reference) < 6 {
		s.err = "Enter the UTR / transaction ID from your payment app."
		return
	}
	age, err := strconv.Atoi(s.age.Text())
	if err != nil || age < 5 || age > 100 {
		s.err = "Enter your age (5 to 100)."
		return
	}
	in.Age = &age
	if in.ReferralID != "" && !referralRE.MatchString(in.ReferralID) {
		s.err = "Referral ID looks like GCM001. Check it, or leave it empty."
		return
	}
	if len(s.shot) == 0 {
		s.err = "Upload the screenshot of your payment."
		return
	}
	in.Screenshot = base64.StdEncoding.EncodeToString(s.shot)
	if amt := strings.TrimSpace(s.amount.Ed.Text()); amt != "" {
		v, err := strconv.ParseFloat(amt, 64)
		if err != nil {
			s.err = "Enter the amount as a number."
			return
		}
		in.Amount = &v
	}
	s.busy, s.err = true, ""
	a.async(func() func() {
		err := a.api.SendClaim(in)
		me, merr := a.api.Me()
		return func() {
			s.busy = false
			if err != nil {
				s.err = err.Error()
				return
			}
			if merr == nil {
				a.me = me
			}
			s.step, s.resend = 3, true
			a.toast("Details sent. Now give us a quick call.", false)
		}
	})
}

func (a *App) stepCall(gtx C, c api.Course) D {
	s := a.checkout
	if s.home.Clicked(gtx) {
		a.checkout = nil
		a.tab = tabHome
	}
	return a.th.card(gtx, 20, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return tinted(gtx, a.th.P.GreenBg, 22, 16, func(gtx C) D { return iconSized(gtx, icCheck, 34, a.th.P.Green) })
			}),
			layout.Rigid(vspace(12)),
			layout.Rigid(func(gtx C) D { return centered(gtx, "Almost done!", a.th, textOpt{size: 21, weight: font.Bold}) }),
			layout.Rigid(vspace(6)),
			layout.Rigid(func(gtx C) D {
				msg := "Call or email us from your registered email " + a.me.Student.Email + ". We unlock your course right after."
				if code := a.me.Student.Code(); code != "" {
					msg = "Call or email us with your Student ID " + code + ". We unlock your course right after."
				}
				return centered(gtx, msg,
					a.th, textOpt{size: 14, color: a.th.P.Muted})
			}),
			layout.Rigid(vspace(16)),
			layout.Rigid(func(gtx C) D { return a.callBox(gtx, c) }),
			layout.Rigid(vspace(16)),
			layout.Rigid(func(gtx C) D { return a.th.button(gtx, &s.home, "Go to home", btnOpt{full: true, kind: btnOutline}) }),
		)
	})
}

func (a *App) callBox(gtx C, c api.Course) D {
	s := a.checkout
	phones := supportPhones(a.me.Payment.ContactPhone)
	for i, p := range phones {
		if s.call[i].Clicked(gtx) {
			a.open(telURL(p))
		}
		if s.copyPhone[i].Clicked(gtx) {
			a.copyText(gtx, p, "Number copied")
		}
	}
	if s.copyMail.Clicked(gtx) {
		a.copyText(gtx, helpEmail, "Email copied")
	}
	if s.mail.Clicked(gtx) {
		st := a.me.Student
		a.open(mailURL(fmt.Sprintf("Payment for %s - %s", c.Title, studentRef(st)),
			fmt.Sprintf("Hi G-CATION,\n\nI have paid %s for \"%s\".\n%s\n\nPlease verify my payment.",
				rupees(c.Price), c.Title, studentLines(st))))
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return tinted(gtx, a.th.P.Lav, 16, 16, func(gtx C) D {
		kids := []layout.FlexChild{layout.Rigid(func(gtx C) D { return a.th.kicker(gtx, "G-CATION SUPPORT", a.th.P.Muted) })}
		for i, p := range phones {
			kids = append(kids, layout.Rigid(vspace(10)), layout.Rigid(func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx C) D { return a.th.label(gtx, p, textOpt{size: 18, weight: font.Black, maxLines: 1}) }),
					layout.Rigid(hspace(6)),
					layout.Rigid(func(gtx C) D { return a.th.iconButton(gtx, &s.copyPhone[i], icCopy, 0) }),
					layout.Rigid(hspace(6)),
					layout.Rigid(func(gtx C) D { return a.th.button(gtx, &s.call[i], "Call", btnOpt{small: true, icon: icCall}) }),
				)
			}))
		}
		kids = append(kids, layout.Rigid(vspace(14)),
			layout.Rigid(func(gtx C) D { return a.th.label(gtx, helpEmail, textOpt{size: 16, weight: font.Bold, maxLines: 1}) }),
			layout.Rigid(vspace(8)),
			layout.Rigid(func(gtx C) D {
				return layout.Flex{}.Layout(gtx,
					layout.Flexed(1, func(gtx C) D {
						return a.th.button(gtx, &s.copyMail, "Copy", btnOpt{full: true, small: true, kind: btnOutline, icon: icCopy})
					}),
					layout.Rigid(hspace(8)),
					layout.Flexed(1, func(gtx C) D {
						return a.th.button(gtx, &s.mail, "Email", btnOpt{full: true, small: true, kind: btnOutline, icon: icMail})
					}),
				)
			}))
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
	})
}

func (a *App) statusPanel(gtx C, c api.Course) []layout.Widget {
	s := a.checkout
	p := c.PendingClaim
	if s.again.Clicked(gtx) {
		s.resend, s.step = true, 1
	}
	timeline := func(gtx C) D {
		type row struct {
			state       int // 0 todo, 1 done, 2 now
			title, text string
			ic          any
		}
		rows := []row{
			{1, "Account created", studentRef(a.me.Student), nil},
			{1, "Payment details sent", fmtDateTime(p.CreatedAt), nil},
			{2, "Admin verification", "Usually within a few hours", nil},
			{0, "Course unlocks", "Class links appear in Updates", nil},
		}
		kids := []layout.FlexChild{
			layout.Rigid(func(gtx C) D { return a.th.title(gtx, "Verification in progress", 18) }),
			layout.Rigid(vspace(4)),
			layout.Rigid(func(gtx C) D {
				return a.th.muted(gtx, "The admin checks the bank account and unlocks your course.", 13.5)
			}),
			layout.Rigid(vspace(14)),
		}
		for _, r := range rows {
			kids = append(kids, layout.Rigid(func(gtx C) D {
				return layout.Inset{Bottom: 14}.Layout(gtx, func(gtx C) D {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							sz := gtx.Dp(28)
							gtx.Constraints = layout.Exact(image.Pt(sz, sz))
							ic := icTick
							fg := white
							switch r.state {
							case 1:
								fillRound(gtx, image.Pt(sz, sz), 14, a.th.P.Green)
							case 2:
								gradientRound(gtx, image.Pt(sz, sz), 14, gradA, gradB)
								ic = icHourglass
							default:
								fillRound(gtx, image.Pt(sz, sz), 14, a.th.P.Lav)
								ic, fg = icSchool, a.th.P.Accent
							}
							return layout.Center.Layout(gtx, func(gtx C) D { return iconSized(gtx, ic, 15, fg) })
						}),
						layout.Rigid(hspace(12)),
						layout.Flexed(1, func(gtx C) D {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx C) D { return a.th.label(gtx, r.title, textOpt{size: 14.5, weight: font.Bold}) }),
								layout.Rigid(func(gtx C) D { return a.th.muted(gtx, r.text, 12.5) }),
							)
						}),
					)
				})
			}))
		}
		return a.th.card(gtx, 18, func(gtx C) D { return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...) })
	}
	sent := func(gtx C) D {
		var lines [][2]string
		if m, ok := a.method(p.QRSlot); ok {
			lines = append(lines, [2]string{"Paid to", m.Label})
		}
		age := ""
		if p.Age != nil {
			age = strconv.Itoa(*p.Age)
		}
		for _, kv := range [][2]string{{"Payer", p.PayerName}, {"Using", p.Method}, {"Date", p.PaidOn}, {"UTR", p.Reference}, {"Age", age}, {"Referred", p.ReferredBy}, {"Referral ID", p.ReferralID}, {"Screenshot", map[bool]string{true: "Attached"}[p.HasScreenshot]}} {
			if kv[1] != "" {
				lines = append(lines, kv)
			}
		}

		return a.th.card(gtx, 18, func(gtx C) D {
			kids := []layout.FlexChild{layout.Rigid(func(gtx C) D { return a.th.title(gtx, "What you sent", 17) }), layout.Rigid(vspace(10))}
			for _, kv := range lines {
				kids = append(kids, layout.Rigid(func(gtx C) D {
					return layout.Inset{Bottom: 6}.Layout(gtx, func(gtx C) D {
						return layout.Flex{}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								gtx.Constraints.Min.X = gtx.Dp(90)
								return a.th.muted(gtx, kv[0], 13.5)
							}),
							layout.Flexed(1, func(gtx C) D { return a.th.label(gtx, kv[1], textOpt{size: 13.5, weight: font.SemiBold}) }),
						)
					})
				}))
			}
			if p.Amount != nil {
				kids = append(kids, layout.Rigid(func(gtx C) D {
					return layout.Flex{}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							gtx.Constraints.Min.X = gtx.Dp(90)
							return a.th.muted(gtx, "Amount", 13.5)
						}),
						layout.Rigid(func(gtx C) D { return a.th.money(gtx, *p.Amount, textOpt{size: 13.5, weight: font.SemiBold}) }),
					)
				}))
			}
			kids = append(kids, layout.Rigid(vspace(6)), layout.Rigid(func(gtx C) D {
				return a.th.button(gtx, &s.again, "Made a mistake? Pay or send again", btnOpt{kind: btnGhost, small: true})
			}))
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
		})
	}
	return []layout.Widget{vspace(16), timeline, vspace(12), sent, vspace(12), func(gtx C) D {
		return a.th.card(gtx, 18, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return a.th.title(gtx, "Speed it up: call or email us", 17) }),
				layout.Rigid(vspace(12)),
				layout.Rigid(func(gtx C) D { return a.callBox(gtx, c) }),
			)
		})
	}}
}
