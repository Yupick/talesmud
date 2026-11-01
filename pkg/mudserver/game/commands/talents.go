package commands

import (
	"fmt"
	"sort"
	"strings"

	"github.com/talesmud/talesmud/pkg/classkit"
	"github.com/talesmud/talesmud/pkg/mudserver/game/def"
	"github.com/talesmud/talesmud/pkg/mudserver/game/messages"
	"github.com/talesmud/talesmud/pkg/talents"
)

// TalentsCommand lists the class talents and spends talent points.
//
//	talents                 show unspent points and the class talent list
//	talents list            same as bare `talents`
//	talents spend <talent>  buy one rank of a talent
type TalentsCommand struct{}

// Key returns the command key matcher.
func (command *TalentsCommand) Key() CommandKey { return &StartsWithCommandKey{} }

// Execute handles list and spend.
func (command *TalentsCommand) Execute(game def.GameCtrl, message *messages.Message) bool {
	if message.Character == nil {
		game.SendMessage() <- message.Reply("You need to select a character first.")
		return true
	}
	parts := strings.Fields(message.Data)
	if len(parts) < 2 || strings.EqualFold(parts[1], "list") {
		command.list(game, message)
		return true
	}
	if strings.EqualFold(parts[1], "spend") {
		command.spend(game, message, strings.Join(parts[2:], " "))
		return true
	}
	game.SendMessage() <- message.Reply("Usage: talents [list] | talents spend <talent>")
	return true
}

func (command *TalentsCommand) list(game def.GameCtrl, message *messages.Message) {
	char := message.Character
	available := command.available(char.Class.ID)

	var sb strings.Builder
	sb.WriteString("\n═══════ TALENTS ═══════\n\n")
	sb.WriteString(fmt.Sprintf("Unspent Points: %d\n", char.Talents.Points))
	if char.Talents.Prestige > 0 {
		sb.WriteString(fmt.Sprintf("Prestige: %d (+%.0f%% XP)\n",
			char.Talents.Prestige, (char.PrestigeMultiplier()-1)*100))
	}
	if len(available) == 0 {
		sb.WriteString("\nYour class has no talents.\n")
		sb.WriteString("═══════════════════════\n")
		game.SendMessage() <- message.Reply(sb.String())
		return
	}

	// Group by category so the layout is stable and readable.
	byCategory := map[talents.Category][]talents.Def{}
	for _, d := range available {
		byCategory[d.Category] = append(byCategory[d.Category], d)
	}
	for _, category := range []talents.Category{talents.Offensive, talents.Defensive, talents.Utility} {
		defs := byCategory[category]
		if len(defs) == 0 {
			continue
		}
		sb.WriteString("\n" + strings.ToUpper(string(category)) + "\n")
		for _, d := range defs {
			rank := char.TalentRank(d.ID)
			sb.WriteString(fmt.Sprintf("  %-22s %-18s rank %d  %s\n", d.ID, d.Name, rank, d.Description))
		}
	}

	sb.WriteString("\nUsage: talents spend <talent id or name>\n")
	sb.WriteString("═══════════════════════\n")
	game.SendMessage() <- message.Reply(sb.String())
}

func (command *TalentsCommand) spend(game def.GameCtrl, message *messages.Message, arg string) {
	char := message.Character
	arg = strings.TrimSpace(arg)
	if arg == "" {
		game.SendMessage() <- message.Reply("Usage: talents spend <talent>")
		return
	}

	combatEngine := game.GetCombatEngine()
	if combatEngine != nil && combatEngine.IsPlayerInCombat(char.Entity.ID) {
		game.SendMessage() <- message.Reply("You cannot spend talent points during combat.")
		return
	}

	def, ok := command.resolveTalent(char.Class.ID, arg)
	if !ok {
		game.SendMessage() <- message.Reply("No talent named '" + arg + "'. Use 'talents list'.")
		return
	}
	if !char.CanLearnTalent(def.ID) {
		game.SendMessage() <- message.Reply("Your class cannot learn " + def.Name + ".")
		return
	}
	if char.Talents.Points <= 0 {
		game.SendMessage() <- message.Reply("You have no talent points to spend. Earn one every 5 levels.")
		return
	}
	if !char.SpendTalent(def.ID) {
		game.SendMessage() <- message.Reply("Could not spend a point on " + def.Name + ".")
		return
	}

	game.GetFacade().CharactersService().Update(char.ID, char)

	rank := char.TalentRank(def.ID)
	game.SendMessage() <- message.Reply(fmt.Sprintf(
		"Learned %s (rank %d). Unspent talent points: %d", def.Name, rank, char.Talents.Points))

	if update := messages.NewCharacterUpdateMessage(message.FromUser.ID, char); update != nil {
		game.SendMessage() <- update
	}
}

// available returns the talent defs for a class in a stable order.
func (command *TalentsCommand) available(classID string) []talents.Def {
	ids := classkit.TalentsForClass(classID)
	out := make([]talents.Def, 0, len(ids))
	seen := map[string]bool{}
	for _, id := range ids {
		def, ok := talents.Lookup(id)
		if !ok || seen[def.ID] {
			continue
		}
		seen[def.ID] = true
		out = append(out, def)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Category != out[j].Category {
			return out[i].Category < out[j].Category
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// resolveTalent matches an argument by id (with or without the talent_ prefix),
// by display name, or by a unique prefix of either.
func (command *TalentsCommand) resolveTalent(classID, arg string) (talents.Def, bool) {
	if def, ok := talents.Lookup(arg); ok {
		return def, true
	}
	needle := strings.ToLower(strings.TrimSpace(arg))
	needle = strings.TrimPrefix(needle, "talent_")
	var matches []talents.Def
	for _, d := range command.available(classID) {
		id := strings.TrimPrefix(strings.ToLower(d.ID), "talent_")
		name := strings.ToLower(d.Name)
		if id == needle || name == needle ||
			strings.HasPrefix(id, needle) || strings.HasPrefix(name, needle) {
			matches = append(matches, d)
		}
	}
	if len(matches) == 1 {
		return matches[0], true
	}
	return talents.Def{}, false
}
