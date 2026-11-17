package api

import (
	"fmt"
	"net/http"
)

// Management team members refer students. They see the courses and prices and share their
// Management ID (GCM001), which students enter as the Referral ID. They cannot enroll.

type Manager struct {
	ID        int64  `json:"id"`
	Code      string `json:"manager_code"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	CreatedAt int64  `json:"created_at"`
	RevokedAt *int64 `json:"revoked_at"`
	// Set by the admin: the member may post updates until it is turned off.
	CanPostUpdates bool `json:"can_post_updates"`
	SignIn
	// Present in the admin's list:
	Approved int `json:"approved"`
	Pending  int `json:"pending"`
}

type ManagerMe struct {
	Manager Manager  `json:"manager"`
	Courses []Course `json:"courses"`
	Unread  int      `json:"unread_announcements"`
}

func (c *Client) ManagerLogout() {
	_ = c.do(http.MethodPost, "/api/management/logout", nil, nil)
	c.setToken(roleManager, "")
}

func (c *Client) ManagerMe() (*ManagerMe, error) {
	var m ManagerMe
	return &m, c.do(http.MethodGet, "/api/management/me", nil, &m)
}

// ManagerUpdates are the updates the admin sent to the management team.
func (c *Client) ManagerUpdates() (*Updates, error) {
	var u Updates
	return &u, c.do(http.MethodGet, "/api/management/me/announcements", nil, &u)
}

func (c *Client) ManagerMarkUpdatesSeen() error {
	return c.do(http.MethodPost, "/api/management/me/announcements/seen", nil, nil)
}

func (c *Client) ManagerChangePassword(current, next string) error {
	return c.do(http.MethodPost, "/api/management/me/password", map[string]string{"current_password": current, "new_password": next}, nil)
}

// ---------- Admin: management team ----------

// Managers returns the referral code needed to register as management and every member.
func (c *Client) Managers() (string, []Manager, error) {
	var out struct {
		InviteCode string    `json:"invite_code"`
		Items      []Manager `json:"items"`
	}
	err := c.do(http.MethodGet, "/api/admin/managers", nil, &out)
	return out.InviteCode, out.Items, err
}

// SetInviteCode changes the management referral code; "" closes registration.
func (c *Client) SetInviteCode(code string) error {
	return c.do(http.MethodPut, "/api/admin/managers/invite-code", map[string]string{"invite_code": code}, nil)
}

// RevokeManager blocks a member's login until RestoreManager is called.
func (c *Client) RevokeManager(id int64) error {
	return c.do(http.MethodPost, fmt.Sprintf("/api/admin/managers/%d/revoke", id), nil, nil)
}

func (c *Client) RestoreManager(id int64) error {
	return c.do(http.MethodPost, fmt.Sprintf("/api/admin/managers/%d/restore", id), nil, nil)
}

// ResetManagerPassword sets a new password for a member who forgot theirs and logs them out everywhere.
func (c *Client) ResetManagerPassword(id int64, pw string) error {
	return c.do(http.MethodPost, fmt.Sprintf("/api/admin/managers/%d/password", id), map[string]string{"new_password": pw}, nil)
}

// SetUpdatesAccess lets a member post updates (on) or takes it back (off).
func (c *Client) SetUpdatesAccess(id int64, enabled bool) error {
	return c.do(http.MethodPut, fmt.Sprintf("/api/admin/managers/%d/updates-access", id), map[string]bool{"enabled": enabled}, nil)
}
