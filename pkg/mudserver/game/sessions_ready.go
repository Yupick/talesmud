package game

import (
	"time"
)

func (r *sessionRegistry) startReadyCheck(partyID string, timeout time.Duration) {
	if partyID == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.readyChecks == nil {
		r.readyChecks = make(map[string]*readyCheckState)
	}
	rc := &readyCheckState{
		ready:   make(map[string]bool),
		started: time.Now(),
		timeout: timeout,
	}
	r.readyChecks[partyID] = rc
}

func (r *sessionRegistry) getReadyCheck(partyID string) (*readyCheckState, bool) {
	if partyID == "" {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	rc, ok := r.readyChecks[partyID]
	if !ok {
		return nil, false
	}
	return rc, true
}

func (r *sessionRegistry) clearReadyCheck(partyID string) {
	if partyID == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.readyChecks, partyID)
}

func (r *sessionRegistry) setReady(partyID, charID string, ready bool) bool {
	if partyID == "" || charID == "" {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.readyChecks == nil {
		r.readyChecks = make(map[string]*readyCheckState)
	}
	rc, ok := r.readyChecks[partyID]
	if !ok {
		rc = &readyCheckState{ready: make(map[string]bool), started: time.Now(), timeout: 30 * time.Second}
		r.readyChecks[partyID] = rc
	}
	old, had := rc.ready[charID]
	if had && old == ready {
		return false
	}
	rc.ready[charID] = ready
	return true
}

func (r *sessionRegistry) readyCheckExpired(partyID string, now time.Time) bool {
	if partyID == "" {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	rc, ok := r.readyChecks[partyID]
	if !ok {
		return false
	}
	if rc.finished {
		return true
	}
	if rc.timeout == 0 {
		rc.timeout = 30 * time.Second
	}
	if now.Sub(rc.started) >= rc.timeout {
		rc.finished = true
		return true
	}
	return false
}

func (r *sessionRegistry) finishReadyCheck(partyID string) {
	if partyID == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if rc, ok := r.readyChecks[partyID]; ok {
		rc.finished = true
	}
}

func (g *Game) StartReadyCheck(partyID string) {
	if g == nil || g.Sessions == nil {
		return
	}
	g.Sessions.startReadyCheck(partyID, 30*time.Second)
}

func (g *Game) GetReadyCheck(partyID string) (ready map[string]bool, started time.Time, ok bool) {
	if g == nil || g.Sessions == nil {
		return nil, time.Time{}, false
	}
	rc, ok := g.Sessions.getReadyCheck(partyID)
	if !ok {
		return nil, time.Time{}, false
	}
	r := make(map[string]bool, len(rc.ready))
	for k, v := range rc.ready {
		r[k] = v
	}
	return r, rc.started, true
}

func (g *Game) SetReady(partyID, charID string, ready bool) bool {
	if g == nil || g.Sessions == nil {
		return false
	}
	return g.Sessions.setReady(partyID, charID, ready)
}

func (g *Game) ReadyCheckExpired(partyID string) bool {
	if g == nil || g.Sessions == nil {
		return false
	}
	return g.Sessions.readyCheckExpired(partyID, time.Now())
}

func (g *Game) ClearReadyCheck(partyID string) {
	if g == nil || g.Sessions == nil {
		return
	}
	g.Sessions.clearReadyCheck(partyID)
}
