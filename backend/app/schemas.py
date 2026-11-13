import re
from typing import Literal

from pydantic import BaseModel, Field, field_validator

EMAIL_RE = re.compile(r"^[^@\s]+@[^@\s]+\.[^@\s]+$")
UPI_RE = re.compile(r"^[\w.\-]{2,256}@[A-Za-z][A-Za-z0-9]{1,63}$")

QrSlot = Literal["primary", "secondary"]
# "app" makes login/register also return the session token in the JSON body (for the native app).
Client = Literal["web", "app"]


def _https_or_empty(v: str, what: str) -> str:
    v = v.strip()
    if v and not v.startswith("https://"):
        raise ValueError(f"{what} must start with https://")
    return v


class RegisterIn(BaseModel):
    name: str = Field(min_length=1, max_length=80)
    email: str = Field(max_length=120)
    phone: str = Field(default="", max_length=20)
    password: str = Field(min_length=8, max_length=128)
    client: Client = "web"

    @field_validator("name", "phone")
    @classmethod
    def strip(cls, v: str) -> str:
        return v.strip()

    @field_validator("email")
    @classmethod
    def valid_email(cls, v: str) -> str:
        v = v.strip().lower()
        if not EMAIL_RE.match(v):
            raise ValueError("Enter a valid email address.")
        return v


class LoginIn(BaseModel):
    email: str = Field(max_length=120)
    password: str = Field(max_length=128)
    client: Client = "web"


class AdminLoginIn(BaseModel):
    username: str = Field(max_length=120)
    password: str = Field(max_length=128)
    client: Client = "web"


class ProfileIn(BaseModel):
    name: str = Field(min_length=1, max_length=80)
    phone: str = Field(default="", max_length=20)

    @field_validator("name", "phone")
    @classmethod
    def strip(cls, v: str) -> str:
        return v.strip()


class PasswordChangeIn(BaseModel):
    current_password: str = Field(max_length=128)
    new_password: str = Field(min_length=8, max_length=128)


class StudentEditIn(ProfileIn):
    email: str = Field(max_length=120)

    @field_validator("email")
    @classmethod
    def valid_email(cls, v: str) -> str:
        v = v.strip().lower()
        if not EMAIL_RE.match(v):
            raise ValueError("Enter a valid email address.")
        return v


class PasswordResetIn(BaseModel):
    new_password: str = Field(min_length=8, max_length=128)


class ClaimIn(BaseModel):
    course_id: int
    qr_slot: QrSlot | Literal[""] = ""
    payer_name: str = Field(default="", max_length=80)
    method: str = Field(default="", max_length=60)
    amount: float | None = Field(default=None, ge=0, le=10_000_000)
    paid_on: str = Field(default="", max_length=20)
    reference: str = Field(default="", max_length=80)


class GrantIn(BaseModel):
    course_id: int
    days: int = Field(ge=1, le=3650)
    start: Literal["now", "after_current"] = "after_current"
    amount: float = Field(default=0, ge=0, le=10_000_000)
    note: str = Field(default="", max_length=500)
    claim_id: int | None = None


class CourseIn(BaseModel):
    title: str = Field(min_length=1, max_length=120)
    description: str = Field(default="", max_length=1000)
    price: float = Field(ge=0, le=10_000_000)
    default_days: int = Field(ge=1, le=3650)
    classroom_link: str = Field(default="", max_length=500)
    is_active: bool = True

    @field_validator("classroom_link")
    @classmethod
    def https_only(cls, v: str) -> str:
        return _https_or_empty(v, "Classroom link")


class PaymentMethodIn(BaseModel):
    label: str = Field(default="", max_length=60)
    upi_id: str = Field(default="", max_length=120)
    payee_name: str = Field(default="", max_length=80)
    is_active: bool = True

    @field_validator("label", "payee_name")
    @classmethod
    def strip(cls, v: str) -> str:
        return v.strip()

    @field_validator("upi_id")
    @classmethod
    def valid_upi(cls, v: str) -> str:
        v = v.strip()
        if v and not UPI_RE.match(v):
            raise ValueError("Enter a valid UPI ID, like name@okaxis")
        return v


class QrImageIn(BaseModel):
    # Base64 image data (PNG, JPEG or WEBP), with or without a data: URL prefix.
    data: str = Field(min_length=10, max_length=3_000_000)


class AnnouncementIn(BaseModel):
    title: str = Field(min_length=1, max_length=120)
    message: str = Field(default="", max_length=3000)
    link_url: str = Field(default="", max_length=500)
    link_label: str = Field(default="", max_length=60)
    course_id: int | None = None

    @field_validator("title", "message", "link_label")
    @classmethod
    def strip(cls, v: str) -> str:
        return v.strip()

    @field_validator("link_url")
    @classmethod
    def https_only(cls, v: str) -> str:
        return _https_or_empty(v, "Link")
