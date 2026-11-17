"""Course access rules and small shared helpers."""

import re
import time

from fastapi import HTTPException
from psycopg import Connection

from .db import STUDENT_NO_LOCK

DAY = 24 * 3600
NO_ACCESS = {"status": "none", "until": None}


def now() -> int:
    return int(time.time())


def _summarise(periods: list[tuple[int, int]], t: int) -> dict:
    """Turn a student's non-revoked access periods for one course into a status."""
    latest_end = max(end for _, end in periods)
    if any(start <= t < end for start, end in periods):
        since = min(start for start, end in periods if end > t)
        return {
            "status": "active",
            "since": since,
            "until": latest_end,
            "days_left": max(0, (latest_end - t + DAY - 1) // DAY),
        }
    if latest_end > t:
        return {"status": "scheduled", "until": latest_end}
    return {"status": "expired", "until": latest_end}


def load_access(db: Connection, student_id: int | None = None) -> dict[tuple[int, int], dict]:
    """Access status keyed by (student_id, course_id), in a single query."""
    sql = "SELECT student_id, course_id, start_at, end_at FROM enrollments WHERE revoked_at IS NULL"
    params: tuple = ()
    if student_id is not None:
        sql += " AND student_id = %s"
        params = (student_id,)
    periods: dict[tuple[int, int], list[tuple[int, int]]] = {}
    for r in db.execute(sql, params):
        periods.setdefault((r["student_id"], r["course_id"]), []).append((r["start_at"], r["end_at"]))
    t = now()
    return {key: _summarise(p, t) for key, p in periods.items()}


def active_course_ids(access: dict[tuple[int, int], dict], student_id: int) -> list[int]:
    return [cid for (sid, cid), a in access.items() if sid == student_id and a["status"] == "active"]


# Every course column except the thumbnail bytes (served separately as an image).
COURSE_COLUMNS = (
    "id, title, description, features, price, default_days, classroom_link, is_active, referral_required, "
    "thumbnail IS NOT NULL AS has_thumbnail, thumbnail_updated_at"
)


def get_course(db: Connection, course_id: int) -> dict:
    course = db.execute(f"SELECT {COURSE_COLUMNS} FROM courses WHERE id = %s", (course_id,)).fetchone()
    if not course:
        raise HTTPException(404, "Course not found.")
    return course


def public_course(c: dict) -> dict:
    out = {k: c[k] for k in ("id", "title", "description", "price", "default_days", "referral_required", "has_thumbnail", "thumbnail_updated_at")}
    out["features"] = [line.strip() for line in c["features"].splitlines() if line.strip()]
    return out


def get_setting(db: Connection, key: str) -> str:
    row = db.execute("SELECT value FROM app_settings WHERE key = %s", (key,)).fetchone()
    return row["value"] if row else ""


def _code(prefix: str, n: int | None) -> str | None:
    return f"{prefix}{n:03d}" if n is not None else None


def student_code(student_no: int | None) -> str | None:
    """Student ID shown to people, e.g. GCS001. None until the student's first payment is approved."""
    return _code("GCS", student_no)


def manager_code(manager_id: int | None) -> str | None:
    """Management ID, e.g. GCM001. Students enter it as the Referral ID when they pay."""
    return _code("GCM", manager_id)


def number_from_code(prefix: str, text: str) -> int | None:
    """'gcs 007' or '7' -> 7 for prefix GCS; None if the text is not such an ID."""
    m = re.fullmatch(rf"(?:{prefix})?0*(\d{{1,9}})", re.sub(r"\s+", "", text).upper())
    return int(m.group(1)) if m else None


def give_student_no(db: Connection, student_id: int) -> None:
    """Gives the student the next Student ID if they have none. Call inside a transaction when a payment is approved."""
    db.execute("SELECT pg_advisory_xact_lock(%s)", (STUDENT_NO_LOCK,))
    db.execute(
        "UPDATE students SET student_no = (SELECT COALESCE(MAX(student_no), 0) + 1 FROM students) "
        "WHERE id = %s AND student_no IS NULL",
        (student_id,),
    )


def _sign_in(a: dict) -> dict:
    """How the account signs in: its log-in mobile (None until it has one), a password, and Google."""
    return {"mobile": a["mobile"], "has_password": bool(a["password_hash"]), "google": bool(a["google_sub"])}


def student_out(s: dict) -> dict:
    return {k: s[k] for k in ("id", "name", "email", "phone", "created_at")} | _sign_in(s) | {
        "student_code": student_code(s["student_no"])
    }


def manager_out(m: dict) -> dict:
    return {k: m[k] for k in ("id", "name", "email", "phone", "created_at", "revoked_at", "can_post_updates")} | _sign_in(m) | {
        "manager_code": manager_code(m["id"])
    }


# What students, management and the admin see of an update. Needs `FROM announcements a`.
ANNOUNCEMENT_COLUMNS = (
    "a.id, a.title, a.message, a.link_url, a.link_label, a.to_management, a.to_all_students, a.course_ids, "
    "a.created_at, a.updated_at"
)


def audience_label(a: dict, titles: dict[int, str]) -> str:
    """e.g. 'Management · All paid students' or 'Management · Course A, Course B'."""
    parts = ["Management"] if a["to_management"] else []
    if a["to_all_students"]:
        parts.append("All paid students")
    elif a["course_ids"]:
        parts.append(", ".join(titles.get(cid, "Deleted course") for cid in a["course_ids"]))
    return " · ".join(parts)


def course_titles(db: Connection) -> dict[int, str]:
    return {c["id"]: c["title"] for c in db.execute("SELECT id, title FROM courses")}


def update_audience(db: Connection) -> dict:
    """How many people each update target reaches: students with active access ("all" and per
    course id) and active management members ("management")."""
    access = load_access(db)
    per_course: dict[int, set[int]] = {}
    for (sid, cid), a in access.items():
        if a["status"] == "active":
            per_course.setdefault(cid, set()).add(sid)
    everyone = set().union(*per_course.values()) if per_course else set()
    team = db.execute("SELECT COUNT(*) AS n FROM managers WHERE revoked_at IS NULL").fetchone()["n"]
    return {"all": len(everyone), "management": team, **{str(cid): len(s) for cid, s in per_course.items()}}


def check_courses(db: Connection, course_ids: list[int]) -> None:
    for cid in course_ids:
        get_course(db, cid)


def insert_update(db: Connection, body, manager_id: int | None = None) -> int:
    """Posts an update (an AnnouncementIn). manager_id is the management member posting it; None is the admin."""
    check_courses(db, body.course_ids)
    t = now()
    return db.execute(
        "INSERT INTO announcements (title, message, link_url, link_label, to_management, to_all_students, course_ids, "
        "manager_id, created_at, updated_at) VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s) RETURNING id",
        (body.title, body.message, body.link_url, body.link_label, body.to_management, body.to_all_students,
         body.course_ids, manager_id, t, t),
    ).fetchone()["id"]


def edit_update(db: Connection, announcement_id: int, body, manager_id: int | None = None) -> None:
    """Saves changes to an update. With manager_id, only that member's own updates can change."""
    check_courses(db, body.course_ids)
    # course_id is cleared: the audience now lives in to_management / to_all_students / course_ids.
    cur = db.execute(
        "UPDATE announcements SET title = %s, message = %s, link_url = %s, link_label = %s, to_management = %s, "
        "to_all_students = %s, course_ids = %s, course_id = NULL, updated_at = %s "
        "WHERE id = %s AND (%s::bigint IS NULL OR manager_id = %s)",
        (body.title, body.message, body.link_url, body.link_label, body.to_management, body.to_all_students,
         body.course_ids, now(), announcement_id, manager_id, manager_id),
    )
    if cur.rowcount == 0:
        raise HTTPException(404, "Announcement not found.")


# Payment note columns, without the screenshot bytes (the admin fetches the image separately).
# Needs `FROM payment_claims p`; pass rows through claim_out().
CLAIM_COLUMNS = (
    "p.id, p.student_id, p.course_id, p.pay_mode, p.qr_slot, p.payer_name, p.method, p.amount, p.paid_on, p.reference, "
    "p.age, p.referred_by, p.referral_code, p.manager_id, p.status, p.created_at, p.screenshot IS NOT NULL AS has_screenshot"
)


def claim_out(c: dict) -> dict:
    """Swaps the internal manager_id and the typed code for the Referral ID people see: the linked
    member's ID (GCM001), else what the student typed, else ''."""
    out = dict(c)
    out["referral_id"] = manager_code(out.pop("manager_id")) or out.pop("referral_code", "") or ""
    out.pop("referral_code", None)
    return out


def method_out(m: dict) -> dict:
    """Payment method without the raw image bytes."""
    return {
        "slot": m["slot"],
        "label": m["label"],
        "upi_id": m["upi_id"],
        "payee_name": m["payee_name"],
        "is_active": m["is_active"],
        "has_image": m["has_image"],
        "updated_at": m["updated_at"],
    }


METHOD_COLUMNS = "slot, label, upi_id, payee_name, is_active, updated_at, image IS NOT NULL AS has_image"
