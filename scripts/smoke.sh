#!/usr/bin/env bash
# Smoke: assert auth + catalog + file ACL over the live origin.
# Env: SKEMAT_ADMIN_EMAIL, SKEMAT_ADMIN_PW, SKEMAT_BASE (default https://skemat.zanaj.pp.ua)
set -euo pipefail
BASE="${SKEMAT_BASE:-https://skemat.zanaj.pp.ua}"
J=$(mktemp)
trap 'rm -f "$J" /tmp/skobj' EXIT
# healthz (public)
[ "$(curl -s -o /dev/null -w '%{http_code}' "$BASE/healthz")" = "200" ]
# anonymous home -> 303 to login
[ "$(curl -s -o /dev/null -w '%{http_code}' "$BASE/")" = "303" ]
# admin login
curl -s -c "$J" -o /dev/null -X POST -d "email=${SKEMAT_ADMIN_EMAIL}&password=${SKEMAT_ADMIN_PW}" "$BASE/login"
# catalog responds after auth
[ "$(curl -s -b "$J" -o /dev/null -w '%{http_code}' "$BASE/")" = "200" ]
# pick a real system via search, then fetch its first file
SYS=$(curl -s -b "$J" "$BASE/search?q=Audi" | grep -oE '/system/[0-9]+' | head -1)
[ -n "$SYS" ]
OBJ=$(curl -s -b "$J" "$BASE$SYS" | grep -oE '/file/[0-9]+' | head -1)
[ -n "$OBJ" ]
F=$(curl -s -b "$J" -o /tmp/skobj -w '%{http_code}' "$BASE$OBJ")
[ "$F" = "200" ] && [ -s /tmp/skobj ]
echo "SMOKE OK: $SYS -> $OBJ ($F)"