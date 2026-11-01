#!/usr/bin/env bash
# test-backend.sh — Run Go test suite (buildvcs=false, count=1)
# Verifies the core packages without network/LLM dependencies when possible.

set -euo pipefail

GAME_DIR="$(cd "$(dirname "$0")/.." && pwd)"
echo "=== Running TalesMUD Go Tests ==="
echo "Working directory: $GAME_DIR"

# pkg/webui and pkg/webuiplay embed dist/ (git-ignored). A clean checkout has
# no dist/, so go:embed fails unless the fallback assets exist first.
make -s -C "$GAME_DIR" prepare-embedded-assets

docker run --rm \
  -v "$GAME_DIR":/app \
  -v talesmud-gomod:/go/pkg/mod \
  -v talesmud-gocache:/root/.cache/go-build \
  -w /app \
  golang:1.26 \
  sh -c "
    go build -buildvcs=false ./... &&
    go test -buildvcs=false -count=1 ./pkg/service/... ./pkg/entities/... ./pkg/classkit/... ./pkg/mudserver/game/balance/... ./pkg/mudserver/game/messages/... 2>&1
  "

echo ""
echo "✅ Go tests finished."