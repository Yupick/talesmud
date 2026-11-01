package combat_test

import (
	"testing"

	"github.com/talesmud/talesmud/pkg/mudserver/game/combat"
	"github.com/talesmud/talesmud/pkg/mudserver/game/combat/simutil"
)

// TestCreateCombatantMirrorsStoredPools is the Phase 2.3 no-regression guard.
//
// Before 2.3 the combatant snapshot copied Character.MaxHitPoints/MaxMana
// directly. The derivation path now feeds those same slots from
// characters.DeriveStats, which mirrors the stored pools. For each of the six
// starter classes and several levels the combatant must still carry the exact
// stored values; any drift would silently rebalance every fight.
func TestCreateCombatantMirrorsStoredPools(t *testing.T) {
	e := combat.NewEngine(combat.NewManager(), nil)

	for _, cfg := range simutil.AllClassConfigs() {
		for _, level := range []int32{1, 3, 5, 8} {
			char := simutil.CreateCharacter(cfg, level)
			ref := e.CreateCombatantFromCharacter(char)

			if ref.MaxHP != char.MaxHitPoints {
				t.Fatalf("%s L%d: MaxHP=%d, stored=%d", cfg.Name, level, ref.MaxHP, char.MaxHitPoints)
			}
			if ref.MaxMana != char.MaxMana {
				t.Fatalf("%s L%d: MaxMana=%d, stored=%d", cfg.Name, level, ref.MaxMana, char.MaxMana)
			}
			if ref.CurrentHP != char.CurrentHitPoints {
				t.Fatalf("%s L%d: CurrentHP=%d, stored=%d", cfg.Name, level, ref.CurrentHP, char.CurrentHitPoints)
			}
			if ref.CurrentMana != char.CurrentMana {
				t.Fatalf("%s L%d: CurrentMana=%d, stored=%d", cfg.Name, level, ref.CurrentMana, char.CurrentMana)
			}
		}
	}
}
