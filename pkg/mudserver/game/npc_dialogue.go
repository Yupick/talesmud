package game

import (
	"context"
	"encoding/json"
	"strings"
	"time"
	"unicode/utf8"

	log "github.com/sirupsen/logrus"

	"github.com/talesmud/talesmud/pkg/entities/conversations"
	"github.com/talesmud/talesmud/pkg/entities/dialogs"
	npc "github.com/talesmud/talesmud/pkg/entities/npcs"
	"github.com/talesmud/talesmud/pkg/mudserver/game/def"
	"github.com/talesmud/talesmud/pkg/mudserver/game/messages"
)

// Free-form NPC dialogue tuning (Phase 6.3 runtime cut). The per-pair cooldown
// is supplied by the NPC (LLMConfig.CooldownSeconds) through the request; the
// generation semaphore keeps slow providers from piling up goroutines.
const (
	npcDialogueTimeout       = 20 * time.Second
	npcDialogueMaxConcurrent = 2
	npcDialogueReplyRunes    = 400
	npcDialogueMaxOptions    = 4
)

// NPCDialogueRequest is the game-facing alias of the def request type.
type NPCDialogueRequest = def.NPCDialogueRequest

// NPCDialogueResult is the game-facing alias of the def result type.
type NPCDialogueResult = def.NPCDialogueResult

// NPCDialogueOption is the game-facing alias of the def option type.
type NPCDialogueOption = def.NPCDialogueOption

// NPCDialogueProviderFunc is the game-facing alias of the provider type.
type NPCDialogueProviderFunc = def.NPCDialogueProviderFunc

// SetNPCDialogueProvider installs (or clears, with nil) the generator used for
// free-form NPC dialogue. The provider must not persist anything: memory is
// recorded by the server wiring after a successful generation.
func (g *Game) SetNPCDialogueProvider(fn def.NPCDialogueProviderFunc) {
	if g == nil {
		return
	}
	g.npcDialogueMu.Lock()
	g.npcDialogueProvider = fn
	g.npcDialogueMu.Unlock()
}

// NPCDialogueProviderEnabled reports whether a dialogue provider is installed.
func (g *Game) NPCDialogueProviderEnabled() bool {
	if g == nil {
		return false
	}
	g.npcDialogueMu.Lock()
	defer g.npcDialogueMu.Unlock()
	return g.npcDialogueProvider != nil
}

// RequestNPCDialogue validates and schedules one free-form NPC reply. The
// provider runs off the command loop and the reply is posted back through
// Call. When the loop is not running (tests, startup) generation runs inline
// so the reply is delivered before the call returns.
func (g *Game) RequestNPCDialogue(req def.NPCDialogueRequest) def.NPCDialogueStatus {
	if g == nil {
		return def.NPCDialogueDisabled
	}
	provider, ok := g.npcDialogueProviderFunc()
	if !ok || strings.TrimSpace(req.CharacterID) == "" ||
		strings.TrimSpace(req.NPCID) == "" || strings.TrimSpace(req.UserID) == "" {
		return def.NPCDialogueDisabled
	}

	generate := func() {
		ctx, cancel := context.WithTimeout(context.Background(), npcDialogueTimeout)
		defer cancel()
		res, err := provider(ctx, req)
		if err != nil {
			log.WithError(err).WithFields(log.Fields{
				"characterID": req.CharacterID,
				"npcID":       req.NPCID,
			}).Warn("NPC dialogue generation failed")
			g.deliverNPCDialogueError(req)
			return
		}
		g.deliverNPCDialogue(req, res)
	}

	if g.loopOn.Load() {
		sem := g.npcDialogueSema()
		select {
		case sem <- struct{}{}:
		default:
			return def.NPCDialogueBusy
		}
		if !g.npcDialogueAllow(req) {
			<-sem
			return def.NPCDialogueRateLimited
		}
		go func() {
			defer func() { <-sem }()
			generate()
		}()
		return def.NPCDialogueScheduled
	}

	if !g.npcDialogueAllow(req) {
		return def.NPCDialogueRateLimited
	}
	generate()
	return def.NPCDialogueScheduled
}

func (g *Game) npcDialogueProviderFunc() (def.NPCDialogueProviderFunc, bool) {
	g.npcDialogueMu.Lock()
	defer g.npcDialogueMu.Unlock()
	if g.npcDialogueProvider == nil {
		return nil, false
	}
	return g.npcDialogueProvider, true
}

// npcDialogueAllow records and checks the per-(character, NPC) cooldown.
func (g *Game) npcDialogueAllow(req def.NPCDialogueRequest) bool {
	g.npcDialogueMu.Lock()
	defer g.npcDialogueMu.Unlock()
	if g.npcDialogueCooldown == nil {
		g.npcDialogueCooldown = map[string]time.Time{}
	}
	window := req.CooldownSeconds
	if window <= 0 {
		window = npc.DefaultLLMCooldownSeconds
	}
	key := req.CharacterID + "::" + req.NPCID
	now := time.Now()
	if !req.Continuation {
		if until, ok := g.npcDialogueCooldown[key]; ok && now.Before(until) {
			return false
		}
	}
	// Even a continuation refreshes the window so a follow-up cannot be used
	// to bypass the per-pair cooldown for the next direct ask.
	g.npcDialogueCooldown[key] = now.Add(time.Duration(window) * time.Second)
	return true
}

func (g *Game) npcDialogueSema() chan struct{} {
	g.npcDialogueMu.Lock()
	defer g.npcDialogueMu.Unlock()
	if g.npcDialogueSem == nil {
		g.npcDialogueSem = make(chan struct{}, npcDialogueMaxConcurrent)
	}
	return g.npcDialogueSem
}

// deliverNPCDialogue posts the generated reply onto the command loop. When the
// model returned options they become a real conversation (DialogID "__llm__")
// with numeric choices handled by DialogSelectCommand. If a conversation
// cannot be opened the reply falls back to the pre-corte-3 plain text.
func (g *Game) deliverNPCDialogue(req def.NPCDialogueRequest, res def.NPCDialogueResult) {
	_ = g.Call(func() {
		reply := sanitizeNPCDialogueLine(res.Reply, npcDialogueReplyRunes)
		if reply == "" {
			reply = neutralNPCDialogueReply(req.NPCName)
		}
		options := normalizeNPCDialogueOptions(res.Options)
		if len(options) == 0 {
			g.sendNPCDialoguePlainWithRelation(req, reply, res.RelationDelta)
			return
		}
		options = g.mapNPCDialogueOptions(req, options)
		if g.sendNPCDialogueDialog(req, reply, options, res.RelationDelta) {
			return
		}
		// No conversation service (or update failed): preserve the old
		// plain-text rendering so the player still sees the choices.
		res.Reply = reply
		res.Options = options
		g.sendNPCDialoguePlainWithRelation(req, formatNPCDialogueReply(res), res.RelationDelta)
	})
}

// sendNPCDialogueDialog opens (or refreshes) the sentinel LLM conversation and
// sends the reply as a DialogMessage with selectable options. It reports false
// when the conversation store is unavailable so the caller can fall back to
// plain text.
func (g *Game) sendNPCDialogueDialog(req def.NPCDialogueRequest, reply string, options []def.NPCDialogueOption, relationDelta float64) bool {
	if g.Facade == nil || strings.TrimSpace(req.UserID) == "" {
		return false
	}
	svc := g.Facade.ConversationsService()
	if svc == nil {
		return false
	}
	conv, err := svc.GetOrCreateConversation(req.CharacterID, req.NPCID, conversations.TargetTypeNPC, def.NPCDialogueDialogID)
	if err != nil || conv == nil {
		log.WithError(err).WithFields(log.Fields{
			"characterID": req.CharacterID,
			"npcID":       req.NPCID,
		}).Warn("NPC dialogue: could not open conversation")
		return false
	}
	payload, err := json.Marshal(options)
	if err != nil {
		return false
	}
	conv.DialogID = def.NPCDialogueDialogID
	conv.CurrentNodeID = "main"
	conv.SetContext("PLAYER", req.CharacterName)
	conv.SetContext("NPC", req.NPCName)
	conv.SetContext(def.NPCDialogueContextOptions, string(payload))
	conv.SetContext(def.NPCDialogueContextTemplate, req.NPCTemplateID)
	conv.SetContext(def.NPCDialogueContextQuestion, req.Question)
	conv.SetContext(def.NPCDialogueContextReply, reply)
	if err := svc.Update(conv.ID, conv); err != nil {
		log.WithError(err).WithField("conversationID", conv.ID).Warn("NPC dialogue: could not persist conversation")
		return false
	}

	dialogOptions := make([]messages.DialogOption, len(options))
	for i, opt := range options {
		dialogOptions[i] = messages.DialogOption{Index: i + 1, Text: opt.Label, Kind: opt.Kind}
	}
	if g.NPCManager != nil {
		g.NPCManager.EngageNPC(req.CharacterID, req.NPCID, "dialog")
	}
	dialogMsg := messages.NewDialogMessage(req.UserID, req.NPCName, reply, dialogOptions, conv.ID)
	// DialogMessage historically leaves Username empty; set it so both the
	// plain-text and dialog transports agree on the author.
	dialogMsg.Username = req.NPCName
	dialogMsg.RelationDelta = relationDelta
	dialogMsg.FreeText = true
	g.sendMessage <- dialogMsg
	return true
}

// mapNPCDialogueOptions enriches model options with authored dialog node IDs
// when a label matches the NPC's authored tree. A mapped option continues that
// authored node; unmapped flavor/end options keep their LLM behaviour.
func (g *Game) mapNPCDialogueOptions(req def.NPCDialogueRequest, options []def.NPCDialogueOption) []def.NPCDialogueOption {
	dialogID := g.npcDialogueAuthoredDialogID(req)
	if dialogID == "" || g.Facade == nil || g.Facade.DialogsService() == nil {
		return options
	}
	dialog, err := g.Facade.DialogsService().FindByID(dialogID)
	if err != nil || dialog == nil {
		return options
	}
	for i := range options {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(options[i].Kind)), "quest:") {
			continue
		}
		if nodeID := matchAuthoredDialogLabel(dialog, options[i].Label); nodeID != "" {
			options[i].NodeID = nodeID
		}
	}
	return options
}

func (g *Game) npcDialogueAuthoredDialogID(req def.NPCDialogueRequest) string {
	if g.NPCManager != nil {
		if inst := g.NPCManager.GetInstance(req.NPCID); inst != nil {
			if id := strings.TrimSpace(inst.DialogID); id != "" {
				return id
			}
			if inst.LLM != nil {
				if id := strings.TrimSpace(inst.LLM.FallbackDialogID); id != "" {
					return id
				}
			}
		}
	}
	if g.Facade == nil || g.Facade.NPCsService() == nil {
		return ""
	}
	for _, id := range []string{req.NPCID, req.NPCTemplateID} {
		if strings.TrimSpace(id) == "" {
			continue
		}
		if n, err := g.Facade.NPCsService().FindByID(id); err == nil && n != nil {
			if dialogID := strings.TrimSpace(n.DialogID); dialogID != "" {
				return dialogID
			}
			if n.LLM != nil {
				if dialogID := strings.TrimSpace(n.LLM.FallbackDialogID); dialogID != "" {
					return dialogID
				}
			}
		}
	}
	return ""
}

// matchAuthoredDialogLabel returns the authored NodeID whose text/name (or
// option text/name) matches the model label. The walk is cycle-safe because
// authored answers can back-reference their parent node.
func matchAuthoredDialogLabel(dialog *dialogs.Dialog, label string) string {
	target := normalizeDialogueLabel(label)
	if target == "" || dialog == nil {
		return ""
	}
	if normalizeDialogueLabel(dialog.Name) == target && strings.TrimSpace(dialog.NodeID) != "" {
		return dialog.NodeID
	}
	visited := map[*dialogs.Dialog]bool{}
	var walk func(node *dialogs.Dialog) string
	walk = func(node *dialogs.Dialog) string {
		if node == nil || visited[node] {
			return ""
		}
		visited[node] = true
		if strings.TrimSpace(node.NodeID) != "" &&
			(normalizeDialogueLabel(node.Text) == target || normalizeDialogueLabel(node.Name) == target) {
			return node.NodeID
		}
		for _, alternate := range node.AlternateTexts {
			if normalizeDialogueLabel(alternate) == target && strings.TrimSpace(node.NodeID) != "" {
				return node.NodeID
			}
		}
		for _, opt := range node.Options {
			if opt == nil {
				continue
			}
			if strings.TrimSpace(opt.NodeID) != "" &&
				(normalizeDialogueLabel(opt.Text) == target || normalizeDialogueLabel(opt.Name) == target) {
				return opt.NodeID
			}
			if found := walk(opt); found != "" {
				return found
			}
		}
		if node.Answer != nil {
			return walk(node.Answer)
		}
		return ""
	}
	return walk(dialog)
}

func normalizeDialogueLabel(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.Trim(s, " \t\r\n?!.,;:¡¿\"'")
	return strings.Join(strings.Fields(s), " ")
}

func (g *Game) deliverNPCDialogueError(req def.NPCDialogueRequest) {
	_ = g.Call(func() {
		g.sendNPCDialoguePlain(req, neutralNPCDialogueReply(req.NPCName))
	})
}

func (g *Game) sendNPCDialoguePlain(req def.NPCDialogueRequest, text string) {
	g.sendNPCDialoguePlainWithRelation(req, text, 0)
}

func (g *Game) sendNPCDialoguePlainWithRelation(req def.NPCDialogueRequest, text string, relationDelta float64) {
	if strings.TrimSpace(text) == "" || strings.TrimSpace(req.UserID) == "" {
		return
	}
	username := strings.TrimSpace(req.NPCName)
	if username == "" {
		username = "NPC"
	}
	g.sendMessage <- messages.MessageResponse{
		Audience:      messages.MessageAudienceUser,
		AudienceID:    req.UserID,
		Type:          messages.MessageTypeDefault,
		Username:      username,
		Message:       text,
		RelationDelta: relationDelta,
	}
}

// formatNPCDialogueReply renders the NPC reply and any suggested follow-ups as
// plain text, so LLM options never interfere with the authored dialog system.
func formatNPCDialogueReply(res def.NPCDialogueResult) string {
	reply := sanitizeNPCDialogueLine(res.Reply, npcDialogueReplyRunes)
	if reply == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString(reply)
	options := normalizeNPCDialogueOptions(res.Options)
	if len(options) > 0 {
		b.WriteString("\n\n")
		for _, opt := range options {
			b.WriteString("- ")
			b.WriteString(opt.Label)
			b.WriteString("\n")
		}
	}
	return strings.TrimSpace(b.String())
}

func normalizeNPCDialogueOptions(options []def.NPCDialogueOption) []def.NPCDialogueOption {
	out := make([]def.NPCDialogueOption, 0, len(options))
	for _, opt := range options {
		label := sanitizeNPCDialogueLine(opt.Label, 120)
		if label == "" {
			continue
		}
		kind := strings.TrimSpace(opt.Kind)
		lower := strings.ToLower(kind)
		if lower == "flavor" || lower == "end" {
			kind = lower
		} else if len(kind) >= len("quest:") && strings.EqualFold(kind[:len("quest:")], "quest:") {
			kind = "quest:" + strings.TrimSpace(kind[len("quest:"):])
		} else if kind == "" {
			kind = "flavor"
		}
		out = append(out, def.NPCDialogueOption{Label: label, Kind: kind})
		if len(out) >= npcDialogueMaxOptions {
			break
		}
	}
	return out
}

// sanitizeNPCDialogueLine strips control characters, collapses whitespace and
// caps the line length defensively (the pipeline already validates the model
// output).
func sanitizeNPCDialogueLine(s string, maxRunes int) string {
	s = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	if maxRunes > 0 && utf8.RuneCountInString(s) > maxRunes {
		s = strings.TrimSpace(string([]rune(s)[:maxRunes]))
	}
	return s
}

func neutralNPCDialogueReply(npcName string) string {
	if name := strings.TrimSpace(npcName); name != "" {
		return name + " te mira en silencio."
	}
	return "El NPC te mira en silencio."
}
