import os
from pathlib import Path

from dotenv import load_dotenv

BASE_DIR = Path(__file__).resolve().parent.parent
load_dotenv(BASE_DIR / ".env")


def _bool(value: str) -> bool:
    return value.strip().lower() in {"1", "true", "yes", "on"}


class Settings:
    # Neon (or any Postgres) connection string. Keep it secret: backend only.
    database_url = os.getenv("DATABASE_URL", "").strip()

    # Fixed admin login. There is no admin registration; change these in .env.
    admin_username = os.getenv("ADMIN_USERNAME", "").strip()
    admin_password = os.getenv("ADMIN_PASSWORD", "")

    # Only used to fill the primary QR the very first time the database is created.
    # After that, QR codes and UPI IDs are managed from the admin dashboard.
    upi_id = os.getenv("UPI_ID", "").strip()
    upi_payee_name = os.getenv("UPI_PAYEE_NAME", "G-CATION").strip()

    # Number students are told to call after paying.
    contact_phone = os.getenv("CONTACT_PHONE", "").strip()

    # Set to true once the site is served over HTTPS.
    cookie_secure = _bool(os.getenv("COOKIE_SECURE", "false"))


settings = Settings()

if not settings.database_url:
    raise RuntimeError("Set DATABASE_URL in .env to your Neon Postgres connection string before starting.")
if not settings.admin_username or len(settings.admin_password) < 8:
    raise RuntimeError(
        "Set ADMIN_USERNAME and ADMIN_PASSWORD (at least 8 characters) in .env before starting."
    )
