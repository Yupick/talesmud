package mudserver

import (
	"sync"
	"testing"

	"github.com/talesmud/talesmud/pkg/entities"
	"github.com/talesmud/talesmud/pkg/entities/characters"
	"github.com/talesmud/talesmud/pkg/entities/traits"
)

// TestPresenceMutationIsRaceFree drives every writer of the same user at the
// same time.
//
// The same *entities.User is shared by the websocket handler goroutine
// (attachSession / handleConnectionClosed) and by the game loop
// (ConnectUserSession / SetUserSessionCharacter / DisconnectUserSession).
// Those writers used to change IsOnline/LastSeen/LastCharacter without any
// synchronisation, and the repository marshalled that live user to JSON while
// another goroutine was still mutating it: a data race, and a mixed snapshot
// on the way to SQLite (Batch 23).
//
// Without the presence lock this test fails under -race; with it the writes
// are serialised per user and the persisted payload is a consistent copy of
// the user.
func TestPresenceMutationIsRaceFree(t *testing.T) {
	srv, facade, user := newTestMud(t)

	char := &characters.Character{
		Entity:      &entities.Entity{ID: "char-race"},
		Name:        "Racer",
		BelongsUser: *traits.BelongsToUser(user.ID),
	}
	if _, err := facade.CharactersService().Import(char); err != nil {
		t.Fatalf("import character: %v", err)
	}

	// Game.Run is not started here: drain the channels the presence paths
	// write to, so nothing wedges on the game loop channels.
	stop := make(chan struct{})
	t.Cleanup(func() { close(stop) })
	go func() {
		for {
			select {
			case <-srv.Game.OnMessageReceived():
			case <-srv.Game.OnUserQuit:
			case <-srv.Game.OnUserJoined:
			case <-stop:
				return
			}
		}
	}()

	const rounds = 50
	for i := 0; i < rounds; i++ {
		conn := srv.attachSession(user, &fakeTransport{})

		var wg sync.WaitGroup
		wg.Add(5)
		go func() {
			defer wg.Done()
			srv.handleInput(user, "look", false)
		}()
		go func() {
			defer wg.Done()
			srv.Game.ConnectUserSession(user)
		}()
		go func() {
			defer wg.Done()
			srv.Game.SetUserSessionCharacter(user, char)
		}()
		go func() {
			defer wg.Done()
			srv.Game.DisconnectUserSession(user.ID)
		}()
		go func() {
			defer wg.Done()
			srv.handleConnectionClosed(user, conn)
		}()
		wg.Wait()
	}

	// The churn must leave a coherent user row behind: the presence fields
	// were written from a snapshot taken under the lock, so the row always
	// carries the refid of the user and a presence timestamp.
	stored, err := facade.UsersService().FindByID(user.ID)
	if err != nil || stored == nil {
		t.Fatalf("user row not readable after the presence churn: %v", err)
	}
	if stored.RefID != user.RefID {
		t.Fatalf("persisted refid = %q, want %q", stored.RefID, user.RefID)
	}
	if stored.LastSeen.IsZero() {
		t.Fatal("persisted presence carries no timestamp")
	}
}
