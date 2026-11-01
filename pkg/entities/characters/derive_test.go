package characters

import (
	"testing"

	"github.com/talesmud/talesmud/pkg/classkit"
)

// deriveTestDef builds a small, explicit class kit so the tests do not depend
// on whatever catalog happens to be loaded in the test binary.
func deriveTestDef(id, primary string, swings int) *classkit.Def {
	return &classkit.Def{
		ID:      id,
		Name:    id,
		Primary: primary,
		Balance: classkit.Row{Swings: swings, DamageDealt: 1},
	}
}

// deriveTestChar builds a character with every attribute needed to exercise the
// formulas. CON is included here (unlike createBaseAttributes) so Resist has a
// real contribution to assert.
func deriveTestChar(classID string, level, str, dex, wis, con, sta, maxHP, maxMana int32) *Character {
	return &Character{
		Class:        Class{ID: classID},
		Level:        level,
		MaxHitPoints: maxHP,
		MaxMana:      maxMana,
		Attributes: Attributes{
			NewAttribute("Strength", "STR", str),
			NewAttribute("Dexterity", "DEX", dex),
			NewAttribute("Constitution", "CON", con),
			NewAttribute("Wisdom", "WIS", wis),
			NewAttribute("Stamina", "STA", sta),
		},
	}
}

func TestDeriveStatsFormulas(t *testing.T) {
	// DEX 18 -> +4; WIS 6 -> -2; CON 12 -> +1; STA 14.
	ch := deriveTestChar("rogue", 4, 10, 18, 6, 12, 14, 19, 0)
	def := deriveTestDef("rogue", "DEX", 2)

	got := DeriveStats(ch, def)

	if got.MaxHP != 19 {
		t.Errorf("MaxHP = %d, want 19", got.MaxHP)
	}
	if got.MaxMana != 0 {
		t.Errorf("MaxMana = %d, want 0", got.MaxMana)
	}
	// STA 14 + 2*4 = 22.
	if got.Stamina != 22 {
		t.Errorf("Stamina = %d, want 22", got.Stamina)
	}
	// DEXMod 4 + Level/2 (2) = 6.
	if got.Initiative != 6 {
		t.Errorf("Initiative = %d, want 6", got.Initiative)
	}
	// WISMod -2 + CONMod 1 + Level/2 2 = 1.
	if got.Resist != 1 {
		t.Errorf("Resist = %d, want 1", got.Resist)
	}
	// 5 base + DEXMod 4 + swings 2 + DEX primary 2 = 13.
	if got.CritChance != 13 {
		t.Errorf("CritChance = %v, want 13", got.CritChance)
	}
}

// TestDeriveStatsKeepsStoredPools is the no-regression invariant: for a case of
// each of the six base classes, the derived MaxHP/MaxMana must be exactly the
// stored character values combat snapshots.
func TestDeriveStatsKeepsStoredPools(t *testing.T) {
	cases := []struct {
		id      string
		primary string
		swings  int
		hp      int32
		mana    int32
	}{
		{"warrior", "STR", 1, 26, 0},
		{"rogue", "DEX", 2, 21, 0},
		{"mage", "INT", 1, 37, 42},
		{"cleric", "WIS", 1, 37, 46},
		{"druid", "INT", 1, 27, 42},
		{"sorcerer", "INT", 1, 27, 42},
	}
	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			ch := deriveTestChar(tc.id, 5, 10, 14, 14, 12, 12, tc.hp, tc.mana)
			def := deriveTestDef(tc.id, tc.primary, tc.swings)
			got := DeriveStats(ch, def)
			if got.MaxHP != ch.MaxHitPoints {
				t.Fatalf("MaxHP = %d, stored = %d", got.MaxHP, ch.MaxHitPoints)
			}
			if got.MaxMana != ch.MaxMana {
				t.Fatalf("MaxMana = %d, stored = %d", got.MaxMana, ch.MaxMana)
			}
		})
	}
}

// TestDeriveStatsMonotonicWithLevel raises the level (and, as leveling does,
// the stored pools) and asserts no derived value ever drops.
func TestDeriveStatsMonotonicWithLevel(t *testing.T) {
	defs := []*classkit.Def{
		deriveTestDef("warrior", "STR", 1),
		deriveTestDef("rogue", "DEX", 2),
		deriveTestDef("mage", "INT", 1),
		deriveTestDef("cleric", "WIS", 1),
		deriveTestDef("druid", "INT", 1),
		deriveTestDef("sorcerer", "INT", 1),
	}
	for _, def := range defs {
		var prev DerivedStats
		for level := int32(1); level <= 10; level++ {
			ch := deriveTestChar(def.ID, level, 10, 14, 14, 12, 12, 20+level, 10+level)
			got := DeriveStats(ch, def)
			if level > 1 {
				if got.MaxHP < prev.MaxHP ||
					got.MaxMana < prev.MaxMana ||
					got.Stamina < prev.Stamina ||
					got.Initiative < prev.Initiative ||
					got.Resist < prev.Resist ||
					got.CritChance < prev.CritChance {
					t.Fatalf("%s level %d regressed: %+v -> %+v", def.ID, level, prev, got)
				}
			}
			prev = got
		}
	}
}

func TestDeriveStatsNilSafe(t *testing.T) {
	if got := DeriveStats(nil, nil); got != (DerivedStats{}) {
		t.Fatalf("nil character = %+v, want zero", got)
	}

	// An unknown class resolves to a nil kit and adds no crit bonus.
	ch := deriveTestChar("no-such-class", 1, 10, 10, 10, 10, 10, 5, 0)
	got := DeriveStats(ch, nil)
	if got.MaxHP != 5 {
		t.Errorf("MaxHP = %d, want 5", got.MaxHP)
	}
	if got.Stamina != 12 {
		t.Errorf("Stamina = %d, want 12", got.Stamina)
	}
	if got.Initiative != 0 {
		t.Errorf("Initiative = %d, want 0", got.Initiative)
	}
	if got.Resist != 0 {
		t.Errorf("Resist = %d, want 0", got.Resist)
	}
	if got.CritChance != 5 {
		t.Errorf("CritChance = %v, want 5", got.CritChance)
	}
}
