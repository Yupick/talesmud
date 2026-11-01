package main

// roster.go builds player characters from the *real* class catalog
// (pkg/classkit + data/classes/*.yaml) and the real leveling / skill systems.
//
// It does not invent stats: base attributes, HP (base 25 scaled by
// hp_multiplier), starting items, default skills and the class balance row all
// come from the shipped catalog via characters.SystemCharacterTemplatePresets.

import (
	"fmt"
	"strings"

	"github.com/talesmud/talesmud/pkg/classkit"
	"github.com/talesmud/talesmud/pkg/entities"
	"github.com/talesmud/talesmud/pkg/entities/characters"
	"github.com/talesmud/talesmud/pkg/entities/items"
	"github.com/talesmud/talesmud/pkg/entities/skills"
	"github.com/talesmud/talesmud/pkg/mudserver/game/leveling"
)

// canonicalClasses are the six classes shipped in data/classes/*.yaml.
var canonicalClasses = []string{"warrior", "rogue", "mage", "cleric", "druid", "sorcerer"}

// templateFor returns the create template for a canonical class id.
func templateFor(classID string) *characters.CharacterTemplate {
	for _, t := range characters.SystemCharacterTemplatePresets() {
		if t == nil {
			continue
		}
		if strings.EqualFold(t.Class.ID, classID) {
			return t
		}
	}
	return nil
}

// buildCharacter creates a fully equipped, skill-loaded character at the
// requested level using the shipped template for the class.
//
// gear selects the equipment tier used by the balance harness:
//
//	starter     — the shipped starting kit, no level bump
//	appropriate — starter weapon/armor + a small per-level bump (default)
//	good        — ~2.2x the appropriate kit
func buildCharacter(tpl *characters.CharacterTemplate, level int32, gear string) *characters.Character {
	char := &characters.Character{
		Entity:           entities.NewEntity(),
		Name:             tpl.Name,
		Description:      tpl.Description,
		Race:             tpl.Race,
		Class:            tpl.Class,
		Level:            1,
		XP:               0,
		MaxHitPoints:     tpl.MaxHitPoints,
		CurrentHitPoints: tpl.MaxHitPoints,
		Attributes:       cloneAttributes(tpl.Attributes),
		EquippedItems:    make(map[items.ItemSlot]*items.Item),
	}

	// Equip the shipped starting items by template name. Attributes maps are
	// deep-copied so gear scaling never mutates the shared catalog template.
	for _, si := range tpl.StartingItems {
		if item := items.StarterItemTemplateByName(si.ItemTemplateName); item != nil {
			itemCopy := *item
			itemCopy.Entity = entities.NewEntity()
			itemCopy.Attributes = cloneAttrs(item.Attributes)
			char.EquippedItems[si.Slot] = &itemCopy
		}
	}

	// Level up through the real leveling system.
	if level > 1 {
		leveling.ApplyLevelUp(char, int(level-1))
	}
	char.CurrentHitPoints = char.MaxHitPoints

	applyGearScaling(char, level, gear)

	// Casters get mana from the real calculator.
	char.MaxMana = char.CalculateMaxMana()
	char.CurrentMana = char.MaxMana

	// Spend the level-up points on the class's real primary/secondary attack
	// attributes (classkit.Primary wins over the legacy leveling switch, which
	// does not know e.g. the sorcerer).
	spendPrimaryAttributes(char)

	// Equip the class kit through the real hotbar filler.
	char.EquippedSkills = skills.FillHotbar(char.Class.ID, char.Level, tpl.DefaultSkills)

	return char
}

func cloneAttributes(in characters.Attributes) characters.Attributes {
	out := make(characters.Attributes, len(in))
	copy(out, in)
	return out
}

func cloneAttrs(in map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// applyGearScaling mirrors simutil.GearScores: "appropriate" adds level/2 to
// weapon damage and level/3 to armor; "good" multiplies the result by 2.2.
func applyGearScaling(char *characters.Character, level int32, gear string) {
	if gear == "starter" || level <= 1 {
		return
	}
	scale := func(base int32, perkLevel int32) int32 {
		v := base + perkLevel/2
		if gear == "good" {
			v = int32(float64(v)*2.2 + 0.5)
		}
		if v < 1 {
			v = 1
		}
		return v
	}
	if w, ok := char.EquippedItems[items.ItemSlotMainHand]; ok && w != nil && w.Attributes != nil {
		if dmg, ok := w.Attributes["damage"]; ok {
			if d, ok := toInt(dmg); ok {
				w.Attributes["damage"] = scale(int32(d), level)
			}
		}
	}
	if a, ok := char.EquippedItems[items.ItemSlotChest]; ok && a != nil && a.Attributes != nil {
		for _, key := range []string{"armor", "defense"} {
			if raw, ok := a.Attributes[key]; ok {
				if d, ok := toInt(raw); ok {
					v := int32(d) + level/3
					if gear == "good" {
						v = int32(float64(v)*2.2 + 0.5)
					}
					if v < 1 {
						v = 1
					}
					a.Attributes[key] = v
				}
			}
		}
	}
}

func toInt(v interface{}) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int32:
		return int64(n), true
	case int64:
		return n, true
	case float64:
		return int64(n), true
	}
	return 0, false
}

// spendPrimaryAttributes spends UnspentAttributePoints on the class primary,
// spilling to secondary then the basic stats. Mirrors simutil but resolves the
// primary from the class catalog so casters (sorcerer/druid) invest correctly.
func spendPrimaryAttributes(char *characters.Character) {
	if char.UnspentAttributePoints <= 0 {
		return
	}
	primary := classkit.Primary(char.Class.ID)
	if primary == "" {
		primary = leveling.GetPrimaryAttribute(char.Class)
	}
	order := []string{
		strings.ToUpper(primary),
		leveling.GetSecondaryAttribute(char.Class),
		"STA", "STR", "DEX", "INT", "WIS",
	}
	seen := map[string]bool{}
	for _, attr := range order {
		if attr == "" || seen[attr] {
			continue
		}
		seen[attr] = true
		for char.UnspentAttributePoints > 0 {
			if msg := leveling.ValidateAttributeSpend(char, attr, 1); msg != "" {
				break
			}
			leveling.ApplyAttributeSpend(char, attr, 1)
		}
	}
}

// rosterEntry caches a built character template for one (class, level).
type rosterEntry struct {
	classID string
	name    string
	level   int32
	tpl     *characters.CharacterTemplate
}

// buildRoster resolves the canonical class templates at each level once.
func buildRoster(levels []int32) ([]rosterEntry, error) {
	var out []rosterEntry
	for _, id := range canonicalClasses {
		tpl := templateFor(id)
		if tpl == nil {
			return nil, fmt.Errorf("class %q not present in catalog (load data/classes)", id)
		}
		for _, lvl := range levels {
			out = append(out, rosterEntry{classID: id, name: tpl.Name, level: lvl, tpl: tpl})
		}
	}
	return out, nil
}
