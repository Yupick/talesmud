package classkit

import (
	"path/filepath"
	"testing"
)

// TestWorldPackDataClasses loads the shipped fork content pack at
// data/classes. It fails if any YAML does not validate (unknown effect,
// unknown school, duplicate id) and checks that all six base classes load.
func TestWorldPackDataClasses(t *testing.T) {
	t.Cleanup(func() { restoreTestPack(t) })

	dir := filepath.Join("..", "..", "data", "classes")
	n, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir(%s): %v", dir, err)
	}
	if n < 6 {
		t.Fatalf("world pack classes = %d, want >= 6", n)
	}
	if !FromPack() {
		t.Fatal("world pack did not install as a pack")
	}

	want := map[string]string{
		"warrior":  "martial",
		"rogue":    "martial",
		"mage":     "arcane",
		"cleric":   "divine",
		"druid":    "nature",
		"sorcerer": "fire",
	}
	for id, school := range want {
		d := Lookup(id)
		if d == nil {
			t.Fatalf("class %q missing from world pack", id)
		}
		if d.School != school {
			t.Fatalf("class %q school = %q, want %q", id, d.School, school)
		}
		if SchoolName(d.School) == "" {
			t.Fatalf("class %q has unregistered school %q", id, d.School)
		}
		if d.Template == nil || d.Template.ID == "" {
			t.Fatalf("class %q has no create template", id)
		}
		requireStartingItems(t, d)
	}

	// Skill ids must stay unique across classes because they feed the hotbar.
	seen := map[string]string{}
	for _, d := range All() {
		for _, s := range d.Skills {
			if prev, ok := seen[s.ID]; ok {
				t.Fatalf("skill id %q used by %s and %s", s.ID, prev, d.ID)
			}
			seen[s.ID] = d.ID
		}
	}
}

// TestWorldPackManaAndComponents pins the mana_cost and per-school component
// carried by the shipped pack and guards the Batch 6 heal multipliers from
// drifting when mana is added.
func TestWorldPackManaAndComponents(t *testing.T) {
	t.Cleanup(func() { restoreTestPack(t) })

	dir := filepath.Join("..", "..", "data", "classes")
	if _, err := LoadDir(dir); err != nil {
		t.Fatalf("LoadDir(%s): %v", dir, err)
	}

	want := map[string]struct {
		mana int
		comp string
		mult float64
	}{
		"cleric_guard":      {8, "divine_wax", 0},
		"cleric_mend":       {10, "divine_wax", 0.30},
		"cleric_glyph":      {8, "divine_wax", 0},
		"cleric_slam":       {6, "divine_wax", 1.30},
		"cleric_renew":      {12, "divine_wax", 0.12},
		"cleric_bolt":       {10, "divine_wax", 0},
		"druid_inscribe":    {6, "nature_herb", 0},
		"druid_glyph":       {8, "nature_herb", 0},
		"druid_renew":       {10, "nature_herb", 0.10},
		"druid_sear":        {12, "nature_herb", 1.60},
		"druid_mend":        {12, "nature_herb", 0.25},
		"mage_inscribe":     {0, "arcane_dust", 0},
		"mage_sear":         {16, "arcane_dust", 1.80},
		"mage_mend":         {16, "arcane_dust", 0.20},
		"mage_glyph":        {12, "arcane_dust", 0},
		"sorcerer_bolt":     {10, "fire_salt", 0},
		"sorcerer_sear":     {14, "fire_salt", 2.00},
		"sorcerer_rig":      {10, "fire_salt", 0.50},
		"sorcerer_overload": {6, "fire_salt", 0.80},
		"sorcerer_mend":     {12, "fire_salt", 0.20},
		"warrior_slam":      {0, "", 1.40},
		"rogue_nick":        {0, "", 0.55},
	}

	specs := map[string]SkillSpec{}
	for _, d := range All() {
		for _, s := range d.Skills {
			specs[s.ID] = s
		}
	}
	for id, exp := range want {
		s, ok := specs[id]
		if !ok {
			t.Fatalf("skill %s missing from pack", id)
		}
		if s.ManaCost != exp.mana || s.Component != exp.comp {
			t.Fatalf("%s mana=%d component=%q, want %d/%q", id, s.ManaCost, s.Component, exp.mana, exp.comp)
		}
		if s.Multiplier != exp.mult {
			t.Fatalf("%s multiplier=%v, want %v (Batch 6 numbers must not drift)", id, s.Multiplier, exp.mult)
		}
	}
}
