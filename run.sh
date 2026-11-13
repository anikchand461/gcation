#!/usr/bin/env bash
# Starts the backend (http://127.0.0.1:8000) and the frontend (http://127.0.0.1:5500) together.
# Press Ctrl+C to stop both.
set -euo pipefail
cd "$(dirname "$0")"

if [ ! -f backend/.env ]; then
  echo "backend/.env is missing. Copy backend/.env.example to backend/.env and fill it in first."
  exit 1
fi

if [ ! -x backend/.venv/bin/uvicorn ]; then
  echo "Setting up the Python environment (first run only)..."
  python3 -m venv backend/.venv
  backend/.venv/bin/pip install -q -r backend/requirements.txt
fi

(cd backend && exec .venv/bin/uvicorn app.main:app --reload --port 8000) &
BACKEND=$!
(cd frontend && exec python3 -m http.server 5500 --bind 127.0.0.1) &
FRONTEND=$!
trap 'kill $BACKEND $FRONTEND 2>/dev/null' EXIT INT TERM

echo
echo "  Website: http://127.0.0.1:5500"
echo "  Admin:   http://127.0.0.1:5500/admin-login.html"
echo "  API:     http://127.0.0.1:8000"
echo "  Press Ctrl+C to stop."
echo
wait
