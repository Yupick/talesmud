# PHASE 5 — Dungeons & Raid Content Pack Design

**Status**: Implemented (`import/adventure/`)
**Engine**: TalesMUD v0.5.0, `pkg/importer` YAML format
**Validation**: `go test ./pkg/importer/...` (`TestAdventurePackHealth` — 0 errors, 0 warnings)

---

## 1. Overview

The adventure pack is a self-contained, importable world extension that plugs into
the existing Millstone Hollow hub through the standard importer pipeline
(`tales -import adventure` / `tales -check adventure`). It adds:

| Zone | Rooms | Level band | Boss |
|------|-------|-----------|------|
| The Training Grounds | 6 (R1001–R1006) | 1–3 | — |
| The Buried Vault | 13 (R2001–R2013) | 6–11 | Hollow Warden (11) |
| Frostspine Caverns | 12 (R3001–R3012) | 12–17 | Frostspine Matriarch (17) |
| The Elder Maw (raid) | 4 (R4001–R4004) | 20–25 | The Elder Maw (25) |

The pack contains **37 rooms, 24 items, 22 NPCs, 9 loot tables, 10 spawners,
1 dialog, 4 scripts, and 2 quests**, all authored in the `pkg/importer` YAML
schema (`data/rooms`, `data/items`, `data/npcs`, `data/loot_tables`,
`data/npc_spawners`, `data/dialogs`, `data/scripts`, `data/quests`).

### Pack location

Content packs live under `import/<name>/data/` because that is the path the
importer reads (`cmd/tales/main.go`: `filepath.Join("import", folderName)`).
The adventure pack is therefore at **`import/adventure/`**. The class catalog
(`data/classes/`) is not overridden; the pack reuses the existing
`data/classes/` from the repo root.

---

## 2. World Connection (startRoomID / areas)

### 2.1 Entry point

The engine's default start room is **`R0001`** (`service.DefaultStartRoomID`,
`pkg/service/start_room.go`). The importer's `applyStartRoom` sets
`ServerSettings.StartRoomID = R0001` whenever a pack contains that room, so the
pack keeps R0001 as its spawn hub.

R0001 ("Awakening", Millstone Hollow) is the village hub. It connects:

```
R0001 Awakening
 ├─ north → R1001 (Training Grounds)
 └─ south → R0002 Old Mill Road
R0002 Old Mill Road
 ├─ north → R0001
 ├─ west  → R2001 (Buried Vault)
 ├─ east  → R3001 (Frostspine Caverns)
 └─ down  → R4001 (The Elder Maw)
```

Every zone is reachable from R0001 through ordinary (non-hidden) exits, so the
content-health `unreachable-rooms` rule passes with zero islands.

### 2.2 Areas

Each room carries an `area` label used by the world-map overlay
(`pkg/contenthealth/overlay.go` groups rooms by `room.Area` for level bands):

| Area | Rooms |
|------|-------|
| Millstone Hollow | R0001, R0002 |
| The Training Grounds | R1001–R1006 |
| The Buried Vault | R2001–R2013 |
| Frostspine Caverns | R3001–R3012 |
| The Elder Maw | R4001–R4004 |

The world index (`pkg/worldindex`) builds rooms/NPCs/spawners/dialogs/scripts
into one graph; the overlay derives per-zone level bands from the placed NPCs
(trash mobs set the band, bosses are marked via spawners).

### 2.3 Importing

```bash
go run ./cmd/tales -import adventure          # live import into SQLite
go run ./cmd/tales -check adventure -fail-on=error   # CI-style validation
```

No migrations are required — the pack uses only existing tables
(`rooms`, `items`, `npcs`, `loot_tables`, `npc_spawners`, `dialogs`, `scripts`,
`quests`).

---

## 3. Zone Designs

### 3.1 The Training Grounds (R1001–R1006)

**Purpose**: safe onboarding zone for levels 1–3. No aggro; teaches movement,
combat basics, and questing.

| Room | Exits | Highlights |
|------|-------|-----------|
| R1001 Mentor's Porch | S→R0001, N→R1002 | **NPC**: Master Aldric (TUT_MENTOR, dialog DLG_TUT_MENTOR, quest giver). `canBind`. |
| R1002 Weapon Practice Yard | S→R1001, E→R1003 | Floor item: ITM_ADV_WOODEN_SWORD. Action: practice stances. |
| R1003 Dummy Row | W→R1002, N→R1004 | **Creatures**: ENM_TUT_DUMMY1, ENM_TUT_DUMMY2 (trivial). Floor item: ITM_ADV_CLOTH_ROBE. |
| R1004 Footwork Circle | S→R1003, E→R1005 | Action: drill footwork. |
| R1005 Focus Garden | W→R1004, N→R1006 | Floor item: ITM_ADV_BREAD. |
| R1006 Grounds Gate | S→R1005, E→R0002 | Exit back to the hub. |

**Quest**: `QST_ADV_FIRST_STEPS` (kill a practice dummy, visit the Focus
Garden) — rewards a healing salve.

### 3.2 The Buried Vault (R2001–R2013)

**Purpose**: first dungeon, levels 6–11. Three themed floors, three traps,
three mini-bosses, one final boss.

**Floor plan**:

```
Floor 1 (R2001–R2006): Antechamber → Flooded Entry → Hall of Echoes →
                        Rubble Passage → Gallery of Kings → Ashen Library
Floor 2 (R2007–R2011): Crumbling Bridge → Smuggler's Cellar → Choir of Teeth →
                        Reliquary → Ossuary
Floor 3 (R2012–R2013): Warden's Threshold → Hollow Warden's Sanctum
```

**Traps** (room actions, narrative damage):

| Room | Action | Effect |
|------|--------|--------|
| R2003 Hall of Echoes | inspect loose flagstone | Poison darts, 5 dmg |
| R2007 Crumbling Bridge | cross the bridge | Fall, 8 dmg |
| R2011 Ossuary | open ornate chest | Dart volley, 6 dmg |

**Hidden exit**: R2004 Rubble Passage has a hidden `down` exit to R2008,
revealed by the "clear rubble" action (`SCR_ADV_REVEAL_CELLAR` →
`tales.game.revealExit("R2004", "down", ctx.characterID)`).

**Encounters**:

| Encounter | Type | Level | Location |
|-----------|------|-------|----------|
| ENM_VAULT_RAT ×2 | normal trash | 6 | R2002 (spawner) |
| ENM_VAULT_SKELETON ×2 | normal trash | 7 | R2005 (spawner) |
| ENM_VAULT_CULTIST ×2 | normal trash | 8 | R2008 (spawner) |
| BOSS_VAULT_KEEPER | hard mini-boss | 8 | R2005 |
| BOSS_VAULT_ACOLYTE | hard mini-boss | 9 | R2009 |
| BOSS_VAULT_GOLEM | hard mini-boss | 10 | R2011 |
| **BOSS_HOLLOW_WARDEN** | **boss** | **11** | **R2013** (spawner, 30m respawn) |

**Boss mechanic** (Hollow Warden): at 30% HP it calls Vault Wisps
(`SCR_ADV_WARDEN_ADDS` → `tales.combat.summon("ENM_VAULT_WISP", 1)`).
`resetOnDisengage: true` so fleeing resets the fight.

**Loot tables**: `LT_ADV_VAULT_TRASH`, `LT_ADV_VAULT_MINI`, `LT_ADV_WARDEN`
(unique boss-only drops: ITM_ADV_BURIAL_MASK, ITM_ADV_RELIC_BLADE).

### 3.3 Frostspine Caverns (R3001–R3012)

**Purpose**: second dungeon, levels 12–17. Cold theme, three mini-bosses,
final boss with adds.

| Room | Highlights |
|------|-----------|
| R3001 Frozen Mouth | Entrance from hub |
| R3003 Wolf Den | ENM_FROST_WOLF ×2 (spawner) |
| R3004 Alpha's Perch | **BOSS_FROST_ALPHA** (hard, 14) |
| R3005 Glacier Crevasse | Trap: cross the ice bridge (4 dmg) |
| R3006 Troll Bridge | ENM_FROST_TROLL ×2 (spawner) |
| R3007 Hot Springs | `canBind`; floor items ITM_ADV_FROST_CRYSTAL, ITM_ADV_WOLF_PELT |
| R3008 Shaman's Circle | **BOSS_FROST_SHAMAN** (hard, 15) |
| R3009 Wyrm's Larder | ENM_FROST_WYRM ×2 (spawner) |
| R3010 Yeti Roost | **BOSS_FROST_YETI** (hard, 16) |
| R3012 Frozen Throne | **BOSS_FROST_MATRIARCH** (boss, 17, spawner 30m) |

**Boss mechanic** (Frostspine Matriarch): at 30% HP calls Frost Whelps
(`SCR_ADV_MATRIARCH_ADDS` → `tales.combat.summon("ENM_FROST_WHELP", 1)`).

**Loot tables**: `LT_ADV_FROST_TRASH`, `LT_ADV_FROST_MINI`, `LT_ADV_MATRIARCH`
(unique boss-only: ITM_ADV_ICEFORGE_BLADE).

### 3.4 The Elder Maw (R4001–R4004) — Raid

**Purpose**: end-game raid for a party of 3–5 players, levels 20–25.

| Room | Highlights |
|------|-----------|
| R4001 Abyssal Rift | Entrance from hub |
| R4002 Sunken Gallery | ENM_MAW_TENDRIL ×2 (spawner) |
| R4003 The Gullet | Transition, no encounters |
| R4004 Maw of the Deep | **BOSS_ELDER_MAW** (boss, 25, spawner 1h respawn) |

**Boss mechanic** (The Elder Maw): at 30% HP calls Maw Spawn ×2
(`SCR_ADV_MAW_ADDS` → `tales.combat.summon("ENM_MAW_SPAWN", 2)`).
Enrage is handled by the engine's `boss_mechanics` config (16 rounds or
30% HP, 1.2× damage, telegraph "Crushing Blow").

**Loot tables**: `LT_ADV_MAW_TRASH`, `LT_ADV_MAW_BOSS` (guaranteed unique
ITM_ADV_MAW_TOOTH, chance unique ITM_ADV_ABYSSAL_CROWN).

---

## 4. Content Reference

### 4.1 ID scheme

| Entity | Prefix | Example |
|--------|--------|---------|
| Rooms | R + zone prefix + number | R1001 (tutorial), R2001 (vault), R3001 (frost), R4001 (maw) |
| Items | ITM_ADV_ | ITM_ADV_WOODEN_SWORD |
| NPCs | ENM_ / BOSS_ / TUT_ | ENM_VAULT_RAT, BOSS_HOLLOW_WARDEN |
| Loot tables | LT_ADV_ | LT_ADV_WARDEN |
| Spawners | SPW_ / SPW_ADV_ | SPW_VAULT_RATS_01 |
| Dialogs | DLG_ | DLG_TUT_MENTOR |
| Scripts | SCR_ADV_ | SCR_ADV_REVEAL_CELLAR |
| Quests | QST_ADV_ | QST_ADV_FIRST_STEPS |

### 4.2 Difficulty tiers

All NPCs use tiers from `config/combat_balance.yaml` `difficulty_multipliers`:
`trivial` (training dummies), `easy` (summon adds), `normal` (trash),
`hard` (mini-bosses), `boss` (final bosses). The content-health
`unknown-difficulty-tier` rule validates this.

### 4.3 Boss loot tables

| Boss | Table | Notable drops |
|------|-------|---------------|
| Hollow Warden | LT_ADV_WARDEN | ITM_ADV_BURIAL_MASK (unique), ITM_ADV_RELIC_BLADE (unique), ITM_ADV_GOLD_IDOL |
| Frostspine Matriarch | LT_ADV_MATRIARCH | ITM_ADV_ICEFORGE_BLADE (unique), ITM_ADV_FROST_AMULET |
| The Elder Maw | LT_ADV_MAW_BOSS | ITM_ADV_MAW_TOOTH (guaranteed unique), ITM_ADV_ABYSSAL_CROWN (unique) |

Unique entries use `rarity: unique` + `boss_only: true`; the engine skips the
drop when every party member already holds the template
(`pkg/mudserver/game/loot.go`).

---

## 5. Validation

`pkg/importer/adventure_pack_test.go` runs the full content-health pipeline over
the pack and asserts:

1. **Load** — `importer.LoadConverted("import/adventure")` succeeds; minimum
   entity counts (30+ rooms, 20+ items, 15+ NPCs, 5+ loot tables, 5+ spawners).
2. **No duplicate IDs** — explicit checks for rooms, items, NPCs, loot tables,
   spawners, scripts, quests.
3. **Content health** — `contenthealth.Run` reports **0 errors and 0 warnings**
   (covers: dangling exits, missing room items, unreachable rooms, hidden
   exits without revealers, unknown difficulty tiers, bosses without spawners,
   unreferenced scripts, impossible quests, dialog integrity, Lua script
   validation).
4. **Reachability** — every room is reachable from R0001; no islands.
5. **Start room** — R0001 exists in the pack.

Run:

```bash
go test ./pkg/importer/... -run TestAdventurePack -v
```
