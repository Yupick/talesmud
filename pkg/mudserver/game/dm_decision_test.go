package game

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/talesmud/talesmud/pkg/mudserver/game/messages"
)

func newDMDecisionTestGame() *Game {
	return &Game{
		sendMessage:            make(chan interface{}, 8),
		dmDecisionCooldown:     map[string]time.Time{},
		dmDecisionWindow:       dmDecisionCooldownDefault,
		dmDecisionTimeout:      dmDecisionTimeoutDefault,
		dmDecisionMaxPerMinute: dmDecisionMaxPerMinute,
		dmDecisionEnabled:      true,
	}
}

func TestMaybeDecideDMAmbushSpawnsCount(t *testing.T) {
	g := newDMDecisionTestGame()
	var spawned []string
	g.dmDecisionSpawn = func(templateID, roomID string) error {
		spawned = append(spawned, templateID+"@"+roomID)
		return nil
	}
	g.SetDMDecisionProvider(func(context.Context, DMDecisionRequest) (DMDecision, error) {
		return DMDecision{
			Kind:       DMDecisionAmbush,
			RoomID:     "model-chosen-room",
			TemplateID: "rat_template",
			Count:      2,
			Reason:     "the party is noisy",
		}, nil
	})

	decision, ok := g.MaybeDecideDM(context.Background(), DMDecisionRequest{
		Event: "roomEnter", RoomID: "R1", RoomName: "Sewers",
	})
	if !ok {
		t.Fatal("valid ambush should be applied")
	}
	if len(spawned) != 2 {
		t.Fatalf("spawn count = %d, want 2", len(spawned))
	}
	for _, got := range spawned {
		if got != "rat_template@R1" {
			t.Errorf("spawned %q, want rat_template@R1", got)
		}
	}
	if decision.RoomID != "R1" {
		t.Errorf("decision room = %q, request room must win", decision.RoomID)
	}
	if decision.Count != 2 {
		t.Errorf("decision count = %d, want 2", decision.Count)
	}

	metrics := g.DMDecisionMetrics()
	if metrics["requested"] != 1 || metrics["applied"] != 1 {
		t.Errorf("unexpected totals: %+v", metrics)
	}
	if metrics["rejected"] != 0 || metrics["errors"] != 0 {
		t.Errorf("unexpected rejection/error counters: %+v", metrics)
	}
	if metrics["requested_ambush"] != 1 || metrics["applied_ambush"] != 1 {
		t.Errorf("unexpected ambush counters: %+v", metrics)
	}
	if metrics["last_decision_unix"] == 0 {
		t.Errorf("last_decision_unix should be set: %+v", metrics)
	}
}

func TestMaybeDecideDMRejectsUnknownKind(t *testing.T) {
	g := newDMDecisionTestGame()
	spawned := 0
	g.dmDecisionSpawn = func(string, string) error { spawned++; return nil }
	g.SetDMDecisionProvider(func(context.Context, DMDecisionRequest) (DMDecision, error) {
		return DMDecision{Kind: "teleport", RoomID: "R1", Message: "hocus pocus"}, nil
	})

	if _, ok := g.MaybeDecideDM(context.Background(), DMDecisionRequest{RoomID: "R1"}); ok {
		t.Fatal("unknown kind must be rejected")
	}
	if spawned != 0 {
		t.Fatalf("spawned = %d, want 0", spawned)
	}
	metrics := g.DMDecisionMetrics()
	if metrics["rejected"] != 1 || metrics["rejected_unknown"] != 1 {
		t.Errorf("unexpected metrics: %+v", metrics)
	}
}

func TestMaybeDecideDMRejectsAmbushWithoutTemplate(t *testing.T) {
	g := newDMDecisionTestGame()
	spawned := 0
	g.dmDecisionSpawn = func(string, string) error { spawned++; return nil }
	g.SetDMDecisionProvider(func(context.Context, DMDecisionRequest) (DMDecision, error) {
		return DMDecision{Kind: DMDecisionAmbush, Count: 1}, nil
	})

	if _, ok := g.MaybeDecideDM(context.Background(), DMDecisionRequest{RoomID: "R1"}); ok {
		t.Fatal("ambush without template must be rejected")
	}
	if spawned != 0 {
		t.Fatalf("spawned = %d, want 0", spawned)
	}
	if got := g.DMDecisionMetrics()["rejected_ambush"]; got != 1 {
		t.Errorf("rejected_ambush = %d, want 1", got)
	}
}

func TestMaybeDecideDMRejectsOutOfRangeCount(t *testing.T) {
	g := newDMDecisionTestGame()
	spawned := 0
	g.dmDecisionSpawn = func(string, string) error { spawned++; return nil }
	g.SetDMDecisionProvider(func(context.Context, DMDecisionRequest) (DMDecision, error) {
		return DMDecision{Kind: DMDecisionAmbush, TemplateID: "rat", Count: -1}, nil
	})

	if _, ok := g.MaybeDecideDM(context.Background(), DMDecisionRequest{RoomID: "R1"}); ok {
		t.Fatal("negative count must be rejected")
	}
	if spawned != 0 {
		t.Fatalf("spawned = %d, want 0", spawned)
	}
}

func TestMaybeDecideDMClampsHighAmbushCount(t *testing.T) {
	g := newDMDecisionTestGame()
	spawned := 0
	g.dmDecisionSpawn = func(string, string) error { spawned++; return nil }
	g.SetDMDecisionProvider(func(context.Context, DMDecisionRequest) (DMDecision, error) {
		return DMDecision{Kind: DMDecisionAmbush, TemplateID: "rat", Count: 99}, nil
	})

	decision, ok := g.MaybeDecideDM(context.Background(), DMDecisionRequest{RoomID: "R1"})
	if !ok {
		t.Fatal("high count should be clamped, not dropped")
	}
	if spawned != dmDecisionMaxAmbush {
		t.Fatalf("spawned = %d, want %d", spawned, dmDecisionMaxAmbush)
	}
	if decision.Count != dmDecisionMaxAmbush {
		t.Fatalf("decision count = %d, want %d", decision.Count, dmDecisionMaxAmbush)
	}
}

func TestMaybeDecideDMCooldownPerRoom(t *testing.T) {
	g := newDMDecisionTestGame()
	providerCalls := 0
	g.dmDecisionSpawn = func(string, string) error { return nil }
	g.SetDMDecisionProvider(func(context.Context, DMDecisionRequest) (DMDecision, error) {
		providerCalls++
		return DMDecision{Kind: DMDecisionAmbush, TemplateID: "rat", Count: 1}, nil
	})

	if _, ok := g.MaybeDecideDM(context.Background(), DMDecisionRequest{RoomID: "R1"}); !ok {
		t.Fatal("first decision should be applied")
	}
	if _, ok := g.MaybeDecideDM(context.Background(), DMDecisionRequest{RoomID: "R1"}); ok {
		t.Fatal("second decision in the same room must be throttled")
	}
	if providerCalls != 1 {
		t.Fatalf("provider calls = %d, want 1 (throttled before the LLM)", providerCalls)
	}
	if _, ok := g.MaybeDecideDM(context.Background(), DMDecisionRequest{RoomID: "R2"}); !ok {
		t.Fatal("a different room should be allowed")
	}
	if providerCalls != 2 {
		t.Fatalf("provider calls = %d, want 2", providerCalls)
	}
	metrics := g.DMDecisionMetrics()
	if metrics["applied"] != 2 || metrics["rejected"] != 1 {
		t.Errorf("unexpected metrics: %+v", metrics)
	}
}

func TestMaybeDecideDMKillSwitchOffNeverCallsProvider(t *testing.T) {
	g := newDMDecisionTestGame()
	g.dmDecisionEnabled = false
	called := 0
	g.SetDMDecisionProvider(func(context.Context, DMDecisionRequest) (DMDecision, error) {
		called++
		return DMDecision{Kind: DMDecisionNarrative, Message: "should not happen"}, nil
	})

	if _, ok := g.MaybeDecideDM(context.Background(), DMDecisionRequest{RoomID: "R1"}); ok {
		t.Fatal("decision must be off when the kill switch is off")
	}
	if called != 0 {
		t.Fatalf("provider called %d times, want 0", called)
	}
	metrics := g.DMDecisionMetrics()
	if metrics["requested"] != 0 || metrics["applied"] != 0 || metrics["rejected"] != 0 {
		t.Errorf("metrics must stay empty while disabled: %+v", metrics)
	}
}

func TestMaybeDecideDMEnvironmentalBroadcastsProse(t *testing.T) {
	g := newDMDecisionTestGame()
	g.SetDMDecisionProvider(func(context.Context, DMDecisionRequest) (DMDecision, error) {
		return DMDecision{Kind: DMDecisionEnvironmental, Message: "  El viento aúlla.\n"}, nil
	})

	decision, ok := g.MaybeDecideDM(context.Background(), DMDecisionRequest{RoomID: "R1"})
	if !ok {
		t.Fatal("environmental decision should be applied")
	}
	if decision.Message != "El viento aúlla." {
		t.Errorf("message = %q, want cleaned prose", decision.Message)
	}

	select {
	case raw := <-g.sendMessage:
		msg, ok := raw.(messages.MessageResponse)
		if !ok {
			t.Fatalf("expected MessageResponse, got %T", raw)
		}
		if msg.Audience != messages.MessageAudienceRoom || msg.AudienceID != "R1" {
			t.Errorf("unexpected audience: %+v", msg)
		}
		if msg.Username != "DM" || msg.Message != "El viento aúlla." {
			t.Errorf("unexpected message: %+v", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for ambient prose")
	}
	if metrics := g.DMDecisionMetrics(); metrics["applied_environmental"] != 1 {
		t.Errorf("unexpected metrics: %+v", metrics)
	}
}

func TestMaybeDecideDMProviderErrorCounts(t *testing.T) {
	g := newDMDecisionTestGame()
	g.SetDMDecisionProvider(func(context.Context, DMDecisionRequest) (DMDecision, error) {
		return DMDecision{}, errors.New("model down")
	})

	if _, ok := g.MaybeDecideDM(context.Background(), DMDecisionRequest{RoomID: "R1"}); ok {
		t.Fatal("provider error must not apply a decision")
	}
	metrics := g.DMDecisionMetrics()
	if metrics["requested"] != 1 || metrics["errors"] != 1 || metrics["applied"] != 0 {
		t.Errorf("unexpected metrics: %+v", metrics)
	}
}
