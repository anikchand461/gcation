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

    # First admin login, copied into the database on first start (or by `python -m app.reset_admin`).
    # After that, change the username and password from Admin → Admin account.
    admin_username = os.getenv("ADMIN_USERNAME", "").strip()
    admin_password = os.getenv("ADMIN_PASSWORD", "")

    # Only used to fill the primary QR the very first time the database is created.
    # After that, QR codes and UPI IDs are managed from the admin dashboard.
    upi_id = os.getenv("UPI_ID", "").strip()
    upi_payee_name = os.getenv("UPI_PAYEE_NAME", "G-CATION").strip()

    # Support phone used only to fill the setting the first time; change it in Admin → Payment QR.
    contact_phone = os.getenv("CONTACT_PHONE", "").strip()

    # Google sign-in (Google Cloud → Google Auth Platform → Clients). The secret stays on the server.
    google_client_id = os.getenv("GOOGLE_CLIENT_ID", "").strip()
    google_client_secret = os.getenv("GOOGLE_CLIENT_SECRET", "").strip()
    # This API's public address, e.g. https://gcation.onrender.com. Google sends app sign-ins back to
    # <API_PUBLIC_URL>/api/auth/google/callback, which must be listed as a redirect URI in Google Cloud.
    api_public_url = os.getenv("API_PUBLIC_URL", "").strip().rstrip("/")

    # Set to true once the site is served over HTTPS.
    cookie_secure = _bool(os.getenv("COOKIE_SECURE", "false"))


settings = Settings()

if not settings.database_url:
    raise RuntimeError("Set DATABASE_URL in .env to your Neon Postgres connection string before starting.")
