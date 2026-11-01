package server

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/talesmud/talesmud/pkg/db/sqlite"
	"github.com/talesmud/talesmud/pkg/entities"
	npc "github.com/talesmud/talesmud/pkg/entities/npcs"
	"github.com/talesmud/talesmud/pkg/entities/rooms"
	"github.com/talesmud/talesmud/pkg/mudserver/game"
	"github.com/talesmud/talesmud/pkg/mudserver/game/def"
	"github.com/talesmud/talesmud/pkg/mudserver/game/messages"
	"github.com/talesmud/talesmud/pkg/repository"
	"github.com/talesmud/talesmud/pkg/service"
	"github.com/talesmud/talesmud/pkg/service/llmgen"
	"github.com/talesmud/talesmud/pkg/service/llmprompt"
)

type npcDialogueTestClient struct {
	response    string
	err         error
	calls       int
	lastUser    string
	temperature float64
}

func (c *npcDialogueTestClient) Complete(_ context.Context, _ string, user string, temperature float64) (string, error) {
	c.calls++
	c.lastUser = user
	c.temperature = temperature
	if c.err != nil {
		return "", c.err
	}
	return c.response, nil
}

func (c *npcDialogueTestClient) Model() string { return "npc-test-model" }

func newNPCDialogueTestFacade(t *testing.T) service.Facade {
	t.Helper()
	client, err := sqlite.Open(filepath.Join(t.TempDir(), "npc-dialogue.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return service.NewFacade(repository.NewSQLiteFactory(client), nil)
}

func newNPCDialogueRuntime(t *testing.T, client *npcDialogueTestClient) llmRuntime {
	t.Helper()
	reg, err := llmprompt.NewRegistry("")
	if err != nil {
		t.Fatalf("prompt registry: %v", err)
	}
	cfg := llmgen.ConfigFromEnv()
	cfg.Limiter = llmgen.NewWindowLimiter(1000, time.Hour)
	cfg.Cache = llmgen.NewCache(16)
	return llmRuntime{registry: reg, pipeline: llmgen.New(client, reg, cfg)}
}

func npcDialogueRequest(characterID string) game.NPCDialogueRequest {
	return game.NPCDialogueRequest{
		CharacterID:     characterID,
		UserID:          "user-1",
		NPCID:           "elder-1",
		NPCTemplateID:   "elder-template",
		NPCName:         "Elder",
		CharacterName:   "Hero",
		PlayerLevel:     3,
		RoomID:          "room-1",
		Question:        "¿Qué sabes del templo?",
		Persona:         "Un anciano que recuerda todas las historias.",
		Voice:           "anciano",
		Temperature:     0.9,
		CooldownSeconds: 3,
	}
}

type npcDialogueTestMessage struct {
	messages.MessageResponse
	Options  []messages.DialogOption
	FreeText bool
}

func drainNPCDialogueMessages(ch <-chan interface{}) []npcDialogueTestMessage {
	var out []npcDialogueTestMessage
	for {
		select {
		case raw := <-ch:
			switch m := raw.(type) {
			case messages.MessageResponse:
				out = append(out, npcDialogueTestMessage{MessageResponse: m})
			case *messages.MessageResponse:
				out = append(out, npcDialogueTestMessage{MessageResponse: *m})
			case messages.DialogMessage:
				out = append(out, npcDialogueTestMessage{MessageResponse: m.MessageResponse, Options: m.Options, FreeText: m.FreeText})
			case *messages.DialogMessage:
				out = append(out, npcDialogueTestMessage{MessageResponse: m.MessageResponse, Options: m.Options, FreeText: m.FreeText})
			}
		default:
			return out
		}
	}
}

func TestInstallNPCDialogueProviderRecordsMemory(t *testing.T) {
	facade := newNPCDialogueTestFacade(t)
	if _, err := facade.RoomsService().Store(&rooms.Room{
		Entity:      &entities.Entity{ID: "room-1"},
		Name:        "Plaza",
		Description: "Una plaza.",
		Exits:       &rooms.Exits{},
	}); err != nil {
		t.Fatalf("store room: %v", err)
	}

	client := &npcDialogueTestClient{response: `{
		"reply":"Los muros recuerdan a los antiguos.",
		"options":[{"label":"Preguntar por los muros","kind":"flavor"},{"label":"Despedirse","kind":"end"}],
		"end":false,
		"memoryNotes":["Preguntó por el templo","Es respetuoso"]
	}`}
	rt := newNPCDialogueRuntime(t, client)
	g := game.New(facade)
	rt.installNPCDialogueProvider(g, facade)

	status := g.RequestNPCDialogue(npcDialogueRequest("char-1"))
	if status != def.NPCDialogueScheduled {
		t.Fatalf("status = %v, want scheduled", status)
	}

	msgs := drainNPCDialogueMessages(g.SendMessage())
	if len(msgs) != 1 {
		t.Fatalf("messages = %d, want 1: %#v", len(msgs), msgs)
	}
	if msgs[0].Username != "Elder" {
		t.Fatalf("username = %q, want Elder", msgs[0].Username)
	}
	if !strings.Contains(msgs[0].Message, "Los muros recuerdan") {
		t.Fatalf("unexpected reply text: %q", msgs[0].Message)
	}
	if len(msgs[0].Options) != 2 || msgs[0].Options[0].Text != "Preguntar por los muros" {
		t.Fatalf("unexpected dialog options: %+v", msgs[0].Options)
	}
	if msgs[0].Options[0].Kind != "flavor" || msgs[0].Options[1].Kind != "end" || !msgs[0].FreeText {
		t.Fatalf("dialog choice contract missing kind/freeText: %+v", msgs[0])
	}

	if client.temperature != 0.9 {
		t.Fatalf("temperature = %v, want 0.9", client.temperature)
	}
	if !strings.Contains(client.lastUser, "¿Qué sabes del templo?") {
		t.Fatalf("question not rendered as untrusted input: %q", client.lastUser)
	}
	if !strings.Contains(client.lastUser, "Un anciano que recuerda") {
		t.Fatalf("persona not rendered: %q", client.lastUser)
	}

	mem, err := facade.NPCMemoriesService().Get("elder-template", "char-1")
	if err != nil {
		t.Fatalf("get memory: %v", err)
	}
	if mem == nil {
		t.Fatal("expected a recorded memory row")
	}
	if mem.Turns != 1 || mem.LastUserLine != "¿Qué sabes del templo?" {
		t.Fatalf("unexpected memory: %+v", mem)
	}
	if len(mem.Facts) != 2 || mem.Facts[0] != "Preguntó por el templo" {
		t.Fatalf("memory notes not recorded: %+v", mem.Facts)
	}
}

func TestInstallNPCDialogueProviderSendsNeutralOnError(t *testing.T) {
	facade := newNPCDialogueTestFacade(t)
	client := &npcDialogueTestClient{err: errors.New("provider down")}
	rt := newNPCDialogueRuntime(t, client)
	g := game.New(facade)
	rt.installNPCDialogueProvider(g, facade)

	status := g.RequestNPCDialogue(npcDialogueRequest("char-2"))
	if status != def.NPCDialogueScheduled {
		t.Fatalf("status = %v, want scheduled", status)
	}

	msgs := drainNPCDialogueMessages(g.SendMessage())
	if len(msgs) != 1 {
		t.Fatalf("messages = %d, want 1: %#v", len(msgs), msgs)
	}
	if msgs[0].Username != "Elder" || !strings.Contains(msgs[0].Message, "silencio") {
		t.Fatalf("expected the neutral NPC fallback, got %#v", msgs[0])
	}

	mem, err := facade.NPCMemoriesService().Get("elder-template", "char-2")
	if err != nil {
		t.Fatalf("get memory: %v", err)
	}
	if mem != nil {
		t.Fatalf("failed generation must not record memory: %+v", mem)
	}
}

func TestInstallNPCDialogueProviderRejectsEmptyReply(t *testing.T) {
	facade := newNPCDialogueTestFacade(t)
	client := &npcDialogueTestClient{response: `{"reply":"","options":[],"end":false,"memoryNotes":[]}`}
	rt := newNPCDialogueRuntime(t, client)
	g := game.New(facade)
	rt.installNPCDialogueProvider(g, facade)

	// Empty replies fail validation. With retries exhausted and RequireLLM
	// set, the provider returns an error and the game sends the neutral line.
	g.RequestNPCDialogue(npcDialogueRequest("char-3"))
	msgs := drainNPCDialogueMessages(g.SendMessage())
	if len(msgs) != 1 || !strings.Contains(msgs[0].Message, "silencio") {
		t.Fatalf("expected the neutral fallback for an empty reply, got %#v", msgs)
	}
	if client.calls != 2 {
		t.Fatalf("calls = %d, want the template's bounded 2 attempts", client.calls)
	}
}

func TestInstallNPCDialogueProviderMemoryPersistsAcrossTurns(t *testing.T) {
	facade := newNPCDialogueTestFacade(t)
	if _, err := facade.RoomsService().Store(&rooms.Room{
		Entity:      &entities.Entity{ID: "room-1"},
		Name:        "Plaza",
		Description: "Una plaza.",
		Exits:       &rooms.Exits{},
	}); err != nil {
		t.Fatalf("store room: %v", err)
	}

	client := &npcDialogueTestClient{}
	rt := newNPCDialogueRuntime(t, client)
	g := game.New(facade)
	rt.installNPCDialogueProvider(g, facade)

	client.response = "{\n\t\t\"reply\":\"Los muros recuerdan a los antiguos.\",\n\t\t\"options\":[{\"label\":\"Preguntar por los muros\",\"kind\":\"flavor\"}],\n\t\t\"end\":false,\n\t\t\"memoryNotes\":[\"Preguntó por el templo\"]\n\t}"
	req := npcDialogueRequest("char-1")
	status := g.RequestNPCDialogue(req)
	if status != def.NPCDialogueScheduled {
		t.Fatalf("first turn status = %v, want scheduled", status)
	}
	msgs := drainNPCDialogueMessages(g.SendMessage())
	if len(msgs) != 1 {
		t.Fatalf("first turn messages = %d", len(msgs))
	}

	mem, err := facade.NPCMemoriesService().Get("elder-template", "char-1")
	if err != nil {
		t.Fatalf("get memory: %v", err)
	}
	if mem == nil || mem.Turns != 1 || mem.LastUserLine != "¿Qué sabes del templo?" {
		t.Fatalf("unexpected first memory: %+v", mem)
	}
	if len(mem.Facts) != 1 || mem.Facts[0] != "Preguntó por el templo" {
		t.Fatalf("first facts missing: %+v", mem.Facts)
	}

	client.response = "{\n\t\t\"reply\":\"Si, lo he visto antes.\",\n\t\t\"options\":[],\n\t\t\"end\":false,\n\t\t\"memoryNotes\":[\"Es curioso\"]\n\t}"
	g2 := game.New(facade)
	rt.installNPCDialogueProvider(g2, facade)
	req2 := npcDialogueRequest("char-1")
	req2.Question = "¿Volverás a contármelo?"
	req2.CooldownSeconds = 0
	status = g2.RequestNPCDialogue(req2)
	if status != def.NPCDialogueScheduled {
		t.Fatalf("second turn status = %v, want scheduled", status)
	}
	msgs = drainNPCDialogueMessages(g2.SendMessage())
	if len(msgs) != 1 {
		t.Fatalf("second turn messages = %d", len(msgs))
	}

	if !strings.Contains(client.lastUser, "Memoria:") {
		t.Fatalf("memory block not injected into prompt: %q", client.lastUser)
	}
	if !strings.Contains(client.lastUser, "Preguntó por el templo") && !strings.Contains(client.lastUser, "Última pregunta") {
		t.Fatalf("previous memory not present in prompt: %q", client.lastUser)
	}
	if !strings.Contains(client.lastUser, "¿Volverás a contármelo?") {
		t.Fatalf("current question missing: %q", client.lastUser)
	}

	mem2, err := facade.NPCMemoriesService().Get("elder-template", "char-1")
	if err != nil {
		t.Fatalf("get memory2: %v", err)
	}
	if mem2 == nil || mem2.Turns != 2 {
		t.Fatalf("unexpected second memory: %+v", mem2)
	}
	if mem2.LastUserLine != "¿Volverás a contármelo?" {
		t.Fatalf("last user line not updated: %q", mem2.LastUserLine)
	}
	found := false
	for _, f := range mem2.Facts {
		if f == "Es curioso" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("second turn facts not merged: %+v", mem2.Facts)
	}
}

func storeNPCDialogueLLMConfig(t *testing.T, facade service.Facade, cfg *npc.LLMConfig) {
	t.Helper()
	if _, err := facade.NPCsService().Import(&npc.NPC{
		Entity: &entities.Entity{ID: "elder-1"},
		Name:   "Elder",
		LLM:    cfg,
	}); err != nil {
		t.Fatalf("store npc: %v", err)
	}
}

func TestNPCDialoguePassesSecretsToPrompt(t *testing.T) {
	facade := newNPCDialogueTestFacade(t)
	secret := "El rey duerme bajo el altar."
	storeNPCDialogueLLMConfig(t, facade, &npc.LLMConfig{
		Enabled: true,
		Persona: "Soy el guardián de la cripta.",
		Secrets: []string{secret},
	})

	client := &npcDialogueTestClient{response: `{
		"reply":"Las historias son largas.",
		"options":[],
		"end":false,
		"memoryNotes":[]
	}`}
	rt := newNPCDialogueRuntime(t, client)
	g := game.New(facade)
	rt.installNPCDialogueProvider(g, facade)

	if status := g.RequestNPCDialogue(npcDialogueRequest("char-secret")); status != def.NPCDialogueScheduled {
		t.Fatalf("status = %v, want scheduled", status)
	}
	msgs := drainNPCDialogueMessages(g.SendMessage())
	if len(msgs) != 1 {
		t.Fatalf("messages = %d, want 1: %#v", len(msgs), msgs)
	}
	if !strings.Contains(client.lastUser, secret) {
		t.Fatalf("secret not passed to prompt: %q", client.lastUser)
	}
	if !strings.Contains(client.lastUser, "Secretos (internos") {
		t.Fatalf("secret block missing from prompt: %q", client.lastUser)
	}
	if strings.Contains(msgs[0].Message, secret) {
		t.Fatalf("reply leaked secret: %q", msgs[0].Message)
	}
}

func TestNPCDialogueRejectsLiteralSecretLeak(t *testing.T) {
	facade := newNPCDialogueTestFacade(t)
	secret := "El rey duerme bajo el altar."
	storeNPCDialogueLLMConfig(t, facade, &npc.LLMConfig{
		Enabled: true,
		Secrets: []string{secret},
	})

	client := &npcDialogueTestClient{response: `{
		"reply":"El rey duerme bajo el altar.",
		"options":[],
		"end":false,
		"memoryNotes":[]
	}`}
	rt := newNPCDialogueRuntime(t, client)
	g := game.New(facade)
	rt.installNPCDialogueProvider(g, facade)

	if status := g.RequestNPCDialogue(npcDialogueRequest("char-leak")); status != def.NPCDialogueScheduled {
		t.Fatalf("status = %v, want scheduled", status)
	}
	msgs := drainNPCDialogueMessages(g.SendMessage())
	if len(msgs) != 1 {
		t.Fatalf("messages = %d, want 1: %#v", len(msgs), msgs)
	}
	if strings.Contains(msgs[0].Message, secret) {
		t.Fatalf("secret leaked to player: %q", msgs[0].Message)
	}
	if !strings.Contains(msgs[0].Message, "silencio") {
		t.Fatalf("expected the neutral fallback after a rejected leak, got %q", msgs[0].Message)
	}
	if client.calls != 2 {
		t.Fatalf("calls = %d, want the template's bounded 2 attempts", client.calls)
	}
}

func TestValidateNPCDialoguePayloadWithSecretsChecksOptionsAndMemoryNotes(t *testing.T) {
	secret := "El rey duerme bajo el altar."
	cfg := &npc.LLMConfig{Secrets: []string{secret}}
	payload := json.RawMessage(`{
		"reply":"Las historias son largas.",
		"options":[
			{"label":"El rey duerme bajo el altar.","kind":"flavor"},
			{"label":"Despedirse","kind":"end"}
		],
		"end":false,
		"memoryNotes":["El rey duerme bajo el altar.","Es respetuoso"]
	}`)

	issues := validateNPCDialoguePayloadWithSecrets(cfg)(context.Background(), llmgen.Request{}, payload)
	leaks := map[string]bool{}
	for _, issue := range issues {
		if issue.Code == "secret_leak" {
			leaks[issue.Path] = true
		}
	}
	if !leaks["options[0].label"] {
		t.Fatalf("option label leak not reported: %+v", issues)
	}
	if !leaks["memoryNotes[0]"] {
		t.Fatalf("memory note leak not reported: %+v", issues)
	}
	if leaks["options[1].label"] || leaks["memoryNotes[1]"] || leaks["reply"] {
		t.Fatalf("innocent fields flagged as leaks: %+v", issues)
	}
}

func TestNPCDialogueQuestKindParsingAndValidation(t *testing.T) {
	parsed, err := parseNPCDialoguePayload(`{"reply":"Hola","options":[{"label":"Aceptar","kind":"Quest:Quest-MixedCase"}],"end":false,"memoryNotes":[]}`)
	if err != nil {
		t.Fatalf("parse quest payload: %v", err)
	}
	var result game.NPCDialogueResult
	if err := json.Unmarshal(parsed, &result); err != nil {
		t.Fatalf("decode parsed payload: %v", err)
	}
	if len(result.Options) != 1 || result.Options[0].Kind != "quest:Quest-MixedCase" {
		t.Fatalf("quest kind case not preserved: %+v", result.Options)
	}
	allowed := map[string]string{"Quest-MixedCase": "accept"}
	if issues := validateNPCDialoguePayloadWithAllowedQuests(parsed, allowed); len(issues) != 0 {
		t.Fatalf("known quest should validate, got %+v", issues)
	}
	for _, kind := range []string{"quest:", "quest:Unknown-Quest"} {
		payload := json.RawMessage(`{"reply":"Hola","options":[{"label":"Elegir","kind":"` + kind + `"}],"end":false}`)
		issues := validateNPCDialoguePayloadWithAllowedQuests(payload, allowed)
		if len(issues) == 0 {
			t.Fatalf("expected invalid quest kind %q to be rejected", kind)
		}
		wantCode := "unknown_quest"
		if kind == "quest:" {
			wantCode = "bad_option_kind"
		}
		found := false
		for _, issue := range issues {
			if issue.Code == wantCode {
				found = true
			}
		}
		if !found {
			t.Fatalf("expected %s for %q, got %+v", wantCode, kind, issues)
		}
	}
	unknownKind := json.RawMessage(`{"reply":"Hola","options":[{"label":"Elegir","kind":"surprise"}],"end":false}`)
	if issues := validateNPCDialoguePayloadWithAllowedQuests(unknownKind, allowed); len(issues) == 0 || issues[0].Code != "bad_option_kind" {
		t.Fatalf("unknown option kind should be rejected, got %+v", issues)
	}
}

func TestNPCDialogueRejectsSecretLeakInOptionsAndMemoryNotes(t *testing.T) {
	secret := "El rey duerme bajo el altar."
	cases := []struct {
		name     string
		response string
	}{
		{
			name: "option label",
			response: `{"reply":"Las historias son largas.","options":[{"label":"` + secret +
				`","kind":"flavor"}],"end":false,"memoryNotes":[]}`,
		},
		{
			name: "memory note",
			response: `{"reply":"Las historias son largas.","options":[],"end":false,"memoryNotes":["` +
				secret + `"]}`,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			facade := newNPCDialogueTestFacade(t)
			storeNPCDialogueLLMConfig(t, facade, &npc.LLMConfig{
				Enabled: true,
				Secrets: []string{secret},
			})
			client := &npcDialogueTestClient{response: tt.response}
			rt := newNPCDialogueRuntime(t, client)
			g := game.New(facade)
			rt.installNPCDialogueProvider(g, facade)

			if status := g.RequestNPCDialogue(npcDialogueRequest("char-" + tt.name)); status != def.NPCDialogueScheduled {
				t.Fatalf("status = %v, want scheduled", status)
			}
			msgs := drainNPCDialogueMessages(g.SendMessage())
			if len(msgs) != 1 {
				t.Fatalf("messages = %d, want 1: %#v", len(msgs), msgs)
			}
			if strings.Contains(msgs[0].Message, secret) {
				t.Fatalf("secret leaked to player: %q", msgs[0].Message)
			}
			if !strings.Contains(msgs[0].Message, "silencio") {
				t.Fatalf("expected the neutral fallback after a rejected leak, got %q", msgs[0].Message)
			}
			if client.calls != 2 {
				t.Fatalf("calls = %d, want the template's bounded 2 attempts", client.calls)
			}
			mem, err := facade.NPCMemoriesService().Get("elder-template", "char-"+tt.name)
			if err != nil {
				t.Fatalf("get memory: %v", err)
			}
			if mem != nil {
				t.Fatalf("rejected generation must not record memory: %+v", mem)
			}
		})
	}
}

func TestInstallNPCDialogueProviderAdjustsRelation(t *testing.T) {
	facade := newNPCDialogueTestFacade(t)
	if _, err := facade.RoomsService().Store(&rooms.Room{
		Entity:      &entities.Entity{ID: "room-1"},
		Name:        "Plaza",
		Description: "Una plaza.",
		Exits:       &rooms.Exits{},
	}); err != nil {
		t.Fatalf("store room: %v", err)
	}

	client := &npcDialogueTestClient{response: `{
		"reply":"Me alegra verte.",
		"options":[],
		"end":false,
		"memoryNotes":[],
		"relationDelta":0.04
	}`}
	rt := newNPCDialogueRuntime(t, client)
	g := game.New(facade)
	rt.installNPCDialogueProvider(g, facade)

	if status := g.RequestNPCDialogue(npcDialogueRequest("char-rel")); status != def.NPCDialogueScheduled {
		t.Fatalf("status = %v, want scheduled", status)
	}
	_ = drainNPCDialogueMessages(g.SendMessage())

	mem, err := facade.NPCMemoriesService().Get("elder-template", "char-rel")
	if err != nil {
		t.Fatalf("get memory: %v", err)
	}
	if mem == nil || mem.Relations["player"] != 0.04 {
		t.Fatalf("unexpected relation: %+v", mem)
	}

	// The next prompt must include the dynamic sentiment.
	client.response = `{"reply":"Otra vez tú.","options":[],"end":false,"memoryNotes":[],"relationDelta":-0.20}`
	g2 := game.New(facade)
	rt.installNPCDialogueProvider(g2, facade)
	req := npcDialogueRequest("char-rel")
	req.Continuation = true
	if status := g2.RequestNPCDialogue(req); status != def.NPCDialogueScheduled {
		t.Fatalf("second status = %v, want scheduled", status)
	}
	_ = drainNPCDialogueMessages(g2.SendMessage())
	if !strings.Contains(client.lastUser, "player(dinámica,0.04)") {
		t.Fatalf("dynamic relation missing from prompt: %q", client.lastUser)
	}
	mem, err = facade.NPCMemoriesService().Get("elder-template", "char-rel")
	if err != nil {
		t.Fatalf("get memory2: %v", err)
	}
	if diff := mem.Relations["player"] - (-0.01); diff < -1e-9 || diff > 1e-9 {
		t.Fatalf("relation = %v, want -0.01 after clamped delta", mem.Relations["player"])
	}
}

func TestNPCRelationDeltaIncludedInDialogMessage(t *testing.T) {
	facade := newNPCDialogueTestFacade(t)
	if _, err := facade.RoomsService().Store(&rooms.Room{
		Entity:      &entities.Entity{ID: "room-1"},
		Name:        "Plaza",
		Description: "Una plaza.",
		Exits:       &rooms.Exits{},
	}); err != nil {
		t.Fatalf("store room: %v", err)
	}
	client := &npcDialogueTestClient{response: `{"reply":"Me alegra verte.","options":[{"label":"Seguir hablando","kind":"flavor"}],"end":false,"memoryNotes":[],"relationDelta":0.04}`}
	rt := newNPCDialogueRuntime(t, client)
	g := game.New(facade)
	rt.installNPCDialogueProvider(g, facade)
	if status := g.RequestNPCDialogue(npcDialogueRequest("char-rel-msg")); status != def.NPCDialogueScheduled {
		t.Fatalf("status = %v, want scheduled", status)
	}

	var got *npcDialogueTestMessage
	for _, msg := range drainNPCDialogueMessages(g.SendMessage()) {
		if msg.Type == messages.MessageTypeDialog {
			copy := msg
			got = &copy
		}
	}
	if got == nil {
		t.Fatal("expected dialog message")
	}
	if got.RelationDelta != 0.04 {
		t.Fatalf("relationDelta = %v, want 0.04", got.RelationDelta)
	}
}
