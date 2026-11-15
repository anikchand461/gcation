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
| GET | `/api/contact` | `{contact_phone}`: the support number the admin set (shown in the website footer). |
| GET | `/api/courses` | Open courses: `id, title, description, features[], price, default_days, has_thumbnail, thumbnail_updated_at`. |
| GET | `/api/courses/{id}/thumbnail` | Course thumbnail image (when `has_thumbnail`); add `?v={thumbnail_updated_at}`. |
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
| GET | `/api/me/receipts/{enrollment_id}.pdf` | PDF receipt (own receipts only). `/api/me` lists them under `receipts`. |
| POST | `/api/me/claims` | "I have paid" note: `{course_id, qr_slot, payer_name, method, amount, paid_on, reference, age, referred_by, referral_id, screenshot}`. `reference` (UTR) is required (6+ characters). `referral_id` is optional but must be a real Management ID (e.g. `GCM001`). `screenshot` is a base64 PNG/JPEG/WEBP (max 2 MB); the website requires it. |
| GET | `/api/me/announcements` | `{seen_at, items[]}`. Only for students with active access. |
| POST | `/api/me/announcements/seen` | Marks all updates as read. |

`student.student_code` is the Student ID (e.g. `GCS001`). It is `null` until the student's first payment
is approved. (`student.id` is the internal account number and is not shown to people.)

`access.status` is one of `active` (with `since`, `until`, `days_left`), `scheduled`, `expired`, `none`.

For an uploaded QR (`has_image: true`), show `/api/payment-methods/{slot}/image?v={updated_at}`;
otherwise show `/api/me/qr/{course_id}.svg?slot={slot}`. To open a UPI app directly, build
`upi://pay?pa={upi_id}&pn={payee_name}&am={price}&cu=INR&tn=GCATION%20{student_code or email}`.

## Management team

Management members see the courses and prices and share their Management ID (`GCM001`), which students
enter as the Referral ID. They cannot enroll. Registering needs the referral code the admin sets.

| Method | Path | Body / notes |
|---|---|---|
| POST | `/api/management/register` | `{name, email, phone, password, invite_code, client}` → `{ok, token?}` |
| POST | `/api/management/login` | `{email, password, client}` → `{ok, token?}` |
| POST | `/api/management/logout` | |
| GET | `/api/management/me` | `{manager: {id, manager_code, name, email, phone, created_at}, courses[], unread_announcements}` |
| GET | `/api/management/me/announcements` | `{seen_at, items[]}`: updates the admin sent to the management team. |
| POST | `/api/management/me/announcements/seen` | Marks all updates as read. |
| POST | `/api/management/me/password` | `{current_password, new_password}`. Logs out other devices. |

## Admin

All need an admin session. Login: `POST /api/admin/login` with `{username, password, client}`.

| Method | Path | Body / notes |
|---|---|---|
| POST | `/api/admin/logout` | |
| GET/PUT | `/api/admin/settings` | `{contact_phone}`: the support number shown to students and on receipts. |
| GET | `/api/admin/account` | Admin username and when it last changed. |
| PUT | `/api/admin/account` | `{current_password, username, new_password}` (empty `new_password` keeps it). Logs out other admin devices. |
| GET | `/api/admin/overview` | Counts and money totals. |
| GET | `/api/admin/students?q=` | Search by name, email, phone or Student ID (`GCS001` or `1`). |
| GET | `/api/admin/students/{id}` | Detail: access per course, grants, payment notes. |
| PUT | `/api/admin/students/{id}` | `{name, email, phone}` |
| POST | `/api/admin/students/{id}/password` | `{new_password}`. Logs the student out everywhere. |
| POST | `/api/admin/students/{id}/grant` | `{course_id, days, start: "after_current"\|"now", amount, note, claim_id?}` |
| GET | `/api/admin/enrollments/{id}/receipt.pdf` | PDF receipt for any access grant. |
| POST | `/api/admin/enrollments/{id}/revoke` | |
| GET | `/api/admin/payments` | All grants with amounts. |
| GET | `/api/admin/claims?status=pending` | Payment notes (`pending`, `approved`, `dismissed`), with `referral_id`, `referral_name`, `has_screenshot`. |
| GET | `/api/admin/claims/{id}/screenshot` | The payment screenshot. Deleted 5 minutes after the payment is approved. |
| GET | `/api/admin/managers` | `{invite_code, items[]}`: management members with `approved` / `pending` referral counts. |
| PUT | `/api/admin/managers/invite-code` | `{invite_code}`: the code needed to register as management (empty closes registration). |
| POST | `/api/admin/managers/{id}/revoke` | Blocks the member's login, logs them out and stops their Referral ID working. |
| POST | `/api/admin/managers/{id}/restore` | Lets a revoked member log in again. |
| POST | `/api/admin/claims/{id}/dismiss` | |
| GET | `/api/admin/payment-methods` | Both QR slots. |
| PUT | `/api/admin/payment-methods/{slot}` | `{label, upi_id, payee_name, is_active}` |
| PUT | `/api/admin/payment-methods/{slot}/image` | `{data}`: base64 PNG/JPEG/WEBP, max 2 MB. |
| DELETE | `/api/admin/payment-methods/{slot}/image` | |
| GET | `/api/admin/payment-methods/{slot}/preview.svg` | QR made from the UPI ID. |
| GET/POST | `/api/admin/announcements` | `{title, message, link_url, link_label, to_management, to_all_students, course_ids[]}`: any mix of the management team, every paid student and the students of some courses (at least one). Older apps may send `course_id` instead. GET adds `audience_label` and `audience` counts (`all`, `management`, per course id). |
| PUT/DELETE | `/api/admin/announcements/{id}` | |
| GET/POST | `/api/admin/courses` | `{title, description, features, price, default_days, classroom_link, is_active}` |
| PUT | `/api/admin/courses/{id}` | Same body as POST; `features` is the highlights, one per line. |
| DELETE | `/api/admin/courses/{id}` | Refused with `409` if the course has payment records (hide it instead). |
| PUT | `/api/admin/courses/{id}/thumbnail` | `{data}`: base64 PNG/JPEG/WEBP, max 2 MB. |
| DELETE | `/api/admin/courses/{id}/thumbnail` | |
