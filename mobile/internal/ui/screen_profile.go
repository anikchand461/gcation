package ui

import (
	"fmt"
	"image"

	"gcation/mobile/internal/api"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/widget"
)

type profileState struct {
	name, phone, cur, next                        *field
	mob                                           mobileForm
	filled                                        bool
	save, changePw, mail, copyMail, theme, logout widget.Clickable
	call, copyPhone                               [2]widget.Clickable
	copyID                                        widget.Clickable
	savingProfile, savingPw                       bool
	profileErr, pwErr                             string
}

func (s *profileState) init() {
	s.name = newField("Full name", "", false, key.HintAny)
	s.phone = newField("Contact phone", "", false, key.HintTelephone)
	s.mob.init()
	s.cur = newField("Current password", "", true, key.HintAny)
	s.next = newField("New password", "At least 8 characters", true, key.HintAny)
}

func (a *App) layoutProfile(gtx C) D {
	s := &a.prof
	me := a.me
	if !s.filled {
		s.name.Ed.SetText(me.Student.Name)
		s.phone.Ed.SetText(me.Student.Phone)
		s.filled = true
	}
	if s.copyID.Clicked(gtx) {
		a.copyText(gtx, me.Student.Code(), "Student ID copied")
	}
	if s.save.Clicked(gtx) && !s.savingProfile {
		a.saveProfile()
	}
	if s.changePw.Clicked(gtx) && !s.savingPw {
		a.changePassword()
	}
	if s.theme.Clicked(gtx) {
		a.toggleTheme()
	}
	if s.logout.Clicked(gtx) {
		go a.api.Logout()
		s.filled = false
		a.signedOut("You are logged out.")
	}
	phones := supportPhones(me.Payment.ContactPhone)
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
		a.open(mailURL("Help needed - "+studentRef(me.Student),
			"Hi G-CATION,\n\nI need help with: \n\n"+studentLines(me.Student)))
	}

	items := []layout.Widget{
		vspace(6),
		func(gtx C) D {
			return a.pageHead(gtx, "PROFILE", "Your account", "Manage your details, receipts and password.")
		},
		vspace(16),
		// Identity
		func(gtx C) D {
			return a.th.card(gtx, 18, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx C) D { return a.th.avatar(gtx, me.Student.Name, 60) }),
							layout.Rigid(hspace(14)),
							layout.Flexed(1, func(gtx C) D {
								return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
									layout.Rigid(func(gtx C) D { return a.th.label(gtx, me.Student.Name, textOpt{size: 18, weight: font.Bold}) }),
									layout.Rigid(func(gtx C) D {
										return a.th.muted(gtx, contactText(me.Student.SignIn, me.Student.Email, me.Student.Phone), 13.5)
									}),
									layout.Rigid(func(gtx C) D {
										return a.th.label(gtx, "Member since "+fmtDate(me.Student.CreatedAt), textOpt{size: 12, color: a.th.P.Faint})
									}),
								)
							}),
						)
					}),
					layout.Rigid(vspace(14)),
					layout.Rigid(func(gtx C) D {
						if me.Student.Code() == "" {
							return a.th.muted(gtx, "Your Student ID is given when your first payment is verified.", 12.5)
						}
						gtx.Constraints.Min.X = 0
						return s.copyID.Layout(gtx, func(gtx C) D {
							return tinted(gtx, a.th.P.Lav, 99, 0, func(gtx C) D {
								return layout.Inset{Left: 14, Right: 12, Top: 8, Bottom: 8}.Layout(gtx, func(gtx C) D {
									return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
										layout.Rigid(func(gtx C) D {
											return a.th.label(gtx, "Student ID  "+me.Student.Code(), textOpt{size: 13.5, weight: font.Bold, color: a.th.P.Accent})
										}),
										layout.Rigid(hspace(8)),
										layout.Rigid(func(gtx C) D { return iconSized(gtx, icCopy, 15, a.th.P.Accent) }),
									)
								})
							})
						})
					}),
				)
			})
		},
		vspace(14),
		func(gtx C) D { return a.receiptsCard(gtx) },
		vspace(14),
		// Personal details
		func(gtx C) D {
			return a.th.card(gtx, 18, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return a.cardTitle(gtx, icPerson, "Personal details") }),
					layout.Rigid(vspace(12)),
					layout.Rigid(func(gtx C) D { return a.th.field(gtx, s.name) }),
					layout.Rigid(vspace(12)),
					layout.Rigid(func(gtx C) D { return a.th.field(gtx, s.phone) }),
					layout.Rigid(func(gtx C) D {
						if me.Student.Email == "" {
							return D{}
						}
						return layout.Inset{Top: 6}.Layout(gtx, func(gtx C) D {
							return a.th.muted(gtx, "Email: "+me.Student.Email+". To change it, contact the G-CATION team.", 12)
						})
					}),
					layout.Rigid(func(gtx C) D { return a.errLine(gtx, s.profileErr) }),
					layout.Rigid(vspace(14)),
					layout.Rigid(func(gtx C) D {
						return a.th.button(gtx, &s.save, "Save changes", btnOpt{full: true, icon: icTick, busy: s.savingProfile})
					}),
				)
			})
		},
		vspace(14),
		func(gtx C) D {
			return a.mobileCard(gtx, &s.mob, api.AsStudent, me.Student.MobileText(), a.reload)
		},
		vspace(14),
		// Password
		func(gtx C) D {
			return a.passwordCard(gtx, me.Student.HasPassword, s.cur, s.next, &s.changePw, s.savingPw, s.pwErr)
		},
		vspace(14),
		// Help + settings
		func(gtx C) D {
			return a.th.card(gtx, 18, func(gtx C) D {
				kids := []layout.FlexChild{
					layout.Rigid(func(gtx C) D { return a.cardTitle(gtx, icCall, "Need help?") }),
					layout.Rigid(vspace(4)),
					layout.Rigid(func(gtx C) D { return a.th.muted(gtx, "Questions about payment, classes or your account.", 12.5) }),
				}
				for i, p := range phones {
					kids = append(kids, layout.Rigid(vspace(12)), layout.Rigid(func(gtx C) D {
						return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
							layout.Flexed(1, func(gtx C) D {
								return a.th.button(gtx, &s.call[i], "Call "+p, btnOpt{full: true, kind: btnOutline, icon: icCall})
							}),
							layout.Rigid(hspace(8)),
							layout.Rigid(func(gtx C) D { return a.th.iconButton(gtx, &s.copyPhone[i], icCopy, 0) }),
						)
					}))
				}
				kids = append(kids, layout.Rigid(vspace(16)),
					layout.Rigid(func(gtx C) D {
						return a.th.label(gtx, helpEmail, textOpt{size: 15, weight: font.SemiBold, maxLines: 1})
					}),
					layout.Rigid(vspace(8)),
					layout.Rigid(func(gtx C) D {
						return layout.Flex{}.Layout(gtx,
							layout.Flexed(1, func(gtx C) D {
								return a.th.button(gtx, &s.copyMail, "Copy", btnOpt{full: true, kind: btnOutline, icon: icCopy})
							}),
							layout.Rigid(hspace(8)),
							layout.Flexed(1, func(gtx C) D {
								return a.th.button(gtx, &s.mail, "Email", btnOpt{full: true, kind: btnOutline, icon: icMail})
							}),
						)
					}))
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
			})
		},
		vspace(14),
		func(gtx C) D {
			return a.settingRow(gtx, &s.theme, themeIcon(a.th.Dark), themeLabel(a.th.Dark), "Switch the app's colours")
		},
		vspace(16),
		func(gtx C) D {
			return a.th.button(gtx, &s.logout, "Log out", btnOpt{full: true, kind: btnDanger, icon: icLogout})
		},
		vspace(10),
		func(gtx C) D {
			return centered(gtx, "G-CATION · Learn. Grow. Succeed.", a.th, textOpt{size: 12, color: a.th.P.Faint})
		},
	}
	return a.page(gtx, "profile", items)
}

// passwordCard changes the password; Google accounts have none yet, so they set one without the current one.
func (a *App) passwordCard(gtx C, has bool, cur, next *field, save *widget.Clickable, busy bool, errMsg string) D {
	title, intro, button := "Change password", "You stay logged in here. Other devices are logged out.", "Update password"
	if !has {
		title, intro, button = "Set a password", "You sign in with Google. Set a password to also log in with your mobile number.", "Set password"
	}
	return a.th.card(gtx, 18, func(gtx C) D {
		kids := []layout.FlexChild{
			layout.Rigid(func(gtx C) D { return a.cardTitle(gtx, icKey, title) }),
			layout.Rigid(vspace(4)),
			layout.Rigid(func(gtx C) D { return a.th.muted(gtx, intro, 12.5) }),
		}
		if has {
			kids = append(kids, layout.Rigid(vspace(12)), layout.Rigid(func(gtx C) D { return a.th.field(gtx, cur) }))
		}
		kids = append(kids,
			layout.Rigid(vspace(12)),
			layout.Rigid(func(gtx C) D { return a.th.field(gtx, next) }),
			layout.Rigid(func(gtx C) D { return a.errLine(gtx, errMsg) }),
			layout.Rigid(vspace(14)),
			layout.Rigid(func(gtx C) D { return a.th.button(gtx, save, button, btnOpt{full: true, icon: icKey, busy: busy}) }),
		)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
	})
}

func themeIcon(dark bool) *widget.Icon {
	if dark {
		return icSun
	}
	return icMoon
}

func themeLabel(dark bool) string {
	if dark {
		return "Light mode"
	}
	return "Dark mode"
}

func (a *App) cardTitle(gtx C, ic *widget.Icon, s string) D {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return iconSized(gtx, ic, 20, a.th.P.Violet) }),
		layout.Rigid(hspace(8)),
		layout.Rigid(func(gtx C) D { return a.th.title(gtx, s, 17) }),
	)
}

func (a *App) errLine(gtx C, msg string) D {
	if msg == "" {
		return D{}
	}
	return layout.Inset{Top: 12}.Layout(gtx, func(gtx C) D {
		return a.th.notice(gtx, icAlert, toneRed, func(gtx C) D { return a.th.label(gtx, msg, textOpt{size: 13.5, color: a.th.P.Red}) })
	})
}

func (a *App) settingRow(gtx C, clk *widget.Clickable, ic *widget.Icon, title, sub string) D {
	return clk.Layout(gtx, func(gtx C) D {
		return a.th.card(gtx, 14, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return tinted(gtx, a.th.P.Lav, 12, 9, func(gtx C) D { return iconSized(gtx, ic, 20, a.th.P.Accent) })
				}),
				layout.Rigid(hspace(12)),
				layout.Flexed(1, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx C) D { return a.th.label(gtx, title, textOpt{size: 15, weight: font.Bold}) }),
						layout.Rigid(func(gtx C) D { return a.th.muted(gtx, sub, 12.5) }),
					)
				}),
				layout.Rigid(func(gtx C) D { return iconSized(gtx, icChevron, 22, a.th.P.Faint) }),
			)
		})
	})
}

func (a *App) receiptsCard(gtx C) D {
	rs := a.me.Receipts
	return a.th.card(gtx, 18, func(gtx C) D {
		kids := []layout.FlexChild{
			layout.Rigid(func(gtx C) D { return a.cardTitle(gtx, icReceipt, "Payment receipts") }),
			layout.Rigid(vspace(4)),
			layout.Rigid(func(gtx C) D {
				return a.th.muted(gtx, "A receipt is created automatically when your payment is approved.", 12.5)
			}),
			layout.Rigid(vspace(8)),
		}
		if len(rs) == 0 {
			kids = append(kids, layout.Rigid(func(gtx C) D {
				return layout.Inset{Top: 6}.Layout(gtx, func(gtx C) D {
					return a.th.muted(gtx, "No receipts yet. Your first one appears here after your payment is verified.", 13.5)
				})
			}))
		}
		for i, r := range rs {
			r := r
			clk := a.clk(fmt.Sprint("receipt", r.ID))
			if clk.Clicked(gtx) {
				a.saveReceipt(r)
			}
			if i > 0 {
				kids = append(kids, layout.Rigid(func(gtx C) D { return a.th.divider(gtx) }))
			}
			kids = append(kids, layout.Rigid(func(gtx C) D {
				return layout.Inset{Top: 10, Bottom: 10}.Layout(gtx, func(gtx C) D {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							s := gtx.Dp(40)
							gtx.Constraints = layout.Exact(image.Pt(s, s))
							fillRound(gtx, image.Pt(s, s), 12, a.th.P.Lav)
							return layout.Center.Layout(gtx, func(gtx C) D { return iconSized(gtx, icReceipt, 20, a.th.P.Accent) })
						}),
						layout.Rigid(hspace(12)),
						layout.Flexed(1, func(gtx C) D {
							meta := fmt.Sprintf("%s · %s · %s", r.ReceiptNo, fmtDate(r.GrantedAt), rupees(r.AmountPaid))
							if r.RevokedAt != nil {
								meta += " · revoked"
							}
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx C) D {
									return a.th.label(gtx, r.CourseTitle, textOpt{size: 14.5, weight: font.Bold, maxLines: 1})
								}),
								layout.Rigid(func(gtx C) D { return a.th.muted(gtx, meta, 12) }),
							)
						}),
						layout.Rigid(hspace(8)),
						layout.Rigid(func(gtx C) D {
							return a.th.button(gtx, clk, "PDF", btnOpt{kind: btnOutline, small: true, icon: icDownload})
						}),
					)
				})
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
	})
}

func (a *App) saveProfile() {
	s := &a.prof
	name, phone := s.name.Text(), s.phone.Text()
	if name == "" {
		s.profileErr = "Enter your name."
		return
	}
	s.savingProfile, s.profileErr = true, ""
	a.async(func() func() {
		err := a.api.UpdateProfile(name, phone)
		return func() {
			s.savingProfile = false
			if err != nil {
				s.profileErr = err.Error()
				return
			}
			a.me.Student.Name, a.me.Student.Phone = name, phone
			a.toast("Profile saved.", false)
		}
	})
}

func (a *App) changePassword() {
	s := &a.prof
	cur, next := s.cur.Ed.Text(), s.next.Ed.Text()
	if len(next) < 8 {
		s.pwErr = "New password must be at least 8 characters."
		return
	}
	s.savingPw, s.pwErr = true, ""
	a.async(func() func() {
		err := a.api.ChangePassword(cur, next)
		return func() {
			s.savingPw = false
			if err != nil {
				s.pwErr = err.Error()
				return
			}
			s.cur.Ed.SetText("")
			s.next.Ed.SetText("")
			a.me.Student.HasPassword = true
			a.toast("Password saved.", false)
		}
	})
}
