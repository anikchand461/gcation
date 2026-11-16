package ui

import (
	"errors"
	"fmt"
	"image"
	"io"
	"regexp"
	"strconv"
	"strings"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/x/explorer"

	"gcation/mobile/internal/api"
)

// ---------- Updates (announcements) ----------

type composer struct {
	editing                 int64 // 0 = new update
	title, msg, link, label *field
	toTeam, toAll           bool           // management team; every paid student
	toCourse                map[int64]bool // students of these courses
	audOpen                 bool           // the "Who should see this?" list is open
	post, cancel            widget.Clickable
}

func (c *composer) init() {
	c.title = newField("Title", "e.g. Live class today at 7 PM", false, key.HintAny)
	c.msg = newArea("Message (optional)", "Instructions, timings, rules…")
	c.link = newField("Link (optional)", "https://meet.google.com/…", false, key.HintURL)
	c.label = newField("Button text", "Join Google Meet", false, key.HintAny)
	c.toAll, c.toCourse = true, map[int64]bool{}
}

func (c *composer) reset() {
	c.editing, c.toTeam, c.toAll, c.toCourse, c.audOpen = 0, false, true, map[int64]bool{}, false
	for _, f := range []*field{c.title, c.msg, c.link, c.label} {
		f.Ed.SetText("")
	}
}

// githubURL is the project's source code.
const githubURL = "https://github.com/anikchand461/gcation"

var templates = []struct{ name, title, msg, label, link string }{
	{"Community link", "Join our student community", "Join the official G-CATION group for class updates, practice material and doubts.\nPlease keep the group respectful.", "Join community", "https://chat.whatsapp.com/"},
	{"Google Meet class", "Live class today, 7:00 PM", "Please join 5 minutes early. Keep your mic muted until you are asked to speak.", "Join Google Meet", "https://meet.google.com/"},
	{"Instructions", "Important instructions", "1. \n2. \n3. ", "", ""},
}

func (a *App) layoutAdminUpdates(gtx C) D {
	s := a.admin
	c := &s.compose
	courses := sortedCourses(s.courses)
	tplLabels := make([]string, len(templates))
	for i, t := range templates {
		tplLabels[i] = t.name
		if a.clk(fmt.Sprint("tpl", i)).Clicked(gtx) {
			c.title.Ed.SetText(t.title)
			c.msg.Ed.SetText(t.msg)
			c.label.Ed.SetText(t.label)
			c.link.Ed.SetText(t.link)
		}
	}
	// Audience list: 0 management team, 1 every paid student, then one per course.
	audLabels := []string{fmt.Sprintf("Management team (%d)", s.audience["management"]), fmt.Sprintf("All paid students (%d)", s.audience["all"])}
	for _, co := range courses {
		audLabels = append(audLabels, fmt.Sprintf("%s students (%d)", co.Title, s.audience[fmt.Sprint(co.ID)]))
	}
	audChecked := func(i int) bool {
		switch i {
		case 0:
			return c.toTeam
		case 1:
			return c.toAll
		}
		return c.toAll || c.toCourse[courses[i-2].ID]
	}
	audLocked := func(i int) bool { return i >= 2 && c.toAll } // covered by "All paid students"
	audToggle := func(i int) {
		switch i {
		case 0:
			c.toTeam = !c.toTeam
		case 1:
			c.toAll = !c.toAll
		default:
			c.toCourse[courses[i-2].ID] = !c.toCourse[courses[i-2].ID]
		}
	}
	if c.cancel.Clicked(gtx) {
		c.reset()
	}
	if c.post.Clicked(gtx) {
		a.submitAnnouncement(courses)
	}

	items := []layout.Widget{
		func(gtx C) D {
			return a.th.muted(gtx, "Share Google Meet links, the community group or instructions. Only students with active access see these.", 13.5)
		},
		vspace(12),
		func(gtx C) D {
			return a.th.card(gtx, 16, func(gtx C) D {
				head := "New update"
				if c.editing != 0 {
					head = "Edit update"
				}
				kids := []layout.FlexChild{
					layout.Rigid(func(gtx C) D { return a.cardTitle(gtx, icMegaphone, head) }),
					layout.Rigid(vspace(10)),
					layout.Rigid(func(gtx C) D { return a.smallLabel(gtx, "Quick start") }),
					layout.Rigid(func(gtx C) D {
						return a.chips(gtx, "templates", tplLabels, func(int) bool { return false }, func(i int) *widget.Clickable { return a.clk(fmt.Sprint("tpl", i)) })
					}),
				}
				for _, f := range []*field{c.title, c.msg, c.link, c.label} {
					kids = append(kids, layout.Rigid(vspace(12)), layout.Rigid(func(gtx C) D { return a.th.field(gtx, f) }))
				}
				kids = append(kids,
					layout.Rigid(vspace(12)),
					layout.Rigid(func(gtx C) D {
						return a.checkDropdown(gtx, "audience", "Who should see this?", "Choose who sees this update", audLabels, audChecked, audLocked, audToggle, &c.audOpen)
					}),
					layout.Rigid(vspace(16)),
					layout.Rigid(func(gtx C) D {
						label := "Post update"
						if c.editing != 0 {
							label = "Save changes"
						}
						return a.th.button(gtx, &c.post, label, btnOpt{full: true, icon: icSend})
					}),
					layout.Rigid(func(gtx C) D {
						if c.editing == 0 {
							return D{}
						}
						return layout.Inset{Top: 6}.Layout(gtx, func(gtx C) D { return a.th.button(gtx, &c.cancel, "Cancel edit", btnOpt{full: true, kind: btnGhost}) })
					}),
				)
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
			})
		},
		a.groupLabel(fmt.Sprintf("POSTED (%d)", len(s.anns))),
	}
	if len(s.anns) == 0 {
		items = append(items, func(gtx C) D {
			return a.emptyState(gtx, icMegaphone, "Nothing posted yet", "Use a quick start above to share your community link or a Google Meet class.", nil, "")
		})
	}
	for _, an := range s.anns {
		an := an
		edit, del := a.clk(fmt.Sprint("ae", an.ID)), a.clk(fmt.Sprint("ad", an.ID))
		if edit.Clicked(gtx) {
			c.editing = an.ID
			c.title.Ed.SetText(an.Title)
			c.msg.Ed.SetText(an.Message)
			c.link.Ed.SetText(an.LinkURL)
			c.label.Ed.SetText(an.LinkLabel)
			c.toTeam, c.toAll, c.toCourse, c.audOpen = an.ToManagement, an.ToAllStudents, map[int64]bool{}, false
			for _, id := range an.CourseIDs {
				c.toCourse[id] = true
			}
			a.list("a.updates").Position.First = 0
			a.list("a.updates").Position.Offset = 0
		}
		if del.Clicked(gtx) {
			a.confirm("Delete this update?", "Nobody will see it any more.", "Delete", true, func() {
				a.adminDo(func() error { return a.api.DeleteAnnouncement(an.ID) }, "Update deleted.", nil)
			})
		}
		items = append(items, func(gtx C) D {
			return a.th.card(gtx, 16, func(gtx C) D {
				who := an.AudienceLabel
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return a.th.label(gtx, an.Title, textOpt{size: 15.5, weight: font.Bold}) }),
					layout.Rigid(func(gtx C) D { return a.th.muted(gtx, fmtDateTime(an.CreatedAt)+" · "+who, 12) }),
					layout.Rigid(func(gtx C) D {
						if an.Message == "" {
							return D{}
						}
						return layout.Inset{Top: 8}.Layout(gtx, func(gtx C) D { return a.th.body(gtx, an.Message) })
					}),
					layout.Rigid(func(gtx C) D {
						if an.LinkURL == "" {
							return D{}
						}
						return layout.Inset{Top: 6}.Layout(gtx, func(gtx C) D {
							return a.th.label(gtx, an.LinkURL, textOpt{size: 12.5, color: a.th.P.Accent, maxLines: 1})
						})
					}),
					layout.Rigid(vspace(10)),
					layout.Rigid(func(gtx C) D {
						return layout.Flex{}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								return a.th.button(gtx, edit, "Edit", btnOpt{kind: btnOutline, small: true, icon: icEdit})
							}),
							layout.Rigid(hspace(8)),
							layout.Rigid(func(gtx C) D { return a.th.button(gtx, del, "Delete", btnOpt{kind: btnDanger, small: true}) }),
						)
					}),
				)
			})
		}, vspace(10))
	}
	return a.page(gtx, "a.updates", items)
}

// fullLink adds https:// to a link typed without it (meet.google.com/abc → https://meet.google.com/abc).
var linkScheme = regexp.MustCompile(`(?i)^https?:`)

func fullLink(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	return "https://" + strings.TrimLeft(linkScheme.ReplaceAllString(v, ""), "/")
}

func (a *App) submitAnnouncement(courses []api.AdminCourse) {
	s := a.admin
	c := &s.compose
	in := api.AnnouncementIn{Title: c.title.Text(), Message: c.msg.Ed.Text(), LinkURL: fullLink(c.link.Text()), LinkLabel: c.label.Text()}
	c.link.Ed.SetText(in.LinkURL)
	switch {
	case in.Title == "":
		a.toast("Add a title.", true)
		return
	case in.LinkURL != "" && !strings.HasPrefix(in.LinkURL, "https://"):
		a.toast("The link must start with https://", true)
		return
	case in.LinkURL == "https://meet.google.com/" || in.LinkURL == "https://chat.whatsapp.com/":
		a.toast("Paste the full group or meeting link, not just the website.", true)
		return
	}
	in.ToManagement, in.ToAllStudents, in.CourseIDs = c.toTeam, c.toAll, []int64{}
	var who []string
	if c.toTeam {
		who = append(who, "The management team")
	}
	if c.toAll {
		who = append(who, "all paid students")
	} else {
		for _, co := range courses {
			if c.toCourse[co.ID] {
				in.CourseIDs = append(in.CourseIDs, co.ID)
				who = append(who, co.Title+" students")
			}
		}
	}
	if len(who) == 0 {
		a.toast("Choose who should see this update.", true)
		return
	}
	id := c.editing
	run := func() {
		msg := "Update posted."
		if id != 0 {
			msg = "Update saved."
		}
		a.adminDo(func() error { return a.api.SaveAnnouncement(id, in) }, msg, c.reset)
	}
	if id != 0 {
		run()
		return
	}
	a.confirm("Post this update?", strings.Join(who, ", ")+" will see it right away.", "Post", false, run)
}

// ---------- More ----------

type moreState struct {
	phone                                   *field
	user, newPw, confirmPw, currentPw       *field
	invite                                  *field // management referral code
	methods                                 map[string]*methodForm
	course                                  courseForm
	rows                                    [8]widget.Clickable
	back, savePhone, saveAccount, addCourse widget.Clickable
	saveInvite, copyLink                    widget.Clickable
}

type methodForm struct {
	label, upi, payee            *field
	active                       bool
	toggle, save, upload, remove widget.Clickable
}

type courseForm struct {
	id                                         int64
	title, desc, feats, price, days, classroom *field
	active                                     bool
	toggle, save, del                          widget.Clickable
}

func (m *moreState) init() {
	m.phone = newField("Support phone number", "+91 98765 43210", false, key.HintTelephone)
	m.user = newField("Username", "", false, key.HintAny)
	m.newPw = newField("New password (optional)", "Leave empty to keep it", true, key.HintAny)
	m.confirmPw = newField("Confirm new password", "", true, key.HintAny)
	m.currentPw = newField("Current password", "Needed to save", true, key.HintAny)
	m.invite = newField("Referral code", "e.g. GCTEAM2026", false, key.HintAny)
	m.methods = map[string]*methodForm{}
	for _, slot := range []string{"primary", "secondary"} {
		m.methods[slot] = &methodForm{
			label: newField("Name shown to students", "", false, key.HintAny),
			upi:   newField("UPI ID", "yourname@okaxis", false, key.HintEmail),
			payee: newField("Payee name", "G-CATION", false, key.HintAny),
		}
	}
	c := &m.course
	c.title = newField("Title", "", false, key.HintAny)
	c.desc = newArea("Description", "What students learn")
	c.feats = newArea("Highlights (one per line)", "Live online classes\nWeekly practice")
	c.price = newField("Price (in rupees)", "", false, key.HintNumeric)
	c.price.Ed.Filter = "0123456789."
	c.days = newField("Default access period (days)", "365", false, key.HintNumeric)
	c.days.Ed.Filter = "0123456789"
	c.classroom = newField("Classroom link (optional)", "https://meet.google.com/…", false, key.HintURL)
}

// fill copies freshly loaded settings into the forms (only for screens not being edited).
func (m *moreState) fill(s *adminState) {
	if s.sub != morePhone {
		m.phone.Ed.SetText(s.phone)
	}
	if s.sub != moreAccount {
		m.user.Ed.SetText(s.username)
		m.invite.Ed.SetText(s.invite)
	}
	if s.sub != moreQR {
		for _, me := range s.methods {
			if f := m.methods[me.Slot]; f != nil {
				f.label.Ed.SetText(me.Label)
				f.upi.Ed.SetText(me.UPIID)
				f.payee.Ed.SetText(me.PayeeName)
				f.active = me.IsActive
			}
		}
	}
}

func (a *App) layoutAdminMore(gtx C) D {
	s := a.admin
	m := &s.more
	type row struct {
		ic         *widget.Icon
		title, sub string
		do         func()
	}
	phoneSub := s.phone
	if phoneSub == "" {
		phoneSub = "Not set"
	}
	rows := []row{
		{icCall, "Support phone", phoneSub, func() { s.sub = morePhone }},
		{icPay, "Payment QR codes", "Primary and backup QR, UPI IDs", func() { s.sub = moreQR }},
		{icSchool, "Courses", plural(len(s.courses), "course"), func() { s.sub = moreCourses }},
		{icShield, "Management team", plural(len(s.managers), "member"), func() { s.sub = moreManagement }},
		{icKey, "Admin account", "Signed in as " + s.username, func() { s.sub = moreAccount }},
		{icOpen, "Open website", "gcation.vercel.app", func() { a.open(a.webURL) }},
		{icCode, "Source code on GitHub", "github.com/anikchand461/gcation", func() { a.open(githubURL) }},
		{icLogout, "Log out", "Leave the admin dashboard", func() {
			a.confirm("Log out?", "You will need the admin password to log in again.", "Log out", true, a.adminLogout)
		}},
	}
	for i, r := range rows {
		if m.rows[i].Clicked(gtx) {
			r.do()
		}
	}
	widgets := make([]layout.Widget, len(rows))
	for i, r := range rows {
		widgets[i] = func(gtx C) D {
			return a.listRow(gtx, &m.rows[i], func(gtx C) D { return a.iconTile(gtx, r.ic, 40) }, r.title, r.sub,
				func(gtx C) D { return iconSized(gtx, icChevron, 22, a.th.P.Faint) })
		}
	}
	items := []layout.Widget{
		a.groupLabel("SETTINGS"), func(gtx C) D { return a.group(gtx, widgets[:5]) },
		a.groupLabel("APP"), func(gtx C) D { return a.group(gtx, widgets[5:]) },
	}
	return a.page(gtx, "a.more", items)
}

func (a *App) layoutMoreSub(gtx C) D {
	s := a.admin
	m := &s.more
	if m.back.Clicked(gtx) {
		s.back()
		return D{}
	}
	title := map[moreScreen]string{morePhone: "Support phone", moreQR: "Payment QR codes", moreCourses: "Courses", moreCourseEdit: "Course", moreAccount: "Admin account", moreManagement: "Management team"}[s.sub]
	if s.sub == moreCourseEdit && m.course.id == 0 {
		title = "New course"
	}
	var items []layout.Widget
	switch s.sub {
	case morePhone:
		items = a.phoneItems(gtx)
	case moreQR:
		items = a.qrItems(gtx)
	case moreCourses:
		items = a.courseListItems(gtx)
	case moreCourseEdit:
		items = a.courseEditItems(gtx)
	case moreAccount:
		items = a.accountItems(gtx)
	case moreManagement:
		items = a.managementItems(gtx)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return a.subBar(gtx, &m.back, title) }),
		layout.Flexed(1, func(gtx C) D { return a.page(gtx, fmt.Sprint("more", s.sub), items) }),
	)
}

func (a *App) phoneItems(gtx C) []layout.Widget {
	m := &a.admin.more
	if m.savePhone.Clicked(gtx) {
		phone := m.phone.Text()
		a.adminDo(func() error { return a.api.SaveSupportPhone(phone) }, "Support number saved.", nil)
	}
	return []layout.Widget{
		func(gtx C) D {
			return a.th.muted(gtx, "Students are asked to call or WhatsApp this number after paying. It is also printed on receipts.", 13.5)
		},
		vspace(14),
		func(gtx C) D { return a.th.field(gtx, m.phone) },
		vspace(14),
		func(gtx C) D { return a.th.button(gtx, &m.savePhone, "Save", btnOpt{full: true, icon: icTick}) },
	}
}

func (a *App) qrItems(gtx C) []layout.Widget {
	s := a.admin
	items := []layout.Widget{func(gtx C) D {
		return a.th.notice(gtx, icInfo, toneBlue, func(gtx C) D {
			return a.th.label(gtx, "Students see the Primary QR first and can switch to the Backup QR. Upload your QR image, or just enter a UPI ID and the app makes a QR with the amount filled in.", textOpt{size: 13, color: a.th.P.Blue})
		})
	}}
	for _, me := range s.methods {
		me := me
		f := s.more.methods[me.Slot]
		if f == nil {
			continue
		}
		if f.toggle.Clicked(gtx) {
			f.active = !f.active
		}
		if f.save.Clicked(gtx) {
			in := api.MethodIn{Label: f.label.Text(), UPIID: f.upi.Text(), PayeeName: f.payee.Text(), IsActive: f.active}
			a.adminDo(func() error { return a.api.SaveMethod(me.Slot, in) }, "QR settings saved.", nil)
		}
		if f.upload.Clicked(gtx) {
			a.pickQRImage(me.Slot)
		}
		if f.remove.Clicked(gtx) {
			a.confirm("Remove QR image?", "Students will see a QR made from the UPI ID instead.", "Remove", true, func() {
				a.adminDo(func() error { return a.api.RemoveQR(me.Slot) }, "QR image removed.", nil)
			})
		}
		items = append(items, vspace(14), func(gtx C) D {
			return a.th.card(gtx, 16, func(gtx C) D {
				name := "Primary QR"
				if me.Slot == "secondary" {
					name = "Backup QR"
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
							layout.Flexed(1, func(gtx C) D { return a.cardTitle(gtx, icPay, name) }),
							layout.Rigid(func(gtx C) D { return a.switchChip(gtx, &f.toggle, f.active, "Shown", "Hidden") }),
						)
					}),
					layout.Rigid(vspace(12)),
					layout.Rigid(func(gtx C) D { return a.adminQRPreview(gtx, me, f) }),
					layout.Rigid(vspace(12)),
					layout.Rigid(func(gtx C) D { return a.th.field(gtx, f.label) }),
					layout.Rigid(vspace(10)),
					layout.Rigid(func(gtx C) D { return a.th.field(gtx, f.upi) }),
					layout.Rigid(vspace(10)),
					layout.Rigid(func(gtx C) D { return a.th.field(gtx, f.payee) }),
					layout.Rigid(vspace(14)),
					layout.Rigid(func(gtx C) D { return a.th.button(gtx, &f.save, "Save", btnOpt{full: true, icon: icTick}) }),
					layout.Rigid(vspace(8)),
					layout.Rigid(func(gtx C) D {
						label := "Upload QR image"
						if me.HasImage {
							label = "Replace QR image"
						}
						kids := []layout.FlexChild{layout.Flexed(1, func(gtx C) D {
							return a.th.button(gtx, &f.upload, label, btnOpt{full: true, kind: btnOutline, icon: icUpload})
						})}
						if me.HasImage {
							kids = append(kids, layout.Rigid(hspace(8)), layout.Rigid(func(gtx C) D {
								return a.th.button(gtx, &f.remove, "Remove", btnOpt{kind: btnDanger})
							}))
						}
						return layout.Flex{}.Layout(gtx, kids...)
					}),
				)
			})
		})
	}
	return items
}

func (a *App) adminQRPreview(gtx C, me api.AdminMethod, f *methodForm) D {
	side := min(gtx.Constraints.Max.X, gtx.Dp(200))
	return layout.Center.Layout(gtx, func(gtx C) D {
		size := image.Pt(side, side)
		gtx.Constraints = layout.Exact(size)
		fillRound(gtx, size, 16, white)
		strokeRound(gtx, size, 16, 1, a.th.P.Line)
		inner := image.Pt(side-gtx.Dp(20), side-gtx.Dp(20))
		st := pushOffset(gtx, image.Pt(gtx.Dp(10), gtx.Dp(10)))
		defer st.Pop()
		gtx.Constraints = layout.Exact(inner)
		switch {
		case me.HasImage:
			if op, ok := a.image(me.ImagePath()); ok {
				widget.Image{Src: op, Fit: widget.Contain, Position: layout.Center}.Layout(gtx)
			}
		case f.upi.Text() != "":
			widget.Image{Src: a.qr("upi://pay?pa=" + f.upi.Text() + "&cu=INR"), Fit: widget.Contain, Position: layout.Center}.Layout(gtx)
		default:
			layout.Center.Layout(gtx, func(gtx C) D { return a.th.label(gtx, "No QR yet", textOpt{size: 13, color: rgb(0x67628a)}) })
		}
		return D{Size: size}
	})
}

// pickQRImage lets the admin choose a QR image from the phone and uploads it.
func (a *App) pickQRImage(slot string) {
	go func() {
		r, err := a.expl.ChooseFile(".png", ".jpg", ".jpeg", ".webp")
		if err != nil {
			if !errors.Is(err, explorer.ErrUserDecline) {
				a.post(func() { a.toast("Could not open the file: "+err.Error(), true) })
			}
			return
		}
		data, err := io.ReadAll(io.LimitReader(r, 2<<20+1))
		r.Close()
		if err == nil && len(data) > 2<<20 {
			err = errors.New("the image must be smaller than 2 MB")
		}
		if err != nil {
			a.post(func() { a.toast("Could not use this image: "+err.Error(), true) })
			return
		}
		a.post(func() {
			a.toast("Uploading QR image…", false)
			a.adminDo(func() error { return a.api.UploadQR(slot, data) }, "QR image uploaded. Students see it right away.", func() {
				delete(a.imgs, a.methodImagePath(slot)) // show the new image
			})
		})
	}()
}

func (a *App) methodImagePath(slot string) string {
	for _, m := range a.admin.methods {
		if m.Slot == slot {
			return m.ImagePath()
		}
	}
	return ""
}

func (a *App) switchChip(gtx C, clk *widget.Clickable, on bool, onLabel, offLabel string) D {
	label, tn := offLabel, toneNeutral
	if on {
		label, tn = onLabel, toneGreen
	}
	return clk.Layout(gtx, func(gtx C) D { return a.th.badge(gtx, label+"  ⇄", tn) })
}

func (a *App) courseListItems(gtx C) []layout.Widget {
	s := a.admin
	m := &s.more
	if m.addCourse.Clicked(gtx) {
		a.editCourse(api.AdminCourse{DefaultDays: 365, IsActive: true})
	}
	courses := sortedCourses(s.courses)
	rows := make([]layout.Widget, 0, len(courses))
	for _, c := range courses {
		c := c
		clk := a.clk(fmt.Sprint("mc", c.ID))
		if clk.Clicked(gtx) {
			a.editCourse(c)
		}
		rows = append(rows, func(gtx C) D {
			return a.listRow(gtx, clk, func(gtx C) D {
				return a.thumb(gtx, api.Course{ID: c.ID, HasThumbnail: c.HasThumbnail, ThumbnailUpdatedAt: c.ThumbnailAt}, image.Pt(gtx.Dp(44), gtx.Dp(44)), 12)
			}, c.Title, fmt.Sprintf("%s · %s", rupees(c.Price), durationLabel(c.DefaultDays)), func(gtx C) D {
				if c.IsActive {
					return a.th.badge(gtx, "Open", toneGreen)
				}
				return a.th.badge(gtx, "Hidden", toneNeutral)
			})
		})
	}
	items := []layout.Widget{func(gtx C) D { return a.th.button(gtx, &m.addCourse, "Add a course", btnOpt{full: true, icon: icAdd}) }, vspace(14)}
	if len(rows) == 0 {
		items = append(items, func(gtx C) D {
			return a.emptyState(gtx, icSchool, "No courses yet", "Add your first course above.", nil, "")
		})
	} else {
		items = append(items, func(gtx C) D { return a.group(gtx, rows) },
			vspace(10), func(gtx C) D {
				return a.th.muted(gtx, "Thumbnails can be uploaded from the website's admin dashboard.", 12.5)
			})
	}
	return items
}

func (a *App) editCourse(c api.AdminCourse) {
	f := &a.admin.more.course
	f.id, f.active = c.ID, c.IsActive
	f.title.Ed.SetText(c.Title)
	f.desc.Ed.SetText(c.Description)
	f.feats.Ed.SetText(c.Features)
	price := ""
	if c.ID != 0 {
		price = strconv.FormatFloat(c.Price, 'f', -1, 64)
	}
	f.price.Ed.SetText(price)
	f.days.Ed.SetText(strconv.Itoa(c.DefaultDays))
	f.classroom.Ed.SetText(c.ClassroomLink)
	a.admin.sub = moreCourseEdit
}

func (a *App) courseEditItems(gtx C) []layout.Widget {
	f := &a.admin.more.course
	if f.toggle.Clicked(gtx) {
		f.active = !f.active
	}
	if f.save.Clicked(gtx) {
		price, perr := strconv.ParseFloat(strings.TrimSpace(f.price.Ed.Text()), 64)
		days, derr := strconv.Atoi(f.days.Text())
		switch {
		case f.title.Text() == "":
			a.toast("Add a title.", true)
		case perr != nil || price < 0:
			a.toast("Enter the price as a number.", true)
		case derr != nil || days < 1:
			a.toast("Enter the access period in days.", true)
		default:
			in := api.CourseIn{Title: f.title.Text(), Description: f.desc.Ed.Text(), Features: f.feats.Ed.Text(), Price: price,
				DefaultDays: days, ClassroomLink: f.classroom.Text(), IsActive: f.active}
			id := f.id
			a.adminDo(func() error { return a.api.SaveCourse(id, in) }, "Course saved.", func() { a.admin.sub = moreCourses })
		}
	}
	if f.del.Clicked(gtx) {
		id, title := f.id, f.title.Text()
		a.confirm(fmt.Sprintf("Delete \"%s\"?", title), "The course and its updates are removed for good.", "Delete", true, func() {
			a.async(func() func() {
				err := a.api.DeleteCourse(id)
				return func() {
					if err == nil {
						a.toast("Course deleted.", false)
						a.admin.sub = moreCourses
						a.adminReload()
						return
					}
					if strings.Contains(err.Error(), "payment record") {
						a.confirm("Hide it instead?", err.Error()+" Hidden courses disappear from the website; paid students keep access.", "Hide course", false, func() {
							f.active = false
							f.save.Click()
						})
						return
					}
					a.toast(err.Error(), true)
				}
			})
		})
	}
	items := []layout.Widget{
		func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx C) D { return a.th.muted(gtx, "Visible on the website and app", 13.5) }),
				layout.Rigid(func(gtx C) D { return a.switchChip(gtx, &f.toggle, f.active, "Open", "Hidden") }),
			)
		},
	}
	for _, fl := range []*field{f.title, f.desc, f.feats, f.price, f.days, f.classroom} {
		items = append(items, vspace(12), func(gtx C) D { return a.th.field(gtx, fl) })
	}
	items = append(items, vspace(16), func(gtx C) D { return a.th.button(gtx, &f.save, "Save course", btnOpt{full: true, icon: icTick}) })
	if f.id != 0 {
		items = append(items, vspace(8), func(gtx C) D { return a.th.button(gtx, &f.del, "Delete course", btnOpt{full: true, kind: btnDanger}) })
	}
	return items
}

func (a *App) accountItems(gtx C) []layout.Widget {
	m := &a.admin.more
	if m.saveAccount.Clicked(gtx) {
		user, pw, conf, cur := m.user.Text(), m.newPw.Ed.Text(), m.confirmPw.Ed.Text(), m.currentPw.Ed.Text()
		switch {
		case len(user) < 3 || strings.ContainsAny(user, " \t"):
			a.toast("Username needs at least 3 characters and no spaces.", true)
		case pw != "" && len(pw) < 8:
			a.toast("New password must be at least 8 characters.", true)
		case pw != conf:
			a.toast("The new passwords do not match.", true)
		case cur == "":
			a.toast("Enter your current password to save.", true)
		default:
			a.adminDo(func() error { return a.api.SaveAdminAccount(cur, user, pw) }, "Admin login updated.", func() {
				for _, f := range []*field{m.newPw, m.confirmPw, m.currentPw} {
					f.Ed.SetText("")
				}
			})
		}
	}
	items := []layout.Widget{
		func(gtx C) D {
			return a.th.muted(gtx, "Other devices are logged out after a change. This phone stays logged in.", 13.5)
		},
	}
	for _, f := range []*field{m.user, m.newPw, m.confirmPw, m.currentPw} {
		items = append(items, vspace(12), func(gtx C) D { return a.th.field(gtx, f) })
	}
	items = append(items, vspace(16), func(gtx C) D {
		return a.th.button(gtx, &m.saveAccount, "Save login details", btnOpt{full: true, icon: icTick})
	},
		vspace(14), func(gtx C) D {
			return a.th.notice(gtx, icInfo, toneBlue, func(gtx C) D {
				return a.th.label(gtx, "Forgot the admin password? On the server run: .venv/bin/python -m app.reset_admin", textOpt{size: 12.5, color: a.th.P.Blue})
			})
		})
	return append(items, a.inviteCodeItems(gtx)...)
}
