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

## Sign-in (students and management)

Students and management sign in with a **mobile number and password**, or with **Google**. Accounts
made before mobile log-in can log in with their **email** instead (send it as `email`), and add a
mobile on their profile; Google sign-in links to them by email, so they keep all their data. On the
first start, older accounts whose saved phone is a valid mobile used by no other account can log in
with it. The admin has its own fixed login. `role` is `student` or `management`; each has its own
list of numbers. Mobiles are 10-digit Indian numbers (`+91` / `0` prefixes are accepted).

| Method | Path | Body / notes |
|---|---|---|
| GET | `/api/auth/config` | `{google_client_id}` (`""` when Google sign-in is not set up). |
| PUT | `/api/me/mobile`, `/api/management/me/mobile` | `{mobile}`: set or change the number the logged-in account logs in with. |
| POST | `/api/auth/google` | Website: `{credential, role, mobile?, invite_code?, client}` with the ID token from Google's button → `{ok, token?}`. Existing accounts are logged in straight away. `428` means the person is new: send the same token again with `mobile` (and, for management, `invite_code`). |
| GET | `/api/auth/google/start?role=&code=` | App: open in the browser. `code` is a random secret (32–128 chars) the app keeps. |
| GET | `/api/auth/google/callback` | Google's redirect (register `<API_PUBLIC_URL>/api/auth/google/callback` in Google Cloud). |
| GET | `/api/auth/google/result?code=` | App: poll every few seconds → `{status: pending \| ok \| need_details \| error, detail?, name?, token?}`. |
| POST | `/api/auth/google/complete` | App, after `need_details`: `{code, mobile, invite_code?}` → `{status: "ok", token}`. Creates the new account. |

Accounts include `mobile` (or `null`), `has_password` and `google`. Google accounts start with no
password; `POST .../me/password` then accepts an empty `current_password`.

## Student

| Method | Path | Body / notes |
|---|---|---|
| POST | `/api/register` | `{name, mobile, password, client}` → `{ok, token?}` |
| POST | `/api/login` | `{mobile, password, client}`, or `{email, password, client}` for older accounts → `{ok, token?}` |
| POST | `/api/logout` | Ends the current session. |
| GET | `/api/me` | Student, courses with `access` and `pending_claim`, payment options, `unread_announcements`. |
| PUT | `/api/me` | `{name, phone}` |
| POST | `/api/me/password` | `{current_password, new_password}`. Logs out other devices. |
| GET | `/api/me/qr/{course_id}.svg?slot=primary` | UPI QR (SVG) with the course amount filled in. |
| GET | `/api/me/receipts/{enrollment_id}.pdf` | PDF receipt (own receipts only). `/api/me` lists them under `receipts`. |
| POST | `/api/me/claims` | "I have paid" note: `{course_id, pay_mode, qr_slot, payer_name, method, amount, paid_on, reference, age, referred_by, referral_id, screenshot}`. `pay_mode` is `upi` (default) or `cash`. For `upi`, `reference` (UTR) is required (6+ characters) and `screenshot` is a base64 PNG/JPEG/WEBP (max 2 MB; the website requires it). For `cash`, the QR, UTR and screenshot are ignored. `referral_id` must be a real Management ID (e.g. `GCM001`); `referred_by` and `referral_id` are required when the course has `referral_required`. |
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
| POST | `/api/management/register` | `{name, mobile, password, invite_code, client}` → `{ok, token?}` |
| POST | `/api/management/login` | `{mobile, password, client}` or `{email, password, client}` → `{ok, token?}` |
| POST | `/api/management/logout` | |
| GET | `/api/management/me` | `{manager: {id, manager_code, name, email, phone, created_at, can_post_updates}, courses[], unread_announcements}` |
| GET | `/api/management/me/announcements` | `{seen_at, items[]}`: updates the admin sent to the management team. |
| POST | `/api/management/me/announcements/seen` | Marks all updates as read. |
| POST | `/api/management/me/password` | `{current_password, new_password}`. Logs out other devices. |
| GET/POST | `/api/management/updates` | Only while the admin has turned on `can_post_updates` (else `403`). GET: `{items[], courses[], audience}` (the member's own updates). POST: same body as the admin's updates. |
| PUT/DELETE | `/api/management/updates/{id}` | Edit or delete one of the member's own updates (`403` while posting is off). |

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
| PUT | `/api/admin/students/{id}` | `{name, email, phone}`. `email` may be empty; a valid mobile in `phone` also becomes the student's log-in number. |
| POST | `/api/admin/students/{id}/password` | `{new_password}`. Logs the student out everywhere. |
| POST | `/api/admin/students/{id}/grant` | `{course_id, days, start: "after_current"\|"now", amount, note, claim_id?}` |
| GET | `/api/admin/enrollments/{id}/receipt.pdf` | PDF receipt for any access grant. |
| POST | `/api/admin/enrollments/{id}/revoke` | |
| GET | `/api/admin/payments` | All grants with amounts, `student_code`, `student_phone` and `pay_mode` (`upi`, `cash`, or `""` when not matched to a payment note). |
| GET | `/api/admin/claims?status=pending` | Payment notes (`pending`, `approved`, `dismissed`), with `pay_mode`, `referral_id`, `referral_name`, `has_screenshot`. |
| GET | `/api/admin/claims/{id}/screenshot` | The payment screenshot. Deleted 5 minutes after the payment is approved. |
| GET | `/api/admin/managers` | `{invite_code, items[]}`: management members with `approved` / `pending` referral counts. |
| PUT | `/api/admin/managers/invite-code` | `{invite_code}`: the code needed to register as management (empty closes registration). |
| POST | `/api/admin/managers/{id}/revoke` | Blocks the member's login, logs them out and stops their Referral ID working. |
| POST | `/api/admin/managers/{id}/restore` | Lets a revoked member log in again. |
| POST | `/api/admin/managers/{id}/password` | `{new_password}`: for members who forgot it. Logs them out everywhere. |
| PUT | `/api/admin/managers/{id}/updates-access` | `{enabled}`: lets the member post updates until it is turned off again. Updates they posted stay. |
| POST | `/api/admin/claims/{id}/dismiss` | |
| GET | `/api/admin/payment-methods` | Both QR slots. |
| PUT | `/api/admin/payment-methods/{slot}` | `{label, upi_id, payee_name, is_active}` |
| PUT | `/api/admin/payment-methods/{slot}/image` | `{data}`: base64 PNG/JPEG/WEBP, max 2 MB. |
| DELETE | `/api/admin/payment-methods/{slot}/image` | |
| GET | `/api/admin/payment-methods/{slot}/preview.svg` | QR made from the UPI ID. |
| GET/POST | `/api/admin/announcements` | `{title, message, link_url, link_label, to_management, to_all_students, course_ids[]}`: any mix of the management team, every paid student and the students of some courses (at least one). Older apps may send `course_id` instead. GET adds `audience_label`, `sent_by` (`""` for the admin, else the management member who posted it), `courses` and `audience` counts (`all`, `management`, per course id). |
| PUT/DELETE | `/api/admin/announcements/{id}` | |
| GET/POST | `/api/admin/courses` | `{title, description, features, price, default_days, classroom_link, is_active, referral_required}` |
| PUT | `/api/admin/courses/{id}` | Same body as POST; `features` is the highlights, one per line. Leaving out `referral_required` keeps it. |
| DELETE | `/api/admin/courses/{id}` | Refused with `409` if the course has payment records (hide it instead). |
| PUT | `/api/admin/courses/{id}/thumbnail` | `{data}`: base64 PNG/JPEG/WEBP, max 2 MB. |
| DELETE | `/api/admin/courses/{id}/thumbnail` | |
