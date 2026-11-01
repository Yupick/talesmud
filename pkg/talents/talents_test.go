package talents

import "testing"

func TestLookupPrefixAndCase(t *testing.T) {
	for _, id := range []string{"talent_critical_eye", "critical_eye", "CRITICAL_EYE", " talent_critical_eye "} {
		def, ok := Lookup(id)
		if !ok || def.ID != "talent_critical_eye" {
			t.Fatalf("Lookup(%q) = %+v, %v", id, def, ok)
		}
	}
	if _, ok := Lookup("talent_nope"); ok {
		t.Fatal("unknown talent must not resolve")
	}
}

func TestCatalogIsWellFormed(t *testing.T) {
	all := All()
	if len(all) != 12 {
		t.Fatalf("catalog size = %d, want 12", len(all))
	}
	seen := map[string]bool{}
	for _, d := range all {
		if d.ID == "" || d.Name == "" || d.Description == "" {
			t.Fatalf("talent needs id/name/description: %+v", d)
		}
		if len(d.Schools) == 0 {
			t.Fatalf("%s has no school", d.ID)
		}
		if d.Amount <= 0 {
			t.Fatalf("%s amount = %v", d.ID, d.Amount)
		}
		switch d.Effect {
		case EffectDamage, EffectCrit, EffectSpellDamage, EffectDamageReduction,
			EffectMaxHP, EffectBlock, EffectInitiative, EffectStaminaRegen,
			EffectSpellCooldown, EffectHeal, EffectEvasion, EffectPotion:
		default:
			t.Fatalf("%s unknown effect %q", d.ID, d.Effect)
		}
		if seen[d.ID] {
			t.Fatalf("duplicate talent %s", d.ID)
		}
		seen[d.ID] = true
	}
}

func TestAvailableBySchool(t *testing.T) {
	if !Available("martial", "talent_power_strike") {
		t.Fatal("martial should learn power_strike")
	}
	if Available("martial", "talent_shadow_step") {
		t.Fatal("martial must not learn shadow_step")
	}
	if !Available("shadow", "talent_shadow_step") {
		t.Fatal("shadow should learn shadow_step")
	}
	if Available("", "talent_power_strike") {
		t.Fatal("empty school must learn nothing")
	}
}

func TestForSchoolOrderIsStable(t *testing.T) {
	fire := IDs(ForSchool("fire"))
	want := []string{"talent_mana_surge", "talent_endure", "talent_spell_momentum"}
	if len(fire) != len(want) {
		t.Fatalf("fire talents = %v, want %v", fire, want)
	}
	for i := range want {
		if fire[i] != want[i] {
			t.Fatalf("fire talents = %v, want %v", fire, want)
		}
	}
}
