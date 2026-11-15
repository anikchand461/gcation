// Package api talks to the G-CATION backend (the same API the website uses).
//
// The app logs in with "client": "app" and keeps the returned token in the app's private
// storage, sending it as "Authorization: Bearer <token>" on every request.
package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // decoders for course thumbnails and QR images
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "golang.org/x/image/webp"
)

// ErrUnauthorized means the saved login is missing, expired or was revoked.
var ErrUnauthorized = errors.New("please log in again")

type Client struct {
	Base string
	http *http.Client
	dir  string

	mu     sync.Mutex
	tokens map[role]string // one saved login per role
}

type role string

const (
	roleStudent role = "session"
	roleAdmin   role = "admin"
)

func roleFor(path string) role {
	if strings.HasPrefix(path, "/api/admin") {
		return roleAdmin
	}
	return roleStudent
}

func New(base, dataDir string) *Client {
	c := &Client{
		Base:   strings.TrimRight(base, "/"),
		http:   &http.Client{Timeout: 60 * time.Second}, // the free server can take a while to wake up
		dir:    dataDir,
		tokens: map[role]string{},
	}
	for _, r := range []role{roleStudent, roleAdmin} {
		if b, err := os.ReadFile(c.tokenPath(r)); err == nil {
			c.tokens[r] = strings.TrimSpace(string(b))
		}
	}
	return c
}

func (c *Client) tokenPath(r role) string { return filepath.Join(c.dir, string(r)+".token") }

// LoggedIn reports whether a student login is saved; AdminLoggedIn the same for the admin.
func (c *Client) LoggedIn() bool      { return c.token(roleStudent) != "" }
func (c *Client) AdminLoggedIn() bool { return c.token(roleAdmin) != "" }

func (c *Client) token(r role) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.tokens[r]
}

func (c *Client) setToken(r role, t string) {
	c.mu.Lock()
	c.tokens[r] = t
	c.mu.Unlock()
	if t == "" {
		os.Remove(c.tokenPath(r))
		return
	}
	_ = os.MkdirAll(c.dir, 0o700)
	_ = os.WriteFile(c.tokenPath(r), []byte(t), 0o600)
}

func (c *Client) request(method, path string, body any) (*http.Response, error) {
	var rd io.Reader
	if body != nil || method != http.MethodGet {
		if body == nil {
			body = struct{}{}
		}
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.Base+path, rd)
	if err != nil {
		return nil, err
	}
	if rd != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if t := c.token(roleFor(path)); t != "" {
		req.Header.Set("Authorization", "Bearer "+t)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, errors.New("cannot reach the server. Check your internet connection")
	}
	return resp, nil
}

// do sends a JSON request and decodes a JSON response into out (if not nil).
func (c *Client) do(method, path string, body, out any) error {
	resp, err := c.request(method, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusUnauthorized && !strings.HasSuffix(path, "/login") {
		c.setToken(roleFor(path), "")
		return ErrUnauthorized
	}
	if resp.StatusCode >= 400 {
		return errors.New(errorText(data, resp.StatusCode))
	}
	if out != nil && len(data) > 0 {
		return json.Unmarshal(data, out)
	}
	return nil
}

// errorText turns FastAPI's {"detail": ...} into a readable message.
func errorText(data []byte, status int) string {
	var e struct {
		Detail json.RawMessage `json:"detail"`
	}
	if json.Unmarshal(data, &e) == nil && len(e.Detail) > 0 {
		var s string
		if json.Unmarshal(e.Detail, &s) == nil {
			return s
		}
		var list []struct {
			Loc []any  `json:"loc"`
			Msg string `json:"msg"`
		}
		if json.Unmarshal(e.Detail, &list) == nil && len(list) > 0 {
			parts := make([]string, 0, len(list))
			for _, d := range list {
				msg := strings.TrimPrefix(d.Msg, "Value error, ")
				if n := len(d.Loc); n > 0 {
					if f, ok := d.Loc[n-1].(string); ok {
						msg = strings.ReplaceAll(f, "_", " ") + ": " + msg
					}
				}
				parts = append(parts, msg)
			}
			return strings.Join(parts, " · ")
		}
	}
	return fmt.Sprintf("Something went wrong (%d). Please try again.", status)
}

// ---------- Auth ----------

func (c *Client) Login(email, password string) error {
	var out struct{ Token string }
	err := c.do(http.MethodPost, "/api/login", map[string]string{"email": email, "password": password, "client": "app"}, &out)
	if err == nil {
		c.setToken(roleStudent, out.Token)
	}
	return err
}

func (c *Client) Register(name, email, phone, password string) error {
	var out struct{ Token string }
	err := c.do(http.MethodPost, "/api/register", map[string]string{
		"name": name, "email": email, "phone": phone, "password": password, "client": "app",
	}, &out)
	if err == nil {
		c.setToken(roleStudent, out.Token)
	}
	return err
}

func (c *Client) Logout() {
	_ = c.do(http.MethodPost, "/api/logout", nil, nil)
	c.setToken(roleStudent, "")
}

// ---------- Student data ----------

func (c *Client) Me() (*Me, error) {
	var m Me
	return &m, c.do(http.MethodGet, "/api/me", nil, &m)
}

func (c *Client) Updates() (*Updates, error) {
	var u Updates
	return &u, c.do(http.MethodGet, "/api/me/announcements", nil, &u)
}

func (c *Client) MarkUpdatesSeen() error {
	return c.do(http.MethodPost, "/api/me/announcements/seen", nil, nil)
}

func (c *Client) SendClaim(in ClaimIn) error {
	return c.do(http.MethodPost, "/api/me/claims", in, nil)
}

func (c *Client) UpdateProfile(name, phone string) error {
	return c.do(http.MethodPut, "/api/me", map[string]string{"name": name, "phone": phone}, nil)
}

func (c *Client) ChangePassword(current, next string) error {
	return c.do(http.MethodPost, "/api/me/password", map[string]string{"current_password": current, "new_password": next}, nil)
}

// Receipt downloads the PDF receipt for one approved payment.
func (c *Client) Receipt(id int64) ([]byte, error) {
	return c.download(fmt.Sprintf("/api/me/receipts/%d.pdf", id))
}

func (c *Client) download(path string) ([]byte, error) {
	resp, err := c.request(http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusUnauthorized {
		c.setToken(roleFor(path), "")
		return nil, ErrUnauthorized
	}
	if resp.StatusCode >= 400 {
		return nil, errors.New(errorText(data, resp.StatusCode))
	}
	return data, nil
}

// Image downloads and decodes a public image (course thumbnail or uploaded QR).
func (c *Client) Image(path string) (image.Image, error) {
	resp, err := c.request(http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("image: %d", resp.StatusCode)
	}
	img, _, err := image.Decode(resp.Body)
	return img, err
}

// ---------- Models (same JSON as the website uses) ----------

type Me struct {
	Student  Student   `json:"student"`
	Courses  []Course  `json:"courses"`
	Payment  Payment   `json:"payment"`
	Unread   int       `json:"unread_announcements"`
	Receipts []Receipt `json:"receipts"`
}

type Student struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	CreatedAt int64  `json:"created_at"`
}

type Course struct {
	ID                 int64    `json:"id"`
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	Features           []string `json:"features"`
	Price              float64  `json:"price"`
	DefaultDays        int      `json:"default_days"`
	HasThumbnail       bool     `json:"has_thumbnail"`
	ThumbnailUpdatedAt int64    `json:"thumbnail_updated_at"`
	Access             Access   `json:"access"`
	PendingClaim       *Claim   `json:"pending_claim"`
	IsOpen             bool     `json:"is_open"`
	ClassroomLink      string   `json:"classroom_link"`
}

func (c Course) ThumbnailPath() string {
	if !c.HasThumbnail {
		return ""
	}
	return fmt.Sprintf("/api/courses/%d/thumbnail?v=%d", c.ID, c.ThumbnailUpdatedAt)
}

// Access.Status is "active", "scheduled", "expired" or "none".
type Access struct {
	Status   string `json:"status"`
	Since    int64  `json:"since"`
	Until    int64  `json:"until"`
	DaysLeft int    `json:"days_left"`
}

type Claim struct {
	QRSlot     string   `json:"qr_slot"`
	PayerName  string   `json:"payer_name"`
	Method     string   `json:"method"`
	Amount     *float64 `json:"amount"`
	PaidOn     string   `json:"paid_on"`
	Reference  string   `json:"reference"`
	Age        *int     `json:"age"`
	ReferredBy string   `json:"referred_by"`
	CreatedAt  int64    `json:"created_at"`
}

type ClaimIn struct {
	CourseID   int64    `json:"course_id"`
	QRSlot     string   `json:"qr_slot"`
	PayerName  string   `json:"payer_name"`
	Method     string   `json:"method"`
	Amount     *float64 `json:"amount"`
	PaidOn     string   `json:"paid_on"`
	Reference  string   `json:"reference"`
	Age        *int     `json:"age"`
	ReferredBy string   `json:"referred_by"`
}

type Payment struct {
	ContactPhone string   `json:"contact_phone"`
	Methods      []Method `json:"methods"`
}

type Method struct {
	Slot      string `json:"slot"`
	Label     string `json:"label"`
	UPIID     string `json:"upi_id"`
	PayeeName string `json:"payee_name"`
	HasImage  bool   `json:"has_image"`
	UpdatedAt int64  `json:"updated_at"`
}

func (m Method) ImagePath() string {
	return fmt.Sprintf("/api/payment-methods/%s/image?v=%d", m.Slot, m.UpdatedAt)
}

type Receipt struct {
	ID          int64   `json:"id"`
	ReceiptNo   string  `json:"receipt_no"`
	CourseID    int64   `json:"course_id"`
	CourseTitle string  `json:"course_title"`
	AmountPaid  float64 `json:"amount_paid"`
	GrantedAt   int64   `json:"granted_at"`
	StartAt     int64   `json:"start_at"`
	EndAt       int64   `json:"end_at"`
	RevokedAt   *int64  `json:"revoked_at"`
}

type Updates struct {
	SeenAt int64    `json:"seen_at"`
	Items  []Update `json:"items"`
}

type Update struct {
	ID          int64   `json:"id"`
	Title       string  `json:"title"`
	Message     string  `json:"message"`
	LinkURL     string  `json:"link_url"`
	LinkLabel   string  `json:"link_label"`
	CourseTitle *string `json:"course_title"`
	CreatedAt   int64   `json:"created_at"`
	UpdatedAt   int64   `json:"updated_at"`
}
