package skills

import (
	"sort"
	"strings"

	"github.com/talesmud/talesmud/pkg/classkit"
	"github.com/talesmud/talesmud/pkg/entities"
)

// Class-kit ids. These are the only skills the five signed classes learn.
const (
	KitBrace    = "brace"
	KitSlam     = "slam"
	KitStand    = "stand"
	KitSlip     = "slip"
	KitNick     = "nick"
	KitSmoke    = "smoke"
	KitInscribe = "inscribe"
	KitSear     = "sear"
	KitGlyph    = "glyph"
	KitGuard    = "guard"
	KitBolt     = "bolt"
	KitRig      = "rig"
	KitOverload = "overload"
	KitMend     = "mend"
	KitRenew    = "renew"
)

const HotbarCap = 4

// IsKitClass reports a catalog class with a hotbar cap.
// Shared rows such as ranger stay off the kit.
func IsKitClass(classID string) bool {
	return classkit.HotbarCap(classID) > 0
}

// ClassKit is the kit loaded from the class catalog.
func ClassKit() []*Skill {
	specs := classkit.Skills()
	out := make([]*Skill, 0, len(specs))
	for _, spec := range specs {
		out = append(out, kit(spec))
	}
	return out
}

func skillTarget(target string) TargetType {
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "ally":
		return TargetAlly
	case "self":
		return TargetSelf
	case "all_enemies":
		return TargetAllEnemies
	default:
		return TargetEnemy
	}
}

// kit builds one engine skill from a catalog spec. A spec with ManaCost > 0
// spends mana; when it also carries a cooldown, the engine pays both (mana is
// deducted AND the cooldown starts). ResourceType reports mana as the primary
// cost when there is a mana_cost, otherwise the cooldown. The physical kit
// keeps ManaCost 0 and remains cooldown-only.
func kit(spec classkit.SkillSpec) *Skill {
	resource := ResourceCooldown
	if spec.ManaCost > 0 {
		resource = ResourceMana
	}
	return &Skill{
		Entity:         &entities.Entity{ID: spec.ID},
		Name:           spec.Name,
		Description:    spec.Tooltip,
		ClassIDs:       []string{spec.ClassID},
		LevelRequired:  spec.Level,
		ResourceType:   resource,
		ManaCost:       int32(spec.ManaCost),
		CooldownRounds: spec.Cooldown,
		Target:         skillTarget(spec.Target),
		Effect:         kitEffect(spec.Effect),
		Kit:            spec.Effect,
		KeepsSwing:     spec.KeepsSwing,
		OncePerFight:   spec.OncePerFight,
		SwingMult:      spec.Multiplier,
		Duration:       spec.Duration,
		Component:      spec.Component,
	}
}

// kitEffect maps a kit primitive onto the generic effect it resolves like.
// Damage is the default; mend and renew resolve as heal and HoT.
func kitEffect(kind string) EffectType {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case KitMend:
		return EffectHeal
	case KitRenew:
		return EffectHot
	default:
		return EffectDamage
	}
}

// FillHotbar appends known kit skills into empty slots, in unlock order.
// It does not remove a skill the player already chose, and it never passes HotbarCap.
func FillHotbar(classID string, level int32, equipped []string) []string {
	if !IsKitClass(classID) {
		return equipped
	}
	if normalizeClassID(classID) == "ward" {
		equipped = migrateWardHotbar(level, equipped)
	}
	out := make([]string, 0, HotbarCap)
	seen := map[string]bool{}
	for _, id := range equipped {
		if id == "" || seen[id] || len(out) >= HotbarCap {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	known := AvailableSkills(classID, level)
	sort.SliceStable(known, func(i, j int) bool {
		if known[i].LevelRequired != known[j].LevelRequired {
			return known[i].LevelRequired < known[j].LevelRequired
		}
		return known[i].Entity.ID < known[j].Entity.ID
	})
	for _, s := range known {
		if s == nil || s.Kit == "" || s.Entity == nil {
			continue
		}
		if seen[s.Entity.ID] || len(out) >= HotbarCap {
			continue
		}
		seen[s.Entity.ID] = true
		out = append(out, s.Entity.ID)
	}
	return out
}

// migrateWardHotbar drops Pin, Hobble, and Reel and keeps Guard and Slam in unlock order.
// It reads ClassKit directly so a character loaded before the skill cache still keeps a bar.
func migrateWardHotbar(level int32, equipped []string) []string {
	out := make([]string, 0, HotbarCap)
	seen := map[string]bool{}
	for _, id := range equipped {
		switch id {
		case "", "hitch_pin", "hitch_hobble", "hitch_reel":
			continue
		}
		if seen[id] || len(out) >= HotbarCap {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	for _, s := range ClassKit() {
		if s == nil || s.Entity == nil || !s.HasClass("ward") || s.LevelRequired > level {
			continue
		}
		if seen[s.Entity.ID] || len(out) >= HotbarCap {
			continue
		}
		seen[s.Entity.ID] = true
		out = append(out, s.Entity.ID)
	}
	return out
}

// AllowsClass reports whether this skill belongs to classID, including aliases.
func (s *Skill) AllowsClass(classID string) bool {
	if s == nil {
		return false
	}
	return s.HasClass(normalizeClassID(classID))
}
