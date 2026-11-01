package commands_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/talesmud/talesmud/pkg/entities"
	"github.com/talesmud/talesmud/pkg/entities/characters"
	npc "github.com/talesmud/talesmud/pkg/entities/npcs"
	"github.com/talesmud/talesmud/pkg/entities/traits"
	"github.com/talesmud/talesmud/pkg/mudserver/game"
	"github.com/talesmud/talesmud/pkg/mudserver/game/messages"
)

// askTestEnv bundles a live game with a room, a character and one NPC.
type askTestEnv struct {
	game      *game.Game
	character *characters.Character
	npc       *npc.NPC
}

func newAskTestEnv(t *testing.T, npcName string, llm *npc.LLMConfig) *askTestEnv {
	t.Helper()
	g, facade := newTalkTestGame(t)
	storeTalkRoom(t, facade, "ask-room")

	env := &askTestEnv{
		game: g,
		character: &characters.Character{
			Entity:      &entities.Entity{ID: "ask-char"},
			Name:        "Hero",
			Level:       3,
			BelongsUser: *traits.BelongsToUser("user-1"),
			CurrentRoom: traits.CurrentRoom{CurrentRoomID: "ask-room"},
		},
		npc: &npc.NPC{
			Entity:      &entities.Entity{ID: npcName + "-1"},
			Name:        npcName,
			CurrentRoom: traits.CurrentRoom{CurrentRoomID: "ask-room"},
			State:       "idle",
			LLM:         llm,
		},
	}
	g.NPCManager.RegisterExistingNPC(env.npc, "ask-room")
	return env
}

func (e *askTestEnv) message(data string) *messages.Message {
	return &messages.Message{
		FromUser:  &entities.User{Entity: &entities.Entity{ID: "user-1"}},
		Character: e.character,
		Data:      data,
	}
}

func (e *askTestEnv) processRaw(t *testing.T, data string) []interface{} {
	t.Helper()
	if !e.game.CommandProcessor.Process(e.game, e.message(data)) {
		t.Fatalf("ask command was not handled: %q", data)
	}
	return drainTalkMessages(e.game.SendMessage())
}

func (e *askTestEnv) process(t *testing.T, data string) []messages.MessageResponse {
	t.Helper()
	var responses []messages.MessageResponse
	for _, raw := range e.processRaw(t, data) {
		switch m := raw.(type) {
		case messages.MessageResponse:
			responses = append(responses, m)
		case *messages.MessageResponse:
			responses = append(responses, *m)
		case *messages.DialogMessage:
			responses = append(responses, m.MessageResponse)
		case messages.DialogMessage:
			responses = append(responses, m.MessageResponse)
		}
	}
	return responses
}

func askHas(responses []messages.MessageResponse, want string) bool {
	for _, r := range responses {
		if strings.Contains(r.Message, want) {
			return true
		}
	}
	return false
}

func askFromNPC(responses []messages.MessageResponse, name string) []messages.MessageResponse {
	var out []messages.MessageResponse
	for _, r := range responses {
		if r.Username == name {
			out = append(out, r)
		}
	}
	return out
}

func TestAskWithoutLLMDegradesToAuthoredDialog(t *testing.T) {
	env := newAskTestEnv(t, "Guide", nil)
	called := 0
	env.game.SetNPCDialogueProvider(func(_ context.Context, _ game.NPCDialogueRequest) (game.NPCDialogueResult, error) {
		called++
		return game.NPCDialogueResult{Reply: "should not happen"}, nil
	})

	responses := env.process(t, "ask guide where is the temple")

	if called != 0 {
		t.Fatalf("provider must not run for an NPC without LLM, called %d times", called)
	}
	if !askHas(responses, "has no free-form voice") {
		t.Fatalf("expected a degradation notice, got %v", responses)
	}
	if !askHas(responses, "doesn't seem to want to talk") {
		t.Fatalf("expected the authored talk fallback, got %v", responses)
	}
}

func TestAskCallsLLMProviderAndDeliversReply(t *testing.T) {
	env := newAskTestEnv(t, "Elder", &npc.LLMConfig{
		Enabled:     true,
		Persona:     "Un anciano que recuerda todas las historias.",
		Voice:       "anciano",
		MemoryTurns: 4,
	})

	var got game.NPCDialogueRequest
	env.game.SetNPCDialogueProvider(func(_ context.Context, req game.NPCDialogueRequest) (game.NPCDialogueResult, error) {
		got = req
		return game.NPCDialogueResult{
			Reply:       "Los antiguos muros guardan secretos, joven Hero.",
			Options:     []game.NPCDialogueOption{{Label: "Preguntar por los muros", Kind: "flavor"}, {Label: "Despedirse", Kind: "end"}},
			MemoryNotes: []string{"Preguntó por el templo"},
		}, nil
	})

	raw := env.processRaw(t, "ask elder where is the temple")
	var responses []messages.MessageResponse
	for _, rawMsg := range raw {
		switch m := rawMsg.(type) {
		case messages.MessageResponse:
			responses = append(responses, m)
		case *messages.MessageResponse:
			responses = append(responses, *m)
		case *messages.DialogMessage:
			responses = append(responses, m.MessageResponse)
		case messages.DialogMessage:
			responses = append(responses, m.MessageResponse)
		}
	}

	if got.Question != "where is the temple" {
		t.Fatalf("question = %q, want %q", got.Question, "where is the temple")
	}
	if got.NPCName != "Elder" || got.NPCTemplateID != env.npc.ID {
		t.Fatalf("unexpected NPC context: %+v", got)
	}
	if got.Persona == "" || got.Voice != "anciano" {
		t.Fatalf("persona/voice not forwarded: %+v", got)
	}
	if got.CooldownSeconds <= 0 {
		t.Fatalf("expected a positive cooldown, got %d", got.CooldownSeconds)
	}

	fromNPC := askFromNPC(responses, "Elder")
	if len(fromNPC) == 0 {
		t.Fatalf("expected a message authored by the NPC, got %v", responses)
	}
	reply := fromNPC[0].Message
	if !strings.Contains(reply, "Los antiguos muros guardan secretos") {
		t.Fatalf("reply missing: %q", reply)
	}
	var dialog *messages.DialogMessage
	for _, rawMsg := range raw {
		if dm, ok := rawMsg.(*messages.DialogMessage); ok {
			dialog = dm
			break
		}
	}
	if dialog == nil {
		t.Fatalf("expected an actionable DialogMessage, got %v", raw)
	}
	if len(dialog.Options) != 2 || dialog.Options[0].Text != "Preguntar por los muros" || dialog.Options[1].Text != "Despedirse" {
		t.Fatalf("unexpected dialog options: %+v", dialog.Options)
	}
}

func TestAskEmptyQuestionIsAllowed(t *testing.T) {
	env := newAskTestEnv(t, "Guard", &npc.LLMConfig{Enabled: true})
	var asked bool
	env.game.SetNPCDialogueProvider(func(_ context.Context, req game.NPCDialogueRequest) (game.NPCDialogueResult, error) {
		asked = true
		if req.Question != "" {
			t.Fatalf("question = %q, want empty", req.Question)
		}
		return game.NPCDialogueResult{Reply: "Buenas tardes."}, nil
	})

	responses := env.process(t, "ask guard")

	if !asked {
		t.Fatal("expected the provider to run for a plain greeting")
	}
	if !askHas(responses, "Buenas tardes.") {
		t.Fatalf("expected the greeting reply, got %v", responses)
	}
}

func TestAskRateLimitedPerCharacterAndNPC(t *testing.T) {
	env := newAskTestEnv(t, "Elder", &npc.LLMConfig{Enabled: true, CooldownSeconds: 60})
	called := 0
	env.game.SetNPCDialogueProvider(func(_ context.Context, _ game.NPCDialogueRequest) (game.NPCDialogueResult, error) {
		called++
		return game.NPCDialogueResult{Reply: "Habla, viajero."}, nil
	})

	env.process(t, "ask elder first question")
	responses := env.process(t, "ask elder second question")

	if called != 1 {
		t.Fatalf("provider calls = %d, want 1", called)
	}
	if !askHas(responses, "still thinking about your last question") {
		t.Fatalf("expected the rate-limit message, got %v", responses)
	}
}

func TestAskUnknownNPCDoesNotCallProvider(t *testing.T) {
	env := newAskTestEnv(t, "Guide", &npc.LLMConfig{Enabled: true})
	called := 0
	env.game.SetNPCDialogueProvider(func(_ context.Context, _ game.NPCDialogueRequest) (game.NPCDialogueResult, error) {
		called++
		return game.NPCDialogueResult{Reply: "x"}, nil
	})

	responses := env.process(t, "ask ghost where are you")

	if called != 0 {
		t.Fatalf("provider must not run for a missing NPC, called %d times", called)
	}
	if !askHas(responses, "There is no one named 'ghost' here.") {
		t.Fatalf("expected a not-found message, got %v", responses)
	}
}

func TestAskProviderErrorStillAnswers(t *testing.T) {
	env := newAskTestEnv(t, "Elder", &npc.LLMConfig{Enabled: true})
	env.game.SetNPCDialogueProvider(func(_ context.Context, _ game.NPCDialogueRequest) (game.NPCDialogueResult, error) {
		return game.NPCDialogueResult{}, errors.New("llm down")
	})

	responses := env.process(t, "ask elder are you well")

	fromNPC := askFromNPC(responses, "Elder")
	if len(fromNPC) == 0 {
		t.Fatalf("expected a neutral NPC line after a provider error, got %v", responses)
	}
	if !strings.Contains(fromNPC[0].Message, "silencio") {
		t.Fatalf("expected the neutral fallback, got %q", fromNPC[0].Message)
	}
}

func TestAskWithoutProviderDegradesToAuthoredDialog(t *testing.T) {
	env := newAskTestEnv(t, "Guide", &npc.LLMConfig{Enabled: true})
	env.game.SetNPCDialogueProvider(nil)

	responses := env.process(t, "ask guide anything")

	if !askHas(responses, "cannot answer freely right now") {
		t.Fatalf("expected a disabled-provider notice, got %v", responses)
	}
	if !askHas(responses, "doesn't seem to want to talk") {
		t.Fatalf("expected the authored talk fallback, got %v", responses)
	}
}
