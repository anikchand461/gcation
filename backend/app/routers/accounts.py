"""Sign-up and log-in for students and management (the admin has its own fixed login).

Two ways in: a mobile number and password, or Google. Accounts made before mobile log-in can
still use their email and password, and Google sign-in links to them by email.
"""

import hashlib
import hmac
import html
import secrets

from fastapi import APIRouter, Depends, HTTPException, Request, Response
from fastapi.responses import HTMLResponse, RedirectResponse
from psycopg import Connection

from .. import auth, google
from ..access import get_setting, now
from ..config import settings
from ..db import get_db
from ..schemas import GoogleCompleteIn, GoogleIn, LoginIn, ManagerRegisterIn, MobileIn, RegisterIn, Role, clean_mobile

router = APIRouter()

TABLES = {"student": "students", "management": "managers"}
WHO = {"student": "An account", "management": "A management account"}
APP_LOGIN_TTL = 10 * 60


def _session(db: Connection, response: Response, role: str, account_id: int, client: str) -> dict:
    ids = {"student_id": account_id} if role == "student" else {"manager_id": account_id}
    token = auth.start_session(db, response, role, **ids)
    return {"ok": True} | ({"token": token} if client == "app" else {})


def _mobile_taken(db: Connection, role: str, mobile: str, account_id: int | None = None) -> bool:
    return bool(db.execute(
        f"SELECT 1 FROM {TABLES[role]} WHERE mobile = %s AND id IS DISTINCT FROM %s", (mobile, account_id)
    ).fetchone())


def _check_invite(db: Connection, request: Request, invite_code: str) -> None:
    code = get_setting(db, "management_invite_code")
    if not code:
        raise HTTPException(403, "Management registration is closed right now. Please contact the G-CATION admin.")
    if not hmac.compare_digest(invite_code.strip().upper().encode(), code.upper().encode()):
        auth.record_failure(request)
        raise HTTPException(403, "This referral code is not correct. Ask the G-CATION admin for the current code.")


# ---------- Settings the sign-in pages need ----------

@router.get("/api/auth/config")
def auth_config():
    return {"google_client_id": settings.google_client_id if google.enabled() else ""}


# ---------- Register & log in with a mobile number ----------

def _register(db: Connection, request: Request, response: Response, role: Role, body: RegisterIn) -> dict:
    auth.throttle(request)
    taken = f"{WHO[role]} with this mobile number already exists. Please log in."
    if _mobile_taken(db, role, body.mobile):
        raise HTTPException(409, taken)
    if role == "management":
        _check_invite(db, request, body.invite_code)
    with db.transaction():
        row = db.execute(
            f"INSERT INTO {TABLES[role]} (name, mobile, phone, password_hash, created_at) VALUES (%s, %s, %s, %s, %s) "
            "ON CONFLICT (mobile) DO NOTHING RETURNING id",
            (body.name, body.mobile, google.display_mobile(body.mobile), auth.hash_password(body.password), now()),
        ).fetchone()
        if not row:  # registered a moment ago in another request
            raise HTTPException(409, taken)
        return _session(db, response, role, row["id"], body.client)


@router.post("/api/register")
def register_student(body: RegisterIn, request: Request, response: Response, db: Connection = Depends(get_db)):
    return _register(db, request, response, "student", body)


@router.post("/api/management/register")
def register_manager(body: ManagerRegisterIn, request: Request, response: Response, db: Connection = Depends(get_db)):
    return _register(db, request, response, "management", body)


def _login(db: Connection, request: Request, response: Response, role: Role, body: LoginIn) -> dict:
    auth.throttle(request)
    login = (body.mobile or body.email).strip().lower()
    if not login:
        raise HTTPException(400, "Enter your mobile number.")
    if "@" in login:  # an account made before mobile log-in, or an older app
        row = db.execute(f"SELECT * FROM {TABLES[role]} WHERE email = %s", (login,)).fetchone()
    else:
        try:
            mobile = clean_mobile(login)
        except ValueError as e:
            raise HTTPException(400, str(e))
        row = db.execute(f"SELECT * FROM {TABLES[role]} WHERE mobile = %s", (mobile,)).fetchone()
    if row and not row["password_hash"]:
        raise HTTPException(400, "This account signs in with Google. Tap Continue with Google.")
    if not row or not auth.verify_password(body.password, row["password_hash"]):
        auth.record_failure(request)
        raise HTTPException(401, "Wrong mobile number or password.")
    if role == "management" and row["revoked_at"] is not None:
        raise HTTPException(403, google.REVOKED)
    return _session(db, response, role, row["id"], body.client)


@router.post("/api/login")
def login_student(body: LoginIn, request: Request, response: Response, db: Connection = Depends(get_db)):
    return _login(db, request, response, "student", body)


@router.post("/api/management/login")
def login_manager(body: LoginIn, request: Request, response: Response, db: Connection = Depends(get_db)):
    return _login(db, request, response, "management", body)


# ---------- Set or change the mobile number a logged-in account logs in with ----------

def _set_mobile(db: Connection, role: Role, account_id: int, body: MobileIn) -> dict:
    if _mobile_taken(db, role, body.mobile, account_id):
        raise HTTPException(409, "This mobile number is already used by another account.")
    db.execute(f"UPDATE {TABLES[role]} SET mobile = %s, phone = %s WHERE id = %s",
               (body.mobile, google.display_mobile(body.mobile), account_id))
    return {"ok": True, "mobile": body.mobile, "phone": google.display_mobile(body.mobile)}


@router.put("/api/me/mobile")
def student_mobile(body: MobileIn, student: dict = Depends(auth.current_student), db: Connection = Depends(get_db)):
    return _set_mobile(db, "student", student["id"], body)


@router.put("/api/management/me/mobile")
def manager_mobile(body: MobileIn, manager: dict = Depends(auth.current_manager), db: Connection = Depends(get_db)):
    return _set_mobile(db, "management", manager["id"], body)


# ---------- Google: website ----------

@router.post("/api/auth/google")
def google_web(body: GoogleIn, request: Request, response: Response, db: Connection = Depends(get_db)):
    """The website's "Continue with Google" button sends the ID token Google gave the page.

    A 428 means the person is new: the page asks for their mobile (and referral code) and sends
    the same token again with them.
    """
    auth.throttle(request)
    user = google.verify_id_token(body.credential)
    with db.transaction():
        account_id = google.sign_in(db, body.role, user, body.mobile, body.invite_code)
        return _session(db, response, body.role, account_id, body.client)


# ---------- Google: Android app ----------
# The app makes a secret code, opens /start in the browser, and asks /result with the same code
# until the person has picked their Google account. A new person then gives their mobile (and
# referral code) in the app, which sends them to /complete. Only hashes of the code are stored.

def _code_hash(code: str) -> str:
    return hashlib.sha256(code.encode()).hexdigest()


@router.get("/api/auth/google/start")
def google_app_start(request: Request, role: Role = "student", code: str = "", db: Connection = Depends(get_db)):
    if not google.enabled() or not settings.google_client_secret:
        return _page("Google sign-in is not set up yet", "Please log in with your mobile number and password.", ok=False)
    if not 32 <= len(code) <= 128:
        raise HTTPException(400, "Invalid sign-in request. Please try again from the app.")
    state = secrets.token_urlsafe(24)
    t = now()
    db.execute("DELETE FROM google_logins WHERE app_code = %s OR created_at < %s", (_code_hash(code), t - APP_LOGIN_TTL))
    db.execute("INSERT INTO google_logins (state, app_code, role, created_at) VALUES (%s, %s, %s, %s)",
               (state, _code_hash(code), role, t))
    return RedirectResponse(google.auth_url(state, str(request.base_url)), status_code=302)


@router.get("/api/auth/google/callback")
def google_app_callback(request: Request, state: str = "", code: str = "", error: str = "",
                        db: Connection = Depends(get_db)):
    row = db.execute(
        "SELECT * FROM google_logins WHERE state = %s AND status = 'pending' AND created_at > %s",
        (state, now() - APP_LOGIN_TTL),
    ).fetchone()
    if not row:
        return _page("This sign-in link has expired", "Go back to the G-CATION app and tap Continue with Google again.", ok=False)
    status, detail, account_id, user = "error", "Google sign-in was cancelled.", None, {}
    if code and not error:
        try:
            user = google.exchange_code(code, str(request.base_url))
            with db.transaction():
                account_id = google.sign_in(db, row["role"], user)
            status, detail = "ok", ""
        except HTTPException as e:
            status = "need_details" if e.status_code == 428 else "error"
            detail = str(e.detail)
    db.execute(
        "UPDATE google_logins SET status = %s, account_id = %s, detail = %s, google_sub = %s, google_email = %s, "
        "google_name = %s WHERE state = %s",
        (status, account_id, detail, user.get("sub", ""), user.get("email", ""), user.get("name", ""), state),
    )
    if status == "ok":
        return _page("You are signed in", "Go back to the G-CATION app to continue.", ok=True)
    if status == "need_details":
        return _page("One more step", "Go back to the G-CATION app and enter your mobile number to finish.", ok=True)
    return _page("Could not sign in", detail + " Go back to the G-CATION app and try again.", ok=False)


def _app_login(db: Connection, code: str) -> dict | None:
    return db.execute(
        "SELECT * FROM google_logins WHERE app_code = %s AND created_at > %s",
        (_code_hash(code), now() - APP_LOGIN_TTL),
    ).fetchone()


@router.get("/api/auth/google/result")
def google_app_result(response: Response, code: str = "", db: Connection = Depends(get_db)):
    """The app asks this every few seconds: {status: pending | ok (+token) | need_details (+name) | error, detail}."""
    row = _app_login(db, code)
    if not row:
        return {"status": "error", "detail": "Sign-in timed out. Please try again."}
    if row["status"] == "pending":
        return {"status": "pending"}
    if row["status"] == "need_details":  # kept until /complete
        return {"status": "need_details", "detail": row["detail"], "name": row["google_name"]}
    db.execute("DELETE FROM google_logins WHERE state = %s", (row["state"],))
    if row["status"] != "ok":
        return {"status": row["status"], "detail": row["detail"]}
    return {"status": "ok"} | _session(db, response, row["role"], row["account_id"], "app")


@router.post("/api/auth/google/complete")
def google_app_complete(body: GoogleCompleteIn, request: Request, response: Response, db: Connection = Depends(get_db)):
    """Creates the account of a new person once the app has their mobile (and referral code)."""
    auth.throttle(request)
    row = _app_login(db, body.code)
    if not row or row["status"] != "need_details":
        raise HTTPException(410, "Sign-in timed out. Please tap Continue with Google again.")
    user = {"sub": row["google_sub"], "email": row["google_email"], "name": row["google_name"]}
    with db.transaction():
        account_id = google.sign_in(db, row["role"], user, body.mobile, body.invite_code)
        db.execute("DELETE FROM google_logins WHERE state = %s", (row["state"],))
        return {"status": "ok"} | _session(db, response, row["role"], account_id, "app")


def _page(title: str, message: str, ok: bool) -> HTMLResponse:
    color = "#047857" if ok else "#be123c"
    mark = "&#10003;" if ok else "!"
    return HTMLResponse(f"""<!DOCTYPE html><html lang="en"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1"><title>G-CATION</title>
<style>body{{margin:0;min-height:100vh;display:grid;place-items:center;font-family:system-ui,sans-serif;
background:#f6f4fc;color:#14102e;padding:24px;box-sizing:border-box}}
.card{{background:#fff;border-radius:22px;padding:32px 26px;max-width:380px;text-align:center;
box-shadow:0 12px 40px rgba(60,30,120,.12)}}.mark{{width:60px;height:60px;border-radius:50%;
display:grid;place-items:center;margin:0 auto 14px;font-size:30px;font-weight:800;color:#fff;background:{color}}}
h1{{font-size:1.3rem;margin:0 0 8px}}p{{margin:0;color:#67628a;line-height:1.5}}
.brand{{font-weight:800;color:#5b21b6;margin-bottom:18px}}</style></head>
<body><div class="card"><div class="brand">G-CATION</div><div class="mark">{mark}</div>
<h1>{html.escape(title)}</h1><p>{html.escape(message)}</p></div></body></html>""")
