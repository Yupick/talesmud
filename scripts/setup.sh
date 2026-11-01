#!/usr/bin/env bash
# setup.sh — One-time environment setup for TalesMUD development
# Prerequisites: Docker + Docker Compose, Git

set -euo pipefail

echo "=== TalesMUD v0.5.0 — Development Environment Setup ==="

# 1. Clone submodules / ensure go modules are present
if [ ! -d "vendor" ] && [ -f "go.mod" ]; then
  echo "Downloading Go modules..."
  go mod download
fi

# 2. Ensure Docker volumes exist (named volumes for Go module cache)
echo "Ensuring Docker volumes..."
docker volume create talesmud-gomod 2>/dev/null || true
docker volume create talesmud-gocache 2>/dev/null || true

# 3. Check .env exists
if [ ! -f ".env" ]; then
  if [ -f ".env.example" ]; then
    echo "Creating .env from .env.example..."
    cp .env.example .env
    echo "⚠️  Remember to review and adjust .env (AUTH_MODE, LLM_PROVIDER, etc.)"
  else
    echo "⚠️  No .env.example found; creating minimal .env"
    cat > .env <<'EOF'
AUTH_MODE=local
LLM_PROVIDER=ollama
OLLAMA_BASE_URL=http://host.docker.internal:11434
OLLAMA_MODEL=llama3.1
GAME_PORT=8010
EOF
  fi
else
  echo ".env already present."
fi

# 4. Frontend dependencies
if [ ! -d "public/mud-client/node_modules" ]; then
  echo "Installing frontend dependencies..."
  cd public/mud-client && npm install && cd -
fi

echo ""
echo "✅ Setup complete."
echo "   - Run 'scripts/build-backend.sh' to compile the Go server"
echo "   - Run 'scripts/build-frontend.sh' to build the web client"
echo "   - Run 'scripts/run-dev.sh'  to start the game via Docker Compose"