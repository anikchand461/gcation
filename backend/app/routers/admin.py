import io
from datetime import datetime
from typing import Literal
from urllib.parse import quote
from zoneinfo import ZoneInfo

import qrcode
import qrcode.image.svg
from fastapi import APIRouter, Depends, HTTPException, Request, Response
from psycopg import Connection

from .. import auth
from ..access import COURSE_COLUMNS, DAY, METHOD_COLUMNS, get_setting, NO_ACCESS, get_course, load_access, method_out, now, student_out
from ..db import get_db
from ..images import decode_image
from ..receipts import load_grant, pdf_response, receipt_no
from ..schemas import (
    AdminAccountIn,
    AdminLoginIn,
    AnnouncementIn,
    CourseIn,
    GrantIn,
    PasswordResetIn,
    PaymentMethodIn,
    QrImageIn,
    QrSlot,
    SettingsIn,
    StudentEditIn,
)

IST = ZoneInfo("Asia/Kolkata")

router = APIRouter(prefix="/api/admin")
admin = Depends(auth.require_admin)


# ---------- Auth ----------

@router.post("/login")
def admin_login(body: AdminLoginIn, request: Request, response: Response, db: Connection = Depends(get_db)):
    auth.throttle(request)
    if not auth.check_admin_credentials(db, body.username, body.password):
        auth.record_failure(request)
        raise HTTPException(401, "Wrong username or password.")
    token = auth.start_session(db, response, "admin")
    return {"ok": True} | ({"token": token} if body.client == "app" else {})


@router.post("/logout")
def admin_logout(request: Request, response: Response, db: Connection = Depends(get_db)):
    auth.end_session(db, request, response, "admin")
    return {"ok": True}


@router.get("/account", dependencies=[admin])
def account(db: Connection = Depends(get_db)):
    return db.execute("SELECT username, updated_at FROM admin_account WHERE id = 1").fetchone()


@router.put("/account", dependencies=[admin])
def update_account(body: AdminAccountIn, request: Request, db: Connection = Depends(get_db)):
    auth.throttle(request)
    current = db.execute("SELECT password_hash FROM admin_account WHERE id = 1").fetchone()
    if not current or not auth.verify_password(body.current_password, current["password_hash"]):
        auth.record_failure(request)
        raise HTTPException(400, "Your current password is not correct.")
    with db.transaction():
        if body.new_password:
            db.execute(
                "UPDATE admin_account SET username = %s, password_hash = %s, updated_at = %s WHERE id = 1",
                (body.username, auth.hash_password(body.new_password), now()),
            )
        else:
            db.execute("UPDATE admin_account SET username = %s, updated_at = %s WHERE id = 1", (body.username, now()))
        auth.end_other_admin_sessions(db, request)
    return {"ok": True, "username": body.username}


@router.get("/settings", dependencies=[admin])
def site_settings(db: Connection = Depends(get_db)):
    return {"contact_phone": get_setting(db, "contact_phone")}


@router.put("/settings", dependencies=[admin])
def update_site_settings(body: SettingsIn, db: Connection = Depends(get_db)):
    db.execute(
        "INSERT INTO app_settings (key, value) VALUES ('contact_phone', %s) "
        "ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value",
        (body.contact_phone,),
    )
    return {"contact_phone": body.contact_phone}


# ---------- Overview & students ----------

@router.get("/overview", dependencies=[admin])
def overview(db: Connection = Depends(get_db)):
    t = now()
    month_start = int(datetime.now(IST).replace(day=1, hour=0, minute=0, second=0, microsecond=0).timestamp())
    access = load_access(db)
    active = {sid for (sid, _), a in access.items() if a["status"] == "active"}
    expiring = {sid for (sid, _), a in access.items() if a["status"] == "active" and a["until"] - t <= 7 * DAY}
    row = db.execute(
        "SELECT (SELECT COUNT(*) FROM students) AS students, "
        "(SELECT COUNT(*) FROM payment_claims WHERE status = 'pending') AS pending_claims, "
        "(SELECT COALESCE(SUM(amount_paid), 0) FROM enrollments) AS total_collected, "
        "(SELECT COALESCE(SUM(amount_paid), 0) FROM enrollments WHERE granted_at >= %s) AS collected_this_month",
        (month_start,),
    ).fetchone()
    return row | {"active_students": len(active), "expiring_soon": len(expiring)}


@router.get("/students", dependencies=[admin])
def students(q: str = "", db: Connection = Depends(get_db)):
    q = q.strip()
    like = f"%{q}%"
    rows = db.execute(
        "SELECT * FROM students WHERE name ILIKE %s OR email ILIKE %s OR phone ILIKE %s OR id::text = %s "
        "ORDER BY created_at DESC",
        (like, like, like, q),
    ).fetchall()
    courses = db.execute("SELECT id, title FROM courses ORDER BY id").fetchall()
    access = load_access(db)
    pending = {
        r["student_id"]: r["n"]
        for r in db.execute(
            "SELECT student_id, COUNT(*) AS n FROM payment_claims WHERE status = 'pending' GROUP BY student_id"
        )
    }
    out = []
    for s in rows:
        items = []
        for c in courses:
            a = access.get((s["id"], c["id"]))
            if a:
                items.append({"course_id": c["id"], "title": c["title"]} | a)
        out.append(student_out(s) | {"access": items, "pending_claims": pending.get(s["id"], 0)})
    return out


@router.get("/students/{student_id}", dependencies=[admin])
def student_detail(student_id: int, db: Connection = Depends(get_db)):
    s = db.execute("SELECT * FROM students WHERE id = %s", (student_id,)).fetchone()
    if not s:
        raise HTTPException(404, "Student not found.")
    access = load_access(db, student_id)
    courses = db.execute("SELECT id, title, price, default_days FROM courses ORDER BY id").fetchall()
    enrollments = db.execute(
        "SELECT e.*, c.title AS course_title FROM enrollments e JOIN courses c ON c.id = e.course_id "
        "WHERE e.student_id = %s ORDER BY e.granted_at DESC",
        (student_id,),
    ).fetchall()
    claims = db.execute(
        "SELECT p.*, c.title AS course_title FROM payment_claims p JOIN courses c ON c.id = p.course_id "
        "WHERE p.student_id = %s ORDER BY p.created_at DESC",
        (student_id,),
    ).fetchall()
    return {
        "student": student_out(s),
        "courses": [c | {"access": access.get((student_id, c["id"]), NO_ACCESS)} for c in courses],
        "enrollments": [e | {"receipt_no": receipt_no(e)} for e in enrollments],
        "claims": claims,
    }


@router.put("/students/{student_id}", dependencies=[admin])
def edit_student(student_id: int, body: StudentEditIn, db: Connection = Depends(get_db)):
    taken = db.execute("SELECT 1 FROM students WHERE email = %s AND id <> %s", (body.email, student_id)).fetchone()
    if taken:
        raise HTTPException(409, "Another student already uses this email.")
    cur = db.execute(
        "UPDATE students SET name = %s, email = %s, phone = %s WHERE id = %s",
        (body.name, body.email, body.phone, student_id),
    )
    if cur.rowcount == 0:
        raise HTTPException(404, "Student not found.")
    return {"ok": True}


@router.post("/students/{student_id}/password", dependencies=[admin])
def reset_student_password(student_id: int, body: PasswordResetIn, db: Connection = Depends(get_db)):
    """For students who forgot their password: admin sets a new one and logs them out everywhere."""
    with db.transaction():
        cur = db.execute(
            "UPDATE students SET password_hash = %s WHERE id = %s", (auth.hash_password(body.new_password), student_id)
        )
        if cur.rowcount == 0:
            raise HTTPException(404, "Student not found.")
        db.execute("DELETE FROM sessions WHERE student_id = %s", (student_id,))
    return {"ok": True}


@router.post("/students/{student_id}/grant", dependencies=[admin])
def grant(student_id: int, body: GrantIn, db: Connection = Depends(get_db)):
    if not db.execute("SELECT 1 FROM students WHERE id = %s", (student_id,)).fetchone():
        raise HTTPException(404, "Student not found.")
    get_course(db, body.course_id)
    t = now()
    start = t
    if body.start == "after_current":
        current = load_access(db, student_id).get((student_id, body.course_id), NO_ACCESS)
        if current["status"] in {"active", "scheduled"}:
            start = current["until"]
    end = start + body.days * DAY
    with db.transaction():
        db.execute(
            "INSERT INTO enrollments (student_id, course_id, start_at, end_at, amount_paid, note, granted_at) "
            "VALUES (%s, %s, %s, %s, %s, %s, %s)",
            (student_id, body.course_id, start, end, body.amount, body.note.strip(), t),
        )
        if body.claim_id is not None:
            db.execute(
                "UPDATE payment_claims SET status = 'approved' WHERE id = %s AND student_id = %s",
                (body.claim_id, student_id),
            )
    return {"ok": True, "start_at": start, "end_at": end}


@router.get("/enrollments/{enrollment_id}/receipt.pdf", dependencies=[admin])
def enrollment_receipt(enrollment_id: int, db: Connection = Depends(get_db)):
    return pdf_response(load_grant(db, enrollment_id), get_setting(db, "contact_phone"))


@router.post("/enrollments/{enrollment_id}/revoke", dependencies=[admin])
def revoke(enrollment_id: int, db: Connection = Depends(get_db)):
    cur = db.execute(
        "UPDATE enrollments SET revoked_at = %s WHERE id = %s AND revoked_at IS NULL", (now(), enrollment_id)
    )
    if cur.rowcount == 0:
        raise HTTPException(404, "Grant not found or already revoked.")
    return {"ok": True}


# ---------- Payments & payment notes ----------

@router.get("/payments", dependencies=[admin])
def payments(db: Connection = Depends(get_db)):
    rows = db.execute(
        "SELECT e.*, s.name AS student_name, s.email AS student_email, c.title AS course_title "
        "FROM enrollments e JOIN students s ON s.id = e.student_id JOIN courses c ON c.id = e.course_id "
        "ORDER BY e.granted_at DESC"
    ).fetchall()
    for r in rows:
        r["receipt_no"] = receipt_no(r)
    return rows


@router.get("/claims", dependencies=[admin])
def claims(status: Literal["pending", "approved", "dismissed"] = "pending", db: Connection = Depends(get_db)):
    return db.execute(
        "SELECT p.*, s.name AS student_name, s.email AS student_email, s.phone AS student_phone, c.title AS course_title "
        "FROM payment_claims p JOIN students s ON s.id = p.student_id JOIN courses c ON c.id = p.course_id "
        "WHERE p.status = %s ORDER BY p.created_at DESC",
        (status,),
    ).fetchall()


@router.post("/claims/{claim_id}/dismiss", dependencies=[admin])
def dismiss_claim(claim_id: int, db: Connection = Depends(get_db)):
    cur = db.execute("UPDATE payment_claims SET status = 'dismissed' WHERE id = %s AND status = 'pending'", (claim_id,))
    if cur.rowcount == 0:
        raise HTTPException(404, "Payment note not found.")
    return {"ok": True}


# ---------- Payment QR codes ----------

def _method(db: Connection, slot: str) -> dict:
    return method_out(db.execute(f"SELECT {METHOD_COLUMNS} FROM payment_methods WHERE slot = %s", (slot,)).fetchone())


@router.get("/payment-methods", dependencies=[admin])
def payment_methods(db: Connection = Depends(get_db)):
    return [method_out(m) for m in db.execute(f"SELECT {METHOD_COLUMNS} FROM payment_methods ORDER BY slot")]


@router.get("/payment-methods/{slot}/preview.svg", dependencies=[admin])
def preview_payment_qr(slot: QrSlot, db: Connection = Depends(get_db)):
    """The QR students get when no image is uploaded (shown here without an amount)."""
    m = db.execute("SELECT upi_id, payee_name FROM payment_methods WHERE slot = %s", (slot,)).fetchone()
    if not m or not m["upi_id"]:
        raise HTTPException(404, "Add a UPI ID first.")
    upi = f"upi://pay?pa={quote(m['upi_id'], safe='@')}&pn={quote(m['payee_name'] or 'G-CATION')}&cu=INR"
    buf = io.BytesIO()
    qrcode.make(upi, image_factory=qrcode.image.svg.SvgPathImage, box_size=10, border=2).save(buf)
    return Response(buf.getvalue(), media_type="image/svg+xml", headers={"Cache-Control": "no-store"})


@router.put("/payment-methods/{slot}", dependencies=[admin])
def update_payment_method(slot: QrSlot, body: PaymentMethodIn, db: Connection = Depends(get_db)):
    db.execute(
        "UPDATE payment_methods SET label = %s, upi_id = %s, payee_name = %s, is_active = %s, updated_at = %s WHERE slot = %s",
        (body.label, body.upi_id, body.payee_name, body.is_active, now(), slot),
    )
    return _method(db, slot)


@router.put("/payment-methods/{slot}/image", dependencies=[admin])
def upload_payment_qr(slot: QrSlot, body: QrImageIn, db: Connection = Depends(get_db)):
    raw, media_type = decode_image(body.data)
    db.execute(
        "UPDATE payment_methods SET image = %s, image_type = %s, updated_at = %s WHERE slot = %s",
        (raw, media_type, now(), slot),
    )
    return _method(db, slot)


@router.delete("/payment-methods/{slot}/image", dependencies=[admin])
def remove_payment_qr(slot: QrSlot, db: Connection = Depends(get_db)):
    db.execute(
        "UPDATE payment_methods SET image = NULL, image_type = '', updated_at = %s WHERE slot = %s", (now(), slot)
    )
    return _method(db, slot)


# ---------- Announcements ----------

def _audience(db: Connection) -> dict:
    """How many students with active access each announcement target reaches."""
    access = load_access(db)
    per_course: dict[int, set[int]] = {}
    for (sid, cid), a in access.items():
        if a["status"] == "active":
            per_course.setdefault(cid, set()).add(sid)
    everyone = set().union(*per_course.values()) if per_course else set()
    return {"all": len(everyone), **{str(cid): len(s) for cid, s in per_course.items()}}


@router.get("/announcements", dependencies=[admin])
def announcements(db: Connection = Depends(get_db)):
    items = db.execute(
        "SELECT a.*, c.title AS course_title FROM announcements a LEFT JOIN courses c ON c.id = a.course_id "
        "ORDER BY a.created_at DESC"
    ).fetchall()
    return {"items": items, "audience": _audience(db)}


@router.post("/announcements", dependencies=[admin])
def create_announcement(body: AnnouncementIn, db: Connection = Depends(get_db)):
    if body.course_id is not None:
        get_course(db, body.course_id)
    t = now()
    row = db.execute(
        "INSERT INTO announcements (title, message, link_url, link_label, course_id, created_at, updated_at) "
        "VALUES (%s, %s, %s, %s, %s, %s, %s) RETURNING id",
        (body.title, body.message, body.link_url, body.link_label, body.course_id, t, t),
    ).fetchone()
    return {"ok": True, "id": row["id"]}


@router.put("/announcements/{announcement_id}", dependencies=[admin])
def update_announcement(announcement_id: int, body: AnnouncementIn, db: Connection = Depends(get_db)):
    if body.course_id is not None:
        get_course(db, body.course_id)
    cur = db.execute(
        "UPDATE announcements SET title = %s, message = %s, link_url = %s, link_label = %s, course_id = %s, updated_at = %s "
        "WHERE id = %s",
        (body.title, body.message, body.link_url, body.link_label, body.course_id, now(), announcement_id),
    )
    if cur.rowcount == 0:
        raise HTTPException(404, "Announcement not found.")
    return {"ok": True}


@router.delete("/announcements/{announcement_id}", dependencies=[admin])
def delete_announcement(announcement_id: int, db: Connection = Depends(get_db)):
    if db.execute("DELETE FROM announcements WHERE id = %s", (announcement_id,)).rowcount == 0:
        raise HTTPException(404, "Announcement not found.")
    return {"ok": True}


# ---------- Courses ----------

@router.get("/courses", dependencies=[admin])
def courses(db: Connection = Depends(get_db)):
    return db.execute(f"SELECT {COURSE_COLUMNS} FROM courses ORDER BY id").fetchall()


@router.post("/courses", dependencies=[admin])
def create_course(body: CourseIn, db: Connection = Depends(get_db)):
    row = db.execute(
        "INSERT INTO courses (title, description, features, price, default_days, classroom_link, is_active) "
        "VALUES (%s, %s, %s, %s, %s, %s, %s) RETURNING id",
        (body.title.strip(), body.description.strip(), body.features, body.price, body.default_days,
         body.classroom_link, body.is_active),
    ).fetchone()
    return {"ok": True, "id": row["id"]}


@router.put("/courses/{course_id}", dependencies=[admin])
def update_course(course_id: int, body: CourseIn, db: Connection = Depends(get_db)):
    get_course(db, course_id)
    db.execute(
        "UPDATE courses SET title = %s, description = %s, features = %s, price = %s, default_days = %s, "
        "classroom_link = %s, is_active = %s WHERE id = %s",
        (body.title.strip(), body.description.strip(), body.features, body.price, body.default_days,
         body.classroom_link, body.is_active, course_id),
    )
    return {"ok": True}


@router.put("/courses/{course_id}/thumbnail", dependencies=[admin])
def upload_course_thumbnail(course_id: int, body: QrImageIn, db: Connection = Depends(get_db)):
    get_course(db, course_id)
    raw, media_type = decode_image(body.data)
    db.execute(
        "UPDATE courses SET thumbnail = %s, thumbnail_type = %s, thumbnail_updated_at = %s WHERE id = %s",
        (raw, media_type, now(), course_id),
    )
    return get_course(db, course_id)


@router.delete("/courses/{course_id}/thumbnail", dependencies=[admin])
def remove_course_thumbnail(course_id: int, db: Connection = Depends(get_db)):
    get_course(db, course_id)
    db.execute(
        "UPDATE courses SET thumbnail = NULL, thumbnail_type = '', thumbnail_updated_at = %s WHERE id = %s",
        (now(), course_id),
    )
    return get_course(db, course_id)


@router.delete("/courses/{course_id}", dependencies=[admin])
def delete_course(course_id: int, db: Connection = Depends(get_db)):
    get_course(db, course_id)
    paid = db.execute("SELECT COUNT(*) AS n FROM enrollments WHERE course_id = %s", (course_id,)).fetchone()["n"]
    if paid:
        # Deleting would erase payment history, so the admin hides the course instead.
        raise HTTPException(
            409, f"This course has {paid} payment record{'s' if paid != 1 else ''}. Hide it instead to keep the history."
        )
    with db.transaction():
        db.execute("DELETE FROM payment_claims WHERE course_id = %s", (course_id,))
        db.execute("DELETE FROM courses WHERE id = %s", (course_id,))  # its updates are removed with it
    return {"ok": True}
