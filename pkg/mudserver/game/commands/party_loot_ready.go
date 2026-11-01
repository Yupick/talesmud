package commands

import (
	"fmt"
	"strings"

	"github.com/talesmud/talesmud/pkg/entities"
	"github.com/talesmud/talesmud/pkg/mudserver/game/def"
	"github.com/talesmud/talesmud/pkg/mudserver/game/messages"
)

func (command *PartyCommand) handleLoot(game def.GameCtrl, message *messages.Message, args []string) {
	party, err := game.GetFacade().PartiesService().FindByCharacterID(message.Character.ID)
	if err != nil || party == nil {
		game.SendMessage() <- messages.Reply(message.FromUser.ID, "You are not in a party.")
		return
	}
	party.EnsureLeader()
	if len(args) < 3 {
		if party.LootRule == "" {
			party.LootRule = "free"
		}
		game.SendMessage() <- messages.Reply(message.FromUser.ID, fmt.Sprintf("[Party] Loot rule: %s", party.LootRule))
		return
	}
	if !party.IsLeader(message.Character.ID) {
		game.SendMessage() <- messages.Reply(message.FromUser.ID, "Only the party leader can change the loot rule.")
		return
	}
	rule := strings.ToLower(strings.TrimSpace(args[2]))
	switch rule {
	case "free", "leader", "roundrobin", "needgreed", "need/greed":
		if rule == "need/greed" {
			rule = "needgreed"
		}
		party.LootRule = rule
		if err := game.GetFacade().PartiesService().UpdateParty(party.ID, party); err != nil {
			game.SendMessage() <- messages.Reply(message.FromUser.ID, "Could not update loot rule.")
			return
		}
		command.sendToParty(game, party.Characters, fmt.Sprintf("[Party] Loot rule set to %s.", party.LootRule))
		command.pushPartyToMembers(game, party)
		return
	default:
		game.SendMessage() <- messages.Reply(message.FromUser.ID, "Usage: party loot <free|leader|roundrobin|needgreed>")
		return
	}
}

func (command *PartyCommand) handleReadyCheck(game def.GameCtrl, message *messages.Message, args []string) {
	party, err := game.GetFacade().PartiesService().FindByCharacterID(message.Character.ID)
	if err != nil || party == nil {
		game.SendMessage() <- messages.Reply(message.FromUser.ID, "You are not in a party.")
		return
	}
	party.EnsureLeader()
	if !party.IsLeader(message.Character.ID) {
		game.SendMessage() <- messages.Reply(message.FromUser.ID, "Only the party leader can start a ready check.")
		return
	}
	game.StartReadyCheck(party.ID)
	command.sendToParty(game, party.Characters, "[Party] Ready check! Type 'party ready' or 'party notready'.")
}

func (command *PartyCommand) handleReady(game def.GameCtrl, message *messages.Message, ready bool) {
	party, err := game.GetFacade().PartiesService().FindByCharacterID(message.Character.ID)
	if err != nil || party == nil {
		game.SendMessage() <- messages.Reply(message.FromUser.ID, "You are not in a party.")
		return
	}
	if game.ReadyCheckExpired(party.ID) {
		readyMap, _, _ := game.GetReadyCheck(party.ID)
		command.sendReadySummary(game, party, readyMap, true)
		game.ClearReadyCheck(party.ID)
		return
	}
	changed := game.SetReady(party.ID, message.Character.ID, ready)
	readyMap, _, ok := game.GetReadyCheck(party.ID)
	if !ok {
		game.StartReadyCheck(party.ID)
		_ = game.SetReady(party.ID, message.Character.ID, ready)
		readyMap, _, _ = game.GetReadyCheck(party.ID)
	}
	if changed || len(readyMap) > 0 {
		command.sendReadyUpdate(game, party, readyMap)
	}
}

func (command *PartyCommand) sendReadyUpdate(game def.GameCtrl, party *entities.Party, readyMap map[string]bool) {
	readyCount := 0
	notReadyCount := 0
	for _, r := range readyMap {
		if r {
			readyCount++
		} else {
			notReadyCount++
		}
	}
	msg := fmt.Sprintf("[Party] Ready: %d/%d · Not ready: %d/%d", readyCount, len(party.Characters), notReadyCount, len(party.Characters))
	leaderID := party.LeaderCharacterID
	if leaderID != "" {
		if leaderUser := findUserIDForChar(game, leaderID); leaderUser != "" {
			game.SendMessage() <- messages.Reply(leaderUser, msg)
		}
	}
	for _, c := range party.Characters {
		if c == leaderID {
			continue
		}
		if userID := findUserIDForChar(game, c); userID != "" {
			game.SendMessage() <- messages.Reply(userID, msg)
		}
	}
}

func (command *PartyCommand) sendReadySummary(game def.GameCtrl, party *entities.Party, readyMap map[string]bool, expired bool) {
	if readyMap == nil {
		readyMap = make(map[string]bool)
	}
	readyCount := 0
	for _, r := range readyMap {
		if r {
			readyCount++
		}
	}
	msg := fmt.Sprintf("[Party] Ready check over: %d/%d ready.", readyCount, len(party.Characters))
	command.sendToParty(game, party.Characters, msg)
}

func findUserIDForChar(game def.GameCtrl, charID string) string {
	for _, p := range game.GetOnlinePlayers() {
		if p.CharacterID == charID {
			return p.UserID
		}
	}
	if game == nil || game.GetFacade() == nil {
		return ""
	}
	ch, err := game.GetFacade().CharactersService().FindByID(charID)
	if err != nil || ch == nil {
		return ""
	}
	if ch.BelongsUserID != "" {
		for _, p := range game.GetOnlinePlayers() {
			if p.UserID == ch.BelongsUserID {
				return p.UserID
			}
		}
	}
	return ""
}
