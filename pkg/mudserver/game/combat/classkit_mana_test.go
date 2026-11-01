package combat

import (
	"strings"
	"testing"

	"github.com/talesmud/talesmud/pkg/entities"
	"github.com/talesmud/talesmud/pkg/entities/characters"
	"github.com/talesmud/talesmud/pkg/entities/skills"
)

// newManaMend installs a kit mend that costs mana and carries a cooldown, for
// classID, and restores the skill cache after the test. It mirrors the class
// kit skills that now spend mana on top of their cooldown.
func newManaMend(t *testing.T, classID string) {
	t.Helper()
	prev := skills.AllSkills()
	mend := &skills.Skill{
		Entity: &entities.Entity{ID: "test_mana_mend"}, Name: "Mana Mend",
		ClassIDs: []string{classID}, LevelRequired: 1,
		ResourceType: skills.ResourceMana, ManaCost: 10, CooldownRounds: 4,
		Target: skills.TargetAlly, Effect: skills.EffectHeal,
		Kit: skills.KitMend, SwingMult: 0.30,
	}
	skills.LoadFromDB([]*skills.Skill{mend})
	t.Cleanup(func() { skills.LoadFromDB(prev) })
}

func TestKitManaInsufficientBlocksCast(t *testing.T) {
	class := characters.Class{ID: "cleric", Name: "Cleric"}
	e, inst, heroID, _ := newFight(class, 5) // 5 mana, the mend costs 10.
	newManaMend(t, "cleric")

	hero := inst.GetCombatantByID(heroID)
	hero.CurrentHP = 100
	hero.EquippedSkills = []string{"test_mana_mend"}
	hero.SkillCooldowns = map[string]int{}
	hero.KitSpent = map[string]bool{}

	res := e.ProcessSkill(inst, heroID, "test_mana_mend", heroID)
	if res.Success {
		t.Fatalf("cast succeeded without mana: %+v", res)
	}
	if len(res.Messages) == 0 || !strings.Contains(res.Messages[0], "Not enough mana") {
		t.Fatalf("missing mana failure message: %+v", res)
	}

	hero = inst.GetCombatantByID(heroID)
	if hero.CurrentMana != 5 {
		t.Fatalf("failed cast spent mana: %d, want 5", hero.CurrentMana)
	}
	if hero.CurrentHP != 100 {
		t.Fatalf("failed cast healed: %d, want 100", hero.CurrentHP)
	}
	if cd := hero.SkillCooldowns["test_mana_mend"]; cd != 0 {
		t.Fatalf("failed cast started cooldown: %d", cd)
	}
}

func TestKitManaSpentAndHealApplied(t *testing.T) {
	class := characters.Class{ID: "cleric", Name: "Cleric"}
	e, inst, heroID, _ := newFight(class, 30)
	newManaMend(t, "cleric")

	hero := inst.GetCombatantByID(heroID)
	hero.CurrentHP = 100
	hero.EquippedSkills = []string{"test_mana_mend"}
	hero.SkillCooldowns = map[string]int{}
	hero.KitSpent = map[string]bool{}

	res := e.ProcessSkill(inst, heroID, "test_mana_mend", heroID)
	if !res.Success || res.TotalHeal != 120 {
		t.Fatalf("mana mend %+v", res)
	}
	if len(res.Messages) == 0 || !strings.Contains(res.Messages[0], "mana -10") {
		t.Fatalf("success message missing mana spend: %+v", res)
	}

	hero = inst.GetCombatantByID(heroID)
	if hero.CurrentMana != 20 {
		t.Fatalf("mana %d, want 20", hero.CurrentMana)
	}
	if hero.CurrentHP != 220 {
		t.Fatalf("hp %d, want 220", hero.CurrentHP)
	}
	if cd := hero.SkillCooldowns["test_mana_mend"]; cd != 4 {
		t.Fatalf("cooldown %d, want 4: mana and cooldown must both be paid", cd)
	}

	// The cooldown is paid on top of the mana: a recast is blocked and spends
	// nothing.
	again := e.ProcessSkill(inst, heroID, "test_mana_mend", heroID)
	if again.Success {
		t.Fatal("mana mend ignored its cooldown")
	}
	hero = inst.GetCombatantByID(heroID)
	if hero.CurrentMana != 20 {
		t.Fatalf("blocked recast spent mana: %d", hero.CurrentMana)
	}

	// Once the cooldown is clear the mend can be paid for again.
	hero.SkillCooldowns = map[string]int{}
	hero.CurrentHP = 100
	e.UpdateCombatant(inst, hero)
	third := e.ProcessSkill(inst, heroID, "test_mana_mend", heroID)
	if !third.Success {
		t.Fatalf("recast after cooldown %+v", third)
	}
	hero = inst.GetCombatantByID(heroID)
	if hero.CurrentMana != 10 {
		t.Fatalf("recast mana %d, want 10", hero.CurrentMana)
	}
}
