# G-CATION

Online education platform for G-CATION. Students register, pay for a course by scanning a UPI QR code,
and get access for a fixed period once the admin verifies the payment. No payment gateway is used.

- **Backend:** FastAPI (Python) with a Neon Postgres database
- **Frontend:** plain HTML, CSS and JavaScript (no build step)

## Features

**Students**
- Register and log in with email and password; edit their profile and change their password
- Pay with a **primary** or **secondary** UPI QR (switch if one fails), or tap "Pay with UPI app" on phones
- Send payment details (payer, app/bank, UTR) and call or WhatsApp the team to confirm
- See active courses, days left, the classroom link, and updates (Google Meet, community, instructions)
  with a NEW badge, unread count and optional browser alerts

**Admin** (one fixed login, no sign-up)
- Overview of what needs attention: payments to verify, courses expiring soon, recent payments
- Students: search and filter, give access for any period, extend or revoke it, edit details, reset passwords
- Payment notes, payment history with CSV export
- Updates for paid students, both payment QR codes (upload an image or use a UPI ID), courses and prices

## Project structure

```
gcation/
├── backend/
│   ├── app/
│   │   ├── main.py          App setup, CORS middleware (allowed frontend addresses)
│   │   ├── config.py        Settings read from .env
│   │   ├── db.py            Neon connection pool and table definitions
│   │   ├── auth.py          Passwords, sessions, cookies and Bearer tokens
│   │   ├── access.py        Course access rules
│   │   ├── schemas.py       Request validation
│   │   └── routers/         public.py, student.py, admin.py
│   ├── API.md               Endpoint reference (also for the future Go + Gio app)
│   ├── requirements.txt
│   └── .env.example         Copy to .env and fill in
├── frontend/
│   ├── index.html           Public website
│   ├── login.html           Student login / register
│   ├── dashboard.html       Student app
│   ├── admin-login.html     Admin login
│   ├── admin.html           Admin app
│   ├── css/style.css
│   ├── js/config.js         Backend address (API_BASE)
│   └── js/                  common.js, dashboard.js, admin.js
└── run.sh                   Starts backend + frontend together
```

## Setup

You need **Python 3.11+** and a free **[Neon](https://neon.tech)** Postgres database
(region: AWS Asia Pacific 1 Singapore; only the Postgres service is needed).

1. Create the settings file:
   ```bash
   cp backend/.env.example backend/.env
   ```
2. Fill in `backend/.env`:

   | Key | What to put |
   |---|---|
   | `DATABASE_URL` | Neon dashboard → **Connect** → copy the connection string |
   | `ADMIN_USERNAME`, `ADMIN_PASSWORD` | The admin login (password at least 8 characters) |
   | `UPI_ID`, `UPI_PAYEE_NAME` | Fills the Primary QR the first time only. Manage QRs later in Admin → Payment QR |
   | `CONTACT_PHONE` | Number students call after paying |
   | `COOKIE_SECURE` | `false` locally, `true` on HTTPS |

3. Run it:
   ```bash
   ./run.sh
   ```
   The first run creates the Python environment and installs packages. Tables are created in Neon
   automatically on first start.

   - Website: http://127.0.0.1:5500
   - Admin: http://127.0.0.1:5500/admin-login.html

Open the site at **127.0.0.1**, not `localhost`. Browsers treat them as different sites and the login
cookie would not be sent (pages opened on `localhost` redirect automatically).

<details>
<summary>Running the two servers by hand instead</summary>

```bash
# Terminal 1: backend on port 8000
cd backend
python3 -m venv .venv
.venv/bin/pip install -r requirements.txt
.venv/bin/uvicorn app.main:app --reload --port 8000

# Terminal 2: frontend on port 5500
cd frontend
python3 -m http.server 5500
```
</details>

## How payment works

1. A student registers and opens a course from **My courses**.
2. They scan the UPI QR (amount and Student ID pre-filled) and pay.
3. They send their payment details and call the admin.
4. The admin checks the bank account, opens the payment note and clicks **Give access**,
   choosing the period (1, 3, 6, 12 months or any number of days).
5. The course unlocks right away and locks again when the period ends. Renewals add time after
   the current end date.

## How frontend and backend connect

They run as separate servers:
- `frontend/js/config.js` holds the backend address in `API_BASE`.
- `backend/app/main.py` lists the frontend addresses allowed to call the API in the CORS middleware
  (`allow_origins`). Every other website is blocked.

## Going live

1. Host the frontend and the API on the same main domain, e.g. `gcation.in` and `api.gcation.in`,
   so login cookies keep working. Put the API server in the **Singapore** region, next to Neon.
2. Set `API_BASE` in `frontend/js/config.js` to the API address, e.g. `https://api.gcation.in`.
3. Add the site address, e.g. `https://gcation.in`, to `allow_origins` in `backend/app/main.py`.
4. Serve both over HTTPS and set `COOKIE_SECURE=true` in the server's `.env`.
5. Start the API with `uvicorn app.main:app --host 0.0.0.0 --port 8000`. All data is in Neon,
   so the server keeps no files.

## Security notes

- `backend/.env` holds the database password and admin login. It is git-ignored; never commit it
  and never put `DATABASE_URL` in the frontend.
- Passwords are hashed with scrypt. Repeated wrong logins are blocked for 15 minutes.
- Every write request must be JSON, which blocks cross-site form attacks; CORS allows only the
  listed frontend addresses.

## Native app

A Go + Gio app can use the same backend: log in with `"client": "app"` to receive a token and send it as
`Authorization: Bearer <token>`. See [`backend/API.md`](backend/API.md).
