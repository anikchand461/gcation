package api

import (
	"net/http"
	"net/url"
	"strings"
)

// Students and management sign in with a mobile number and password, or with Google. Accounts
// made before mobile log-in can type their email instead (in the same box) and add a mobile on
// their profile. (The admin has its own fixed login, AdminLogin.)

// Who is signing in.
const (
	AsStudent    = "student"
	AsManagement = "management"
)

// SignIn says how an account signs in.
type SignIn struct {
	Mobile      *string `json:"mobile"` // 10-digit log-in mobile; nil until the account has one
	HasPassword bool    `json:"has_password"`
	Google      bool    `json:"google"`
}

// MobileText is the log-in mobile as "+91 9876543210", or "".
func (s SignIn) MobileText() string {
	if s.Mobile == nil {
		return ""
	}
	return "+91 " + *s.Mobile
}

func tokenRole(who string) role {
	if who == AsManagement {
		return roleManager
	}
	return roleStudent
}

func base(who string) string {
	if who == AsManagement {
		return "/api/management"
	}
	return "/api"
}

func (c *Client) signedIn(who, path string, body any) error {
	var out struct{ Token string }
	err := c.do(http.MethodPost, path, body, &out)
	if err == nil {
		c.setToken(tokenRole(who), out.Token)
	}
	return err
}

// Login with a mobile number, or with the email of an account made before mobile log-in.
func (c *Client) Login(who, mobileOrEmail, password string) error {
	key := "mobile"
	if strings.Contains(mobileOrEmail, "@") {
		key = "email"
	}
	return c.signedIn(who, base(who)+"/login", map[string]string{key: mobileOrEmail, "password": password, "client": "app"})
}

// Register with a name, 10-digit mobile number and password. Management also needs the admin's referral code.
func (c *Client) Register(who, name, mobile, password, inviteCode string) error {
	body := map[string]string{"name": name, "mobile": mobile, "password": password, "client": "app"}
	if who == AsManagement {
		body["invite_code"] = inviteCode
	}
	return c.signedIn(who, base(who)+"/register", body)
}

// SetMobile sets or changes the mobile number the logged-in account logs in with.
func (c *Client) SetMobile(who, mobile string) error {
	return c.do(http.MethodPut, base(who)+"/me/mobile", map[string]string{"mobile": mobile}, nil)
}

// GoogleStartURL is the page the app opens in the browser to sign in with Google. code is a long
// random secret the app keeps; GoogleResult with the same code collects the login afterwards.
func (c *Client) GoogleStartURL(who, code string) string {
	return c.Base + "/api/auth/google/start?" + url.Values{"role": {who}, "code": {code}}.Encode()
}

// GoogleResult reports "pending", "ok" (the login is saved), "need_details" (someone new to
// G-CATION must give their mobile, and management the referral code, then call GoogleComplete)
// or "error". detail is a message to show; name is the person's Google name for need_details.
func (c *Client) GoogleResult(who, code string) (status, detail, name string, err error) {
	var out struct{ Status, Detail, Name, Token string }
	if err = c.do(http.MethodGet, "/api/auth/google/result?code="+url.QueryEscape(code), nil, &out); err != nil {
		return "", "", "", err
	}
	if out.Status == "ok" {
		c.setToken(tokenRole(who), out.Token)
	}
	return out.Status, out.Detail, out.Name, nil
}

// GoogleComplete creates the account of someone new who signed in with Google, with their mobile
// number (and, for management, the referral code), and saves the login.
func (c *Client) GoogleComplete(who, code, mobile, inviteCode string) error {
	return c.signedIn(who, "/api/auth/google/complete", map[string]string{"code": code, "mobile": mobile, "invite_code": inviteCode})
}
