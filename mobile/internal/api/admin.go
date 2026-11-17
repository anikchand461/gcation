package api

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
)

// ---------- Admin auth ----------

func (c *Client) AdminLogin(username, password string) error {
	var out struct{ Token string }
	err := c.do(http.MethodPost, "/api/admin/login", map[string]string{"username": username, "password": password, "client": "app"}, &out)
	if err == nil {
		c.setToken(roleAdmin, out.Token)
	}
	return err
}

func (c *Client) AdminLogout() {
	_ = c.do(http.MethodPost, "/api/admin/logout", nil, nil)
	c.setToken(roleAdmin, "")
}

// ---------- Admin data ----------

type Overview struct {
	Students           int     `json:"students"`
	ActiveStudents     int     `json:"active_students"`
	PendingClaims      int     `json:"pending_claims"`
	ExpiringSoon       int     `json:"expiring_soon"`
	TotalCollected     float64 `json:"total_collected"`
	CollectedThisMonth float64 `json:"collected_this_month"`
}

// StudentRow is one student in the admin list. Access holds only courses they ever had.
type StudentRow struct {
	Student
	Access []struct {
		CourseID int64  `json:"course_id"`
		Title    string `json:"title"`
		Access
	} `json:"access"`
	PendingClaims int `json:"pending_claims"`
}

type StudentDetail struct {
	Student Student `json:"student"`
	Courses []struct {
		ID          int64   `json:"id"`
		Title       string  `json:"title"`
		Price       float64 `json:"price"`
		DefaultDays int     `json:"default_days"`
		Access      Access  `json:"access"`
	} `json:"courses"`
	Enrollments []Enrollment `json:"enrollments"`
	Claims      []ClaimRow   `json:"claims"`
}

type Enrollment struct {
	ID          int64   `json:"id"`
	StudentID   int64   `json:"student_id"`
	CourseID    int64   `json:"course_id"`
	CourseTitle string  `json:"course_title"`
	StartAt     int64   `json:"start_at"`
	EndAt       int64   `json:"end_at"`
	AmountPaid  float64 `json:"amount_paid"`
	Note        string  `json:"note"`
	GrantedAt   int64   `json:"granted_at"`
	RevokedAt   *int64  `json:"revoked_at"`
	ReceiptNo   string  `json:"receipt_no"`
	// Present in the payments list:
	StudentName  string  `json:"student_name"`
	StudentEmail string  `json:"student_email"`
	StudentPhone string  `json:"student_phone"`
	StudentCode  *string `json:"student_code"`
	PayMode      string  `json:"pay_mode"` // "upi", "cash", or "" when not matched to a payment note
}

type ClaimRow struct {
	ID            int64    `json:"id"`
	StudentID     int64    `json:"student_id"`
	CourseID      int64    `json:"course_id"`
	CourseTitle   string   `json:"course_title"`
	PayMode       string   `json:"pay_mode"` // "upi" or "cash"
	QRSlot        string   `json:"qr_slot"`
	PayerName     string   `json:"payer_name"`
	Method        string   `json:"method"`
	Amount        *float64 `json:"amount"`
	PaidOn        string   `json:"paid_on"`
	Reference     string   `json:"reference"`
	Age           *int     `json:"age"`
	ReferredBy    string   `json:"referred_by"`
	ReferralID    string   `json:"referral_id"`
	ReferralName  *string  `json:"referral_name"`
	HasScreenshot bool     `json:"has_screenshot"`
	Status        string   `json:"status"`
	CreatedAt     int64    `json:"created_at"`
	StudentName   string   `json:"student_name"`
	StudentPhone  string   `json:"student_phone"`
	StudentCode   *string  `json:"student_code"`
}

// ScreenshotPath is where the admin fetches the payment screenshot (deleted 5 minutes after approval).
func (c ClaimRow) ScreenshotPath() string {
	return fmt.Sprintf("/api/admin/claims/%d/screenshot", c.ID)
}

type GrantIn struct {
	CourseID int64   `json:"course_id"`
	Days     int     `json:"days"`
	Start    string  `json:"start"` // "after_current" or "now"
	Amount   float64 `json:"amount"`
	Note     string  `json:"note"`
	ClaimID  *int64  `json:"claim_id"`
}

type Announcement struct {
	ID            int64   `json:"id"`
	Title         string  `json:"title"`
	Message       string  `json:"message"`
	LinkURL       string  `json:"link_url"`
	LinkLabel     string  `json:"link_label"`
	ToManagement  bool    `json:"to_management"`
	ToAllStudents bool    `json:"to_all_students"`
	CourseIDs     []int64 `json:"course_ids"`
	AudienceLabel string  `json:"audience_label"` // e.g. "Management · All paid students"
	SentBy        string  `json:"sent_by"`        // "" for the admin, e.g. "Riya Sen (GCM002)" for a management member
	CreatedAt     int64   `json:"created_at"`
	UpdatedAt     int64   `json:"updated_at"`
}

// AnnouncementIn says who sees an update: any mix of the management team, every paid student
// and the students of some courses.
type AnnouncementIn struct {
	Title         string  `json:"title"`
	Message       string  `json:"message"`
	LinkURL       string  `json:"link_url"`
	LinkLabel     string  `json:"link_label"`
	ToManagement  bool    `json:"to_management"`
	ToAllStudents bool    `json:"to_all_students"`
	CourseIDs     []int64 `json:"course_ids"`
}

type AdminCourse struct {
	ID               int64   `json:"id"`
	Title            string  `json:"title"`
	Description      string  `json:"description"`
	Features         string  `json:"features"` // one per line
	Price            float64 `json:"price"`
	DefaultDays      int     `json:"default_days"`
	ClassroomLink    string  `json:"classroom_link"`
	IsActive         bool    `json:"is_active"`
	ReferralRequired bool    `json:"referral_required"`
	HasThumbnail     bool    `json:"has_thumbnail"`
	ThumbnailAt      int64   `json:"thumbnail_updated_at"`
}

type CourseIn struct {
	Title            string  `json:"title"`
	Description      string  `json:"description"`
	Features         string  `json:"features"`
	Price            float64 `json:"price"`
	DefaultDays      int     `json:"default_days"`
	ClassroomLink    string  `json:"classroom_link"`
	IsActive         bool    `json:"is_active"`
	ReferralRequired bool    `json:"referral_required"`
}

func (a AdminCourse) In() CourseIn {
	return CourseIn{a.Title, a.Description, a.Features, a.Price, a.DefaultDays, a.ClassroomLink, a.IsActive, a.ReferralRequired}
}

type AdminMethod struct {
	Method
	IsActive bool `json:"is_active"`
}

type MethodIn struct {
	Label     string `json:"label"`
	UPIID     string `json:"upi_id"`
	PayeeName string `json:"payee_name"`
	IsActive  bool   `json:"is_active"`
}

func (c *Client) Overview() (*Overview, error) {
	var o Overview
	return &o, c.do(http.MethodGet, "/api/admin/overview", nil, &o)
}

func (c *Client) Students(q string) ([]StudentRow, error) {
	var out []StudentRow
	return out, c.do(http.MethodGet, "/api/admin/students?q="+url.QueryEscape(q), nil, &out)
}

func (c *Client) StudentDetail(id int64) (*StudentDetail, error) {
	var d StudentDetail
	return &d, c.do(http.MethodGet, fmt.Sprintf("/api/admin/students/%d", id), nil, &d)
}

func (c *Client) EditStudent(id int64, name, email, phone string) error {
	return c.do(http.MethodPut, fmt.Sprintf("/api/admin/students/%d", id), map[string]string{"name": name, "email": email, "phone": phone}, nil)
}

func (c *Client) ResetStudentPassword(id int64, pw string) error {
	return c.do(http.MethodPost, fmt.Sprintf("/api/admin/students/%d/password", id), map[string]string{"new_password": pw}, nil)
}

func (c *Client) Grant(studentID int64, in GrantIn) error {
	return c.do(http.MethodPost, fmt.Sprintf("/api/admin/students/%d/grant", studentID), in, nil)
}

func (c *Client) Revoke(enrollmentID int64) error {
	return c.do(http.MethodPost, fmt.Sprintf("/api/admin/enrollments/%d/revoke", enrollmentID), nil, nil)
}

func (c *Client) AdminReceipt(enrollmentID int64) ([]byte, error) {
	return c.download(fmt.Sprintf("/api/admin/enrollments/%d/receipt.pdf", enrollmentID))
}

func (c *Client) Claims() ([]ClaimRow, error) {
	var out []ClaimRow
	return out, c.do(http.MethodGet, "/api/admin/claims?status=pending", nil, &out)
}

func (c *Client) DismissClaim(id int64) error {
	return c.do(http.MethodPost, fmt.Sprintf("/api/admin/claims/%d/dismiss", id), nil, nil)
}

func (c *Client) Payments() ([]Enrollment, error) {
	var out []Enrollment
	return out, c.do(http.MethodGet, "/api/admin/payments", nil, &out)
}

// Where updates are posted: the admin's updates, or a management member's own (while the admin allows it).
const (
	AdminUpdates   = "/api/admin/announcements"
	ManagerUpdates = "/api/management/updates"
)

// UpdateCourse is a course an update can be sent to.
type UpdateCourse struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
}

// PostedUpdates lists the updates at base (AdminUpdates or ManagerUpdates), the courses they can
// go to, and how many people each audience reaches.
func (c *Client) PostedUpdates(base string) ([]Announcement, []UpdateCourse, map[string]int, error) {
	var out struct {
		Items    []Announcement `json:"items"`
		Courses  []UpdateCourse `json:"courses"`
		Audience map[string]int `json:"audience"`
	}
	err := c.do(http.MethodGet, base, nil, &out)
	return out.Items, out.Courses, out.Audience, err
}

func (c *Client) SaveUpdate(base string, id int64, in AnnouncementIn) error {
	if id == 0 {
		return c.do(http.MethodPost, base, in, nil)
	}
	return c.do(http.MethodPut, fmt.Sprintf("%s/%d", base, id), in, nil)
}

func (c *Client) DeleteUpdate(base string, id int64) error {
	return c.do(http.MethodDelete, fmt.Sprintf("%s/%d", base, id), nil, nil)
}

func (c *Client) AdminCourses() ([]AdminCourse, error) {
	var out []AdminCourse
	return out, c.do(http.MethodGet, "/api/admin/courses", nil, &out)
}

func (c *Client) SaveCourse(id int64, in CourseIn) error {
	if id == 0 {
		return c.do(http.MethodPost, "/api/admin/courses", in, nil)
	}
	return c.do(http.MethodPut, fmt.Sprintf("/api/admin/courses/%d", id), in, nil)
}

func (c *Client) DeleteCourse(id int64) error {
	return c.do(http.MethodDelete, fmt.Sprintf("/api/admin/courses/%d", id), nil, nil)
}

func (c *Client) PaymentMethods() ([]AdminMethod, error) {
	var out []AdminMethod
	return out, c.do(http.MethodGet, "/api/admin/payment-methods", nil, &out)
}

func (c *Client) SaveMethod(slot string, in MethodIn) error {
	return c.do(http.MethodPut, "/api/admin/payment-methods/"+slot, in, nil)
}

// UploadQR sends a PNG/JPEG/WEBP image (max 2 MB) as the QR for slot.
func (c *Client) UploadQR(slot string, image []byte) error {
	return c.do(http.MethodPut, "/api/admin/payment-methods/"+slot+"/image",
		map[string]string{"data": base64.StdEncoding.EncodeToString(image)}, nil)
}

func (c *Client) RemoveQR(slot string) error {
	return c.do(http.MethodDelete, "/api/admin/payment-methods/"+slot+"/image", nil, nil)
}

func (c *Client) SupportPhone() (string, error) {
	var out struct {
		ContactPhone string `json:"contact_phone"`
	}
	err := c.do(http.MethodGet, "/api/admin/settings", nil, &out)
	return out.ContactPhone, err
}

func (c *Client) SaveSupportPhone(phone string) error {
	return c.do(http.MethodPut, "/api/admin/settings", map[string]string{"contact_phone": phone}, nil)
}

func (c *Client) AdminAccount() (string, error) {
	var out struct {
		Username string `json:"username"`
	}
	err := c.do(http.MethodGet, "/api/admin/account", nil, &out)
	return out.Username, err
}

func (c *Client) SaveAdminAccount(current, username, newPassword string) error {
	return c.do(http.MethodPut, "/api/admin/account", map[string]string{
		"current_password": current, "username": username, "new_password": newPassword,
	}, nil)
}
