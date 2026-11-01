#!/usr/bin/env bash
# run-dev.sh — Start the game via Docker Compose for local development
# Supports two profiles:
#   - default:      auth local + no LLM (fast, no Ollama needed)
#   - with-ollama:  adds the Ollama service (LLM enabled)
set -euo pipefail

GAME_DIR="$(cd "$(dirname "$0")/.." && pwd)"
CDOYA="$GAME_DIR/docker-compose.yml"

echo "=== TalesMUD v0.5.0 — Development Mode ==="
echo ""

# Ask which profile
echo "Choose profile:"
echo "  1) Local auth, no LLM (fast, Ollama not required)"
echo "  2) Local auth + Ollama (LLM enabled, needs Ollama running)"
read -r choice

case "$choice" in
  1)
    PROFILE=""
    echo "Starting with: local auth, LLM disabled"
    ;;
  2)
    PROFILE="--profile ollama"
    echo "Starting with: local auth + Ollama"
    echo "⚠️  Make sure Ollama is reachable at http://host.docker.internal:11434"
    ;;
  *)
    echo "Invalid choice. Exiting."
    exit 1
    ;;
esac

echo ""
docker-compose -f "$CDOYA" up $PROFILE

# On exit, also stop the services
trap "docker-compose -f '$CDOYA' down" EXIT