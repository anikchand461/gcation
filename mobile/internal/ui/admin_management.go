package ui

import (
	"fmt"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"

	"gcation/mobile/internal/api"
)

// joinDot joins the non-empty parts with " · ".
func joinDot(parts ...string) string {
	kept := parts[:0:0]
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, " · ")
}

// studentIDText is "Student ID GCS001", or a note that the student has not paid yet.
func studentIDText(st api.Student) string {
	if st.Code() == "" {
		return "No Student ID yet"
	}
	return "Student ID " + st.Code()
}

// referralText is who referred a payment: the name the student typed and the Management ID.
func referralText(c api.ClaimRow) string {
	id := c.ReferralID
	if id != "" && c.ReferralName != nil {
		id += " (" + *c.ReferralName + ")"
	}
	return joinDot(c.ReferredBy, id)
}

// registrationURL is the website page where new management members register.
func (a *App) registrationURL() string {
	return strings.TrimRight(a.webURL, "/") + "/management-login.html#register"
}

// ---------- Payment note ----------

// layoutScreenshot shows one payment note in full: every detail the student sent, Accept /
// Reject, and the payment screenshot (fetched in the background, see prefetchScreenshots).
func (a *App) layoutScreenshot(gtx C) D {
	s := a.admin
	c := s.shot
	back, accept, reject := a.clk("shot.back"), a.clk("shot.accept"), a.clk("shot.reject")
	if back.Clicked(gtx) {
		s.shot = nil
		return D{}
	}
	if accept.Clicked(gtx) {
		s.shot = nil
		a.openStudent(c.StudentID, c.ID)
		return D{}
	}
	if reject.Clicked(gtx) {
		a.confirm("Reject this payment note?", "Do this only if no matching payment arrived in your account. The student can send their details again.", "Reject", true, func() {
			a.adminDo(func() error { return a.api.DismissClaim(c.ID) }, "Payment note rejected.", func() { s.shot = nil })
		})
	}
	amount := "Amount not given"
	if c.Amount != nil {
		amount = rupees(*c.Amount)
	}
	age := ""
	if c.Age != nil {
		age = fmt.Sprint(*c.Age)
	}
	student := c.StudentName
	if c.StudentCode != nil {
		student += " · " + *c.StudentCode
	}
	rows := [][2]string{
		{"Student", student}, {"Phone", c.StudentPhone}, {"Course", c.CourseTitle},
		{"Paid to", a.qrLabel(c.QRSlot)}, {"Paid using", c.Method}, {"Account name", c.PayerName},
		{"Payment date", c.PaidOn}, {"UTR", c.Reference}, {"Age", age}, {"Referred by", referralText(*c)},
		{"Sent", fmtDateTime(c.CreatedAt)},
	}
	items := []layout.Widget{
		func(gtx C) D {
			return a.th.card(gtx, 16, func(gtx C) D {
				kids := []layout.FlexChild{}
				kids = append(kids,
					layout.Rigid(func(gtx C) D { return a.th.label(gtx, amount, textOpt{size: 24, weight: font.Black}) }),
					layout.Rigid(vspace(10)))
				for _, kv := range rows {
					if kv[1] == "" {
						continue
					}
					kids = append(kids, layout.Rigid(func(gtx C) D {
						return layout.Inset{Bottom: 7}.Layout(gtx, func(gtx C) D {
							return layout.Flex{}.Layout(gtx,
								layout.Rigid(func(gtx C) D {
									gtx.Constraints.Min.X = gtx.Dp(110)
									return a.th.muted(gtx, kv[0], 13.5)
								}),
								layout.Flexed(1, func(gtx C) D { return a.th.label(gtx, kv[1], textOpt{size: 13.5, weight: font.SemiBold}) }),
							)
						})
					}))
				}
				kids = append(kids, layout.Rigid(vspace(8)), layout.Rigid(func(gtx C) D {
					return layout.Flex{}.Layout(gtx,
						layout.Flexed(1, func(gtx C) D {
							return a.th.button(gtx, reject, "Reject", btnOpt{full: true, kind: btnDanger, icon: icClose})
						}),
						layout.Rigid(hspace(10)),
						layout.Flexed(1, func(gtx C) D {
							return a.th.button(gtx, accept, "Accept", btnOpt{full: true, kind: btnSuccess, icon: icTick})
						}),
					)
				}))
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
			})
		},
		a.groupLabel("PAYMENT SCREENSHOT"),
		func(gtx C) D {
			if !c.HasScreenshot {
				return a.th.muted(gtx, "The student did not attach a screenshot.", 13.5)
			}
			op, ok := a.image(c.ScreenshotPath())
			if !ok {
				msg := "Loading screenshot…"
				if e := a.imgs[c.ScreenshotPath()]; e != nil && !e.loading {
					msg = "The screenshot is no longer available. It is deleted 5 minutes after the payment is approved."
				}
				return a.th.card(gtx, 24, func(gtx C) D {
					return layout.Center.Layout(gtx, func(gtx C) D { return centered(gtx, msg, a.th, textOpt{size: 14, color: a.th.P.Muted}) })
				})
			}
			// Full width, at the picture's own shape.
			w := gtx.Constraints.Max.X
			sz := op.Size()
			h := w * sz.Y / max(1, sz.X)
			gtx.Constraints = layout.Exact(pt(w, h))
			return imageBox(gtx, op, pt(w, h), 14)
		},
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return a.subBar(gtx, back, "Payment note") }),
		layout.Flexed(1, func(gtx C) D { return a.page(gtx, fmt.Sprint("note", c.ID), items) }),
	)
}

// prefetchScreenshots starts downloading the screenshots of waiting payment notes, so they are
// ready by the time the admin opens one.
func (a *App) prefetchScreenshots() {
	for _, c := range a.admin.claims {
		if c.HasScreenshot {
			a.image(c.ScreenshotPath())
		}
	}
}

// ---------- Management team ----------

func (a *App) managementItems(gtx C) []layout.Widget {
	s := a.admin
	if a.clk("mgmt.link").Clicked(gtx) {
		a.copyText(gtx, a.registrationURL(), "Registration link copied")
	}
	code := "not set, so registration is closed"
	if s.invite != "" {
		code = "now " + s.invite
	}
	items := []layout.Widget{
		func(gtx C) D {
			return a.th.muted(gtx, "People who refer students. They see courses and prices and share their Management ID (GCM…), which students enter as the Referral ID.", 13.5)
		},
		vspace(12),
		func(gtx C) D {
			return a.th.notice(gtx, icInfo, toneBlue, func(gtx C) D {
				return a.th.label(gtx, fmt.Sprintf("New members register once with the management referral code (%s). After that they log in with just their email and password. Change the code in Admin account.", code),
					textOpt{size: 13, color: a.th.P.Blue})
			})
		},
		vspace(12),
		func(gtx C) D {
			return a.th.button(gtx, a.clk("mgmt.link"), "Copy registration link", btnOpt{full: true, kind: btnOutline, icon: icCopy})
		},
		vspace(14),
	}
	if len(s.managers) == 0 {
		return append(items, func(gtx C) D {
			return a.emptyState(gtx, icShield, "No management members yet", "Set the referral code in Admin account and share the registration link with your team.", nil, "")
		})
	}
	for _, m := range s.managers {
		m := m
		toggle := a.clk(fmt.Sprint("mgmt.t", m.ID))
		call := a.clk(fmt.Sprint("mgmt.c", m.ID))
		if call.Clicked(gtx) && m.Phone != "" {
			a.open(telURL(m.Phone))
		}
		if toggle.Clicked(gtx) {
			if m.RevokedAt == nil {
				a.confirm(fmt.Sprintf("Revoke %s's access?", m.Name),
					fmt.Sprintf("%s is logged out and cannot log in, and students can no longer use %s as a Referral ID. Past referrals are kept. You can restore access any time.", m.Name, m.Code),
					"Revoke access", true, func() {
						a.adminDo(func() error { return a.api.RevokeManager(m.ID) }, m.Name+"'s access revoked.", nil)
					})
			} else {
				a.confirm(fmt.Sprintf("Restore %s's access?", m.Name),
					fmt.Sprintf("%s can log in again with the same email and password, and %s works as a Referral ID again.", m.Name, m.Code),
					"Restore access", false, func() {
						a.adminDo(func() error { return a.api.RestoreManager(m.ID) }, m.Name+" can log in again.", nil)
					})
			}
		}
		items = append(items, func(gtx C) D {
			return a.th.card(gtx, 16, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx C) D { return a.th.avatar(gtx, m.Name, 44) }),
							layout.Rigid(hspace(12)),
							layout.Flexed(1, func(gtx C) D {
								return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
									layout.Rigid(func(gtx C) D { return a.th.label(gtx, m.Name, textOpt{size: 16, weight: font.Bold, maxLines: 1}) }),
									layout.Rigid(func(gtx C) D { return a.th.muted(gtx, m.Email, 12.5) }),
								)
							}),
							layout.Rigid(func(gtx C) D {
								if m.RevokedAt != nil {
									return a.th.badge(gtx, "Revoked", toneRed)
								}
								return a.th.badge(gtx, "Active", toneGreen)
							}),
						)
					}),
					layout.Rigid(vspace(12)),
					layout.Rigid(func(gtx C) D {
						lines := [][2]string{
							{"Management ID", m.Code},
							{"Phone", m.Phone},
							{"Referrals", fmt.Sprintf("%d approved · %d waiting", m.Approved, m.Pending)},
							{"Joined", fmtDate(m.CreatedAt)},
						}
						if m.RevokedAt != nil {
							lines = append(lines, [2]string{"Revoked", fmtDate(*m.RevokedAt)})
						}
						kids := []layout.FlexChild{}
						for _, kv := range lines {
							if kv[1] == "" {
								continue
							}
							kids = append(kids, layout.Rigid(func(gtx C) D {
								return layout.Inset{Bottom: 5}.Layout(gtx, func(gtx C) D {
									return layout.Flex{}.Layout(gtx,
										layout.Rigid(func(gtx C) D {
											gtx.Constraints.Min.X = gtx.Dp(116)
											return a.th.muted(gtx, kv[0], 13)
										}),
										layout.Flexed(1, func(gtx C) D { return a.th.label(gtx, kv[1], textOpt{size: 13, weight: font.SemiBold}) }),
									)
								})
							}))
						}
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
					}),
					layout.Rigid(vspace(8)),
					layout.Rigid(func(gtx C) D {
						kids := []layout.FlexChild{}
						if m.Phone != "" {
							kids = append(kids, layout.Flexed(1, func(gtx C) D {
								return a.th.button(gtx, call, "Call", btnOpt{full: true, small: true, kind: btnSoft, icon: icCall})
							}), layout.Rigid(hspace(8)))
						}
						kids = append(kids, layout.Flexed(1, func(gtx C) D {
							if m.RevokedAt != nil {
								return a.th.button(gtx, toggle, "Restore", btnOpt{full: true, small: true, kind: btnSuccess, icon: icTick})
							}
							return a.th.button(gtx, toggle, "Revoke", btnOpt{full: true, small: true, kind: btnDanger, icon: icClose})
						}))
						return layout.Flex{}.Layout(gtx, kids...)
					}),
				)
			})
		}, vspace(12))
	}
	return items
}

// inviteCodeItems is the "Management referral code" card on the Admin account screen.
func (a *App) inviteCodeItems(gtx C) []layout.Widget {
	m := &a.admin.more
	if m.saveInvite.Clicked(gtx) {
		code := m.invite.Text()
		switch {
		case code != "" && (len(code) < 4 || strings.ContainsAny(code, " \t")):
			a.toast("Use at least 4 characters and no spaces.", true)
		default:
			msg := "Referral code saved."
			if code == "" {
				msg = "Management registration closed."
			}
			a.adminDo(func() error { return a.api.SetInviteCode(code) }, msg, nil)
		}
	}
	if m.copyLink.Clicked(gtx) {
		a.copyText(gtx, a.registrationURL(), "Registration link copied")
	}
	return []layout.Widget{
		a.groupLabel("MANAGEMENT REFERRAL CODE"),
		func(gtx C) D {
			return a.th.card(gtx, 16, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return a.th.muted(gtx, "New management members enter this once, when they register. People who already registered just log in with their email and password, even after you change it. Leave it empty to close registration.", 13)
					}),
					layout.Rigid(vspace(12)),
					layout.Rigid(func(gtx C) D { return a.th.field(gtx, m.invite) }),
					layout.Rigid(vspace(12)),
					layout.Rigid(func(gtx C) D { return a.th.button(gtx, &m.saveInvite, "Save code", btnOpt{full: true, icon: icTick}) }),
					layout.Rigid(vspace(8)),
					layout.Rigid(func(gtx C) D {
						return a.th.button(gtx, &m.copyLink, "Copy registration link", btnOpt{full: true, kind: btnGhost, icon: icCopy})
					}),
				)
			})
		},
	}
}
