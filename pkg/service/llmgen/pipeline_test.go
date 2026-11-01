package llmgen

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/talesmud/talesmud/pkg/entities/quests"
	"github.com/talesmud/talesmud/pkg/service/llmprompt"
)

type fakeClient struct {
	responses []string
	errs      []error
	calls     int
	lastUser  string
	model     string
}

func (f *fakeClient) Complete(_ context.Context, _ string, user string, _ float64) (string, error) {
	f.lastUser = user
	i := f.calls
	f.calls++
	if i < len(f.errs) && f.errs[i] != nil {
		return "", f.errs[i]
	}
	if i < len(f.responses) {
		return f.responses[i], nil
	}
	return "", errors.New("fake: no response")
}

func (f *fakeClient) Model() string { return f.model }

func testRegistry(t *testing.T) llmprompt.Registry {
	t.Helper()
	reg, err := llmprompt.NewRegistry("")
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	return reg
}

func testConfig() Config {
	cfg := ConfigFromEnv()
	cfg.Limiter = NewWindowLimiter(1000, time.Hour)
	cfg.Cache = NewCache(64)
	return cfg
}

func TestPipelineTextSuccess(t *testing.T) {
	client := &fakeClient{responses: []string{"Aragorn"}, model: "test-model"}
	p := New(client, testRegistry(t), testConfig())
	resp, err := p.Generate(context.Background(), Request{
		Task: TaskCharacterName,
		Vars: map[string]any{"race": "humano"},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if resp.Source != SourceLLM {
		t.Errorf("source = %q, want llm", resp.Source)
	}
	if resp.Text != "Aragorn" {
		t.Errorf("text = %q", resp.Text)
	}
	if client.calls != 1 {
		t.Errorf("calls = %d, want 1", client.calls)
	}
}

func TestPipelineServesFromCache(t *testing.T) {
	client := &fakeClient{responses: []string{"Aragorn"}, model: "test-model"}
	p := New(client, testRegistry(t), testConfig())
	req := Request{Task: TaskCharacterName, Vars: map[string]any{"race": "humano"}}
	if _, err := p.Generate(context.Background(), req); err != nil {
		t.Fatalf("first Generate: %v", err)
	}
	resp, err := p.Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("second Generate: %v", err)
	}
	if !resp.Cached || resp.Source != SourceCache {
		t.Errorf("expected cache hit, got source=%q cached=%v", resp.Source, resp.Cached)
	}
	if client.calls != 1 {
		t.Errorf("calls = %d, want 1", client.calls)
	}
}

func TestPipelineRetriesInvalidJSON(t *testing.T) {
	valid := `{"quests":[{"name":"Q","description":"D","category":"side","source":{"type":"auto"},"objectives":[{"id":"obj1","type":"custom","description":"x","checkScriptId":"s1","amount":1}],"rewards":{"xp":1}}]}`
	client := &fakeClient{responses: []string{"not json at all", valid}, model: "m"}
	p := New(client, testRegistry(t), testConfig())
	resp, err := p.Generate(context.Background(), Request{
		Task: TaskQuestGenerate,
		Vars: map[string]any{"area": "Z1", "catalog": "AREAS:\n  - Z1\n"},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if client.calls != 2 {
		t.Errorf("calls = %d, want 2", client.calls)
	}
	if resp.Attempts != 2 {
		t.Errorf("attempts = %d, want 2", resp.Attempts)
	}
	if !strings.Contains(resp.Text, "\"quests\"") {
		t.Errorf("unexpected payload: %s", resp.Text)
	}
	if resp.Source != SourceLLM {
		t.Errorf("source = %q, want llm", resp.Source)
	}
}

func TestPipelineDeterministicWhenNoClient(t *testing.T) {
	p := New(nil, testRegistry(t), testConfig())
	resp, err := p.Generate(context.Background(), Request{
		Task: TaskCharacterName,
		Vars: map[string]any{"race": "elfo"},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if resp.Source != SourceDeterministic {
		t.Errorf("source = %q, want deterministic", resp.Source)
	}
	if strings.TrimSpace(resp.Text) == "" {
		t.Error("deterministic name is empty")
	}
}

func TestPipelineRequireLLMFailsWithoutClient(t *testing.T) {
	p := New(nil, testRegistry(t), testConfig())
	_, err := p.Generate(context.Background(), Request{
		Task:       TaskCharacterName,
		Vars:       map[string]any{"race": "elfo"},
		RequireLLM: true,
	})
	if err == nil {
		t.Fatal("expected error when RequireLLM and no client")
	}
}

func TestPipelineRateLimitFallsBack(t *testing.T) {
	cfg := testConfig()
	cfg.Limiter = NewWindowLimiter(1, time.Hour)
	client := &fakeClient{responses: []string{"Uno", "Dos"}, model: "m"}
	p := New(client, testRegistry(t), cfg)
	if _, err := p.Generate(context.Background(), Request{Task: TaskCharacterName, Vars: map[string]any{"race": "x"}, UserID: "u1"}); err != nil {
		t.Fatalf("first: %v", err)
	}
	resp, err := p.Generate(context.Background(), Request{Task: TaskCharacterName, Vars: map[string]any{"race": "y"}, UserID: "u1"})
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if resp.Source != SourceDeterministic {
		t.Errorf("source = %q, want deterministic after rate limit", resp.Source)
	}
	if client.calls != 1 {
		t.Errorf("calls = %d, want 1", client.calls)
	}
}

func TestOutputAllowedRejectsDMLeaks(t *testing.T) {
	if OutputAllowed(TaskDMNarration, "El enemigo cae en la sala R101.") {
		t.Error("expected dm narration with room id to be rejected")
	}
	if OutputAllowed(TaskDMNarration, strings.Repeat("a", 300)) {
		t.Error("expected oversized dm narration to be rejected")
	}
	if !OutputAllowed(TaskDMNarration, "El silencio vuelve al lugar.") {
		t.Error("plain narration should be allowed")
	}
}

func TestQuestGeneratorDryRun(t *testing.T) {
	cat := QuestCatalog{
		NPCs:  []CatalogRef{{ID: "npc-wolf", Name: "Lobo", Kind: "npc"}},
		Rooms: []CatalogRef{{ID: "R001", Name: "Claro", Kind: "room"}},
	}
	gen := NewQuestGenerator(
		New(nil, testRegistry(t), testConfig()),
		func() QuestCatalog { return cat },
		func(q *quests.Quest) []QuestIssue { return nil },
	)
	res, err := gen.GenerateQuestsDetailed(context.Background(), GenerateQuestsRequest{
		Area:   "Z1",
		DryRun: true,
	}, func([]*quests.Quest) error {
		t.Fatal("onCommit must not run in dry-run")
		return nil
	})
	if err != nil {
		t.Fatalf("GenerateQuestsDetailed: %v", err)
	}
	if len(res.Quests) != 1 {
		t.Fatalf("quests = %d, want 1", len(res.Quests))
	}
	q := res.Quests[0]
	if q.Source.Type != "npc" || q.Source.NPCID != "npc-wolf" {
		t.Errorf("unexpected source: %+v", q.Source)
	}
	if len(q.Objectives) != 1 || q.Objectives[0].Type != quests.ObjectiveKill {
		t.Errorf("unexpected objectives: %+v", q.Objectives)
	}
	if q.ID == "" {
		t.Error("quest has no id")
	}
}

func TestExtractJSONFromFencedCode(t *testing.T) {
	raw := "Aquí tienes:\n```json\n{\"a\":1}\n```\nFin"
	payload, err := ExtractJSON(raw)
	if err != nil {
		t.Fatalf("ExtractJSON: %v", err)
	}
	var got map[string]int
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["a"] != 1 {
		t.Errorf("got %+v", got)
	}
}
