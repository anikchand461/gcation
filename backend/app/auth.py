import hashlib
import hmac
import secrets
import time

from fastapi import Depends, HTTPException, Request, Response
from psycopg import Connection

from .config import settings
from .db import get_db
from .passwords import hash_password, verify_password  # noqa: F401 (re-exported)

STUDENT_COOKIE = "gc_session"
ADMIN_COOKIE = "gc_admin"
STUDENT_TTL = 30 * 24 * 3600
ADMIN_TTL = 12 * 3600

# In-memory brute-force guard: max failed logins per IP per window.
_FAILED: dict[str, list[float]] = {}
MAX_FAILURES = 10
WINDOW = 15 * 60


def check_admin_credentials(db: Connection, username: str, password: str) -> bool:
    account = db.execute("SELECT username, password_hash FROM admin_account WHERE id = 1").fetchone()
    if not account:
        return False
    user_ok = hmac.compare_digest(username.strip().encode(), account["username"].encode())
    pass_ok = verify_password(password, account["password_hash"])  # always runs, so timing does not leak the username
    return user_ok and pass_ok


def _token_hash(token: str) -> str:
    return hashlib.sha256(token.encode()).hexdigest()


def throttle(request: Request) -> None:
    ip = request.client.host if request.client else "unknown"
    now = time.time()
    recent = [t for t in _FAILED.get(ip, []) if now - t < WINDOW]
    _FAILED[ip] = recent
    if len(recent) >= MAX_FAILURES:
        raise HTTPException(429, "Too many failed attempts. Try again in 15 minutes.")


def record_failure(request: Request) -> None:
    ip = request.client.host if request.client else "unknown"
    _FAILED.setdefault(ip, []).append(time.time())


def start_session(db: Connection, response: Response, role: str, student_id: int | None = None) -> str:
    """Creates a session, sets the browser cookie and returns the token (native apps use it as a Bearer token)."""
    token = secrets.token_urlsafe(32)
    ttl = ADMIN_TTL if role == "admin" else STUDENT_TTL
    now = int(time.time())
    # Occasional cleanup of expired sessions; not worth a database round trip on every login.
    if secrets.randbelow(20) == 0:
        db.execute("DELETE FROM sessions WHERE expires_at < %s", (now,))
    db.execute(
        "INSERT INTO sessions (token_hash, role, student_id, expires_at) VALUES (%s, %s, %s, %s)",
        (_token_hash(token), role, student_id, now + ttl),
    )
    response.set_cookie(
        ADMIN_COOKIE if role == "admin" else STUDENT_COOKIE,
        token,
        max_age=ttl,
        httponly=True,
        samesite="strict" if role == "admin" else "lax",
        secure=settings.cookie_secure,
    )
    return token


def request_token(request: Request, role: str) -> str | None:
    """Session token from an `Authorization: Bearer` header (apps) or the login cookie (website)."""
    header = request.headers.get("authorization", "")
    if header.lower().startswith("bearer "):
        return header[7:].strip() or None
    return request.cookies.get(ADMIN_COOKIE if role == "admin" else STUDENT_COOKIE)


def end_session(db: Connection, request: Request, response: Response, role: str) -> None:
    token = request_token(request, role)
    if token:
        db.execute("DELETE FROM sessions WHERE token_hash = %s AND role = %s", (_token_hash(token), role))
    response.delete_cookie(ADMIN_COOKIE if role == "admin" else STUDENT_COOKIE)


def end_other_sessions(db: Connection, request: Request, student_id: int) -> None:
    """Logs a student out everywhere except the current device (after a password change)."""
    token = request_token(request, "student")
    db.execute(
        "DELETE FROM sessions WHERE student_id = %s AND token_hash <> %s",
        (student_id, _token_hash(token) if token else ""),
    )


def end_other_admin_sessions(db: Connection, request: Request) -> None:
    """Logs the admin out everywhere except this device (after the login details change)."""
    token = request_token(request, "admin")
    db.execute(
        "DELETE FROM sessions WHERE role = 'admin' AND token_hash <> %s", (_token_hash(token) if token else "",)
    )


def current_student(request: Request, db: Connection = Depends(get_db)) -> dict:
    token = request_token(request, "student")
    student = token and db.execute(
        "SELECT st.* FROM sessions se JOIN students st ON st.id = se.student_id "
        "WHERE se.token_hash = %s AND se.role = 'student' AND se.expires_at > %s",
        (_token_hash(token), int(time.time())),
    ).fetchone()
    if not student:
        raise HTTPException(401, "Please log in.")
    return student


def require_admin(request: Request, db: Connection = Depends(get_db)) -> None:
    token = request_token(request, "admin")
    found = token and db.execute(
        "SELECT 1 FROM sessions WHERE token_hash = %s AND role = 'admin' AND expires_at > %s",
        (_token_hash(token), int(time.time())),
    ).fetchone()
    if not found:
        raise HTTPException(401, "Admin login required.")
