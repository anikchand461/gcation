"""Locked out? Resets the admin login to ADMIN_USERNAME / ADMIN_PASSWORD from backend/.env.

Run from the backend folder:  .venv/bin/python -m app.reset_admin
"""
from .db import close_db, init_db, pool, set_admin_from_env


def main() -> None:
    init_db()
    with pool.connection() as conn:
        set_admin_from_env(conn)
        conn.execute("DELETE FROM sessions WHERE role = 'admin'")
    close_db()
    print("Admin login reset to the username and password in .env. All admin sessions were logged out.")


if __name__ == "__main__":
    main()
