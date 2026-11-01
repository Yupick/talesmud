package commands_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/talesmud/talesmud/pkg/entities"
	"github.com/talesmud/talesmud/pkg/entities/characters"
	"github.com/talesmud/talesmud/pkg/entities/conversations"
	"github.com/talesmud/talesmud/pkg/entities/dialogs"
	npc "github.com/talesmud/talesmud/pkg/entities/npcs"
	"github.com/talesmud/talesmud/pkg/entities/traits"
	"github.com/talesmud/talesmud/pkg/mudserver/game"
	"github.com/talesmud/talesmud/pkg/mudserver/game/commands"
	"github.com/talesmud/talesmud/pkg/mudserver/game/def"
	"github.com/talesmud/talesmud/pkg/mudserver/game/messages"
	"github.com/talesmud/talesmud/pkg/service"
)

func newLLMDialogTestEnv(t *testing.T) (*game.Game, service.Facade, *characters.Character, *npc.NPC) {
	t.Helper()
	g, facade := newTalkTestGame(t)
	storeTalkRoom(t, facade, "llm-room")
	char := &characters.Character{
		Entity:      &entities.Entity{ID: "char-llm"},
		Name:        "Hero",
		BelongsUser: *traits.BelongsToUser("user-1"),
		CurrentRoom: traits.CurrentRoom{CurrentRoomID: "llm-room"},
	}
	if _, err := facade.CharactersService().Store(char); err != nil {
		t.Fatalf("store character: %v", err)
	}
	target := &npc.NPC{
		Entity:      &entities.Entity{ID: "elder-llm"},
		Name:        "Elder",
		CurrentRoom: traits.CurrentRoom{CurrentRoomID: "llm-room"},
		State:       "idle",
	}
	g.NPCManager.RegisterExistingNPC(target, "llm-room")
	return g, facade, char, target
}

func storeLLMConversation(t *testing.T, facade service.Facade, charID, npcID string, options []def.NPCDialogueOption) *conversations.Conversation {
	t.Helper()
	conv, err := facade.ConversationsService().GetOrCreateConversation(charID, npcID, conversations.TargetTypeNPC, def.NPCDialogueDialogID)
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	payload, err := json.Marshal(options)
	if err != nil {
		t.Fatalf("marshal options: %v", err)
	}
	conv.DialogID = def.NPCDialogueDialogID
	conv.CurrentNodeID = "main"
	conv.SetContext("NPC", "Elder")
	conv.SetContext(def.NPCDialogueContextOptions, string(payload))
	conv.UpdateInteraction()
	if err := facade.ConversationsService().Update(conv.ID, conv); err != nil {
		t.Fatalf("update conversation: %v", err)
	}
	return conv
}

func llmSelectMessage(char *characters.Character, input string) *messages.Message {
	return &messages.Message{
		FromUser:  &entities.User{Entity: &entities.Entity{ID: "user-1"}},
		Character: char,
		Data:      input,
	}
}

func TestLLMDialogFlavorSelectionRequestsContinuation(t *testing.T) {
	g, facade, char, target := newLLMDialogTestEnv(t)
	conv := storeLLMConversation(t, facade, char.ID, target.ID, []def.NPCDialogueOption{
		{Label: "Preguntar por los muros", Kind: "flavor"},
		{Label: "Despedirse", Kind: "end"},
	})

	var got game.NPCDialogueRequest
	g.SetNPCDialogueProvider(func(_ context.Context, req game.NPCDialogueRequest) (game.NPCDialogueResult, error) {
		got = req
		return game.NPCDialogueResult{Reply: "Los muros son antiguos."}, nil
	})

	if !commands.DialogSelectCommand(nil, g, llmSelectMessage(char, "1")) {
		t.Fatal("expected numeric input to be handled as a dialog selection")
	}
	if !got.Continuation {
		t.Fatalf("continuation flag not set: %+v", got)
	}
	if got.Question != "Preguntar por los muros" || got.NPCID != target.ID {
		t.Fatalf("unexpected continuation request: %+v", got)
	}

	updated, err := facade.ConversationsService().FindByID(conv.ID)
	if err != nil {
		t.Fatalf("reload conversation: %v", err)
	}
	if updated.DialogID != "" || updated.Context[def.NPCDialogueContextOptions] != "" {
		t.Fatalf("stale LLM choices were not cleared: %+v", updated)
	}
	if !updated.LastInteracted.IsZero() {
		t.Fatalf("closed conversation should not stay active: %v", updated.LastInteracted)
	}
}

func TestLLMDialogEndSelectionClosesConversation(t *testing.T) {
	g, facade, char, target := newLLMDialogTestEnv(t)
	conv := storeLLMConversation(t, facade, char.ID, target.ID, []def.NPCDialogueOption{
		{Label: "Despedirse", Kind: "end"},
	})

	called := false
	g.SetNPCDialogueProvider(func(_ context.Context, _ game.NPCDialogueRequest) (game.NPCDialogueResult, error) {
		called = true
		return game.NPCDialogueResult{Reply: "no debería ejecutarse"}, nil
	})

	if !commands.DialogSelectCommand(nil, g, llmSelectMessage(char, "1")) {
		t.Fatal("expected numeric input to be handled as a dialog selection")
	}
	if called {
		t.Fatal("end option must not call the LLM provider")
	}

	var ended bool
	for _, raw := range drainTalkMessages(g.SendMessage()) {
		switch m := raw.(type) {
		case messages.MessageResponse:
			if m.Type == messages.MessageTypeDialogEnd {
				ended = true
			}
		case *messages.MessageResponse:
			if m.Type == messages.MessageTypeDialogEnd {
				ended = true
			}
		case *messages.DialogMessage:
			if strings.Contains(strings.ToLower(m.NPCText), "conversation") {
				ended = true
			}
		}
	}
	if !ended {
		t.Fatal("expected a dialog end message")
	}
	updated, err := facade.ConversationsService().FindByID(conv.ID)
	if err != nil {
		t.Fatalf("reload conversation: %v", err)
	}
	if updated.DialogID != "" || !updated.LastInteracted.IsZero() {
		t.Fatalf("end option did not close the conversation: %+v", updated)
	}
}

func TestLLMDialogUnavailableQuestDoesNotMutateQuest(t *testing.T) {
	g, facade, char, target := newLLMDialogTestEnv(t)
	conv := storeLLMConversation(t, facade, char.ID, target.ID, []def.NPCDialogueOption{{Label: "Quest antigua", Kind: "quest:missing-quest"}})
	if !commands.DialogSelectCommand(nil, g, llmSelectMessage(char, "1")) {
		t.Fatal("quest selection should be handled")
	}
	progress, err := facade.QuestsService().GetProgress(char.ID, "missing-quest")
	if err != nil {
		t.Fatalf("query missing quest progress: %v", err)
	}
	if progress != nil {
		t.Fatalf("unknown quest must not mutate progress: %+v", progress)
	}
	updated, err := facade.ConversationsService().FindByID(conv.ID)
	if err != nil {
		t.Fatalf("reload conversation: %v", err)
	}
	if updated.DialogID != def.NPCDialogueDialogID || updated.Context[def.NPCDialogueContextOptions] == "" {
		t.Fatalf("unavailable quest must preserve choices: %+v", updated)
	}
	var explained bool
	for _, raw := range drainTalkMessages(g.SendMessage()) {
		if response, ok := raw.(messages.MessageResponse); ok && strings.Contains(response.Message, "no longer available") {
			explained = true
		}
	}
	if !explained {
		t.Fatal("expected unavailable quest explanation")
	}
}

func TestLLMDialogUnknownSelectionKeepsConversation(t *testing.T) {
	g, facade, char, target := newLLMDialogTestEnv(t)
	conv := storeLLMConversation(t, facade, char.ID, target.ID, []def.NPCDialogueOption{
		{Label: "Preguntar por los muros", Kind: "flavor"},
	})
	if !commands.DialogSelectCommand(nil, g, llmSelectMessage(char, "9")) {
		t.Fatal("expected invalid numeric input to be handled")
	}
	updated, err := facade.ConversationsService().FindByID(conv.ID)
	if err != nil {
		t.Fatalf("reload conversation: %v", err)
	}
	if updated.DialogID != def.NPCDialogueDialogID {
		t.Fatalf("invalid selection should keep the choices: %+v", updated)
	}
}

func TestLLMDialogMappedOptionContinuesAuthoredNode(t *testing.T) {
	g, facade, char, target := newLLMDialogTestEnv(t)
	target.LLM = &npc.LLMConfig{Enabled: true}
	target.DialogID = "dialog-llm"
	if _, err := facade.DialogsService().Import(&dialogs.Dialog{
		Entity: &entities.Entity{ID: "dialog-llm"},
		NodeID: "main",
		Text:   "Bienvenido, viajero.",
		Options: []*dialogs.Dialog{
			{
				Entity: &entities.Entity{ID: "opt-walls"},
				NodeID: "walls",
				Name:   "Preguntar por los muros",
				Text:   "Los muros son antiguos.",
			},
		},
	}); err != nil {
		t.Fatalf("store dialog: %v", err)
	}
	g.SetNPCDialogueProvider(func(_ context.Context, _ game.NPCDialogueRequest) (game.NPCDialogueResult, error) {
		return game.NPCDialogueResult{
			Reply:   "Puedo contarte muchas cosas.",
			Options: []game.NPCDialogueOption{{Label: "Preguntar por los muros", Kind: "flavor"}},
		}, nil
	})

	msg := llmSelectMessage(char, "ask elder where is the temple")
	if !g.CommandProcessor.Process(g, msg) {
		t.Fatal("ask command was not handled")
	}
	_ = drainTalkMessages(g.SendMessage())

	if !commands.DialogSelectCommand(nil, g, llmSelectMessage(char, "1")) {
		t.Fatal("mapped option was not handled")
	}
	var gotWalls bool
	for _, raw := range drainTalkMessages(g.SendMessage()) {
		switch m := raw.(type) {
		case *messages.DialogMessage:
			if strings.Contains(m.NPCText, "Los muros") {
				gotWalls = true
			}
		}
	}
	if !gotWalls {
		t.Fatal("expected the authored walls node to be rendered")
	}
}

func TestLLMDialogClosedConversationIgnoresOpenNPCSession(t *testing.T) {
	g, facade, char, target := newLLMDialogTestEnv(t)
	conv := storeLLMConversation(t, facade, char.ID, target.ID, []def.NPCDialogueOption{
		{Label: "Preguntar por los muros", Kind: "flavor"},
	})
	conv.DialogID = ""
	conv.CurrentNodeID = ""
	conv.SetContext(def.NPCDialogueContextOptions, "")
	conv.LastInteracted = time.Time{}
	if err := facade.ConversationsService().Update(conv.ID, conv); err != nil {
		t.Fatalf("close conversation: %v", err)
	}
	g.NPCManager.EngageNPC(char.ID, target.ID, "dialog")

	if commands.DialogSelectCommand(nil, g, llmSelectMessage(char, "1")) {
		t.Fatal("closed conversation must not intercept numeric input")
	}
}

func TestLLMDialogFreeTextInterceptsRecentOpenSession(t *testing.T) {
	g, facade, char, target := newLLMDialogTestEnv(t)
	target.LLM = &npc.LLMConfig{Enabled: true}
	conv := storeLLMConversation(t, facade, char.ID, target.ID, []def.NPCDialogueOption{{Label: "Continuar", Kind: "flavor"}})
	g.NPCManager.EngageNPC(char.ID, target.ID, "dialog")
	var got game.NPCDialogueRequest
	g.SetNPCDialogueProvider(func(_ context.Context, req game.NPCDialogueRequest) (game.NPCDialogueResult, error) {
		got = req
		return game.NPCDialogueResult{Reply: "Te escucho."}, nil
	})
	msg := llmSelectMessage(char, "He venido a hablar del puente.")
	if !commands.LLMDialogFreeText(g, msg) {
		t.Fatal("free text should be intercepted by active LLM conversation")
	}
	if !got.Continuation || got.Question != msg.Data || got.NPCID != target.ID {
		t.Fatalf("unexpected free-text continuation: %+v", got)
	}
	updated, err := facade.ConversationsService().FindByID(conv.ID)
	if err != nil {
		t.Fatalf("reload conversation: %v", err)
	}
	if updated.DialogID == def.NPCDialogueDialogID {
		t.Fatalf("scheduled free text should close old choices: %+v", updated)
	}
	var acknowledged bool
	for _, raw := range drainTalkMessages(g.SendMessage()) {
		if response, ok := raw.(messages.MessageResponse); ok && response.Message == "Elder considers your words..." {
			acknowledged = true
		}
	}
	if !acknowledged {
		t.Fatal("expected free-text acknowledgement")
	}
}

func TestLLMDialogFreeTextCanUseRecentConversationWithoutOpenSession(t *testing.T) {
	g, facade, char, target := newLLMDialogTestEnv(t)
	target.LLM = &npc.LLMConfig{Enabled: true}
	conv := storeLLMConversation(t, facade, char.ID, target.ID, []def.NPCDialogueOption{{Label: "Continuar", Kind: "flavor"}})
	var got game.NPCDialogueRequest
	g.SetNPCDialogueProvider(func(_ context.Context, req game.NPCDialogueRequest) (game.NPCDialogueResult, error) {
		got = req
		return game.NPCDialogueResult{Reply: "Te escucho."}, nil
	})
	msg := llmSelectMessage(char, "Una frase libre")
	if !commands.LLMDialogFreeText(g, msg) {
		t.Fatal("recent LLM conversation should allow free-text continuation")
	}
	if !got.Continuation || got.Question != msg.Data {
		t.Fatalf("unexpected continuation request: %+v", got)
	}
	updated, err := facade.ConversationsService().FindByID(conv.ID)
	if err != nil {
		t.Fatalf("reload conversation: %v", err)
	}
	if updated.DialogID != "" {
		t.Fatalf("scheduled free text should close old options: %+v", updated)
	}
}

func TestLLMDialogFreeTextDoesNotInterceptWithoutSessionWindowOrNearbyNPC(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*game.Game, service.Facade, *characters.Character, *npc.NPC)
	}{
		{name: "no conversation", setup: func(*game.Game, service.Facade, *characters.Character, *npc.NPC) {}},
		{name: "expired conversation", setup: func(_ *game.Game, facade service.Facade, char *characters.Character, target *npc.NPC) {
			conv := storeLLMConversation(t, facade, char.ID, target.ID, []def.NPCDialogueOption{{Label: "Continue", Kind: "flavor"}})
			conv.LastInteracted = time.Now().Add(-61 * time.Second)
			if err := facade.ConversationsService().Update(conv.ID, conv); err != nil {
				t.Fatalf("expire conversation: %v", err)
			}
		}},
		{name: "closed session and expired conversation", setup: func(_ *game.Game, facade service.Facade, char *characters.Character, target *npc.NPC) {
			conv := storeLLMConversation(t, facade, char.ID, target.ID, []def.NPCDialogueOption{{Label: "Continue", Kind: "flavor"}})
			conv.LastInteracted = time.Now().Add(-61 * time.Second)
			if err := facade.ConversationsService().Update(conv.ID, conv); err != nil {
				t.Fatalf("expire conversation: %v", err)
			}
		}},
		{name: "LLM disabled", setup: func(_ *game.Game, facade service.Facade, char *characters.Character, target *npc.NPC) {
			_ = storeLLMConversation(t, facade, char.ID, target.ID, []def.NPCDialogueOption{{Label: "Continue", Kind: "flavor"}})
			target.LLM = nil
		}},
		{name: "dead NPC", setup: func(_ *game.Game, facade service.Facade, char *characters.Character, target *npc.NPC) {
			_ = storeLLMConversation(t, facade, char.ID, target.ID, []def.NPCDialogueOption{{Label: "Continue", Kind: "flavor"}})
			target.IsDead = true
		}},
		{name: "outside room", setup: func(_ *game.Game, facade service.Facade, char *characters.Character, target *npc.NPC) {
			_ = storeLLMConversation(t, facade, char.ID, target.ID, []def.NPCDialogueOption{{Label: "Continue", Kind: "flavor"}})
			target.CurrentRoomID = "elsewhere"
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, facade, char, target := newLLMDialogTestEnv(t)
			target.LLM = &npc.LLMConfig{Enabled: true}
			tt.setup(g, facade, char, target)
			if commands.LLMDialogFreeText(g, llmSelectMessage(char, "hello")) {
				t.Fatal("ordinary text must not be intercepted")
			}
		})
	}
}
