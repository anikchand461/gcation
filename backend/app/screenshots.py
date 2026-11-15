"""Payment screenshots: kept until the admin approves the payment, then deleted 5 minutes later.

To let Neon sleep when the site is idle, the database is only checked when a deletion is due.
If the server itself was asleep at that time, the screenshot is deleted when it starts again,
and it is never shown once its time has passed.
"""

import asyncio
import logging
import time

from psycopg import Connection

from .db import pool

KEEP_AFTER_APPROVAL = 5 * 60

_next_due: float | None = None  # earliest time a screenshot is due for deletion, if any


def _note_due(t: float | None) -> None:
    global _next_due
    if t is not None and (_next_due is None or t < _next_due):
        _next_due = t


def schedule_delete(db: Connection, claim_id: int) -> None:
    """Called when the admin approves the payment note."""
    due = int(time.time()) + KEEP_AFTER_APPROVAL
    db.execute(
        "UPDATE payment_claims SET screenshot_delete_at = %s WHERE id = %s AND screenshot IS NOT NULL",
        (due, claim_id),
    )
    _note_due(due)


def delete_due(conn: Connection) -> None:
    """Deletes screenshots whose time is up and remembers when the next one is due."""
    global _next_due
    now = int(time.time())
    conn.execute(
        "UPDATE payment_claims SET screenshot = NULL, screenshot_type = '' "
        "WHERE screenshot IS NOT NULL AND screenshot_delete_at <= %s",
        (now,),
    )
    row = conn.execute(
        "SELECT MIN(screenshot_delete_at) AS t FROM payment_claims WHERE screenshot IS NOT NULL AND screenshot_delete_at IS NOT NULL"
    ).fetchone()
    _next_due = None
    _note_due(row["t"])


def _run_due() -> None:
    with pool.connection() as conn:
        delete_due(conn)


async def delete_loop() -> None:
    """Runs for the life of the server; touches the database only when a deletion is due."""
    while True:
        await asyncio.sleep(20)
        if _next_due is not None and time.time() >= _next_due:
            try:
                await asyncio.to_thread(_run_due)
            except Exception:
                logging.exception("Deleting payment screenshots failed; will retry.")
