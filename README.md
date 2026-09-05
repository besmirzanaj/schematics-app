# Schematics Web App

Paid automotive-schematics viewer. Go backend + HTMX + SQLite on a VPS behind Cloudflare.

- Data source: `sample/source/` (small demo tree) — or the full `../Skemat/` tree
  (datasht-2005/2008/2013 + Te tjera) for production
- Ingest: `ingest/ingest.py` (Python stdlib only) -> normalized tree in `data/live` + `data/skemat.db`
- Server: `cmd/server` (systemd unit + runbook in `deploy/`)
- Stack: Go 1.27, modernc.org/sqlite, HTMX 2.0.4 vendored

## Quick start (sample data)

    python3 -m unittest ingest/test_ingest.py   # golden ingest tests
    go test ./...
    ./scripts/seed_dev.sh

Then open http://127.0.0.1:8080 — log in as `admin@example.com`, password `changeme`
(change it from `/admin`; `SKEMAT_ADMIN_EMAIL` overrides the username).

The bundled sample (`sample/source/`) intentionally exercises the normalization
quirks: a `(AUS)` region tree, the `pdf` reference folder (staff-only),
`Te tjera` (staff-only), and systems mixing pdf/png/swf.

## Run locally against the real data

    go get modernc.org/sqlite golang.org/x/crypto/bcrypt
    go build ./...
    python3 ingest/ingest.py --source ../Skemat --dest data/live --db data/skemat.db \
      --schema internal/store/schema.sql
    SKEMAT_DATA=./data/live SKEMAT_DB=./data/skemat.db \
    SKEMAT_ADMIN_EMAIL=you@example.com go run ./cmd/server