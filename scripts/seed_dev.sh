#!/usr/bin/env bash
# Local dev: ingest the bundled sample schematics + run the server with an admin.
# Login: use SKEMAT_ADMIN_EMAIL (default admin@example.com) with password 'changeme'.
set -euo pipefail
cd "$(dirname "$0")/.."
GO111MODULE=on
python3 ingest/ingest.py --source sample/source --dest data/live --db data/skemat.db \
  --schema internal/store/schema.sql
SKEMAT_ADMIN_EMAIL="${SKEMAT_ADMIN_EMAIL:-admin@example.com}" \
SKEMAT_DATA="$PWD/data/live" SKEMAT_DB="$PWD/data/skemat.db" \
  go run ./cmd/server