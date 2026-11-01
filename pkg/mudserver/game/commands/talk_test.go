package commands_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/talesmud/talesmud/pkg/db/sqlite"
	"github.com/talesmud/talesmud/pkg/entities"
	"github.com/talesmud/talesmud/pkg/entities/characters"
	"github.com/talesmud/talesmud/pkg/entities/conversations"
	npc "github.com/talesmud/talesmud/pkg/entities/npcs"
	"github.com/talesmud/talesmud/pkg/entities/quests"
	"github.com/talesmud/talesmud/pkg/entities/rooms"
	"github.com/talesmud/talesmud/pkg/entities/traits"
	"github.com/talesmud/talesmud/pkg/mudserver/game"
	"github.com/talesmud/talesmud/pkg/mudserver/game/commands"
	"github.com/talesmud/talesmud/pkg/mudserver/game/def"
	"github.com/talesmud/talesmud/pkg/mudserver/game/messages"
	"github.com/talesmud/talesmud/pkg/repository"
	"github.com/talesmud/talesmud/pkg/service"
)

func newTalkTestGame(t *testing.T) (*game.Game, service.Facade) {
	t.Helper()

	client, err := sqlite.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	facade := service.NewFacade(repository.NewSQLiteFactory(client), nil)
	return game.New(facade), facade
}

func drainTalkMessages(ch <-chan interface{}) []interface{} {
	var result []interface{}
	for {
		select {
		case msg := <-ch:
			result = append(result, msg)
		default:
			return result
		}
	}
}

func storeTalkRoom(t *testing.T, facade service.Facade, id string) {
	t.Helper()
	if _, err := facade.RoomsService().Store(&rooms.Room{
		Entity:      &entities.Entity{ID: id},
		Name:        id,
		Description: id,
		Exits:       &rooms.Exits{},
	}); err != nil {
		t.Fatalf("store room: %v", err)
	}
}

func TestSpeakAliasStartsNPCDialog(t *testing.T) {
	g, facade := newTalkTestGame(t)
	storeTalkRoom(t, facade, "town")

	character := &characters.Character{
		Entity:      &entities.Entity{ID: "char-speak"},
		Name:        "Speaker",
		BelongsUser: *traits.BelongsToUser("user-1"),
		CurrentRoom: traits.CurrentRoom{CurrentRoomID: "town"},
	}
	guide := &npc.NPC{
		Entity:      &entities.Entity{ID: "guide-1"},
		Name:        "Guide",
		CurrentRoom: traits.CurrentRoom{CurrentRoomID: "town"},
		State:       "idle",
	}
	g.NPCManager.RegisterExistingNPC(guide, "town")

	msg := &messages.Message{
		FromUser:  &entities.User{Entity: &entities.Entity{ID: "user-1"}},
		Character: character,
		Data:      "speak to guide",
	}

	if !g.CommandProcessor.Process(g, msg) {
		t.Fatal("expected speak alias to be handled by talk command")
	}
}

func TestTalkAutoOpensLLMWhenNoAuthoredDialogOrQuest(t *testing.T) {
	g, facade := newTalkTestGame(t)
	storeTalkRoom(t, facade, "town")
	character := &characters.Character{
		Entity:      &entities.Entity{ID: "char-auto-llm"},
		Name:        "Speaker",
		BelongsUser: *traits.BelongsToUser("user-1"),
		CurrentRoom: traits.CurrentRoom{CurrentRoomID: "town"},
	}
	target := &npc.NPC{
		Entity:      &entities.Entity{ID: "guide-auto-llm"},
		Name:        "Guide",
		CurrentRoom: traits.CurrentRoom{CurrentRoomID: "town"},
		State:       "idle",
		LLM:         &npc.LLMConfig{Enabled: true},
	}
	g.NPCManager.RegisterExistingNPC(target, "town")
	var gotQuestion string
	g.SetNPCDialogueProvider(func(_ context.Context, req game.NPCDialogueRequest) (game.NPCDialogueResult, error) {
		gotQuestion = req.Question
		return game.NPCDialogueResult{Reply: "Welcome."}, nil
	})
	msg := &messages.Message{
		FromUser:  &entities.User{Entity: &entities.Entity{ID: "user-1"}},
		Character: character,
		Data:      "talk guide",
	}
	if !g.CommandProcessor.Process(g, msg) {
		t.Fatal("talk command did not handle LLM NPC")
	}
	if gotQuestion != "" {
		t.Fatalf("auto-greeting question = %q, want empty", gotQuestion)
	}
	var greeted bool
	for _, raw := range drainTalkMessages(g.SendMessage()) {
		if response, ok := raw.(messages.MessageResponse); ok && response.Message == "You greet Guide." {
			greeted = true
		}
	}
	if !greeted {
		t.Fatal("expected scheduled auto-greeting response")
	}
}

func TestTalkAutoOpenFallsBackWhenLLMDisabled(t *testing.T) {
	g, facade := newTalkTestGame(t)
	storeTalkRoom(t, facade, "town")
	character := &characters.Character{
		Entity:      &entities.Entity{ID: "char-auto-disabled"},
		Name:        "Speaker",
		BelongsUser: *traits.BelongsToUser("user-1"),
		CurrentRoom: traits.CurrentRoom{CurrentRoomID: "town"},
	}
	target := &npc.NPC{
		Entity:      &entities.Entity{ID: "guide-disabled"},
		Name:        "Guide",
		CurrentRoom: traits.CurrentRoom{CurrentRoomID: "town"},
		State:       "idle",
		LLM:         &npc.LLMConfig{Enabled: true},
	}
	g.NPCManager.RegisterExistingNPC(target, "town")
	msg := &messages.Message{
		FromUser:  &entities.User{Entity: &entities.Entity{ID: "user-1"}},
		Character: character,
		Data:      "talk guide",
	}
	if !g.CommandProcessor.Process(g, msg) {
		t.Fatal("talk command did not handle disabled LLM NPC")
	}
	var fallback bool
	for _, raw := range drainTalkMessages(g.SendMessage()) {
		if response, ok := raw.(messages.MessageResponse); ok && strings.Contains(response.Message, "doesn't seem to want to talk") {
			fallback = true
		}
	}
	if !fallback {
		t.Fatal("expected original no-dialog fallback when LLM provider is disabled")
	}
}

func TestQuestOnlyNPCSelectionAcceptsQuest(t *testing.T) {
	g, facade := newTalkTestGame(t)
	storeTalkRoom(t, facade, "quest-room")

	character, err := facade.CharactersService().Store(&characters.Character{
		Entity:      &entities.Entity{ID: "char-quest"},
		Name:        "Questor",
		BelongsUser: *traits.BelongsToUser("user-1"),
		CurrentRoom: traits.CurrentRoom{CurrentRoomID: "quest-room"},
	})
	if err != nil {
		t.Fatalf("store character: %v", err)
	}

	questGiver := &npc.NPC{
		Entity:      &entities.Entity{ID: "quest-giver-1"},
		Name:        "Elder",
		CurrentRoom: traits.CurrentRoom{CurrentRoomID: "quest-room"},
		State:       "idle",
	}
	g.NPCManager.RegisterExistingNPC(questGiver, "quest-room")
	if _, err := facade.NPCsService().Import(questGiver); err != nil {
		t.Fatalf("store quest giver: %v", err)
	}

	quest := &quests.Quest{
		Entity:           &entities.Entity{ID: "quest-1"},
		Name:             "First Errand",
		Description:      "Do a useful thing.",
		AcceptDialogText: "I knew you would help.",
		Source: quests.QuestSource{
			Type:  "npc",
			NPCID: questGiver.ID,
		},
		Objectives: []quests.Objective{
			{
				ID:          "talk-objective",
				Type:        quests.ObjectiveTalk,
				Description: "Talk to Elder.",
				TargetID:    questGiver.ID,
				Amount:      1,
			},
		},
	}
	if _, err := facade.QuestsService().Store(quest); err != nil {
		t.Fatalf("store quest: %v", err)
	}

	talk := &messages.Message{
		FromUser:  &entities.User{Entity: &entities.Entity{ID: "user-1"}},
		Character: character,
		Data:      "talk elder",
	}
	if !g.CommandProcessor.Process(g, talk) {
		t.Fatal("talk command did not handle quest NPC")
	}

	var sawQuestOption bool
	for _, out := range drainTalkMessages(g.SendMessage()) {
		if dialogMsg, ok := out.(*messages.DialogMessage); ok &&
			len(dialogMsg.Options) == 1 &&
			strings.Contains(dialogMsg.Options[0].Text, "First Errand") &&
			dialogMsg.ConversationID != "" {
			sawQuestOption = true
		}
	}
	if !sawQuestOption {
		t.Fatal("expected quest-only NPC to send selectable dialog with a conversation ID")
	}

	selectMsg := &messages.Message{
		FromUser:  &entities.User{Entity: &entities.Entity{ID: "user-1"}},
		Character: character,
		Data:      "1",
	}
	if !g.RoomProcessor.Process(g, selectMsg) {
		t.Fatal("expected numeric quest selection to be handled")
	}

	progress, err := facade.QuestsService().GetProgress(character.ID, quest.ID)
	if err != nil {
		t.Fatalf("get quest progress: %v", err)
	}
	if progress == nil || progress.Status != quests.QuestStatusActive {
		t.Fatalf("expected quest to be active after numeric selection, got %#v", progress)
	}
}

func TestAvailableQuestPromptContextMatchesTalkQuestOptions(t *testing.T) {
	_, facade := newTalkTestGame(t)
	storeTalkRoom(t, facade, "quest-context-room")
	char, err := facade.CharactersService().Store(&characters.Character{
		Entity:      &entities.Entity{ID: "char-context"},
		Name:        "Questor",
		BelongsUser: *traits.BelongsToUser("user-1"),
		CurrentRoom: traits.CurrentRoom{CurrentRoomID: "quest-context-room"},
	})
	if err != nil {
		t.Fatalf("store character: %v", err)
	}
	questNPC := &npc.NPC{Entity: &entities.Entity{ID: "elder-template"}, Name: "Elder", IsTemplate: true, State: "idle"}
	if _, err := facade.NPCsService().Import(questNPC); err != nil {
		t.Fatalf("store quest NPC: %v", err)
	}
	quest := &quests.Quest{
		Entity:      &entities.Entity{ID: "Quest-MixedCase"},
		Name:        "A Mixed Quest",
		Description: "A quest used to build the prompt.",
		Source:      quests.QuestSource{Type: "npc", NPCID: "elder-template"},
		Objectives:  []quests.Objective{{ID: "visit", Type: quests.ObjectiveVisit, Description: "Visit the room.", TargetID: "quest-context-room", Amount: 1}},
	}
	if _, err := facade.QuestsService().Store(quest); err != nil {
		t.Fatalf("store quest: %v", err)
	}
	text, ids := commands.AvailableQuestPromptContext(facade, char.ID, "elder-template", "elder-instance")
	if ids[quest.ID] != "accept" || !strings.Contains(text, quest.ID) || !strings.Contains(text, "A Mixed Quest") || !strings.Contains(text, "accept") {
		t.Fatalf("unexpected quest prompt context: %q, %#v", text, ids)
	}
}

func TestQuestOnlyConversationPreservesQuestActionOptions(t *testing.T) {
	for _, action := range []string{"accept", "complete", "progress"} {
		t.Run(action, func(t *testing.T) {
			g, facade := newTalkTestGame(t)
			storeTalkRoom(t, facade, "quest-action-room")
			char := &characters.Character{
				Entity:      &entities.Entity{ID: "char-action-" + action},
				Name:        "Questor",
				BelongsUser: *traits.BelongsToUser("user-1"),
				CurrentRoom: traits.CurrentRoom{CurrentRoomID: "quest-action-room"},
			}
			if _, err := facade.CharactersService().Store(char); err != nil {
				t.Fatalf("store character: %v", err)
			}
			target := &npc.NPC{
				Entity:      &entities.Entity{ID: "elder-action-" + action},
				Name:        "Elder",
				TemplateID:  "elder-template-" + action,
				CurrentRoom: traits.CurrentRoom{CurrentRoomID: "quest-action-room"},
				State:       "idle",
			}
			g.NPCManager.RegisterExistingNPC(target, "quest-action-room")
			template := &npc.NPC{Entity: &entities.Entity{ID: target.TemplateID}, Name: "Elder", IsTemplate: true, State: "idle"}
			if _, err := facade.NPCsService().Import(template); err != nil {
				t.Fatalf("store NPC template: %v", err)
			}
			quest := &quests.Quest{
				Entity:      &entities.Entity{ID: "Quest-" + action},
				Name:        "Action Quest",
				Description: "Quest used to test an LLM quest action.",
				Source:      quests.QuestSource{Type: "npc", NPCID: target.TemplateID},
				Objectives:  []quests.Objective{{ID: "visit", Type: quests.ObjectiveVisit, Description: "Visit the room.", TargetID: "quest-action-room", Amount: 1}},
			}
			if _, err := facade.QuestsService().Store(quest); err != nil {
				t.Fatalf("store quest: %v", err)
			}
			switch action {
			case "complete":
				if _, err := facade.QuestsService().AcceptQuest(char.ID, quest.ID); err != nil {
					t.Fatalf("accept quest: %v", err)
				}
				if _, err := facade.QuestsService().IncrementObjective(char.ID, quest.ID, "visit", 1); err != nil {
					t.Fatalf("complete objective: %v", err)
				}
			case "progress":
				if _, err := facade.QuestsService().AcceptQuest(char.ID, quest.ID); err != nil {
					t.Fatalf("accept quest: %v", err)
				}
			}
			_, actionIDs := commands.AvailableQuestPromptContext(facade, char.ID, target.TemplateID, target.ID)
			if actionIDs[quest.ID] != action {
				t.Fatalf("quest %q is not currently available for %s: %#v", quest.ID, action, actionIDs)
			}
			conv, err := facade.ConversationsService().GetOrCreateConversation(char.ID, target.ID, conversations.TargetTypeNPC, def.NPCDialogueDialogID)
			if err != nil {
				t.Fatalf("create conversation: %v", err)
			}
			payload, err := json.Marshal([]def.NPCDialogueOption{{Label: "Elegir", Kind: "quest:" + quest.ID}})
			if err != nil {
				t.Fatalf("marshal options: %v", err)
			}
			conv.SetContext("NPC", "Elder")
			conv.SetContext(def.NPCDialogueContextOptions, string(payload))
			conv.UpdateInteraction()
			if err := facade.ConversationsService().Update(conv.ID, conv); err != nil {
				t.Fatalf("update conversation: %v", err)
			}
			if !commands.DialogSelectCommand(nil, g, &messages.Message{
				FromUser:  &entities.User{Entity: &entities.Entity{ID: "user-1"}},
				Character: char,
				Data:      "1",
			}) {
				t.Fatal("quest selection was not handled")
			}
			progress, err := facade.QuestsService().GetProgress(char.ID, quest.ID)
			if err != nil {
				t.Fatalf("get progress: %v", err)
			}
			switch action {
			case "accept":
				if progress == nil || progress.Status != quests.QuestStatusActive {
					t.Fatalf("accept did not activate quest: %#v", progress)
				}
				updated, err := facade.ConversationsService().FindByID(conv.ID)
				if err != nil {
					t.Fatalf("reload conversation: %v", err)
				}
				var refreshedOptions []def.NPCDialogueOption
				if err := json.Unmarshal([]byte(updated.Context[def.NPCDialogueContextOptions]), &refreshedOptions); err != nil {
					t.Fatalf("decode refreshed options: %v", err)
				}
				if len(refreshedOptions) != 1 || !strings.Contains(refreshedOptions[0].Label, "[In Progress]") {
					t.Fatalf("LLM options were not refreshed after accept: %#v", refreshedOptions)
				}
				var refreshed bool
				for _, raw := range drainTalkMessages(g.SendMessage()) {
					if dialogMsg, ok := raw.(*messages.DialogMessage); ok && dialogMsg.ConversationID == conv.ID && len(dialogMsg.Options) == 1 && strings.Contains(dialogMsg.Options[0].Text, "[In Progress]") {
						refreshed = true
					}
				}
				if !refreshed {
					t.Fatal("expected refreshed LLM dialog after accepting a quest")
				}
			case "complete":
				if progress == nil || progress.Status != quests.QuestStatusCompleted {
					t.Fatalf("turn-in did not complete quest: %#v", progress)
				}
			case "progress":
				if progress == nil || progress.Status != quests.QuestStatusActive {
					t.Fatalf("progress action mutated quest: %#v", progress)
				}
			}
		})
	}
}
