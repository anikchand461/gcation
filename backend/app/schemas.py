import re
from typing import Literal

from pydantic import BaseModel, Field, field_validator, model_validator

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


Role = Literal["student", "management"]  # who may sign up or log in themselves (never the admin)


MOBILE_RE = re.compile(r"^[6-9]\d{9}$")


def clean_mobile(v: str) -> str:
    """'+91 98765-43210', '098765 43210' or '9876543210' -> '9876543210'."""
    digits = re.sub(r"\D", "", v)
    if len(digits) == 12 and digits.startswith("91"):
        digits = digits[2:]
    elif len(digits) == 11 and digits.startswith("0"):
        digits = digits[1:]
    if not MOBILE_RE.match(digits):
        raise ValueError("Enter a valid 10-digit mobile number.")
    return digits


class RegisterIn(BaseModel):
    """Register with a mobile number and password (Google sign-in is the other way)."""
    name: str = Field(min_length=1, max_length=80)
    mobile: str = Field(default="", max_length=20)
    password: str = Field(min_length=8, max_length=128)
    client: Client = "web"
    email: str = Field(default="", max_length=120)  # sent only by app versions from before mobile log-in

    @field_validator("name")
    @classmethod
    def strip(cls, v: str) -> str:
        return v.strip()

    @model_validator(mode="after")
    def check(self):
        if not self.mobile and self.email:
            raise ValueError("Please update the G-CATION app to register.")
        self.mobile = clean_mobile(self.mobile)
        return self


class ManagerRegisterIn(RegisterIn):
    invite_code: str = Field(max_length=60)  # the code the admin gives the management team


class MobileIn(BaseModel):
    """Sets or changes the mobile number the logged-in account logs in with."""
    mobile: str = Field(max_length=20)

    @field_validator("mobile")
    @classmethod
    def valid(cls, v: str) -> str:
        return clean_mobile(v)


class GoogleIn(BaseModel):
    credential: str = Field(min_length=20, max_length=5000)  # the ID token from Google's sign-in button
    role: Role = "student"
    # Only for people new to G-CATION: their mobile number, and for management the referral code.
    mobile: str = Field(default="", max_length=20)
    invite_code: str = Field(default="", max_length=60)
    client: Client = "web"


class GoogleCompleteIn(BaseModel):
    """App: finishes a Google sign-up that stopped for the mobile number (and referral code)."""
    code: str = Field(min_length=32, max_length=128)
    mobile: str = Field(max_length=20)
    invite_code: str = Field(default="", max_length=60)


class InviteCodeIn(BaseModel):
    invite_code: str = Field(default="", max_length=60)  # empty closes management registration

    @field_validator("invite_code")
    @classmethod
    def no_spaces(cls, v: str) -> str:
        v = v.strip()
        if v and (len(v) < 4 or any(ch.isspace() for ch in v)):
            raise ValueError("Use at least 4 characters and no spaces.")
        return v


class LoginIn(BaseModel):
    mobile: str = Field(default="", max_length=120)  # a mobile number, or the email of an older account
    email: str = Field(default="", max_length=120)  # what app versions from before mobile log-in send
    password: str = Field(max_length=128)
    client: Client = "web"


class AdminLoginIn(BaseModel):
    username: str = Field(max_length=120)
    password: str = Field(max_length=128)
    client: Client = "web"


class AdminAccountIn(BaseModel):
    current_password: str = Field(max_length=128)
    username: str = Field(min_length=3, max_length=60)
    new_password: str = Field(default="", max_length=128)  # empty keeps the current password

    @field_validator("username")
    @classmethod
    def clean_username(cls, v: str) -> str:
        v = v.strip()
        if len(v) < 3 or any(ch.isspace() for ch in v):
            raise ValueError("Username needs at least 3 characters and no spaces.")
        return v

    @field_validator("new_password")
    @classmethod
    def strong_enough(cls, v: str) -> str:
        if v and len(v) < 8:
            raise ValueError("New password must be at least 8 characters.")
        return v


class SettingsIn(BaseModel):
    contact_phone: str = Field(default="", max_length=20)

    @field_validator("contact_phone")
    @classmethod
    def phone_chars(cls, v: str) -> str:
        v = v.strip()
        if v and not re.fullmatch(r"[0-9+\-() ]{6,20}", v):
            raise ValueError("Enter a phone number using digits, spaces, + - ( ) only.")
        return v


class ProfileIn(BaseModel):
    name: str = Field(min_length=1, max_length=80)
    phone: str = Field(default="", max_length=20)

    @field_validator("name", "phone")
    @classmethod
    def strip(cls, v: str) -> str:
        return v.strip()


class PasswordChangeIn(BaseModel):
    current_password: str = Field(default="", max_length=128)  # not needed for Google accounts with no password yet
    new_password: str = Field(min_length=8, max_length=128)


class StudentEditIn(ProfileIn):
    email: str = Field(default="", max_length=120)  # optional: accounts registered with a mobile number have none

    @field_validator("email")
    @classmethod
    def valid_email(cls, v: str) -> str:
        v = v.strip().lower()
        if v and not EMAIL_RE.match(v):
            raise ValueError("Enter a valid email address.")
        return v


class PasswordResetIn(BaseModel):
    new_password: str = Field(min_length=8, max_length=128)


class UpdatesAccessIn(BaseModel):
    enabled: bool


class ClaimIn(BaseModel):
    course_id: int
    # "cash" payments have no QR, UTR or screenshot.
    pay_mode: Literal["upi", "cash"] = "upi"
    qr_slot: QrSlot | Literal[""] = ""
    payer_name: str = Field(default="", max_length=80)
    method: str = Field(default="", max_length=60)
    amount: float | None = Field(default=None, ge=0, le=10_000_000)
    paid_on: str = Field(default="", max_length=20)
    reference: str = Field(default="", max_length=80)
    age: int | None = Field(default=None, ge=5, le=100)
    referred_by: str = Field(default="", max_length=80)
    referral_id: str = Field(default="", max_length=60)  # anything; a Management ID (GCM001) links the referrer
    # Base64 payment screenshot (PNG, JPEG or WEBP). The website requires it; older app
    # versions do not send one yet, so it stays optional here.
    screenshot: str = Field(default="", max_length=3_000_000)

    @field_validator("payer_name", "method", "paid_on", "reference", "referred_by", "referral_id")
    @classmethod
    def strip(cls, v: str) -> str:
        return v.strip()

    @model_validator(mode="after")
    def by_pay_mode(self):
        if self.pay_mode == "cash":
            self.qr_slot, self.method, self.reference, self.screenshot = "", "Cash", "", ""
        elif len(self.reference) < 6:
            raise ValueError("Enter the UTR / transaction ID from your payment app.")
        return self


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
    features: str = Field(default="", max_length=1500)  # highlights, one per line
    price: float = Field(ge=0, le=10_000_000)
    default_days: int = Field(ge=1, le=3650)

    @field_validator("features")
    @classmethod
    def tidy_features(cls, v: str) -> str:
        lines = [line.strip() for line in v.splitlines() if line.strip()]
        if len(lines) > 12:
            raise ValueError("Keep it to 12 highlights or fewer.")
        if any(len(line) > 120 for line in lines):
            raise ValueError("Each highlight can be at most 120 characters.")
        return "\n".join(lines)
    classroom_link: str = Field(default="", max_length=500)
    is_active: bool = True
    # Students must give "Referred by" and a Referral ID. None (older apps) keeps the current setting.
    referral_required: bool | None = None

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
    # Used for both payment QR images and course thumbnails.
    data: str = Field(min_length=10, max_length=3_000_000)


class AnnouncementIn(BaseModel):
    title: str = Field(min_length=1, max_length=120)
    message: str = Field(default="", max_length=3000)
    link_url: str = Field(default="", max_length=500)
    link_label: str = Field(default="", max_length=60)
    # Who sees it: any mix of the management team, every paid student and the students of some courses.
    to_management: bool = False
    to_all_students: bool | None = None
    course_ids: list[int] | None = Field(default=None, max_length=100)
    course_id: int | None = None  # older apps send one course (or none = every paid student)

    @model_validator(mode="after")
    def audience(self):
        if self.to_all_students is None and self.course_ids is None:  # older app
            self.to_all_students = self.course_id is None
            self.course_ids = [self.course_id] if self.course_id is not None else []
        self.to_all_students = bool(self.to_all_students)
        self.course_ids = sorted(set(self.course_ids or []))
        if not (self.to_management or self.to_all_students or self.course_ids):
            raise ValueError("Choose who should see this update.")
        return self

    @field_validator("title", "message", "link_label")
    @classmethod
    def strip(cls, v: str) -> str:
        return v.strip()

    @field_validator("link_url")
    @classmethod
    def https_only(cls, v: str) -> str:
        return _https_or_empty(v, "Link")
