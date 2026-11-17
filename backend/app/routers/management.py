"""Management team accounts: they see the courses and prices and share their Management ID
(GCM001) with students, who enter it as the Referral ID when they pay. They cannot enroll."""

from fastapi import APIRouter, Depends, HTTPException, Request, Response
from psycopg import Connection

from .. import auth
from ..access import (
    ANNOUNCEMENT_COLUMNS,
    COURSE_COLUMNS,
    audience_label,
    course_titles,
    edit_update,
    insert_update,
    manager_out,
    now,
    public_course,
    update_audience,
)
from ..db import get_db
from ..schemas import AnnouncementIn, PasswordChangeIn

router = APIRouter(prefix="/api/management")


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
    # Google accounts start with no password, so they set one without a current password.
    if manager["password_hash"] and not auth.verify_password(body.current_password, manager["password_hash"]):
        raise HTTPException(400, "Your current password is not correct.")
    db.execute(
        "UPDATE managers SET password_hash = %s WHERE id = %s", (auth.hash_password(body.new_password), manager["id"])
    )
    auth.end_other_manager_sessions(db, request, manager["id"])
    return {"ok": True}


# ---------- Posting updates (only while the admin allows it) ----------

def poster(manager: dict = Depends(auth.current_manager)) -> dict:
    if not manager["can_post_updates"]:
        raise HTTPException(403, "The admin has turned off posting updates for your account.")
    return manager


@router.get("/updates")
def posted_updates(manager: dict = Depends(poster), db: Connection = Depends(get_db)):
    """Updates this member posted, plus the courses and audience counts for the composer."""
    titles = course_titles(db)
    items = db.execute(
        f"SELECT {ANNOUNCEMENT_COLUMNS} FROM announcements a WHERE a.manager_id = %s ORDER BY a.created_at DESC",
        (manager["id"],),
    ).fetchall()
    for a in items:
        a["audience_label"] = audience_label(a, titles)
    courses = db.execute("SELECT id, title FROM courses ORDER BY id").fetchall()
    return {"items": items, "courses": courses, "audience": update_audience(db)}


@router.post("/updates")
def create_update(body: AnnouncementIn, manager: dict = Depends(poster), db: Connection = Depends(get_db)):
    return {"ok": True, "id": insert_update(db, body, manager["id"])}


@router.put("/updates/{announcement_id}")
def save_update(
    announcement_id: int, body: AnnouncementIn, manager: dict = Depends(poster), db: Connection = Depends(get_db)
):
    edit_update(db, announcement_id, body, manager["id"])
    return {"ok": True}


@router.delete("/updates/{announcement_id}")
def delete_update(announcement_id: int, manager: dict = Depends(poster), db: Connection = Depends(get_db)):
    cur = db.execute("DELETE FROM announcements WHERE id = %s AND manager_id = %s", (announcement_id, manager["id"]))
    if cur.rowcount == 0:
        raise HTTPException(404, "Announcement not found.")
    return {"ok": True}
