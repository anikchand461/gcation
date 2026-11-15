package ui

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/widget"

	"gcation/mobile/internal/api"
)

const day = 86400

var presets = []struct {
	days  int
	label string
}{{30, "1 month"}, {90, "3 months"}, {180, "6 months"}, {365, "1 year"}}

type studentDetail struct {
	id      int64
	claimID int64 // payment note to pre-select (0 = none)
	data    *api.StudentDetail
	err     string

	course, claim  int  // selected index (claim: 0 = none, i+1 = pending[i])
	fromToday      bool // start now instead of after current access
	amount, days   *field
	note           *field
	name, email    *field
	phone, newPass *field
	filled         bool

	back, give, call, wa, mail, saveEdit, genPw, setPw widget.Clickable
}

func (a *App) openStudent(id, claimID int64) {
	d := &studentDetail{
		id: id, claimID: claimID,
		amount:  newField("Amount received (₹)", "", false, key.HintNumeric),
		days:    newField("Days of access", "", false, key.HintNumeric),
		note:    newArea("Payment notes", "e.g. SBI, paid by Rahul, UTR 4123…, confirmed on call"),
		name:    newField("Full name", "", false, key.HintAny),
		email:   newField("Email", "", false, key.HintEmail),
		phone:   newField("Phone", "", false, key.HintTelephone),
		newPass: newField("New password", "At least 8 characters", false, key.HintAny),
	}
	d.amount.Label = "Amount received (in rupees)"
	d.amount.Ed.Filter = "0123456789."
	d.days.Ed.Filter = "0123456789"
	a.admin.detail = d
	a.loadStudent(d)
}

func (a *App) loadStudent(d *studentDetail) {
	a.async(func() func() {
		data, err := a.api.StudentDetail(d.id)
		return func() {
			if errors.Is(err, api.ErrUnauthorized) {
				a.signedOut("Admin session ended. Please log in again.")
				return
			}
			if err != nil {
				d.err = err.Error()
				return
			}
			d.data = data
			if !d.filled {
				d.filled = true
				d.name.Ed.SetText(data.Student.Name)
				d.email.Ed.SetText(data.Student.Email)
				d.phone.Ed.SetText(data.Student.Phone)
				pending := d.pending()
				for i, c := range pending {
					if c.ID == d.claimID || (d.claimID == 0 && len(pending) == 1) {
						d.pickClaim(i + 1)
					}
				}
				if d.claim == 0 {
					d.pickCourse(0)
				}
			}
		}
	})
}

func (d *studentDetail) pending() []api.ClaimRow {
	var out []api.ClaimRow
	for _, c := range d.data.Claims {
		if c.Status == "pending" {
			out = append(out, c)
		}
	}
	return out
}

func (d *studentDetail) pickCourse(i int) {
	if i < 0 || i >= len(d.data.Courses) {
		return
	}
	d.course = i
	c := d.data.Courses[i]
	d.amount.Ed.SetText(strconv.FormatFloat(c.Price, 'f', -1, 64))
	d.days.Ed.SetText(strconv.Itoa(c.DefaultDays))
}

// pickClaim selects a payment note and fills the form from it.
func (d *studentDetail) pickClaim(i int) {
	d.claim = i
	if i == 0 {
		return
	}
	c := d.pending()[i-1]
	for ci, course := range d.data.Courses {
		if course.ID == c.CourseID {
			d.pickCourse(ci)
		}
	}
	if c.Amount != nil {
		d.amount.Ed.SetText(strconv.FormatFloat(*c.Amount, 'f', -1, 64))
	}
	var parts []string
	for _, p := range []string{c.Method, prefix("paid by ", c.PayerName), prefix("UTR ", c.Reference), prefix("on ", c.PaidOn)} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	d.note.Ed.SetText(strings.Join(parts, ", "))
}

func prefix(p, v string) string {
	if v == "" {
		return ""
	}
	return p + v
}

func (a *App) layoutStudentDetail(gtx C) D {
	d := a.admin.detail
	if d.back.Clicked(gtx) {
		a.admin.detail = nil
		return D{}
	}
	title := "Student"
	if d.data != nil {
		title = d.data.Student.Name
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return a.subBar(gtx, &d.back, title) }),
		layout.Flexed(1, func(gtx C) D {
			if d.data == nil {
				return layout.Center.Layout(gtx, func(gtx C) D {
					if d.err != "" {
						return pad(24).Layout(gtx, func(gtx C) D { return centered(gtx, d.err, a.th, textOpt{size: 15, color: a.th.P.Red}) })
					}
					return a.th.muted(gtx, "Loading…", 15)
				})
			}
			return a.page(gtx, fmt.Sprint("detail", d.id), a.studentDetailItems(gtx, d))
		}),
	)
}

func (a *App) studentDetailItems(gtx C, d *studentDetail) []layout.Widget {
	st := d.data.Student
	pending := d.pending()
	if d.call.Clicked(gtx) {
		a.open(telURL(st.Phone))
	}
	if d.wa.Clicked(gtx) {
		a.open(waURL(st.Phone, fmt.Sprintf("Hi %s, this is G-CATION.", firstName(st.Name))))
	}
	if d.mail.Clicked(gtx) {
		a.open("mailto:" + st.Email)
	}
	for i := range d.data.Courses {
		if a.clk(fmt.Sprint("dc", i)).Clicked(gtx) {
			d.pickCourse(i)
		}
	}
	for i := 0; i <= len(pending); i++ {
		if a.clk(fmt.Sprint("dcl", i)).Clicked(gtx) {
			d.pickClaim(i)
		}
	}
	for i, p := range presets {
		if a.clk(fmt.Sprint("dp", i)).Clicked(gtx) {
			d.days.Ed.SetText(strconv.Itoa(p.days))
		}
	}
	for i := 0; i < 2; i++ {
		if a.clk(fmt.Sprint("ds", i)).Clicked(gtx) {
			d.fromToday = i == 1
		}
	}
	if d.give.Clicked(gtx) {
		a.confirmGrant(d)
	}
	if d.saveEdit.Clicked(gtx) {
		name, email, phone := d.name.Text(), d.email.Text(), d.phone.Text()
		a.adminDo(func() error { return a.api.EditStudent(d.id, name, email, phone) }, "Student details saved.", func() { a.loadStudent(d) })
	}
	if d.genPw.Clicked(gtx) {
		d.newPass.Ed.SetText(randomPassword())
	}
	if d.setPw.Clicked(gtx) {
		pw := d.newPass.Ed.Text()
		if len(pw) < 8 {
			a.toast("Password must be at least 8 characters.", true)
		} else {
			a.confirm("Set a new password?", st.Name+" will be logged out and must use the new password.", "Set password", false, func() {
				a.adminDo(func() error { return a.api.ResetStudentPassword(d.id, pw) }, "", func() {
					a.copyTextNow(pw, "Password set and copied. Share it with the student.")
					d.newPass.Ed.SetText("")
				})
			})
		}
	}

	items := []layout.Widget{
		// Contact card
		func(gtx C) D {
			return a.th.card(gtx, 16, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx C) D { return a.th.avatar(gtx, st.Name, 54) }),
							layout.Rigid(hspace(12)),
							layout.Flexed(1, func(gtx C) D {
								return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
									layout.Rigid(func(gtx C) D { return a.th.label(gtx, st.Name, textOpt{size: 17, weight: font.Bold}) }),
									layout.Rigid(func(gtx C) D { return a.th.muted(gtx, fmt.Sprintf("#%d · %s", st.ID, st.Email), 13) }),
									layout.Rigid(func(gtx C) D { return a.th.muted(gtx, "Joined "+fmtDate(st.CreatedAt), 12) }),
								)
							}),
						)
					}),
					layout.Rigid(vspace(12)),
					layout.Rigid(func(gtx C) D {
						kids := []layout.FlexChild{}
						if st.Phone != "" {
							kids = append(kids,
								layout.Flexed(1, func(gtx C) D {
									return a.th.button(gtx, &d.call, "Call", btnOpt{kind: btnSoft, small: true, full: true, icon: icCall})
								}),
								layout.Rigid(hspace(6)),
								layout.Flexed(1, func(gtx C) D {
									return a.th.button(gtx, &d.wa, "WhatsApp", btnOpt{kind: btnSoft, small: true, full: true, icon: icChat})
								}),
								layout.Rigid(hspace(6)))
						}
						kids = append(kids, layout.Flexed(1, func(gtx C) D {
							return a.th.button(gtx, &d.mail, "Email", btnOpt{kind: btnSoft, small: true, full: true, icon: icMail})
						}))
						return layout.Flex{}.Layout(gtx, kids...)
					}),
				)
			})
		},
		a.groupLabel("COURSE ACCESS"),
		func(gtx C) D {
			rows := make([]layout.Widget, 0, len(d.data.Courses))
			for _, c := range d.data.Courses {
				c := c
				rows = append(rows, func(gtx C) D {
					sub, tn, lb := "No access yet", toneNeutral, "Not paid"
					switch c.Access.Status {
					case "active":
						sub, tn, lb = fmt.Sprintf("Until %s · %s left", fmtDate(c.Access.Until), plural(c.Access.DaysLeft, "day")), toneGreen, "Active"
					case "scheduled":
						sub, tn, lb = "Scheduled until "+fmtDate(c.Access.Until), toneViolet, "Later"
					case "expired":
						sub, tn, lb = "Ended "+fmtDate(c.Access.Until), toneRed, "Expired"
					}
					return a.listRow(gtx, nil, func(gtx C) D { return a.iconTile(gtx, icSchool, 40) }, c.Title, sub,
						func(gtx C) D { return a.th.badge(gtx, lb, tn) })
				})
			}
			return a.group(gtx, rows)
		},
	}

	if len(pending) > 0 {
		items = append(items, a.groupLabel("PAYMENT NOTES FROM STUDENT"))
		for _, c := range pending {
			c := c
			items = append(items, func(gtx C) D {
				return a.th.notice(gtx, icHourglass, toneAmber, func(gtx C) D {
					amount := "Amount not given"
					if c.Amount != nil {
						amount = rupees(*c.Amount)
					}
					line := fmt.Sprintf("%s to %s", amount, a.qrLabel(c.QRSlot))
					age := ""
					if c.Age != nil {
						age = fmt.Sprint("age ", *c.Age)
					}
					for _, p := range []string{prefix("via ", c.Method), prefix("payer ", c.PayerName), c.PaidOn, prefix("UTR ", c.Reference), age, prefix("referred by ", c.ReferredBy)} {
						if p != "" {
							line += " · " + p
						}
					}
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return a.th.label(gtx, line, textOpt{size: 13.5, weight: font.SemiBold, color: a.th.P.Amber})
						}),
						layout.Rigid(func(gtx C) D {
							return a.th.label(gtx, c.CourseTitle+" · sent "+fmtDateTime(c.CreatedAt), textOpt{size: 12, color: a.th.P.Amber})
						}),
					)
				})
			}, vspace(8))
		}
	}

	// Give access
	items = append(items, a.groupLabel("GIVE COURSE ACCESS"), func(gtx C) D {
		return a.th.card(gtx, 16, func(gtx C) D {
			courseLabels := make([]string, len(d.data.Courses))
			for i, c := range d.data.Courses {
				courseLabels[i] = c.Title
			}
			presetLabels := make([]string, len(presets))
			for i, p := range presets {
				presetLabels[i] = p.label
			}
			daysNow, _ := strconv.Atoi(d.days.Text())
			kids := []layout.FlexChild{
				layout.Rigid(func(gtx C) D {
					return a.th.muted(gtx, "Do this after you have confirmed the payment in your bank account.", 13)
				}),
				layout.Rigid(vspace(12)),
				layout.Rigid(func(gtx C) D { return a.smallLabel(gtx, "Course") }),
				layout.Rigid(func(gtx C) D {
					return a.chips(gtx, fmt.Sprint("dcourse", d.id), courseLabels, func(i int) bool { return i == d.course }, func(i int) *widget.Clickable { return a.clk(fmt.Sprint("dc", i)) })
				}),
			}
			if len(pending) > 0 {
				labels := []string{"No note"}
				for _, c := range pending {
					l := fmtDate(c.CreatedAt)
					if c.Amount != nil {
						l += " · " + rupees(*c.Amount)
					}
					labels = append(labels, l)
				}
				kids = append(kids, layout.Rigid(vspace(12)), layout.Rigid(func(gtx C) D { return a.smallLabel(gtx, "Matches payment note") }),
					layout.Rigid(func(gtx C) D {
						return a.chips(gtx, fmt.Sprint("dclaim", d.id), labels, func(i int) bool { return i == d.claim }, func(i int) *widget.Clickable { return a.clk(fmt.Sprint("dcl", i)) })
					}))
			}
			kids = append(kids,
				layout.Rigid(vspace(12)),
				layout.Rigid(func(gtx C) D { return a.th.field(gtx, d.amount) }),
				layout.Rigid(vspace(12)),
				layout.Rigid(func(gtx C) D { return a.smallLabel(gtx, "Access period") }),
				layout.Rigid(func(gtx C) D {
					return a.chips(gtx, fmt.Sprint("dpreset", d.id), presetLabels, func(i int) bool { return presets[i].days == daysNow }, func(i int) *widget.Clickable { return a.clk(fmt.Sprint("dp", i)) })
				}),
				layout.Rigid(vspace(8)),
				layout.Rigid(func(gtx C) D { return a.th.field(gtx, d.days) }),
				layout.Rigid(vspace(12)),
				layout.Rigid(func(gtx C) D { return a.smallLabel(gtx, "Start") }),
				layout.Rigid(func(gtx C) D {
					return a.chips(gtx, fmt.Sprint("dstart", d.id), []string{"After current access", "From today"}, func(i int) bool { return (i == 1) == d.fromToday }, func(i int) *widget.Clickable { return a.clk(fmt.Sprint("ds", i)) })
				}),
				layout.Rigid(vspace(12)),
				layout.Rigid(func(gtx C) D { return a.th.field(gtx, d.note) }),
				layout.Rigid(vspace(12)),
				layout.Rigid(func(gtx C) D {
					start, end, cont := d.period()
					msg := fmt.Sprintf("Access: %s → %s", fmtDate(start), fmtDate(end))
					if cont {
						msg += " (after current access)"
					}
					return a.th.notice(gtx, icCalendar, toneViolet, func(gtx C) D { return a.th.label(gtx, msg, textOpt{size: 13.5, color: a.th.P.Accent}) })
				}),
				layout.Rigid(vspace(14)),
				layout.Rigid(func(gtx C) D {
					return a.th.button(gtx, &d.give, "Give access", btnOpt{full: true, kind: btnSuccess, icon: icCheck})
				}),
			)
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
		})
	})

	// History
	items = append(items, a.groupLabel("ACCESS & PAYMENT HISTORY"), func(gtx C) D {
		if len(d.data.Enrollments) == 0 {
			return a.group(gtx, []layout.Widget{func(gtx C) D {
				return a.listRow(gtx, nil, func(gtx C) D { return a.iconTile(gtx, icReceipt, 40) }, "No access given yet", "", nil)
			}})
		}
		now := time.Now().Unix()
		rows := make([]layout.Widget, 0, len(d.data.Enrollments))
		for _, e := range d.data.Enrollments {
			e := e
			rec, rev := a.clk(fmt.Sprint("dr", e.ID)), a.clk(fmt.Sprint("dv", e.ID))
			if rec.Clicked(gtx) {
				a.saveAdminReceipt(e)
			}
			if rev.Clicked(gtx) {
				a.confirm("Revoke this access?", "The student loses access for this period right away. The payment record is kept.", "Revoke", true, func() {
					a.adminDo(func() error { return a.api.Revoke(e.ID) }, "Access revoked.", func() { a.loadStudent(d) })
				})
			}
			rows = append(rows, func(gtx C) D {
				sub := fmt.Sprintf("%s → %s · given %s", fmtDate(e.StartAt), fmtDate(e.EndAt), fmtDate(e.GrantedAt))
				if e.RevokedAt != nil {
					sub += " · revoked"
				}
				if e.Note != "" {
					sub += "\n" + e.Note
				}
				return a.listRow(gtx, nil, func(gtx C) D { return a.iconTile(gtx, icReceipt, 40) }, e.CourseTitle+" · "+rupees(e.AmountPaid), sub,
					func(gtx C) D {
						kids := []layout.FlexChild{a.barIcon(rec, icDownload, 0)}
						if e.RevokedAt == nil && e.EndAt > now {
							kids = append(kids, layout.Rigid(func(gtx C) D { return a.th.button(gtx, rev, "Revoke", btnOpt{kind: btnDanger, small: true}) }))
						}
						return layout.Flex{Alignment: layout.Middle}.Layout(gtx, kids...)
					})
			})
		}
		return a.group(gtx, rows)
	})

	// Account
	items = append(items, a.groupLabel("ACCOUNT"), func(gtx C) D {
		return a.th.card(gtx, 16, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return a.cardTitle(gtx, icEdit, "Edit student details") }),
				layout.Rigid(vspace(10)),
				layout.Rigid(func(gtx C) D { return a.th.field(gtx, d.name) }),
				layout.Rigid(vspace(10)),
				layout.Rigid(func(gtx C) D { return a.th.field(gtx, d.email) }),
				layout.Rigid(vspace(10)),
				layout.Rigid(func(gtx C) D { return a.th.field(gtx, d.phone) }),
				layout.Rigid(vspace(12)),
				layout.Rigid(func(gtx C) D {
					return a.th.button(gtx, &d.saveEdit, "Save details", btnOpt{full: true, kind: btnOutline, icon: icTick})
				}),
				layout.Rigid(vspace(18)),
				layout.Rigid(func(gtx C) D { return a.th.divider(gtx) }),
				layout.Rigid(vspace(16)),
				layout.Rigid(func(gtx C) D { return a.cardTitle(gtx, icKey, "Reset password") }),
				layout.Rigid(vspace(4)),
				layout.Rigid(func(gtx C) D {
					return a.th.muted(gtx, "For students who forgot their password. They are logged out everywhere.", 12.5)
				}),
				layout.Rigid(vspace(10)),
				layout.Rigid(func(gtx C) D { return a.th.field(gtx, d.newPass) }),
				layout.Rigid(vspace(10)),
				layout.Rigid(func(gtx C) D {
					return layout.Flex{}.Layout(gtx,
						layout.Flexed(1, func(gtx C) D {
							return a.th.button(gtx, &d.genPw, "Generate", btnOpt{full: true, kind: btnSoft, icon: icRefresh})
						}),
						layout.Rigid(hspace(8)),
						layout.Flexed(1, func(gtx C) D { return a.th.button(gtx, &d.setPw, "Set password", btnOpt{full: true, icon: icKey}) }),
					)
				}),
			)
		})
	})
	return items
}

// period returns the access start and end the form would create, and whether it continues current access.
func (d *studentDetail) period() (start, end int64, continues bool) {
	days, _ := strconv.Atoi(d.days.Text())
	start = time.Now().Unix()
	if d.course < len(d.data.Courses) && !d.fromToday {
		acc := d.data.Courses[d.course].Access
		if acc.Status == "active" || acc.Status == "scheduled" {
			start, continues = acc.Until, true
		}
	}
	return start, start + int64(days)*day, continues
}

func (a *App) confirmGrant(d *studentDetail) {
	if len(d.data.Courses) == 0 {
		a.toast("Add a course first (More → Courses).", true)
		return
	}
	days, err := strconv.Atoi(d.days.Text())
	if err != nil || days < 1 || days > 3650 {
		a.toast("Enter the number of days (1 to 3650).", true)
		return
	}
	amount, err := strconv.ParseFloat(strings.TrimSpace(d.amount.Ed.Text()), 64)
	if err != nil || amount < 0 {
		a.toast("Enter the amount received as a number.", true)
		return
	}
	course := d.data.Courses[d.course]
	in := api.GrantIn{CourseID: course.ID, Days: days, Start: "after_current", Amount: amount, Note: d.note.Ed.Text()}
	if d.fromToday {
		in.Start = "now"
	}
	if d.claim > 0 {
		id := d.pending()[d.claim-1].ID
		in.ClaimID = &id
	}
	msg := fmt.Sprintf("%s gets \"%s\" for %s. Amount recorded: %s.", d.data.Student.Name, course.Title, plural(days, "day"), rupees(amount))
	a.confirm("Give access?", msg, "Give access", false, func() {
		a.adminDo(func() error { return a.api.Grant(d.id, in) }, "Access given to "+d.data.Student.Name+".", func() {
			d.claim, d.claimID = 0, 0
			d.note.Ed.SetText("")
			a.loadStudent(d)
		})
	})
}

func (a *App) smallLabel(gtx C, s string) D {
	return layout.Inset{Bottom: 6}.Layout(gtx, func(gtx C) D {
		return a.th.label(gtx, s, textOpt{size: 13, weight: font.SemiBold, color: a.th.P.Ink2})
	})
}

func randomPassword() string {
	const chars = "abcdefghjkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 10)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		b[i] = chars[n.Int64()]
	}
	return string(b)
}
