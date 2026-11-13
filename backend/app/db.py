import time

from psycopg.rows import dict_row
from psycopg_pool import ConnectionPool

from .config import settings

SCHEMA = """
CREATE TABLE IF NOT EXISTS students (
    id                    BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name                  TEXT NOT NULL,
    email                 TEXT NOT NULL UNIQUE,
    phone                 TEXT NOT NULL DEFAULT '',
    password_hash         TEXT NOT NULL,
    created_at            BIGINT NOT NULL,
    announcements_seen_at BIGINT NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS sessions (
    token_hash TEXT PRIMARY KEY,
    role       TEXT NOT NULL CHECK (role IN ('student', 'admin')),
    student_id BIGINT REFERENCES students(id) ON DELETE CASCADE,
    expires_at BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS courses (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    title          TEXT NOT NULL,
    description    TEXT NOT NULL DEFAULT '',
    price          NUMERIC(12, 2) NOT NULL,
    default_days   INTEGER NOT NULL,
    classroom_link TEXT NOT NULL DEFAULT '',
    is_active      BOOLEAN NOT NULL DEFAULT TRUE
);

-- One row per access grant by the admin. Doubles as the payment record.
CREATE TABLE IF NOT EXISTS enrollments (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    student_id  BIGINT NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    course_id   BIGINT NOT NULL REFERENCES courses(id),
    start_at    BIGINT NOT NULL,
    end_at      BIGINT NOT NULL,
    amount_paid NUMERIC(12, 2) NOT NULL DEFAULT 0,
    note        TEXT NOT NULL DEFAULT '',
    granted_at  BIGINT NOT NULL,
    revoked_at  BIGINT
);

-- Optional "I have paid" notes a student leaves before calling the admin.
CREATE TABLE IF NOT EXISTS payment_claims (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    student_id BIGINT NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    course_id  BIGINT NOT NULL REFERENCES courses(id),
    qr_slot    TEXT NOT NULL DEFAULT '',
    payer_name TEXT NOT NULL DEFAULT '',
    method     TEXT NOT NULL DEFAULT '',
    amount     NUMERIC(12, 2),
    paid_on    TEXT NOT NULL DEFAULT '',
    reference  TEXT NOT NULL DEFAULT '',
    status     TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'dismissed')),
    created_at BIGINT NOT NULL
);

-- The two QR codes students can pay to. Managed from the admin dashboard.
CREATE TABLE IF NOT EXISTS payment_methods (
    slot       TEXT PRIMARY KEY CHECK (slot IN ('primary', 'secondary')),
    label      TEXT NOT NULL DEFAULT '',
    upi_id     TEXT NOT NULL DEFAULT '',
    payee_name TEXT NOT NULL DEFAULT '',
    image      BYTEA,
    image_type TEXT NOT NULL DEFAULT '',
    is_active  BOOLEAN NOT NULL DEFAULT TRUE,
    updated_at BIGINT NOT NULL
);

-- Messages and links (community, Google Meet, instructions) for paid students.
-- course_id NULL means every student with active access.
CREATE TABLE IF NOT EXISTS announcements (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    title      TEXT NOT NULL,
    message    TEXT NOT NULL DEFAULT '',
    link_url   TEXT NOT NULL DEFAULT '',
    link_label TEXT NOT NULL DEFAULT '',
    course_id  BIGINT REFERENCES courses(id) ON DELETE CASCADE,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_enrollments_student ON enrollments(student_id, course_id);
CREATE INDEX IF NOT EXISTS idx_claims_status ON payment_claims(status);
CREATE INDEX IF NOT EXISTS idx_sessions_expiry ON sessions(expires_at);
CREATE INDEX IF NOT EXISTS idx_announcements_created ON announcements(created_at DESC);
"""

DEFAULT_COURSE = (
    "Online Spoken English Batch",
    "A one-year online Spoken English learning package with live classes, "
    "speaking practice and communication skill development.",
    3099,
    365,
)

# min_size=0 lets Neon scale the database to zero when the site is idle, and max_idle=60 drops
# idle connections well before Neon suspends, so a stale connection is never handed out.
# autocommit avoids BEGIN/COMMIT round trips; endpoints that write several rows use db.transaction().
# prepare_threshold=None keeps queries compatible with Neon's pooled (PgBouncer) endpoint.
pool = ConnectionPool(
    settings.database_url,
    min_size=0,
    max_size=5,
    max_idle=60,
    open=False,
    kwargs={"row_factory": dict_row, "prepare_threshold": None, "autocommit": True},
)


def init_db() -> None:
    pool.open()
    now = int(time.time())
    with pool.connection() as conn:
        conn.execute(SCHEMA)
        if conn.execute("SELECT COUNT(*) AS n FROM courses").fetchone()["n"] == 0:
            conn.execute(
                "INSERT INTO courses (title, description, price, default_days) VALUES (%s, %s, %s, %s)",
                DEFAULT_COURSE,
            )
        conn.execute(
            "INSERT INTO payment_methods (slot, label, upi_id, payee_name, updated_at) VALUES "
            "('primary', 'Primary QR', %s, %s, %s), ('secondary', 'Secondary QR', '', %s, %s) "
            "ON CONFLICT (slot) DO NOTHING",
            (settings.upi_id, settings.upi_payee_name, now, settings.upi_payee_name, now),
        )


def close_db() -> None:
    pool.close()


def get_db():
    with pool.connection() as conn:
        yield conn
