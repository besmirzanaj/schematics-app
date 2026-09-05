# Production deploy — skemat.zanaj.pp.ua

Human-operated runbook. Execute on the VPS as root (Debian/Ubuntu). Steps 1-2
build and ship the app; step 3 uploads the raw schematics tree and builds the
local normalized layout + SQLite index on the VPS.

## 1. Create service user + dirs

    useradd --system --home /srv/skemat --shell /usr/sbin/nologin skemat
    mkdir -p /srv/skemat/{data/live,bin}

## 2. Copy app

Build from the Mac (produces linux/amd64):

    ./scripts/build.sh            # -> bin/skemat-server

Ship:

    scp bin/skemat-server root@<VPS>:/usr/local/bin/skemat-server && chmod +x /usr/local/bin/skemat-server
    scp -r ingest root@<VPS>:/srv/skemat/ingest
    scp internal/store/schema.sql root@<VPS>:/srv/skemat/schema.sql

## 3. Upload the normalized data + build the index ON the VPS

    rsync -avz --info=progress2 \
      "/Users/bzanaj/D_LENOVO_T480/personal_docs_to_clean/GlobalJig Skemat/Skemat" \
      root@<VPS>:/srv/skemat/source/
    cd /srv/skemat && python3 ingest/ingest.py --source /srv/skemat/source \
      --dest /srv/skemat/data/live --db /srv/skemat/data/skemat.db \
      --schema /srv/skemat/schema.sql
    chown -R skemat:skemat /srv/skemat

## 4. Env file — /etc/skemat.env

    SKEMAT_ADDR=127.0.0.1:8080
    SKEMAT_DATA=/srv/skemat/data/live
    SKEMAT_DB=/srv/skemat/data/skemat.db
    SKEMAT_ADMIN_EMAIL=you@zanaj.pp.ua
    SKEMAT_SECURE_COOKIES=1

Remember: after first boot the admin login starts with the default password
`changeme` — change it immediately from the `/admin` UI (Reset password form).

## 5. Install unit

    cp deploy/skemat.service /etc/systemd/system/skemat.service
    systemctl daemon-reload && systemctl enable --now skemat
    systemctl status skemat
    curl -s http://127.0.0.1:8080/healthz   # -> ok