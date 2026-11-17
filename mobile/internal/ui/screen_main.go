package ui

import (
	"fmt"
	"image"
	"image/color"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"gcation/mobile/internal/api"
)

type homeState struct{ copyID widget.Clickable }
type coursesState struct{}
type updatesState struct{ seenAt int64 }

// Per-course gradients (same idea as the website's colour per course).
var thumbGradients = [][2]color.NRGBA{
	{rgb(0x4f46e5), rgb(0xdb2777)}, {rgb(0x0ea5e9), rgb(0x8b5cf6)}, {rgb(0xf97316), rgb(0xa855f7)},
	{rgb(0x10b981), rgb(0x6366f1)}, {rgb(0xf43f5e), rgb(0xfacc15)}, {rgb(0x7c3aed), rgb(0xf472b6)},
}

func (a *App) latestReceipt(courseID int64) *api.Receipt {
	for i := range a.me.Receipts {
		r := &a.me.Receipts[i]
		if r.CourseID == courseID && r.RevokedAt == nil {
			return r
		}
	}
	return nil
}

func (a *App) openCheckout(c api.Course) {
	a.checkout = &checkoutState{courseID: c.ID}
	a.checkout.init(a, c)
}

// ---------- Home ----------

func (a *App) layoutHome(gtx C) D {
	me := a.me
	if a.home.copyID.Clicked(gtx) {
		a.copyText(gtx, me.Student.Code(), "Student ID copied")
	}
	var act []api.Course
	for _, c := range me.Courses {
		if c.Access.Status == "active" || c.Access.Status == "scheduled" {
			act = append(act, c)
		}
	}

	items := []layout.Widget{vspace(4), func(gtx C) D { return a.homeHero(gtx, len(act)) }}

	// Things that need the student's attention
	type att struct {
		tn      tone
		ic      *widget.Icon
		title   string
		text    string
		action  string
		onClick func()
	}
	var atts []att
	if me.Student.Mobile == nil {
		atts = append(atts, att{toneBlue, icCall, "Add your mobile number", "Then you can also log in with your mobile number and password.", "Add", func() { a.tab = tabProfile }})
	}
	if me.Unread > 0 {
		atts = append(atts, att{toneGreen, icMegaphone, plural(me.Unread, "new update") + " from G-CATION", "Class links, community and instructions.", "Open", func() { a.tab = tabUpdates }})
	}
	for _, c := range me.Courses {
		c := c
		switch {
		case c.PendingClaim != nil && c.Access.Status != "active":
			atts = append(atts, att{toneAmber, icHourglass, "Payment for " + c.Title + " is being verified", "Give us a quick call if you have not already.", "Status", func() { a.openCheckout(c) }})
		case c.Access.Status == "active" && c.Access.DaysLeft <= 14:
			atts = append(atts, att{toneViolet, icClock, fmt.Sprintf("%s ends in %s", c.Title, plural(c.Access.DaysLeft, "day")), "Renew to keep learning after " + fmtDate(c.Access.Until) + ".", "Renew", func() { a.openCheckout(c) }})
		case c.Access.Status == "expired" && c.PendingClaim == nil:
			atts = append(atts, att{toneViolet, icAlert, c.Title + " has ended", "Your access ended on " + fmtDate(c.Access.Until) + ".", "Renew", func() { a.openCheckout(c) }})
		}
	}
	for i, at := range atts {
		at := at
		clk := a.clk(fmt.Sprint("att", i))
		if clk.Clicked(gtx) {
			at.onClick()
		}
		items = append(items, vspace(12), func(gtx C) D {
			fg, bg := a.th.toneColors(at.tn)
			return clk.Layout(gtx, func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return tinted(gtx, bg, 18, 14, func(gtx C) D {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return tinted(gtx, withAlpha(fg, 40), 12, 9, func(gtx C) D { return iconSized(gtx, at.ic, 20, fg) })
						}),
						layout.Rigid(hspace(12)),
						layout.Flexed(1, func(gtx C) D {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx C) D { return a.th.label(gtx, at.title, textOpt{size: 14.5, weight: font.Bold}) }),
								layout.Rigid(func(gtx C) D { return a.th.muted(gtx, at.text, 13) }),
							)
						}),
						layout.Rigid(hspace(8)),
						layout.Rigid(func(gtx C) D {
							return a.th.label(gtx, at.action+" ›", textOpt{size: 13, weight: font.Bold, color: a.th.P.Accent})
						}),
					)
				})
			})
		})
	}

	allCourses := a.clk("home.allcourses")
	if allCourses.Clicked(gtx) {
		a.tab = tabCourses
	}
	items = append(items, func(gtx C) D {
		return a.th.sectionTitle(gtx, "My learning", func(gtx C) D { return a.th.linkText(gtx, allCourses, "All courses →") })
	})
	if len(act) == 0 {
		explore := a.clk("home.explore")
		if explore.Clicked(gtx) {
			a.tab = tabCourses
		}
		items = append(items, func(gtx C) D {
			return a.emptyState(gtx, icSchool, "No active course yet", "Choose a course, pay with UPI and we unlock it after a quick verification.", explore, "Explore courses")
		})
	}
	for _, c := range act {
		c := c
		items = append(items, func(gtx C) D { return a.learnCard(gtx, c) }, vspace(12))
	}

	if a.updates != nil && len(a.updates.Items) > 0 {
		seeAll := a.clk("home.seeall")
		if seeAll.Clicked(gtx) {
			a.tab = tabUpdates
		}
		items = append(items, func(gtx C) D {
			return a.th.sectionTitle(gtx, "Latest updates", func(gtx C) D { return a.th.linkText(gtx, seeAll, "See all →") })
		})
		for i, u := range a.updates.Items {
			if i == 2 {
				break
			}
			u := u
			items = append(items, func(gtx C) D { return a.updateCard(gtx, u, a.updates.SeenAt) }, vspace(12))
		}
	}
	return a.page(gtx, "home", items)
}

func (a *App) homeHero(gtx C, active int) D {
	me := a.me
	return a.th.heroCard(gtx, func(gtx C) D {
		sub := "Pick a course to start your learning journey."
		if active > 0 {
			sub = fmt.Sprintf("You have %s. Keep going!", plural(active, "active course"))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return a.th.kicker(gtx, greeting(), rgb(0xd7b9ff)) }),
			layout.Rigid(vspace(6)),
			layout.Rigid(func(gtx C) D {
				return a.th.label(gtx, "Hello, "+firstName(me.Student.Name), textOpt{size: 26, weight: font.Bold, color: white})
			}),
			layout.Rigid(vspace(4)),
			layout.Rigid(func(gtx C) D { return a.th.label(gtx, sub, textOpt{size: 14, color: rgb(0xdccdf0)}) }),
			layout.Rigid(func(gtx C) D {
				if me.Student.Code() == "" {
					return D{} // students get a Student ID when their first payment is approved
				}
				gtx.Constraints.Min.X = 0
				return layout.Inset{Top: 14}.Layout(gtx, func(gtx C) D {
					return a.home.copyID.Layout(gtx, func(gtx C) D {
						return withBackground(gtx, func(gtx C, size image.Point) {
							fillRound(gtx, size, 99, argb(0xffffff, 30))
							strokeRound(gtx, size, 99, 1, argb(0xffffff, 70))
						}, func(gtx C) D {
							return layout.Inset{Left: 14, Right: 12, Top: 7, Bottom: 7}.Layout(gtx, func(gtx C) D {
								return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
									layout.Rigid(func(gtx C) D {
										return a.th.label(gtx, "Student ID  "+me.Student.Code(), textOpt{size: 13, weight: font.Bold, color: white})
									}),
									layout.Rigid(hspace(8)),
									layout.Rigid(func(gtx C) D { return iconSized(gtx, icCopy, 15, white) }),
								)
							})
						})
					})
				})
			}),
		)
	})
}

func (a *App) learnCard(gtx C, c api.Course) D {
	open, rec, renew := a.clk(fmt.Sprint("learn.open", c.ID)), a.clk(fmt.Sprint("learn.rec", c.ID)), a.clk(fmt.Sprint("learn.renew", c.ID))
	r := a.latestReceipt(c.ID)
	if open.Clicked(gtx) {
		if c.ClassroomLink != "" {
			a.open(c.ClassroomLink)
		} else {
			a.tab = tabUpdates
		}
	}
	if rec.Clicked(gtx) && r != nil {
		a.saveReceipt(*r)
	}
	if renew.Clicked(gtx) {
		a.openCheckout(c)
	}
	acc := c.Access
	return a.th.card(gtx, 16, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return a.daysBadge(gtx, acc) }),
					layout.Rigid(hspace(14)),
					layout.Flexed(1, func(gtx C) D {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx C) D { return a.th.label(gtx, c.Title, textOpt{size: 16, weight: font.Bold, maxLines: 2}) }),
							layout.Rigid(vspace(3)),
							layout.Rigid(func(gtx C) D {
								if acc.Status == "scheduled" {
									return a.th.muted(gtx, "Starts later · until "+fmtDate(acc.Until), 13)
								}
								return a.th.muted(gtx, "Valid until "+fmtDate(acc.Until), 13)
							}),
						)
					}),
				)
			}),
			layout.Rigid(vspace(12)),
			layout.Rigid(func(gtx C) D {
				frac := float32(1)
				if acc.Until > acc.Since {
					frac = float32(acc.Until-time.Now().Unix()) / float32(acc.Until-acc.Since)
				}
				return a.th.progress(gtx, frac, a.th.P.Lav, gradA, gradB)
			}),
			layout.Rigid(vspace(12)),
			layout.Rigid(func(gtx C) D {
				label, ic := "Class links", icMegaphone
				if c.ClassroomLink != "" {
					label, ic = "Open classroom", icVideo
				}
				kids := []layout.FlexChild{layout.Rigid(func(gtx C) D { return a.th.button(gtx, open, label, btnOpt{kind: btnSoft, small: true, icon: ic}) })}
				if r != nil {
					kids = append(kids, layout.Rigid(hspace(8)), layout.Rigid(func(gtx C) D {
						return a.th.button(gtx, rec, "Receipt", btnOpt{kind: btnGhost, small: true, icon: icDownload})
					}))
				}
				if acc.Status == "active" && acc.DaysLeft <= 14 {
					kids = append(kids, layout.Rigid(hspace(8)), layout.Rigid(func(gtx C) D {
						return a.th.button(gtx, renew, "Renew", btnOpt{kind: btnOutline, small: true})
					}))
				}
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx, kids...)
			}),
		)
	})
}

// daysBadge is the round "N DAYS" badge on active courses.
func (a *App) daysBadge(gtx C, acc api.Access) D {
	s := gtx.Dp(64)
	gtx.Constraints = layout.Exact(image.Pt(s, s))
	gradientRound(gtx, image.Pt(s, s), 32, gradA, gradB)
	inner := gtx.Dp(54)
	off := (s - inner) / 2
	st := pushOffset(gtx, image.Pt(off, off))
	fillRound(gtx, image.Pt(inner, inner), 27, a.th.P.Surface)
	st.Pop()
	layout.Center.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return a.th.label(gtx, fmt.Sprint(acc.DaysLeft), textOpt{size: 17, weight: font.Black}) }),
			layout.Rigid(func(gtx C) D {
				return a.th.label(gtx, "DAYS", textOpt{size: 8.5, weight: font.Bold, color: a.th.P.Muted})
			}),
		)
	})
	return D{Size: image.Pt(s, s)}
}

// ---------- Courses ----------

func (a *App) layoutCourses(gtx C) D {
	var mine, open []api.Course
	for _, c := range a.me.Courses {
		if c.Access.Status != "none" {
			mine = append(mine, c)
		} else if c.IsOpen {
			open = append(open, c)
		}
	}
	items := []layout.Widget{vspace(6), func(gtx C) D {
		return a.pageHead(gtx, "COURSES", "Your learning", "Enrolled courses and courses you can join.")
	}}
	if len(mine) > 0 {
		items = append(items, func(gtx C) D { return a.th.sectionTitle(gtx, "Enrolled", nil) })
		for _, c := range mine {
			c := c
			items = append(items, func(gtx C) D { return a.courseCard(gtx, c) }, vspace(14))
		}
	}
	title := "Available courses"
	if len(mine) > 0 {
		title = "More courses"
	}
	items = append(items, func(gtx C) D { return a.th.sectionTitle(gtx, title, nil) })
	if len(open) == 0 {
		items = append(items, func(gtx C) D {
			if len(mine) > 0 {
				return a.emptyState(gtx, icSchool, "You are enrolled in everything", "New courses will show up here.", nil, "")
			}
			return a.emptyState(gtx, icSchool, "New batches coming soon", "Check back shortly for upcoming courses.", nil, "")
		})
	}
	for _, c := range open {
		c := c
		items = append(items, func(gtx C) D { return a.courseCard(gtx, c) }, vspace(14))
	}
	return a.page(gtx, "courses", items)
}

// pageHead is the line under the top bar's title (the title itself is in the top bar).
func (a *App) pageHead(gtx C, _, _, sub string) D {
	return a.th.muted(gtx, sub, 14)
}

func (a *App) statusBadge(gtx C, c api.Course) D {
	acc := c.Access
	switch {
	case acc.Status == "active" && acc.DaysLeft <= 7:
		return a.th.badge(gtx, plural(acc.DaysLeft, "day")+" left", toneAmber)
	case acc.Status == "active":
		return a.th.badge(gtx, "Active", toneGreen)
	case acc.Status == "scheduled":
		return a.th.badge(gtx, "Starts later", toneViolet)
	case c.PendingClaim != nil:
		return a.th.badge(gtx, "Verifying payment", toneAmber)
	case acc.Status == "expired":
		return a.th.badge(gtx, "Expired", toneRed)
	}
	return D{}
}

// thumb draws the course picture: the uploaded thumbnail or the course's gradient.
func (a *App) thumb(gtx C, c api.Course, size image.Point, radius unit.Dp) D {
	if op, ok := a.image(c.ThumbnailPath()); ok {
		return imageBox(gtx, op, size, radius)
	}
	g := thumbGradients[int(c.ID)%len(thumbGradients)]
	gtx.Constraints = layout.Exact(size)
	gradientRound(gtx, size, radius, g[0], g[1])
	return layout.Center.Layout(gtx, func(gtx C) D {
		return iconSized(gtx, icSchool, unit.Dp(float32(min(size.X, size.Y))/gtx.Metric.PxPerDp*0.32), white)
	})
}

func (a *App) courseCard(gtx C, c api.Course) D {
	acc := c.Access
	main, second := a.clk(fmt.Sprint("course.main", c.ID)), a.clk(fmt.Sprint("course.second", c.ID))
	r := a.latestReceipt(c.ID)
	var mainLabel, secondLabel string
	var mainKind = btnPrimary
	var mainIcon *widget.Icon
	switch {
	case acc.Status == "active":
		mainLabel, mainKind, mainIcon = "Class links", btnSoft, icMegaphone
		if c.ClassroomLink != "" {
			mainLabel, mainIcon = "Open classroom", icVideo
		}
		if r != nil {
			secondLabel = "Receipt"
		}
	case acc.Status == "scheduled":
	case c.PendingClaim != nil:
		mainLabel, mainKind, mainIcon = "View status", btnOutline, icHourglass
	case acc.Status == "expired":
		mainLabel = "Renew"
	default:
		mainLabel = "Enroll"
	}
	if main.Clicked(gtx) {
		switch {
		case acc.Status == "active" && c.ClassroomLink != "":
			a.open(c.ClassroomLink)
		case acc.Status == "active":
			a.tab = tabUpdates
		default:
			a.openCheckout(c)
		}
	}
	if second.Clicked(gtx) && r != nil {
		a.saveReceipt(*r)
	}

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
						layout.Rigid(func(gtx C) D {
							return layout.Flex{}.Layout(gtx,
								layout.Flexed(1, func(gtx C) D { return a.th.label(gtx, c.Title, textOpt{size: 17, weight: font.Bold}) }),
								layout.Rigid(hspace(8)),
								layout.Rigid(func(gtx C) D { return a.statusBadge(gtx, c) }),
							)
						}),
						layout.Rigid(func(gtx C) D {
							if c.Description == "" {
								return D{}
							}
							return layout.Inset{Top: 6}.Layout(gtx, func(gtx C) D {
								return a.th.label(gtx, c.Description, textOpt{size: 13.5, color: a.th.P.Muted, maxLines: 3})
							})
						}),
						layout.Rigid(func(gtx C) D {
							if acc.Status != "none" || len(c.Features) == 0 {
								return D{}
							}
							return layout.Inset{Top: 10}.Layout(gtx, func(gtx C) D { return a.featureList(gtx, c.Features, 3) })
						}),
						layout.Rigid(func(gtx C) D {
							if acc.Status != "active" && acc.Status != "scheduled" {
								return D{}
							}
							return layout.Inset{Top: 10}.Layout(gtx, func(gtx C) D {
								return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
									layout.Rigid(func(gtx C) D { return iconSized(gtx, icCalendar, 16, a.th.P.Muted) }),
									layout.Rigid(hspace(6)),
									layout.Rigid(func(gtx C) D {
										return a.th.label(gtx, "Valid until "+fmtDate(acc.Until), textOpt{size: 13.5, color: a.th.P.Ink2})
									}),
								)
							})
						}),
						layout.Rigid(vspace(14)),
						layout.Rigid(func(gtx C) D {
							return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
								layout.Flexed(1, func(gtx C) D {
									if acc.Status == "active" || acc.Status == "scheduled" {
										return D{}
									}
									return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
										layout.Rigid(func(gtx C) D { return a.th.money(gtx, c.Price, textOpt{size: 20, weight: font.Black}) }),
										layout.Rigid(func(gtx C) D { return a.th.muted(gtx, " / "+durationLabel(c.DefaultDays), 12.5) }),
									)
								}),
								layout.Rigid(func(gtx C) D {
									if secondLabel == "" {
										return D{}
									}
									return layout.Inset{Right: 8}.Layout(gtx, func(gtx C) D {
										return a.th.button(gtx, second, secondLabel, btnOpt{kind: btnGhost, small: true, icon: icDownload})
									})
								}),
								layout.Rigid(func(gtx C) D {
									if mainLabel == "" {
										return D{}
									}
									return a.th.button(gtx, main, mainLabel, btnOpt{kind: mainKind, small: true, icon: mainIcon})
								}),
							)
						}),
					)
				})
			}),
		)
	})
}

func (a *App) featureList(gtx C, feats []string, limit int) D {
	if limit > 0 && len(feats) > limit {
		feats = feats[:limit]
	}
	kids := make([]layout.FlexChild, 0, len(feats))
	for _, f := range feats {
		kids = append(kids, layout.Rigid(func(gtx C) D {
			return layout.Inset{Bottom: 6}.Layout(gtx, func(gtx C) D {
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Inset{Top: 1, Right: 8}.Layout(gtx, func(gtx C) D { return iconSized(gtx, icTick, 16, a.th.P.Green) })
					}),
					layout.Flexed(1, func(gtx C) D { return a.th.label(gtx, f, textOpt{size: 13.5, color: a.th.P.Ink2}) }),
				)
			})
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
}

func (a *App) emptyState(gtx C, ic *widget.Icon, title, text string, clk *widget.Clickable, action string) D {
	return a.th.card(gtx, 26, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return tinted(gtx, a.th.P.Lav, 20, 16, func(gtx C) D { return iconSized(gtx, ic, 30, a.th.P.Accent) })
			}),
			layout.Rigid(vspace(12)),
			layout.Rigid(func(gtx C) D { return centered(gtx, title, a.th, textOpt{size: 17, weight: font.Bold}) }),
			layout.Rigid(vspace(4)),
			layout.Rigid(func(gtx C) D { return centered(gtx, text, a.th, textOpt{size: 13.5, color: a.th.P.Muted}) }),
			layout.Rigid(func(gtx C) D {
				if clk == nil {
					return D{}
				}
				return layout.Inset{Top: 16}.Layout(gtx, func(gtx C) D { return a.th.button(gtx, clk, action, btnOpt{icon: icChevron}) })
			}),
		)
	})
}

// ---------- Updates ----------

func (a *App) layoutUpdates(gtx C) D {
	items := []layout.Widget{vspace(6), func(gtx C) D {
		return a.pageHead(gtx, "UPDATES", "From G-CATION", "Class links, community and instructions.")
	}, vspace(16)}
	switch {
	case !hasActive(a.me):
		viewCourses := a.clk("upd.courses")
		if viewCourses.Clicked(gtx) {
			a.tab = tabCourses
		}
		items = append(items, func(gtx C) D {
			return a.emptyState(gtx, icLock, "Class links unlock after payment", "Google Meet links, the community group and instructions appear here once your course is active.", viewCourses, "View courses")
		})
	case a.updates == nil || len(a.updates.Items) == 0:
		items = append(items, func(gtx C) D {
			return a.emptyState(gtx, icMegaphone, "No updates yet", "Class links and announcements from the team will appear here.", nil, "")
		})
	default:
		// Mark everything read once, but keep the NEW badges visible while on this page.
		if !a.seenMarked {
			a.seenMarked = true
			a.upd.seenAt = a.updates.SeenAt
			if a.me.Unread > 0 {
				a.async(func() func() {
					err := a.api.MarkUpdatesSeen()
					return func() {
						if err == nil {
							a.me.Unread = 0
						}
					}
				})
			}
		}
		for _, u := range a.updates.Items {
			u := u
			items = append(items, func(gtx C) D { return a.updateCard(gtx, u, a.upd.seenAt) }, vspace(12))
		}
	}
	return a.page(gtx, "updates", items)
}

func updateIcon(u api.Update) *widget.Icon {
	l := strings.ToLower(u.LinkURL)
	switch {
	case strings.Contains(l, "meet.google") || strings.Contains(l, "zoom.us"):
		return icVideo
	case strings.Contains(l, "whatsapp") || strings.Contains(l, "t.me") || strings.Contains(l, "telegram"):
		return icChat
	case l != "":
		return icLink
	}
	return icMegaphone
}

func (a *App) updateCard(gtx C, u api.Update, seenAt int64) D {
	link := a.clk(fmt.Sprint("upd.link", u.ID))
	if link.Clicked(gtx) {
		a.open(u.LinkURL)
	}
	unread := u.CreatedAt > seenAt
	return a.th.card(gtx, 16, func(gtx C) D {
		return layout.Flex{}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				s := gtx.Dp(44)
				gtx.Constraints = layout.Exact(image.Pt(s, s))
				gradientRound(gtx, image.Pt(s, s), 14, gradA, gradB)
				return layout.Center.Layout(gtx, func(gtx C) D { return iconSized(gtx, updateIcon(u), 22, white) })
			}),
			layout.Rigid(hspace(12)),
			layout.Flexed(1, func(gtx C) D {
				meta := timeAgo(u.CreatedAt) + " · All students"
				if u.CourseTitle != nil {
					meta = timeAgo(u.CreatedAt) + " · " + *u.CourseTitle
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
							layout.Flexed(1, func(gtx C) D { return a.th.label(gtx, u.Title, textOpt{size: 15.5, weight: font.Bold}) }),
							layout.Rigid(func(gtx C) D {
								if !unread {
									return D{}
								}
								return layout.Inset{Left: 6}.Layout(gtx, func(gtx C) D {
									return tinted(gtx, rgb(0xe5484d), 99, 0, func(gtx C) D {
										return layout.Inset{Left: 7, Right: 7, Top: 2, Bottom: 2}.Layout(gtx, func(gtx C) D {
											return a.th.label(gtx, "NEW", textOpt{size: 10, weight: font.Bold, color: white})
										})
									})
								})
							}),
						)
					}),
					layout.Rigid(func(gtx C) D { return a.th.muted(gtx, meta, 12) }),
					layout.Rigid(func(gtx C) D {
						if u.Message == "" {
							return D{}
						}
						return layout.Inset{Top: 8}.Layout(gtx, func(gtx C) D { return a.th.body(gtx, u.Message) })
					}),
					layout.Rigid(func(gtx C) D {
						if u.LinkURL == "" {
							return D{}
						}
						label := u.LinkLabel
						if label == "" {
							label = "Open link"
						}
						return layout.Inset{Top: 12}.Layout(gtx, func(gtx C) D {
							return a.th.button(gtx, link, label, btnOpt{small: true, icon: icOpen})
						})
					}),
				)
			}),
		)
	})
}
