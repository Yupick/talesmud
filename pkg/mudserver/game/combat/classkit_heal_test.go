package combat

import (
	"testing"

	"github.com/talesmud/talesmud/pkg/entities"
	"github.com/talesmud/talesmud/pkg/entities/characters"
	entcombat "github.com/talesmud/talesmud/pkg/entities/combat"
	"github.com/talesmud/talesmud/pkg/entities/skills"
)

// newHealKit returns a mend (direct heal) and a renew (HoT) for classID and
// installs them in the skill cache for the duration of the test.
func newHealKit(t *testing.T, classID string) (*skills.Skill, *skills.Skill) {
	t.Helper()
	prev := skills.AllSkills()
	mend := &skills.Skill{
		Entity: &entities.Entity{ID: "test_mend"}, Name: "Mend",
		ClassIDs: []string{classID}, LevelRequired: 1,
		ResourceType: skills.ResourceCooldown, CooldownRounds: 4,
		Target: skills.TargetAlly, Effect: skills.EffectHeal,
		Kit: skills.KitMend, SwingMult: 0.30,
	}
	renew := &skills.Skill{
		Entity: &entities.Entity{ID: "test_renew"}, Name: "Renew",
		ClassIDs: []string{classID}, LevelRequired: 1,
		ResourceType: skills.ResourceCooldown, CooldownRounds: 6,
		Target: skills.TargetAlly, Effect: skills.EffectHot,
		Kit: skills.KitRenew, SwingMult: 0.12, Duration: 3,
	}
	skills.LoadFromDB([]*skills.Skill{mend, renew})
	t.Cleanup(func() { skills.LoadFromDB(prev) })
	return mend, renew
}

func TestKitMendHealsAllyAndFallsBackToSelf(t *testing.T) {
	class := characters.Class{ID: "cleric", Name: "Cleric"}
	e, inst, heroID, enemyID := newFight(class, 0)
	newHealKit(t, "cleric")

	hero := inst.GetCombatantByID(heroID)
	hero.CurrentHP = 100
	hero.EquippedSkills = []string{"test_mend"}
	hero.SkillCooldowns = map[string]int{}
	hero.KitSpent = map[string]bool{}

	// 30% of 400 max HP.
	res := e.ProcessSkill(inst, heroID, "test_mend", heroID)
	if !res.Success || res.TotalHeal != 120 {
		t.Fatalf("mend self %+v", res)
	}
	hero = inst.GetCombatantByID(heroID)
	if hero.CurrentHP != 220 {
		t.Fatalf("mend hp %d, want 220", hero.CurrentHP)
	}
	if again := e.ProcessSkill(inst, heroID, "test_mend", heroID); again.Success {
		t.Fatal("mend ignored cooldown")
	}

	// An enemy target is not a legal ally; the caster is the fallback.
	hero.SkillCooldowns = map[string]int{}
	e.UpdateCombatant(inst, hero)
	res = e.ProcessSkill(inst, heroID, "test_mend", enemyID)
	if !res.Success || res.TotalHeal != 120 {
		t.Fatalf("mend enemy fallback %+v", res)
	}
}

func TestKitRenewAppliesHotAndTicks(t *testing.T) {
	class := characters.Class{ID: "druid", Name: "Druid"}
	e, inst, heroID, _ := newFight(class, 0)
	newHealKit(t, "druid")

	hero := inst.GetCombatantByID(heroID)
	hero.CurrentHP = 100
	hero.EquippedSkills = []string{"test_renew"}
	hero.SkillCooldowns = map[string]int{}
	hero.KitSpent = map[string]bool{}

	res := e.ProcessSkill(inst, heroID, "test_renew", heroID)
	if !res.Success {
		t.Fatalf("renew %+v", res)
	}
	hero = inst.GetCombatantByID(heroID)
	var hot *entcombat.StatusEffect
	for i := range hero.StatusEffects {
		if hero.StatusEffects[i].Type == "hot" && hero.StatusEffects[i].SkillID == "test_renew" {
			hot = &hero.StatusEffects[i]
		}
	}
	if hot == nil || hot.Value != 48 || hot.Duration != 3 {
		t.Fatalf("renew effect %+v", hot)
	}

	// The round-start tick heals and decrements the remaining rounds.
	e.ProcessStatusEffects(inst, hero)
	hero = inst.GetCombatantByID(heroID)
	if hero.CurrentHP != 148 {
		t.Fatalf("hot tick hp %d, want 148", hero.CurrentHP)
	}
	if len(hero.StatusEffects) != 1 || hero.StatusEffects[0].Duration != 2 {
		t.Fatalf("hot did not tick down: %+v", hero.StatusEffects)
	}
}
