#!/usr/bin/env bash
# build-frontend.sh — Build the Mud client (Svelte + Rollup) for production
set -euo pipefail

GAME_DIR="$(cd "$(dirname "$0")/.." && pwd)"
CLIENT_DIR="$GAME_DIR/public/mud-client"

echo "=== Building TalesMUD Frontend ==="
echo "Client directory: $CLIENT_DIR"

cd "$CLIENT_DIR"

# Build rollup; outputs to public/ directory (relative to client)
npm run build

if [ $? -eq 0 ]; then
  echo "✅ Frontend build successful."
  echo "   Artifacts generated in $CLIENT_DIR/public/"
else
  echo "❌ Frontend build failed."
  exit 1
fi