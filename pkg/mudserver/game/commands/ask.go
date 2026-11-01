package commands

import (
	"strings"

	npc "github.com/talesmud/talesmud/pkg/entities/npcs"
	"github.com/talesmud/talesmud/pkg/mudserver/game/def"
	"github.com/talesmud/talesmud/pkg/mudserver/game/messages"
)

// AskCommand asks an NPC a free-form question (Phase 6.3 runtime cut). NPCs
// without LLM dialogue degrade to their authored dialog, so every NPC keeps
// answering. The model call never runs on the command loop: the game schedules
// it and posts the reply back asynchronously.
type AskCommand struct{}

// Key returns the command key matcher.
func (cmd *AskCommand) Key() CommandKey { return &StartsWithCommandKey{} }

// Execute handles the ask command.
func (cmd *AskCommand) Execute(game def.GameCtrl, message *messages.Message) bool {
	if message.Character == nil {
		game.SendMessage() <- message.Reply("You need to select a character first.")
		return true
	}
	if message.Character.CurrentRoomID == "" {
		game.SendMessage() <- message.Reply("You are not in a room.")
		return true
	}

	rest := trimToPrefix(stripFirstField(message.Data))
	if rest == "" {
		game.SendMessage() <- message.Reply("Ask whom? Usage: ask <npc-name> [question]")
		return true
	}

	npcManager := game.GetNPCInstanceManager()
	if npcManager == nil {
		game.SendMessage() <- message.Reply("Error: NPC system not available.")
		return true
	}

	target, question := resolveAskTarget(npcManager, message.Character.CurrentRoomID, rest)
	if target == nil {
		name := strings.Fields(rest)[0]
		game.SendMessage() <- message.Reply("There is no one named '" + name + "' here.")
		return true
	}

	// NPCs without the LLM opt-in keep their authored dialog. Tell the player
	// plainly instead of failing silently.
	if !target.LLMEnabled() {
		game.SendMessage() <- message.Reply(target.Name + " has no free-form voice; you speak with them the usual way.")
		return talkToNPC(game, message, target)
	}

	req := buildNPCDialogueRequest(message, target, strings.TrimSpace(question), false)

	switch game.RequestNPCDialogue(req) {
	case def.NPCDialogueDisabled:
		// The server has no LLM provider wired. Keep the NPC responsive via
		// the authored dialog path.
		game.SendMessage() <- message.Reply(target.Name + " cannot answer freely right now; you speak with them the usual way.")
		return talkToNPC(game, message, target)
	case def.NPCDialogueRateLimited:
		game.SendMessage() <- message.Reply(target.Name + " is still thinking about your last question. Try again shortly.")
	case def.NPCDialogueBusy:
		game.SendMessage() <- message.Reply(target.Name + " is busy with another question. Try again shortly.")
	default:
		if strings.TrimSpace(question) == "" {
			game.SendMessage() <- message.Reply("You greet " + target.Name + ".")
		} else {
			game.SendMessage() <- message.Reply(target.Name + " considers your words...")
		}
	}
	return true
}

// buildNPCDialogueRequest assembles the shared request used by ask and by
// follow-up options selected from an LLM conversation. Continuation requests
// still go through the semaphore but skip the game-level cooldown rejection.
func buildNPCDialogueRequest(message *messages.Message, target *npc.NPC, question string, continuation bool) def.NPCDialogueRequest {
	req := def.NPCDialogueRequest{
		CharacterID:     message.Character.ID,
		NPCID:           target.ID,
		NPCName:         target.Name,
		CharacterName:   message.Character.Name,
		PlayerLevel:     int(message.Character.Level),
		RoomID:          message.Character.CurrentRoomID,
		Question:        strings.TrimSpace(question),
		CooldownSeconds: target.EffectiveCooldownSeconds(),
		Continuation:    continuation,
	}
	if message.FromUser != nil {
		req.UserID = message.FromUser.ID
	}
	req.NPCTemplateID = strings.TrimSpace(target.TemplateID)
	if req.NPCTemplateID == "" {
		req.NPCTemplateID = target.ID
	}
	if target.LLM != nil {
		req.Persona = target.LLM.Persona
		req.Voice = target.LLM.Voice
		req.Temperature = target.LLM.Temperature
	}
	return req
}

// resolveAskTarget splits "<npc name> <question>" by trying the longest name
// prefix that matches an NPC in the room. The remainder is the question, which
// may be empty (a plain greeting).
func resolveAskTarget(manager def.NPCInstanceCtrl, roomID, rest string) (*npc.NPC, string) {
	tokens := strings.Fields(rest)
	for i := len(tokens); i >= 1; i-- {
		name := strings.Join(tokens[:i], " ")
		if target := manager.FindInstanceByNameInRoom(roomID, name); target != nil {
			return target, strings.Join(tokens[i:], " ")
		}
	}
	return nil, ""
}

// stripFirstField removes the command word from the raw input, tolerating
// leading whitespace.
func stripFirstField(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, " \t"); i >= 0 {
		return strings.TrimSpace(s[i:])
	}
	return ""
}

// trimToPrefix drops an optional leading "to " (as in "ask to the guard").
func trimToPrefix(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(strings.ToLower(s), "to ") {
		return strings.TrimSpace(s[3:])
	}
	return s
}
