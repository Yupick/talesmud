# talesmud-scripts — Helpers for TalesMUD v0.5.0 development

This directory contains scripts to build, test, and run the TalesMUD fork.

## Available scripts

| Script | Description |
|---|---|
| `setup.sh` | One‑time environment setup: downloads Go modules, creates `.env` from `.env.example`, installs Node deps. |
| `build-backend.sh` | Compiles the Go server via Docker (Go 1.26, `-buildvcs=false`). Output: `talesmud-server` binary. |
| `build-frontend.sh` | Runs `npm run build` inside `public/mud-client`; generates `public/bundle.js` etc. |
| `run-dev.sh` | Starts the full stack with `docker compose up`. Prompts for profile: (1) local auth, no LLM, or (2) local auth + Ollama. |
| `test-backend.sh` | Runs `go build -buildvcs=false ./...` and `go test -count=1` on key packages (service, entities, classkit, balance, messages). |
| `test-frontend.sh` | Runs the JS test suite via `node --test`. Known preexisting failures: `websocketGate_test.mjs` (Node 18 WebSocket), `fogOverlay_test.mjs` (canvas). All others should pass. |
| `REGRESSION_REPORT.md` *(root docs/qa/)* | Full regression report from QA verification (PASS with conditions). |

## Quick start

```bash
# 1. Setup (once)
./scripts/setup.sh

# 2. Build backend
./scripts/build-backend.sh

# 3. Build frontend
./scripts/build-frontend.sh

# 4. Run dev (choose profile)
./scripts/run-dev.sh
```

## Requirements

- **Docker** + **Docker Compose** (for backend + compose profile)
- **Node.js v18** (the repo’s local version; `public/mud-client/package.json` engines.node = "18")
- **Go 1.26** (only used inside the Docker container; host Go is not required)
- **Git** (for submodules / module caching)
- Optional: **Ollama** service running at `http://host.docker.internal:11434` if using the `with-ollama` profile.

## Notes

- The build always requires `-buildvcs=false` because we mount the repo into Docker via a bind‑mount that may not preserve git ownership.
- Frontend tests on Node 18 have two known failures unrelated to recent changes (WebSocket global + canvas). Do not modify those files.
- All scripts assume they are run from the repo root (`mi-mud/`).