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

## 6. Cloudflare Tunnel (zero inbound ports)

Requires a Cloudflare account with `zanaj.pp.ua` in its zone. Run on the VPS.

    curl -L https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64 \
      -o /usr/local/bin/cloudflared && chmod +x /usr/local/bin/cloudflared
    cloudflared tunnel login          # browser flow with your Cloudflare account
    cloudflared tunnel create skemat  # writes ~/.cloudflared/<id>.json
    cp ~/.cloudflared/*.json /etc/cloudflared/skemat.json
    cp deploy/cloudflared-config.yml /etc/cloudflared/config.yml
    cloudflared tunnel route dns skemat skemat.zanaj.pp.ua   # creates CNAME
    cloudflared service install       # systemd unit
    systemctl restart cloudflared
    curl -s https://skemat.zanaj.pp.ua/healthz   # -> ok over the tunnel

Result: the VPS exposes zero inbound ports; all traffic arrives via the
outbound-only tunnel.

## 7. Cloudflare Access (gate /admin at the edge)

Dashboard (free up to 50 users):

- Zero Trust > Access > Applications > Add
- Type: Self-hosted; Domain: `skemat.zanaj.pp.ua/admin`
- Policy: Allow — the staff email addresses; deny everyone else
- Session duration: 24h

Result: `/admin/*` is additionally gated by Cloudflare identity. Customers
never reach the admin UI; providers get an extra factor independent of the
app's session cookie.

## 8. Nightly offsite backups

- `scp scripts/backup.sh /srv/skemat/scripts/` (mkpath `/srv/skemat/scripts` first)
- `cp deploy/skemat-backup.service deploy/skemat-backup.timer /etc/systemd/system/`
- Create `/etc/skemat-backup.env`:

      BACKUP_VPS=backup@<backup-vps>
      BACKUP_DIR=/srv/backups/skemat

- Ensure passwordless SSH from the VPS to the backup host:
  `ssh-keygen -t ed25519`, then append the public key to
  `backup@<backup-vps>:~/.ssh/authorized_keys`.
- `systemctl enable --now skemat-backup.timer`
- Test: `systemctl start skemat-backup.service && systemctl status skemat-backup`

What it does: an online SQLite snapshot (safe to take while the app streams
files — WAL), keep the last 7 daily snapshots, and rsync-mirror the served
`data/live` tree to the backup host.

## 9. Full real ingest + smoke test

After steps 1-5 are live (over the tunnel):

    cd /srv/skemat
    python3 ingest/ingest.py --source /srv/skemat/source \
      --dest /srv/skemat/data/live --db /srv/skemat/data/skemat.db \
      --schema /srv/skemat/schema.sql
    # expect roughly: makes≈60 models≈2500 systems≈9500 objects≈25000
    chown -R skemat:skemat /srv/skemat
    systemctl restart skemat && systemctl status skemat
    SKEMAT_ADMIN_EMAIL=you@zanaj.pp.ua SKEMAT_ADMIN_PW='<initial>' scripts/smoke.sh
    # -> SMOKE OK: /system/<id> -> /file/<id> (200)

Then review normalization output (canonical make folding):

    sqlite3 /srv/skemat/data/skemat.db \
      "SELECT mk.name, m.display_name FROM models m JOIN makes mk ON mk.id=m.make_id
       WHERE mk.name IN ('Mitsubishi','SsangYong','Volkswagen','Land Rover','Alfa Romeo','Dr');"

Expected: each appears once as a single canonical make. If a display name is
wrong, adjust `ingest/aliases.py`, re-run the ingest (idempotent), restart.
First admin login uses `changeme` until you set a real password from `/admin`.