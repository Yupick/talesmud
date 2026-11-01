package game

import (
	"context"
	"strings"
	"testing"
	"time"

	def "github.com/talesmud/talesmud/pkg/mudserver/game/def"
	"github.com/talesmud/talesmud/pkg/mudserver/game/messages"
)

func newDMTestGame() *Game {
	g := &Game{
		sendMessage:    make(chan interface{}, 4),
		dmCooldown:     map[string]time.Time{},
		dmSem:          make(chan struct{}, dmMaxConcurrent),
		dmCooldownWin:  dmCooldownWindowDefault,
		dmTimeout:      dmTimeoutDefault,
		dmMaxPerMinute: dmMaxPerMinuteDefault,
		dmEnabled:      true,
	}
	return g
}

func TestDMNarratorPostsRoomMessage(t *testing.T) {
	g := newDMTestGame()
	g.SetDMNarrator(func(_ context.Context, _ DMNarrationRequest) (string, error) {
		return "  El ambiente se tensa.\n", nil
	})
	g.TriggerDMNarration(DMNarrationRequest{Event: "combatStart", RoomID: "R1", NPCName: "Rata"})

	select {
	case raw := <-g.sendMessage:
		msg, ok := raw.(messages.MessageResponse)
		if !ok {
			t.Fatalf("expected MessageResponse, got %T", raw)
		}
		if msg.Audience != messages.MessageAudienceRoom {
			t.Errorf("audience = %v, want room", msg.Audience)
		}
		if msg.AudienceID != "R1" {
			t.Errorf("audienceID = %q, want R1", msg.AudienceID)
		}
		if msg.Username != "DM" {
			t.Errorf("username = %q, want DM", msg.Username)
		}
		if msg.Message != "El ambiente se tensa." {
			t.Errorf("unexpected message: %q", msg.Message)
		}
		if strings.Contains(msg.Message, "\n") || strings.HasPrefix(msg.Message, "[DM]") {
			t.Errorf("message should be plain single line: %q", msg.Message)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for DM narration")
	}
}

func TestDMNarratorPostsUserMessage(t *testing.T) {
	g := newDMTestGame()
	seen := make(chan DMNarrationRequest, 1)
	g.SetDMNarrator(func(_ context.Context, req DMNarrationRequest) (string, error) {
		seen <- req
		return "Has crecido.", nil
	})
	g.NotifyDM(def.DMEvent{
		Kind:        "levelUp",
		CharacterID: "c1",
		UserID:      "u1",
		Facts:       map[string]string{"level": "4"},
	})

	select {
	case raw := <-g.sendMessage:
		msg, ok := raw.(messages.MessageResponse)
		if !ok {
			t.Fatalf("expected MessageResponse, got %T", raw)
		}
		if msg.Audience != messages.MessageAudienceUser {
			t.Errorf("audience = %v, want user", msg.Audience)
		}
		if msg.AudienceID != "u1" {
			t.Errorf("audienceID = %q, want u1", msg.AudienceID)
		}
		if msg.Username != "DM" {
			t.Errorf("username = %q, want DM", msg.Username)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for DM narration")
	}
	select {
	case req := <-seen:
		if req.Event != "levelUp" || req.UserID != "u1" || req.Audience != messages.MessageAudienceUser {
			t.Errorf("unexpected request: %+v", req)
		}
	case <-time.After(time.Second):
		t.Fatal("narrator was not called")
	}
}

func TestDMNarratorDisabledDoesNothing(t *testing.T) {
	g := newDMTestGame()
	g.dmEnabled = false
	g.SetDMNarrator(func(context.Context, DMNarrationRequest) (string, error) {
		t.Fatal("narrator must not run when disabled")
		return "", nil
	})
	if g.DMNarratorEnabled() {
		t.Fatal("narrator should be disabled")
	}
	g.TriggerDMNarration(DMNarrationRequest{Event: "combatStart", RoomID: "R1"})
	select {
	case msg := <-g.sendMessage:
		t.Fatalf("expected no message, got %+v", msg)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestDMNarratorCooldownPerRoom(t *testing.T) {
	g := newDMTestGame()
	if !g.dmAllowRoom("R1") {
		t.Fatal("first narration should be allowed")
	}
	if g.dmAllowRoom("R1") {
		t.Fatal("second narration within the window should be denied")
	}
	if !g.dmAllowRoom("R2") {
		t.Fatal("a different room should be allowed")
	}
}

func TestDMNarratorCooldownPerUser(t *testing.T) {
	g := newDMTestGame()
	if !g.dmAllowUser("u1") {
		t.Fatal("first private narration should be allowed")
	}
	if g.dmAllowUser("u1") {
		t.Fatal("second private narration within the window should be denied")
	}
	if !g.dmAllowUser("u2") {
		t.Fatal("a different user should be allowed")
	}
}

func TestDMNarratorGlobalBudget(t *testing.T) {
	g := newDMTestGame()
	g.dmMaxPerMinute = 2
	if !g.dmAllowGlobal() || !g.dmAllowGlobal() {
		t.Fatal("first two events should fit the budget")
	}
	if g.dmAllowGlobal() {
		t.Fatal("third event should exceed the per-minute budget")
	}

	// Events older than a minute are pruned and free budget again.
	g.dmMinuteEvents = []time.Time{time.Now().Add(-2 * time.Minute)}
	if !g.dmAllowGlobal() {
		t.Fatal("expired events should not count against the budget")
	}
}

func TestNotifyDMParsesFacts(t *testing.T) {
	g := newDMTestGame()
	seen := make(chan DMNarrationRequest, 1)
	g.SetDMNarrator(func(_ context.Context, req DMNarrationRequest) (string, error) {
		seen <- req
		return "La refriega continúa.", nil
	})
	g.NotifyDM(def.DMEvent{
		Kind:   "combatStart",
		RoomID: "R1",
		Facts: map[string]string{
			"npcName":   "Rata",
			"round":     "3",
			"partySize": "4",
			"outcome":   "ongoing",
		},
	})

	select {
	case req := <-seen:
		if req.Round != 3 || req.PartySize != 4 || req.NPCName != "Rata" || req.Outcome != "ongoing" {
			t.Errorf("unexpected request: %+v", req)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for narration")
	}
}

func TestDMPartySizeFallsBack(t *testing.T) {
	g := newDMTestGame()
	if got := g.dmPartySize("", 1); got != 1 {
		t.Errorf("empty character: got %d, want 1", got)
	}
	if got := g.dmPartySize("c1", 3); got != 3 {
		t.Errorf("no party state: got %d, want fallback 3", got)
	}
}

func TestCleanDMTextTruncatesAndFlattens(t *testing.T) {
	long := strings.Repeat("a", 500)
	got := cleanDMText("\"line one\nline two\"")
	if got != "line one line two" {
		t.Errorf("got %q", got)
	}
	got = cleanDMText(long)
	if len([]rune(got)) != dmMaxRunes {
		t.Errorf("truncated length = %d, want %d", len([]rune(got)), dmMaxRunes)
	}
}
