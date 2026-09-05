#!/bin/bash
# Build classic swfrender (with -r resolution flag) into ingest/bin/.
# The Homebrew bottle is the rewritten swfrender which lacks -r; the official
# swftools tree keeps it. Requires autotools + a C++ toolchain + libjpeg,
# libpng, zlib, freetype dev headers (brew install pkg-config jpeg libpng
# freetype).
set -euo pipefail

cd "$(dirname "$0")/.."
ROOT="$PWD"
BIN="$ROOT/ingest/bin"
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

git clone -q --depth 1 https://github.com/swftools/swftools.git "$TMP/swftools"
cd "$TMP/swftools"
./configure --disable-debug --without-x >/dev/null
make -j"$(sysctl -n hw.ncpu)" src/swfrender >/dev/null
mkdir -p "$BIN"
cp src/swfrender "$BIN/swfrender"
echo "built $BIN/swfrender"
"$BIN/swfrender" -h | head -3