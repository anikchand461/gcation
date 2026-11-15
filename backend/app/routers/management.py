"""Management team accounts: they see the courses and prices and share their Management ID
(GCM001) with students, who enter it as the Referral ID when they pay. They cannot enroll."""

import hmac

from fastapi import APIRouter, Depends, HTTPException, Request, Response
from psycopg import Connection

from .. import auth
from ..access import ANNOUNCEMENT_COLUMNS, COURSE_COLUMNS, get_setting, manager_out, now, public_course
from ..db import get_db
from ..schemas import LoginIn, ManagerRegisterIn, PasswordChangeIn

router = APIRouter(prefix="/api/management")

REVOKED = "Your management access has been revoked. Please contact the G-CATION admin to get it back."


@router.post("/register")
def register(body: ManagerRegisterIn, request: Request, response: Response, db: Connection = Depends(get_db)):
    auth.throttle(request)
    code = get_setting(db, "management_invite_code")
    if not code:
        raise HTTPException(403, "Management registration is closed right now. Please contact the G-CATION admin.")
    if not hmac.compare_digest(body.invite_code.strip().upper().encode(), code.upper().encode()):
        auth.record_failure(request)
        raise HTTPException(403, "This referral code is not correct. Ask the G-CATION admin for the current code.")
    with db.transaction():
        manager = db.execute(
            "INSERT INTO managers (name, email, phone, password_hash, created_at) VALUES (%s, %s, %s, %s, %s) "
            "ON CONFLICT (email) DO NOTHING RETURNING id",
            (body.name, body.email, body.phone, auth.hash_password(body.password), now()),
        ).fetchone()
        if not manager:
            raise HTTPException(409, "A management account with this email already exists. Please log in.")
        token = auth.start_session(db, response, "management", manager_id=manager["id"])
    return {"ok": True} | ({"token": token} if body.client == "app" else {})


@router.post("/login")
def login(body: LoginIn, request: Request, response: Response, db: Connection = Depends(get_db)):
    auth.throttle(request)
    manager = db.execute("SELECT * FROM managers WHERE email = %s", (body.email.strip().lower(),)).fetchone()
    if not manager or not auth.verify_password(body.password, manager["password_hash"]):
        auth.record_failure(request)
        raise HTTPException(401, "Wrong email or password.")
    if manager["revoked_at"] is not None:
        raise HTTPException(403, REVOKED)
    token = auth.start_session(db, response, "management", manager_id=manager["id"])
    return {"ok": True} | ({"token": token} if body.client == "app" else {})


@router.post("/logout")
def logout(request: Request, response: Response, db: Connection = Depends(get_db)):
    auth.end_session(db, request, response, "management")
    return {"ok": True}


def _updates(db: Connection) -> list[dict]:
    return db.execute(
        f"SELECT {ANNOUNCEMENT_COLUMNS} FROM announcements a WHERE a.to_management ORDER BY a.created_at DESC LIMIT 50"
    ).fetchall()


@router.get("/me")
def me(manager: dict = Depends(auth.current_manager), db: Connection = Depends(get_db)):
    courses = db.execute(f"SELECT {COURSE_COLUMNS} FROM courses WHERE is_active ORDER BY id").fetchall()
    unread = sum(1 for a in _updates(db) if a["created_at"] > manager["announcements_seen_at"])
    return {"manager": manager_out(manager), "courses": [public_course(c) for c in courses], "unread_announcements": unread}


@router.get("/me/announcements")
def my_announcements(manager: dict = Depends(auth.current_manager), db: Connection = Depends(get_db)):
    """Updates the admin sent to the management team."""
    return {"seen_at": manager["announcements_seen_at"], "items": _updates(db)}


@router.post("/me/announcements/seen")
def mark_announcements_seen(manager: dict = Depends(auth.current_manager), db: Connection = Depends(get_db)):
    db.execute("UPDATE managers SET announcements_seen_at = %s WHERE id = %s", (now(), manager["id"]))
    return {"ok": True}


@router.post("/me/password")
def change_password(
    body: PasswordChangeIn,
    request: Request,
    manager: dict = Depends(auth.current_manager),
    db: Connection = Depends(get_db),
):
    if not auth.verify_password(body.current_password, manager["password_hash"]):
        raise HTTPException(400, "Your current password is not correct.")
    db.execute(
        "UPDATE managers SET password_hash = %s WHERE id = %s", (auth.hash_password(body.new_password), manager["id"])
    )
    auth.end_other_manager_sessions(db, request, manager["id"])
    return {"ok": True}
