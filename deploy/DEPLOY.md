# Production deploy — skemat.zanaj.pp.ua

Human-operated runbook for Linux VPSs (tested on Rocky/Alma 9; works on
Debian/Ubuntu). Two entry points:

- **Fresh deploy** — greenfield VPS, below.
- **Migrate an existing instance to a new VPS** — bottom section.

Deploy is deliberately boring: one static Go binary, a prebuilt SQLite index,
a read-only file tree, outbound-only access via Cloudflare Tunnel. The software
pipeline (build, ingest source, convert SWF, ingest) runs once during setup —
after that the VPS is static, and the Mac remains the source of truth for DR.

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

## 8. Disaster recovery — rebuild on a new VPS from scratch

The VPS is disposable. Nothing on it is irreplaceable: the archive and the
conversion toolchain live on the Mac, so every artifact can be rebuilt. There
is no offsite backup host to babysit — evidence-of-life is that a fresh VPS
passes this procedure end to end.

Recovery targets:

    /srv/skemat/source        30,361 files (2.3 GB) from the Mac archive
    /srv/skemat/data/live     the served tree, rebuilt by the ingest
    /srv/skemat/data/skemat.db  rebuilt by the ingest (73/2365/3156/30361)
    /etc/skemat.env           admin email + paths (6 lines, below)

DR run (root on the new VPS, ~1-2 h -- almost all of it the upload):

    1.  Provision a Linux VPS (Rocky/Alma 9 or Debian/Ubuntu are fine).
    2.  useradd --system --home /srv/skemat --shell /usr/sbin/nologin skemat
        mkdir -p /srv/skemat/{data/live,bin,source}
    3.  Build the app from the repo (Mac):  ./scripts/build.sh
        scp bin/skemat-server root@<new>:/usr/local/bin/skemat-server
        scp -r ingest root@<new>:/srv/skemat/ingest
        scp internal/store/schema.sql root@<new>:/srv/skemat/schema.sql
        scp deploy/skemat.service /etc/systemd/system/skemat.service
        scp ingest/convert_swf.py ingest/fetch_swfrender.sh /srv/skemat/ingest/
    4.  Ship the archive (Mac):
        rsync -avz --info=progress2 \
          "/Users/bzanaj/D_LENOVO_T480/.../GlobalJig Skemat/Skemat" \
          root@<new>:/srv/skemat/source/
    5.  Convert SWF once (Mac), then ship the PDFs and drop the SWFs on the
        new VPS (reproduces the live catalog exactly -- see step 3a above).
        If the lost VPS is still reachable, skip 4-5 and rsync its
        /srv/skemat/{source,data} instead -- same result.
    6.  systemctl daemon-reload && systemctl enable --now skemat
        Test: curl http://127.0.0.1:8080/healthz   # -> ok
    7.  Tunnel: recreate the Zero Trust tunnel + public hostname from the
        dashboard, then on the VPS (cloudflared already installed):
        sudo cloudflared service install <token-from-dashboard>
    8.  Verify publicly + smoke (Mac):
        curl https://skemat.zanaj.pp.ua/healthz                        # ok
        SKEMAT_ADMIN_EMAIL=... SKEMAT_ADMIN_PW=... scripts/smoke.sh    # SMOKE OK

There is no state worth backing up: every file, the DB, and the admin account
are deterministic outputs of the ingest + a 6-line env file. If the mailbox
email changes, update /etc/skemat.env and re-run step 6.

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
    scp /srv/skemat/schema.sql /etc/skemat.env \
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

Then steps 5-7: install the unit, wire the Cloudflare Tunnel, re-create the
Access application. Update DNS/any hostname references if the name changed.

No re-ingest, no SWF conversion — those ran once on the original and the
artifacts (live tree + skemat.db) are the source of truth for serving.
If the old VPS is gone, use the disaster-recovery procedure in section 8
instead (rebuild from the Mac archive).