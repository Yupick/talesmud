package game

import (
	"context"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	def "github.com/talesmud/talesmud/pkg/mudserver/game/def"
	"github.com/talesmud/talesmud/pkg/mudserver/game/messages"
)

// DM narration tuning (Phase 6.5, mode A). Narration is prose only: it never
// changes game state. Server-side env knobs are read once in New; the defaults
// below are also used by zero-value Games in tests.
const (
	dmCooldownWindowDefault = 45 * time.Second
	dmTimeoutDefault        = 8 * time.Second
	dmMaxRunes              = 240
	dmMaxConcurrent         = 2
	dmMaxPerMinuteDefault   = 10
)

// DMNarrationRequest is the confirmed, code-derived context handed to the
// narrator. Player-authored text is never included here.
type DMNarrationRequest struct {
	Event       string
	RoomID      string
	RoomName    string
	NPCName     string
	Outcome     string
	Round       int
	PartySize   int
	UserID      string
	CharacterID string
	Audience    messages.AudienceType
	Facts       map[string]string
}

// DMNarratorFunc produces one short prose line. It must honour ctx and is
// expected to return ("", nil) when narration is disabled.
type DMNarratorFunc func(ctx context.Context, req DMNarrationRequest) (string, error)

// SetDMNarrator installs (or clears, with nil) the prose narrator.
func (g *Game) SetDMNarrator(fn DMNarratorFunc) {
	if g == nil {
		return
	}
	g.dmMu.Lock()
	g.dmNarrator = fn
	g.dmMu.Unlock()
}

// DMNarratorEnabled reports whether a narrator is installed and the runtime
// kill switch is on.
func (g *Game) DMNarratorEnabled() bool {
	if g == nil || !g.dmEnabled {
		return false
	}
	g.dmMu.Lock()
	defer g.dmMu.Unlock()
	return g.dmNarrator != nil
}

// TriggerDMNarration schedules a narration off the command loop. It is
// non-blocking and safe to call from inside the loop: the provider call runs
// in a goroutine and the resulting line is posted back through Call.
//
// A request passes through two independent throttles before spending an LLM
// call: a per-room (or per-user) cooldown and a global per-minute budget. When
// either denies, the event is dropped silently.
func (g *Game) TriggerDMNarration(req DMNarrationRequest) {
	if g == nil || !g.dmEnabled {
		return
	}
	g.dmMu.Lock()
	fn := g.dmNarrator
	g.dmMu.Unlock()
	if fn == nil {
		return
	}

	audience := req.Audience
	if audience == 0 {
		audience = dmAudienceForEvent(req.Event)
	}
	switch audience {
	case messages.MessageAudienceRoom:
		if strings.TrimSpace(req.RoomID) == "" {
			return
		}
		if !g.dmAllowRoom(req.RoomID) {
			return
		}
	case messages.MessageAudienceUser:
		if strings.TrimSpace(req.UserID) == "" {
			return
		}
		if !g.dmAllowUser(req.UserID) {
			return
		}
	default:
		// Global/system audiences are deliberately unsupported: DM prose is
		// never broadcast to everyone (design §6.5 guardrail 7).
		return
	}
	if !g.dmAllowGlobal() {
		return
	}

	if g.dmSem == nil {
		return
	}
	select {
	case g.dmSem <- struct{}{}:
	default:
		return
	}
	go func() {
		defer func() { <-g.dmSem }()
		timeout := g.dmTimeout
		if timeout <= 0 {
			timeout = dmTimeoutDefault
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		text, err := fn(ctx, req)
		if err != nil {
			return
		}
		text = cleanDMText(text)
		if text == "" {
			return
		}
		_ = g.Call(func() {
			switch audience {
			case messages.MessageAudienceUser:
				if req.UserID == "" {
					return
				}
				g.sendMessage <- messages.MessageResponse{
					Audience:   messages.MessageAudienceUser,
					AudienceID: req.UserID,
					Type:       messages.MessageTypeDefault,
					Username:   "DM",
					Message:    text,
				}
			case messages.MessageAudienceRoom:
				if req.RoomID == "" {
					return
				}
				g.sendMessage <- messages.MessageResponse{
					Audience:   messages.MessageAudienceRoom,
					AudienceID: req.RoomID,
					Type:       messages.MessageTypeDefault,
					Username:   "DM",
					Message:    text,
				}
			}
		})
	}()
}

// NotifyDM adapts the typed event bus used by the command package to the
// narrator request. It fills the fields the prompt knows about from Facts and
// keeps the audience rule in one place.
func (g *Game) NotifyDM(e def.DMEvent) {
	if g == nil {
		return
	}
	req := DMNarrationRequest{
		Event:       e.Kind,
		RoomID:      e.RoomID,
		CharacterID: e.CharacterID,
		UserID:      e.UserID,
		Audience:    dmAudienceForEvent(e.Kind),
		Facts:       e.Facts,
	}
	if e.Facts != nil {
		req.RoomName = e.Facts["roomName"]
		req.NPCName = e.Facts["npcName"]
		req.Outcome = e.Facts["outcome"]
		if v, err := strconv.Atoi(strings.TrimSpace(e.Facts["round"])); err == nil {
			req.Round = v
		}
		if v, err := strconv.Atoi(strings.TrimSpace(e.Facts["partySize"])); err == nil {
			req.PartySize = v
		}
	}
	if req.PartySize <= 0 && req.CharacterID != "" {
		req.PartySize = g.dmPartySize(req.CharacterID, 1)
	}
	if req.RoomName == "" && req.RoomID != "" {
		req.RoomName = g.dmRoomName(req.RoomID)
	}
	g.TriggerDMNarration(req)
}

// dmAudienceForEvent maps event kinds to the audience fixed by design §6.5:
// quest/level-up lines are private, every other event is room-scoped.
func dmAudienceForEvent(kind string) messages.AudienceType {
	switch kind {
	case "questCompleted", "levelUp":
		return messages.MessageAudienceUser
	default:
		return messages.MessageAudienceRoom
	}
}

// dmAllowRoom applies the per-room cooldown.
func (g *Game) dmAllowRoom(roomID string) bool {
	return g.dmAllowKey("room:" + roomID)
}

// dmAllowUser applies the per-user cooldown used by private events.
func (g *Game) dmAllowUser(userID string) bool {
	return g.dmAllowKey("user:" + userID)
}

// dmAllowKey records one successful claim for key. The cooldown window is
// runtime-configurable (LLM_DM_COOLDOWN_SECONDS, default 45s).
func (g *Game) dmAllowKey(key string) bool {
	if key == "" {
		return false
	}
	g.dmMu.Lock()
	defer g.dmMu.Unlock()
	if g.dmCooldown == nil {
		g.dmCooldown = map[string]time.Time{}
	}
	now := time.Now()
	if until, ok := g.dmCooldown[key]; ok && now.Before(until) {
		return false
	}
	window := g.dmCooldownWin
	if window <= 0 {
		window = dmCooldownWindowDefault
	}
	g.dmCooldown[key] = now.Add(window)
	return true
}

// dmAllowGlobal enforces the global per-minute budget
// (LLM_DM_MAX_PER_MIN, default 10). A value <= 0 disables narration.
func (g *Game) dmAllowGlobal() bool {
	g.dmMinuteMu.Lock()
	defer g.dmMinuteMu.Unlock()
	max := g.dmMaxPerMinute
	if max == 0 {
		max = dmMaxPerMinuteDefault
	}
	if max < 0 {
		return false
	}
	now := time.Now()
	cutoff := now.Add(-time.Minute)
	kept := g.dmMinuteEvents[:0]
	for _, at := range g.dmMinuteEvents {
		if at.After(cutoff) {
			kept = append(kept, at)
		}
	}
	g.dmMinuteEvents = kept
	if len(kept) >= max {
		return false
	}
	g.dmMinuteEvents = append(g.dmMinuteEvents, now)
	return true
}

// dmPartySize returns the real group size for characterID: the party
// membership (when any) plus any live followers, deduplicated. fallback is
// used when no richer state is available (for example, a solo fight).
func (g *Game) dmPartySize(characterID string, fallback int) int {
	if fallback < 1 {
		fallback = 1
	}
	if g == nil || characterID == "" {
		return fallback
	}
	seen := map[string]bool{characterID: true}
	size := 1
	if g.Facade != nil {
		if party, err := g.Facade.PartiesService().FindByCharacterID(characterID); err == nil && party != nil {
			for _, id := range party.Characters {
				if id == "" || seen[id] {
					continue
				}
				seen[id] = true
				size++
			}
		}
	}
	if g.Sessions != nil {
		for _, id := range g.Sessions.followersOf(characterID) {
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			size++
		}
	}
	if size < fallback {
		return fallback
	}
	return size
}

// dmRoomName resolves a room name for the prompt. The server-side provider
// repeats this lookup as a fallback, so a nil facade is harmless.
func (g *Game) dmRoomName(roomID string) string {
	if g == nil || g.Facade == nil || roomID == "" {
		return ""
	}
	room, err := g.Facade.RoomsService().FindByID(roomID)
	if err != nil || room == nil {
		return ""
	}
	return room.Name
}

func cleanDMText(text string) string {
	s := strings.TrimSpace(text)
	s = strings.Trim(s, "\"'`")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) > dmMaxRunes {
		runes := []rune(s)
		s = strings.TrimSpace(string(runes[:dmMaxRunes]))
	}
	return s
}
