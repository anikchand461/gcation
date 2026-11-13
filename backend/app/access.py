"""Course access rules and small shared helpers."""

import time

from fastapi import HTTPException
from psycopg import Connection

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


def student_out(s: dict) -> dict:
    return {k: s[k] for k in ("id", "name", "email", "phone", "created_at")}


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
