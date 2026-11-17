package ui

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gioui.org/gpu/headless"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"gcation/mobile/internal/api"
)

// TestScreenshots renders every screen with sample data, in light and dark, at phone size.
// Run with: SHOTS_DIR=/some/dir go test ./internal/ui -run TestScreenshots
func TestScreenshots(t *testing.T) {
	dir := os.Getenv("SHOTS_DIR")
	if dir == "" {
		t.Skip("set SHOTS_DIR to write screenshots")
	}
	const pxPerDp = 2
	size := image.Pt(412*pxPerDp, 915*pxPerDp)
	win, err := headless.NewWindow(size.X, size.Y)
	if err != nil {
		t.Skip("no headless GPU:", err)
	}
	defer win.Release()

	for _, dark := range []bool{false, true} {
		a := testApp(t, dark)
		shots := []struct {
			name  string
			setup func()
		}{
			{"01-login", func() { a.screen = scrAuth }},
			{"01b-login-management", func() { a.auth.mgmt = true }},
			{"02b-register-management", func() { a.auth.register = true }},
			{"02-register", func() { a.screen, a.auth.register, a.auth.mgmt = scrAuth, true, false }},
			{"02c-register-filled", func() {
				a.auth.rName.Ed.SetText("Anik Chand")
				a.auth.rPhone.Ed.SetText("98765 43210")
			}},
			{"02e-google-wait", func() {
				a.auth.register, a.auth.googleWait = false, true
				a.auth.list.Position.First, a.auth.list.Position.Offset = 1, 250*pxPerDp
			}},
			{"02g-google-details", func() {
				a.auth.googleWait, a.auth.needDetails, a.auth.googleName = false, true, "Anik Chand"
				a.auth.list.Position.First, a.auth.list.Position.Offset = 1, 450*pxPerDp
			}},
			{"02f-login", func() {
				a.auth.needDetails = false
				a.auth.googleWait = false
				a.auth.list.Position.First, a.auth.list.Position.Offset = 0, 0
			}},
			{"03-home", func() { a.screen, a.tab = scrMain, tabHome }},
			{"03b-home-unpaid", func() { a.me.Student.StudentCode = nil }},
			{"04-courses", func() { a.tab = tabCourses }},
			{"05-updates", func() { a.tab = tabUpdates }},
			{"06-profile", func() { a.tab = tabProfile }},
			{"06b-profile-mobile", func() {
				l := a.list("profile")
				l.Position.First, l.Position.Offset = 8, 0
			}},
			{"07-checkout-pay", func() { a.openCheckout(a.me.Courses[1]) }},
			{"08-checkout-details", func() { a.checkout.step = 2 }},
			{"08b-checkout-paid-using", func() { a.checkout.method, a.checkout.methodOpen = 1, true }},
			{"08c-checkout-screenshot", func() {
				c := a.checkout
				c.methodOpen = false
				c.ref.Ed.SetText("412345678901")
				c.referralID.Ed.SetText("GCM001")
				c.shot, c.shotPreview = []byte{1}, testShot()
				l := a.list("checkout2")
				l.Position.First, l.Position.Offset = 5, 1250*pxPerDp
			}},
			{"08d-checkout-cash", func() { a.checkout.step, a.checkout.cash, a.checkout.method = 1, true, cashMethod }},
			{"08e-checkout-cash-details", func() { a.checkout.step = 2 }},
			{"09-checkout-call", func() { a.checkout.step, a.checkout.cash = 3, false }},
			{"10-checkout-status", func() { a.openCheckout(a.me.Courses[2]) }},
			{"11-splash", func() { a.checkout, a.screen = nil, scrSplash }},
			{"12-admin-login", func() { a.screen, a.auth.admin, a.auth.register = scrAuth, true, false }},
			{"13-admin-overview", func() { a.auth.admin = false; a.screen, a.admin = scrMain, testAdmin(a) }},
			{"14-admin-students", func() { a.admin.tab = aStudents }},
			{"15-admin-detail", func() { a.openStudent(1, 0); a.admin.detail.data = testDetail(); a.admin.detail.pickCourse(1) }},
			{"16-admin-payments", func() { a.admin.detail, a.admin.tab = nil, aPayments }},
			{"17-admin-updates", func() { a.admin.tab = aUpdates }},
			{"17b-admin-updates-audience", func() {
				a.admin.compose.audOpen, a.admin.compose.toTeam, a.admin.compose.toAll = true, true, false
				a.admin.compose.toCourse[1] = true
				l := a.list("a.updates")
				l.Position.First, l.Position.Offset = 2, 520*pxPerDp
			}},
			{"18-admin-more", func() { a.admin.tab = aMore }},
			{"19-admin-qr", func() { a.admin.sub = moreQR; a.admin.more.fill(a.admin) }},
			{"20-admin-course", func() { a.editCourse(a.admin.courses[0]) }},
			{"21-dialog", func() {
				a.admin.sub = moreNone
				a.confirm("Give access?", "Anik Chand gets \"Interview Skills Bootcamp\" for 90 days. Amount recorded: ₹1,499.", "Give access", false, func() {})
			}},
			{"22-admin-management", func() { a.dialog = nil; a.admin.sub = moreManagement }},
			{"22b-admin-management-password", func() {
				a.admin.managers[0].CanPostUpdates = true
				a.admin.more.pwFor = 1
				a.admin.more.managerPw.Ed.SetText("x7Kp2mQr9a")
			}},
			{"23-admin-account", func() { a.admin.sub = moreAccount; a.admin.more.fill(a.admin) }},
			{"24-admin-screenshot", func() {
				a.admin.sub, a.admin.tab = moreNone, aPayments
				a.admin.claims[0].Age = &[]int{19}[0]
				a.admin.claims[0].PaidOn, a.admin.claims[0].PayerName = "2026-11-16", "Priya Sen"
				c := a.admin.claims[0]
				a.imgs[c.ScreenshotPath()] = &imgEntry{op: testShot(), ready: true}
				a.admin.shot = &c
			}},
			{"25-mgmt-home", func() { a.admin = nil; a.mgmt = testManagement(a) }},
			{"26-mgmt-courses", func() { a.mgmt.tab = mgmtCourses }},
			{"26b-mgmt-updates", func() { a.mgmt.tab = mgmtUpdates }},
			{"27-mgmt-profile", func() { a.mgmt.tab = mgmtProfile }},
			{"28-mgmt-post", func() {
				a.mgmt.me.Manager.CanPostUpdates = true
				a.mgmt.compose.courses = []api.UpdateCourse{{ID: 1, Title: "Online Spoken English Batch"}}
				a.mgmt.compose.audience = map[string]int{"all": 8, "management": 3, "1": 7}
				a.mgmt.tab = mgmtPost
			}},
		}
		for _, s := range shots {
			s.setup()
			var ops op.Ops
			for i := 0; i < 2; i++ { // second frame picks up anything decided in the first
				ops.Reset()
				gtx := layout.Context{Ops: &ops, Now: time.Now(), Metric: unit.Metric{PxPerDp: pxPerDp, PxPerSp: pxPerDp}, Constraints: layout.Exact(size)}
				a.Layout(gtx)
			}
			if err := win.Frame(&ops); err != nil {
				t.Fatal(err)
			}
			img := image.NewRGBA(image.Rectangle{Max: size})
			if err := win.Screenshot(img); err != nil {
				t.Fatal(err)
			}
			theme := "light"
			if dark {
				theme = "dark"
			}
			f, _ := os.Create(filepath.Join(dir, theme+"-"+s.name+".png"))
			png.Encode(f, img)
			f.Close()
		}
	}
}

func testApp(t *testing.T, dark bool) *App {
	a := &App{
		th: NewTheme(dark), api: api.New("http://127.0.0.1:1", t.TempDir()), results: make(chan func(), 256),
		imgs: map[string]*imgEntry{}, qrs: map[string]paint.ImageOp{}, lists: map[string]*widget.List{},
		clicks: map[string]*widget.Clickable{}, knownUpd: map[int64]bool{}, webURL: "https://gcation.vercel.app",
		lastPoll: time.Now(),
	}
	a.auth.init()
	a.prof.init()
	if img, err := png.Decode(bytesReader(logoPNG)); err == nil {
		a.logo = paint.NewImageOp(img)
	}
	now := time.Now().Unix()
	amt := 1499.0
	course := "Online Spoken English Batch"
	a.me = &api.Me{
		Student: api.Student{ID: 4, Name: "Anik Chand", Email: "anikchand461@gmail.com", Phone: "9153772355", CreatedAt: now - 30*86400, StudentCode: ptr("GCS003")},
		Courses: []api.Course{
			{ID: 1, Title: course, Description: "A one-year online learning package with live classes, practice and communication skill development.",
				Price: 3099, DefaultDays: 365, IsOpen: true, Access: api.Access{Status: "active", Since: now - 20*86400, Until: now + 345*86400, DaysLeft: 345},
				ClassroomLink: "https://meet.google.com/abc"},
			{ID: 2, Title: "Interview Skills Bootcamp", Description: "Mock interviews and personal feedback over 3 months.",
				Features: []string{"Weekly mock interviews", "Personal feedback", "Certificate on completion"},
				Price:    1499, DefaultDays: 90, IsOpen: true, Access: api.Access{Status: "none"}},
			{ID: 3, Title: "Public Speaking Masterclass", Description: "Speak on stage with confidence.",
				Price: 2499, DefaultDays: 180, IsOpen: true, Access: api.Access{Status: "none"},
				PendingClaim: &api.Claim{QRSlot: "secondary", PayerName: "Anik Chand", Method: "PhonePe", Amount: &amt, PaidOn: "2026-11-13", Reference: "412345678901",
					ReferralID: "GCM001", HasScreenshot: true, CreatedAt: now - 3600}},
		},
		Payment: api.Payment{ContactPhone: "+91 9091455863", Methods: []api.Method{
			{Slot: "primary", Label: "Primary QR", UPIID: "gcation@okaxis", PayeeName: "G-CATION"},
			{Slot: "secondary", Label: "Backup QR (PhonePe)", UPIID: "gcation.backup@ybl", PayeeName: "G-CATION"},
		}},
		Unread:   1,
		Receipts: []api.Receipt{{ID: 1, ReceiptNo: "GC-2026-00001", CourseID: 1, CourseTitle: course, AmountPaid: 3099, GrantedAt: now - 20*86400}},
	}
	title := course
	a.updates = &api.Updates{SeenAt: now - 7200, Items: []api.Update{
		{ID: 2, Title: "Live class: Friday, 7:00 PM", Message: "Please join 5 minutes early. Keep your mic muted until you are asked to speak.",
			LinkURL: "https://meet.google.com/abc-defg-hij", LinkLabel: "Join Google Meet", CreatedAt: now - 600, CourseTitle: &title},
		{ID: 1, Title: "Join our student community", Message: "Class updates, practice material and doubts.",
			LinkURL: "https://chat.whatsapp.com/xyz", LinkLabel: "Join community", CreatedAt: now - 86400},
	}}
	return a
}

func testAdmin(a *App) *adminState {
	s := &adminState{search: newField("", "Search name, email, phone or ID", false, 0), loaded: true,
		paySearch: newField("", "Search student, course or receipt", false, 0), teamSearch: newField("", "Search name or Management ID", false, 0)}
	s.compose.init(api.AdminUpdates)
	s.more.init()
	now := time.Now().Unix()
	amt := 1499.0
	s.ov = &api.Overview{Students: 12, ActiveStudents: 8, PendingClaims: 2, ExpiringSoon: 1, TotalCollected: 35990, CollectedThisMonth: 7598}
	s.claims = []api.ClaimRow{
		{ID: 1, StudentID: 2, CourseTitle: "Interview Skills Bootcamp", QRSlot: "secondary", Method: "PhonePe", Reference: "412345678901", Amount: &amt, CreatedAt: now - 1800, StudentName: "Priya Sen", StudentPhone: "9000000001",
			ReferralID: "GCM001", ReferralName: ptr("Rahul Mitra"), HasScreenshot: true},
		{ID: 2, StudentID: 3, CourseTitle: "Online Spoken English Batch", QRSlot: "primary", Method: "GPay", CreatedAt: now - 7200, StudentName: "Rahul Das"},
	}
	row := func(id int64, name, status string, days int, pending int) api.StudentRow {
		r := api.StudentRow{Student: api.Student{ID: id, Name: name, Email: strings.ToLower(strings.Fields(name)[0]) + "@example.com", Phone: "9153772355", CreatedAt: now - 9*86400}, PendingClaims: pending}
		if status != "" {
			r.Access = append(r.Access, struct {
				CourseID int64  `json:"course_id"`
				Title    string `json:"title"`
				api.Access
			}{1, "Online Spoken English Batch", api.Access{Status: status, Until: now + int64(days)*86400, DaysLeft: days}})
		}
		return r
	}
	s.students = []api.StudentRow{row(1, "Anik Chand", "active", 345, 0), row(2, "Priya Sen", "", 0, 1), row(4, "Meera Roy", "active", 5, 0), row(5, "Arjun Paul", "expired", 0, 0), row(6, "Sara Khan", "", 0, 0)}
	s.payments = []api.Enrollment{{ID: 1, StudentID: 1, CourseTitle: "Online Spoken English Batch", StartAt: now - 20*86400, EndAt: now + 345*86400, AmountPaid: 3099, GrantedAt: now - 20*86400, ReceiptNo: "GC-2026-00001", StudentName: "Anik Chand"}}
	cid := int64(1)
	title := "Online Spoken English Batch"
	s.compose.anns = []api.Announcement{{ID: 1, Title: "Live class today, 7:00 PM", Message: "Please join 5 minutes early.", LinkURL: "https://meet.google.com/abc-defg-hij", ToManagement: true, CourseIDs: []int64{cid}, AudienceLabel: "Management · " + title, CreatedAt: now - 600}}
	s.compose.audience = map[string]int{"all": 8, "management": 3, "1": 7, "2": 1}
	s.compose.courses = []api.UpdateCourse{{ID: 1, Title: title}, {ID: 2, Title: "Interview Skills Bootcamp"}}
	s.courses = []api.AdminCourse{
		{ID: 1, Title: title, Description: "A one-year online learning package.", Features: "Live online classes\nWeekly practice", Price: 3099, DefaultDays: 365, IsActive: true},
		{ID: 2, Title: "Interview Skills Bootcamp", Price: 1499, DefaultDays: 90, IsActive: false},
	}
	s.methods = []api.AdminMethod{
		{Method: api.Method{Slot: "primary", Label: "Primary QR", UPIID: "gcation@okaxis", PayeeName: "G-CATION"}, IsActive: true},
		{Method: api.Method{Slot: "secondary", Label: "Backup QR (PhonePe)", UPIID: "gcation.backup@ybl", PayeeName: "G-CATION"}, IsActive: true},
	}
	s.phone, s.username = "+91 9091455863", "admin"
	s.students[0].StudentCode = ptr("GCS001")
	s.invite = "GCTEAM2026"
	revoked := now - 86400
	s.managers = []api.Manager{
		{ID: 1, Code: "GCM001", Name: "Rahul Mitra", Email: "rahul@example.com", Phone: "9000000002", CreatedAt: now - 20*86400, Approved: 4, Pending: 1},
		{ID: 2, Code: "GCM002", Name: "Sneha Pal", Email: "sneha@example.com", CreatedAt: now - 9*86400, RevokedAt: &revoked},
	}
	return s
}

func testManagement(a *App) *mgmtState {
	a.startManagementState()
	m := a.mgmt
	m.me = &api.ManagerMe{
		Manager: api.Manager{ID: 1, Code: "GCM001", Name: "Rahul Mitra", Email: "rahul@example.com", Phone: "9000000002", CreatedAt: time.Now().Unix() - 20*86400},
		Courses: []api.Course{a.me.Courses[0], a.me.Courses[1]},
		Unread:  1,
	}
	now := time.Now().Unix()
	team := "Management team"
	m.updates = &api.Updates{SeenAt: now - 7200, Items: []api.Update{
		{ID: 9, Title: "Team meeting on Sunday, 11 AM", Message: "We will plan the new batch. Please bring your referral list.",
			LinkURL: "https://meet.google.com/xyz-abcd-efg", LinkLabel: "Join Google Meet", CreatedAt: now - 900, CourseTitle: &team},
		{ID: 8, Title: "New brochure is ready", Message: "Share it with students who ask about prices.", CreatedAt: now - 3*86400, CourseTitle: &team},
	}}
	return m
}

func ptr(s string) *string { return &s }

// testShot is a stand-in payment screenshot: a phone-shaped green image.
func testShot() paint.ImageOp {
	img := image.NewRGBA(image.Rect(0, 0, 90, 180))
	for i := range img.Pix {
		img.Pix[i] = []byte{16, 185, 129, 255}[i%4]
	}
	return paint.NewImageOp(img)
}

func testDetail() *api.StudentDetail {
	now := time.Now().Unix()
	amt := 1499.0
	d := &api.StudentDetail{Student: api.Student{ID: 2, Name: "Priya Sen", Email: "priya@example.com", Phone: "9000000001", CreatedAt: now - 3*86400}}
	d.Courses = append(d.Courses,
		struct {
			ID          int64      `json:"id"`
			Title       string     `json:"title"`
			Price       float64    `json:"price"`
			DefaultDays int        `json:"default_days"`
			Access      api.Access `json:"access"`
		}{1, "Online Spoken English Batch", 3099, 365, api.Access{Status: "none"}},
		struct {
			ID          int64      `json:"id"`
			Title       string     `json:"title"`
			Price       float64    `json:"price"`
			DefaultDays int        `json:"default_days"`
			Access      api.Access `json:"access"`
		}{2, "Interview Skills Bootcamp", 1499, 90, api.Access{Status: "none"}})
	d.Claims = []api.ClaimRow{{ID: 1, StudentID: 2, CourseID: 2, CourseTitle: "Interview Skills Bootcamp", QRSlot: "secondary", Method: "PhonePe", Reference: "412345678901", Amount: &amt, Status: "pending", CreatedAt: now - 1800,
		ReferredBy: "Rahul", ReferralID: "GCM001", ReferralName: ptr("Rahul Mitra"), HasScreenshot: true}}
	return d
}

// TestBackHistory checks that the back button retraces tabs and screens in order.
func TestBackHistory(t *testing.T) {
	a := testApp(t, false)
	a.screen = scrMain
	a.trackNav()
	for _, tb := range []tab{tabCourses, tabProfile, tabUpdates} {
		a.tab = tb
		a.trackNav()
	}
	a.checkout = &checkoutState{courseID: 2, step: 1}
	a.trackNav()
	a.checkout.step = 2
	a.trackNav()

	want := []navSnap{
		{tab: tabUpdates, checkout: a.checkout, step: 1},
		{tab: tabUpdates},
		{tab: tabProfile},
		{tab: tabCourses},
		{tab: tabHome},
	}
	for i, w := range want {
		if !a.goBack() {
			t.Fatalf("back %d: nowhere to go", i)
		}
		if got := a.snap(); got != w {
			t.Fatalf("back %d: at %+v, want %+v", i, got, w)
		}
	}
	if a.goBack() {
		t.Fatal("back on the home tab should leave the app")
	}

	// Closing the checkout with its own arrow does not leave it in the history.
	a.tab = tabCourses
	a.trackNav()
	a.checkout = &checkoutState{courseID: 2, step: 1}
	a.trackNav()
	a.checkout = nil
	a.trackNav()
	if !a.goBack() || a.snap() != (navSnap{tab: tabHome}) || len(a.history) != 0 {
		t.Fatalf("after closing checkout: at %+v, history %d", a.snap(), len(a.history))
	}
}
