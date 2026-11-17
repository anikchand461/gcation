package ui

import (
	"crypto/rand"
	"encoding/base64"
	"image"
	"regexp"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"

	"gcation/mobile/internal/api"
)

type authState struct {
	admin    bool // Student / Management / Admin switch: admin
	mgmt     bool // ... management team
	register bool // student or management: create account instead of log in

	roleStudent, roleMgmt, roleAdmin, switchMode, submit, themeBtn widget.Clickable
	help, github                                                   widget.Clickable // open the website / the code on GitHub
	google, cancelGoogle, finishGoogle                             widget.Clickable
	lUser, lPass                                                   *field // lUser: mobile number, or an older account's email
	rName, rPhone, rPass                                           *field
	mCode                                                          *field // management referral code
	gMobile                                                        *field // mobile of someone new signing up with Google
	aUser, aPass                                                   *field

	// Someone new to G-CATION signed in with Google: they add their mobile (and, for management, the
	// referral code) and the account is made with googleCode, the app's secret for that sign-in.
	needDetails bool
	googleName  string
	googleCode  string
	googleWait  bool // waiting for the browser sign-in to finish
	googleStop  chan struct{}
	busy        bool
	err         string
	list        widget.List
}

func (s *authState) init() {
	s.lUser = newField("Mobile number", "10-digit mobile number (or email)", false, key.HintAny)
	s.lPass = newField("Password", "Your password", true, key.HintAny)
	s.rName = newField("Full name", "Your full name", false, key.HintAny)
	s.rPhone = newField("Mobile number", "10-digit mobile number", false, key.HintTelephone)
	s.rPass = newField("Password", "At least 8 characters", true, key.HintAny)
	s.mCode = newField("Referral code", "The code the G-CATION admin gave you", false, key.HintAny)
	s.gMobile = newField("Mobile number", "10-digit mobile number", false, key.HintTelephone)
	s.aUser = newField("Admin username", "Username", false, key.HintAny)
	s.aPass = newField("Password", "Admin password", true, key.HintAny)
	s.list.Axis = layout.Vertical
}

// who is the account type being signed in: api.AsStudent or api.AsManagement.
func (s *authState) who() string {
	if s.mgmt {
		return api.AsManagement
	}
	return api.AsStudent
}

// mobileDigits turns "+91 98765-43210" or "098765 43210" into "9876543210".
func mobileDigits(text string) string {
	d := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, text)
	switch {
	case len(d) == 12 && strings.HasPrefix(d, "91"):
		d = d[2:]
	case len(d) == 11 && strings.HasPrefix(d, "0"):
		d = d[1:]
	}
	return d
}

var mobileRE = regexp.MustCompile(`^[6-9]\d{9}$`)

func (a *App) layoutAuth(gtx C) D {
	s := &a.auth
	if s.roleStudent.Clicked(gtx) {
		s.admin, s.mgmt, s.err, s.needDetails = false, false, "", false
	}
	if s.roleMgmt.Clicked(gtx) {
		s.admin, s.mgmt, s.register, s.err, s.needDetails = false, true, false, "", false
	}
	if s.roleAdmin.Clicked(gtx) {
		s.admin, s.mgmt, s.register, s.err, s.needDetails = true, false, false, "", false
	}
	if s.switchMode.Clicked(gtx) {
		s.register, s.err, s.needDetails = !s.register, "", false
	}
	if s.google.Clicked(gtx) && !s.googleWait {
		a.startGoogle()
	}
	if s.cancelGoogle.Clicked(gtx) {
		a.stopGoogle()
	}
	if s.finishGoogle.Clicked(gtx) && !s.busy {
		a.finishGoogle()
	}
	if s.themeBtn.Clicked(gtx) {
		a.toggleTheme()
	}
	if s.submit.Clicked(gtx) && !s.busy {
		a.submitAuth()
	}
	if s.help.Clicked(gtx) {
		a.open(a.webURL)
	}
	if s.github.Clicked(gtx) {
		a.open(githubURL)
	}

	screenH := gtx.Constraints.Max.Y
	heroH := max(gtx.Dp(250), screenH*30/100)
	items := []layout.Widget{
		func(gtx C) D { return a.authHeader(gtx, heroH) },
		func(gtx C) D {
			// The form sits on a sheet with rounded top corners, pulled up over the header.
			gtx.Constraints.Min.Y = screenH - heroH + gtx.Dp(28)
			return withBackground(gtx, func(gtx C, size image.Point) {
				r := gtx.Dp(28)
				paint.FillShape(gtx.Ops, a.th.P.Bg, clip.RRect{Rect: image.Rectangle{Max: size}, NW: r, NE: r}.Op(gtx.Ops))
			}, func(gtx C) D {
				return layout.Inset{Left: 22, Right: 22, Top: 26, Bottom: 28}.Layout(gtx, a.authForm)
			})
		},
	}
	return s.list.List.Layout(gtx, len(items), func(gtx C, i int) D {
		if i == 1 {
			// Overlap the header by the sheet's corner radius.
			st := pushOffset(gtx, image.Pt(0, -gtx.Dp(28)))
			d := items[i](gtx)
			st.Pop()
			d.Size.Y -= gtx.Dp(28)
			return d
		}
		return items[i](gtx)
	})
}

func (a *App) authHeader(gtx C, height int) D {
	size := image.Pt(gtx.Constraints.Max.X, height)
	gtx.Constraints = layout.Exact(size)
	gradientRound(gtx, size, 0, heroA, heroB)
	glow(gtx, image.Pt(size.X, 0), size.Y, argb(0xc98cff, 120))
	glow(gtx, image.Pt(0, size.Y), size.Y*2/3, argb(0xdb2777, 70))

	// Theme toggle in the corner
	st := pushOffset(gtx, image.Pt(size.X-gtx.Dp(58), gtx.Dp(14)))
	s := &a.auth
	s.themeBtn.Layout(gtx, func(gtx C) D {
		b := gtx.Dp(42)
		fillRound(gtx, image.Pt(b, b), 21, argb(0xffffff, 30))
		gtx.Constraints = layout.Exact(image.Pt(b, b))
		ic := icMoon
		if a.th.Dark {
			ic = icSun
		}
		return layout.Center.Layout(gtx, func(gtx C) D { return iconSized(gtx, ic, 21, white) })
	})
	st.Pop()

	return layout.Center.Layout(gtx, func(gtx C) D {
		return layout.Inset{Bottom: 24}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return imageBox(gtx, a.logo, image.Pt(gtx.Dp(92), gtx.Dp(92)), 0) }),
				layout.Rigid(vspace(10)),
				layout.Rigid(func(gtx C) D {
					return a.th.label(gtx, "G-CATION", textOpt{size: 28, weight: font.Black, color: white})
				}),
				layout.Rigid(vspace(2)),
				layout.Rigid(func(gtx C) D {
					return a.th.label(gtx, "Learn. Grow. Succeed.", textOpt{size: 14, color: rgb(0xd7c6f5)})
				}),
			)
		})
	})
}

func (a *App) authForm(gtx C) D {
	s := &a.auth
	title, sub, button := "Welcome back", "Log in to see your courses and updates.", "Log in"
	var fields []layout.Widget
	field := func(f *field) layout.Widget { return func(gtx C) D { return a.th.field(gtx, f) } }
	switch {
	case s.admin:
		title, sub, button = "Admin login", "Manage students, payments and updates.", "Log in as admin"
		fields = []layout.Widget{field(s.aUser), field(s.aPass)}
	case s.register:
		title, sub, button = "Create your account", "Registration is free. You only pay when you enroll.", "Create account"
		if s.mgmt {
			title, sub, button = "Join the team", "You need the referral code from the G-CATION admin.", "Create management account"
		}
		fields = []layout.Widget{field(s.rName), field(s.rPhone), field(s.rPass)}
		if s.mgmt {
			fields = append(fields, field(s.mCode))
		}
	default:
		if s.mgmt {
			title, sub = "Management login", "See courses and your Management ID."
		}
		fields = []layout.Widget{field(s.lUser), field(s.lPass)}
	}
	kids := []layout.FlexChild{
		layout.Rigid(func(gtx C) D { return a.roleSwitch(gtx) }),
		layout.Rigid(vspace(22)),
		layout.Rigid(func(gtx C) D { return a.th.title(gtx, title, 26) }),
		layout.Rigid(vspace(4)),
		layout.Rigid(func(gtx C) D { return a.th.muted(gtx, sub, 14) }),
	}
	for _, w := range fields {
		kids = append(kids, layout.Rigid(vspace(16)), layout.Rigid(w))
	}
	kids = append(kids,
		layout.Rigid(func(gtx C) D { return a.errLine(gtx, s.err) }),
		layout.Rigid(vspace(22)),
		layout.Rigid(func(gtx C) D { return a.th.button(gtx, &s.submit, button, btnOpt{full: true, busy: s.busy}) }),
	)
	if !s.admin {
		kids = append(kids, layout.Rigid(vspace(18)), layout.Rigid(a.googleBlock))
	}
	kids = append(kids,
		layout.Rigid(vspace(16)),
		layout.Rigid(func(gtx C) D {
			if s.admin {
				return centered(gtx, "Only the G-CATION admin can log in here.", a.th, textOpt{size: 13, color: a.th.P.Muted})
			}
			q, action := "New to G-CATION?", "Create an account"
			switch {
			case s.register:
				q, action = "Already have an account?", "Log in"
			case s.mgmt:
				q, action = "New to the team?", "Register"
			}
			return layout.Center.Layout(gtx, func(gtx C) D {
				return s.switchMode.Layout(gtx, func(gtx C) D {
					return layout.Inset{Top: 6, Bottom: 6}.Layout(gtx, func(gtx C) D {
						return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
							layout.Rigid(func(gtx C) D { return a.th.muted(gtx, q+" ", 14) }),
							layout.Rigid(func(gtx C) D {
								return a.th.label(gtx, action, textOpt{size: 14, weight: font.Bold, color: a.th.P.Accent})
							}),
						)
					})
				})
			})
		}),
		layout.Rigid(func(gtx C) D {
			if s.admin || s.register {
				return D{}
			}
			return layout.Inset{Top: 6}.Layout(gtx, func(gtx C) D {
				msg := "Forgot your password? Call the G-CATION team."
				if s.mgmt {
					msg = "Forgot your password? Contact the G-CATION admin."
				}
				return centered(gtx, msg, a.th, textOpt{size: 12.5, color: a.th.P.Faint})
			})
		}),
		// Help (the website's home page) and the code on GitHub, at the bottom of every form.
		layout.Rigid(vspace(18)),
		layout.Rigid(func(gtx C) D {
			return layout.Center.Layout(gtx, func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return a.th.button(gtx, &s.help, "Help", btnOpt{kind: btnOutline, small: true, icon: icInfo})
					}),
					layout.Rigid(hspace(10)),
					layout.Rigid(func(gtx C) D {
						return a.th.button(gtx, &s.github, "GitHub", btnOpt{kind: btnOutline, small: true, logo: githubMark})
					}),
				)
			})
		}),
	)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
}

// googleBlock is "Continue with Google", a note while the browser sign-in is open, or the last step
// for someone new: their mobile (and referral code).
func (a *App) googleBlock(gtx C) D {
	s := &a.auth
	kids := []layout.FlexChild{
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx C) D { return a.th.divider(gtx) }),
				layout.Rigid(func(gtx C) D {
					return layout.Inset{Left: 12, Right: 12}.Layout(gtx, func(gtx C) D { return a.th.muted(gtx, "OR", 12) })
				}),
				layout.Flexed(1, func(gtx C) D { return a.th.divider(gtx) }),
			)
		}),
		layout.Rigid(vspace(16)),
	}
	if s.needDetails {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, append(kids, layout.Rigid(a.googleDetails))...)
	}
	if s.googleWait {
		kids = append(kids,
			layout.Rigid(func(gtx C) D {
				return a.th.notice(gtx, icInfo, toneBlue, func(gtx C) D {
					return a.th.label(gtx, "Choose your Google account in the browser, then come back here. You will be signed in automatically.",
						textOpt{size: 13, color: a.th.P.Blue})
				})
			}),
			layout.Rigid(vspace(10)),
			layout.Rigid(func(gtx C) D { return a.th.button(gtx, &s.cancelGoogle, "Cancel", btnOpt{full: true, kind: btnGhost}) }),
		)
	} else {
		kids = append(kids, layout.Rigid(func(gtx C) D {
			return a.th.button(gtx, &s.google, "Continue with Google", btnOpt{full: true, kind: btnOutline, logo: googleMark})
		}))
	}
	kids = append(kids, layout.Rigid(func(gtx C) D {
		return layout.Inset{Top: 8}.Layout(gtx, func(gtx C) D {
			return centered(gtx, "Registered with your Gmail before? Use the same Google account and your data stays.", a.th,
				textOpt{size: 12, color: a.th.P.Faint})
		})
	}))
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
}

// roleSwitch is the Student / Management / Admin segmented control.
func (a *App) roleSwitch(gtx C) D {
	s := &a.auth
	seg := func(clk *widget.Clickable, ic *widget.Icon, label string, on bool) layout.FlexChild {
		return layout.Flexed(1, func(gtx C) D {
			return clk.Layout(gtx, func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return withBackground(gtx, func(gtx C, size image.Point) {
					if on {
						gradientRound(gtx, size, 12, gradA, gradB)
					}
				}, func(gtx C) D {
					return layout.Inset{Top: 11, Bottom: 11}.Layout(gtx, func(gtx C) D {
						return layout.Center.Layout(gtx, func(gtx C) D {
							fg := a.th.P.Muted
							if on {
								fg = white
							}
							return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
								layout.Rigid(func(gtx C) D { return iconSized(gtx, ic, 17, fg) }),
								layout.Rigid(hspace(5)),
								layout.Rigid(func(gtx C) D {
									return a.th.label(gtx, label, textOpt{size: 13.5, weight: font.Bold, color: fg, maxLines: 1})
								}),
							)
						})
					})
				})
			})
		})
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return tinted(gtx, a.th.P.Lav, 15, 4, func(gtx C) D {
		return layout.Flex{}.Layout(gtx,
			seg(&s.roleStudent, icSchool, "Student", !s.admin && !s.mgmt),
			layout.Rigid(hspace(4)),
			seg(&s.roleMgmt, icPerson, "Management", s.mgmt),
			layout.Rigid(hspace(4)),
			seg(&s.roleAdmin, icShield, "Admin", s.admin),
		)
	})
}

func (a *App) submitAuth() {
	s := &a.auth
	type rule = struct {
		ok  bool
		msg string
	}
	check := func(rules ...rule) string {
		for _, r := range rules {
			if !r.ok {
				return r.msg
			}
		}
		return ""
	}
	switch {
	case s.admin:
		s.err = check(rule{s.aUser.Text() != "", "Enter the admin username."}, rule{s.aPass.Ed.Text() != "", "Enter the password."})
	case s.register:
		s.err = check(
			rule{s.rName.Text() != "", "Enter your full name."},
			rule{mobileRE.MatchString(mobileDigits(s.rPhone.Text())), "Enter a valid 10-digit mobile number."},
			rule{len(s.rPass.Ed.Text()) >= 8, "Password must be at least 8 characters."},
			rule{!s.mgmt || s.mCode.Text() != "", "Enter the referral code from the G-CATION admin."},
		)
	default:
		login := s.lUser.Text()
		s.err = check(rule{strings.Contains(login, "@") || mobileRE.MatchString(mobileDigits(login)), "Enter your 10-digit mobile number."},
			rule{s.lPass.Ed.Text() != "", "Enter your password."})
	}
	if s.err != "" {
		return
	}
	s.busy = true
	admin, who := s.admin, s.who()
	user, apass := s.aUser.Text(), s.aPass.Ed.Text()
	var work func() error
	switch {
	case admin:
		work = func() error { return a.api.AdminLogin(user, apass) }
	case s.register:
		name, mobile, pass, invite := s.rName.Text(), mobileDigits(s.rPhone.Text()), s.rPass.Ed.Text(), s.mCode.Text()
		work = func() error { return a.api.Register(who, name, mobile, pass, invite) }
	default:
		login, pass := s.lUser.Text(), s.lPass.Ed.Text()
		if !strings.Contains(login, "@") {
			login = mobileDigits(login)
		}
		work = func() error { return a.api.Login(who, login, pass) }
	}
	a.async(func() func() {
		err := work()
		return func() {
			s.busy = false
			if err != nil {
				s.err = err.Error()
				return
			}
			if admin {
				for _, f := range []*field{s.aPass} {
					f.Ed.SetText("")
				}
				s.err = ""
				a.startAdmin()
				return
			}
			a.signedIn(who)
		}
	})
}

// signedIn clears the forms and opens the student or management side.
func (a *App) signedIn(who string) {
	s := &a.auth
	for _, f := range []*field{s.lPass, s.rPass, s.aPass, s.mCode} {
		f.Ed.SetText("")
	}
	s.err, s.register, s.needDetails = "", false, false
	if who == api.AsManagement {
		a.startManagement()
		return
	}
	a.screen, a.tab = scrSplash, tabHome
	a.reload()
}

// startGoogle opens Google sign-in in the browser, then waits for the API to say it is done.
// googleDetails asks someone new who signed in with Google for their mobile (and referral code).
func (a *App) googleDetails(gtx C) D {
	s := &a.auth
	who := "Hi " + firstName(s.googleName) + ", one more step: enter your mobile number to finish creating your account."
	if s.googleName == "" {
		who = "One more step: enter your mobile number to finish creating your account."
	}
	if s.mgmt {
		who = strings.Replace(who, "your mobile number", "your mobile number and the referral code from the G-CATION admin", 1)
	}
	return a.th.card(gtx, 16, func(gtx C) D {
		kids := []layout.FlexChild{
			layout.Rigid(func(gtx C) D { return a.th.title(gtx, "One more step", 18) }),
			layout.Rigid(vspace(4)),
			layout.Rigid(func(gtx C) D { return a.th.muted(gtx, who, 13) }),
			layout.Rigid(vspace(12)),
			layout.Rigid(func(gtx C) D { return a.th.field(gtx, s.gMobile) }),
		}
		if s.mgmt {
			kids = append(kids, layout.Rigid(vspace(12)), layout.Rigid(func(gtx C) D { return a.th.field(gtx, s.mCode) }))
		}
		label := "Create account"
		if s.mgmt {
			label = "Create management account"
		}
		kids = append(kids,
			layout.Rigid(vspace(14)),
			layout.Rigid(func(gtx C) D { return a.th.button(gtx, &s.finishGoogle, label, btnOpt{full: true, busy: s.busy}) }),
			layout.Rigid(vspace(6)),
			layout.Rigid(func(gtx C) D { return a.th.button(gtx, &s.cancelGoogle, "Cancel", btnOpt{full: true, kind: btnGhost}) }),
		)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
	})
}

// finishGoogle creates the account of someone new who signed in with Google.
func (a *App) finishGoogle() {
	s := &a.auth
	mobile, invite := mobileDigits(s.gMobile.Text()), s.mCode.Text()
	switch {
	case !mobileRE.MatchString(mobile):
		s.err = "Enter a valid 10-digit mobile number."
		return
	case s.mgmt && invite == "":
		s.err = "Enter the referral code from the G-CATION admin."
		return
	}
	s.busy, s.err = true, ""
	who, code := s.who(), s.googleCode
	a.async(func() func() {
		err := a.api.GoogleComplete(who, code, mobile, invite)
		return func() {
			s.busy = false
			if err != nil {
				s.err = err.Error()
				return
			}
			s.gMobile.Ed.SetText("")
			a.signedIn(who)
		}
	})
}

func (a *App) startGoogle() {
	s := &a.auth
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		s.err = "Could not start Google sign-in. Please try again."
		return
	}
	code := base64.RawURLEncoding.EncodeToString(b)
	who := s.who()
	s.err, s.googleWait, s.googleCode = "", true, code
	stop := make(chan struct{})
	s.googleStop = stop
	a.open(a.api.GoogleStartURL(who, code))
	a.async(func() func() {
		deadline := time.Now().Add(10 * time.Minute)
		for time.Now().Before(deadline) {
			select {
			case <-stop:
				return nil
			case <-time.After(2 * time.Second):
			}
			status, detail, name, err := a.api.GoogleResult(who, code)
			if err != nil || status == "pending" {
				continue // offline for a moment, or the person is still choosing
			}
			return func() {
				if s.googleStop != stop {
					return // cancelled meanwhile
				}
				s.googleWait = false
				switch status {
				case "ok":
					a.signedIn(who)
				case "need_details":
					s.needDetails, s.googleName = true, name
				default:
					s.err = detail
				}
			}
		}
		return func() {
			if s.googleStop == stop {
				s.googleWait, s.err = false, "Google sign-in timed out. Please try again."
			}
		}
	})
}

func (a *App) stopGoogle() {
	s := &a.auth
	if s.googleStop != nil {
		close(s.googleStop)
		s.googleStop = nil
	}
	s.googleWait, s.needDetails, s.googleCode = false, false, ""
}
