#!/usr/bin/env bash
# build-backend.sh — Compile the Go backend using Docker
# TalesMUD v0.5.0 requires Go 1.26; this script uses the official Go Docker image.
# Build is done with -buildvcs=false because we run inside a bind-mount without .git ownership.

set -euo pipefail

GAME_DIR="$(cd "$(dirname "$0")/.." && pwd)"
echo "=== Building TalesMUD Backend (Go) ==="
echo "Working directory: $GAME_DIR"

docker run --rm \
  -v "$GAME_DIR":/app \
  -v talesmud-gomod:/go/pkg/mod \
  -v talesmud-gocache:/root/.cache/go-build \
  -w /app \
  golang:1.26 \
  sh -c "
    go mod tidy &&
    go build -buildvcs=false -o talesmud-server ./cmd/tales/
  "

if [ $? -eq 0 ]; then
  echo "✅ Backend build successful: talesmud-server"
else
  echo "❌ Backend build failed."
  exit 1
fi