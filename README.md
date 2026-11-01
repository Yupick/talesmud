<div align="center">

# ⚔️ TalesMUD

### A Modern, Extensible MUD Framework for Web-Based RPGs

*Build immersive text-based multiplayer adventures in any genre — fantasy, sci-fi, horror, and beyond*

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://golang.org/)
[![Svelte](https://img.shields.io/badge/Svelte-4.2-FF3E00?logo=svelte)](https://svelte.dev/)
[![Live Demo](https://img.shields.io/badge/Demo-talesmud.io-success)](https://talesmud.io)

[**Live Demo**](https://talesmud.io) • [**Documentation**](ARCHITECTURE.md) • [**Contributing**](#contributing) • [**Roadmap**](#roadmap)

</div>

---

## 📖 Overview

**TalesMUD** is a complete, production-ready framework for building browser-based Multi-User Dungeons (MUDs) — text-based multiplayer RPGs that run entirely in modern web browsers. Unlike traditional telnet-based MUDs, TalesMUD offers a rich, real-time web experience with persistent world state, live collaboration, and a powerful content creation suite.

Built with **Go** and **Svelte**, TalesMUD provides everything you need to launch your own multiplayer text adventure:

- ⚔️ **Full RPG Systems** — Combat, character progression, skills, quests, items, and loot
- 🗺️ **World Building Tools** — Visual editors for rooms, NPCs, dialogs, and interactive maps
- 🎨 **Genre-Agnostic Design** — Easily adapt for fantasy, sci-fi, horror, steampunk, or custom settings
- 🔧 **Lua Scripting** — Extend gameplay with custom mechanics, puzzles, and events
- 🌐 **Real-Time Multiplayer** — WebSocket-based gameplay with chat, emotes, and player presence
- 🎮 **Modern UX** — Rich terminal interface, graphical overlays, mobile-responsive design
- 🔐 **Local Auth by default** — Username/password accounts (Argon2id) with built-in JWT sessions; Auth0 OAuth2 optional via `AUTH_MODE=auth0`

### 🎯 Live Demo

See the framework at **[talesmud.io](https://talesmud.io)**. A playable world is a content pack you import. This repository ships the engine and a neutral landing page.

---

## 📸 Screenshots

### In-Game Experience
![TalesMUD Game Client](screenshots/mud-client-web.png)
*Explore richly described rooms with visual backgrounds, interactive terminals, and real-time multiplayer chat*

### Social & Multiplayer
![TalesMUD Chat](screenshots/mud-client-chat.png)
*Engage in conversations with NPCs, collaborate with other players, and experience dynamic storytelling*

---

## ✨ Key Features

### New in v0.5.0

- **SSH access** — Classic text MUD and 80×25 Door clients share the engine; optional device confirmation and linked SSH keys. See [SSH access](docs/ssh-access.md) and [device login](docs/device-login.md).
- **Creator tools** — Ctrl+K search, references before deletion, content health checks, NPC/room inspectors, quest debugger, loot previews, spawner editors, map overlays, audit history and admin undo/live operations.
- **NPC movement** — Timed routes and roaming with route validation and engagement holds; NPCs remain stationary unless configured.
- **Game client** — Structured item inspection, readable effect lines, a pickup panel, optional protocol hello, and combat snapshots on reconnect. See [structured messages](docs/structured-messages.md).
- **Engine fixes** — Safe rooms suppress automatic aggression, bosses respect respawn delays and reset HP, and Door supports catalog trading and scripted fights.

### Fork additions (`mi-mud`, Batches 14–26)

- **Dungeon Builder (FASE 5.1)** — panel cliente (tecla `D`) + API `/api/dungeons`; diseños persistidos, sin spawn todavía.
- **Contextual quests** — comando `offer <npc>` genera quests con LLM (o fallback determinista) y se aceptan con `offer accept`.
- **NPC free-form dialogue (FASE 6.3, cortes 1+2)** — comando `ask <npc> [pregunta]`: los NPCs con `llm.enabled` responden en persona vía el LLM con memoria persistente por personaje (`npc_memories`, clave `<npcTemplateID>::<characterID>` — resumen determinista + resumen LLM asíncrono con procedencia `summarySource`) y su personalidad estructurada (rasgos Big5, objetivos, secretos y relaciones, p. ej. `player(mentor,0.70)`) inyectada al prompt; la plantilla ordena no revelar secretos literalmente y un guard descarta/reintenta respuestas, opciones o notas de memoria que los reproducen (los secretos además se redactan de las lecturas de plantillas para usuarios que no son creators). Config del `llm` por YAML del content pack, plantilla de NPC REST (creator) o el panel del NPC factory; los spawners heredan la config de la plantilla. Sin el opt-in (o sin proveedor) **degrada al diálogo autoral de `talk`** con un aviso. Guardrails: cooldown por personaje+NPC (3 s por defecto), 2 generaciones concurrentes, timeout 20 s, trabajo LLM fuera del bucle de juego, filtro de contenido, rechazo de secretos literales y línea neutra de fallback. **Nunca muta estado** (solo mensaje + memoria); `talk` sigue siendo el flujo principal con quests.
- **DM narrator (FASE 6.5, modo A)** — narrador reactivo en prosa cableado al bucle de juego vía un bus de eventos tipado (`dm_narrator.go` + `def.DMEvent`): comenta `combatStart`/`combatEnd`/`questCompleted`/`levelUp`/`roomEnter` con mensajes `Username: "DM"` a la sala o en privado; **nunca muta estado**. Guardrails: cooldown por sala/usuario (`LLM_DM_COOLDOWN_SECONDS`, 45 s), timeout con 1 intento (`LLM_DM_TIMEOUT_SECONDS`, 8 s), presupuesto global (`LLM_DM_MAX_PER_MIN`, 10/min), kill switch `LLM_DM_ENABLED` y fallback = silencio. Pendiente el modo B (`DMDecision`).
- **Social** — clanes (roles, banco, panel) y mensajería privada persistente (panel + tabs).
- **NPC factory (FASE 5.3)** — plantillas de NPC con escalado por nivel (`npcBehavior.js`); la tabla `npc_templates` aún sin consumidor.
- **UX de cliente** — inventario con peso (aviso visual, tecla `I`), hoja de atajos `?`, audit panel (creator).
- **QA** — harness `tools/loadtest` (modo `-mint`), corrida real 100–500 conexiones: el **P0 de deadlock del bucle de juego a ≥200 conexiones está cerrado** — corregido en Batch 20 (Backend) y **verificado por QA en B21** (200/300/500 × 2 corridas, `recv == sent`, 0 `msgErrors`, sin wedge); `docs/qa/REGRESSION_REPORT.md` §B19 (historia) y §B21 (cierre).
- **Engine (Batch 23)** — la **race preexistente de presencia del `*entities.User` compartido** (§B22.4) está **cerrada en Batch 23**: lock de presencia por usuario (`pkg/mudserver/game/presence.go`, `ApplyPresence`/`PresenceOf`) + copia desligada `User.Snapshot()` + test `pkg/mudserver/presence_race_test.go` (0 `DATA RACE` con `-race`; verificado por QA en §B23).

### 🎮 Complete RPG Framework

<table>
<tr>
<td width="50%" valign="top">

**Character System**
- 6 base classes from the `data/classes` content pack (Warrior, Rogue, Mage, Cleric, Druid, Sorcerer)
- Schools of magic per class (martial, arcane, divine, nature, fire…)
- Multiple races with unique attributes
- Distributable stat points on level-up
- Equipment system with 10 slots
- Mana system for spellcasters
- Persistent character progression

</td>
<td width="50%" valign="top">

**Combat & Skills**
- Turn-based tactical combat
- 29+ skills/spells across classes
- Status effects (buffs, debuffs, DoTs, HoTs)
- Auto-attack with special abilities
- Skill cooldowns & resource management
- Loot tables with item rarities

</td>
</tr>
<tr>
<td width="50%" valign="top">

**Quest System**
- Multi-objective quests (Kill, Collect, Deliver, Talk, Visit)
- Quest prerequisites and level requirements
- Automatic progress tracking
- NPC dialog integration
- XP, gold, and item rewards
- Repeatable daily quests

</td>
<td width="50%" valign="top">

**World Building**
- Interconnected room-based world
- Visual backgrounds and mood settings
- Coordinate-based mapping (X, Y, Z grid)
- Dynamic NPC/item spawning
- Hidden exits and secret areas
- Custom room actions

</td>
</tr>
</table>

### 🛠️ Content Creation Suite

- **Web-Based Editor** — Full-featured CRUD interface for all game entities
- **Visual Map Editor** — Grid-based world map with room coordinates
- **Dialog Tree Builder** — Create branching NPC conversations with conditional logic
- **Lua Script IDE** — Syntax highlighting, test runner, and sandboxed execution
- **Dungeon Builder** — Grid-based dungeon designer (tecla `D`) exported as opaque JSON to `/api/dungeons`
- **NPC Factory** — Level-scaled NPC/mob blueprints (tecla `N`)
- **Live Preview** — Test changes instantly without server restarts
- **YAML Import/Export** — Version control friendly data formats

### 🎨 Genre Flexibility

TalesMUD's **generic, reusable architecture** makes it perfect for any setting:

- **Fantasy** — Optional sample classes and skills show one fantasy rules shape
- **Sci-Fi** — Space stations, alien worlds, cyberpunk cities
- **Horror** — Lovecraftian mysteries, survival horror, gothic tales
- **Modern** — Contemporary settings, spy thrillers, urban fantasy
- **Custom** — Mix genres or create entirely new worlds

**All core systems** (combat, quests, items, NPCs, dialogs) are **setting-neutral** and configurable through the content editor.

### 🚀 Modern Architecture

- **Go 1.26** backend with Gin HTTP framework
- **Svelte 4** frontend with TailwindCSS
- **SQLite** for persistence (simple deployment, no database server needed)
- **WebSocket** real-time communication
- **Docker** containerization for easy hosting
- **Local auth** by default (username/password + own JWT), Auth0 OAuth2 optional
- **Swappable LLM provider** for AI-powered content generation — local Ollama by default, Groq optional

---

## 🚀 Quick Start

### Prerequisites

- **Go 1.26+** ([Download](https://golang.org/dl/))
- **Node.js 22.18+** ([Download](https://nodejs.org/))
- **Make** (optional, for convenience commands)

### Installation

```bash
# Clone the repository
git clone https://github.com/TalesMUD/talesmud.git
cd talesmud

# Build everything (backend + frontend)
make build

# Run the server
make run-server
```

The server will start on `http://localhost:8010`. Visit in your browser to begin!

With no flags the server boots from `config/gamemode.yaml` (local auth, port 8010); pass `-config path/to/mode.yaml` to load another game-mode profile, and `AUTH_MODE` / `PRESENTATION` env vars override the file. Readiness is exposed at `GET /health`.

### Docker Deployment

```bash
# Start the stack (no Auth0 tenant required — local auth is the default)
docker compose up -d

# Optionally start the local Ollama sidecar for AI generation
docker compose --profile ollama up -d

# The container's healthcheck polls GET /health
curl http://localhost:8010/health
```

### Configuration

Create a `.env` file in the project root:

```bash
# Server
PORT=8010
GIN_MODE=debug
SQLITE_PATH=./talesmud.db

# Authentication: local (default) | auth0
AUTH_MODE=local

# Auth0 (optional — only read when AUTH_MODE=auth0)
AUTH0_DOMAIN=https://your-domain.auth0.com/
AUTH0_AUDIENCE=your-api-audience
AUTH0_WK_JWKS=https://your-domain.auth0.com/.well-known/jwks.json
TRUSTED_PROXIES=127.0.0.1,::1

# Admin (for export/import endpoints). Leave blank to disable.
# Password must be at least 12 characters and not admin/password/changeme.
ADMIN_USER=
ADMIN_PASSWORD=

# AI generation: ollama (default) | groq | none
LLM_PROVIDER=ollama
OLLAMA_BASE_URL=http://localhost:11434
OLLAMA_MODEL=llama3.1

# Dungeon Master narrator (FASE 6.5, mode A): prose only, never mutates state.
# With no provider the DM stays silent (fallback = silence).
LLM_DM_ENABLED=1
LLM_DM_COOLDOWN_SECONDS=45
LLM_DM_TIMEOUT_SECONDS=8
LLM_DM_MAX_PER_MIN=10

# Optional: Groq API key (only used when LLM_PROVIDER=groq)
GROQ_API_KEY=your-groq-api-key
```

See [.env.example](.env.example) for the full variable list.

---

## 🎯 Technology Stack

### Backend
| Component | Technology |
|-----------|------------|
| Language | Go 1.26 |
| HTTP Framework | Gin |
| WebSocket | Gorilla WebSocket |
| Database | SQLite (JSON document storage) |
| Scripting | Lua (gopher-lua) |
| Authentication | Local JWT — `POST /api/auth/register` & `/login` (default), Auth0 optional |
| AI Integration | Swappable LLM provider: Ollama (default), Groq, or none |
| Logging | Logrus |

### Frontend
| Component | Technology |
|-----------|------------|
| Framework | Svelte 4.2 |
| Build Tool | Vite 8 |
| Styling | TailwindCSS 3.4 |
| Terminal | xterm.js |
| HTTP Client | Axios |
| Router | yrv |
| Flow Diagrams | xyflow |

### Infrastructure
| Component | Technology |
|-----------|------------|
| Container | Docker |
| Orchestration | Docker Compose |
| CI/CD | GitHub Actions |

---

## 📚 Core Concepts

### Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                    Web Browser Client                        │
│              (Svelte SPA + xterm.js Terminal)               │
└──────────────────────┬──────────────────────────────────────┘
                       │ WebSocket + REST API
                       ▼
┌─────────────────────────────────────────────────────────────┐
│                   Go Server (Gin)                            │
├──────────────────┬──────────────────┬───────────────────────┤
│  HTTP API        │  MUD Server      │  WebSocket Handler    │
│  (REST + Auth)   │  (Game Engine)   │  (Real-time Comms)    │
└──────────────────┴──────────────────┴───────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────┐
│              Service Layer (Business Logic)                  │
│  Characters • Rooms • Items • NPCs • Quests • Dialogs       │
└──────────────────────────────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────┐
│            Repository Layer (Data Access)                    │
│              Generic SQLite with JSON payloads              │
└──────────────────────────────────────────────────────────────┘
```

### Game Loop

The game engine runs a concurrent event loop processing:

- **Player Commands** — Movement, combat, item interactions
- **NPC Behaviors** — Idle chatter, patrol paths, combat AI
- **Combat Instances** — Turn-based battles with initiative tracking
- **Quest Tracking** — Automatic progress updates from game events
- **Room Updates** — Spawner checks, NPC respawns, state sync

### Message System

All game output is routed through a **message audience system**:

- `Origin` — To the command sender only
- `User` — To a specific user
- `Room` — To all players in a room
- `RoomWithoutOrigin` — To room except sender
- `Global` — Server-wide broadcast

---

## 🎮 Game Systems

### Combat

Turn-based combat with:
- Initiative rolls (1d20 + DEX modifier)
- Auto-attack each round
- Skills/spells with mana costs or cooldowns
- Defensive stance (+50% defense)
- Flee attempts (50% + DEX bonus)
- Status effects (stun, DoT, HoT, buffs, debuffs)
- XP and loot on victory

### Character Progression

- **Leveling** — Flattened XP curve (gentle L2-5, transitional L6-15, steeper L16+)
- **Distributable Stats** — 2 points per level to allocate freely
- **Class Caps** — Prevent degenerate builds (e.g., warriors cap INT at 5)
- **Exploration XP** — 5 XP per new room, 15 XP for new zones
- **Mana System** — Scales with level and INT for casters
- **Equipment** — Weapons, armor, accessories with stat bonuses

### Quests

Data-driven quest definitions with:
- Multiple objective types (Kill, Collect, Deliver, Visit, Talk, Custom Lua)
- Prerequisites (required quests, level)
- Rewards (XP, gold, items)
- Automatic progress tracking
- NPC dialog integration (offer/progress/turn-in)

### Items & Economy

- **Item Types** — Weapon, Armor, Consumable, Currency, Quest, Crafting Material
- **Quality Tiers** — Normal, Magic, Rare, Legendary, Mythic
- **Templates & Instances** — Reusable blueprints with unique instances
- **Merchants** — NPC traders with inventory, restock, price multipliers
- **Containers** — Nested items, lockable chests

### NPCs & Dialogs

- **Dialog Trees** — Branching conversations with options and prerequisites
- **Free-form NPC dialogue** — `ask <npc> [question]` for LLM-enabled NPCs (structured Big5 personality/goals/secrets/relations, persistent memory with deterministic + async LLM summaries; degrades to the authored dialog otherwise)
- **Template System** — Spawn multiple instances from blueprints
- **Behavior Traits** — Enemy (combat stats, aggro, loot) and Merchant (trading)
- **AI States** — Idle, patrol, combat, fleeing, dead
- **Spawners** — Automatic respawn with intervals and caps

---

## 📖 Documentation

### Core Documentation
- **[docs/index.md](docs/index.md)** — Documentation index (fork): qué existe en `docs/`, estado por documento
- **[ARCHITECTURE.md](ARCHITECTURE.md)** — System design, data flow, and component structure
- **[PROJECT.md](PROJECT.md)** — Feature list, configuration, and development status
- **[FEATURES.md](FEATURES.md)** — Feature catalog: systems, entities, APIs, per-phase status
- **[docs/API.md](docs/API.md)** — REST + WebSocket contract (auth, characters, social, dungeons, creator, ops)
- **[docs/fork-setup.md](docs/fork-setup.md)** — Fork setup: local auth endpoints, LLM provider, game-mode config

### Design & Development
- **[docs/design/GAME_DESIGN.md](docs/design/GAME_DESIGN.md)** — Game design document and roadmap
- **[docs/design/SCRIPTING.md](docs/design/SCRIPTING.md)** — Lua scripting API and examples
- **[docs/design/QUEST_AUTHORING.md](docs/design/QUEST_AUTHORING.md)** — Quest creation guide
- **[docs/design/](docs/design/)** — Complete design documentation (PRDs, dialog system, world maps)
- **[docs/development/](docs/development/)** — Developer documentation (agents, combat balance)
- **[docs/qa/REGRESSION_REPORT.md](docs/qa/REGRESSION_REPORT.md)** — QA batches B12–B24 (load, seguridad, balance, combate, LLM); registra el **P0 de deadlock a ≥200 conexiones** (§B19), **corregido en Batch 20 (Backend) y cerrado por QA en B21** (500 concurrentes sin wedge, 0 `msgErrors`), la **race de presencia** (§B22.4), **cerrada en Batch 23 y verificada por QA en §B23**, y la **FASE 6.5 DM narrator** (§B24, **APROBADO**)
- **[docs/qa/BALANCE_ANALYSIS.md](docs/qa/BALANCE_ANALYSIS.md)** — B17 quantitative balance analysis (read-only)

### Player Guide
- **[docs/player-guide/](docs/player-guide/)** — Complete player documentation and tutorials
- **[docs/user/](docs/user/)** — Guías cortas por sistema (player-guide, admin-guide, comandos, clanes, DMs, parties)

---

## 🤝 Contributing

We welcome contributions! TalesMUD is actively developed with ongoing work in the `master` branch.

### Development Workflow

1. **Fork the repository**
2. **Create a feature branch** from `master`
3. **Make your changes** following existing code patterns
4. **Test thoroughly** (use dialog sandbox for NPC-related changes)
5. **Submit a pull request** with a clear description

### Building from Source

```bash
# Install dependencies
go mod download
cd public/app && npm install

# Build backend (by package — see note below)
go build -buildvcs=false -o bin/tales ./cmd/tales

# Build frontend
cd public/app && npm run build

# Or let the Makefile do it (frontends + backend in one go)
make build

# Run development server
make run-server

# Run frontend dev server (with hot reload)
make run-frontend
```

**Build notes:** the `Makefile` targets compile **by package**, never a loose file: `build`/`build-backend` run `go build -buildvcs=false -o bin/tales ./cmd/tales`, `build-dialogs-sandbox` runs `go build -buildvcs=false -o bin/dialog_sandbox ./cmd/dialog_sandbox`, and `run-server`/`run`/`dev` run `go run ./cmd/tales`. Since Batch 20 the package has `cmd/tales/pprof.go`, so `go build cmd/tales/main.go` fails with `undefined: startPprof`. `-buildvcs=false` skips the VCS build stamp and is required where `.git` is unavailable (Docker bind mounts, containers, root-owned checkouts).

### Code Structure

```
mi-mud/
├── cmd/                    # Application entry points
│   ├── tales/              # Main server
│   └── combat_simulator/   # Combat testing CLI
├── pkg/                    # Go packages
│   ├── entities/           # Data models (incl. dungeons, npc templates)
│   ├── mudserver/          # Game server & engine
│   ├── server/             # HTTP API + handlers (incl. creator LLM, dungeons, audit)
│   ├── service/            # Business logic (facade)
│   ├── repository/         # Data access (SQLite JSON store + specialized repos)
│   └── scripts/            # Lua scripting engine
├── public/
│   ├── app/                # Play + Creator client (Svelte)
│   └── mud-client/         # Terminal MUD client (Svelte)
├── import/                 # World packs (YAML + assets)
├── config/                 # gamemode.yaml / ruleset.yaml
├── tools/                  # loadtest harness, map tile tools, importers
├── data/                   # Exported world data (YAML)
└── docs/                   # Markdown documentation (start at docs/index.md)
```

---

## 🗺️ Roadmap

TalesMUD is under active development. Upcoming features include:

### Short Term
- [x] **Fix P0 game-loop deadlock at ≥200 concurrent connections** (Batch 20 Backend: inline `lc`/`selectcharacter`, non-blocking join queue, transport write deadline; `docs/qa/REGRESSION_REPORT.md` §B19.7)
- [x] **Load re-verification of the P0 fix (200/300/500 workers)** — **QA B21: closed**, 6/6 runs × 80 s, `recv == sent`, 0 `msgErrors`, no wedge
- [x] **Fix `writeLoop` goroutine leak found by QA (§B21.5)** (Batch 22: idempotent transport close on disconnect + `transport_leak_test.go`)
- [x] **Synchronise `user.LastSeen` / `user.IsOnline` / `user.LastCharacter` (presence race on the shared `*entities.User`)** — **Batch 23 Backend: closed**, per-user presence lock (`pkg/mudserver/game/presence.go`, `ApplyPresence`/`PresenceOf`) + detached `User.Snapshot()` persist + regression test `pkg/mudserver/presence_race_test.go` (0 `DATA RACE` with `-race`; QA-verified in §B23)
- [ ] Dungeon instances in play (FASE 5.1 builder + `/api/dungeons` landed; spawn pendiente)
- [x] **NPC runtime dialogue with LLM (Fase 6.3, cortes 1+2 — Batches 25–26)** — `ask <npc> [question]` answered by the LLM for `llm.enabled` NPCs, with persistent per-character memory (`npc_memories`, deterministic + async LLM summaries via `summarySource`) and structured personality (Big5/goals/secrets/relations, literal secret leaks rejected and secrets redacted from template reads for non-creators); degrades to the authored `talk` dialog otherwise
- [ ] Remaining Fase 6.3 dialogue debt (free-text interception during an open session, auto-open on `talk`, `FallbackDialogID` consumer)
- [ ] Wire the `npc_templates` table (repo exists, no consumer yet)
- [x] Guild/clan management (clan service, bank, panel; sin comando MUD `clan`)
- [ ] Crafting and resource gathering systems
- [ ] Player housing and customization
- [ ] Advanced combat mechanics (formations, combo attacks)

### Medium Term
- [ ] Multi-server clustering for scalability
- [ ] Redis state sharing for horizontal scaling
- [x] Advanced AI content generation — quests (creator + contextual `offer`) and DM narrator
- [ ] Advanced AI content generation — rooms and dialog trees
- [ ] Mobile native app (React Native or Flutter)
- [ ] Voice chat integration

### Long Term
- [ ] Modding API and plugin system
- [ ] Marketplace for community-created content
- [ ] Game master tools for live events
- [ ] Procedural world generation
- [ ] GraphQL API for third-party integrations

---

## 📜 License

TalesMUD is released under the **MIT License**. See [LICENSE](LICENSE) for details.

```
MIT License

Copyright (c) 2020 Marcus Koerner

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:
...
```

---

## 🌟 Showcase

### Current Deployments

- **[talesmud.io](https://talesmud.io)** — framework site

*Using TalesMUD for your project? [Open an issue](https://github.com/TalesMUD/talesmud/issues) to get listed here!*

---

## 💬 Community & Support

- **Issues** — [GitHub Issue Tracker](https://github.com/TalesMUD/talesmud/issues)
- **Discussions** — [GitHub Discussions](https://github.com/TalesMUD/talesmud/discussions)
- **Live Demo** — [talesmud.io](https://talesmud.io)

---

## 🙏 Acknowledgments

TalesMUD builds on the shoulders of giants:

- **[MUD History](https://en.wikipedia.org/wiki/MUD)** — Inspired by decades of text-based virtual worlds
- **[gopher-lua](https://github.com/yuin/gopher-lua)** — Lua VM for Go
- **[xterm.js](https://xtermjs.org/)** — Terminal emulator for the web
- **[Svelte](https://svelte.dev/)** — Cybernetically enhanced web apps
- **[Gin](https://gin-gonic.com/)** — High-performance Go HTTP framework

---

<div align="center">

**Built with ❤️ by the TalesMUD community**

*Build your world on [talesmud.io](https://talesmud.io)*

</div>
