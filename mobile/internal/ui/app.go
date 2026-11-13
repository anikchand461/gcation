// Package ui is the G-CATION app: the student side (home, courses, checkout, updates, profile)
// and the admin side (overview, students, payments, updates, settings).
package ui

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/x/explorer"
	qrcode "github.com/skip2/go-qrcode"

	"gcation/mobile/internal/api"
	"gcation/mobile/internal/platform"
)

//go:embed assets/logo.png
var logoPNG []byte

type screen int

const (
	scrSplash screen = iota
	scrAuth
	scrMain
)

type tab int

const (
	tabHome tab = iota
	tabCourses
	tabUpdates
	tabProfile
)

type imgEntry struct {
	op      paint.ImageOp
	ready   bool
	loading bool
}

// App holds all state. Everything is changed on the UI goroutine only: background work
// (network, saving files) hands its result back through post().
type App struct {
	win     *app.Window
	th      *Theme
	api     *api.Client
	expl    *explorer.Explorer
	dataDir string
	webURL  string
	results chan func()

	screen   screen
	tab      tab
	checkout *checkoutState // set while the checkout screen is open
	admin    *adminState    // set when logged in as the admin
	dialog   *dialogState   // confirm dialog on top of everything
	intro    introState     // opening animation

	history []navSnap // earlier places, for the back button
	lastNav navSnap
	navSet  bool

	me         *api.Me
	updates    *api.Updates
	knownUpd   map[int64]bool
	loadErr    string
	loading    bool
	lastPoll   time.Time
	polling    bool
	seenMarked bool

	pendingCopy string // clipboard text to write on the next frame (from background results)

	toastMsg   string
	toastErr   bool
	toastUntil time.Time

	imgs map[string]*imgEntry
	qrs  map[string]paint.ImageOp
	logo paint.ImageOp

	auth    authState
	home    homeState
	courses coursesState
	upd     updatesState
	prof    profileState

	nav      [4]widget.Clickable
	themeBtn widget.Clickable
	navItems []navItem
	bellBtn  widget.Clickable
	retryBtn widget.Clickable
	lists    map[string]*widget.List
	clicks   map[string]*widget.Clickable // buttons inside lists, keyed by purpose + id
}

type prefs struct {
	Dark bool `json:"dark"`
}

func New(w *app.Window, apiBase, webURL, dataDir string) *App {
	a := &App{
		win: w, api: api.New(apiBase, dataDir), expl: explorer.NewExplorer(w),
		dataDir: dataDir, webURL: webURL, results: make(chan func(), 256),
		imgs: map[string]*imgEntry{}, qrs: map[string]paint.ImageOp{}, lists: map[string]*widget.List{},
		clicks:   map[string]*widget.Clickable{},
		knownUpd: map[int64]bool{},
	}
	var p prefs
	if b, err := os.ReadFile(filepath.Join(dataDir, "prefs.json")); err == nil {
		_ = json.Unmarshal(b, &p)
	}
	a.th = NewTheme(p.Dark)
	if img, err := png.Decode(bytes.NewReader(logoPNG)); err == nil {
		a.logo = paint.NewImageOp(img)
	}
	a.auth.init()
	a.prof.init()
	a.intro.start = time.Now()
	if a.api.AdminLoggedIn() {
		a.startAdmin()
	} else if a.api.LoggedIn() {
		a.screen = scrSplash
		a.reload()
	} else {
		a.screen = scrAuth
	}
	return a
}

// StatusBarColor is the colour for Android's status and navigation bars.
func (a *App) StatusBarColor() (top, bottom [4]uint8) {
	c := a.th.P.Bg
	n := a.th.P.Nav
	if a.screen != scrMain || a.checkout != nil || (a.admin != nil && (a.admin.detail != nil || a.admin.sub != moreNone)) {
		n = c
	}
	if a.screen == scrAuth {
		c = heroA // the login header is dark
	}
	if !a.intro.start.IsZero() || a.screen == scrSplash {
		c, n = heroA, heroB
	}
	return [4]uint8{c.R, c.G, c.B, c.A}, [4]uint8{n.R, n.G, n.B, n.A}
}

func (a *App) Dark() bool { return a.th.Dark }

func (a *App) Explorer() *explorer.Explorer { return a.expl }

// ---------- Background work ----------

func (a *App) post(f func()) {
	a.results <- f
	if a.win != nil { // nil only in the screenshot test
		a.win.Invalidate()
	}
}

func (a *App) async(work func() func()) {
	go func() { a.post(work()) }()
}

func (a *App) drain() {
	for {
		select {
		case f := <-a.results:
			if f != nil {
				f()
			}
		default:
			return
		}
	}
}

// reload fetches the student's data (and updates, once a course is active).
func (a *App) reload() {
	if a.loading {
		return
	}
	a.loading = true
	a.async(func() func() {
		me, err := a.api.Me()
		var upd *api.Updates
		if err == nil && hasActive(me) {
			upd, _ = a.api.Updates()
		}
		return func() {
			a.loading = false
			if errors.Is(err, api.ErrUnauthorized) {
				a.signedOut("")
				return
			}
			if err != nil {
				a.loadErr = err.Error()
				if a.me == nil && a.screen == scrSplash {
					a.screen = scrMain // shows the error with a retry button
				}
				return
			}
			a.loadErr = ""
			a.me = me
			a.updates = upd
			if upd != nil {
				for _, u := range upd.Items {
					a.knownUpd[u.ID] = true
				}
			}
			a.lastPoll = time.Now()
			if a.screen != scrMain {
				a.screen = scrMain
			}
		}
	})
}

// pollUpdates checks for new updates every minute while the app is open.
func (a *App) pollUpdates() {
	if a.polling || a.me == nil || !hasActive(a.me) {
		return
	}
	a.polling = true
	a.async(func() func() {
		upd, err := a.api.Updates()
		return func() {
			a.polling = false
			a.lastPoll = time.Now()
			if err != nil {
				return
			}
			var fresh []api.Update
			for _, u := range upd.Items {
				if !a.knownUpd[u.ID] {
					fresh = append(fresh, u)
					a.knownUpd[u.ID] = true
				}
			}
			a.updates = upd
			if len(fresh) > 0 {
				a.me.Unread += len(fresh)
				a.seenMarked = false
				a.toast("New update: "+fresh[0].Title, false)
			}
		}
	})
}

func (a *App) signedOut(msg string) {
	a.me, a.updates, a.checkout, a.admin, a.dialog = nil, nil, nil, nil, nil
	a.tab = tabHome
	a.screen = scrAuth
	a.knownUpd = map[int64]bool{}
	if msg != "" {
		a.toast(msg, false)
	}
}

func (a *App) toggleTheme() {
	a.th.SetDark(!a.th.Dark)
	// The tap is handled part-way through a frame, so draw the whole screen again right away.
	if a.win != nil {
		a.win.Invalidate()
	}
	b, _ := json.Marshal(prefs{Dark: a.th.Dark})
	_ = os.MkdirAll(a.dataDir, 0o700)
	_ = os.WriteFile(filepath.Join(a.dataDir, "prefs.json"), b, 0o600)
}

func (a *App) toast(msg string, isErr bool) {
	a.toastMsg, a.toastErr = msg, isErr
	dur := 3200 * time.Millisecond
	if isErr {
		dur = 5 * time.Second
	}
	a.toastUntil = time.Now().Add(dur)
	if a.win != nil {
		a.win.Invalidate()
	}
}

func (a *App) open(url string) {
	go func() {
		if err := platform.Open(url); err != nil {
			a.post(func() { a.toast("No app found to open this link.", true) })
		}
	}()
}

func (a *App) copyText(gtx C, s, label string) {
	gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(s))})
	a.toast(label, false)
}

// copyTextNow copies text from code that has no frame context (e.g. after a network call).
func (a *App) copyTextNow(s, label string) {
	a.pendingCopy = s
	a.toast(label, false)
}

// saveReceipt downloads a student's receipt and asks where to save it.
func (a *App) saveReceipt(r api.Receipt) {
	a.saveFile("G-CATION-receipt-"+r.ReceiptNo+".pdf", func() ([]byte, error) { return a.api.Receipt(r.ID) })
}

// saveFile downloads a file, then shows Android's "Save as" screen to choose where it goes.
func (a *App) saveFile(name string, fetch func() ([]byte, error)) {
	a.toast("Downloading…", false)
	go func() {
		data, err := fetch()
		if err != nil {
			a.post(func() { a.toast(err.Error(), true) })
			return
		}
		w, err := a.expl.CreateFile(name)
		if err != nil {
			if !errors.Is(err, explorer.ErrUserDecline) {
				a.post(func() { a.toast("Could not save the file: "+err.Error(), true) })
			}
			return
		}
		_, err = w.Write(data)
		if cerr := w.Close(); err == nil {
			err = cerr
		}
		a.post(func() {
			if err != nil {
				a.toast("Could not save the file: "+err.Error(), true)
			} else {
				a.toast("Saved.", false)
			}
		})
	}()
}

// ---------- Images ----------

// image returns a cached remote image, loading it in the background on first use.
func (a *App) image(path string) (paint.ImageOp, bool) {
	if path == "" {
		return paint.ImageOp{}, false
	}
	e := a.imgs[path]
	if e == nil {
		e = &imgEntry{loading: true}
		a.imgs[path] = e
		go func() {
			img, err := a.api.Image(path)
			a.post(func() {
				e.loading = false
				if err == nil {
					e.op, e.ready = paint.NewImageOp(img), true
				}
			})
		}()
	}
	return e.op, e.ready
}

// qr renders a QR code for content locally (used for the UPI QR when no image is uploaded).
func (a *App) qr(content string) paint.ImageOp {
	if op, ok := a.qrs[content]; ok {
		return op
	}
	var img image.Image = image.NewGray(image.Rect(0, 0, 1, 1))
	if q, err := qrcode.New(content, qrcode.Medium); err == nil {
		img = q.Image(640)
	}
	op := paint.NewImageOp(img)
	a.qrs[content] = op
	return op
}

// clk returns a stable Clickable for a button that is created per item (e.g. per course).
func (a *App) clk(key string) *widget.Clickable {
	c := a.clicks[key]
	if c == nil {
		c = new(widget.Clickable)
		a.clicks[key] = c
	}
	return c
}

func (a *App) list(name string) *widget.List {
	l := a.lists[name]
	if l == nil {
		l = &widget.List{List: layout.List{Axis: layout.Vertical}}
		a.lists[name] = l
	}
	return l
}

// ---------- Frame ----------

// Layout draws one frame. gtx must come from app.NewContext, which already keeps the page clear of
// the status bar, navigation bar and on-screen keyboard.
func (a *App) Layout(gtx C) D {
	a.drain()
	if a.pendingCopy != "" {
		gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(a.pendingCopy))})
		a.pendingCopy = ""
	}
	a.trackNav()
	a.handleBack(gtx)

	paint.Fill(gtx.Ops, a.th.P.Bg)
	if a.screen == scrMain && a.admin == nil && a.me != nil {
		if time.Since(a.lastPoll) > time.Minute {
			a.pollUpdates()
		}
		gtx.Execute(op.InvalidateCmd{At: a.lastPoll.Add(time.Minute + time.Second)})
	}

	switch a.screen {
	case scrSplash:
		a.layoutSplash(gtx)
	case scrAuth:
		a.layoutAuth(gtx)
	default:
		switch {
		case a.admin != nil:
			a.layoutAdmin(gtx)
		case a.checkout != nil:
			a.layoutCheckout(gtx)
		default:
			a.layoutMain(gtx)
		}
	}
	a.layoutDialog(gtx)
	a.layoutToast(gtx)
	a.layoutIntro(gtx)
	return D{Size: gtx.Constraints.Max}
}

// navSnap is where the user is in the app: tab, open sub-screen and checkout step.
type navSnap struct {
	admin    bool
	tab      tab
	checkout *checkoutState
	step     int
	atab     adminTab
	sub      moreScreen
	detail   *studentDetail
}

func (a *App) snap() navSnap {
	n := navSnap{admin: a.admin != nil}
	if s := a.admin; s != nil {
		n.atab, n.sub, n.detail = s.tab, s.sub, s.detail
		return n
	}
	n.tab, n.checkout = a.tab, a.checkout
	if a.checkout != nil {
		n.step = a.checkout.step
	}
	return n
}

// trackNav records each move between screens, so the back button can retrace them.
// It runs at the start of every frame and compares with where the user was last frame.
func (a *App) trackNav() {
	cur := a.snap()
	if a.screen != scrMain || !a.navSet || cur.admin != a.lastNav.admin {
		a.history, a.lastNav, a.navSet = nil, cur, a.screen == scrMain
		return
	}
	prev := a.lastNav
	if cur == prev {
		return
	}
	a.lastNav = cur
	closed := prev.checkout != nil && prev.checkout != cur.checkout || prev.detail != nil && prev.detail != cur.detail ||
		cur.step == 3 && prev.step != 3 // details were sent: don't step back into the form
	if closed {
		// A screen that was closed is not reopened by back.
		kept := a.history[:0]
		for _, h := range a.history {
			if !(prev.checkout != nil && h.checkout == prev.checkout || prev.detail != nil && h.detail == prev.detail) {
				kept = append(kept, h)
			}
		}
		a.history = kept
	}
	if n := len(a.history); n > 0 && a.history[n-1] == cur {
		a.history = a.history[:n-1] // went back with an on-screen back arrow
		return
	}
	if !closed {
		a.history = append(a.history, prev)
		if len(a.history) > 40 {
			a.history = a.history[1:]
		}
	}
}

// goBack returns to the previous place. It reports false when there is nowhere to go back to.
func (a *App) goBack() bool {
	n := len(a.history)
	if n == 0 {
		// Nothing recorded: fall back to the first tab.
		switch {
		case a.checkout != nil:
			a.checkout = nil
		case a.admin != nil && a.admin.canGoBack():
			a.admin.back()
		case a.admin == nil && a.tab != tabHome:
			a.tab = tabHome
		default:
			return false
		}
		a.lastNav = a.snap()
		return true
	}
	h := a.history[n-1]
	a.history = a.history[:n-1]
	if s := a.admin; s != nil {
		s.tab, s.sub, s.detail = h.atab, h.sub, h.detail
	} else {
		if h.tab != a.tab {
			a.reload()
		}
		a.tab, a.checkout = h.tab, h.checkout
		if h.checkout != nil {
			h.checkout.step = h.step
		}
	}
	a.lastNav = h
	return true
}

// handleBack: the phone's back button closes dialogs, then retraces the user's steps through
// tabs and screens; with nowhere left to go it sends the app to the background, as Android apps do.
func (a *App) handleBack(gtx C) {
	for {
		ev, ok := gtx.Event(key.Filter{Name: key.NameBack})
		if !ok {
			return
		}
		e, ok := ev.(key.Event)
		if !ok || e.State != key.Press {
			continue
		}
		switch {
		case a.dialog != nil:
			a.dialog = nil
		case a.screen == scrAuth && (a.auth.register || a.auth.admin):
			a.auth.register, a.auth.admin, a.auth.err = false, false, ""
		case a.screen == scrMain && a.goBack():
		default:
			platform.MoveToBack()
		}
		if a.win != nil {
			a.win.Invalidate()
		}
	}
}

// safeArea adds a little breathing room under the status bar (the bars themselves are
// already excluded by app.NewContext).
func (a *App) safeArea(gtx C, w layout.Widget) D {
	return layout.Inset{Top: 4}.Layout(gtx, w)
}

func (a *App) wordmark(gtx C, size unit.Sp) D {
	return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return a.th.label(gtx, "G", textOpt{size: size, weight: font.Black, color: a.th.P.Violet})
		}),
		layout.Rigid(func(gtx C) D {
			return a.th.label(gtx, "-CATION", textOpt{size: size, weight: font.Black, color: a.th.P.Ink})
		}),
	)
}

// layoutMain is the tabbed shell: top bar, scrolling page, bottom tab bar.
func (a *App) layoutMain(gtx C) D {
	for i := range a.nav {
		if a.nav[i].Clicked(gtx) {
			if tab(i) == a.tab {
				a.list(tabName(a.tab)).Position.First, a.list(tabName(a.tab)).Position.Offset = 0, 0
			}
			a.tab = tab(i)
			if a.tab != tabUpdates {
				a.seenMarked = false
			}
			a.reload()
		}
	}
	if a.themeBtn.Clicked(gtx) {
		a.toggleTheme()
	}
	if a.bellBtn.Clicked(gtx) {
		a.tab = tabUpdates
	}
	if a.retryBtn.Clicked(gtx) {
		a.reload()
	}
	return a.safeArea(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				ic, n := icMoon, 0
				if a.th.Dark {
					ic = icSun
				}
				if a.me != nil {
					n = a.me.Unread
				}
				title := [...]string{"", "Courses", "Updates", "Profile"}[a.tab]
				return a.appBar(gtx, title, a.tab == tabHome, a.barIcon(&a.themeBtn, ic, 0), a.barIcon(&a.bellBtn, icBell, n))
			}),
			layout.Flexed(1, func(gtx C) D {
				if a.me == nil {
					return a.loadingOrError(gtx)
				}
				switch a.tab {
				case tabCourses:
					return a.layoutCourses(gtx)
				case tabUpdates:
					return a.layoutUpdates(gtx)
				case tabProfile:
					return a.layoutProfile(gtx)
				}
				return a.layoutHome(gtx)
			}),
			layout.Rigid(func(gtx C) D {
				unread := a.me != nil && a.me.Unread > 0
				items := []navItem{{icHome, "Home", false}, {icSchool, "Courses", false}, {icMegaphone, "Updates", unread}, {icPerson, "Profile", false}}
				return a.navBar(gtx, items, int(a.tab), a.nav[:])
			}),
		)
	})
}

func tabName(t tab) string { return [...]string{"home", "courses", "updates", "profile"}[t] }

func (a *App) loadingOrError(gtx C) D {
	return layout.Center.Layout(gtx, func(gtx C) D {
		return pad(28).Layout(gtx, func(gtx C) D {
			if a.loadErr == "" {
				return a.th.muted(gtx, "Loading…", 15)
			}
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return iconSized(gtx, icAlert, 40, a.th.P.Violet) }),
				layout.Rigid(vspace(12)),
				layout.Rigid(func(gtx C) D { return centered(gtx, a.loadErr, a.th, textOpt{size: 15, color: a.th.P.Ink2}) }),
				layout.Rigid(vspace(16)),
				layout.Rigid(func(gtx C) D { return a.th.button(gtx, &a.retryBtn, "Try again", btnOpt{icon: icRefresh}) }),
			)
		})
	})
}

// page lays out a scrolling page with side padding.
func (a *App) page(gtx C, name string, items []layout.Widget) D {
	// Plain list without a scrollbar, as is usual on phones.
	return a.list(name).List.Layout(gtx, len(items)+1, func(gtx C, i int) D {
		if i == len(items) {
			return layout.Spacer{Height: 28}.Layout(gtx)
		}
		return layout.Inset{Left: 16, Right: 16}.Layout(gtx, items[i])
	})
}

func (a *App) layoutToast(gtx C) D {
	if a.toastMsg == "" || time.Now().After(a.toastUntil) {
		a.toastMsg = ""
		return D{}
	}
	gtx.Execute(op.InvalidateCmd{At: a.toastUntil})
	bottom := unit.Dp(90) // above the tab bar
	if a.screen != scrMain || a.checkout != nil {
		bottom = 24
	}
	return layout.S.Layout(gtx, func(gtx C) D {
		return layout.Inset{Left: 20, Right: 20, Bottom: bottom}.Layout(gtx, func(gtx C) D {
			bg := rgb(0x1f0d3a)
			if a.toastErr {
				bg = rgb(0x7a1a14)
			}
			return tinted(gtx, bg, 14, 0, func(gtx C) D {
				return layout.Inset{Left: 16, Right: 16, Top: 12, Bottom: 12}.Layout(gtx, func(gtx C) D {
					ic := icCheck
					if a.toastErr {
						ic = icAlert
					}
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx C) D { return iconSized(gtx, ic, 18, rgb(0xc4b5fd)) }),
						layout.Rigid(hspace(10)),
						layout.Flexed(1, func(gtx C) D {
							return a.th.label(gtx, a.toastMsg, textOpt{size: 14, color: white})
						}),
					)
				})
			})
		})
	})
}

func hasActive(me *api.Me) bool {
	if me == nil {
		return false
	}
	for _, c := range me.Courses {
		if c.Access.Status == "active" {
			return true
		}
	}
	return false
}
