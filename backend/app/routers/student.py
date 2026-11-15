import io
from urllib.parse import quote

import qrcode
import qrcode.image.svg
from fastapi import APIRouter, Depends, HTTPException, Request, Response
from psycopg import Connection

from .. import auth
from ..access import (
    COURSE_COLUMNS,
    METHOD_COLUMNS,
    get_setting,
    active_course_ids,
    get_course,
    load_access,
    method_out,
    now,
    public_course,
    student_out,
    NO_ACCESS,
)
from ..db import get_db
from ..receipts import RECEIPT_SQL, load_grant, pdf_response, receipt_summary
from ..schemas import ClaimIn, LoginIn, PasswordChangeIn, ProfileIn, QrSlot, RegisterIn

router = APIRouter(prefix="/api")


# ---------- Auth ----------

@router.post("/register")
def register(body: RegisterIn, response: Response, db: Connection = Depends(get_db)):
    if db.execute("SELECT 1 FROM students WHERE email = %s", (body.email,)).fetchone():
        raise HTTPException(409, "An account with this email already exists. Please log in.")
    with db.transaction():
        student = db.execute(
            "INSERT INTO students (name, email, phone, password_hash, created_at) VALUES (%s, %s, %s, %s, %s) "
            "ON CONFLICT (email) DO NOTHING RETURNING id",
            (body.name, body.email, body.phone, auth.hash_password(body.password), now()),
        ).fetchone()
        if not student:  # registered a moment ago in another request
            raise HTTPException(409, "An account with this email already exists. Please log in.")
        token = auth.start_session(db, response, "student", student["id"])
    return {"ok": True} | ({"token": token} if body.client == "app" else {})


@router.post("/login")
def login(body: LoginIn, request: Request, response: Response, db: Connection = Depends(get_db)):
    auth.throttle(request)
    student = db.execute("SELECT * FROM students WHERE email = %s", (body.email.strip().lower(),)).fetchone()
    if not student or not auth.verify_password(body.password, student["password_hash"]):
        auth.record_failure(request)
        raise HTTPException(401, "Wrong email or password.")
    token = auth.start_session(db, response, "student", student["id"])
    return {"ok": True} | ({"token": token} if body.client == "app" else {})


@router.post("/logout")
def logout(request: Request, response: Response, db: Connection = Depends(get_db)):
    auth.end_session(db, request, response, "student")
    return {"ok": True}


# ---------- Dashboard ----------

def _visible_announcements(db: Connection, course_ids: list[int]) -> list[dict]:
    """Announcements for students with active access to any of course_ids."""
    if not course_ids:
        return []
    return db.execute(
        "SELECT a.id, a.title, a.message, a.link_url, a.link_label, a.course_id, a.created_at, a.updated_at, "
        "c.title AS course_title FROM announcements a LEFT JOIN courses c ON c.id = a.course_id "
        "WHERE a.course_id IS NULL OR a.course_id = ANY(%s) ORDER BY a.created_at DESC LIMIT 50",
        (course_ids,),
    ).fetchall()


@router.get("/me")
def me(student: dict = Depends(auth.current_student), db: Connection = Depends(get_db)):
    access = load_access(db, student["id"])
    claims = {
        c["course_id"]: c
        for c in db.execute(
            "SELECT DISTINCT ON (course_id) course_id, qr_slot, payer_name, method, amount, paid_on, reference, age, referred_by, "
            "created_at "
            "FROM payment_claims WHERE student_id = %s AND status = 'pending' ORDER BY course_id, id DESC",
            (student["id"],),
        )
    }
    courses = []
    # Open courses, plus hidden ones this student has (or had) access to.
    enrolled = [cid for (_, cid) in access]
    for c in db.execute(f"SELECT {COURSE_COLUMNS} FROM courses WHERE is_active OR id = ANY(%s) ORDER BY id", (enrolled,)):
        a = access.get((student["id"], c["id"]), NO_ACCESS)
        item = public_course(c) | {"access": a, "pending_claim": claims.get(c["id"]), "is_open": c["is_active"]}
        if a["status"] == "active":
            item["classroom_link"] = c["classroom_link"]
        courses.append(item)

    methods = [
        method_out(m)
        for m in db.execute(f"SELECT {METHOD_COLUMNS} FROM payment_methods WHERE is_active ORDER BY slot")
        if m["upi_id"] or m["has_image"]
    ]
    announcements = _visible_announcements(db, active_course_ids(access, student["id"]))
    unread = sum(1 for a in announcements if a["created_at"] > student["announcements_seen_at"])
    receipts = db.execute(RECEIPT_SQL + "WHERE e.student_id = %s ORDER BY e.granted_at DESC", (student["id"],)).fetchall()
    return {
        "student": student_out(student),
        "courses": courses,
        "receipts": [receipt_summary(r) for r in receipts],
        "payment": {"contact_phone": get_setting(db, "contact_phone"), "methods": methods},
        "unread_announcements": unread,
    }


@router.put("/me")
def update_profile(body: ProfileIn, student: dict = Depends(auth.current_student), db: Connection = Depends(get_db)):
    db.execute("UPDATE students SET name = %s, phone = %s WHERE id = %s", (body.name, body.phone, student["id"]))
    return {"ok": True}


@router.post("/me/password")
def change_password(
    body: PasswordChangeIn,
    request: Request,
    student: dict = Depends(auth.current_student),
    db: Connection = Depends(get_db),
):
    if not auth.verify_password(body.current_password, student["password_hash"]):
        raise HTTPException(400, "Your current password is not correct.")
    db.execute(
        "UPDATE students SET password_hash = %s WHERE id = %s", (auth.hash_password(body.new_password), student["id"])
    )
    auth.end_other_sessions(db, request, student["id"])
    return {"ok": True}


@router.get("/me/receipts/{enrollment_id}.pdf")
def my_receipt(enrollment_id: int, student: dict = Depends(auth.current_student), db: Connection = Depends(get_db)):
    return pdf_response(load_grant(db, enrollment_id, student["id"]), get_setting(db, "contact_phone"))


@router.get("/me/announcements")
def my_announcements(student: dict = Depends(auth.current_student), db: Connection = Depends(get_db)):
    access = load_access(db, student["id"])
    return {
        "seen_at": student["announcements_seen_at"],
        "items": _visible_announcements(db, active_course_ids(access, student["id"])),
    }


@router.post("/me/announcements/seen")
def mark_announcements_seen(student: dict = Depends(auth.current_student), db: Connection = Depends(get_db)):
    db.execute("UPDATE students SET announcements_seen_at = %s WHERE id = %s", (now(), student["id"]))
    return {"ok": True}


@router.get("/me/qr/{course_id}.svg")
def payment_qr(
    course_id: int,
    slot: QrSlot = "primary",
    student: dict = Depends(auth.current_student),
    db: Connection = Depends(get_db),
):
    course = get_course(db, course_id)
    method = db.execute("SELECT upi_id, payee_name FROM payment_methods WHERE slot = %s", (slot,)).fetchone()
    if not method or not method["upi_id"]:
        raise HTTPException(404, "This payment option has no UPI ID.")
    # The note carries the student ID so the admin can match the bank entry to an account.
    note = f"GCATION ID {student['id']}"
    upi = (
        f"upi://pay?pa={quote(method['upi_id'], safe='@')}&pn={quote(method['payee_name'] or 'G-CATION')}"
        f"&am={course['price']:.2f}&cu=INR&tn={quote(note)}"
    )
    img = qrcode.make(upi, image_factory=qrcode.image.svg.SvgPathImage, box_size=10, border=2)
    buf = io.BytesIO()
    img.save(buf)
    return Response(buf.getvalue(), media_type="image/svg+xml", headers={"Cache-Control": "private, max-age=300"})


@router.post("/me/claims")
def create_claim(body: ClaimIn, student: dict = Depends(auth.current_student), db: Connection = Depends(get_db)):
    get_course(db, body.course_id)
    pending = db.execute(
        "SELECT COUNT(*) AS n FROM payment_claims WHERE student_id = %s AND status = 'pending'", (student["id"],)
    ).fetchone()["n"]
    if pending >= 3:
        raise HTTPException(429, "You already have payment details waiting. Please call the admin.")
    db.execute(
        "INSERT INTO payment_claims (student_id, course_id, qr_slot, payer_name, method, amount, paid_on, reference, "
        "age, referred_by, created_at) VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)",
        (student["id"], body.course_id, body.qr_slot, body.payer_name, body.method, body.amount,
         body.paid_on, body.reference, body.age, body.referred_by, now()),
    )
    return {"ok": True}
