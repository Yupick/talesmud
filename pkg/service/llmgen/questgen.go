package llmgen

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/talesmud/talesmud/pkg/entities"
	"github.com/talesmud/talesmud/pkg/entities/quests"
	"github.com/talesmud/talesmud/pkg/service/llmprompt"
)

// maxGeneratedQuests caps one generation batch.
const maxGeneratedQuests = 3

// QuestIssue describes a validation problem in a generated quest. It mirrors
// the shape used by the REST authoring validator.
type QuestIssue struct {
	Severity string `json:"severity"`
	Path     string `json:"path,omitempty"`
	Code     string `json:"code,omitempty"`
	Message  string `json:"message"`
}

// QuestValidator validates a quest against the live world (definition and
// references). It is injected by the server wiring.
type QuestValidator func(q *quests.Quest) []QuestIssue

// QuestGenerator orchestrates LLM/deterministic quest creation on top of a
// Pipeline.
type QuestGenerator struct {
	pipe     Generator
	catalog  func() QuestCatalog
	validate QuestValidator
}

// NewQuestGenerator wires the generator. catalog and validate are required.
func NewQuestGenerator(pipe Generator, catalog func() QuestCatalog, validate QuestValidator) *QuestGenerator {
	return &QuestGenerator{pipe: pipe, catalog: catalog, validate: validate}
}

// GenerateQuestsRequest parameterizes one batch.
type GenerateQuestsRequest struct {
	Area        string `json:"area"`
	Level       int    `json:"level"`
	SourceNPCID string `json:"sourceNpcId"`
	Theme       string `json:"theme"`
	UserID      string `json:"-"`
	DryRun      bool   `json:"dryRun"`
	RequireLLM  bool   `json:"-"`
}

// DetailedResult is the outcome of a batch, valid or not.
type DetailedResult struct {
	Quests   []*quests.Quest `json:"quests"`
	Issues   []QuestIssue    `json:"issues,omitempty"`
	Source   Source          `json:"source"`
	Attempts int             `json:"attempts"`
	Cached   bool            `json:"cached"`
}

// GenerateQuestsDetailed runs the pipeline and returns generated quests even
// when some failed validation. When onCommit is non-nil and DryRun is false,
// only fully valid quests are committed through it.
func (g *QuestGenerator) GenerateQuestsDetailed(ctx context.Context, req GenerateQuestsRequest, onCommit func([]*quests.Quest) error) (DetailedResult, error) {
	if g == nil || g.pipe == nil {
		return DetailedResult{}, fmt.Errorf("%w: quest generator is not configured", ErrGeneration)
	}
	var cat QuestCatalog
	if g.catalog != nil {
		cat = g.catalog()
	}

	vars := map[string]any{
		"area":          strings.TrimSpace(req.Area),
		"level":         req.Level,
		"sourceNpcId":   strings.TrimSpace(req.SourceNPCID),
		"catalog":       BuildCatalog(cat),
		"maxXP":         cat.MaxXP,
		"maxGold":       cat.MaxGold,
		"theme":         strings.TrimSpace(req.Theme),
		"_catalogNPCs":  cat.NPCs,
		"_catalogRooms": cat.Rooms,
		"_catalogItems": cat.Items,
	}

	resp, err := g.pipe.Generate(ctx, Request{
		Task:       TaskQuestGenerate,
		Vars:       vars,
		UserID:     req.UserID,
		RequireLLM: req.RequireLLM,
		Parse:      parseQuestEnvelope,
		Validate:   g.validateEnvelope,
		Fallback:   g.deterministicFallback(req),
	})
	if err != nil {
		return DetailedResult{}, err
	}

	generated, decodeErr := decodeQuestEnvelope(resp.JSON)
	result := DetailedResult{Source: resp.Source, Attempts: resp.Attempts, Cached: resp.Cached}
	if decodeErr != nil {
		return result, fmt.Errorf("%w: %v", ErrGeneration, decodeErr)
	}

	var accepted []*quests.Quest
	for i, q := range generated {
		if q == nil {
			continue
		}
		var issues []QuestIssue
		if g.validate != nil {
			issues = g.validate(q)
		}
		for j := range issues {
			if issues[j].Path == "" {
				issues[j].Path = fmt.Sprintf("quests[%d]", i)
			}
		}
		result.Issues = append(result.Issues, issues...)
		result.Quests = append(result.Quests, q)
		if len(issues) == 0 {
			accepted = append(accepted, q)
		}
	}

	if !req.DryRun && onCommit != nil && len(accepted) > 0 {
		if err := onCommit(accepted); err != nil {
			return result, err
		}
	}
	return result, nil
}

// GenerateQuests is the convenience path returning just the generated quests.
func (g *QuestGenerator) GenerateQuests(ctx context.Context, req GenerateQuestsRequest) ([]*quests.Quest, error) {
	res, err := g.GenerateQuestsDetailed(ctx, req, nil)
	if err != nil {
		return nil, err
	}
	return res.Quests, nil
}

func (g *QuestGenerator) validateEnvelope(_ context.Context, _ Request, payload json.RawMessage) []Issue {
	generated, err := decodeQuestEnvelope(payload)
	if err != nil {
		return []Issue{{Severity: "error", Code: "invalid_json", Message: err.Error()}}
	}
	if len(generated) == 0 {
		return []Issue{{Severity: "error", Code: "no_quests", Message: "No se generó ninguna quest."}}
	}
	if len(generated) > maxGeneratedQuests {
		return []Issue{{Severity: "error", Code: "too_many_quests", Message: "Se generaron demasiadas quests."}}
	}
	if g.validate == nil {
		return nil
	}
	var issues []Issue
	for i, q := range generated {
		for _, qi := range g.validate(q) {
			path := qi.Path
			if path == "" {
				path = fmt.Sprintf("quests[%d]", i)
			}
			issues = append(issues, Issue{
				Severity: firstNonEmpty(qi.Severity, "error"),
				Path:     path,
				Code:     qi.Code,
				Message:  qi.Message,
			})
		}
	}
	return issues
}

func (g *QuestGenerator) deterministicFallback(req GenerateQuestsRequest) FallbackFunc {
	return func(r Request, rendered llmprompt.Rendered) (Response, error) {
		payload, err := deterministicQuestJSON(r, rendered)
		if err != nil {
			return Response{}, err
		}
		return Response{
			Task:   TaskQuestGenerate,
			Text:   string(payload),
			JSON:   payload,
			Source: SourceDeterministic,
		}, nil
	}
}

func parseQuestEnvelope(raw string) (json.RawMessage, error) {
	payload, err := ExtractJSON(raw)
	if err != nil {
		return nil, err
	}
	generated, err := decodeQuestEnvelope(payload)
	if err != nil {
		return nil, err
	}
	envelope := struct {
		Quests []*quests.Quest `json:"quests"`
	}{Quests: generated}
	return json.Marshal(envelope)
}

func decodeQuestEnvelope(payload json.RawMessage) ([]*quests.Quest, error) {
	if len(payload) == 0 {
		return nil, fmt.Errorf("empty quest payload")
	}
	trimmed := strings.TrimSpace(string(payload))
	var envelope struct {
		Quests []*quests.Quest `json:"quests"`
	}
	if strings.HasPrefix(trimmed, "[") {
		if err := json.Unmarshal([]byte(trimmed), &envelope.Quests); err != nil {
			return nil, err
		}
	} else {
		if err := json.Unmarshal([]byte(trimmed), &envelope); err != nil {
			return nil, err
		}
	}
	if len(envelope.Quests) > maxGeneratedQuests {
		envelope.Quests = envelope.Quests[:maxGeneratedQuests]
	}
	for i := range envelope.Quests {
		normalizeQuest(envelope.Quests[i])
	}
	return envelope.Quests, nil
}

func normalizeQuest(q *quests.Quest) {
	if q == nil {
		return
	}
	if q.Entity == nil {
		q.Entity = entities.NewEntity()
	} else if strings.TrimSpace(q.ID) == "" {
		q.ID = entities.NewEntity().ID
	}
	if strings.TrimSpace(q.Category) == "" {
		q.Category = "side"
	}
	if strings.TrimSpace(q.Source.Type) == "" {
		q.Source.Type = "auto"
	}
	for i := range q.Objectives {
		if strings.TrimSpace(q.Objectives[i].ID) == "" {
			q.Objectives[i].ID = fmt.Sprintf("obj%d", i+1)
		}
		if q.Objectives[i].Order == 0 {
			q.Objectives[i].Order = int32(i + 1)
		}
		if q.Objectives[i].Type != quests.ObjectiveCustom && q.Objectives[i].Amount < 1 {
			q.Objectives[i].Amount = 1
		}
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
