# TalesMUD Architecture Documentation

## v0.5.0 Components and Data Flow

`pkg/mudserver.Transport` (`Send`, `Close`, `RemoteIP`, `Kind`) separates session lifecycle from delivery. WebSockets use `wsTransport`; `pkg/sshgate` attaches classic text or Door sessions with `MUDServer.AttachExternal`. Both feed the existing command/game loop and session registry; replacement closes the prior transport without deleting the new session. SSH queues output off the game loop and applies session limits and terminal sanitization.

`pkg/devicecode` holds expiring pending requests and confirmation/rate-limit state in memory. Classic public `POST /api/device/start` and `/api/device/poll` share it with SSH. Authenticated `/api/ssh/device/{lookup,confirm,deny}` uses explicit `/activate` browser approval; `pkg/sshkeys` persists signature-proven keys in SQLite. `pkg/authsession` signs HS256 account sessions with required issuer, audience, expiry, account ID and session version; middleware receives its verifier explicitly. User session-version changes revoke tokens. SQLite adds SSH key, content-health and audit storage while retaining JSON entities.

`pkg/worldindex` builds entity references, reachability and search data. `pkg/contenthealth` consumes the same world snapshot for validation, quest debugging, map overlays, pack drift/export and the `tales -check` CLI. `pkg/inspect` produces static NPC/room inspector responses. Creator endpoints add search, refs, health, world overlays, NPC/room inspection, quest debugging, loot preview, audit and live NPC/instance views. Admin live character inspection, audited operations and undo are scheduled on the game loop rather than mutating live state from HTTP goroutines.

`import/adventure/` is a shippable content pack in the `pkg/importer` YAML layout (four zones on top of the R0001 hub: Training Grounds tutorial, Buried Vault, Frostspine Caverns and the Elder Maw raid, plus items/NPCs/loot tables/spawners/scripts/quests under `import/adventure/data/`). `pkg/importer/adventure_pack_test.go` loads it into a `contenthealth.World`, and `TestAdventurePackHealth` fails on any error **or** warning while `TestAdventurePackReachability` fails on unreachable rooms. Design: `docs/design/PHASE5_DUNGEONS.md`. Fork runbook: `docs/fork-setup.md`; player-facing system guides: `docs/user/clans.md`, `docs/user/dms.md`, `docs/user/channels.md`, `docs/user/party.md`, `docs/user/combat.md`.

NPC movement configuration is persisted with templates/spawners; runtime clocks and engagement holds are memory-only. Structured examine, aggression markers, guest identity and reattach combat snapshots extend messages additively; optional `hello` negotiates capabilities without dispatching a command. See [structured messages](docs/structured-messages.md).

Dungeon authoring (FASE 5.1) adds an independent data path: the play client's Dungeon Builder (key `D`) keeps designs in `localStorage` and mirrors them best-effort to `/api/dungeons`, served by `handler/dungeons.go` → `DungeonsService` → `Factory.Dungeons()` → table `dungeons` (`definition` is an opaque JSON blob). Nothing instantiates dungeon designs in the game yet. FASE 5.3 adds the `npc_templates` table + `Factory.NPCTemplates()` (blueprints shaped by the client's `npcFactory.js`, with `Behavior`/`Scaling` raw JSON from `npcBehavior.js`). B30 wires the service and creator REST (`/api/npc-templates`): create/update attempt a best-effort live spawn when the blueprint has `spawnRoomId` + `spawnCount`, and `POST /api/npc-templates/:id/spawn` forces one instance. `GET /api/npcs/templates` still reads the `npcs` collection for compatibility. Contextual quests reach gameplay through the `offer` command (`game/commands/offer.go` + `game/quest_offer.go`), whose provider is installed at startup in `pkg/server/llm.go`. Contract: `docs/API.md`.

Progression (Phases 2.3 and 2.4) adds three modules with narrow responsibilities: `pkg/entities/characters/derive.go` derives combat-facing stats and folds in talent ranks, `pkg/talents` is a pure-data talent catalog (12 talents, 3 categories), and `pkg/mudserver/game/leveling` owns the XP curve plus the one-point-per-5-levels milestones. Talent/prestige player input is two commands (`commands/talents.go`, `commands/prestige.go`); state travels inside the `characters.data` JSON document (`CharacterTalents`), so there is no new table. Client-side Phase 3.x work is pure helpers in `public/mud-client/src/game/` (`commandHistory.js`, `minimapDirections.js`) plus responsive breakpoints. Design: `docs/design/PHASE2_4_PROGRESSION.md`.

Fase 6 (LLM integration beyond character generation) is designed in `docs/design/PHASE6_LLM.md` and partially implemented in the working tree (uncommitted; verified 2026-11-01). NPC runtime FASE 6.3 is implemented through mechanisms B/C (Batches 25–29): `talk` priority authored dialog → available quests → LLM greeting, raw-text continuation of an eligible recent `__llm__` conversation, and `quest:<id>` options validated against current available quest actions. The `npc.dialogue` prompt is v1.0.3. B30 adds DM mode B (`DMDecision` + ambush/environmental/narrative events) as opt-in (`LLM_DM_DECISION_ENABLED` off by default). Pending: the short-term memory ring, complete combat guardrails and `FallbackDialogID` as an error fallback. Documentation map: `docs/index.md`; HTTP/WS contract: `docs/API.md`.

REST accepts Bearer tokens only. WebSocket upgrades retain the single-use ticket or Bearer-header contract. Guest HMAC, server device sessions, local username/password sessions and Auth0 RS256 are separate verification paths. This fork boots with `auth: local` from `config/gamemode.yaml`, so local sessions are the default and no Auth0 tenant is required; Auth0 audience/issuer checks are mandatory only when `AUTH_MODE=auth0` selects the external provider. All paths use JWT v5. Query secret redaction includes tickets and device codes at key boundaries, and path-only logging remains available. Docker/CI builds use Go 1.26; `go.mod` pins `go1.26.9` and updated crypto/network dependencies.

Frontend Docker stages use Node 22. `netlify.toml` pins Node 22.18.0 for Creator UI builds, sets `public/app` as the build base, installs from the npm lockfile with the existing legacy peer policy, and publishes Vite's `dist` output. Netlify hosts this frontend; the Go process still owns the game, API and SSH listeners.

## System Overview

TalesMUD follows a layered architecture with clear separation between HTTP API, real-time game server, business logic, and data access layers.

```
┌─────────────────────────────────────────────────────────────────┐
│                         Clients                                  │
│              (Browser / WebSocket / REST API / SSH)                    │
└─────────────────────────────────────────────────────────────────┘
                              │
         ┌────────────────────┼────────────────────┐
         ▼                    ▼                    ▼
┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐
│   HTTP Server   │  │   MUD Server    │  │  Embedded SPA   │
│   (Gin Router)  │  │   (Transport)  │  │  (go:embed FS)  │
└────────┬────────┘  └────────┬────────┘  └─────────────────┘
         │                    │
         │                    ▼
         │           ┌─────────────────┐
         │           │   Game Engine   │
         │           │ (Command Loop)  │
         │           └────────┬────────┘
         │                    │
         ▼                    ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Service Layer                               │
│  (Characters, Rooms, Users, Items, Scripts, Parties, Quests)   │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                     Repository Layer                             │
│                     (SQLite backend)                             │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
              ┌───────────────────────────────┐
              │            SQLite             │
              │ (tables with JSON payloads)   │
              └───────────────────────────────┘
```

## Backend Architecture

### Entry Point

**File:** `cmd/tales/main.go`

```go
func main() {
    flag.Parse()
    godotenv.Load()                    // 1. .env configuration
    // 2. -config wins; otherwise config/gamemode.yaml is applied when present
    //    (fork default: presentation classic, auth local, port 8010).
    gamemode.ApplyFile(effectiveConfig)
    gamemode.ApplyEnv()                // 3. AUTH_MODE / PRESENTATION / ... override the file
    server.NewApp().Run()
}
```

`gamemode.ApplyFile` parses the YAML with the `pkg/gamemode.Config` tags and copies `port` / `sqlite_path` into `PORT` / `SQLITE_PATH`. The default file lives at `config/gamemode.yaml`; the Docker image copies it to `/app/config/gamemode.yaml` so the `ENTRYPOINT ["/app/tales"]` needs no flags. Game-mode files must not carry ruleset keys (`progression`, `death`, `new_day`, `resources`, `combat`, `regen`), which belong to `config/ruleset.yaml`.

### Server Layer (`pkg/server/`)

The HTTP server handles REST API requests and serves as the entry point for WebSocket connections.
It wraps the Gin router with CORS handling that allows the production domains,
local development origins for the Creator and MUD clients, and optional
additional origins from `CORS_ALLOWED_ORIGINS`.
Authenticated profile updates derive the target user from the JWT subject in
request context and only merge editable profile fields, rather than trusting
identity or authorization fields from the request body.
The generate handler (`handler.GenerateHandler`) is wired with
`llm.NewFromEnv()` into its `LLMClient` field, so AI character generation
uses the provider selected by `LLM_PROVIDER` (local Ollama by default)
through the provider-agnostic `llm.Client` contract instead of a
hard-coded client.

#### Server Structure

```go
type app struct {
    Router    *gin.Engine      // HTTP router
    Facade    service.Facade   // Service layer access
    mud       mud.MUDServer    // Game server instance
}
```

#### Database Configuration

`server.NewApp()` initializes the SQLite storage backend at startup.

Use `SQLITE_PATH` to specify the database file path (defaults to `talesmud.db`).

#### Route Organization

```
/                          # Static files (frontend)
/play                      # MUD client SPA (JS/CSS/HTML Cache-Control: no-cache)
/health                    # Health check
/ws                        # WebSocket (game connection)
/api/
    ├── characters/        # Player: list/get/delete/profile-update/newcharacter; raw POST is creator-only
    ├── ws-ticket          # POST single-use 60s WebSocket ticket (Bearer auth)
    ├── rooms/             # Room CRUD (creator level, including GET)
    ├── items/             # Item CRUD (creator level, including GET)
    ├── scripts/           # Script CRUD (creator level, including GET)
    ├── npcs/              # NPC CRUD (creator level, including GET)
    ├── dialogs/           # Dialog CRUD (creator level, including GET)
    ├── quests/            # Quest definition CRUD (creator level, including GET)
    ├── quest-progress/    # Quest log per character (owner/admin)
    ├── clans/             # Clan create/mine/invite/kick/rank/bank/disband (owner; leader/officer rules)
    ├── dm/                # Private messaging channels and messages (owner; channel participants)
    ├── dungeons/          # Dungeon designs (FASE 5.1): GET for any session, POST/PUT/DELETE creator-gated
    ├── creator/llm/       # Creator LLM authoring: quests generate/contextual, room description, prompts
    ├── audit/             # Creator: recent content edits (+ POST /api/audit/:id/undo for admin)
    ├── characters/:id/map # Per-character discovered-world atlas (owner/admin)
    ├── portraits/:filename # Public NPC/enemy/player portrait images (no auth, guest-ok)
    ├── world/validation   # Creator world health diagnostics
    ├── diagnostics/world  # Creator world health diagnostics
    ├── validate/:entityType # Draft Creator entity validation
    ├── preview/dialog|quest|room|merchant # Draft preview/test endpoints
    ├── user               # User profile (player level)
    ├── admin/
    │   └── users/         # User management (admin only)
    └── templates/         # Public templates
/admin/
    ├── export             # World export (explicit basic auth credentials required)
    ├── import             # World import (explicit basic auth credentials required, validates before drop)
    └── world              # World map (basic auth)
```

`GET /api/server-info` adds `envLabel` from `ADMIN_ENV_LABEL` and `host` from `ADMIN_ENV_HOST` or the request host. The top bar shows a red `{label} · {host}` badge only when the label is non-empty. The same payload includes `defaultTheme` from the active ruleset key `ui.default_theme`. Omitted, empty, and unknown-at-startup values stay `dark-fantasy`. An unknown theme string is rejected and does not replace the current profile. The play client stores `auto` unless the player picks Dark Fantasy, Clean HUD, or Gilded Frame. `auto` paints the server theme. A saved `dark-fantasy` without that explicit choice becomes `auto` on load. Theme CSS is `public/mud-client/public/themes.css` plus `public/themes/gilded/` (`tokens.css`, `components.css`, `chrome.css`, `w2-screens.css`, `w3-screens.css`). Inventory, equipment, the character sheet, item cards, party, recipes, and sign-in are in `w2-screens.css`. Dialog, quest log, shop, notices, combat stage, and room entity cards are in `w3-screens.css`, loaded after it. A content pack may set the same key in `config/ruleset.yaml`. Import writes `import/.active`, and the server merges that file onto the engine profile at startup. Omitted keys stay on the engine values. The cache tag on the play CSS links stays with the rest of the play CSS.

Creator writes (POST/PUT/DELETE for rooms, items, NPCs, loot tables, spawners, dialogs, quests, scripts, skills, character templates, and settings) pass through `AuditWrites`. Each successful write stores actor, action, entity, before JSON, and after JSON in `audit_log`. `POST /api/loot-tables/:id/roll` is a creator preview and stays off that middleware: it does not record a row, drop the search index, or create items. Deletes keep the full before document. `POST /api/audit/:id/undo` (admin) restores that before state, or runs the stored inverse for a live op. Undo is refused when the row is not undoable, already undone, or the current document no longer matches the recorded after state. The undo itself is an audit row.

`POST /api/ops/:action` (admin) requires `confirm: true` and runs inside `Game.Call`, on the same command loop as player commands. It does not write an online character's database row from the HTTP handler. Each success records an audit row. `end-combat` and `instance-cleanup` are not undoable. Quest completion with an objective id only advances that step. Completion without an objective id grants rewards; undoing the progress row does not claw those rewards back.

`GET /api/live/characters` and `GET /api/live/characters/:id` are admin-only. They list characters who are online or whose user was seen within 30 days, and the detail includes inventory and the quest log. `all=1` includes older characters. A creator receives 403. NPC and instance reads stay on the creator routes and use the same game-loop call.

`GET /api/quest-progress/:characterId` returns quest progress merged with quest definition fields for the player UI. Objective rows include `objectiveId`, definition `description`, current/required counts, and completion state so REST refreshes and WebSocket quest log messages have matching player-facing text.

Social routes (`pkg/server/handler/clans.go`, `pkg/server/handler/dms.go`) run at the authenticated player level. The acting character is passed explicitly (`?characterId=` for `GET`, JSON body for `POST`) and `resolveOwnedCharacter` verifies `character.BelongsUserID == user.ID` (admins may act on any character) before the service is called. Clan permissions are enforced in `ClansService`: create when not already in a clan, invite/kick as leader or officer (the leader can never be kicked), rank changes and bank withdrawals/disband as leader, deposits as any member. DM permissions are enforced in `DMService`: send/get/mark-read require the character to be a channel participant (anti-spoof), and `FindOrCreateChannel` canonicalizes the pair. Service errors are mapped to stable HTTP status codes (400/401/403/404/409).

`GET /api/portraits/:filename` is public (no Auth0), same pattern as room backgrounds. The importer copies NPC/enemy sprites into `uploads/portraits/` (flat `{id}.png`); player race/class sprites are published there as `player-<race>-<class>.png`. Room NPC, combatant, and party roster payloads include portrait URLs. The client maps player race/class for the paper doll and uses a built-in class silhouette if a file fails to load.

Combat start and action `CombatantView` snapshots include participant type, class ID, HP/MP, alive/fled state, level, and portrait. When a player joins an existing fight, the joining client gets `combatStart` and existing fighters get a `combatAction` with `action: "join"` and the full roster. The client merges snapshots by participant type so new players enter the ally list without waiting for the next attack.

The play embed includes `fonts/MaterialIcons-Regular.woff2` and its Apache license. `icons.css` declares the font with `font-display: block`, and the initial HTML preloads it. The client enables icon visibility after the local FontFace loads; failed loads leave the ligatures hidden. Icon rendering no longer depends on a Google Fonts request.

Private cellars: an exit with `type: instance` or `instance: true`, or a normal exit from a non-instance room into a room tagged `instance`/`instanced`, clones the dest room plus rooms reachable without returning to the hub. Each character gets their own copy; the hub stays shared. Empty instances are deleted. `MUDServer.Run` sweeps persisted `~` room rows, NPC/spawner rows that are copies, and characters saved in a copy or a missing room before the game loop and before `ListenAndServe`. Relocation order is hub, return exit, start room, bind room. The one-line login notice is in-memory until that character connects.

`GET /api/characters/:id/map` returns that character's fog-of-war atlas. `pkg/worldmap` lays out authored area-local coordinates and compass exits deterministically, then translates zones onto compact centers from embedded `map_layout.json` (unknown zones attach through inter-zone compass exits). Anonymous biome ground and broad land bridges form a connected continent without inventing rooms/exits. Above-ground interiors project onto graph-nearest exterior anchors, including upstairs rooms; outdoor positive Z is elevation on Overworld, while explicit depth and underground context use Lower. Reveal applies discovered rooms, unnamed neighbors through visible exits, area hulls, semantic layers, and fog-masked ground. Discovered places include exits, danger, a short summary, and optional NPC/enemy residents. Hidden exits and entrance stamps stay hidden until revealed. The JSON is the contract for both the web atlas widget and a future mobile renderer.

Terrain classification happens once per room during `worldmap.Compile`, using embedded `pkg/worldmap/map_terrain.json`. `Place.terrain` is a derived JSON field, supplied to both WebSocket and HTTP atlas consumers; Reveal replaces it with `fog` for unexplored neighbors. Additional derived fields are `mapRole` (surface/interior/underground), `surfaceRoomId`, `town`, visible `entrances` room IDs, disclosed `mapFeatures`/`artSeed`, and `undergroundStyle`; `PlayerMap.landscape` is anonymous `{x,y,terrain}` ground. These presentation fields do not change room storage, discovery storage, gameplay coordinates, or travel topology. `art.go` derives ordered feature hints from tags, room/area types, names, service action names, descriptive fallbacks, and bind context; an FNV seed varies artwork deterministically with room customization. Underground style uses local room context. Reveal emits these only for discovered rooms; it never exports descriptions, service scripts, or action parameters as art data. Client merging preserves charted art against stale fog and fingerprints feature/seed/style updates. Exact outdoor water and biome cells take precedence over nearby vegetation, keeping bridge locations intact. The separate content `build_public_map_data.py` still produces public lore/zone map data, not the in-game room atlas.

Both map views share the canvas renderer and one 288×9024 RGBA sprite sheet with native 48px cells: six variants each for 14 terrains, fog/sea, 40 decoration rows, four underground floor rows, and 128 precomposed directional dither rows. Production sheet is the P1j imagegen craft atlas (`c8169d17ef81`); the Pillow path (`generate_map_tiles.py` / `map_hires_art.py`) remains available to rebuild the older P1g look. `tools/sync_map_tiles.py` verifies rows, dimensions and content hash before copying the runtime sheet and generating metadata. `GET/HEAD /api/map-tiles/terrain-sheet.png` exposes only that file from the play embed with cache headers/versioned URL. Client assets must be copied before the local Go embed build.

`surfaceAtlas.js` groups interiors under exterior anchors and filters charted outdoor roads. `mapArt.js` shares deterministic sprite choice and canvas/bitmap helpers. Sprite source rectangles follow the sheet manifest. World geometry retains 32 logical units per cell, while detailed/town/Lower canvases allocate 48 pixels per cell through a scaled context. The far overview keeps 32 pixels per cell to avoid repeated large bitmap downscales; the painter derives each bitmap’s cell size from its width and world bounds, keeping projections/hits and the 1–10/220px zoom caps unchanged. Lower scenes freeze to reusable ImageBitmaps where supported. `mapDetails.js` traces angular walls around the union of disclosed courtyards/streets, puts towers at corners and gates at incoming road intersections, derives mountain relief from biome-edge distance, and chooses biome tree species. `coastline.js` blurs a sampled binary land field, builds sand/foam/depth bands with twelve samples per cell and shared RGBA lookup tables, and traces a compact vector contour for clipping; its most recent geometry/fog mask is cached. A dedicated `worldmapWorker.js` builds scenes with OffscreenCanvas and transfers completed ImageBitmaps plus overview/close cell sizes and coast sample scale. `worldmapSceneStore.js` matches results by disclosed scene signature, ignores marker-only differences, and keeps one running job plus one replaceable queued job. The main thread holds a bounded four-scene cache; worker scene/town caches are bounded separately. Close LOD is requested lazily and keeps the matching overview visible until ready. Worker completion schedules a repaint through the shared scene-ready listener. Browser capability/load failures use the synchronous canvas fallback. Rollup emits the same-origin versioned `worldmap-worker.js` alongside the client bundle; it is included in the normal local play-asset embed copy.

`continentRenderer.js` bakes an overview and lazily builds close detail. Dither overlays are authored into the sheet, avoiding runtime tiny-canvas blending. A separate bounded town/road/prop overlay cache reuses unchanged room art when ground changes; keys include disclosure, room customization, geometry and water crossings. The final scene cache includes every art/fog input and excludes marker-only flags. Both scenes retain varied foothills/central peaks, biome species, street paving, angular walls/towers/gates, room-driven roofs/props, roads, bridges, offshore rocks and coastal cliffs. Roads batch each unique exit into one compound stroke under mountain and canopy stamps; the later town, prop and building overlay covers them, while opaque town paving hides dirt inside streets. Painting crops cached scenes to the viewport; charted and fog terrain both come from cached pixels. Real room centers and the current marker have hit priority over nearby decorative props. Minimum scale and Fit world share the continent frame at 78% of map height when width permits; minimap/recenter retain exterior groups.

`undergroundRenderer.js` caches rock/floor scenes with disclosed cave/crypt/cellar/sewer styles, torches, stairs, bones/barrels and dim cluster light in dark void. Only short correctly aligned compass exits become visible tunnels; long/misaligned/vertical links remain navigation exits without lines across void. Tree/building drop shadows are removed; mountain faces retain restrained shading. The full overlay schedules 650 ms water/smoke accents only while open/visible, respects reduced motion, and disposes its timer. Real rooms/roof/glyph stamps provide hit targets; filler and offshore decoration do not. Development snapshot/preview tools use local read-only inputs and an in-memory character, without gameplay or database writes.

#### Landing Page Middleware

**File:** `pkg/server/landing.go`

Optional middleware that serves a static landing page from the OS filesystem when `LANDING_PATH` is set. Sits between the `/play` SPA and `/` SPA in the middleware chain.

- Skips `/api/`, `/admin/`, `/ws`, `/play` prefixes
- Skips Auth0 callbacks (`?code=` or `?error=` query params) so the main SPA handles them
- Serves `index.html` for `/` and static assets for other matching files
- Returns a no-op handler when `LANDING_PATH` is unset or `index.html` is missing

#### Authentication & Authorization Middleware

**File:** `pkg/server/auth.go`

```
REST: Authorization Bearer → Guest HMAC / Server Session / Local Session / Auth0 RS256 (aud+iss required) → Load User → Check Ban → Set Context
WebSocket: POST /api/ws-ticket (Bearer) → GET /ws?ticket=… (single use, 60s) or Authorization Bearer
```

- REST rejects `?access_token=` so session JWTs are not written to access logs
- WebSocket origin is allowlisted (same list as CORS); empty Origin allowed for non-browser clients
- **Token validation order**: guest HMAC-SHA256 token first (fast), then `authsession` device sessions (`authsession.IsIssuer`), then local username/password sessions (`authlocal.IsLocalIssuer`, only when `auth: local`), then Auth0 RS256 (`github.com/golang-jwt/jwt/v5`)
- Guest tokens signed with `GUEST_SECRET` env var, validated via `GuestService.ValidateGuestToken()`
- Guest session expiry checked at auth layer (returns 401 if expired)
- Local sessions are installed by `server.UseLocalAuth` in `server.NewApp` when `gamemode.LocalAuth()` is true; `pkg/authlocal` verifies its own JWT issuer and never sends those tokens to Auth0
- Auth0 tokens validated against JWKS endpoint (with in-memory cache, 1-hour TTL); algorithm pinned to RS256
- Auth0 audience/issuer are required by `getKeyFunc`; they are only exercised in `AUTH_MODE=auth0`, since local and guest tokens terminate before the Auth0 branch
- Creates new user on first login
- Syncs admin role from `MUD_ADMIN_OAUTHID` env var on every login
- Rejects banned users with 403 at the auth layer
- Sets `userid` and `user` in Gin context

**Role-Based Middleware:**

- `CreatorMiddleware()` — Requires creator or admin role for world-definition reads (rooms, scripts, NPCs, dialogs, quests, items, loot, skills, settings, dungeons) and all content writes
- `AdminMiddleware()` — Requires admin role for user management endpoints

**Access Level Hierarchy:**

| Role | Player Routes | Creator Routes | Admin Routes |
|------|:---:|:---:|:---:|
| Player | Yes | No | No |
| MUD Creator | Yes | Yes | No |
| MUD Admin | Yes | Yes | Yes |

### MUD Server (`pkg/mudserver/`)

The MUD server manages WebSocket and external SSH sessions and routes messages between clients and the game engine.

#### Connection Model

```go
type Connection struct {
    User   *entities.User   // Authenticated user
    t      Transport        // WebSocket or SSH transport
    mu     sync.Mutex       // Thread-safe writes
    active bool             // Connection state
}
```

The client registry supports stale-safe replacement for reconnects. When a user opens a new WebSocket, the server replaces the previous connection and only a matching current connection can mark the user offline or emit quit cleanup. This prevents an old socket close from deleting a fresh connection or incorrectly clearing online state.

#### Server Components

```go
type server struct {
    Facade    service.Facade           // Service access
    Game      *game.Game               // Game instance
    Clients   *clientRegistry          // Mutex-protected active connections
    Broadcast chan interface{}         // Global messages
    Upgrader  websocket.Upgrader       // HTTP→WS upgrade
}
```

#### Live Session Registry

The game engine owns an in-memory session registry for live player state. The
registry maps connected user IDs to their currently selected character, room,
and last-seen timestamp. WebSocket connect/read/disconnect paths update this
registry and persist `User.IsOnline` as a secondary status field. `combat.disconnect: continue` (the default) leaves that fight running. `release` ends it without a defeat penalty and, when `combat.safe_room` says so, moves the character before an instance copy is deleted. A generated instance that times out still does that move. A text-client connect runs the new-day pass and refills configured resources without requiring another character select. Player-directed replies are recorded by the text client and drawn back into its frame off the message-drain goroutine.

Room message fan-out, `who`, private tells, friends online flags, regeneration ticks (1s clock; out-of-combat, resting, and in-combat pools each use their ruleset interval, default 10s), and room player
payloads use the live session registry instead of scanning all users with
stale `IsOnline` flags. Persisted `Room.Characters` still records character
location and is periodically cleaned, but it is no longer the source of truth
for whether a player is reachable. `Character.FriendIDs` persists a per-character
friends list in the character JSON blob; `friend`/`friends` commands and a
`friends` WebSocket payload drive the play-client overlay.

#### Concurrent Goroutines

The MUD server runs 4+ concurrent goroutines:

1. **Message Receiver** - Routes game output to appropriate clients
2. **Game Loop** - Processes commands and updates game state
3. **Broadcast Handler** - Sends global messages to all clients
4. **Timeout Handler** - Sends ping every 60 seconds
5. **Guest Cleanup Loop** - Removes expired guest accounts every 5 minutes
6. **Per-Guest Timeout** (per connection) - Warning at 25 min, disconnect at 30 min
7. **Per-Guest Disconnect Cleanup** (per connection) - 5-min grace period before deleting guest data

#### Message Flow

```
Client WebSocket
       │
       ▼ (JSON IncomingMessage)
HandleConnections()
       │
       ▼ (creates Message struct)
OnMessageReceived channel
       │
       ▼
Game.Run() processes
       │
       ▼ (creates MessageResponse)
SendMessage channel
       │
       ▼
receiveMessages() routes by Audience
       │
       ├──► sendMessage(userID)     [Single user]
       ├──► sendToRoom(room)        [All in room]
       ├──► sendToRoomWithout()     [Room except origin]
       └──► Broadcast channel       [Global]
```

Character selection, room movement, and disconnect cleanup also emit `roomPresence` messages. These are silent room-scoped WebSocket payloads that refresh the client player list without re-rendering the full room description.

### Game Engine (`pkg/mudserver/game/`)

The game engine contains the core game loop, command processing, and state management.

#### Game Structure

```go
type Game struct {
    Facade           service.Facade        // Service access
    SystemUser       *entities.User        // System actor

    // Event Channels
    onMessageReceived chan interface{}     // Player input
    sendMessage       chan interface{}     // Game output
    OnUserJoined      chan *m.UserJoined   // Login events
    OnUserQuit        chan *m.UserQuit     // Logout events

    // Processors
    CommandProcessor  *CommandProcessor    // Global commands
    RoomProcessor     *RoomProcessor       // Room commands

    Avatars          map[string]*Avatar    // Active players
    QuestTracker     *QuestTracker         // Quest progress tracking
}
```

#### Game Loop

```go
func (g *Game) Run() {
    for {
        select {
        case joined := <-g.OnUserJoined:
            g.handleUserJoined(joined.User)

        case quit := <-g.OnUserQuit:
            g.handleUserQuit(quit.User)

        case msg := <-g.onMessageReceived:
            message := msg.(*m.Message)
            // Try global commands first
            if !g.CommandProcessor.Process(g, message) {
                // Then room commands
                g.RoomProcessor.Process(g, message)
            }
        }
    }
}
```

#### Update Cycle

The game runs multiple update tickers:

1. **Room Updates** (10 seconds)
   - Remove offline characters from rooms
   - Sync room state to database
   - Clean up stale connections

2. **NPC Updates** (10 seconds)
   - Process NPC state machine (idle, patrol, combat, fleeing)
   - Handle respawning for dead NPCs
   - Execute behavior updates: patrol advances through configured room IDs; idle wandering chooses visible exits within `WanderRadius` from spawn
   - Trigger idle dialog messages using `IdleDialogTimeout`
   - Broadcast NPC movement/chatter messages and send silent room updates to online occupants in affected rooms

3. **Spawner Updates** (5 seconds)
   - Check each spawner's instance count
   - Fill `InitialCount` immediately until the spawner has lost an instance
   - After a death, wait the longer of `RespawnDelay` and `combat.respawn_min_seconds` (default 30s)
   - Let `SpawnInterval` space only the extras between `InitialCount` and `MaxInstances`
   - Clean up dead instances from spawner tracking

4. **Combat Updates** (2 seconds)
   - Check turn timeouts (60 seconds per player turn)
   - Process NPC turns when it's their turn
   - Handle AFK auto-flee after 3 consecutive timeouts
   - Check global combat timeout (30 minutes)

#### NPCInstanceManager

Manages in-memory NPC instances that are spawned from templates:

```go
type NPCInstanceManager struct {
    instances    map[string]*npc.NPC      // Active NPC instances
    spawnerState map[string]*SpawnerState // Runtime spawner state
    facade       service.Facade
}
```

Resident NPC initialization (two-pass):
1. Load NPCs referenced in `Room.NPCs` lists
2. Load any remaining non-template NPCs that have `CurrentRoomID` set (auto-spawn unique NPCs into their assigned room)

Key methods:
- `Initialize()` - Load spawners, create initial instances, and register all resident NPCs
- `SpawnInstance(spawner)` - Create instance from spawner template
- `SpawnInstanceDirect(templateID, roomID)` - Spawn without spawner
- `GetInstancesInRoom(roomID)` - Query instances by room
- `KillInstance(id)` - Mark instance as dead
- `RespawnInstance(id)` - Restore instance to alive state

#### CombatController

Manages turn-based combat instances:

```go
type CombatController struct {
    manager *combat.Manager    // In-memory combat instances
    engine  *combat.Engine     // Combat logic and calculations
    game    *Game              // Game reference for notifications
}
```

Key methods:
- `IsPlayerInCombat(characterID)` - Check if player is in combat
- `IsNPCInCombat(npcID)` - Check if NPC is in combat
- `InitiateCombat(roomID, players, enemies)` - Start new combat
- `ProcessPlayerAttack(characterID, targetID)` - Handle attack action
- `ProcessPlayerDefend(characterID)` - Handle defend action
- `ProcessPlayerFlee(characterID)` - Handle flee attempt
- `GetCombatStatus(characterID)` - Get formatted combat status
- `Update()` - Process combat tick (turn timeouts, NPC turns)

### Combat System (`pkg/mudserver/game/combat/`)

Turn-based combat occurs in isolated **Combat Instances** that manage fights between players and NPCs.

Aggro on sight uses that same start. `combat.aggro_on_sight` (default on, 2.5s grace, level gap 5, 15s reaggro cooldown) schedules one watch per player and aggressive NPC when the player enters the room or the NPC arrives. A death replacement and a unique NPC respawn use `respawn_grace_seconds` (default 8) for players already in the room. The first fill, a walk-in, and a summon keep the short grace. After a victory or flee, `post_combat_grace_seconds` (default 10) holds aggressive NPCs in the fight room for players still standing there. Entering a different room ends that hold. `spawn_aggro_cooldown_seconds` (default 60) is stored on the spawner id, or on the resident NPC id, so a new instance remembers the player it already auto-aggroed. The timer enqueues onto the command loop. The loop re-checks, posts `The <name> spots you.` as a normal message, and calls `BeginEngagement`, which calls `InitiateCombat`. Swarm pack, one `onAggro`, and the party assist nudge match a manual `attack`. Leaving during the grace cancels the watch. The per-instance cooldown starts when the fight ends. A room with `noAggro` is skipped at schedule and again when the timer fires. Room NPC payloads include optional `aggroOnSight: true` (`omitempty`, absent for passive NPCs) so the play client can mark those cards. The field is copied through both the runtime room summary and the websocket response. Look and examine of that NPC add a line `Aggressive`.

#### Combat Instance Model

```go
type CombatInstance struct {
    ID              string
    OriginRoomID    string          // Room where combat started

    Players         []CombatantRef  // Player combatants
    Enemies         []CombatantRef  // NPC combatants

    TurnOrder       []CombatantRef  // Initiative-sorted order
    CurrentTurnIdx  int             // Current turn
    TurnStartTime   time.Time       // For timeout tracking
    Round           int             // Current combat round

    State           CombatState     // pending, active, victory, defeat, fled, timeout
    Log             []CombatLogEntry
}

type CombatantRef struct {
    ID, Name        string
    Type            CombatantType   // player or npc
    Initiative      int             // Turn order priority
    IsAlive, HasFled bool

    Level           int32           // Snapshot for level-gap modifiers
    MaxHP, CurrentHP int32
    AttackPower, Defense int32
    STRMod, DEXMod, CONMod int      // Attribute modifiers
    INTMod, WISMod int              // Magic attribute modifiers
    DefenseBonus    int32           // From defend action

    // Mana & Skills
    MaxMana, CurrentMana, ManaRegen int32
    EquippedSkills []string         // Skill IDs available in combat
    SkillCooldowns map[string]int   // SkillID → rounds remaining
    StatusEffects  []StatusEffect   // Active buffs/debuffs/DoTs/HoTs
    QueuedSkillID  string           // Skill queued for next action
}

type StatusEffect struct {
    ID, SkillID, Name string
    Type    string    // "buff", "debuff", "dot", "hot", "stun"
    Stat    string    // "attack", "defense", "dodge"
    Value   int32     // Flat value modifier
    Percent float64   // Percentage modifier (e.g. 0.30 = +30%)
    Duration int      // Rounds remaining
    SourceID string   // Caster ID
}
```

#### Combat Flow

```
1. INITIATION
   └── Player attacks NPC OR aggressive NPC detects player
   └── Create CombatInstance with participants
   └── Roll initiative (1d20 + DEX mod), sort turn order

2. COMBAT ROUND
   └── At round start: tick cooldowns, regenerate mana
   └── For each combatant in turn order:
       ├── Process status effects (DoT damage, HoT healing, stun skip)
       ├── Player turn: 60-second timer, choose action
       │   ├── attack <target> - Roll to hit, deal damage
       │   │     Level gap (config/combat_balance.yaml level_gap) shifts hit, crit, and damage
       │   │     for attacks and skills. Gap = attacker level − defender level, clamped ±6.
       │   │     Gap 0 matches the pre-gap formulas.
       │   │     class_balance then scales damage dealt and taken per class.
       │   │     The pack catalog wins. The YAML row is the fallback.
       │   │     early_normal scales normal-tier enemies at level 6 and below.
       │   │     newbie_curve adds level 1–5 factors. Level 10 does not use them.
       │   │     A soak class's Slam starts its cooldown only when the swing hits.
       │   │     wizard uses the mage row; behind_dealt applies when lower level.
       │   │     boss_mechanics: bosses and hard elites wind up one action before
       │   │     the hit; bosses enrage on round 16 or at 30% HP (1.20× damage).
       │   │     Room NPC and combat payloads include a viewer-relative threat tier
       │   │     (grey..skull). Orange+ blocks the first attack until attack! or a repeat.
       │   ├── cast <skill> [target] - Use skill (mana/cooldown cost)
       │   ├── defend - +50% defense until next turn
       │   ├── flee - Chance-based escape (50% + DEX bonus)
       │   └── timeout - Auto-defend after 60s
       └── NPC turn: AI decides action
           ├── If HP < flee threshold → attempt flee
           └── Otherwise → attack weakest player

3. RESOLUTION
   ├── Victory (all enemies dead) → XP/gold rewards scaled by threat tier against the highest level in the split (equal split: living combatants + online same-room party; leftover to the engager when it is a party share). Boss first-kill bonus is per character (`firstBossKills`). Item drops follow the party loot rule: `free` (default) leaves them in the room, while `leader`/`roundrobin`/`needgreed` auto-assign each drop to a recipient picked in `selectLootRecipient` (`loot_party.go`) and store it straight into that character's inventory.
   ├── Defeat (nobody still fighting, and at least one player actually dead) → 10% XP loss, 1 gold loss, respawn at bind point, for the dead only. A player who fled or slipped keeps hit points, gold, and room.
   └── Fled (every player fled or slipped, nobody dead) → no death penalty. NPCs reset to idle. A slip exit is left in place.
```

#### Combat Commands

| Command | Aliases | Description |
|---------|---------|-------------|
| `attack <target>` | `a`, `hit` | Attack enemy (or initiate combat) |
| `cast <skill> [target]` | `spell` | Use a skill in combat (costs mana or sets cooldown) |
| `defend` | `d`, `guard` | Take defensive stance (+50% defense) |
| `flee` | `run`, `escape` | Attempt to escape (50% + DEX bonus) |
| `skills` | `spells`, `abilities` | List/equip/unequip skills (out of combat) |
| `status` | `cs`, `combat` | Show combat status |
| `bind` | - | Bind respawn point at current room |

Combat commands validate that the character's persisted combat flag still maps to a live combat instance. If not, they clear `InCombat` and `CombatInstanceID` before responding, preventing stale state from blocking normal play.

#### Combat Update Cycle (2 seconds)

```go
func (c *CombatController) Update() {
    for _, instance := range c.manager.GetActiveInstances() {
        // Check turn timeout
        if instance.IsTurnTimedOut() {
            // Player: auto-defend, check AFK auto-flee
            // Advance to next turn
        }

        // Check global timeout (30 minutes)
        if time.Since(instance.CreatedAt) >= 30*time.Minute {
            // End combat as timeout
        }
    }
}
```

### Command System (`pkg/mudserver/game/commands/`)

#### Command Interface

```go
type Command interface {
    Execute(game def.GameCtrl, message *messages.Message) bool
    Key() CommandKey
}

type CommandKey interface {
    Matches(key string, cmd string) bool
}
```

#### Command Types

- **ExactCommandKey** - Exact match (e.g., "help")
- **StartsWithCommandKey** - Prefix match (e.g., "sc" matches "sc warrior")

#### Command Processor Flow

```
Input: "selectcharacter warrior"
       │
       ▼
Parse first word: "selectcharacter"
       │
       ▼
Look up in commands map
       │
       ▼
CommandKey.Matches("sc", "selectcharacter") → true
       │
       ▼
Command.Execute(game, message)
       │
       ▼
Return: handled (true/false)
```

#### Room Processor Flow

```
Input: "north"
       │
       ▼
Check character has room
       │
       ▼
Try static commands (n/s/e/w/look/room)
       │
       ▼
Try dynamic exits (custom exit names)
       │
       ▼
Try room actions (custom interactions)
  ├── response → reply to player
  ├── response_room → broadcast to room
  └── script → execute Lua script via Runner
       │
       ▼
Execute matched command
```

**Global chat channels** (`ChannelsCommand`): the `chat <channel> <message>`
command (plus direct shortcuts `general`/`lfg`/`roleplay`/`admin`) builds a
`MessageResponse` with `Audience: MessageAudienceGlobal` and
`Type: MessageTypeDefault`. The mudserver `receiveMessages()` loop already
fans `MessageAudienceGlobal` out to every client (`server.Broadcast`), so a
channel line needs no extra routing. The optional `Channel` field tags the
payload (`general|trade|lfg|help|roleplay|admin`); the play client routes those
tags (and `[Channel] …` prose from older senders) into its **channel** chat tab
(`public/mud-client/src/game/chatTabs.js`, `ui/ChatPanel.svelte`).
`help` and `trade` are not registered as channel shortcuts because those keys
belong to the help and merchant-list commands; they are reached via `chat`.
The `admin` channel checks `FromUser.IsAdmin()` before broadcasting. Player
guide: `docs/user/channels.md`.

**Party loot rule & ready check** (`PartyCommand`, `party_loot_ready.go`):
`party loot [rule]` reads or (leader only) writes `Party.LootRule` — `free`,
`leader`, `roundrobin`, `needgreed` — through `PartiesService().UpdateParty`,
so the rule persists on the Party row and rides the `party` payload as
`lootRule` (`PartyMessage.LootRule`). At victory, `game_combat.go` builds a
`LootAssignmentContext` from the first living player's party; when the rule is
not `free`, `selectLootRecipient` (`loot_party.go`) picks a recipient per drop
and `giveItemToCharacter` stores it in that inventory, announcing
`[Party] <item> → <name> (<reason>)` (a failed write falls back to the room).
`party readycheck` (leader only) opens a 30-second window in the in-memory
session registry (`sessions_ready.go`); `party ready` / `party notready` update
it and push `[Party] Ready: n/m · Not ready: n/m`. There is no timer goroutine —
expiry is checked lazily, so the first answer after the elapsed window sends
`[Party] Ready check over: n/m ready.` and clears the check. Neither ready
state nor the window is persisted. Player guide: `docs/user/party.md`.

**Talents & prestige** (`TalentsCommand`, `PrestigeCommand` — `talents.go`,
`prestige.go`, registered at `commandprocessor.go:150-151` as
`talents`/`talent` and `prestige`): `talents` (or `talents list`) renders the
unspent points, the prestige line, and the class talents grouped by category;
`talents spend <arg>` resolves the argument by id (with or without the
`talent_` prefix), display name, or a unique prefix of either, then checks
`CanLearnTalent` (classkit school or the class's explicit `talents:` list) and
`Talents.Points > 0` before calling `SpendTalent`. Both `talents spend` and
`prestige confirm` are refused while `combatEngine.IsPlayerInCombat` reports a
fight; the write goes through `CharactersService().Update` and is followed by a
`characterUpdate` message. `prestige` (bare) prints availability
(`leveling.MaxLevel`), `prestige confirm` runs `ApplyPrestige()` — an
irreversible reset that keeps talents, gold, gear, discoveries, friends, and
all-time stats.

### Item Commands

| Command | Aliases | Description |
|---------|---------|-------------|
| `pickup <item>` | `get`, `take` | Pick up item from room |
| `drop <item> [qty]` | - | Drop item to room |
| `examine <item>` | `inspect` | Detailed item inspection |
| `inventory` | `i` | Show inventory (categorized) |
| `equipment` | `eq`, `gear` | Show equipped items |
| `equip <item>` | `wear` | Equip item from inventory |
| `unequip <slot\|item>` | `remove` | Unequip to inventory |

### Trade Commands

| Command | Aliases | Description |
|---------|---------|-------------|
| `list` | `shop`, `trade` | Show merchant inventory |
| `buy <item> [qty]` | - | Purchase from merchant; stackable quantities can occupy one inventory stack |
| `sell <item> [qty]` | - | Sell to merchant if accepted and not bound |
| `value <item>` | `price` | Check sell price |

### Dialog Commands

| Command | Aliases | Description |
|---------|---------|-------------|
| `talk <npc>` (también `talk to <npc>`) | `speak` | Authored dialog flow (dialog graph + quest options); the main, quest-aware conversation path |
| `ask <npc> [question]` | - | Pregunta libre a un NPC habilitado para LLM (FASE 6.3, Batches 25–29): memoria y personalidad estructuradas, opciones accionables incluidas quests elegibles `quest:<id>`, continuación por texto crudo de conversación `__llm__` activa/reciente y saludo LLM desde `talk` tras diálogo autoral y quests. Ver la subsección de diálogo LLM de NPC |

### Quest Commands

| Command | Aliases | Description |
|---------|---------|-------------|
| `quests` | `ql`, `questlog` | Show quest log (active quests with progress) |
| `quest <name>` | - | Show details of a specific quest |
| `abandon <name>` | - | Abandon an active quest |
| `offer <npc>` | - | Ask an NPC quest-giver for a contextual (LLM) quest |
| `offer accept` / `offer decline` | `no`, `reject`, `pass` (decline) | Accept (persists the quest) or decline the pending offer |

**Contextual offers** (`commands/offer.go` + `game/quest_offer.go`, Batch 18): only a quest-giver NPC in the room answers (same `IsQuestGiver` contract as the dialog UI). The command builds a `ContextualQuestRequest` from the character level and `room.Area` (or `room.ID`), hands it to `Game.RequestContextualQuestOffer`, served by the provider installed at startup by `pkg/server/llm.go` (`installContextualQuestProvider` → `llmgen.GenerateContextualQuest`), and lets generation finish **outside** the command loop, publishing the prose result with `Game.Call`. Limits: 60 s per-character cooldown plus a 2-slot semaphore; with `LLM_PROVIDER=none` the generator falls back to deterministic content, and without a provider the command reports no work. The quest is written only on `offer accept` (`QuestsService.Store` + `AcceptQuest` → `questAccepted` + `questLog`), re-pointing `source`/`turnIn` at the offering NPC only while it still validates (`bindQuestToSourceNPC`).

#### QuestTracker

Listens to game events and automatically updates quest objective progress:

| Event | Source | Objectives Updated |
|-------|--------|-------------------|
| NPC killed | `game_combat.go` (processCombatVictory) | Kill objectives matching NPC template |
| Item pickup | `pickup.go` command | Collect objectives matching item template and picked-up stack quantity |
| Room enter | `room_takeexit.go` command | Visit objectives matching room ID |
| Dialog node | `talk.go` command | Talk objectives matching NPC + dialog node |
| Talk to NPC | `talk.go` command | Deliver objectives; required inventory items are consumed before progress is granted |

Progress updates include the changed objective when available. If all objectives are complete, the tracker sends a `questReady` message instead of a generic progress message so the client can show turn-in state clearly.

When talking to a quest-source NPC, quest dialog options (offer, progress check, turn-in) are automatically injected into the NPC's conversation. NPCs that have quests but no full dialog tree use a synthetic quest-only conversation so numeric option selection still goes through `DialogSelectCommand`. Dialog quest actions send enriched `questLog` WebSocket payloads using the same quest definition details as character selection.

### Service Layer (`pkg/service/`)

Business logic layer using the Facade pattern.

#### Facade Interface

```go
type Facade interface {
    CharactersService() CharactersService
    PartiesService() PartiesService
    UsersService() UsersService
    RoomsService() RoomsService
    ScriptsService() ScriptsService
    ItemsService() ItemsService
    NPCsService() NPCsService
    NPCSpawnersService() NPCSpawnersService
    DialogsService() DialogsService
    ConversationsService() ConversationsService
    LootTablesService() LootTablesService
    ServerSettingsService() ServerSettingsService
    QuestsService() QuestsService
    SkillsService() SkillsService
    CharacterTemplatesRepo() repository.CharacterTemplatesRepository
    GuestService() GuestService
    GuestStatsService() GuestStatsService
    AuditService() AuditService
    ClansService() ClansService
    DMService() DMService
    NPCMemoriesService() NPCMemoriesService
    DungeonsService() DungeonsService

    Runner() scripts.ScriptRunner
}
```

#### Service Responsibilities

| Service | Responsibilities |
|---------|------------------|
| UsersService | User CRUD, online status, find/create on login, role management (SetRole), ban/unban, admin sync from env var |
| CharactersService | Character CRUD, templates, name validation |
| RoomsService | Room CRUD, room queries |
| ItemsService | Item CRUD, create from template |
| ScriptsService | Script CRUD, execution |
| PartiesService | Party/group management. Follow state is in-memory on the game session registry, not on the Party row |
| LootTablesService | Loot table CRUD, loot rolling |
| QuestsService | Quest definition CRUD, quest progress tracking, normalized event application, accept/abandon/complete/turn-in quests, objective progress, prerequisite checks |
| SkillsService | Skill CRUD, DB seeding on first run, in-memory cache refresh on mutations |
| GuestService | Guest session creation, HMAC token signing/validation, expired guest cleanup, IP rate limiting |
| ClansService | Clan CRUD plus membership/invite/bank rules: one clan per character, leader/officer authorization, invite lifecycle, append-only bank ledger |
| DMService | Private messaging: per-character channels, canonical find-or-create, participant-validated send/read, unread counters |
| AuditService | Content edit history (`audit_log`): before/after snapshots, actor, inverse payload for undo, search invalidation hooks |
| ServerSettingsService | Persisted server settings document (`server_settings`) used by the creator settings UI |
| GuestStatsService | Guest usage counters behind `GET /api/admin/stats/guests*` |
| NPCMemoriesService | Windowed long-term NPC memory over `npc_memories` (keyed `<npcTemplateID>::<characterID>`, §6.3). Exposed on the facade and consumed by the game since Batch 25 (`loadNPCMemoryFull`/`recordNPCMemoryTurn` in `pkg/server/llm.go` feed the `ask` dialogue provider); since Batch 26 it also persists async LLM summaries (`triggerNPCMemorySummaryIfNeeded`, `TaskNPCMemorySummary`) with provenance (`SummarySource`: `"deterministic"` \| `"llm"`), which later deterministic rebuilds respect |
| DungeonsService | Dungeon designs for FASE 5.1: validates name/grid/opaque `definition` JSON, preserves `CreatedAt` on update, backs `/api/dungeons` |

`pkg/worldmap` is a layout compiler (not a facade service). `Compile` pins authored coords, clusters by area, and packs zones apart; `Reveal` applies per-character discovery. `worldmap.MarkOn` records entered rooms on the character document during `TakeExit`, party-follow relocation, and character select. Party Follow v1 pulls online followers only after a normal exit walk.

#### Creator Validation Service

`pkg/service/validation` provides shared validation rules for Creator-authored content. It loads a `WorldSnapshot` from the facade, validates draft JSON entities and stored world data, and returns structured issues with severity, entity type, entity ID, field path, code, and message. Creator save handlers use the same rules to reject broken references before writes, and `/api/diagnostics/world` runs the rules across the whole world for health checks.

#### LLM Provider Services (`pkg/service/llm`, `pkg/service/ollama`, `pkg/service/groq`)

AI text generation (character names and descriptions for `POST /api/generate/character`) goes through the provider-agnostic `llm.Client` interface — `Complete(ctx, systemPrompt, userPrompt, temperature) (string, error)` — so the backend can swap providers without touching handlers.

| Package | Provider | Configuration |
|---------|----------|---------------|
| `pkg/service/ollama` | Local Ollama daemon via its OpenAI-compatible `POST {OLLAMA_BASE_URL}/v1/chat/completions`. Default provider; no API key required | `OLLAMA_BASE_URL` (default `http://localhost:11434`), `OLLAMA_MODEL` (default `llama3.1`), `LLM_TIMEOUT_SECONDS` (default 60) |
| `pkg/service/groq` | Hosted Groq API (`https://api.groq.com/openai/v1/chat/completions`, model `llama-3.3-70b-versatile`, fixed 15s timeout) | Requires `GROQ_API_KEY`; `NewClient` returns nil without it |

`llm.NewFromEnv()` is the environment-driven factory. `LLM_PROVIDER` selects the provider (`ollama` | `groq` | `none`/`off`, case-insensitive; unknown or empty values disable generation). When `LLM_PROVIDER` is unset, legacy behavior is preserved: Groq when `GROQ_API_KEY` is present, otherwise Ollama so key-less deployments generate out of the box. A nil return means "generation disabled" (not an error); the handler answers `503 AI generation is not configured`. Both provider clients satisfy the contract, enforced by compile-time assertions in `pkg/service/llm` (they cannot live in the provider packages, which would create an import cycle).

Deployment: `docker-compose.yml` passes the LLM variables to the `talesmud` service and defaults `OLLAMA_BASE_URL` to `http://ollama:11434`. The `ollama` service is opt-in via `profiles: ["ollama"]` (start with `docker-compose --profile ollama up -d`) and `talesmud` does not depend on it, so the server still boots with generation disabled/unreachable when the sidecar is not running.

#### Phase 6 LLM packages (design; implementation in progress)

The Fase 6 design ([`docs/design/PHASE6_LLM.md`](docs/design/PHASE6_LLM.md), §0–6.7) defines two new service packages **built on top of** the existing `llm.Client` contract (the contract and `NewFromEnv()` are not replaced; extensions are additive optional interfaces by type-assert):

| Package | Responsibility | Status (2026-10-14) |
|---------|----------------|----------------|
| `pkg/service/llmprompt` | Versioned prompt templates: one YAML manifest per task under `prompts/` (`//go:embed`), rendered with `text/template`. Declares variables (`required`, `untrusted`, `maxRunes`), sanitises them, wraps untrusted blocks in `UNTRUSTED INPUT` (generalising `handler/generate.go:209`), and ships a deterministic `fallback` body. Exposes `Registry.Get/List/Render` → `Rendered` (`Task`, `Version`, `Hash`, `System`, `User`, `Temperature`, `MaxTokens`, `Timeout`, `Output`, `Retry`, `Fallback`) | **Present (uncommitted)**: `prompt.go`, `registry.go`, `prompts/` with `character.name`, `character.description`, `npc.dialogue`, `quest.generate`, `room.description`, `dm.narration`; consumed by `llmgen`, `handler/generate.go` and `pkg/server/llm.go` (`LLM_PROMPT_DIR` overrides the embed dir) |
| `pkg/service/llmgen` | Generation pipeline: context vars → `llmprompt.Render` → `llm.Client` (ctx deadline + global semaphore) → parse (text / first balanced JSON block) → domain validation → repair retry with validation feedback → cache / rate limit / content filter → deterministic fallback. Also holds `questgen` and `npcgen` business logic. Must not import `pkg/mudserver` or `pkg/server`; game consumers import `llmgen` | **Present (uncommitted)**: `pipeline.go` (+ tests) plus the cache/config/fallback/filter/limiter/parse/stats/catalog blocks; consumers: `handler/generate.go`, `handler/llm.go` (`questgen` + `questgen_context`) and the contextual-quest provider installed in `pkg/server/llm.go` for the `offer` command |

Consumers: **in use** — `handler/generate.go` (character name/description on `llmprompt.Registry` + `llmgen.Pipeline`, `LLM_PROMPT_DIR` overrides the embedded templates); `handler/llm.go` (creator endpoints: quests generate/contextual, room description, prompt overrides/tests); `pkg/server/llm.go` (`buildLLMRuntime` + `installDMNarrator` → `game.SetDMNarrator`, `installContextualQuestProvider` → `game.SetContextualQuestProvider` for the `offer` command, and `installNPCDialogueProvider` → `game.SetNPCDialogueProvider` for the `ask` command). The additive `ClientWithOptions` / `CompleteWith(ctx, system, user, Options)` extension is **already implemented**: `pkg/service/llmopts` defines `Options{Temperature, MaxTokens}`, `llm.Options` aliases it, and both providers honour it (0 keeps the provider default of 300, so `max_tokens` is no longer hard-coded).

#### DM subsystem (FASE 6.5, modes A+B — Batches 24/30)

A reactive prose narrator (mode A) that comments game events and **never mutates state**, plus an opt-in validated decision layer (mode B) that may add one ambient twist. Mode B is off by default for beta (`LLM_DM_DECISION_ENABLED`). Verified against the code 2026-11-10.

- **Event bus:** hooks post a typed `def.DMEvent{Kind, RoomID, CharacterID, UserID, Facts}` (`pkg/mudserver/game/def/gamectrl.go`) through `GameCtrl.NotifyDM`. Kinds: `combatStart` (`game_aggro.go`, after `BeginEngagement`), `combatEnd` (`game_combat.go`, `cleanupCombatInstance`), `questCompleted` and `levelUp` (`commands/dm_notify.go`, called from the quest-complete and level-up paths), `roomEnter` (`commands/room_takeexit.go`, only when the **area** changes).
- **Flow (off the tick loop):** hook → `NotifyDM` (`dm_narrator.go`, adapts `Facts` → `DMNarrationRequest`, resolves audience) → throttles — per-room (or per-user for private events) cooldown, default 45 s, and a global sliding-window budget, default 10/min — → non-blocking semaphore claim (max 2 concurrent) → goroutine with a context timeout (default 8 s, **single attempt**) → `llmgen.Pipeline.Generate` (`TaskDMNarration`, `RequireLLM: true`, installed by `pkg/server/llm.go` `installDMNarrator`) → output filter (`OutputAllowed`: ≤240 runes, no suspicious ids) + `cleanDMText` → the line is posted back through `game.Call` as a normal message with `Username: "DM"`.
- **Audience:** `Room` for combat/roomEnter events, `User` for questCompleted/levelUp; global/system audiences are rejected. The narrator never broadcasts and never includes other players' private data (only code-derived `Facts` reach the prompt).
- **Never in the tick, never mutates:** the provider call runs in its own goroutine; game state is untouched on every path.
- **Fallback = silence:** no provider, kill switch off (`LLM_DM_ENABLED`), cooldown/budget denial, timeout, provider error or filtered output → nothing is emitted (`llmgen/fallback.go` returns an empty response for `TaskDMNarration`; the prompt template's `fallback` is `""`).
- **Config:** `LLM_DM_ENABLED`, `LLM_DM_COOLDOWN_SECONDS`, `LLM_DM_TIMEOUT_SECONDS`, `LLM_DM_MAX_PER_MIN` (`.env.example`, `docker-compose.yml`; read once in `game.go` `New`). Note: the per-request pipeline timeout in `installDMNarrator` is hardcoded to 8 s; the game-side context honours the env knob.
- **Mode B (B30, opt-in):** `pkg/mudserver/game/dm_decision.go` defines `DMDecision{Kind, TemplateID, Count, Message, Reason}` with the whitelist `none|ambush|environmental|narrative`; `installDMDecision` (`pkg/server/llm.go`) no-ops without an LLM or with the switch off. `NotifyDM` is the single entry point for `roomEnter` and `combatStart`, so prose and decisions cannot double-fire. Guardrails: per-room cooldown, request timeout, per-minute budget, ambush template id resolved against the live template catalog, ambush count clamped to 1..3, prose capped and broadcast through `game.Call`, and atomic counters for requested/allowed/rejected/emitted decisions.
- **Mode B config:** `LLM_DM_DECISION_ENABLED` (default off), `LLM_DM_DECISION_COOLDOWN_SECONDS` (60), `LLM_DM_DECISION_TIMEOUT_SECONDS` (8), `LLM_DM_DECISION_MAX_PER_MIN` (10), read once in `game.go` `New`; the provider is installed only when the switch is truthy.
- **Tests:** `pkg/mudserver/game/dm_narrator_test.go`, `pkg/mudserver/game/dm_decision_test.go`, `pkg/mudserver/game/commands/dm_notify_test.go`, `pkg/service/llmgen/fallback_dm_test.go` (+ `OutputAllowed` cases in `pipeline_test.go`). QA §B24: verified (APROBADO, 2026-10-22).

#### NPC runtime LLM dialogue subsystem (FASE 6.3, cortes 1+2+3 + mecanismos B/C — Batches 25–29)

Diálogo libre de NPC: el jugador puede usar `ask <npc> [pregunta]` o continuar una conversación LLM con texto crudo; `talk` saluda con LLM solo si no hay diálogo autoral ni quests disponibles. El pipeline incluye memoria persistente por personaje, personalidad estructurada, opciones accionables y relación dinámica. El diálogo autoral y las quests mantienen prioridad. Verificado contra el código 2026-11-01; sin commitear.

**Batch 29 (mecanismos B/C y opciones quest):** `talk` prioriza diálogo autoral → quests disponibles → saludo LLM. Si el proveedor está deshabilitado, conserva el mensaje actual de NPC sin diálogo. `game.processIncoming` intercepta la entrada solo cuando no coincide comando, acción de sala ni habilidad; requiere NPC LLM habilitado, vivo y en la misma sala, además de sesión dialog abierta o `LastInteracted < 60 s`. Envía el texto crudo con `Continuation=true`; la conversación `__llm__` se cierra únicamente cuando se programa la petición y se conserva ante `RateLimited`, `Busy` o `Disabled`. El prompt `npc.dialogue` v1.0.3 recibe `AvailableQuestPromptContext`; el validador admite únicamente IDs de quests disponibles y la selección enruta `quest:<id>` antes de `NodeID` por `handleQuestDialogOption`. Una quest inexistente/no disponible se informa sin mutar el estado de quest. El contrato de mensaje expone `DialogOption.Kind` y `DialogMessage.FreeText`; el store recibe `freeText` y `DialogOverlay` envía el texto crudo. Son guardrails básicos de elegibilidad, no guardrails completos de combate; `FallbackDialogID` sigue sin ser fallback de error y se usa solo para mapear etiquetas a diálogo autoral.

**Request/data flow:** `talk` resolves authorial dialog and quest availability before calling `RequestNPCDialogue` for an automatic greeting. Unmatched text reaches `LLMDialogFreeText` from `game.processIncoming`, which selects the latest active `__llm__` conversation and checks NPC/session eligibility before scheduling a continuation. The server provider calculates the current quest prompt and ID allowlist together; the option handler re-resolves the quest choice at selection time and delegates to `handleQuestDialogOption`. The dialog payload uses the additive `DialogOption.Kind` / `DialogMessage.FreeText` contract; `DialogOverlay` passes entered text as `freeText` through the client store.

- **Command:** `AskCommand` (`pkg/mudserver/game/commands/ask.go`, registered in `commandprocessor.go` as `ask`, no aliases) resolves the longest NPC-name prefix in the current room (`resolveAskTarget`, tolerating "ask to …") and splits the remainder as the (optional) question. NPCs without the opt-in — `!target.LLMEnabled()`, i.e. `llm.enabled` unset/false — get an explicit notice and are routed through the shared `talkToNPC` helper (extracted from `commands/talk.go`), the exact same authored dialog/quest flow `talk` uses. An empty question is allowed (a greeting). With the adventure pack imported, **Master Aldric** (`TUT_MENTOR`, `import/adventure/data/npcs/tut_mentor.yaml`) ships with `llm.enabled: true` (persona, voice, Big5, goals, one secret, a `player/mentor` relation), so `ask Master Aldric …` in R1001 exercises the full path whenever a provider is installed.
- **Scheduling (`RequestNPCDialogue`, `pkg/mudserver/game/npc_dialogue.go`):** validates provider + IDs, then applies the per-`(characterID, npcID)` cooldown (`npc.EffectiveCooldownSeconds()`, default 3 s, `npcDialogueAllow`), a non-blocking semaphore of **2** concurrent generations (`NPCDialogueBusy` when full), and a **20 s** context timeout. With the game loop running, the provider executes on a goroutine and the reply is posted back via `g.Call` — the LLM call never runs on the command loop (without a running loop — tests/startup — generation runs inline). Statuses: `NPCDialogueScheduled`, `NPCDialogueDisabled` (no provider or missing IDs → the command degrades to `talkToNPC` with a notice), `NPCDialogueRateLimited`, `NPCDialogueBusy`.
- **Config (`npc.LLMConfig`, `pkg/entities/npcs/llm.go`, corte 2):** the per-NPC opt-in lives in the NPC's JSON blob (`npcs.data.llm`, no migration) and carries `Enabled`, `Persona`, `Voice`, `Temperature`, `MemoryTurns`, `CooldownSeconds`, `FallbackDialogID` (since corte 3 it is read as a **secondary source of the authored dialog** when mapping LLM option labels — `npcDialogueAuthoredDialogID` — but it is still not an error-fallback dialog) plus the corte-2 fields: `Personality *LLMPersonality` (Big5 — openness/conscientiousness/extraversion/agreeableness/neuroticism, each in [0,1]), `Goals []string`, `Secrets []string` and `Relations []LLMRelation` (`Target` = NPC/faction/`"player"`, `Kind ∈ {ally, rival, family, faction, mentor}`, `Sentiment` in [-1,1]). Helpers: `LLMEnabled()`, `EffectiveMemoryTurns()` (default 8), `EffectiveCooldownSeconds()` (default 3), `EffectivePersonality()`, `HasPersonality()` and a deep `Clone()`. **Three authoring paths:** (1) YAML content packs — `importer.YAMLNPC.LLM` (`pkg/importer/models.go`, camelCase `yaml:` tags) deep-copied into the entity by `ToEntity()` (`converters.go`); (2) NPCTemplate REST — the `llm` field rides the detail DTO and the upsert request of `handler/npc_templates.go` (copied with `Clone()` both ways); (3) the play client's NPC factory panel ("Dialogue / LLM", `NpcFactory.svelte` + `normalizeLLM` round-trip in `npcFactory.js`). Spawned instances inherit the template's config via `template.LLM.Clone()` in `SpawnFromTemplate` (`pkg/service/npcs.go`), so an instance cannot mutate the template's goals/secrets/relations.
- **Provider (`installNPCDialogueProvider`, `pkg/server/llm.go`):** no-op without a live `llmgen.Pipeline`. Generates through `llmgen.Pipeline.Generate` with `TaskNPCDialogue`, `RequireLLM: true`, a 20 s timeout, per-request temperature and rate-limit bucket `npc:<npcID>:<characterID>`. The `npc.dialogue` template is v1.0.3 and receives quest context from `AvailableQuestPromptContext`; validation only accepts currently available quest IDs. It also includes personality, relations and memory; validates the reply/options, runs the content filter and secret-leak guard, and records the turn.
- **Secret-leak guard (corte 2):** `validateNPCDialoguePayloadWithSecrets` wraps the generic validator with `leaksSecret(text, cfg.Secrets)` over the player-facing **and** replayed fields — `reply`, every `options[i].label` and every `memoryNotes[i]`: both sides are normalised (lowercase, punctuation → spaces, collapsed whitespace) and matched on token boundaries, so text that reproduces a configured secret verbatim fails validation with `secret_leak` on the offending path; the pipeline then retries with feedback and, if the leak persists, the game falls back to the neutral line. Paraphrases/insinuations are intentionally **not** rejected — only literal disclosure is. (Both QA B26 observations are closed: this guard covers `options`/`memoryNotes` as well as `reply`, and template reads redact secrets — see below.)
- **Memory (`NPCMemoriesService`, `pkg/service/npcmemories.go` + `pkg/repository/npcmemories_sqlite.go`, table `npc_memories`):** one row per `(npc, character)` keyed `<npcTemplateID>::<characterID>`. `RecordTurn` bumps the turn counter, merges/dedupes facts (cap 24), stores the last user line and — when the turn count crosses `SummaryAtTurns()` (default 8) — rebuilds the `Summary` **deterministically** (facts + last question concatenated) *unless* the stored summary must be preserved. Provenance lives in `repository.NPCMemory.SummarySource` (`"deterministic"` | `"llm"`, stored inside the JSON blob — no migration; empty = legacy row): a summary written by the async LLM summariser (`"llm"`) or an external/author-edited summary that differs from the previous deterministic rebuild is left untouched by subsequent turns. Defaults: summary at 8 turns, hard cap 50 turns.
- **LLM memory summary (corte 2):** `triggerNPCMemorySummaryIfNeeded` (`pkg/server/llm.go`) fires after each recorded turn once `Turns % SummaryAtTurns() == 0`, off the tick in a fire-and-forget goroutine (12 s context): it calls `llmgen` with `TaskNPCMemorySummary` (template `npc.memory_summary.yaml`, temperature 0.2, `RequireLLM: false`, no cache TTL, limit key `npcmem:<npcID>:<characterID>`) feeding NPC name/persona, facts, last user line and the existing summary; the model returns `{"summary": "…"}` capped at 400 runes. On empty/failed output the deterministic summary stays; on success the row is reloaded (to avoid clobbering concurrent turns) and saved with `SummarySource = "llm"`.
- **Delivery (`deliverNPCDialogue`, `npc_dialogue.go`, cortes 1–3 + Batch 29):** the reply is sanitised and options may be `flavor`, `end` or validated `quest:<id>`. With Conversation Service, `sendNPCDialogueDialog` sends numbered options under `DialogID="__llm__"`; quest options retain their `Kind` for routing into the shared quest action handler. Provider error/timeout/filter failure still returns the neutral line; `FallbackDialogID` is not an error fallback.
- **Option selection (corte 3 + Batch 29):** selection routes `end`, authored `NodeID`, `flavor` continuation or `quest:<id>`. Quest IDs are re-resolved against the current quest options and passed to `handleQuestDialogOption`; absent/ineligible IDs report without quest mutation.
- **Dynamic relations (corte 3):** `NPCDialogueResult.RelationDelta` is clamped per exchange by `npc.ClampRelationDelta` (±0.05, `pkg/entities/npcs/llm.go`) when parsing the payload, and `applyNPCRelationDelta` (`pkg/server/llm.go`) persists it through `NPCMemoriesService.AdjustRelation` → `npc.ApplyRelationDelta` onto `NPCMemory.Relations` (`map[characterLowercase]float64`, cumulative clamp to [-1,1], stored inside the `npc_memories` JSON — no migration, defaults to `player`). A missing delta defaults to **+0.02** (0 when the chosen option is `end`), so a well-behaved model never needs to emit the field. `mergeDynamicRelations` injects the stored value into the prompt as `player(dinámica,0.04)`, replacing the static `player` entry; `formatNPCPersonalityContext`'s static relations are unaffected.
- **Client feedback (corte 3 + wiring B28):** `public/mud-client/src/game/relationFeedback.js` extracts a relation cue from the message payload; `Client.js` appends it to dialogue/plain messages and passes it to `mux.setDialog`; `DialogOverlay.svelte` renders a subtle ▲/▼/• row. Batch 28 adds `relationDelta` to `MessageResponse` and attaches the successfully persisted/clamped delta to `DialogMessage` and plain NPC replies, completing the server→client path. No cue is rendered if no relation change is sent; it is cosmetic only.
- **State invariant:** generar una respuesta u opción no muta por sí solo el estado de quests. Una opción `quest:<id>` elegida explícitamente y revalidada delega en la acción de quest existente y puede modificar su estado. Las demás escrituras del diálogo LLM son filas `npc_memories` (hechos/resumen y relación dinámica, contexto de rol). No hay anillo `llm_npc_session.go`; proveedor, cooldown y semáforo viven en `Game` detrás de `npcDialogueMu`.
- **Secrets handling on template reads (B26 observation, closed):** `GET /api/npc-templates/:id` serves the `llm` config to any authenticated user, but `llm.secrets` is redacted unless the caller is a creator/admin (`canViewNPCTemplateSecrets`, fail-closed on missing/unknown user context; `npcTemplateDetail(tpl, includeSecrets)`); write routes (creator-gated) always round-trip the full config. Test: `TestNPCTemplateGetRedactsSecretsForNonCreators`.
- **Known limits (debt):** la caché del pipeline `npc.dialogue` (TTL 30 min) puede repetir la respuesta ante un prompt idéntico; `FallbackDialogID` es solo fuente secundaria para mapear etiquetas al diálogo autoral, no fallback de error; el anillo de memoria corta y los guardrails completos de combate siguen pendientes. Los mecanismos B/C y las opciones de quest validadas están implementados.
- **Tests:** `pkg/mudserver/game/commands/ask_test.go` (7: degradation without LLM/provider, provider call + delivery, empty question, per character+NPC rate limit, unknown NPC, provider error still answers), `pkg/mudserver/game/commands/llm_dialog_test.go` (5 — corte 3: `flavor` → follow-up request with `Continuation`, `end` closes, unknown selection keeps the options, a mapped option continues the authored node, a closed conversation is not re-opened by a stale NPC session), `pkg/mudserver/game/npc_dialogue_test.go` (`TestMatchAuthoredDialogLabelFindsNodeAndOption` plus the corte-1 scheduling/delivery suite), `pkg/server/llm_npc_test.go` (8: memory recorded, neutral fallback on error, empty reply rejected, memory persists across turns, secrets passed to prompt, literal secret leak rejected, leak guard over `options`/`memoryNotes` — validator + payload — and `TestInstallNPCDialogueProviderAdjustsRelation`: delta 0.04 persisted, prompt shows `player(dinámica,0.04)`, −0.20 clamped to −0.01), `pkg/service/npcmemories_test.go` (deterministic rebuild, LLM summary preserved, external summary without marker preserved, `TestAdjustRelationClampsAndPersists`), `pkg/service/npc_templates_test.go` (`TestSpawnFromTemplateInheritsLLM`), `pkg/importer/llm_yaml_test.go` (camelCase YAML → entity deep copy), `pkg/server/handler/npc_templates_test.go` (`TestNPCTemplateGetRedactsSecretsForNonCreators`) and `public/mud-client/src/game/relationFeedback_test.mjs` (frontend, run with `node --test`).

Cuts: **A** MVP (templates + pipeline + character generation) — **landed**; **B** NPC runtime LLM — **landed as cortes 1–3 plus mechanisms B/C in Batches 25–29** (`ask`, persistent memory, personality, actionable options, `talk` priority authored dialog → quests → LLM greeting, raw-text continuation of recent `__llm__` conversations, validated `quest:<id>` actions, dynamic relations and client feedback); **C** maturity — implemented in part (quest generator, DM mode A and Creator endpoints; DM mode B remains pending). Guardrails include validation, rate limits, content filtering and literal-secret rejection; complete combat guardrails and the short-term memory ring remain pending. `FallbackDialogID` is not an error fallback. Verified 2026-11-01 against the working tree; uncommitted.

### Estado actual de los batches (corte 2026-10-30)

Resumen consolidado de los batches 14–27 en lo que toca a la capa de servicio (detalle en `FEATURES.md`):

> Actualización Batch 29 (2026-11-01): la nota NPC de este resumen queda ampliada a FASE 6.3 mecanismos B/C y opciones quest `quest:<id>` validadas; la descripción completa está en “NPC runtime LLM dialogue subsystem” arriba. No implica la implementación del anillo de memoria corta, guardrails completos de combate ni `FallbackDialogID` como fallback de error.

- **DM narrator (FASE 6.5, modos A+B, Batches 24/30): implementado** — bus `DMEvent`, los 5 eventos, guardrails y fallback silencio (modo A) más `DMDecision` opt-in con ambush/environmental/narrative, presupuesto y métricas (modo B; ver "DM subsystem" arriba). Modo B apagado por defecto en beta; QA §B24 verificado (APROBADO).
- **NPC runtime LLM (Fase 6.3, cortes 1–3 + mecanismos B/C, Batches 25–29): implementado.** El comando `ask <npc> [pregunta]` programa generaciones vía `RequestNPCDialogue` (`pkg/mudserver/game/npc_dialogue.go`, cooldown por personaje+NPC, semáforo de 2, timeout 20 s, fuera del bucle con `game.Call`), el proveedor `installNPCDialogueProvider` (`pkg/server/llm.go`, plantilla `npc.dialogue`, `RequireLLM`, filtro `OutputAllowed`, fallback neutro) consume y actualiza la memoria `npc_memories` vía `NPCMemoriesService` (`loadNPCMemoryFull`/`recordNPCMemoryTurn`), y los NPCs sin `llm.enabled` degradan al diálogo autoral (`talkToNPC`, compartido con `talk`). **Corte 2 (Batch 26):** `LLMConfig` con personalidad Big5, `Goals`, `Secrets` y `Relations` inyectados al prompt; resumen LLM asíncrono (`npc.memory_summary`) con preservación vía `SummarySource`; config por YAML importer, NPCTemplate REST y NPC factory UI; spawn con `template.LLM.Clone()`; guard anti-fuga de secretos literales sobre `reply`, `options[].label` y `memoryNotes`, y redacción de `llm.secrets` en `GET /api/npc-templates/:id` para no-creators (las dos observaciones menores de QA B26, cerradas). **Corte 3 (Batch 27):** `options[]` accionables sobre la conversación centinela `__llm__` (`sendNPCDialogueDialog` + `handleLLMDialogSelection` en `commands/llm_dialog.go`: `end` / nodo autoral vía `talkToNPCAt` / `flavor` con `Continuation`), label→nodo autoral vía `matchAuthoredDialogLabel` (instancia → plantilla → `FallbackDialogID`), relaciones dinámicas `relationDelta` (±0.05 por intercambio, acumulado [-1,1] en `NPCMemory.Relations`, prompt `player(dinámica,x.xx)`), `npc.dialogue` v1.0.2 y feedback sutil en el cliente (`relationFeedback.js` + `DialogOverlay.svelte`); QA **APROBADO CON OBSERVACIONES** (una observación no bloqueante ajena al batch: test de combate intermitente `TestWardGritAndRetaliate` en `ward_test.go`, los `TestWardGuard*` pasan). **Batch 29:** mecanismos B/C implementados (auto-saludo en `talk` con prioridad autoral → quest → LLM, texto libre sobre conversación `__llm__` elegible) y `kind=quest:<id>` validado contra quests disponibles; `npc.dialogue` v1.0.3. Siguen pendientes el anillo de memoria corta, los guardrails completos de combate y `FallbackDialogID` como fallback de error. Ver "NPC runtime LLM dialogue subsystem" arriba.
- **`LevelCap` unificado** a `ruleset.LevelCap()` en `pkg/mudserver/game/leveling` (sin gateo duro `MaxLevel=50`).
- **Inventario visual (FASE 3.5):** `inventoryEngine.js` + `InventoryWidget.svelte` (tecla `I`); el peso es **solo aviso visual** — `maxWeight = 100` fijo, sin `weight` del servidor y sin encumbrance ni rechazo de `get`.
- **Audit Log viewer (FASE 5.4):** `AuditPanel.svelte` + `auditView.js` (filtros, búsqueda, paginación, export CSV); consolidado.
- **Nuevos servicios (B16–B19):** `ClansService`, `DMService`, `ServerSettingsService`, `GuestStatsService`, `AuditService`, `NPCMemoriesService` y `DungeonsService`, con `AuditWrites` en los handlers creator.

---

### Repository Layer (`pkg/repository/`)

Data access layer with SQLite backend using JSON document storage.

#### Generic Repository

```go
type sqliteGenericRepo struct {
    db        *sql.DB
    table     string
    generator func() interface{}  // Entity factory
}
```

#### Operations

- `FindByID(id)` - Single entity by ID
- `FindByField(key, value)` - Query by field
- `FindAll(collector)` - Stream all with callback
- `FindAllWithParam(params, collector)` - Parameterized query
- `Store(entity)` - Insert new
- `Update(entity)` - Update existing
- `Delete(id)` - Remove entity

#### Specialized Repositories

Most repositories wrap the generic JSON store and add filtered lookups. A few add domain logic on top:

- **`ClanRepository`** (`clans_sqlite.go`) — wraps four generic tables (`clans`, `clan_members`, `clan_invites`, `clan_bank_ledger`). `AddMember` enforces one clan per character (checked before insert, backed by a unique index on `characterID`+`clanID`). `Deposit`/`Withdraw` update the clan bank and append an immutable ledger row.
- **`DMRepository`** (`dm_sqlite.go`) — custom implementation because `dm_messages` has real projection columns. `FindOrCreateChannel` canonicalizes the two participant IDs so a pair maps to exactly one channel; `FindMessagesSince` uses `channel_id`/`sent_at` directly.
- **`DungeonsRepository`** (`dungeons_sqlite.go`, FASE 5.1) — single `dungeons` table; list filtering by `name`/`author` via `json_extract` indexes. The `definition` blob is stored/returned as authored (opaque JSON).
- **`NPCTemplatesRepository`** (`npc_templates_sqlite.go`, FASE 5.3) — `npc_templates` table of mob/NPC blueprints authored by the client's NPC factory, with `Behavior`/`Scaling` kept as raw JSON; indexes on `name` and `role`. `Drop()` exists for tests. **No service/route consumes it yet.**
- **`NPCMemoriesRepository`** (`npcmemories_sqlite.go`) — long-term NPC memory rows for Fase 6.3, backed by `NPCMemoriesService`.

Exposed through `Factory` as `Clans()`, `DMChannels()`, `Dungeons()`, `NPCTemplates()` and `NPCMemories()`.

### Database Layer (`pkg/db/` + `pkg/db/sqlite/`)

SQLite JSON document storage with one row per entity, using JSON1 extension for queries, WAL mode, and busy timeout.

#### Query Parameter Builder

```go
NewQueryParams().
    With(QueryParam{Key: "area", Value: "sampletown"}).
    With(QueryParam{Key: "roomType", Value: "tavern"})
```

#### Collections

| Collection | Entity |
|------------|--------|
| users | User accounts |
| characters | Player characters |
| charactertemplates | Character blueprints |
| rooms | Game locations |
| items | Items (templates and instances, distinguished by `isTemplate` field) |
| npcs | NPC templates and singletons |
| npc_spawners | NPC spawn point definitions |
| dialogs | Dialog trees |
| conversations | Active dialog sessions |
| scripts | Game scripts |
| parties | Player groups |
| loot_tables | Loot drop configurations |
| quests | Quest definitions (objectives, rewards, prerequisites) |
| quest_progress | Per-character quest state and objective progress |
| skills | Skill/spell definitions (multi-class, cached in memory) |
| clans | Clans (name, tag, leader, bank, level/XP, MOTD, soft-delete `active`) |
| clan_members | Character ↔ clan membership (rank, contribution); one clan per character |
| clan_invites | Pending/answered clan invitations |
| clan_bank_ledger | Append-only audit trail of clan bank deposits/withdrawals |
| dm_channels | Private conversation channels between two characters |
| dm_messages | Private messages; carries real `channel_id`/`sent_at`/`from_char_id` columns (see note) |
| server_settings | Persisted server settings document (creator settings UI) |
| guest_statistics | Guest usage counters (`/api/admin/stats/guests*`) |
| character_resources | Optional per-character refilling resources (`config/ruleset.yaml` keys) |
| content_health | Content-health counters/mutes (`tales -check`, `GET /api/health`) |
| audit_log | Content edit history with before/after snapshots and undo metadata |
| ssh_keys | SSH public keys proven by signature challenge |
| dungeons | Dungeon designs (FASE 5.1): name/grid metadata + opaque `definition` JSON; indexes on `name`, `author` |
| npc_templates | NPC/mob blueprints (FASE 5.3): entity `pkg/entities/npcs/templates.go`, `Behavior`/`Scaling` raw JSON; indexes on `name`, `role`. Repo exists, no service/route consumer yet |
| npc_memories | Long-term NPC memories (Fase 6.3), keyed `<npcTemplateID>::<characterID>` |

**`dm_messages` indexing note:** this is the one table that deliberately deviates from the pure `(id, data)` JSON pattern. It adds real `channel_id TEXT`, `sent_at INTEGER` (Unix milliseconds) and `from_char_id TEXT` columns plus `idx_dm_messages_channel_time (channel_id, sent_at DESC)` and a partial unread index. The columns are always written in the same `INSERT` as the JSON payload and are treated as read-only projections, so `FindMessagesSince` can use `WHERE channel_id = ? AND sent_at > ?` instead of `json_extract`.


## Entity Model

### Base Entity

```go
type Entity struct {
    ID string `json:"id"`  // UUID
}
```

### Trait Composition

Entities use embedded traits for shared behavior:

```go
// Location tracking
type CurrentRoom struct {
    CurrentRoomID string `bson:"currentRoomID"`
}

// User ownership
type BelongsUser struct {
    BelongsUserID string `bson:"belongsUserID"`
}

// Inspection text
type LookAt struct {
    Detail string `bson:"detail"`
}
```

### Character Entity

```go
type Character struct {
    *entities.Entity
    traits.BelongsUser
    traits.CurrentRoom

    Name, Description string
    Race, Class

    CurrentHitPoints, MaxHitPoints int32
    CurrentMana, MaxMana int32          // Mana resource (casters only)
    XP, Level int32
    Gold int64

    Inventory      items.Inventory
    EquippedItems  map[items.ItemSlot]*items.Item
    EquippedSkills []string             // Skill IDs in active slots

    // Combat state
    InCombat         bool    // Currently in combat
    CombatInstanceID string  // Active combat instance
    BoundRoomID      string  // Respawn location (set via /bind)

    // Scripting flags (puzzle state, quest progress, etc.)
    Flags map[string]interface{}
    FriendIDs []string  // other character IDs

    // Progression (Phase 2.4) — JSON field on the character document
    Talents CharacterTalents  // Points (unspent), Spent (talentID -> rank), Prestige

    AllTimeStats
}
```

`CharacterTalents` (`character.go`) travels inside `characters.data`, so talents and prestige need no extra table or migration; legacy rows load with the zero value and every helper treats a nil `Spent` map as empty.

Helper methods for combat:
- `GetAttribute(short)` - Get attribute value by short name (STR, DEX, etc.)
- `GetAttributeModifier(short)` - Get modifier ((value - 10) / 2)
- `GetSTRMod()`, `GetDEXMod()`, `GetCONMod()`, `GetINTMod()`, `GetWISMod()` - Specific modifiers
- `GetWeaponDamage()` - Main hand weapon damage (1 if unarmed)
- `GetArmorDefense()` - Total defense from equipped armor
- `CalculateMaxMana()` - Max mana based on class, level, and INT (casters: `20 + Level*5 + INTMod*4`)
- `CalculateManaRegen()` - Per-round combat mana (`1 + WISMod`, minimum 1). Separate from the `regen.in_combat` tick pools.

Helper methods for progression (`pkg/entities/characters/talents.go`):
- `EnsureTalents()` - Initializes the spent map (nil-safe on legacy characters)
- `TalentRank(id)` - Ranks bought for a talent (0 when unknown)
- `CanLearnTalent(id)` - Whether `classkit.TalentsForClass(c.Class.ID)` offers the talent
- `SpendTalent(id) bool` - Buys one rank; false when unknown, not learnable, or no point
- `PrestigeMultiplier()` - `1 + 0.05 * Prestige` XP multiplier (1.0 with no prestige)
- `ApplyPrestige()` - Irreversible reset: `ResetToLevelOne()` then `Prestige++` and `Points++`
- `ResetToLevelOne()` - Level/XP/attributes/pools back to a fresh class start; talents untouched

### Room Entity

```go
type Room struct {
    *entities.Entity
    traits.LookAt

    Name, Description string
    RoomType, Area    string
    Tags              []string

    Actions    *Actions     // Custom interactions (response, response_room, script)
    Exits      *Exits       // Room connections (supports hidden exits)

    Items      *Items       // Items in room
    Characters *Characters  // Players in room
    NPCs       *NPCs        // NPCs in room

    Coords *struct{X, Y, Z int32}  // Grid position
    Meta   *struct{Mood, Background string}

    CanBind bool  // If true, players can /bind here for respawn
}
```

### NPC Entity

```go
type NPC struct {
    *entities.Entity
    traits.BelongsUser
    traits.CurrentRoom

    Name, Description string
    Race, Class

    CurrentHitPoints, MaxHitPoints int32
    Level int32

    // Template System
    IsTemplate     bool    // True if this is a blueprint
    TemplateID     string  // For instances: source template
    InstanceSuffix string  // Unique suffix (e.g., "abc123")

    // Behavior Configuration
    SpawnRoomID  string         // Respawn location
    RespawnTime  time.Duration  // Time to respawn (0 = no respawn)
    WanderRadius int            // Rooms to wander from spawn
    PatrolPath   []string       // Room IDs for patrol route

    // State Tracking
    IsDead    bool       // Currently dead
    DeathTime time.Time  // When died
    State     string     // FSM: idle, combat, patrol, dead, fleeing

    // Combat state
    InCombat         bool    // Currently in combat instance
    CombatInstanceID string  // Active combat instance

    // Behavior Traits
    EnemyTrait    *EnemyTrait    // Combat behavior
    MerchantTrait *MerchantTrait // Trading behavior (see below)

    // Dialog references
    DialogID          string
    IdleDialogID      string
    IdleDialogTimeout time.Duration
    LastIdleDialog    time.Time
}
```

NPC room payloads are generated by `util.GetRoomNPCs` from live `NPCInstanceManager` data. They include combat/trade/dialog state plus `isQuestGiver`, `hasDialog`, `hasIdleDialog`, and `state` flags for the MUD client entity panel.

#### EnemyTrait

```go
type EnemyTrait struct {
    // Classification
    CreatureType string  // beast, humanoid, undead, elemental, construct, demon, dragon, aberration
    CombatStyle  string  // melee, ranged, magic, swarm, brute, agile
    Difficulty   string  // trivial, easy, normal, hard, boss

    // Combat Stats
    AttackPower  int32
    Defense      int32
    AttackSpeed  float64 // attacks per round; 0 or omitted is one swing and the old beat

    // Behavior
    AggroRadius   int     // Detection range (0 = passive)
    AggroOnSight  bool    // same-room engage after combat.aggro_on_sight grace
    CallForHelp   bool    // Alert nearby enemies
    FleeThreshold float64 // HP % to flee

    // Rewards
    XPReward       int64
    GoldDrop       Range     // {Min, Max}
    LootTableID    string    // Reference to loot table
    GuaranteedLoot []string  // Item template IDs that always drop
    MaxDrops       int32     // Max items from loot table (0 = unlimited)

    // Event Scripts (once per fight; heal, apply an existing buff/debuff, or summon)
    OnAggroScript      string
    OnDeathScript      string
    OnFleeScript       string
    OnLowHealthScript  string  // first drop below LowHealthThreshold while still alive
    LowHealthThreshold float64 // 0 = 0.30; otherwise a fraction in (0, 1)
}
```

Loot entries carry `rarity` (`unique`), `bossOnly`, and `dropChance`. YAML `chance` is an alias stored as `dropChance` (a non-zero chance wins). `bossOnly` rolls only when difficulty is `boss`. A unique entry is skipped when every victory recipient already holds that template in the bag, equipped gear, or a nested container. There is no separate item bank. A `rarity: unique` drop sends a room `combatEvent` with hook `unique`. A template `unique` flag caps ownership and does not send that chip.

#### MerchantTrait

```go
type MerchantTrait struct {
    MerchantType   string         // general, blacksmith, alchemist, etc.
    Inventory      []MerchantItem // Items for sale
    RestockMinutes int32          // Restock interval (0 = never)
    LastRestock    time.Time      // Last restock timestamp
    BuyMultiplier  float64        // Price multiplier when buying (1.0 = normal)
    SellMultiplier float64        // Price multiplier when selling (0.5 = half price)
    AcceptedTypes  []string       // Item types merchant will buy (empty = all)
    RejectedTags   []string       // Tags that prevent buying (soulbound, quest)
}

type MerchantItem struct {
    ItemTemplateID string  // Item template to sell
    BasePrice      int64   // Override price (0 = use item's BasePrice)
    PriceOverride  int64   // Force specific price (ignores multipliers)
    Quantity       int32   // Current stock (-1 = unlimited)
    MaxQuantity    int32   // Max after restock
    RequiredLevel  int32   // Player level requirement
}
```

Merchant inventories live on NPC instances at runtime. Merchant commands lazily restock eligible inventories, reject trading while the player is in combat, and send inventory updates after successful buy/sell operations.

### NPCSpawner Entity

Defines automatic NPC spawning points:

```go
type NPCSpawner struct {
    *entities.Entity

    TemplateID    string         // NPC template to spawn
    RoomID        string         // Where to spawn
    MaxInstances  int            // Max alive at once
    SpawnInterval time.Duration  // Time between extra spawns above InitialCount
    InitialCount  int            // Spawn on world load
    RespawnDelay  time.Duration  // Wait after a death; the ruleset floor can extend it

    RespawnTimeOverride *time.Duration  // Override template respawn
}
```

`combat.respawn_min_seconds` (default 30) is the floor under `RespawnDelay` after a death. `0` disables that floor. A spawner that has not lost an instance still fills `InitialCount` on the next tick.

### LootTable Entity

Defines item drop tables for NPCs:

```go
type LootTable struct {
    *entities.Entity

    Name           string       // Display name
    Description    string       // Editor description
    Entries        []LootEntry  // Potential drops
    GoldMultiplier float64      // Multiplier for gold drops (1.0 = normal)
    DropBonus      float64      // Flat bonus to all drop chances
}

type LootEntry struct {
    ItemTemplateID string   // Item to drop
    DropChance     float64  // 0.0-1.0 probability
    MinQuantity    int32    // Min items (stackable)
    MaxQuantity    int32    // Max items (stackable)
    Guaranteed     bool     // Always drops
    MinPlayerLevel int32    // Level requirement
    RequiredTags   []string // Player must have tags
}
```

### Quest Entity

```go
type Quest struct {
    *entities.Entity

    Name        string       // Quest display name
    Description string       // Quest journal text
    Category    string       // "main", "side", "daily"
    Area        string       // Optional region label for filtering and organization
    Level       int32        // Recommended level
    Repeatable            bool   // Can be completed again
    RepeatReset           string // "daily", "cooldown", or empty (immediate re-accept)
    RepeatCooldownSeconds int64  // Wait after completion for a cooldown reset

    Source      QuestSource  // How player gets quest (NPC, item, auto, script)
    Objectives  []Objective  // Kill, Collect, Deliver, Visit, Talk, Custom objectives
    Rewards     Reward       // XP, Gold, Item grants on completion

    RequiredQuestIDs []string // Prerequisite quests
    RequiredLevel    int32    // Minimum level to accept

    // NPC dialog integration text
    AcceptDialogText, ProgressDialogText, CompleteDialogText string
    OnCompleteScriptID string // Optional Lua script on completion
}
```

When a collect quest is accepted, `QuestsService` initializes objective progress from matching items already in the character inventory. Stackable item quantities count toward the initial progress total.

Delivery objectives are completed through `QuestsService.CompleteDeliveryObjective`, which verifies matching inventory item/template quantities, consumes the required items, persists the character inventory, and then advances the objective.

Quest definitions are validated on create/update for required source fields, objective target fields, duplicate objective IDs, negative rewards, self-prerequisites, and missing referenced NPCs, items, rooms, scripts, or prerequisite quests.

#### QuestProgress (Per-Character State)

```go
type QuestProgress struct {
    *entities.Entity

    CharacterID string           // Owner character
    QuestID     string           // Quest definition
    Status      QuestStatus      // available, active, completed, failed, abandoned
    Objectives  []ObjectiveProgress // Per-objective current/required counts
    AcceptedAt  time.Time
    CompletedAt time.Time
}
```

### Item Entity

Items use a unified template/instance pattern similar to NPCs:

Stackable item quantity changes from consumable use and partial stack drops are persisted to both the embedded character inventory and the backing item instance, keeping future room/world references consistent.

```go
type Item struct {
    *entities.Entity
    traits.LookAt

    // Template System (matching NPC pattern)
    IsTemplate     bool    // True if this is a blueprint
    TemplateID     string  // For instances: source template ID
    InstanceSuffix string  // Unique suffix for instance identification

    Name, Description string
    Type    ItemType     // weapon, armor, etc.
    SubType ItemSubType  // sword, shield, etc.
    Slot    ItemSlot     // equipment slot
    Quality ItemQuality  // rarity tier
    Level   int32

    Properties map[string]interface{}
    Attributes map[string]interface{}

    // Container support
    Closed, Locked bool
    Items          Items
    MaxItems       int32

    // Interaction flags
    NoPickup   bool   // Cannot be picked up

    // Stacking and economy
    Stackable bool   // Can stack in inventory
    Quantity  int32  // Current stack count
    MaxStack  int32  // Max stack size (0 = unlimited)
    BasePrice int64  // Default price in gold
}
```

`equip <item>` validates `class:*` tags, minimum `Item.Level`, and explicit armor weight (`Properties.armorWeight` or an `armor:cloth|leather|plate` tag) against the character class before moving an item from inventory. The play client mirrors these checks in its shared item card and computes class-weighted comparison deltas from `Attributes`. Optional item weight is read from `Properties.weight` when present; current item data need not supply it.

**Template/Instance Lifecycle:**
- Templates (`IsTemplate=true`) are blueprints stored in the database
- Instances (`IsTemplate=false`, `TemplateID` set) are created from templates
- Use `CreateInstanceFromTemplate(templateID)` to spawn new instances
- Instances track their source template via `TemplateID`

### Skill Entity (`pkg/entities/skills/`)

Skills are class-specific abilities used in combat. They are stored in the database and loaded into an in-memory cache at startup. Editable via Creator UI (Skills tab).

```go
type Skill struct {
    *entities.Entity               // DB ID
    Name           string
    Description    string
    ClassIDs       []string        // Multi-class: ["warrior"], ["cleric", "druid"], etc.
    LevelRequired  int32
    ResourceType   ResourceType    // "mana" (casters) or "cooldown" (physical)
    ManaCost       int32           // Mana cost per use (mana-based skills)
    CooldownRounds int             // Rounds before reuse (cooldown-based skills)
    Target         TargetType      // "enemy", "self", "all_enemies"
    Effect         EffectType      // "damage", "heal", "buff", "debuff", "dot", "hot"
    ScalingAttr    string          // "STR", "DEX", "INT", "WIS"
    BasePower      int32
    ScalingFactor  float64
    Duration       int             // Rounds (0 = instant)
    BuffStat       string          // "ATK", "DEF", "STR", etc.
    BuffPercent    float64         // e.g. 0.30 = +30%
    IgnoresDefense bool
    HitCount       int
    // Secondary effect fields (optional)
}
```

**Storage pattern:** DB-backed with in-memory cache. `LoadFromDB()` at startup, `RefreshCache()` after CRUD. Combat engine and commands read from cache (zero service threading needed). 29 default skills seeded on first run when DB is empty.

**Registry functions:**
- `SkillByID(id)` - Look up skill by ID (from cache)
- `SkillsForClass(classID)` - All skills for a class (checks `ClassIDs` slice)
- `AvailableSkills(classID, level)` - Skills available at a given level
- `MaxSkillSlots(classID, level)` - Equippable slot count (casters start with 2, physical with 1)
- `IsCasterClass(classID)` - Whether class uses mana
- `HasClass(classID)` - Skill method to check if a class can use this skill

**Class ID normalization:** The registry maps `"wizard"` → `"mage"` since `ClassWizard.ID` is `"wizard"` but skills are registered under `"mage"`.

**API routes:** `GET /api/skills`, `GET /api/skills/:id`, `POST/PUT/DELETE /api/skills` (creator). `/api/spells` and `/api/spells/:id` are aliases for the same handlers.

### Class Catalog Persistence (`pkg/classkit/persist.go`)

The world-pack class catalog is now writable from the Creator UI. `SaveDef`,
`DeleteDef`, and `FindDef` validate a `Def`, write or remove its `*.yaml` file in
the active class directory (`ActiveDir`, set by the last successful `LoadDir` /
`LoadWorld`), and reload the in-memory registry. Saves are atomic
(temp file + rename) and roll back on reload failure; deletes preserve the
`ErrNotFound` contract. The creator routes are `GET/POST/PUT/DELETE
/api/classes` and `/api/classes/:id` plus `POST /api/classes/:id/duplicate`.
`GET /api/classes` stays public but switches to the full editor DTO when an
optional creator/admin token is present. The class editor in the play client
uses these routes when authenticated and only falls back to localStorage when
the network is unreachable.

**Skill slot progression:**

| Class | L1 | L10 | L15 | L20 | L30 |
|-------|:--:|:---:|:---:|:---:|:---:|
| Mage/Cleric/Druid | 2 | 2 | 3 | 3 | 4 |
| Warrior/Rogue/Ranger | 1 | 2 | 2 | 3 | 4 |

### Clan Entity (`pkg/entities/clan/`)

Phase 4 social system. A clan is a persistent player organization with a name/tag, a leader, a shared bank, and members with ranks.

```go
type Rank int // RankMember=1, RankOfficer=2, RankLeader=3

type Clan struct {
    *entities.Entity
    Name, Tag      string
    LeaderCharID   string
    FoundedAt      time.Time
    Level          int
    XP             int64
    Bank           int64     // Clan gold
    MOTD, Description string
    Active         bool      // false = dissolved (soft delete)
    MaxMembers     int
    CreatedAt, UpdatedAt time.Time
}

type ClanMember struct {   // alias: clan.Member
    *entities.Entity
    CharacterID, ClanID string
    Rank         Rank
    JoinedAt     time.Time
    Contribution int64
    LastOnline   time.Time
}

type ClanInvite struct {   // alias: clan.Invite
    *entities.Entity
    ClanID, CharacterID, InviterID string
    Status     string    // pending | accepted | declined | expired
    CreatedAt, ExpiresAt time.Time
}

type ClanBankLedger struct { // alias: clan.BankLedgerEntry, append-only
    *entities.Entity
    ClanID, CharacterID string
    Type    string       // deposit | withdraw
    Amount  int64
    Balance int64        // Balance after the transaction
    Note    string
    Timestamp time.Time
}
```

A character may belong to only one clan at a time; membership uniqueness is enforced in `AddMember`. `ClansService` (`pkg/service/clans.go`) adds the authorization rules on top: invites (leader/officer, no duplicates), accept/decline by invite UUID, kick (leader/officer, leader immune), rank changes (leader only, `ParseClanRank` accepts `member`/`officer`/`leader`), bank deposit (any member) and withdraw (leader only), and disband (leader only). Exposed at `/api/clans`.

### DM Entity (`pkg/entities/dm/`)

Phase 4 persistent private messaging. Channels are created on demand the first time two characters exchange a message.

```go
type DMChannel struct {  // alias: dm.Channel
    *entities.Entity
    ParticipantA, ParticipantB string // canonical order; characterIDs
    CreatedAt     time.Time
    LastMessageAt time.Time
    LastMessageID string
    UnreadA, UnreadB int          // Denormalized per-participant unread counters
}

type DMMessage struct {  // alias: dm.Message
    *entities.Entity
    ChannelID  string
    FromCharID string
    Body       string
    SentAt     time.Time
    ReadAt     *time.Time // nil = unread
}
```

**Read state:** a single `readAt` per message plus denormalized `UnreadA`/`UnreadB` counters on the channel. `MarkRead(channelID, reader)` zeroes the reader's counter and stamps every message not sent by the reader as read.

`DMService` (`pkg/service/dms.go`) wraps the repository with participant checks: `SendMessage` rejects a non-participant sender (anti-spoof), `GetMessages`/`MarkRead` refuse non-participants, and `FindOrCreateChannel` rejects self-DMs and delegates canonicalization to the repository. Exposed at `/api/dm/channels`.

### Dungeon Entity (`pkg/entities/dungeons/`)

`Dungeon` embeds `entities.Entity` plus `Name`, `Description`, `GridWidth`, `GridHeight`, `Author`, `Tags`, `Definition json.RawMessage`, `CreatedAt` and `UpdatedAt`. `Definition` is the opaque blob exported by the client builder (`dungeonLibrary.js`): `DungeonsService` validates only that it is valid JSON, never interprets it. List reads return a summary DTO without `definition`; `GET /api/dungeons/:id` returns the full document. Nothing in the game instantiates a `Dungeon` yet.

### NPCTemplate Entity (`pkg/entities/npcs/templates.go`, FASE 5.3)

`NPCTemplate` is a persisted mob blueprint mirroring the client `npcFactory.js` payload: identity (`Name`, `Description`, `Role`, `CreatureType`, `CombatStyle`, `Difficulty`), combat numbers (`Level`, `MaxHp`, `DamageMin/Max`), aggression (`Aggressive`, `AggroRadius`), rewards (`XpReward`, `GoldMin/Max`, `LootTableID`, raw `Loot`), spawn hints (`SpawnRoomId`, `SpawnCount`, `RespawnSeconds`), raw `Behavior`/`Scaling` JSON written by `npcBehavior.js`, and `IsTemplate`. Stored in `npc_templates` through `Factory.NPCTemplates()`; **no route or service consumes it yet** (`GET /api/npcs/templates` still reads the `npcs` collection).

### Combat Engine — Skill Processing (`pkg/mudserver/game/combat/engine_skills.go`)

The skill processing pipeline in the combat engine:

1. **`ProcessSkill(instance, casterID, skillID, targetID)`** — Validates skill ownership, checks mana/cooldown resources, deducts costs, then dispatches by effect type
2. **`ProcessStatusEffects(instance, combatant)`** — Called at turn start: ticks DoTs/HoTs, checks stun (skip turn), decrements durations, removes expired effects
3. **`ProcessManaRegen(instance, combatant)`** — Adds `ManaRegen` to `CurrentMana` at round start
4. **`TickCooldowns(instance, combatant)`** — Decrements all cooldowns at round start

**Skill damage formula:** `basePower + int32(float64(attrMod) * scalingFactor)`

**Buff/debuff application:** Status effects modify combat stats (attack power, defense, dodge chance) by percentage for a set duration.

**Class-kit dispatch:** kit skills (`skill.Kit != ""`) bypass the generic effect switch and resolve in `processClassKit` (`pkg/mudserver/game/combat/classkit.go`) by `skill.Kit`. `pkg/entities/skills/classkit.go` still maps each kit id onto a generic effect for clients and sims via `kitEffect`: `mend` → `EffectHeal`, `renew` → `EffectHot`, everything else → `EffectDamage`. The healing kits resolve against a friendly target (`livingAllyTarget`): the explicit target when alive, in the fight, and on the caster's side (same `CombatantType`, or self), otherwise the caster. `mend` restores `max HP × swing_mult` instantly (default 0.25); `renew` applies a `hot` `StatusEffect` ticking `max HP × swing_mult` (default 0.10) per round for `duration` rounds (3 when unset). Both pay their cooldown before landing a heal.

**Class catalog, mana and components:** kit skills are authored in the world-pack class catalog (`pkg/classkit`, `data/classes/*.yaml` — `warrior`, `rogue`, `mage`, `cleric`, `druid`, `sorcerer`; each class also declares a `school` resolved by `classkit.SchoolName`, and an unknown school or effect fails the whole directory load). A skill may declare `mana_cost` and a `component` reagent hook (`arcane_dust` arcane, `fire_salt` fire, `divine_wax` divine, `nature_herb` nature). `processClassKit` validates mana **before** committing anything (`Not enough mana for <skill> (need N, have M).`), then `pay()` deducts the mana and starts the cooldown in the same step. `component` flows `classkit.SkillSpec` → `skills.Skill.Component` → the `/api/classes` payload (`ClientSkill.component`) with no reagent inventory attached yet. Cleric/Druid `mend`/`renew` are authored `target: ally`; `mage_mend` and `sorcerer_mend` are authored `target: self`, and the cast/shortcut commands only resolve targets for enemy skills (plus `guard`), so those casts land on the caster.

## Progression & Derived Stats (Phases 2.3 & 2.4)

XP, derived attributes, the talent catalog, and prestige are split across three modules: `pkg/mudserver/game/leveling` owns the curve and the milestone points, `pkg/talents` owns the catalog, and `pkg/entities/characters` owns storage plus the projection.

### Derived Stats (`pkg/entities/characters/derive.go`)

`DeriveStats(c, classDef) DerivedStats` is the single read-only projection of combat-facing numbers. `classDef` is resolved from the catalog via `classkit.Lookup(c.Class.ID)` when nil.

```
Stamina    = STA + 2*Level              (min 1)
Initiative = DEXMod + Level/2           (min 0)
Resist     = WISMod + CONMod + Level/2  (min 0)
CritChance = 5% + DEXMod% + kit bonus   (clamped to [0, 95])
```

The kit bonus adds `+2%` for a multi-swing balance row (`Balance.Swings > 1`) and `+2%` when the kit's primary attack attribute is `DEX`. `MaxHP`/`MaxMana` are returned verbatim from the stored character fields, which are the same fields combat snapshots, so the derived view can never reshape a live fight; the engine's natural-20 crit roll and `1d20+DEXMod` initiative roll are likewise untouched.

`DeriveStats` ends with `c.ApplyTalentBonuses(&ds)` (talent ranks folded in per effect, so map order is irrelevant) and re-clamps `CritChance`. Talent effects land in dedicated projection fields (`damageBonus`, `spellDamageBonus`, `damageReduction`, `maxHpBonusPercent`, `blockChance`, `staminaRegen`, `spellCooldownBonus`, `healBonus`, `evasion`, `potionBonus`) rather than into `MaxHP`, because `MaxHP` must keep mirroring the pool combat reads.

Consumers: `NewCharacterUpdateMessage` (`messages/responses.go:459`) emits the derived fields + `talents` on the `characterUpdate` payload; `CreateCombatantFromCharacter` (`combat/engine.go:114`) copies `derived.MaxHP`/`derived.MaxMana` into the `CombatantRef`.

### Talent Catalog (`pkg/talents/`)

Pure-data progression package — no imports beyond `strings`. Each `talents.Def` carries `ID`, `Name`, `Category` (`offensive`/`defensive`/`utility`), one `Effect` (12 kinds: damage, crit, spell_damage, damage_reduction, max_hp, block, initiative, stamina_regen, spell_cooldown, heal, evasion, potion), a per-rank `Amount`, the allowed `Schools`, and a `Description`. The shipped catalog holds 12 talents.

| API | Purpose |
|-----|---------|
| `All()` | Copy of the catalog in stable order (UI/tests) |
| `Lookup(id)` | Case-insensitive; also accepts the id without the `talent_` prefix |
| `ForSchool(school)` | Talents a school may learn, in catalog order |
| `Available(school, id)` | Membership check used by gating |
| `IDs(defs)` | Id list in order |

Characters store only which talents they bought (`Talents.Spent`); effects are always projected through `DeriveStats`. Class gating lives in `classkit.TalentsForClass(id)` (`pkg/classkit/registry.go`): an explicit `talents:` list on the class YAML wins, otherwise every talent of the class's school is offered — so a content pack needs no edit for the default case.

### Progression flow (XP → talent points → prestige)

```
XP grants (combat game_combat.go / quests service/quests.go / exploration leveling/xp_sources.go)
        │  × PrestigeMultiplier() on combat and quest XP
        ▼
leveling.MaybeLevelUp / CheckLevelUp / ApplyLevelUp        (leveling/leveling.go)
        │  XPTable/ruleset override (leveling/xp_curve.go, ruleset.XPRequired)
        │  TalentMilestonesBetween(old, new) → Talents.Points += n   (levels 5,10,…,50)
        ▼
talents spend <id>  →  Character.SpendTalent → Talents.Spent[id]++   (commands/talents.go)
        ▼
DeriveStats → ApplyTalentBonuses → DerivedStats (client + combat)
        ▼
prestige confirm (level 50) → ApplyPrestige → ResetToLevelOne + Prestige++ + Points++
```

Milestone counting is `(oldLevel, newLevel]`, so a multi-level jump credits every crossed milestone once and a save/load cycle never re-grants points. `prestige`/`prestige confirm` refuse to run during combat and persist through `CharactersService().Update`.

## Dialog System

### Dialog Structure

```go
type Dialog struct {
    ID                     string
    Text                   string      // Primary speech
    AlternateTexts         []string    // Random variations
    OrderedTexts           *bool       // Sequential on repeat

    Options                []*Dialog   // Player choices
    Answer                 *Dialog     // Auto-response

    RequiresVisitedDialogs []string    // Prerequisites
    ShowOnlyOnce           *bool       // One-time option
    IsDialogExit           *bool       // End conversation
}
```

### Dialog State

```go
type DialogState struct {
    CurrentDialogID  string              // Active node
    DialogVisited    map[string]int      // Visit counts
    Context          map[string]string   // Static variables
    DynamicContext   map[string]func() string  // Runtime variables
}
```

### Dialog Flow

```
┌─────────────────────────────────────────┐
│              Start: "main"               │
└─────────────────┬───────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│         Display NPC Text                 │
│   (with variable substitution)           │
└─────────────────┬───────────────────────┘
                  │
        ┌─────────┴─────────┐
        ▼                   ▼
┌───────────────┐   ┌───────────────┐
│  Has Options  │   │  Has Answer   │
│    (choice)   │   │   (auto)      │
└───────┬───────┘   └───────┬───────┘
        │                   │
        ▼                   ▼
┌───────────────┐   ┌───────────────┐
│ Player Selects│   │ Jump to Answer│
│    Option     │   │     Node      │
└───────┬───────┘   └───────────────┘
        │
        ▼
┌───────────────────────────────────────────┐
│ Navigate to selected dialog node          │
│ (check IsDialogExit → return to "main")   │
└───────────────────────────────────────────┘
```

## Message System

### Message Types

```go
const (
    MessageTypeDefault          // Generic message
    MessageTypeEnterRoom        // Room entry
    MessageTypeCreateCharacter  // Character creation prompt
    MessageTypeSelectCharacter  // Character selection
    MessageTypeCharacterSelected // Selection confirmed
    MessageTypePing             // Keep-alive
    MessageTypeGuideHint        // One next-step hint, or empty text to clear it
    MessageTypeQuestAccepted    // Quest accepted
    MessageTypeQuestProgress    // Quest objective updated
    MessageTypeQuestReady       // Quest objectives complete and ready for turn-in
    MessageTypeQuestCompleted   // Quest completed
    MessageTypeQuestLog         // Full quest log
)
```

### Message Audience

```go
const (
    MessageAudienceOrigin           // To sender only
    MessageAudienceUser             // To specific user
    MessageAudienceRoom             // To all in room
    MessageAudienceRoomWithoutOrigin // To room except sender
    MessageAudienceGlobal           // To all connected
    MessageAudienceSystem           // System announcement
)
```

### Message Structure

```go
// Incoming (Client → Server)
type IncomingMessage struct {
    Message string
}

// Internal Processing
type Message struct {
    FromUser  *entities.User
    Character *characters.Character
    Data      string
}

// Outgoing (Server → Client)
type MessageResponse struct {
    Audience   AudienceType
    AudienceID string
    OriginID   string
    Type       MessageType
    Username   string
    Message    string
    Style      string // omitempty; "combatEvent" on enemy-hook room lines
    Hook       string // omitempty; onAggro | onLowHealth | onDeath | onFlee
    Source     string // omitempty; display name of the NPC whose hook ran
}
```

Enemy-hook `msgToRoom` / `msgToRoomExcept` lines keep `type` `message` and `username` `SYSTEM`. The three fields are omitted on every other message. The stamp lives on that script run's Lua state, not a process global. `combatStatus` may include `combatants` (the same roster view as `combatAction`) when a hook flush summoned adds, so the client can refresh the fight before the next action.

`combatEnd` keeps `outcome` and `message`. Optional `rewards`, `loot`, `levelUp`, and `defeat` objects ride on the same message. `combatAction` may include `ability` when a named blow lands. Clients that only read `message` still work. Outcome `fled` is a successful flee or slip, with no `defeat` payload. In a mixed party, only the dead receive outcome `defeat`.

### Boss phase data flow

`BossMechanicsConfig.Phases` validates descending HP bands with an opening threshold of 1.0. `phase_tiers` defaults to `boss`; an absent `phases` list disables phases. `CombatantRef` holds encounter-local `BossPhase`, `BossPhaseLabel`, and `BossPhaseCount`, separately from the combat instance's turn-pacing `Phase`. Every engine `UpdateCombatant` checks live NPC HP and records monotonic `phase-enter` log entries, including skill and DoT changes. The controller publishes those entries as structured combat actions at resolution boundaries; start/join/action `CombatantView` snapshots include `bossPhase`, `bossPhaseLabel`, and `bossPhaseCount`. The play store preserves the snapshots and retains a short-lived phase notice independently of hit FX. BattleStage renders the notice and persistent labels without changing the layout focus or party contracts. Configuration optionally replaces telegraph labels and enrage multipliers or scales outgoing damage; existing global enrage timing and wind-up cancellation still apply.

## Frontend Architecture

The play client observes `combatPhase` in `Game.svelte` and calls `LayoutStore.syncCombatFocus`: active starts the existing BattleStage cover, ending retains it, and idle restores prior focus. The cover uses `focusId`/`focusSnapshot` for normal-layout persistence and a transient `combatFocusReturn` for prior manual focus; it does not change grid geometry or remount terminals. The stage keyboard-focus action guards text entry and restores the prior DOM control on removal.

`SettingsStore` persists `interface.reducedMotion` (`system`/`on`/`off`) and `interface.combatAutoFocus` (default true) alongside the existing inventory mode. `Game.svelte` passes the auto-focus or manual-stage choice into `LayoutStore.syncCombatFocus` and gates Stage rendering; switching the preference during combat restores or opens the transient cover immediately. The shared `prefersReducedMotion()` helper resolves the stored override before the OS media query, and BattleStage CSS uses that resolved state. Map ambience uses the same helper.

### Play Client — Social Panels (clans, DMs, chat tabs)

The play client (`public/mud-client`) adds two REST-backed social overlays next
to the existing Party/Friends panels:

- **`ClanPanel.svelte`** — toggled by the `C` key or the header shield button in
  `CharacterSwitcher.svelte` (both driven by `clanPanelOpen` in `uiChrome.js`;
  header buttons are hidden for guests). It calls `public/mud-client/src/api/clans.js`
  against `/api/clans*` with the session JWT: `GET /api/clans/mine` on open plus a
  manual Refresh (no polling, no push), create, invite, accept/decline, kick,
  rank promote/demote, deposit/withdraw, disband.
- **`DMsPanel.svelte`** — toggled by the `M` key or the header mail button
  (`dmsPanelOpen`). While open it polls `GET /api/dm/channels` every 5 seconds
  (`POLL_MS`), fetches the active conversation with `limit: 100`, renders an
  unread badge from `unreadA`/`unreadB`, posts new messages, and marks the read
  channel via `POST /api/dm/channels/:channelId/read` (`public/mud-client/src/api/dms.js`).
  No WebSocket message exists for DMs; new traffic only appears on the next poll.
- **Shortcut arbitration** — `Game.svelte` `onShortcutKey` ignores `C`/`M` while a
  text field has focus or another panel is on top, `Escape` closes the top panel
  (`topOpenPanel`/`shortcutFlags`), and the same ids are listed in
  `keyboardShortcuts.js` for the `?` cheat sheet.
- **Chat tabs** — `chatTabs.js` splits terminal lines into
  `say|tell|party|channel|system` tabs (structured `channel` field first, then
  `[Channel] …` prose) and tracks per-tab unread counts; `ChatPanel.svelte`
  renders them.

Player-facing guides: `docs/user/clans.md`, `docs/user/dms.md`,
`docs/user/channels.md`.

### Play Client — Authoring Panels (Dungeon Builder, NPC factory)

Two authoring overlays live in the play client and share the same shortcut arbitration as the social panels:

- **Dungeon Builder (key `D`, FASE 5.1)** — `game/ui/DungeonBuilder.svelte` over pure modules `game/dungeonBuilder.js` (grid, rooms, corridors, encounters, loot tables, BFS entry→exit validation, overlap detection, test mode) and `game/dungeonLibrary.js` (export/import of the opaque `definition`). `src/api/dungeons.js` reads/writes `/api/dungeons` with the session JWT when the user is a creator; `localStorage` stays the source of truth and any 401/403/404 or network failure degrades to local storage with a visible warning. The panel id is `dungeonBuilder` in `keyboardShortcuts.js` and is listed in `ShortcutSheet.svelte`.
- **NPC factory (key `N`, Batches 16/19)** — `game/ui/NpcFactory.svelte` + `game/npcFactory.js` (template shape, range validation, payload mapping) with `localStorage` (`mud-client.npc-factory.v1`) and best-effort `GET /api/npcs/templates` / `POST`/`PUT /api/npcs` when a creator token exists (`src/api/npcs.js`).
- **Level scaling helper** — `game/npcBehavior.js` (`scaleTemplate(template, targetLevel)`, deterministic rates; serializable state machine + `toLuaBehavior()` as preview/export only). Consumed by the factory UI; the server never runs it. Tests: `npcBehavior_test.mjs`, `npcFactory_test.mjs`, `dungeonBuilder_test.mjs`, `dungeonLibrary_test.mjs`, `api/dungeons_test.mjs`.

REST contract for the dungeon library: [`docs/API.md`](docs/API.md) §5. Guides: `docs/user/admin-guide.md`.
### Responsive presentation ownership

`public/mud-client/public/global.css` defines the page z-index ladder with `--layer-*` properties. Shell chrome is below sheets. BattleStage is below NPC dialogs, shop, and Need/Greed. Component decoration keeps its local stacking context.

`mobileStore.js` owns the viewport query for the mobile Room/Console shell, including short landscape. `App.svelte` uses the matching media query to hide its desktop home link. The mobile room scrolls independently from the console tab. `EntityPanel` accepts `--entity-card-width`; the short landscape room sets it to keep NPC cards inside the hero image. `phoneSheets.js` maps the bare commands `character`, `equipment`, `skills`, and `quests` to phone sheets. `MobileLayout` measures the header and the visible hotbar into `--sheet-top` and `--sheet-bottom`, and the sheet band stays between them. Typing those words in the console still sends the command.

The existing `MUDXPlusStore.questNotifications` and `overlayStore` arrays own queued events. Producers enqueue and explicitly remove messages. They do not expire unseen messages or truncate the pending queue. `notificationPresentation.js` provides the combat/outcome/roll presentation gate and `createVisibleLifetime`, a Svelte action. Each renderer owns one action instance; each rendered card owns one timeout. Removing a card pauses its remaining reading time in a WeakMap keyed by the notification object. Rendering it again resumes that clock. Expiry and explicit dismissal remove the queued event. Mobile room tab visibility also gates room messages. The client keeps combat validation feedback in the BattleStage action banner.

The outcome card contains a scrollable reward body and a separate Continue button. Short landscape reserves separate horizontal lanes for the outcome and Need/Greed.

### MUD Client — Onboarding Flow

The MUD client (`/play`) uses a phase-based routing system in `App.svelte` to guide new players through onboarding before showing the game UI. After character select, the server pushes `guideHint`. `GuideHint.svelte` renders that one line above the hotbar and sends the hint command, or `guide dismiss`. `pkg/guide` chooses the step from quest progress and the room graph, with no world-pack ids in the engine. `guide.enabled` in the ruleset defaults to on. `false` builds no hint and sends no `guideHint`. A game-mode file may set the same key. Hint text names the action in ordinary words (`Attack the <name>.`) so telnet, SSH, and Door can show the same line.

```
App.svelte (phase-based routing)
├── LoadingScreen           (phase: "loading" — Auth0 initializing)
├── WelcomeScreen           (phase: "welcome" — unauthenticated)
│   ├── Continue with X / Google (Auth0 connection) or Email and password
│   └── Play as Guest (POST /api/guest → skip onboarding, go to "ready")
├── NicknameSetup           (phase: "nickname" — new user, needs display name)
├── CharacterCreationWizard (phase: "character" — no characters yet)
│   ├── Step 1: Choose Template (from GET /api/templates/characters)
│   │     Class cards, blurbs, and race lists come from GET /api/classes
│   │     (pkg/classkit, loaded from data/classes). No pack means the sample trio.
│   ├── Step 2: Name & Describe Character
│   └── Step 3: Confirm & Create (POST /api/newcharacter)
└── Game + UserMenu + SettingsModal (phase: "ready" — normal gameplay)
```

Phase detection:
- After Auth0 login, fetches `GET /api/user` to check `isNewUser` / `nickname`
- Then fetches `GET /api/characters` to check if user has characters
- Transitions through phases automatically until reaching "ready"

Onboarding components are in `src/onboarding/`:
- `LoadingScreen.svelte` — Minimal dark loading screen
- `WelcomeScreen.svelte` — Cinematic landing. X and Google log in through a named Auth0 connection. Email opens universal login. Guest play stays on the card
- `NicknameSetup.svelte` — Glass-morphism card for nickname entry
- `CharacterCreationWizard.svelte` — Three-step full-page wizard

### Admin/Creator App — Component Hierarchy

```
App.svelte (role-aware navigation: Creator/Admin links gated by user role)
├── AppContent.svelte (router)
│   ├── Welcome.svelte (landing)
│   ├── Game.svelte (gameplay)
│   │   ├── MUDXPlus.svelte (UI overlay)
│   │   │   └── Inventory.svelte
│   │   └── xterm (terminal)
│   ├── Characters.svelte
│   │   ├── CharacterCard.svelte
│   │   └── CharacterCreator.svelte
│   ├── Creator.svelte (editor, creator/admin role)
│   │   ├── CRUDEditor.svelte (shared master-detail layout)
│   │   │   ├── ValidationPanel.svelte (inline validation issues)
│   │   │   ├── DataTable.svelte (filterable, sortable data table)
│   │   │   └── DataTableFilterBar.svelte (per-column filter inputs)
│   │   ├── WorldHealth.svelte (world diagnostics)
│   │   ├── RoomsEditor.svelte
│   │   ├── ItemsEditor.svelte
│   │   ├── ItemTemplatesEditor.svelte
│   │   ├── NPCsEditor.svelte
│   │   ├── DialogsEditor.svelte
│   │   ├── QuestsEditor.svelte
│   │   ├── ScriptsEditor.svelte
│   │   ├── CharacterTemplatesEditor.svelte
│   │   ├── GridWorldEditor.svelte
│   │   └── tableColumns.js (column definitions per entity type)
│   └── admin/
│       └── UserManagement.svelte (admin only, user table + ban modal)
├── UserForm.svelte
└── UserMenu.svelte
```

#### Creator UI Data Table Pattern

The Creator World Health route calls `GET /api/world/validation` to surface broken cross-system references, including rooms, NPCs, items, loot tables, quests, dialogs, scripts, spawners, and character template starting item references.

All entity editors (Rooms, Items, Item Templates, NPCs, Dialogs, Quests, Scripts, Character Templates) share the `CRUDEditor` component which provides a side-by-side master-detail layout:

- **Table panel (left)**: Full-width filterable, sortable data table showing entity-specific columns. When no entity is selected/detail is closed, the table expands to full width.
- **Detail panel (right)**: Edit form with all entity fields, shown when a table row is clicked. Includes close button to return to full-width table, and prev/next navigation.
- **Column definitions**: Defined in `tableColumns.js` per entity type, supporting text, number, select, boolean, and computed column types.
- **Client-side filtering**: Instant filtering on the full preloaded dataset with per-column filter inputs (text search, dropdowns for enums). No API calls per filter change.
- **Client-side sorting**: Click column headers to sort (ascending → descending → none).
- **Inline validation**: Editors with an `entityType` call `/api/validate/:entityType` for draft validation, show broken-reference warnings/errors through `ValidationPanel`, and disable save while error-severity issues are present.
- **World health diagnostics**: `WorldHealth.svelte` calls `/api/diagnostics/world` to list cross-entity issues and surface row indicators in Creator tables.

`QuestsEditor.svelte` adds quest-specific validation and player flow preview inside the shared detail panel. The validation mirrors server-side quest definition checks where possible and resolves entity references from the preloaded Creator datasets. The preview summarizes source, objective progression, ready turn-in target, and rewards before the creator saves.

### State Management

| Store | Purpose |
|-------|---------|
| `stores.js` | Global user state, menu state |
| `auth.js` | Auth0 authentication state |
| `MUDXPlusStore.js` | Game UI state (exits, actions, background, mana, equipped skills, connection status, room players) |
| `CRUDEditorStore.js` | Editor state (elements, selection, filters, table filter/sort state, detail panel open/close) |
| `Client.js` | WebSocket client state (room, character, reconnect lifecycle, room presence messages) |
| `overlayStore.js` | Transient text overlay messages (auto-dismiss with timers) |

### WebSocket Client

**File:** `public/app/src/game/Client.js`

```javascript
class GameClient {
    ws          // WebSocket connection
    activeRoom  // Current room data
    currentCharacter

    sendMessage(msg)      // Send command to server
    setWSClient(ws)       // Update WebSocket
    onMessage(type, handler)  // Register handler
}
```

`Game.svelte` owns reconnect scheduling because it has access to auth state and
the active token. A process-wide `websocketGate.js` allows only one CONNECTING,
OPEN, or CLOSING socket (survives Game remounts). Close code 4001 means another
connection took the session — the client shows that and does not auto-reconnect.
`Client.js` only sends over an open socket and reports
reconnecting state to the UI store when a player tries to send while offline.
It also handles `roomPresence` messages by updating `MUDXPlusStore.players`
without re-rendering the full room.
`CharacterSwitcher.svelte` shows the active character and connection state.
Switch character opens `CharacterPicker.svelte`, which loads `/api/my-characters`
and sends `sc <name>`. The same entry is on the phone account menu. Logout
clears the Auth0 session and the tab guest token before returning to `/play`.

### Terminal Integration

Uses xterm.js with custom input handling:

- **LocalEchoController** - Input line editing, history
- **HistoryController** - Command history with up/down navigation
- **FitAddon** - Responsive terminal sizing

## Concurrency Model

### Goroutines

```
main()
  │
  ├── HTTP Server (Gin)
  │
  └── MUD Server
        │
        ├── receiveMessages()      // Game → Clients
        ├── Game.Run()             // Command processing
        ├── handleBroadcast()      // Global messages
        ├── handleTimeouts()       // Ping/keep-alive
        │
        ├── Per-client read loops  // WebSocket read loops
        │
        └── Per-connection writeLoop   // sole writer; exits when the transport closes
```

### Channels

| Channel | Buffer | Purpose |
|---------|--------|---------|
| `OnMessageReceived` | 20 | Player input → Game |
| `SendMessage` | 20 | Game output → MUD Server |
| `OnUserJoined` | 20 | Login events |
| `OnUserQuit` | 20 | Logout events |
| `Broadcast` | unbuffered | Global messages |

### Thread Safety

- `Connection.mu sync.Mutex` - Protects WebSocket writes
- SQLite uses WAL mode and busy timeout for concurrent access
- Channels prevent race conditions on shared state
- **Presence lock per user (Batch 23, closed):** the same `*entities.User` is shared by the WebSocket handler goroutine (`attachSession`/`handleConnectionClosed`), the game loop (`ConnectUserSession`/`DisconnectUserSession`/`SetUserSessionCharacter`) and `game_handlers` (`handleUserJoined`). Those goroutines used to write `IsOnline`/`LastSeen`/`LastCharacter` without synchronisation while `UsersService().Update` serialised the live struct to JSON — a data race plus a mixed snapshot persisted to SQLite (reported as open P2 in QA §B22.4). Fixed in Batch 23:

  - **`pkg/mudserver/game/presence.go`** — `presenceRegistry` hands out **one mutex per user id** with reference counting (the entry is dropped with the last holder, so the registry stays bounded). Unrelated players never wait for each other: player A's lock is independent of player B's, and the lock is a **leaf lock** (taken only here, only held across the repository write) so it cannot deadlock with the session registry or the game loop.
  - **`ApplyPresence(user, mutate)`** runs `mutate` under the user's lock; when it returns true, a **detached copy** (`entities.User.Snapshot()`, also copying the embedded `Entity`) is taken under the same lock and handed to `UsersService().Update`. The lock is held across the write, so the persisted row always matches one consistent state transition (an offline write can never be undone by a stale online write).
  - **`PresenceOf(user)`** returns a `def.PresenceState{IsOnline, LastSeen, LastCharacter}` snapshot under the same lock; it is the only sanctioned way for another goroutine to read those fields (`GameCtrl.PresenceOf`, used by `mudserver.go`, `game_handlers.go`, `game/util/game_utils.go`, `commands/select_character.go`, `presentation/doorview/view.go`).
  - **Invariant:** nothing mutates the presence fields of a shared `*entities.User` outside `ApplyPresence`; every cross-goroutine read goes through `PresenceOf`. Regression test: `pkg/mudserver/presence_race_test.go` (`TestPresenceMutationIsRaceFree` — 50 rounds × 5 concurrent writers), which fails under `-race` without the lock.
  - **Debounce preserved:** the lock does not change the B19.6-1 semantics — presence is persisted only on the offline→online / online→offline transition, not on every input line.
  - **Future recommendation:** the clean end state is a **single owner** of the `*entities.User` in the game loop (WS handlers only enqueue events via `OnUserJoined`/`OnUserQuit` + `forwardJoins`), which would retire the per-user lock; until then `ApplyPresence`/`PresenceOf` is the required access path.

### Batch 19 QA blocker — self-deadlock of the game loop at ≥200 connections (**closed: fixed in Batch 20, verified by QA in B21**)

What happened: the layout above has one consumer per channel: `Game.Run()` is the **only** reader of `OnMessageReceived` (buffer 20) and `OnUserJoined` (buffer 20), yet `handleUserJoined` used to write back into `OnMessageReceived` from that same goroutine. Once the buffer filled, the loop blocked on a send it alone could drain, and the cascading `OnUserJoined` sends in `attachSession` froze every WebSocket read loop. Measured (§B19): 100 connections healthy (heartbeat included); **≥200 wedged permanently until restart** (probabilistic, observed after 74–140 joins; CPU ~0%).

Fix landed in Batch 20 (Backend), verified against the tree:

- `game/game_handlers.go` dispatches the synthesized `lc` and `selectcharacter` commands **inline** through `Game.processIncoming` on the game-loop goroutine (the constraint is documented on `processIncoming` in `game/game.go`); it no longer sends to `onMessageReceived`.
- `mudserver/joinqueue.go` (new) + `mudserver/mudserver.go`: joins are pushed onto an unbounded, non-blocking FIFO (`enqueueUserJoined` returns immediately from `attachSession`); a single `forwardJoins` goroutine is the only sender that may block on `Game.OnUserJoined`, with one saturation warning at 64 joins.
- `mudserver/transport.go`: per-connection outbound queue (128) drained by a dedicated `writeLoop`; `Send` enqueues and returns, each frame written under a `SetWriteDeadline` of 10 s, so a slow consumer is disconnected instead of stalling broadcasts/pings. Batch 22: the transport is now closed idempotently on every disconnect path (`defer connection.closeTransport(0, "")` in `HandleConnections` + `connection.closeTransport(0, "")` in `handleConnectionClosed`), so each `writeLoop` exits instead of leaking ~1 goroutine per session (QA §B21.5; test `mudserver/transport_leak_test.go`).
- `game/sessions.go` + `mudserver.go`: `LastSeen`/`IsOnline` persisted only on connect/disconnect (no `UsersService().Update` per message).
- `cmd/tales/pprof.go`: optional `net/http/pprof` endpoint (`ENABLE_PPROF`, `PPROF_ADDR`, default `127.0.0.1:6060`).

Evidence: Batch 20 load test with 300 workers → 300/300 OK, 0 `msgErrors` (previously wedged from ~200). **Status: closed — QA-verified in B21** (2026-10-19): 200/300/500 connections × 2 runs × 80 s, `recv == sent`, **0 `msgErrors`** in 6/6 runs, post-carga probe healthy (§B21.2/§B21.8). Scale figures are no longer provisional: 500 concurrent logins are served without wedge.

Build note (Batch 22): `.gitignore` now uses `/tales` instead of `tales`, so `cmd/tales/pprof.go` and `cmd/tales/pprof_test.go` are no longer silently excluded from `git add`; and the Makefile compiles **by package** (`go build -buildvcs=false -o bin/tales ./cmd/tales`, `go run ./cmd/tales`) — building `cmd/tales/main.go` alone fails with `undefined: startPprof`.

## Design Patterns

### Patterns Used

| Pattern | Usage |
|---------|-------|
| **Facade** | Service layer (`service.Facade`) |
| **Repository** | Data access (`GenericRepo`) |
| **Factory** | Item templates, character creation |
| **Observer** | Channel-based event system |
| **Command** | Game command handlers |
| **Singleton** | Game state cache |
| **Composition** | Entity traits |
| **State Machine** | Dialog execution |

### Dependency Injection

```
main()
  └── NewApp()
        ├── db.Connect()
        ├── NewFacade(repos...)
        │     ├── NewCharactersRepo(db)
        │     ├── NewRoomsRepo(db)
        │     ├── NewUsersRepo(db)
        │     └── ...
        └── mud.NewServer(facade)
              └── game.NewGame(facade)
```

## Security

### Authentication Flow

```
Client → Local Login | Auth0 Login | Guest Session → Token (JWT or HMAC)
                              │
                              ▼
          HTTP/WS Request with Token → AuthMiddleware
                              │
                              ▼
        Try guest HMAC (fast, local) ──valid──► Load guest user (check expiry)
                              │ invalid
                              ▼
        authsession.IsIssuer?        ──yes────► Device-session user (check ban)
                              │ no
                              ▼
        authlocal.IsLocalIssuer?     ──yes────► Local user (auth: local only, check ban)
                              │ no
                              ▼
        Auth0 RS256 verify (JWKS; requires audience + issuer)
                              │
                              ▼
        Find/Create user → check ban → set context (userid, user)
                              │
                              ▼
                        Handler executes
```

The three short-circuit branches (guest, device, local) each set `userid`/`user` and continue without contacting Auth0. Only tokens that match none of them reach the Auth0 JWKS path.

### Guest Authentication

**File:** `pkg/service/guest.go`

Guest sessions use HMAC-SHA256 tokens (not Auth0 JWTs):
1. Client calls `POST /api/guest` (public, no auth)
2. Server creates temporary User + Character, equips the template's starting items, and signs an HMAC token with `GUEST_SECRET`. Signed-in `CreateNewCharacter` uses that same equip step.
3. Client stores token in `sessionStorage` (dies with browser tab)
4. Auth middleware validates HMAC token before trying Auth0 JWT
5. Guest sessions expire after 30 minutes; cleanup goroutine deletes stale data

### Authorization

- Protected endpoints require valid JWT or guest HMAC token
- Frontend auth/session debug state must not log or retain token values, including truncated token excerpts
- Legacy admin endpoints (export/import) require explicit basic auth credentials; unset credentials disable the endpoints and `admin/admin` is rejected in release mode
- Direct character access and quest progress routes are limited to the owning user or admins
- Three-tier role system enforced via middleware:
  - **Player** (default): Can access own characters, read game data, play the game
  - **Guest**: Same as Player but with level cap (5) and session timeout (30 min)
  - **MUD Creator**: Full access to Creator area (write rooms, items, scripts, NPCs, etc.)
  - **MUD Admin**: All Creator access plus User Management (promote/demote/ban users)
- Admin role is assigned exclusively via `MUD_ADMIN_OAUTHID` environment variable
- Banned users are rejected at the auth middleware layer (403 on all authenticated endpoints)
- Ban enforcement stores both Reference ID and email for cross-account prevention

## Scalability Considerations

### Current Limitations

1. **Single Game Instance** - Game loop runs on single server
2. **In-Memory State** - Game state cache not distributed
3. **Channel Buffers** - Fixed size (20) and single-consumer sends remain a structural constraint; the self-deadlock they caused at ≥200 connections was fixed in Batch 20 and closed by QA in B21 (see "Batch 19 QA blocker" under Concurrency Model and `docs/qa/REGRESSION_REPORT.md` §B19.7/§B21), but a send to `onMessageReceived` from the game-loop goroutine is still a self-deadlock and must stay inline
4. **Synchronous SQLite writes on the hot path** - mitigated in Batch 20: `LastSeen`/`IsOnline` are persisted only on connect/disconnect instead of two `Update` writes per client message (§B19.6-1); the writes themselves are now serialised per user by the presence lock (Batch 23, see Thread Safety); other writes still block the calling goroutine
5. **Slow WebSocket consumers** - mitigated in Batch 20: `wsTransport.Send` enqueues to a per-connection queue and every frame is written under a `SetWriteDeadline` (10 s), so a non-reading client is disconnected rather than stalling broadcast/ping rounds (§B19.6-2); the companion `writeLoop` leak found by QA in §B21.5 was fixed in Batch 22 (transport closed idempotently on disconnect)

### Potential Improvements

1. **Clustering** - Distribute game across multiple nodes
2. **Redis State** - Shared state for multi-server
3. **Message Queues** - Replace channels for distribution
4. **Horizontal Scaling** - Stateless API servers

## File Organization

```
pkg/
├── entities/          # Data models
│   ├── characters/    # Player characters (character.go, derive.go derived stats, talents.go talent/prestige helpers)
│   ├── rooms/         # Locations
│   ├── items/         # Items and templates
│   ├── npcs/          # Non-player characters
│   ├── dialogs/       # Conversation system
│   ├── quests/        # Quest definitions and progress
│   ├── skills/        # Character abilities
│   └── traits/        # Shared behaviors
├── talents/           # Progression talent catalog (pure data, Phase 2.4)
├── mudserver/         # Game server
│   ├── game/          # Game engine
│   │   ├── combat/    # Combat engine, skill processing, simulator
│   │   ├── commands/  # Command handlers (incl. talents.go, prestige.go, channels.go)
│   │   ├── leveling/  # XP curves, level-up logic, stat scaling, talent milestones
│   │   ├── messages/  # Message types
│   │   └── def/       # Interfaces
│   └── mudserver.go   # WebSocket handling
├── server/            # HTTP API
│   ├── handler/       # Route handlers
│   ├── dto/           # Data transfer objects
│   └── auth.go        # Authentication
├── service/           # Business logic (+ llm/, llmopts/, llmprompt/, llmgen/ [Fase 6, cut A wired to handler/generate.go], ollama/, groq/ providers)
├── repository/        # Data access
├── db/                # Database client
├── resources/         # Per-character refilling balances
├── ruleset/           # Level cap, level-up mode, death, new day, resource catalog, passive regen, guide switch
├── gamemode/          # Process presentation and auth mode
├── authlocal/         # Local username/password sessions (fork default, Argon2id)
├── presentation/      # Text-client frame renderer and view
├── scripts/           # Script execution
│   ├── scripts.go     # Script entity and types
│   ├── scriptrunner.go # Runner interface
│   ├── events/        # Event system
│   │   ├── events.go  # Event types
│   │   ├── context.go # Event context
│   │   └── registry.go # Event handlers
│   └── runner/
│       ├── factory.go # Multi-runner factory
│       ├── defaultscriptrunner.go # JavaScript (deprecated)
│       └── lua/       # Lua runner
│           ├── luarunner.go
│           ├── sandbox.go
│           ├── pool.go
│           └── modules/ # tales.* API
└── util/              # Utilities
```

## Scripting System Architecture

The scripting system uses Lua (via gopher-lua) for dynamic game content. JavaScript support is deprecated but maintained for backward compatibility.

Refilling resources are a SQLite table (`character_resources`) owned by `pkg/resources`. The HTTP startup attaches one empty store to the game and to the Lua runner. Callers configure allowances later. Until then, `tales.resources.get` and `consume` report the key as missing.

### Script Runner Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                      MultiRunner                                 │
│           (Routes to appropriate runner by language)            │
└─────────────────────────────────────────────────────────────────┘
                    │                    │
         ┌──────────┴──────────┐ ┌──────┴─────────┐
         ▼                     ▼ ▼                ▼
┌─────────────────────┐  ┌─────────────────────────────┐
│  DefaultScriptRunner │  │       LuaRunner             │
│     (JavaScript)     │  │  (Primary, recommended)     │
│     [DEPRECATED]     │  └──────────────┬──────────────┘
└─────────────────────┘                   │
                                         ▼
                           ┌──────────────────────────┐
                           │        VM Pool           │
                           │  (Reusable Lua states)   │
                           └──────────────┬───────────┘
                                         │
                           ┌─────────────┼─────────────┐
                           ▼             ▼             ▼
                    ┌──────────┐  ┌──────────┐  ┌──────────┐
                    │ Sandbox  │  │ Modules  │  │ Context  │
                    │ Security │  │tales.*API│  │  Data    │
                    └──────────┘  └──────────┘  └──────────┘
```

### Lua API Modules

| Module | Purpose |
|--------|---------|
| `tales.items` | Item and template operations |
| `tales.rooms` | Room queries and management |
| `tales.characters` | Character operations (damage, heal, teleport, gold, bind, apply levels) |
| `tales.npcs` | NPC operations (templates, instances, spawning) |
| `tales.dialogs` | Dialog and conversation management |
| `tales.game` | Messaging, flags, items, room manipulation |
| `tales.quests` | Quest operations (accept, complete, progress, grant, abandon) |
| `tales.utils` | Utilities (random, UUID, dice rolling) |
| `tales.resources` | Configured refilling balances (`get`, `consume`) |
| `tales.instances` | Generate a private room line (`generate`) |

#### tales.game Functions

| Function | Description |
|----------|-------------|
| `msgToRoom(roomID, message)` | Send message to all players in a room |
| `msgToCharacter(characterID, message)` | Send message to a specific character |
| `msgToUser(userID, message)` | Send message to a specific user |
| `broadcast(message)` | Send message to all connected players |
| `msgToRoomExcept(roomID, message, excludeCharID)` | Send to room excluding one player |
| `log(level, message)` | Log a message (debug/info/warn/error) |
| `hasItem(characterID, itemID)` | Check if character has item (by ID or TemplateID) |
| `getFlag(characterID, flagName)` | Get a game flag from character's Flags map |
| `setFlag(characterID, flagName, value)` | Set a game flag (persisted to DB) |
| `revealExit(roomID, exitName, characterID)` | Reveal a hidden exit for a specific character (per-character, persisted on character) |
| `giveItem(characterID, templateID)` | Create item from template and add to inventory |
| `hasEquipped(characterID, slotName)` | Check if character has item in equipment slot |

#### tales.npcs Functions

| Function | Description |
|----------|-------------|
| `get(id)` | Get NPC by ID (template or persisted) |
| `getTemplates()` | Get all NPC templates |
| `isTemplate(id)` | Check if NPC is a template |
| `spawnFromTemplate(templateId, roomId)` | Create instance in memory |
| `getInstance(id)` | Get instance from NPCInstanceManager |
| `getInstancesInRoom(roomId)` | Get all instances in a room |
| `kill(id)` | Mark instance as dead |
| `setState(id, state)` | Set instance FSM state |
| `getState(id)` | Get instance state |
| `damageInstance(id, amount)` | Apply damage, returns true if died |
| `healInstance(id, amount)` | Restore health |
| `moveInstance(id, roomId)` | Move instance to room |

### Event System

Events allow scripts to respond to game actions:

```
Game Events                 Event Registry              Script Handlers
┌─────────┐                ┌─────────────┐            ┌─────────────────┐
│ player  │ ──dispatch──▶  │  handlers   │ ──execute──▶│ Script 1 (Lua) │
│ entered │                │  by event   │            │ Script 2 (Lua) │
│  room   │                │   type      │            │ ...            │
└─────────┘                └─────────────┘            └─────────────────┘
```

Event types include:
- Player events: `player.enter_room`, `player.leave_room`, `player.join`, `player.quit`
- NPC events: `npc.death`, `npc.spawn`, `npc.idle`
- Item events: `item.pickup`, `item.drop`, `item.use`, `item.create`
- Dialog events: `dialog.start`, `dialog.end`, `dialog.option`
- Room events: `room.action` (triggered by script-type room actions), `room.update`
- Quest events: `quest.start`, `quest.complete`, `quest.progress`

See [SCRIPTING.md](docs/design/SCRIPTING.md) for full documentation.
# A10 combat focus

`combatStart.targetId` names the enemy engaged by the initiating or joining player. The web client keeps the local focus ID and short lived threat warning in `MUDXPlusStore`; BattleStage and Tab send `focus <enemy ID>` when the selection changes. The command validates a living enemy in the player's combat instance and updates `CombatantRef.AutoAttackTargetID` without queuing an action. It sends a `combatAction` with `action: "focus"` to synchronize the client. An in-combat `attack <name>` still queues the hit and now sends the same focus action. Existing attack and hostile skill commands read the server target, then fall back to a living enemy.

## Quiet Dark theme selection

Quiet Dark: optional minimal dark client theme, opt-in via `ui.default_theme` or Settings. The theme registry accepts `quiet-dark` beside `dark-fantasy`, `clean-hud`, and `gilded`. Engine fallback remains `dark-fantasy`. `/api/server-info` `defaultTheme` still flows through SettingsStore to `body[data-theme]`. Theme CSS and OFL fonts live under `public/mud-client/public/themes/quiet-dark/`.
