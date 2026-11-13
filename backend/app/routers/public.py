from fastapi import APIRouter, Depends, HTTPException, Response
from psycopg import Connection

from ..access import public_course
from ..db import get_db
from ..schemas import QrSlot

router = APIRouter()


@router.get("/")
def health():
    return {"status": "ok", "service": "G-CATION API"}


@router.get("/api/courses")
def list_courses(db: Connection = Depends(get_db)):
    rows = db.execute("SELECT * FROM courses WHERE is_active ORDER BY id").fetchall()
    return [public_course(c) for c in rows]


@router.get("/api/payment-methods/{slot}/image")
def payment_qr_image(slot: QrSlot, db: Connection = Depends(get_db)):
    row = db.execute("SELECT image, image_type FROM payment_methods WHERE slot = %s", (slot,)).fetchone()
    if not row or row["image"] is None:
        raise HTTPException(404, "No QR image uploaded.")
    # The URL carries ?v=<updated_at>, so a replaced QR gets a new URL and caching is safe.
    return Response(
        bytes(row["image"]),
        media_type=row["image_type"],
        headers={"Cache-Control": "public, max-age=86400", "X-Content-Type-Options": "nosniff"},
    )
