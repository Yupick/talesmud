package commands

import (
	"strings"

	log "github.com/sirupsen/logrus"

	npc "github.com/talesmud/talesmud/pkg/entities/npcs"
	"github.com/talesmud/talesmud/pkg/mudserver/game/def"
	"github.com/talesmud/talesmud/pkg/mudserver/game/messages"
)

// OfferQuestCommand asks a quest-giver NPC for a dynamically generated
// contextual quest (Phase 6.4). It also handles "offer accept" and
// "offer decline" for the pending offer. It never generates the quest itself:
// the game delivers it asynchronously and stores it only on acceptance.
type OfferQuestCommand struct{}

// Key returns the command key matcher.
func (cmd *OfferQuestCommand) Key() CommandKey { return &StartsWithCommandKey{} }

// Execute handles the offer command.
func (cmd *OfferQuestCommand) Execute(game def.GameCtrl, message *messages.Message) bool {
	if message.Character == nil {
		game.SendMessage() <- message.Reply("You need to select a character first.")
		return true
	}
	if message.Character.CurrentRoomID == "" {
		game.SendMessage() <- message.Reply("You are not in a room.")
		return true
	}

	parts := strings.Fields(message.Data)
	if len(parts) >= 2 {
		switch strings.ToLower(parts[1]) {
		case "accept":
			return cmd.accept(game, message)
		case "decline", "no", "reject", "pass":
			game.ClearPendingContextualQuest(message.Character.ID)
			game.SendMessage() <- message.Reply("You let the offer pass.")
			return true
		}
	}

	return cmd.request(game, message, parts)
}

// request resolves the NPC from the room and schedules a contextual generation.
func (cmd *OfferQuestCommand) request(game def.GameCtrl, message *messages.Message, parts []string) bool {
	if len(parts) < 2 {
		game.SendMessage() <- message.Reply("Ask whom for work? Usage: offer <npc-name>")
		return true
	}

	npcName := strings.Join(parts[1:], " ")
	if strings.HasPrefix(strings.ToLower(npcName), "to ") {
		npcName = strings.TrimSpace(npcName[3:])
	}

	npcManager := game.GetNPCInstanceManager()
	if npcManager == nil {
		game.SendMessage() <- message.Reply("Error: NPC system not available.")
		return true
	}
	npc := npcManager.FindInstanceByNameInRoom(message.Character.CurrentRoomID, npcName)
	if npc == nil {
		game.SendMessage() <- message.Reply("There is no one named '" + npcName + "' here.")
		return true
	}

	if !npcOffersContextualQuests(game, npc) {
		game.SendMessage() <- message.Reply(npc.Name + " has no work to offer you.")
		return true
	}

	room, err := game.GetFacade().RoomsService().FindByID(message.Character.CurrentRoomID)
	if err != nil || room == nil {
		game.SendMessage() <- message.Reply("You cannot make out where you are.")
		return true
	}
	zoneID := strings.TrimSpace(room.Area)
	if zoneID == "" {
		zoneID = room.ID
	}

	level := int(message.Character.Level)
	if level < 1 {
		game.SendMessage() <- message.Reply(npc.Name + " has nothing suitable for a novice like you.")
		return true
	}

	sourceNPCID := strings.TrimSpace(npc.TemplateID)
	if sourceNPCID == "" {
		sourceNPCID = npc.ID
	}
	userID := ""
	if message.FromUser != nil {
		userID = message.FromUser.ID
	}

	req := def.ContextualQuestRequest{
		CharacterID:   message.Character.ID,
		UserID:        userID,
		PlayerLevel:   level,
		ZoneID:        zoneID,
		SourceNPCID:   sourceNPCID,
		SourceNPCName: npc.Name,
	}

	switch game.RequestContextualQuestOffer(req) {
	case def.QuestOfferDisabled:
		game.SendMessage() <- message.Reply(npc.Name + " has nothing planned for you right now.")
	case def.QuestOfferRateLimited:
		game.SendMessage() <- message.Reply(npc.Name + " is still considering your last request. Try again shortly.")
	case def.QuestOfferBusy:
		game.SendMessage() <- message.Reply(npc.Name + " is busy with another request. Try again shortly.")
	default:
		game.SendMessage() <- message.Reply(npc.Name + " considers what you might do for them...")
	}
	return true
}

// accept stores and accepts the pending contextual quest.
func (cmd *OfferQuestCommand) accept(game def.GameCtrl, message *messages.Message) bool {
	char := message.Character
	pending, ok := game.PendingContextualQuest(char.ID)
	if !ok || pending == nil {
		game.SendMessage() <- message.Reply("You have no quest offer to accept.")
		return true
	}

	svc := game.GetFacade().QuestsService()
	stored, err := svc.Store(pending)
	if err != nil {
		game.SendMessage() <- message.Reply("The offer could not be taken: " + err.Error())
		return true
	}
	quest := stored
	if quest == nil {
		quest = pending
	}

	if _, err := svc.AcceptQuest(char.ID, quest.ID); err != nil {
		game.SendMessage() <- message.Reply("The offer could not be taken: " + err.Error())
		return true
	}
	game.ClearPendingContextualQuest(char.ID)

	acceptMsg := strings.TrimSpace(quest.Name)
	if desc := strings.TrimSpace(quest.Description); desc != "" {
		acceptMsg += "\n\n" + desc
	}
	objectives := make([]messages.QuestObjectiveProgress, 0, len(quest.Objectives))
	for _, obj := range quest.Objectives {
		required := obj.Amount
		if required < 1 {
			required = 1
		}
		objectives = append(objectives, messages.QuestObjectiveProgress{
			ObjectiveID: obj.ID,
			Description: obj.Description,
			Current:     0,
			Required:    required,
			Completed:   false,
		})
	}
	game.SendMessage() <- messages.QuestUpdateMessage{
		MessageResponse: messages.MessageResponse{
			Audience:   messages.MessageAudienceUser,
			AudienceID: char.BelongsUserID,
			Type:       messages.MessageTypeQuestAccepted,
			Message:    acceptMsg,
		},
		QuestName:  quest.Name,
		QuestID:    quest.ID,
		Status:     "accepted",
		Objectives: objectives,
	}

	questLog, _ := svc.GetQuestLog(char.ID)
	game.SendMessage() <- messages.QuestLogMessage{
		MessageResponse: messages.MessageResponse{
			Audience:   messages.MessageAudienceUser,
			AudienceID: char.BelongsUserID,
			Type:       messages.MessageTypeQuestLog,
		},
		Quests: buildQuestLogEntries(game, questLog),
	}

	log.WithFields(log.Fields{
		"characterID": char.ID,
		"questID":     quest.ID,
		"questName":   quest.Name,
		"source":      "contextual",
	}).Info("Contextual quest accepted")

	runQuestAcceptScript(game, message, quest.OnAcceptScriptID)
	return true
}

// npcOffersContextualQuests reports whether the NPC is a quest giver: it must
// have at least one authored quest sourced from it (same rule the room payload
// uses to flag IsQuestGiver to the client).
func npcOffersContextualQuests(game def.GameCtrl, npc *npc.NPC) bool {
	if npc == nil || game.GetFacade() == nil {
		return false
	}
	svc := game.GetFacade().QuestsService()
	if svc == nil {
		return false
	}
	ids := []string{npc.ID}
	if strings.TrimSpace(npc.TemplateID) != "" && npc.TemplateID != npc.ID {
		ids = append(ids, npc.TemplateID)
	}
	for _, id := range ids {
		authored, err := svc.FindBySourceNPC(id)
		if err == nil && len(authored) > 0 {
			return true
		}
	}
	return false
}
