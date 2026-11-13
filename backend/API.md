# G-CATION API

Base URL: your backend address, e.g. `https://api.gcation.in` (locally `http://127.0.0.1:8000`).
All request and response bodies are JSON. Times are Unix seconds. Money is in rupees.
Errors return an HTTP status with `{"detail": "..."}` (a string, or a list of field errors for 422).

## Authentication

The website uses login cookies. **Native apps (Go + Gio) should use tokens:**

1. Log in with `"client": "app"` in the body. The response includes `"token"`.
2. Send it on every request: `Authorization: Bearer <token>`.
3. Store it securely on the device. Student tokens last 30 days, admin tokens 12 hours.
   A `401` means the token expired or was revoked: send the user back to login.

Every `POST`/`PUT`/`DELETE` must send `Content-Type: application/json` (an empty body is `{}`).

```go
req, _ := http.NewRequest("GET", base+"/api/me", nil)
req.Header.Set("Authorization", "Bearer "+token)
```

## Public

| Method | Path | Notes |
|---|---|---|
| GET | `/` | Health check. |
| GET | `/api/courses` | Open courses: `id, title, description, price, default_days`. |
| GET | `/api/payment-methods/{slot}/image` | Uploaded QR image (`slot` = `primary` or `secondary`). |

## Student

| Method | Path | Body / notes |
|---|---|---|
| POST | `/api/register` | `{name, email, phone, password, client}` → `{ok, token?}` |
| POST | `/api/login` | `{email, password, client}` → `{ok, token?}` |
| POST | `/api/logout` | Ends the current session. |
| GET | `/api/me` | Student, courses with `access` and `pending_claim`, payment options, `unread_announcements`. |
| PUT | `/api/me` | `{name, phone}` |
| POST | `/api/me/password` | `{current_password, new_password}`. Logs out other devices. |
| GET | `/api/me/qr/{course_id}.svg?slot=primary` | UPI QR (SVG) with the course amount filled in. |
| POST | `/api/me/claims` | "I have paid" note: `{course_id, qr_slot, payer_name, method, amount, paid_on, reference}` |
| GET | `/api/me/announcements` | `{seen_at, items[]}`. Only for students with active access. |
| POST | `/api/me/announcements/seen` | Marks all updates as read. |

`access.status` is one of `active` (with `since`, `until`, `days_left`), `scheduled`, `expired`, `none`.

For an uploaded QR (`has_image: true`), show `/api/payment-methods/{slot}/image?v={updated_at}`;
otherwise show `/api/me/qr/{course_id}.svg?slot={slot}`. To open a UPI app directly, build
`upi://pay?pa={upi_id}&pn={payee_name}&am={price}&cu=INR&tn=GCATION%20ID%20{student_id}`.

## Admin

All need an admin session. Login: `POST /api/admin/login` with `{username, password, client}`.

| Method | Path | Body / notes |
|---|---|---|
| POST | `/api/admin/logout` | |
| GET | `/api/admin/overview` | Counts and money totals. |
| GET | `/api/admin/students?q=` | Search by name, email, phone or ID. |
| GET | `/api/admin/students/{id}` | Detail: access per course, grants, payment notes. |
| PUT | `/api/admin/students/{id}` | `{name, email, phone}` |
| POST | `/api/admin/students/{id}/password` | `{new_password}`. Logs the student out everywhere. |
| POST | `/api/admin/students/{id}/grant` | `{course_id, days, start: "after_current"\|"now", amount, note, claim_id?}` |
| POST | `/api/admin/enrollments/{id}/revoke` | |
| GET | `/api/admin/payments` | All grants with amounts. |
| GET | `/api/admin/claims?status=pending` | Payment notes (`pending`, `approved`, `dismissed`). |
| POST | `/api/admin/claims/{id}/dismiss` | |
| GET | `/api/admin/payment-methods` | Both QR slots. |
| PUT | `/api/admin/payment-methods/{slot}` | `{label, upi_id, payee_name, is_active}` |
| PUT | `/api/admin/payment-methods/{slot}/image` | `{data}`: base64 PNG/JPEG/WEBP, max 2 MB. |
| DELETE | `/api/admin/payment-methods/{slot}/image` | |
| GET | `/api/admin/payment-methods/{slot}/preview.svg` | QR made from the UPI ID. |
| GET/POST | `/api/admin/announcements` | `{title, message, link_url, link_label, course_id?}` |
| PUT/DELETE | `/api/admin/announcements/{id}` | |
| GET/POST | `/api/admin/courses` | `{title, description, price, default_days, classroom_link, is_active}` |
| PUT | `/api/admin/courses/{id}` | |
