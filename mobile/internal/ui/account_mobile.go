package ui

import (
	"errors"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/widget"

	"gcation/mobile/internal/api"
)

// mobileForm sets or changes the mobile number people log in with. Some older accounts and Google
// accounts have none until they add one here.
type mobileForm struct {
	mobile *field
	save   widget.Clickable
	busy   bool
	err    string
}

func (f *mobileForm) init() {
	f.mobile = newField("Mobile number", "10-digit mobile number", false, key.HintTelephone)
}

// mobileCard shows the current log-in mobile and the form to set or change it. who is api.AsStudent
// or api.AsManagement; current is the mobile as shown ("" if none); onSaved reloads the account.
func (a *App) mobileCard(gtx C, f *mobileForm, who, current string, onSaved func()) D {
	if f.save.Clicked(gtx) && !f.busy {
		a.saveMobile(f, who, onSaved)
	}
	intro := "Add your mobile number to log in with it and your password."
	if current != "" {
		intro = "You log in with " + current + ". Enter a new number to change it."
	}
	return a.th.card(gtx, 18, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return a.cardTitle(gtx, icCall, "Mobile number for log in") }),
			layout.Rigid(vspace(4)),
			layout.Rigid(func(gtx C) D { return a.th.muted(gtx, intro, 12.5) }),
			layout.Rigid(vspace(12)),
			layout.Rigid(func(gtx C) D { return a.th.field(gtx, f.mobile) }),
			layout.Rigid(func(gtx C) D { return a.errLine(gtx, f.err) }),
			layout.Rigid(vspace(14)),
			layout.Rigid(func(gtx C) D {
				return a.th.button(gtx, &f.save, "Save mobile number", btnOpt{full: true, icon: icTick, busy: f.busy})
			}),
		)
	})
}

func (a *App) saveMobile(f *mobileForm, who string, onSaved func()) {
	mobile := mobileDigits(f.mobile.Text())
	if !mobileRE.MatchString(mobile) {
		f.err = "Enter a valid 10-digit mobile number."
		return
	}
	f.busy, f.err = true, ""
	a.async(func() func() {
		err := a.api.SetMobile(who, mobile)
		return func() {
			f.busy = false
			if errors.Is(err, api.ErrUnauthorized) {
				a.signedOut("Please log in again.")
				a.auth.mgmt = who == api.AsManagement
				return
			}
			if err != nil {
				f.err = err.Error()
				return
			}
			f.mobile.Ed.SetText("")
			a.toast("Mobile number saved. You can log in with it now.", false)
			onSaved()
		}
	})
}

// contactText is how an account is shown: its log-in mobile, else its email, else its phone.
func contactText(s api.SignIn, email, phone string) string {
	if m := s.MobileText(); m != "" {
		return m
	}
	if email != "" {
		return email
	}
	return phone
}
