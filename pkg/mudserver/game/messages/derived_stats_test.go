package messages

import (
	"testing"

	"github.com/talesmud/talesmud/pkg/entities"
	"github.com/talesmud/talesmud/pkg/entities/characters"
)

// TestCharacterUpdateCarriesDerivedStats pins the Phase 2.3 client payload:
// maxHitPoints/maxMana must stay the stored pools (verbatim) while the new
// stamina/initiative/resist/critChance projections follow DeriveStats.
func TestCharacterUpdateCarriesDerivedStats(t *testing.T) {
	ch := &characters.Character{
		Entity:           &entities.Entity{ID: "c1"},
		Name:             "Hero",
		Class:            characters.ClassWarrior,
		Level:            4,
		MaxHitPoints:     33,
		CurrentHitPoints: 20,
		MaxMana:          7,
		CurrentMana:      3,
		Attributes: characters.Attributes{
			characters.NewAttribute("Strength", "STR", 14),
			characters.NewAttribute("Dexterity", "DEX", 16),
			characters.NewAttribute("Constitution", "CON", 12),
			characters.NewAttribute("Wisdom", "WIS", 14),
			characters.NewAttribute("Stamina", "STA", 13),
		},
	}

	msg := NewCharacterUpdateMessage("u1", ch)
	if msg == nil {
		t.Fatal("nil character update")
	}

	// Stored pools are passed through, never recomputed.
	if msg.MaxHitPoints != 33 || msg.CurrentHitPoints != 20 {
		t.Fatalf("HP = %d/%d, want 20/33", msg.CurrentHitPoints, msg.MaxHitPoints)
	}
	if msg.MaxMana != 7 || msg.CurrentMana != 3 {
		t.Fatalf("Mana = %d/%d, want 3/7", msg.CurrentMana, msg.MaxMana)
	}

	// DEX 16 -> +3; WIS 14 -> +2; CON 12 -> +1; level 4 -> /2 = 2.
	if msg.Stamina != 21 { // STA 13 + 2*4
		t.Errorf("Stamina = %d, want 21", msg.Stamina)
	}
	if msg.Initiative != 5 { // DEXMod 3 + 2
		t.Errorf("Initiative = %d, want 5", msg.Initiative)
	}
	if msg.Resist != 5 { // WISMod 2 + CONMod 1 + 2
		t.Errorf("Resist = %d, want 5", msg.Resist)
	}
	// Warrior: primary STR, one swing -> no kit bonus; 5% + DEXMod 3 = 8%.
	if msg.CritChance != 8 {
		t.Errorf("CritChance = %v, want 8", msg.CritChance)
	}
}
