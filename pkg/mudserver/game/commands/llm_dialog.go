package commands

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/talesmud/talesmud/pkg/entities/conversations"
	"github.com/talesmud/talesmud/pkg/mudserver/game/def"
	"github.com/talesmud/talesmud/pkg/mudserver/game/messages"
)

// handleLLMDialogSelection resolves a numeric choice from the sentinel
// "__llm__" conversation. Authored node matches continue the real dialog tree;
// flavor options ask the NPC again without a new cooldown penalty; end closes
// the conversation.
func handleLLMDialogSelection(game def.GameCtrl, message *messages.Message, conv *conversations.Conversation, optionIndex int) bool {
	options, err := parseLLMDialogOptions(conv)
	if err != nil || len(options) == 0 {
		game.SendMessage() <- message.Reply("That choice is no longer available.")
		closeLLMDialogConversation(game, conv)
		return true
	}
	if optionIndex < 1 || optionIndex > len(options) {
		game.SendMessage() <- message.Reply("Invalid option. Please choose 1-" + strconv.Itoa(len(options)))
		return true
	}
	option := options[optionIndex-1]
	npcName := strings.TrimSpace(conv.Context["NPC"])
	if npcName == "" {
		npcName = "NPC"
	}

	target := game.GetNPCInstanceManager().GetInstance(conv.TargetID)
	if target == nil {
		game.SendMessage() <- messages.NewDialogEndMessage(message.FromUser.ID, npcName, npcName+" is no longer here.")
		closeNPCSession(game, message, "dialog")
		closeLLMDialogConversation(game, conv)
		return true
	}

	if option.Kind == "end" {
		game.SendMessage() <- messages.NewDialogEndMessage(message.FromUser.ID, npcName, npcName+" nods, and the conversation comes to a close.")
		closeNPCSession(game, message, "dialog")
		closeLLMDialogConversation(game, conv)
		return true
	}
	if strings.HasPrefix(option.Kind, "quest:") {
		questID := strings.TrimSpace(strings.TrimPrefix(option.Kind, "quest:"))
		for _, qo := range questOptionsForConversation(game, message.Character.ID, conv) {
			if qo.questID == questID {
				handleQuestDialogOption(game, message, qo, npcName, conv)
				return true
			}
		}
		game.SendMessage() <- message.Reply("That quest is no longer available. Choose another option.")
		return true
	}

	if strings.TrimSpace(option.NodeID) != "" {
		// Continue the authored tree at the node the model label matched.
		return talkToNPCAt(game, message, target, option.NodeID)
	}

	// Flavor follow-up: ask the NPC to answer the selected line. The
	// continuation flag bypasses the cooldown rejection but still honours the
	// generation semaphore.
	req := buildNPCDialogueRequest(message, target, option.Label, true)
	switch game.RequestNPCDialogue(req) {
	case def.NPCDialogueScheduled:
		closeLLMDialogConversation(game, conv)
		game.SendMessage() <- message.Reply(npcName + " considers your words...")
	case def.NPCDialogueRateLimited:
		game.SendMessage() <- message.Reply(npcName + " is still thinking about your last question. Try again shortly.")
	case def.NPCDialogueBusy:
		game.SendMessage() <- message.Reply(npcName + " is busy with another question. Try again shortly.")
	default:
		game.SendMessage() <- message.Reply(npcName + " cannot answer freely right now.")
	}
	return true
}

const llmFreeTextWindow = 60 * time.Second

// LLMDialogFreeText handles ordinary chat input as a continuation only while
// the character has a recent, live LLM conversation with the NPC.
func LLMDialogFreeText(game def.GameCtrl, message *messages.Message) bool {
	if game == nil || message == nil || message.Character == nil || strings.TrimSpace(message.Data) == "" {
		return false
	}
	manager := game.GetNPCInstanceManager()
	if manager == nil {
		return false
	}
	conv := latestLLMDialogConversation(game, message.Character.ID)
	if conv == nil {
		return false
	}
	target := manager.GetInstance(conv.TargetID)
	if target == nil || target.IsDead || target.State == "dead" ||
		(target.MaxHitPoints > 0 && target.CurrentHitPoints <= 0) ||
		target.CurrentRoomID != message.Character.CurrentRoomID || !target.LLMEnabled() {
		return false
	}
	if !npcSessionOpen(game, message.Character.ID, target.ID, "dialog") &&
		(time.Since(conv.LastInteracted) >= llmFreeTextWindow || conv.LastInteracted.IsZero()) {
		return false
	}
	req := buildNPCDialogueRequest(message, target, strings.TrimSpace(message.Data), true)
	switch game.RequestNPCDialogue(req) {
	case def.NPCDialogueScheduled:
		closeLLMDialogConversation(game, conv)
		game.SendMessage() <- message.Reply(target.Name + " considers your words...")
	case def.NPCDialogueRateLimited:
		game.SendMessage() <- message.Reply(target.Name + " is still thinking about your last question. Try again shortly.")
	case def.NPCDialogueBusy:
		game.SendMessage() <- message.Reply(target.Name + " is busy with another question. Try again shortly.")
	default:
		game.SendMessage() <- message.Reply(target.Name + " cannot answer freely right now.")
	}
	return true
}

func latestLLMDialogConversation(game def.GameCtrl, characterID string) *conversations.Conversation {
	if game == nil || game.GetFacade() == nil || game.GetFacade().ConversationsService() == nil {
		return nil
	}
	convs, err := game.GetFacade().ConversationsService().FindAllForCharacter(characterID)
	if err != nil {
		return nil
	}
	var latest *conversations.Conversation
	for _, conv := range convs {
		if conv == nil || conv.DialogID != def.NPCDialogueDialogID || conv.TargetType != conversations.TargetTypeNPC || conv.LastInteracted.IsZero() {
			continue
		}
		if latest == nil || conv.LastInteracted.After(latest.LastInteracted) {
			latest = conv
		}
	}
	return latest
}

// closeLLMDialogConversation clears the pending choices and the sentinel
// DialogID so a stale numeric input cannot reopen an old reply. It keeps the
// row (GetOrCreateConversation reuses it) but removes it from the active set.
func closeLLMDialogConversation(game def.GameCtrl, conv *conversations.Conversation) {
	if game == nil || game.GetFacade() == nil || game.GetFacade().ConversationsService() == nil || conv == nil {
		return
	}
	conv.DialogID = ""
	conv.CurrentNodeID = ""
	conv.SetContext(def.NPCDialogueContextOptions, "")
	conv.SetContext(def.NPCDialogueContextQuestion, "")
	conv.LastInteracted = time.Time{}
	_ = game.GetFacade().ConversationsService().Update(conv.ID, conv)
}

// refreshLLMDialog updates quest labels in place after a quest action accepted
// from an LLM option. Keeping the same indexes is important: the client still
// shows the previous option list until this refresh arrives, so reordering
// would make a click select the wrong line.
func refreshLLMDialog(game def.GameCtrl, message *messages.Message, npcName string, conv *conversations.Conversation) {
	if game == nil || message == nil || message.Character == nil || conv == nil {
		return
	}
	options, err := parseLLMDialogOptions(conv)
	if err != nil || len(options) == 0 {
		return
	}
	available := make(map[string]questDialogOption)
	for _, qo := range questOptionsForConversation(game, message.Character.ID, conv) {
		available[qo.questID] = qo
	}
	changed := false
	for i := range options {
		if !strings.HasPrefix(options[i].Kind, "quest:") {
			continue
		}
		questID := strings.TrimSpace(strings.TrimPrefix(options[i].Kind, "quest:"))
		qo, ok := available[questID]
		if !ok || options[i].Label == qo.text {
			continue
		}
		options[i].Label = qo.text
		changed = true
	}
	if !changed {
		return
	}
	payload, err := json.Marshal(options)
	if err != nil {
		return
	}
	conv.SetContext(def.NPCDialogueContextOptions, string(payload))
	svc := game.GetFacade().ConversationsService()
	if svc == nil {
		return
	}
	if err := svc.Update(conv.ID, conv); err != nil {
		return
	}
	if message.FromUser == nil {
		return
	}
	reply := strings.TrimSpace(conv.Context[def.NPCDialogueContextReply])
	if reply == "" {
		reply = npcName + " waits for your answer."
	}
	dialogOptions := make([]messages.DialogOption, len(options))
	for i, opt := range options {
		dialogOptions[i] = messages.DialogOption{Index: i + 1, Text: opt.Label, Kind: opt.Kind}
	}
	dialogMsg := messages.NewDialogMessage(message.FromUser.ID, npcName, reply, dialogOptions, conv.ID)
	dialogMsg.Username = npcName
	dialogMsg.FreeText = true
	game.SendMessage() <- dialogMsg
}

func parseLLMDialogOptions(conv *conversations.Conversation) ([]def.NPCDialogueOption, error) {
	if conv == nil {
		return nil, nil
	}
	raw := strings.TrimSpace(conv.Context[def.NPCDialogueContextOptions])
	if raw == "" {
		return nil, nil
	}
	var options []def.NPCDialogueOption
	if err := json.Unmarshal([]byte(raw), &options); err != nil {
		return nil, err
	}
	out := make([]def.NPCDialogueOption, 0, len(options))
	for _, opt := range options {
		label := strings.TrimSpace(opt.Label)
		if label == "" {
			continue
		}
		opt.Label = label
		opt.Kind = normalizeLLMOptionKind(opt.Kind)
		opt.NodeID = strings.TrimSpace(opt.NodeID)
		out = append(out, opt)
	}
	return out, nil
}

func normalizeLLMOptionKind(kind string) string {
	kind = strings.TrimSpace(kind)
	lower := strings.ToLower(kind)
	if lower == "flavor" || lower == "end" {
		return lower
	}
	if len(kind) >= len("quest:") && strings.EqualFold(kind[:len("quest:")], "quest:") {
		return "quest:" + strings.TrimSpace(kind[len("quest:"):])
	}
	return kind
}
