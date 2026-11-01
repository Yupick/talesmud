package commands

import (
	"fmt"
	"strings"

	"github.com/talesmud/talesmud/pkg/mudserver/game/def"
	"github.com/talesmud/talesmud/pkg/mudserver/game/leveling"
	"github.com/talesmud/talesmud/pkg/mudserver/game/messages"
)

// PrestigeCommand resets a maxed character to level 1 in exchange for a
// permanent +5% XP bonus and one talent point per prestige.
//
//	prestige          show whether prestige is available
//	prestige confirm  perform the irreversible reset
type PrestigeCommand struct{}

// Key returns the command key matcher.
func (command *PrestigeCommand) Key() CommandKey { return &StartsWithCommandKey{} }

// Execute handles the status and confirm flows.
func (command *PrestigeCommand) Execute(game def.GameCtrl, message *messages.Message) bool {
	if message.Character == nil {
		game.SendMessage() <- message.Reply("You need to select a character first.")
		return true
	}
	parts := strings.Fields(message.Data)
	if len(parts) >= 2 && strings.EqualFold(parts[1], "confirm") {
		command.confirm(game, message)
		return true
	}
	command.status(game, message)
	return true
}

func (command *PrestigeCommand) status(game def.GameCtrl, message *messages.Message) {
	char := message.Character
	var sb strings.Builder
	sb.WriteString("\n═══════ PRESTIGE ═══════\n\n")
	sb.WriteString(fmt.Sprintf("Current Prestige: %d (+%.0f%% XP)\n",
		char.Talents.Prestige, (char.PrestigeMultiplier()-1)*100))
	sb.WriteString(fmt.Sprintf("Level: %d / %d\n\n", char.Level, leveling.MaxLevel))

	if char.Level < leveling.MaxLevel {
		sb.WriteString(fmt.Sprintf("You must reach level %d before you can prestige.\n", leveling.MaxLevel))
	} else {
		sb.WriteString("Prestige is available!\n")
		sb.WriteString("It will reset you to level 1 with class base attributes, and grant:\n")
		sb.WriteString("  +5% XP per prestige level (permanent)\n")
		sb.WriteString("  +1 talent point (permanent)\n\n")
		sb.WriteString("Talents, gold, gear, discoveries, and friends are kept. This is irreversible.\n")
		sb.WriteString("Type 'prestige confirm' to proceed.\n")
	}
	sb.WriteString("═══════════════════════\n")
	game.SendMessage() <- message.Reply(sb.String())
}

func (command *PrestigeCommand) confirm(game def.GameCtrl, message *messages.Message) {
	char := message.Character

	if char.Level < leveling.MaxLevel {
		game.SendMessage() <- message.Reply(fmt.Sprintf(
			"You must reach level %d before you can prestige.", leveling.MaxLevel))
		return
	}

	combatEngine := game.GetCombatEngine()
	if combatEngine != nil && combatEngine.IsPlayerInCombat(char.Entity.ID) {
		game.SendMessage() <- message.Reply("You cannot prestige during combat.")
		return
	}

	char.ApplyPrestige()
	if err := game.GetFacade().CharactersService().Update(char.ID, char); err != nil {
		game.SendMessage() <- message.Reply("Could not save your prestige. Try again.")
		return
	}

	game.SendMessage() <- message.Reply(fmt.Sprintf(
		"You have ascended to Prestige %d! You are level 1 again with %d talent point(s) and +%.0f%% XP.",
		char.Talents.Prestige, char.Talents.Points, (char.PrestigeMultiplier()-1)*100))

	if update := messages.NewCharacterUpdateMessage(message.FromUser.ID, char); update != nil {
		game.SendMessage() <- update
	}
}
