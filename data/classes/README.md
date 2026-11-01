# Base Class Pack

This directory is the world-pack class catalog for the fork. On startup the
server calls `classkit.LoadWorld()`, which searches, in order:

1. `CLASSKIT_DIR`
2. `import/mvp-rpg-1/data/classes`
3. `../talesmud-rpg-1/data/classes`
4. `data/classes` (this directory, found by walking up from the working dir)

Every `*.yaml` / `*.yml` file here is one class. The first directory that has
files wins, so this pack overrides the generic sample trio when no other pack
is configured. `LoadDir` validates the whole directory as one unit: a single
bad file (unknown effect, unknown school, duplicate id) fails the load and the
server falls back to the previous catalog.

## Files

| File | id | Name | School | Role |
|------|----|------|--------|------|
| `warrior.yaml` | `warrior` | Warrior / Guerrero | `martial` | Plate front line, one heavy swing, Brace. |
| `rogue.yaml` | `rogue` | Rogue / Pícaro | `martial` | Two lighter swings, Nick, one Slip, Smoke. |
| `mage.yaml` | `mage` | Mage / Mago | `arcane` | Cloth caster, Inscribe burn, Sear, Glyph. |
| `cleric.yaml` | `cleric` | Cleric / Clérigo | `divine` | Melee bulwark: Guard, Mend, Sigil, blessed Slam, Renew, Retribution bolt. |
| `druid.yaml` | `druid` | Druid / Druida | `nature` | Fae-fire burn, bark shield, Rejuvenate, wildfire burst, Verdant Mend. |
| `sorcerer.yaml` | `sorcerer` | Sorcerer / Hechicero | `fire` | Glass cannon: Ember Discharge, Fireblast, ember construct + Overload. |

Spanish display names are searchable through `aliases` (`guerrero`, `mago`,
`picaro`, `clerigo`, `druida`, `hechicero` and the feminine forms). The
`name:` field stays in English because the play client and the portrait set
are English.

## Format

`pkg/classkit/catalog.go` is the schema. The important fields:

- `id`, `name`, `description`, `order` (unique 1..6 here).
- `armor_type`: `Cloth`, `Leather`, or `Plate` (equip training ranks).
- `combat_type`: `Melee` or `Magic` (display label; mana use is `caster`).
- `school`: one of the registered schools below. Omit it for a physical class
  (this pack still writes `martial` to be explicit).
- `primary`: the basic-attack attribute (`STR`, `DEX`, `INT`, `WIS`).
- `skill_class`: the id stamped on this class's skill rows. Defaults to `id`.
- `aliases` / `also`: extra ids that resolve to this class / share this row.
  `also` rows (`ranger`, `hunter`) are never playable on their own.
- `races`: a subset of `human, dwarf, elf, construct`.
- `hp_multiplier`, `caster`, `hotbar_cap`, the charge fields
  (`brace_charges`, `slip_charges`, `bolt_charges`, `rig_charges`),
  `inscribe_basic`, and `balance`.
- `skills`: kit skills. Each `effect` must be one of `KnownEffects`:
  `brace, slam, stand, slip, nick, smoke, inscribe, sear, glyph, guard,
  bolt, rig, overload, mend, renew`.

`mend` is a direct heal and `renew` is a heal-over-time (HoT). Both target
`ally`, and the engine falls back to the caster when the target is an enemy,
dead, or gone. `tick = max HP × multiplier`; `renew` uses its `multiplier`
per round and lasts `duration` rounds (3 when omitted).

Skill ids are prefixed with the class (`<class>_<skill>_`) and are unique
across the whole pack, because they feed the hotbar.

### Schools

`pkg/classkit/schools.go` is the registry. The id is the YAML value; the
display name is what `SchoolName(id)` returns.

| id | Name | Spanish |
|----|------|---------|
| `martial` | Martial | (ninguna, clase física) |
| `fire` | Fire | Fuego |
| `ice` | Ice | Hielo |
| `arcane` | Arcane | Arcano |
| `divine` | Divine | Divino |
| `nature` | Nature | Naturaleza |
| `shadow` | Shadow | Sombras |

An unknown `school` value fails validation:
`<path>: class <id> has unknown school "<value>"`.

### Templates

Each class carries one `template` block. `starting_items` names must exist in
`items.StarterItemTemplateByName` (see
`pkg/entities/items/starteritems_presets.go`), which is why the pack uses
`Rusty Sword`, `Worn Dagger`, `Apprentice Staff`, `Simple Mace`, `Wooden
Staff`, `Leather Armor`, and `Cloth Robe`. `default_skills` lists the level-1
skill the hotbar seeds with.

## Adding a class

1. Copy a file and change `id`, `name`, `order`, and `school`.
2. Keep every skill `effect` inside `KnownEffects`.
3. Prefix every skill id with the new class id.
4. Run `go test ./pkg/classkit/...`; `worldpack_data_test.go` loads this
   directory and fails if any file does not validate.
