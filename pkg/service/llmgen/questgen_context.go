package llmgen

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/talesmud/talesmud/pkg/entities/quests"
)

// Level bounds accepted by contextual quest generation. MaxQuestLevel
// mirrors leveling.MaxLevel (the hard character cap) without importing the
// game layer into the generation pipeline.
const (
	MinQuestLevel = 1
	MaxQuestLevel = 50

	// questLevelTolerance is how far a generated quest level may drift from
	// the requesting player's level before the quest is considered incoherent.
	questLevelTolerance = 5
)

// QuestContext describes the in-game situation a dynamic quest is generated
// for: who is asking (level) and where (zone), plus optional hints.
type QuestContext struct {
	// PlayerLevel is the requesting character's level. Required, and must be
	// within [MinQuestLevel, MaxQuestLevel].
	PlayerLevel int
	// ZoneID is the area/zone the quest takes place in. Required.
	ZoneID string
	// Theme is an optional free-text hint (sanitized as untrusted input).
	Theme string
	// SourceNPCID optionally pins the NPC that offers the quest.
	SourceNPCID string
	// CharacterID tags the request for logs and per-character rate limiting.
	CharacterID string
	// UserID identifies the API user for rate limiting (optional).
	UserID string
	// RequireLLM disables the deterministic fallback: generation fails instead
	// of degrading to canned content.
	RequireLLM bool
}

// ContextualQuestResult is the outcome of one contextual generation.
type ContextualQuestResult struct {
	Quest    *quests.Quest `json:"quest,omitempty"`
	Issues   []QuestIssue  `json:"issues,omitempty"`
	Source   Source        `json:"source"`
	Attempts int           `json:"attempts"`
	Cached   bool          `json:"cached"`
}

// ValidateQuestStructure checks that a generated quest has the required
// shape before it may enter the world: a title, a description, at least one
// objective and a non-empty reward. When playerLevel is > 0 the quest level
// must also be coherent with it (within questLevelTolerance and never above
// MaxQuestLevel). It performs no world lookups; reference checks are the
// injected QuestValidator's job.
func ValidateQuestStructure(q *quests.Quest, playerLevel int) []QuestIssue {
	var issues []QuestIssue
	add := func(path, code, message string) {
		issues = append(issues, QuestIssue{Severity: "error", Path: path, Code: code, Message: message})
	}

	if q == nil {
		add("", "missing_quest", "No se generó ninguna quest.")
		return issues
	}

	if strings.TrimSpace(q.Name) == "" {
		add("name", "missing_name", "La quest necesita un título (name).")
	}
	if strings.TrimSpace(q.Description) == "" {
		add("description", "missing_description", "La quest necesita una descripción.")
	}
	if len(q.Objectives) == 0 {
		add("objectives", "missing_objectives", "La quest necesita al menos un objetivo.")
	}
	for i, obj := range q.Objectives {
		path := fmt.Sprintf("objectives[%d]", i)
		if strings.TrimSpace(obj.Description) == "" {
			add(path, "missing_objective_description", "Cada objetivo necesita una descripción.")
		}
		switch obj.Type {
		case quests.ObjectiveKill, quests.ObjectiveCollect, quests.ObjectiveDeliver,
			quests.ObjectiveVisit, quests.ObjectiveTalk, quests.ObjectiveCustom:
		default:
			add(path, "invalid_objective_type", fmt.Sprintf("Tipo de objetivo inválido: %q.", obj.Type))
		}
		if obj.Amount < 1 {
			add(path, "invalid_objective_amount", "El amount de un objetivo debe ser al menos 1.")
		}
	}

	if q.Rewards.XP <= 0 && q.Rewards.Gold <= 0 && len(q.Rewards.ItemTemplateIDs) == 0 {
		add("rewards", "missing_rewards", "La quest necesita una recompensa (xp, gold o ítems).")
	}

	switch {
	case q.Level < 0:
		add("level", "invalid_level", "El nivel de la quest no puede ser negativo.")
	case q.Level > MaxQuestLevel:
		add("level", "level_out_of_range", fmt.Sprintf(
			"El nivel de la quest (%d) supera el máximo permitido (%d).", q.Level, MaxQuestLevel))
	case playerLevel > 0 && q.Level > 0 && !levelWithinTolerance(int(q.Level), playerLevel):
		add("level", "level_out_of_range", fmt.Sprintf(
			"El nivel de la quest (%d) no es coherente con el jugador (nivel %d, tolerancia ±%d).",
			q.Level, playerLevel, questLevelTolerance))
	}

	return issues
}

func levelWithinTolerance(questLevel, playerLevel int) bool {
	diff := questLevel - playerLevel
	return diff >= -questLevelTolerance && diff <= questLevelTolerance
}

// validateRewardBudget checks that quest rewards stay inside the level-scaled
// budgets advertised to the model.
func validateRewardBudget(q *quests.Quest, maxXP, maxGold int) []QuestIssue {
	var issues []QuestIssue
	if q == nil {
		return issues
	}
	if maxXP > 0 && int(q.Rewards.XP) > maxXP {
		issues = append(issues, QuestIssue{
			Severity: "error",
			Path:     "rewards.xp",
			Code:     "xp_over_budget",
			Message:  fmt.Sprintf("La recompensa de XP (%d) supera el presupuesto del nivel (%d).", q.Rewards.XP, maxXP),
		})
	}
	if maxGold > 0 && q.Rewards.Gold > int64(maxGold) {
		issues = append(issues, QuestIssue{
			Severity: "error",
			Path:     "rewards.gold",
			Code:     "gold_over_budget",
			Message:  fmt.Sprintf("La recompensa de oro (%d) supera el presupuesto del nivel (%d).", q.Rewards.Gold, maxGold),
		})
	}
	return issues
}

// contextualBudgets scales the advertised reward caps with the player level,
// never exceeding the world catalog caps (with hard defaults when unset).
func contextualBudgets(cat QuestCatalog, playerLevel int) (maxXP, maxGold int) {
	maxXP, maxGold = cat.MaxXP, cat.MaxGold
	if maxXP <= 0 {
		maxXP = 500
	}
	if maxGold <= 0 {
		maxGold = 200
	}
	if xp := 50 + 25*playerLevel; xp < maxXP {
		maxXP = xp
	}
	if gold := 10 + 5*playerLevel; gold < maxGold {
		maxGold = gold
	}
	return maxXP, maxGold
}

// GenerateContextualQuest creates one quest tailored to the player's level
// and the zone they are in. It fills the quest.generate template variables
// from QuestContext plus the live world catalog, runs the shared pipeline
// (LLM, or the deterministic fallback when no provider is configured) and
// structurally validates the result before returning it. Nothing is
// persisted: the caller decides whether to store the quest.
func (g *QuestGenerator) GenerateContextualQuest(ctx context.Context, qc QuestContext) (ContextualQuestResult, error) {
	if g == nil || g.pipe == nil {
		return ContextualQuestResult{}, fmt.Errorf("%w: quest generator is not configured", ErrGeneration)
	}
	if err := validateQuestContext(qc); err != nil {
		return ContextualQuestResult{}, err
	}

	var cat QuestCatalog
	if g.catalog != nil {
		cat = g.catalog()
	}
	maxXP, maxGold := contextualBudgets(cat, qc.PlayerLevel)

	vars := map[string]any{
		"area":          strings.TrimSpace(qc.ZoneID),
		"level":         qc.PlayerLevel,
		"sourceNpcId":   strings.TrimSpace(qc.SourceNPCID),
		"catalog":       BuildCatalog(cat),
		"maxXP":         maxXP,
		"maxGold":       maxGold,
		"theme":         strings.TrimSpace(qc.Theme),
		"_catalogNPCs":  cat.NPCs,
		"_catalogRooms": cat.Rooms,
		"_catalogItems": cat.Items,
	}

	var limitKey string
	if id := strings.TrimSpace(qc.CharacterID); id != "" {
		limitKey = "quest:ctx:" + id
	}

	resp, err := g.pipe.Generate(ctx, Request{
		Task:       TaskQuestGenerate,
		Vars:       vars,
		UserID:     qc.UserID,
		LimitKey:   limitKey,
		RequireLLM: qc.RequireLLM,
		Parse:      parseQuestEnvelope,
		Validate:   g.contextualValidator(qc, maxXP, maxGold),
		Fallback:   g.deterministicFallback(GenerateQuestsRequest{Area: qc.ZoneID, Level: qc.PlayerLevel}),
	})
	result := ContextualQuestResult{Source: resp.Source, Attempts: resp.Attempts, Cached: resp.Cached}
	if err != nil {
		return result, err
	}

	generated, decodeErr := decodeQuestEnvelope(resp.JSON)
	if decodeErr != nil {
		return result, fmt.Errorf("%w: %v", ErrGeneration, decodeErr)
	}
	if len(generated) == 0 {
		return result, fmt.Errorf("%w: no se generó ninguna quest", ErrGeneration)
	}

	q := generated[0]
	applyQuestContext(q, qc)

	// Re-validate the final quest (defense in depth): LLM output already went
	// through the pipeline validator, but the deterministic fallback does not,
	// and contextual fields may have been filled in above.
	issues := ValidateQuestStructure(q, qc.PlayerLevel)
	issues = append(issues, validateRewardBudget(q, maxXP, maxGold)...)
	if g.validate != nil {
		issues = append(issues, g.validate(q)...)
	}
	result.Issues = issues
	if errs := errorQuestIssues(issues); len(errs) > 0 {
		return result, fmt.Errorf("%w: %s", ErrGeneration, joinQuestIssues(errs))
	}
	result.Quest = q
	return result, nil
}

// validateQuestContext rejects requests the pipeline cannot honor.
func validateQuestContext(qc QuestContext) error {
	if strings.TrimSpace(qc.ZoneID) == "" {
		return fmt.Errorf("llmgen: quest context requires a zone id")
	}
	if qc.PlayerLevel < MinQuestLevel || qc.PlayerLevel > MaxQuestLevel {
		return fmt.Errorf("llmgen: player level %d out of range [%d, %d]", qc.PlayerLevel, MinQuestLevel, MaxQuestLevel)
	}
	return nil
}

// applyQuestContext fills the contextual fields the model may have omitted.
func applyQuestContext(q *quests.Quest, qc QuestContext) {
	if q == nil {
		return
	}
	if strings.TrimSpace(q.Area) == "" {
		q.Area = strings.TrimSpace(qc.ZoneID)
	}
	if q.Level <= 0 {
		q.Level = int32(qc.PlayerLevel)
	}
}

// contextualValidator checks the envelope mid-pipeline so the repair loop can
// steer the model: exactly one structurally valid quest, rewards within the
// level budget and consistent with the live world.
func (g *QuestGenerator) contextualValidator(qc QuestContext, maxXP, maxGold int) ValidateFunc {
	return func(_ context.Context, _ Request, payload json.RawMessage) []Issue {
		generated, err := decodeQuestEnvelope(payload)
		if err != nil {
			return []Issue{{Severity: "error", Code: "invalid_json", Message: err.Error()}}
		}
		if len(generated) == 0 {
			return []Issue{{Severity: "error", Code: "no_quests", Message: "No se generó ninguna quest."}}
		}
		if len(generated) > 1 {
			return []Issue{{
				Severity: "error",
				Code:     "too_many_quests",
				Message:  "La generación contextual debe devolver exactamente una quest.",
			}}
		}

		q := generated[0]
		var issues []Issue
		for _, qi := range ValidateQuestStructure(q, qc.PlayerLevel) {
			issues = append(issues, toIssue(qi))
		}
		for _, qi := range validateRewardBudget(q, maxXP, maxGold) {
			issues = append(issues, toIssue(qi))
		}
		if g.validate != nil {
			for _, qi := range g.validate(q) {
				issues = append(issues, toIssue(qi))
			}
		}
		return issues
	}
}

func toIssue(qi QuestIssue) Issue {
	return Issue{
		Severity: firstNonEmpty(qi.Severity, "error"),
		Path:     qi.Path,
		Code:     qi.Code,
		Message:  qi.Message,
	}
}

func errorQuestIssues(issues []QuestIssue) []QuestIssue {
	errs := make([]QuestIssue, 0, len(issues))
	for _, i := range issues {
		if i.Severity == "error" {
			errs = append(errs, i)
		}
	}
	return errs
}

func joinQuestIssues(issues []QuestIssue) string {
	parts := make([]string, 0, len(issues))
	for _, i := range issues {
		if i.Path != "" {
			parts = append(parts, i.Path+": "+i.Message)
		} else {
			parts = append(parts, i.Message)
		}
	}
	return strings.Join(parts, "; ")
}
