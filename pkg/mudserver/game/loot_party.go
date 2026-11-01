package game

import (
	"math/rand"

	"github.com/talesmud/talesmud/pkg/entities/characters"
	"github.com/talesmud/talesmud/pkg/entities/items"
	"github.com/talesmud/talesmud/pkg/mudserver/game/messages"
	"github.com/talesmud/talesmud/pkg/service"
)

// LootAssignmentContext carries context for party loot assignment.
type LootAssignmentContext struct {
	Game           *Game
	PartyID        string
	LootRule       string
	Eligible       []string // character IDs eligible for assignment
	KillerID       string
	RoomID         string
	RoundRobin     *int
}

// GetLootRuleFromParty returns party loot rule or "free" default.
func GetLootRuleFromParty(facade service.Facade, partyID string) string {
	if facade == nil || partyID == "" {
		return "free"
	}
	party, err := facade.PartiesService().GetPartyByID(partyID)
	if err != nil || party == nil {
		return "free"
	}
	if party.LootRule == "" {
		return "free"
	}
	return party.LootRule
}

func getCharacterByID(facade service.Facade, charID string) (*characters.Character, error) {
	if facade == nil || charID == "" {
		return nil, nil
	}
	return facade.CharactersService().FindByID(charID)
}

func giveItemToCharacter(game *Game, charID string, item *items.Item) (*items.Item, error) {
	if game == nil || game.Facade == nil || charID == "" || item == nil {
		return nil, nil
	}
	stored, err := game.Facade.ItemsService().Store(item)
	if err != nil {
		return nil, err
	}
	char, err := game.Facade.CharactersService().FindByID(charID)
	if err != nil || char == nil {
		return stored, nil
	}
	if err := char.Inventory.AddItem(stored); err != nil {
		return stored, err
	}
	if err := game.Facade.CharactersService().Update(char.ID, char); err != nil {
		return stored, err
	}
	if update := messages.NewCharacterUpdateMessage(char.BelongsUserID, char); update != nil && game.sendMessage != nil {
		game.sendMessage <- update
	}
	game.sendMessage <- messages.InventoryUpdateMessage{
		MessageResponse: messages.MessageResponse{
			Audience:   messages.MessageAudienceUser,
			AudienceID: char.BelongsUserID,
			Type:       messages.MessageTypeInventoryUpdate,
		},
		Inventory:     char.Inventory,
		EquippedItems: char.EquippedItems,
		Gold:          char.Gold,
	}
	return stored, nil
}

func selectLootRecipient(ctx *LootAssignmentContext) (string, string, string) {
	if ctx == nil || len(ctx.Eligible) == 0 {
		return "", "", ""
	}
	rule := ctx.LootRule
	if rule == "" {
		rule = "free"
	}
	switch rule {
	case "leader":
		party, err := ctx.Game.Facade.PartiesService().GetPartyByID(ctx.PartyID)
		if err == nil && party != nil {
			party.EnsureLeader()
			for _, e := range ctx.Eligible {
				if e == party.LeaderCharacterID {
					return e, party.LeaderCharacterID, "leader loot"
				}
			}
			return party.LeaderCharacterID, party.LeaderCharacterID, "leader loot"
		}
		if ctx.KillerID != "" {
			return ctx.KillerID, ctx.KillerID, "leader loot"
		}
		return ctx.Eligible[0], ctx.Eligible[0], "leader loot"
	case "roundrobin":
		if ctx.RoundRobin == nil {
			v := 0
			ctx.RoundRobin = &v
		}
		idx := *ctx.RoundRobin % len(ctx.Eligible)
		*ctx.RoundRobin = idx + 1
		c := ctx.Eligible[idx]
		return c, c, "round robin"
	case "needgreed":
		c := ctx.Eligible[rand.Intn(len(ctx.Eligible))]
		return c, c, "need/greed"
	default:
		return "", "", ""
	}
}

func charName(facade service.Facade, charID string) string {
	if facade == nil || charID == "" {
		return ""
	}
	ch, err := facade.CharactersService().FindByID(charID)
	if err == nil && ch != nil {
		return ch.Name
	}
	return charID
}

func partyForChar(facade service.Facade, charID string) (*struct{ ID string }, string) {
	if facade == nil || charID == "" {
		return nil, ""
	}
	party, err := facade.PartiesService().FindByCharacterID(charID)
	if err != nil || party == nil {
		return nil, ""
	}
	return &struct{ ID string }{ID: party.ID}, party.LootRule
}

func sendPartyMessage(game *Game, partyID string, text string) {
	if game == nil || partyID == "" || text == "" {
		return
	}
	party, err := game.Facade.PartiesService().GetPartyByID(partyID)
	if err != nil || party == nil {
		return
	}
	userByChar := map[string]string{}
	for _, p := range game.GetOnlinePlayers() {
		if p.CharacterID != "" {
			userByChar[p.CharacterID] = p.UserID
		}
	}
	sent := map[string]bool{}
	for _, cid := range party.Characters {
		uid := userByChar[cid]
		if uid == "" {
			ch, err := game.Facade.CharactersService().FindByID(cid)
			if err == nil && ch != nil && ch.BelongsUserID != "" {
				uid = ch.BelongsUserID
			}
		}
		if uid != "" && !sent[uid] {
			game.sendMessage <- messages.Reply(uid, text)
			sent[uid] = true
		}
	}
}
