package mudserver

import (
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/talesmud/talesmud/pkg/mudserver/game/messages"
)

// joinQueueWarnAt is the backlog size at which the queue reports saturation.
// It is purely observational: the queue never drops joins, it only logs so an
// operator can see that the game loop is draining slower than logins arrive.
const joinQueueWarnAt = 64

// joinQueue is a non-blocking, unbounded FIFO for join events.
//
// attachSession runs on each connection's read-loop goroutine. The game loop
// exposes OnUserJoined with a small buffer (20) and one consumer, so a burst
// of logins used to park hundreds of read loops on a channel send — the
// cascade half of the QA B19.3 wedge. Producers here push and return
// immediately; a single forwarder goroutine (server.forwardJoins) owns the
// blocking hand-off to the game loop. Joins are login-critical, so the queue
// absorbs bursts instead of dropping them.
type joinQueue struct {
	mu     sync.Mutex
	cond   *sync.Cond
	items  []*messages.UserJoined
	closed bool

	// warnAt/saturated drive one saturation warning and one recovery log so a
	// stalled game loop is visible without spamming the log per join.
	warnAt    int
	saturated bool
}

func newJoinQueue(warnAt int) *joinQueue {
	q := &joinQueue{warnAt: warnAt}
	q.cond = sync.NewCond(&q.mu)
	return q
}

// push enqueues a join without ever blocking the caller.
func (q *joinQueue) push(joined *messages.UserJoined) {
	if q == nil || joined == nil {
		return
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		log.WithField("userId", joined.User.ID).Warn("Join queue closed; join not forwarded")
		return
	}
	q.items = append(q.items, joined)
	if q.warnAt > 0 && !q.saturated && len(q.items) >= q.warnAt {
		q.saturated = true
		log.WithField("backlog", len(q.items)).Warn("Join queue saturated: game loop is draining joins slowly")
	}
	q.cond.Signal()
}

// pop blocks until a join is available or the queue is closed.
func (q *joinQueue) pop() (*messages.UserJoined, bool) {
	if q == nil {
		return nil, false
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.items) == 0 && !q.closed {
		q.cond.Wait()
	}
	if len(q.items) == 0 {
		return nil, false
	}
	joined := q.items[0]
	q.items[0] = nil
	q.items = q.items[1:]
	if q.saturated && len(q.items) <= q.warnAt/2 {
		q.saturated = false
		log.WithField("backlog", len(q.items)).Info("Join queue recovered")
	}
	return joined, true
}

// backlog reports how many joins are waiting for the forwarder.
func (q *joinQueue) backlog() int {
	if q == nil {
		return 0
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

// close releases every blocked popper. Only tests and shutdown need it.
func (q *joinQueue) close() {
	if q == nil {
		return
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return
	}
	q.closed = true
	q.cond.Broadcast()
}
