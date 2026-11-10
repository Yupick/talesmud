package classkit

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func restorePackCatalog(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		dir := findUp("pkg/classkit/testdata/classes")
		if dir == "" {
			return
		}
		if _, err := LoadDir(dir); err != nil {
			t.Errorf("restore testdata pack: %v", err)
		}
	})
}

func persistTestDef(id, name string) *Def {
	return &Def{
		ID:           id,
		Name:         name,
		Description:  name + " description",
		ArmorType:    "Cloth",
		CombatType:   "Magic",
		School:       "arcane",
		Primary:      "INT",
		SkillClass:   "",
		Aliases:      []string{strings.ToLower(name)},
		Races:        []string{"human", "elf"},
		HPMultiplier: 0.9,
		Caster:       true,
		Hotbar:       4,
		Order:        42,
		Balance: Row{
			DamageDealt: 1.1, DamageTaken: 1.2, BehindDealt: 1.3, Swings: 1,
		},
		Skills: []SkillSpec{{
			ID: id + "_sear", Name: "Sear", Effect: "sear", Level: 2,
			Cooldown: 4, Multiplier: 1.4, Tooltip: "burn", Target: "enemy",
		}},
		Template: &Template{
			ID:        "tpl-" + id,
			Backstory: "from a test",
			Origin:    "Town",
			Archetype: id,
			Race:      "human",
			Str:       4, Dex: 6, Int: 18, Wis: 14, Sta: 8, Mana: 20,
			Items:         []StartItem{{Slot: "main_hand", Name: "Apprentice Staff"}},
			DefaultSkills: []string{id + "_sear"},
		},
	}
}

func TestSaveDefRoundTrip(t *testing.T) {
	restorePackCatalog(t)
	dir := t.TempDir()

	d := persistTestDef("Spark", "Spark")
	if err := SaveDef(dir, d); err != nil {
		t.Fatalf("save: %v", err)
	}
	if got := ActiveDir(); got != dir {
		t.Fatalf("active dir %q, want %q", got, dir)
	}
	if _, err := os.Stat(filepath.Join(dir, "spark.yaml")); err != nil {
		t.Fatalf("expected spark.yaml: %v", err)
	}

	got := FindDef("spark")
	if got == nil {
		t.Fatal("saved class not in catalog")
	}
	if got.ID != "spark" || got.Name != "Spark" || got.School != "arcane" {
		t.Fatalf("unexpected class: %+v", got)
	}
	if got.Balance.DamageDealt != 1.1 || got.Hotbar != 4 {
		t.Fatalf("balance/hotbar not persisted: %+v", got)
	}
	if got.Template == nil || got.Template.Str != 4 || got.Template.Mana != 20 {
		t.Fatalf("template not persisted: %+v", got.Template)
	}
	if len(got.Skills) != 1 || got.Skills[0].Effect != "sear" {
		t.Fatalf("skills not persisted: %+v", got.Skills)
	}

	// A second save updates the same file instead of adding a duplicate.
	d.Name = "Spark II"
	if err := SaveDef(dir, d); err != nil {
		t.Fatalf("update: %v", err)
	}
	if got := FindDef("spark"); got == nil || got.Name != "Spark II" {
		t.Fatalf("update did not reload: %+v", got)
	}

	if err := DeleteDef(dir, "spark"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if got := FindDef("spark"); got != nil {
		t.Fatalf("class still present after delete: %+v", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "spark.yaml")); !os.IsNotExist(err) {
		t.Fatalf("class file still present: %v", err)
	}
	if err := DeleteDef(dir, "spark"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestSaveDefValidation(t *testing.T) {
	restorePackCatalog(t)
	dir := t.TempDir()

	badEffect := persistTestDef("bad_effect", "Bad Effect")
	badEffect.Skills[0].Effect = "not_a_known_effect"

	unsafeID := persistTestDef("../escape", "Escape")

	cases := []struct {
		name string
		def  *Def
	}{
		{name: "empty id", def: persistTestDef("", "No Id")},
		{name: "blank name", def: persistTestDef("no_name", "  ")},
		{name: "unknown effect", def: badEffect},
		{name: "unsafe id", def: unsafeID},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			err := SaveDef(dir, tt.def)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if !IsValidationError(err) {
				t.Fatalf("expected ValidationError, got %T %v", err, err)
			}
		})
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read temp dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("validation left files behind: %v", entries)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "escape.yaml")); !os.IsNotExist(err) {
		t.Fatalf("unsafe id escaped the directory: %v", err)
	}
}

func TestSaveDefRollsBackFailedReload(t *testing.T) {
	restorePackCatalog(t)
	dir := t.TempDir()

	// An invalid sibling makes the post-write LoadDir fail.
	if err := os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte("id: \"\"\nname: Bad\n"), 0o644); err != nil {
		t.Fatalf("write bad file: %v", err)
	}
	if err := SaveDef(dir, persistTestDef("ok", "OK")); err == nil {
		t.Fatal("expected reload failure")
	}
	if _, err := os.Stat(filepath.Join(dir, "ok.yaml")); !os.IsNotExist(err) {
		t.Fatalf("failed save must not leave ok.yaml: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read temp dir: %v", err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp") {
			t.Fatalf("failed save left a temp file: %s", entry.Name())
		}
	}
}

func TestSaveDefDuplicateClass(t *testing.T) {
	restorePackCatalog(t)
	dir := t.TempDir()

	source := persistTestDef("source", "Source")
	if err := SaveDef(dir, source); err != nil {
		t.Fatalf("save source: %v", err)
	}
	copyDef := FindDef("source")
	if copyDef == nil {
		t.Fatal("source missing")
	}
	copyDef.ID = "source_copy"
	copyDef.Name = "Source (copy)"
	if err := SaveDef(dir, copyDef); err != nil {
		t.Fatalf("save copy: %v", err)
	}
	if got := FindDef("source_copy"); got == nil || got.Name != "Source (copy)" {
		t.Fatalf("copy not loaded: %+v", got)
	}
	if len(All()) != 2 {
		t.Fatalf("catalog count %d, want 2", len(All()))
	}
	if len(Playable()) != 2 {
		t.Fatalf("playable count %d, want 2", len(Playable()))
	}
}
