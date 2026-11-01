package characters

import (
	"math"
	"strings"

	"github.com/talesmud/talesmud/pkg/classkit"
	"github.com/talesmud/talesmud/pkg/talents"
)

// Derivation tuning. None of these values feed back into live combat: the
// basic-hit crit roll and the 1d20+DEXMod initiative roll are untouched.
const (
	// derivedStaminaPerLevel is the stamina pool added per character level on
	// top of the STA attribute. Phase 2.4 will spend this pool on stamina
	// regeneration and skill costs; nothing consumes it yet.
	derivedStaminaPerLevel = 2

	// derivedCritBasePercent is the engine's natural-20 crit chance (5%).
	derivedCritBasePercent = 5.0
	// derivedCritPerDEXMod is the crit percent added per point of DEX modifier.
	derivedCritPerDEXMod = 1.0
	// derivedCritSwingsBonus rewards agile kits. The classkit balance row
	// lists a kit as multi-swing (Swings > 1); those kits crit more often.
	derivedCritSwingsBonus = 2.0
	// derivedCritDEXPrimaryBonus rewards kits whose basic attack scales on DEX.
	derivedCritDEXPrimaryBonus = 2.0
	// derivedCritCapPercent mirrors the 95% ceiling the engine applies.
	derivedCritCapPercent = 95.0
)

// DerivedStats are read-only combat values computed from a character's base
// attributes and class kit.
//
// MaxHP and MaxMana mirror the character's stored fields exactly. Combat
// snapshots Character.MaxHitPoints and Character.MaxMana (see
// combat.Engine.CreateCombatantFromCharacter), so the derived view must never
// reshape a live fight; the two fields exist here so the client gets one
// consistent payload. The remaining fields are new projections.
type DerivedStats struct {
	MaxHP      int32   `json:"maxHp"`
	MaxMana    int32   `json:"maxMana"`
	Stamina    int32   `json:"stamina"`
	Initiative int     `json:"initiative"`
	Resist     int     `json:"resist"`
	CritChance float64 `json:"critChance"` // percent, e.g. 7.5 means 7.5%

	// Talent projections. All are zero for a character with no talents, so a
	// legacy character derives exactly the pre-talent values.
	//
	// MaxHPBonusPercent is kept separate from MaxHP: MaxHP mirrors the stored
	// pool combat snapshots, and talents must not reshape that pool. Callers
	// that want the talent-adjusted pool multiply MaxHP by (1 + pct/100).
	DamageBonus        float64 `json:"damageBonus"`        // +% basic attack damage
	SpellDamageBonus   float64 `json:"spellDamageBonus"`   // +% spell damage
	DamageReduction    float64 `json:"damageReduction"`    // +% damage taken reduction
	MaxHPBonusPercent  float64 `json:"maxHpBonusPercent"`  // +% max HP
	BlockChance        float64 `json:"blockChance"`        // +% block chance
	StaminaRegen       int32   `json:"staminaRegen"`       // + stamina regen
	SpellCooldownBonus float64 `json:"spellCooldownBonus"` // -% spell cooldown
	HealBonus          float64 `json:"healBonus"`          // +% heal output
	Evasion            float64 `json:"evasion"`            // +% evasion
	PotionBonus        float64 `json:"potionBonus"`        // +% potion strength
}

// DeriveStats builds DerivedStats for c.
//
// MaxHP/MaxMana reuse the stored character values verbatim, which are the same
// fields combat reads. The new projections are:
//
//	Stamina    = STA + 2*Level               (min 1)
//	Initiative = DEXMod + Level/2            (min 0)
//	Resist     = WISMod + CONMod + Level/2   (min 0)
//	CritChance = 5% + DEXMod% + kit bonus    (clamped to [0, 95])
//
// The kit bonus adds +2% for a multi-swing balance row and +2% for a DEX
// primary attack attribute. classDef is the class kit for the character. A nil
// classDef is resolved from the catalog by c.Class.ID; an unknown class adds
// no kit bonus.
func DeriveStats(c *Character, classDef *classkit.Def) DerivedStats {
	if c == nil {
		return DerivedStats{}
	}
	if classDef == nil {
		classDef = classkit.Lookup(c.Class.ID)
	}

	level := int(c.Level)
	if level < 0 {
		level = 0
	}
	levelHalf := level / 2

	stamina := c.GetAttribute("STA") + int32(level*derivedStaminaPerLevel)
	if stamina < 1 {
		stamina = 1
	}

	initiative := c.GetDEXMod() + levelHalf
	if initiative < 0 {
		initiative = 0
	}

	resist := c.GetWISMod() + c.GetCONMod() + levelHalf
	if resist < 0 {
		resist = 0
	}

	crit := derivedCritBasePercent + float64(c.GetDEXMod())*derivedCritPerDEXMod
	if classDef != nil {
		if classDef.Balance.Swings > 1 {
			crit += derivedCritSwingsBonus
		}
		if strings.EqualFold(classDef.Primary, "DEX") {
			crit += derivedCritDEXPrimaryBonus
		}
	}
	crit = math.Max(0, math.Min(derivedCritCapPercent, crit))

	ds := DerivedStats{
		MaxHP:      c.MaxHitPoints,
		MaxMana:    c.MaxMana,
		Stamina:    stamina,
		Initiative: initiative,
		Resist:     resist,
		CritChance: crit,
	}
	c.ApplyTalentBonuses(&ds)
	// Talents may push crit past the ceiling; clamp once more after they add.
	ds.CritChance = math.Max(0, math.Min(derivedCritCapPercent, ds.CritChance))
	return ds
}

// ApplyTalentBonuses folds every bought talent into ds. The sum is per effect,
// so map iteration order does not matter and the result is deterministic.
// Unknown talent ids (for example an id removed from the catalog) are ignored.
func (c *Character) ApplyTalentBonuses(ds *DerivedStats) {
	if c == nil || ds == nil || len(c.Talents.Spent) == 0 {
		return
	}
	for id, rank := range c.Talents.Spent {
		if rank <= 0 {
			continue
		}
		def, ok := talents.Lookup(id)
		if !ok {
			continue
		}
		amount := def.Amount * float64(rank)
		switch def.Effect {
		case talents.EffectDamage:
			ds.DamageBonus += amount
		case talents.EffectCrit:
			ds.CritChance += amount
		case talents.EffectSpellDamage:
			ds.SpellDamageBonus += amount
		case talents.EffectDamageReduction:
			ds.DamageReduction += amount
		case talents.EffectMaxHP:
			ds.MaxHPBonusPercent += amount
		case talents.EffectBlock:
			ds.BlockChance += amount
		case talents.EffectInitiative:
			ds.Initiative += int(amount)
		case talents.EffectStaminaRegen:
			ds.StaminaRegen += int32(amount)
		case talents.EffectSpellCooldown:
			ds.SpellCooldownBonus += amount
		case talents.EffectHeal:
			ds.HealBonus += amount
		case talents.EffectEvasion:
			ds.Evasion += amount
		case talents.EffectPotion:
			ds.PotionBonus += amount
		}
	}
}
