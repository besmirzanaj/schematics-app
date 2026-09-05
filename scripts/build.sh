#!/usr/bin/env bash
# Cross-compile the server for the Linux VPS (amd64). Running on macOS would
# otherwise produce a darwin binary that cannot run on the deploy target.
set -euo pipefail
cd "$(dirname "$0")/.."
export GO111MODULE=on
mkdir -p bin
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -trimpath -ldflags="-s -w" -o bin/skemat-server ./cmd/server
echo "built bin/skemat-server (linux/amd64)"