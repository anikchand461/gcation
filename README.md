<p align="center">
  <img src="frontend/assets/logo.png" alt="G-CATION logo" width="160">
</p>

<h1 align="center">G-CATION</h1>

<p align="center"><b>Learn. Grow. Succeed.</b><br>
An online education platform where students join courses, pay with UPI and learn with live classes,
community and updates, all in one place.</p>

<p align="center">
  <a href="https://gcation.vercel.app"><img src="https://img.shields.io/badge/%E2%96%B2%20Live%20website-gcation.vercel.app-7c3aed?style=for-the-badge&logo=vercel&logoColor=white&labelColor=000000" alt="Live website: gcation.vercel.app"></a>
  <a href="https://gcation.onrender.com"><img src="https://img.shields.io/badge/API-gcation.onrender.com-46E3B7?style=for-the-badge&logo=render&logoColor=white&labelColor=000000" alt="API: gcation.onrender.com"></a>
  <a href="https://github.com/anikchand461/gcation"><img src="https://img.shields.io/badge/GitHub-anikchand461%2Fgcation-24292f?style=for-the-badge&logo=github&logoColor=white&labelColor=000000" alt="GitHub repository"></a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/FastAPI-009688?style=flat-square&logo=fastapi&logoColor=white" alt="FastAPI">
  <img src="https://img.shields.io/badge/Python-3.12-3776AB?style=flat-square&logo=python&logoColor=white" alt="Python">
  <img src="https://img.shields.io/badge/Postgres-Neon-00E599?style=flat-square&logo=postgresql&logoColor=white" alt="Neon Postgres">
  <img src="https://img.shields.io/badge/JavaScript-vanilla-F7DF1E?style=flat-square&logo=javascript&logoColor=black" alt="Vanilla JavaScript">
  <img src="https://img.shields.io/badge/Payments-UPI%20QR-5f259f?style=flat-square" alt="UPI QR payments">
</p>

<p align="center"><a href="https://gcation.vercel.app"><b>→ Open the website</b></a></p>

---

G-CATION runs any kind of course: academic subjects, languages, skills, test preparation,
bootcamps and more. Students register, choose a course, pay by scanning a UPI QR code, and get access
for a fixed period once the admin verifies the payment. A receipt is created automatically for every
approved payment. No payment gateway is needed.

- **Backend:** FastAPI (Python) with a Neon Postgres database
- **Frontend:** plain HTML, CSS and JavaScript (no build step), light and dark themes, works on phones and laptops

## Features

**Students**
- Register and log in; edit their profile and change their password
- Browse courses with thumbnails, highlights, price and duration
- Pay with a **primary** or **secondary** UPI QR (switch if one fails), or tap "Pay with UPI app" on phones
- Send payment details (payer, app/bank, UTR) and call or WhatsApp the team to confirm
- Follow verification on a status timeline; the course unlocks as soon as it is approved
- See active courses and days left, open the classroom, and read updates (Google Meet links,
  community links, instructions) with a NEW badge, unread count and optional browser alerts
- **Download a PDF receipt** for every approved payment

**Admin** (one login, no sign-up)
- Overview of what needs attention: payments to verify, courses expiring soon, recent payments
- Students: search and filter, give access for any period, extend or revoke it, edit details, reset passwords
- Payment notes, payment history with CSV export, and **PDF receipts** for every payment
- Updates for paid students (all students or one course)
- Both payment QR codes: upload an image or use a UPI ID, change any time
- Courses: thumbnail, highlights, price and period; hide or show in one click; delete courses without payments
- Change the admin username and password from **Admin account**

**Everyone**
- Light and dark themes (follows the device until you pick one with the sun/moon button)
- Logo loading screen, smooth page transitions, layouts for phone and laptop

## Receipts

When the admin gives a student access (**Give access**), that payment gets a receipt automatically.
It is built from the saved payment record with fixed rules (no AI or outside service) and contains:
receipt number (`GC-<year>-<number>`), date, student details, course, access period, amount paid,
payment details (bank, payer, UTR) and a PAID stamp, or ACCESS REVOKED if the access was taken back later.

- Students: **Profile → Payment receipts**, or the **Receipt** button on a paid course
- Admin: **Payments** page, or a student's **Access & payment history**

The PDF uses standard fonts, so names should be written in English letters.

## Project structure

```
gcation/
├── backend/
│   ├── app/
│   │   ├── main.py          App setup, CORS middleware (allowed frontend addresses)
│   │   ├── config.py        Settings read from .env
│   │   ├── db.py            Neon connection pool and table definitions
│   │   ├── auth.py          Sessions, cookies and Bearer tokens
│   │   ├── passwords.py     Password hashing
│   │   ├── access.py        Course access rules
│   │   ├── receipts.py      PDF receipts
│   │   ├── images.py        Image upload checks (QR codes, thumbnails)
│   │   ├── reset_admin.py   Resets the admin login from .env
│   │   ├── schemas.py       Request validation
│   │   ├── assets/logo.png  Logo used on receipts
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
│   ├── assets/              logo.png (original), logo-mark.png (trimmed, used on the site)
│   ├── css/style.css        Design system, light + dark themes
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
   | `ADMIN_USERNAME`, `ADMIN_PASSWORD` | The first admin login (password at least 8 characters). After the first start, change it in Admin → Admin account |
   | `UPI_ID`, `UPI_PAYEE_NAME`, `CONTACT_PHONE` | Optional. Only fill the first QR and support number on a brand-new database. Manage them later in Admin → Payment QR |
   | `COOKIE_SECURE` | `false` locally, `true` on HTTPS |

3. Run it:
   ```bash
   ./run.sh
   ```
   The first run creates the Python environment and installs packages. Tables are created in Neon
   automatically on first start, and existing databases are upgraded in place.

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

## Forgot the admin password?

On the server, in the `backend` folder, run:
```bash
.venv/bin/python -m app.reset_admin
```
It resets the admin login to `ADMIN_USERNAME` / `ADMIN_PASSWORD` from `backend/.env` and logs out all admin sessions.

## How payment works

1. A student registers and opens a course from **My courses**.
2. They scan the UPI QR (amount and Student ID pre-filled) and pay.
3. They send their payment details and call the admin.
4. The admin checks the bank account, opens the payment note and clicks **Give access**,
   choosing the period (1, 3, 6, 12 months or any number of days).
5. The course unlocks right away, a receipt is created, and access ends automatically when the period
   is over. Renewals add time after the current end date.

## How frontend and backend connect

They run as separate servers:
- `frontend/js/config.js` holds the backend address in `API_BASE`.
- `backend/app/main.py` lists the frontend addresses allowed to call the API in the CORS middleware
  (`allow_origins`). Every other website is blocked.

## Deployment

Live setup: **frontend on Vercel** (https://gcation.vercel.app), **backend on Render**
(https://gcation.onrender.com, Singapore region, next to Neon), **database on Neon**.

**Backend (Render web service)**

| Setting | Value |
|---|---|
| Language | Python 3 |
| Root Directory | `backend` |
| Build Command | `pip install -r requirements.txt` |
| Start Command | `uvicorn app.main:app --host 0.0.0.0 --port $PORT` |
| Health Check Path | `/` |
| Environment | `PYTHON_VERSION=3.12.8`, `DATABASE_URL`, `ADMIN_USERNAME`, `ADMIN_PASSWORD`, `COOKIE_SECURE=true` |

**Frontend (Vercel)**: deploy the `frontend` folder as a static site (no build command).

**Connecting them**
1. `frontend/js/config.js`: set `API_BASE` to the backend address.
2. `backend/app/main.py`: add the website address to `allow_origins` in the CORS middleware.

The website logs in with a token sent as `Authorization: Bearer …`, so the frontend and backend can be on
different domains (like vercel.app and onrender.com) and login works in every browser, including Safari on iPhone.

## Security notes

- `backend/.env` holds the database password and admin login. It is git-ignored; never commit it
  and never put `DATABASE_URL` in the frontend.
- Passwords are hashed with scrypt. Repeated wrong logins are blocked for 15 minutes.
- Every write request must be JSON, which blocks cross-site form attacks; CORS allows only the
  listed frontend addresses.
- Login tokens are kept in the browser's storage and sent with each request; logging out removes them,
  and a password change or reset ends the old sessions.
- Students can only download their own receipts.

## Native app

A Go + Gio app can use the same backend: log in with `"client": "app"` to receive a token and send it as
`Authorization: Bearer <token>`. See [`backend/API.md`](backend/API.md).
