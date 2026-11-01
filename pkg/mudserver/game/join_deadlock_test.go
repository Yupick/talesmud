package game

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/talesmud/talesmud/pkg/entities"
	"github.com/talesmud/talesmud/pkg/mudserver/game/messages"
)

// TestHandleUserJoinedDispatchesFallbackInlineOnFullInputBuffer pins the P0
// fix for the QA B19.3 wedge: handleUserJoined runs on the game loop, the
// only consumer of onMessageReceived. The old code sent the synthesized "lc"
// (and "selectcharacter") command to that very channel, so a full buffer
// blocked the loop on a send only it could service — a permanent deadlock
// under a burst of logins. The fix dispatches inline: the join completes even
// with the input queue at capacity, the queue is left untouched, and the
// command's output still reaches the user.
func TestHandleUserJoinedDispatchesFallbackInlineOnFullInputBuffer(t *testing.T) {
	g, facade := newSessionTestGame(t)

	user := &entities.User{
		Entity:   &entities.Entity{ID: "user-join"},
		RefID:    "ref-join",
		Nickname: "Joiner",
	}
	if _, err := facade.UsersService().Import(user); err != nil {
		t.Fatalf("import user: %v", err)
	}

	// Fill the loop's input queue to exactly capacity, as a login burst does.
	want := cap(g.onMessageReceived)
	for i := 0; i < want; i++ {
		select {
		case g.onMessageReceived <- messages.NewMessage(user, fmt.Sprintf("filler-%d", i)):
		default:
			t.Fatalf("input queue not full after %d messages (cap %d)", i, want)
		}
	}

	// handleUserJoined must return even though the queue it used to feed is
	// completely full.
	done := make(chan struct{})
	go func() {
		defer close(done)
		g.handleUserJoined(user)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("handleUserJoined blocked on its own input queue: game-loop self deadlock (QA B19.3)")
	}

	if got := len(g.onMessageReceived); got != want {
		t.Fatalf("input queue changed: got %d messages, want %d — the join must not enqueue onto the loop's own queue", got, want)
	}

	// The fallback command still ran inline: a user without characters is
	// told so instead of silently losing the synthesized message.
	select {
	case out := <-g.SendMessage():
		msg, ok := out.(messages.MessageResponder)
		if !ok {
			t.Fatalf("unexpected inline output %#v", out)
		}
		if !strings.Contains(msg.GetMessage(), "no characters") {
			t.Fatalf("inline \"lc\" output changed: %q", msg.GetMessage())
		}
	case <-time.After(3 * time.Second):
		t.Fatal("inline \"lc\" produced no output — the login fallback was lost")
	}
}

// TestGameLoopSurvivesJoinBurst pushes far more joins through OnUserJoined
// than the old 20-slot input buffer could absorb (the old code wedged after
// ~74-140 joins) and requires the loop to keep accepting work afterwards.
// It is the load-shaped companion of the deterministic test above.
func TestGameLoopSurvivesJoinBurst(t *testing.T) {
	g, facade := newSessionTestGame(t)

	const burst = 150
	users := make([]*entities.User, 0, burst)
	for i := 0; i < burst; i++ {
		u := &entities.User{
			Entity:   &entities.Entity{ID: fmt.Sprintf("user-burst-%d", i)},
			RefID:    fmt.Sprintf("ref-burst-%d", i),
			Nickname: fmt.Sprintf("Burst%d", i),
		}
		if _, err := facade.UsersService().Import(u); err != nil {
			t.Fatalf("import user %d: %v", i, err)
		}
		users = append(users, u)
	}

	// Drain outbound traffic; in production receiveMessages does this job.
	stop := make(chan struct{})
	t.Cleanup(func() { close(stop) })
	var outbound atomic.Int64
	go func() {
		for {
			select {
			case <-g.SendMessage():
				outbound.Add(1)
			case <-stop:
				return
			}
		}
	}()

	g.Run()

	pushDone := make(chan struct{})
	go func() {
		defer close(pushDone)
		for _, u := range users {
			g.OnUserJoined <- &messages.UserJoined{User: u}
		}
	}()
	select {
	case <-pushDone:
	case <-time.After(20 * time.Second):
		t.Fatalf("join producers blocked with %d joins still queued: game loop wedged", len(g.OnUserJoined))
	}

	// Every join must be processed: each user without a character gets one
	// inline "lc" reply.
	deadline := time.Now().Add(15 * time.Second)
	for outbound.Load() < burst && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if got := outbound.Load(); got < burst {
		t.Fatalf("only %d/%d joins produced output after the burst", got, burst)
	}

	// And the loop must still accept fresh work.
	liveDone := make(chan error, 1)
	go func() { liveDone <- g.Call(func() {}) }()
	select {
	case err := <-liveDone:
		if err != nil {
			t.Fatalf("game loop did not run the job: %v", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("game loop unresponsive after the join burst")
	}
}
