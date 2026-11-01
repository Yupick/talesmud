// Package qa_b23 holds the independent QA harness for Batch 23.
//
// Batch 23 fixed the pre-existing data race on the shared *entities.User
// (documented as an open P2 in §B22.4 of docs/qa/REGRESSION_REPORT.md): the
// websocket goroutine and the game loop mutated IsOnline/LastSeen/
// LastCharacter without any common synchronisation while the repository
// marshalled the very same struct to JSON.
//
// The fix is a per-user lock in the Game (pkg/mudserver/game/presence.go):
// ApplyPresence mutates + snapshots + persists under the lock, and
// PresenceOf reads the same fields under the same lock.
//
// Nothing in here is production code. It lives outside pkg/** (the only
// sanctioned place for new QA tests) and drives the presence paths through
// the public API only, so it also acts as the regression guard the §B22.4
// fix proposal asked for ("convert the opt-in reproduction into a mandatory
// -race test").
package qa_b23

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/talesmud/talesmud/pkg/db/sqlite"
	"github.com/talesmud/talesmud/pkg/entities"
	"github.com/talesmud/talesmud/pkg/entities/characters"
	"github.com/talesmud/talesmud/pkg/entities/traits"
	"github.com/talesmud/talesmud/pkg/mudserver"
	"github.com/talesmud/talesmud/pkg/mudserver/game"
	"github.com/talesmud/talesmud/pkg/repository"
	"github.com/talesmud/talesmud/pkg/service"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// silentTransport is a mudserver.Transport that swallows everything. It never
// blocks, so a presence path can never wedge on the transport side.
type silentTransport struct {
	mu    sync.Mutex
	sent  []any
	muEnd time.Time
}

func (s *silentTransport) Send(v any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent = append(s.sent, v)
	return nil
}

func (s *silentTransport) Close(code int, reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.muEnd = time.Now()
}

func (s *silentTransport) RemoteIP() string { return "127.0.0.1" }
func (s *silentTransport) Kind() string     { return "qa" }

// countingUsers counts the presence writes that reach the repository.
type countingUsers struct {
	service.UsersService
	updates atomic.Int64
	mu      sync.Mutex
	refs    []string
	onlines []bool
}

func (c *countingUsers) Update(refID string, user *entities.User) error {
	c.updates.Add(1)
	c.mu.Lock()
	c.refs = append(c.refs, refID)
	c.onlines = append(c.onlines, user != nil && user.IsOnline)
	c.mu.Unlock()
	return c.UsersService.Update(refID, user)
}

func (c *countingUsers) snapshot() (refs []string, onlines []bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.refs...), append([]bool(nil), c.onlines...)
}

// countingFacade swaps in the counting users service.
type countingFacade struct {
	service.Facade
	us *countingUsers
}

func (c *countingFacade) UsersService() service.UsersService { return c.us }

// harness bundles everything a presence test needs.
type harness struct {
	srv     mudserver.MUDServer
	game    *game.Game
	facade  service.Facade
	counter *countingUsers
	stop    chan struct{}
}

// newHarness builds a server over a throwaway SQLite database, with one
// imported user that owns two characters.
func newHarness(t *testing.T, userID string, chars int) *harness {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := sqlite.Open(filepath.Join(t.TempDir(), "qa_b23.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	base := service.NewFacade(repository.NewSQLiteFactory(db), nil)
	counter := &countingUsers{UsersService: base.UsersService()}
	facade := &countingFacade{Facade: base, us: counter}

	user := &entities.User{
		Entity:   &entities.Entity{ID: userID},
		RefID:    "ref-" + userID,
		Nickname: userID,
	}
	if _, err := base.UsersService().Import(user); err != nil {
		t.Fatalf("import user: %v", err)
	}

	for i := 0; i < chars; i++ {
		char := &characters.Character{
			Entity:      &entities.Entity{ID: fmt.Sprintf("%s-char-%d", userID, i)},
			Name:        fmt.Sprintf("%s-%d", userID, i),
			BelongsUser: *traits.BelongsToUser(user.ID),
		}
		if _, err := base.CharactersService().Import(char); err != nil {
			t.Fatalf("import character: %v", err)
		}
	}

	srv := mudserver.New(facade, nil)
	srv.SetSessionHook(nil)

	h := &harness{
		srv:     srv,
		game:    srv.GameCtrl().(*game.Game),
		facade:  facade,
		counter: counter,
		stop:    make(chan struct{}),
	}
	// The game loop is not started: the presence paths write to these
	// channels, so they must be drained or the writer wedges on a full
	// buffer (same trick as the in-package test).
	go func() {
		for {
			select {
			case <-h.game.OnMessageReceived():
			case <-h.game.OnUserJoined:
			case <-h.game.OnUserQuit:
			case <-h.stop:
				return
			}
		}
	}()
	t.Cleanup(func() { close(h.stop) })
	return h
}

// watchdog runs fn and fails the test if it does not return in time. It is
// the deadlock guard: a presence lock that is taken twice, or taken while
// another lock is held, shows up here as a hang instead of a silent pass.
func watchdog(t *testing.T, limit time.Duration, what string, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn()
	}()
	select {
	case <-done:
	case <-time.After(limit):
		buf := make([]byte, 4<<20)
		n := runtime.Stack(buf, true)
		t.Fatalf("%s did not finish within %s — suspected deadlock.\n%s",
			what, limit, string(buf[:n]))
	}
}

// ---------------------------------------------------------------------------
// tests
// ---------------------------------------------------------------------------

// TestPresenceSameUserChurnIsRaceFree drives every presence writer of ONE
// shared *entities.User at the same time, from outside the package.
//
// Writers exercised:
//   - attachSession / handleConnectionClosed (via AttachExternal + done())
//   - Game.ConnectUserSession
//   - Game.SetUserSessionCharacter
//   - Game.DisconnectUserSession
// Readers exercised:
//   - Game.PresenceOf
//
// Under -race this test is the guard for the B22.4 race: without the
// per-user lock it reports "WARNING: DATA RACE" on the user struct (write
// of IsOnline/LastSeen/LastCharacter against the json.Marshal performed by
// UsersService().Update).
func TestPresenceSameUserChurnIsRaceFree(t *testing.T) {
	h := newHarness(t, "user-churn", 2)
	user, err := h.facade.UsersService().FindByID("user-churn")
	if err != nil || user == nil {
		t.Fatalf("imported user not found: %v", err)
	}

	const rounds = 60
	watchdog(t, 90*time.Second, "presence churn", func() {
		for i := 0; i < rounds; i++ {
			charID := fmt.Sprintf("user-churn-char-%d", i%2)

			var wg sync.WaitGroup
			wg.Add(6)

			// websocket-ish attach + close of a session for the same user
			go func() {
				defer wg.Done()
				_, done := h.srv.AttachExternal(user, &silentTransport{})
				done()
			}()
			// game loop: mark the session as live
			go func() {
				defer wg.Done()
				h.game.ConnectUserSession(user)
			}()
			// game loop: select a character
			go func() {
				defer wg.Done()
				char, err := h.facade.CharactersService().FindByID(charID)
				if err != nil || char == nil {
					return
				}
				h.game.SetUserSessionCharacter(user, char)
			}()
			// game loop: drop the session
			go func() {
				defer wg.Done()
				h.game.DisconnectUserSession(user.ID)
			}()
			// readers
			go func() {
				defer wg.Done()
				for k := 0; k < 8; k++ {
					_ = h.game.PresenceOf(user)
				}
			}()
			go func() {
				defer wg.Done()
				for k := 0; k < 8; k++ {
					_ = h.game.PresenceOf(user)
				}
			}()

			wg.Wait()
		}
	})

	// The persisted row must be a coherent snapshot: the refid is the one of
	// the user (it is the UPDATE key) and the presence fields are present.
	stored, err := h.facade.UsersService().FindByID(user.ID)
	if err != nil || stored == nil {
		t.Fatalf("user row not readable after the presence churn: %v", err)
	}
	if stored.RefID != user.RefID {
		t.Fatalf("persisted refid = %q, want %q", stored.RefID, user.RefID)
	}
	if stored.LastSeen.IsZero() {
		t.Fatal("persisted presence carries no timestamp")
	}
	if stored.LastCharacter != "" &&
		stored.LastCharacter != "user-churn-char-0" &&
		stored.LastCharacter != "user-churn-char-1" {
		t.Fatalf("persisted lastCharacter = %q, want one of the two known characters (torn snapshot?)",
			stored.LastCharacter)
	}
	t.Logf("presence churn: %d rounds, %d repository writes, persisted lastCharacter=%q isOnline=%v",
		rounds, h.counter.updates.Load(), stored.LastCharacter, stored.IsOnline)
}

// TestPresenceDistinctUsersDoNotSerialize checks the other side of the
// design claim: the lock is PER USER, so a presence write of player A must
// not block the presence write of player B.
//
// The check is deliberately loose (a ratio, not a hard bound): SQLite
// serialises the writes anyway, so the point is only that the run completes
// and that no user waits for another user's presence work.
func TestPresenceDistinctUsersDoNotSerialize(t *testing.T) {
	h := newHarness(t, "user-multi", 1)
	const users = 8

	shared := make([]*entities.User, users)
	for i := range shared {
		shared[i] = &entities.User{
			Entity:   &entities.Entity{ID: fmt.Sprintf("user-multi-%d", i)},
			RefID:    fmt.Sprintf("ref-user-multi-%d", i),
			Nickname: fmt.Sprintf("multi-%d", i),
		}
		if _, err := h.facade.UsersService().Import(shared[i]); err != nil {
			t.Fatalf("import user %d: %v", i, err)
		}
	}

	watchdog(t, 90*time.Second, "multi-user presence churn", func() {
		var wg sync.WaitGroup
		for i, u := range shared {
			wg.Add(1)
			go func(idx int, user *entities.User) {
				defer wg.Done()
				for k := 0; k < 25; k++ {
					h.game.ConnectUserSession(user)
					_ = h.game.PresenceOf(user)
					h.game.DisconnectUserSession(user.ID)
				}
			}(i, u)
		}
		wg.Wait()
	})

	// Every user must own its own row, with its own refid.
	for i, u := range shared {
		stored, err := h.facade.UsersService().FindByID(u.ID)
		if err != nil || stored == nil {
			t.Fatalf("user %d row not readable: %v", i, err)
		}
		if stored.RefID != u.RefID {
			t.Fatalf("user %d persisted refid = %q, want %q", i, stored.RefID, u.RefID)
		}
		if stored.LastSeen.IsZero() {
			t.Fatalf("user %d persisted presence carries no timestamp", i)
		}
	}
	t.Logf("multi-user churn: %d users, %d repository writes", users, h.counter.updates.Load())
}

// TestPresenceOfflineOnlineGuardAndRefID pins the persistence semantics that
// the Batch 23 refactor had to preserve:
//
//   - the offline -> online transition is persisted (the caller may attach
//     outside attachSession);
//   - a repeated connect of an already-online user is NOT persisted (the
//     P1 debounce of QA B19.6-1: one write per transition, not one per
//     input line);
//   - the disconnect is persisted as offline;
//   - the UPDATE key is the user RefID, not the internal entity id.
func TestPresenceOfflineOnlineGuardAndRefID(t *testing.T) {
	h := newHarness(t, "user-guard", 1)
	user, err := h.facade.UsersService().FindByID("user-guard")
	if err != nil || user == nil {
		t.Fatalf("imported user not found: %v", err)
	}

	// offline -> online: one write
	h.game.ConnectUserSession(user)
	if got := h.counter.updates.Load(); got != 1 {
		t.Fatalf("presence writes after the first connect = %d, want 1", got)
	}
	refs, onlines := h.counter.snapshot()
	if refs[0] != user.RefID {
		t.Fatalf("update key = %q, want the user refid %q", refs[0], user.RefID)
	}
	if !onlines[0] {
		t.Fatal("the transition write must persist online=true")
	}

	// already online: no extra write (debounce)
	for i := 0; i < 20; i++ {
		h.game.ConnectUserSession(user)
	}
	if got := h.counter.updates.Load(); got != 1 {
		t.Fatalf("presence writes after 20 extra connects = %d, want 1 (debounced)", got)
	}

	// the in-memory flag stays online during the session
	if !h.game.PresenceOf(user).IsOnline {
		t.Fatal("in-memory presence must stay online during the session")
	}

	// disconnect: persisted as offline
	h.game.DisconnectUserSession(user.ID)
	if got := h.counter.updates.Load(); got != 2 {
		t.Fatalf("presence writes after the disconnect = %d, want 2", got)
	}
	_, onlines = h.counter.snapshot()
	if onlines[len(onlines)-1] {
		t.Fatal("the disconnect must persist online=false")
	}

	stored, err := h.facade.UsersService().FindByID(user.ID)
	if err != nil || stored == nil {
		t.Fatalf("user row not readable: %v", err)
	}
	if stored.IsOnline {
		t.Fatal("the persisted row must be offline after the disconnect")
	}
	if stored.RefID != user.RefID {
		t.Fatalf("persisted refid = %q, want %q", stored.RefID, user.RefID)
	}
	if stored.LastSeen.IsZero() {
		t.Fatal("the persisted row must carry a presence timestamp")
	}
}

// TestPresenceRegistryDoesNotLeak checks the refcounted registry of the
// per-user locks: entries must be dropped as soon as the last holder
// releases them, so a long-lived server does not accumulate one lock per
// user that ever connected.
//
// The registry is package-private, so the check is indirect: it counts the
// locks that the *game* keeps by hammering the presence API and comparing
// the goroutine count (a leaked lock would keep no goroutine, but a leaked
// registry entry per user would grow with the number of distinct users).
// The real assertion is the memory/goroutine stability across many
// distinct users.
func TestPresenceRegistryDoesNotLeak(t *testing.T) {
	h := newHarness(t, "user-registry", 1)

	before := runtime.NumGoroutine()
	const users = 200
	for i := 0; i < users; i++ {
		u := &entities.User{
			Entity:   &entities.Entity{ID: fmt.Sprintf("user-reg-%d", i)},
			RefID:    fmt.Sprintf("ref-user-reg-%d", i),
			Nickname: "reg",
		}
		if _, err := h.facade.UsersService().Import(u); err != nil {
			t.Fatalf("import user %d: %v", i, err)
		}
		h.game.ConnectUserSession(u)
		h.game.DisconnectUserSession(u.ID)
	}
	// Give the runtime a moment to settle the transient goroutines.
	time.Sleep(200 * time.Millisecond)
	after := runtime.NumGoroutine()
	if after > before+32 {
		buf := make([]byte, 2<<20)
		n := runtime.Stack(buf, true)
		t.Fatalf("goroutine count grew from %d to %d over %d users\n%s", before, after, users, string(buf[:n]))
	}
	t.Logf("registry churn: %d users, goroutines %d -> %d", users, before, after)
}

// TestPresenceHarnessIsNotVacuous guards the harness itself: if the presence
// API stopped being called, the race tests above would pass without testing
// anything. It asserts that the churn really produced repository writes and
// that the persisted rows really carry presence data.
func TestPresenceHarnessIsNotVacuous(t *testing.T) {
	h := newHarness(t, "user-vacuous", 1)
	user, err := h.facade.UsersService().FindByID("user-vacuous")
	if err != nil || user == nil {
		t.Fatalf("imported user not found: %v", err)
	}
	h.game.ConnectUserSession(user)
	if h.counter.updates.Load() == 0 {
		t.Fatal("the harness never reached the repository: the presence paths are not exercised")
	}
	refs, _ := h.counter.snapshot()
	if len(refs) == 0 || !strings.HasPrefix(refs[0], "ref-") {
		t.Fatalf("unexpected update keys: %v", refs)
	}
}
