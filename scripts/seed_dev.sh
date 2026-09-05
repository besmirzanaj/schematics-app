#!/usr/bin/env bash
# Local dev: ingest a tiny fixture + run the server with an admin.
set -euo pipefail
cd "$(dirname "$0")/.."
FIX="$(mktemp -d)"
mkdir -p "$FIX/Skemat/datasht-2013/audi/a3[2012]/2058"
printf 'x' > "$FIX/Skemat/datasht-2013/audi/a3[2012]/2058/2058_1.pdf"
python3 ingest/ingest.py --source "$FIX/Skemat" --dest data/live --db data/skemat.db \
  --schema internal/store/schema.sql
SKEMAT_ADMIN_EMAIL="${SKEMAT_ADMIN_EMAIL:-admin@example.com}" \
SKEMAT_DATA="$PWD/data/live" SKEMAT_DB="$PWD/data/skemat.db" \
  go run ./cmd/server
