package ui

import (
	"errors"
	"fmt"
	"image"
	"sort"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/widget"

	"gcation/mobile/internal/api"
)

type adminTab int

const (
	aOverview adminTab = iota
	aStudents
	aPayments
	aUpdates
	aMore
)

type moreScreen int

const (
	moreNone moreScreen = iota
	morePhone
	moreQR
	moreCourses
	moreCourseEdit
	moreAccount
)

// adminState is everything the admin side shows and edits.
type adminState struct {
	tab adminTab
	nav [5]widget.Clickable

	loading bool
	loaded  bool
	err     string

	ov       *api.Overview
	claims   []api.ClaimRow
	students []api.StudentRow
	payments []api.Enrollment
	anns     []api.Announcement
	audience map[string]int
	courses  []api.AdminCourse
	methods  []api.AdminMethod
	phone    string
	username string

	search  *field
	filter  int
	paySeg  int
	detail  *studentDetail
	compose composer
	sub     moreScreen
	more    moreState
	retry   widget.Clickable
	refresh widget.Clickable
	theme   widget.Clickable
}

func (s *adminState) canGoBack() bool {
	return s.detail != nil || s.sub != moreNone || s.tab != aOverview
}

func (s *adminState) back() {
	switch {
	case s.detail != nil:
		s.detail = nil
	case s.sub == moreCourseEdit:
		s.sub = moreCourses
	case s.sub != moreNone:
		s.sub = moreNone
	default:
		s.tab = aOverview
	}
}

func (a *App) startAdmin() {
	s := &adminState{search: newField("", "Search name, email, phone or ID", false, key.HintAny)}
	s.compose.init()
	s.more.init()
	a.admin = s
	a.screen = scrMain
	a.adminReload()
}

// adminReload fetches everything the admin screens show.
func (a *App) adminReload() {
	s := a.admin
	if s == nil || s.loading {
		return
	}
	s.loading = true
	a.async(func() func() {
		var (
			ov       *api.Overview
			claims   []api.ClaimRow
			students []api.StudentRow
			payments []api.Enrollment
			anns     []api.Announcement
			aud      map[string]int
			courses  []api.AdminCourse
			methods  []api.AdminMethod
			phone    string
			user     string
		)
		var err error
		steps := []func() error{
			func() (e error) { ov, e = a.api.Overview(); return },
			func() (e error) { claims, e = a.api.Claims(); return },
			func() (e error) { students, e = a.api.Students(""); return },
			func() (e error) { payments, e = a.api.Payments(); return },
			func() (e error) { anns, aud, e = a.api.Announcements(); return },
			func() (e error) { courses, e = a.api.AdminCourses(); return },
			func() (e error) { methods, e = a.api.PaymentMethods(); return },
			func() (e error) { phone, e = a.api.SupportPhone(); return },
			func() (e error) { user, e = a.api.AdminAccount(); return },
		}
		for _, step := range steps {
			if err = step(); err != nil {
				break
			}
		}
		return func() {
			s.loading = false
			if errors.Is(err, api.ErrUnauthorized) {
				a.signedOut("Admin session ended. Please log in again.")
				return
			}
			if err != nil {
				s.err = err.Error()
				return
			}
			s.err, s.loaded = "", true
			s.ov, s.claims, s.students, s.payments = ov, claims, students, payments
			s.anns, s.audience, s.courses, s.methods = anns, aud, courses, methods
			s.phone, s.username = phone, user
			a.admin.more.fill(s)
		}
	})
}

// adminDo runs an admin action in the background, then refreshes the data and shows a message.
func (a *App) adminDo(work func() error, okMsg string, after func()) {
	a.async(func() func() {
		err := work()
		return func() {
			if errors.Is(err, api.ErrUnauthorized) {
				a.signedOut("Admin session ended. Please log in again.")
				return
			}
			if err != nil {
				a.toast(err.Error(), true)
				return
			}
			if okMsg != "" {
				a.toast(okMsg, false)
			}
			if after != nil {
				after()
			}
			a.adminReload()
		}
	})
}

func (a *App) adminLogout() {
	go a.api.AdminLogout()
	a.signedOut("Logged out of admin.")
	a.auth.admin = true
}

// ---------- Shell ----------

func (a *App) layoutAdmin(gtx C) D {
	s := a.admin
	for i := range s.nav {
		if s.nav[i].Clicked(gtx) {
			s.tab, s.detail, s.sub = adminTab(i), nil, moreNone
			a.adminReload()
		}
	}
	if s.refresh.Clicked(gtx) {
		a.adminReload()
		a.toast("Refreshing…", false)
	}
	if s.theme.Clicked(gtx) {
		a.toggleTheme()
	}
	if s.detail != nil {
		return a.safeArea(gtx, a.layoutStudentDetail)
	}
	if s.tab == aMore && s.sub != moreNone {
		return a.safeArea(gtx, a.layoutMoreSub)
	}
	return a.safeArea(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				title := [...]string{"", "Students", "Payments", "Updates", "More"}[s.tab]
				ic := icMoon
				if a.th.Dark {
					ic = icSun
				}
				return a.appBar(gtx, title, s.tab == aOverview, a.barIcon(&s.theme, ic, 0), a.barIcon(&s.refresh, icRefresh, 0))
			}),
			layout.Flexed(1, func(gtx C) D {
				if !s.loaded {
					return a.adminLoading(gtx)
				}
				switch s.tab {
				case aStudents:
					return a.layoutAdminStudents(gtx)
				case aPayments:
					return a.layoutAdminPayments(gtx)
				case aUpdates:
					return a.layoutAdminUpdates(gtx)
				case aMore:
					return a.layoutAdminMore(gtx)
				}
				return a.layoutOverview(gtx)
			}),
			layout.Rigid(func(gtx C) D {
				items := []navItem{
					{icHome, "Overview", false}, {icPerson, "Students", false}, {icPay, "Payments", len(s.claims) > 0},
					{icMegaphone, "Updates", false}, {icSettingsMore, "More", false},
				}
				return a.navBar(gtx, items, int(s.tab), s.nav[:])
			}),
		)
	})
}

func (a *App) adminLoading(gtx C) D {
	s := a.admin
	return layout.Center.Layout(gtx, func(gtx C) D {
		return pad(28).Layout(gtx, func(gtx C) D {
			if s.err == "" {
				return a.th.muted(gtx, "Loading…", 15)
			}
			if s.retry.Clicked(gtx) {
				a.adminReload()
			}
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return iconSized(gtx, icAlert, 40, a.th.P.Violet) }),
				layout.Rigid(vspace(12)),
				layout.Rigid(func(gtx C) D { return centered(gtx, s.err, a.th, textOpt{size: 15, color: a.th.P.Ink2}) }),
				layout.Rigid(vspace(16)),
				layout.Rigid(func(gtx C) D { return a.th.button(gtx, &s.retry, "Try again", btnOpt{icon: icRefresh}) }),
			)
		})
	})
}

// ---------- Overview ----------

func (a *App) layoutOverview(gtx C) D {
	s := a.admin
	ov := s.ov
	items := []layout.Widget{
		func(gtx C) D {
			return a.th.label(gtx, greetingTitle()+" · "+time.Now().In(ist).Format("Monday, 2 Jan"), textOpt{size: 14, color: a.th.P.Muted})
		},
		vspace(14),
		func(gtx C) D {
			type tile struct {
				ic    *widget.Icon
				label string
				value func(gtx C) D
				a, b  uint32
				to    func()
			}
			num := func(n int) func(gtx C) D {
				return func(gtx C) D { return a.th.label(gtx, fmt.Sprint(n), textOpt{size: 22, weight: font.Black}) }
			}
			money := func(v float64) func(gtx C) D {
				return func(gtx C) D { return a.th.money(gtx, v, textOpt{size: 20, weight: font.Black}) }
			}
			tiles := []tile{
				{icPerson, "Students", num(ov.Students), 0x4f46e5, 0xdb2777, func() { s.tab, s.filter = aStudents, 0 }},
				{icCheck, "Active now", num(ov.ActiveStudents), 0x10b981, 0x14b8a6, func() { s.tab, s.filter = aStudents, 1 }},
				{icHourglass, "To verify", num(ov.PendingClaims), 0xf59e0b, 0xf97316, func() { s.tab, s.paySeg = aPayments, 0 }},
				{icClock, "Expiring in 7 days", num(ov.ExpiringSoon), 0xf43f5e, 0xec4899, func() { s.tab, s.filter = aStudents, 3 }},
				{icCalendar, "This month", money(ov.CollectedThisMonth), 0x0ea5e9, 0x6366f1, func() { s.tab, s.paySeg = aPayments, 1 }},
				{icPay, "Total collected", money(ov.TotalCollected), 0x7c3aed, 0xc026d3, func() { s.tab, s.paySeg = aPayments, 1 }},
			}
			rows := make([]layout.FlexChild, 0, 6)
			for r := 0; r < len(tiles); r += 2 {
				pair := tiles[r : r+2]
				if r > 0 {
					rows = append(rows, layout.Rigid(vspace(10)))
				}
				rows = append(rows, layout.Rigid(func(gtx C) D {
					cells := []layout.FlexChild{}
					for i, t := range pair {
						if i > 0 {
							cells = append(cells, layout.Rigid(hspace(10)))
						}
						clk := a.clk("ov.tile" + t.label)
						if clk.Clicked(gtx) {
							t.to()
						}
						cells = append(cells, layout.Flexed(1, func(gtx C) D {
							return clk.Layout(gtx, func(gtx C) D {
								return a.th.card(gtx, 14, func(gtx C) D {
									return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
										layout.Rigid(func(gtx C) D {
											sz := gtx.Dp(36)
											gtx.Constraints = layout.Exact(pt(sz, sz))
											gradientRound(gtx, pt(sz, sz), 11, rgb(t.a), rgb(t.b))
											return layout.Center.Layout(gtx, func(gtx C) D { return iconSized(gtx, t.ic, 19, white) })
										}),
										layout.Rigid(vspace(10)),
										layout.Rigid(t.value),
										layout.Rigid(func(gtx C) D { return a.th.label(gtx, t.label, textOpt{size: 12.5, color: a.th.P.Muted, maxLines: 1}) }),
									)
								})
							})
						}))
					}
					return layout.Flex{}.Layout(gtx, cells...)
				}))
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
		},
		a.groupLabel("PAYMENTS TO VERIFY"),
		func(gtx C) D { return a.claimRows(gtx, 5) },
		a.groupLabel("EXPIRING IN 7 DAYS"),
		func(gtx C) D {
			var rows []layout.Widget
			for _, st := range s.students {
				for _, ac := range st.Access {
					if ac.Status != "active" || ac.DaysLeft > 7 {
						continue
					}
					st, ac := st, ac
					wa := a.clk(fmt.Sprint("exp.wa", st.ID, ac.CourseID))
					open := a.clk(fmt.Sprint("exp.open", st.ID, ac.CourseID))
					if wa.Clicked(gtx) {
						a.open(waURL(st.Phone, fmt.Sprintf("Hi %s, your G-CATION course \"%s\" ends on %s. Renew to keep learning!", firstName(st.Name), ac.Title, fmtDate(ac.Until))))
					}
					if open.Clicked(gtx) {
						a.openStudent(st.ID, 0)
					}
					rows = append(rows, func(gtx C) D {
						return a.listRow(gtx, open, func(gtx C) D { return a.th.avatar(gtx, st.Name, 40) }, st.Name,
							fmt.Sprintf("%s · %s left", ac.Title, plural(ac.DaysLeft, "day")),
							func(gtx C) D {
								if st.Phone == "" {
									return D{}
								}
								return a.th.button(gtx, wa, "Remind", btnOpt{kind: btnSoft, small: true, icon: icChat})
							})
					})
				}
			}
			if len(rows) == 0 {
				return a.group(gtx, []layout.Widget{func(gtx C) D {
					return a.listRow(gtx, nil, func(gtx C) D { return a.iconTile(gtx, icCalendar, 40) }, "Nothing ending soon", "No access ends in the next 7 days.", nil)
				}})
			}
			return a.group(gtx, rows)
		},
		a.groupLabel("RECENT PAYMENTS"),
		func(gtx C) D { return a.paymentRows(gtx, 5) },
	}
	return a.page(gtx, "a.overview", items)
}

func greetingTitle() string {
	g := greeting()
	return strings.ToUpper(g[:1]) + strings.ToLower(g[1:])
}

func (a *App) qrLabel(slot string) string {
	for _, m := range a.admin.methods {
		if m.Slot == slot && m.Label != "" {
			return m.Label
		}
	}
	switch slot {
	case "primary":
		return "Primary QR"
	case "secondary":
		return "Secondary QR"
	}
	return "QR not given"
}

// claimRows lists pending payment notes with Verify / Dismiss.
func (a *App) claimRows(gtx C, limit int) D {
	s := a.admin
	if len(s.claims) == 0 {
		return a.group(gtx, []layout.Widget{func(gtx C) D {
			return a.listRow(gtx, nil, func(gtx C) D { return a.iconTile(gtx, icCheck, 40) }, "All caught up", "No payments waiting to be verified.", nil)
		}})
	}
	var rows []layout.Widget
	for i, c := range s.claims {
		if limit > 0 && i == limit {
			break
		}
		c := c
		verify, dismiss := a.clk(fmt.Sprint("claim.v", c.ID)), a.clk(fmt.Sprint("claim.d", c.ID))
		if verify.Clicked(gtx) {
			a.openStudent(c.StudentID, c.ID)
		}
		if dismiss.Clicked(gtx) {
			a.confirm("Dismiss this payment note?", "Do this only if no matching payment arrived in your account.", "Dismiss", true, func() {
				a.adminDo(func() error { return a.api.DismissClaim(c.ID) }, "Payment note dismissed.", nil)
			})
		}
		amount := "amount not given"
		if c.Amount != nil {
			amount = rupees(*c.Amount)
		}
		parts := []string{a.qrLabel(c.QRSlot)}
		for _, p := range []string{c.Method, c.Reference, prefix("ref. ", c.ReferredBy)} {
			if p != "" {
				parts = append(parts, p)
			}
		}
		parts = append(parts, timeAgo(c.CreatedAt))
		rows = append(rows, func(gtx C) D {
			return a.listRow(gtx, nil, func(gtx C) D { return a.th.avatar(gtx, c.StudentName, 40) },
				c.StudentName+" · "+amount, c.CourseTitle+"\n"+strings.Join(parts, " · "),
				func(gtx C) D {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx C) D { return a.th.button(gtx, verify, "Verify", btnOpt{kind: btnSuccess, small: true}) }),
						layout.Rigid(hspace(4)),
						a.barIcon(dismiss, icClose, 0),
					)
				})
		})
	}
	return a.group(gtx, rows)
}

// paymentRows lists recorded payments (access given) with a receipt button.
func (a *App) paymentRows(gtx C, limit int) D {
	s := a.admin
	if len(s.payments) == 0 {
		return a.group(gtx, []layout.Widget{func(gtx C) D {
			return a.listRow(gtx, nil, func(gtx C) D { return a.iconTile(gtx, icPay, 40) }, "No payments yet", "Payments appear here when you give access.", nil)
		}})
	}
	now := time.Now().Unix()
	var rows []layout.Widget
	for i, p := range s.payments {
		if limit > 0 && i == limit {
			break
		}
		p := p
		open, rec := a.clk(fmt.Sprint("pay.o", p.ID)), a.clk(fmt.Sprint("pay.r", p.ID))
		if open.Clicked(gtx) {
			a.openStudent(p.StudentID, 0)
		}
		if rec.Clicked(gtx) {
			a.saveAdminReceipt(p)
		}
		status := "active"
		switch {
		case p.RevokedAt != nil:
			status = "revoked"
		case p.EndAt <= now:
			status = "ended"
		case p.StartAt > now:
			status = "starts later"
		}
		rows = append(rows, func(gtx C) D {
			return a.listRow(gtx, open, func(gtx C) D { return a.th.avatar(gtx, p.StudentName, 40) },
				p.StudentName+" · "+rupees(p.AmountPaid),
				fmt.Sprintf("%s · %s → %s · %s", p.CourseTitle, fmtDate(p.StartAt), fmtDate(p.EndAt), status),
				func(gtx C) D { return layout.Flex{}.Layout(gtx, a.barIcon(rec, icDownload, 0)) })
		})
	}
	return a.group(gtx, rows)
}

func (a *App) saveAdminReceipt(p api.Enrollment) {
	a.saveFile("G-CATION-receipt-"+p.ReceiptNo+".pdf", func() ([]byte, error) { return a.api.AdminReceipt(p.ID) })
}

// ---------- Students ----------

var studentFilters = []string{"All", "Active", "Waiting", "Expiring soon", "Expired", "Not paid"}

func studentState(s api.StudentRow) int {
	minDays, active, scheduled, expired := 1<<30, false, false, false
	for _, ac := range s.Access {
		switch ac.Status {
		case "active":
			active = true
			minDays = min(minDays, ac.DaysLeft)
		case "scheduled":
			scheduled = true
		case "expired":
			expired = true
		}
	}
	switch {
	case active && minDays <= 7:
		return 3
	case active || scheduled:
		return 1
	case s.PendingClaims > 0:
		return 2
	case expired:
		return 4
	}
	return 5
}

func matchesFilter(s api.StudentRow, f int) bool {
	st := studentState(s)
	return f == 0 || st == f || (f == 1 && st == 3)
}

func (a *App) layoutAdminStudents(gtx C) D {
	s := a.admin
	q := strings.ToLower(s.search.Text())
	var shown []api.StudentRow
	counts := make([]int, len(studentFilters))
	for _, st := range s.students {
		if q != "" && !strings.Contains(strings.ToLower(st.Name+" "+st.Email+" "+st.Phone+" "+fmt.Sprint(st.ID)), q) {
			continue
		}
		for f := range studentFilters {
			if matchesFilter(st, f) {
				counts[f]++
			}
		}
		if matchesFilter(st, s.filter) {
			shown = append(shown, st)
		}
	}
	labels := make([]string, len(studentFilters))
	for i, l := range studentFilters {
		labels[i] = fmt.Sprintf("%s  %d", l, counts[i])
		if c := a.clk(fmt.Sprint("sf", i)); c.Clicked(gtx) {
			s.filter = i
		}
	}
	items := []layout.Widget{
		func(gtx C) D { return a.th.field(gtx, s.search) },
		vspace(12),
		func(gtx C) D {
			return a.chips(gtx, "students", labels, func(i int) bool { return i == s.filter }, func(i int) *widget.Clickable { return a.clk(fmt.Sprint("sf", i)) })
		},
		vspace(12),
	}
	if len(shown) == 0 {
		items = append(items, func(gtx C) D {
			msg := "No students in this group."
			if len(s.students) == 0 {
				msg = "No students have registered yet."
			} else if q != "" {
				msg = "No students match your search."
			}
			return a.emptyState(gtx, icPerson, "Nobody here", msg, nil, "")
		})
	} else {
		items = append(items, func(gtx C) D {
			rows := make([]layout.Widget, 0, len(shown))
			for _, st := range shown {
				st := st
				clk := a.clk(fmt.Sprint("stu", st.ID))
				if clk.Clicked(gtx) {
					a.openStudent(st.ID, 0)
				}
				rows = append(rows, func(gtx C) D {
					return a.listRow(gtx, clk, func(gtx C) D { return a.th.avatar(gtx, st.Name, 42) }, st.Name,
						fmt.Sprintf("#%d · %s", st.ID, st.Email), func(gtx C) D { return a.studentBadge(gtx, st) })
				})
			}
			return a.group(gtx, rows)
		})
	}
	return a.page(gtx, "a.students", items)
}

func (a *App) studentBadge(gtx C, st api.StudentRow) D {
	switch studentState(st) {
	case 1:
		return a.th.badge(gtx, "Active", toneGreen)
	case 2:
		return a.th.badge(gtx, "Waiting", toneAmber)
	case 3:
		d := 1 << 30
		for _, ac := range st.Access {
			if ac.Status == "active" {
				d = min(d, ac.DaysLeft)
			}
		}
		return a.th.badge(gtx, fmt.Sprintf("%dd left", d), toneAmber)
	case 4:
		return a.th.badge(gtx, "Expired", toneRed)
	}
	return a.th.badge(gtx, "Not paid", toneNeutral)
}

// ---------- Payments ----------

func (a *App) layoutAdminPayments(gtx C) D {
	s := a.admin
	segs := []string{fmt.Sprintf("To verify  %d", len(s.claims)), fmt.Sprintf("History  %d", len(s.payments))}
	for i := range segs {
		if a.clk(fmt.Sprint("pseg", i)).Clicked(gtx) {
			s.paySeg = i
		}
	}
	total := 0.0
	for _, p := range s.payments {
		total += p.AmountPaid
	}
	items := []layout.Widget{
		func(gtx C) D {
			return a.chips(gtx, "payseg", segs, func(i int) bool { return i == s.paySeg }, func(i int) *widget.Clickable { return a.clk(fmt.Sprint("pseg", i)) })
		},
		vspace(12),
	}
	if s.paySeg == 0 {
		items = append(items,
			func(gtx C) D {
				return a.th.muted(gtx, "Match each note with your bank statement, then Verify to give access.", 13.5)
			},
			vspace(10),
			func(gtx C) D { return a.claimRows(gtx, 0) })
	} else {
		items = append(items,
			func(gtx C) D {
				return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return a.th.muted(gtx, "Total collected  ", 14) }),
					layout.Rigid(func(gtx C) D { return a.th.money(gtx, total, textOpt{size: 18, weight: font.Black}) }),
				)
			},
			vspace(10),
			func(gtx C) D { return a.paymentRows(gtx, 0) })
	}
	return a.page(gtx, "a.payments", items)
}

func pt(x, y int) image.Point { return image.Pt(x, y) }

// sortedCourses returns courses ordered by id (stable chip order).
func sortedCourses(cs []api.AdminCourse) []api.AdminCourse {
	out := append([]api.AdminCourse(nil), cs...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
