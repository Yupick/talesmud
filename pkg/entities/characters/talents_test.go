package characters

import (
	"testing"

	"github.com/talesmud/talesmud/pkg/mudserver/game/balance"
)

func talentTestChar(classID string) *Character {
	return &Character{
		Class:            Class{ID: classID, Name: classID},
		Level:            5,
		MaxHitPoints:     30,
		CurrentHitPoints: 30,
		MaxMana:          0,
		Attributes: Attributes{
			NewAttribute("Strength", "STR", 10),
			NewAttribute("Dexterity", "DEX", 10),
			NewAttribute("Constitution", "CON", 10),
			NewAttribute("Wisdom", "WIS", 10),
			NewAttribute("Stamina", "STA", 10),
			NewAttribute("Intelligence", "INT", 10),
		},
	}
}

// TestLegacyCharacterWithoutTalents is the nil-safe regression: a character
// saved before Phase 2.4 has the zero CharacterTalents and must derive exactly
// the pre-talent values with no bonus fields set.
func TestLegacyCharacterWithoutTalents(t *testing.T) {
	ch := talentTestChar("warrior")
	if ch.Talents.Spent != nil {
		t.Fatal("fresh character should have a nil spent map")
	}
	if ch.TalentRank("talent_critical_eye") != 0 {
		t.Fatal("legacy character must have no talent ranks")
	}
	if ch.PrestigeMultiplier() != 1.0 {
		t.Fatalf("prestige multiplier = %v, want 1", ch.PrestigeMultiplier())
	}

	got := DeriveStats(ch, nil)
	if got.DamageBonus != 0 || got.SpellDamageBonus != 0 || got.DamageReduction != 0 ||
		got.MaxHPBonusPercent != 0 || got.BlockChance != 0 || got.StaminaRegen != 0 ||
		got.SpellCooldownBonus != 0 || got.HealBonus != 0 || got.Evasion != 0 || got.PotionBonus != 0 {
		t.Fatalf("legacy character has talent bonuses: %+v", got)
	}
	// DEX 10 -> +0, no swings/primary bonus for this bare kit: 5% base crit.
	if got.CritChance != 5 {
		t.Fatalf("CritChance = %v, want 5", got.CritChance)
	}
}

func TestSpendTalentAppliesToDerivedStats(t *testing.T) {
	ch := talentTestChar("warrior")
	ch.Talents.Points = 2

	if !ch.CanLearnTalent("talent_critical_eye") {
		t.Fatal("warrior should learn critical_eye (martial)")
	}
	if ch.CanLearnTalent("talent_shadow_step") {
		t.Fatal("warrior must not learn shadow_step (shadow)")
	}
	if ch.SpendTalent("talent_shadow_step") {
		t.Fatal("spend must reject a talent the class cannot learn")
	}

	if !ch.SpendTalent("critical_eye") { // prefix-less id must resolve
		t.Fatal("spend critical_eye failed")
	}
	if !ch.SpendTalent("talent_iron_skin") {
		t.Fatal("spend iron_skin failed")
	}
	if ch.Talents.Points != 0 {
		t.Fatalf("unspent points = %d, want 0", ch.Talents.Points)
	}
	if ch.TalentRank("talent_critical_eye") != 1 || ch.TalentRank("talent_iron_skin") != 1 {
		t.Fatalf("ranks = %+v", ch.Talents.Spent)
	}
	if ch.SpendTalent("talent_power_strike") {
		t.Fatal("spend must fail with no unspent points")
	}

	ds := DeriveStats(ch, nil)
	if ds.CritChance != 7 { // 5 base + 2
		t.Fatalf("CritChance = %v, want 7", ds.CritChance)
	}
	if ds.DamageReduction != 3 {
		t.Fatalf("DamageReduction = %v, want 3", ds.DamageReduction)
	}
}

func TestTalentRanksStackAndMapOrderIsStable(t *testing.T) {
	ch := talentTestChar("warrior")
	ch.Talents.Points = 4
	ch.Talents.Spent = map[string]int32{
		"talent_critical_eye": 2,
		"talent_endure":       1,
		"talent_wind_step":    1,
	}
	// Derive repeatedly: map iteration order must not change additive results.
	for i := 0; i < 5; i++ {
		ds := DeriveStats(ch, nil)
		if ds.CritChance != 9 { // 5 + 2*2
			t.Fatalf("CritChance = %v, want 9", ds.CritChance)
		}
		if ds.MaxHPBonusPercent != 5 {
			t.Fatalf("MaxHPBonusPercent = %v, want 5", ds.MaxHPBonusPercent)
		}
		if ds.Initiative != 3 { // level 5 -> level/2 = 2, +1 wind step
			t.Fatalf("Initiative = %d, want 3", ds.Initiative)
		}
	}
}

func TestApplyPrestigeResetsAndGrantsBonus(t *testing.T) {
	ch := talentTestChar("warrior")
	ch.Level = 50
	ch.XP = 30000
	ch.MaxHitPoints = 400
	ch.CurrentHitPoints = 400
	ch.UnspentAttributePoints = 12
	ch.SpentAttributePoints = map[string]int32{"STR": 10}
	ch.Talents.Points = 1
	ch.Talents.Spent = map[string]int32{"talent_critical_eye": 2}
	ch.Attributes = Attributes{
		NewAttribute("Strength", "STR", 30),
		NewAttribute("Dexterity", "DEX", 20),
		NewAttribute("Intelligence", "INT", 4),
		NewAttribute("Wisdom", "WIS", 5),
		NewAttribute("Stamina", "STA", 30),
	}

	ch.ApplyPrestige()

	if ch.Level != 1 {
		t.Fatalf("Level = %d, want 1", ch.Level)
	}
	if ch.XP != 0 {
		t.Fatalf("XP = %d, want 0", ch.XP)
	}
	if ch.Talents.Prestige != 1 {
		t.Fatalf("Prestige = %d, want 1", ch.Talents.Prestige)
	}
	if ch.Talents.Points != 2 { // 1 carried + 1 prestige
		t.Fatalf("Points = %d, want 2", ch.Talents.Points)
	}
	if ch.TalentRank("talent_critical_eye") != 2 {
		t.Fatal("spent talents must persist through prestige")
	}
	if ch.PrestigeMultiplier() != 1.05 {
		t.Fatalf("PrestigeMultiplier = %v, want 1.05", ch.PrestigeMultiplier())
	}
	if ch.UnspentAttributePoints != 0 || ch.SpentAttributePoints != nil {
		t.Fatal("attribute points must reset")
	}
	// Attributes return to the class template base (warrior STR 14).
	if ch.GetAttribute("STR") != 14 {
		t.Fatalf("STR = %d, want 14", ch.GetAttribute("STR"))
	}
	wantHP := balance.ScaleClassHP("warrior", classHPBase)
	if ch.MaxHitPoints != wantHP || ch.CurrentHitPoints != wantHP {
		t.Fatalf("HP = %d/%d, want %d", ch.CurrentHitPoints, ch.MaxHitPoints, wantHP)
	}
}
