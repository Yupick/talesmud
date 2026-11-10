package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	log "github.com/sirupsen/logrus"
	npc "github.com/talesmud/talesmud/pkg/entities/npcs"
	"github.com/talesmud/talesmud/pkg/entities/quests"
	"github.com/talesmud/talesmud/pkg/mudserver/game"
	"github.com/talesmud/talesmud/pkg/mudserver/game/commands"
	"github.com/talesmud/talesmud/pkg/repository"
	"github.com/talesmud/talesmud/pkg/service"
	"github.com/talesmud/talesmud/pkg/service/llm"
	"github.com/talesmud/talesmud/pkg/service/llmgen"
	"github.com/talesmud/talesmud/pkg/service/llmprompt"
)

// llmRuntime bundles the shared LLM building blocks wired once at startup.
type llmRuntime struct {
	registry llmprompt.Registry
	pipeline *llmgen.Pipeline
	questgen *llmgen.QuestGenerator
}

// buildLLMRuntime loads the prompt registry, builds the generation pipeline
// and the quest generator. Failures degrade to a nil registry/pipeline so
// the server still starts without AI features.
func buildLLMRuntime(facade service.Facade) llmRuntime {
	rt := llmRuntime{}
	reg, err := llmprompt.NewRegistry(strings.TrimSpace(os.Getenv("LLM_PROMPT_DIR")))
	if err != nil {
		log.WithError(err).Error("Failed to load LLM prompt registry; AI features disabled")
		return rt
	}
	rt.registry = reg
	rt.pipeline = llmgen.New(llm.NewFromEnv(), reg, llmgen.ConfigFromEnv())
	if facade != nil {
		rt.questgen = llmgen.NewQuestGenerator(rt.pipeline, questCatalogFunc(facade), questValidator(facade.QuestsService()))
	}
	return rt
}

// questCatalogFunc returns a closure that snapshots the world for prompt
// catalog injection and deterministic quest fallbacks.
func questCatalogFunc(facade service.Facade) func() llmgen.QuestCatalog {
	return func() llmgen.QuestCatalog {
		cat := llmgen.QuestCatalog{MaxXP: 500, MaxGold: 200}
		areaSeen := map[string]bool{}

		if rooms, err := facade.RoomsService().FindAll(); err == nil {
			for _, r := range rooms {
				if r == nil {
					continue
				}
				cat.Rooms = append(cat.Rooms, llmgen.CatalogRef{
					ID: r.ID, Name: r.Name, Kind: "room", Area: r.Area,
				})
				if a := strings.TrimSpace(r.Area); a != "" && !areaSeen[a] {
					areaSeen[a] = true
					cat.Areas = append(cat.Areas, llmgen.CatalogRef{ID: a, Name: a, Kind: "area"})
				}
			}
		}
		if npcs, err := facade.NPCsService().FindAll(); err == nil {
			for _, n := range npcs {
				if n == nil {
					continue
				}
				cat.NPCs = append(cat.NPCs, llmgen.CatalogRef{
					ID: n.ID, Name: n.Name, Kind: "npc", Level: int(n.Level),
				})
			}
		}
		if itemsList, err := facade.ItemsService().FindAllTemplates(repository.ItemsQuery{}); err == nil {
			for _, it := range itemsList {
				if it == nil {
					continue
				}
				cat.Items = append(cat.Items, llmgen.CatalogRef{
					ID: it.ID, Name: it.Name, Kind: "item",
				})
			}
		}
		return cat
	}
}

// questValidator adapts the quest service validator to the llmgen shape.
func questValidator(svc service.QuestsService) llmgen.QuestValidator {
	return func(q *quests.Quest) []llmgen.QuestIssue {
		if svc == nil {
			return nil
		}
		issues := svc.ValidateQuest(q)
		out := make([]llmgen.QuestIssue, 0, len(issues))
		for _, i := range issues {
			out = append(out, llmgen.QuestIssue{
				Severity: i.Severity,
				Path:     i.Path,
				Code:     i.Code,
				Message:  i.Message,
			})
		}
		return out
	}
}

// installDMNarrator wires the mode-A prose narrator onto the live game. It is
// a no-op when no provider is configured or when LLM_DM_ENABLED is off.
func (rt llmRuntime) installDMNarrator(g *game.Game, facade service.Facade) {
	if g == nil || rt.pipeline == nil || !rt.pipeline.Enabled() || !dmNarrationEnabled() {
		return
	}
	g.SetDMNarrator(func(ctx context.Context, req game.DMNarrationRequest) (string, error) {
		roomName := req.RoomName
		if roomName == "" && req.RoomID != "" && facade != nil {
			if room, err := facade.RoomsService().FindByID(req.RoomID); err == nil && room != nil {
				roomName = room.Name
			}
		}
		limitKey := "dm:room:" + req.RoomID
		if strings.TrimSpace(req.RoomID) == "" {
			limitKey = "dm:user:" + req.UserID
		}
		resp, err := rt.pipeline.Generate(ctx, llmgen.Request{
			Task:       llmgen.TaskDMNarration,
			UserID:     "dm",
			LimitKey:   limitKey,
			Timeout:    8 * time.Second,
			RequireLLM: true, // design §6.5: fallback is silence, not deterministic prose
			Vars: map[string]any{
				"event":    req.Event,
				"roomName": roomName,
				"npcName":  req.NPCName,
				"outcome":  req.Outcome,
				"round":    req.Round,
				"party":    req.PartySize,
			},
		})
		if err != nil {
			return "", err
		}
		return resp.Text, nil
	})
}

// installDMDecision wires the mode-B decision provider onto the live game. It is
// a no-op without a configured LLM or when the LLM_DM_DECISION_ENABLED kill
// switch is off, so beta deployments keep the DM silent by default.
func (rt llmRuntime) installDMDecision(g *game.Game, facade service.Facade) {
	if g == nil || rt.pipeline == nil || !rt.pipeline.Enabled() || !dmDecisionEnabled() {
		return
	}
	client := rt.pipeline.Client()
	if client == nil {
		return
	}
	g.SetDMDecisionProvider(func(ctx context.Context, req game.DMDecisionRequest) (game.DMDecision, error) {
		roomName := strings.TrimSpace(req.RoomName)
		if roomName == "" && req.RoomID != "" && facade != nil {
			if room, err := facade.RoomsService().FindByID(req.RoomID); err == nil && room != nil {
				roomName = room.Name
			}
		}
		catalog := dmDecisionTemplateCatalog(facade)
		facts := dmDecisionFactsLine(req.Facts)
		system := `Eres el Dungeon Master de un MUD de fantasía en español.
Decide si añadir UN giro de ambiente al evento. Responde SOLO con un objeto JSON, sin markdown:
{"kind":"none|ambush|environmental|narrative","templateId":"","count":0,"message":"","reason":""}
Reglas:
- Usa "none" si no conviene ningún giro.
- "ambush" solo con un templateId EXACTO de la lista y count entre 1 y 3.
- "environmental" o "narrative" solo requieren message breve (máximo 240 caracteres).
- No inventes ids, no reveles secretos y no describas cambios de estado no confirmados.
- Si eliges prosa, no incluyas números de mecánica ni ids.`
		user := fmt.Sprintf(
			"Evento: %s\nSala: %s\nGrupo: %d\nRonda: %d\nHechos: %s\nPlantillas disponibles:\n%s",
			oneLine(req.Event), oneLine(roomName), req.PartySize, req.Round, facts, catalog,
		)
		raw, err := llm.Complete(ctx, client, system, user, llm.Options{Temperature: 0.4, MaxTokens: 220})
		if err != nil {
			return game.DMDecision{}, err
		}
		payload, err := llmgen.ExtractJSON(raw)
		if err != nil {
			return game.DMDecision{}, fmt.Errorf("dm decision: extract json: %w", err)
		}
		var decision game.DMDecision
		if err := json.Unmarshal(payload, &decision); err != nil {
			return game.DMDecision{}, fmt.Errorf("dm decision: decode payload: %w", err)
		}
		return decision, nil
	})
}

// dmDecisionTemplateCatalog renders a short id/name list for the decision
// prompt. The game layer still validates the returned id by attempting the
// spawn, so a truncated catalog is safe.
func dmDecisionTemplateCatalog(facade service.Facade) string {
	if facade == nil {
		return "(sin catálogo)"
	}
	templates, err := facade.NPCsService().FindAllTemplates()
	if err != nil || len(templates) == 0 {
		return "(sin plantillas)"
	}
	var b strings.Builder
	count := 0
	for _, t := range templates {
		if t == nil || strings.TrimSpace(t.ID) == "" {
			continue
		}
		fmt.Fprintf(&b, "- %s (%s)\n", t.ID, oneLine(t.Name))
		count++
		if count >= 12 {
			break
		}
	}
	if b.Len() == 0 {
		return "(sin plantillas)"
	}
	return b.String()
}

// dmDecisionFactsLine flattens code-derived facts into one prompt line.
func dmDecisionFactsLine(facts map[string]string) string {
	if len(facts) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(facts))
	for k, v := range facts {
		k, v = oneLine(k), oneLine(v)
		if k == "" || v == "" {
			continue
		}
		parts = append(parts, k+"="+v)
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, "; ")
}

// oneLine collapses whitespace so prompt fields cannot inject new sections.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// dmDecisionEnabled reports the LLM_DM_DECISION_ENABLED kill switch (default
// off for beta).
func dmDecisionEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("LLM_DM_DECISION_ENABLED"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// installContextualQuestProvider wires dynamic quest offers onto the live game.
// It works with or without an LLM provider: when no provider is configured the
// quest generator degrades to its deterministic fallback. It is a no-op when
// the generator could not be built.
func (rt llmRuntime) installContextualQuestProvider(g *game.Game, facade service.Facade) {
	if g == nil || rt.questgen == nil {
		return
	}
	g.SetContextualQuestProvider(func(ctx context.Context, req game.ContextualQuestRequest) (*quests.Quest, error) {
		res, err := rt.questgen.GenerateContextualQuest(ctx, llmgen.QuestContext{
			PlayerLevel: req.PlayerLevel,
			ZoneID:      req.ZoneID,
			Theme:       req.Theme,
			SourceNPCID: req.SourceNPCID,
			CharacterID: req.CharacterID,
			UserID:      req.UserID,
		})
		if err != nil {
			return nil, err
		}
		bindQuestToSourceNPC(facade, res.Quest, req.SourceNPCID)
		return res.Quest, nil
	})
}

// installNPCDialogueProvider wires free-form NPC dialogue onto the live game.
// It is a no-op without an actual LLM provider: without one the command
// degrades to the authored dialog path instead of paying for a doomed call.
func (rt llmRuntime) installNPCDialogueProvider(g *game.Game, facade service.Facade) {
	if g == nil || rt.pipeline == nil || !rt.pipeline.Enabled() {
		return
	}
	g.SetNPCDialogueProvider(func(ctx context.Context, req game.NPCDialogueRequest) (game.NPCDialogueResult, error) {
		memory := strings.TrimSpace(req.Memory)
		memObj, effectiveTurns := loadNPCMemoryFull(facade, req)
		if renderedMem := formatNPCMemoryLimited(memObj, effectiveTurns); renderedMem != "" {
			memory = renderedMem
		} else if mem := loadNPCMemory(facade, req.NPCTemplateID, req.CharacterID); mem != "" && memObj == nil {
			// fallback to old path if memObj not loaded
			memory = mem
		}
		roomName := ""
		if facade != nil && strings.TrimSpace(req.RoomID) != "" {
			if room, err := facade.RoomsService().FindByID(req.RoomID); err == nil && room != nil {
				roomName = room.Name
			}
		}

		llmCfg := npcLLMConfig(facade, req)
		personalityText, goalsText, secretsText, relationsText := formatNPCPersonalityContext(llmCfg)
		relationsText = mergeDynamicRelations(relationsText, memObj)
		questPrompt, questIDs := commands.AvailableQuestPromptContext(facade, req.CharacterID, req.NPCTemplateID, req.NPCID)

		resp, err := rt.pipeline.Generate(ctx, llmgen.Request{
			Task:        llmgen.TaskNPCDialogue,
			UserID:      req.UserID,
			RoomID:      req.RoomID,
			Timeout:     20 * time.Second,
			RequireLLM:  true,
			LimitKey:    "npc:" + req.NPCID + ":" + req.CharacterID,
			Temperature: req.Temperature,
			Vars: map[string]any{
				"npcName":     req.NPCName,
				"persona":     req.Persona,
				"voice":       req.Voice,
				"roomName":    roomName,
				"playerName":  req.CharacterName,
				"playerLevel": req.PlayerLevel,
				"memory":      memory,
				"question":    req.Question,
				"personality": personalityText,
				"goals":       goalsText,
				"secrets":     secretsText,
				"relations":   relationsText,
				"quests":      questPrompt,
			},
			Parse:    parseNPCDialoguePayload,
			Validate: validateNPCDialoguePayloadWithSecrets(llmCfg, questIDs),
			Fallback: npcDialogueFallback,
		})
		if err != nil {
			return game.NPCDialogueResult{}, err
		}

		var result game.NPCDialogueResult
		if len(resp.JSON) == 0 {
			return game.NPCDialogueResult{}, fmt.Errorf("npc dialogue: empty payload")
		}
		if err := json.Unmarshal(resp.JSON, &result); err != nil {
			return game.NPCDialogueResult{}, fmt.Errorf("npc dialogue: decode payload: %w", err)
		}
		if strings.TrimSpace(result.Reply) == "" {
			return game.NPCDialogueResult{}, fmt.Errorf("npc dialogue: empty reply")
		}
		if !llmgen.OutputAllowed(llmgen.TaskNPCDialogue, result.Reply) {
			return game.NPCDialogueResult{}, fmt.Errorf("npc dialogue: reply rejected by content filter")
		}

		recordNPCMemoryTurn(facade, req, result.MemoryNotes)
		result.RelationDelta = applyNPCRelationDelta(facade, req, result.RelationDelta, result.End)
		triggerNPCMemorySummaryIfNeeded(rt, facade, req, memObj)
		return result, nil
	})
}

func npcLLMConfig(facade service.Facade, req game.NPCDialogueRequest) *npc.LLMConfig {
	if facade == nil {
		return nil
	}
	if n, err := facade.NPCsService().FindByID(req.NPCID); err == nil && n != nil && n.LLM != nil {
		if c := n.LLM.Clone(); c != nil {
			return c
		}
		return n.LLM
	}
	if strings.TrimSpace(req.NPCTemplateID) != "" && req.NPCTemplateID != req.NPCID {
		if n, err := facade.NPCsService().FindByID(req.NPCTemplateID); err == nil && n != nil && n.LLM != nil {
			if c := n.LLM.Clone(); c != nil {
				return c
			}
			return n.LLM
		}
		if templates := facade.NPCTemplatesService(); templates != nil {
			if tpl, err := templates.FindByID(req.NPCTemplateID); err == nil && tpl != nil && tpl.LLM != nil {
				return tpl.LLM.Clone()
			}
		}
	}
	return nil
}

func formatNPCPersonalityContext(cfg *npc.LLMConfig) (personality, goals, secrets, relations string) {
	if cfg == nil {
		return "", "", "", ""
	}
	if cfg.Personality != nil {
		p := cfg.Personality
		parts := make([]string, 0, 5)
		if p.Openness != 0 {
			parts = append(parts, "apertura:"+strconv.FormatFloat(p.Openness, 'f', 2, 64))
		}
		if p.Conscientiousness != 0 {
			parts = append(parts, "responsabilidad:"+strconv.FormatFloat(p.Conscientiousness, 'f', 2, 64))
		}
		if p.Extraversion != 0 {
			parts = append(parts, "extraversión:"+strconv.FormatFloat(p.Extraversion, 'f', 2, 64))
		}
		if p.Agreeableness != 0 {
			parts = append(parts, "amabilidad:"+strconv.FormatFloat(p.Agreeableness, 'f', 2, 64))
		}
		if p.Neuroticism != 0 {
			parts = append(parts, "neuroticismo:"+strconv.FormatFloat(p.Neuroticism, 'f', 2, 64))
		}
		if len(parts) > 0 {
			personality = strings.Join(parts, ", ")
		}
	}
	if len(cfg.Goals) > 0 {
		clean := make([]string, 0, len(cfg.Goals))
		for _, g := range cfg.Goals {
			if g = strings.TrimSpace(g); g != "" {
				clean = append(clean, g)
			}
		}
		if len(clean) > 0 {
			goals = strings.Join(clean, "; ")
		}
	}
	if len(cfg.Secrets) > 0 {
		// Secrets are trusted, internal context: the prompt tells the model to
		// use them for tone/hints but never reveal them literally. The reply
		// validator rejects any literal leak as a second line of defence.
		clean := make([]string, 0, len(cfg.Secrets))
		for _, s := range cfg.Secrets {
			if st := strings.Join(strings.Fields(s), " "); st != "" {
				clean = append(clean, st)
			}
		}
		if len(clean) > 0 {
			secrets = strings.Join(clean, "; ")
		}
	}
	if len(cfg.Relations) > 0 {
		parts := make([]string, 0, len(cfg.Relations))
		for _, r := range cfg.Relations {
			target := strings.TrimSpace(r.Target)
			if target == "" {
				continue
			}
			kind := strings.TrimSpace(r.Kind)
			if kind == "" {
				kind = "desconocido"
			}
			sent := r.Sentiment
			parts = append(parts, fmt.Sprintf("%s(%s,%.2f)", target, kind, sent))
		}
		if len(parts) > 0 {
			relations = strings.Join(parts, "; ")
		}
	}
	return
}

// loadNPCMemory renders the stored summary/facts for the prompt. The service
// already bounds the window; this cut intentionally uses the deterministic
// summary, not the future npc.memory_summary LLM summariser.
func loadNPCMemory(facade service.Facade, npcTemplateID, characterID string) string {
	if facade == nil || strings.TrimSpace(npcTemplateID) == "" || strings.TrimSpace(characterID) == "" {
		return ""
	}
	svc := facade.NPCMemoriesService()
	if svc == nil {
		return ""
	}
	mem, err := svc.Get(npcTemplateID, characterID)
	if err != nil || mem == nil {
		return ""
	}
	return formatNPCMemory(mem.Summary, mem.Facts, mem.LastUserLine)
}

func loadNPCMemoryFull(facade service.Facade, req game.NPCDialogueRequest) (*repository.NPCMemory, int) {
	if facade == nil {
		return nil, 0
	}
	svc := facade.NPCMemoriesService()
	if svc == nil {
		return nil, 0
	}
	npcID := strings.TrimSpace(req.NPCTemplateID)
	if npcID == "" {
		npcID = strings.TrimSpace(req.NPCID)
	}
	if npcID == "" || strings.TrimSpace(req.CharacterID) == "" {
		return nil, 0
	}
	mem, err := svc.Get(npcID, req.CharacterID)
	if err != nil || mem == nil {
		return nil, 0
	}
	eff := 0
	if n, err := facade.NPCsService().FindByID(npcID); err == nil && n != nil {
		eff = n.EffectiveMemoryTurns()
	}
	if eff == 0 && strings.TrimSpace(req.NPCID) != "" && req.NPCID != npcID {
		if n, err := facade.NPCsService().FindByID(req.NPCID); err == nil && n != nil {
			eff = n.EffectiveMemoryTurns()
		}
	}
	return mem, eff
}

func formatNPCMemory(summary string, facts []string, lastUserLine string) string {
	parts := make([]string, 0, 3)
	if s := strings.TrimSpace(summary); s != "" {
		parts = append(parts, s)
	}
	if len(facts) > 0 {
		clean := make([]string, 0, len(facts))
		for _, f := range facts {
			if f = strings.TrimSpace(f); f != "" {
				clean = append(clean, f)
			}
		}
		if len(clean) > 0 {
			parts = append(parts, "Hechos: "+strings.Join(clean, "; "))
		}
	}
	if last := strings.TrimSpace(lastUserLine); last != "" {
		parts = append(parts, "Última pregunta: "+last)
	}
	return strings.Join(parts, "\n")
}

func formatNPCMemoryLimited(mem *repository.NPCMemory, effectiveTurns int) string {
	if mem == nil {
		return ""
	}
	parts := make([]string, 0, 3)
	if s := strings.TrimSpace(mem.Summary); s != "" {
		parts = append(parts, s)
	}
	if len(mem.Facts) > 0 {
		facts := mem.Facts
		if effectiveTurns > 0 && len(facts) > effectiveTurns {
			facts = facts[len(facts)-effectiveTurns:]
		}
		clean := make([]string, 0, len(facts))
		for _, f := range facts {
			if f = strings.TrimSpace(f); f != "" {
				clean = append(clean, f)
			}
		}
		if len(clean) > 0 {
			parts = append(parts, "Hechos: "+strings.Join(clean, "; "))
		}
	}
	if last := strings.TrimSpace(mem.LastUserLine); last != "" {
		parts = append(parts, "Última pregunta: "+last)
	}
	return strings.Join(parts, "\n")
}

// mergeDynamicRelations appends the stored per-character sentiment to the
// static relationship context. It is prompt-only; the numeric value is never
// exposed as a game stat.
func mergeDynamicRelations(static string, mem *repository.NPCMemory) string {
	static = strings.TrimSpace(static)
	if mem == nil || len(mem.Relations) == 0 {
		return static
	}
	sentiment, ok := mem.Relations["player"]
	if !ok {
		return static
	}
	dynamic := fmt.Sprintf("player(dinámica,%.2f)", npc.ClampRelationSentiment(sentiment))
	if static == "" {
		return dynamic
	}
	// Replace a static "player" entry instead of listing two conflicting
	// sentiments for the same target.
	parts := strings.Split(static, ";")
	out := make([]string, 0, len(parts)+1)
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		target := part
		if i := strings.Index(target, "("); i >= 0 {
			target = target[:i]
		}
		if strings.EqualFold(strings.TrimSpace(target), "player") {
			continue
		}
		out = append(out, part)
	}
	out = append(out, dynamic)
	return strings.Join(out, "; ")
}

// applyNPCRelationDelta persists the bounded sentiment change after a
// successful reply. A zero model hint uses the conservative default +0.02; the
// service clamps the delta and the stored total so no single exchange can move
// the relationship by more than 0.05 or outside [-1,1].
func applyNPCRelationDelta(facade service.Facade, req game.NPCDialogueRequest, hinted float64, end bool) float64 {
	if facade == nil {
		return 0
	}
	svc := facade.NPCMemoriesService()
	if svc == nil {
		return 0
	}
	npcID := strings.TrimSpace(req.NPCTemplateID)
	if npcID == "" {
		npcID = strings.TrimSpace(req.NPCID)
	}
	if npcID == "" || strings.TrimSpace(req.CharacterID) == "" {
		return 0
	}
	delta := npc.ClampRelationDelta(hinted)
	if delta == 0 {
		if end {
			return 0
		}
		delta = 0.02
	}
	if _, err := svc.AdjustRelation(npcID, req.CharacterID, "player", delta); err != nil {
		log.WithError(err).WithFields(log.Fields{
			"npcID":       npcID,
			"characterID": req.CharacterID,
		}).Warn("Failed to adjust NPC relationship")
		return 0
	}
	return delta
}

func recordNPCMemoryTurn(facade service.Facade, req game.NPCDialogueRequest, notes []string) {
	if facade == nil {
		return
	}
	svc := facade.NPCMemoriesService()
	if svc == nil {
		return
	}
	npcID := strings.TrimSpace(req.NPCTemplateID)
	if npcID == "" {
		npcID = strings.TrimSpace(req.NPCID)
	}
	if npcID == "" || strings.TrimSpace(req.CharacterID) == "" {
		return
	}
	if _, err := svc.RecordTurn(npcID, req.CharacterID, req.Question, notes); err != nil {
		log.WithError(err).WithFields(log.Fields{
			"npcID":       npcID,
			"characterID": req.CharacterID,
		}).Warn("Failed to record NPC dialogue memory")
	}
}

func triggerNPCMemorySummaryIfNeeded(rt llmRuntime, facade service.Facade, req game.NPCDialogueRequest, memBefore *repository.NPCMemory) {
	if facade == nil || rt.pipeline == nil || !rt.pipeline.Enabled() {
		return
	}
	svc := facade.NPCMemoriesService()
	if svc == nil {
		return
	}
	npcID := strings.TrimSpace(req.NPCTemplateID)
	if npcID == "" {
		npcID = strings.TrimSpace(req.NPCID)
	}
	if npcID == "" || strings.TrimSpace(req.CharacterID) == "" {
		return
	}
	mem, err := svc.Get(npcID, req.CharacterID)
	if err != nil || mem == nil {
		return
	}
	threshold := svc.SummaryAtTurns()
	if threshold <= 0 {
		threshold = 8
	}
	if mem.Turns == 0 || mem.Turns%threshold != 0 {
		return
	}

	// Fire and forget; do not block tick
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()

		facts := strings.Join(mem.Facts, "; ")
		existing := mem.Summary
		persona := req.Persona
		resp, err := rt.pipeline.Generate(ctx, llmgen.Request{
			Task:       llmgen.TaskNPCMemorySummary,
			UserID:     req.UserID,
			RoomID:     req.RoomID,
			Timeout:    10 * time.Second,
			RequireLLM: false,
			LimitKey:   "npcmem:" + npcID + ":" + req.CharacterID,
			Vars: map[string]any{
				"npcName":         req.NPCName,
				"persona":         persona,
				"facts":           facts,
				"lastUserLine":    mem.LastUserLine,
				"existingSummary": existing,
			},
			Parse: func(raw string) (json.RawMessage, error) {
				p, err := llmgen.ExtractJSON(raw)
				if err != nil {
					return nil, err
				}
				var out struct {
					Summary string `json:"summary"`
				}
				if err := json.Unmarshal(p, &out); err != nil {
					return nil, err
				}
				out.Summary = strings.TrimSpace(out.Summary)
				if utf8.RuneCountInString(out.Summary) > 400 {
					out.Summary = strings.TrimSpace(string([]rune(out.Summary)[:400]))
				}
				return json.Marshal(out)
			},
			Validate: func(_ context.Context, _ llmgen.Request, payload json.RawMessage) []llmgen.Issue {
				var out struct {
					Summary string `json:"summary"`
				}
				if err := json.Unmarshal(payload, &out); err != nil {
					return []llmgen.Issue{{Severity: "error", Code: "invalid_json", Message: err.Error()}}
				}
				if utf8.RuneCountInString(strings.TrimSpace(out.Summary)) > 400 {
					return []llmgen.Issue{{Severity: "error", Code: "summary_too_long", Path: "summary", Message: "summary exceeds 400 runes"}}
				}
				return nil
			},
			Fallback: func(_ llmgen.Request, rendered llmprompt.Rendered) (llmgen.Response, error) {
				p, err := llmgen.ExtractJSON(rendered.Fallback)
				if err != nil {
					return llmgen.Response{}, err
				}
				return llmgen.Response{Task: llmgen.TaskNPCMemorySummary, Text: string(p), JSON: p, Source: llmgen.SourceFallback}, nil
			},
		})
		if err != nil {
			return
		}
		if len(resp.JSON) == 0 {
			return
		}
		var out struct {
			Summary string `json:"summary"`
		}
		if err := json.Unmarshal(resp.JSON, &out); err != nil {
			return
		}
		out.Summary = strings.TrimSpace(out.Summary)
		if out.Summary == "" {
			return // keep deterministic summary
		}
		if utf8.RuneCountInString(out.Summary) > 400 {
			out.Summary = strings.TrimSpace(string([]rune(out.Summary)[:400]))
		}
		// reload to avoid clobbering concurrent turns
		cur, err := svc.Get(npcID, req.CharacterID)
		if err != nil || cur == nil {
			return
		}
		cur.Summary = out.Summary
		cur.SummarySource = repository.NPCMemorySummaryLLM
		cur.Updated = time.Now()
		if _, err := svc.Upsert(cur); err != nil {
			log.WithError(err).WithFields(log.Fields{"npcID": npcID, "characterID": req.CharacterID}).Debug("Failed to save LLM summary")
		}
	}()
}

// parseNPCDialoguePayload extracts the JSON envelope and normalises the
// player-facing fields (reply cap, option kinds, memory note cap).
func parseNPCDialoguePayload(raw string) (json.RawMessage, error) {
	payload, err := llmgen.ExtractJSON(raw)
	if err != nil {
		return nil, err
	}
	var result game.NPCDialogueResult
	if err := json.Unmarshal(payload, &result); err != nil {
		return nil, err
	}
	result.Reply = strings.TrimSpace(result.Reply)
	if utf8.RuneCountInString(result.Reply) > 400 {
		result.Reply = strings.TrimSpace(string([]rune(result.Reply)[:400]))
	}
	if len(result.Options) > 4 {
		result.Options = result.Options[:4]
	}
	for i := range result.Options {
		result.Options[i].Label = strings.TrimSpace(result.Options[i].Label)
		result.Options[i].Kind = normalizeNPCDialogueOptionKind(result.Options[i].Kind)
	}
	if len(result.MemoryNotes) > 8 {
		result.MemoryNotes = result.MemoryNotes[:8]
	}
	notes := result.MemoryNotes[:0]
	for _, note := range result.MemoryNotes {
		note = strings.TrimSpace(note)
		if utf8.RuneCountInString(note) > 200 {
			note = strings.TrimSpace(string([]rune(note)[:200]))
		}
		if note != "" {
			notes = append(notes, note)
		}
	}
	result.MemoryNotes = notes
	result.RelationDelta = npc.ClampRelationDelta(result.RelationDelta)
	return json.Marshal(result)
}

func normalizeNPCDialogueOptionKind(kind string) string {
	kind = strings.TrimSpace(kind)
	lower := strings.ToLower(kind)
	if lower == "flavor" || lower == "end" {
		return lower
	}
	if len(kind) >= len("quest:") && strings.EqualFold(kind[:len("quest:")], "quest:") {
		return "quest:" + strings.TrimSpace(kind[len("quest:"):])
	}
	return kind
}

func validateNPCDialoguePayload(_ context.Context, _ llmgen.Request, payload json.RawMessage) []llmgen.Issue {
	return validateNPCDialoguePayloadWithAllowedQuests(payload, nil)
}

func validateNPCDialoguePayloadWithAllowedQuests(payload json.RawMessage, allowed map[string]string) []llmgen.Issue {
	var result game.NPCDialogueResult
	if err := json.Unmarshal(payload, &result); err != nil {
		return []llmgen.Issue{{Severity: "error", Code: "invalid_json", Message: err.Error()}}
	}
	var issues []llmgen.Issue
	if strings.TrimSpace(result.Reply) == "" {
		issues = append(issues, llmgen.Issue{Severity: "error", Code: "empty_reply", Message: "La respuesta del NPC está vacía."})
	}
	if utf8.RuneCountInString(result.Reply) > 400 {
		issues = append(issues, llmgen.Issue{Severity: "error", Code: "reply_too_long", Path: "reply", Message: "La respuesta supera 400 caracteres."})
	}
	if len(result.Options) > 4 {
		issues = append(issues, llmgen.Issue{Severity: "error", Code: "too_many_options", Path: "options", Message: "Se permiten como máximo 4 opciones."})
	}
	for i, opt := range result.Options {
		path := fmt.Sprintf("options[%d]", i)
		if strings.TrimSpace(opt.Label) == "" {
			issues = append(issues, llmgen.Issue{Severity: "error", Code: "empty_option", Path: path, Message: "La opción no tiene etiqueta."})
		}
		kind := normalizeNPCDialogueOptionKind(opt.Kind)
		if kind == "flavor" || kind == "end" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(kind), "quest:") {
			questID := strings.TrimSpace(kind[len("quest:"):])
			if questID == "" {
				issues = append(issues, llmgen.Issue{Severity: "error", Code: "bad_option_kind", Path: path, Message: "quest kind requiere un questID."})
			} else if _, ok := allowed[questID]; !ok {
				issues = append(issues, llmgen.Issue{Severity: "error", Code: "unknown_quest", Path: path, Message: "La quest no está disponible para este personaje."})
			}
			continue
		}
		issues = append(issues, llmgen.Issue{Severity: "error", Code: "bad_option_kind", Path: path, Message: "kind debe ser flavor, end o quest:<questID>."})
	}
	return issues
}

// validateNPCDialoguePayloadWithSecrets builds the validator for one NPC. In
// addition to the generic checks it rejects replies that reproduce a configured
// secret literally, backing the prompt instruction "no revelar literalmente".
func validateNPCDialoguePayloadWithSecrets(cfg *npc.LLMConfig, allowedQuestSets ...map[string]string) llmgen.ValidateFunc {
	var secrets []string
	if cfg != nil {
		secrets = cfg.Secrets
	}
	var allowed map[string]string
	if len(allowedQuestSets) > 0 {
		allowed = allowedQuestSets[0]
	}
	return func(ctx context.Context, req llmgen.Request, payload json.RawMessage) []llmgen.Issue {
		issues := validateNPCDialoguePayloadWithAllowedQuests(payload, allowed)
		if len(secrets) == 0 {
			return issues
		}
		var result game.NPCDialogueResult
		if err := json.Unmarshal(payload, &result); err != nil {
			return issues
		}
		if leaksSecret(result.Reply, secrets) {
			issues = append(issues, secretLeakIssue("reply", "La respuesta revela un secreto interno literalmente."))
		}
		// Options are rendered to the player and memory notes are replayed in
		// later prompts, so both are just as leak-sensitive as the reply.
		for i, opt := range result.Options {
			if leaksSecret(opt.Label, secrets) {
				issues = append(issues, secretLeakIssue(
					fmt.Sprintf("options[%d].label", i),
					"La opción revela un secreto interno literalmente.",
				))
			}
		}
		for i, note := range result.MemoryNotes {
			if leaksSecret(note, secrets) {
				issues = append(issues, secretLeakIssue(
					fmt.Sprintf("memoryNotes[%d]", i),
					"La nota de memoria revela un secreto interno literalmente.",
				))
			}
		}
		return issues
	}
}

func secretLeakIssue(path, message string) llmgen.Issue {
	return llmgen.Issue{
		Severity: "error",
		Code:     "secret_leak",
		Path:     path,
		Message:  message,
	}
}

// leaksSecret reports whether reply reproduces any secret verbatim (ignoring
// case, whitespace and punctuation). Paraphrases/hints are intentionally not
// rejected: the prompt allows insinuation, only literal disclosure is banned.
func leaksSecret(reply string, secrets []string) bool {
	haystack := normalizedDialogueText(reply)
	if haystack == "" {
		return false
	}
	for _, secret := range secrets {
		needle := normalizedDialogueText(secret)
		// Match on token boundaries so a short secret such as "si" does not
		// reject an innocent reply that merely contains it as a substring.
		if needle != "" && strings.Contains(" "+haystack+" ", " "+needle+" ") {
			return true
		}
	}
	return false
}

func normalizedDialogueText(s string) string {
	s = strings.ToLower(s)
	s = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' || r == '\n' || r == '\t' {
			return r
		}
		return ' '
	}, s)
	return strings.Join(strings.Fields(s), " ")
}

// npcDialogueFallback returns the YAML fallback (an in-character silence). It
// is only consulted when deterministic fallback is allowed; the live provider
// uses RequireLLM so failures surface to the game, which sends the neutral
// line instead.
func npcDialogueFallback(_ llmgen.Request, rendered llmprompt.Rendered) (llmgen.Response, error) {
	payload, err := llmgen.ExtractJSON(rendered.Fallback)
	if err != nil {
		return llmgen.Response{}, err
	}
	return llmgen.Response{
		Task:   llmgen.TaskNPCDialogue,
		Text:   string(payload),
		JSON:   payload,
		Source: llmgen.SourceFallback,
	}, nil
}

// bindQuestToSourceNPC re-points a generated quest at the NPC that offered it
// so turn-in and the authored quest list agree, but only when the mutation
// still validates against the live world; otherwise the generated quest is
// left untouched.
func bindQuestToSourceNPC(facade service.Facade, q *quests.Quest, npcID string) {
	if q == nil || facade == nil || strings.TrimSpace(npcID) == "" {
		return
	}
	if npc, err := facade.NPCsService().FindByID(npcID); err != nil || npc == nil {
		return
	}
	prevSource, prevTurnIn := q.Source, q.TurnIn
	q.Source = quests.QuestSource{Type: "npc", NPCID: npcID}
	if turnIn := strings.TrimSpace(q.TurnIn); turnIn == "" || strings.EqualFold(turnIn, "anywhere") {
		q.TurnIn = "npc:" + npcID
	}
	if len(facade.QuestsService().ValidateQuest(q)) > 0 {
		q.Source, q.TurnIn = prevSource, prevTurnIn
	}
}

// dmNarrationEnabled reports the LLM_DM_ENABLED kill switch (default on).
func dmNarrationEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("LLM_DM_ENABLED"))) {
	case "", "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
