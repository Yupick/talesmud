package classkit

import (
	"strings"
	"testing"
)

func TestLoadDirExplicitTalents(t *testing.T) {
	t.Cleanup(func() { restoreTestPack(t) })
	dir := writeClassYAML(t, `
id: bladedancer
name: Blade Dancer
school: martial
talents:
  - talent_power_strike
  - talent_critical_eye
`)
	if _, err := LoadDir(dir); err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	got := TalentsForClass("bladedancer")
	want := []string{"talent_power_strike", "talent_critical_eye"}
	if len(got) != len(want) {
		t.Fatalf("talents = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("talents = %v, want %v", got, want)
		}
	}
}

func TestTalentsForClassFallsBackToSchool(t *testing.T) {
	t.Cleanup(func() { restoreTestPack(t) })
	dir := writeClassYAML(t, `
id: pyromancer
name: Pyromancer
school: fire
`)
	if _, err := LoadDir(dir); err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	got := TalentsForClass("pyromancer")
	joined := strings.Join(got, ",")
	for _, want := range []string{"talent_mana_surge", "talent_endure", "talent_spell_momentum"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("fire talents = %v, missing %s", got, want)
		}
	}
	if strings.Contains(joined, "talent_power_strike") {
		t.Fatalf("fire talents = %v, must not include martial-only power_strike", got)
	}
}

func TestLoadDirUnknownTalentRejected(t *testing.T) {
	t.Cleanup(func() { restoreTestPack(t) })
	dir := writeClassYAML(t, `
id: badkit
name: Bad Kit
talents:
  - talent_not_real
`)
	if _, err := LoadDir(dir); err == nil {
		t.Fatal("unknown talent id must fail the load")
	} else if !strings.Contains(err.Error(), "unknown talent") {
		t.Fatalf("err = %v", err)
	}
	if Lookup("badkit") != nil {
		t.Fatal("a failed load must not install a class")
	}
}
