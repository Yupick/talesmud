# TalesMUD Core Systems & Features Reference

**Purpose**: This document catalogs ALL core systems, data structures, features, and APIs available in TalesMUD. It is designed to provide complete context for AI agents reworking game content to leverage the latest capabilities.

**Last Updated**: 2026-11-01 (Batch 29 — FASE 6.3 mecanismos B/C, opciones `quest:<id>` validadas, contrato de texto libre y prompt `npc.dialogue` v1.0.3; Batch 28 — conexión de `relationDelta` a mensajes; Batch 27 — opciones LLM accionables, relaciones dinámicas y feedback; Batch 26 — personalidad/memoria/config; Batch 25 — `ask` + `npc_memories`; Batch 24 — DM narrator modo A; batches anteriores 15–23)

**Upstream sync**: 2026-11-11 — integrado `v0.5.6` (base documental `2026-10-11`); se conservan ambas líneas de desarrollo.

El detalle histórico de Batches 25–27 que sigue documenta el estado de cada batch en su fecha. El estado vigente de FASE 6.3, incluido el cierre B/C, está en la sección “NPC runtime dialogue” y en `docs/design/PHASE6_LLM.md` §6.3.

---

## Table of Contents

1. [Data Structures (Entities)](#data-structures-entities)
2. [Room System](#room-system)
3. [Item System](#item-system)
4. [Character System](#character-system)
5. [Multiplayer Session & Social System](#multiplayer-session--social-system)
6. [NPC System](#npc-system)
7. [Combat System](#combat-system)
8. [Skills & Spells System](#skills--spells-system)
9. [Quest System](#quest-system)
10. [Dialog System](#dialog-system)
11. [Dungeon Builder (Phase 5.1)](#dungeon-builder-phase-51)
12. [Scripting System (Lua API)](#scripting-system-lua-api)
13. [Creator UI Capabilities](#creator-ui-capabilities)
14. [Recent Features & Best Practices](#recent-features--best-practices)
15. [Discovered World Atlas](#discovered-world-atlas)
16. [Game Client Tab Container Widget](#game-client-tab-container-widget)
17. [SSH Access](#ssh-access)
18. [Derived Attributes (Phase 2.3)](#derived-attributes-phase-23)
19. [Progression (Phase 2.4): XP Curve, Talents, Prestige](#progression-phase-24-xp-curve-talents-prestige)
20. [Client UX Enhancements (Phases 3.2, 3.4, 3.7)](#client-ux-enhancements-phases-32-34-37)
21. [Phase 6 LLM Integration (Design And Foundation)](#phase-6-llm-integration-design-and-foundation)

---

## v0.5.0 Capabilities and Authoring

- **NPC movement:** `NPC.Tags` and optional `movement` are shared with templates and spawner overrides. `mode` is `route` or `roam`; `route`, `loop` (`loop`/`pingpong`), `zones`, `rooms`, `intervalSeconds`, `jitterSeconds`, `pauseOnEngageSeconds`, `leaveMessage` and `arriveMessage` configure it. Default interval is 90 seconds and engagement hold is 120 seconds. Movement clocks/direction are memory-only. Legacy patrol paths and wander radii still work; absent movement leaves an NPC stationary. Validate walkable routes; hidden, locked, teleport and instance exits and `noNPC`/`noWander` rooms are excluded.
- **Rooms and spawners:** Room `noAggro` suppresses automatic attack on arrival. Room NPC payloads include `aggroOnSight` for aggressive labels. Spawner `respawnDelay` controls boss reappearance and respawn restores full HP.
- **Item effects:** `effects` contains `{trigger, name, text}` entries (`onHit`, `onEquip` or `onUse`; custom triggers are displayed as authored). Author player-readable effect lines; missing lines fall back to the linked script's display name and generic effect text. Inventory copies backfill template effect data; examine, shop and item cards show the same lines. These descriptions do not execute scripts.
- **Protocol and character state:** Item examine sends a structured `examineResult` payload alongside compatible text. Optional `hello` advertises protocol version/capabilities. Combat reattach restores the roster, actor, target, deadline and queued action; selected-character payloads mark guest identity. See [structured messages](docs/structured-messages.md).
- **Creator workflow:** Search with Ctrl+K; review references before deletion. Content health and `tales -check` validate reachability, entity references, quests and movement. Room/NPC inspectors, quest debugger, deterministic loot previews, loot-table/spawner editors, map overlays and live NPC/instance views share this data. Admins can inspect live characters, run operations and undo audited edits through the game loop. Entity IDs use `EntitySelectButton` and its filterable modal.
- **Access and security:** Optional SSH supports classic text play and 80×25 Door play with device confirmation, linked keys, guest limits and sanitized terminal output. Public classic device login issues expiring, revocable account sessions via [device login](docs/device-login.md). The fork defaults to local username/password accounts (`auth: local` in `config/gamemode.yaml`); Auth0 is an optional mode that requires audience and issuer only when enabled. HTTP Bearer auth and one-use WebSocket tickets remain the browser contract. Door now supports catalog buy/sell, display names and scripted fights.
- **Adventure content pack:** `import/adventure/` adds four zones on top of the hub — The Training Grounds (tutorial, 6 rooms), The Buried Vault (13), Frostspine Caverns (12) and The Elder Maw (4-room raid), plus 2 Millstone Hollow hub rooms (37 room files in total) — and 24 items, 22 NPCs, 9 loot tables, 10 spawners, 4 scripts and 2 quests in `pkg/importer` YAML. `pkg/importer/adventure_pack_test.go` loads the pack and `TestAdventurePackHealth` fails the build on any content-health error **or** warning; `TestAdventurePackReachability` fails on unreachable islands. Design: `docs/design/PHASE5_DUNGEONS.md`.
- **Fork docs:** setup and deployment in `docs/fork-setup.md`; player-facing system guides in `docs/user/` — `clans.md`, `dms.md`, `channels.md`, `party.md`, `combat.md`.

---

## Data Structures (Entities)

### Base Entity
All entities inherit from:
```go
type Entity struct {
    ID string `json:"id"`  // UUID
}
```

### Common Traits
Reusable trait composition pattern:
- `BelongsUser` - User ownership tracking (`BelongsUserID`)
- `CurrentRoom` - Location tracking (`CurrentRoomID`)
- `LookAt` - Inspection text (`Detail` field)

---

## Room System

### Room Entity Structure
```go
type Room struct {
    *entities.Entity
    traits.LookAt

    Name, Description string
    RoomType, Area, AreaType string
    Tags []string

    // Scripting Hooks
    OnEnterScriptID string  // Lua script executed when player enters room

    Actions *Actions  // Custom player interactions
    Exits   *Exits    // Room connections

    // Live Data (runtime state)
    Items      *Items       // Item IDs in room
    Characters *Characters  // Character IDs in room
    NPCs       *NPCs        // NPC resident IDs (not instances!)

    // Grid Positioning (optional)
    Coords *struct{X, Y, Z int32}

    // Client Meta
    Meta *struct{
        Mood       string  // Ambient mood setting
        Background string  // Background image ID
    }

    CanBind bool  // Allow /bind command for respawn point
}
```

### Room Actions
Players can interact with rooms via custom actions:
```go
type Action struct {
    Name        string  // Command trigger (e.g., "examine statue")
    Description string  // Shown in "You can:" list
    Response    string  // Reply to player (type: response)
    Type        RoomActionType  // "response", "response_room", "script"
    ScriptId    string  // Lua script to execute (type: script)
    Params      map[string]interface{}
}
```

**Action Types**:
- `response` - Send message to player only
- `response_room` - Broadcast message to all in room
- `script` - Execute Lua script with context: `ctx.room`, `ctx.character`, `ctx.action`

### Room Exits
```go
type Exit struct {
    Name        string  // Exit command (e.g., "north", "trapdoor")
    Description string  // Shown in exits list
    Type        RoomExitType  // "normal", "direction", "teleport"
    Hidden      bool    // Not shown until revealed (per-character)
    Target      string  // Target room ID
    Params      map[string]interface{}
}
```

**Exit Types**:
- `direction` - Standard cardinal directions (n/s/e/w/u/d)
- `normal` - Named exits (e.g., "door", "gate")
- `teleport` - Instant transport to distant location

### Per-Character Hidden Exit Reveals
Hidden exits can be revealed on a **per-character basis** using scripting:
```lua
-- In a room action script:
tales.game.revealExit(roomID, "secret-passage", characterID)
```

- Reveal is tracked on `Character.RevealedExits` map (persisted)
- Other players do NOT see the exit unless they also reveal it
- Client receives automatic room update showing new exit

### Room Discovery & Exploration XP
```go
// On Character entity:
DiscoveredRooms map[string]bool  // Room IDs
DiscoveredAreas map[string]bool  // Area names
```

**Exploration XP Rewards**:
- **5 XP** per new room discovered (grant path is currently gated; discovery itself still records)
- **15 XP** for first room in a new area/zone

**Atlas API**: `GET /api/characters/:id/map` returns the character's fog-of-war atlas (places, paths, area hulls, overworld/lower/upper layers). Layout preserves authored area-local geometry and compass exits, translates zones onto compact configurable centers, and fills anonymous ground between zones into one connected continent. Hidden exits stay off the map until `revealExit`.

### Terrain atlas and continent (Worldmap P1/P1b/P1c/P1d/P1e/P1f/P1g/P1j)
Discovered atlas places carry `terrain`: grassland, forest, farmland, city, castle, dungeon, swamp, mountain, snow, desert, water, shore, ruins, or interior. Unexplored neighbors carry `terrain: "fog"`; their art remains hidden. `pkg/worldmap/map_terrain.json` owns ordered aliases, area defaults, and the unknown-ground default (grassland). RoomType/areaType and specific tags take priority, followed by name, indoor/underground context, area, then descriptive/legacy fallbacks. Classification does not add persisted entity fields or scripting APIs.

The overview and minimap share a compact cached 32px overview with precomposed directional dither sprites from the native 48px art sheet and 48px close detail baked on demand with six stable variants per terrain, nearest-neighbor sampling, ordered-dither edge/corner transitions, a smoothed organic contour with sand/foam/depth bands, mixed rock foothills/taller central peaks and biome-specific oak/pine/dead-tree clumps, and quieter dirt roads/bridges following charted outdoor compass exits. The road network batches unique exit segments into one path beneath mountains, tree canopies, town paving, walls, buildings and props; full-opacity town paving hides dirt inside streets and thinner strokes reduce far-zoom clutter. Towns render street-shaped paving, varied red/brown/blue roofs, angular wall polygons following courtyard/street footprints, corner towers, incoming road gatehouses, keeps, and small hamlet clusters. Crisp town/keep/village glyphs remain readable at far zoom and select their owning real room. P1g redrew all terrain and decoration rows at 48px. **P1j** ships the imagegen craft atlas at the same 48px / 288×9024 layout (seamless terrains, chroma/alpha stamps, recomposed dither blends). Shore bands use twelve samples per cell with broken foam glints; image scaling uses nearest-neighbor sampling. P1e roads-under-stamps, P1f zoom clamps (1–10, 220px cap), and A9 remain intact. Close zoom retains small pixel trees; overview uses larger biome-specific clumps and varied ridge chains, with snow caps only on the highest peak variants. Towns/farms add wells, carts, lanterns, fences and one windmill per farm zone; plateau/highland rooms add mesa edges/outcrops. Seas use varied wave texture, offshore rocks and highland coastal cliffs. World fit and minimum zoom share a frame filling roughly 70–80% of desktop map height (width permitting). Wheel and pinch share a 1–10 scale clamp; the tile-size cap is 220px, so the closest view is twice P1e’s maximum in both world-fit and local recenter frames. High zoom keeps nearest-neighbor pixel sampling. Visible underground passages have cave or mine entrances. Shops, taverns, houses, halls, and upstairs rooms share exterior anchors instead of separate overworld floor tiles. Selecting a town/building offers a filterable list of discovered interiors; the selected interior keeps its actual room ID, intel, exits, and Travel action. Visible entrance choices switch to Lower, without disclosing unexplored names or hidden exits.

Derived presentation fields are `Place.mapRole` (`surface`, `interior`, `underground`), `surfaceRoomId` (interior anchor), `town`, and `entrances` (visible Lower target IDs). Discovered rooms additionally carry `mapFeatures` (derived decoration keys), `artSeed` (deterministic customization seed), and `undergroundStyle` (`cave`, `crypt`, `cellar`, `sewer` on Lower); fog rooms omit all three. `PlayerMap.landscape` contains decorative `{x,y,terrain}` cells: no room IDs, names, hit targets, or travel destinations. Ground reveals near discovered surface/interior rooms or across a fully charted surface area; other ground is fogged. Terrain/fog, coordinates, grouping, towns, entrances, features, art seeds, and underground style updates repaint; stale fog cannot replace charted ground. Outdoor positive Z represents elevation on Overworld; subterranean rooms stay Lower. Other untagged above-ground floors can still use Upper. Gold current-room glow (including instanced interiors), zone labels, selection/intel, travel, layer tabs, zoom/pan, Fit world, and local recenter remain available.

Room art follows tags, room/area types, names, service action names, descriptions/detail fallbacks, and bind context. Forge roofs have chimney smoke; shrines spires; taverns/shops signs; farms fields/farmhouses; guards/gates towers; ruins broken walls; graveyards headstones; docks piers; mines timber entrances; magic sites glowing stones. Reeds, stumps, flowers, rocks, and tree species vary deterministically. Hints do not export source descriptions, script IDs, or service parameters. Outdoor water rooms retain their own terrain even beside forests/swamps. Art changes require no new authored entity fields or Lua API.

Lower uses dedicated cave/crypt/cellar/sewer floors, merged adjacent rooms, actual-exit rock-sided corridors, perimeter stone walls, torch light, and disclosed stair/entrance marks against dark void. Unknown rooms keep fog and expose no art hints. The full overlay has low-rate water shimmer and smoke; hidden/closed views pause it, reduced motion fixes the phase and suppresses its timer, and teardown clears it. Cached overview/close/Lower scenes preserve marker-only reuse and fog isolation.

P1d Lower uses rough cave rock, crypt paving/bones, sewer water channels, and wooden cellar floors/barrels with dim cluster light. Only short, aligned known compass exits get visible tunnels; longer/ambiguous links remain usable navigation exits without grey lines across void. Tree/building drop shadows are removed; only restrained mountain face shading remains. On supported browsers an OffscreenCanvas worker builds scene bitmaps while the main thread remains responsive; only one job runs and stale queued exploration work is replaced. Close LOD builds on demand; worker/canvas fallback preserves map behavior if unavailable. Unexplored overworld ground uses a soft volumetric fog overlay (blurred explore-boundary mask + cloud wash) instead of per-tile fog stamps; You / Turn-in / Selected markers paint above it. Authored room coords that treat +Y as north are flipped at compile so Cartographer north paints upward (`?v=mapnorth1`). Cached fog ground is painted once, and real room centers win hit testing over neighboring decorative props.

`GET/HEAD /api/map-tiles/terrain-sheet.png` serves the 288×9024 RGBA sheet with a content-hash query version; live client JS/CSS uses `?v=creamtimber1` (sheet still content-hash busted). Production art is the P1j imagegen craft atlas (content `assets/map-tiles`, version `5caee700810a` (cream-timber settlement stamp remapper on P1j; prior `c8169d17ef81`)); Pillow generators (`tools/generate_map_tiles.py`, `tools/map_hires_art.py`) remain for regenerating the older P1g look only. `pkg/worldmap/map_layout.json` configures zone centers, natural ground, town flags, separation, and coast padding; unconfigured zones attach using exits. The layout does not mutate stored coordinates or gameplay topology. See `tools/WORLDMAP-PREVIEW.md`. Ornate banners and map framing ornaments remain later work.

### NPC / enemy portraits
Room presence sends `portrait` URLs (`/api/portraits/{templateOrId}.png`). Import copies `assets/images/sprites/{npcs,enemies}/` into `uploads/portraits/`. Sprites are 512px full-figure art; the original NPC/enemy cards clip a 48px square around the body (`object-fit: cover` + zoom). Missing files fall back to hashed `img/avatars/{1-14}p.png`. Component CSS lives in `public/mud-client/public/extra.css` and must be deployed with `bundle.js`.

### Player portraits
The equipment paper doll, combat card, and party roster use 512px transparent sprites at `/api/portraits/player-<race>-<class>.png`. The set covers Human, Dwarf, Elf × Warrior, Rogue, Mage, Ranger, Cleric, Druid. Stored `elve` maps to `elf`, `wizard` to `mage`, and `hunter` to `ranger`. The server supplies combat and party portrait URLs; the client derives the equipment URL from character race/class. An unavailable combination or failed image uses a class silhouette.

### Group combat cards
BattleStage shows the local player and all other combat players (up to the five-player party cap). Compact ally cards display portrait, class, level, live HP/MP, current turn, and down/fled state. gilded fantasy frame buff/debuff icon rectangles sit above/beside combat portraits (self, allies, enemies) with remaining rounds from statusEffects. Combat action snapshots carry participant type, class, mana, and status; a join sends the roster to existing fighters immediately and triggers a short join banner. Healing and buffs currently target self or enemies only; ally-card clicks do not queue unsupported commands.

**Battle layout B (default):** room-field combat for players with no saved preference. Party stands on the left — the player on a gold marker, and, when other players are in the fight, a compact left ally strip (existing portrait, name, slim HP). Enemies stand on the right. Detail frames stay top-left (you: name, HP/MP, buffs) and top-right (focused foe). Sprites use existing race/class and enemy portraits (`/api/portraits/player-<race>-<class>.png` and `/api/portraits/{template}.png`); a class or enemy silhouette is only the failed-load fallback. Over each sprite, only a slim HP bar. Mid-fight Resolving/Waiting pills and turn-name chips stay hidden; the round chip and your-turn countdown remain. Full hotbar 1–9 + Flee dock + room vignette unchanged. Classic cards remain in Settings → Gameplay (turn Battle layout B off), `localStorage.talesmud_battle_layout_b=0`, or `?battleLayout=classic` / `?battlepoc=0`. `?battleLayout=b` / `?battlepoc=1` forces layout B. URL wins over localStorage; an explicit `0`/`1` wins over Settings. Client cache `?v=battleb1`.

The self card and ally cards share a desktop row and stack on phones, with horizontally scrollable phone allies. Only structured actions generate an action banner; join prose is not repeated in stage banners. Defeated enemies keep a grey sprite and Defeated label. Damage and healing numbers sit over player/enemy sprites with a dark outline and hold full opacity before fading; reduced motion disables their animation. Material Icons use a preloaded local WOFF2 font and remain hidden if it fails to load.

WebSocket connects go through a process-wide gate (`websocketGate.js`): one CONNECTING/OPEN/CLOSING socket, no reactive `ws=null` reconnect, and close code 4001 (session replaced) does not auto-reconnect. The Map overview is an Inventory-style body-portal panel (`map-panel`, never Materialize's `.modal`) with explicit pixel size so the canvas fills the stage. `/play` JS/CSS/HTML is served `Cache-Control: no-cache` plus `?v=` on asset URLs so deploys are not stuck behind a cached `bundle.js`.

### Instanced cellars
Exits with `type: instance` or `instance: true`, or a normal directional exit from a shared/hub room into a room tagged `instance`/`instanced`, create a private copy of the destination plus rooms reachable without walking back into the hub. Two guests share the town room (e.g. The Weary Wanderer `R0203`) and get different cellar IDs (`R0215~aabbccdd`). Hidden cellar wings (R0230+) are cloned with the entrance. When the last occupant leaves, clones and copied NPCs are destroyed.

Copy ids use `~` (`pkg/instances.IsCloneID`). The in-memory manager is empty after a restart, so before the process accepts players it deletes every persisted room id that contains `~`. NPC copies are memory-only. Ground item ids are stored on the room document, so deleting the copy removes that placement and leaves the item catalog. An NPC row whose id contains `~`, or a spawner whose id or room id contains `~`, is deleted too. Characters saved in a copy, or in a room id that no longer exists, move to that template's hub (the non-instance room whose exit enters the graph), else the copy's return exit, else the world start room, else the bind room. An empty saved room is left empty. Stale combat flags are cleared on the characters who move. The next login in that process sends `You find yourself back at <room name>.` once. A later login does not repeat it. A live copy still known to the manager is left alone. An id that contains `~` but is missing from that map still counts as a copy for disconnect release and for the login room check. There is no SIGTERM hook; the next start runs the same sweep.

### Armor durability
Dying chips equipped armor (default 4 hits to broken). Defense scales with remaining durability. Items are never deleted. `repair` at a merchant restores them.

### Merchant prices
Each NPC `MerchantTrait` has its own `buyMultiplier` / `sellMultiplier` / `priceOverride` on the same item template, so overlapping stock can cost differently per shop.

---

## Item System

### Item Entity Structure
```go
type Item struct {
    *entities.Entity
    traits.LookAt

    // Template/Instance Pattern (same as NPCs)
    IsTemplate     bool    // True if blueprint
    TemplateID     string  // Source template ID (for instances)
    InstanceSuffix string  // Unique suffix (e.g., "abc123")

    Name, Description string
    Type    ItemType     // "currency", "consumable", "armor", "weapon", "collectible", "quest", "crafting_material"
    SubType ItemSubType  // "sword", "twohandsword", "axe", "spear", "shield"
    Slot    ItemSlot     // Equipment slot or "inventory"
    Quality ItemQuality  // "normal", "magic", "rare", "legendary", "mythic"
    Level   int32

    // Custom Properties
    Properties map[string]interface{}  // Arbitrary key-value data
    Attributes map[string]interface{}  // Stats (e.g., "healthRestore": 50)

    // Container System
    Closed   bool
    Locked   bool
    LockedBy string  // Key item ID
    Items    Items   // Nested items
    MaxItems int32

    // Interaction Flags
    NoPickup           bool    // Cannot be picked up
    CopyOnPickup       bool    // Create personal copy instead of removing from room
    BoundToCharacterID string  // Item is bound to this character (cannot drop/sell/trade)

    // Stacking & Economy
    Stackable bool
    Quantity  int32
    MaxStack  int32
    BasePrice int64  // Gold value

    // Scripting & Consumption
    OnUseScriptID string  // Lua script executed when item is used
    Consumable    bool    // Remove/decrement on use

    // Metadata
    Tags      []string
    Created   time.Time
    CreatedBy string
    Meta      *struct{Img string}
}
```

### Item Types & Slots
Stackable item quantities are kept consistent when consumed or partially dropped: the character inventory and backing item instance are both updated.

`unique: true` on an item template means a character can hold at most one copy. The count covers the bag, equipped gear, and items nested in a container. There is no separate item bank. Pickup of another is refused with "You already have the <name>." Extra copies already in the bag are trimmed to one on that attempt. The ground drop is left for someone else.

A loot entry may set `rarity: unique`, `chance` (a non-zero value wins over `dropChance` at import), and `boss_only: true`. Boss-only entries roll only when the dead NPC's difficulty is `boss`. The unique roll is skipped when every victory recipient already holds that template. A drop from a `rarity: unique` entry sends a room message `UNIQUE: <item name>` with `style: combatEvent`, `hook: unique`, and `source` set to the NPC's display name. The play client frames that victory row and the item card. A template `unique: true` flag still caps ownership and frames the item card on examine. It does not send the room chip on its own.

**Item Types**:
- `currency` - Gold, tokens
- `consumable` - Potions, food, scrolls
- `armor` - Wearable protection
- `weapon` - Swords, axes, staves
- `collectible` - Quest items, trophies
- `quest` - Quest-specific items
- `crafting_material` - Components for gathering/crafting recipes

### Gathering & Crafting (v1 — no professions)
- **Everyone** can gather and craft. No skill ranks, no profession unlocks.
- **Gathering**: room actions `GATHER …` / `FORAGE …` / `HARVEST …` (scripted nodes with deplete flags) plus commands `gather` / `forage` / `harvest [target]`.
- **Crafting**: `recipes` lists all recipes; bare `craft` opens the same list; `craft <recipe>` consumes mats and grants output. Client seeds a **Recipes** action-bar pin; R0209 also exposes **CRAFT** / **RECIPES** room chips.
- Optional **stations**: recipe `station: forge` requires room tag `forge` or `crafting`; `campfire` requires `campfire`/`kitchen`/`hearth`. Food/leather recipes craft anywhere.
- Recipe YAML lives in content `data/recipes/` (loaded from `import/mvp-rpg-1/data/recipes` at runtime; seed fallback in engine).

**Equipment Slots**:
- `head`, `chest`, `legs`, `boots`, `hands`, `neck`, `ring1`, `ring2`
- `main_hand`, `off_hand`
- Special: Two-handed weapons occupy both hand slots

### CopyOnPickup Feature
When `Item.CopyOnPickup = true`, `Item.IsTemplate = true`, or the item is a world catalog ID placed in a room (no `templateId` / instance suffix):
1. **First pickup**: Creates a personal instance from the blueprint, adds to inventory
2. **Instance is bound**: `BoundToCharacterID` is set on the instance, making it character-bound
3. **Room remains unchanged** - the world item stays for other players and later guests
4. **Character tracking**: Marks item as collected via `Character.Flags["collected_item:<templateID>"]`
5. **Subsequent pickups**: Player sees "You have already collected X"
6. **Client behavior**: Item is hidden in room display for that character
World import sets `copyOnPickup` on every item referenced by a room so tutorial caches (torch, flint, dagger, fragments) cannot be consumed by the first guest. Dropped loot instances (templateId set, not a template) are still taken from the room.

Room presence and other room-audience messages are fanned out from in-memory sessions, not a SQLite room reload on the send goroutine. Reloading there can deadlock the game loop after an on-enter script, so a follow-up `north` never leaves the room. Unique NPCs with only `spawnRoomId` (empty `currentRoomID`) are still placed on server start. Spawners refill to `initialCount` immediately; `spawnInterval` only throttles extra instances up to `maxInstances`.

**Bound item restrictions** (enforced on items with `BoundToCharacterID`):
- Cannot be dropped (`drop` command rejects with message to use `destroy`)
- Cannot be sold to merchants
- Cannot be traded
- `destroy` command removes from inventory, clears collected flag, and refreshes room

**Helper methods**: `Item.IsBound()`, `Item.IsOwnedBy(characterID)`

**Use case**: Quest items, story artifacts that all players should find

### Item Usage & Consumables
```go
// Built-in effects via Attributes:
Attributes: {
    "healthRestore": 50,      // Restore 50 HP
    "manaRestore": 30,        // Restore 30 mana
    "useMessage": "You feel refreshed"  // Custom message
}
```

**Usage flow**:
1. Player uses item: `use health potion` or `use flint on torch` (optional `on <target>`)
2. System checks `Attributes` for built-in effects
3. If `OnUseScriptID` is set, executes Lua script (preferred over built-in torch lighting)
4. Fire-starters without a script auto-target a carried `light_source` on bare `use`; otherwise soft-hint `use <item> on <target>`
5. If `Consumable = true`, decrements quantity or removes item

World YAML may set `onUseScript` or `onUseScriptId` (importer accepts both). Inventory UI shows **Use** for usable items and **Use on…** for tools/fire-starters to pick a second inventory item without Terminal X.

### Item Template/Instance Pattern
```go
// Create instance from template:
instance, err := itemsService.CreateInstanceFromTemplate(templateID)

// Check if item is an instance:
if item.IsInstance() {  // Has TemplateID and InstanceSuffix
    // ...
}
```

---

## Character System

### Character Entity Structure
```go
type Character struct {
    *entities.Entity
    traits.BelongsUser
    traits.CurrentRoom

    Name, Description string
    Race  Race   // Human, Elf, Dwarf, Halfling, Orc
    Class Class  // Warrior, Mage, Rogue, Cleric, Ranger, Druid

    // Core Stats
    CurrentHitPoints, MaxHitPoints int32
    CurrentMana, MaxMana int32  // Casters only
    XP, Level int32
    Gold int64
    MaxLevelCap int32  // Per-character level cap (0 = use global MaxLevel)

    // Attribute System
    Attributes Attributes  // []{Name, Short, Value} - STR, DEX, CON, INT, WIS, CHA

    // Distributable Attribute Points (2 per level)
    UnspentAttributePoints int32
    SpentAttributePoints   map[string]int32  // Tracks total spent per attribute

    // Inventory & Equipment
    Inventory     items.Inventory
    EquippedItems map[items.ItemSlot]*items.Item
    EquippedSkills []string  // Skill IDs (max 4, level-gated)

    // Combat State
    InCombat         bool
    CombatInstanceID string
    BoundRoomID      string  // Respawn location (set via /bind)

    // Per-Character Game State
    Flags map[string]interface{}  // Script-settable flags
    RevealedExits map[string][]string  // roomID → exit names
    DiscoveredRooms map[string]bool
    DiscoveredAreas map[string]bool
    FriendIDs []string  // Other character IDs on this character's friends list

    // All-Time Statistics
    AllTimeStats struct {
        PlayersKilled   int32
        GoldCollected   int32
        QuestsCompleted int32
        RoomsDiscovered int32
    }
}
```

### Attribute System
Six core attributes (D&D-style):
- **STR** (Strength) - Melee damage, warrior scaling
- **DEX** (Dexterity) - Dodge, rogue/ranger scaling, initiative
- **CON** (Constitution) - HP bonus
- **INT** (Intelligence) - Mage/Druid spell power, mana capacity
- **WIS** (Wisdom) - Cleric spell power, mana regeneration
- **CHA** (Charisma) - Social interactions (future)

**Attribute Modifiers**:
```go
modifier = (value - 10) / 2
// Example: STR 14 → +2 modifier, STR 8 → -1 modifier
```

### Distributable Attribute Points
Players earn **2 points per level-up** to spend freely:
```bash
# Terminal commands:
spend          # Show status table with current values, spent/cap
spend str 3    # Allocate 3 points to STR
spend dex      # Allocate 1 point to DEX
```

**Class-based Caps** prevent degenerate builds:
- Warriors: INT capped at 5
- Mages: STR capped at 5
- Each class has primary/secondary attribute caps defined

**Retroactive Points**: Existing characters receive `(level - 1) * 2` points on login

### Character Methods (Combat-Related)
```go
// Attribute access
GetAttribute(short string) int32          // Get raw value
GetAttributeModifier(short string) int    // Get (value-10)/2
GetSTRMod(), GetDEXMod(), GetCONMod() int // Specific modifiers
GetINTMod(), GetWISMod() int

// Combat calculations
GetWeaponDamage() int32        // Main hand damage (1 if unarmed)
GetArmorDefense() int32        // Total from equipped armor
CalculateMaxMana() int32       // Caster: 20 + Level*5 + INTMod*4
CalculateManaRegen() int32     // Per combat round: 1 + WISMod (min 1). Separate from regen.in_combat.
```

### Character Flags System
Arbitrary key-value storage for script state:
```lua
-- Set flag:
tales.game.setFlag(characterID, "puzzle_solved_statue", true)

-- Get flag:
local solved = tales.game.getFlag(characterID, "puzzle_solved_statue")
```

**Common uses**: Quest state, puzzle progress, secret discoveries

`LastResetDay` records the calendar day of a new-day heal. `AwaitingReset` is set when the death policy's respawn mode is `next_reset`. Both stay empty on existing characters.

### Ruleset profile

`config/ruleset.yaml` sits beside `config/combat_balance.yaml` and must not repeat its keys (`level_gap`, `threat`, `reward_scale`, `class_balance`, difficulty multipliers, named overrides). The shipped file matches current play: level cap 50, automatic level-up, 10% XP loss and 1 on-hand gold on defeat, respawn at the bind room with half HP, no dawn heal, no resource keys, combat pacing `auto`, `combat.safe_room: stay`, `combat.disconnect: continue`, and `combat.bare_attack: ask`.

An enemy's authored XP reward is the base. When that reward is 0, `progression.base_xp_by_enemy_level` supplies the base, and otherwise the built-in `15*level+5` curve does. `reward_scale` multiplies that base afterward. `level_up_mode: trainer` banks combat, quest, exploration, and select catch-up until `tales.characters.applyLevels`. Quest XP is not multiplied by `reward_scale`.

Death math is `ruleset.ApplyDeath`, called from defeat only. A fight is a defeat when nobody is still fighting and at least one player is actually dead. When every player has fled or slipped, the fight ends as fled: no XP loss, no gold loss, no armor damage, and no respawn move. A slip that already took one exit stays in that room. In a mixed party only the dead take the penalty. Anyone who fled gets the escaped notice and keeps their hit points, gold, and room. Post-combat cleanup still clears `InCombat` and syncs the escaper's combat hit points.

`combat.pacing: auto` keeps the 5 second decision window and resolves a queued action on the next beat. `turn_based` leaves that window open until the player sends a command. NPCs still take their own turns afterward. The default file is `auto`. During a fight, a bare `attack` queues a swing on the current target or the first living enemy so a turn-based round advances. Outside combat, `combat.bare_attack: ask` (the default) still answers "Attack whom?". `first_hostile` starts the fight against the first hostile in the room, and says nobody is there when none is.

`combat.disconnect: continue` (the default) leaves a dropped connection in the fight and does not move the character. `release` ends that fight as a flee: no gold loss, no XP loss, and no death flag. `combat.safe_room` is `stay` (default), `bind`, or `start`, and applies only when disconnect is `release`. `stay` leaves the character in a real room. `bind` and `start` move them. A room id containing `~` is treated as an instance even when the in-memory map has lost it, so release still moves that character. A generated instance that times out still moves its occupant to the return room and ends the fight without a defeat. On the next enter, a saved room that no longer exists is replaced by the bind room, then the start room. An orphan `~` id uses the startup sweep's room order instead.

Passive regeneration is the `regen` block in the ruleset, and a world game-mode file may carry the same block. `out_of_combat`, `resting`, and `in_combat` each have HP and mana pools with `enabled`, `percent`, `flat`, and `interval_seconds`. Missing keys keep out of combat at 2% HP and 5% mana, resting at 10% HP and 15% mana, and in combat at 0.5% HP and 1% mana, every 10 seconds, flat 0. The gain is `int(max * percent / 100) + flat`, at least 1 when the pool is active. A pool is active when `enabled` is true and percent or flat is positive. `enabled: false`, or percent and flat both 0, grants nothing and is not due. An explicit `interval_seconds` below 1 is rejected.

Intervals follow one server clock, not the character. The first tick after login lands anywhere from 0 to interval−1 seconds in. A very small enabled interval saves the character and sends one websocket update per regenerating player per interval. Turning every pool off also skips the fully-rested and in-combat resting cleanup on that clock. `applyRegeneration` and `InterruptRest` still clear the resting flag.

Off: `enabled: false` on that pool (`regen.out_of_combat.hp`, `regen.resting.hp`, `regen.in_combat.hp`, and the matching mana pool). Slow: `percent: 0.5` and `interval_seconds: 60` out of combat, `percent: 1` and `interval_seconds: 60` while resting, `percent: 0.1` and `interval_seconds: 30` in combat.

`CalculateManaRegen` (1 + WISMod per combat round, minimum 1) is separate from `regen.in_combat`.

`guide.enabled` defaults to on. A world game-mode file may set the same key. `false` builds no next-step hint and the server sends no `guideHint`. An omitted key stays on.

`combat.respawn_min_seconds` defaults to 30. After a death, a spawner waits at least that long before it replaces the corpse. An authored `respawnDelay` still waits when it is longer. `0` disables the floor. Server start still fills `initialCount` immediately. `combat.aggro_on_sight.respawn_grace_seconds` defaults to 8 and holds a fresh respawn that appears on players already in the room. `0` uses `grace_seconds`. `post_combat_grace_seconds` defaults to 10 and holds aggressive NPCs in the fight room after a victory or flee. Leaving that room ends it. `spawn_aggro_cooldown_seconds` defaults to 60 and stops that same spawn, including a replacement, from auto-aggroing the same player again. Another player is still engaged. `attack` still starts a fight. `0` disables a window.

### Refilling resources

Per-character balances live in the `character_resources` table. A key grants uses only after something configures an allowance (calendar day in a timezone, or a fixed interval). An optional `label` is a display name; an empty label leaves the key as the name. Inside a period, raising the allowance or a modifier does not give the extra uses back; the next period refills to the new amount. An empty catalog, which is the process default, answers every key as not configured and writes no row.

Another world can use a key for a daily gathering node or a delve ticket, spent from a room-action script. No content ships a key, so play is unchanged. Entering play calls `Get` for each configured key, so a new period is refilled even before a script reads it. A one-word room action accepts a trailing argument (`deposit 20`); the script sees it as `ctx.args`. Multi-word action names stay exact.

### Procedural instances

`tales.instances.generate(characterID, playerLevel, spec)` builds a private line of up to 20 rooms from a template pool. Encounters whose level band contains `playerLevel` are returned as a spawn plan and placed when a game is attached. A second character gets a different copy. The same character cannot hold two instances. Leaving to a non-clone room destroys the line, and a timeout (default 30 minutes) destroys only these generated instances. Authored cellar graphs are unchanged. Every generated exit is marked so Party Follow does not cross it. The generator does not spend a resource and does not move the character; the room-action script does. A timeout still ends a fight in that copy without a defeat penalty and moves the character to the return room before the copy is deleted. A disconnect does that only when `combat.disconnect` is `release`.

### CopyOnPickup Tracking
```go
// Character methods:
HasCollectedCopyItem(templateID string) bool
MarkCollectedCopyItem(templateID string)

// Stored in Flags["collected_copy_items"] as array of template IDs
```

### AI Character Generation (Swappable LLM Provider)
**Feature**: Character creation can generate names and descriptions through any LLM provider behind the `llm.Client` contract (`Complete(ctx, systemPrompt, userPrompt, temperature)`), with a local **Ollama** daemon as the default provider and hosted **Groq** as the alternative.

**Provider selection** (`LLM_PROVIDER`, via `llm.NewFromEnv()`):
- `ollama` (default) — local daemon at `OLLAMA_BASE_URL` (default `http://localhost:11434`), model `OLLAMA_MODEL` (default `llama3.1`), HTTP timeout `LLM_TIMEOUT_SECONDS` (default 60). No API key.
- `groq` — hosted API (`llama-3.3-70b-versatile`), requires `GROQ_API_KEY`.
- `none` / `off` / empty — generation disabled.
- Unset — legacy behavior: Groq when `GROQ_API_KEY` exists, otherwise Ollama.
- Unknown value — generation disabled.

**Behavior**:
- Endpoint: `POST /api/generate/character` (auth required; guests rejected; per-user rate limit of 10 requests/hour).
- `generate` field: `"name"`, `"description"`, or `"both"`.
- Requests send system + user prompts with the template hints (archetype, race, class, backstory) wrapped in an untrusted-input guard.
- No provider configured → `503 {"error": "AI generation is not configured"}`; provider failure → `500`.
- Handler depends only on the `llm.Client` interface (`handler.GenerateHandler.LLMClient`), so providers are interchangeable without handler changes.

---

## Multiplayer Session & Social System

### Live Session Presence
The game engine keeps an in-memory live session registry keyed by connected
user ID. Each live session tracks the selected character, character name,
current room, and last-seen timestamp.

Uses:
- Room player lists in `enterRoom` and `roomUpdate` WebSocket messages
- Room chat and room-based message fan-out
- `who` command output
- `tell`/`whisper` target lookup
- Passive regeneration ticks
- Periodic cleanup of stale `Room.Characters` entries

`User.IsOnline` remains persisted for administrative/status visibility, but
live routing and room presence use the session registry so stale database flags
do not make disconnected players appear reachable.

### Character Selection Lifecycle
On WebSocket connect, the game attaches the user's last character if available
or auto-selects the first owned character. Selecting another character with
`sc <name>` removes the previous character from its room, updates
`User.LastCharacter`, refreshes the live session, sends `characterSelected`,
then sends the current room, character stats, inventory, and quest log.
Character switch, room movement, and disconnect paths emit silent room presence
refreshes so other clients update player lists without waiting for a full room
render.

### Room Presence WebSocket Message
```json
{
  "type": "roomPresence",
  "roomId": "room-uuid",
  "players": [
    { "id": "character-uuid", "name": "Aster", "isYou": false }
  ]
}
```

The server broadcasts this to rooms with online active characters only. The
client marks `isYou` locally based on the currently selected character because a
single broadcast is shared by multiple users.

### Global Chat Channels (Phase 4.6)

Server-wide chat channels broadcast to every connected client. Supported
channels: **general**, **trade**, **lfg**, **help**, **roleplay**, **admin**.

```bash
chat general Anyone around?      # Canonical form for all channels
chat trade WTB iron ore
general hello                    # Direct shortcut: general, lfg, roleplay, admin
lfg need a healer for the cellar
```

`help` and `trade` are deliberately **not** registered as direct shortcuts —
the existing `help` command and the merchant `trade` alias keep those words.
Use `chat help ...` / `chat trade ...` for those two channels. Unknown first
words (e.g. `foo hola`) are not intercepted and fall through to room chat.

Each line is pushed with `Audience: MessageAudienceGlobal` as a normal
`message` type (`[Channel] Name: text`). The payload carries an optional
`channel` tag (`general|trade|lfg|help|roleplay|admin`) and the play client
routes tagged lines (and `[Channel] …` prose from older senders) into its
**channel** chat tab with per-tab unread counts — `public/mud-client/src/game/chatTabs.js`
(`CHAT_TABS = say|tell|party|channel|system`) and `ui/ChatPanel.svelte`. The
**admin** channel is restricted:
`message.FromUser.IsAdmin()` must be true, otherwise the sender gets
"Admin channel is restricted." Backend only — `/pkg/mudserver/game/commands/channels.go`.
Player guide: `docs/user/channels.md`.

### Party Commands

`party create`, `party invite <player>`, `party accept`, `party decline`,
`party leave`, `party kick <player>` (leader), `party promote <player>` (leader),
`party list`, `party follow`, `party unfollow`, `party say <message>` (or `party <message>`),
`party loot [rule]`, `party readycheck`, `party ready`, `party notready`
(short alias `p` throughout; usage line: `partyUsage()` in
`pkg/mudserver/game/commands/party.go`).

**Loot rule** (`party_loot_ready.go` → `handleLoot`): bare `party loot` prints
the current rule (default `free`). Changing it is **leader only** and persists
on the Party row (`LootRule`), so it survives restarts. Accepted values:
`free`, `leader`, `roundrobin`, `needgreed` (the alias `need/greed` is
normalized to `needgreed`). The rule is pushed to the overlay as `lootRule` in
the `party` payload (`PartyMessage.LootRule`).

**Ready check** (`handleReadyCheck` / `handleReady`): `party readycheck` is
**leader only** and starts a 30-second window on the in-memory session registry
(`pkg/mudserver/game/sessions_ready.go`). Members answer with `party ready` /
`party notready`; each change pushes `[Party] Ready: n/m · Not ready: n/m` to
the party. There is no timer goroutine: expiry is evaluated lazily, so the first
answer *after* the window elapsed sends `[Party] Ready check over: n/m ready.`
and clears the check. Ready state is not persisted.

Party membership is persisted in the existing `Party` entity (SQLite JSON),
including `leaderCharacterId`. Creator is leader. Soft/hard cap: **5** members
(`entities.MaxPartySize`) enforced on invite and accept. Pending invites
remain in-memory on the game server and expire after **45 seconds** (both sides
get a clear timeout notice + `party_invite{pending:false}`). Guests may invite,
accept, decline, follow, and chat in parties (Friends still refuse guests).

Structured WebSocket payloads:
`party` `{inParty,partyId,partyName,leaderId,maxMembers,members[{id,name,online,level,class,portrait,isLeader}],following,lootRule}`
and `party_invite` `{pending,inviterName,partyId,expiresAt}` (clear with `pending:false`).

Client (Party invite popup + guest parties, cache-bust `?v=partyinvite1`): HUD Party
button opens a gold-bordered panel — party name title + `N/M members · K online`
subtitle; member rows with avatar/initial, You/Leader badges, class · level, online
pill; sticky action bar (Say primary, Invite secondary, Leave danger+confirm);
party-say strip (~8 lines); Create/Invite empty state; mobile bottom-sheet. A
pending invite opens a centered Accept/Decline popup with a countdown (not only a
terminal line). Friends rows use matching **Invite** outline. Leader sees Kick on
other members.

The same panel holds a **Loot** radio group (leader only; sends
`party loot <rule>`, read-only label for everyone else) and a **Ready check** control (leader
starts it, members answer with Ready / Not ready; the panel classifies the
`[Party] Ready check…` lines to show active/complete state) —
`public/mud-client/src/game/ui/PartyOverlay.svelte`, rule labels in
`public/mud-client/src/game/partyState.js`.

Room players overlay and Friends rows can invite online players. Guests see the
Party button and full party UI.

### Party Combat Assist (v1)
Same-room players can join an in-progress fight by `attack <npc>` on an enemy
already in combat. Joiners receive `combatStart` (BattleStage), initiative is
rolled, and turn order is rebuilt (current actor preserved). Cross-room join
is refused. Party membership is **not** required to join; when combat starts,
same-room online party members get a `[Party] … Type 'attack <enemy>' to join`
nudge. Out of scope: auto-pull without attack, following anyone except the
party leader, Flutter. Item Need/Greed is Party Need/Greed v1; the party loot
rule remains for free/leader/roundrobin auto-assignment.

### Party Loot & XP Share (v1)
On victory, gold and XP split equally among the living combatants **and**
online party members standing in the killer's room (persisted `CurrentRoomID`).
The killer is the first living combatant (the player who engaged). Offline
members and members in another room get nothing. A solo victor, or a fight
where no two recipients share a party, is unchanged: each living combatant
gets `total/n` and any remainder is dropped.

When two or more recipients are in the same party, leftover gold and XP
(`total % n`) go to the killer so nothing is discarded. The victory combat log
lists each recipient (`PARTY SHARE`), and every recipient gets a one-line
`[Party] Equal split…` toast. Quest kill credit stays with living combatants only.

**Item drops** use Party Need/Greed when 2+ eligible party members are present;
otherwise the party loot rule applies (`party loot`, default `free`):

- `free` — drops stay on the ground for normal `pickup` (unchanged legacy behavior).
- `leader` / `roundrobin` / `needgreed` — each drop is assigned at victory by
  `selectLootRecipient` (`pkg/mudserver/game/loot_party.go`), using the living
  players in the fight as the eligible list and the party/leader of the first
  living player: the party leader (even when they are not among the eligible),
  the next player in a rotation, or a random eligible player (`needgreed` is a
  random pick when the roll round does not apply). Every hand-out is announced as
  `[Party] <item> → <name> (<reason>)` and lands straight in that character's
  inventory; if the write fails the item falls back to the room. Assignment runs
  only when the killer's party rule is not `free` (`game_combat.go` builds
  `LootAssignmentContext`). Round-robin position resets per victory.

See `docs/user/party.md` for the player-facing command table.

### Party Need/Greed (v1)
When a victory has **2+ living same-room online party members** (same eligibility
band as Party Loot & XP Share), each **item** drop opens a Need / Greed / Pass
round instead of becoming free ground loot. Gold share is unchanged. Solo and
non-party kills still drop items to the ground.

Every eligible member gets a `need_greed` popup (item name + quality chrome,
Need / Greed / Pass, 30s countdown). Commands: `need`, `greed`, `pass`, and
`loot need|greed|pass`. Need beats Greed beats Pass; ties are uniform random
among the highest tier. The winner receives the item in inventory (or on the
ground at their feet if inventory is full). A short `[Party]` line announces who
won and Need vs Greed. If everyone Passes, or the TTL ends with no Need/Greed,
the item stays on the corpse/ground as free-for-all. The play roll panel is portaled above the battle stage, so Need, Greed, and Pass stay visible and clickable while the victory or defeat card is up. On narrow or short screens the roll sits at the bottom and the outcome card yields height.

Multiple drops from one kill resolve **one at a time**. Disconnect, leave room,
die, or leave/kick from the party mid-round counts as Pass. Still out of scope:
master-looter, bind-on-pickup, cross-room rolls, Flutter.

### Party Follow
`party follow` starts following the current party leader. `party unfollow` stops.
While following, a normal exit walk by the leader (`TakeExit`, exit type empty /
`normal` / `direction`, arriving at the authored target) moves followers who are
standing in the room the leader just left. Anyone left behind walks a path of
those same ordinary exits (up to 12 rooms) toward the leader: a late `party follow`,
the end of combat, or a reconnect. Followers in combat stay put until the fight
ends, then catch up. A disconnect keeps the flag; an offline body is not moved,
and the other side is told the follow is still on. Teleports, portals, bindstones,
script relocations, hidden exits (except the step just taken with the leader),
and private-instance crossings are not used for the chase. Guests, characters who
are not in a party, the leader, and anyone already in combat cannot start following.
Leaving the party, being kicked, the leader leaving, or a leadership change clears
the follow flag. The flag is in-memory on the game server. The party payload
includes `following` for the recipient. The reply is a `[Party] You are following <leader>`
line (party strip and room toast). Still out of scope: auto-join combat without
`attack`, and following a member who is not the leader. Item Need/Greed is
Party Need/Greed v1; the party loot rule remains for free/leader/roundrobin
auto-assignment.

### Friends (v1)
Per-character friends list stored as `Character.FriendIDs` (character UUIDs) in
the SQLite character JSON blob. Names are resolved at display time.

```bash
friend add <name>       # Add by live session name, or exact offline character name
friend remove <name>    # Remove from your list
friend list / friends   # Show Online/Offline from the live session registry
```

Rules: no self-add; idempotent add; guests are refused politely (and cannot be
added). Open add-by-name (not a mutual request flow). Presence: friends who
are watching you get a short "came online / went offline" line. The `friends`
WebSocket message carries `{id,name,online}` for the play client overlay.

Client: HUD group button (next to the character switcher) opens a Friends
panel — list, Tell (whisper), Remove, add-by-name. Room players overlay can
add the other player. Guests hide the HUD entry.

### Clans (Phase 4)

Persistent clan storage plus the `ClansService` business layer, the
player-level REST API under `/api/clans`, and the play-client **ClanPanel** are
implemented. There are still **no game commands** (`clan …`) and **no
WebSocket payloads** for clans — the panel talks REST only. Player guide:
`docs/user/clans.md`.

**Client** (`public/mud-client/src/game/ui/ClanPanel.svelte`): toggled by the
**`C`** key or the header shield button (`CharacterSwitcher.svelte`, hidden for
guests) from `clanPanelOpen` in `uiChrome.js`; `Escape`, the close button or an
outside click dismisses it. It loads `GET /api/clans/mine` on open (manual
**Refresh**, no polling and no push notifications) and drives every action over
`public/mud-client/src/api/clans.js`: create, invite by character ID, accept /
decline pending invites, kick, cycle ranks (leader only), deposit / withdraw
gold, disband, plus the member list with rank badges.

Entities (`pkg/entities/clan/`):

```go
type Clan struct {
    *entities.Entity
    Name, Tag      string    // Tag is short, e.g. "DRK"
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

type ClanMember struct {
    *entities.Entity
    CharacterID, ClanID string
    Rank         Rank        // member | officer | leader
    JoinedAt     time.Time
    Contribution int64
    LastOnline   time.Time
}

type ClanInvite struct {
    *entities.Entity
    ClanID, CharacterID, InviterID string
    Status     string        // pending | accepted | declined | expired
    CreatedAt, ExpiresAt time.Time
}

type ClanBankLedger struct {  // append-only
    *entities.Entity
    ClanID, CharacterID string
    Type    string            // deposit | withdraw
    Amount, Balance int64
    Note    string
    Timestamp time.Time
}
```

Persistence (`ClanRepository`, `pkg/repository/clans_sqlite.go`):

- Tables `clans`, `clan_members`, `clan_invites`, `clan_bank_ledger` use the
  standard `(id, data)` JSON pattern with `json_extract` indexes.
- **One clan per character:** `AddMember` rejects a character that already has a
  membership; a unique index on `(characterID, clanID)` additionally prevents
  duplicate rows.
- **Bank:** `Deposit`/`Withdraw` update `Clan.Bank` and append an immutable
  `clan_bank_ledger` row (type, amount, resulting balance, note, timestamp).
  Withdrawals fail on insufficient funds.
- Level/XP helpers: `AddXP`, `UpdateLevel`. Soft delete via `Active=false`.

Service (`ClansService`, `pkg/service/clans.go`):

- `CreateClan(actorCharID, name, tag, description)` — fails when the character
  already belongs to a clan; the founder joins as `leader`.
- `Invite(clanID, actorCharID, targetCharID)` — leader/officer only; rejects
  self-invites, characters already in a clan, and duplicate pending invites.
- `AcceptInvite(inviteID, characterID)` / `DeclineInvite(inviteID, characterID)`
  — the invite is located among the character's own invites by UUID; accepting
  adds the character with the default `member` rank.
- `Kick(clanID, actorCharID, targetCharID)` — leader/officer only; the leader
  can never be kicked.
- `ChangeRank(clanID, actorCharID, targetCharID, rank)` — leader only;
  `ParseClanRank` maps the wire names `member`/`officer`/`leader` to `clan.Rank`.
- `Deposit`/`Withdraw(clanID, actorCharID, amount)` — append-only ledger plus
  balance update; only the leader may withdraw.
- `Disband(clanID, actorCharID)` — leader only; removes members then deletes
  the clan. `ListMembers`, `GetPendingInvitesForCharacter` support the API.

REST API (`pkg/server/handler/clans.go`, all authenticated):

| Method + Path | Permission |
|---------------|------------|
| `POST /api/clans` | acting character owner |
| `GET /api/clans/mine?characterId=` | acting character owner |
| `POST /api/clans/:id/invite` | leader/officer of the clan |
| `POST /api/clans/invites/:inviteId/accept` | invite owner |
| `POST /api/clans/invites/:inviteId/decline` | invite owner |
| `POST /api/clans/:id/kick` | leader/officer; leader immune |
| `POST /api/clans/:id/rank` | leader only |
| `POST /api/clans/:id/deposit` | any clan member |
| `POST /api/clans/:id/withdraw` | leader only |
| `POST /api/clans/:id/disband` | leader only |

### Private Messaging / DMs (Phase 4)

Persistent private messaging with the `DMService` business layer, the
player-level REST API under `/api/dm`, and the play-client **DMsPanel**. The
`tell` command is separate and still in-memory (online players only, nothing
persisted); wiring `tell` to this layer comes later. Player guide:
`docs/user/dms.md`.

Entities (`pkg/entities/dm/`):

```go
type DMChannel struct {
    *entities.Entity
    ParticipantA, ParticipantB string  // canonical order, characterIDs
    CreatedAt     time.Time
    LastMessageAt time.Time
    LastMessageID string
    UnreadA, UnreadB int               // denormalized unread counters
}

type DMMessage struct {
    *entities.Entity
    ChannelID  string
    FromCharID string
    Body       string
    SentAt     time.Time
    ReadAt     *time.Time              // nil = unread
}
```

Persistence (`DMRepository`, `pkg/repository/dm_sqlite.go`):

- Channels are created **on demand** by `FindOrCreateChannel`: the two
  participant IDs are stored in canonical (sorted) order, so a pair of
  characters maps to exactly one channel regardless of who starts it.
- `dm_messages` is the one table that deviates from the pure JSON pattern. It
  also stores real `channel_id`, `sent_at` (Unix ms) and `from_char_id` columns
  plus indexes, so `FindMessagesSince(channelID, since)` runs
  `WHERE channel_id = ? AND sent_at > ? ORDER BY sent_at ASC`.
- Reads: `MarkMessageRead(id)` stamps one message; `MarkRead(channelID, reader)`
  zeroes the reader's unread counter and marks all messages they did not send as
  read.
- Helpers: `FindChannelsByCharacter`, `UpdateLastMessage`, `IncrementUnread`,
  `GetUnreadCount`, `FindMessagesByChannel` (latest N, oldest first),
  `DeleteMessage`.

Service (`DMService`, `pkg/service/dms.go`):

- `GetChannels(characterID)` — channels where the character is a participant.
- `FindOrCreateChannel(a, b)` — delegates to the repository, which canonicalizes
  the pair so `(a,b)` and `(b,a)` map to the same channel; self-DMs are rejected.
- `SendMessage(channelID, fromCharID, body)` — validates that `fromCharID` is a
  participant (anti-spoof), stamps `sentAt`, updates `lastMessageAt` and
  increments the recipient's unread counter.
- `GetMessages(channelID, characterID, limit, since)` — participants only;
  `since` returns messages after a timestamp (index-backed), otherwise the
  latest `limit` messages oldest-first.
- `MarkRead(channelID, characterID)` — participants only; zeroes the reader's
  unread counter and stamps received messages as read.

REST API (`pkg/server/handler/dms.go`, all authenticated; acting character must
be owned by the user, and the service enforces channel participation):

| Method + Path | Permission |
|---------------|------------|
| `GET /api/dm/channels?characterId=` | acting character owner |
| `POST /api/dm/channels` | acting character owner |
| `GET /api/dm/channels/:channelId/messages?characterId=&limit=&since=` | channel participant |
| `POST /api/dm/channels/:channelId/messages` | channel participant (anti-spoof) |
| `POST /api/dm/channels/:channelId/read` | channel participant |

**Client** (`public/mud-client/src/game/ui/DMsPanel.svelte` + fetch helpers in
`public/mud-client/src/api/dms.js`): toggled by the **`M`** key or the header
mail button (`CharacterSwitcher.svelte`, hidden for guests) from `dmsPanelOpen`
in `uiChrome.js`. While open it re-fetches the channel list every **5 seconds**
(`POLL_MS`), renders a per-channel **unread badge** from `unreadA`/`unreadB`,
and calls `POST …/read` for the channel being read. Starting a conversation
uses `POST /api/dm/channels` (find-or-create), messages load with
`limit: 100`, and sending posts to the channel. There is **no WebSocket push**
for DMs — the panel only sees new traffic on its next poll.
### Responsive notification and sheet behavior

Page layers are named in `public/mud-client/public/global.css`. Shell chrome stays below inventory, social sheets, and invite dialogs. Combat stays below NPC dialogs and Need/Greed. Inventory includes safe-area padding.

`mobileStore.MOBILE_QUERY` selects the Room/Console shell at widths through 768px and at landscape widths through 1024px when height is at most 500px. The home link is hidden for the same query. The room scrolls to its image, NPC cards, hotbar, and actions; Console remains a separate tab.

`QuestNotifications` and `RoomTextOverlay` defer presentation during active combat, an ending outcome, or a pending loot roll. Queues retain unseen entries. The first prioritized quest moment and up to four corner or room messages render at once. Reading clocks pause on removal from the rendered view and resume with the remaining time. A new examine replaces the previous examine and goes first. Continue stays outside the outcome rewards scroll area. Starting, ending, clearing, or dismissing combat cancels the prior outcome timeout. Compact hotbars keep their content height inside the scrollable room. Short landscape uses `--entity-card-width` to fit NPC cards inside the hero image. Short landscape puts the outcome and roll in separate horizontal lanes.

The next-step guide hides over inventory, party, friends, recipes, and map sheets, and hides when no room hotbar is visible. Scroll events clear its old placement before the next frame so it cannot cover newly scrolled hotbar slots. Notifications and mobile sheets honor the system reduced-motion preference.

### Client Session UX
The MUD client exposes connection state in `MUDXPlusStore`:
`disconnected`, `connecting`, `connected`, and `reconnecting`.
`Game.svelte` owns reconnect scheduling and retries automatically after socket
close. `CharacterSwitcher.svelte` shows the active character and connection state.
**Switch character** in the account menu (desktop chip and phone header) opens
`CharacterPicker.svelte`, which lists every character from `/api/my-characters`.
Choosing one sends `sc <name>`. A signed-in player with more than one character
sees that picker once per login; the server still enters on `lastCharacter`.
Guests get **Continue with X**, **Continue with Google**, and **Email and password**
(Auth0 `loginWithRedirect`; X and Google pass `connection` so the password form
is not the default). The Auth0 buttons apply when `AUTH_MODE=auth0`; in the
default local mode the server exposes `POST /api/auth/register` and
`POST /api/auth/login` for username/password accounts. **Log out** clears the Auth0 session and this tab's guest token and
returns to the welcome choice instead of restoring a guest. `Client.js` handles `roomPresence` messages and
updates `MUDXPlusStore.players` without changing the room description.

---

## NPC System

### NPC Entity Structure
```go
type NPC struct {
    *entities.Entity
    traits.BelongsUser
    traits.CurrentRoom

    Name, Description string
    Race  Race
    Class Class

    CurrentHitPoints, MaxHitPoints int32
    Level int32

    // Template/Instance Pattern
    IsTemplate     bool    // Blueprint vs spawned instance
    TemplateID     string  // Source template for instances
    InstanceSuffix string  // Unique ID suffix

    // Behavior Configuration
    SpawnRoomID  string         // Where NPC respawns
    RespawnTime  time.Duration  // Time to respawn (0 = no respawn)
    WanderRadius int            // Rooms to wander from spawn
    PatrolPath   []string       // Room IDs for patrol route

    // State Machine
    IsDead    bool
    DeathTime time.Time
    State     string  // "idle", "combat", "patrol", "dead", "fleeing"

    // Combat State
    InCombat         bool
    CombatInstanceID string

    // Behavior Traits
    EnemyTrait    *EnemyTrait
    MerchantTrait *MerchantTrait

    // Dialog References
    DialogID          string          // Main dialog tree
    IdleDialogID      string          // Ambient chatter
    IdleDialogTimeout time.Duration   // Idle dialog frequency
    LastIdleDialog    time.Time       // Last ambient chatter trigger

    // Free-form LLM dialogue opt-in (Phase 6.3, cortes 1–3, Batches 25–27; nil = legacy NPC)
    LLM               *LLMConfig      // see "NPC free-form dialogue" below
}
```

Field details (`LLMConfig` in `pkg/entities/npcs/llm.go`, including the structured Big5
personality/goals/secrets/relations added in Batch 26) and the `ask` command that consumes
them are documented in the Phase 6 section, under
["NPC runtime dialogue (`ask`, FASE 6.3 cortes 1+2+3 — Batches 25–27)"](#phase-6-llm-integration-design-and-foundation).

### NPC Behavior Loop
NPCs are processed by the game update loop every 10 seconds:
- Idle/patrol NPCs use the optional `movement` block (`mode: route|roam`, `route`, `loop: loop|pingpong`, `zones`, `rooms`, `intervalSeconds`, `jitterSeconds`, `pauseOnEngageSeconds`, `leaveMessage`, `arriveMessage`). A spawner may replace the whole block. Timing defaults to 90 seconds, jitter defaults to 0 and adds uniform 0–jitter seconds, and engage pause defaults to 120 seconds. Timings are integers in 0–86400; zero uses the timing defaults except jitter. The first move is scheduled on registration. Ticks round execution up by at most 10 seconds.
- `PatrolPath` without a movement block maps to a loop route in idle or patrol state. `WanderRadius` without a block maps to random roaming within that many permitted exit hops of spawn. A movement block takes precedence.
- Movement is strictly opt-in per NPC. No block, positive radius or path means stationary, including shop/dialog NPCs and patrol-state NPCs. Spawners inherit explicit template configuration and add no movement defaults.
- Route steps, loop closure and both directions of pingpong must have adjacent ordinary exits. Invalid configuration is reported by the `npc-movement` health rule / `tales -check` and skipped at runtime. Walking excludes hidden, locked (typed flag or `params.locked`), instance and teleport exits, instance rooms/copies, and rooms tagged `noNPC`/`no-NPC` or `noWander`/`no-wander`. Zone and room whitelists intersect.
- Talking, looking/examining, targeting, shop/trade/buy/sell refresh the per-instance pause. A dialog or shop session holds movement until the dialog ends, the shop closes, the player leaves the room, changes characters, or disconnects. An authored exit (an option or answer with `isDialogExit`, or an option with no further choices) sends `dialogEnd` and releases the hold. The web dialog panel shows the portrait, name, speech, and those choices; it has no separate leave button and closes on `dialogEnd`. Shop close and Escape still send `shopclose`. Other clients may send `dialogclose` or `shopclose`. A client that never ends the dialog keeps the hold until the player leaves or disconnects. NPC movement is serialized with commands. Combat/dead NPCs never walk.
- Runtime `nextMoveAt`, `pausedUntil` and pingpong direction are memory-only. On registration an invalid current room resets to an allowed spawn/first route room; no valid start means no walking. Spawns and residents return to authored placements after restart. Aggressive wanderers must be non-boss and level >= 10; arrival uses normal aggro scheduling and room sanctuary rules.
- The NPC inspector shows static movement and live current room, next move time/ETA, paused-until and session hold; `/api/live/npcs` adds these fields. The world map has a Wanderers layer from live positions (refresh to update).
- NPCs with `IdleDialogID` and `IdleDialogTimeout` broadcast ambient chatter to their current room when the cooldown has elapsed.
- Dead spawned instances are removed for spawner replacement. The spawner waits `respawnDelay`, and at least `combat.respawn_min_seconds`, after that death. Dead unique NPCs respawn at `SpawnRoomID` after `RespawnTime`.
- An enemy with `aggroOnSight` engages a player who enters its room, and engages players already there when it spawns, respawns, or walks in. `config/ruleset.yaml` `combat.aggro_on_sight` defaults to on, 2.5s grace, level gap 5, and 15s reaggro cooldown. That quiet period also covers other aggressive NPCs still in the room. A fresh respawn uses `respawn_grace_seconds` (default 8) for players already there. After a victory or flee, `post_combat_grace_seconds` (default 10) holds the fight room, and ends when the player leaves. `spawn_aggro_cooldown_seconds` (default 60) is per player and spawn, so a replacement remembers the same player. `enabled: false` stops sight aggro. The sight line is ordinary text. The fight then uses the same path as `attack`. A room with `noAggro` skips both the grace watch and the fire. Look and examine of an `aggroOnSight` NPC include the line `Aggressive`.
- NPC movement sends ordinary room prose to both rooms and existing `roomUpdate` payloads so web/Flutter refresh presence without reprinting the room description. Messages default to `{name} heads {dir}.` and `{name} arrives from the {fromDir}.`; direction comes from the real exit.

Room NPC payloads sent to the MUD client include `isEnemy`, `isMerchant`, `isQuestGiver`, `hasDialog`, `hasIdleDialog`, `aggroOnSight`, and `state` so the UI can show interaction badges without duplicating backend lookup rules. An `aggroOnSight` enemy card shows a red Aggressive label. The `aggroOnSight` JSON field is omitted for passive NPCs. `noAggro` survives room import/export and instance cloning; it suppresses automatic aggression without blocking a manual attack.

### NPC Factory and Level Scaling (client, Batches 16/19)

- **NPC factory panel (key `N`):** `public/mud-client/src/game/ui/NpcFactory.svelte` + pure module `public/mud-client/src/game/npcFactory.js` (templates, range validation, backend payload mapping). State persists in `localStorage` (`mud-client.npc-factory.v1`) and, when a creator token is present, syncs best-effort against `/api/npcs/templates` (pull) and `POST/PUT /api/npcs` (push) through `src/api/npcs.js` (`NPC_CRUD_AVAILABLE = true`); without a token everything stays local.
- **Level scaling:** `public/mud-client/src/game/npcBehavior.js` → `scaleTemplate(template, targetLevel)` applies deterministic rates over the current base (HP ×0.15/level, damage ×0.10, XP ×0.25, gold ×0.20) and records `scaling.targetLevel`. Used by the factory's preview (`NpcFactory.svelte`), not by the server.
- **Behavior model:** the same module exports a serializable state machine (states + aggro + abilities) and `toLuaBehavior()` as **preview/export only** — nothing in the client executes it. Tests: `npcBehavior_test.mjs`.
- **Storage (FASE 5.3, working tree):** table `npc_templates` (migration `migrateBatch20NPCTemplates`), entity `pkg/entities/npcs/templates.go` (`NPCTemplate`, with `Behavior`/`Scaling` kept as `json.RawMessage` so the frontend payload round-trips) and `Factory.NPCTemplates()` (`pkg/repository/npc_templates_sqlite.go`, indexes on `name`/`role`). **No service/endpoint consumes it yet**: `GET /api/npcs/templates` still reads the `npcs` collection filtered by `isTemplate`.

### Enemy Trait
```go
type EnemyTrait struct {
    // Classification
    CreatureType string  // "beast", "humanoid", "undead", "elemental", "construct", "demon", "dragon", "aberration"
    CombatStyle  string  // "melee", "ranged", "magic", "swarm", "brute", "agile"
    Difficulty   string  // "trivial", "easy", "normal", "hard", "boss"

    // Combat Stats (base values, modified by difficulty multipliers)
    AttackPower  int32
    Defense      int32
    AttackSpeed  float64 // attacks per round; 0 or omitted is one swing and the old beat

    // AI Behavior
    AggroRadius   int     // Detection range in rooms (0 = passive)
    AggroOnSight  bool    // same-room engage after combat.aggro_on_sight grace
    CallForHelp   bool    // Alert nearby enemies
    FleeThreshold float64 // HP % to flee (e.g., 0.2 = flee at 20% HP)

    // Rewards
    XPReward       int64
    GoldDrop       Range{Min, Max int64}
    LootTableID    string    // Reference to loot table
    GuaranteedLoot []string  // Item template IDs that always drop
    MaxDrops       int32     // Max items from loot table (0 = unlimited)

    // Event Scripts (once per fight, sandboxed; errors are logged and swallowed)
    OnAggroScript      string  // when this NPC enters the fight
    OnDeathScript      string  // when this NPC dies, before loot and XP
    OnFleeScript       string  // when this NPC first chooses to flee
    OnLowHealthScript  string  // first time HP drops below LowHealthThreshold while still alive
    LowHealthThreshold float64 // fraction of max HP; 0 or unset = 0.30; clamp to (0, 1)
}
```

### Merchant Trait
```go
type MerchantTrait struct {
    MerchantType   string  // "general", "blacksmith", "alchemist"
    Inventory      []MerchantItem
    RestockMinutes int32
    LastRestock    time.Time
    BuyMultiplier  float64  // Price when buying (1.0 = normal)
    SellMultiplier float64  // Price when selling (0.5 = half)
    AcceptedTypes  []string // Item types merchant will buy (empty = all)
    RejectedTags   []string // Tags preventing buy (e.g., "soulbound", "quest")
}

type MerchantItem struct {
    ItemTemplateID string
    BasePrice      int64   // Override item base price
    PriceOverride  int64   // Force specific price (ignores multipliers)
    Quantity       int32   // Current stock (-1 = unlimited)
    MaxQuantity    int32   // Max after restock
    RequiredLevel  int32   // Player level requirement
}
```

Merchant commands are available in rooms with merchant NPCs:
- `list`, `shop`, or `trade` (exact key — no NPC name) opens a structured `shop` WS payload for the client overlay and refreshes after buy/sell
- `buy <item|number> [quantity]` purchases stock. A bare number is the 1-based catalog row. Stackable quantities can fit in one inventory stack. `sell` accepts the same number for items that merchant will take.
- `sell <item> [quantity]` sells accepted, unbound inventory items
- `value <item>` / `price <item>` checks the merchant's sell price

Web/Flutter clients inject a **Trade / Shop** dialog option when talking to a merchant (`isMerchant`), open a room-widget shop overlay (2-column horizontal buy/sell rows with icon + name + price, paginated), and send bare `list` (not `trade <name>`). Shop/inventory icons use `/api/item-art/{templateId}.png` (never `meta.img` prose prompts), then generic PNG, then local SVG.

Trading is blocked while the character is in combat. Merchant stock can restock lazily when a player interacts after the configured interval.

### NPC Spawner System
```go
type NPCSpawner struct {
    *entities.Entity

    TemplateID    string         // NPC template to spawn
    RoomID        string         // Where to spawn
    MaxInstances  int            // Max alive at once
    SpawnInterval time.Duration  // Time between extra spawns above InitialCount
    InitialCount  int            // Spawn on world load
    RespawnDelay  time.Duration  // Wait after a death before replacing

    RespawnTimeOverride *time.Duration  // Override template respawn
}
```

**Spawner Behavior** (automated by server):
- Checks every 5 seconds
- Fills `InitialCount` immediately when the spawner has not lost an instance yet
- After a death, waits the longer of `RespawnDelay` and `combat.respawn_min_seconds` (default 30s, `0` disables the floor)
- `SpawnInterval` spaces only the extras between `InitialCount` and `MaxInstances`
- Tracks live instances via `NPCInstanceManager`

### NPC Resident System
Two methods for placing NPCs:
1. **Via spawners** - Dynamic, respawning instances
2. **Via Room.NPCs list** - Static residents
3. **Via NPC.CurrentRoomID** - Unique NPCs auto-spawn into their assigned room on server start

**Important**: `Room.NPCs` contains template/unique NPC IDs, NOT instance IDs

---

## Combat System

### Joining in-progress fights (Party Combat Assist v1)
Same-room players may `attack <npc>` to join an active combat instance that
NPC is already in (`JoinCombat`). Cross-room joins are refused. Living joiners
share victory XP/gold. Online party members in the killer's room are added to
that equal split (Party Loot & XP Share v1); leftover goes to the engager when
the split is a party share. Loot remains room drops.

### Combat Instance Model
```go
type CombatInstance struct {
    ID              string
    OriginRoomID    string  // Room where combat started

    Players         []CombatantRef
    Enemies         []CombatantRef

    TurnOrder       []CombatantRef  // Initiative-sorted
    CurrentTurnIdx  int
    TurnStartTime   time.Time
    Round           int

    State           CombatState  // "pending", "active", "victory", "defeat", "fled", "timeout"
    Log             []CombatLogEntry
}
```

### Combatant Reference
```go
type CombatantRef struct {
    ID, Name        string
    Type            CombatantType  // "player" or "npc"
    Initiative      int
    IsAlive, HasFled bool

    // Core Stats
    Level           int32  // Snapshot used for level-gap modifiers
    MaxHP, CurrentHP int32
    AttackPower, Defense int32
    STRMod, DEXMod, CONMod, INTMod, WISMod int
    DefenseBonus    int32  // From defend action

    // Mana & Skills
    MaxMana, CurrentMana, ManaRegen int32
    EquippedSkills []string
    SkillCooldowns map[string]int  // SkillID → rounds remaining
    StatusEffects  []StatusEffect
    QueuedSkillID  string
}
```

### Status Effects
```go
type StatusEffect struct {
    ID, SkillID, Name string
    Type    string    // "buff", "debuff", "dot", "hot", "stun"
    Stat    string    // "attack", "defense", "dodge"
    Value   int32     // Flat modifier
    Percent float64   // Percentage modifier (0.30 = +30%)
    Duration int      // Rounds remaining
    SourceID string   // Caster ID
}
```

### Combat Flow
```
1. INITIATION
   - Player: attack <npc>
   - NPC: aggroOnSight, same room, after combat.aggro_on_sight grace (AggroRadius is not a leash)
   - Create CombatInstance, roll initiative (1d20 + DEX mod)

2. TURN ORDER
   - Sorted by initiative (highest first)
   - Each combatant takes action in sequence
   - Round increments after all combatants act

3. PLAYER ACTIONS (60-second timer per turn)
   - attack <target> - Melee/ranged attack
   - cast <skill> [target] - Use skill (mana/cooldown)
   - defend - +50% defense until next turn
   - flee - Attempt escape (50% + DEX bonus)
   - timeout - Auto-defend after 60 seconds

4. NPC ACTIONS (automated AI)
   - If HP < FleeThreshold → attempt flee
   - Otherwise → attack weakest player

5. ROUND START EFFECTS
   - Process DoT/HoT ticks
   - Regenerate mana (1 + WISMod per round)
   - Decrement cooldowns
   - Check stun effects

6. RESOLUTION
  - Victory (all enemies dead) → XP, gold, loot
  - Defeat (all players dead) → 10% XP loss, 1 gold loss, respawn at bind point
  - Fled (all players escaped) → NPCs reset to idle
  - Timeout (30 minutes) → Combat ends, no rewards
```
If a character remains flagged as in combat after the runtime combat instance is gone, combat commands clear the stale flag and return the normal not-in-combat response.

### Combat Commands
```bash
attack <target>    # Attack enemy or switch target
a <target>         # Alias

cast <skill> [target]  # Use skill
spell <skill>          # Alias
1, 2, 3, 4             # Quick-cast slot shortcuts

defend             # Defensive stance (+50% defense)
d, guard           # Aliases

flee               # Attempt to escape
run, escape        # Aliases

status             # Show combat status
cs, combat         # Aliases
```

### Difficulty Balance System
**Config File**: `config/combat_balance.yaml`

```yaml
difficulty_multipliers:
  trivial:
    hp: 3.75
    attack: 4.0
    defense: 1.0
  easy:
    hp: 2.9
    attack: 1.67
    defense: 1.0
  normal:
    hp: 2.67
    attack: 1.5
    defense: 2.0
  hard:
    hp: 1.25
    attack: 1.18
    defense: 1.25
  boss:
    hp: 2.0
    attack: 1.5
    defense: 1.67
```

**How it works**:
- Enemy base stats (HP, Attack, Defense) are defined on `EnemyTrait`
- On combat initiation, stats are multiplied by difficulty tier
- Allows fine-tuning balance without editing all NPCs

### Level-gap modifiers
**Config**: `level_gap` in `config/combat_balance.yaml` (defaults in `pkg/mudserver/game/balance/level_gap.go`).

Signed gap = attacker level − defender level, clamped (default ±6). Gap 0 leaves hit, crit, and damage unchanged.

| Knob | Per level of attacker advantage | Where it applies |
| --- | --- | --- |
| `hit_chance` | +3.5% (about +1 on a d20 every one or two levels) | Basic attacks add it to the d20 roll. Natural 1 always misses, natural 20 always hits. Skills have no armor class: a negative delta is a per-hit miss chance. |
| `crit_chance` | +1% | Added to the 5% natural-20 base on basic attacks. Skills have no base crit; only a positive delta can crit (2×). |
| `damage_dealt`, `damage_taken` | +3.5% and +2% | Multiplied together after defense and before crit, then clamped (default 0.40–1.80). Same multiplier on basic attacks, skill hits, and DoT ticks (scaled when the DoT is applied). |

Players and NPCs both use `CombatantRef.Level`, copied from the character or NPC at combat start. A missed damage skill does not apply its secondary effect or Shield Bash stun. Numbers stay in config so the engine stays world-neutral. Feel targets: an enemy 3 levels up is hard in level-appropriate gear and fair in good gear; +5 is a skull fight; −3 or lower feels trivial.

### Class balance
**Config**: `class_balance` in `config/combat_balance.yaml` (defaults in `pkg/mudserver/game/balance/class_balance.go`).

After the level-gap multiplier and before a crit, `damage_dealt` scales hits that class lands and `damage_taken` scales hits that class receives. `behind_dealt` multiplies `damage_dealt` again when that class is the lower level. Class id `wizard` uses the `mage` row. A missing class or a multiplier of 1 leaves that side unchanged. A loaded class pack overrides this file. Ward's pack row takes damage at 1.00, so a soak hit is not larger than the unscaled blow, and the template's stamina is what grows the later hit-point pool. Grit is still gained from a connecting hit. A soak class's Slam prints a miss and starts its cooldown only when the swing hits. The level-10 gap table uses this so warrior, rogue, ranger, and mage share one band: at-level bosses about 50–65%, and a good-gear boss three levels up about 50%.

### Early-game curve
**Config**: `early_normal` and `newbie_curve` in `config/combat_balance.yaml` (defaults in `pkg/mudserver/game/balance/difficulty.go`).

`early_normal` multiplies the normal-tier factors again, before the integer truncate, when the enemy level is in 1..`max_level` (6). Other tiers, named overrides, and normal enemies above that level stay on the tier table. A level-2 Sample Thug authored at 18 HP / 4 ATK / 1 DEF is `18 * 2.7 * 0.42 = 20` HP and `4 * 1.35 * 1.45 = 7` ATK. Level 0 (the multiplier helper with no level) still returns the old 48 / 5 / 1, so a caller that does not know the level does not invent the curve.

`newbie_curve` applies only while that combatant is level 1–5. It does not change `class_balance` and does not move the level-10 gap table. Mage damage dealt is ×1.15 and damage taken is ×0.32. Rigger damage dealt is ×1.30 and damage taken is ×0.95. When the attacker is at least `uphill_gap` (2) levels higher and is itself level 6 or below, damage taken is multiplied again by that class's `uphill_taken` (warrior 2.2, rogue 1.5, ward 1.5, mage 1.0, rigger 1.0). A level-9 bounty spider does not get the uphill factor. Import stores the scaled stats once. The content-health NPC export writes the authored numbers back into the importer fields and adds a `combat` block with authored and effective HP, attack, and defense. See `docs/COMBAT-BALANCE.md`.

### Boss telegraph and enrage
**Config**: `boss_mechanics` in `config/combat_balance.yaml`.

Bosses and elites (`hard`) spend `telegraph_turns` actions winding up `telegraph_label` before that hit lands. BattleStage shows a banner and pulses the nameplate for that window (`telegraph_ms`). The resolving hit carries `ability` (Crushing Blow) so the nameplate flash and the floating number are heavier than a normal crit. Bosses enrage after `enrage_after_rounds` or at `enrage_below_hp`, gain an Enraged badge, hit for `enrage_damage`, and stop starting new wind-ups. Trash does not wind up. Elites do not enrage. A miss floats the word "miss". `prefers-reduced-motion` leaves the number in place and skips the flash.

Boss phases are configured by `boss_mechanics.phase_tiers` (defaults to bosses only) and `phases`, an ordered list of `label` / `below_hp` bands. The opening band must be 1.0; later thresholds must descend and remain above zero. Current defaults are Opening (100%), Escalation (66%), and Last Stand (33%). Omit the list to disable phases; explicitly add `hard` to opt elites in. Phase state is per enemy and per encounter (`bossPhase`, `bossPhaseLabel`, `bossPhaseCount`), advances once at or below each threshold on attacks, skills, or DoT, and never rolls back after healing. Large nonlethal hits emit every crossed threshold; lethal hits do not announce a phase.

A phase can override `telegraph_label`, `damage_dealt`, and `enrage_damage`; omitted values inherit the global behavior (phase damage defaults to 1). Phase damage multiplies after class/level scaling and before crits. An enrage override replaces the global enrage multiplier. A6 enrage still triggers at round 16 or 30% HP in any phase and cancels/skips wind-ups. A wind-up already in progress retains its original ability label across a phase transition unless enrage cancels it. Each phase entry sends a structured `combatAction` with action/result/fxId `phase-enter`, human text, and current combatant snapshots to all living participants. BattleStage shows a four-second phase banner and a persistent phase number/name under each boss nameplate; reduced motion disables the arrival animation. Late joiners see the current phase without replaying a transition. Group roster and contextual focus/restore behavior are preserved.

### Threat colors
`threat` in `config/combat_balance.yaml` maps `(enemyLevel - playerLevel)` to `grey / green / yellow / orange / red / skull` (defaults: ≤ −3 grey, −2..−1 green, 0..+1 yellow, +2 orange, +3..+4 red, ≥ +5 skull). The tier is on the room NPC payload (`threat`) and on combat enemy views, computed for the viewer. Room cards and BattleStage nameplates use that color; skull enemies also show ☠. `attack` on orange, red, or skull warns once ("X is much stronger than you") and does not engage. `attack!` or a second `attack` on that enemy does. The room Attack button confirms, then sends `attack!`.

In combat, the gold nameplate and portrait ring mark the local player's focus target. Clicking a living enemy portrait/nameplate or cycling with Tab sends `focus <enemy ID>` to update the engine's auto attack aim without queuing an attack. `attack <name>` also changes focus and queues an attack; the BattleStage Attack button sends the exact enemy ID so duplicate names are unambiguous. The combat start payload identifies the engaged enemy, including for players joining an existing fight. Basic attacks and untargeted hostile skill commands use the focused living enemy; an invalid or defeated focus falls back to the first living enemy. Selecting a different orange, red, or skull foe shows "X is much stronger than you." in its threat color for 3.2 seconds; clicking the same foe does not repeat it. The only hostile is selected automatically, and ending combat clears focus. The warning banner has no motion.

### Equipment and inventory item cards

Clicking or pressing Enter on an equipped paper-doll slot opens the shared item card. Right-click opens it too. The item stays equipped until the Unequip button is pressed; Escape, Close, or clicking outside closes the card. Inventory tiles and rows open the same card with explicit Equip, Use, Use on… (tools/fire-starters), Examine, Sell, and Drop actions. Use on… picks another inventory item and sends `use A on B`; bare Use still works (flint lights a carried torch). The card shows art, rarity, slot, stats, description, value, and weight when the item supplies it. Inventory cards compare stat differences with equipped gear; rings use the weaker worn ring, an empty ring slot counts as a full gain, and two-handed weapons compare with both hand slots. A green arrow marks a clear class-relevant, usable upgrade. Class tags, item level, and explicit armor-weight metadata can block equipping; the card explains the requirement and disables Equip. The `equip` command enforces the same requirements, including when typed in the terminal.

### Viewport layout presets
Combat start/join promotes the existing full-screen BattleStage cover using the layout focus/save contract. The widgets and terminal remain mounted with their prior geometry and active tabs. Victory/defeat dismissal, outcome timeout, and combatLeave restore the previous arrangement, including a manually focused panel; viewport fitting resumes on restore. Save and Save as template retain the normal arrangement during combat. Keyboard focus moves to the stage only when no command input or other text field is active, and returns to the prior control when the stage closes unless the player has focused another field. The combat stage fits Compact and phone viewports without document scrolling.

On phone, tablet, and short landscape, Stats, Equip, Skills, and Quests open the same Character, Equipment, and Quest Log widgets inside the bottom sheet. Skills is the Character widget opened on the skills tab, including Add to hotbar. A quest tap opens one framed popup with the description, objectives and progress, area, turn-in room, rewards, and turn-in hint when the log entry has those fields. Tap outside the sheet or the close button dismisses it. The sheet band stops at the header and the visible hotbar, so those controls stay clickable. The room hotbar wraps so each slot is at least 44px and at most 56px. Header, badge, console input, font step, and sheet controls use the same 44px hit area. Phone console body text is at least 14px. Typing the same words in the console still sends them. The desktop grid is unchanged. Play cache tag `?v=v056`.

The Settings panel opens from the account chip and Escape closes it. `interface.battleLayoutB` defaults to true (Layout B). Turning it off selects Classic cards and stores `talesmud_battle_layout_b=0`; a missing key is not an opt-out. `interface.combatAutoFocus` defaults to true; turning it off keeps the room layout visible on combat start/join and provides an explicit Open BattleStage control during active combat. That control uses the same transient focus/restore behavior. `interface.reducedMotion` defaults to `system`; `on` suppresses BattleStage phase, hit, ally, and loot animations plus map ambience, while `off` permits them even when the OS requests reduced motion. `interface.inventoryOpenMode` chooses the existing overlay or layout widget and applies immediately. These choices persist in local storage, not server settings. The General tab labels audio as coming soon because no game audio path consumes the stored sound fields. The old Compact Mode and Room Text Overlay fields remain readable for existing local settings but their controls are hidden because they have no active presentation consumer.

With no usable saved layout, the play client picks Compact (under 1100px wide, room stacked over the terminal), Desktop, or Wide from the window size, and sizes the grid so the widgets fit the viewport height with no page scroll. Empty, unknown, malformed, or wholly hidden saved grids fall back to that preset. Desktop and Wide put the room on the left. On the right, Character and Equipment share one tab container (Character open), Terminal, Quest Log, and Map share another (Terminal open), and Inventory sits under those tabs, with the action bar across the bottom. Nothing in that preset crosses the 24-column grid or another widget. The room scene shrinks to share the panel with the description, and a long description scrolls inside the room. Panel padding and the character sheet are tight enough that attributes and combat stats fit in the Character tab at 1080p. Compact stays a stack and does not add the sheet. A saved layout that is taller than the window is scaled down for display (the action bar stays on the bottom row); Save still writes the player's unscaled rows. Compact and phone may still scroll. The spell bar is docked in the top of the action bar instead of a separate row and has nine slots. Keys 1–9 fire those slots, Tab cycles living combat targets, Escape closes the top open panel (shortcut list, dialogs, map, inventory, the battle outcome, the account menu, a focused widget, then edit mode), and `?` opens the shortcut list. Those keys do nothing while the command line or any text field is focused. Focusing a widget still covers the grid, and Save stores the arrangement from before that cover. Resize reflows that preset. A saved layout is kept, clamped back onto the 24-column grid (minimum 2×2, nothing past the right edge), and scaled vertically when it is taller than the window. A saved full-width spell bar that sits directly on the action bar is folded into that dock on load; a spell bar placed somewhere else stays its own widget and can still be moved in edit mode. Edit mode can switch Compact / Desktop / Wide without deleting a saved layout until Save. Guests open the same editor from the account menu. The toolbar has multi-step Undo, Reset, and a Lock toggle that keeps edit mode open but stops dragging and resizing. Corner handles stay visible while the layout is unlocked, and a gold ghost shows where a widget will land. A guest token in this tab is restored after a reload, so crossing into a mobile-emulation reload does not dump the session back to the welcome screen. Panels share one header (title, collapse, focus) in the same type and padding; the inventory overlay keeps a single title. A tab container uses that same single row: the tabs sit on the left, and collapse and focus stay on the right. Extra tabs scroll sideways and, past three, also open from a More menu. There is no separate "Tab container" title. The account chip, Edit Layout, and the Party and Friends buttons share one header row in that top band: same height, gold border, and gold hover. Edit Layout is its own button until the window is under 1100px wide, where it moves into the account menu. The menu is gold, lines up with the chip's right edge, and closes on Escape or an outside click. On a phone the same menu hangs from the account button in the room header and includes Switch character, Log in / Save progress for guests, and Log out for a signed-in player. Terminal lines wrap on word boundaries inside the panel; a token longer than the row may still break. Resizing the terminal reflows that scrollback.

### Reward scaling
`reward_scale` in `config/combat_balance.yaml` multiplies each enemy's base XP and gold by that threat tier. The reference level is the **highest** level among characters who receive the victory split (living fighters plus same-room online party), so a high-level member greys out the whole award. Defaults: grey 15%, green 60%, yellow 100%, orange 125%, red 150%, skull 200%. A boss's first kill for a character adds `first_kill_bonus` (default 50%) of that character's own share of the boss, once, stored on `Character.FirstBossKills` (`tpl:<templateId>` or `name:<lower name>`). A missing item icon swaps once to `/api/item-art/generic-<type>.png`, then the default generic, then a built-in silhouette. A missing dialog bust tries the full-body portrait, then a built-in silhouette (enemies darker, friendly NPCs gold). A missing enemy or NPC portrait swaps once to that silhouette. A player with no portrait file, including a guest on the battle card, uses a class silhouette. Those stand-ins are data URIs, so a failed image cannot loop or stay as a broken icon. `combatEnd` still carries `outcome` and the human `message`. Victory adds `rewards` (base, level modifier, first-kill, share) plus `loot` (`name`, `quality`, `quantity`) and `levelUp` (`oldLevel`, `newLevel`) when a level was gained. Defeat adds `defeat` (`xpLost`, `goldLost`, `armor`, `respawnRoom`, `hp`, `maxHp`). Older clients ignore the extra fields. BattleStage shows that breakdown, reveals each drop one at a time in its rarity color, and calls out the new level. The terminal still gets the full text. The panel sits over the room, dismisses on click, Enter, or Escape, and does not take pointer events away from the terminal. Typing in the command line keeps Enter.

---

## Skills & Spells System

### Skill Entity Structure
```go
type Skill struct {
    *entities.Entity

    Name, Description string
    ClassIDs       []string  // Multi-class: ["warrior"], ["cleric", "druid"]
    LevelRequired  int32

    // Resource System
    ResourceType   ResourceType  // "mana" (casters) or "cooldown" (physical)
    ManaCost       int32         // Mana cost per use
    CooldownRounds int           // Rounds before reuse

    // Targeting & Effects
    Target         TargetType    // "enemy", "self", "all_enemies"
    Effect         EffectType    // "damage", "heal", "buff", "debuff", "dot", "hot"
    ScalingAttr    string        // "STR", "DEX", "INT", "WIS"
    BasePower      int32
    ScalingFactor  float64
    Duration       int           // Rounds (0 = instant)

    // Buff/Debuff Details
    BuffStat       string        // "ATK", "DEF", "STR", "DEX"
    BuffPercent    float64       // 0.30 = +30%

    // Special Mechanics
    IgnoresDefense bool
    HitCount       int           // Multi-hit attacks

    // Secondary Effect (optional, for hybrid skills)
    SecondaryEffect, SecondaryTarget, etc.
}
```

### Skill Storage & Cache
- **Database-backed** with in-memory cache
- `LoadFromDB()` at server startup
- `RefreshCache()` after CRUD operations
- Combat engine reads from cache (zero service threading)

### Skill Registry Functions
```go
SkillByID(id) *Skill                      // Look up by ID
SkillsForClass(classID) []*Skill          // All skills for class
AvailableSkills(classID, level) []*Skill  // Skills at level
MaxSkillSlots(classID, level) int         // Equippable slots
IsCasterClass(classID) bool               // Uses mana
```

### Skill Slot Progression
A class with `hotbar_cap` in the class catalog always has that many slots (the signed pack uses 4). Cleric and druid stay on the caster curve (L1=2, L15=3, L30=4). Ranger stays on the physical curve (L1=1, L10=2, L20=3, L30=4). Stored ids such as hitch still resolve through the catalog.

### Class kit
Class names, blurbs, race lists, portraits, and kit skills live in the world pack under `data/classes/*.yaml`. The engine loads them into `pkg/classkit` at startup and on import. `GET /api/classes` is the public payload the play client reads (`?v=classkit1`). With no pack, the catalog is the generic Warrior / Rogue / Mage sample. `ClassKit()` builds skill rows from that catalog. `SkillsForClass` drops legacy seed rows for a kit class. Cleric, ranger, and druid stay in the static client catalog. Old ids such as Fireball and Power Strike remain display fallbacks for a saved hotbar and are not offered as Available. `config/combat_balance.yaml` `class_balance` is only the numeric fallback when the catalog has no row.

### Skill Management Commands
```bash
skills                  # List available and equipped skills
skills equip <name>     # Equip skill to next slot
skills unequip <name>   # Remove skill from slots
```

**Combat Restrictions**: Cannot equip/unequip during combat

### Default Skills Seeding
An empty database seeds `SeedSkills()` (legacy class rows plus `ClassKit`). An existing database upserts the class kit on startup. Kit classes only see kit skills. Cleric, ranger, and druid still use their seeded rows.

### Class Catalog & Magic Schools (`pkg/classkit`)
The combat kit's classes come from a **world-pack catalog**, not the database. `pkg/classkit` loads every `*.yaml` in the first directory found, in order: `CLASSKIT_DIR`, `import/mvp-rpg-1/data/classes`, `../talesmud-rpg-1/data/classes`, then `data/classes` (found by walking up from the working dir). With no pack, the catalog falls back to the generic Warrior/Rogue/Mage sample trio. The fork ships its own pack in `data/classes/` (format and schools documented in `data/classes/README.md`).

Each class YAML defines the create-screen card (`template`), the balance row, races, charges, and hotbar skills. `skills[].effect` must be one of the engine primitives in `KnownEffects`: `brace, slam, stand, slip, nick, smoke, inscribe, sear, glyph, guard, bolt, rig, overload, mend, renew`. `mend` is a **direct heal** and `renew` is a **heal-over-time (HoT)**; both target `ally`, fall back to the caster when the target is an enemy, dead, or fled, and scale off the caster's max HP (`tick = max HP × multiplier`). `renew` heals once per round for `duration` rounds (3 when omitted). Both are cooldown-gated like the rest of the kit. See the healing/kit mapping in `pkg/mudserver/game/combat/classkit.go` (`KitMend`, `KitRenew`).

**Mana cost & per-school components (`mana_cost`, `component`):** a kit skill may list `mana_cost` (integer, default `0`) and a free-text `component` (the per-school reagent hook). When `mana_cost > 0`, the caster pays it in the same `pay()` step as a cooldown: **the mana is deducted AND the cooldown still starts** when the skill also has a `cooldown`. A cast is rejected before anything is paid when `CurrentMana < mana_cost` (`Not enough mana for <skill> (need N, have M).`), so a failed cast spends no mana and starts no cooldown. `mage_inscribe` stays at `mana_cost: 0` (its tooltip and the no-mana regression test depend on it) but still carries the `arcane_dust` hook. `component` is **structural only** today: there is no physical reagent inventory; it flows from the YAML through `classkit.SkillSpec` into `skills.Skill`, the `/api/classes` payload (`ClientSkill.component`), and `contenthealth` so a later inventory pass can require the reagent without a schema change. Components by school: `arcane_dust` (arcane, high mana), `fire_salt` (fire, medium), `divine_wax` (divine, medium), `nature_herb` (nature, medium), `shadow_essence` (shadow, unused so far). Physical (`martial`) skills keep `mana_cost: 0` and no component. Mage and Sorcerer also gain a self-only mend (`mage_mend`, `sorcerer_mend`) at `multiplier: 0.20` with a visible mana cost.

**Magic schools (additive `school` field):** an optional `school` key tags a class with a discipline. Empty means a physical class. Registered ids: `martial` (none), `fire` (Fuego), `ice` (Hielo), `arcane` (Arcano), `divine` (Divino), `nature` (Naturaleza), `shadow` (Sombras). `classkit.SchoolName(id)` returns the display name. An unregistered value fails the load (`class <id> has unknown school "<value>"`); a YAML with no `school` still loads.

**Creator editing and persistence:** the class balance editor saves through
`GET/POST/PUT/DELETE /api/classes`, `/api/classes/:id`, and
`POST /api/classes/:id/duplicate`. `pkg/classkit.SaveDef` validates and writes
the active pack's YAML atomically, then reloads the registry; `DeleteDef`
removes the matching file. Validation failures return 400 with issues, missing
classes 404. `GET /api/classes` remains public but returns the full editor DTO
for creator/admin tokens. The play client's Classes editor uses the authenticated
routes and only falls back to localStorage on a network error. The `/api/spells`
and `/api/spells/:id` routes alias the existing skill CRUD.

**Base pack (`data/classes/`):**

| id | Name (es) | School | Role | Kit effects |
|----|-----------|--------|------|-------------|
| `warrior` | Warrior (Guerrero) | `martial` | Plate front line | brace, slam, stand |
| `rogue` | Rogue (Pícaro) | `martial` | Two-swing skirmisher | slip, nick, smoke |
| `mage` | Mage (Mago) | `arcane` | Cloth caster | inscribe, sear, mend (self), glyph |
| `cleric` | Cleric (Clérigo) | `divine` | Divine bulwark & healer | guard, mend, glyph, slam, renew, bolt |
| `druid` | Druid (Druida) | `nature` | Fae-fire caster & healer | inscribe, glyph, renew, sear, mend |
| `sorcerer` | Sorcerer (Hechicero) | `fire` | Glass cannon | bolt, sear, rig, overload, mend (self) |

Spanish forms are aliases (`guerrero`, `mago`, `picaro`, `clerigo`, `druida`, `hechicero` and the feminine forms). Skill ids are prefixed by class (`<class>_...`) so they stay unique across the hotbar. The Sorcerer has no dedicated portrait and uses `portrait_class: mage`.

Player guide for equipping/using these skills and for what `mend` / `renew` do in a fight: `docs/user/combat.md`.

---

## Quest System

### Quest Entity Structure
```go
type Quest struct {
    *entities.Entity

    Name, Description string
    Category    string  // "main", "side", "daily"
    Area        string  // Optional region/zone label
    Level       int32
    Repeatable  bool

    Source      QuestSource  // "npc", "item", "auto", "script"
    Objectives  []Objective
    Rewards     Reward

    RequiredQuestIDs []string  // Prerequisite quests
    RequiredLevel    int32

    // NPC Dialog Integration
    AcceptDialogText, ProgressDialogText, CompleteDialogText string
    OnCompleteScriptID string  // Lua script on completion
}
```

### Quest Source Types
```go
type QuestSource struct {
    Type   string  // "npc", "item", "auto", "script"
    NPCID  string  // Quest-giving NPC
    ItemID string  // Quest-triggering item
}
```

### Objective Types
```go
type Objective struct {
    ID          string
    Type        ObjectiveType  // "kill", "collect", "deliver", "visit", "talk", "custom"
    Description string

    // Type-specific fields
    TargetID        string  // NPC/item/room target depending on type
    TargetName      string
    Amount          int32
    DeliverToNPCID  string  // For deliver
    DialogNodeID    string  // Optional for talk
    CheckScriptID   string  // For custom (Lua)
    Order           int32
}
```

### Quest Progress (Per-Character)
```go
type QuestProgress struct {
    *entities.Entity

    CharacterID string
    QuestID     string
    Status      QuestStatus  // "available", "active", "completed", "failed", "abandoned"
    Objectives  []ObjectiveProgress
    AcceptedAt, CompletedAt time.Time
}
```

### Quest Rewards
```go
type Reward struct {
    XP    int64
    Gold  int64
    Items []RewardItem  // Item template IDs with quantities
}
```

### Quest Definition Validation
Quest create/update validates impossible definitions before persistence:
- Required name, description, source type, and at least one objective
- Required source references for NPC/item sources
- Required objective IDs, descriptions, target fields, and custom scripts
- Duplicate objective IDs, negative objective amounts, negative rewards
- Empty reward/prerequisite IDs and self-prerequisites
- Missing referenced NPCs, item templates, rooms, scripts, and prerequisite quests

### Quest Tracker (Automatic Progress)
The `QuestTracker` listens to game events and updates objectives:

| Event | Source | Objectives Updated |
|-------|--------|-------------------|
| NPC killed | Combat victory | Kill objectives (matches template ID) |
| Item pickup | Pickup command | Collect objectives (matches template ID and stack quantity) |
| Room enter | Room navigation | Visit objectives (matches room ID) |
| Dialog node | Talk command | Talk objectives (matches NPC + node) |
| Talk to NPC | Talk command | Deliver objectives (requires and consumes matching item quantity) |

`QuestsService.ApplyQuestEvent` provides the same normalized event matching for service-level workflows and tests, returning progress or ready-to-turn-in results. `QuestsService.TurnInQuest` validates the turn-in NPC, completes the quest, grants rewards, and returns the reward summary in one call.

### Quest Dialog Integration
When talking to a quest-source NPC:
1. **Offer option** - If quest is available and prerequisites met
2. **Progress option** - If quest is active but not complete
3. **Turn-in option** - If all objectives complete

Dialog options are **automatically injected** into NPC conversations.
Quest-source NPCs do not need a full dialog tree: if an NPC has quest options but no `DialogID`, the server opens a quest-only conversation and numeric selection accepts, checks, or turns in the quest.

### Quest Commands
```bash
quests          # Show quest log (active quests + progress)
ql, questlog    # Aliases

quest <name>    # Show quest details

abandon <name>  # Abandon active quest

offer <npc>     # Ask an NPC quest-giver for a contextual quest (Batch 18)
offer accept    # Accept the pending offer (persists the quest)
offer decline   # Decline it (aliases: no, reject, pass)
```

### Contextual Quest Offers (`offer` command, Batch 18)

- **Files:** `pkg/mudserver/game/commands/offer.go` (+ `offer_test.go`), `pkg/mudserver/game/quest_offer.go`, provider installed in `pkg/server/llm.go` (`installContextualQuestProvider`), registered in `commandprocessor.go`.
- **Who answers:** only a quest-giver NPC in the current room (same `IsQuestGiver` contract as the dialog UI: authored quest whose `source.npcId` is the template/instance).
- **Context:** character level + `room.Area` (falls back to `room.ID`) + NPC `TemplateID` → `Game.RequestContextualQuestOffer` → `llmgen.GenerateContextualQuest` (`QuestContext{PlayerLevel, ZoneID, Theme, SourceNPCID, CharacterID, UserID}`).
- **Asynchronous:** generation runs outside the command loop and is published with `Game.Call`; the command answers in prose (`offer.go`), it never blocks the loop.
- **Limits:** 60 s per-character cooldown + a semaphore of 2 concurrent generations (`QuestOfferBusy`), plus the pipeline's own `quest:ctx:<characterID>` limit key.
- **Fallback:** with `LLM_PROVIDER=none` (nil client) the generator returns its deterministic fallback; without an installed provider the command says there is no work.
- **On accept:** `QuestsService.Store` + `AcceptQuest`, then `questAccepted` + `questLog` messages; `bindQuestToSourceNPC` re-points `source`/`turnIn` at the offering NPC only if the mutated quest still validates against the live world. The quest is persisted **only** on accept.

### Quest UI Features

#### Quest Log Widget
The quest log widget provides a comprehensive quest management interface:

**Core Features:**
- **Real-time Quest Search**
  - Search across quest names, descriptions, and objectives
  - Case-insensitive, instant filtering
  - Clear button (×) to reset search

- **Category Filtering**
  - Filter by: All Types, Main, Side, Daily
  - Toggle visibility: Completed quests, Abandoned quests
  - Helps focus on relevant quests

- **Sort Options**
- Sort by Status (Ready → Active → Completed → Abandoned)
  - Sort by Name (alphabetical)
  - Sort by Level (ascending)
  - Sort by Category (Main/Side/Daily)

- **Quest Pinning System**
  - Pin up to 5 priority quests
  - Pinned section always at top
  - Golden pulsing indicator for pinned quests
  - Persisted in localStorage
  - Pin/Unpin buttons in quest details

**Quest Display:**
- Quest name with level badge (e.g., "L5")
- Ready-to-turn-in badge and highlighted row when all active objectives are complete
- Category badge with color coding:
  - 🟠 Main quests (amber/orange #f59e0b)
  - 🔵 Side quests (blue #3b82f6)
  - 🟣 Daily quests (purple #8b5cf6)
- Expandable quest details
- Objective progress (X/Y format) with checkmarks
- Objective descriptions are populated from quest definitions in both REST quest-log refreshes and WebSocket quest-log messages
- Rewards preview (XP, Gold, Items)
- Abandon button for active quests
- Pin/Unpin button for tracking

**Quest Sections:**
1. **📌 Pinned** - Priority quests (if any)
2. **Active** - In-progress quests
3. **Completed** - Finished quests with completion dates
4. **Abandoned** - Dropped quests (when visible)
5. **Failed** - Failed quests (when visible)

#### Quest History & Statistics Panel

Click the 📊 button in quest log header to access:

**Quest Statistics:**
- Total quests encountered
- Active quest count
- Completed quest count
- Completion rate percentage
- Total XP earned from quests
- Total gold earned from quests

**Category Breakdown:**
- Main quests completed
- Side quests completed
- Daily quests completed

**Quest Achievements:**
8 built-in achievements that unlock automatically:

| Achievement | Requirement |
|-------------|-------------|
| First Steps | Complete 1 quest |
| Quest Novice | Complete 5 quests |
| Quest Veteran | Complete 10 quests |
| Quest Master | Complete 25 quests |
| Story Seeker | Complete 5 main quests |
| Side Quest Hero | Complete 10 side quests |
| Daily Devotee | Complete 5 daily quests |
| Completionist | 100% completion rate (min 5 quests) |

**Achievement Display:**
- 🏆 Unlocked achievements (gold border, highlighted)
- 🔒 Locked achievements (grayed out, collapsible)
- Shows name and description
- Real-time progress tracking

#### Quest Notifications

Enhanced notification system with interactions:

**Notification Types:**
- **Quest Accepted** / **Quest Complete** — centered moment cards (amber/brass), not top-right chips
- **Quest Progress** (blue border) - shows the changed objective and current/required counts
- **Quest Ready** (yellow border) - shown when all objectives are complete and the quest can be turned in

**Interactions:**
- **Click to View** - Opens quest in quest log
- **Dismiss Button (×)** - Manual dismiss with slide-out animation
- **Hover Effects** - Highlights notification
- **Auto-dismiss** - Corner toasts after 5s; accepted banner after 10s

**Features:**
- Unique notification IDs
- Notification queue capped to the latest 4 visible events
- Slide-in and slide-out animations
- Smooth transitions
- Accepting a quest refreshes the open Talk dialog so `[Quest]` becomes `[In Progress]` without closing Talk
- Quest log: READY + turnInAnywhere shows **Turn In** (`complete <name>`); otherwise a **Turn in: &lt;NPC&gt;** hint
- **Next-step hint** (`guideHint`): one leather/bronze chip for a character who has not finished a quest and has not dismissed it. Optional quest field `guide: true` (YAML `guide`, JSON `guide`) selects that quest. With no flag, active quests are the pool and an open kill wins. Steps are talk/accept, head along a visible exit, fight (`attack <name>`, text `Attack the <name>.`), and turn in (`complete <name>` or talk). Hint text does not say tap or click, so web, telnet, SSH, and Door show the same line. `guide dismiss` sets `flags.guideDismissed` on that character. The chip is portaled to the page, sits above the hotbar, and hides during combat, dialog, shop, and Need/Greed. It does not cover `.room-name` or `.hotbar-widget`. `guide.enabled: false` in the ruleset or a game-mode file turns the hint off for every character and sends nothing.

### Spell Bar / Hotbar
- Nine square slots docked on top of the desktop action bar (a placed hotbar widget still floats on its own). Mobile keeps its own strip. At 375px the room strip and the combat-dock row shrink the slot and the gap so slots 1 through 9 stay fully inside the viewport.
- Bind equipped combat skills (`cast` / combat-only) or inventory consumables (`use`)
- Look / Talk / Flee are bindable actions but **not** seeded by default
- Rest is seeded on an empty/default hotbar (slot 7). Customized binds are never overwritten
- Out of combat, a **Resting** chip shows near HP while `Flags.resting` is true; combat or movement clears it
- Search is not a look alias and is not offered as a hotbar action
- Binds persist in `talesmud_settings_v1` (`interface.hotbarBinds`); empty equipped list shows "spellbook empty"
- Does not use or overload the respawn `bind` command

### Quest API Endpoints

#### Get Quest Log with Full Details
```
GET /api/quest-progress/:characterId
```

Requires the authenticated user to own `characterId`, unless the user is an admin.

**Response includes:**
- Quest progress (status, objectives)
- Quest definition (name, description, category, level, objective descriptions)
- Rewards (XP, gold, item template IDs)
- Timestamps (acceptedAt, completedAt)

**Response Format:**
```json
[
  {
    "questId": "quest-uuid",
    "questName": "The Wolf Problem",
    "status": "active",
    "description": "Wolves have been attacking travelers...",
    "category": "main",
    "level": 3,
    "objectives": [
      {
        "objectiveId": "obj-1",
        "description": "Defeat wolves",
        "current": 3,
        "required": 5,
        "completed": false
      }
    ],
    "rewards": {
      "xp": 500,
      "gold": 50,
      "itemTemplateIds": ["item-template-1"]
    },
    "acceptedAt": "2026-02-16T10:30:00Z",
    "completedAt": null
  }
]
```

### Quest WebSocket Messages

#### Quest Log Message
Sent on character selection and quest updates:
```json
{
  "type": "questLog",
  "quests": [ /* array of QuestLogEntry */ ]
}
```
Dialog-driven quest accepts and completions send the same enriched quest entries as character selection, including quest name, description, category, level, objective descriptions, rewards, and timestamps when available.

#### Quest Update Messages
```json
{
  "type": "questAccepted",
  "questId": "quest-uuid",
  "questName": "The Wolf Problem",
  "message": "Quest accepted!"
}

{
  "type": "questProgress",
  "questId": "quest-uuid",
  "questName": "The Wolf Problem",
  "objectives": [ /* updated objectives */ ],
  "changedObjective": {
    "objectiveId": "obj-1",
    "description": "Defeat wolves",
    "current": 5,
    "required": 5,
    "completed": true
  }
}

{
  "type": "questReady",
  "questId": "quest-uuid",
  "questName": "The Wolf Problem",
  "objectives": [ /* all objectives complete */ ],
  "changedObjective": { /* last changed objective */ }
}

{
  "type": "questCompleted",
  "questId": "quest-uuid",
  "questName": "The Wolf Problem",
  "message": "Quest completed!"
}
```

### Quest Client Store

The MUD client stores quest data in `MUDXPlusStore`:

**State:**
```javascript
{
  quests: [],              // Full quest log with details
  questNotifications: [],  // Active notifications
  pinnedQuests: []        // Stored in localStorage
}
```

**Methods:**
- `updateQuests(questLog)` - Update full quest list
- `addQuestNotification(notification)` - Add notification with auto-dismiss
- Quest notifications auto-dismiss after 5 seconds and keep only the newest 4 visible entries

**LocalStorage:**
- `pinnedQuests` - JSON array of quest IDs (max 5)
- Persists across sessions
- Shared between widget and overlay

### Quest Tracker Implementation

Automatic progress tracking on game events:

**OnNPCKilled(characterID, userID, deadNPC):**
- Checks all active quests for kill objectives
- Matches NPC template ID
- Increments objective counter
- Sends progress update message

**OnItemPickup(characterID, userID, item):**
- Checks collect objectives
- Matches item template ID
- Increments by picked-up stack quantity when present
- Sends progress update

When a collect quest is accepted, matching items already in inventory initialize objective progress. Stackable quantities count toward the initial objective amount.

**OnRoomEnter(characterID, userID, room):**
- Checks visit objectives
- Matches room ID
- Marks objective complete
- Sends progress update

**OnDialogNode(characterID, userID, npcID, dialogID, nodeID):**
- Checks talk objectives
- Matches NPC and optional dialog node
- Marks complete
- Sends progress update

**OnTalkToNPC(characterID, userID, npc):**
- Checks deliver objectives
- Verifies player has required item quantity
- Consumes matching items from inventory
- Marks objective complete
- Sends progress update

### Quest Completion Flow

1. **Check Objectives** - All must be complete (checked server-side)
2. **Mark Complete** - Update quest progress status
3. **Grant Rewards**
   - Add XP to character
   - Add gold to character
   - Create items from templates
   - Add to character inventory
4. **Send Notification** - WebSocket message with rewards
5. **Update Quest Log** - Client refreshes via API call
6. **Update Achievements** - Calculated client-side on quest log update

### Quest Scripting API
```lua
-- Check quest status
local status = tales.quests.getStatus(characterID, questID)

-- Accept quest
tales.quests.accept(characterID, questID)

-- Complete quest
tales.quests.complete(characterID, questID)

-- Update objective progress
tales.quests.updateProgress(characterID, questID, objectiveIndex, count)

-- Grant quest items
tales.quests.grantItems(characterID, questID)

-- Abandon quest
tales.quests.abandon(characterID, questID)

-- Check if quest can be accepted
local canAccept = tales.quests.canAccept(characterID, questID)
```

---

## Dialog System

### Dialog Entity Structure
```go
type Dialog struct {
    ID   string
    Text string  // Primary speech

    // Text Variations
    AlternateTexts []string  // Random variations
    OrderedTexts   *bool     // Sequential on repeat

    // Dialog Flow
    Options []Dialog  // Player choices (branching)
    Answer  *Dialog   // Auto-response (linear)

    // Conditional Display
    RequiresVisitedDialogs []string  // Prerequisites
    ShowOnlyOnce           *bool     // One-time option
    IsDialogExit           *bool     // End conversation
}
```

### Dialog State (Per-Conversation)
```go
type DialogState struct {
    CurrentDialogID  string
    DialogVisited    map[string]int  // Node ID → visit count
    Context          map[string]string  // Static variables
    DynamicContext   map[string]func() string  // Runtime variables
}
```

### Dialog Variable Substitution
```
{{PLAYER}}   → Character name
{{NPC}}      → NPC name
{{TIME}}     → Current time
{{CUSTOM}}   → Script-set context variables
```

### Dialog Features
- **Branching** - Multiple player choices via `Options`
- **Linear** - Automatic progression via `Answer`
- **Conditional** - Options shown only if prerequisites visited
- **One-time** - `ShowOnlyOnce` options disappear after first selection
- **Variations** - Random or sequential alternate texts
- **Exit markers** - Return to "main" node on completion
- **Free-form complement (`ask` and `talk`, Batches 25–29)** — además de `ask <npc> [question]`, `talk` saluda con LLM solo cuando faltan diálogo autoral y quests; el texto libre puede continuar una conversación `__llm__` elegible. Las opciones del LLM pueden continuar el árbol autoral, pedir seguimiento, terminar o ejecutar una acción `quest:<id>` disponible y validada. Memoria/personalidad y relaciones dinámicas siguen el flujo descrito en Fase 6; el LLM no crea quests. El flujo autoral y sus quests mantienen prioridad.
- **Play overlay** - Dialog and shop are portaled to the page layer (`position: fixed`). Phone and tablet (≤768px), and any window shorter than 500px, use a scrollable bottom sheet with safe-area padding. The shop close control stays in the header. A dialog × is shown only when the current node has an exit choice, and it selects that choice. There is no separate Leave conversation control. Taller desktop windows keep centered panels that scroll inside the viewport. Play cache tag `?v=curve1`.

### Dialog API
```lua
-- Get dialog
local dialog = tales.dialogs.get(dialogID)

-- Get conversation state
local conv = tales.dialogs.getConversation(characterID, npcID)

-- Set context variable
tales.dialogs.setContext(conversationID, "playerClass", "warrior")

-- Get context variable
local class = tales.dialogs.getContext(conversationID, "playerClass")

-- Check if visited
local visited = tales.dialogs.hasVisited(conversationID, nodeID)

-- Get visit count
local count = tales.dialogs.getVisitCount(conversationID, nodeID)
```

---

## Dungeon Builder (Phase 5.1)

Authoring tool for dungeon layouts: a full-screen panel in the play client that stores designs locally and, with a creator session, mirrors them to the REST API. **Persistence only — designs are not instantiated in the game yet.**

### Client

- **Panel:** `public/mud-client/src/game/ui/DungeonBuilder.svelte`, opened with the **`D` key** (wired in `Game.svelte` as `dungeonBuilderOpen`, listed in `ShortcutSheet.svelte` and in `keyboardShortcuts.js` as the `dungeonBuilder` panel).
- **Pure modules:** `src/game/dungeonBuilder.js` (grid, rooms, corridors, encounters, loot tables, entry→exit reachability by BFS, overlap detection, test mode) and `src/game/dungeonLibrary.js` (export/import of the `definition` blob). Tests: `dungeonBuilder_test.mjs`, `dungeonLibrary_test.mjs`.
- **Storage:** `localStorage` is the source of truth; `src/api/dungeons.js` (`DUNGEON_CRUD_AVAILABLE = true`) does **best-effort** sync to `/api/dungeons` when a creator token is present. 401/403/404 or network failure degrades to local storage with a visible warning.

### Server (`/api/dungeons`)

- **Data path:** table `dungeons` (migration `migratePhase5Dungeons`, `(id, data)` JSON + `idx_dungeons_name`/`idx_dungeons_author`) → `pkg/entities/dungeons/` → `pkg/repository/dungeons_sqlite.go` (`Factory.Dungeons()`) → `DungeonsService` (`pkg/service/dungeons.go`, exposed on the `Facade`) → `pkg/server/handler/dungeons.go`.
- **Routes:** authenticated reads `GET /api/dungeons?name=&author=&limit=` (summary DTO without `definition`) and `GET /api/dungeons/:id` (detail with `definition`); creator-gated writes `POST /api/dungeons` (201), `PUT /api/dungeons/:id` (200/404), `DELETE /api/dungeons/:id` (204/404); 400 on validation errors, 403 without the role.
- **Validation:** `name` and grid required, `definition` must be valid JSON (opaque blob, round-tripped as authored); `CreatedAt` is preserved on update.
- **Tests:** `pkg/server/handler/dungeons_test.go`, `pkg/server/dungeons_http_test.go` (role gate + full HTTP round trip).
- **Contract:** [`docs/API.md`](docs/API.md) §5.

**Not implemented yet:** no compile/run endpoint and no spawn/instancing of a saved design inside the game world (Batch 19 pendiente).

---

## Scripting System (Lua API)

### Lua Runner Architecture
- **Language**: Lua 5.1 (via gopher-lua)
- **VM Pool**: Reusable Lua states for performance
- **Timeout**: 5-second execution limit
- **Sandbox**: Restricted environment (no file I/O, no network)

### Script Types
- `item` - Item creation, behavior, OnUse handlers
- `room` - Room actions, OnEnter handlers
- `npc` - NPC behavior, aggro/death/flee events
- `quest` - Quest logic, custom objectives
- `event` - Event handlers (player.enter_room, npc.death, etc.)
- `custom` - General purpose

### Context Variable (`ctx`)
Scripts receive context data via global `ctx`:
```lua
-- Room enter script:
ctx.eventType   -- "player.enter_room"
ctx.room        -- Room entity
ctx.character   -- Character entity
ctx.user        -- User entity

-- Item use script:
ctx.eventType   -- "item.use"
ctx.item        -- Item entity
ctx.character   -- Character entity
ctx.room        -- Room entity (if in room)

-- Room action script:
ctx.eventType   -- "room.action"
ctx.room        -- Room entity
ctx.character   -- Character entity
ctx.action      -- Action entity
```

### tales.game Module

All functions below are called as `tales.game.functionName(...)`. Every parameter is **required** unless noted otherwise. Functions that perform writes return `bool` (true on success, false on failure). Missing or wrong-type arguments cause a **Lua error that aborts the script**.

#### Messaging
```lua
tales.game.msgToRoom(roomID, message)
-- Send message to ALL players in a room. Returns bool.
-- roomID: string (room entity ID, e.g., "R0004")
-- message: string

tales.game.msgToCharacter(characterID, message)
-- Send message to a specific character (via their user's socket). Returns bool.
-- characterID: string (character entity ID)
-- message: string

tales.game.msgToUser(userID, message)
-- Send message to a specific user by user ID. Returns bool.
-- userID: string (user entity ID — NOT character ID)
-- message: string

tales.game.broadcast(message)
-- Send message to ALL connected players globally. Returns bool.
-- message: string

tales.game.msgToRoomExcept(roomID, message, excludeCharacterID)
-- Send message to all players in room EXCEPT the specified character. Returns bool.
-- roomID: string
-- message: string
-- excludeCharacterID: string (character entity ID to exclude)
```

#### Logging
```lua
tales.game.log(level, message)
-- Write to server log. Returns nothing.
-- level: string — "debug", "info", "warn", "error"
-- message: string
```

#### Inventory & Equipment Checks
```lua
tales.game.hasItem(characterID, itemID)
-- Check if character has an item in inventory. Returns bool.
-- Checks by exact item ID first, then by TemplateID (so you can pass "ITM0001").
-- characterID: string
-- itemID: string (item instance ID or template ID)

tales.game.hasEquipped(characterID, slotName)
-- Check if character has an item equipped in a specific slot. Returns bool.
-- characterID: string
-- slotName: string — "head", "chest", "legs", "boots", "hands", "neck",
--   "ring1", "ring2", "main_hand", "off_hand"
```

#### Character Flags (Per-Character State)
```lua
tales.game.getFlag(characterID, flagName)
-- Get a flag value from a character. Returns the value or nil if not set.
-- characterID: string
-- flagName: string
-- Return types: bool, number, string, or nil

tales.game.setFlag(characterID, flagName, value)
-- Set a flag on a character. Persisted to database immediately. Returns bool.
-- characterID: string
-- flagName: string
-- value: bool, number, string, or nil (nil deletes the flag)

tales.game.clearGear(characterID)
-- Deletes worn items and bagged weapons and armor. Other bag items stay. Returns how many were removed.

tales.game.equipFromTemplate(characterID, templateID)
-- Creates an instance of a template and wears it in that template's slot, replacing the piece already there.
```

#### Hidden Exit Reveals (Per-Character)

**CRITICAL**: Note the different parameter orders between `revealExit` and `hasRevealedExit`.

```lua
tales.game.revealExit(roomID, exitName, characterID)
-- Reveal a hidden exit for a specific character. Returns bool.
-- The exit must already exist on the room with hidden=true.
-- The reveal is stored on the character (Character.RevealedExits), not the room.
-- Sends a silent room update to the client (no full re-render).
-- Idempotent: safe to call multiple times (returns true if already revealed).
-- ALL THREE PARAMETERS ARE REQUIRED.
-- roomID: string (room entity ID where the exit is defined)
-- exitName: string (exit name, case-insensitive match, e.g., "north")
-- characterID: string (character entity ID to reveal the exit for)

tales.game.hasRevealedExit(characterID, roomID, exitName)
-- Check if a character has revealed a specific hidden exit. Returns bool.
-- NOTE: Parameter order is (characterID, roomID, exitName) — different from revealExit!
-- characterID: string
-- roomID: string
-- exitName: string (case-insensitive match)
```

#### Item Rewards
```lua
tales.game.giveItem(characterID, templateID)
-- Create an item instance from a template and add it to the character's inventory.
-- Returns bool. Persisted to database immediately.
-- characterID: string
-- templateID: string (item template ID, e.g., "ITM0022")
```

#### CopyOnPickup Item Tracking
```lua
tales.game.hasCollectedItem(characterID, templateID)
-- Check if a character has already collected a CopyOnPickup item. Returns bool.
-- characterID: string
-- templateID: string (item template ID)

tales.game.resetCollectedItem(characterID, templateID)
-- Reset the collected flag for a CopyOnPickup item, allowing re-collection. Returns bool.
-- characterID: string
-- templateID: string (item template ID)
```

### tales.combat Module (enemy hooks)

Callable from sandboxed scripts. Failures return a zero result and do not abort combat.

```lua
tales.combat.healNpc(npcID, amount)       -- HP restored (0 if none)
tales.combat.applyEffect(targetID, effectID) -- existing buff or debuff only
tales.combat.summon(templateId, count)    -- enemy-template adds spawned (0 on failure)
```

`summon` uses the running enemy-hook fight and room. `count` clamps to 1..3 and to whatever remains of a fight cap of 3. Adds copy template stats, are not charged to a spawner, grant no loot, gold, XP, or quest credit, and are removed when the fight ends (win, lose, flee, or timeout). Each add may run its own `onAggroScript` once. When any combatant dies or flees, they leave the turn order and the current index stays inside the remaining order. If that index is past the end, the next tick wraps into the following round and ends the fight when it is over, so an add dying cannot stall the fight until the idle timeout.

### tales.items Module
```lua
-- Get item
local item = tales.items.get(itemID)

-- Find by name
local items = tales.items.findByName("sword")

-- Templates
local template = tales.items.getTemplate(templateID)
local templates = tales.items.findTemplates("sword")

-- Create instance
local newItem = tales.items.createFromTemplate(templateID)

-- Delete
local success = tales.items.delete(itemID)
```

### tales.rooms Module
```lua
-- Get room
local room = tales.rooms.get(roomID)

-- Find rooms
local rooms = tales.rooms.findByName("tavern")
local rooms = tales.rooms.findByArea("dungeon")
local allRooms = tales.rooms.getAll()

-- Room contents
local characters = tales.rooms.getCharacters(roomID)
local npcs = tales.rooms.getNPCs(roomID)
local items = tales.rooms.getItems(roomID)
```

### tales.characters Module
```lua
-- Get character
local char = tales.characters.get(characterID)

-- Find characters
local chars = tales.characters.findByName("hero")
local allChars = tales.characters.getAll()

-- Get character's room
local room = tales.characters.getRoom(characterID)

-- Character operations
tales.characters.damage(characterID, amount)
tales.characters.heal(characterID, amount)
tales.characters.teleport(characterID, roomID)
tales.characters.giveXP(characterID, amount)

-- Signed gold change. A debit that would go below zero is refused and returns false.
tales.characters.addGold(characterID, delta)

-- Set the respawn room. An empty room id clears it. Unknown rooms return false.
tales.characters.setBind(characterID, roomID)

-- Apply levels the current XP can already buy. Returns how many levels were gained.
tales.characters.applyLevels(characterID)

-- Set level and XP directly. Level clamps to 1..the effective cap. Optional maxHP replaces hit points.
-- Class, skills, inventory, gold, and flags are left alone.
tales.characters.setProgress(characterID, level, xp, maxHP)

-- Permanent combat grant. kind is "attack", "defense", or "maxHP".
-- One call changes that stat by at most 10. Attack and defense grants stay in 0..40.
-- Max hit points stay in 1..5000. A positive maxHP grant also raises current hit points.
-- Zero leaves a classic character unchanged. Returns false for an unknown kind or a zero delta.
tales.characters.grant(characterID, kind, delta)

-- Read-only top list. n defaults to 12 and is capped at 50.
-- sortKey "xp" orders by experience. Any other key orders by level, then experience.
local rows = tales.characters.top(n, sortKey) -- rows[i].name, .level, .xp
```

### tales.resources Module

Balances for configured keys. An unknown key returns `ok=false` and does not create a row.

```lua
local allowance, remaining, ok = tales.resources.get(characterID, key)
local remaining, ok = tales.resources.consume(characterID, key, n)
```

`consume` with `n <= 0` returns the current remaining and `ok=true` when the key exists. Spending more than `remaining` leaves the balance unchanged and returns `ok=false`.

### tales.npcs Module
```lua
-- Get NPC
local npc = tales.npcs.get(npcID)

-- Find NPCs
local npcs = tales.npcs.findByName("guard")
local npcs = tales.npcs.findInRoom(roomID)
local allNPCs = tales.npcs.getAll()

-- NPC operations
tales.npcs.damage(npcID, amount)
tales.npcs.heal(npcID, amount)
tales.npcs.moveTo(npcID, roomID)

-- NPC checks
local isDead = tales.npcs.isDead(npcID)
local isEnemy = tales.npcs.isEnemy(npcID)
local isMerchant = tales.npcs.isMerchant(npcID)

-- Template & instance operations
local templates = tales.npcs.getTemplates()
local isTemplate = tales.npcs.isTemplate(npcID)
local instance = tales.npcs.spawnFromTemplate(templateID, roomID)
local started = tales.npcs.beginFight(characterID, instanceID)
-- Starts a real fight when both are in the same room, the target is a living enemy,
-- and neither is already fighting. Returns false during the post-combat breath window.
-- The caller chose the target, so this does not print an over-level warning.
local inst = tales.npcs.getInstance(instanceID)
local instances = tales.npcs.getInstancesInRoom(roomID)

-- Instance operations
tales.npcs.kill(instanceID)
tales.npcs.setState(instanceID, state)  -- "idle", "combat", "patrol", "fleeing"
local state = tales.npcs.getState(instanceID)
local died = tales.npcs.damageInstance(instanceID, amount)
tales.npcs.healInstance(instanceID, amount)
tales.npcs.moveInstance(instanceID, roomID)

-- Delete the saved row and, when a game is running, the live instance.
tales.npcs.delete(npcID)
```

### tales.dialogs Module
```lua
-- Get dialog
local dialog = tales.dialogs.get(dialogID)

-- Find dialogs
local dialogs = tales.dialogs.findByName("greeting")
local allDialogs = tales.dialogs.getAll()

-- Conversation management
local conv = tales.dialogs.getConversation(characterID, npcID)
tales.dialogs.setContext(conversationID, "key", "value")
local value = tales.dialogs.getContext(conversationID, "key")

-- Visit tracking
local visited = tales.dialogs.hasVisited(conversationID, nodeID)
local count = tales.dialogs.getVisitCount(conversationID, nodeID)
```

### tales.quests Module
```lua
-- Quest status
local status = tales.quests.getStatus(characterID, questID)

-- Quest operations
tales.quests.accept(characterID, questID)
tales.quests.complete(characterID, questID)
tales.quests.abandon(characterID, questID)

-- Progress tracking
tales.quests.updateProgress(characterID, questID, objectiveIndex, count)

-- Rewards
tales.quests.grantItems(characterID, questID)

-- Checks
local canAccept = tales.quests.canAccept(characterID, questID)
```

### tales.utils Module
```lua
-- Random numbers
local num = tales.utils.random(1, 100)  -- Inclusive
local f = tales.utils.randomFloat()     -- 0.0-1.0

-- UUID
local id = tales.utils.uuid()

-- Time
local now = tales.utils.now()           -- Unix timestamp
local nowMs = tales.utils.nowMs()       -- Milliseconds
local str = tales.utils.formatTime(timestamp)

-- Dice rolling
local result = tales.utils.roll("2d6+3")

-- Probability
local success = tales.utils.chance(25)  -- 25% chance → true/false

-- Array utilities
local picked = tales.utils.pick({"sword", "axe", "spear"})
local shuffled = tales.utils.shuffle(myArray)

-- Math utilities
local clamped = tales.utils.clamp(value, 0, 100)
local lerped = tales.utils.lerp(0, 100, 0.5)  -- Returns 50
```

### Script Execution Contexts

Each script type receives a `ctx` global table with different fields. Access fields as `ctx.fieldName`. Object fields expose entity properties (e.g., `ctx.character.ID`, `ctx.room.Name`).

**Room OnEnter Script** (`type: room`, assigned via room's `onEnterScript`):
- Triggered when player enters room (walk, teleport, or character select)
- Context variables:
  - `ctx.eventType` — string: `"player.enter_room"`
  - `ctx.room` — Room entity object (access `.ID`, `.Name`, etc.)
  - `ctx.toRoom` — Room entity object (same as `ctx.room`)
  - `ctx.character` — Character entity object (access `.ID`, `.Name`, etc.)
  - `ctx.user` — User entity object
- **Getting room ID**: Use `ctx.room.ID`
- **Getting character ID**: Use `ctx.character.ID` or store as `local charID = character.ID`

**Room Action Script** (`type: custom`, assigned via room action's `scriptId`):
- Triggered by player room action (e.g., `EXAMINE RUNES`, `PRESS CIRCLE`)
- Context variables:
  - `ctx.eventType` — string: `"room.action"`
  - `ctx.room` — Room entity object
  - `ctx.roomID` — string: room ID (convenience shortcut for `ctx.room.ID`)
  - `ctx.character` — Character entity object
  - `ctx.characterID` — string: character ID (convenience shortcut for `ctx.character.ID`)
  - `ctx.action` — string: action name (e.g., `"PRESS CIRCLE"`)
  - `ctx.params` — table: action params from YAML (e.g., `{action="press", symbol="circle"}`)
- **Note**: Room action scripts get BOTH object and string ID shortcuts (`ctx.roomID` and `ctx.room.ID` both work)

**Item OnUse Script** (`type: item`, assigned via item's `onUseScriptID`):
- Triggered by `use <item>` command
- Context variables:
  - `ctx.eventType` — string: `"item.use"`
  - `ctx.item` — Item entity object
  - `ctx.character` — Character entity object
  - `ctx.room` — Room entity object (if character is in a room)

**NPC Event Scripts** (`type: npc`):
- `OnAggroScript`, `OnDeathScript`, `OnFleeScript`, `OnLowHealthScript`
- `LowHealthThreshold` is a fraction of max HP. `0` or unset means `0.30`. A killing blow from above the line does not run the low-health script.
- Context varies by event type. Enemy hooks set `ctx.hook`, `ctx.roomId`, `ctx.npc`, `ctx.opponents`, and `ctx.allies`.
- `tales.combat.summon(templateId, count)` spawns enemy-template adds into the current fight. Count clamps to 1..3 and to the remaining fight cap of 3. Adds grant no loot, gold, XP, or quest credit and are removed when the fight ends. Unknown template, non-enemy, no fight, or a full cap returns 0.
- Room lines from `tales.game.msgToRoom` and `tales.game.msgToRoomExcept` during an enemy hook stay `type: "message"` and `username: "SYSTEM"`. They also carry `style: "combatEvent"`, `hook` (`onAggro`, `onLowHealth`, `onDeath`, `onFlee`), and `source` (that NPC's display name). Other scripts omit those fields. A hook flush that summoned adds sends `combatStatus` with the live `combatants` roster immediately.
- The play client renders `style === "combatEvent"` as a gold chip in the BattleStage combat log (icon per hook, `source` as a small label) and still prints the plain line. Hook `unique` is the room announcement for a one-per-character drop, sent by the loot path rather than a Lua hook. Cache-bust for that client is `?v=uniques1`.

### Context Variable Quick Reference

| Variable | OnEnter (room) | Action (custom) | Item Use |
|----------|:-:|:-:|:-:|
| `ctx.eventType` | `"player.enter_room"` | `"room.action"` | `"item.use"` |
| `ctx.room` | Room object | Room object | Room object |
| `ctx.roomID` | — | string shortcut | — |
| `ctx.character` | Character object | Character object | Character object |
| `ctx.characterID` | — | string shortcut | — |
| `ctx.user` | User object | — | — |
| `ctx.action` | — | action name string | — |
| `ctx.params` | — | action params table | — |
| `ctx.item` | — | — | Item object |

**Important**: In OnEnter scripts, you must use `ctx.room.ID` and `ctx.character.ID` (no string shortcuts). In Action scripts, you can use either `ctx.roomID` or `ctx.room.ID`.

---

## Creator UI Capabilities

### Data Table System
All entity editors use a unified **filterable, sortable data table**:
- **Per-column filtering** - Text search, enum dropdowns
- **Client-side filtering** - Instant, no API calls
- **Sorting** - Click column headers (asc → desc → none)
- **Master-detail layout** - Table + edit form side-by-side
- **Full-width mode** - Close detail panel to maximize table view

### Entity Select Modal
**MANDATORY UI GUIDELINE**:
- **NEVER use `<select>` dropdowns for entity ID references**
- **ALWAYS use `EntitySelectButton` + `EntitySelectModal`**
- Provides filterable DataTable for selecting rooms, NPCs, items, scripts, dialogs, quests, and loot tables
- Scales to hundreds of entries with search and filter

```svelte
<EntitySelectButton
  value={roomID}
  elements={rooms}
  columns={roomColumns}
  title="Select Room"
  placeholder="Select a room..."
  on:change={(e) => roomID = e.detail}
/>
```

### Creator Validation And Diagnostics
Creator editors share backend validation rules from `pkg/service/validation`:
- Inline validation panels show errors and warnings for the selected draft entity.
- Save/update requests for rooms, items, NPCs, NPC spawners, dialogs, loot tables, quests, and scripts reject error-severity broken references before data is stored.
- World Health runs cross-world diagnostics and reports structured issues with `severity`, `entityType`, `entityId`, `field`, `code`, and `message`. The message column is visible and wraps. Room, NPC, dialog, quest, item, and script IDs link to that editor with `?id=`.
- Content health (`GET /api/health`, `tales -check`) adds reachability islands, reveal scripts that name a missing exit, hidden exits with no revealer, quest objectives that cannot complete, bosses with no spawner, unknown difficulty tiers, unreferenced scripts, dangling exits, missing room items, and pack rules. A quest reward counts as an obtainable item when that quest can complete and its prerequisites can precede the quest that needs the item. Reachability starts at `startRoomID` when that room exists, otherwise the engine default. The content commit is that folder's own git HEAD, or `CONTENT_COMMIT` / `.content-commit`, or `unknown`. Live health warns with `deploy-tree-dirty` when the server checkout has tracked changes outside `import/`. `tales -check` does not run that rule. Hits can be muted. Drift compares the database with the last import. An enemy export writes authored HP, attack, and defense into the importer fields when a content base exists, and adds a `combat` block with authored and effective numbers. See `docs/content-health.md`.
- Dialog validation allows a node ID to be reached more than once, including shared branches and cycles. It reports `duplicate_dialog_node` only when two definitions of the same node disagree. Option edges that point at a node are not a second definition.
- Merchant stock with `maxQuantity` -1 is unlimited. Only values below -1 are warned.
- Deletes that call a DELETE API open a confirm dialog (entity type, name, and ID) before the request. The same dialog covers map room delete, saved spawner delete, room item removal, special exit removal, dialog-graph option and answer removal, and bulk deletion of deprecated scripts. Unsaved form rows (patrol stops, quest objectives, cardinal exits, room actions, alternate texts) stay immediate.
- Data tables show the full entity ID, with a tooltip and a copy button. The row dot marks validation errors and warnings. NPC type (Enemy, Merchant, both, Neutral) is a text badge.
- Opening a creator editor URL with `?id=<entityId>` selects that entity after the list loads. The dialog graph honors the same query.
- Ctrl+K or Cmd+K opens a search palette over rooms, NPCs, items, dialogs (including node text), quests, scripts, loot tables, spawners, skills, and character templates. Results stay grouped by type. Enter opens that entity's Creator path. A loot table opens `/creator/loot-tables?id=`. A spawner opens `/creator/spawners?id=`. The shortcut is ignored inside an input, a textarea, or the script editor. The top-bar search box (`data-creator-search`) opens the same palette and shares its query.
- Rooms, NPCs, items, dialogs, quests, scripts, loot tables, and spawners show a collapsed "Referenced by (n)" panel under the form. Deleting one of those entities warns when something still references it.
- Data table rows surface warning/error indicators when stored entities have diagnostics. An editor-specific row indicator is not used for NPC type.
- Preview/test tools validate draft dialogs, quests, rooms, merchants, and Lua scripts before content is published into the world.

### Creator navigation
The Creator screens sit in a left sidebar instead of a horizontal tab row. Groups are World (Rooms, World / Zones, Spawners, Health), Actors (NPCs, Loot tables, Character Templates, Items), Narrative (Dialogs, Dialog Graph, Quests), Systems (Skills, Scripts, Settings), and Operate (Players, Audit log, Drift). Players stays admin-only. Audit log and Drift stay visible to creators. `/creator` still opens Rooms. `?id=` links are unchanged. `/creator/quests/debug` stays on the Quests item. `/creator/npcs?view=inspector` and `/creator/rooms?view=inspector` open the inspector for the selected id. `?tab=inspector` opens the same inspector on both editors. New links use `?view=inspector`.

The sidebar collapses to an icon rail from its toggle or Ctrl+B / Cmd+B. The shortcut is ignored in inputs, textareas, selects, contenteditable editors (the script editor), and CodeMirror. Collapse and per-group folds persist in `localStorage` (`tales.creator.nav.v1`). The group that contains the current page opens on navigation. Below 1024px the rail is an off-canvas drawer: the top bar shows a menu button, and the backdrop, Escape, or a navigation click closes it.

The top bar keeps the app links, a search box (`data-creator-search`) that opens the search palette, and the red LIVE badge when `ADMIN_ENV_LABEL` is set. Below 1024px the text links and the search box are hidden so the badge stays on one row. Ctrl+K still opens the palette.

### Creator screens
1. **Rooms** - Full room editor (exits, actions, spawners, NPCs, items, scripts) plus an Inspector tab
2. **Items** - Live item instances at `/creator/items`. That route stays available and is not in the sidebar.
3. **Item Templates** - Reusable item blueprints. The sidebar entry is labeled Items.
4. **NPCs** - NPC templates and unique NPCs. The detail switches between the form and an Inspector. The loot table id links to the loot tables page.
4a. **Loot tables** - `/creator/loot-tables`. Entries pick an item template and set chance, min/max quantity, boss-only, and guaranteed. Roll ×N calls `POST /api/loot-tables/:id/roll` and shows how often each item dropped. The same seed repeats. The preview does not create items.
4b. **Spawners** - `/creator/spawners` under World. The list shows room, template, max, respawn time, and a live count from `GET /api/live/npcs` (same template and room, skipping dead copies). Filters are template, room, zone (room area), and unique NPCs with no spawner. Room and NPC links open the inspectors.
5. **Dialogs** - Dialog tree editor
6. **Quests** - Quest editor (objectives, rewards, prerequisites)
7. **Skills** - Skill/spell editor (multi-class, effects)
8. **Scripts** - Lua script editor with syntax highlighting
9. **Character Templates** - Archetype editor with modal item-template selection for starting gear
10. **World Map** - Grid-based world visualization. Fit view uses the same axis as the tiles: smaller Y is north and sits toward the top, matching authored room coordinates, so a zone filter stays on the canvas. The Reachability layer colors rooms from `GET /api/world/reachability`: green when reachable from the start room, indigo for an instance template that is reachable, and red for an unreachable island. Hidden exits are dashed and one-way exits are dotted while that layer is on. Clicking an island frames those rooms and shows each reason on its own line. Room ids in a reason open the room inspector (`?id=&view=inspector`). Script ids open the script editor. Further layers sit beside Reachability and can be on together. The choice is stored in `localStorage` (`tales.creator.mapLayers.v1`). They read `GET /api/world/overlays`: level band (NPC level range in the room, or the zone when the room has none; color is the midpoint), aggressive enemies (`aggroOnSight` count), spawners (icon, respawn time on hover), players online (live; an admin sees names, a creator sees the count), quest targets (rooms a visit, kill, talk, or deliver objective names; the badge reads Q×N and the tooltip lists the quest ids), missing art (no background image), and live instance copies (count of `~` rooms for that template). Overlay badges stay about 12px on screen at any zoom. When the tile is too small for that row, the extra badges collapse into +N instead of shrinking. The tile geometry is unchanged.
11. **World Health** - Content-health report (reachability, reveal scripts, impossible quests, bosses, pack rules, drift) plus cross-system diagnostics for broken entity references and suspicious content values, including character template starting item references
12. **Players** - Admin only. Live characters (online, guest, zone, in combat; optional older rows), a side drawer, and confirmed ops: teleport (room picker), give item (template picker), take item (that character's inventory), end combat, quest complete/reset/abandon, and re-grant starter kit. Instance copies and cleanup sit on the same page. Creators do not see this page.
13. **Audit log** - Filterable creator and ops history, before/after JSON, and Undo for an admin
14. **Drift** - Operate page at `/creator/drift`. Lists entities from `GET /api/health/drift` (added, changed, removed) and shows each changed field's before and after value. Export YAML downloads one entity from `GET /api/health/drift/export`. Export all downloads the current entities as one concatenated YAML file and skips removed rows. Health links here.

### Live Ops And Audit
- `ADMIN_ENV_LABEL` (optional `ADMIN_ENV_HOST`) feeds `GET /api/server-info`. A non-empty label shows a red `LABEL · host` badge in the top bar. Local servers leave the label empty.
- `defaultTheme` on that same response is the ruleset key `ui.default_theme` (`dark-fantasy`, `clean-hud`, or `gilded`). The engine default is `dark-fantasy`. A content pack sets it in `config/ruleset.yaml`; import records that pack and the server merges the file at startup. The play setting `auto` uses it. Settings still offers Dark Fantasy, Clean HUD, and Gilded Frame, and an explicit pick wins. Gilded Frame styles live in `public/mud-client/public/themes/gilded/`. Dialog, the quest log, the shop, notices, the combat stage, and room entity cards add that skin in `themes/gilded/w3-screens.css`. Prices stay one gold integer, drawn in the gold coin colour. Layout, timings, and the other themes stay as they were.
- Live ops and live reads run on the game command loop through `Game.Call`. The HTTP handler does not edit an online character behind that loop.
- Every op body must include `confirm: true`. The Players page and `OpsButtons` open the shared confirm dialog before the request. A success toast offers Undo only when the audit row is undoable.
- `OpsButtons` modes are `character` (end combat, re-grant starter kit), `quest` (complete quest, abandon, reset quest, mark one step done, reset one step), `npc` (heal, respawn, despawn, end combat), and `room` (teleport here, clean up one instance copy). The NPC inspector embeds the npc mode. The room inspector embeds the room mode. The component renders only when the signed-in user is an admin. Room teleport does not pass `force`.
- Teleport moves online and offline characters through `RelocateCharacter`. A fight blocks the move unless `force` aborts it first. Undo teleports back when the character is still in the destination and not in a new fight.
- Give and take follow unique-item rules and tell an online player. Undo of a give removes the added pieces. Undo of a take puts those pieces back.
- `end-combat` aborts the fight: no rewards, penalties, or healing. It is not undoable. `instance-cleanup` relocates players in a chosen copy, or deletes only empty copies when `allEmpty` is set. It does not abort a fight in the copy and is not undoable.
- `quest-step` `op=complete` with `objectiveId` advances that objective and does not grant rewards. Without `objectiveId` it completes the quest and grants rewards. Undo restores the previous progress row and does not remove XP, gold, or items already granted.
- `regrant-starter-kit` adds missing class-kit or character-template starter pieces. It does not replace equipped gear. A class with no starter list is a skipped, non-undoable success.
- NPC heal restores full HP. Respawn creates or revives an instance only when it is not already alive. Despawn refuses an NPC who is in combat. A persisted unique is marked dead rather than deleted as content.
- Audit undo of a creator update or delete restores the stored before JSON, or recreates a deleted row. It returns 409 when the entity changed since the audited write.

### Room Editor Features
- **Exit management** - Add/edit/delete exits, toggle hidden
- **Action management** - Custom interactions (response, broadcast, script)
- **NPC residents** - Assign unique NPCs to room
- **Item placement** - Add items to room
- **Spawner configuration** - NPC spawner setup
- **OnEnter script** - Lua script triggered on room entry
- **Background & mood** - Visual settings
- **Coordinates** - Grid positioning (X, Y, Z)
- **Bind point** - Allow `/bind` for respawn
- **Inspector** - Inspector tab on the room detail. It calls `GET /api/rooms/:id/inspect` for exits in and out (a hidden exit lists the script that reveals it, or says none does), resident and spawn-room NPCs, spawners (template, max, respawn time), items, the on-enter script, actions, and quests with a visit objective for this room. Reachability is the world-index result from the start room. An instance-copy id resolves to its template. Live characters in this room or a copy of it come from `GET /api/live/characters?all=1` and are admin only. Live NPC instances come from `GET /api/live/npcs`. Instance copies come from `GET /api/live/instances`. Admin ops are teleport into this room or a chosen copy, and instance cleanup for that copy, each behind the shared confirm dialog and the audit toast. Creators see the live NPC and copy rows and a note that those ops are admin only. Inbound references stay on the editor's Referenced by panel under the form. A spawner id opens `/creator/spawners?id=`. `?view=inspector` opens this tab.

### NPC Editor Features
- **Template vs. Unique** - Toggle `IsTemplate` flag
- **Basic info** - Name, description, race, class, level
- **Enemy trait** - Grouped into combat stats, behaviour, loot and rewards, and Lua hooks. Fields: creature type, combat style, difficulty, attack, defense, attack speed, XP, aggro radius, aggro on sight, call for help, flee threshold (shown as a percent), gold range, loot table, guaranteed item templates, max drops, and on-aggro, on-death, on-flee, and on-low-health scripts with the low-health line as a percent and resulting HP. Help text states the engine default when a field is unset. Loot tables, item templates, and scripts use `EntitySelectButton`.
- **Content base vs effective stats** - Imported enemies store unscaled `enemyTrait.baseStats` (`maxHitPoints`, `attackPower`, `defense`) beside the effective stats written to `maxHitPoints`, `attackPower`, and `defense`. The enemy tab labels which values the inputs edit. Effective stats are read-only and named with the tier. A named override replaces that tier's factors. An unknown tier shows `unknown difficulty tier — no scaling applied` and keeps the base numbers. Editor-created enemies have no content base; their inputs edit the stored combat stats. Saving recomputes effective stats on the server when a content base is present, and a wounded NPC keeps its current HP. YAML export writes the content base when it is present so a round trip does not scale twice.
- **Merchant trait** - Inventory, pricing, restock, accepted items
- **Dialog assignment** - Main dialog, idle dialog
- **Behavior** - State, spawn room, wander radius, patrol path, idle chatter dialog, idle chatter timeout, respawn time
- **Resident placement** - Assign `CurrentRoomID` for auto-spawn
- **Inspector** - Edit / Inspector on the NPC detail. Inspector calls `GET /api/npcs/:id/inspect` for the template summary: type, level, difficulty, content base versus effective stats (same multipliers as `GET /api/balance/enemy-scaling`; a named override replaces the tier; an unknown tier keeps the base), loot-table chances, dialog and idle-dialog links, on-aggro / on-death / on-flee / on-low-health scripts, spawners (room, respawn time, max), and quests with a kill, talk, or deliver objective for this NPC. The loot table id opens `/creator/loot-tables?id=`. A spawner id opens `/creator/spawners?id=`. A running instance id resolves to its template. Live instances come from `GET /api/live/npcs?templateId=` (room, HP, opponent names, dead-until). Admin ops on that list are heal, respawn, despawn, and end combat, each behind the shared confirm dialog and the audit toast. Creators see the live rows and a note that ops are admin only. Inbound references stay on the editor's Referenced by panel under the form. `?view=inspector` opens this tab for the `?id=` selection.

### Quest Editor Features
- **Objectives** - Add kill/collect/deliver/visit/talk/custom objectives
- **Area** - Optional regional label with dynamic suggestions and table filtering
- **Rewards** - XP, gold, item grants
- **Prerequisites** - Required quests, level requirement
- **Dialog integration** - Accept/progress/complete text
- **Repeatable** - Daily/recurring quests. `repeatReset: daily` waits until `quests.daily_reset_hour_utc` (default 4). `repeatReset: cooldown` waits `repeatCooldownSeconds`. The quest log, dialog offer, and `quests` command say "Available again in 5h 12m" until then. Client payloads carry `repeats`, `repeatReset`, and `nextAvailableAt`. Plain `repeatable: true` is unchanged. The next-step guide skips a quest with a reset while any normal quest is still open.
- **OnComplete script** - Lua script on quest completion
- **Validation panel** - Flags missing source references, objective targets, duplicate IDs, invalid rewards, missing scripts, self-prerequisites, and unresolved entity IDs before save
- **Player flow preview** - Summarizes offer source, objective lines, ready turn-in target, and rewards for quick testing while authoring
- **Quest debugger** - `GET /api/quests/:id/debug` and `/creator/quests/debug?id=`. The id comes from the URL on load and whenever that query changes, including Debug quest and Health links. Shows the step chain, where each objective can be satisfied, whether that place is reachable, and a green, amber, or red verdict using the same completability check as content health. Also shows prerequisites, follow-on quests, the offer and turn-in, rewards, and characters currently on the quest. Each character row includes the open `objectiveId`. Reset quest step is admin-only. It confirms, then POSTs `/api/ops/quest-step` with `op=reset` for that objective. The toast shows the summary and can undo.

### Skill Editor Features
- **Multi-class assignment** - Skills can belong to multiple classes
- **Resource type** - Mana-based or cooldown-based
- **Effect system** - Damage, heal, buff, debuff, DoT, HoT, stun
- **Scaling** - Attribute-based damage/healing (STR/DEX/INT/WIS)
- **Secondary effects** - Hybrid skills (e.g., damage + debuff)

### Script Editor Features
- **Syntax highlighting** - Lua code highlighting
- **Test runner** - Execute scripts with test context
- **Shared validation** - Static Lua checks are shared with import validation and Creator save/preview flows
- **Type categorization** - item, room, npc, quest, event, custom

---

## Recent Features & Best Practices

### Gilded Frame on inventory and character screens

When `body[data-theme="gilded"]` is set, the play client restyles inventory, equipment, the character sheet, item detail (quality-coloured name, green/red compare deltas, UNIQUE mark), Need/Greed, party, recipes, and the sign-in / character-creation frames. Layout, controls, and test ids stay put. The sign-in card keeps its size. Play as guest is a leather plaque with gold text. Continue with X and Continue with Google are a separate bronze plaque. Quality colours follow the theme variables (`magic` green, `rare` blue, `legendary` purple, `mythic` and `unique` orange). Empty equipment slots draw line silhouettes, and a stack count sits in the slot corner. Other themes keep their previous colours and do not show that count. Stylesheet: `public/mud-client/public/themes/gilded/w2-screens.css`.

### Additive structured messages (protocol 1)

- `examine`/`inspect` retains its prose and adds `examineResult` for items:
  `name`, `slot`, `quality`, `stats`, `value`, `description`, `detail`, and
  `effects[]` strings from `items.FormatEffect`, including script fallbacks.
  The play client consumes `examineResult` for the RoomTextOverlay item card
  (effects shown as explicit lines); prose-only examine (NPC / old server)
  still uses the dump parser.
- `characterSelected.guest: true` identifies guests; account sessions omit it.
- Character selection/reconnect in a live fight sends one `combatStatus` with
  the full roster, round, actor, deadline, target, and player queue/cooldowns.
  It does not advance or restart combat. Stale `inCombat` flags produce no snapshot.
- Optional socket `hello` negotiates protocol 1 and server build identity with
  `ok`, `upgrade-recommended`, or `too-old`. It never executes a game command.
- The web battle store restores the snapshot immediately. New fields use
  `omitempty`; clients must ignore unknown fields and types. Protocol notes:
  `docs/structured-messages.md`.

### Per-Character Hidden Exit Reveals
**Feature**: Hidden exits can be revealed individually per character via scripting.

**Best Practice — Revealing an exit in an action script** (`type: custom`):
```lua
-- In a puzzle room action script (has ctx.roomID and ctx.characterID shortcuts):
local charID = ctx.character.ID
if tales.game.hasItem(charID, "bronze-key-template-id") then
    tales.game.msgToCharacter(charID, "The key fits! A secret passage opens.")
    -- ALL THREE PARAMS REQUIRED: roomID, exitName, characterID
    tales.game.revealExit(ctx.roomID, "secret-passage", charID)
else
    tales.game.msgToCharacter(charID, "You need a bronze key to unlock this.")
end
```

**Best Practice — State reconciliation in OnEnter script** (`type: room`):

When a puzzle sets a flag AND reveals an exit, the exit reveal could fail independently. Always reconcile state in the room's OnEnter script to ensure consistency:
```lua
-- In the room's onEnterScript (type: room — uses ctx.room.ID, NOT ctx.roomID):
local charID = ctx.character.ID
local puzzleSolved = tales.game.getFlag(charID, "puzzle_solved")
if puzzleSolved then
    -- Reconcile: ensure the exit is actually revealed for this character
    if not tales.game.hasRevealedExit(charID, ctx.room.ID, "north") then
        tales.game.revealExit(ctx.room.ID, "north", charID)
    end
    tales.game.msgToCharacter(charID, "The passage stands open.")
end
```

**Storage**: `Character.RevealedExits` map (roomID → exit names), persisted to DB
**Client**: Automatic silent room update shows new exit without re-rendering description
**Idempotent**: Calling `revealExit` on an already-revealed exit is a no-op (returns true)

**IMPORTANT — Parameter order difference**:
- `revealExit(roomID, exitName, characterID)` — room first
- `hasRevealedExit(characterID, roomID, exitName)` — character first

### CopyOnPickup Items
**Feature**: Items that create personal copies instead of removing from room. Instances are bound to the collecting character. Room-placed catalog items are treated as copy-on-pickup even if the YAML flag was omitted.

**Best Practice**:
- Use for **quest items** that all players should find
- Use for **story artifacts** that don't deplete
- Set `Item.CopyOnPickup = true` in Creator UI (import also sets this for any item listed on a room)
- System tracks via `Character.Flags["collected_item:<templateID>"]`
- Instances are automatically bound (`BoundToCharacterID`) — cannot be dropped, sold, or traded
- Players use `destroy` command to discard bound items (clears collected flag, allowing re-pickup)

**Example**: Ancient scroll in library - all players can read it, but each gets a bound copy

### Item Consumables with Lua Scripts
**Feature**: Items can have both data-driven effects AND custom Lua logic.

**Best Practice**:
```go
// In Creator UI - Item Attributes:
{
  "healthRestore": 50,
  "useMessage": "You feel refreshed"
}

// Set OnUseScriptID for additional effects:
OnUseScriptID: "potion-buff-script"
```

**Script Example**:
```lua
-- potion-buff-script (type: item — ctx.character is a Character object)
local charID = ctx.character.ID
tales.game.msgToCharacter(charID, "A warm glow surrounds you.")
-- Could add temporary buff, quest progress, etc.
```

### Distributable Attribute Points
**Feature**: Players earn 2 points per level to spend on attributes.

**Best Practice**:
- Design quests/challenges that reward specific builds
- Use class caps to prevent degenerate builds (warriors can't max INT)
- Show unspent points badge in character widget
- Remind players to spend points at level-up

### Combat Balance via YAML Config
**Feature**: Difficulty multipliers loaded from `config/combat_balance.yaml`.

**Best Practice**:
- Set base stats conservatively on NPCs
- Use difficulty tier to scale for different zones
- Trivial: Tutorial enemies
- Easy: Early-game
- Normal: Mid-game
- Hard: Late-game
- Boss: Unique encounters

**Tuning**: Adjust YAML file without editing individual NPCs

### Quest Dialog Auto-Injection
**Feature**: Quest offer/progress/complete options auto-injected into NPC dialogs.

**Best Practice**:
- Design NPC dialog trees WITHOUT quest options
- System automatically adds quest options at runtime
- Set `AcceptDialogText`, `ProgressDialogText`, `CompleteDialogText` on Quest
- Quest-giver NPC should have general greeting dialog
- Use the Quest editor validation/preview panel before saving to confirm the offer, progress, ready turn-in, and reward flow
- Quest-source NPCs without a main dialog still open a quest-only conversation so numbered quest choices can accept, show progress, or turn in quests.

### Exploration XP System
**Feature**: Automatic XP rewards for discovering rooms/areas.

**Best Practice**:
- Design areas with hidden rooms for bonus XP
- Use revealed exits to reward thorough exploration
- Track via `Character.DiscoveredRooms` and `DiscoveredAreas`
- **5 XP** per room, **15 XP** for first room in new area

### Room OnEnter Scripts
**Feature**: Lua scripts executed when player enters room. Script `type` must be `room`.

**Best Practice**:
```lua
-- OnEnter scripts use ctx.character.ID (uppercase .ID, not .id)
local character = ctx.character
if not character then return end
local charID = character.ID

-- First-visit message:
if not tales.game.getFlag(charID, "visited_dungeon") then
    tales.game.msgToCharacter(charID, "You enter a dark, foreboding dungeon.")
    tales.game.setFlag(charID, "visited_dungeon", true)
end

-- State reconciliation for hidden exits (if this room has puzzle-revealed exits):
local puzzleSolved = tales.game.getFlag(charID, "dungeon_puzzle_solved")
if puzzleSolved and not tales.game.hasRevealedExit(charID, ctx.room.ID, "north") then
    tales.game.revealExit(ctx.room.ID, "north", charID)
end
```

**Context**: `ctx.room` (Room object), `ctx.character` (Character object), `ctx.user` (User object)
**Note**: OnEnter scripts do NOT have `ctx.roomID` or `ctx.characterID` string shortcuts — use `ctx.room.ID` and `ctx.character.ID` instead.

### Multi-Class Skills
**Feature**: Skills can be assigned to multiple classes.

**Best Practice**:
- Healing skills for both Cleric and Druid
- Defensive buffs for Warrior, Ranger, Rogue
- Set `ClassIDs: ["cleric", "druid"]` in Creator UI
- Registry handles lookups automatically

### NPC Auto-Spawn via CurrentRoomID
**Feature**: Unique NPCs auto-spawn into their assigned room on server start.

**Best Practice**:
- Use for quest-givers, merchants, named NPCs
- Set `NPC.CurrentRoomID` in Creator UI
- Do NOT add to spawners
- Do NOT add to `Room.NPCs` list (system handles it)

### Item Template/Instance Pattern
**Feature**: Items follow same template/instance pattern as NPCs.

**Best Practice**:
- Create item **templates** with `IsTemplate = true`
- Use `CreateInstanceFromTemplate()` for loot, rewards
- Instances track `TemplateID` for quest matching
- Use `CopyOnPickup` for shared quest items

---

## Summary of Key Patterns

### Entity ID References
- **Rooms** - Reference other rooms via `Exit.Target`
- **Items** - Reference templates via `TemplateID`, scripts via `OnUseScriptID`
- **NPCs** - Reference templates via `TemplateID`, dialogs via `DialogID`, scripts via `OnAggroScript`/`OnDeathScript`/`OnFleeScript`/`OnLowHealthScript`
- **Quests** - Reference NPCs via `Source.NPCID`, scripts via `OnCompleteScriptID`
- **Skills** - Reference classes via `ClassIDs` array

### Per-Character State
- **Flags** - Arbitrary script data (`Character.Flags`)
- **Revealed Exits** - Hidden exit discoveries (`Character.RevealedExits`)
- **Discovered Rooms** - Exploration tracking (`Character.DiscoveredRooms`)
- **Collected Copy Items** - CopyOnPickup tracking (`Character.Flags["collected_copy_items"]`)
- **Quest Progress** - Active quests (`QuestProgress` entity)
- **Equipped Skills** - Active skill slots (`Character.EquippedSkills`)

### Template/Instance Pattern
**Used by**: Items, NPCs

**Fields**:
- `IsTemplate` - True if blueprint
- `TemplateID` - Source template ID (for instances)
- `InstanceSuffix` - Unique suffix (e.g., "abc123")

**Creation**:
```go
instance, err := service.CreateInstanceFromTemplate(templateID)
```

### Scripting Hooks
**Room**:
- `OnEnterScriptID` - Player enters room

**Item**:
- `OnUseScriptID` - Player uses item

**NPC**:
- `OnAggroScript` - Enemy aggros
- `OnDeathScript` - Enemy dies
- `OnFleeScript` - Enemy flees
- `OnLowHealthScript` - Enemy HP first drops below `LowHealthThreshold` while still alive (`tales.combat.summon` may add up to 3 enemy-template copies to that fight; they grant no rewards and leave when the fight ends)

**Quest**:
- `OnCompleteScriptID` - Quest completed

**Room Action**:
- `Action.ScriptId` - Player triggers action

---

## Discovered World Atlas

### Overview
The atlas is a per-character fog-of-war map. The server compiles a **stable world layout** from room exits (and optional `coords`), then reveals only rooms this character has entered plus unnamed fog neighbors through visible exits. Web and mobile clients render the same JSON.

Area-local authored coordinates and compass exits define geography. Compact zone translations and anonymous filler ground make one overworld continent with blended biomes, coastal sea, dirt paths, and towns. Interiors remain real selectable rooms grouped under exterior anchors; decorative ground never becomes a room.

### Server
- `Character.DiscoveredRooms` / `DiscoveredAreas` persist on enter (`worldmap.MarkOn` during `TakeExit` and character select)
- `GET /api/characters/:id/map` (owner or admin) returns `PlayerMap`
- Layout package: `pkg/worldmap` — `Compile(rooms)` then `Reveal(world, character)`
- Layers: semantic `overworld` for outdoors (including positive elevation) and anchored interiors, `lower` for subterranean context/negative depth, `upper` for other positive floors
- Hidden exits do not appear until the character has revealed them
- Optional room `coords` define area-local geometry. Embedded `map_layout.json` translates zones to compact centers; no room YAML migration is required.

### Payload
```json
{
  "characterId": "...",
  "currentRoomId": "R0102",
  "currentLayer": "overworld",
  "layers": [{"id": "overworld", "name": "Overworld", "kind": "overworld"}],
  "places": [{"id": "R0102", "name": "Wildflower Field", "x": 2, "y": -1, "layer": "overworld", "biome": "meadow", "terrain": "grassland", "mapRole": "surface", "kind": "wild", "discovered": true, "canTravel": true}],
  "paths": [{"from": "R0101", "to": "R0102", "dir": "north", "kind": "trail", "layer": "overworld"}],
  "regions": [{"id": "Z01_meadows_forest_path:overworld", "name": "Meadows Forest Path", "hull": [[1, -2], [3, -2], [3, 0]], "biome": "meadow"}],
  "landscape": [{"x": 2, "y": -1, "terrain": "grassland"}]
}
```
Fog neighbors are places with `discovered: false`, empty `name`, and `kind: "uncharted"`.

### Client
- Map widget (player-facing name; same `minimap` widget slot / atlas protocol) receives the atlas over WebSocket on enter, and can also fetch `GET /api/characters/:id/map`
- Action-bar **Map** chrome / Expand always opens a real fullscreen Map overlay (dimmed play surface, Esc/X close) via `MapOverviewOverlay` portaled to `document.body` — Inventory-style centered panel (`#map-overview-overlay` / `.map-panel`), not clipped to the Map widget and not Materialize `.modal`
- Area names: drawn over landscape regions (gold/cream + dark stroke, font scales with zoom); uses `region.name` / `place.areaName` only — never invents labels. Room-name LOD unchanged: mid = current + adjacent; zoomed in = more room names (collision-aware). Compass/vertical exit words are never painted (exit ticks only)
- Cartographer overlay fills ~80% of the viewport on desktop (side intel rail). On phone (≤768px) it is full-bleed / safe-area; intel is a bottom sheet (peek summary + Travel, expand for exits/residents). Tap selects; Travel is a primary button on select (no double-tap required; Inspect remains optional). Pinch-zoom and pan keep scale. Compact Map tab selection shows Travel immediately (Inspect optional); the inner Map title / Open Map chrome is removed so the MAP tab is the only header. During Travel the camera smoothly follows the player; when Travel completes in the Cartographer overlay, the overlay fades out and closes (compact Map tab stays open). Active quests in the Quest Log group by area with collapse/expand (current area expanded). Equipment paper-doll uses larger slots and a denser column layout. BattleStage shows gilded fantasy frame buff/debuff icon rows with remaining rounds from combatant `statusEffects`.
- Exterior elevations and upstairs interiors stay on Overworld; dungeons, crypts, cellars, and sewers use Lower. Interior selection retains the actual room ID.
- Title stays **Map**. Layer tabs (Overworld/Lower/Upper) only when `atlas.layers` has more than one entry. Compact optional widget opens fullscreen; Map chrome pin is primary
- Compact minimap and fullscreen MapOverviewOverlay share `atlasRenderer.paintAtlas`, surface grouping, and the cached blended continent. Lower has a dedicated torch-lit rock/floor/corridor scene; Upper retains terrain tiles. Fog hides uncharted art; one gold marker follows the current room or its exterior anchor.
- The widget auto-fits discovered places into its panel and keeps that fit (canvas is out of flow so it cannot resize the widget)
- Layer tabs, pan, wheel zoom, click-to-travel along discovered paths
- Desktop/mobile action bars (Option C): room-only dirs + room actions + Shop when a merchant is present; fixed INV / MAP / SAY chrome; **Recipes** seeded by default for crafting discoverability; optional Look/Rest/… pins via ⋯; layout revision migrates legacy Look/pin clutter and seeds Recipes onto rev-2 bars
- Pickup is one shared panel (`PickupPanel.svelte`, logic in `pickupPanel.js`), opened from the desktop Pickup button and from the phone action-bar button and room hand button. Rows use item art, normal-case names, rarity colors (unique is gold), a ×N stack, and a `type · effect` hint. Currency is `type: currency` or a `currency` tag; a coin name matches only when the row has no type. Pick up all is a separate gold footer button when more than one item is on the ground. Keys 1–9, Enter, and Esc apply while the panel is the top overlay and a text field is not focused. The list scrolls; the header and footer stay put. On screens 600px and under the panel is a full-width bottom sheet. It closes itself when nothing pickable remains. Room item rows keep `id`, `name`, `noPickup`, and `copyOnPickup`, and add optional `type`, `subType`, `quality`, `unique`, `quantity`, and `effect` (one `FormatEffect` line, or `Heals N HP`). Play cache tag `curve1`.
- Inventory chrome opens a popup overlay by default; preference can switch to on-screen widget / mobile sheet
- Room action/system reaction toasts render large and centered on the room hero art (not the command log)
- Same JSON is the contract for a future mobile renderer

### Key Files
- `pkg/worldmap/` — layout, biomes, hulls, discovery, reveal
- `pkg/server/handler/charactermap.go` — REST endpoint
- `public/mud-client/src/game/widgets/MinimapWidget.svelte` — local Map renderer + fullscreen overlay host
- `public/mud-client/src/game/widgets/atlasRenderer.js` — layer framing, hit targets, label LOD, single you-marker
- `public/mud-client/src/game/widgets/continentRenderer.js` / `coastline.js` / `mapArt.js` / `surfaceAtlas.js` — cached zoom scenes, organic shores, room art, town/interior grouping, roads/bridges
- `public/mud-client/src/game/widgets/undergroundRenderer.js` — rock-sided corridors, themed floors, torches, stairs
- `public/mud-client/public/map-tiles/terrain-sheet.png` — shared 48px terrain and transparent building sprites
- `public/mud-client/src/game/hudPrefs.js` — Option C action-bar chrome/pins and hotbar helpers. Kit skills come from `GET /api/classes` (`?v=classkit1`)

---

## Game Client Tab Container Widget

### Overview
The Tab Container widget allows players to group multiple widgets into a single resizable container with switchable tabs. This reduces layout clutter, especially on smaller screens (e.g., Character + Inventory + Equipment in one tab group).

### Features
- **Multiple tab containers**: Unlimited tab containers can exist on screen simultaneously
- **Tab switching**: Click tabs to switch between contained widgets; active tab is highlighted amber
- **State preservation**: All tab content is kept mounted (via CSS visibility) — scroll positions, terminal buffers, and widget state are preserved when switching tabs
- **Tab labels**: Automatically derived from widget registry (icon + name)
- **Add/remove tabs**: Available in edit mode only — "+" button opens a dropdown of available widget types, "x" button removes individual tabs
- **No nesting**: Tab containers cannot be placed inside other tab containers (enforced at registry, store, and UI levels)
- **maxInstances enforcement**: Widgets inside tab containers count toward their `maxInstances` limit (e.g., you can't add Character both as a standalone widget and inside a tab)
- **Persistence**: Tab configuration (which widgets, active tab) persists to localStorage alongside layout data

### Data Model
Tab containers store extra fields on the widget layout item:
```javascript
{
  id: 'tabcontainer-1707000000000',
  widgetType: 'tabcontainer',
  tabs: [
    { widgetType: 'character', id: 'tab-character-001' },
    { widgetType: 'inventory', id: 'tab-inventory-002' }
  ],
  activeTabIndex: 0
}
```

### Key Files
- `public/mud-client/src/game/widgets/TabContainerWidget.svelte` — The tab container component
- `public/mud-client/src/game/layout/WidgetComponents.js` — Shared component map used by both WidgetGrid and TabContainerWidget
- `public/mud-client/src/game/layout/WidgetRegistry.js` — Registry entry (`tabcontainer` type, `layout` category)
- `public/mud-client/src/game/layout/LayoutStore.js` — Tab management methods (`addTabToContainer`, `removeTabFromContainer`, `setActiveTab`)

### LayoutStore Tab Methods
```javascript
layoutStore.addTabToContainer(containerId, widgetType)  // Add a widget as a new tab
layoutStore.removeTabFromContainer(containerId, tabIndex) // Remove a tab by index
layoutStore.setActiveTab(containerId, tabIndex)           // Switch active tab
```

---

## Local Account Authentication (Fork Default)

### Overview
The fork boots with `auth: local` (from `config/gamemode.yaml`), so registered users sign in with a username and password instead of an external Auth0 tenant. Auth0 stays available by setting `AUTH_MODE=auth0`.

### Service (`pkg/authlocal/`)
- Passwords are hashed with Argon2id (`pkg/authlocal/password.go`); API responses never include the hash.
- Session tokens are HS256 JWTs with issuer `talesmud-local`, `kind: "local"`, a 7-day TTL, subject = user ID, and `sv` = `SessionVersion`. Resetting a password bumps `SessionVersion` and revokes existing tokens.
- `Register`/`Login` create or verify an account. Usernames are 3–20 lowercase letters/digits/underscore, passwords 8–128 characters, and emails must be full addresses. Login attempts burn a dummy Argon2id hash to reduce timing leaks.
- Secret resolution (`authlocal.ResolveSecret`): `SESSION_SECRET`, then game-mode `session_secret`, then a generated key persisted at `secret_path` (default `data/session.key`, mode 0600).
- Forgot/reset use hashed one-time tokens in the `auth_tokens` table; the default `OutboxMailer` writes them to `data/auth-outbox.log` and reports `DeliversExternally() == false`, so `/api/auth/forgot` and `/api/auth/reset` are mounted only when a real mailer is configured.

### Endpoints
| Method | Path | Auth | Notes |
|--------|------|------|-------|
| POST | `/api/auth/register` | public | Create account, returns a session token |
| POST | `/api/auth/login` | public | Username + password, returns a session token |
| GET | `/api/auth/me` | Bearer | Current account view |
| POST | `/api/auth/forgot` | public | Only when reset mail is configured |
| POST | `/api/auth/reset` | public | Only when reset mail is configured |

Local tokens work as `Authorization: Bearer` on protected REST routes and on `POST /api/ws-ticket` for WebSocket upgrades. `AuthMiddleware` checks guest HMAC, device sessions, then local sessions before attempting Auth0 RS256, so local tokens never reach the external provider.

### Configuration
- `AUTH_MODE=local` (default) enables the service; `AUTH_MODE=auth0` disables it and uses Auth0 (`AUTH0_AUDIENCE`/`AUTH0_DOMAIN`/`AUTH0_WK_JWKS` required only then).
- `config/gamemode.yaml` sets `auth: local`; the binary loads it automatically when no `-config` flag is passed, and the Docker image copies it into `/app/config/`.

---

## Guest Mode System

### Overview
Guest mode allows anonymous visitors to play TalesMUD for 30 minutes without Auth0 registration. Designed for embedding a live demo on the website.

### Guest Service (`pkg/service/guest.go`)
```go
type GuestService interface {
    CreateGuestSession(remoteIP string) (token string, err error)
    ValidateGuestToken(tokenStr string) (userID string, err error)
    CleanupExpiredGuests()
    StartCleanupLoop()
}
```

### Guest Session Lifecycle
1. Client calls `POST /api/guest` (public endpoint)
2. Server checks `ServerSettings.GuestsAllowed` and `MaxGuestAccounts`
3. IP rate limit checked (10 per hour per IP)
4. Random character created from system template presets with full starter items. Signed-in create uses the same equip path: each `StartingItems` name becomes a new item copy (`IsTemplate` false) in the listed slot. Presets take those names from the class catalog `starting_items`. An id that is not on the roster falls back to the stored template's names. Existing characters are not updated.
5. Character spawned in `ServerSettings.StartRoomID` (default `R0001` if that room exists); auto quests for that zone are granted
5a. Entering a room grants auto-source quests for that room's area (Z01 meadows: QST010*) so they fire after leaving Z00
5b. Lua `tales.game.giveItem` notifies collect-quest progress (foraging, script rewards)
5c. Quest YAML `onAcceptScriptId` runs after a dialog accept (Z01 Wren reveals the creek burrow)
6. Character `MaxLevelCap` set to 5
7. User created with `IsGuest=true`, `GuestExpiresAt=now+30min`
8. HMAC-SHA256 token signed with `GUEST_SECRET` env var
9. Token returned to client, stored in `sessionStorage`
10. On WebSocket connect: timeout goroutines start (5-min warning + expiry)
11. On disconnect: 5-min grace period before deleting guest data
12. Background cleanup loop removes expired guests every 5 minutes

### Server Configuration
```go
// In ServerSettings:
GuestsAllowed    bool  // Enable/disable guest mode (default: true)
MaxGuestAccounts int   // Max concurrent guests (default: 20, 0 = unlimited)
StartRoomID      string // New/guest spawn room (default "R0001")
```

### User Entity Guest Fields
```go
// In User:
IsGuest        bool       // Marks temporary guest account
GuestExpiresAt time.Time  // When guest session expires
```

### Per-Character Level Cap
```go
// In Character:
MaxLevelCap int32  // 0 = use global MaxLevel, otherwise per-character cap

// Helper method:
func (c *Character) GetEffectiveMaxLevel(globalMax int32) int32
```

The leveling system (`CheckLevelUp`, `ApplyLevelUp`) respects `MaxLevelCap` automatically.

### Frontend Guest Flow
- `WelcomeScreen.svelte` — logged-out choice: "Continue with X", "Continue with Google", "Email and password", and "Play as guest"
- `App.svelte` — `handleGuestPlay()` stores token in sessionStorage, skips onboarding. Logout clears that token so the next load is the welcome choice
- Account menu — guests see Continue with X, Continue with Google, Email and password, and End Session; a signed-in player sees "Switch character" and "Log out"
- `api/guest.js` — `createGuestSession()` API client

### Authentication
- Classic public device login: `POST /api/device/start` (`client`, `client_version`, `proto: 1`) and `POST /api/device/poll` (`device_code`) share `pkg/devicecode` with SSH. A separate hashed 256-bit poll secret, app client metadata, interval backoff, IP/global start limits and IP/device poll limits protect the flow. Explicit authenticated `/activate` confirmation issues one signed 30-minute account JWT accepted by character APIs and `/ws`; guests cannot confirm. `pkg/authsession` checks issuer, audience, mandatory expiry, bans and the existing session version, using the `SESSION_SECRET` / persistent session-key resolver. Door remains on its existing login. See `docs/device-login.md`.
- Guest HMAC tokens are validated before Auth0 JWTs in `AuthMiddleware`. The token is accepted only when the loaded user is a guest. An Auth0 token must include the configured audience and issuer.
- Token claims: `sub` (RefID), `uid` (user entity ID), `exp` (30min), `guest: true`
- If `GUEST_SECRET` is not set, a random key is generated at startup
- Local username/password sessions (Argon2id) are the fork default: `config/gamemode.yaml` sets `auth: local`, or set `AUTH_MODE=local`. Set `AUTH_MODE=auth0` to use Auth0 instead. API responses omit the password hash. Login attempts are limited per client address. `X-Forwarded-For` is trusted only from loopback unless `trusted_proxies` or `TRUSTED_PROXIES` says otherwise.
- `presentation: door_tui` serves `public/door` and `GET /api/door/config` (title, subtitle, token key). Classic mode does not mount `/door`. The page paints live rooms, exits, actions, NPCs, resources, and combat status, plus the last few command replies. Keys and typed lines become engine commands. A world pack may add `keymap.yaml` (per room, per area, and a combat overlay) and `character_paths.yaml`. `d` stays down. With no character selected, the page asks for a name, then a numbered path, then sex. An existing name is selected. A new character uses the pack path when that file is present, and otherwise a numbered system template. Reconnect runs the new-day pass without another select.
- `presentation: door_tui` serves `public/door` and `GET /api/door/config` (title, subtitle, token key). Classic mode does not mount `/door`. The page paints live rooms, exits, actions, NPCs, resource labels, and combat status. The header uses live combat hit points and shows experience beside gold. The fight log stays until any key after the fight ends, including Enter, and a bare extra attack on that screen is not sent. The status line stays pinned above the log. The stats key paints hit points, gold, worn gear, and a gems count. A compass letter uses an open exit before a menu bind, and open east and west exits are listed. The index heading uses the configured title. A one-shot reply stays on the screen that produced it. A merchant room shows a numbered catalog, six rows at a time. `buy` and `sell` accept that number or a name. Keys and typed lines become engine commands. `:` starts a command line in the browser before later letters can hit hotkeys. A pack key whose command is `logout` tells the page to drop the session and return to the sign-in form. A world pack may add `keymap.yaml` (per room, per area, and a combat overlay) and `character_paths.yaml`. `d` stays down. With no character selected, the page asks for a name, then a numbered path, then sex, and the movement legend stays off that prompt. An existing name is selected. A new character uses the pack path when that file is present, and otherwise a numbered system template. The path name is stored on the `path` flag and the stats screen shows it. Reconnect runs the new-day pass without another select.

---

## SSH Access

SSH is off by default. `ssh.enabled` starts one listener in the current process. Classic Auth0 and door local-auth are separate processes and separate ports. Setup, limits, and player commands are in `docs/ssh-access.md`.

### What a session can do

- Guest SSH reuses guest creation. The account stays `IsGuest` with role `player` and no creator or admin role. It cannot confirm or deny a device code, link a key, open a creator or admin route, or change account fields (`PUT /api/user` is 403). The same connection stays that guest. The device lobby does not enter the game before confirm.
- A linked public key enters as that account. Creators and admins use the same check as everyone else.
- Keyboard-interactive reaches a lobby only. The game attaches after the web confirm.
- `ssh.max_session: 0` leaves account sessions uncapped. Guest sessions use `ssh.guest.max_session` (default 30 minutes). Idle applies to play on both.
- Output is queued per session (128 messages). A full queue drops that session instead of blocking the game loop.
- Door frames sanitize player and creator text. OSC, DCS, C1, and bidi controls are dropped. Pack screen CSI is kept on the art path. SGR color in a frame is kept.
- Deleting a linked key, or banning the account, closes that account's live SSH sessions in this process.

### Stored keys

`ssh_keys` is a SQLite JSON row (`id`, `data`) plus a unique index on `json_extract(data, '$.fingerprint')`. The row stores the account ref, fingerprint, key type, authorized_keys line, label, timestamps, and `created_via` (`device` only). A web paste is rejected and does not occupy that index. The local activate page lists and revokes linked keys and has no paste field. HTTP list responses omit the key blob and the last-used address. The owner still receives the full fingerprint. Device lookup returns only the shortened fingerprint.

Accepted public keys: ed25519, sk-ed25519, ecdsa-sha2-nistp256/384/521, and ssh-rsa. `ssh-dss`, private-key blobs, and newlines are rejected. Signature algorithms stay the modern set from the SSH library (RSA signatures are rsa-sha2 only).

### Play client

Classic `/activate` is a standalone Auth0 page for SSH and app codes, with “Connect this terminal/app” wording and explicit Confirm and Deny actions. Legacy `/play/?activate=<code>` links go there. The code is kept in `sessionStorage` under `talesmud_ssh_activate` and removed from the address bar before Auth0 redirects. Guests cannot confirm and do not get the SSH keys menu. The account menu item is on the desktop chip, the phone header, and the onboarding menu.

### Lua

`tales.users.isGuest(id)` is false when the user is missing. `ctx.user.isGuest` is the same flag. `tales.characters.top` skips guest rows unless `guests.persistent_effects` is on. A row with no user id stays on the board.

---

## Quiet Dark appearance

Quiet Dark: optional minimal dark client theme, opt-in via `ui.default_theme` or Settings. `dark-fantasy` stays the engine default. Players can also pick it in Settings, or follow the server with Auto.

---

## Derived Attributes (Phase 2.3)

Phase 2.3 introduced a single read-only derivation for combat-facing numbers (`pkg/entities/characters/derive.go`) so the combat engine, the client payload, and the talent system all read one shared projection instead of recomputing per caller.

### DeriveStats formulas

```go
// pkg/entities/characters/derive.go
func DeriveStats(c *Character, classDef *classkit.Def) DerivedStats
```

| Field | Formula | Bounds |
|-------|---------|--------|
| `MaxHP` | `c.MaxHitPoints` verbatim | mirrors the stored pool |
| `MaxMana` | `c.MaxMana` verbatim | mirrors the stored pool |
| `Stamina` | `STA + 2 * Level` | min 1 |
| `Initiative` | `DEXMod + Level/2` | min 0 |
| `Resist` | `WISMod + CONMod + Level/2` | min 0 |
| `CritChance` | `5% + DEXMod * 1%` + kit bonus | clamped to `[0, 95]` |

The kit bonus comes from the `classkit.Def` for the character (`classDef` is resolved with `classkit.Lookup(c.Class.ID)` when nil; an unknown class adds no bonus): `+2%` when `Balance.Swings > 1` (multi-swing kit) and `+2%` when `Primary == "DEX"`.

The tuning constants live at the top of `derive.go`: `derivedStaminaPerLevel = 2`, `derivedCritBasePercent = 5.0`, `derivedCritPerDEXMod = 1.0`, `derivedCritSwingsBonus = 2.0`, `derivedCritDEXPrimaryBonus = 2.0`, `derivedCritCapPercent = 95.0`.

`MaxHP`/`MaxMana` deliberately repeat the stored character fields: combat snapshots them when building a `CombatantRef` (`pkg/mudserver/game/combat/engine.go:114`), so the derived view never reshapes a live fight. The engine's natural-20 crit roll and the `1d20 + DEXMod` initiative roll are untouched by derivation — `CritChance` and `Initiative` are projections for UI and talent math, not inputs to those rolls.

### Talent projections on DerivedStats

`DeriveStats` finishes with `c.ApplyTalentBonuses(&ds)` and then re-clamps `CritChance`. The talent projections are separate fields so a legacy character (no talents) derives exactly the pre-talent values:

| JSON field | Type | Meaning |
|------------|------|---------|
| `damageBonus` | float | +% basic attack damage |
| `spellDamageBonus` | float | +% spell damage |
| `damageReduction` | float | +% damage taken reduction |
| `maxHpBonusPercent` | float | +% max HP |
| `blockChance` | float | +% block chance |
| `staminaRegen` | int32 | + stamina regen |
| `spellCooldownBonus` | float | -% spell cooldown |
| `healBonus` | float | +% heal output |
| `evasion` | float | +% evasion |
| `potionBonus` | float | +% potion strength |

`MaxHPBonusPercent` is intentionally **not** folded into `MaxHP`: `MaxHP` mirrors the pool combat snapshots, so callers that want the talent-adjusted pool multiply `MaxHP` by `(1 + pct/100)`. Talent application sums per effect (`ApplyTalentBonuses` in `derive.go`), so map iteration order does not matter; unknown talent ids (for example an id removed from the catalog) are skipped, and `nil` character/map inputs are no-ops.

### Where derived stats are used

- **Character update payload** — `NewCharacterUpdateMessage` (`pkg/mudserver/game/messages/responses.go:459`) computes `DeriveStats` once and emits `stamina`, `initiative`, `resist`, `critChance`, the `talents` state, and the talent projections (`omitempty`, so they are absent at zero).
- **Combat** — `CreateCombatantFromCharacter` copies `derived.MaxHP` / `derived.MaxMana` into the `CombatantRef`.
- **Talent effects** — every rank bought through `talents spend` changes the next `DeriveStats` call; nothing has to be stored on the derived side.

---

## Progression (Phase 2.4): XP Curve, Talents, Prestige

Design document: [`docs/design/PHASE2_4_PROGRESSION.md`](docs/design/PHASE2_4_PROGRESSION.md). Everything in this section is implemented (steps 2.4.1–2.4.7) unless stated otherwise.

### XP curve and level cap

- `pkg/mudserver/game/leveling/xp_curve.go` precomputes `XPTable[51]int32` with a 3-segment piecewise formula:

  | Levels | Formula per level |
  |--------|-------------------|
  | 2–5 | `30 + 10 * l^0.60` |
  | 6–15 | `50 + 15 * l^1.05` |
  | 16–50 | `85 + 17 * l^1.10` |

- Cumulative targets (verified in code): L2 ≈ 45, L5 ≈ 203, L10 ≈ 1,120, L20 ≈ 4,947, L30 ≈ 11,794, L40 ≈ 21,271, L50 ≈ 33,454.
- `GetXPRequired(level)` consults `ruleset.XPRequired(level)` first, so a `progression.xp_required` override in `config/ruleset.yaml` wins over the built-in table without a code change.
- Cap: `leveling.MaxLevel = 50` (hard cap) with `ruleset.LevelCap()` (`config/ruleset.yaml` → `progression.level_cap: 50`) driving `CheckLevelUp`, `GetXPForNextLevel`, and `GetXPProgress`. Guest characters keep their per-character `MaxLevelCap` (5).
- XP sources: combat (`game_combat.go`), quests (`pkg/service/quests.go`), exploration (`leveling/xp_sources.go`, 5 XP per new room / 15 XP per new area).

### Talent milestones

- `leveling.TalentMilestoneInterval = 5`: levels **5, 10, 15, 20, 25, 30, 35, 40, 45, 50** each grant 1 point → **10 points** before prestige.
- `TalentMilestonesBetween(oldLevel, newLevel)` counts milestones in `(old, new]`, so a multi-level jump credits every crossed milestone exactly once and never double counts on a reload.
- `ApplyLevelUp` adds the points (`char.Talents.Points += gained`) and returns them in `LevelUpResult.TalentPointsGained` / `UnspentTalentPoints`; the level-up banner ends with `+ N Talent Point(s) earned!` and points at `talents list` / `talents spend <talent>`.

### Storage

`Character.Talents` is a JSON field on the character document — **no table and no DDL migration**:

```go
type CharacterTalents struct {
    Points   int32            `json:"points"`   // unspent talent points
    Spent    map[string]int32 `json:"spent"`    // talentID -> rank
    Prestige int32            `json:"prestige"` // prestige level (0 = none)
}
```

Legacy characters load with the zero value (`Points: 0`, `Spent: nil`, `Prestige: 0`); `EnsureTalents()` lazily initializes the spent map on write, and `TalentRank()`, `SpendTalent()`, `CanLearnTalent()`, `PrestigeMultiplier()` and `ApplyPrestige()` (`pkg/entities/characters/talents.go`) all treat nil as empty.

### Talent catalog (`pkg/talents`)

Pure data package: id, name, category, effect, per-rank amount, allowed schools, description. 12 talents in 3 categories; one deterministic effect per talent so application order is irrelevant.

| ID | Name | Category | Effect | Amount/rank | Schools |
|----|------|----------|--------|-------------|---------|
| `talent_power_strike` | Golpe Poderoso | offensive | basic attack damage | +5% | martial |
| `talent_critical_eye` | Ojo Crítico | offensive | crit chance | +2% | martial, shadow |
| `talent_mana_surge` | Oleada de Maná | offensive | spell damage | +10% | fire, ice, arcane |
| `talent_iron_skin` | Piel de Hierro | defensive | damage reduction | +3% | martial, ice |
| `talent_endure` | Resistencia | defensive | max HP | +5% | martial, divine, fire, ice, arcane, nature |
| `talent_shield_master` | Maestro Escudo | defensive | block chance | +2% | martial, divine |
| `talent_wind_step` | Paso del Viento | utility | initiative | +1 | martial, shadow, nature |
| `talent_second_wind` | Segundo Aire | utility | stamina regen | +1 | martial, nature |
| `talent_spell_momentum` | Impulso Arcano | utility | spell cooldown | -5% | arcane, fire, ice |
| `talent_healer_gift` | Dón Sanador | utility | heal output | +5% | divine, nature |
| `talent_shadow_step` | Paso Sombrío | utility | evasion | +2% | shadow |
| `talent_nature_bond` | Vínculo Natural | utility | potion strength | +5% | nature |

API: `All()` (stable order), `Lookup(id)` (case-insensitive, also accepts the id without the `talent_` prefix), `ForSchool(school)`, `Available(school, id)`, `IDs(defs)`.

**Class gating** goes through `classkit.TalentsForClass(classID)` (`pkg/classkit/registry.go`): an explicit `talents:` list on the class YAML wins, otherwise the class offers every talent of its classkit `school`. No content-pack edit is required for the default case.

### Applying effects

`Character.ApplyTalentBonuses(&ds)` (called at the end of `DeriveStats`) walks `Talents.Spent`, looks each id up in `pkg/talents`, multiplies `Amount` by the rank and adds it to the matching `DerivedStats` field. Buying a rank therefore changes combat/derived values on the next `DeriveStats` call with no separate cache to invalidate.

### Command: `talents`

Registered as `talents` / `talent` (`commandprocessor.go:150`, `pkg/mudserver/game/commands/talents.go`):

| Command | Behavior |
|---------|----------|
| `talents` or `talents list` | Unspent points, prestige line (when `Prestige > 0`, with the +XP%), and the class talents grouped by `OFFENSIVE` / `DEFENSIVE` / `UTILITY` with id, name, current rank and description |
| `talents spend <talent>` | Buys one rank. Rejected in combat, for an unknown id/name (id, display name, or a unique prefix of either), for a talent the class cannot learn, or when `Points <= 0`. Persists through `CharactersService().Update` and pushes a `characterUpdate` message |

Spending is blocked while `combatEngine.IsPlayerInCombat(char)`; the reply points at `talents list` and reminds the player that a point is earned every 5 levels.

### Prestige

- **Requirement:** level 50 (`leveling.MaxLevel`). `prestige` prints status at any time (`Current Prestige`, `Level: x / 50`, and — once eligible — what a reset grants); `prestige confirm` is refused below level 50 and while the character is in combat.
- **`prestige confirm`** runs `ApplyPrestige()` (`pkg/entities/characters/talents.go`), which is irreversible: `ResetToLevelOne()` rewinds level → 1, XP → 0, attributes → class template base, unspent/spent attribute points → cleared, HP/mana pools → class base, hotbar → `skills.FillHotbar(classID, 1, nil)`; then `Talents.Prestige++` and `Talents.Points++`.
- **Persist across prestige:** spent talents, prestige level, gold, inventory, equipped items, discoveries, friends, and `AllTimeStats`.
- **Permanent bonus:** `PrestigeMultiplier() = 1 + 0.05 * Prestige` (+5% XP per prestige level), applied at every XP grant:
  - combat share + first-kill XP — `pkg/mudserver/game/game_combat.go:1415`
  - quest rewards XP — `pkg/service/quests.go:1086`
  - gold is **not** multiplied.
- Persisted through `CharactersService().Update`; the client gets a `characterUpdate` message with the new level, points, and prestige.

### Content pack impact

None. `import/adventure/` XP rewards and quest XP are transparent to the curve, the talent catalog, and the prestige multiplier; talents are filtered by class school, not by pack.

---

## Client UX Enhancements (Phases 3.2, 3.4, 3.7)

### Command history + tab completion (3.2)

`public/mud-client/src/game/commandHistory.js` is pure helpers (no Svelte, no DOM); persistence goes through an injected `Storage`-like object so tests can pass a fake and SSR/quota failures stay silent.

- **History:** `pushHistory` appends with consecutive-dedup and a cap of `HISTORY_LIMIT = 50`; `createHistoryState` / `historyPrev` / `historyNext` / `resetHistoryCursor` implement ArrowUp/ArrowDown browsing that preserves the in-progress draft when you leave history; `loadHistory` / `saveHistory` round-trip `localStorage` under `talesmud_command_history`.
- **Tab completion:** `filterCompletions`, `autocompleteSource`, and `tabComplete` complete the first token against known commands and later tokens against room player names. `GAME_COMMANDS` ships a static list (movement, perception/info, social + global channels, combat, items/world, party) so completion works before any server payload; the terminal widget merges in server-known commands when available.
- **Wiring:** `widgets/TerminalXWidget.svelte` owns the draft/history state and calls the helpers directly (`createHistoryState`, `pushHistory`, `historyPrev`/`historyNext`, `loadHistory`/`saveHistory`, `tabComplete`). The desktop `widgets/TerminalWidget.svelte` gets the same behavior through `echo/LocalEchoController.js` → `echo/HistoryController.js`, and contributes its autocomplete sources (`GAME_COMMANDS` for token 0, room player names afterwards).
- **Tests:** `src/game/commandHistory_test.mjs`, `src/game/HistoryController_test.mjs`.

### Minimap exit legend (3.4)

`public/mud-client/src/game/minimapDirections.js` turns server-emitted room exits into a compact direction strip for the minimap.

- `DIRECTION_ORDER` (N/NE/E/SE/S/SW/W/NW/up/down/in/out), `DIRECTION_GLYPHS`, and `DIRECTION_ALIASES` (single letters, `upstairs`, `enter`, …) feed `normalizeDirection()` and `directionGlyph()`.
- `buildExitLegend({ exits, places, visitedRooms })` returns `{ key, glyph, label, explored }` per exit, using the atlas discovery flags (`places[].discovered`) or locally visited rooms. Undiscovered or unknown targets render as `?` — nothing the server has not disclosed is invented.
- **Wiring:** `widgets/MinimapWidget.svelte` renders an "Exits" strip from `exitLegend`.
- **Tests:** `src/game/minimapDirections_test.mjs`.

### Responsive layout (3.7)

Phone-width media queries added to the play shell and overlays:

| File | Breakpoint | Change |
|------|-----------|--------|
| `src/game/Game.svelte` | `max-width: 768px` / `420px` | `.gameContainer` padding tightened (mobile container keeps `padding: 0`) |
| `src/game/ui/InventoryOverlay.svelte` | `max-width: 768px` | Full-width overlay panel, no side gutters, smaller radius |
| `src/game/ui/QuestNotifications.svelte` | `max-width: 480px` | Notification stack pinned to the viewport with `min-width: 0` / `width: 100%` so it no longer forces a horizontal scrollbar |

### Inventory weight display (3.5, parcial)

- `game/inventoryEngine.js` — motor puro de inventario con helpers de peso (`carryWeight`, `isOverweight`, `canCarry`), `InventoryItem`, `InventoryEngine` (`addItem`, `removeItem`, `getSummary`, `findItemByName/Id`, `getEquippedItems`, `getGrayedSlots`, `onDirty`).
- `game/ui/InventoryWidget.svelte` — muestra `totalCarriedWeight/maxWeight` con la clase CSS `overweight`; el overlay se alterna con la tecla `I` (`KEY_INVENTORY_QUICK` en `keyboardShortcuts.js`).
- **Límite actual:** es **solo aviso visual**. `maxWeight` está fijo en 100 y nada lo sobreescribe; el servidor no emite `weight` por objeto (el contador se queda en `0/100`) y no hay encumbrance ni rechazo de `get`/`pickup` por peso.

---

## Phase 6 LLM Integration (Design And Foundation)

**Status (2026-11-01):** diseñado en [`docs/design/PHASE6_LLM.md`](docs/design/PHASE6_LLM.md) (§0–6.7) y parcialmente implementado en el árbol de trabajo (sin commitear). FASE 6.3 runtime —incluidos mecanismos B/C— está cableada; otros bloques de Fase 6 permanecen en curso:

- **Cut A — generación de personaje:** `pkg/server/handler/generate.go` renderiza prompts de `llmprompt` a través de `llmgen.Pipeline` (`POST /api/generate/character`).
- **B16 — endpoints creator:** `pkg/server/handler/llm.go` con `POST /api/creator/llm/quests/generate` (dry-run por defecto), `POST /api/creator/llm/quests/contextual`, `POST /api/creator/llm/rooms/:id/description` (preview, no persiste) y `GET/POST /api/creator/llm/prompts` (+ `/prompts/:id/test`).
- **B16/B24 — narrador DM (FASE 6.5, modo A):** ver el bloque "Dungeon Master AI (FASE 6.5, modo A)" más abajo.
- **B18 — quests contextuales en juego:** comando `offer <npc>` / `offer accept|decline` sobre `llmgen.GenerateContextualQuest` (ver "Contextual Quest Offers" en Quest System).
- **B25–B29 — NPC runtime (FASE 6.3, cortes 1+2+3 y mecanismos B/C):** `ask <npc> [pregunta]`, memoria persistente con resumen LLM, personalidad Big5/objetivos/secretos/relaciones y guard de secreto literal; config por YAML/REST/NPC factory y herencia al spawn; opciones accionables, relación dinámica y feedback. Batch 29 completa el auto-saludo de `talk` (prioridad diálogo autoral → quest → LLM), la continuación por texto libre de una conversación `__llm__` activa/reciente y `kind=quest:<id>` validado. La plantilla `npc.dialogue` v1.0.3 contiene quests disponibles; el modelo no puede crear quests. El cliente expone input libre mediante `DialogMessage.FreeText`.

**FASE 6.5 modo B (B30):** implementado como opt-in (`DMDecision` + ambush/environmental/narrative, kill switch `LLM_DM_DECISION_ENABLED` apagado por defecto). **Pendiente:** para FASE 6.3, el anillo de memoria corta, guardrails completos de combate y `FallbackDialogID` como fallback de error. El `talk` autoral y sus quests mantienen prioridad sobre el saludo LLM.

El texto histórico de los Batches 25–27 que aparece más abajo describe el estado de cada batch en su fecha; el estado vigente tras el Batch 29 es el resumido aquí y en la subsección de deuda anterior.

### NPC runtime dialogue (`ask` y `talk`, FASE 6.3 cortes 1+2+3 + mecanismos B/C — Batches 25–29)

Pregunta libre a un NPC que responde el LLM con la personalidad de ese NPC y opciones accionables; también permite un saludo desde `talk` cuando no hay diálogo autoral ni quests disponibles y continuación por texto libre en una conversación LLM reciente. **Prioridad de `talk`: diálogo autoral → quests → LLM.** La generación no puede crear quests: las opciones `quest:<id>` se limitan a acciones actualmente disponibles. Verificado contra el código 2026-11-01; sin commitear.

- **Comando:** `ask <npc-name> [question]` (`pkg/mudserver/game/commands/ask.go`, registro en
  `commandprocessor.go` **sin alias**; la pregunta es opcional — sin ella es un saludo). Resuelve el
  prefijo de nombre más largo del NPC en la sala (`resolveAskTarget`, tolera `ask to …`).
- **Degradación (siempre responde alguien):**
  - NPC **sin** `llm.enabled` (`npc.LLMEnabled() == false`, incluye NPCs legados con `LLM == nil`) →
    aviso *"<name> has no free-form voice; you speak with them the usual way."* y pasa al diálogo
    **autoral** vía `talkToNPC` (función extraída de `commands/talk.go` y compartida con `talk`).
  - Sin proveedor LLM instalado / IDs faltantes (`NPCDialogueDisabled`) → aviso
    *"<name> cannot answer freely right now; you speak with them the usual way."* + `talk`.
  - Cooldown activo (`NPCDialogueRateLimited`) → *"<name> is still thinking about your last
    question. Try again shortly."*; semáforo lleno (`NPCDialogueBusy`) →
    *"<name> is busy with another question. Try again shortly."* (sin degradación).
- **Orquestación (`Game.RequestNPCDialogue`, `pkg/mudserver/game/npc_dialogue.go`):** cooldown por
  **personaje+NPC** (`npc.EffectiveCooldownSeconds()`, default **3 s**), semáforo de **2**
  concurrencias y timeout de **20 s**; el trabajo LLM corre **fuera del bucle de juego** y el
  resultado vuelve con `game.Call` (sin bucle vivo — tests/arranque — se ejecuta inline).
- **Proveedor (`installNPCDialogueProvider`, `pkg/server/llm.go`):** no-op sin `llmgen.Pipeline`
  activa. Genera con `TaskNPCDialogue` (`RequireLLM: true`, timeout 20 s), plantilla
  `pkg/service/llmprompt/prompts/npc.dialogue.yaml` (v1.0.3) y payload JSON
  `{reply, options[], end, relationDelta, memoryNotes[]}` (`relationDelta` declarado en el
  template y acotado por el juego a ±0.05); `Temperature` de la petición **sobreescribe** la 0.8 del
  prompt cuando `LLM.Temperature > 0`. `npcLLMConfig` resuelve la config viva (instancia primero,
  luego su plantilla) y `formatNPCPersonalityContext` renderiza los bloques del corte 2 en las
  variables del prompt `personality`, `goal`/`goals`, `secrets` y `relations`; el system prompt del
  template usa el tono de los Big5/goals y **ordena no revelar los secretos literalmente** (solo
  indirectas, y solo con alta confianza). Tras validar (reply no vacío ≤ 400 runes, ≤ 4 opciones con
  `kind ∈ {flavor, end}`, ≤ 8 `memoryNotes` de ≤ 200 runes; en el corte 2 también anti-fuga
  literal de secretos) aplica el filtro de contenido `OutputAllowed` y registra el turno de memoria.
  `AvailableQuestPromptContext` proporciona la lista contextual de quests disponibles y la allowlist
  de IDs al prompt/validador; `kind=quest:<id>` se acepta solo para esos IDs.
- **Entrega (`deliverNPCDialogue`, cortes 1–3 + Batch 29):** reply sanitizado (≤ 400 runes) y opciones
  normalizadas (`normalizeNPCDialogueOptions`: hasta 4, `kind ∈ {flavor, end, quest:<id>}`, label ≤ 120 runes)
  y enriquecidas con `mapNPCDialogueOptions`/`matchAuthoredDialogLabel`, que compara cada label con
  el nombre/texto/textos alternativos/opciones de los nodos del diálogo autoral del NPC (instancia →
  plantilla → `llm.FallbackDialogID`) y guarda `NodeID` + índice original cuando coincide. Si hay
  Conversation Service, `sendNPCDialogueDialog` abre/actualiza una conversación con el **`DialogID`
  centinela `"__llm__"`** (`def.NPCDialogueDialogID`), guarda las opciones en `context`
  (`LLM_OPTIONS`, sin migración), engagea la sesión NPC (`EngageNPC(…"dialog")`) y manda un
  `messages.NewDialogMessage` con opciones numeradas (`Index = i+1`, `Username = <nombre del NPC>`);
  sin servicio de conversación cae al texto plano `- label` del corte 1. En error/timeout/salida
  filtrada manda la línea neutra *"<NPC> te mira en silencio."* (`neutralNPCDialogueReply`).
- **Selección de opción (cortes 3 + Batch 29):** `dialog_select.go` ignora conversaciones con `DialogID` vacío
  (una sesión NPC abierta no reactiva opciones ya cerradas) y enruta `DialogID == "__llm__"` a
  `handleLLMDialogSelection` (`pkg/mudserver/game/commands/llm_dialog.go`, nuevo): `end` cierra
  conversación y sesión NPC (`NewDialogEndMessage` + `closeLLMDialogConversation`); una opción con
  `NodeID` continúa el árbol autoral vía `talkToNPCAt` (`commands/talk.go`), que voltea la misma
  conversación de `__llm__` al `DialogID` autoral con `CurrentNodeID = startNode`; cualquier otra es
  un seguimiento `flavor` lanzado con `buildNPCDialogueRequest(..., continuation = true)`
  (`npcDialogueAllow` permite a un `Continuation` saltar el rechazo por cooldown refrescando la
  ventana; el semáforo de 2 sigue aplicando). Índice fuera de rango → *"Invalid option. Please
  choose 1-N"* sin cerrar las opciones. `quest:<id>` se procesa antes de `NodeID`, se vuelve a
  resolver contra las opciones disponibles y delega en `handleQuestDialogOption`; si ya no está
  disponible, informa al jugador y no muta el estado de quest. El cliente envía `String(option.index)`.
- **Relaciones dinámicas (corte 3):** el payload puede traer `relationDelta`, acotado por
  `npc.ClampRelationDelta` (±0.05 por intercambio) y persistido con
  `NPCMemoriesService.AdjustRelation` → `npc.ApplyRelationDelta` en `NPCMemory.Relations`
  (`map[character]float64`, acumulado clampado a [-1,1], dentro del JSON de `npc_memories` —
  **sin migración**). Si el modelo no sugiere delta el default es **+0.02** (0 cuando la opción es
  `end`). `mergeDynamicRelations` lo inyecta al prompt como `player(dinámica,0.04)` sustituyendo la
  entrada estática `player`.
- **Feedback en el cliente (corte 3, frontend):** `public/mud-client/src/game/relationFeedback.js`
  extrae la pista de relación del payload (tolerante a `relation`, `relationDelta`, `relationLabel`,
  before/after), `Client.js` añade la línea entre corchetes y pasa el quinto argumento a
  `mux.setDialog`, `MUDXPlusStore.dialogRelation` la conserva y `DialogOverlay.svelte` dibuja una
  fila sutil ▲/▼/• (wired desde `RoomWidget.svelte` y `MobileRoomView.svelte`). **Solo se pinta si
  el mensaje trae información de relación**; es puramente cosmético.
- **Rate limit del pipeline:** además del cooldown del juego, cada llamada pasa por el `Limiter` de
  `llmgen` con la clave `npc:<npcID>:<characterID>` (`LLM_RATE_PER_HOUR`, **10/h** por defecto);
  agotada la cuota, `RequireLLM` devuelve `ErrRateLimited` y el jugador recibe **la misma línea
  neutra** (no un aviso de espera). Las respuestas en caché (30 min) no consumen cuota. El pipeline
  tiene además su propio semáforo (2 en paralelo, espera máx. 2 s).
- **Invariante de estado:** `ask` y la generación de respuestas/opciones no mutan combate,
  inventario, sala ni quests; las escrituras propias de diálogo LLM son las filas de `npc_memories`.
  Al elegir expresamente un `kind=quest:<id>` validado sí se ejecuta la acción de quest existente.

#### Config por NPC (`npcs.data.llm`, campo JSON nuevo sin migración)

| Campo | Default | Uso real hoy |
|---|---|---|
| `Enabled` | `false` | Único gate: sin él `ask` degrada a `talk` |
| `Persona` | — | 1–3 frases en primera persona; va al prompt (`vars.persona`) |
| `Voice` | — | Registro ("brusco", "anciano"); va al prompt (`vars.voice`) |
| `Temperature` | 0 → 0.8 (prompt) | Override por petición en `pipeline.Generate` |
| `MemoryTurns` | 0 → 8 (`EffectiveMemoryTurns`) | Ventana de **facts** inyectados al prompt (últimos N); el umbral de resumen lo fija `NPCMemoryConfig.SummaryAtTurns` (8) del servicio |
| `FallbackDialogID` | — | **Batch 27:** leído como **fuente secundaria del diálogo autoral** al mapear los labels de las `options` (`npcDialogueAuthoredDialogID`); sigue **sin ser** el fallback de error — ese papel lo tiene la línea neutra |
| `CooldownSeconds` | 0 → 3 (`EffectiveCooldownSeconds`) | Ventana del cooldown por (personaje, NPC) |
| `Personality` (`LLMPersonality`) | — | **Batch 26.** Big5 — `openness`, `conscientiousness`, `extraversion`, `agreeableness`, `neuroticism`, cada uno en `[0,1]` — al prompt (`vars.personality`, p.ej. `apertura:0.40, responsabilidad:0.90, …`); helpers `EffectivePersonality()`/`HasPersonality()` |
| `Goals` (`[]string`) | — | **Batch 26.** Objetivos del NPC al prompt (`vars.goal`/`goals`) |
| `Secrets` (`[]string`) | — | **Batch 26.** Secretos al prompt (`vars.secrets`) con la instrucción de no revelarlos **literalmente**; un reply que los reproduce palabra por palabra se rechaza (`secret_leak`) y se reintenta con feedback |
| `Relations` (`[]LLMRelation`) | — | **Batch 26.** `{Target, Kind ∈ {ally, rival, family, faction, mentor}, Sentiment ∈ [-1,1]}` — p.ej. `player(mentor,0.70)` — al prompt (`vars.relations`); `Target: "player"` = la relación con quien pregunta |

**Tres vías de autoría (Batch 26):** (1) **YAML importer** — `importer.YAMLNPC.LLM` (tags
camelCase) deep-copiado al entity por `ToEntity()` (`converters.go`); (2) **NPCTemplate REST** —
el campo `llm` va en el detail DTO y en el upsert de `handler/npc_templates.go` (`Clone()` en
ambas direcciones; las lecturas `GET` son solo autenticadas, y `llm.secrets` se **redacta** salvo
para creators/admins — `canViewNPCTemplateSecrets`, fail-closed, observación QA B26 cerrada);
(3) **NPC factory UI** del play client (panel "Dialogue / LLM", tecla
N, `NpcFactory.svelte` + `normalizeLLM` en `npcFactory.js`). Los NPCs creados por spawner
**heredan la config de la plantilla** vía `template.LLM.Clone()` en `SpawnFromTemplate`
(`pkg/service/npcs.go`), de modo que una instancia no puede mutar los goals/secretos/relaciones de
la plantilla. El contenido incluido trae un ejemplo: **Master Aldric** (`TUT_MENTOR`,
`import/adventure/data/npcs/tut_mentor.yaml`) con `llm.enabled: true`, Big5, 2 goals, 1 secret y
relación `player/mentor` 0.7 (`import/adventure` está en `.gitignore`, así que el ejemplo vive en
el árbol de trabajo, no en git).

#### Memoria persistente (`npc_memories`)

- **Tabla** `npc_memories` `(id, data JSON)` (migración `migratePhase6NPCMemories()` en
  `pkg/db/sqlite/sqlite.go`); clave determinista `<npcTemplateID>::<characterID>` (`MemoryID`,
  `pkg/repository/npc_memory.go`) — nunca se clave por ID de instancia.
- **Entidad** `repository.NPCMemory{Summary, Facts[], Turns, LastUserLine, SummarySource, Relations map[string]float64, Updated}` — `Relations` (Batch 27, dentro del blob JSON **sin migración**) guarda la relación acumulada por NPC↔personaje (clave en minúsculas, p.ej. `master aldric`; por defecto `player`), clamp [-1,1] con `npc.ApplyRelationDelta`/`AdjustRelation`;
  repo `pkg/repository/npcmemories_sqlite.go` + servicio `NPCMemoriesService`
  (`pkg/service/npcmemories.go`, expuesto en el facade). `SummarySource` (`""` legado |
  `"deterministic"` | `"llm"`, dentro del blob JSON — **sin migración**) marca la procedencia del
  resumen: escrito por el summariser LLM (`"llm"`) o editado externamente, `RecordTurn` lo
  **preserva** y no lo reconstruye de forma determinista; los turnos posteriores solo reescriben
  `Summary` cuando el guardado es `"deterministic"`/legado.
- **Por turno** (`recordNPCMemoryTurn` → `RecordTurn`): suma turno (cap 50), fusiona/dedupe hechos
  (cap 24, máx. 200 runes c/u), guarda la última pregunta (200 runes) y, al cruzar el umbral de
  8 turnos, reconstruye `Summary` de forma **determinista** (concatena hechos + última pregunta; cap
  400 runes) salvo que el resumen deba preservarse (ver `SummarySource`).
- **Resumen LLM asíncrono (corte 2):** `triggerNPCMemorySummaryIfNeeded` (`pkg/server/llm.go`)
  dispara tras cada turno registrado, en una goroutine fire-and-forget (ctx 12 s), cuando
  `Turns % SummaryAtTurns() == 0`: llama a `llmgen` con `TaskNPCMemorySummary` (plantilla
  `pkg/service/llmprompt/prompts/npc.memory_summary.yaml`, temperatura 0.2, `RequireLLM: false`,
  sin caché, limit key `npcmem:<npcID>:<characterID>`) alimentado con nombre/persona del NPC,
  facts, última pregunta y el resumen existente; el modelo devuelve `{"summary": "…"}` cap 400
  runes. Output vacío/fallido → el resumen determinista sigue; éxito → recarga la fila (evita
  pisar turnos concurrentes) y guarda con `SummarySource = "llm"`. La caché del task
  `npc.memory_summary` tiene TTL 0 (no cachea).
- **Siguiente pregunta** (`loadNPCMemoryFull` → `Get` → `formatNPCMemoryLimited`): inyecta
  `summary` + los últimos `EffectiveMemoryTurns()` facts + `Última pregunta: …` en la variable
  `memory` del prompt (más nombre/persona del NPC, sala, nombre/nivel del jugador y la pregunta,
  declarada `untrusted: true` y cap 200 runes).
- **Tests:** `pkg/mudserver/game/commands/ask_test.go` (7 tests: degradación sin LLM/proveedor,
  llamada al proveedor + entrega, pregunta vacía, rate limit por personaje+NPC, NPC inexistente,
  error del proveedor sigue respondiendo), `pkg/server/llm_npc_test.go` (8 tests: registro de
  memoria, fallback neutro, rechazo de reply vacío, memoria entre turnos, secretos pasados al
  prompt, rechazo de fuga literal de secretos, y el guard sobre `options`/`memoryNotes` — validator
  y payload), `pkg/service/npcmemories_test.go` (3 tests: reconstrucción determinista, preservación
  de resumen LLM, preservación de resumen externo sin marcador), `pkg/importer/llm_yaml_test.go`
  (YAML camelCase → deep copy en el entity) y `pkg/server/handler/npc_templates_test.go`
  (`TestNPCTemplateGetRedactsSecretsForNonCreators`); tests del corte 3:
  `pkg/mudserver/game/commands/llm_dialog_test.go` (5: `flavor` → seguimiento con `Continuation`,
  `end` cierra, selección desconocida mantiene las opciones, opción mapeada continúa nodo autoral,
  conversación cerrada no la reactiva una sesión NPC abierta), `TestMatchAuthoredDialogLabelFindsNodeAndOption`
  (`npc_dialogue_test.go`), `TestAdjustRelationClampsAndPersists` (`service/npcmemories_test.go`),
  `TestInstallNPCDialogueProviderAdjustsRelation` (`server/llm_npc_test.go`: delta 0.04 persistido,
  prompt con `player(dinámica,0.04)`, clamp de −0.20 a −0.01) y
  `public/mud-client/src/game/relationFeedback_test.mjs` (frontend). QA: Batches 25/26 verificados
  (el 26 **con observaciones menores, ya cerradas** — `llm.secrets` se redacta en GET de plantillas
  para no-creators y el anti-fuga cubre `reply`, `options[].label` y `memoryNotes`) y **Batch 27
  APROBADO CON OBSERVACIONES** (2026-10-30): única observación no bloqueante ajena al batch, el test
  de combate intermitente `TestWardGritAndRetaliate` (`pkg/mudserver/game/combat/ward_test.go:111`,
  "tiny hit 2" — los tests de guardia `TestWardGuard*` del mismo fichero pasan), 0 `DATA RACE`;
  sin sección propia en `docs/qa/REGRESSION_REPORT.md`.

#### Estado y deuda de FASE 6.3 tras los cortes 1–3 y Batch 29

- **Implementado en Batch 29:** mecanismo B de saludo LLM en `talk` únicamente cuando no hay
  diálogo autoral ni quests disponibles; mecanismo C de texto libre en conversación `__llm__`;
  opciones `kind=quest:<id>` con allowlist contextual; contrato de `DialogOption.Kind` y
  `DialogMessage.FreeText` e input libre en `DialogOverlay`.
- Prioridad de `talk`: **diálogo autoral → quests disponibles → LLM**. Si el proveedor está
  deshabilitado, conserva el mensaje actual de NPC sin diálogo. La intercepción solo aplica a un NPC
  LLM habilitado, vivo y en la misma sala, con sesión dialog abierta o interacción de menos de 60 s;
  la conversación se cierra al programar la continuación, no ante `RateLimited`, `Busy` o `Disabled`.
- El proveedor calcula `AvailableQuestPromptContext` para el prompt `npc.dialogue` v1.0.3 y valida
  los IDs contra esa allowlist. El modelo solo elige acciones de quest existentes/disponibles, no
  crea quests. En selección se vuelve a resolver la disponibilidad y se reutiliza
  `handleQuestDialogOption`; quest inexistente/no disponible no muta estado.
- Pendientes: `llm_npc_session.go` (anillo de ventana corta) y guardrails completos de combate;
  no se debe inferir de los checks de texto libre que toda interacción en combate esté bloqueada.
- Caché del pipeline para `npc.dialogue` (30 min, `llmgen/config.go`): puede repetir la misma
  respuesta ante un prompt idéntico.
- `FallbackDialogID`: solo como **fuente secundaria del diálogo autoral** al mapear labels de
  opciones (corte 3); el fallback de error sigue siendo la línea neutra.
- Anti-fuga de secretos: el guard cubre `reply`, `options[].label` y `memoryNotes` (la
  observación QA B26 que lo limitaba a `reply` quedó **cerrada**); las paráfrasis/insinuaciones no
  se rechazan (solo la divulgación literal).
- El error/timeout/filtro sigue usando la línea neutra; `FallbackDialogID` no está implementado como
  fallback de error. `talk` mantiene el flujo principal autoral y de quests; `ask` es complementario.
- **Contrato de opción de quest:** el selector del diálogo LLM conserva `Kind=quest:<id>`; al
  seleccionarlo, el servidor revalida la disponibilidad actual y delega en el mismo manejador que
  las opciones de `talk`. No se crean definiciones de quest a partir de texto del modelo.

### Dungeon Master AI (FASE 6.5, modo A — Batch 24)

Narrador reactivo en **prosa** que comenta eventos del juego y **nunca muta estado** (diseño `docs/design/PHASE6_LLM.md` §6.5, nota "Implementado (Batch 24, modo A)"). Verificado contra el código 2026-10-22; sin commitear.

- **Bus de eventos tipado:** `def.DMEvent{Kind, RoomID, CharacterID, UserID, Facts}` (`pkg/mudserver/game/def/gamectrl.go`), publicado vía `GameCtrl.NotifyDM`. `Kind ∈ {combatStart, combatEnd, questCompleted, levelUp, roomEnter}`.
- **Núcleo:** `pkg/mudserver/game/dm_narrator.go` — `DMNarrationRequest`, `DMNarratorFunc`, `SetDMNarrator`, `DMNarratorEnabled`, `TriggerDMNarration`, `NotifyDM` (adapta `DMEvent` → request; parsea `Facts`), `dmAllowRoom`/`dmAllowUser`/`dmAllowGlobal`, `dmPartySize` (fiesta + followers reales, dedup), `dmRoomName`, `cleanDMText` (trim, colapsa saltos, ≤240 runes). Tests: `dm_narrator_test.go`.
- **Hooks (los 5 eventos):**
  - `combatStart` → `game_aggro.go:571` (tras `BeginEngagement`; NPCName, Round, PartySize) — audiencia Room.
  - `combatEnd` → `game_combat.go:1187` (`cleanupCombatInstance`; Outcome, Round, PartySize, nombre del primer enemigo) — Room.
  - `questCompleted` → `commands/dm_notify.go` (desde `questcommand.go:264` y `dialog_select.go:417`; `questName`) — **User** (privado).
  - `levelUp` → `commands/dm_notify.go` (desde `level_up_notify.go:19`; `level`) — **User**.
  - `roomEnter` → `commands/room_takeexit.go:113`, **solo al cambiar de `Area`** (`roomName`, `area`) — Room.
- **Flujo (nunca en el tick, nunca muta):** hook → `NotifyDM` → throttles (cooldown por sala/usuario 45 s + presupuesto global 10/min) → semáforo de 2 → goroutine con ctx 8 s, 1 intento → `llmgen.Pipeline` (`TaskDMNarration`, `RequireLLM: true`) → filtro `OutputAllowed` + `cleanDMText` → resultado vuelve por `game.Call` → mensaje `Username: "DM"` a la sala o al usuario. Audiencia global rechazada.
- **Fallback = silencio:** sin proveedor, con error/timeout, o si el filtro descarta la línea → no se emite nada (`llmgen/fallback.go` devuelve vacío para `TaskDMNarration`; `fallback: ""` en la plantilla; `RequireLLM` impide prosa determinista).
- **Plantilla:** `pkg/service/llmprompt/prompts/dm.narration.yaml` (una frase corta en español, sin mecánicas ni ids; temperatura 0.9, maxTokens 120, retry 0).
- **Instalación:** `pkg/server/llm.go` (`installDMNarrator` e `installDMDecision`, ambos no-op sin proveedor o con su kill switch off) → `server.go` al cablear el juego vivo.
- **Config (`LLM_DM_*`):** `LLM_DM_ENABLED` (kill switch, default 1), `LLM_DM_COOLDOWN_SECONDS` (default 45), `LLM_DM_TIMEOUT_SECONDS` (default 8), `LLM_DM_MAX_PER_MIN` (default 10) — en `.env.example` y `docker-compose.yml`; leídas en `game.go` `New`.
- **Modo B (B30, opt-in):** `game/dm_decision.go` (`DMDecision` con whitelist `none|ambush|environmental|narrative`, `templateId` validado contra el catálogo vivo, `count` 1..3, prosa ≤240 runes), cooldown por sala (default 60 s), timeout (8 s), presupuesto global (10/min) y métricas atómicas. `NotifyDM` es el único punto de entrada para `roomEnter`/`combatStart` (sin doble narración/decisión). Config: `LLM_DM_DECISION_ENABLED` (default **0**), `LLM_DM_DECISION_COOLDOWN_SECONDS`, `LLM_DM_DECISION_TIMEOUT_SECONDS`, `LLM_DM_DECISION_MAX_PER_MIN`.
- **Bugs corregidos en el corte:** `Round` (antes 0 → ahora `instance.Round`), `PartySize` (antes solo `len(Players)` → ahora `dmPartySize` con fiesta + followers reales) y `RoomName` (lookup `dmRoomName` + repetición en el proveedor).
- **Pendientes:** tono por zona y exposición de métricas fuera del proceso. QA: **§B24 verificado (APROBADO, 2026-10-22)**; modo B cubierto por tests unitarios en B30.

Lo que ya existía y se reutiliza tal cual (la regla del diseño es "construir sobre `llm.Client`, nunca reemplazarlo"):

- Contrato `llm.Client` (`Complete(ctx, systemPrompt, userPrompt, temperature)`) + factory `NewFromEnv()` en `pkg/service/llm`, providers `pkg/service/ollama` (por defecto) y `pkg/service/groq`.

Paquetes presentes en el árbol de trabajo (sin commitear):

| Package / artifact | Role | State (2026-10-19) |
|--------------------|------|---------------------|
| `pkg/service/llmopts` (new) | Provider-agnostic `Options{Temperature, MaxTokens}`; `llm.Options` is an alias of it. `CompleteWith(ctx, system, user, Options)` is implemented by both providers, so `max_tokens` is no longer fixed at 300 (0 still means the provider default of 300) — this is the design's additive §6.2 extension | present; imported by `llm`, `ollama`, `groq` |
| `pkg/service/llmprompt` (new) | Versioned YAML prompt templates (`prompts/<task>.yaml`, `//go:embed`) + `text/template` rendering (`prompt.go`, `registry.go`, `prompts/`: `character.name`, `character.description`, `npc.dialogue` (v1.0.3, incl. available quests, personality/relations and `relationDelta`), `npc.memory_summary` (Batch 26), `quest.generate`, `room.description`, `dm.narration`) | present; consumed by `llmgen` and by `pkg/server/llm.go` / `handler/generate.go` (`LLM_PROMPT_DIR` overrides the embed dir) |
| `pkg/service/llmgen` (new) | Generation pipeline (`pipeline.go` + `pipeline_test.go`) and building blocks (cache, config, deterministic fallback, content filter, limiter, parse, stats, world `catalog`) plus `questgen.go` / `questgen_context.go` | present; consumed by `handler/generate.go`, `handler/llm.go` y el proveedor del comando `offer` instalado en `pkg/server/llm.go` |
| `npc_memories` table + `pkg/repository/npc_memory.go`, `npcmemories_sqlite.go` (+ tests), `Factory.NPCMemories()`, `pkg/service/npcmemories.go` (`NPCMemoriesService` en el facade) | Long-term NPC memory store for §6.3, keyed `<npcTemplateID>::<characterID>`; migration `migratePhase6NPCMemories()` in `pkg/db/sqlite/sqlite.go`; since Batch 26 the rows carry `SummarySource` (`deterministic` \| `llm`, JSON blob — no migration) and since Batch 27 a `Relations` map (cumulative NPC↔player relation, [-1,1]) | present; **consumed by the game since Batch 25** (`loadNPCMemoryFull` / `recordNPCMemoryTurn` in `pkg/server/llm.go`, backing the `ask` command), **with async LLM summaries since Batch 26** (`triggerNPCMemorySummaryIfNeeded`, `TaskNPCMemorySummary`) |

Pendientes del diseño:

| Piece | Role |
|-------|------|
| Anillo de ventana corta (`llm_npc_session.go`) | Sigue pendiente. Los mecanismos B/C sí están implementados: `talk` puede saludar con LLM tras diálogo/quests, y el texto libre puede continuar una conversación `__llm__` reciente. |
| `pkg/mudserver/game/dm_decision.go` + `pkg/server/llm.go` (`installDMDecision`) | FASE 6.5 **modo B implementado (B30)**: `DMDecision` (whitelist none/ambush/environmental/narrative + template validado + count clamp 1..3), cooldown/timeout/presupuesto y métricas; opt-in con `LLM_DM_DECISION_ENABLED` |

Cuts: **A** MVP (templates + pipeline + character generation) — **landed**; **B** NPC runtime LLM — **landed as cortes 1–3 plus mechanisms B/C in Batches 25–29** (`ask` + `npc_memories` + structured personality + async summaries + YAML/REST/UI config + actionable options + dynamic relations; `talk` priority authored dialog → quests → LLM greeting; raw-text continuation of recent `__llm__` conversations; validated `kind=quest:<id>` limited to available quest actions); **C** maturity (quest generator, DM AI, admin tools) — **landed in part** (`questgen`/`questgen_context`, DM mode A, `handler/llm.go`; DM mode B remains pending). NPC prompt validation/rate limits/content filter and literal-secret rejection are basic guardrails; complete combat guardrails and the short-term memory ring remain pending. `FallbackDialogID` only maps to authored dialog and is not an error fallback.

Note: `docs/qa/REGRESSION_REPORT.md` corre hasta §B24 (2026-10-22). Batches 25–27 fueron verificados por QA con tests dedicados sin sección propia en el reporte; las observaciones B26 están cerradas y el B27 tuvo una observación de test intermitente de combate ajena al batch. Batch 29 está verificado por sus tests de prioridad/autosaludo, selección de quest, validación de IDs, texto libre y overlay descritos en el bloque NPC runtime.

---

## Estado actual — Batches 14–29 (corte 2026-11-01)

Cada sistema vivo está documentado en su sección de arriba (Dungeon Builder,
NPC Factory, Contextual Quest Offers, Fase 6, Client UX). Aquí solo queda lo
que fue **decisión de batch y sigue vigente**. Todo verificado contra el árbol
de trabajo sin commitear.

### Cliente — atajos y paneles (Batch 15)

- `Game.svelte` (`onShortcutKey`) cablea `C` (clanes), `M` (DMs), `K` (spellbook), `L` (editor de clases), `I` (inventario), `N` (NPC factory), `D` (dungeon builder), `A` (audit), `1`–`9` (hotbar), `Tab`/`Shift+Tab` (ciclar objetivo, solo con combate activo), `?` y `Escape`; nada se dispara con el foco en un campo de texto.
- `ShortcutSheet.svelte` (hoja `?`) lista hoy `1`–`9`, Tab, C, M, K, L, I, N, D, A, Escape y `?`.

### Base de datos — pool e índices (Batch 16, FASE 7.1 corte implementado)

- `pkg/db/sqlite/sqlite.go`: pragmas (`_busy_timeout=5000`, `_journal_mode=WAL`, `_synchronous=NORMAL`) embebidos en el DSN para que **todas** las conexiones del pool los hereden; pool por defecto `SetMaxOpenConns(4)` / `SetMaxIdleConns(4)` (`DB_MAX_OPEN_CONNS`) y `SetConnMaxLifetime(5m)`. Antes: una sola conexión.
- `migratePhase16Indexes()`: elimina el índice muerto `idx_characters_user` (apuntaba a `$.belongsUserID` cuando el campo real es `$.belongsUser`) y lo recrea bien; añade `idx_characters_name`, `idx_rooms_name`, `idx_rooms_area`, `idx_npc_spawners_room`, `idx_npc_spawners_template`, `idx_users_online`, `idx_users_refid`, `idx_users_username`. Idempotente y aditivo.

### Seguridad (Batch 16, FASE 7.2 + hallazgo B19.4)

- Auditoría estática → `docs/qa/REGRESSION_REPORT.md` §B16: **sin hallazgos de severidad alta**; condición del PASS: atender B16-F1/F2 (disponibilidad), cabeceras globales (F3), quitar orígenes loopback en producción (F4).
- Endurecimiento presente en el árbol: `SecurityHeadersMiddleware` (`X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: no-referrer`; CSP solo en `/activate`), `http.Server` con timeouts explícitos (`newHTTPServer`, env `HTTP_READ_HEADER_TIMEOUT`/`HTTP_READ_TIMEOUT`/`HTTP_WRITE_TIMEOUT`/`HTTP_IDLE_TIMEOUT`; por defecto 5s/15s/30s/60s) y fetch JWKS acotado (`JWKS_TIMEOUT_SECONDS` 10s + cuerpo limitado a 1 MiB, `pkg/server/auth.go`).
- **Riesgo medio (§B19.4):** `authLimiter` (12 registros/min/IP) usa `c.ClientIP()` y `TrustedProxies()` por defecto confía en `X-Forwarded-For` de loopback → cualquier proceso local puede rotar XFF y obtener presupuesto ilimitado de auth. En despliegue real: exponer el puerto solo a procesos de confianza o un proxy con `TRUSTED_PROXIES` explícito (`pkg/server/trusted_proxy_test.go` cubre el lado opuesto).

### QA — regresión, balance, carga (Batches 14–21; P0 cerrado en B21; race cerrada en B23)

- Batches 14–19: build Go EXIT 0, suite Go 0 FAIL, `npm run build` EXIT 0 en todos los cortes. Los 2 fallos de tests frontend de entorno (Node 18 WebSocket, canvas) son preexistentes.
- **B15** (readiness de carga) y **B16** (seguridad): PASS con condiciones. **B17** balance: `docs/qa/BALANCE_ANALYSIS.md` (solo lectura). **B18**: harness `tools/loadtest` + combat sims.
- **B19 (carga real 100–500):** 100 conexiones sanas (heartbeat de 60 s verificado con corrida de 75 s, 99.96 % de respuestas); **≥200 conexiones wedgeaban el servidor de forma permanente** hasta el reinicio. Herramienta: modo `-mint` de `tools/loadtest` (`mint.go` acuña N session JWTs con pacing o IPs sintéticas `10.x.y.z` vía `X-Forwarded-For`; tests `mint_test.go`).
- **P0 (cerrado — fix en Batch 20, Backend; verificado por QA en B21):** deadlock autoinfligido del bucle de juego — `handleUserJoined` enviaba bloqueantemente a `onMessageReceived` (buffer 20, `game.go`) desde la única goroutine consumidora, y la cascada en `mudserver.go` congelaba todos los read loops WS. Causa raíz y evidencia en `docs/qa/REGRESSION_REPORT.md` §B19; recomendaciones de §B19.7 **ya implementadas** en B20 (ver el bloque "Batch 20 — Backend" más abajo). Evidencia de cierre (§B21): 200/300/500 × 2 corridas × 80 s, `recv == sent`, **0 `msgErrors`** en 6/6, sonda post-carga sana. Mitigados en B20: §B19.6-1 (escrituras SQLite por mensaje) y §B19.6-2 (transporte WS sin write deadline); **sigue abierto** §B19.4 (bypass de rate-limits por `X-Forwarded-For` de loopback).
- **Regresión detectada en B21 y corregida en B22:** leak de ~1 goroutine `writeLoop` por desconexión (§B21.5) — cerrado con transporte idempotente en `mudserver.go` + `transport_leak_test.go`.
- **Race preexistente (cerrada en Batch 23):** el `*entities.User` compartido por el handler WS, el bucle de juego y `game_handlers` se mutaba sin sincronía (`IsOnline`/`LastSeen`/`LastCharacter`) mientras `UsersService().Update` lo serializaba a JSON → data race + snapshot mezclado persistido (reportada en §B22.4 como P2). **Corregida en Batch 23 (Backend)** con lock de presencia por usuario (`pkg/mudserver/game/presence.go`: `ApplyPresence`/`PresenceOf` + refcount), copia desligada `entities.User.Snapshot()` y test de regresión `pkg/mudserver/presence_race_test.go` → `0 DATA RACE` con `-race`. **Verificación QA §B23: `go test -race ./...` completo con 0 `DATA RACE`; el control negativo reproduce 6–19 warnings sin el lock, confirmando que el test detecta la race.**

### Batch 20 — Backend (fix P0 de carga); Batch 21 — QA+Docs; Batch 22 — infra+Docs (19-oct-2026); Batch 23 — fix de la race de presencia (21-oct-2026)

**Batch 20 — Backend (fix del P0 de carga, verificado contra el código):**

- `pkg/mudserver/game/game_handlers.go`: `lc` y `selectcharacter` se procesan **inline** con `game.processIncoming(...)` en el bucle de juego (único consumidor de `onMessageReceived`); la restricción queda documentada en `pkg/mudserver/game/game.go` (`processIncoming`).
- `pkg/mudserver/joinqueue.go` (nuevo) + `pkg/mudserver/mudserver.go`: cola FIFO no bloqueante de joins (`enqueueUserJoined` desde `attachSession`, nunca bloquea el read loop); una sola goroutine `forwardJoins` hace el hand-off bloqueante a `Game.OnUserJoined`; aviso de saturación a 64 y log de recuperación.
- `pkg/mudserver/transport.go`: cola de salida por conexión (128) + `writeLoop` propio; `Send` encola y retorna; `SetWriteDeadline` de 10 s por frame → consumidor lento se desconecta en vez de bloquear broadcasts/pings.
- `pkg/mudserver/game/sessions.go` + `pkg/mudserver/mudserver.go`: `LastSeen`/`IsOnline` se persisten solo en connect/disconnect (sin `UsersService().Update` por mensaje).
- `cmd/tales/pprof.go` (+ `pprof_test.go`): pprof opcional (`ENABLE_PPROF`, `PPROF_ADDR`, por defecto `127.0.0.1:6060`).
- Tests: `game/join_deadlock_test.go`, `mudserver/joinqueue_test.go`, `mudserver/presence_debounce_test.go`, `mudserver/transport_test.go`, `cmd/tales/pprof_test.go`.
- Evidencia: load test 300 workers → 300/300 OK, 0 `msgErrors` (antes wedgeaba a ~200). **Estado: cerrado — re-verificado por QA en B21** (200/300/500 × 2 corridas sin wedge, 0 `msgErrors`).

**Batch 20 — Docs (sincronización con los Batches 14–19, sin cambios en `pkg/**`, `public/**` ni `tools/**`):**

- **`PROJECT.md`:** sección "Estado actual — Batches 17–19" (entonces "Batches 17–23", hoy "Batches 17–27"), estado de fases 5.1/5.3/6/7.1/7.2, roadmap actualizado, catálogo de endpoints con puntero a `docs/API.md` y apartado del P0 con enlace a `docs/qa/REGRESSION_REPORT.md` §B19 y §B19.7.
- **`FEATURES.md`:** secciones nuevas "Dungeon Builder (Phase 5.1)", "Contextual Quest Offers (`offer` command)", "NPC Factory and Level Scaling" e "Inventory weight display (3.5)"; Fase 6 reescrita con lo cableado (B16/B18) y lo pendiente; afirmaciones obsoletas corregidas (hoja `?` con `I`/`D`, load test ejecutado, Dungeon Builder existente); esta sección consolidada reemplaza los apartados por batch.
- **`ARCHITECTURE.md`:** ruta de datos de mazmorras, `npc_templates`, comando `offer`, fábrica de NPCs/escalado en cliente, facade/repos/tablas actualizados, entidades `Dungeon`/`NPCTemplate` y nota del P0 en el modelo de concurrencia.
- **`docs/index.md`:** `qa/BALANCE_ANALYSIS.md` al índice, verificación B14–B19, fila 7.1 con el bloqueador y fila 7.3 (balance). **`README.md`:** enlaces de docs, árbol de carpetas y roadmap corregidos; bloque "Fork additions".
- **P0 documentado en ese corte como "abierto, sin fix en `pkg/**`"** en `PROJECT.md`, `ARCHITECTURE.md`, `docs/index.md` y `README.md` — **corregido por el Batch 21** (carrera de cronología: el fix del Backend del mismo Batch 20 no estaba reflejado).

**Batch 21 — QA + Docs (cierre del P0):**

- **QA (§B21):** re-verificación de carga 200/300/500 × 2 corridas × 80 s → **P0 cerrado** (`recv == sent`, 0 `msgErrors`, sin wedge, heartbeat OK); detecta el leak de `writeLoop` (P2, §B21.5).
- **Docs:** `PROJECT.md`, `ARCHITECTURE.md`, `FEATURES.md`, `README.md` y `docs/index.md` dejan de decir que el P0 está "abierto/sin fix": estado nuevo **"corregido en Batch 20 (Backend); pendiente re-verificación de carga por QA (B21)"** (posteriormente cerrado por el propio QA en §B21), conservando la traza histórica (qué, causa raíz, fix, archivos, §B19/§B19.7).
- `docs/index.md` fila 7.1 actualizada; se consolida el estado "Batches 17–21" en los docs raíz (sin secciones de batch duplicadas).
- **Nota operativa retirada en B22:** la advertencia de `git add -f` para `cmd/tales/pprof*.go` quedó obsoleta al corregirse `.gitignore` (`tales` → `/tales`).

**Batch 22 — Backend/infra + Docs (19-oct-2026):**

- **Fix de build crítico:** el `Makefile` compilaba `cmd/tales/main.go` como archivo suelto y desde el Batch 20 eso falla con `undefined: startPprof` → rotos `make build`, `make build-backend`, `make run-server`, `make run` y `make dev`. Los targets ahora compilan **por paquete**: `go build -buildvcs=false -o bin/tales ./cmd/tales`, `go build -buildvcs=false -o bin/dialog_sandbox ./cmd/dialog_sandbox`, `go run ./cmd/tales`, `go run ./cmd/dialog_sandbox`.
- **`.gitignore`:** `tales` → `/tales` (solo el binario de la raíz); añadidos `bin/dialog_sandbox` y `/dialog_sandbox`. `cmd/tales/pprof.go` y `pprof_test.go` dejan de estar ignorados (verificado con `git check-ignore`): **ya no hace falta `git add -f`**.
- **Fix P2 (leak de goroutines, §B21.5):** cierre idempotente del transporte en `pkg/mudserver/mudserver.go` — `HandleConnections` añade `defer connection.closeTransport(0, "")` y `handleConnectionClosed` llama a `connection.closeTransport(0, "")` (sync.Once en `wsTransport.Close`), de modo que `writeLoop` sale de su `select` al desconectar. Test: `pkg/mudserver/transport_leak_test.go` (ciclos de conectar/desconectar que exigen volver al baseline de goroutines).
- **Docs (rol Docs):** comandos de build sincronizados (compilación por paquete + `-buildvcs=false` documentado) en `README.md`, `PROJECT.md`, `docs/fork-setup.md` y `docs/development/AGENTS.md`; P0 pasado a **cerrado — verificado por QA en B21** en `PROJECT.md`, `ARCHITECTURE.md`, `FEATURES.md`, `README.md` y `docs/index.md`; nota de `git add -f` retirada; Batch 22 y la race preexistente registrados.
- **Pendiente conocido (resuelto en Batch 23):** la race en `attachSession`/`sessions.go` (`user.LastSeen`/`user.IsOnline`/`user.LastCharacter` sin sincronizar, detectada con `-race`, §B22.4) **quedó cerrada en Batch 23** — ver el bloque siguiente.

**Batch 23 — Backend (fix de la race de presencia) + Docs (21-oct-2026):**

- **Causa:** el mismo `*entities.User` era mutado sin sincronía por el handler WS (`attachSession`/`handleConnectionClosed`), el bucle de juego (`ConnectUserSession`/`DisconnectUserSession`/`SetUserSessionCharacter`) y `game_handlers` (`handleUserJoined`), mientras `UsersService().Update` serializaba ese struct a JSON → data race + snapshot mezclado persistido en SQLite (reportado por QA en §B22.4 como P2, no de B22).
- **Fix (verificado contra el código):** lock de presencia por usuario con refcount — `pkg/mudserver/game/presence.go` (`presenceRegistry`, `ApplyPresence(user, mutate)`, `PresenceOf(user)`); copia desligada `entities.User.Snapshot()` (`pkg/entities/user.go`, copia también el `Entity` embebido) persistida **bajo el lock**, de modo que la fila serializada es siempre un snapshot consistente y el orden de los writes sigue el orden de las transiciones de estado. `def.PresenceState` + `GameCtrl.PresenceOf` (`pkg/mudserver/game/def/gamectrl.go`).
- **Call sites migrados:** `pkg/mudserver/mudserver.go`, `pkg/mudserver/game/sessions.go`, `pkg/mudserver/game/game_handlers.go`, `pkg/mudserver/game/util/game_utils.go`, `pkg/mudserver/game/commands/select_character.go`, `pkg/presentation/doorview/view.go`.
- **Invariante:** nada de mutar los campos de presencia de un `*entities.User` compartido fuera de `ApplyPresence`; toda lectura cross-goroutine pasa por `PresenceOf`.
- **Semántica preservada (B19.6-1):** solo se persiste la transición offline→online / online→offline, no en cada input; el lock no cambia el debounce.
- **Test:** `pkg/mudserver/presence_race_test.go` (`TestPresenceMutationIsRaceFree`: 50 rondas × 5 writers concurrentes sobre el mismo user; sin el lock falla bajo `-race`).
- **Docs (rol Docs):** la race pasa de "pendiente" a **corregida en Batch 23** en `PROJECT.md`, `ARCHITECTURE.md` (Thread Safety: lock de presencia por usuario + invariante + recomendación de dueño único), `FEATURES.md`, `README.md` y `docs/index.md`; Batch 23 registrado con traza histórica.

**Batch 24 — Backend (FASE 6.5, modo A — Dungeon Master AI) + Docs (22-oct-2026):**

- **Qué es:** el narrador DM en prosa queda completo en **modo A** (solo comenta, **nunca muta estado**); se completó la implementación parcial previa y se corrigieron bugs (`Round`, `PartySize`, `RoomName`). Detalle completo en la sección "Dungeon Master AI (FASE 6.5, modo A)" de la Fase 6, más arriba.
- **Archivos clave:** `pkg/mudserver/game/dm_narrator.go` + `dm_decision.go` (+ tests), `pkg/mudserver/game/def/gamectrl.go` (`DMEvent`/`NotifyDM`), hooks en `game_aggro.go`, `game_combat.go`, `commands/dm_notify.go`, `commands/room_takeexit.go`, `commands/level_up_notify.go`, `commands/questcommand.go`, `commands/dialog_select.go`; proveedor en `pkg/server/llm.go` (`installDMNarrator`, `installDMDecision`); filtro/fallback en `pkg/service/llmgen/{filter,fallback}.go`; plantilla `pkg/service/llmprompt/prompts/dm.narration.yaml`; config `LLM_DM_*` en `.env.example` y `docker-compose.yml`.
- **Guardrails vigentes:** cooldown 45 s por sala (y por usuario en eventos privados), timeout 8 s con 1 intento, presupuesto global 10/min, semáforo de 2 concurrencias, kill switch `LLM_DM_ENABLED`, filtro de salida (≤240 runes, sin ids sospechosos), audiencia Room/User con `Username: "DM"` (nunca global) y **fallback = silencio**. Modo B añade cooldown por sala, timeout, presupuesto propio, whitelist de acciones, template validado y count 1..3.
- **Estado:** modo A cerrado en Batch 24; modo B implementado en B30 como opt-in (`LLM_DM_DECISION_ENABLED` apagado por defecto). QA **§B24 verificado (APROBADO)**.
- **Docs (rol Docs):** nota "Implementado (Batch 24, modo A)" en `docs/design/PHASE6_LLM.md` §6.5; `PROJECT.md` (roadmap Fase 6 + bloque "Batches 17–24"), `ARCHITECTURE.md` (subsystem DM: flujo evento → `NotifyDM` → throttles → goroutine → mensaje), `FEATURES.md` (esta sección + Last Updated), `README.md` y `docs/index.md`.

**Batch 25 — Backend (FASE 6.3, corte 1 — NPC runtime LLM, comando `ask`) + Docs (22-oct-2026):**

- **Qué es:** el jugador pregunta libremente a un NPC (`ask <npc> [pregunta]`) y este responde en persona vía el LLM, con memoria persistente por personaje; los NPCs sin `llm.enabled` (o sin proveedor instalado) siguen respondiendo por el diálogo autoral. **Nunca muta estado** (solo mensaje + fila de memoria). Detalle completo en el bloque "NPC runtime dialogue (`ask`, FASE 6.3 cortes 1+2+3)" de la Fase 6, más arriba (el corte 2 amplía este corte; los pendientes de abajo son los del momento del Batch 25).
- **Archivos clave:** `pkg/mudserver/game/commands/ask.go` (+ `ask_test.go`, 7 tests), `pkg/mudserver/game/npc_dialogue.go` (orquestación: cooldown, semáforo, timeout, `game.Call`), `talkToNPC` extraído y compartido en `commands/talk.go`, proveedor `installNPCDialogueProvider` en `pkg/server/llm.go` (+ `llm_npc_test.go`, 4 tests), plantilla `pkg/service/llmprompt/prompts/npc.dialogue.yaml`, `pkg/entities/npcs/llm.go` (`LLMConfig`), `pkg/service/npcmemories.go` + `pkg/repository/npcmemories_sqlite.go` (tabla `npc_memories`) y registro en `commandprocessor.go` (sin alias).
- **Guardrails vigentes:** cooldown por personaje+NPC (`LLM.CooldownSeconds`, default 3 s), semáforo de 2 concurrencias, timeout 20 s, trabajo LLM fuera del bucle de juego, rate-limit del pipeline por par npc+personaje (`LimitKey`, `LLM_RATE_PER_HOUR` 10/h → línea neutra al agotarse), validación de payload (reply ≤ 400 runes, ≤ 4 opciones, ≤ 8 `memoryNotes`), filtro `OutputAllowed`, pregunta del jugador como input no confiable (≤ 200 runes en la plantilla) y fallback neutro ("… te mira en silencio").
- **Estado histórico al cerrar Batch 25:** en ese corte estaban pendientes personalidad, resumen LLM, mecanismos B/C y configuración Creator; esos puntos se implementaron en Batches 26–29. Sigue siendo válida la advertencia de caché de 30 min de `npc.dialogue`. QA del batch: tests dedicados, sin sección propia en `docs/qa/REGRESSION_REPORT.md`.
- **Docs (rol Docs):** FASE 6.3 corte 1 pasa a **implementado (corte runtime)** en `PROJECT.md`, `ARCHITECTURE.md` (subsistema NPC LLM), esta sección (bloque Fase 6 + estado de batches), `README.md`, `docs/index.md`, `docs/user/commands.md` (`ask` documentado y fuera de la lista de inexistentes) y nota "Implementado (Batch 25, corte 1)" en `docs/design/PHASE6_LLM.md` §6.3.

**Batch 26 — Backend (FASE 6.3, corte 2 — personalidad LLM del NPC, resumen LLM y config) + Frontend + QA + Docs (26-oct-2026):**

- **Qué es:** FASE 6.3 pasa a **cortes 1+2 cableados**. El diálogo `ask` ganó personalidad estructurada y memoria de largo plazo real: `LLMConfig` (`pkg/entities/npcs/llm.go`) ahora lleva `Personality *LLMPersonality` (Big5 en [0,1]), `Goals`, `Secrets` y `Relations []LLMRelation` (Kind ∈ ally/rival/family/faction/mentor, Sentiment −1..1), con `Clone()`, `EffectivePersonality()`, `HasPersonality()` y `EffectiveMemoryTurns()` (ventana de facts); el proveedor (`pkg/server/llm.go`) resuelve la config viva (`npcLLMConfig`: instancia → plantilla) e inyecta las variables `personality`, `goals`, `secrets` y `relations` al prompt `npc.dialogue.yaml` (v1.0.1), cuyo system prompt ordena **no revelar secretos literalmente**. Un guard anti-fuga (`validateNPCDialoguePayloadWithSecrets` + `leaksSecret`) rechaza replies,
`options[].label` y `memoryNotes` que reproducen un secreto palabra por palabra (`secret_leak` → retry con feedback → fallback neutro); las paráfrasis no se rechazan.
- **Resumen LLM de memoria:** `triggerNPCMemorySummaryIfNeeded` dispara tras cada turno, en goroutine (ctx 12 s), cuando `Turns % SummaryAtTurns() == 0`, con `TaskNPCMemorySummary` (plantilla `npc.memory_summary.yaml`, temp 0.2, `RequireLLM: false`, sin caché, limit key `npcmem:<npcID>:<characterID>`); éxito → recarga la fila y guarda `Summary = …` con `SummarySource = "llm"`; fallo/vacío → el resumen determinista sigue. `repository.NPCMemory.SummarySource` (`""`/`deterministic`/`llm`, en el blob JSON — sin migración) hace que `RecordTurn` **preserve** los resúmenes LLM/externos y solo reconstruya los deterministas.
- **Config en tres vías:** (1) YAML importer (`importer.YAMLNPC.LLM`, camelCase, deep copy en `ToEntity`/`converters.go`); (2) NPCTemplate REST (campo `llm` en detail DTO y upsert de `handler/npc_templates.go`, `Clone()` en ambas direcciones; lecturas solo autenticadas); (3) NPC factory UI del play client (panel "Dialogue / LLM", `NpcFactory.svelte` + `normalizeLLM` en `npcFactory.js`). **Spawn hereda:** `SpawnFromTemplate` (`pkg/service/npcs.go`) copia `template.LLM.Clone()`, de modo que una instancia no muta la plantilla. Contenido de ejemplo: **Master Aldric** (`TUT_MENTOR`, `import/adventure/data/npcs/tut_mentor.yaml`, gitignored) con `llm.enabled`, Big5, 2 goals, 1 secret y relación `player/mentor` 0.7 — `ask Master Aldric …` en R1001 ejercita el camino completo con proveedor instalado.
- **Archivos clave:** `pkg/entities/npcs/llm.go`, `pkg/server/llm.go` (`formatNPCPersonalityContext`, `npcLLMConfig`, `triggerNPCMemorySummaryIfNeeded`, `validateNPCDialoguePayloadWithSecrets`, `leaksSecret`, `formatNPCMemoryLimited`, `loadNPCMemoryFull`), `pkg/repository/npc_memory.go` (`SummarySource`), `pkg/service/npcmemories.go` (`RecordTurn` preserva), `pkg/service/npcs.go` (spawn), `pkg/importer/{models,converters}.go`, `pkg/server/handler/npc_templates.go`, `pkg/service/llmprompt/prompts/{npc.dialogue,npc.memory_summary}.yaml`, `import/adventure/data/npcs/tut_mentor.yaml`, `public/mud-client/src/game/ui/NpcFactory.svelte`, `public/mud-client/src/game/npcFactory.js`.
- **Tests:** `importer/llm_yaml_test.go` (YAML camelCase → deep copy), `service/npcmemories_test.go` (3: determinista, preserva resumen LLM, preserva resumen externo), `TestSpawnFromTemplateInheritsLLM` (spawn hereda `llm`), `server/llm_npc_test.go` (+4 sobre el corte 1: secretos pasados al prompt, fuga literal rechazada, guard sobre `options`/`memoryNotes` — validator y payload —; total 8) y `server/handler/npc_templates_test.go` (`TestNPCTemplateGetRedactsSecretsForNonCreators`). `go build`/`go vet`/suite verde en el corte.
- **Estado histórico al cerrar Batch 26:** aún quedaban B/C, `FallbackDialogID` sin uso y caché de 30 min; B/C se completan en Batch 29 y `FallbackDialogID` ahora sirve como origen secundario para mapear etiquetas, aunque sigue sin ser fallback de error. Las observaciones QA de secretos y cobertura del guard quedaron cerradas. Sin sección propia en el reporte QA (corre hasta §B24).
- **Docs (rol Docs):** FASE 6.3 cortes 1+2 pasan a **implementado** en `PROJECT.md` (roadmap, tabla de fases, comandos, bloques NPC/FASE 6, verificación — bloque consolidado pasa a "Batches 17–26"), `ARCHITECTURE.md` (párrafo Fase 6, subsistema NPC LLM con config/personalidad/resumen/anti-fuga, filas `ask` y `NPCMemoriesService`, estado de batches), `FEATURES.md` (esta sección: Last Updated, snippet `NPC.LLM`, comando `ask` en diálogos, bloque Fase 6 y estado de batches), `README.md` (fork additions + roadmap), `docs/index.md` y `docs/user/commands.md` (`ask` con personalidad/secretos/resumen/config); nota "Implementado (Batch 26, corte 2)" en `docs/design/PHASE6_LLM.md` §6.3 (sin reescribir el diseño).

**Batch 27 — Backend + Frontend (FASE 6.3, corte 3 — opciones LLM accionables, conversación `__llm__`, relaciones dinámicas y feedback de cliente) + QA + Docs (30-oct-2026):**

- **Qué es:** FASE 6.3 pasa a **cortes 1–3 cableados**. Las `options[]` que antes se pintaban como texto plano pasan a ser **accionables**: `deliverNPCDialogue` normaliza las opciones (`normalizeNPCDialogueOptions`: ≤4, `kind ∈ {flavor,end}`, label ≤120 runes) y las enriquece con `mapNPCDialogueOptions`/`matchAuthoredDialogLabel` (label → nodo del diálogo autoral: instancia → plantilla → `llm.FallbackDialogID`); con Conversation Service, `sendNPCDialogueDialog` abre una conversación con el **`DialogID` centinela `"__llm__"`** (`def.NPCDialogueDialogID`), guarda las opciones en `context` (`LLM_OPTIONS`), engagea la sesión NPC y manda `messages.NewDialogMessage` numerado (`Index = i+1`, `Username = <NPC>`). El input numérico pasa por `dialog_select.go` → `handleLLMDialogSelection` (`commands/llm_dialog.go`): `end` cierra, `NodeID` continúa el árbol autoral vía el nuevo `talkToNPCAt` (la conversación vira de `__llm__` al `DialogID` autoral con `CurrentNodeID = startNode`), el resto es `flavor` con `Continuation = true` (salta el rechazo por cooldown, refresca la ventana, respeta el semáforo); selección inválida → *"Invalid option. Please choose 1-N"* sin cerrar. Las conversaciones con `DialogID` vacío se ignoran (una sesión NPC abierta no reactiva opciones cerradas). **Relaciones dinámicas:** `relationDelta` del payload (clamp ±0.05 con `npc.ClampRelationDelta`), `applyNPCRelationDelta` → `NPCMemoriesService.AdjustRelation` → `npc.ApplyRelationDelta` (acumulado [-1,1] en `NPCMemory.Relations`, sin migración; default +0.02, 0 si la opción es `end`) y `mergeDynamicRelations` al prompt como `player(dinámica,0.04)`; `npc.dialogue.yaml` sube a **v1.0.2**. **Cliente:** `relationFeedback.js` + `Client.js` + `MUDXPlusStore.dialogRelation` + `DialogOverlay.svelte` (fila ▲/▼/•, wired en `RoomWidget.svelte` y `MobileRoomView.svelte`), **solo si el mensaje trae info de relación**.
- **Archivos clave:** `pkg/mudserver/game/npc_dialogue.go` (`normalizeNPCDialogueOptions`, `mapNPCDialogueOptions`, `matchAuthoredDialogLabel`, `sendNPCDialogueDialog`, `npcDialogueAllow` con `Continuation`), `pkg/mudserver/game/commands/llm_dialog.go` (nuevo: `handleLLMDialogSelection`, `closeLLMDialogConversation`, `parseLLMDialogOptions`), `pkg/mudserver/game/commands/dialog_select.go` (routing `__llm__` + guard de `DialogID` vacío), `pkg/mudserver/game/commands/talk.go` (`talkToNPCAt`), `pkg/mudserver/game/commands/ask.go` (`buildNPCDialogueRequest` con `continuation`), `pkg/entities/npcs/llm.go` (`ClampRelationDelta`, `ApplyRelationDelta`, `MaxRelationDelta = 0.05`), `pkg/service/npcmemories.go` (`AdjustRelation`), `pkg/repository/npc_memory.go` (`Relations`), `pkg/server/llm.go` (`mergeDynamicRelations`, `applyNPCRelationDelta`), `pkg/service/llmprompt/prompts/npc.dialogue.yaml` (v1.0.2), `def/gamectrl.go` (`NPCDialogueDialogID`) y en el cliente `public/mud-client/src/game/relationFeedback.js`, `ui/DialogOverlay.svelte`, `game/Client.js`, `game/MUDXPlusStore.js`, `widgets/RoomWidget.svelte`, `mobile/MobileRoomView.svelte`.
- **Tests:** `commands/llm_dialog_test.go` (5), `npc_dialogue_test.go` (`TestMatchAuthoredDialogLabelFindsNodeAndOption`), `service/npcmemories_test.go` (`TestAdjustRelationClampsAndPersists`), `server/llm_npc_test.go` (`TestInstallNPCDialogueProviderAdjustsRelation`) y `public/mud-client/src/game/relationFeedback_test.mjs` (+ `dialogOverlay_test.mjs` ejecutados a mano).
- **QA (APROBADO CON OBSERVACIONES, 2026-10-30):** build/vet/suite completos en verde con Docker `golang:1.26`, `-race` en `pkg/mudserver/... pkg/server/... pkg/service/... pkg/entities/npcs/...` sin `DATA RACE`, `npm run build` OK; contrato verificado (opciones → nodo autoral/`flavor`/`end`, selección inválida mantiene opciones, delta acotado + persistido + en el prompt, secretos nunca expuestos, conversación stale no intercepta, `Username` en `DialogMessage`, sin regresión de `ask`/`talk`). **Observación no bloqueante ajena al batch:** `TestWardGritAndRetaliate` (`pkg/mudserver/game/combat/ward_test.go:111`, "tiny hit 2") falló de forma intermitente en la primera corrida con `-race` y pasó 10/10 en aislamiento (flakiness preexistente por crítico aleatorio; los tests de guardia `TestWardGuard*` del mismo fichero pasan). Warnings frontend no bloqueantes (selectores CSS sin uso, `autofocus` A11y). Sin sección propia en `docs/qa/REGRESSION_REPORT.md`.
- **Pendientes al cierre del Batch 27 (histórico):** mecanismos B/C y `kind=quest:<id>` estaban pendientes entonces y se implementaron en Batch 29. Siguen pendientes la caché de 30 min que puede repetir respuestas y `FallbackDialogID` como fallback de error; este último solo sirve como fuente secundaria de diálogo autoral.
- **Docs (rol Docs):** FASE 6.3 corte 3 pasa a **implementado** en `PROJECT.md` (roadmap, tabla de fases, fila `ask`, bloque consolidado → "Batches 17–27"), `ARCHITECTURE.md` (párrafo Fase 6, subsistema NPC LLM: entrega/selección/relaciones/feedback, filas `ask` y estado de batches), `FEATURES.md` (esta sección: Last Updated, snippet `NPC.LLM`, comando `ask`, bloque Fase 6, deuda y estado de batches), `docs/design/PHASE6_LLM.md` (nota "Implementado (Batches 25–27, cortes 1–3)") y `docs/user/commands.md` (`ask` con opciones accionables, relaciones, feedback y ejemplo con **Master Aldric**); nota del Batch 27 en `PROJECT.md`. **QA guard:** el observador señaló `TestWardGritAndRetaliate` de `ward_test.go` (guard tests `TestWardGuard*` en el mismo fichero).

---

## Document Maintenance

**Update this file when**:
- Adding new entity fields
- Adding new Lua API functions
- Adding new game systems
- Adding new Creator UI features
- Changing data structures

**Keep in sync with**:
- `PROJECT.md` - High-level features
- `ARCHITECTURE.md` - System architecture
- `docs/design/SCRIPTING.md` - Scripting documentation
- Creator UI column definitions (`tableColumns.js`)

---

**End of Document**
