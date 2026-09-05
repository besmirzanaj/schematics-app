#!/usr/bin/env bash
# Offsite backup: sqlite snapshot (online-safe WAL) + file mirror.
# Env: SKEMAT_DB, SKEMAT_DATA, BACKUP_VPS (user@host), BACKUP_DIR (default /srv/backups/skemat)
set -euo pipefail
: "${SKEMAT_DB:=/srv/skemat/data/skemat.db}"
: "${SKEMAT_DATA:=/srv/skemat/data/live}"
: "${BACKUP_VPS:?set BACKUP_VPS=user@host}"
: "${BACKUP_DIR:=/srv/backups/skemat}"
mkdir -p "$BACKUP_DIR/db"
TS=$(date +%Y%m%d-%H%M%S)
# 1. sqlite online backup copy
sqlite3 "$SKEMAT_DB" ".backup '$BACKUP_DIR/db/skemat-$TS.db'" 2>/dev/null \
  || python3 - "$SKEMAT_DB" "$BACKUP_DIR/db/skemat-$TS.db" <<'PY'
import sqlite3, sys
src = sqlite3.connect(sys.argv[1])
dst = sqlite3.connect(sys.argv[2])
src.backup(dst)
dst.close(); src.close()
PY
# 2. retain 7 snapshots
find "$BACKUP_DIR/db" -name 'skemat-*.db' -mtime +7 -delete
# 3. mirror the live data tree
rsync -aHK --delete "$SKEMAT_DATA/" "$BACKUP_VPS:$BACKUP_DIR/data/"
echo "backup complete: $TS"