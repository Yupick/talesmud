package leveling

import (
	"testing"

	"github.com/talesmud/talesmud/pkg/entities/characters"
)

func TestTalentMilestonesBetween(t *testing.T) {
	cases := []struct {
		old, new int32
		want     int
	}{
		{1, 1, 0},
		{4, 5, 1},
		{1, 5, 1},
		{1, 10, 2},
		{5, 10, 1},
		{4, 11, 2},
		{10, 10, 0},
		{50, 50, 0},
		{1, 0, 0},
		{0, 5, 1},
	}
	for _, tc := range cases {
		if got := TalentMilestonesBetween(tc.old, tc.new); got != tc.want {
			t.Errorf("TalentMilestonesBetween(%d, %d) = %d, want %d", tc.old, tc.new, got, tc.want)
		}
	}
}

// TestApplyLevelUpAwardsTalentPoints checks the milestone count on a multi-level
// jump and that re-checking the same state awards nothing more.
func TestApplyLevelUpAwardsTalentPoints(t *testing.T) {
	char := createTestCharacter(characters.ClassWarrior, 1)
	char.XP = GetXPRequired(10)

	levelsGained, _ := CheckLevelUp(char)
	if levelsGained != 9 {
		t.Fatalf("levelsGained = %d, want 9", levelsGained)
	}
	result := ApplyLevelUp(char, levelsGained)
	if result == nil {
		t.Fatal("ApplyLevelUp returned nil")
	}
	if char.Talents.Points != 2 {
		t.Fatalf("talent points = %d, want 2 (levels 5 and 10)", char.Talents.Points)
	}
	if result.TalentPointsGained != 2 || result.UnspentTalentPoints != 2 {
		t.Fatalf("result talents = %d/%d, want 2/2", result.TalentPointsGained, result.UnspentTalentPoints)
	}
	if char.Talents.Spent != nil {
		t.Fatal("a level-up must not create a spent map")
	}

	// No new XP: nothing pending, no duplicate points.
	again, _ := CheckLevelUp(char)
	if again != 0 {
		t.Fatalf("CheckLevelUp = %d, want 0", again)
	}
	if char.Talents.Points != 2 {
		t.Fatalf("talent points duplicated: %d", char.Talents.Points)
	}
}

// TestTalentPointsOnlyOnMilestones walks one level at a time and checks that
// only levels divisible by five pay out.
func TestTalentPointsOnlyOnMilestones(t *testing.T) {
	char := createTestCharacter(characters.ClassWarrior, 1)
	for level := int32(2); level <= 12; level++ {
		char.XP = GetXPRequired(level)
		levelsGained, _ := CheckLevelUp(char)
		if levelsGained != 1 {
			t.Fatalf("step to %d: levelsGained = %d", level, levelsGained)
		}
		ApplyLevelUp(char, levelsGained)
		want := int32(level / TalentMilestoneInterval) // 5 -> 1, 10 -> 2
		if char.Talents.Points != want {
			t.Fatalf("after level %d: talent points = %d, want %d", level, char.Talents.Points, want)
		}
	}
}
