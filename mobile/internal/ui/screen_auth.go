package ui

import (
	"image"
	"regexp"
	"strings"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"
)

var emailRE = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

type authState struct {
	admin    bool // Student / Admin switch
	register bool // student: create account instead of log in

	roleStudent, roleAdmin, switchMode, submit, themeBtn widget.Clickable
	lEmail, lPass                                        *field
	rName, rEmail, rPhone, rPass                         *field
	aUser, aPass                                         *field
	busy                                                 bool
	err                                                  string
	list                                                 widget.List
}

func (s *authState) init() {
	s.lEmail = newField("Email", "you@gmail.com", false, key.HintEmail)
	s.lPass = newField("Password", "Your password", true, key.HintAny)
	s.rName = newField("Full name", "Your full name", false, key.HintAny)
	s.rEmail = newField("Email", "you@gmail.com", false, key.HintEmail)
	s.rPhone = newField("Phone number", "10-digit mobile number", false, key.HintTelephone)
	s.rPass = newField("Password", "At least 8 characters", true, key.HintAny)
	s.aUser = newField("Admin username", "Username", false, key.HintAny)
	s.aPass = newField("Password", "Admin password", true, key.HintAny)
	s.list.Axis = layout.Vertical
}

func (a *App) layoutAuth(gtx C) D {
	s := &a.auth
	if s.roleStudent.Clicked(gtx) {
		s.admin, s.err = false, ""
	}
	if s.roleAdmin.Clicked(gtx) {
		s.admin, s.register, s.err = true, false, ""
	}
	if s.switchMode.Clicked(gtx) {
		s.register, s.err = !s.register, ""
	}
	if s.themeBtn.Clicked(gtx) {
		a.toggleTheme()
	}
	if s.submit.Clicked(gtx) && !s.busy {
		a.submitAuth()
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
	fields := []*field{s.lEmail, s.lPass}
	switch {
	case s.admin:
		title, sub, button = "Admin login", "Manage students, payments and updates.", "Log in as admin"
		fields = []*field{s.aUser, s.aPass}
	case s.register:
		title, sub, button = "Create your account", "Registration is free. You only pay when you enroll.", "Create account"
		fields = []*field{s.rName, s.rEmail, s.rPhone, s.rPass}
	}
	kids := []layout.FlexChild{
		layout.Rigid(func(gtx C) D { return a.roleSwitch(gtx) }),
		layout.Rigid(vspace(22)),
		layout.Rigid(func(gtx C) D { return a.th.title(gtx, title, 26) }),
		layout.Rigid(vspace(4)),
		layout.Rigid(func(gtx C) D { return a.th.muted(gtx, sub, 14) }),
	}
	for _, f := range fields {
		kids = append(kids, layout.Rigid(vspace(16)), layout.Rigid(func(gtx C) D { return a.th.field(gtx, f) }))
	}
	kids = append(kids,
		layout.Rigid(func(gtx C) D { return a.errLine(gtx, s.err) }),
		layout.Rigid(vspace(22)),
		layout.Rigid(func(gtx C) D { return a.th.button(gtx, &s.submit, button, btnOpt{full: true, busy: s.busy}) }),
		layout.Rigid(vspace(16)),
		layout.Rigid(func(gtx C) D {
			if s.admin {
				return centered(gtx, "Only the G-CATION admin can log in here.", a.th, textOpt{size: 13, color: a.th.P.Muted})
			}
			q, action := "New to G-CATION?", "Create an account"
			if s.register {
				q, action = "Already have an account?", "Log in"
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
				return centered(gtx, "Forgot your password? Call the G-CATION team.", a.th, textOpt{size: 12.5, color: a.th.P.Faint})
			})
		}),
	)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
}

// roleSwitch is the Student / Admin segmented control.
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
								layout.Rigid(func(gtx C) D { return iconSized(gtx, ic, 18, fg) }),
								layout.Rigid(hspace(6)),
								layout.Rigid(func(gtx C) D { return a.th.label(gtx, label, textOpt{size: 14.5, weight: font.Bold, color: fg}) }),
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
			seg(&s.roleStudent, icSchool, "Student", !s.admin),
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
		digits := strings.Map(func(r rune) rune {
			if r >= '0' && r <= '9' {
				return r
			}
			return -1
		}, s.rPhone.Text())
		s.err = check(
			rule{s.rName.Text() != "", "Enter your full name."},
			rule{emailRE.MatchString(s.rEmail.Text()), "Enter a valid email address."},
			rule{len(digits) >= 10, "Enter a valid phone number."},
			rule{len(s.rPass.Ed.Text()) >= 8, "Password must be at least 8 characters."},
		)
	default:
		s.err = check(rule{strings.Contains(s.lEmail.Text(), "@"), "Enter your email address."}, rule{s.lPass.Ed.Text() != "", "Enter your password."})
	}
	if s.err != "" {
		return
	}
	s.busy = true
	admin, register := s.admin, s.register
	name, email, phone, pass := s.rName.Text(), s.rEmail.Text(), s.rPhone.Text(), s.rPass.Ed.Text()
	if !register {
		email, pass = s.lEmail.Text(), s.lPass.Ed.Text()
	}
	user, apass := s.aUser.Text(), s.aPass.Ed.Text()
	a.async(func() func() {
		var err error
		switch {
		case admin:
			err = a.api.AdminLogin(user, apass)
		case register:
			err = a.api.Register(name, email, phone, pass)
		default:
			err = a.api.Login(email, pass)
		}
		return func() {
			s.busy = false
			if err != nil {
				s.err = err.Error()
				return
			}
			s.err = ""
			for _, f := range []*field{s.lPass, s.rPass, s.aPass} {
				f.Ed.SetText("")
			}
			if admin {
				a.startAdmin()
				return
			}
			a.screen, a.tab = scrSplash, tabHome
			a.reload()
		}
	})
}
