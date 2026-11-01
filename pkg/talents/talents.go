// Package talents is the progression talent catalog.
//
// A talent is a passive, per-character upgrade bought with talent points earned
// every five character levels (plus one per prestige). The catalog is pure data:
// an id, a display name, a category, the schools allowed to learn it, and one
// deterministic numeric effect. Characters store only which talents they bought
// and how many ranks; the effect is projected through characters.DeriveStats.
//
// No content pack edit is required: a class is offered the talents of its
// classkit school. A pack may still pin an explicit list per class through the
// optional `talents:` key in a class YAML; classkit then validates the ids.
package talents

import "strings"

// Category groups a talent for the `talents list` UI.
type Category string

const (
	Offensive Category = "offensive"
	Defensive Category = "defensive"
	Utility   Category = "utility"
)

// Effect is the single derived-stat projection a talent feeds. Keeping one
// effect per talent makes application order irrelevant and the result stable.
type Effect string

const (
	EffectDamage          Effect = "damage"           // +% basic attack damage
	EffectCrit            Effect = "crit"             // +% crit chance
	EffectSpellDamage     Effect = "spell_damage"     // +% spell damage
	EffectDamageReduction Effect = "damage_reduction" // +% damage taken reduction
	EffectMaxHP           Effect = "max_hp"           // +% max HP
	EffectBlock           Effect = "block"            // +% block chance
	EffectInitiative      Effect = "initiative"       // + initiative
	EffectStaminaRegen    Effect = "stamina_regen"    // + stamina regen
	EffectSpellCooldown   Effect = "spell_cooldown"   // -% spell cooldown
	EffectHeal            Effect = "heal"             // +% heal output
	EffectEvasion         Effect = "evasion"          // +% evasion
	EffectPotion          Effect = "potion"           // +% potion strength
)

// Def describes one talent. Amount is applied per rank.
type Def struct {
	ID          string
	Name        string
	Category    Category
	Effect      Effect
	Amount      float64
	Schools     []string
	Description string
}

// catalog is the sample roster. Order is stable for UI and tests.
var catalog = []Def{
	{
		ID: "talent_power_strike", Name: "Golpe Poderoso", Category: Offensive,
		Effect: EffectDamage, Amount: 5, Schools: []string{"martial"},
		Description: "+5% basic attack damage per rank.",
	},
	{
		ID: "talent_critical_eye", Name: "Ojo Crítico", Category: Offensive,
		Effect: EffectCrit, Amount: 2, Schools: []string{"martial", "shadow"},
		Description: "+2% crit chance per rank.",
	},
	{
		ID: "talent_mana_surge", Name: "Oleada de Maná", Category: Offensive,
		Effect: EffectSpellDamage, Amount: 10, Schools: []string{"fire", "ice", "arcane"},
		Description: "+10% spell damage per rank.",
	},
	{
		ID: "talent_iron_skin", Name: "Piel de Hierro", Category: Defensive,
		Effect: EffectDamageReduction, Amount: 3, Schools: []string{"martial", "ice"},
		Description: "+3% damage reduction per rank.",
	},
	{
		ID: "talent_endure", Name: "Resistencia", Category: Defensive,
		Effect: EffectMaxHP, Amount: 5, Schools: []string{"martial", "divine", "fire", "ice", "arcane", "nature"},
		Description: "+5% max HP per rank.",
	},
	{
		ID: "talent_shield_master", Name: "Maestro Escudo", Category: Defensive,
		Effect: EffectBlock, Amount: 2, Schools: []string{"martial", "divine"},
		Description: "+2% block chance per rank.",
	},
	{
		ID: "talent_wind_step", Name: "Paso del Viento", Category: Utility,
		Effect: EffectInitiative, Amount: 1, Schools: []string{"martial", "shadow", "nature"},
		Description: "+1 initiative per rank.",
	},
	{
		ID: "talent_second_wind", Name: "Segundo Aire", Category: Utility,
		Effect: EffectStaminaRegen, Amount: 1, Schools: []string{"martial", "nature"},
		Description: "+1 stamina regen per rank.",
	},
	{
		ID: "talent_spell_momentum", Name: "Impulso Arcano", Category: Utility,
		Effect: EffectSpellCooldown, Amount: 5, Schools: []string{"arcane", "fire", "ice"},
		Description: "-5% spell cooldown per rank.",
	},
	{
		ID: "talent_healer_gift", Name: "Dón Sanador", Category: Utility,
		Effect: EffectHeal, Amount: 5, Schools: []string{"divine", "nature"},
		Description: "+5% heal output per rank.",
	},
	{
		ID: "talent_shadow_step", Name: "Paso Sombrío", Category: Utility,
		Effect: EffectEvasion, Amount: 2, Schools: []string{"shadow"},
		Description: "+2% evasion per rank.",
	},
	{
		ID: "talent_nature_bond", Name: "Vínculo Natural", Category: Utility,
		Effect: EffectPotion, Amount: 5, Schools: []string{"nature"},
		Description: "+5% potion strength per rank.",
	},
}

// byID is built once from catalog.
var byID = func() map[string]Def {
	m := make(map[string]Def, len(catalog))
	for _, d := range catalog {
		m[d.ID] = d
	}
	return m
}()

// All returns a copy of the catalog in stable order.
func All() []Def {
	out := make([]Def, len(catalog))
	copy(out, catalog)
	return out
}

// Lookup returns the talent for an id or a nil-safe false. Matching is
// case-insensitive and also accepts the id without the "talent_" prefix.
func Lookup(id string) (Def, bool) {
	key := norm(id)
	if d, ok := byID[key]; ok {
		return d, true
	}
	if !strings.HasPrefix(key, "talent_") {
		if d, ok := byID["talent_"+key]; ok {
			return d, true
		}
	}
	return Def{}, false
}

// ForSchool returns the talents a school may learn, in catalog order.
func ForSchool(school string) []Def {
	school = norm(school)
	if school == "" {
		return nil
	}
	var out []Def
	for _, d := range catalog {
		if hasSchool(d, school) {
			out = append(out, d)
		}
	}
	return out
}

// Available reports whether a school may learn the talent id.
func Available(school, id string) bool {
	d, ok := Lookup(id)
	if !ok {
		return false
	}
	return hasSchool(d, norm(school))
}

// IDs returns the ids of defs in order.
func IDs(defs []Def) []string {
	out := make([]string, 0, len(defs))
	for _, d := range defs {
		out = append(out, d.ID)
	}
	return out
}

func hasSchool(d Def, school string) bool {
	for _, s := range d.Schools {
		if norm(s) == school {
			return true
		}
	}
	return false
}

func norm(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
