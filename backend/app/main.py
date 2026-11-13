from contextlib import asynccontextmanager

from fastapi import FastAPI, Request
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import JSONResponse

from .db import close_db, init_db
from .routers import admin, public, student

# Addresses the frontend is opened from. Only these may call the API with login cookies.
# Add your live site here when you deploy, e.g. "https://gcation.in".


@asynccontextmanager
async def lifespan(_app: FastAPI):
    init_db()
    yield
    close_db()


app = FastAPI(title="G-CATION", lifespan=lifespan, docs_url=None, redoc_url=None)


@app.middleware("http")
async def require_json_for_writes(request: Request, call_next):
    # Cross-site HTML forms cannot send application/json, so this blocks CSRF on every write.
    if request.url.path.startswith("/api/") and request.method in {"POST", "PUT", "PATCH", "DELETE"}:
        if not request.headers.get("content-type", "").startswith("application/json"):
            return JSONResponse({"detail": "Expected a JSON request."}, status_code=415)
    return await call_next(request)


# Connects the separately served frontend to this API. Added last so it runs first
# and preflight (OPTIONS) requests are answered before the JSON check above.
app.add_middleware(
    CORSMiddleware,
    allow_origins=[
        "http://127.0.0.1:5500",
        "http://localhost:5500",
    ],
    allow_credentials=True,
    allow_methods=["GET", "POST", "PUT", "DELETE"],
    allow_headers=["Content-Type"],
)

app.include_router(public.router)
app.include_router(student.router)
app.include_router(admin.router)
