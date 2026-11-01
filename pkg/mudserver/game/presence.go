package game

import (
	"sync"

	"github.com/talesmud/talesmud/pkg/entities"
	def "github.com/talesmud/talesmud/pkg/mudserver/game/def"
)

// presenceLock is the per-user mutex that owns the mutable presence fields of
// a live *entities.User: IsOnline, LastSeen and LastCharacter.
type presenceLock struct {
	mu   sync.Mutex
	refs int
}

// presenceRegistry hands out one mutex per user id.
//
// The same *entities.User is shared by the websocket handler goroutine, the
// game loop and the connection-close goroutine. Those goroutines used to
// write IsOnline/LastSeen and persist the user without any synchronisation,
// so UsersService().Update serialised a half-mutated struct (mixed
// snapshot) while another goroutine was writing the same fields: a data
// race (Batch 23).
//
// Trade-off: a per-user lock keeps unrelated players completely independent —
// the game loop of player A never waits for player B's SQLite write — at the
// cost of one registry entry per connected user. Entries are reference
// counted and dropped as soon as the last holder releases them, so the
// registry does not grow without bound.
//
// A single global presence mutex would be simpler but would serialise every
// presence write of the whole server behind one SQLite round trip; a striped
// lock array would bound the memory but could block two unrelated users that
// happen to share a stripe.
type presenceRegistry struct {
	mu    sync.Mutex
	locks map[string]*presenceLock
}

// acquire returns the lock of userID and takes a reference on it, so the
// entry cannot be dropped while the caller holds or waits for it.
func (r *presenceRegistry) acquire(userID string) *presenceLock {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.locks == nil {
		r.locks = map[string]*presenceLock{}
	}
	lock, ok := r.locks[userID]
	if !ok {
		lock = &presenceLock{}
		r.locks[userID] = lock
	}
	lock.refs++
	return lock
}

// release drops a reference; the entry disappears with the last one.
func (r *presenceRegistry) release(userID string, lock *presenceLock) {
	r.mu.Lock()
	defer r.mu.Unlock()
	lock.refs--
	if lock.refs > 0 {
		return
	}
	if current, ok := r.locks[userID]; ok && current == lock {
		delete(r.locks, userID)
	}
}

// PresenceState is a consistent view of the mutable presence fields of a user.
type PresenceState = def.PresenceState

// ApplyPresence applies a presence change to a live user and persists it.
//
// mutate runs under the presence lock of the user, so the fields it changes
// (IsOnline, LastSeen, LastCharacter) can never be observed half-written by
// another goroutine. It returns true when the change must be written to the
// users store.
//
// The write uses a detached copy of the user taken under the same lock: the
// repository serialises that copy to JSON, so the persisted payload is always
// a consistent snapshot instead of a mix of two concurrent states. The lock
// is held across the write too, so the order of the persisted rows matches
// the order of the state transitions (an offline write can never be undone
// by an older online write).
//
// The presence lock is a leaf lock: it is taken here and nowhere else, and
// the only work done under it is the repository write. It therefore cannot
// deadlock with the session registry or the game loop, and no path acquires
// it in the opposite order.
func (g *Game) ApplyPresence(user *entities.User, mutate func(u *entities.User) bool) {
	if g == nil || user == nil || mutate == nil || g.Facade == nil {
		return
	}
	lock := g.presence.acquire(user.ID)
	defer g.presence.release(user.ID, lock)

	lock.mu.Lock()
	defer lock.mu.Unlock()

	if !mutate(user) {
		return
	}
	snapshot := user.Snapshot()
	if snapshot == nil {
		return
	}
	_ = g.Facade.UsersService().Update(snapshot.RefID, snapshot)
}

// PresenceOf returns the presence fields of the user under its presence lock.
// Callers on another goroutine must use it to read the mutable presence
// fields of a shared user.
func (g *Game) PresenceOf(user *entities.User) PresenceState {
	if g == nil || user == nil {
		return PresenceState{}
	}
	lock := g.presence.acquire(user.ID)
	defer g.presence.release(user.ID, lock)

	lock.mu.Lock()
	defer lock.mu.Unlock()

	return PresenceState{
		IsOnline:      user.IsOnline,
		LastSeen:      user.LastSeen,
		LastCharacter: user.LastCharacter,
	}
}
