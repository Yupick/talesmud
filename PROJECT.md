# TalesMUD Project Documentation

## Overview

TalesMUD is a browser-based Multi-User Dungeon (MUD) framework built with Go and Svelte. It provides a complete platform for creating and playing text-based multiplayer adventure games, featuring real-time WebSocket communication, a web-based content editor, and persistent game state via SQLite.

**Repository:** [github.com/TalesMUD/talesmud](https://github.com/TalesMUD/talesmud)

## v0.5.0 Engine Update

- SSH is an optional transport for classic MUD and 80×25 Door play. Configure `ssh.enabled`, listener and host-key paths, key/device/guest policy, timeouts and limits in game-mode YAML; [SSH access](docs/ssh-access.md) lists environment overrides and defaults. Public hosts and activation URLs belong to deployment configuration. Both browser pages use `VITE_AUTH0_DOMAIN`, `VITE_AUTH0_CLIENT_ID` and `VITE_AUTH0_AUDIENCE` at build time only when Auth0 login is enabled.
- Classic clients can start and poll device login independently of the SSH listener. Browser `/activate` confirmation issues a 30-minute account session; `SESSION_SECRET` or the persistent game-mode key signs it. REST remains Bearer-only and WebSockets use single-use tickets or a Bearer header. See [device login](docs/device-login.md).
- Creator search (Ctrl+K), where-used checks, content health, room/NPC inspectors, quest debugging, loot previews, loot-table/spawner pages, reachability and map overlays share the world reference index. The sidebar is collapsible; entity references use filterable modal selectors.
- Audit history records content edits. World definitions and raw character creation require a creator role. Admin undo and live operations run through the game loop. Creator live views expose NPCs and instances; character details and mutations require admin access.
- `tales -check sample-world` validates an imported pack without running a server. `-json` emits the report and `-fail-on error|warning` sets the failure threshold. See [content health](docs/content-health.md).
- NPC routes/roaming are timed and validated, pause for player engagement, and default to stationary. Structured examine/effect lines, pickup controls, optional WebSocket hello, combat reattach snapshots, `noAggro` rooms and `aggroOnSight` markers improve client behavior. Boss spawners respect `respawnDelay` and restore full HP; Door trading uses catalog names and supports scripted combat.
- Backend builds require Go 1.26 (toolchain `go1.26.9`); Docker uses Go 1.26. JWT verification stays on `github.com/golang-jwt/jwt/v5`. This fork defaults to local username/password sessions (`auth: local`) so no Auth0 tenant is required; Auth0 audience/issuer checks are mandatory only in the optional `AUTH_MODE=auth0` mode. Logs redact `access_token`, `ticket`, `code`, `activate`, `device_code` and `token` at query-key boundaries; terminal output strips control escapes and bidi controls.
- Frontend builds use Node 22. `netlify.toml` builds the Creator UI in `public/app` from its lockfile and publishes Vite's `dist` directory; existing plugin peer ranges use `--legacy-peer-deps`. Game and API hosting stay in the Go process.

## Documentation Index

- **Documentation index (all of `docs/`):** `docs/index.md`
- **Architecture:** `ARCHITECTURE.md`
- **Core Systems & Features:** `FEATURES.md` (comprehensive reference for all systems, data structures, and APIs)
- **Game design + MVP backlog:** `docs/design/GAME_DESIGN.md`
- **Door on the shared engine:** `docs/DOOR-ON-MUD.md`
- **Ruleset profile:** `config/ruleset.yaml` (level cap, level-up mode, death, new day, resource keys, combat pacing, regen for out of combat, resting, and in combat, `guide.enabled`, `combat.respawn_min_seconds`, aggro grace windows, `ui.default_theme`). Combat math stays in `config/combat_balance.yaml`. `ui.default_theme` is `dark-fantasy` when omitted. A deployment may set `clean-hud` or `gilded`.
- **Default game mode (fork):** `config/gamemode.yaml` (local username/password auth, classic presentation, port 8010, SQLite path, title/subtitle). Loaded automatically when no `-config` flag is passed.
- **Scripting system:** `docs/design/SCRIPTING.md`
- **World map implementation:** `docs/design/WORLD_MAP_IMPLEMENTATION.md`
- **Quest authoring guide:** `docs/design/QUEST_AUTHORING.md`
- **Dungeons & raid content pack design:** `docs/design/PHASE5_DUNGEONS.md` (adventure pack: `import/adventure/`)
- **Progression design (XP, talents, prestige):** `docs/design/PHASE2_4_PROGRESSION.md` (implemented in this fork — see FEATURES.md)
- **REST/WS API contract:** `docs/API.md` (catálogo real por grupo: auth/sesiones, personajes, quests, clanes/DMs, `/api/dungeons`, creator + LLM del creator, ops/admin, audit y protocolo `/ws`)
- **LLM integration design (Fase 6):** `docs/design/PHASE6_LLM.md` (implementación parcial; cut A, endpoints Creator, `offer`, FASE 6.5 modos A/B y FASE 6.3 cortes 1–3 están cableados. Batch 29 cierra los mecanismos B/C: saludo LLM desde `talk` solo tras diálogo autoral y quests, intercepción de texto libre en conversación `__llm__` y opciones `quest:<id>` validadas contra quests disponibles. B30 añade el modo B opt-in (`DMDecision`, ambush/environmental/narrative, kill switch `LLM_DM_DECISION_ENABLED` apagado por defecto). Siguen pendientes el anillo de memoria corta, guardrails completos de combate y `FallbackDialogID` como fallback de error; ver FEATURES.md)
- **QA regression report:** `docs/qa/REGRESSION_REPORT.md` (B14–B24; veredicto histórico **PASS con condiciones**; §B23 — verificación de la race de presencia cerrada en Batch 23; **§B24 — FASE 6.5 DM narrator — APROBADO**) + `docs/qa/BALANCE_ANALYSIS.md` (balance B17/B18). **P0 de carga cerrado — verificado por QA en B21:** deadlock del servidor a ≥200 conexiones (§B19, fix Batch 20) — ver "P0 de carga" más abajo
- **Fork setup guide:** `docs/fork-setup.md` (prerequisites, `.env`, local auth, LLM provider, game-mode YAML, Docker/Ollama)
- **Fork player-facing system docs:** `docs/user/clans.md`, `docs/user/dms.md`, `docs/user/channels.md`, `docs/user/party.md`, `docs/user/combat.md`
- **Player guide:** `docs/player-guide/`
- **Development docs:** `docs/development/`

## MVP Roadmap (next up)

Prioridades verificadas contra el árbol de trabajo (2026-10-30, Batch 28):

- **P0 de carga: cerrado** — deadlock ≥200 conexiones corregido en Batch 20 (Backend) y **verificado por QA en B21** (500 concurrentes sin wedge, `recv == sent`, 0 `msgErrors`); ver "P0 de carga" más abajo.
- **Race preexistente (cerrada en Batch 23):** el `*entities.User` compartido se mutaba sin sincronía desde el handler WS (`attachSession`/`handleConnectionClosed`), el bucle de juego (`ConnectUserSession`/`DisconnectUserSession`/`SetUserSessionCharacter`) y `game_handlers` (`handleUserJoined`); reportada como P2 abierta en §B22.4 (QA). **Corregida en Batch 23 (Backend)** con lock de presencia por usuario (`pkg/mudserver/game/presence.go`) + test de regresión `pkg/mudserver/presence_race_test.go` → `0 DATA RACE` con `-race` en el test de regresión. **Verificado por QA en §B23** (`go test -race ./...` completo: 0 `DATA RACE`; control negativo reproduce 6–19 warnings sin el lock).
- **FASE 5.1:** spawn/instanciación de mazmorras en juego desde las definiciones guardadas (hoy solo la biblioteca de diseños; faltan compile/run).
- **FASE 6:** FASE 6.3 está implementada hasta mecanismos B/C (Batches 25–29): `talk` mantiene prioridad autoral → quest → LLM; el texto libre puede continuar una conversación LLM reciente; las opciones `quest:<id>` se validan y delegan al manejador existente de quests. `npc.dialogue` v1.0.3 incluye el contexto de quests disponibles; el contrato `DialogOption.Kind` / `DialogMessage.FreeText` llega al cliente. El proveedor deshabilitado conserva el mensaje actual de NPC sin diálogo. FASE 6.5 modo B (`DMDecision`, ambush/environmental/narrative) está implementado en B30 como opt-in (kill switch apagado por defecto). Los guardrails son básicos; siguen pendientes protección completa frente a combate, anillo de memoria corta y `FallbackDialogID` como fallback de error.
- **FASE 5.3:** `npc_templates` ya tiene servicio, rutas REST creator (`/api/npc-templates`) y spawn en vivo best-effort desde el builder; `GET /api/npcs/templates` sigue leyendo la colección `npcs` por compatibilidad.
- **QA/ops:** pentest dinámico y SAST en CI, aplicar las recomendaciones de balance de §B17/§B18.
- Backlog histórico: `docs/design/GAME_DESIGN.md` y `MISSING_MVP_ITEMS.md`.

## Features

### Core Game Features

- **Structured WebSocket Protocol**
  - Optional protocol-1 `hello` negotiation preserves existing clients and command input.
  - Item `examine`/`inspect` adds `examineResult` with stats, value, description, and explicit effect lines alongside the original prose. The play client builds the RoomTextOverlay item card from `examineResult` when present (effects as explicit lines), with prose-parse fallback.
  - Guest sessions report `characterSelected.guest`; reconnecting during a live fight restores the web battle roster, turn, target, and deadline immediately.
  - Additive fields use `omitempty`; clients ignore unknown fields and types. See `docs/structured-messages.md`.

- **Room-Based World System**
  - Interconnected rooms with customizable exits (directional, named, teleport)
  - Hidden/secret exits (toggleable visibility in editor)
  - Room actions for custom player interactions (respond, broadcast, run script)
  - Room action names match case-insensitively and beat global `examine`/`take`/`use` when they collide
  - Movement exits match case-insensitively; room presence fan-out does not block the game loop on SQLite
  - Response actions send the narrative `response` text, not the help `description`
  - Action descriptions shown in room text ("You can:" section)
  - Visual backgrounds and mood settings
  - Coordinate-based world mapping (X, Y, Z grid) plus a compiled atlas: authored area-local coords and compass exits preserve room geometry; compact configurable zone centers form one continent with decorative biome ground; per-character fog of war at `GET /api/characters/:id/map`
  - Dynamic item and NPC spawning
  - Unique NPCs auto-spawn into their assigned room on server start via `CurrentRoomID`
  - Per-character friends list (`friend add/remove/list`); HUD overlay with online flags + whisper; guests refused
  - Guest-public NPC, enemy, and player portraits (`/api/portraits/:filename`). Player art covers Human, Dwarf, and Elf across Warrior, Rogue, Mage/Wizard, Ranger/Hunter, Cleric, and Druid; missing art falls back to a class silhouette.
  - Merchant shop overlay (structured `shop` WS message with item stats/description; click inspects, explicit Buy/Sell confirm; gilded fantasy frame compare-to-equipped deltas on buy inspect; dialog Trade inject). On phone, tablet, and short landscape the shop is a bottom sheet portaled above the room; taller desktop windows keep a centered panel. The close control stays in the sheet header.
  - NPC dialog overlay shows the portrait, the name, the speech, and the authored choices. There is no separate Leave conversation button. Choosing an exit option closes the panel when the server sends `dialogEnd`. Phone, tablet, and short landscape render the dialog as a scrollable bottom sheet above the room (a × runs the authored exit when that choice is on the node). Taller desktop windows use a centered scrollable panel. Play JS and CSS use cache tag `?v=v056`.
  - Phone, tablet, and short landscape open Character, Equipment, Skills, and Quest Log from Stats, Equip, Skills, and Quests. The sheets reuse the desktop widgets. A quest tap opens a framed detail with the description, objectives, progress, area, turn-in room, rewards, and turn-in hint. The sheet sits between the header and the hotbar. Header, hotbar, badge, console, and sheet controls use a 44px hit area. Phone body text in those sheets and the console is at least 14px. Typing `character`, `equipment`, `skills`, or `quests` in the console still prints the command. Desktop keeps the grid.
  - Player chrome Map: Cartographer overlay (desktop ~80% + intel rail; phone full-bleed + bottom intel sheet); tap select with Travel primary (Inspect optional); compact Map tab has no duplicate Map/Open Map header; 14 terrain types with six native 48px pixel-art variants plus room-driven roof/keep/service/landmark stamps and four underground floor styles on one sprite sheet; dithered biomes, smooth organic beaches/foam/depth bands and offshore rocks/coastal cliffs, mixed foothills and central peaks, biome-specific oaks/pines/marsh trees, crisp town glyphs, quiet dirt roads batched beneath relief, canopies, buildings and opaque town paving, water bridges along charted outdoor exits, street-shaped town paving with angular footprint walls, corner towers and road gates, roofs and town/farm props with filterable interior selection, parchment fog, textured sea, glowing you-marker, Fit world / recenter and zoom/pan, with up to 10× fit scale and 220px tiles (twice the previous closest view), using crisp nearest-neighbor sampling. World fit and minimum zoom share a continent frame at roughly 70–80% of the map height on desktop. The minimap shares cached overview/close landscapes and the underground renderer; 48px close detail bakes on demand in an OffscreenCanvas worker where supported, with a synchronous fallback. The distant overview retains a compact 32px cache; finer leaves, bark, timber lookout towers, roof tiles, masonry, bridge planks, crop rows and props remain crisp at maximum zoom. Lower uses 48px themed floors, bones/barrels and dim cluster light, with no long void-crossing connectors. The full map has low-rate water/smoke accents that respect reduced motion; trees and buildings have no drop shadows. Outdoor elevations stay on Overworld, interiors share exterior anchors, underground rooms stay on Lower. Terrain rules/defaults live in `pkg/worldmap/map_terrain.json`; compact centers/ground/town flags in `pkg/worldmap/map_layout.json`; read-only preview instructions are in `tools/WORLDMAP-PREVIEW.md`
  - Play client WS: single-flight socket gate; close 4001 (session replaced) does not auto-reconnect; `/play` JS/CSS served no-cache. An optional SSH listener (`ssh.enabled`, default off) uses the same session replace across web and SSH. See `docs/ssh-access.md`.
- Device login: classic `/activate` is a standalone “Connect this terminal/app” page and confirms only after a click. Public `/api/device/start` and `/api/device/poll` let installable clients obtain a signed 30-minute account session using the shared SSH device store. Set `SSH_ACTIVATE_URL` and a persistent `SESSION_SECRET` or session key path; see `docs/device-login.md`. Account menus list and revoke SSH keys linked from an SSH sign-in. Guests cannot confirm codes.
  - Action bar Option C: room dirs + room actions + Shop; fixed INV/MAP/SAY chrome; **Recipes** pin seeded by default; optional Look/Rest/… via ⋯
  - Gathering & crafting v1 (no professions): room GATHER chips + recipes/craft; R0209 CRAFT/RECIPES chips; R0102 first-gather hint
  - Spell Bar / Hotbar: docked on the desktop action bar (a moved hotbar widget stays separate); nine slots; skills + consumables; Rest seeded on empty/default bar (slot 7); Look/Talk/Flee bindable; no Search=look. The phone strip and the combat-dock hotbar shrink slot size and gap so all nine slots sit inside a 375px viewport. Keys 1–9 fire those slots, Tab cycles combat targets, Escape closes the top panel, and `?` opens the shortcut list. None of those fire while a command or other text field is focused.
  - Desktop and wide presets keep the room on the left. Character and Equipment share one tab (Character open), Terminal, Quest Log, and Map share another (Terminal open), and Inventory sits under those tabs. The play shell is the window height, and a saved layout taller than the window is scaled so the action bar stays on screen. Invalid saved layouts fall back to the viewport preset. Saving a layout while a widget is focused stores the arrangement from before that expansion.
  - Play terminal wraps room lines on word boundaries and reflows that scrollback when the panel is resized
  - Play header: one row for Edit Layout, Party, Friends, and the account chip, with a gold menu aligned to the chip
  - Material Icons are served locally and preloaded; icon ligatures remain hidden until the font loads, so a font failure leaves empty icons.
  - OOC Resting chip on character HP / mobile header while `Flags.resting`; clears on combat or rest end
  - BattleStage: arena art clipped to the fight band; Attack/Defend/Items/Flee + hotbar share one dock strip; FF-style plates; queue chip centered in the dock. Layout B is the default fight view (party left, enemies right, corner detail frames, slim over-sprite HP, existing race/class and enemy portraits). Other players show on a compact left ally strip. Classic cards remain a Settings opt-out (`?battleLayout=classic`). Every fighter in a group appears: the player's large card and up to four compact ally cards with live HP/MP, turn, down/fled, hit FX, and join banner. Damage numbers float over the struck sprite in white with a dark outline, holding full opacity before fading. A missing portrait or item icon swaps once to a class, enemy, or generic silhouette instead of a broken image.
  - Combat start/join focuses the existing BattleStage cover by default. The local Settings panel can turn auto-focus off; a manual Open BattleStage button remains available during combat. Dismissing victory/defeat, outcome timeout, or leaving combat restores the previous panel focus. Layout and template saves keep the normal arrangement; command input and text fields keep their typing focus.
  - Settings persist locally: theme, parchment room descriptions, reduced motion (system/on/off), combat auto-focus, Battle layout B (default on; off selects Classic), and inventory opening as an overlay or layout widget. Theme `auto` (the stored default) follows `defaultTheme` from server-info. A named choice (Dark Fantasy, Clean HUD, Gilded Frame, Quiet Dark) is kept. A saved `dark-fantasy` that was never explicitly chosen migrates to `auto`. Gilded Frame paints dialogs, the quest log, the shop, notices, the combat stage, and room entity cards from `public/mud-client/public/themes/gilded/w3-screens.css` inside the existing boxes. Audio controls are held until game sound exists; inactive Compact Mode and Room Text Overlay controls are hidden.
  - Combat self and allies share one row on desktop and a stacked section on phones. Joins show one banner; combat prose stays in the log. Defeated enemy sprites remain visible in grey with a Defeated label.
  - In multi-enemy combat, clicking a BattleStage enemy or pressing Tab changes the gold focus highlight and server aim without spending a turn. Basic attacks and hostile skills use that living target; switching to an orange, red, or skull foe repeats the level-gap warning once per change. A dead target falls back to a living foe.
  - Say chrome opens a message popup, then sends `say <text>`
  - Inventory chrome opens overlay by default (preference: overlay | on-screen widget). Inventory and party sheets paint above the phone header and Room/Console tabs.
  - Phones in short landscape use the scrollable Room/Console shell through 1024px width and 500px height. The desktop home link stays outside that room layout.
  - Quest and room notifications wait through combat, victory/defeat, and pending loot rolls. Their reading time starts on display. A previous fight’s timeout cannot dismiss a later result. Outcome rewards scroll while Continue stays visible; short landscape places outcomes beside Need/Greed.
  - Equipment paper-doll: square slots around portrait (head/neck/chest/hands | legs/boots/ring1/ring2; main_hand + off_hand under); compact HP/MP/ATK/DEF strip below all slots. The panel scrolls when the doll needs more height, so the stat strip never covers the portrait or weapons.
  - Gilded Frame (`body[data-theme="gilded"]`) repaints inventory, equipment slots, the character sheet, item cards, Need/Greed, party, recipes, and sign-in with bronze frames, leather section plaques, gold labels, and quality borders. The sign-in card keeps its size. Play as guest is a leather plaque with gold text. Continue with X and Continue with Google are a separate bronze plaque. Empty equipment slots draw line silhouettes, and a stack count sits in the slot corner. `dark-fantasy` stays the engine default. The screen stylesheet is `public/mud-client/public/themes/gilded/w2-screens.css` (Marcellus, OFL).
  - Quiet Dark: optional minimal dark client theme, opt-in via `ui.default_theme` or Settings. `dark-fantasy` stays the default. OFL fonts ship with the theme under `public/mud-client/public/themes/quiet-dark/`.
  - Clicking equipped gear opens a detail card; Unequip is an explicit action. Inventory item cards show stats, value, available weight, usability requirements, and differences from worn gear. Clear class-relevant upgrades get a green badge. Equip, Use, Sell, and Drop remain explicit buttons. A unique item card has a gold frame and a UNIQUE mark.
  - A loot entry can be `rarity: unique` with a `chance` and `boss_only`. The roll is skipped when every victory recipient already holds that template (bag, equipped, nested container; no separate item bank). That drop announces `UNIQUE:` as a gold room chip. A template `unique` flag caps ownership and does not send the chip.
  - Room action/system reaction toast: centered on room hero art; LOOK-sized padding (no half-cut last line)
  - Quest Accepted / Complete: centered moment cards; open Talk dialog refreshes `[Quest]` → `[In Progress]`
  - Quest log Turn In for anywhere-ready quests; otherwise Turn in: NPC hint
  - Gilded fantasy frame private cellar instances: `type: instance` exits, or a shared-room exit into a room tagged `instance`, clone a small room graph per character; town hub stays shared; empty copies are destroyed. Startup deletes leftover `~` room rows and moves characters to the hub, the copy's return exit, the start room, or the bind room, then tells them once on the next login

- **Character System**
  - Full RPG character creation with races and classes
  - Six-attribute system (STR, DEX, CON, INT, WIS, CHA)
  - Equipment system with 10 equipment slots
  - Equipped armor takes durability damage on death (not deleted); `repair` at a merchant restores it
  - Inventory management
  - Experience and leveling with flattened early-game XP curve (piecewise formula: gentle L2-5, transitional L6-15, steeper L16+)
  - Exploration XP: awards 5 XP per new room discovered, 15 XP for first room in a new area/zone
  - **Distributable Attribute Points**: 2 points per level-up for players to allocate freely into STR, DEX, INT, WIS, or STA
  - Class-based attribute caps prevent degenerate builds (e.g., warriors cap INT at 5, wizards cap STR at 5)
  - Terminal command `spend <attr> [amount]` to allocate points; `spend` with no args shows status table with current values, spent/cap per attribute
  - Character widget shows unspent points badge and interactive "+" buttons on each attribute when points are available
  - **Derived Combat Stats Display**: Character widget shows computed ATK (weapon damage + STR modifier), DEF (total armor from equipment), and MP/RND (mana regen per combat round, caster classes only). These update live when equipment or attributes change.
  - Existing characters receive retroactive points on login ((level - 1) * 2)
  - Server-side room/area discovery tracking per character, used by the discovered-world atlas (web + mobile JSON)
  - All-time statistics tracking (including rooms discovered)
  - Mana system for caster classes (Mage, Cleric, Druid) with level and INT scaling
  - Mana regeneration: `regen.out_of_combat` defaults to 2% HP and 5% mana every 10s, `regen.resting` to 10% HP and 15% mana every 10s, and `regen.in_combat` to 0.5% HP and 1% mana every 10s. Intervals follow the server clock. Per-round combat mana stays `CalculateManaRegen` (1+WISMod) and is separate from `regen.in_combat`.
  - Mana potions (Small/Medium/Large) as consumable items

- **Progression: Derived Stats, Talents & Prestige (Phases 2.3 & 2.4)**
  - **Derived combat stats** (`pkg/entities/characters/derive.go`): one shared `DeriveStats` projection — `Stamina = STA + 2×Level`, `Initiative = DEXMod + Level/2`, `Resist = WISMod + CONMod + Level/2`, `CritChance = 5% + DEXMod% + kit bonus` (clamped 0–95), with `MaxHP`/`MaxMana` mirroring the stored pools that combat snapshots. Emitted on the `characterUpdate` payload (`stamina`, `initiative`, `resist`, `critChance`, `talents`, plus talent projections)
  - **Talent points on milestones**: 1 point at each of levels 5, 10, 15, 20, 25, 30, 35, 40, 45, 50 → **10 points** before prestige (`leveling.TalentMilestoneInterval = 5`, counted once per crossed milestone, reported in `LevelUpResult` and in the level-up banner)
  - **Talent catalog** (`pkg/talents`): 12 passive talents in 3 categories (offensive / defensive / utility), one deterministic effect per talent, gated by classkit school (`classkit.TalentsForClass`; an explicit `talents:` list on the class YAML wins). Stored as JSON on the character document (`Character.Talents`: `Points`, `Spent`, `Prestige`) — no table, no DDL migration
  - **Effects**: `ApplyTalentBonuses` folds every rank into `DerivedStats` at the end of `DeriveStats` (crit re-clamped afterwards); `MaxHPBonusPercent` stays separate from the pool combat snapshots
  - **Command `talents`** (`talents` / `talent`): `talents [list]` groups the class talents by category with ranks and unspent points; `talents spend <id|name|unique prefix>` buys one rank (blocked during combat, persists through `CharactersService().Update`)
  - **Command `prestige`**: available at level 50, `prestige` shows status, `prestige confirm` performs the irreversible reset to level 1 with class base attributes, then grants +1 prestige and +1 talent point. Permanent `+5% XP per prestige` (`PrestigeMultiplier()`) applies to combat XP (share + first-kill) and quest XP; talents, gold, gear, inventory, discoveries, friends, and all-time stats persist
  - Design reference: `docs/design/PHASE2_4_PROGRESSION.md`; detailed reference in `FEATURES.md`

- **Clans & Private Messaging (Phase 4)**
  - Persistent clan storage: `Clan`, `ClanMember`, `ClanInvite`, `ClanBankLedger` entities (name/tag, leader, level/XP, shared bank, MOTD, soft-delete `active`)
  - One clan per character enforced in `ClanRepository.AddMember`; ranks member/officer/leader; invite lifecycle (pending/accepted/declined/expired)
  - Append-only clan bank ledger with deposit/withdraw and balance snapshots
  - Service layer (`ClansService`): create clan, invite/accept/decline, kick, change rank, deposit/withdraw, disband, list members, with leader/officer authorization rules
  - Persistent private messaging: `DMChannel` (on-demand, one canonical channel per character pair) and `DMMessage` with per-message `readAt`
  - `dm_messages` uses real `channel_id`/`sent_at`/`from_char_id` columns so `FindMessagesSince` is index-backed instead of `json_extract`
  - Service layer (`DMService`): channels per character, find-or-create (canonical pair), send (anti-spoof: sender must be a participant), fetch messages, mark read
  - Player-level REST API under `/api/clans` and `/api/dm` (ownership checked on the acting character; routes mounted in `pkg/server/server.go`)
  - Play client UI is wired over REST — no game commands (`clan …`) and no WebSocket push: `ClanPanel` (`public/mud-client/src/game/ui/ClanPanel.svelte`, toggled by `C` or the header shield button in `CharacterSwitcher.svelte`) and `DMsPanel` (`public/mud-client/src/game/ui/DMsPanel.svelte`, toggled by `M`) call `/api/clans*` and `/api/dm*` with the session JWT. The DM panel refreshes every 5s while open, shows per-channel unread badges from `unreadA`/`unreadB`, and marks the open channel read. Header buttons are hidden for guests; the in-game `tell` command stays in-memory (online players only). See `docs/user/clans.md` and `docs/user/dms.md`.

- **Skills & Spells System**
  - Database-stored skills, editable via Creator UI (Skills tab)
  - Multi-class support: skills can be assigned to multiple classes (e.g., Heal for Cleric and Druid)
  - 29 default abilities across 6 classes, seeded on first run
  - **World-pack class catalog** (`pkg/classkit`): the combat kit's classes load from YAML (`data/classes`, `CLASSKIT_DIR`, or a pack's `data/classes`) instead of the built-in sample trio
  - **Magic schools**: optional `school` field per class (`martial`, `fire`, `ice`, `arcane`, `divine`, `nature`, `shadow`) with a `classkit.SchoolName(id)` lookup; an unknown school or effect fails the whole directory load
  - **Fork base classes** in `data/classes/`: `warrior`, `rogue`, `mage`, `cleric`, `druid`, `sorcerer`, with Spanish aliases (`guerrero`, `picaro`, `mago`, `clerigo`, `druida`, `hechicero`); Cleric and Druid now carry the `mend` (direct heal) and `renew` (HoT) kit effects, both ally-targeted and scaled off max HP, while `mage_mend` and `sorcerer_mend` are self-only (`target: self`, 20% of max HP)
  - **Mana cost + per-school component**: kit skills may declare `mana_cost` (paid in the same step as the cooldown; a cast that cannot afford it is rejected before any mana or cooldown is spent) and a `component` reagent hook — `arcane_dust` (arcane), `fire_salt` (fire), `divine_wax` (divine), `nature_herb` (nature). `component` is structural only today (no reagent inventory); it flows into the `/api/classes` payload. Physical (`martial`) skills keep `mana_cost: 0`. See `pkg/classkit/catalog.go` and `pkg/classkit/worldpack_data_test.go`.
  - Two resource types: mana-based (casters) and cooldown-based (physical classes)
  - Equippable skill slots (1-4 per class, level-gated progression)
  - Skill management: equip/unequip outside combat, locked during combat
  - Skill effects: damage, heal, buff, debuff, DoT, HoT, stun, multi-hit, AoE
  - Status effect system: buffs, debuffs, DoTs, HoTs with duration tracking
  - Attribute-scaled damage: STR (warrior), DEX (rogue/ranger), INT (mage/druid), WIS (cleric)
  - Mana shield absorption mechanic
  - Skill cooldown tracking per combat instance
  - Combat commands clear stale character combat flags when no live combat instance exists
  - In-combat commands: `cast <skill> [target]`, numeric shortcuts `1`-`4`
  - Management commands: `skills`, `skills equip <name>`, `skills unequip <name>`
  - YAML import/export for skills data

- **Item System**
  - Multiple item types: Currency, Consumable, Armor, Weapon, Collectible, Quest, Crafting Material
  - Quality tiers: Normal, Magic, Rare, Legendary, Mythic
  - Item templates for reusable definitions
  - Stackable consumables and partial drops persist reduced quantities consistently between character inventory and item instances
  - Container support with nested items

- **Quest System**
  - Data-driven quest definitions with multiple objective types: Kill, Collect, Deliver, Visit, Talk, Custom (Lua)
  - Quest progress tracking per character with persistent state
  - NPC dialog integration: automatic quest offer/turn-in options injected into NPC conversations, including quest-only NPCs without full dialog trees
  - Real-time quest log WebSocket updates include quest definition details, objectives, and rewards after dialog quest actions
  - Quest rewards: XP, Gold, and item grants on completion
  - Quest prerequisites: required quest completions and level requirements
  - Repeatable quests support. Optional `repeatReset: daily` or `repeatReset: cooldown` blocks the next accept until the ruleset hour (`quests.daily_reset_hour_utc`, default 4) or `repeatCooldownSeconds`. Plain `repeatable: true` still reopens immediately. Quest payloads include `repeats`, `repeatReset`, and `nextAvailableAt` (absolute UTC) while the window is closed.
  - Quest categories and area labels for filtering and organizing regional quest lines
  - QuestTracker: automatic progress updates from game events (NPC kills, item pickups with stack quantities, room entries, dialog nodes, NPC delivery checks)
  - Accepting a collect quest pre-fills progress from matching items already in the character inventory, including stack quantities
  - Delivery objectives require and consume matching inventory items before progress is granted
  - Player quest log shows enriched objective descriptions, ready-to-turn-in state, and quest notifications for accept/progress/ready/complete events
  - Lua scripting API (`tales.quests`) for custom quest logic
  - Creator UI: full quest editor with objectives, rewards, prerequisites, dialog text configuration, validation feedback, and player flow preview
  - Player commands: `quests`/`ql` (quest log), `quest <name>` (details), `abandon <name>` (abandon quest), `guide dismiss` (close the next-step hint)
  - Next-step hint: one dismissible chip for a new character. A quest may set `guide: true`. When none do, the hint uses the active quest log and prefers an open kill. A quest with `repeatReset` is not chosen while a normal quest is still open. It walks room exits to the objective, then offers attack or `complete <name>`. The fight line is `Attack the <name>.` The same wording is what telnet, SSH, and Door show. Dismissal is stored on the character (`flags.guideDismissed`) and the chip stays hidden after any completed quest. The client keeps it off the room name, the hotbar, dialogs, the shop, and Need/Greed. `config/ruleset.yaml` `guide.enabled` defaults to on. A game-mode file may set the same key. `false` builds no hint and sends nothing.

- **Guest Mode (Play as Guest)**
  - Anonymous 30-minute demo sessions without Auth0 registration
  - "Play as guest" button on the logged-out welcome screen, under Continue with X, Continue with Google, and Email and password
  - Random character with random class from system templates
  - Spawns in `ServerSettings.StartRoomID` (default `R0001` when that room exists)
  - Auto-grants `source.type: auto` quests for the start room's zone (Z00 catacombs: QST0001–QST0004)
  - Entering a new area grants that zone's auto quests (Z01 meadows: QST010*)
  - Full starter items equipped automatically
  - Per-character level cap of 5 for guest characters
  - Full chat access during session
  - 5-minute warning before session expiry
  - Auto-deletion of guest user + character after session ends or disconnect (5-min grace period for reconnection)
  - Server-configurable: `GuestsAllowed` (default: true), `MaxGuestAccounts` (default: 20)
  - IP-based rate limiting (10 guest sessions per IP per hour)
  - HMAC-SHA256 guest tokens (separate from Auth0 JWTs), signed with `GUEST_SECRET` env var
  - Background cleanup goroutine removes expired guest accounts every 5 minutes

- **New Player Onboarding**
  - Phase-based flow: Welcome Screen, Nickname Setup, Character Creation Wizard, Game
  - Logged-out players see **Continue with X**, **Continue with Google**, **Email and password**, and **Play as guest**. X and Google pass `connection` (`twitter`, `google-oauth2`) so Auth0 skips the universal login password form. Email is the only button that opens that form
  - Logout clears the Auth0 session and this tab's guest token, then returns to that choice. A guest reload still resumes the guest session
  - The account menu has **Switch character**, which opens a picker of every character from `/api/my-characters` and sends `sc <name>`. Guests get the same X, Google, and email choices. Signed-in players get **Log out**
  - A signed-in player with more than one character sees that picker once per login (the server still enters on `lastCharacter`)
  - First-time users prompted to choose a display name/nickname
  - Three-step character creation wizard: Choose Template, Name Character, Confirm & Create
  - Signed-in create equips that template's starting items: a fresh copy of each named starter, not the template itself. Guests use the same equip path. Characters already in the world are left unchanged.
  - Automatic phase detection from user profile and character data
  - Guest users skip onboarding (character auto-created server-side)

- **Room Text Overlay**
  - Room actions, chat, and item examine cards appear over the room image. Combat feedback stays in BattleStage and the console.
  - Up to four room messages render at once. Later messages queue; a deliberate new examine replaces the previous examine and appears first.
  - Reading time scales with text length. Hidden room tabs, combat outcomes, and loot rolls pause presentation.
  - Entry animations respect the system reduced-motion preference. Dismissal removes cards immediately.

- **Multiplayer**
  - Real-time player interactions via WebSocket
  - Players see each other in rooms
  - Global and room-based chat
  - Global chat channels: `general`, `trade`, `lfg`, `help`, `roleplay`, `admin` (`chat <channel> <message>`; `admin` restricted to admins)
  - Channel lines carry a `channel` tag that the play client routes into its `channel` chat tab with per-tab unread counts (`public/mud-client/src/game/chatTabs.js`, `ui/ChatPanel.svelte`)
  - Private tells/whispers and minimal party chat
  - Party flow: create parties, invite online players, accept/decline invites, list members, leave party, and send party chat
  - Party Combat Assist v1: same-room players join in-progress fights via `attack <npc>`; party members get a join nudge; XP/gold split among living joiners
  - Party Loot & XP Share v1: victory gold and XP also split equally with online party members in the killer's room (leftover to the engager; out-of-room and offline members excluded)
  - Party Need/Greed v1: with 2+ eligible party members, item drops open a 30s Need/Greed/Pass round (Need > Greed > Pass; all-pass/timeout → ground; sequential multi-drop). The roll panel stays above the victory/defeat card and stays clickable while the timer runs.
  - Party loot rule: `party loot <free|leader|roundrobin|needgreed>` (leader-only change, persisted on the Party row and echoed as `lootRule` in the `party` payload). Under `free` drops stay in the room; under any other rule victory drops are auto-assigned among the living players in the fight with a `[Party] item → player (reason)` line (`pkg/mudserver/game/loot_party.go`). The rule applies when the Need/Greed round does not.
  - Party ready check: `party readycheck` (leader-only) opens a 30-second window, members answer with `party ready` / `party notready`, and the party sees `[Party] Ready: n/m · Not ready: n/m` updates plus a final summary. State is in-memory on the session registry (`pkg/mudserver/game/sessions_ready.go`, `pkg/mudserver/game/commands/party_loot_ready.go`)
  - Party Follow v1: `party follow` / `party unfollow` trail the party leader through normal exits (`RelocateCharacter`); combat, teleports, portals, and private instances do not pull followers
  - Party UI: HUD launch next to Friends, PartyOverlay (create/invite/list/say/leave), leader-only Loot radio group and Ready check controls, centered Accept/Decline invite popup with countdown/timeout; guests can party like signed-in players
  - Emote system
  - Live session-based player presence tracking for room UI, chat routing, `who`, and silent room presence refreshes
  - Reconnect-aware client state with visible connecting/reconnecting status and automatic reconnect attempts
  - In-game character switcher for changing active characters without leaving the play UI

- **Play Client UX (Phases 3.2, 3.4, 3.7)**
  - **Command history + tab completion** (3.2): ArrowUp/ArrowDown browse the last 50 commands (persisted in `localStorage` under `talesmud_command_history`) while preserving the draft you were typing; Tab completes command names and room player names (`public/mud-client/src/game/commandHistory.js`, used directly by `TerminalXWidget.svelte` and, through `echo/HistoryController.js`, by `TerminalWidget.svelte`)
  - **Minimap exit legend** (3.4): a compact `N / NE / … / ↑ / ↓` strip built from the server-emitted exits and the atlas discovery flags; undiscovered targets show `?` and nothing undisclosed is invented (`public/mud-client/src/game/minimapDirections.js`, rendered by `MinimapWidget.svelte`)
  - **Responsive play shell** (3.7): media queries at 768px / 480px / 420px tighten container padding (`Game.svelte`), make the inventory overlay full-width (`InventoryOverlay.svelte`), and pin the quest-notification stack so phones get no horizontal scrollbar (`QuestNotifications.svelte`)

### Content Creation

- **Dungeon Builder (FASE 5.1, key `D`)**
  - Client panel `DungeonBuilder.svelte` with pure modules `dungeonBuilder.js` (grid, rooms, corridors, encounters, loot tables, BFS entry→exit validation, overlap checks, test mode) and `dungeonLibrary.js` (export/import of the opaque `definition`)
  - Persistence: `localStorage` as the source of truth with **best-effort** sync to `POST/PUT/DELETE /api/dungeons` when a creator token is present (401/403/404 or network failure degrades to local with a visible warning)
  - Server side: `DungeonsService` + `Factory.Dungeons()` over the `dungeons` table; authenticated reads (`GET /api/dungeons`, `GET /api/dungeons/:id`), creator-gated writes. Contract: `docs/API.md` §5
  - **Not implemented yet**: spawning/instancing dungeons in game from a saved design (no compile/run endpoint)

- **Web-Based Editor**
  - Full-width filterable data tables for browsing all entity types (Rooms, Items, Item Templates, NPCs, Dialogs, Quests, Skills, Scripts, Character Templates)
  - Per-column filtering (text search, enum dropdowns) with instant client-side filtering and sorting
  - Side-by-side master-detail layout: data table + edit form shown together, closeable to full-width table view
  - **Entity Selection Modal**: All entity ID selectors (rooms, NPCs, items, scripts, dialogs, quests, character template starting items) use a centered modal dialog with a full filterable DataTable instead of simple dropdowns. This scales to hundreds of entries with per-column search, sort, and filter support. Components: `EntitySelectButton` (inline trigger) + `EntitySelectModal` (table dialog). **UI Guideline: Never use `<select>` dropdowns for entity ID references. Always use `EntitySelectButton` with the appropriate column definitions from `tableColumns.js`.**
  - Room editor with exit, action, spawner, items, and NPC resident configuration. An Inspector tab shows exits in and out (hidden exits name the script that reveals them), NPCs and spawners, items, on-enter and action scripts, visit quests, and reachability from the start room. Live characters, NPC instances, and instance copies come from the existing live endpoints. Admin teleport and instance cleanup use the shared confirm dialog. Inbound references stay on the Referenced by panel under the form.
  - Item and item template management with attributes and properties
  - NPC editor with behavior controls for state, spawn room, wander radius, patrol paths, idle chatter, enemy traits, and merchant traits. The enemy tab edits combat stats, behaviour, loot, and Lua hooks. Imported enemies keep unscaled content base stats beside the effective stats combat uses. `GET /api/balance/enemy-scaling` supplies the tier and named-override factors. An unknown tier warns and applies no scaling. The detail can switch from the form to an Inspector: type, level, difficulty, content base versus effective stats, loot chances, dialogs, hooks, spawners, and kill/talk/deliver quests. The loot table id opens `/creator/loot-tables`. Live instances come from `GET /api/live/npcs`. Admin heal, respawn, despawn, and end combat use the shared confirm dialog. Inbound references stay on the Referenced by panel under the form. NPC type is a table badge; the row dot is reserved for validation issues. Tables show full entity IDs with a copy button.
  - Loot tables (`/creator/loot-tables?id=`) list and edit drop entries: item template, chance, quantity, boss-only, and guaranteed. `POST /api/loot-tables/:id/roll?n=1000&seed=` previews drop frequency. The preview is seeded, creates no items, and is not an audit write. Inbound references stay on the Referenced by panel.
  - Spawners (`/creator/spawners?id=`) list room, template, max, respawn time, and the live count from `GET /api/live/npcs`. Filters cover template, room, zone (the room area), and unique NPCs that have no spawner. The editor uses the existing spawner API. Room and NPC links open those inspectors (`?view=inspector`). Both editors also open the inspector for `?tab=inspector`.
  - Lua script editor with syntax highlighting and integrated test runner
  - Dialog tree editor with options and alternate texts
  - Quest editor with validation, player flow preview, and a quest debugger for the step chain, reachability, prerequisites, and live character progress
  - Character template editor with archetype selection and starting gear
  - Skills editor with multi-class assignment, resource types, effects, and secondary effects
  - World map visualization (GridWorldEditor). Fit view frames tiles with north up (smaller Y toward the top), so a zone filter such as Z02 stays on the canvas. A Reachability layer colors rooms and lists unreachable islands from `GET /api/world/reachability`. Each island reason is its own line, and room and script ids link to the room inspector (`?view=inspector`) and the script editor. Level, aggro, spawners, players, quests, missing art, and live instance copies are separate toggles on `GET /api/world/overlays`, saved in `localStorage`. Player names are admin only. Overlay badges stay about 12px on screen at any zoom. A quest badge reads Q×N and keeps the quest ids in the tooltip. Badges that do not fit the tile at that size collapse into +N. The tile itself is unchanged.
  - World Health diagnostics for broken cross-system references across rooms, NPCs, items, loot tables, quests, dialogs, scripts, spawners, and character template starting gear. The message column stays visible, and room, NPC, dialog, quest, item, and script IDs link to `?id=` on that editor.
  - Content health: one rule report for reachability, reveal scripts, quests that cannot complete, bosses without spawners, unknown difficulty tiers, unreferenced scripts, and pack rules from `data/rules`. An item from a completable quest reward counts as obtainable when that quest's prerequisites can come first. Reachability starts at the configured start room, then the engine default. The content commit is that folder's own git HEAD, or `CONTENT_COMMIT` / `.content-commit`, or `unknown`. Live health warns with `deploy-tree-dirty` when the server checkout has tracked changes outside `import/`. The Creator Health page and `tales -check` share the content rules. See `docs/content-health.md`.
  - Creator quality validation: inline warnings/errors, broken-reference detection, save blocking for invalid references, and a world health diagnostics tab. Dialog node IDs may repeat when the same node is linked again; a duplicate is an error only when two definitions of that node disagree. Opening `/creator/<tab>?id=<entityId>` selects that entity. Deletes that call the API, plus room, room-item, special-exit, and dialog-node deletes, ask in a confirm dialog first. Merchant stock `maxQuantity` of -1 is unlimited.
  - Live ops on the Players tab (admin only): teleport, give and take items, end a fight, quest step changes, re-grant a starter kit, and instance cleanup. Creators do not see the Players tab. NPC heal, respawn, and despawn, and room teleport and instance cleanup, share `OpsButtons`, which stays hidden unless the user is an admin. Every op confirms first, runs on the game command loop, and can offer Undo from the audit log when the change is reversible.
  - Audit log: creator CRUD writes and live ops, with before/after JSON and admin undo. A red LIVE badge appears in the top bar when `ADMIN_ENV_LABEL` is set.
  - Drift page (`/creator/drift`, Operate): entities changed since the last import, with each field's before and after value. Export YAML downloads one entity. Export all downloads every added or changed entity as one concatenated YAML file. Removed entities are listed and are not exported.
  - Creator navigation is a left sidebar (World, Actors, Narrative, Systems, Operate). World includes Spawners. Actors includes Loot tables. It collapses to icons with Ctrl+B or Cmd+B, folds each group on its own, and becomes an off-canvas drawer below 1024px. The top-bar search field opens the creator search palette. Below 1024px the text links and that search field are hidden so the LIVE badge stays on one row. Classes have no editor route, so they are not in the sidebar. `/creator/items` stays reachable and is not listed.
  - Creator search (`GET /api/search`): Ctrl+K or Cmd+K opens a palette. Results are grouped by type, and Enter opens the matching Creator editor. The shortcut is ignored while typing in a field or the script editor. Rooms, NPCs, items, dialogs, quests, and scripts show a collapsed "Referenced by (n)" panel under the form. Deleting one of those entities names the inbound references.
  - Preview/test tools for dialogs, quests, rooms, merchants, and Lua scripts
  - CRUD operations with live preview

- **Scripting System**
  - Lua-based scripting via gopher-lua (primary)
  - Room action scripts (type "script" triggers Lua execution with room.action context)
  - Room on-enter scripts
  - Item behavior scripts
  - NPC behavior scripts
  - Quest scripting support
  - Game API: messaging, inventory checks (`hasItem`, `hasEquipped`), character flags (`getFlag`/`setFlag`), item rewards (`giveItem`), per-character exit reveals (`revealExit`)

### AI / LLM Integration

- **Swappable LLM Provider (Ollama by default)**
  - Provider-agnostic contract `llm.Client` (`pkg/service/llm/`): `Complete(ctx, systemPrompt, userPrompt, temperature) (string, error)`
  - Two interchangeable implementations behind the contract: local **Ollama** (`pkg/service/ollama/`, default, no API key) and hosted **Groq** (`pkg/service/groq/`)
  - Provider selection via `LLM_PROVIDER` (`ollama` | `groq` | `none`/`off`); unset keeps backward compatibility (Groq when `GROQ_API_KEY` is present, Ollama otherwise)
  - Character creation: AI-generated names and descriptions based on selected template (archetype, race, class, backstory)
  - Protected API endpoint: `POST /api/generate/character` (auth required, guests rejected, per-user rate limit)
  - Ollama settings: `OLLAMA_BASE_URL` (default `http://localhost:11434`), `OLLAMA_MODEL` (default `llama3.1`), `LLM_TIMEOUT_SECONDS` (default 60)
  - Graceful degradation: no configured provider → `503 AI generation is not configured`
  - Also wired today: quest generation (creator endpoints + in-game `offer` command), room-description previews, combat DM narration and the **`ask` command** for free-form NPC dialogue with persistent memory, structured personality and LLM memory summaries (see Fase 6 below)

- **Fase 6 — LLM integration (status: in progress)**
  - The design in `docs/design/PHASE6_LLM.md` (§0–6.7) specifies `pkg/service/llmprompt` (versioned YAML prompt templates + `text/template` rendering, sanitised/`UNTRUSTED INPUT` variables, deterministic fallbacks) and `pkg/service/llmgen` (context → render → LLM → parse → validate → repair → fallback pipeline with cache, rate limits and a concurrency semaphore), plus NPC runtime dialogue, a dynamic quest generator, Dungeon Master narration, and Creator admin endpoints
  - **Present (working tree, uncommitted):** `pkg/service/llmopts`, `pkg/service/llmprompt` (+ 6 prompt YAMLs), `pkg/service/llmgen` (pipeline + cache/config/fallback/filter/limiter/parse/stats/catalog/questgen) and the `npc_memories` table with `Factory.NPCMemories()` + `NPCMemoriesService`.
  - **Wired:** cut A — `pkg/server/handler/generate.go` renders character name/description through `llmprompt.Registry` + `llmgen.Pipeline`; **B16** — creator endpoints in `pkg/server/handler/llm.go` (incl. `POST /api/creator/llm/quests/contextual`) and the DM narrator scaffold (`game/dm_narrator.go` installed from `pkg/server/llm.go`, kill switch `LLM_DM_ENABLED`); **B18** — in-game contextual quests via `offer <npc>` (`pkg/mudserver/game/commands/offer.go` + `quest_offer.go` over `llmgen.GenerateContextualQuest`); **B24** — FASE 6.5 DM narrator **mode A completed**: typed `DMEvent` bus (`def/gamectrl.go`), all five events (combatStart/combatEnd/questCompleted/levelUp/roomEnter), guardrails (cooldown per room/user, 8 s timeout, 10/min budget, content filter) and silence fallback; **B30** — FASE 6.5 DM **mode B completed** as opt-in: `game/dm_decision.go` (`DMDecision`, whitelist none/ambush/environmental/narrative, provider, per-room cooldown, timeout, per-minute budget, ambush count clamp 1..3, atomic metrics), `installDMDecision` in `pkg/server/llm.go`, kill switch `LLM_DM_DECISION_ENABLED` (off by default) and `NotifyDM` as the single entry point for `roomEnter`/`combatStart`; **B25** — FASE 6.3 NPC runtime dialogue **corte 1 completed**: in-game `ask` command (`pkg/mudserver/game/commands/ask.go`) over `Game.RequestNPCDialogue` (`pkg/mudserver/game/npc_dialogue.go`), provider `installNPCDialogueProvider` (`pkg/server/llm.go`, `TaskNPCDialogue` + `RequireLLM`) and the first game consumer of `npc_memories` (`loadNPCMemory`/`recordNPCMemoryTurn` over `NPCMemoriesService`); **B26** — FASE 6.3 **corte 2 completed** (QA-approved with minor observations, since fixed): `LLMConfig` extended with Big5 `Personality`, `Goals`, `Secrets` and `Relations` (`pkg/entities/npcs/llm.go`), all of them injected into the `npc.dialogue` prompt; LLM memory summaries (`TaskNPCMemorySummary`, async every `SummaryAtTurns` turns, protected from deterministic rebuilds via `NPCMemory.SummarySource`); config flows through the YAML importer (`importer.YAMLNPC.llm` with camelCase YAML tags + deep copy), the NPCTemplate REST DTOs (`handler/npc_templates.go`) and the NPC factory UI (tecla `N`); spawned instances inherit `template.LLM.Clone()`; replies (or option labels / memory notes) that reproduce a configured secret literally are rejected (`validateNPCDialoguePayloadWithSecrets`/`leaksSecret`) and fall back to the neutral line; `llm.secrets` is redacted from `GET /api/npc-templates/:id` for non-creators (`canViewNPCTemplateSecrets`, fail-closed); **B27** — FASE 6.3 **corte 3 completed** (QA-approved with observations): the model's `options[]` became **actionable** — a reply with options opens a real conversation under the sentinel `DialogID = "__llm__"` (`def.NPCDialogueDialogID`, `deliverNPCDialogue`/`sendNPCDialogueDialog` in `npc_dialogue.go`) and `DialogSelectCommand` routes numeric input to `handleLLMDialogSelection` (`commands/llm_dialog.go`): `end` closes, a label that matches an authored dialog node continues the tree through the new `talkToNPCAt` (`commands/talk.go`), anything else is a `flavor` follow-up scheduled with `Continuation=true` (skips the cooldown rejection, still honours the semaphore); **dynamic relations** — the payload gained `relationDelta` (per-exchange clamp ±0.05 via `npc.ClampRelationDelta`, default +0.02 unless `end`), persisted per `(npc, character)` in `NPCMemory.Relations` (cumulative clamp [-1,1] via `npc.ApplyRelationDelta`, no migration — it lives in the `npc_memories` JSON) and fed back into the prompt as `player(dinámica,<sentiment>)` (`mergeDynamicRelations`); **client feedback** — `public/mud-client/src/game/relationFeedback.js` renders a subtle ▲/▼ cue in `DialogOverlay.svelte` and bracketed lines in `Client.js`, only when the message actually carries relation info.
  - **Still pending (FASE 6):** an NPC short-term memory ring, complete combat guardrails and `FallbackDialogID` as an error fallback. Batch 29 implements talk auto-greeting, free-text interception and validated `kind=quest:<id>`; these options are no longer coerced to `flavor`. B30 implements DM mode B behind the off-by-default `LLM_DM_DECISION_ENABLED` switch. `FallbackDialogID` remains only a secondary source for authored-dialog label mapping; runtime error fallback is still the neutral line. B26 secret-read and leak-guard observations remain closed.
  - The existing `llm.Client` contract stays the only provider entry point; the design's additive extension (`llm.Options` / `CompleteWith`, implemented via `pkg/service/llmopts`) lifts the old fixed `max_tokens = 300` — 0 still means the provider default of 300
  - `docs/qa/REGRESSION_REPORT.md` now runs B14–B24 (2026-10-22); §B24 (FASE 6.5 DM narrator, mode A) is QA-verified (APROBADO). Batches 25 (FASE 6.3 corte 1) and 26 (corte 2, **QA CON OBSERVACIONES** — minor: `llm.secrets` readable via template GET for non-creators; anti-leak guard only inspects `reply` — both since fixed in the tree: secret redaction + guard extended to `options[].label`/`memoryNotes`) were QA-verified with dedicated tests but without their own report sections, and so was **27 (corte 3, QA APROBADO CON OBSERVACIONES on 2026-10-30** — one non-blocking observation: the pre-existing combat test `TestWardGritAndRetaliate` (`pkg/mudserver/game/combat/ward_test.go:111`, "tiny hit 2" — the same file holds the `TestWardGuard*` **guard tests**, which passed) failed once under `-race` and passed 10/10 in isolation, a random-crit flake unrelated to this batch, plus non-blocking frontend build warnings for unused CSS selectors and an `autofocus` A11y hint; 0 `DATA RACE`**)

### Technical Features

- **Authentication & Authorization**
  - Local username/password accounts (Argon2id) with JWT sessions are the fork default (`AUTH_MODE=local`, set by `config/gamemode.yaml`)
  - Auth0 OAuth2 remains available as an optional mode (`AUTH_MODE=auth0`); `AUTH0_AUDIENCE`, `AUTH0_DOMAIN` and `AUTH0_WK_JWKS` are only required in that mode
  - JWT-based API protection
  - Guest mode with HMAC-SHA256 tokens (no Auth0 required, works in either auth mode)
  - Auth middleware order: guest HMAC token, then server device session, then local session, then Auth0 RS256
  - Frontend session state avoids logging or retaining auth token excerpts
  - Basic auth for legacy admin endpoints (export/import), with explicit credentials required and insecure release defaults rejected
  - Session management
  - Three-tier role system: MUD Admin, MUD Creator, Player
  - MUD Admin configured via `MUD_ADMIN_OAUTHID` env var (has full access)
  - MUD Creators can modify game content (Creator area)
  - Players can play the game and access only their own characters and quest progress
  - User ban system (bans by Reference ID and email)

- **User Management (Admin Only)**
  - View all registered users with ID, Name, Nickname, Email, Access Level
  - Promote players to Creator role or demote Creators to Player
  - Ban/unban players with double-confirmation modal
  - Banned users are blocked from all authenticated endpoints

- **Optional Landing Page**
  - Serve a static landing page at `/` from the OS filesystem via `LANDING_PATH`
  - When disabled (default), the main app SPA is served at `/` as usual
  - Auth0 callbacks (`?code=` / `?error=`) pass through to the main SPA automatically
  - Static assets (images, CSS) in the landing directory are served alongside `index.html`

- **Data Persistence**
  - SQLite for all game data
  - World export/import functionality
  - YAML/JSON data file support
  - Optional per-character refilling resources (`character_resources`). Keys come from `config/ruleset.yaml`. The shipped file lists none, so a default server never spends a balance. Scripts read them through `tales.resources`.
  - Ruleset death penalties, level-up mode (`auto` or `trainer`), and an optional dawn heal. Defaults match the previous 10% XP loss, 1 gold, bind-point respawn, cap 50, and immediate level-up. A successful flee or slip is not a defeat: the escaper keeps hit points, gold, and the room they reached. A mixed party penalizes only the players who died.
  - Procedural private instances (`tales.instances.generate`): a per-character room line from templates, level-filtered encounters, cleanup on leave or timeout. Existing cellar instances and Party Follow are unchanged. Generated exits do not pull followers.
  - Optional text client at `/door` when `presentation: door_tui`. Classic mode does not mount that path. The page title, subtitle, and token key come from the game-mode file, with generic TalesMUD defaults. It paints rooms, exits, actions, NPCs, resources, combat status, and recent command replies on the same screen. The header shows experience beside gold. The fight log stays for the whole fight and clears on any key after it, including Enter. A bare extra attack on the victory screen is not sent. The status line stays pinned above the log. The stats key paints hit points, gold, worn gear, and a gems count. A compass letter uses an open exit before a menu bind, and open east and west exits are listed. The index heading uses the configured title. A one-shot reply belongs to the key that produced it. It sends normal commands. A repeated attack for a swing that is already queued does not print a second line. A pack `keymap.yaml` can bind keys per room or area. A new character picks a numbered path, then sex. Reconnect applies the new-day pass. `-config` can point a second process at its own port and database. Classic play is unchanged when no config is set.
  - Phase 4 social data: `clans`, `clan_members`, `clan_invites`, `clan_bank_ledger`, `dm_channels` and `dm_messages` tables are created by `migratePhase4SocialTables()`. `dm_messages` intentionally carries real `channel_id`/`sent_at`/`from_char_id` columns for indexed range queries. Exposed via `ClanRepository`/`DMRepository` and `Factory.Clans()`/`Factory.DMChannels()`.

## Technology Stack

### Backend
| Component | Technology |
|-----------|------------|
| Language | Go 1.18 |
| HTTP Framework | Gin |
| Database | SQLite |
| WebSocket | Gorilla WebSocket |
| Authentication | Local username/password (default) · Auth0 JWT (optional) |
| Scripting | Otto (JavaScript VM) |
| AI / LLM | Ollama (local daemon, default) · Groq API (optional) |
| Logging | Logrus |

### Frontend
| Component | Technology |
|-----------|------------|
| Framework | Svelte 3.59 |
| UI Library | Materialize CSS |
| Terminal | xterm.js |
| HTTP Client | Axios |
| Router | yrv |
| Build Tool | Rollup |

### Infrastructure
| Component | Technology |
|-----------|------------|
| Container | Docker |
| Orchestration | Docker Compose |
| CI/CD | GitHub Actions |
| Database | SQLite |

## Project Structure

```
talesmud/
├── cmd/                    # Application entry points
│   ├── tales/              # Main server
│   └── dialog_sandbox/     # Dialog testing tool
├── pkg/                    # Go packages
│   ├── entities/           # Data models (characters, rooms, items, NPCs, dialogs, clans, dms)
│   │   └── characters/     # character.go (+ derive.go derived stats, talents.go talent/prestige helpers)
│   ├── talents/            # Progression talent catalog (pure data: 12 talents, 3 categories)
│   ├── mudserver/          # Game server (WebSocket, game loop, commands)
│   │   └── game/commands/  # Command handlers (incl. talents.go, prestige.go, channels.go)
│   ├── server/             # HTTP API server
│   ├── service/            # Business logic layer
│   ├── repository/         # Data access layer
│   ├── db/                 # Database utilities (SQLite)
│   └── scripts/            # Script execution engine
├── public/                 # Frontend
│   └── app/
│       └── src/            # Svelte source
│           ├── game/       # Game client
│           ├── creator/    # Content editor
│           ├── characters/ # Character management
│           └── api/        # API clients
├── api/                    # API test files & sample data
├── data/                   # Sample game data
└── bin/                    # Compiled binaries
```

## Game Commands

| Command | Aliases | Description |
|---------|---------|-------------|
| `north`, `south`, `east`, `west` | `n`, `s`, `e`, `w` | Move between rooms |
| `look` | `l` | Examine current room |
| `inventory` | `i` | Display inventory |
| `selectcharacter` | `sc` | Select active character |
| `listcharacters` | `lc` | List your characters |
| `newcharacter` | `nc` | Create new character |
| `who` | - | List online players |
| `party create` | `p create` | Create a party with your current character |
| `party invite <player>` | `p invite <player>` | Invite an online player to a party |
| `party accept` / `party decline` | - | Respond to a pending party invite |
| `party list` | - | List party members |
| `party leave` | - | Leave the current party |
| `party kick <player>` | - | Remove a member from your party (leader only) |
| `party promote <player>` | - | Transfer leadership to a member (leader only) |
| `party follow` / `party unfollow` | - | Trail the party leader through normal exits |
| `party say <message>` / `party <message>` | `p say <message>` / `p <message>` | Send party chat |
| `party loot [rule]` | - | Show or set the loot rule: `free`, `leader`, `roundrobin`, `needgreed` (leader only) |
| `party readycheck` | - | Start a ready check (leader only, 30s window) |
| `party ready` / `party notready` | - | Answer the active ready check |
| `need` / `greed` / `pass` | `loot need\|greed\|pass` | Party Need/Greed loot roll (30s popup when 2+ eligible party members) |
| `scream` | - | Broadcast to room |
| `shrug` | - | Emote action |
| `chat <channel> <message>` | `general`, `lfg`, `roleplay`, `admin` (direct shortcuts) | Global chat: `general`\|`trade`\|`lfg`\|`help`\|`roleplay`\|`admin` |
| `help` | `h` | Show help |
| `attack` | `a`, `hit` | Attack a target / switch combat target |
| `defend` | `d`, `guard` | Queue defensive stance for next combat turn |
| `flee` | `run`, `escape` | Queue a flee attempt. Success ends as escaped, with no death penalty |
| `status` | `cs`, `combat` | Show combat status |
| `cast` | `spell` | Use a skill in combat: cast \<skill\> [target] |
| `skills` | `spells`, `abilities` | Manage skills: skills [equip\|unequip] [name] |
| `quests` | `ql`, `questlog` | Show quest log |
| `quest` | - | Show quest details: quest [name] |
| `abandon` | - | Abandon a quest: abandon [name] |
| `offer <npc>` | - | Ask a quest-giver NPC for a contextual quest (level + current area; async LLM generation, 60 s per-character cooldown, deterministic fallback) |
| `offer accept` / `offer decline` | - | Accept (persists the quest, emits `questAccepted` + `questLog`) or decline the pending offer |
| `ask <npc> [question]` | - | Ask an NPC a free-form question answered by the LLM (`llm.enabled` NPCs only; per character+NPC cooldown, max 2 concurrent, 20 s timeout, runs off the game loop; plus the pipeline rate limit `LLM_RATE_PER_HOUR`, 10/h per npc+character → neutral line when exhausted). The prompt carries the NPC persona/voice plus Big5 personality, goals, secrets (instructed as internal — replies reproducing a secret literally are rejected) and relations; memory persists an LLM-written summary (with deterministic fallback). NPCs without LLM (or no provider installed) degrade to the authored `talk` dialog with a notice; persistent per-character memory in `npc_memories` |
| `guide` | - | Dismiss the next-step hint: guide dismiss |
| `spend` | - | Spend attribute points: spend \<attr\> [amount] |
| `talents` | `talent` | Show unspent talent points and the class talent list (`talents list`) |
| `talents spend <talent>` | - | Buy one rank of a talent by id, name, or unique prefix (blocked during combat) |
| `prestige` | - | Show prestige status (available at level 50) |
| `prestige confirm` | - | Irreversible reset to level 1 for +1 prestige (+5% XP) and +1 talent point |
| `pickup` | `get`, `take` | Pick up an item from the room (room-placed catalog items copy per character and stay for the next guest; dropped loot is still taken). The play client opens one pickup panel for desktop and phone: each ground row is tappable, 1–9 and Enter take a row or everything, and the panel closes when the ground is clear. Ground rows include optional quality, stack, type, and one effect line. Play cache tag `curve1`. |
| `drop` | - | Drop an item to the room (blocked for bound items) |
| `destroy` | `discard` | Destroy an item from inventory |
| `examine` | `inspect` | Examine an item in detail |
| `use` | `eat`, `drink`, `consume` | Use an item (`use flint on torch` for item-on-item); Inventory Use on… picks a target |
| `equip` | `wear` | Equip an item |
| `unequip` | `remove` | Unequip an item |
| `equipment` | `eq`, `gear` | Show equipped items |
| `list` | `shop`, `trade` | List merchant inventory |
| `buy` | - | Buy from merchant; stackable quantities fit in a single stack when possible |
| `sell` | - | Sell to merchant (blocked for bound items and unsupported item types) |
| `value` | `price` | Check item sell price |

## Phase Status

Status of the fork roadmap (code verified against `docs/qa/REGRESSION_REPORT.md`):

| Phase | Scope | Status |
|-------|-------|--------|
| **2.3** | Derived attributes — `pkg/entities/characters/derive.go`, `DerivedStats` payload, combat `CombatantRef` | **Done** (implemented + tested) |
| **2.4** | Progression — XP curve, talent milestones, `pkg/talents` catalog, `talents` / `prestige` commands, XP multiplier | **Done** (implemented + tested; design in `docs/design/PHASE2_4_PROGRESSION.md`) |
| **3.2** | Client command history + Tab completion (`commandHistory.js`) | **Done** |
| **3.4** | Minimap exit legend (`minimapDirections.js`) | **Done** |
| **3.7** | Responsive play shell breakpoints | **Done** |
| **4** | Social systems — clans, DMs, global channels | **Done** |
| **5** | Dungeons & raid content pack (`import/adventure/`) | **Done** (health + reachability tests green) |
| **5.1** | Dungeon Builder — REST `/api/dungeons` + client panel (key `D`) | **Partial (Batch 19)** — design library persists (localStorage + best-effort creator sync); **no in-game spawn/instances** yet, no compile/run endpoint (see `docs/API.md` §5) |
| **5.3** | NPC/mob template scaling — `npcBehavior.js` (`scaleTemplate`) + `npc_templates` service/REST/spawn | **Done (B30)** — client scaling/preview wired into the NPC factory (key `N`); `npc_templates` has service + creator REST (`/api/npc-templates`) and best-effort live spawn on create/update plus `POST /api/npc-templates/:id/spawn` |
| **6** | LLM integration (`llmprompt` / `llmgen`, NPC LLM, quest gen, DM AI) | **En curso** — cut A, Creator LLM, quests contextuales, FASE 6.5 modos A/B y FASE 6.3 cortes 1–3 + mecanismos B/C (Batches 25–29) cableados; modo B opt-in (B30, apagado por defecto); pendiente anillo de memoria corta, guardrails completos de combate y `FallbackDialogID` como fallback de error; diseño en `docs/design/PHASE6_LLM.md` |
| **7.1** | Load testing (FASE 7.1) | **Closed — QA-verified in B21** — 100 connections healthy (heartbeat verified); ≥200 connections used to deadlock the game loop permanently (P0 in B19), **fixed in Batch 20 (Backend)** and **re-verified by QA in B21**: 200/300/500 workers × 2 runs × 80 s, `recv == sent`, **0 `msgErrors`**, no wedge; see `docs/qa/REGRESSION_REPORT.md` §B19 (history) and §B21 (closure) |
| **7.2** | Security (FASE 7.2) | **Audit PASS** (§B16, no high findings) + hardening in the tree (global security headers, HTTP timeouts, bounded JWKS fetch); medium finding on the loopback `X-Forwarded-For` rate-limit bypass (§B19.4) |
| **7.3** | Balance (FASE 7.3) | **Audited, read-only** — B17 analysis in `docs/qa/BALANCE_ANALYSIS.md` + B18 combat sims; no balance changes applied |

Regression verification: backend build + full Go suite and frontend build green on every code batch (last QA-verified run: 2026-10-18, Batch 19, `go build`/`go vet`/targeted `go test` EXIT 0). Batches 20–27: **20 Backend** (2026-10-19, P0 fix in `pkg/mudserver/**` + optional `cmd/tales/pprof.go`), **21 QA** (load re-verification, §B21: P0 closed), **22 Backend+Docs** (build/`.gitignore` fix + `writeLoop` leak fix + docs sync), **23 Backend+QA+Docs** (2026-10-21, presence race fix + QA verification §B23 + docs sync), **24 Backend+QA+Docs** (2026-10-22, FASE 6.5 DM narrator mode A + QA verification §B24 + docs sync), **25 Backend+QA+Docs** (2026-10-22, FASE 6.3 corte 1 — comando `ask` + memoria NPC en `npc_memories` + QA verification + docs sync) and **26 Backend+QA+Frontend+Docs** (2026-10-26, FASE 6.3 corte 2 — Big5/goals/secrets/relations, resumen LLM con `SummarySource`, config YAML/NPCTemplate REST, spawn hereda `llm`, guard anti-fuga de secretos; QA **CON OBSERVACIONES** — puntos menores: `llm.secrets` legible vía `GET /api/npc-templates/:id` para usuarios autenticados no-creators y el guard anti-fuga solo inspecciona `reply`, no `options[].label`/`memoryNotes` — **ambas observaciones cerradas en el árbol**: redacción de secretos para no-creators (`canViewNPCTemplateSecrets`, fail-closed) y guard extendido a `reply`/`options[].label`/`memoryNotes`, con tests; sin sección propia en `docs/qa/REGRESSION_REPORT.md`) and **27 Backend+QA+Frontend+Docs** (2026-10-30, FASE 6.3 corte 3 — LLM `options[]` accionables sobre la conversación centinela `__llm__` (`commands/llm_dialog.go` + routing en `dialog_select.go`, `end`/nodo autoral vía `talkToNPCAt`/`flavor` con `Continuation`), relaciones dinámicas `relationDelta` (±0.05 por intercambio, acumulado [-1,1] en `NPCMemory.Relations`, prompt `player(dinámica,x.xx)`) y feedback sutil de relación en el cliente (`relationFeedback.js`/`DialogOverlay.svelte`/`Client.js`); QA **APROBADO CON OBSERVACIONES** — una sola observación no bloqueante: test de combate preexistente `TestWardGritAndRetaliate` (`ward_test.go:111`, "tiny hit 2"; los tests de guardia `TestWardGuard*` del mismo fichero pasan) intermitente en la primera corrida con `-race` y 10/10 en aislamiento, ajeno al batch; 0 `DATA RACE`, sin regresión de `ask`/`talk`; sin sección propia en `docs/qa/REGRESSION_REPORT.md`). The load/security/balance audits live in `docs/qa/REGRESSION_REPORT.md` (B14–B24): historical verdict **PASS with conditions**; the **P0 load blocker is closed — QA-verified in B21** and the **presence race is closed in Batch 23 — QA-verified in §B23** — see "P0 de carga" below.

**Operational note (resolved in Batch 22):** `.gitignore` used to ignore every path with a `tales` component, which hid `cmd/tales/pprof.go` and `cmd/tales/pprof_test.go` from plain `git add` (a fresh clone then failed with `undefined: startPprof`). The pattern is now `/tales` (root binary only), so the pprof files stage normally — no `git add -f` needed. Related: the Makefile builds by package (`go build -buildvcs=false -o bin/tales ./cmd/tales`), which is what pulls `pprof.go` into the binary.

## Current Development Status

Fork development happens on `master`; batches 14–25 live mostly in the **working tree (uncommitted)**. Feature detail: `FEATURES.md`. HTTP/WS contract: `docs/API.md`. QA verdicts: `docs/qa/REGRESSION_REPORT.md`.

### NPC systems and dialog (consolidated)

1. **NPC entity and traits** — NPC data mirrors player characters with trait composition (Dialog, Merchant, Enemy), room presence tracking, health/level/class, plus `npc_templates` (FASE 5.3, table + repo, pending wiring).
2. **Behavior and movement loop** — idle wander (`WanderRadius`), deterministic patrol routes, roam/route movement with engagement holds, idle chatter cooldowns, respawn cleanup, `aggroOnSight`; runtime clocks are memory-only.
3. **Dialog engine** — branching trees with state, template variables, conditional/ordered options, show-once options, quest-only conversations for NPCs without a tree, dialog sandbox for testing.
4. **Auto-attack combat** — initiative rounds with queued actions (target switch, defend, flee), level-gap/class/boss modifiers from `config/combat_balance.yaml`, threat colors, victory/defeat panels, enemy Lua hooks (`onAggro`/`onDeath`/`onFlee`/`onLowHealth`).
5. **Quest interaction** — `talk`/`speak` inject offer/progress/turn-in options; the `offer` command (Batch 18) generates a contextual quest from level + area with a 60 s cooldown and deterministic fallback.
6. **Client tooling** — NPC factory panel (key `N`, `npcFactory.js`, localStorage + best-effort `/api/npcs` sync) and `npcBehavior.js` scaling (`scaleTemplate`, behavior model → `toLuaBehavior()` preview only).
7. **NPC runtime LLM (FASE 6.3, Batches 25–29)** — `ask <npc> [question]`, persistent per-character memory, structured personality, actionable options and relation feedback. Batch 29 completes mechanisms B/C: `talk` applies authored dialog → available quests → LLM greeting priority; unrecognized free text continues an eligible, recent `__llm__` conversation; `quest:<id>` options are validated against the current quest allowlist and routed through the existing quest handler. Prompt `npc.dialogue` is v1.0.3 and the dialog contract includes `Kind`/`FreeText`. Pending: the short-term memory ring, complete combat guardrails, and `FallbackDialogID` as an error fallback; FASE 6.5 mode B is also pending.

Detail: `FEATURES.md` (secciones NPC System, Combat System, Quest System), `docs/user/admin-guide.md` y `docs/user/commands.md`.
1. **NPC Entity System**
   - Core NPC data structure mirroring player characters
   - Trait-based composition (DialogTrait, MerchantTrait, EnemyTrait)
   - Room integration with NPC presence tracking
   - Health, level, and class systems
   - Runtime NPC behavior loop for idle, patrol, dead/respawn, and combat states
   - Deterministic patrol paths and bounded wandering from spawn rooms

2. **Dialog Engine**
   - Full dialog tree system with branching conversations
   - State management tracking visited dialogs
   - Template rendering with dynamic variables ({{PLAYER}}, {{NPC}}, {{TIME}})
   - Conditional option display based on conversation history
   - Alternate text variations for natural dialogue
   - Ordered responses (different text on repeated visits)
   - Dialog sandbox for testing conversations

3. **Dialog Features**
   - Interactive dialogs (triggered by player interaction)
   - Idle dialogs (ambient NPC chatter with timeout)
   - Show-once options
   - Dialog exit markers
   - YAML serialization for dialog definitions

4. **Auto-Attack Combat System**
   - Automatic combat rounds (players and NPCs auto-attack each turn)
   - Turn-order initiative system with auto-processing
   - Players can queue special actions between auto-attacks: target switch, defend, flee
   - Combat starts with `attack`/`kill` and proceeds automatically. Enemies with `aggroOnSight` start the same way after `combat.aggro_on_sight` grace (default 2.5s, gap 5, 15s reaggro cooldown; `enabled: false` turns it off). The sight line is ordinary text, then one fight, including swarm pack and the party assist nudge. A room with `noAggro` does not schedule or start that fight. The play client shows a red Aggressive label on an `aggroOnSight` enemy card and on look or examine. The client bundle and stylesheet use cache tag `aggro1`. Sanctuary flags survive import/export and private room cloning.
   - A spawner replaces a corpse only after `respawnDelay`, and at least `combat.respawn_min_seconds` (default 30). Server start still fills the room immediately. A fresh respawn waits `respawn_grace_seconds` (default 8) before it engages players already there. After a victory or flee, `post_combat_grace_seconds` (default 10) holds aggressive enemies in that room, and walking out ends the hold. The next room keeps the short grace. `spawn_aggro_cooldown_seconds` (default 60) stops that spawn from auto-aggroing the same player again. Another player is still engaged on sight. `attack` still starts a fight during that window. Movement stays open once the fight has ended.
   - Level-gap modifiers (`config/combat_balance.yaml` `level_gap`): hit, crit, and damage dealt/taken scale with attacker level minus defender level, clamped (default ±6). Equal levels are unchanged. Applies to basic attacks and skills for players and NPCs.
   - Class balance (`class_balance`): per-class damage dealt and taken, plus an uphill `behind_dealt` multiplier capped at 1.15. The world-pack class catalog wins when it has a row. `config/combat_balance.yaml` is the fallback. Ward takes hits at 1.00. A soak class's Slam starts its cooldown only when the swing hits. `wizard` uses the mage row. Ranger and hunter share the rogue row. `early_normal` shortens normal-tier enemies at level 6 and below (an authored 18 HP Sample Thug lives at 20 HP, not 48). `newbie_curve` adds level 1–5 class factors and a +2 damage-taken multiplier for those same early attackers. Level 10 and hard/boss tiers stay on the old table.
   - Boss mechanics (`boss_mechanics`): bosses and elites telegraph blows; bosses enrage on a round count or HP percent. Bosses also progress through Opening, Escalation (66% HP), and Last Stand (33% HP), with a transition banner and persistent BattleStage phase label. YAML controls tiers, bands, labels, and optional damage/enrage overrides.
   - Threat colors from the same gap (grey through skull) on room enemy names and battle nameplates. Orange or worse asks once before `attack`; `attack!` or a second `attack` engages.
   - Victory XP and gold scale by that tier against the highest level in the reward split. Bosses pay a one-time first-kill bonus per character. The battle victory panel shows base, level modifier, and a first-kill bonus when one was paid, then reveals each drop in its rarity color and calls out a level-up. Defeat lists XP, gold, battered armor, and the room you wake in. The panel dismisses on click, Enter, or Escape and leaves the terminal usable. Hits float a number over the struck nameplate (a crit is larger, a miss reads "miss"); crits and Crushing Blow flash harder. `prefers-reduced-motion` turns those animations off.
   - No turn timeouts or AFK mechanics needed
   - Enemy `attackSpeed` is attacks per round. 0 or omitted keeps one swing on the old beat. `onAggroScript`, `onDeathScript`, `onFleeScript`, and `onLowHealthScript` run once per fight in the sandboxed Lua runner. Low health defaults to 30% of max HP and does not run when the blow kills. A script can heal an ally, apply an existing buff or debuff, or `tales.combat.summon` up to 3 enemy-template adds per fight. Adds match template stats, grant no loot or XP, and leave when the fight ends. A death or flee drops that combatant from the turn order and keeps the turn index in range, so the fight continues and ends promptly when the last enemy dies. Hook room lines stay ordinary system messages and add `style` `combatEvent`, `hook`, and `source`. A summon during a hook flush sends the live roster on `combatStatus` at once. The play client shows those lines as a gold combat-log chip (`?v=uniques1`). A `rarity: unique` drop uses the same chip with hook `unique`.

5. **NPC Behavior and Quest Interaction**
   - NPC update loop handles idle wandering, ordered patrol paths, respawn cleanup, and idle chatter cooldowns
   - `talk` and `speak` open NPC dialogs and inject quest offer/progress/turn-in options
   - Quest-giving NPCs without a main dialog open quest-only conversations for numbered quest choices
   - MUD client NPC cards show enemy, merchant, quest giver, dialog, idle chatter, and current state badges
   - Creator NPC editor exposes behavior controls and uses modal entity selectors for patrol and room references

### Recent Commits (NPCs Branch)

| Commit | Description |
|--------|-------------|
| `b17856d` | Fixed Svelte issues |
| `6b621a2` | Huge improvements on player and NPC interaction |
| `29674d5` | New work on Dialogs |
| `b54c92e` | Further work on dialogs |
| `b5ae2c3` | More progress on dialog logic |

## Configuration

### Environment Variables (.env)

```bash
# Server Configuration
GIN_MODE=debug
PORT=8010

# Optional comma-separated CORS origins in addition to the built-in production
# domains and local dev origins for the Creator and MUD clients.
CORS_ALLOWED_ORIGINS=

# SQLite database path
SQLITE_PATH=./talesmud.db

# Authentication
# local (fork default) | auth0
AUTH_MODE=local

# Auth0 (only read when AUTH_MODE=auth0; do not commit production tenant values)
AUTH0_AUDIENCE=http://localhost:8010/api
AUTH0_DOMAIN=https://your-tenant.auth0.com/
AUTH0_WK_JWKS=https://your-tenant.auth0.com/.well-known/jwks.json
TRUSTED_PROXIES=127.0.0.1,::1
TALESMUD_ALLOW_JS_SCRIPTS=

# Admin (basic auth for legacy export/import)
# Leave either value blank to disable these endpoints.
# admin/admin is rejected in release mode.
ADMIN_USER=admin
ADMIN_PASSWORD=changeme

# MUD Admin OAuth ID (Auth0 sub claim, e.g. "twitter|16651340")
# The user with this OAuth ID gets full admin access
MUD_ADMIN_OAUTHID=

# Guest mode secret key for signing guest JWTs (HMAC-SHA256)
# If not set, a random key is generated at startup (guest tokens won't survive server restart)
GUEST_SECRET=

# Confirmed public device logins use signed 30-minute account sessions.
# SESSION_SECRET wins over game-mode session_secret / secret_path.
# If unset, a persistent key is generated at data/session.key (0600).
SESSION_SECRET=

# SSH listener (also settable in the game-mode ssh: block). All default off.
# These enable the listener and set its public address. They do not name a user or a key.
# SSH_ENABLED=false
# SSH_LISTEN=127.0.0.1:2222
# SSH_HOST_KEY_PATH=/var/lib/talesmud/ssh/host_ed25519
# SSH_PUBLIC_HOST=mud.example.com
# SSH_PUBLIC_PORT=2222
# SSH_ACTIVATE_URL=https://mud.example.com/activate
# Also required for public app device login; points at the existing /activate.
# SSH_GUEST_ENABLED=false
# SSH_DEVICE_ENABLED=false
# Device codes also stop at max_pending (default 100) for the process.
# Per-address caps bucket IPv6 as a /64. See docs/ssh-access.md.

# Creator live badge. Empty locally. Set ADMIN_ENV_LABEL=LIVE on a production admin.
# ADMIN_ENV_HOST overrides the request host shown after the label.
ADMIN_ENV_LABEL=
ADMIN_ENV_HOST=

# Optional landing page (path to directory with index.html + static assets)
# LANDING_PATH=./public/landing

# AI Generation — character name/description generation via a
# swappable LLM provider (POST /api/generate/character)
# ollama (default, local daemon, no API key) | groq | none/off
LLM_PROVIDER=ollama
# Local Ollama daemon, OpenAI-compatible endpoint (ollama provider)
OLLAMA_BASE_URL=http://localhost:11434
OLLAMA_MODEL=llama3.1
LLM_TIMEOUT_SECONDS=60
# Groq API key (https://console.groq.com) — only used by the groq provider
GROQ_API_KEY=

# Dungeon Master narrator (FASE 6.5, modo A) — solo prosa, nunca muta estado.
# Sin proveedor el DM se queda en silencio (fallback = silencio).
LLM_DM_ENABLED=1
LLM_DM_COOLDOWN_SECONDS=45
LLM_DM_TIMEOUT_SECONDS=8
LLM_DM_MAX_PER_MIN=10

# Dungeon Master decisions (FASE 6.5, modo B) — opt-in, apagado por defecto.
# Una decisión validada por roomEnter/combatStart (ambush/environmental/narrative).
LLM_DM_DECISION_ENABLED=0
LLM_DM_DECISION_COOLDOWN_SECONDS=60
LLM_DM_DECISION_TIMEOUT_SECONDS=8
LLM_DM_DECISION_MAX_PER_MIN=10
```

## Building & Running

### Prerequisites
- Go 1.26+ (toolchain go1.26.9)
- Node.js (for frontend build)

### Build Commands

```bash
# Build everything
make build

# Build frontend only
make build-frontend

# Build backend only
make build-backend

# Build the dialog sandbox only
make build-dialogs-sandbox

# Backend-only builds and server runs prepare fallback embedded frontend assets
# if pkg/webui/dist or pkg/webuiplay/dist has not been generated yet. Run
# `make build` to embed freshly rebuilt frontend bundles.

# Run the server. When no -config flag is passed and config/gamemode.yaml
# exists, it is loaded automatically (fork default: local auth, port 8010).
make run-server

# Explicitly select a game-mode file
./bin/tales -config config/gamemode.yaml

# Run a second process with its own port and database: create a separate
# game-mode YAML (do not reuse config/gamemode.yaml) and pass it with -config.
# Example: a door_tui + auth local server on port 8030.

# Forwarded client IPs are trusted only from loopback unless this is set
# TRUSTED_PROXIES=127.0.0.1,::1

# Run the server with SQLite (single binary + embedded frontend)
DB_DRIVER=sqlite SQLITE_PATH=./talesmud.db ./bin/tales

# Run frontend dev server
make run-frontend

# Run dialog sandbox
make run-dialogs-sandbox
```

**How the Makefile builds (Batch 22):** the Go targets compile **by package**, never as loose files — `build`/`build-backend` run `go build -buildvcs=false -o bin/tales ./cmd/tales`, `build-dialogs-sandbox` runs `go build -buildvcs=false -o bin/dialog_sandbox ./cmd/dialog_sandbox`, and `run-server`/`run`/`dev` run `go run ./cmd/tales`. Compiling `cmd/tales/main.go` alone is wrong since Batch 20 added `cmd/tales/pprof.go` (`undefined: startPprof`). `-buildvcs=false` is required wherever the VCS stamp is unavailable (Docker bind mounts, containers without `.git`, root-owned checkouts); drop it only in a normal clone if you want the version stamp.

### Docker Deployment

The container image bundles the default `config/gamemode.yaml`, so the binary starts in local-auth mode with no Auth0 values. Auth0 variables are optional and only used when `AUTH_MODE=auth0`.

```bash
# Start the server (local auth by default, no Auth0 tenant required)
docker-compose up -d

# Optional local LLM sidecar. talesmud does not depend on it and does not
# start it unless the profile is enabled.
docker-compose --profile ollama up -d

# To keep Auth0 login instead: set AUTH_MODE=auth0 and the AUTH0_* values.
```

The `talesmud` service healthcheck probes `GET /health` on port 8010. The optional `ollama` service exposes `11434` and stores models in the `ollama-models` volume; when it runs, set `OLLAMA_BASE_URL=http://ollama:11434` (the compose default) so the backend reaches it by container name.

### Data Import

Import world data into SQLite:

```bash
go run cmd/migrate/main.go -input export.json -sqlite talesmud.db
```

## API Endpoints

> Resumen ejecutivo. El contrato completo y actualizado (paths, códigos, payloads y protocolo WS) vive en **`docs/API.md`**; si esta lista difiere, manda `docs/API.md`.

### Public Endpoints
- `GET /health` - Health check
- `GET /api/templates/characters` - Character creation templates
- `GET /api/classes` - Class catalog; a creator/admin Bearer token receives the full editor DTO
- `GET /api/room-of-the-day` - Featured room
- `POST /api/guest` - Create guest session (returns HMAC token)
- `GET /api/server-info` - Public server info: `serverName`, `envLabel` (`ADMIN_ENV_LABEL`), `host` (`ADMIN_ENV_HOST` or the request host), and `defaultTheme` (ruleset `ui.default_theme`, falling back to `dark-fantasy`)
- `POST /api/auth/register`, `POST /api/auth/login` - Local username/password accounts (mounted only when `auth: local`; forgot/reset are added when reset mail is configured)
- `GET /api/ssh/info` - SSH listener status. `{enabled:false}` when SSH is off. When it is on, the body includes host, port, full host-key fingerprints, guest flag, and activate URL
- `GET /activate` - Device-code page. Local auth serves a sign-in form. Classic auth redirects to `/play/?activate=<code>`

### Protected Endpoints (Require Auth - Player Level)
- `GET /api/auth/me` - Current local account (mounted only when `auth: local`)
- `GET /api/characters`, `POST /api/newcharacter` - Character management; direct character object access is owner/admin only
- `POST /api/generate/character` - AI-powered character name/description generation
- `GET /api/rooms`, `GET /api/items`, `GET /api/skills` - Read game data
- `GET /api/dungeons`, `GET /api/dungeons/:id` - Dungeon designs: summary list (`?name=&author=&limit=`) and detail with the opaque `definition` (see `docs/API.md` §5)
- `GET /api/user`, `PUT /api/user` - User profile. A guest may read it. PUT returns 403 and leaves name, email, nickname, picture, role, and the guest flag unchanged
- `GET/POST /api/ssh/keys`, `DELETE /api/ssh/keys/:id` - List and revoke SSH public keys linked from an SSH sign-in. POST does not activate a pasted key. Guests and banned accounts are refused. 404 while the key store is off
- `POST /api/ssh/device/lookup|confirm|deny` - Confirm an SSH device code. Lookup returns a csrf nonce. Confirm and deny send it back. Nothing is confirmed automatically

### Protected Endpoints (Player Level - Quests)
- `GET /api/quest-progress/:characterId` - Get character quest log for own/admin character
- `POST /api/quest-progress/:characterId/accept/:questId` - Accept quest for own/admin character
- `POST /api/quest-progress/:characterId/abandon/:questId` - Abandon quest for own/admin character
- `POST /api/quest-progress/:characterId/complete/:questId` - Complete a ready quest for own/admin character

### Protected Endpoints (Player Level - Social: Clans & DMs)
The acting character is passed in the request (query string for `GET`, JSON body for `POST`) and must belong to the authenticated user (admins may act on any character).
- `POST /api/clans` - Create a clan (`characterId`, `name`, `tag`, `description`); fails if the character is already in one
- `GET /api/clans/mine?characterId=` - Current clan + members + pending invites for the character
- `POST /api/clans/:id/invite` - Invite a character (leader/officer only; no duplicate pending invites)
- `POST /api/clans/invites/:inviteId/accept` - Accept an invite (joins as `member`)
- `POST /api/clans/invites/:inviteId/decline` - Decline an invite
- `POST /api/clans/:id/kick` - Remove a member (leader/officer; the leader cannot be kicked)
- `POST /api/clans/:id/rank` - Change a member rank (leader only; `member`/`officer`/`leader`)
- `POST /api/clans/:id/deposit` - Deposit gold into the clan bank (any member)
- `POST /api/clans/:id/withdraw` - Withdraw gold from the clan bank (leader only)
- `POST /api/clans/:id/disband` - Dissolve the clan and remove its members (leader only)
- `GET /api/dm/channels?characterId=` - DM channels for the character
- `POST /api/dm/channels` - Find-or-create the canonical channel with a target character
- `GET /api/dm/channels/:channelId/messages?characterId=&limit=&since=` - Messages (participants only; `since` is RFC3339)
- `POST /api/dm/channels/:channelId/messages` - Send a message (participants only; anti-spoof)
- `POST /api/dm/channels/:channelId/read` - Mark received messages read (participants only)

### Creator Endpoints (Require Creator or Admin Role)
- `POST/PUT/DELETE /api/rooms` - Room management
- `GET /api/rooms/:id/inspect` - Static room inspector (creator or admin). An instance-copy id resolves to its template. Live rows stay on the live endpoints. Reachability uses the world index.
- `POST/PUT/DELETE /api/items` - Item management
- `POST/PUT/DELETE /api/scripts` - Script management
- `POST/PUT/DELETE /api/npcs` - NPC management (`GET /api/npcs/templates` lists NPC templates). When `enemyTrait.baseStats` is set, create and update recompute effective HP, attack, and defense from the content base and the combat balance table.
- `GET /api/npcs/:id/inspect` - Static NPC inspector (creator or admin). An instance id resolves to its template. Live rows stay on `GET /api/live/npcs`.
- `GET /api/balance/enemy-scaling` - Read-only difficulty tiers and named overrides (`hp`, `attack`, `defense`)
- `POST/PUT/DELETE /api/dialogs` - Dialog management
- `GET /api/quests`, `GET /api/quests/:id` - Quest definitions (read)
- `POST/PUT/DELETE /api/quests` - Quest management
- `GET /api/quests/:id/debug` - Quest debugger: step chain, where each objective can be satisfied, reachability, prerequisite graph, offer and turn-in, rewards, and character progress. Each character row includes the open `objectiveId`. `ops.questStep.implemented` is true. The Creator page follows `?id=` on load and on navigation. An admin resets that open step with `POST /api/ops/quest-step`.
- `POST/PUT/DELETE /api/skills` - Skill management (`/api/spells` is an alias for the same CRUD)
- `GET /api/classes/:id`, `POST/PUT/DELETE /api/classes`, `POST /api/classes/:id/duplicate` - Class-kit YAML management; `GET /api/classes` returns the full balance DTO for creators and the public catalog otherwise
- `POST/PUT/DELETE /api/dungeons` - Dungeon designs (`docs/API.md` §5; 201/200/204)
- `POST /api/creator/llm/quests/generate`, `POST /api/creator/llm/quests/contextual`, `POST /api/creator/llm/rooms/:id/description`, `GET/POST /api/creator/llm/prompts` - LLM authoring (`docs/API.md` §6)
- `GET /api/world/validation` - World Health diagnostics
- `GET /api/diagnostics/world` - World health diagnostics across rooms, NPCs, dialogs, quests, loot, items, and scripts
- `GET /api/world/reachability?from=` - World-map reachability from the start room, or from `from` when that room exists. Same walk as content health. Unknown `from` is 404.
- `GET /api/world/overlays` - World-map layers for a creator or admin. A room is included when it has a level band, an aggressive enemy, a spawner, an online player, a quest target, missing art, or a live instance copy. Level band is the NPC level range in the room, or the zone range when the room has none. The band is the midpoint: 0–4, 5–9, 10–14, 15–19, 20–29, 30+. Aggressive count is each `aggroOnSight` resident plus each aggro spawner's max instances. Spawners include the respawn time (override, then delay, then the template). Quest ids are rooms named by a visit, kill, talk, or deliver objective. Missing art is an empty background image. Copies is the number of live `~` rooms for that template. `playerNames` is present only for an admin; a creator still gets `players`. `live` is false when the game is not running, and the static layers still return.
- `GET /api/search?q=&types=&limit=` - Creator search over rooms, NPCs, items, dialogs (including node text), quests, scripts (name and body), loot tables, spawners, skills, and character templates. Exact id ranks above an id prefix, then an id fragment, then a name, then other text. Each hit includes `type`, `id`, `name`, a snippet, and a Creator path. Loot tables open `/creator/loot-tables?id=`. Spawners open `/creator/spawners?id=`. `limit` defaults to 25 and caps at 100. A blank query returns no hits.
- `POST /api/loot-tables/:id/roll?n=&seed=&playerLevel=&boss=` - Creator or admin. Seeded drop-frequency preview for one loot table. `n` defaults to 1000 and caps at 10000. It does not create items, change the table, or write an audit row. `POST /api/loottables/:id/roll` remains the single roll that creates item instances.
- `GET /api/refs/:type/:id` - Inbound and outbound references for one entity, grouped by type, with the field and a short reason. Unknown type is 400. A missing id is 404. The index is cached until a successful creator POST, PUT, PATCH, or DELETE.
- `GET /api/health` - Content-health report (summary, rules, live anomalies)
- `PUT /api/health/mute` - Mute or unmute a content-health rule (`{"ruleId","muted"}`)
- `GET /api/health/drift` - Entities changed since the last import baseline. The Creator Drift page lists them with before/after field values.
- `GET /api/health/drift/export?type=&id=` - Importer-format YAML for one drifted entity. Export all on that page concatenates one response per added or changed entity.
- `POST /api/validate/:entityType` - Validate a draft Creator entity before save
- `POST /api/preview/dialog`, `/api/preview/quest`, `/api/preview/room`, `/api/preview/merchant` - Preview/test draft content with validation issues
- `PUT /api/settings` - Server settings
- `GET /api/audit?entityType=&entityId=&limit=` - Audit log of creator writes and live ops
- `GET /api/live/npcs?templateId=&roomId=` - Running NPC instances, including room name, who they are fighting, and `deadUntil` when a corpse has a respawn time
- `GET /api/live/instances` - Instance room copies

### Admin API Endpoints (Require Admin Role)
- `GET /api/admin/users` - List all users
- `PUT /api/admin/users/:id/role` - Change user role
- `POST /api/admin/users/:id/ban` - Ban user
- `POST /api/admin/users/:id/unban` - Unban user
- `GET /api/live/characters` - Online characters and anyone seen in the last 30 days (`online`, `guest`, `inCombat`, `zone`, `all=1`). A creator receives 403.
- `GET /api/live/characters/:id` - Inventory, equipment, quest log, revealed exits, and current combat. A creator receives 403.
- `POST /api/audit/:id/undo` - Restore a creator write, or run a live op's inverse. Refuses when the row is not undoable, already undone, or the entity changed.
- `POST /api/ops/:action` - Live op on the game command loop. Body must include `confirm: true`. Actions: `teleport`, `give-item`, `take-item`, `npc-heal`, `npc-respawn`, `npc-despawn`, `end-combat`, `instance-cleanup`, `quest-step`, `regrant-starter-kit`.

### Legacy Admin Endpoints (Basic Auth)
- `GET /admin/export` - Export world data; requires explicit `ADMIN_USER` and `ADMIN_PASSWORD`
- `POST /admin/import` - Import world data; validates JSON before replacing stored data
- `GET /admin/world` - World map rendering

### WebSocket
- `GET /ws` - Game connection (authenticated)

## File Statistics

Approximate counts of the working tree (2026-10-19):

| Category | Count |
|----------|-------|
| Go source files (`pkg/`, `cmd/`, excl. tests) | 453 |
| Go test files | 264 |
| Svelte components (`public/`) | 436 |
| JavaScript modules (`*/src/`) | 443 |
| Backend code (excl. tests) | ~2.5 MB |
| Frontend code (`*.js` + `*.svelte` in `*/src/`) | ~3.4 MB |

## Adventure Content Pack

`import/adventure/` — importable world extension (YAML, `pkg/importer` format). Adds 4 zones on top of the R0001 hub: **The Training Grounds** (6 rooms, lvl 1–3, mentor NPC + practice dummies), **The Buried Vault** (13 rooms, lvl 6–11, 3 floors, 3 traps, 3 mini-bosses + Hollow Warden), **Frostspine Caverns** (12 rooms, lvl 12–17, 3 mini-bosses + Frostspine Matriarch), and **The Elder Maw** (4 rooms, lvl 20–25, raid boss for parties of 3–5). Includes 24 items, 22 NPCs, 9 loot tables, 10 spawners, 4 scripts, and 2 quests. Validated by `go test ./pkg/importer/...` (content-health run: 0 errors, 0 warnings). Design: `docs/design/PHASE5_DUNGEONS.md`.

## License

See LICENSE file for details.

## Contributing

This project is actively developed. In this fork the work happens on `master` (the batches below live in the working tree until committed); upstream still carries the historical `NPCs` branch on origin.

### Development Workflow
1. Fork the repository
2. Create a feature branch from `master`
3. Make changes following existing patterns
4. Test with dialog sandbox for NPC-related changes
5. Submit pull request

## Estado actual — Batches 17–29 (17 oct–1 nov 2026)

Consolidado de los batches 17–29. Detalle por sistema: `FEATURES.md`; contrato HTTP/WS: `docs/API.md`; QA histórico: `docs/qa/REGRESSION_REPORT.md` §B15–B24. El Batch 29 y su batería de tests se documentan en FASE 6.3; Batches 25–27 fueron verificados por QA sin sección propia en el reporte.

- **Batch 17 (docs):** guías `docs/user/*` e `docs/index.md` sincronizadas con el código; `FEATURES.md` con un apartado de estado por batch (hoy consolidado en "Estado actual — Batches 14–27").
- **Batch 18 (backend + tools):** comando `offer <npc>` y `offer accept|decline` (FASE 6.4) en `pkg/mudserver/game/commands/offer.go` + `game/quest_offer.go`, registrado en `commandprocessor.go`. Solo NPC quest-giver (contrato `IsQuestGiver` de la UI); contexto = nivel del personaje + área de la sala; la generación corre fuera del bucle de comandos y publica con `Game.Call`; proveedor `llmgen.GenerateContextualQuest` instalado en `pkg/server/llm.go`; cooldown de 60 s por personaje + semáforo de 2 generaciones; fallback determinista con `LLM_PROVIDER=none`; la quest solo se persiste al aceptar (`QuestsService.Store` + `AcceptQuest`). Tests: `pkg/mudserver/game/commands/offer_test.go`. Además, entrypoint `main.go` del harness `tools/loadtest`.
- **Batch 19 — Backend (FASE 5.1, mazmorras REST):** tabla `dungeons`, entidad `pkg/entities/dungeons/`, repo `pkg/repository/dungeons_sqlite.go` (`Factory.Dungeons()`) y `DungeonsService` (`pkg/service/dungeons.go`: nombre/rejilla/`definition` JSON opaco, `CreatedAt` preservado en update). Handler `pkg/server/handler/dungeons.go`: lectura autenticada `GET /api/dungeons?name=&author=&limit=` (resumen sin `definition`) y `GET /api/dungeons/:id` (detalle); escritura creator `POST` (201) / `PUT` (200/404) / `DELETE` (204/404). Tests: `dungeons_test.go` y `pkg/server/dungeons_http_test.go` (gate + viaje HTTP).
- **Batch 19 — Frontend:** `DungeonBuilder.svelte` (`public/mud-client/src/game/ui/`) con `dungeonBuilder.js` (rejilla, salas, corredores, encuentros, validación BFS entrada→salida) y `dungeonLibrary.js` (export/import); tecla `D` cableada en `Game.svelte` y listada en `ShortcutSheet.svelte`; `src/api/dungeons.js` usa `localStorage` como fuente de verdad y sincroniza **best-effort** con `/api/dungeons` con token creator (degrada a local con aviso). `npcBehavior.js`: `scaleTemplate` determinista por nivel + modelo de comportamiento serializable a Lua (preview/export only), consumido por la fábrica de NPCs (tecla `N`).
- **Batch 19 — Docs (rol Docs):** `docs/API.md` nuevo; `docs/index.md`, `FEATURES.md` y `docs/user/{admin-guide,commands}.md` actualizados.
- **Batch 19 — QA (FASE 7.1):** load test real 100–500 ejecutado (ver P0 de carga más abajo). Harness con nuevo modo `-mint` (`tools/loadtest/mint.go`, 550 identidades con `X-Forwarded-For` de loopback) + `mint_test.go` (7 tests).
- **Batch 20 — Backend (fix P0 de carga):** corrección del deadlock ≥200 conexiones descrito en §B19 — detalle completo en "P0 de carga" más abajo (`game_handlers.go`, `joinqueue.go`, `mudserver.go`, `transport.go`, `sessions.go`, `cmd/tales/pprof.go`).
- **Batch 20 — Docs (rol Docs):** sincronización de los docs raíz con Batches 14–19 (`PROJECT.md`, `ARCHITECTURE.md`, `FEATURES.md`, `README.md`, `docs/index.md`); en ese corte el P0 se documentó aún **abierto** (carrera de cronología con el fix del Backend del mismo batch).
- **Batch 21 — QA + Docs:** QA ejecuta la re-verificación de carga (§B21) y **cierra el P0** (500 concurrentes sin wedge, 0 `msgErrors`); detecta de paso el leak de `writeLoop` (P2, §B21.5). En Docs, el P0 pasa de "abierto/sin fix" a corregido en `PROJECT.md`, `ARCHITECTURE.md`, `FEATURES.md`, `README.md` y `docs/index.md` (fila 7.1).
- **Batch 22 — Backend/infra (fix de build + fix P2):**
  - **Build roto:** el Batch 20 añadió `cmd/tales/pprof.go`, pero el `Makefile` compilaba `cmd/tales/main.go` como archivo suelto → `make build`, `make build-backend`, `make run-server`, `make run` y `make dev` fallaban con `undefined: startPprof`. Los targets ahora compilan **por paquete** (`go build -buildvcs=false -o bin/tales ./cmd/tales`, `go run ./cmd/tales` y análogo para `./cmd/dialog_sandbox`).
  - **`.gitignore`:** la línea `tales` ignoraba cualquier ruta con componente `tales` (incluidos `cmd/tales/pprof.go` y `cmd/tales/pprof_test.go`); cambiada a `/tales`, más `bin/dialog_sandbox` y `/dialog_sandbox`. Ya no hace falta `git add -f`.
  - **Leak P2 de goroutines (§B21.5):** el transporte con un `writeLoop` por conexión no se cerraba al desconectar (~1 goroutine filtrada por sesión). Corregido con cierre idempotente del transporte en `pkg/mudserver/mudserver.go` (`HandleConnections` con `defer connection.closeTransport(0, "")` + `handleConnectionClosed`), test `pkg/mudserver/transport_leak_test.go`.
- **Batch 22 — Docs (rol Docs):** comandos de build sincronizados (compilación por paquete + `-buildvcs=false`), P0 documentado como **cerrado (verificado por QA en B21)**, nota de `git add -f` retirada y Batch 22 registrado en `PROJECT.md`, `ARCHITECTURE.md`, `FEATURES.md`, `README.md`, `docs/index.md`, `docs/fork-setup.md` y `docs/development/AGENTS.md`.
- **Batch 23 — Backend (fix de la race de presencia):** el `*entities.User` compartido por el handler WS, el bucle de juego y `game_handlers` se mutaba sin sincronía (`IsOnline`/`LastSeen`/`LastCharacter`) mientras `UsersService().Update` lo serializaba a JSON → data race + snapshot mezclado persistido (reportada en §B22.4 como P2). Fix: lock de presencia por usuario con refcount (`pkg/mudserver/game/presence.go`: `presenceRegistry`, API `ApplyPresence(user, mutate)` y `PresenceOf(user)`), copia desligada `entities.User.Snapshot()` (`pkg/entities/user.go`) persistida bajo el lock, `def.PresenceState` + `GameCtrl.PresenceOf` (`pkg/mudserver/game/def/gamectrl.go`). Escrituras/lecturas migradas en `mudserver.go`, `game/sessions.go`, `game_handlers.go`, `game/util/game_utils.go`, `commands/select_character.go` y `presentation/doorview/view.go`. Invariante: nada de mutar `*entities.User` fuera de `ApplyPresence`. Semántica B19.6-1 preservada (solo se persiste la transición offline↔online, no en cada input). Test de regresión: `pkg/mudserver/presence_race_test.go` (`TestPresenceMutationIsRaceFree`, 50 rondas con 5 writers concurrentes).
- **Batch 23 — Docs (rol Docs):** la race preexistente pasa de "pendiente" a **corregida en Batch 23** en `PROJECT.md`, `ARCHITECTURE.md` (Thread Safety: lock de presencia por usuario + invariante), `FEATURES.md`, `README.md` y `docs/index.md`; Batch 23 registrado con traza histórica.
- **Batch 24 — Backend (FASE 6.5, modo A — Dungeon Master AI):** el narrador reactivo en prosa queda completo y corregido (verificado contra el código 2026-10-22, sin commitear). Bus de eventos tipado `def.DMEvent{Kind, RoomID, CharacterID, UserID, Facts}` con `GameCtrl.NotifyDM` (`pkg/mudserver/game/def/gamectrl.go`); núcleo en `pkg/mudserver/game/dm_narrator.go` (`DMNarrationRequest`, `SetDMNarrator`, `TriggerDMNarration`, `NotifyDM`, cooldown por sala **y por usuario**, presupuesto global por minuto, semáforo de 2, `cleanDMText` ≤240 runes); proveedor instalado en `pkg/server/llm.go` (`installDMNarrator`, `RequireLLM: true` → fallback silencio vía `llmgen/fallback.go` + `OutputAllowed` en `llmgen/filter.go`); plantilla `pkg/service/llmprompt/prompts/dm.narration.yaml`. Los 5 eventos cableados: `combatStart` (`game_aggro.go`), `combatEnd` (`game_combat.go`), `questCompleted`/`levelUp` (`commands/dm_notify.go`) y `roomEnter` (`commands/room_takeexit.go`, solo al cambiar de área). Audiencia Room (con `Username: "DM"`) o User (quests/level-up); nunca global; **nunca muta estado ni corre en el tick**. Bugs corregidos: `Round` (usa `instance.Round`), `PartySize` (fiesta + followers reales vía `dmPartySize`) y `RoomName` (lookup `dmRoomName` + fallback en el proveedor). Config `LLM_DM_ENABLED` / `LLM_DM_COOLDOWN_SECONDS` / `LLM_DM_TIMEOUT_SECONDS` / `LLM_DM_MAX_PER_MIN` en `.env.example` y `docker-compose.yml`. Tests: `dm_narrator_test.go`, `commands/dm_notify_test.go`, `llmgen/fallback_dm_test.go`. **Pendiente en su fecha:** modo B (`DMDecision`), eventos ambush/environmental, métricas; **implementado después en B30** (`game/dm_decision.go`, opt-in). QA §B24: **verificado (APROBADO)**.
- **Batch 24 — Docs (rol Docs):** nota "Implementado (Batch 24, modo A)" en `docs/design/PHASE6_LLM.md` §6.5 (divergencias reales + pendientes, sin reescribir el diseño); este bloque pasa a "Batches 17–24"; `FEATURES.md` (sección FASE 6.5 + Last Updated), `ARCHITECTURE.md` (subsystem DM: flujo evento → NotifyDM → throttles → goroutine → mensaje), `README.md` y `docs/index.md` actualizados.
- **Batch 25 — Backend (FASE 6.3 corte 1 — NPC runtime LLM, comando `ask`):** el jugador pregunta libremente a un NPC y este responde en persona vía el LLM. Comando `ask <npc> [pregunta]` (`pkg/mudserver/game/commands/ask.go`, registro en `commandprocessor.go`, sin alias); si el NPC no tiene `llm.enabled` (`npc.LLMEnabled()`) **degrada al diálogo autoral** de `talk` (función `talkToNPC` extraída de `talk.go` y compartida) avisándolo, igual que cuando no hay proveedor instalado. Orquestación en `pkg/mudserver/game/npc_dialogue.go`: `RequestNPCDialogue` valida, aplica **cooldown por personaje+NPC** (`npc.EffectiveCooldownSeconds()`, default 3 s), semáforo de **2 concurrencias** y timeout de **20 s**, y el trabajo LLM corre **fuera del bucle de juego** (el resultado vuelve con `g.Call`); `NPCDialogueRateLimited`/`NPCDialogueBusy` responden con mensajes propios. Proveedor en `pkg/server/llm.go` (`installNPCDialogueProvider`): plantilla `npc.dialogue` (`TaskNPCDialogue`, JSON `{reply,options,end,memoryNotes}`), `RequireLLM: true`, `Temperature` por request (override opcional del 0.8 del prompt), bucket de rate-limit `LimitKey "npc:<npcID>:<characterID>"` (`llmgen.Limiter`, `LLM_RATE_PER_HOUR` 10/h por par; agotado → `ErrRateLimited` y línea neutra), filtro de contenido `OutputAllowed` y fallback neutro ("… te mira en silencio") en error/timeout/JSON inválido. **Memoria persistente** en `npc_memories` (clave `<npcTemplateID>::<characterID>`): `loadNPCMemory` inyecta resumen/hechos/última pregunta en el prompt y `recordNPCMemoryTurn` registra cada turno con los `memoryNotes` del modelo vía `NPCMemoriesService.RecordTurn` (resumen **determinista**, no LLM). En ese corte las `options` del modelo se mostraban como **texto plano** y no tocaban el grafo de diálogos autoral (el Batch 27 las hace accionables). **Nunca muta estado**: `ask` solo produce un mensaje (audiencia usuario, `Username` = nombre del NPC) y actualiza memoria. Tests: `pkg/mudserver/game/commands/ask_test.go` (7 tests) y `pkg/server/llm_npc_test.go` (4 tests). **Pendiente deuda (aquel momento):** personalidad estructurada (Big5/objetivos/secretos/relaciones), resumen LLM (`npc.memory_summary`), intercepción de texto libre y caché de 30 min que puede repetir reply — los dos primeros items se cerraron en el Batch 26 (corte 2); siguen abiertos la intercepción de texto libre (mecanismo C) y la caché.
- **Batch 25 — Docs (rol Docs):** FASE 6.3 corte 1 pasa a **implementado (corte runtime)** en `PROJECT.md` (roadmap, tabla de fases, comandos, bloques NPC/FASE 6 y este consolidado — el bloque pasa a "Batches 17–25"), `ARCHITECTURE.md` (nuevo subsistema NPC LLM: flujo `ask` → `RequestNPCDialogue` → proveedor → `TaskNPCDialogue` → `NPCMemoriesService` → entrega; `talk` autoral vs `ask` LLM; invariante "no muta estado"; fila `NPCMemoriesService` ahora con consumidor), `FEATURES.md` (comando `ask`, campos `LLM.Persona/Voice/Temperature/MemoryTurns/CooldownSeconds/FallbackDialogID`, memoria NPC, estado de Fase 6), `README.md` (mención en fork additions + roadmap), `docs/index.md`, `docs/user/commands.md` (`ask` documentado; sale de la lista de "comandos que no existen") y nota "Implementado (Batch 25, corte 1)" en `docs/design/PHASE6_LLM.md` §6.3 (divergencias reales + deuda restante, sin reescribir el diseño).
- **Batch 26 — Backend+Frontend (FASE 6.3 corte 2 — personalidad, memoria y config de NPC LLM):** cierra la deuda del corte 1 y hace el runtime usable (verificado contra el código 2026-10-26, sin commitear). **Config** (`pkg/entities/npcs/llm.go`): `LLMConfig` gana `Personality *LLMPersonality` (Big5 en [0,1]: openness/conscientiousness/extraversion/agreeableness/neuroticism), `Goals []string`, `Secrets []string` y `Relations []LLMRelation` (`Target`, `Kind ∈ {ally,rival,family,faction,mentor}`, `Sentiment` en [-1,1]), con `yaml` tags camelCase, `Clone()` en profundidad, `EffectivePersonality()` y `HasPersonality()`. **Prompt**: `formatNPCPersonalityContext` (`pkg/server/llm.go`) serializa los cuatro bloques y el proveedor los pasa como vars `personality/goals/secrets/relations` de la plantilla `npc.dialogue` (v1.0.1), cuyo system prompt exige Big5 para el tono, usar objetivos, **no revelar secretos literalmente** y matizar por relaciones. **Anti-fuga**: `validateNPCDialoguePayloadWithSecrets` + `leaksSecret` rechazan (`secret_leak`) cualquier reply, `options[].label` o `memoryNotes` que reproduzca un secreto literalmente (normalización case/espacios/puntuación + fronteras de token) → reintento del pipeline y, si persiste, la línea neutra. **Memoria**: plantilla `npc.memory_summary` (`TaskNPCMemorySummary`) rescribe el `Summary` de forma **asíncrona** cada `SummaryAtTurns` turnos (`triggerNPCMemorySummaryIfNeeded`, goroutine con ctx 12 s, cap 400 runes, fallback determinista si el modelo devuelve vacío); `repository.NPCMemory` gana `SummarySource` (`deterministic`/`llm`, sin migración — va en el JSON) y `RecordTurn` ya **no pisa** un resumen escrito por el LLM (tampoco uno externo distinto al determinista previo). **Config vía contenido/Creator**: `importer.YAMLNPC.llm` (`pkg/importer/models.go`) + deep copy en `ToEntity()` (`converters.go`); NPCTemplate REST con `llm` en DTO detalle y upsert (`handler/npc_templates.go`, con `Clone()`); `SpawnFromTemplate` copia `template.LLM.Clone()` (`service/npcs.go`) para que las instancias no muten la plantilla; panel **Dialogue / LLM** en el NPC factory del cliente (`NpcFactory.svelte` + round-trip `normalizeLLM` en `npcFactory.js`). **Contenido demo**: `import/adventure/data/npcs/tut_mentor.yaml` (Master Aldric) trae `llm.enabled: true` con persona, voz, Big5, 2 objetivos, 1 secreto y relación `player/mentor` — con proveedor activo, `ask Master Aldric …` responde en persona. Tests: `importer/llm_yaml_test.go`, `service/npcmemories_test.go` (3), `service/npc_templates_test.go` (`TestSpawnFromTemplateInheritsLLM`), `server/llm_npc_test.go` (+4 sobre el corte 1: secretos al prompt, fuga literal rechazada, guard sobre `options`/`memoryNotes` — validator y payload —, total 8) y `server/handler/npc_templates_test.go` (`TestNPCTemplateGetRedactsSecretsForNonCreators`).
- **Batch 26 — QA (CON OBSERVACIONES):** verificación del contrato del corte 2 (personalidad inyectada en el prompt, `llm` YAML→entidad, round-trip de plantilla, resumen LLM con preservación, cap de `MemoryTurns`), `-race` sin regresiones y sin romper `ask`/`talk`; fixes post-QA aplicados: 26.6.1 `llm` en los DTO REST de NPCTemplate, 26.6.2 spawn hereda `LLM.Clone()`, 26.6.3 `SummarySource` + preservación de resúmenes, 26.6.4 YAML tags camelCase, 26.6.5 secretos al prompt + validador anti-fuga. **Observaciones menores (cerradas en el árbol):** (a) `llm.secrets` legible vía `GET /api/npc-templates/:id` para cualquier autenticado — **corregido**: los secretos se redactan salvo para creators/admins (`canViewNPCTemplateSecrets`, fail-closed; test `TestNPCTemplateGetRedactsSecretsForNonCreators`); (b) el guard anti-fuga solo inspeccionaba `reply` — **corregido**: `validateNPCDialoguePayloadWithSecrets` también rechaza fugas en `options[].label` y `memoryNotes` (tests `TestValidateNPCDialoguePayloadWithSecretsChecksOptionsAndMemoryNotes` y `TestNPCDialogueRejectsSecretLeakInOptionsAndMemoryNotes`). Sin sección propia en `docs/qa/REGRESSION_REPORT.md`.
- **Batch 26 — Docs (rol Docs):** FASE 6.3 corte 2 pasa a **implementado** en `PROJECT.md` (roadmap, tabla de fases 6, fila `ask`, bloque consolidado — el bloque pasa a "Batches 17–26"), `ARCHITECTURE.md` (subsistema NPC LLM: config extendida, inyección de personalidad, resumen LLM con `SummarySource`, anti-fuga, spawn/herencia, config por YAML/REST/UI), `FEATURES.md` (tabla de campos `llm`, memoria con `SummarySource`, deuda actualizada, sección Batch 26), `README.md`, `docs/user/commands.md` (sección `ask` ampliada) y nota "Implementado (Batch 26, corte 2)" en `docs/design/PHASE6_LLM.md` §6.3.
- **Batch 27 — Backend+Frontend (FASE 6.3 corte 3 — opciones LLM accionables, conversaciones `__llm__`, relaciones dinámicas y feedback de cliente):** hace accionables las `options[]` que antes solo se pintaban como texto y añade la relación NPC↔jugador (verificado contra el código 2026-10-30, sin commitear). **Opciones accionables** (`pkg/mudserver/game/npc_dialogue.go`): `deliverNPCDialogue` normaliza las opciones (≤4, `kind ∈ {flavor,end}`, label ≤120 runes), las enriquece con `mapNPCDialogueOptions`/`matchAuthoredDialogLabel` (compara label con nombre/texto/textos alternativos/opciones de los nodos del diálogo autoral del NPC — resuelto por instancia → plantilla → `LLM.FallbackDialogID`) y, si hay Conversation Service, `sendNPCDialogueDialog` abre/actualiza una conversación con el **`DialogID` centinela `__llm__`** (`def.NPCDialogueDialogID`, `def/gamectrl.go`), guarda las opciones en `context` (`LLM_OPTIONS`, sin migración), engagea la sesión NPC (`EngageNPC(…"dialog")`) y manda un `messages.NewDialogMessage` con opciones numeradas (`Index: i+1`, `Username` = nombre del NPC); sin servicio de conversación cae al texto plano anterior. **Selección numérica** (`pkg/mudserver/game/commands/llm_dialog.go`, nuevo): `handleLLMDialogSelection` resuelve `end` (cierra sesión + conversación), `NodeID` (continúa el árbol autoral con el nuevo `talkToNPCAt` de `commands/talk.go`, que fija `conv.CurrentNodeID = startNode`) o `flavor` (nueva petición con `Continuation=true`: salta el rechazo por cooldown pero refresca la ventana, y sigue respetando el semáforo); índice inválido responde `Invalid option. Please choose 1-N` sin cerrar. `dialog_select.go` ignora conversaciones con `DialogID` vacío (una sesión NPC abierta no reactiva opciones cerradas) y enruta `DialogID == "__llm__"` al nuevo manejador. **Relaciones dinámicas**: `NPCDialogueResult.RelationDelta` acotado por `npc.ClampRelationDelta` (±0.05; en `parseNPCDialoguePayload` se clampa lo que devuelva el modelo), `applyNPCRelationDelta` lo persiste con `NPCMemoriesService.AdjustRelation` → `npc.ApplyRelationDelta` (acumulado clampado [-1,1] en `NPCMemory.Relations`, dentro del JSON `npc_memories` — sin migración) con default **+0.02** cuando el modelo no sugiere delta (0 si la opción fue `end`), y `mergeDynamicRelations` lo inyecta al prompt como `player(dinámica,0.04)` reemplazando la entrada estática `player`. **Cliente (27.4)**: `public/mud-client/src/game/relationFeedback.js` (nuevo, tolera varios shapes de payload: `relation`, `relationDelta`, `relationLabel`, before/after…) + `Client.js` (línea entre corchetes en el diálogo/mensajes planos y quinto argumento de `mux.setDialog`) + `DialogOverlay.svelte` (fila sutil ▲/▼/•) + `MUDXPlusStore.dialogRelation` (wired en `RoomWidget.svelte` y `MobileRoomView.svelte`); **solo se pinta si el mensaje trae info de relación**. Prompt `npc.dialogue` sube a **v1.0.2** (declara `relationDelta` y el rango ±0.05). Tests: `commands/llm_dialog_test.go` (5: flavor → continuación, `end` cierra, selección desconocida mantiene opciones, opción mapeada continúa nodo autoral, conversación cerrada no la reactiva una sesión NPC abierta), `npc_dialogue_test.go` (`TestMatchAuthoredDialogLabelFindsNodeAndOption`), `service/npcmemories_test.go` (`TestAdjustRelationClampsAndPersists`), `server/llm_npc_test.go` (`TestInstallNPCDialogueProviderAdjustsRelation`: delta 0.04 persistido, prompt con `player(dinámica,0.04)` y clamp del −0.20 a −0.01) y `public/mud-client/src/game/relationFeedback_test.mjs`.
- **Batch 27 — QA (APROBADO CON OBSERVACIONES):** verificación del corte 3 con Docker `golang:1.26` (`go build`/`go vet`/`go test ./...` en verde, `-race` en `pkg/mudserver/... pkg/server/... pkg/service/... pkg/entities/npcs/...` sin `WARNING: DATA RACE`, `npm run build` OK): opciones numéricas enrutan a nodo autoral/`flavor`/`end`, selección inválida mantiene las opciones, delta acotado y persistido en `NPCMemory`, inyectado al prompt, secretos nunca expuestos (los guards del Batch 26 siguen en pie), conversación stale no intercepta, `Username` en `DialogMessage`, sin regresión en `ask`/`talk` y frontend OK. **Observación no bloqueante (ajena al batch):** `TestWardGritAndRetaliate` (`pkg/mudserver/game/combat/ward_test.go:111`, aserto `tiny hit 2`) falló de forma intermitente en la primera corrida con `-race` y pasó 10/10 en aislamiento (y el paquete `combat` completo en el reintento): flakiness preexistente por un crítico aleatorio en un test que asume daño mínimo no crítico — los tests de guardia `TestWardGuard*` del mismo fichero pasaron. Warnings frontend no bloqueantes (selectores CSS sin uso, `autofocus` A11y). Sin sección propia en `docs/qa/REGRESSION_REPORT.md`.
- **Batch 27 — Docs (rol Docs):** FASE 6.3 corte 3 pasa a **implementado** en `PROJECT.md` (roadmap, tabla de fases 6, bloque consolidado — el bloque pasa a "Batches 17–27"), `ARCHITECTURE.md` (subsistema NPC LLM: conversación `__llm__`, routing de opciones, relaciones dinámicas, contrato de mensaje con relación), `FEATURES.md` (sección Batch 27, deuda actualizada, `Last Updated`), `docs/design/PHASE6_LLM.md` (nota "Implementado (Batches 25–27, cortes 1–3)") y `docs/user/commands.md` (sección `ask` ampliada: opciones accionables, relaciones, feedback y ejemplo con **Master Aldric**).
- **Verificación:** `go build -buildvcs=false ./...` y `go vet` en verde; `go test` de `pkg/server`, `pkg/service`, `pkg/repository` y `pkg/mudserver/...` OK (último corte QA de build/suite: Batch 19, 2026-10-18; presencia: §B23, 2026-10-21 — `go test -race ./...` sin DATA RACE; carga: **B21, P0 cerrado**). La suite completa tras el Batch 24 (FASE 6.5) quedó verificada por QA en §B24 (APROBADO, 2026-10-22; `-race` sin DATA RACE). Batches 25–27 suman tests dedicados y QA según se documenta arriba. Para Batch 29 se verificaron los cambios con tests de `talk`, selección y contexto de quests, validación de IDs, continuación de texto libre y overlay (`dialogOverlay_test.mjs`).
- **Pendiente:** spawn/instanciación de mazmorras en juego y endpoints de compile/run; §B19.4 (bypass de rate-limits por `X-Forwarded-For` de loopback); deuda de FASE 6.3: anillo de memoria corta, guardrails completos de combate, `FallbackDialogID` como fallback de error y caché de 30 min que puede repetir reply. FASE 6.5 modo B (`DMDecision` + ambush/environmental/narrative) implementado en B30 como opt-in (kill switch apagado por defecto). B/C y `kind=quest:<id>` quedan implementados en Batch 29. Las observaciones QA de B26 siguen cerradas; la de B27 fue un test de combate intermitente ajeno al batch. La race de presencia quedó cerrada y verificada en §B23.

### P0 de carga — deadlock del servidor a ≥200 conexiones (cerrado: fix en B20, verificado por QA en B21)

- **Qué ocurría (histórico, §B19):** con 100 conexiones simultáneas el servidor estaba sano (heartbeat de 60 s verificado con una corrida de 75 s, 99.96 % de respuestas); a **≥200 conexiones el bucle de juego quedaba bloqueado de forma permanente hasta el reinicio**. Umbral probabilístico: wedge observado tras 74–140 joins. Techo efectivo previo: ~<100 logins simultáneos.
- **Causa raíz:** `handleUserJoined` enviaba bloqueantemente a `onMessageReceived` desde la propia goroutine que es su único consumidor (buffer 20, `pkg/mudserver/game/game_handlers.go`), y el enqueue de `OnUserJoined` en `mudserver.go` dejaba los read loops WS atascados (712+ goroutines en `chan send` en el volcado SIGQUIT, CPU ~0 %). No era CPU, BD, red ni pings.
- **Cómo se corrigió (Batch 20, Backend — verificado contra el código):**
  - `pkg/mudserver/game/game_handlers.go`: los comandos sintetizados `lc` y `selectcharacter` se procesan **inline** con `game.processIncoming(...)` desde el propio bucle de juego, en vez de enviar a `onMessageReceived`. La restricción queda documentada en `pkg/mudserver/game/game.go` (`processIncoming`).
  - `pkg/mudserver/joinqueue.go` (nuevo) + `pkg/mudserver/mudserver.go`: cola FIFO **no bloqueante** de joins (`push` nunca espera); `attachSession` llama a `enqueueUserJoined` y regresa, y una única goroutine `forwardJoins` es la que bloquea en `Game.OnUserJoined`. Aviso único de saturación a 64 joins y log de recuperación.
  - `pkg/mudserver/transport.go`: cola de salida por conexión (128) con `writeLoop` dedicado; `Send` encola y retorna; cada frame se escribe con `SetWriteDeadline` de 10 s. Un consumidor lento se desconecta (`slow consumer`) en lugar de bloquear broadcasts/pings.
  - `pkg/mudserver/game/sessions.go` + `mudserver.go`: `LastSeen`/`IsOnline` se persisten solo en connect/disconnect (sin `UsersService().Update` por cada mensaje).
  - `cmd/tales/pprof.go`: endpoint `net/http/pprof` opcional (`ENABLE_PPROF`, `PPROF_ADDR`, por defecto `127.0.0.1:6060`), para diagnosticar un proceso atascado sin SIGQUIT.
  - Tests que fijan el fix: `pkg/mudserver/game/join_deadlock_test.go`, `pkg/mudserver/joinqueue_test.go`, `pkg/mudserver/presence_debounce_test.go`, `pkg/mudserver/transport_test.go`, `cmd/tales/pprof_test.go`.
- **Evidencia intermedia (Batch 20, Backend):** load test con 300 workers → **300/300 OK, 0 `msgErrors`** (antes wedgeaba a partir de ~200).
- **Estado:** **cerrado — verificado por QA en B21** (2026-10-19): 200/300/500 conexiones × 2 corridas × 80 s sin wedge, `recv == sent` exacto y **0 `msgErrors`** en 6/6 corridas, sonda post-carga sana y heartbeat de 60 s cruzado con 500 sesiones vivas (§B21.2/§B21.8). FASE 7.1 queda cerrada.
- **Dónde está documentado:** `docs/qa/REGRESSION_REPORT.md` **§B19** (metodología, resultados por concurrencia, causa raíz y evidencia del bug), **§B19.7** (recomendaciones, ya implementadas en B20) y **§B21** (re-verificación y cierre).
- **Riesgos asociados (P1/P2):** §B19.6-1 (2 escrituras SQLite por mensaje) y §B19.6-2 (transporte WS sin write deadline) **mitigados en B20**; el leak de `writeLoop` detectado en §B21.5 **mitigado en B22**; §B19.4 (confianza loopback en `X-Forwarded-For` que permite rotar los rate-limits por IP desde procesos locales) **sigue abierto**.

### Historial — Batches 14–16 (15–17 oct 2026)

- **B14:** inventario visual 3.5 (tecla `I`), fundamentos de load testing y consolidación de docs; cierra los batches B1→B13.
- **B15 (FASE 7.1):** auditoría de readiness de carga → §B15 (pool de 1 conexión, HTTP sin timeouts, heartbeat WS solo de aplicación).
- **B16 (FASE 7.2):** auditoría de seguridad sin hallazgos altos → §B16; endurecimiento (cabeceras de seguridad globales, timeouts HTTP con env, JWKS acotado), pool SQLite a 4 conexiones (`DB_MAX_OPEN_CONNS` + pragmas en DSN) e `migratePhase16Indexes`, `POST /api/creator/llm/quests/contextual` + `llmgen.GenerateContextualQuest`, narrador DM con LLM (`LLM_DM_ENABLED`) y fábrica de NPCs en el cliente (tecla `N`).
- **Notas ya superadas (2026-10-19):** la hoja `?` sí lista `I` y `D` y el Dungeon Builder existe (Batch 19); el peso del inventario sigue siendo solo aviso visual (`maxWeight = 100`, sin `weight` del servidor); el editor de clases (`L`) ya persiste en el servidor vía `/api/classes` y los YAML de `data/classes/`, mientras que la fábrica de NPCs (`N`) mantiene `localStorage` con sincronización best-effort.
- Detalle: `FEATURES.md` (secciones por sistema + "Estado actual — Batches 14–27").

---

## Related Resources

- [MUD Wikipedia](https://en.wikipedia.org/wiki/MUD) - Background on Multi-User Dungeons
- [Go Documentation](https://golang.org/doc/) - Go language reference
- [Svelte Tutorial](https://svelte.dev/tutorial) - Svelte framework guide
- [SQLite Documentation](https://www.sqlite.org/docs.html) - Database documentation

### Timed wandering NPCs
NPC templates support routes (loop or pingpong) and constrained roaming with intervals, jitter and engagement pauses. Spawners may override movement. Open dialogs/shops hold NPCs; movement sends departure/arrival prose and room presence updates. Creator NPC inspectors show movement and timers, the world map shows live wanderers, and content health validates routes. Legacy wanderRadius/patrolPath use the same timed system.
Movement is strictly opt-in: only an explicit movement block, positive wanderRadius or non-empty patrolPath enables it. Existing shopkeepers and unconfigured NPCs stay in place; spawners add no movement defaults.
