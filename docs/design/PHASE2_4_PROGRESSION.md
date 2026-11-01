# Fase 2.4 — Progresión (XP, Talents, Prestige)

> **Estado:** Implementado  
> **Scope:** Curva XP, sistema de talents cada 5 niveles, prestige.  
> **Referencia:** TalesMUD v0.5.0, Go 1.26.

---

## 0. Puntos de partida verificados en código

| Área | Archivo | Estado actual |
|------|---------|---------------|
| Curva XP | `pkg/mudserver/game/leveling/xp_curve.go` | Fórmula piece-wise 3 tramos. `XPTable[51]int32` precomputada. Targets: L5≈203, L10≈1120, L20≈4947, L30≈11794, L40≈21271, L50≈33454. |
| Level cap | `config/ruleset.yaml` + `pkg/ruleset/ruleset.go` | `progression.level_cap: 50`. `ruleset.LevelCap()` devuelve 50 por defecto. |
| Level-up | `pkg/mudserver/game/leveling/leveling.go` | `CheckLevelUp` (respeta `MaxLevelCap`), `ApplyLevelUp` (HP, atributos, mana, hotbar), `MaybeLevelUp` (auto mode), `ApplyPendingLevels` (trainer mode). |
| XP en combate | `pkg/mudserver/game/game_combat.go:1430` | `char.XP += int32(awardedXP)` → `leveling.MaybeLevelUp(char)`. |
| XP en quests | `pkg/service/quests.go:1086` | `char.XP += quest.Rewards.XP` → `leveling.MaybeLevelUp(char)`. |
| XP exploración | `pkg/mudserver/game/leveling/xp_sources.go` | 5 XP por sala, 15 XP por área nueva. |
| Atributos | `pkg/mudserver/game/leveling/stat_calculator.go` | `CalculateHPGain` (5 base + STA_mod×2 por nivel), `CalculateAttributeGains` (primary +1 cada 5, secondary +1 cada 10). |
| Puntos atributo | `pkg/mudserver/game/leveling/attribute_caps.go` | `PointsPerLevel = 2`. Caps por clase (`ClassAttributeCaps`). |
| DeriveStats | `pkg/entities/characters/derive.go` | MaxHP, MaxMana, Stamina, Initiative, Resist, CritChance. Punto de enganche para talents. |
| Classkit skills | `pkg/classkit/catalog.go` + `pkg/entities/skills/classkit.go` | `SkillSpec.Level` (nivel requerido). `FillHotbar` rellena hotbar con skills conocidas. `KnownEffects`: brace, slam, stand, slip, nick, smoke, inscribe, sear, glyph, guard, bolt, rig, overload, mend, renew. |
| Escuelas | `pkg/classkit/schools.go` | martial, fire, ice, arcane, divine, nature, shadow. |
| SQLite schema | `pkg/db/sqlite/sqlite.go` | Patrón `(id TEXT PRIMARY KEY, data TEXT NOT NULL)` JSON. Migraciones inline en `runMigrations()`. Fase 4 añadió tablas sociales. |
| Character JSON | `pkg/entities/characters/character.go` | `XP`, `Level`, `MaxHitPoints`, `MaxMana`, `Attributes`, `UnspentAttributePoints`, `SpentAttributePoints`, `MaxLevelCap`, `AllTimeStats`. |

---

## 1. Curva de XP

### 1.1 Situación actual

La curva actual en `xp_curve.go` usa 3 tramos piece-wise:

```
L2–5:   xpForLevel = 30 + 10 * l^0.60
L6–15:  xpForLevel = 50 + 15 * l^1.05
L16–50: xpForLevel = 85 + 17 * l^1.10
```

Acumulado (verificado en código):
| Nivel | XP acumulada |
|-------|-------------|
| 2 | ~45 |
| 5 | ~203 |
| 10 | ~1,120 |
| 20 | ~4,947 |
| 30 | ~11,794 |
| 40 | ~21,271 |
| 50 | ~33,454 |

### 1.2 Propuesta

**Mantener la curva actual como base.** Los targets son razonables para un MUD de 50 niveles. No se sustituye; se **parametriza** para允许 ajuste fino sin tocar código.

#### Cambio: parametrización en ruleset.yaml

```yaml
progression:
  level_cap: 50
  level_up_mode: auto
  # Override opcional: si existe, sustituye la curva built-in.
  # xp_required:
  #   1: 0
  #   2: 45
  #   5: 203
  #   10: 1120
  #   ...
```

`ruleset.XPRequired(level)` ya existe y `GetXPRequired` ya lo consulta. No requiere código nuevo.

#### Cambio: ajuste fino de la fórmula built-in

Si se desea un tweak sin tocar la tabla, la fórmula piece-wise se mantiene pero con constantes exportadas para test:

```go
// xp_curve.go — propuesta de constantes (diseño, no implementar aún)
const (
    curveBaseEarly    = 30.0
    curvePowEarly     = 0.60
    curveBaseMid      = 50.0
    curvePowMid       = 1.05
    curveBaseLate     = 85.0
    curvePowLate      = 1.10
    curveSplitEarly   = 5
    curveSplitMid      = 15
)
```

#### Tabla de referencia (built-in actual)

| Nivel | XP acumulada | XP para siguiente |
|-------|-------------|-------------------|
| 1 | 0 | 45 |
| 5 | 203 | ~85 |
| 10 | 1,120 | ~180 |
| 20 | 4,947 | ~320 |
| 30 | 11,794 | ~480 |
| 40 | 21,271 | ~660 |
| 50 | 33,454 | — (cap) |

---

## 2. Talents cada 5 niveles

### 2.1 Estructura

- **Hitos de talento:** niveles 5, 10, 15, 20, 25, 30, 35, 40, 45, 50 → **10 puntos** totales.
- **Puntos por hito:** 1 punto.
- **Categorías:** Ofensivo, Defensivo, Utilidad.
- **Talents por clase/escuela:** cada clase tiene acceso a un subconjunto de talents según su escuela (`classkit.Def.School`).

### 2.2 Catálogo de talents (diseño)

| ID | Nombre | Categoría | Efecto | Clase/Escuela |
|----|--------|-----------|--------|---------------|
| `talent_power_strike` | Golpe Poderoso | Ofensivo | +5% daño básico | martial |
| `talent_critical_eye` | Ojo Crítico | Ofensivo | +2% crit chance | martial, shadow |
| `talent_mana_surge` | Oleada de Maná | Ofensivo | +10% daño spells | fire, ice, arcane |
| `talent_iron_skin` | Piel de Hierro | Defensivo | +3% reducción daño | martial |
| `talent_endure` | Resistencia | Defensivo | +5% max HP | martial, divine |
| `talent_shield_master` | Maestro Escudo | Defensivo | +2% block chance | martial, divine |
| `talent_wind_step` | Paso del Viento | Utilidad | +1 initiative | martial, shadow, nature |
| `talent_second_wind` | Segundo Aire | Utilidad | +1 stamina regen | martial, nature |
| `talent_spell_momentum` | Impulso Arcano | Utilidad | -5% cooldown spells | arcane, fire, ice |
| `talent_healer_gift` | Dón Sanador | Utilidad | +5% heal output | divine, nature |
| `talent_shadow_step` | Paso Sombrío | Utilidad | +2% evasion | shadow |
| `talent_nature_bond` | Vínculo Natural | Utilidad | +5% potions | nature |

### 2.3 Schema / Almacenamiento

**Decisión: campo JSON en el `Character`, NO tabla nueva.**

Justificación:
- El patrón `(id TEXT PRIMARY KEY, data TEXT NOT NULL)` JSON ya se usa para `DiscoveredRooms`, `RevealedExits`, `FriendIDs`, `FirstBossKills` en `character.go`.
- Los talents son 1 por carácter; no requieren queries relacionales.
- Evita una migración de tabla y mantiene la consistencia del patrón existente.

#### Estructura JSON en `character.go` (propuesta)

```go
// CharacterTalents holds the per-character talent state.
type CharacterTalents struct {
    Points   int32            `json:"points"`             // Unspent talent points
    Spent    map[string]int32 `json:"spent"`              // talentID -> rank (0 = not taken)
    Prestige int32            `json:"prestige"`           // prestige level (0 = none)
}
```

Campo en `Character`:
```go
Talents CharacterTalents `bson:"talents,omitempty" json:"talents,omitempty"`
```

#### Migración (DDL inline en `sqlite.go`)

No requiere DDL nueva — el campo viaja dentro del JSON de `characters.data`. Personajes existentes tendrán `talents: null` al cargar; el código debe tratarlo como `{Points: 0, Spent: {}, Prestige: 0}`.

### 2.4 Aplicación de efectos (enganche con `DeriveStats`)

`DeriveStats` en `derive.go` ya calcula `CritChance`, `Stamina`, `Initiative`, `Resist`. Los talents se aplican como modificadores post-DeriveStats:

```go
// derive.go — propuesta de extensión
func (c *Character) ApplyTalentBonuses(ds *DerivedStats) {
    for talentID, rank := range c.Talents.Spent {
        if rank == 0 { continue }
        switch talentID {
        case "talent_critical_eye":
            ds.CritChance += 2.0 * float64(rank)
        case "talent_endure":
            ds.MaxHP = int32(float64(ds.MaxHP) * (1 + 0.05*float64(rank)))
        // ... resto de talents
        }
    }
}
```

**Punto de llamada:** `DeriveStats` invoca `ApplyTalentBonuses` al final, antes de retornar. El combate ya llama `DeriveStats` en `combat/engine.go:114` y `messages/responses.go:446`.

### 2.5 Flujo de gasto

1. Personaje alcanza nivel hito → `UnspentTalentPoints++` (nuevo campo o reutilizar `UnspentAttributePoints`? — **no, separado**).
2. Jugador usa comando `talents spend <id>` (nuevo comando).
3. Se valida: punto disponible, talent existe, clase tiene acceso.
4. `Spent[talentID]++`, `Points--`.
5. `DeriveStats` aplica el bonus en el próximo tick de combate.

---

## 3. Prestige

### 3.1 Mecánica

- **Requisito:** nivel 50 alcanzado.
- **Reset:** nivel → 1, XP → 0, atributos → base de clase, puntos atributo → base.
- **Bonus permanente:** +5% XP ganado por nivel de prestige (multiplicador `1 + 0.05 * prestige`).
- **Punto extra:** +1 talent point por prestige (se suma a los 10 de hitos).
- **Qué persiste:** `Spent` (talents), `Prestige` (nivel), `AllTimeStats`, `DiscoveredRooms`, `DiscoveredAreas`, `RevealedExits`, `FriendIDs`, `Gold` (o se resetea? — **propuesta: gold se mantiene**), `EquippedItems`, `Inventory`.

### 3.2 Schema

Ya contenido en `CharacterTalents.Prestige`. No requiere campo nuevo.

```go
type CharacterTalents struct {
    Points   int32            `json:"points"`
    Spent    map[string]int32 `json:"spent"`
    Prestige int32            `json:"prestige"`  // ← nivel de prestige
}
```

### 3.3 Flujo de prestige

1. Comando `prestige` disponible en nivel 50.
2. Confirmación (es irreversible).
3. Reset de nivel/XP/atributos.
4. `Prestige++`.
5. `Points += 1` (punto talento bonus).
6. Bonus XP aplica en `game_combat.go` y `quests.go` al sumar XP:

```go
// game_combat.go — propuesta
prestigeMult := 1.0 + 0.05*float64(char.Talents.Prestige)
awardedXP := int32(float64(share.XP+fkXP) * prestigeMult)
char.XP += awardedXP
```

### 3.4 DDL

Sin DDL nueva — todo viaja en `characters.data` JSON.

---

## 4. Impacto

### 4.1 Archivos Go a tocar

| Archivo | Cambio |
|---------|--------|
| `pkg/entities/characters/character.go` | Añadir campo `Talents CharacterTalents` |
| `pkg/entities/characters/derive.go` | Añadir `ApplyTalentBonuses` + llamada desde `DeriveStats` |
| `pkg/mudserver/game/leveling/xp_curve.go` | Exportar constantes de curva (opcional) |
| `pkg/mudserver/game/leveling/leveling.go` | `CheckLevelUp` otorga puntos de talento en hitos (nivel % 5 == 0) |
| `pkg/mudserver/game/game_combat.go` | Aplicar multiplicador prestige en XP |
| `pkg/service/quests.go` | Aplicar multiplicador prestige en XP |
| `pkg/mudserver/game/commands/xp_grants.go` | Actualizar mensaje de level-up para mencionar talento |
| `pkg/mudserver/game/commands/talents.go` | **Nuevo:** comando `talents spend/list` |
| `pkg/mudserver/game/commands/prestige.go` | **Nuevo:** comando `prestige` |
| `pkg/ruleset/ruleset.go` | (Opcional) parsear `progression.xp_required` override |

### 4.2 Migraciones necesarias

| Migración | Tipo | Descripción |
|-----------|------|-------------|
| `talents_field` | Sin DDL | Campo nuevo en JSON `characters.data`. Defaults en código. |
| `prestige_field` | Sin DDL | Contenido en `talents.prestige`. |
| `xp_curve_override` | Sin DDL | `ruleset.XPRequired` ya soporta override. |

**No se requiere DDL de SQLite** — todo viaja en el JSON existente.

### 4.3 Riesgos de compatibilidad

| Riesgo | Mitigación |
|--------|------------|
| Personajes existentes sin campo `talents` | Código trata `nil` como `{Points:0, Spent:{}, Prestige:0}` |
| Personajes en nivel 50+ | No pierden progreso; `Prestige` arranca en 0 |
| Curva XP override | `GetXPRequired` ya consulta `ruleset.XPRequired` antes de `XPTable` |
| `MaxLevelCap` (guests capped at 5) | No pueden hacer prestige (requiere nivel 50); talents funcionan normal |
| Save/load cycle | `Character` se serializa completo; campo nuevo se persiste automáticamente |

### 4.4 Plan por fases (cortes pequeños)

| Paso | Descripción | Risk |
|------|-------------|------|
| **2.4.1** | Campo `Talents` en `Character` + defaults nil-safe. Test de regreso. | Bajo |
| **2.4.2** | `ApplyTalentBonuses` en `DeriveStats` (sin talents todavía). Test de que no cambia nada. | Bajo |
| **2.4.3** | Lógica de hito en `CheckLevelUp` → `Points++`. Test con XP grants. | Medio |
| **2.4.4** | Comando `talents spend/list`. UI de talentos. | Medio |
| **2.4.5** | Multiplicador prestige en `game_combat.go` + `quests.go`. | Medio |
| **2.4.6** | Comando `prestige` + reset flow. Test end-to-end. | Alto |
| **2.4.7** | Integración content pack: talents por escuela en classkit YAML. | Medio |

---

## 5. Integración con content pack y clases/escuelas

### 5.1 Content pack (`import/adventure/`)

El content pack ya define:
- **NPCs** con `XPReward` (XP directa por kill).
- **Quests** con `Rewards.XP`.
- **Items** con bonos.

**Sin cambios en el content pack.** La curva XP y el sistema de talents son transparentes a los datos importados.

### 5.2 Talents por clase/escuela

Los talents se filtran por `classkit.Def.School`:

| Escuela | Talents ofensivos | Talents defensivos | Talents utilidad |
|---------|-------------------|-------------------|------------------|
| martial | power_strike, critical_eye | iron_skin, endure, shield_master | wind_step, second_wind |
| fire | mana_surge | endure | spell_momentum |
| ice | mana_surge | iron_skin | spell_momentum |
| arcane | mana_surge | endure | spell_momentum |
| divine | — | endure, shield_master | healer_gift |
| nature | — | endure | nature_bond, second_wind |
| shadow | critical_eye | — | shadow_step, wind_step |

### 5.3 Definición en classkit YAML (propuesta)

Cada clase en `classkit` puede declarar sus talents:

```yaml
# import/mvp-rpg-1/data/classes/warrior.yaml
id: warrior
school: martial
talents:
  - talent_power_strike
  - talent_critical_eye
  - talent_iron_skin
  - talent_endure
  - talent_shield_master
  - talent_wind_step
  - talent_second_wind
```

`classkit.Def` añade:
```go
Talents []string `yaml:"talents" json:"-"`
```

`classkit.TalentsForClass(classID) []string` devuelve la lista.

### 5.4 Comando `talents list`

Muestra talents disponibles para la clase del personaje, filtrados por `classkit.TalentsForClass`, con indicador de "disponible" / "no disponible por clase".

---

## 6. Resumen de decisiones

| Decisión | Elección | Justificación |
|----------|----------|---------------|
| Curva XP | Mantener built-in + override en ruleset | No rompe balance; ya soportado por `ruleset.XPRequired` |
| Almacenamiento talents | Campo JSON en `Character` | Sigue patrón existente; no requiere DDL |
| Aplicación efectos | `DeriveStats` → `ApplyTalentBonces` | Punto central ya usado por combate y UI |
| Prestige | `CharacterTalents.Prestige` | Sin campo nuevo; reset en comando |
| Migraciones | Ninguna DDL | Todo en JSON |
| Content pack | Sin cambios | Transparente |
| Clases/escuelas | `classkit.Def.Talents []string` | Extensión del catálogo existente |

---

## 7. Referencias

- `pkg/mudserver/game/leveling/xp_curve.go` — Curva XP piece-wise
- `pkg/mudserver/game/leveling/leveling.go` — `CheckLevelUp`, `ApplyLevelUp`
- `pkg/mudserver/game/leveling/stat_calculator.go` — HP y atributos por nivel
- `pkg/mudserver/game/leveling/attribute_caps.go` — Puntos de atributo
- `pkg/entities/characters/character.go` — Struct `Character`
- `pkg/entities/characters/derive.go` — `DeriveStats` (punto de enganche)
- `pkg/mudserver/game/game_combat.go` — XP en combate (línea 1430)
- `pkg/service/quests.go` — XP en quests (línea 1086)
- `pkg/db/sqlite/sqlite.go` — Schema y migraciones
- `pkg/classkit/catalog.go` — `Def`, `SkillSpec`
- `pkg/classkit/schools.go` — Escuelas
- `config/ruleset.yaml` — `progression.level_cap`

---

## Estado de implementación

> Verificado contra el código el **2026-10-15**. La cabecera original de este documento decía
> **"Estado: Diseño"**: el diseño que sigue a continuación se escribió antes de codificar y ya **no**
> describe un trabajo pendiente — la progresión (talents + prestige, y la curva XP que ya existía)
> está implementada y probada. Se actualizó la cabecera a "Implementado" y se dejó aquí el mapa de
> archivos reales.

| Pieza del diseño (§) | Archivo real | Qué hay |
|---|---|---|
| §1 Curva XP / level cap | `pkg/mudserver/game/leveling/xp_curve.go`, `leveling.go`, `config/ruleset.yaml` + `pkg/ruleset/ruleset.go` | Ya existían antes del diseño (ver §0); se mantienen sin cambios |
| §2 Talents: catálogo | `pkg/talents/talents.go` (+ `talents_test.go`) | Categorías (`offensive`/`defensive`/`utility`) y efectos deterministas (`EffectDamage`, `EffectMaxHP`…) |
| §2 Talents: estado en personaje | `pkg/entities/characters/talents.go` (+ `talents_test.go`) | `Character.Talents` (`Points`, `Spent`, `Prestige`), `TalentRank`, `CanLearnTalent`, `PrestigeMultiplier`, `ApplyPrestige` |
| §3 / §5.4 Efectos y filtro por clase | `pkg/entities/characters/derive.go` (proyección en `DeriveStats`), `pkg/classkit/registry.go` (`TalentsForClass`) (+ `pkg/classkit/talents_test.go`) | Un talento por efecto; la clase oferta los de su escuela |
| §2 / §5 Puntos por hito de 5 niveles | `pkg/mudserver/game/leveling/leveling.go` (`TalentMilestoneInterval`, `TalentPointsGained`) (+ `leveling/talents_test.go`) | Se otorgan al subir de nivel y se anuncian en el mensaje de level-up |
| §5.4 Comando `talents [list]` / `talents spend <talent>` | `pkg/mudserver/game/commands/talents.go` (+ `talents_test.go`) | Registrado como `talents` / `talent` en `commandprocessor.go` |
| §5 Prestige | `pkg/mudserver/game/commands/prestige.go` | `prestige` (estado) y `prestige confirm` (nivel 50+, no en combate, irreversible); registrado en `commandprocessor.go` |

No hay DDL nueva: todo el estado de talents vive en el JSON del personaje (decisión §6), así que
`pkg/db/sqlite/sqlite.go` no requirió migración para esta fase.

Guía para jugadores: [`docs/user/commands.md`](../user/commands.md).
