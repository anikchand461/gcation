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
    "id, title, description, features, price, default_days, classroom_link, is_active, "
    "thumbnail IS NOT NULL AS has_thumbnail, thumbnail_updated_at"
)


def get_course(db: Connection, course_id: int) -> dict:
    course = db.execute(f"SELECT {COURSE_COLUMNS} FROM courses WHERE id = %s", (course_id,)).fetchone()
    if not course:
        raise HTTPException(404, "Course not found.")
    return course


def public_course(c: dict) -> dict:
    out = {k: c[k] for k in ("id", "title", "description", "price", "default_days", "has_thumbnail", "thumbnail_updated_at")}
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


def student_out(s: dict) -> dict:
    return {k: s[k] for k in ("id", "name", "email", "phone", "created_at")} | {"student_code": student_code(s["student_no"])}


def manager_out(m: dict) -> dict:
    return {k: m[k] for k in ("id", "name", "email", "phone", "created_at", "revoked_at")} | {"manager_code": manager_code(m["id"])}


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


# Payment note columns, without the screenshot bytes (the admin fetches the image separately).
# Needs `FROM payment_claims p`; pass rows through claim_out().
CLAIM_COLUMNS = (
    "p.id, p.student_id, p.course_id, p.qr_slot, p.payer_name, p.method, p.amount, p.paid_on, p.reference, "
    "p.age, p.referred_by, p.manager_id, p.status, p.created_at, p.screenshot IS NOT NULL AS has_screenshot"
)


def claim_out(c: dict) -> dict:
    """Swaps the internal manager_id for the Referral ID people see (GCM001, or '' when none)."""
    out = dict(c)
    out["referral_id"] = manager_code(out.pop("manager_id")) or ""
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
