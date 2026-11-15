package ui

import (
	"errors"
	"fmt"
	"image"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/widget"

	"gcation/mobile/internal/api"
)

// The management team side: Home, Courses, Updates and Profile. Members see the courses and
// prices and the updates the admin sends the team, and share their Management ID with students;
// they cannot enroll.

type mgmtTab int

const (
	mgmtHome mgmtTab = iota
	mgmtCourses
	mgmtUpdates
	mgmtProfile
)

type mgmtState struct {
	tab     mgmtTab
	nav     [4]widget.Clickable
	me      *api.ManagerMe
	updates *api.Updates
	loading bool
	err     string

	seenMarked  bool
	shownSeenAt int64 // keeps NEW tags while Updates stays open after it marked them read (0 = none)
	bell        widget.Clickable
	allUpdates  widget.Clickable

	retry, theme, copyID, copyID2, viewCourses, allCourses, changePw, logout widget.Clickable
	cur, next                                                                *field
	savingPw                                                                 bool
	pwErr                                                                    string
}

func (a *App) startManagement() {
	a.startManagementState()
	a.mgmtReload()
}

func (a *App) startManagementState() {
	a.mgmt = &mgmtState{
		cur:  newField("Current password", "", true, key.HintAny),
		next: newField("New password", "At least 8 characters", true, key.HintAny),
	}
	a.screen = scrMain
}

func (a *App) mgmtReload() {
	m := a.mgmt
	if m == nil || m.loading {
		return
	}
	m.loading = true
	a.async(func() func() {
		me, err := a.api.ManagerMe()
		var upd *api.Updates
		if err == nil {
			upd, err = a.api.ManagerUpdates()
		}
		return func() {
			m.loading = false
			if errors.Is(err, api.ErrUnauthorized) {
				a.signedOut("Please log in again.")
				a.auth.mgmt = true
				return
			}
			if err != nil {
				m.err = err.Error()
				return
			}
			m.err, m.me, m.updates = "", me, upd
			team := "Management team"
			for i := range upd.Items {
				upd.Items[i].CourseTitle = &team // shown on each card in place of a course
			}
		}
	})
}

func (a *App) mgmtLogout() {
	go a.api.ManagerLogout()
	a.signedOut("You are logged out.")
	a.auth.mgmt = true
}

func (a *App) layoutManagement(gtx C) D {
	m := a.mgmt
	for i := range m.nav {
		if m.nav[i].Clicked(gtx) {
			if mgmtTab(i) == m.tab {
				l := a.list(fmt.Sprint("mgmt", i))
				l.Position.First, l.Position.Offset = 0, 0
			}
			if mgmtTab(i) != mgmtUpdates {
				m.shownSeenAt = 0
			}
			m.tab = mgmtTab(i)
			a.mgmtReload()
		}
	}
	if m.bell.Clicked(gtx) || m.allUpdates.Clicked(gtx) {
		m.tab = mgmtUpdates
	}
	if m.theme.Clicked(gtx) {
		a.toggleTheme()
	}
	if m.retry.Clicked(gtx) {
		a.mgmtReload()
	}
	return a.safeArea(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				title := [...]string{"", "Courses", "Updates", "Profile"}[m.tab]
				unread := 0
				if m.me != nil {
					unread = m.me.Unread
				}
				return a.appBar(gtx, title, m.tab == mgmtHome, a.barIcon(&m.theme, themeIcon(a.th.Dark), 0), a.barIcon(&m.bell, icBell, unread))
			}),
			layout.Flexed(1, func(gtx C) D {
				if m.me == nil {
					return a.mgmtLoading(gtx)
				}
				switch m.tab {
				case mgmtCourses:
					return a.mgmtCoursesPage(gtx)
				case mgmtUpdates:
					return a.mgmtUpdatesPage(gtx)
				case mgmtProfile:
					return a.mgmtProfilePage(gtx)
				}
				return a.mgmtHomePage(gtx)
			}),
			layout.Rigid(func(gtx C) D {
				unread := m.me != nil && m.me.Unread > 0
				items := []navItem{{icHome, "Home", false}, {icSchool, "Courses", false}, {icMegaphone, "Updates", unread}, {icPerson, "Profile", false}}
				return a.navBar(gtx, items, int(m.tab), m.nav[:])
			}),
		)
	})
}

func (a *App) mgmtLoading(gtx C) D {
	m := a.mgmt
	return layout.Center.Layout(gtx, func(gtx C) D {
		return pad(28).Layout(gtx, func(gtx C) D {
			if m.err == "" {
				return a.th.muted(gtx, "Loading…", 15)
			}
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return iconSized(gtx, icAlert, 40, a.th.P.Violet) }),
				layout.Rigid(vspace(12)),
				layout.Rigid(func(gtx C) D { return centered(gtx, m.err, a.th, textOpt{size: 15, color: a.th.P.Ink2}) }),
				layout.Rigid(vspace(16)),
				layout.Rigid(func(gtx C) D { return a.th.button(gtx, &m.retry, "Try again", btnOpt{icon: icRefresh}) }),
			)
		})
	})
}

// idChip is the tappable "Management ID GCM001" pill that copies the ID.
func (a *App) idChip(gtx C, clk *widget.Clickable, onHero bool) D {
	code := a.mgmt.me.Manager.Code
	if clk.Clicked(gtx) {
		a.copyText(gtx, code, "Management ID copied")
	}
	gtx.Constraints.Min.X = 0
	fg := a.th.P.Accent
	if onHero {
		fg = white
	}
	return clk.Layout(gtx, func(gtx C) D {
		return withBackground(gtx, func(gtx C, size image.Point) {
			if onHero {
				fillRound(gtx, size, 99, argb(0xffffff, 30))
				strokeRound(gtx, size, 99, 1, argb(0xffffff, 70))
			} else {
				fillRound(gtx, size, 99, a.th.P.Lav)
			}
		}, func(gtx C) D {
			return layout.Inset{Left: 14, Right: 12, Top: 8, Bottom: 8}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return a.th.label(gtx, "Management ID  "+code, textOpt{size: 13.5, weight: font.Bold, color: fg})
					}),
					layout.Rigid(hspace(8)),
					layout.Rigid(func(gtx C) D { return iconSized(gtx, icCopy, 15, fg) }),
				)
			})
		})
	})
}

func (a *App) mgmtHomePage(gtx C) D {
	m := a.mgmt
	me := m.me
	if m.viewCourses.Clicked(gtx) || m.allCourses.Clicked(gtx) {
		m.tab = mgmtCourses
	}
	steps := [][3]any{
		{icPerson, "Tell students about G-CATION", "Share the courses and prices you see here."},
		{icSend, "Give them your Management ID", fmt.Sprintf("They type %s as the Referral ID when they send their payment details.", me.Manager.Code)},
		{icCheck, "Payment is verified", "The admin sees your ID on their payment and their receipt."},
	}
	items := []layout.Widget{
		vspace(4),
		func(gtx C) D {
			return a.th.heroCard(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return a.th.kicker(gtx, greeting(), rgb(0xd7b9ff)) }),
					layout.Rigid(vspace(6)),
					layout.Rigid(func(gtx C) D {
						return a.th.label(gtx, "Hello, "+firstName(me.Manager.Name), textOpt{size: 26, weight: font.Bold, color: white})
					}),
					layout.Rigid(vspace(4)),
					layout.Rigid(func(gtx C) D {
						return a.th.label(gtx, "Share your Management ID with students. They enter it as the Referral ID when they pay.", textOpt{size: 14, color: rgb(0xdccdf0)})
					}),
					layout.Rigid(vspace(14)),
					layout.Rigid(func(gtx C) D { return a.idChip(gtx, &m.copyID, true) }),
					layout.Rigid(vspace(10)),
					layout.Rigid(func(gtx C) D {
						gtx.Constraints.Min.X = 0
						return a.th.button(gtx, &m.viewCourses, "View courses", btnOpt{kind: btnWhite, small: true, icon: icSchool})
					}),
				)
			})
		},
		func(gtx C) D { return a.th.sectionTitle(gtx, "How referrals work", nil) },
	}
	for i, st := range steps {
		ic, title, text := st[0].(*widget.Icon), st[1].(string), st[2].(string)
		items = append(items, func(gtx C) D {
			return a.th.card(gtx, 16, func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						sz := gtx.Dp(40)
						gtx.Constraints = layout.Exact(pt(sz, sz))
						gradientRound(gtx, pt(sz, sz), 12, gradA, gradB)
						return layout.Center.Layout(gtx, func(gtx C) D { return iconSized(gtx, ic, 20, white) })
					}),
					layout.Rigid(hspace(14)),
					layout.Flexed(1, func(gtx C) D {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								return a.th.label(gtx, fmt.Sprintf("%d. %s", i+1, title), textOpt{size: 15, weight: font.Bold})
							}),
							layout.Rigid(vspace(2)),
							layout.Rigid(func(gtx C) D { return a.th.muted(gtx, text, 13) }),
						)
					}),
				)
			})
		}, vspace(10))
	}
	if upd := m.updates; upd != nil && len(upd.Items) > 0 {
		items = append(items, func(gtx C) D {
			return a.th.sectionTitle(gtx, "Latest updates", func(gtx C) D { return a.th.linkText(gtx, &m.allUpdates, "See all →") })
		})
		for i, u := range upd.Items {
			if i == 2 {
				break
			}
			u := u
			items = append(items, func(gtx C) D { return a.updateCard(gtx, u, upd.SeenAt) }, vspace(10))
		}
	}
	items = append(items, func(gtx C) D {
		return a.th.sectionTitle(gtx, "Courses", func(gtx C) D { return a.th.linkText(gtx, &m.allCourses, "All courses →") })
	})
	items = append(items, a.mgmtCourseItems(2)...)
	return a.page(gtx, "mgmt0", items)
}

func (a *App) mgmtCourseItems(limit int) []layout.Widget {
	courses := a.mgmt.me.Courses
	if len(courses) == 0 {
		return []layout.Widget{func(gtx C) D {
			return a.emptyState(gtx, icSchool, "No courses open right now", "New courses will show up here.", nil, "")
		}}
	}
	if limit > 0 && len(courses) > limit {
		courses = courses[:limit]
	}
	var items []layout.Widget
	for _, c := range courses {
		c := c
		items = append(items, func(gtx C) D { return a.mgmtCourseCard(gtx, c) }, vspace(14))
	}
	return items
}

// mgmtCourseCard shows a course with its price, without any enroll button.
func (a *App) mgmtCourseCard(gtx C, c api.Course) D {
	return a.th.card(gtx, 0, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Top: 12, Left: 12, Right: 12}.Layout(gtx, func(gtx C) D {
					w := gtx.Constraints.Max.X
					return a.thumb(gtx, c, image.Pt(w, w*9/20), 14)
				})
			}),
			layout.Rigid(func(gtx C) D {
				return pad(16).Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx C) D { return a.th.label(gtx, c.Title, textOpt{size: 17, weight: font.Bold}) }),
						layout.Rigid(func(gtx C) D {
							if c.Description == "" {
								return D{}
							}
							return layout.Inset{Top: 6}.Layout(gtx, func(gtx C) D {
								return a.th.label(gtx, c.Description, textOpt{size: 13.5, color: a.th.P.Muted, maxLines: 3})
							})
						}),
						layout.Rigid(func(gtx C) D {
							if len(c.Features) == 0 {
								return D{}
							}
							return layout.Inset{Top: 10}.Layout(gtx, func(gtx C) D { return a.featureList(gtx, c.Features, 4) })
						}),
						layout.Rigid(vspace(10)),
						layout.Rigid(func(gtx C) D {
							return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
								layout.Rigid(func(gtx C) D { return a.th.money(gtx, c.Price, textOpt{size: 20, weight: font.Black}) }),
								layout.Rigid(func(gtx C) D { return a.th.muted(gtx, " / "+durationLabel(c.DefaultDays), 12.5) }),
							)
						}),
					)
				})
			}),
		)
	})
}

func (a *App) mgmtCoursesPage(gtx C) D {
	items := []layout.Widget{
		vspace(6),
		func(gtx C) D { return a.th.muted(gtx, "Everything students can join right now.", 14) },
		vspace(12),
		func(gtx C) D {
			return a.th.notice(gtx, icInfo, toneBlue, func(gtx C) D {
				return a.th.label(gtx, "Management accounts cannot enroll. Students join from their own student account.", textOpt{size: 13, color: a.th.P.Blue})
			})
		},
		vspace(14),
	}
	return a.page(gtx, "mgmt1", append(items, a.mgmtCourseItems(0)...))
}

func (a *App) mgmtUpdatesPage(gtx C) D {
	m := a.mgmt
	upd := m.updates
	// Opening Updates marks everything as read.
	if !m.seenMarked && m.me.Unread > 0 {
		m.seenMarked, m.shownSeenAt = true, upd.SeenAt
		a.async(func() func() {
			err := a.api.ManagerMarkUpdatesSeen()
			return func() {
				m.seenMarked = false
				if err == nil && m.me != nil {
					m.me.Unread = 0
				}
			}
		})
	}
	seenAt := upd.SeenAt
	if m.shownSeenAt != 0 {
		seenAt = m.shownSeenAt
	}
	items := []layout.Widget{
		vspace(6),
		func(gtx C) D {
			return a.th.muted(gtx, "Messages, meeting links and instructions for the management team.", 14)
		},
		vspace(14),
	}
	if len(upd.Items) == 0 {
		items = append(items, func(gtx C) D {
			return a.emptyState(gtx, icMegaphone, "No updates yet", "Messages from the G-CATION admin for the team will appear here.", nil, "")
		})
	}
	for _, u := range upd.Items {
		u := u
		items = append(items, func(gtx C) D { return a.updateCard(gtx, u, seenAt) }, vspace(10))
	}
	return a.page(gtx, "mgmt2", items)
}

func (a *App) mgmtProfilePage(gtx C) D {
	m := a.mgmt
	mg := m.me.Manager
	if m.changePw.Clicked(gtx) && !m.savingPw {
		a.mgmtChangePassword()
	}
	if m.logout.Clicked(gtx) {
		a.confirm("Log out?", "You will need your email and password to log in again.", "Log out", true, a.mgmtLogout)
	}
	items := []layout.Widget{
		vspace(6),
		func(gtx C) D { return a.th.muted(gtx, "Your details and password.", 14) },
		vspace(16),
		func(gtx C) D {
			return a.th.card(gtx, 18, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx C) D { return a.th.avatar(gtx, mg.Name, 60) }),
							layout.Rigid(hspace(14)),
							layout.Flexed(1, func(gtx C) D {
								return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
									layout.Rigid(func(gtx C) D { return a.th.label(gtx, mg.Name, textOpt{size: 18, weight: font.Bold}) }),
									layout.Rigid(func(gtx C) D { return a.th.muted(gtx, mg.Email, 13.5) }),
									layout.Rigid(func(gtx C) D {
										return a.th.label(gtx, "Team member since "+fmtDate(mg.CreatedAt), textOpt{size: 12, color: a.th.P.Faint})
									}),
								)
							}),
						)
					}),
					layout.Rigid(vspace(14)),
					layout.Rigid(func(gtx C) D { return a.idChip(gtx, &m.copyID2, false) }),
				)
			})
		},
		vspace(14),
		func(gtx C) D {
			return a.th.card(gtx, 18, func(gtx C) D {
				kids := []layout.FlexChild{layout.Rigid(func(gtx C) D { return a.cardTitle(gtx, icPerson, "Personal details") }), layout.Rigid(vspace(10))}
				for _, kv := range [][2]string{{"Name", mg.Name}, {"Email", mg.Email}, {"Phone", mg.Phone}} {
					if kv[1] == "" {
						continue
					}
					kids = append(kids, layout.Rigid(func(gtx C) D {
						return layout.Inset{Bottom: 6}.Layout(gtx, func(gtx C) D {
							return layout.Flex{}.Layout(gtx,
								layout.Rigid(func(gtx C) D {
									gtx.Constraints.Min.X = gtx.Dp(70)
									return a.th.muted(gtx, kv[0], 13.5)
								}),
								layout.Flexed(1, func(gtx C) D { return a.th.label(gtx, kv[1], textOpt{size: 13.5, weight: font.SemiBold}) }),
							)
						})
					}))
				}
				kids = append(kids, layout.Rigid(vspace(4)), layout.Rigid(func(gtx C) D {
					return a.th.muted(gtx, "To change these details, contact the G-CATION admin.", 12)
				}))
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
			})
		},
		vspace(14),
		func(gtx C) D {
			return a.th.card(gtx, 18, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return a.cardTitle(gtx, icKey, "Change password") }),
					layout.Rigid(vspace(4)),
					layout.Rigid(func(gtx C) D { return a.th.muted(gtx, "You stay logged in here. Other devices are logged out.", 12.5) }),
					layout.Rigid(vspace(12)),
					layout.Rigid(func(gtx C) D { return a.th.field(gtx, m.cur) }),
					layout.Rigid(vspace(12)),
					layout.Rigid(func(gtx C) D { return a.th.field(gtx, m.next) }),
					layout.Rigid(func(gtx C) D { return a.errLine(gtx, m.pwErr) }),
					layout.Rigid(vspace(14)),
					layout.Rigid(func(gtx C) D {
						return a.th.button(gtx, &m.changePw, "Update password", btnOpt{full: true, icon: icKey, busy: m.savingPw})
					}),
				)
			})
		},
		vspace(16),
		func(gtx C) D {
			return a.th.button(gtx, &m.logout, "Log out", btnOpt{full: true, kind: btnDanger, icon: icLogout})
		},
	}
	return a.page(gtx, "mgmt3", items)
}

func (a *App) mgmtChangePassword() {
	m := a.mgmt
	cur, next := m.cur.Ed.Text(), m.next.Ed.Text()
	if len(next) < 8 {
		m.pwErr = "New password must be at least 8 characters."
		return
	}
	m.savingPw, m.pwErr = true, ""
	a.async(func() func() {
		err := a.api.ManagerChangePassword(cur, next)
		return func() {
			m.savingPw = false
			if errors.Is(err, api.ErrUnauthorized) {
				a.signedOut("Please log in again.")
				a.auth.mgmt = true
				return
			}
			if err != nil {
				m.pwErr = err.Error()
				return
			}
			m.cur.Ed.SetText("")
			m.next.Ed.SetText("")
			a.toast("Password updated.", false)
		}
	})
}
