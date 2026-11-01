package game

import (
	"context"
	"fmt"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/talesmud/talesmud/pkg/entities/quests"
	"github.com/talesmud/talesmud/pkg/mudserver/game/def"
	"github.com/talesmud/talesmud/pkg/mudserver/game/messages"
)

// Contextual quest offer tuning (Phase 6.4). The cooldown is per character so
// one player cannot spin the generator, and it also covers the deterministic
// fallback path (which bypasses the pipeline's own limiter when no provider is
// configured).
const (
	questOfferCooldownWindow = 60 * time.Second
	questOfferTimeout        = 30 * time.Second
	questOfferMaxConcurrent  = 2
)

// ContextualQuestRequest is the game-facing alias of the def request type.
type ContextualQuestRequest = def.ContextualQuestRequest

// ContextualQuestProviderFunc is the game-facing alias of the provider type.
type ContextualQuestProviderFunc = def.ContextualQuestProviderFunc

// SetContextualQuestProvider installs (or clears, with nil) the generator used
// to produce contextual quest offers in the live game. The provider must not
// persist the quest: the game stores it when the player accepts.
func (g *Game) SetContextualQuestProvider(fn def.ContextualQuestProviderFunc) {
	if g == nil {
		return
	}
	g.questOfferMu.Lock()
	g.questOfferProvider = fn
	g.questOfferMu.Unlock()
}

// ContextualQuestProviderEnabled reports whether a provider is installed.
func (g *Game) ContextualQuestProviderEnabled() bool {
	if g == nil {
		return false
	}
	g.questOfferMu.Lock()
	defer g.questOfferMu.Unlock()
	return g.questOfferProvider != nil
}

// RequestContextualQuestOffer validates and schedules one contextual quest
// generation. When the game loop is not running (tests, startup) the provider
// runs synchronously so the offer is delivered before the call returns.
func (g *Game) RequestContextualQuestOffer(req def.ContextualQuestRequest) def.QuestOfferStatus {
	if g == nil {
		return def.QuestOfferDisabled
	}
	provider, ok := g.contextualQuestProvider()
	if !ok || strings.TrimSpace(req.CharacterID) == "" {
		return def.QuestOfferDisabled
	}

	generate := func() {
		ctx, cancel := context.WithTimeout(context.Background(), questOfferTimeout)
		defer cancel()
		q, err := provider(ctx, req)
		if err != nil {
			log.WithError(err).WithField("characterID", req.CharacterID).
				Warn("Contextual quest generation failed")
			g.deliverQuestOfferError(req)
			return
		}
		g.deliverQuestOffer(req, q)
	}

	if g.loopOn.Load() {
		sem := g.questOfferSema()
		select {
		case sem <- struct{}{}:
		default:
			return def.QuestOfferBusy
		}
		if !g.questOfferAllow(req.CharacterID) {
			<-sem
			return def.QuestOfferRateLimited
		}
		go func() {
			defer func() { <-sem }()
			generate()
		}()
		return def.QuestOfferScheduled
	}

	if !g.questOfferAllow(req.CharacterID) {
		return def.QuestOfferRateLimited
	}
	generate()
	return def.QuestOfferScheduled
}

// PendingContextualQuest returns the last generated offer awaiting acceptance.
func (g *Game) PendingContextualQuest(characterID string) (*quests.Quest, bool) {
	if g == nil {
		return nil, false
	}
	g.questOfferMu.Lock()
	defer g.questOfferMu.Unlock()
	q, ok := g.questOfferPending[characterID]
	return q, ok
}

// ClearPendingContextualQuest drops any pending offer for the character.
func (g *Game) ClearPendingContextualQuest(characterID string) {
	if g == nil {
		return
	}
	g.questOfferMu.Lock()
	delete(g.questOfferPending, characterID)
	g.questOfferMu.Unlock()
}

func (g *Game) contextualQuestProvider() (def.ContextualQuestProviderFunc, bool) {
	g.questOfferMu.Lock()
	defer g.questOfferMu.Unlock()
	if g.questOfferProvider == nil {
		return nil, false
	}
	return g.questOfferProvider, true
}

// questOfferAllow records and checks the per-character cooldown.
func (g *Game) questOfferAllow(characterID string) bool {
	g.questOfferMu.Lock()
	defer g.questOfferMu.Unlock()
	if g.questOfferCooldown == nil {
		g.questOfferCooldown = map[string]time.Time{}
	}
	now := time.Now()
	if until, ok := g.questOfferCooldown[characterID]; ok && now.Before(until) {
		return false
	}
	g.questOfferCooldown[characterID] = now.Add(questOfferCooldownWindow)
	return true
}

func (g *Game) questOfferSema() chan struct{} {
	g.questOfferMu.Lock()
	defer g.questOfferMu.Unlock()
	if g.questOfferSem == nil {
		g.questOfferSem = make(chan struct{}, questOfferMaxConcurrent)
	}
	return g.questOfferSem
}

// deliverQuestOffer posts the generated quest onto the command loop, stores it
// as pending and tells the player about it.
func (g *Game) deliverQuestOffer(req def.ContextualQuestRequest, q *quests.Quest) {
	if q == nil {
		g.deliverQuestOfferError(req)
		return
	}
	_ = g.Call(func() {
		g.questOfferMu.Lock()
		if g.questOfferPending == nil {
			g.questOfferPending = map[string]*quests.Quest{}
		}
		g.questOfferPending[req.CharacterID] = q
		g.questOfferMu.Unlock()
		g.sendQuestOfferMessage(req, q)
	})
}

func (g *Game) deliverQuestOfferError(req def.ContextualQuestRequest) {
	_ = g.Call(func() {
		g.sendQuestOfferPlain(req.UserID, "No task is within reach right now. Try again later.")
	})
}

func (g *Game) sendQuestOfferMessage(req def.ContextualQuestRequest, q *quests.Quest) {
	g.sendQuestOfferPlain(req.UserID, formatContextualQuestOffer(req.SourceNPCName, q))
}

func (g *Game) sendQuestOfferPlain(userID, text string) {
	if strings.TrimSpace(text) == "" {
		return
	}
	g.sendMessage <- messages.MessageResponse{
		Audience:   messages.MessageAudienceUser,
		AudienceID: userID,
		Type:       messages.MessageTypeDefault,
		Message:    text,
	}
}

// formatContextualQuestOffer renders the player-facing offer: title,
// description, objectives and rewards, plus how to accept or decline it.
func formatContextualQuestOffer(npcName string, q *quests.Quest) string {
	if q == nil {
		return ""
	}
	var b strings.Builder
	if name := strings.TrimSpace(npcName); name != "" {
		fmt.Fprintf(&b, "%s has work for you.\n", name)
	} else {
		b.WriteString("A task is offered to you.\n")
	}
	fmt.Fprintf(&b, "\n--- Quest Available: %s ---\n", strings.TrimSpace(q.Name))
	if desc := strings.TrimSpace(q.Description); desc != "" {
		b.WriteString(desc + "\n")
	}
	if len(q.Objectives) > 0 {
		b.WriteString("\nObjectives:\n")
		for _, obj := range q.Objectives {
			amount := obj.Amount
			if amount < 1 {
				amount = 1
			}
			fmt.Fprintf(&b, "  - %s (0/%d)\n", strings.TrimSpace(obj.Description), amount)
		}
	}
	if rewards := formatQuestOfferRewards(q); rewards != "" {
		fmt.Fprintf(&b, "\nRewards: %s\n", rewards)
	}
	b.WriteString("\nType 'offer accept' to take it, or 'offer decline' to pass.")
	return b.String()
}

func formatQuestOfferRewards(q *quests.Quest) string {
	var parts []string
	if q.Rewards.XP > 0 {
		parts = append(parts, fmt.Sprintf("%d XP", q.Rewards.XP))
	}
	if q.Rewards.Gold > 0 {
		parts = append(parts, fmt.Sprintf("%d gold", q.Rewards.Gold))
	}
	if n := len(q.Rewards.ItemTemplateIDs); n > 0 {
		parts = append(parts, fmt.Sprintf("%d item(s)", n))
	}
	return strings.Join(parts, ", ")
}
