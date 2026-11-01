package llmgen

import (
	"context"
	"strings"
	"testing"

	"github.com/talesmud/talesmud/pkg/entities/quests"
)

func contextualCatalog() QuestCatalog {
	return QuestCatalog{
		NPCs:    []CatalogRef{{ID: "npc-wolf", Name: "Lobo", Kind: "npc", Level: 4}},
		Rooms:   []CatalogRef{{ID: "R001", Name: "Claro", Kind: "room", Area: "Z1_bosque"}},
		MaxXP:   500,
		MaxGold: 200,
	}
}

func contextTestGenerator(pipe Generator) *QuestGenerator {
	return NewQuestGenerator(
		pipe,
		contextualCatalog,
		func(*quests.Quest) []QuestIssue { return nil },
	)
}

func TestGenerateContextualQuestDeterministicFallback(t *testing.T) {
	gen := contextTestGenerator(New(nil, testRegistry(t), testConfig()))
	res, err := gen.GenerateContextualQuest(context.Background(), QuestContext{
		PlayerLevel: 3,
		ZoneID:      "Z1_bosque",
	})
	if err != nil {
		t.Fatalf("GenerateContextualQuest: %v", err)
	}
	if res.Source != SourceDeterministic {
		t.Errorf("source = %q, want deterministic", res.Source)
	}
	if res.Quest == nil {
		t.Fatal("expected a quest")
	}
	if res.Quest.Area != "Z1_bosque" {
		t.Errorf("area = %q, want Z1_bosque", res.Quest.Area)
	}
	if res.Quest.Level != 3 {
		t.Errorf("level = %d, want 3", res.Quest.Level)
	}
	if len(res.Quest.Objectives) == 0 {
		t.Error("expected at least one objective")
	}
	if len(res.Issues) != 0 {
		t.Errorf("unexpected issues: %+v", res.Issues)
	}
}

func TestGenerateContextualQuestLLMSuccess(t *testing.T) {
	payload := `{"quests":[{"name":"Cazar lobos","description":"Los lobos asolan el bosque.",` +
		`"category":"side","area":"Z1_bosque","level":5,"source":{"type":"npc","npcId":"npc-wolf"},` +
		`"objectives":[{"id":"obj1","type":"kill","description":"Derrota a 3 lobos.","targetId":"npc-wolf","amount":3}],` +
		`"rewards":{"xp":120,"gold":20}}]}`
	client := &fakeClient{responses: []string{payload}, model: "test-model"}
	gen := contextTestGenerator(New(client, testRegistry(t), testConfig()))

	res, err := gen.GenerateContextualQuest(context.Background(), QuestContext{
		PlayerLevel: 5,
		ZoneID:      "Z1_bosque",
		UserID:      "u1",
	})
	if err != nil {
		t.Fatalf("GenerateContextualQuest: %v", err)
	}
	if res.Source != SourceLLM {
		t.Errorf("source = %q, want llm", res.Source)
	}
	if res.Quest == nil || res.Quest.Name != "Cazar lobos" {
		t.Fatalf("unexpected quest: %+v", res.Quest)
	}
	if client.calls != 1 {
		t.Errorf("calls = %d, want 1", client.calls)
	}
}

func TestGenerateContextualQuestRejectsInvalidContext(t *testing.T) {
	gen := contextTestGenerator(New(nil, testRegistry(t), testConfig()))

	cases := []struct {
		name string
		qc   QuestContext
	}{
		{"missing zone", QuestContext{PlayerLevel: 5}},
		{"level too low", QuestContext{PlayerLevel: 0, ZoneID: "Z1"}},
		{"level too high", QuestContext{PlayerLevel: MaxQuestLevel + 1, ZoneID: "Z1"}},
		{"negative level", QuestContext{PlayerLevel: -2, ZoneID: "Z1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := gen.GenerateContextualQuest(context.Background(), tc.qc); err == nil {
				t.Fatalf("expected error for %+v", tc.qc)
			}
		})
	}
}

func TestGenerateContextualQuestRequireLLMFailsOnBadOutput(t *testing.T) {
	client := &fakeClient{responses: []string{"not json", "still not json"}, model: "m"}
	gen := contextTestGenerator(New(client, testRegistry(t), testConfig()))

	_, err := gen.GenerateContextualQuest(context.Background(), QuestContext{
		PlayerLevel: 5,
		ZoneID:      "Z1_bosque",
		RequireLLM:  true,
	})
	if err == nil {
		t.Fatal("expected error when RequireLLM and provider returns unusable output")
	}
}

func TestGenerateContextualQuestRejectsIncompleteLLMQuest(t *testing.T) {
	payload := `{"quests":[{"name":"","description":"","objectives":[],"rewards":{"xp":0,"gold":0}}]}`
	client := &fakeClient{responses: []string{payload, payload}, model: "m"}
	gen := contextTestGenerator(New(client, testRegistry(t), testConfig()))

	_, err := gen.GenerateContextualQuest(context.Background(), QuestContext{
		PlayerLevel: 5,
		ZoneID:      "Z1_bosque",
		RequireLLM:  true,
	})
	if err == nil {
		t.Fatal("expected validation error for incomplete quest")
	}
	if !strings.Contains(err.Error(), "missing_") && !strings.Contains(err.Error(), "La quest") {
		t.Errorf("error does not describe the structural problem: %v", err)
	}
}

func TestValidateQuestStructureRejectsIncomplete(t *testing.T) {
	q := &quests.Quest{}
	issues := ValidateQuestStructure(q, 10)
	codes := map[string]bool{}
	for _, i := range issues {
		if i.Severity != "error" {
			t.Errorf("issue %q severity = %q, want error", i.Code, i.Severity)
		}
		codes[i.Code] = true
	}
	for _, want := range []string{"missing_name", "missing_description", "missing_objectives", "missing_rewards"} {
		if !codes[want] {
			t.Errorf("missing issue %q (got %+v)", want, issues)
		}
	}
}

func TestValidateQuestStructureLevelCoherence(t *testing.T) {
	base := func() *quests.Quest {
		q := &quests.Quest{Name: "Q", Description: "D", Rewards: quests.Reward{XP: 10}}
		q.Objectives = []quests.Objective{{
			ID:          "obj1",
			Type:        quests.ObjectiveKill,
			Description: "Mata",
			TargetID:    "npc-wolf",
			Amount:      1,
		}}
		return q
	}

	q := base()
	q.Level = 40
	issues := ValidateQuestStructure(q, 5)
	if !hasQuestIssue(issues, "level_out_of_range") {
		t.Errorf("expected level_out_of_range, got %+v", issues)
	}

	q = base()
	q.Level = 6
	if issues := ValidateQuestStructure(q, 5); len(issues) != 0 {
		t.Errorf("coherent level should pass, got %+v", issues)
	}

	q = base()
	q.Level = MaxQuestLevel + 1
	if issues := ValidateQuestStructure(q, MaxQuestLevel); !hasQuestIssue(issues, "level_out_of_range") {
		t.Errorf("expected level_out_of_range above cap, got %+v", issues)
	}

	q = base()
	q.Level = 0
	if issues := ValidateQuestStructure(q, 5); len(issues) != 0 {
		t.Errorf("unset level should pass (filled later), got %+v", issues)
	}
}

func TestContextualBudgetsScaleWithLevel(t *testing.T) {
	cat := QuestCatalog{MaxXP: 500, MaxGold: 200}

	lowXP, lowGold := contextualBudgets(cat, 1)
	if lowXP != 75 || lowGold != 15 {
		t.Errorf("level 1 budgets = (%d, %d), want (75, 15)", lowXP, lowGold)
	}

	highXP, highGold := contextualBudgets(cat, 50)
	if highXP != 500 || highGold != 200 {
		t.Errorf("level 50 budgets = (%d, %d), want (500, 200)", highXP, highGold)
	}

	defaultXP, defaultGold := contextualBudgets(QuestCatalog{}, 1)
	if defaultXP != 75 || defaultGold != 15 {
		t.Errorf("default budgets = (%d, %d), want (75, 15)", defaultXP, defaultGold)
	}
}

func hasQuestIssue(issues []QuestIssue, code string) bool {
	for _, i := range issues {
		if i.Code == code {
			return true
		}
	}
	return false
}
