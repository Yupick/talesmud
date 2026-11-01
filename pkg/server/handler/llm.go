package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/talesmud/talesmud/pkg/entities/quests"
	"github.com/talesmud/talesmud/pkg/service"
	"github.com/talesmud/talesmud/pkg/service/llmgen"
	"github.com/talesmud/talesmud/pkg/service/llmprompt"
)

// LLMHandler exposes the creator and admin LLM tools built on the shared
// generation pipeline.
type LLMHandler struct {
	Pipeline      *llmgen.Pipeline
	Registry      llmprompt.Registry
	QuestGen      *llmgen.QuestGenerator
	RoomsService  service.RoomsService
	QuestsService service.QuestsService
}

// GenerateQuestsRequest is the body for POST llm/quests/generate.
type GenerateQuestsRequest struct {
	Area        string `json:"area"`
	Level       int    `json:"level"`
	SourceNPCID string `json:"sourceNpcId"`
	Theme       string `json:"theme"`
	Commit      bool   `json:"commit"`
}

// GenerateQuests generates quest definitions. By default it is a dry run;
// set commit=true to persist the quests that pass validation.
func (h *LLMHandler) GenerateQuests(c *gin.Context) {
	if h.QuestGen == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "LLM quest generation is not configured"})
		return
	}
	var body GenerateQuestsRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if body.Commit && h.QuestsService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "quest storage is not available"})
		return
	}

	onCommit := func(qs []*quests.Quest) error {
		for _, q := range qs {
			if _, err := h.QuestsService.Store(q); err != nil {
				return err
			}
		}
		return nil
	}

	res, err := h.QuestGen.GenerateQuestsDetailed(c.Request.Context(), llmgen.GenerateQuestsRequest{
		Area:        body.Area,
		Level:       body.Level,
		SourceNPCID: body.SourceNPCID,
		Theme:       body.Theme,
		UserID:      llmUserID(c),
		DryRun:      !body.Commit,
	}, onCommit)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, llmgen.ErrLLMUnavailable) {
			status = http.StatusServiceUnavailable
		}
		if body.Commit {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	questsOut := res.Quests
	if questsOut == nil {
		questsOut = []*quests.Quest{}
	}
	issuesOut := res.Issues
	if issuesOut == nil {
		issuesOut = []llmgen.QuestIssue{}
	}
	c.JSON(http.StatusOK, gin.H{
		"quests":   questsOut,
		"issues":   issuesOut,
		"dryRun":   !body.Commit,
		"source":   res.Source,
		"attempts": res.Attempts,
		"cached":   res.Cached,
	})
}

// GenerateContextualQuestRequest is the body for POST llm/quests/contextual.
type GenerateContextualQuestRequest struct {
	PlayerLevel int    `json:"playerLevel"`
	ZoneID      string `json:"zoneId"`
	Theme       string `json:"theme"`
}

// GenerateContextualQuest generates a single contextual quest definition.
func (h *LLMHandler) GenerateContextualQuest(c *gin.Context) {
	if h.QuestGen == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "LLM quest generation is not configured"})
		return
	}
	var body GenerateContextualQuestRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	res, err := h.QuestGen.GenerateContextualQuest(c.Request.Context(), llmgen.QuestContext{
		PlayerLevel: body.PlayerLevel,
		ZoneID:      body.ZoneID,
		Theme:       body.Theme,
		UserID:      llmUserID(c),
	})
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, llmgen.ErrLLMUnavailable) {
			status = http.StatusServiceUnavailable
		} else if errors.Is(err, llmgen.ErrGeneration) {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	issuesOut := res.Issues
	if issuesOut == nil {
		issuesOut = []llmgen.QuestIssue{}
	}
	c.JSON(http.StatusOK, gin.H{
		"quest":    res.Quest,
		"issues":   issuesOut,
		"source":   res.Source,
		"attempts": res.Attempts,
		"cached":   res.Cached,
	})
}

// GenerateRoomDescriptionRequest is the optional body for the room endpoint.
type GenerateRoomDescriptionRequest struct {
	Theme string `json:"theme"`
}

// GenerateRoomDescription produces a preview description for a room. It does
// not persist anything; the Creator UI decides whether to save it.
func (h *LLMHandler) GenerateRoomDescription(c *gin.Context) {
	if h.Pipeline == nil || h.RoomsService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "LLM room description is not configured"})
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "room id is required"})
		return
	}
	room, err := h.RoomsService.FindByID(id)
	if err != nil || room == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "room not found"})
		return
	}
	var body GenerateRoomDescriptionRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	resp, err := h.Pipeline.Generate(c.Request.Context(), llmgen.Request{
		Task:   llmgen.TaskRoomDescription,
		UserID: llmUserID(c),
		Vars: map[string]any{
			"name":               room.Name,
			"area":               room.Area,
			"roomType":           room.RoomType,
			"currentDescription": room.Description,
			"theme":              body.Theme,
		},
		Parse: parseRoomDescription,
	})
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"roomId":      room.ID,
		"description": strings.TrimSpace(resp.Text),
		"current":     room.Description,
		"source":      resp.Source,
		"cached":      resp.Cached,
	})
}

func parseRoomDescription(raw string) (json.RawMessage, error) {
	desc := llmgen.CleanTextOutput(raw, 400)
	if desc == "" {
		return nil, errors.New("model returned an empty description")
	}
	return json.RawMessage(desc), nil
}

// ListPrompts returns every loaded template (embedded plus overrides).
func (h *LLMHandler) ListPrompts(c *gin.Context) {
	if h.Registry == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "prompt registry is not available"})
		return
	}
	templates := h.Registry.List()
	out := make([]gin.H, 0, len(templates))
	for _, t := range templates {
		out = append(out, promptJSON(t))
	}
	c.JSON(http.StatusOK, gin.H{"prompts": out})
}

// SavePrompt stores a YAML override for a template. Requires LLM_PROMPT_DIR.
func (h *LLMHandler) SavePrompt(c *gin.Context) {
	if h.Registry == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "prompt registry is not available"})
		return
	}
	savable, ok := h.Registry.(llmprompt.Savable)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "prompt overrides are not enabled (set LLM_PROMPT_DIR)"})
		return
	}
	var tpl llmprompt.Template
	if err := c.ShouldBindJSON(&tpl); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := savable.Save(&tpl); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	saved, err := h.Registry.Get(tpl.ID)
	if err != nil {
		saved = &tpl
	}
	c.JSON(http.StatusOK, gin.H{"prompt": promptJSON(saved)})
}

// TestPromptRequest is the body for POST llm/prompts/:id/test.
type TestPromptRequest struct {
	Variables map[string]any `json:"variables"`
	Execute   bool           `json:"execute"`
}

// TestPrompt renders a template and, optionally, runs it through the pipeline.
func (h *LLMHandler) TestPrompt(c *gin.Context) {
	if h.Registry == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "prompt registry is not available"})
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	tpl, err := h.Registry.Get(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	var body TestPromptRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}
	if body.Variables == nil {
		body.Variables = map[string]any{}
	}
	rendered, err := h.Registry.Render(id, body.Variables)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result := gin.H{
		"id":          tpl.ID,
		"version":     rendered.Version,
		"hash":        rendered.Hash,
		"system":      rendered.System,
		"user":        rendered.User,
		"temperature": rendered.Temperature,
		"maxTokens":   rendered.MaxTokens,
		"timeout":     rendered.Timeout.String(),
		"output":      rendered.Output,
	}

	if body.Execute && h.Pipeline != nil {
		resp, genErr := h.Pipeline.Generate(c.Request.Context(), llmgen.Request{
			Task:   llmgen.Task(tpl.ID),
			Vars:   body.Variables,
			UserID: llmUserID(c),
			Parse:  promptParse(tpl),
		})
		if genErr != nil {
			result["error"] = genErr.Error()
		} else {
			result["response"] = resp
		}
	}
	c.JSON(http.StatusOK, result)
}

// Status reports provider configuration and pipeline counters. Admin only.
func (h *LLMHandler) Status(c *gin.Context) {
	if h.Pipeline == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "LLM pipeline is not configured"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"enabled":  h.Pipeline.Enabled(),
		"provider": h.Pipeline.Provider(),
		"stats":    h.Pipeline.Stats(),
	})
}

// Stats returns just the counters. Admin only.
func (h *LLMHandler) Stats(c *gin.Context) {
	if h.Pipeline == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "LLM pipeline is not configured"})
		return
	}
	c.JSON(http.StatusOK, h.Pipeline.Stats())
}

func promptParse(tpl *llmprompt.Template) llmgen.ParseFunc {
	return func(raw string) (json.RawMessage, error) {
		if tpl != nil && tpl.Output == llmprompt.OutputJSON {
			return llmgen.ExtractJSON(raw)
		}
		text := llmgen.CleanTextOutput(raw, 0)
		if strings.TrimSpace(text) == "" {
			return nil, errors.New("model returned empty output")
		}
		return json.RawMessage(text), nil
	}
}

func promptJSON(t *llmprompt.Template) gin.H {
	if t == nil {
		return gin.H{}
	}
	return gin.H{
		"id":             t.ID,
		"version":        t.Version,
		"task":           t.Task,
		"output":         t.Output,
		"temperature":    t.Temperature,
		"maxTokens":      t.MaxTokens,
		"timeoutSeconds": t.TimeoutSeconds,
		"retry":          t.Retry,
		"variables":      t.Variables,
		"system":         t.System,
		"user":           t.User,
		"fallback":       t.Fallback,
		"hash":           t.Hash(),
	}
}

func llmUserID(c *gin.Context) string {
	actor := actorFrom(c)
	if actor.UserID != "" {
		return actor.UserID
	}
	return ""
}
