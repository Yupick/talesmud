package leveling

import (
	"fmt"

	"github.com/talesmud/talesmud/pkg/entities/characters"
	"github.com/talesmud/talesmud/pkg/entities/skills"
	"github.com/talesmud/talesmud/pkg/ruleset"
)

const (
	MaxLevel = 50 // Hard level cap

	// TalentMilestoneInterval is the level step that awards one talent point.
	// Levels 5, 10, ... 50 therefore grant 10 points before prestige.
	TalentMilestoneInterval = 5
)

// TalentMilestonesBetween counts the milestone levels in (oldLevel, newLevel].
// It is the number of talent points a level-up jump should award. The range is
// half-open so a normal single-level step awards at most one point and calling
// it twice for the same step would award zero the second time.
func TalentMilestonesBetween(oldLevel, newLevel int32) int {
	if newLevel <= oldLevel {
		return 0
	}
	start := oldLevel + 1
	if start < 1 {
		start = 1
	}
	count := 0
	for level := start; level <= newLevel; level++ {
		if level%TalentMilestoneInterval == 0 {
			count++
		}
	}
	return count
}

// LevelUpResult contains all changes from a level-up event
type LevelUpResult struct {
	OldLevel               int32
	NewLevel               int32
	LevelsGained           int
	HPGained               int32
	ManaGained             int32
	AttributeGains         map[string]int32 // attribute short name -> gain amount
	AttributePointsGained  int32            // distributable points earned
	UnspentAttributePoints int32            // total unspent after this level-up
	TalentPointsGained     int32            // talent points earned on milestones
	UnspentTalentPoints    int32            // total unspent talent points after this level-up
	MaxHitPoints           int32            // max HP after level-up
	MaxMana                int32            // max mana after level-up
	Message                string           // Formatted level-up message for player
}

// CheckLevelUp determines if a character should level up based on their current XP
// Returns the number of levels gained and the new level
// Supports gaining multiple levels at once (for massive XP grants)
// Respects the character's MaxLevelCap if set (e.g., guest characters capped at level 5)
func CheckLevelUp(char *characters.Character) (levelsGained int, newLevel int32) {
	effectiveMax := char.GetEffectiveMaxLevel(ruleset.LevelCap())

	// Already at max level
	if char.Level >= effectiveMax {
		return 0, effectiveMax
	}

	levelsGained = 0
	currentLevel := char.Level
	currentXP := char.XP

	// Check if character has enough XP to level up (possibly multiple times)
	for currentLevel < effectiveMax {
		xpNeeded := GetXPRequired(currentLevel + 1)
		if currentXP >= xpNeeded {
			levelsGained++
			currentLevel++
		} else {
			break
		}
	}

	return levelsGained, currentLevel
}

// ApplyLevelUp applies all stat changes for leveling up
// Updates character level, HP, and attributes
// Returns a LevelUpResult with details for messaging and events
// Respects the character's MaxLevelCap if set (e.g., guest characters capped at level 5)
func ApplyLevelUp(char *characters.Character, levelsGained int) *LevelUpResult {
	if levelsGained <= 0 {
		return nil
	}

	effectiveMax := char.GetEffectiveMaxLevel(ruleset.LevelCap())

	oldLevel := char.Level
	newLevel := oldLevel + int32(levelsGained)

	// Enforce level cap (per-character or global)
	if newLevel > effectiveMax {
		newLevel = effectiveMax
		levelsGained = int(newLevel - oldLevel)
	}

	// Calculate HP gain
	hpGained := CalculateHPGain(char, levelsGained)

	// Calculate attribute gains
	attributeGains := CalculateAttributeGains(char, oldLevel, newLevel)

	// Apply level increase
	char.Level = newLevel

	// Apply HP increase
	char.MaxHitPoints += hpGained
	char.CurrentHitPoints = char.MaxHitPoints // Fully restore HP on level-up

	// Apply attribute gains
	for attrShortName, gainAmount := range attributeGains {
		applyAttributeGain(char, attrShortName, gainAmount)
	}

	// Award distributable attribute points
	pointsGained := int32(levelsGained) * int32(PointsPerLevel)
	char.UnspentAttributePoints += pointsGained

	// Recalculate and restore mana (after attributes have been updated)
	oldMaxMana := char.MaxMana
	newMaxMana := char.CalculateMaxMana()
	char.MaxMana = newMaxMana
	char.CurrentMana = newMaxMana // Fully restore mana on level-up
	manaGained := newMaxMana - oldMaxMana

	char.EquippedSkills = skills.FillHotbar(char.Class.ID, char.Level, char.EquippedSkills)

	// Award a talent point at each five-level milestone crossed. oldLevel is
	// the level before this call, so each milestone is counted exactly once.
	talentPointsGained := int32(TalentMilestonesBetween(oldLevel, newLevel))
	if talentPointsGained > 0 {
		char.Talents.Points += talentPointsGained
	}

	// Build result with formatted message
	result := &LevelUpResult{
		OldLevel:               oldLevel,
		NewLevel:               newLevel,
		LevelsGained:           levelsGained,
		HPGained:               hpGained,
		ManaGained:             manaGained,
		AttributeGains:         attributeGains,
		AttributePointsGained:  pointsGained,
		UnspentAttributePoints: char.UnspentAttributePoints,
		TalentPointsGained:     talentPointsGained,
		UnspentTalentPoints:    char.Talents.Points,
		MaxHitPoints:           char.MaxHitPoints,
		MaxMana:                char.MaxMana,
		Message:                formatLevelUpMessage(oldLevel, newLevel, levelsGained, hpGained, manaGained, attributeGains, pointsGained, talentPointsGained, char),
	}

	return result
}

// formatLevelUpMessage creates a formatted level-up notification message
func formatLevelUpMessage(oldLevel, newLevel int32, levelsGained int, hpGained int32, manaGained int32, attributeGains map[string]int32, pointsGained int32, talentPointsGained int32, char *characters.Character) string {
	// Header
	message := "╔══════════════════════════════════════════════════╗\n"
	message += "║                    LEVEL UP!                     ║\n"
	message += "╚══════════════════════════════════════════════════╝\n\n"

	// Level announcement
	if levelsGained == 1 {
		message += fmt.Sprintf("You have reached Level %d!\n\n", newLevel)
	} else {
		message += fmt.Sprintf("You have reached Level %d! (+%d levels)\n\n", newLevel, levelsGained)
	}

	// Stat increases
	message += "STAT INCREASES:\n"
	message += fmt.Sprintf("  + %d Max HP (now %d HP)\n", hpGained, char.MaxHitPoints)
	if manaGained > 0 {
		message += fmt.Sprintf("  + %d Max Mana (now %d Mana)\n", manaGained, char.MaxMana)
	}

	// Add attribute gains
	if len(attributeGains) > 0 {
		message += FormatAttributeGains(attributeGains, char)
	}

	// Distributable attribute points
	if pointsGained > 0 {
		message += fmt.Sprintf("  + %d Attribute Point(s) to distribute! (total unspent: %d)\n", pointsGained, char.UnspentAttributePoints)
		message += "  Use 'spend <attribute> [amount]' to allocate.\n"
	}

	// Talent milestone points
	if talentPointsGained > 0 {
		message += fmt.Sprintf("  + %d Talent Point(s) earned! (total unspent: %d)\n", talentPointsGained, char.Talents.Points)
		message += "  Use 'talents list' and 'talents spend <talent>'.\n"
	}

	// Flavor text
	message += "\n"
	if levelsGained == 1 {
		message += "You feel more powerful!\n"
	} else {
		message += "You feel significantly more powerful!\n"
	}

	message += "\n══════════════════════════════════════════════════"

	return message
}

// GetLevelProgress returns information about current level progress
func GetLevelProgress(char *characters.Character) (currentLevel int32, currentXP int32, xpForNext int32, progressPercent float64) {
	currentLevel = char.Level
	currentXP = char.XP
	xpForNext = GetXPForNextLevel(currentLevel, currentXP)
	progressPercent = GetXPProgress(currentLevel, currentXP)

	return
}

// MaybeLevelUp applies pending levels in auto mode.
// Trainer mode leaves the XP in place and returns nil.
func MaybeLevelUp(char *characters.Character) *LevelUpResult {
	if char == nil || ruleset.LevelUpMode() == ruleset.ModeTrainer {
		return nil
	}
	levelsGained, _ := CheckLevelUp(char)
	if levelsGained <= 0 {
		return nil
	}
	return ApplyLevelUp(char, levelsGained)
}

// ApplyPendingLevels applies every level the current XP can buy.
// A trainer script calls this. Auto mode is a no-op when nothing is pending.
func ApplyPendingLevels(char *characters.Character) *LevelUpResult {
	if char == nil {
		return nil
	}
	levelsGained, _ := CheckLevelUp(char)
	if levelsGained <= 0 {
		return nil
	}
	return ApplyLevelUp(char, levelsGained)
}
