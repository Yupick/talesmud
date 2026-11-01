package service

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/talesmud/talesmud/pkg/db/sqlite"
	"github.com/talesmud/talesmud/pkg/repository"
)

func newNPCMemoriesTestService(t *testing.T) NPCMemoriesService {
	t.Helper()
	client, err := sqlite.Open(filepath.Join(t.TempDir(), "npc-memories.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return NewNPCMemoriesServiceWithConfig(
		repository.NewSQLiteFactory(client).NPCMemories(),
		NPCMemoryConfig{SummaryAtTurns: 2},
	)
}

func TestRecordTurnRebuildsDeterministicSummary(t *testing.T) {
	svc := newNPCMemoriesTestService(t)

	if _, err := svc.RecordTurn("npc-elder", "char-1", "first question", []string{"first fact"}); err != nil {
		t.Fatalf("first turn: %v", err)
	}
	mem, err := svc.RecordTurn("npc-elder", "char-1", "second question", []string{"second fact"})
	if err != nil {
		t.Fatalf("second turn: %v", err)
	}
	if mem.SummarySource != repository.NPCMemorySummaryDeterministic {
		t.Fatalf("summary source = %q, want deterministic", mem.SummarySource)
	}
	if !strings.Contains(mem.Summary, "first fact") || !strings.Contains(mem.Summary, "second fact") {
		t.Fatalf("deterministic summary missing facts: %q", mem.Summary)
	}

	mem, err = svc.RecordTurn("npc-elder", "char-1", "third question", []string{"third fact"})
	if err != nil {
		t.Fatalf("third turn: %v", err)
	}
	if mem.SummarySource != repository.NPCMemorySummaryDeterministic {
		t.Fatalf("summary source = %q, want deterministic", mem.SummarySource)
	}
	if !strings.Contains(mem.Summary, "third fact") {
		t.Fatalf("deterministic summary did not refresh: %q", mem.Summary)
	}
}

func TestRecordTurnPreservesLLMSummary(t *testing.T) {
	svc := newNPCMemoriesTestService(t)

	if _, err := svc.RecordTurn("npc-elder", "char-1", "first question", []string{"first fact"}); err != nil {
		t.Fatalf("first turn: %v", err)
	}
	mem, err := svc.RecordTurn("npc-elder", "char-1", "second question", []string{"second fact"})
	if err != nil {
		t.Fatalf("second turn: %v", err)
	}

	// Simulate the async LLM summariser saving its own summary.
	mem.Summary = "Resumen escrito por el modelo."
	mem.SummarySource = repository.NPCMemorySummaryLLM
	if _, err := svc.Upsert(mem); err != nil {
		t.Fatalf("save llm summary: %v", err)
	}

	for i := 0; i < 2; i++ {
		mem, err = svc.RecordTurn("npc-elder", "char-1", "later question", []string{"later fact"})
		if err != nil {
			t.Fatalf("later turn %d: %v", i, err)
		}
		if mem.Summary != "Resumen escrito por el modelo." {
			t.Fatalf("LLM summary was overwritten: %q", mem.Summary)
		}
		if mem.SummarySource != repository.NPCMemorySummaryLLM {
			t.Fatalf("summary source = %q, want llm", mem.SummarySource)
		}
	}
}

func TestRecordTurnPreservesExternalSummaryWithoutMarker(t *testing.T) {
	svc := newNPCMemoriesTestService(t)

	if _, err := svc.RecordTurn("npc-elder", "char-1", "first question", []string{"first fact"}); err != nil {
		t.Fatalf("first turn: %v", err)
	}
	mem, err := svc.RecordTurn("npc-elder", "char-1", "second question", []string{"second fact"})
	if err != nil {
		t.Fatalf("second turn: %v", err)
	}

	// Simulate an LLM summary written before the marker existed.
	mem.Summary = "Resumen externo sin marcador."
	mem.SummarySource = ""
	if _, err := svc.Upsert(mem); err != nil {
		t.Fatalf("save external summary: %v", err)
	}

	mem, err = svc.RecordTurn("npc-elder", "char-1", "third question", []string{"third fact"})
	if err != nil {
		t.Fatalf("third turn: %v", err)
	}
	if mem.Summary != "Resumen externo sin marcador." {
		t.Fatalf("external summary was overwritten: %q", mem.Summary)
	}
}

func TestAdjustRelationClampsAndPersists(t *testing.T) {
	svc := newNPCMemoriesTestService(t)

	mem, err := svc.AdjustRelation("npc-elder", "char-rel", "PLAYER", 0.04)
	if err != nil {
		t.Fatalf("adjust relation: %v", err)
	}
	if mem == nil || mem.Relations["player"] != 0.04 {
		t.Fatalf("unexpected relation after first delta: %+v", mem)
	}

	// A single delta is clamped to +0.05 and the total to [-1,1].
	mem, err = svc.AdjustRelation("npc-elder", "char-rel", "player", 9)
	if err != nil {
		t.Fatalf("adjust relation clamp: %v", err)
	}
	if mem.Relations["player"] != 0.09 {
		t.Fatalf("relation = %v, want 0.09", mem.Relations["player"])
	}

	for i := 0; i < 30; i++ {
		mem, err = svc.AdjustRelation("npc-elder", "char-rel", "player", 0.05)
		if err != nil {
			t.Fatalf("adjust relation loop %d: %v", i, err)
		}
	}
	if mem.Relations["player"] != 1 {
		t.Fatalf("relation = %v, want capped at 1", mem.Relations["player"])
	}

	// Recording a turn must preserve the relationship map.
	mem, err = svc.RecordTurn("npc-elder", "char-rel", "hello", []string{"met the elder"})
	if err != nil {
		t.Fatalf("record turn: %v", err)
	}
	if mem.Relations["player"] != 1 {
		t.Fatalf("relation lost after RecordTurn: %+v", mem.Relations)
	}
}
