package commands_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/talesmud/talesmud/pkg/db/sqlite"
	"github.com/talesmud/talesmud/pkg/entities"
	"github.com/talesmud/talesmud/pkg/entities/characters"
	npc "github.com/talesmud/talesmud/pkg/entities/npcs"
	"github.com/talesmud/talesmud/pkg/entities/quests"
	"github.com/talesmud/talesmud/pkg/entities/rooms"
	"github.com/talesmud/talesmud/pkg/entities/traits"
	"github.com/talesmud/talesmud/pkg/mudserver/game"
	"github.com/talesmud/talesmud/pkg/mudserver/game/messages"
	"github.com/talesmud/talesmud/pkg/repository"
	"github.com/talesmud/talesmud/pkg/service"
	"github.com/talesmud/talesmud/pkg/service/llmgen"
	"github.com/talesmud/talesmud/pkg/service/llmprompt"
)

// offerTestEnv bundles the live game plus the entities a contextual offer test
// needs.
type offerTestEnv struct {
	game       *game.Game
	facade     service.Facade
	character  *characters.Character
	questGiver *npc.NPC
	bystander  *npc.NPC
}

func newOfferTestEnv(t *testing.T) *offerTestEnv {
	t.Helper()

	client, err := sqlite.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	facade := service.NewFacade(repository.NewSQLiteFactory(client), nil)
	g := game.New(facade)
	env := &offerTestEnv{game: g, facade: facade}

	if _, err := facade.RoomsService().Store(&rooms.Room{
		Entity:      &entities.Entity{ID: "offer-room"},
		Name:        "Town Square",
		Description: "A bustling square.",
		Area:        "Z1_training",
		Exits:       &rooms.Exits{},
	}); err != nil {
		t.Fatalf("store room: %v", err)
	}

	env.character = env.storeCharacter(t, "offer-char", "Hero", "offer-room")
	env.questGiver = env.storeQuestGiver(t, "offer-giver", "Elder", "offer-room")
	env.bystander = env.storeBystander(t, "offer-bystander", "Drifter", "offer-room")

	installOfferProvider(t, g, llmgen.QuestCatalog{
		NPCs: []llmgen.CatalogRef{
			{ID: env.questGiver.ID, Name: env.questGiver.Name, Kind: "npc", Level: 3},
		},
		Rooms: []llmgen.CatalogRef{
			{ID: "offer-room", Name: "Town Square", Kind: "room", Area: "Z1_training"},
		},
		MaxXP:   500,
		MaxGold: 200,
	})

	return env
}

func (e *offerTestEnv) storeCharacter(t *testing.T, id, name, roomID string) *characters.Character {
	t.Helper()
	char, err := e.facade.CharactersService().Store(&characters.Character{
		Entity:      &entities.Entity{ID: id},
		Name:        name,
		Level:       3,
		BelongsUser: *traits.BelongsToUser("user-1"),
		CurrentRoom: traits.CurrentRoom{CurrentRoomID: roomID},
	})
	if err != nil {
		t.Fatalf("store character: %v", err)
	}
	return char
}

func (e *offerTestEnv) storeQuestGiver(t *testing.T, id, name, roomID string) *npc.NPC {
	t.Helper()
	giver := &npc.NPC{
		Entity:      &entities.Entity{ID: id},
		Name:        name,
		CurrentRoom: traits.CurrentRoom{CurrentRoomID: roomID},
		State:       "idle",
	}
	e.game.NPCManager.RegisterExistingNPC(giver, roomID)
	if _, err := e.facade.NPCsService().Import(giver); err != nil {
		t.Fatalf("import quest giver: %v", err)
	}
	// An authored quest marks the NPC as a quest giver (IsQuestGiver contract).
	authored := &quests.Quest{
		Entity:      &entities.Entity{ID: id + "-authored"},
		Name:        "Old Business",
		Description: "Already known work.",
		Source:      quests.QuestSource{Type: "npc", NPCID: id},
		Objectives: []quests.Objective{{
			ID:          "authored-obj",
			Type:        quests.ObjectiveTalk,
			Description: "Talk to the Elder.",
			TargetID:    id,
			Amount:      1,
		}},
	}
	if _, err := e.facade.QuestsService().Store(authored); err != nil {
		t.Fatalf("store authored quest: %v", err)
	}
	return giver
}

func (e *offerTestEnv) storeBystander(t *testing.T, id, name, roomID string) *npc.NPC {
	t.Helper()
	bystander := &npc.NPC{
		Entity:      &entities.Entity{ID: id},
		Name:        name,
		CurrentRoom: traits.CurrentRoom{CurrentRoomID: roomID},
		State:       "idle",
	}
	e.game.NPCManager.RegisterExistingNPC(bystander, roomID)
	return bystander
}

// installOfferProvider wires the real llmgen quest generator with a nil client,
// exercising the deterministic fallback exactly like LLM_PROVIDER=none does.
func installOfferProvider(t *testing.T, g *game.Game, catalog llmgen.QuestCatalog) {
	t.Helper()
	reg, err := llmprompt.NewRegistry("")
	if err != nil {
		t.Fatalf("prompt registry: %v", err)
	}
	cfg := llmgen.ConfigFromEnv()
	cfg.Limiter = llmgen.NewWindowLimiter(1000, time.Hour)
	gen := llmgen.NewQuestGenerator(llmgen.New(nil, reg, cfg), func() llmgen.QuestCatalog {
		return catalog
	}, nil)

	g.SetContextualQuestProvider(func(ctx context.Context, req game.ContextualQuestRequest) (*quests.Quest, error) {
		res, err := gen.GenerateContextualQuest(ctx, llmgen.QuestContext{
			PlayerLevel: req.PlayerLevel,
			ZoneID:      req.ZoneID,
			Theme:       req.Theme,
			SourceNPCID: req.SourceNPCID,
			CharacterID: req.CharacterID,
			UserID:      req.UserID,
		})
		if err != nil {
			return nil, err
		}
		return res.Quest, nil
	})
}

func (e *offerTestEnv) message(data string) *messages.Message {
	return e.messageFor(e.character, "user-1", data)
}

func (e *offerTestEnv) messageFor(char *characters.Character, userID, data string) *messages.Message {
	return &messages.Message{
		FromUser:  &entities.User{Entity: &entities.Entity{ID: userID}},
		Character: char,
		Data:      data,
	}
}

func offerTexts(msgs []interface{}) []string {
	var texts []string
	for _, raw := range msgs {
		switch m := raw.(type) {
		case messages.MessageResponse:
			texts = append(texts, m.Message)
		case *messages.MessageResponse:
			texts = append(texts, m.Message)
		}
	}
	return texts
}

func containsSubstr(texts []string, want string) bool {
	for _, text := range texts {
		if strings.Contains(text, want) {
			return true
		}
	}
	return false
}

func TestOfferQuestGeneratesForQuestGiver(t *testing.T) {
	env := newOfferTestEnv(t)

	if !env.game.CommandProcessor.Process(env.game, env.message("offer elder")) {
		t.Fatal("offer command was not handled")
	}

	texts := offerTexts(drainTalkMessages(env.game.SendMessage()))
	if !containsSubstr(texts, "Quest Available") {
		t.Fatalf("expected a quest offer, got %v", texts)
	}
	if !containsSubstr(texts, "Objectives:") {
		t.Fatalf("expected objectives in the offer, got %v", texts)
	}
	if !containsSubstr(texts, "offer accept") {
		t.Fatalf("expected accept hint in the offer, got %v", texts)
	}

	if _, ok := env.game.PendingContextualQuest(env.character.ID); !ok {
		t.Fatal("expected a pending contextual quest after offering")
	}
}

// TestOfferQuestUsesCharacterAndRoomContext pins the Phase 6.4 wiring: the
// generator must receive the requesting character's level and the area of the
// room they are standing in, not generic defaults.
func TestOfferQuestUsesCharacterAndRoomContext(t *testing.T) {
	env := newOfferTestEnv(t)

	if !env.game.CommandProcessor.Process(env.game, env.message("offer elder")) {
		t.Fatal("offer command was not handled")
	}
	drainTalkMessages(env.game.SendMessage())

	pending, ok := env.game.PendingContextualQuest(env.character.ID)
	if !ok || pending == nil {
		t.Fatal("expected a pending contextual quest")
	}
	if pending.Area != "Z1_training" {
		t.Fatalf("expected the quest area from the current room, got %q", pending.Area)
	}
	// The deterministic fallback stamps the requesting player's level.
	if pending.Level != int32(env.character.Level) {
		t.Fatalf("expected quest level %d from the character, got %d", env.character.Level, pending.Level)
	}
	if len(pending.Objectives) == 0 {
		t.Fatal("expected at least one objective on the offered quest")
	}
	if pending.Rewards.XP <= 0 && pending.Rewards.Gold <= 0 && len(pending.Rewards.ItemTemplateIDs) == 0 {
		t.Fatal("expected a non-empty reward on the offered quest")
	}
}

func TestOfferQuestSkippedForNonQuestGiver(t *testing.T) {
	env := newOfferTestEnv(t)

	if !env.game.CommandProcessor.Process(env.game, env.message("offer drifter")) {
		t.Fatal("offer command was not handled")
	}

	texts := offerTexts(drainTalkMessages(env.game.SendMessage()))
	if containsSubstr(texts, "Quest Available") {
		t.Fatalf("non quest-giver should not generate a quest, got %v", texts)
	}
	if !containsSubstr(texts, "has no work to offer you") {
		t.Fatalf("expected refusal message, got %v", texts)
	}
	if _, ok := env.game.PendingContextualQuest(env.character.ID); ok {
		t.Fatal("non quest-giver must not leave a pending offer")
	}
}

func TestOfferQuestAcceptAddsToLog(t *testing.T) {
	env := newOfferTestEnv(t)

	if !env.game.CommandProcessor.Process(env.game, env.message("offer elder")) {
		t.Fatal("offer command was not handled")
	}
	pending, ok := env.game.PendingContextualQuest(env.character.ID)
	if !ok || pending == nil {
		t.Fatal("expected a pending offer before accepting")
	}
	drainTalkMessages(env.game.SendMessage())

	if !env.game.CommandProcessor.Process(env.game, env.message("offer accept")) {
		t.Fatal("offer accept was not handled")
	}
	drainTalkMessages(env.game.SendMessage())

	progress, err := env.facade.QuestsService().GetProgress(env.character.ID, pending.ID)
	if err != nil {
		t.Fatalf("get progress: %v", err)
	}
	if progress == nil || progress.Status != quests.QuestStatusActive {
		t.Fatalf("expected accepted quest, got %#v", progress)
	}
	if _, ok := env.game.PendingContextualQuest(env.character.ID); ok {
		t.Fatal("pending offer should be cleared after acceptance")
	}
}

func TestOfferQuestRateLimitedPerCharacter(t *testing.T) {
	env := newOfferTestEnv(t)

	if !env.game.CommandProcessor.Process(env.game, env.message("offer elder")) {
		t.Fatal("first offer was not handled")
	}
	drainTalkMessages(env.game.SendMessage())

	if !env.game.CommandProcessor.Process(env.game, env.message("offer elder")) {
		t.Fatal("second offer was not handled")
	}
	texts := offerTexts(drainTalkMessages(env.game.SendMessage()))
	if !containsSubstr(texts, "still considering") {
		t.Fatalf("expected rate-limit message, got %v", texts)
	}
}

// TestOfferQuestRateLimitIsPerCharacter proves the cooldown is keyed by
// character id, so one player spinning the command cannot starve another.
func TestOfferQuestRateLimitIsPerCharacter(t *testing.T) {
	env := newOfferTestEnv(t)

	// The first character spends their cooldown.
	if !env.game.CommandProcessor.Process(env.game, env.message("offer elder")) {
		t.Fatal("first character offer was not handled")
	}
	drainTalkMessages(env.game.SendMessage())

	// A second character is not gated by the first one's cooldown.
	other := env.storeCharacter(t, "offer-char-2", "Sidekick", "offer-room")
	if !env.game.CommandProcessor.Process(env.game, env.messageFor(other, "user-2", "offer elder")) {
		t.Fatal("second character offer was not handled")
	}
	texts := offerTexts(drainTalkMessages(env.game.SendMessage()))
	if !containsSubstr(texts, "Quest Available") {
		t.Fatalf("expected the second character to get an offer, got %v", texts)
	}
	if _, ok := env.game.PendingContextualQuest(other.ID); !ok {
		t.Fatal("expected a pending offer for the second character")
	}
}

func TestOfferQuestDisabledWithoutProvider(t *testing.T) {
	env := newOfferTestEnv(t)
	env.game.SetContextualQuestProvider(nil)

	if !env.game.CommandProcessor.Process(env.game, env.message("offer elder")) {
		t.Fatal("offer command was not handled")
	}
	texts := offerTexts(drainTalkMessages(env.game.SendMessage()))
	if !containsSubstr(texts, "has nothing planned for you") {
		t.Fatalf("expected disabled message, got %v", texts)
	}
}
