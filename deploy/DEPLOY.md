# Production deploy — skemat.zanaj.pp.ua

Human-operated runbook for Linux VPSs (tested on Rocky/Alma 9; works on
Debian/Ubuntu). Two entry points:

- **Fresh deploy** — greenfield VPS, below.
- **Migrate an existing instance to a new VPS** — bottom section.

Deploy is deliberately boring: one static Go binary, a prebuilt SQLite index,
a read-only file tree, outbound-only access via Cloudflare Tunnel. The software
pipeline (build, ingest source, convert SWF, ingest) runs once during setup —
after that nothing on the VPS changes except backups.

Everything runs as root; SSH as the admin user and `sudo -i` first.

## 0. Quick facts (verified final ingest)

    73 makes, 2365 models, 3156 systems, 30361 objects (files)
    source tree  : 30361 files / 2.3 GB, under /srv/skemat/source
    data/live    : the normalized served tree (same rel_path as DB)
    skemat.db    : SQLite with catalog + auth + entitlements

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

## 3. Upload source + build the index ON the VPS

    rsync -avz --info=progress2 \
      "/Users/bzanaj/D_LENOVO_T480/personal_docs_to_clean/GlobalJig Skemat/Skemat" \
      root@<VPS>:/srv/skemat/source/
    cd /srv/skemat && python3 ingest/ingest.py --source /srv/skemat/source \
      --dest /srv/skemat/data/live --db /srv/skemat/data/skemat.db \
      --schema /srv/skemat/schema.sql
    # -> makes=73 models=2365 systems=3156 objects=30361

### 3a. Convert SWF schematics to PDF (one-shot)

The archive keeps the original `.swf` files (Flash), which no browser can
render since Adobe retired the player. Convert them once on the Mac, ship the
PDFs, then drop the SWFs from the VPS source tree so the ingest serves only
`pdf`.

Prereqs on the Mac: classic `swfrender` (the brew bottle lacks `-r`) and
ImageMagick:

    brew install imagemagick
    ingest/fetch_swfrender.sh      # builds swfrender with -r into ingest/bin/

Convert (runs over all `**/*.swf`, writes a mirrored `swfout/` tree; report in
`swfout/swf_conversion_report.txt`):

    python3 ingest/convert_swf.py --src "/…/GlobalJig Skemat/Skemat" --out ingest/swfout

Verified: 3559/3559 rendered at 288 dpi (3280x2332 px), every output non-blank.
Then ship and re-ingest:

    rsync -avz ingest/swfout/ root@<VPS>:/srv/skemat/source/
    ssh root@<VPS> 'find /srv/skemat/source -name "*.swf" -delete'
    ssh root@<VPS> 'cd /srv/skemat && python3 ingest/ingest.py --source /srv/skemat/source \
        --dest /srv/skemat/data/live --db /srv/skemat/data/skemat.db \
        --schema /srv/skemat/schema.sql'   # objects stays 30361, kind now pdf
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
    # -> makes=73 models=2365 systems=3156 objects=30361
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

---

## Migrate an existing instance to a new VPS

The data is static, so migration is a copy job, not a rebuild — no need to
re-run the ingest/SWF pipeline. Run when buying a better VPS. Two parts: ship
the data, then re-provision the runtime.

### A. Ship the data (new VPS)

    rsync -avz --delete \
      /srv/skemat/source/ root@<new>:~/skemat-source/
    rsync -avz --delete \
      /srv/skemat/data/   root@<new>:~/skemat-data/
    scp /srv/skemat/data/skemat.db root@<new>:~/skemat-data/skemat.db
    scp /srv/skemat/schema.sql /etc/skemat.env /etc/skemat-backup.env \
        root@<new>:/root/skemat-conf/

`skemat.db` is an online-consistent SQLite snapshot (WAL) — take it while the
app runs. Copy `data/live` (the served tree) and `source` verbatim.

### B. Re-provision the new VPS

Run fresh install steps 1, 2 (service user, app binary, `netdata` not needed),
then instead of step 3's ingest:

    useradd --system --home /srv/skemat --shell /usr/sbin/nologin skemat
    mkdir -p /srv/skemat/{data/live,bin,source}
    rsync -avz root@<old>:~/skemat-data/ /srv/skemat/data/
    mv <conf>/skemat.db /srv/skemat/data/skemat.db     # replace any stub
    rsync -avz root@<old>:~/skemat-source/ /srv/skemat/source/
    cp /root/skemat-conf/schema.sql /srv/skemat/schema.sql
    cp /root/skemat-conf/skemat.env /etc/skemat.env
    chown -R skemat:skemat /srv/skemat
    # sanity: row counts match the old DB, and data/live count == source count
    sqlite3 /srv/skemat/data/skemat.db "select count(*) from objects;"  # 30361

Then steps 5-8: install the unit, wire the Cloudflare Tunnel, re-create the
Access application, point the backup timer at the same backup host. Update any
DNS/backup references if the hostname changed.

No re-ingest, no SWF conversion — those ran once on the original and the
artifacts (live tree + skemat.db) are the source of truth for serving.