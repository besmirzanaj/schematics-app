# Schematics Web App

Paid automotive-schematics viewer. Go backend + HTMX + SQLite on a VPS behind Cloudflare.

- Data source: `../Skemat/` (datasht-2005/2008/2013 + Te tjera)
- Ingest: `ingest/ingest.py` (stdlib only) -> normalized tree in `data/live` + `data/skemat.db`
- Server: `cmd/server` (systemd unit in deploy/)
- Stack: Go 1.27, modernc.org/sqlite, HTMX 2.0.4 vendored

Run locally:

    go get modernc.org/sqlite golang.org/x/crypto/bcrypt
    go build ./...
    SKEMAT_DATA=./data/live SKEMAT_DB=./data/skemat.db \
    SKEMAT_ADMIN_EMAIL=you@example.com go run ./cmd/server