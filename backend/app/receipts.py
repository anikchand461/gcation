"""Payment receipts: a PDF built from the saved access grant (no external service involved).

Each access grant the admin gives is one receipt. The receipt number comes from the grant's id,
so the same grant always produces the same receipt.
"""
from datetime import datetime
from pathlib import Path
from zoneinfo import ZoneInfo

from fastapi import HTTPException, Response
from fpdf import FPDF
from psycopg import Connection

from .access import manager_code, student_code

IST = ZoneInfo("Asia/Kolkata")
LOGO = Path(__file__).parent / "assets" / "logo.png"
INK = (20, 15, 46)
MUTED = (103, 98, 138)
BRAND = (91, 33, 182)
LINE = (232, 228, 247)
TINT = (247, 245, 255)

# The payment note (claim) is joined in when the admin verified the grant against one.
RECEIPT_SQL = (
    "SELECT e.id, e.course_id, e.start_at, e.end_at, e.amount_paid, e.note, e.granted_at, e.revoked_at, e.student_id, "
    "s.name AS student_name, s.email AS student_email, s.phone AS student_phone, s.student_no, c.title AS course_title, "
    "p.pay_mode, p.payer_name, p.method AS paid_using, p.paid_on, p.reference, p.age, p.referred_by, p.referral_code, p.manager_id, m.label AS paid_to "
    "FROM enrollments e JOIN students s ON s.id = e.student_id JOIN courses c ON c.id = e.course_id "
    "LEFT JOIN payment_claims p ON p.id = e.claim_id LEFT JOIN payment_methods m ON m.slot = p.qr_slot "
)


def receipt_no(grant: dict) -> str:
    year = datetime.fromtimestamp(grant["granted_at"], IST).year
    return f"GC-{year}-{grant['id']:05d}"


def receipt_summary(grant: dict) -> dict:
    """What lists of receipts show (no PDF)."""
    return {
        "id": grant["id"],
        "receipt_no": receipt_no(grant),
        "course_id": grant["course_id"],
        "course_title": grant["course_title"],
        "amount_paid": grant["amount_paid"],
        "granted_at": grant["granted_at"],
        "start_at": grant["start_at"],
        "end_at": grant["end_at"],
        "revoked_at": grant["revoked_at"],
    }


def _date(ts: int) -> str:
    return datetime.fromtimestamp(ts, IST).strftime("%d %b %Y")


def _paid_on(value: str | None) -> str:
    """'2026-11-15' -> '15 Nov 2026'; anything else is shown as typed."""
    try:
        return datetime.strptime(value or "", "%Y-%m-%d").strftime("%d %b %Y")
    except ValueError:
        return value or ""


def _inr(amount) -> str:
    """Indian digit grouping: 123456.5 -> 'INR 1,23,456.50'."""
    whole, frac = f"{float(amount):.2f}".split(".")
    head, tail = whole[:-3], whole[-3:]
    groups = []
    while len(head) > 2:
        groups.insert(0, head[-2:])
        head = head[:-2]
    if head:
        groups.insert(0, head)
    return "INR " + ",".join(groups + [tail]) + "." + frac


def _t(text) -> str:
    # The built-in PDF fonts cover Latin-1 only; anything else (e.g. emoji) becomes "?".
    return str(text or "").encode("latin-1", "replace").decode("latin-1")


def build_pdf(g: dict, contact_phone: str = "") -> bytes:
    pdf = FPDF(format="A4")
    pdf.set_auto_page_break(False)
    pdf.add_page()
    pdf.set_margins(18, 18, 18)
    width = pdf.w - 36

    # Header band
    pdf.set_fill_color(*TINT)
    pdf.rect(0, 0, pdf.w, 48, style="F")
    pdf.set_fill_color(*BRAND)
    pdf.rect(0, 48, pdf.w, 1.2, style="F")
    if LOGO.exists():
        pdf.image(str(LOGO), x=18, y=11, h=26)
    pdf.set_xy(47, 15)
    pdf.set_font("Helvetica", "B", 20)
    pdf.set_text_color(*BRAND)
    pdf.cell(0, 9, "G-CATION")
    pdf.set_xy(47, 25)
    pdf.set_font("Helvetica", "", 9.5)
    pdf.set_text_color(*MUTED)
    pdf.cell(0, 6, "Learn. Grow. Succeed.")
    pdf.set_xy(18, 14)
    pdf.set_font("Helvetica", "B", 15)
    pdf.set_text_color(*INK)
    pdf.cell(width, 8, "PAYMENT RECEIPT", align="R")
    pdf.set_xy(18, 23)
    pdf.set_font("Helvetica", "", 9.5)
    pdf.set_text_color(*MUTED)
    pdf.cell(width, 5.5, f"Receipt no.  {receipt_no(g)}", align="R")
    pdf.set_xy(18, 29)
    pdf.cell(width, 5.5, f"Date  {_date(g['granted_at'])}", align="R")

    # Billed to + status
    y = 62
    pdf.set_xy(18, y)
    pdf.set_font("Helvetica", "B", 8.5)
    pdf.set_text_color(*MUTED)
    pdf.cell(0, 5, "BILLED TO")
    pdf.set_xy(18, y + 6)
    pdf.set_font("Helvetica", "B", 13)
    pdf.set_text_color(*INK)
    pdf.cell(0, 7, _t(g["student_name"]))
    pdf.set_font("Helvetica", "", 10)
    pdf.set_text_color(*MUTED)
    code = student_code(g["student_no"])
    lines = [line for line in (f"Student ID: {code}" if code else "", _t(g["student_email"]), _t(g["student_phone"])) if line]
    if g.get("age"):
        lines.append(f"Age: {g['age']}")
    for i, line in enumerate(lines):
        pdf.set_xy(18, y + 14 + i * 5.5)
        pdf.cell(0, 5.5, line)

    revoked = g["revoked_at"] is not None
    stamp, color = ("ACCESS REVOKED", (190, 18, 60)) if revoked else ("PAID", (4, 120, 87))
    pdf.set_draw_color(*color)
    pdf.set_text_color(*color)
    pdf.set_line_width(0.6)
    pdf.set_font("Helvetica", "B", 12)
    stamp_w = pdf.get_string_width(stamp) + 12
    pdf.set_xy(18 + width - stamp_w, y + 4)
    pdf.cell(stamp_w, 10, stamp, border=1, align="C")

    # Line items
    y = 104
    cols = [(width * 0.46, "Description", "L"), (width * 0.32, "Access period", "L"), (width * 0.22, "Amount", "R")]
    pdf.set_fill_color(*TINT)
    pdf.set_draw_color(*LINE)
    pdf.set_line_width(0.3)
    pdf.set_xy(18, y)
    pdf.set_font("Helvetica", "B", 9)
    pdf.set_text_color(*MUTED)
    for w, label, align in cols:
        pdf.cell(w, 10, "  " + label + "  ", align=align, fill=True)
    pdf.set_xy(18, y + 10)
    pdf.set_font("Helvetica", "", 10.5)
    pdf.set_text_color(*INK)
    period = f"{_date(g['start_at'])} - {_date(g['end_at'])}"
    for (w, _, align), value in zip(cols, (_t(g["course_title"]), period, _inr(g["amount_paid"]))):
        pdf.cell(w, 12, "  " + value + "  ", align=align, border="B")

    pdf.set_xy(18, y + 26)
    pdf.set_font("Helvetica", "B", 11.5)
    pdf.cell(cols[0][0] + cols[1][0], 10, "Total paid  ", align="R")
    pdf.set_text_color(*BRAND)
    pdf.cell(cols[2][0], 10, _inr(g["amount_paid"]) + "  ", align="R")

    # Payment details: what the student sent with the payment, two per row.
    y = 150
    details = [
        (label, value)
        for label, value in (
            ("Paid to", g.get("paid_to")),
            ("Paid using", g.get("paid_using")),
            ("Paid by" if g.get("pay_mode") == "cash" else "Paid by (account name)", g.get("payer_name")),
            ("Payment date", _paid_on(g.get("paid_on"))),
            ("UTR / transaction ID", g.get("reference")),
            ("Referred by", g.get("referred_by")),
            ("Referral ID", manager_code(g.get("manager_id")) or g.get("referral_code")),
        )
        if value
    ]
    if details:
        pdf.set_xy(18, y)
        pdf.set_font("Helvetica", "B", 8.5)
        pdf.set_text_color(*MUTED)
        pdf.cell(0, 5, "PAYMENT DETAILS")
        y += 8
        half = width / 2
        for i, (label, value) in enumerate(details):
            x = 18 + (i % 2) * half
            row_y = y + (i // 2) * 13
            pdf.set_xy(x, row_y)
            pdf.set_font("Helvetica", "", 8.5)
            pdf.set_text_color(*MUTED)
            pdf.cell(half - 4, 4.5, label)
            pdf.set_xy(x, row_y + 4.8)
            pdf.set_font("Helvetica", "B", 10.5)
            pdf.set_text_color(*INK)
            pdf.cell(half - 4, 6, _t(value))
        y += ((len(details) + 1) // 2) * 13 + 4
    if g["note"]:
        pdf.set_xy(18, y)
        pdf.set_font("Helvetica", "B", 8.5)
        pdf.set_text_color(*MUTED)
        pdf.cell(0, 5, "NOTES" if details else "PAYMENT DETAILS")
        pdf.set_xy(18, y + 6)
        pdf.set_font("Helvetica", "", 10)
        pdf.set_text_color(*INK)
        pdf.multi_cell(width, 5.5, _t(g["note"]))
        y = pdf.get_y() + 6
    if revoked:
        pdf.set_xy(18, y)
        pdf.set_font("Helvetica", "B", 10)
        pdf.set_text_color(190, 18, 60)
        pdf.cell(0, 6, f"Access for this period was revoked on {_date(g['revoked_at'])}.")

    # Footer
    pdf.set_draw_color(*LINE)
    pdf.line(18, 268, 18 + width, 268)
    pdf.set_xy(18, 271)
    pdf.set_font("Helvetica", "", 8.5)
    pdf.set_text_color(*MUTED)
    contact = f"  |  Support: {contact_phone}" if contact_phone else ""
    pdf.cell(width, 5, _t(f"This is a computer-generated receipt and does not need a signature.{contact}"), align="C")
    return bytes(pdf.output())


def load_grant(db: Connection, enrollment_id: int, student_id: int | None = None) -> dict:
    sql = RECEIPT_SQL + "WHERE e.id = %s"
    params: tuple = (enrollment_id,)
    if student_id is not None:  # students may only fetch their own receipts
        sql += " AND e.student_id = %s"
        params += (student_id,)
    grant = db.execute(sql, params).fetchone()
    if not grant:
        raise HTTPException(404, "Receipt not found.")
    return grant


def pdf_response(grant: dict, contact_phone: str = "") -> Response:
    return Response(
        build_pdf(grant, contact_phone),
        media_type="application/pdf",
        headers={
            "Content-Disposition": f'attachment; filename="G-CATION-receipt-{receipt_no(grant)}.pdf"',
            "Cache-Control": "private, no-store",
        },
    )
