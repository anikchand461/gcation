import time

from psycopg.rows import dict_row
from psycopg_pool import ConnectionPool

from .config import settings
from .passwords import hash_password

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

-- The single admin login. Seeded from .env; changed from the admin dashboard.
CREATE TABLE IF NOT EXISTS admin_account (
    id            SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    username      TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    updated_at    BIGINT NOT NULL
);

-- Course thumbnail uploaded by the admin, shown to students.
ALTER TABLE courses ADD COLUMN IF NOT EXISTS thumbnail BYTEA;
-- Course highlights written by the admin, one per line (shown as a checklist).
ALTER TABLE courses ADD COLUMN IF NOT EXISTS features TEXT NOT NULL DEFAULT '';
ALTER TABLE courses ADD COLUMN IF NOT EXISTS thumbnail_type TEXT NOT NULL DEFAULT '';
ALTER TABLE courses ADD COLUMN IF NOT EXISTS thumbnail_updated_at BIGINT NOT NULL DEFAULT 0;

-- Extra details students give with a payment, printed on the receipt.
ALTER TABLE payment_claims ADD COLUMN IF NOT EXISTS age SMALLINT;
ALTER TABLE payment_claims ADD COLUMN IF NOT EXISTS referred_by TEXT NOT NULL DEFAULT '';
-- The payment note an access grant was verified against (its details go on the receipt).
ALTER TABLE enrollments ADD COLUMN IF NOT EXISTS claim_id BIGINT REFERENCES payment_claims(id) ON DELETE SET NULL;

-- Student ID, shown as GCS001. Only students whose payment was approved get one; see give_student_no().
ALTER TABLE students ADD COLUMN IF NOT EXISTS student_no INTEGER UNIQUE;

-- Management team: they refer students and see the courses, but cannot enroll.
-- Their Management ID is GCM + id, e.g. GCM001. Registering needs the invite code the admin sets.
CREATE TABLE IF NOT EXISTS managers (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name          TEXT NOT NULL,
    email         TEXT NOT NULL UNIQUE,
    phone         TEXT NOT NULL DEFAULT '',
    password_hash TEXT NOT NULL,
    created_at    BIGINT NOT NULL
);
-- Set when the admin revokes a member's access: they cannot log in until the admin restores it.
ALTER TABLE managers ADD COLUMN IF NOT EXISTS revoked_at BIGINT;
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS manager_id BIGINT REFERENCES managers(id) ON DELETE CASCADE;
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'sessions_role_check'
                   AND pg_get_constraintdef(oid) LIKE '%management%') THEN
        ALTER TABLE sessions DROP CONSTRAINT IF EXISTS sessions_role_check;
        ALTER TABLE sessions ADD CONSTRAINT sessions_role_check CHECK (role IN ('student', 'admin', 'management'));
    END IF;
END $$;

-- The management member whose Referral ID the student entered with a payment.
ALTER TABLE payment_claims ADD COLUMN IF NOT EXISTS manager_id BIGINT REFERENCES managers(id) ON DELETE SET NULL;
-- Payment screenshot. Deleted 5 minutes after the admin approves the payment (see screenshots.py).
ALTER TABLE payment_claims ADD COLUMN IF NOT EXISTS screenshot BYTEA;
ALTER TABLE payment_claims ADD COLUMN IF NOT EXISTS screenshot_type TEXT NOT NULL DEFAULT '';
ALTER TABLE payment_claims ADD COLUMN IF NOT EXISTS screenshot_delete_at BIGINT;

-- Who sees an update (any mix): the management team, every paid student, and/or the students of
-- some courses. Older updates used course_id (NULL = every paid student); they are copied over once.
ALTER TABLE announcements ADD COLUMN IF NOT EXISTS to_management BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE announcements ADD COLUMN IF NOT EXISTS to_all_students BOOLEAN;
ALTER TABLE announcements ADD COLUMN IF NOT EXISTS course_ids BIGINT[] NOT NULL DEFAULT '{}';
UPDATE announcements SET to_all_students = (course_id IS NULL),
    course_ids = CASE WHEN course_id IS NULL THEN '{}' ELSE ARRAY[course_id] END
    WHERE to_all_students IS NULL;
-- When a management member last opened Updates (for the unread count).
ALTER TABLE managers ADD COLUMN IF NOT EXISTS announcements_seen_at BIGINT NOT NULL DEFAULT 0;

-- How the student paid: 'upi' (QR, with UTR and screenshot) or 'cash' (no UTR or screenshot).
ALTER TABLE payment_claims ADD COLUMN IF NOT EXISTS pay_mode TEXT NOT NULL DEFAULT 'upi' CHECK (pay_mode IN ('upi', 'cash'));
-- When set, students must give "Referred by" and a Referral ID to pay for the course.
ALTER TABLE courses ADD COLUMN IF NOT EXISTS referral_required BOOLEAN NOT NULL DEFAULT FALSE;
-- The admin can let a management member post updates (e.g. while the admin is away), and turn it off again.
ALTER TABLE managers ADD COLUMN IF NOT EXISTS can_post_updates BOOLEAN NOT NULL DEFAULT FALSE;
-- The management member who posted an update (NULL = the admin).
ALTER TABLE announcements ADD COLUMN IF NOT EXISTS manager_id BIGINT REFERENCES managers(id) ON DELETE SET NULL;

-- Sign-in by mobile number (10 digits) and password, or by Google. Accounts made before mobile
-- log-in can still use their email; Google links to an existing account by email.
-- Students and management each have their own list of numbers.
ALTER TABLE students ALTER COLUMN email DROP NOT NULL;
ALTER TABLE students ADD COLUMN IF NOT EXISTS mobile TEXT UNIQUE;
ALTER TABLE students ADD COLUMN IF NOT EXISTS google_sub TEXT UNIQUE;
ALTER TABLE managers ALTER COLUMN email DROP NOT NULL;
ALTER TABLE managers ADD COLUMN IF NOT EXISTS mobile TEXT UNIQUE;
ALTER TABLE managers ADD COLUMN IF NOT EXISTS google_sub TEXT UNIQUE;

-- Google sign-in from the Android app: the app opens the browser, Google sends the browser back to
-- the API, and the app then collects the login with its secret code (only a hash of it is kept).
CREATE TABLE IF NOT EXISTS google_logins (
    state      TEXT PRIMARY KEY,
    app_code   TEXT NOT NULL UNIQUE,
    role       TEXT NOT NULL,
    status     TEXT NOT NULL DEFAULT 'pending',  -- pending, ok, need_details, error
    account_id BIGINT,
    detail     TEXT NOT NULL DEFAULT '',
    created_at BIGINT NOT NULL
);
-- A new person must add their mobile (and, for management, the referral code) before the account
-- is made; until then the Google details wait here.
ALTER TABLE google_logins ADD COLUMN IF NOT EXISTS google_sub TEXT NOT NULL DEFAULT '';
ALTER TABLE google_logins ADD COLUMN IF NOT EXISTS google_email TEXT NOT NULL DEFAULT '';
ALTER TABLE google_logins ADD COLUMN IF NOT EXISTS google_name TEXT NOT NULL DEFAULT '';

-- Site settings edited from the admin dashboard (e.g. the support phone number).
CREATE TABLE IF NOT EXISTS app_settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_enrollments_student ON enrollments(student_id, course_id);
CREATE INDEX IF NOT EXISTS idx_claims_status ON payment_claims(status);
CREATE INDEX IF NOT EXISTS idx_sessions_expiry ON sessions(expires_at);
CREATE INDEX IF NOT EXISTS idx_announcements_created ON announcements(created_at DESC);
"""

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
        conn.execute(
            "INSERT INTO payment_methods (slot, label, upi_id, payee_name, updated_at) VALUES "
            "('primary', 'Primary QR', %s, %s, %s), ('secondary', 'Secondary QR', '', %s, %s) "
            "ON CONFLICT (slot) DO NOTHING",
            (settings.upi_id, settings.upi_payee_name, now, settings.upi_payee_name, now),
        )
        conn.execute(
            "INSERT INTO app_settings (key, value) VALUES ('contact_phone', %s) ON CONFLICT (key) DO NOTHING",
            (settings.contact_phone,),
        )
        if not conn.execute("SELECT 1 FROM admin_account").fetchone():
            set_admin_from_env(conn)
        number_paid_students(conn)
        for table in ("students", "managers"):
            mobiles_from_phones(conn, table)


# Serialises Student ID numbering, so two approvals at once never get the same number.
STUDENT_NO_LOCK = 4207001


def number_paid_students(conn) -> None:
    """Gives a Student ID to every paid student who has none, in the order they first paid.

    The first run numbers all existing paid students GCS001, GCS002, ...; after that, approvals
    number students as they happen (access.give_student_no), so this finds nothing to do.
    """
    with conn.transaction():
        conn.execute("SELECT pg_advisory_xact_lock(%s)", (STUDENT_NO_LOCK,))
        conn.execute(
            "WITH firsts AS (SELECT e.student_id, MIN(e.granted_at) AS t, MIN(e.id) AS e FROM enrollments e "
            "  JOIN students s ON s.id = e.student_id WHERE e.revoked_at IS NULL AND s.student_no IS NULL "
            "  GROUP BY e.student_id), "
            "todo AS (SELECT student_id, ROW_NUMBER() OVER (ORDER BY t, e) AS n FROM firsts) "
            "UPDATE students s SET student_no = (SELECT COALESCE(MAX(student_no), 0) FROM students) + todo.n "
            "FROM todo WHERE s.id = todo.student_id"
        )


def mobiles_from_phones(conn, table: str) -> None:
    """Lets accounts made before mobile log-in log in with the phone number they gave.

    The number is used when it is a valid 10-digit Indian mobile (+91 / 0 prefixes allowed) that no
    other account in the table has. After the first run this finds nothing to do.
    """
    conn.execute(
        f"WITH d AS (SELECT id, regexp_replace(phone, '\\D', '', 'g') AS n FROM {table} WHERE mobile IS NULL), "
        "m AS (SELECT id, CASE WHEN length(n) = 12 AND n LIKE '91%' THEN right(n, 10) "
        "  WHEN length(n) = 11 AND n LIKE '0%' THEN right(n, 10) ELSE n END AS mobile FROM d), "
        "ok AS (SELECT id, mobile FROM m WHERE mobile ~ '^[6-9][0-9]{9}$'), "
        "single AS (SELECT mobile FROM ok GROUP BY mobile HAVING COUNT(*) = 1) "
        f"UPDATE {table} t SET mobile = ok.mobile FROM ok JOIN single USING (mobile) "
        f"WHERE t.id = ok.id AND NOT EXISTS (SELECT 1 FROM {table} x WHERE x.mobile = ok.mobile)"
    )


def set_admin_from_env(conn) -> None:
    """Creates or resets the admin login from ADMIN_USERNAME / ADMIN_PASSWORD in .env."""
    if not settings.admin_username or len(settings.admin_password) < 8:
        raise RuntimeError("Set ADMIN_USERNAME and ADMIN_PASSWORD (at least 8 characters) in .env.")
    conn.execute(
        "INSERT INTO admin_account (id, username, password_hash, updated_at) VALUES (1, %s, %s, %s) "
        "ON CONFLICT (id) DO UPDATE SET username = EXCLUDED.username, password_hash = EXCLUDED.password_hash, "
        "updated_at = EXCLUDED.updated_at",
        (settings.admin_username, hash_password(settings.admin_password), int(time.time())),
    )


def close_db() -> None:
    pool.close()


def get_db():
    with pool.connection() as conn:
        yield conn
