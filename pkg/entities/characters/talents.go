package characters

import (
	"github.com/talesmud/talesmud/pkg/classkit"
	"github.com/talesmud/talesmud/pkg/entities/skills"
	"github.com/talesmud/talesmud/pkg/mudserver/game/balance"
	"github.com/talesmud/talesmud/pkg/talents"
)

// EnsureTalents initializes the spent map so callers can write into it. It is
// safe to call on a legacy character whose `talents` field was absent.
func (c *Character) EnsureTalents() {
	if c == nil {
		return
	}
	if c.Talents.Spent == nil {
		c.Talents.Spent = map[string]int32{}
	}
}

// TalentRank returns the ranks bought for a talent id (0 when unknown/nil-safe).
func (c *Character) TalentRank(id string) int32 {
	if c == nil || c.Talents.Spent == nil {
		return 0
	}
	if def, ok := talents.Lookup(id); ok {
		id = def.ID
	}
	return c.Talents.Spent[id]
}

// CanLearnTalent reports whether the character's class offers the talent.
// It uses the classkit school (or an explicit per-class list) so a talent not
// available to the class cannot be bought.
func (c *Character) CanLearnTalent(id string) bool {
	if c == nil {
		return false
	}
	def, ok := talents.Lookup(id)
	if !ok {
		return false
	}
	for _, allowed := range classkit.TalentsForClass(c.Class.ID) {
		if sameTalent(allowed, def.ID) {
			return true
		}
	}
	return false
}

// SpendTalent buys one rank of a talent. It returns false when the id is
// unknown, the class cannot learn it, or the character has no unspent point.
func (c *Character) SpendTalent(id string) bool {
	if c == nil || c.Talents.Points <= 0 {
		return false
	}
	def, ok := talents.Lookup(id)
	if !ok || !c.CanLearnTalent(def.ID) {
		return false
	}
	c.EnsureTalents()
	c.Talents.Spent[def.ID]++
	c.Talents.Points--
	return true
}

// PrestigeMultiplier is the XP awarded multiplier: 1 + 5% per prestige level.
func (c *Character) PrestigeMultiplier() float64 {
	if c == nil || c.Talents.Prestige <= 0 {
		return 1.0
	}
	return 1.0 + 0.05*float64(c.Talents.Prestige)
}

// ApplyPrestige performs the irreversible reset: back to level 1 with class
// base attributes, then +1 prestige and +1 talent point. Spent talents, gold,
// inventory, equipment, discoveries, friends, and all-time stats persist; the
// character's own XP, levels, and distributable attribute points do not.
func (c *Character) ApplyPrestige() {
	if c == nil {
		return
	}
	c.ResetToLevelOne()
	c.EnsureTalents()
	c.Talents.Prestige++
	c.Talents.Points++
}

// ResetToLevelOne rewinds level, XP, attributes, and pools to a fresh class
// start. Talents (spent ranks, points, prestige) are left untouched.
func (c *Character) ResetToLevelOne() {
	if c == nil {
		return
	}
	def := classkit.Lookup(c.Class.ID)

	baseHP := classHPBase
	if def != nil {
		baseHP = balance.ScaleClassHP(def.ID, classHPBase)
		if def.Template != nil {
			t := def.Template
			c.Attributes = createBaseAttributes(t.Str, t.Dex, t.Int, t.Wis, t.Sta)
		}
	}

	c.Level = 1
	c.XP = 0
	c.UnspentAttributePoints = 0
	c.SpentAttributePoints = nil
	c.MaxHitPoints = baseHP
	c.CurrentHitPoints = baseHP
	c.MaxMana = c.CalculateMaxMana()
	c.CurrentMana = c.MaxMana
	c.EquippedSkills = skills.FillHotbar(c.Class.ID, c.Level, nil)
}

func sameTalent(a, b string) bool {
	if a == b {
		return true
	}
	da, aok := talents.Lookup(a)
	db, bok := talents.Lookup(b)
	return aok && bok && da.ID == db.ID
}
