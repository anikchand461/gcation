import base64
import binascii

from fastapi import HTTPException

MAX_IMAGE_BYTES = 2 * 1024 * 1024


def decode_image(data: str) -> tuple[bytes, str]:
    """Base64 (optionally a data: URL) -> (bytes, media type). Only real PNG/JPEG/WEBP images pass."""
    if data.startswith("data:"):
        data = data.split(",", 1)[-1]
    try:
        raw = base64.b64decode(data, validate=True)
    except (binascii.Error, ValueError):
        raise HTTPException(400, "The image could not be read.")
    if len(raw) > MAX_IMAGE_BYTES:
        raise HTTPException(413, "Image must be smaller than 2 MB.")
    # Trust the file's own bytes, not its name.
    if raw.startswith(b"\x89PNG\r\n\x1a\n"):
        return raw, "image/png"
    if raw.startswith(b"\xff\xd8\xff"):
        return raw, "image/jpeg"
    if raw[:4] == b"RIFF" and raw[8:12] == b"WEBP":
        return raw, "image/webp"
    raise HTTPException(400, "Upload a PNG, JPG or WEBP image.")
