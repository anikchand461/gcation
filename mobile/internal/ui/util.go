package ui

import (
	"bytes"
	"fmt"
	"image"
	"math"
	"net/url"
	"runtime"
	"slices"
	"strings"
	"time"
	"unicode"

	"gioui.org/op"
)

var ist = time.FixedZone("IST", 5*3600+1800)

func pushOffset(gtx C, p image.Point) op.TransformStack { return op.Offset(p).Push(gtx.Ops) }

func initials(name string) string {
	var out []rune
	for _, w := range strings.Fields(name) {
		for _, r := range w {
			out = append(out, unicode.ToUpper(r))
			break
		}
		if len(out) == 2 {
			break
		}
	}
	if len(out) == 0 {
		return "?"
	}
	return string(out)
}

func firstName(name string) string {
	if f := strings.Fields(name); len(f) > 0 {
		return f[0]
	}
	return name
}

// rupees formats with Indian digit grouping: 123456.5 -> ₹1,23,456.5
func rupees(v float64) string {
	neg := v < 0
	v = math.Abs(v)
	whole := int64(v)
	frac := int64(math.Round((v - float64(whole)) * 100))
	if frac == 100 {
		whole, frac = whole+1, 0
	}
	s := fmt.Sprint(whole)
	if len(s) > 3 {
		head, tail := s[:len(s)-3], s[len(s)-3:]
		var groups []string
		for len(head) > 2 {
			groups = append([]string{head[len(head)-2:]}, groups...)
			head = head[:len(head)-2]
		}
		if head != "" {
			groups = append([]string{head}, groups...)
		}
		s = strings.Join(groups, ",") + "," + tail
	}
	if frac > 0 {
		s += fmt.Sprintf(".%02d", frac)
	}
	if neg {
		s = "-" + s
	}
	return "₹" + s
}

func fmtDate(ts int64) string {
	if ts == 0 {
		return "—"
	}
	return time.Unix(ts, 0).In(ist).Format("2 Jan 2006")
}

func fmtDateTime(ts int64) string {
	if ts == 0 {
		return "—"
	}
	return time.Unix(ts, 0).In(ist).Format("2 Jan 2006, 3:04 pm")
}

func timeAgo(ts int64) string {
	d := time.Since(time.Unix(ts, 0))
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d min ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d hr ago", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%d days ago", int(d.Hours()/24))
	}
	return fmtDate(ts)
}

func durationLabel(days int) string {
	switch {
	case days%365 == 0:
		if days == 365 {
			return "1 year"
		}
		return fmt.Sprintf("%d years", days/365)
	case days%30 == 0:
		if days == 30 {
			return "1 month"
		}
		return fmt.Sprintf("%d months", days/30)
	}
	return fmt.Sprintf("%d days", days)
}

func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// Extra help contacts shown to students, next to the admin-set support number.
const (
	helpPhone2 = "+91 89726 21425"
	helpEmail  = "gcationadmin27@gmail.com"
)

// supportPhones lists both support numbers, without blanks or repeats.
func supportPhones(adminPhone string) []string {
	var out []string
	for _, p := range []string{adminPhone, helpPhone2} {
		if p != "" && !slices.Contains(out, p) {
			out = append(out, p)
		}
	}
	return out
}

// mailURL writes to the help email: the mail app on a phone, Gmail in the browser on a computer
// (computers often have no mail app set up).
func mailURL(subject, body string) string {
	if runtime.GOOS == "android" {
		esc := func(s string) string { return strings.ReplaceAll(url.QueryEscape(s), "+", "%20") } // mail apps read + literally
		return "mailto:" + helpEmail + "?subject=" + esc(subject) + "&body=" + esc(body)
	}
	return "https://mail.google.com/mail/?view=cm&fs=1&to=" + helpEmail + "&su=" + url.QueryEscape(subject) + "&body=" + url.QueryEscape(body)
}

// telURL and waURL build phone and WhatsApp links (assumes India when no country code is given).
func telURL(phone string) string {
	return "tel:" + strings.Map(func(r rune) rune {
		if unicode.IsDigit(r) || r == '+' {
			return r
		}
		return -1
	}, phone)
}

func waURL(phone, msg string) string {
	digits := strings.Map(func(r rune) rune {
		if unicode.IsDigit(r) {
			return r
		}
		return -1
	}, phone)
	if len(digits) == 10 {
		digits = "91" + digits
	}
	u := "https://wa.me/" + digits
	if msg != "" {
		u += "?text=" + url.QueryEscape(msg)
	}
	return u
}

// upiURL opens any installed UPI app with the payee, amount and Student ID filled in.
func upiURL(upiID, payee string, amount float64, studentID int64) string {
	if payee == "" {
		payee = "G-CATION"
	}
	return fmt.Sprintf("upi://pay?pa=%s&pn=%s&am=%.2f&cu=INR&tn=%s",
		strings.ReplaceAll(url.QueryEscape(upiID), "%40", "@"), url.QueryEscape(payee), amount,
		url.QueryEscape(fmt.Sprintf("GCATION ID %d", studentID)))
}

func greeting() string {
	h := time.Now().In(ist).Hour()
	switch {
	case h < 12:
		return "GOOD MORNING"
	case h < 17:
		return "GOOD AFTERNOON"
	}
	return "GOOD EVENING"
}

func bytesReader(b []byte) *bytes.Reader { return bytes.NewReader(b) }
