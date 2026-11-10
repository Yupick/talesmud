package game

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/talesmud/talesmud/pkg/mudserver/game/messages"
)

// DM decision tuning (Phase 6.5, mode B). Unlike mode A, a decision is allowed
// to change runtime state, but only through a small, validated action set:
// spawning an ambush or broadcasting short ambient prose. The kill switch is
// deliberately off by default for beta.
const (
	dmDecisionCooldownDefault = 60 * time.Second
	dmDecisionTimeoutDefault  = 8 * time.Second
	dmDecisionMaxPerMinute    = 10
	dmDecisionMaxAmbush       = 3
)

// DMDecisionKind is the closed set of actions a DM decision may request.
type DMDecisionKind string

const (
	DMDecisionNone          DMDecisionKind = "none"
	DMDecisionAmbush        DMDecisionKind = "ambush"
	DMDecisionEnvironmental DMDecisionKind = "environmental"
	DMDecisionNarrative     DMDecisionKind = "narrative"
)

// dmDecisionKinds is the ordered whitelist used by validation and metrics.
var dmDecisionKinds = [...]DMDecisionKind{
	DMDecisionNone,
	DMDecisionAmbush,
	DMDecisionEnvironmental,
	DMDecisionNarrative,
}

// dmDecisionKindSlots is the whitelist size plus the "unknown" metrics bucket.
const dmDecisionKindSlots = len(dmDecisionKinds) + 1

// DMDecision is the validated payload returned by a DMDecisionProvider. Fields
// that do not apply to Kind are ignored by MaybeDecideDM.
type DMDecision struct {
	Kind       DMDecisionKind `json:"kind"`
	RoomID     string         `json:"roomId,omitempty"`
	TemplateID string         `json:"templateId,omitempty"`
	Count      int            `json:"count,omitempty"`
	Message    string         `json:"message,omitempty"`
	Reason     string         `json:"reason,omitempty"`
}

// DMDecisionRequest is the confirmed, code-derived context handed to the
// decision provider. Player-authored text is never included.
type DMDecisionRequest struct {
	Event       string            `json:"event,omitempty"`
	RoomID      string            `json:"roomId,omitempty"`
	RoomName    string            `json:"roomName,omitempty"`
	CharacterID string            `json:"characterId,omitempty"`
	UserID      string            `json:"userId,omitempty"`
	PartySize   int               `json:"partySize,omitempty"`
	Round       int               `json:"round,omitempty"`
	Facts       map[string]string `json:"facts,omitempty"`
}

// DMDecisionProvider asks the model for one decision. It must honour ctx and
// return an error when no decision could be produced (silence is preferred
// over a guessed action).
type DMDecisionProvider func(ctx context.Context, req DMDecisionRequest) (DMDecision, error)

// dmDecisionStats holds thread-safe counters. The zero value is ready to use.
type dmDecisionStats struct {
	requested atomic.Int64
	applied   atomic.Int64
	rejected  atomic.Int64
	errors    atomic.Int64

	lastLatencyMS    atomic.Int64
	lastDecisionUnix atomic.Int64

	// byKind is indexed by dmDecisionKindIndex; the final slot is the
	// "unknown" bucket for malformed model output.
	byKind [dmDecisionKindSlots]dmDecisionKindStats
}

type dmDecisionKindStats struct {
	requested atomic.Int64
	applied   atomic.Int64
	rejected  atomic.Int64
}

// SetDMDecisionProvider installs (or clears, with nil) the mode-B provider.
// Installing a provider does not enable the feature: the LLM_DM_DECISION_ENABLED
// kill switch still applies.
func (g *Game) SetDMDecisionProvider(fn DMDecisionProvider) {
	if g == nil {
		return
	}
	g.dmDecisionMu.Lock()
	g.dmDecisionProvider = fn
	g.dmDecisionMu.Unlock()
}

// DMDecisionEnabled reports whether the kill switch is on and a provider is
// installed.
func (g *Game) DMDecisionEnabled() bool {
	if g == nil || !g.dmDecisionEnabled {
		return false
	}
	g.dmDecisionMu.Lock()
	defer g.dmDecisionMu.Unlock()
	return g.dmDecisionProvider != nil
}

// MaybeDecideDM runs the mode-B pipeline: gate, call the provider once with a
// timeout, validate the answer against a strict whitelist, and apply the
// resulting action. The bool reports whether a decision was applied.
//
// Gating order is enabled -> provider -> room -> per-room cooldown -> global
// per-minute budget. A denied request never spends an LLM call.
func (g *Game) MaybeDecideDM(ctx context.Context, req DMDecisionRequest) (DMDecision, bool) {
	if g == nil || !g.dmDecisionEnabled {
		return DMDecision{}, false
	}
	g.dmDecisionMu.Lock()
	fn := g.dmDecisionProvider
	g.dmDecisionMu.Unlock()
	if fn == nil {
		return DMDecision{}, false
	}
	if strings.TrimSpace(req.RoomID) == "" {
		g.dmDecisionStats.rejected.Add(1)
		g.dmDecisionStats.byKind[dmDecisionKindIndex("")].rejected.Add(1)
		return DMDecision{}, false
	}
	if !g.dmDecisionAllowRoom(req.RoomID) {
		g.dmDecisionStats.rejected.Add(1)
		g.dmDecisionStats.byKind[dmDecisionKindIndex("")].rejected.Add(1)
		return DMDecision{}, false
	}
	if !g.dmDecisionAllowGlobal() {
		g.dmDecisionStats.rejected.Add(1)
		g.dmDecisionStats.byKind[dmDecisionKindIndex("")].rejected.Add(1)
		return DMDecision{}, false
	}

	if ctx == nil {
		ctx = context.Background()
	}
	timeout := g.dmDecisionTimeout
	if timeout <= 0 {
		timeout = dmDecisionTimeoutDefault
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()
	g.dmDecisionStats.requested.Add(1)
	decision, err := fn(callCtx, req)
	latency := time.Since(start).Milliseconds()
	g.dmDecisionStats.lastLatencyMS.Store(latency)
	if err != nil {
		g.dmDecisionStats.errors.Add(1)
		return DMDecision{}, false
	}

	kind := normalizeDMDecisionKind(decision.Kind)
	idx := dmDecisionKindIndex(kind)
	g.dmDecisionStats.byKind[idx].requested.Add(1)

	switch kind {
	case DMDecisionNone:
		g.dmDecisionStats.rejected.Add(1)
		g.dmDecisionStats.byKind[idx].rejected.Add(1)
		return DMDecision{}, false

	case DMDecisionAmbush:
		if !g.dmDecisionValidAmbush(decision) {
			g.dmDecisionStats.rejected.Add(1)
			g.dmDecisionStats.byKind[idx].rejected.Add(1)
			return DMDecision{}, false
		}
		count := clampAmbushCount(decision.Count)
		// The model never chooses the destination room.
		decision.RoomID = req.RoomID
		decision.Count = count
		decision.Message = ""
		for i := 0; i < count; i++ {
			if err := g.spawnForDMDecision(decision.TemplateID, req.RoomID); err != nil {
				g.dmDecisionStats.errors.Add(1)
				return DMDecision{}, false
			}
		}
		g.dmDecisionStats.applied.Add(1)
		g.dmDecisionStats.byKind[idx].applied.Add(1)
		g.dmDecisionStats.lastDecisionUnix.Store(time.Now().Unix())
		return decision, true

	case DMDecisionEnvironmental, DMDecisionNarrative:
		text := cleanDMText(decision.Message)
		if text == "" {
			g.dmDecisionStats.rejected.Add(1)
			g.dmDecisionStats.byKind[idx].rejected.Add(1)
			return DMDecision{}, false
		}
		// Prose decisions are pure room ambience: no state is mutated.
		decision.RoomID = req.RoomID
		decision.TemplateID = ""
		decision.Count = 0
		decision.Message = text
		g.broadcastDMDecision(req.RoomID, text)
		g.dmDecisionStats.applied.Add(1)
		g.dmDecisionStats.byKind[idx].applied.Add(1)
		g.dmDecisionStats.lastDecisionUnix.Store(time.Now().Unix())
		return decision, true

	default:
		g.dmDecisionStats.rejected.Add(1)
		g.dmDecisionStats.byKind[idx].rejected.Add(1)
		return DMDecision{}, false
	}
}

// DMDecisionMetrics returns a snapshot of the mode-B counters. Keys are:
// requested, applied, rejected, errors, last_latency_ms, last_decision_unix and
// <requested|applied|rejected>_<kind> for every whitelisted kind plus
// "unknown" for malformed model output.
func (g *Game) DMDecisionMetrics() map[string]int64 {
	out := map[string]int64{}
	if g == nil {
		return out
	}
	out["requested"] = g.dmDecisionStats.requested.Load()
	out["applied"] = g.dmDecisionStats.applied.Load()
	out["rejected"] = g.dmDecisionStats.rejected.Load()
	out["errors"] = g.dmDecisionStats.errors.Load()
	out["last_latency_ms"] = g.dmDecisionStats.lastLatencyMS.Load()
	out["last_decision_unix"] = g.dmDecisionStats.lastDecisionUnix.Load()

	for i := 0; i < len(g.dmDecisionStats.byKind); i++ {
		kind := "unknown"
		if i < len(dmDecisionKinds) {
			kind = string(dmDecisionKinds[i])
		}
		out["requested_"+kind] = g.dmDecisionStats.byKind[i].requested.Load()
		out["applied_"+kind] = g.dmDecisionStats.byKind[i].applied.Load()
		out["rejected_"+kind] = g.dmDecisionStats.byKind[i].rejected.Load()
	}
	return out
}

// spawnForDMDecision applies an ambush. Tests may inject a counter through
// dmDecisionSpawn; production uses the runtime NPC manager. Without either,
// the decision is rejected instead of silently doing nothing.
func (g *Game) spawnForDMDecision(templateID, roomID string) error {
	if g == nil {
		return errors.New("dm decision: nil game")
	}
	if g.dmDecisionSpawn != nil {
		return g.dmDecisionSpawn(templateID, roomID)
	}
	if g.NPCManager == nil {
		return errors.New("dm decision: npc manager unavailable")
	}
	_, err := g.NPCManager.SpawnInstanceDirect(templateID, roomID)
	return err
}

func (g *Game) dmDecisionValidAmbush(decision DMDecision) bool {
	if strings.TrimSpace(decision.TemplateID) == "" {
		return false
	}
	// A negative count is meaningless. Zero means "one" because JSON authors
	// often omit it; counts above the cap are clamped in clampAmbushCount.
	if decision.Count < 0 {
		return false
	}
	if g.dmDecisionSpawn == nil && g.NPCManager == nil {
		return false
	}
	return true
}

func clampAmbushCount(count int) int {
	if count <= 0 {
		return 1
	}
	if count > dmDecisionMaxAmbush {
		return dmDecisionMaxAmbush
	}
	return count
}

// broadcastDMDecision posts one room-scoped prose line. It never edits combat
// or inventory state.
func (g *Game) broadcastDMDecision(roomID, text string) {
	if g == nil || strings.TrimSpace(roomID) == "" || strings.TrimSpace(text) == "" {
		return
	}
	g.sendMessage <- messages.MessageResponse{
		Audience:   messages.MessageAudienceRoom,
		AudienceID: roomID,
		Type:       messages.MessageTypeDefault,
		Username:   "DM",
		Message:    text,
	}
}

// maybeTriggerDMDecision schedules one asynchronous decision for an already
// confirmed event. It is fire-and-forget: the provider call must never block
// the command loop.
func (g *Game) maybeTriggerDMDecision(req DMDecisionRequest) {
	if g == nil || !g.dmDecisionEnabled || strings.TrimSpace(req.RoomID) == "" {
		return
	}
	go func() {
		_, _ = g.MaybeDecideDM(context.Background(), req)
	}()
}

func (g *Game) dmDecisionAllowRoom(roomID string) bool {
	if g == nil || strings.TrimSpace(roomID) == "" {
		return false
	}
	g.dmDecisionMu.Lock()
	defer g.dmDecisionMu.Unlock()
	if g.dmDecisionCooldown == nil {
		g.dmDecisionCooldown = map[string]time.Time{}
	}
	key := "room:" + roomID
	now := time.Now()
	if until, ok := g.dmDecisionCooldown[key]; ok && now.Before(until) {
		return false
	}
	window := g.dmDecisionWindow
	if window <= 0 {
		window = dmDecisionCooldownDefault
	}
	g.dmDecisionCooldown[key] = now.Add(window)
	return true
}

func (g *Game) dmDecisionAllowGlobal() bool {
	g.dmDecisionMinuteMu.Lock()
	defer g.dmDecisionMinuteMu.Unlock()
	max := g.dmDecisionMaxPerMinute
	if max == 0 {
		max = dmDecisionMaxPerMinute
	}
	if max < 0 {
		return false
	}
	now := time.Now()
	cutoff := now.Add(-time.Minute)
	kept := g.dmDecisionMinuteEvents[:0]
	for _, at := range g.dmDecisionMinuteEvents {
		if at.After(cutoff) {
			kept = append(kept, at)
		}
	}
	g.dmDecisionMinuteEvents = kept
	if len(kept) >= max {
		return false
	}
	g.dmDecisionMinuteEvents = append(g.dmDecisionMinuteEvents, now)
	return true
}

func normalizeDMDecisionKind(kind DMDecisionKind) DMDecisionKind {
	s := strings.ToLower(strings.TrimSpace(string(kind)))
	switch s {
	case "", string(DMDecisionNone):
		return DMDecisionNone
	case string(DMDecisionAmbush):
		return DMDecisionAmbush
	case string(DMDecisionEnvironmental):
		return DMDecisionEnvironmental
	case string(DMDecisionNarrative):
		return DMDecisionNarrative
	default:
		return DMDecisionKind(s)
	}
}

// dmDecisionKindIndex maps a kind to its metrics slot. Unknown values use the
// final ("unknown") slot.
func dmDecisionKindIndex(kind DMDecisionKind) int {
	for i, k := range dmDecisionKinds {
		if kind == k {
			return i
		}
	}
	return len(dmDecisionKinds)
}

// dmDecisionKillSwitch reads LLM_DM_DECISION_ENABLED. Mode B is opt-in: any
// value other than an explicit truthy one keeps it off.
func dmDecisionKillSwitch() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("LLM_DM_DECISION_ENABLED"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
