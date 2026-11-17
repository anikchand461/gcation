"""Google sign-in for students and management (never the admin).

The website uses Google's sign-in button, which hands the page an ID token. The Android app opens the
browser instead: Google sends the browser back to /api/auth/google/callback with a code, which is
swapped for an ID token using the client secret. Either way the token is checked with Google.

A Google account links to an existing account with the same (verified) email, so people who
registered before Google sign-in existed keep all their data.
"""

import hmac
import json
import logging
import urllib.parse
import urllib.request

from fastapi import HTTPException
from psycopg import Connection

from .access import get_setting, now
from .schemas import clean_mobile
from .config import settings

AUTH_URL = "https://accounts.google.com/o/oauth2/v2/auth"
TOKEN_URL = "https://oauth2.googleapis.com/token"
TOKENINFO_URL = "https://oauth2.googleapis.com/tokeninfo"

# Asked of people who are new to G-CATION before their account is made.
NEED_DETAILS = {
    "student": "One more step: enter your mobile number to finish creating your account.",
    "management": "One more step: enter your mobile number and the referral code from the G-CATION admin.",
}
REVOKED = "Your management access has been revoked. Please contact the G-CATION admin to get it back."


def enabled() -> bool:
    return bool(settings.google_client_id)


def redirect_uri(base_url: str) -> str:
    return (settings.api_public_url or base_url.rstrip("/")) + "/api/auth/google/callback"


def auth_url(state: str, base_url: str) -> str:
    return AUTH_URL + "?" + urllib.parse.urlencode({
        "client_id": settings.google_client_id,
        "redirect_uri": redirect_uri(base_url),
        "response_type": "code",
        "scope": "openid email profile",
        "state": state,
        "prompt": "select_account",
    })


def _fetch(url: str, data: dict | None = None) -> dict:
    body = urllib.parse.urlencode(data).encode() if data else None
    try:
        with urllib.request.urlopen(urllib.request.Request(url, data=body), timeout=15) as resp:
            return json.loads(resp.read())
    except Exception as e:
        logging.warning("Google request failed: %s %s", e, getattr(e, "read", lambda: b"")()[:300])
        raise HTTPException(401, "Google sign-in failed. Please try again.")


def verify_id_token(id_token: str) -> dict:
    """Checks an ID token with Google; returns {sub, email, name}. Only verified emails are accepted."""
    if not enabled():
        raise HTTPException(503, "Google sign-in is not set up yet.")
    info = _fetch(TOKENINFO_URL + "?" + urllib.parse.urlencode({"id_token": id_token}))
    if info.get("aud") != settings.google_client_id or info.get("iss") not in {"accounts.google.com", "https://accounts.google.com"}:
        raise HTTPException(401, "Google sign-in failed. Please try again.")
    if str(info.get("email_verified")).lower() != "true" or not info.get("email"):
        raise HTTPException(401, "Your Google account's email is not verified.")
    return {"sub": info["sub"], "email": info["email"].strip().lower(), "name": (info.get("name") or "").strip()}


def exchange_code(code: str, base_url: str) -> dict:
    """Swaps the code from Google's redirect for the user's details (app sign-in)."""
    tokens = _fetch(TOKEN_URL, {
        "code": code,
        "client_id": settings.google_client_id,
        "client_secret": settings.google_client_secret,
        "redirect_uri": redirect_uri(base_url),
        "grant_type": "authorization_code",
    })
    if not tokens.get("id_token"):
        raise HTTPException(401, "Google sign-in failed. Please try again.")
    return verify_id_token(tokens["id_token"])


# Gmail ignores dots and anything after '+' in the name, so rahul.das@gmail.com is rahuldas@gmail.com.
_GMAIL = ("gmail.com", "googlemail.com")
_EMAIL_KEY = (
    "CASE WHEN split_part(email, '@', 2) IN ('gmail.com', 'googlemail.com') "
    "THEN replace(split_part(split_part(email, '@', 1), '+', 1), '.', '') || '@gmail.com' ELSE email END"
)


def email_key(email: str) -> str:
    name, _, domain = email.partition("@")
    if domain in _GMAIL:
        return name.split("+")[0].replace(".", "") + "@gmail.com"
    return email


def sign_in(db: Connection, role: str, user: dict, mobile: str = "", invite_code: str = "") -> int:
    """Finds (or, for new people, creates) the student or management account for a Google user.

    Returns the account id. A new account needs a mobile number, and management also the referral
    code; until they are given, a 428 asks for them (NEED_DETAILS).
    """
    table = "students" if role == "student" else "managers"
    row = db.execute(f"SELECT * FROM {table} WHERE google_sub = %s", (user["sub"],)).fetchone()
    if not row:
        row = db.execute(
            f"SELECT * FROM {table} WHERE email IS NOT NULL AND {_EMAIL_KEY} = %s ORDER BY id LIMIT 1",
            (email_key(user["email"]),),
        ).fetchone()
        if row and row["google_sub"]:
            raise HTTPException(409, "This email is already linked to a different Google account.")
        if row:  # an account made before Google sign-in: link it, keeping all its data
            db.execute(f"UPDATE {table} SET google_sub = %s WHERE id = %s", (user["sub"], row["id"]))
    if row:
        if role == "management" and row["revoked_at"] is not None:
            raise HTTPException(403, REVOKED)
        return row["id"]

    if role == "management" and not get_setting(db, "management_invite_code"):
        raise HTTPException(403, "Management registration is closed right now. Please contact the G-CATION admin.")
    if not mobile or (role == "management" and not invite_code):
        raise HTTPException(428, NEED_DETAILS[role])
    try:
        mobile = clean_mobile(mobile)
    except ValueError as e:
        raise HTTPException(400, str(e))
    if db.execute(f"SELECT 1 FROM {table} WHERE mobile = %s", (mobile,)).fetchone():
        raise HTTPException(409, "This mobile number is already used by another account. Log in with it, or use another number.")
    if role == "management":
        code = get_setting(db, "management_invite_code")
        if not hmac.compare_digest(invite_code.strip().upper().encode(), code.upper().encode()):
            raise HTTPException(403, "This referral code is not correct. Ask the G-CATION admin for the current code.")
    name = user["name"] or user["email"].split("@")[0]
    return db.execute(
        f"INSERT INTO {table} (name, email, mobile, phone, password_hash, google_sub, created_at) "
        "VALUES (%s, %s, %s, %s, '', %s, %s) RETURNING id",
        (name[:80], user["email"], mobile, display_mobile(mobile), user["sub"], now()),
    ).fetchone()["id"]


def display_mobile(mobile: str) -> str:
    """'9876543210' -> '+91 98765 43210' (kept as the contact phone)."""
    return f"+91 {mobile[:5]} {mobile[5:]}"
