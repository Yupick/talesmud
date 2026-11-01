package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/talesmud/talesmud/pkg/entities"
	"github.com/talesmud/talesmud/pkg/service/llm"
	"github.com/talesmud/talesmud/pkg/service/llmgen"
	"github.com/talesmud/talesmud/pkg/service/llmprompt"
)

const (
	groqFieldMaxRunes     = 80
	groqBackstoryMaxRunes = 500
	groqRateLimitPerUser  = 10
	groqRateLimitWindow   = time.Hour
)

// GenerateHandler handles AI text generation requests.
type GenerateHandler struct {
	LLMClient llm.Client
	// Pipeline is the shared generation pipeline. When nil it is built
	// lazily from LLMClient and a registry (see resolve).
	Pipeline *llmgen.Pipeline
	// Registry is the prompt registry. When nil it is loaded lazily.
	Registry llmprompt.Registry
	limiter  *windowLimiter
}

func (h *GenerateHandler) rateLimiter() *windowLimiter {
	if h.limiter == nil {
		h.limiter = newWindowLimiter(groqRateLimitPerUser, groqRateLimitWindow)
	}
	return h.limiter
}

// resolve lazily builds the prompt registry and pipeline from the injected
// LLMClient. It is safe to call from concurrent requests because wiring
// happens before the server serves traffic.
func (h *GenerateHandler) resolve() *llmgen.Pipeline {
	if h.Registry == nil {
		reg, err := llmprompt.NewRegistry(strings.TrimSpace(os.Getenv("LLM_PROMPT_DIR")))
		if err != nil {
			log.WithError(err).Error("Failed to load LLM prompt registry")
			return nil
		}
		h.Registry = reg
	}
	if h.Pipeline == nil {
		h.Pipeline = llmgen.New(h.LLMClient, h.Registry, llmgen.ConfigFromEnv())
	}
	return h.Pipeline
}

// GenerateCharacterRequest is the JSON body for the generate endpoint.
type GenerateCharacterRequest struct {
	Generate     string `json:"generate" binding:"required"` // "name", "description", or "both"
	TemplateName string `json:"templateName"`
	Archetype    string `json:"archetype"`
	Race         string `json:"race"`
	Class        string `json:"class"`
	Description  string `json:"description"`
	Backstory    string `json:"backstory"`
	OriginArea   string `json:"originArea"`
	CurrentName  string `json:"currentName,omitempty"`
}

// GenerateCharacterResponse is the JSON response.
type GenerateCharacterResponse struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

// GenerateCharacter generates a character name and/or description using the
// configured LLM provider. When no provider is configured the pipeline
// returns deterministic content so the endpoint keeps working (HTTP 200).
func (h *GenerateHandler) GenerateCharacter(c *gin.Context) {
	usr, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	user, ok := usr.(*entities.User)
	if !ok || user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	if user.IsGuest {
		c.JSON(http.StatusForbidden, gin.H{"error": "AI generation is not available for guest sessions"})
		return
	}

	var req GenerateCharacterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := req.validateLengths(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req.Generate = strings.ToLower(req.Generate)
	if req.Generate != "name" && req.Generate != "description" && req.Generate != "both" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "generate must be 'name', 'description', or 'both'",
		})
		return
	}

	if !h.rateLimiter().Allow(user.ID) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
		return
	}

	pipeline := h.resolve()
	if pipeline == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AI generation is not configured"})
		return
	}

	resp := GenerateCharacterResponse{}

	if req.Generate == "name" || req.Generate == "both" {
		name, err := h.generateName(c, pipeline, user, &req)
		if err != nil {
			log.WithError(err).Error("Failed to generate character name")
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to generate name. Please try again.",
			})
			return
		}
		resp.Name = name
	}

	if req.Generate == "description" || req.Generate == "both" {
		nameForDesc := req.CurrentName
		if resp.Name != "" {
			nameForDesc = resp.Name
		}
		desc, err := h.generateDescription(c, pipeline, user, &req, nameForDesc)
		if err != nil {
			log.WithError(err).Error("Failed to generate character description")
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to generate description. Please try again.",
			})
			return
		}
		resp.Description = desc
	}

	c.JSON(http.StatusOK, resp)
}

func (h *GenerateHandler) generateName(c *gin.Context, pipeline *llmgen.Pipeline, user *entities.User, req *GenerateCharacterRequest) (string, error) {
	result, err := pipeline.Generate(c.Request.Context(), llmgen.Request{
		Task:   llmgen.TaskCharacterName,
		UserID: user.ID,
		Vars: map[string]any{
			"templateName": req.TemplateName,
			"archetype":    req.Archetype,
			"race":         req.Race,
			"class":        req.Class,
			"originArea":   req.OriginArea,
			"backstory":    req.Backstory,
		},
		Parse: parseCharacterName,
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(result.Text), nil
}

func (h *GenerateHandler) generateDescription(c *gin.Context, pipeline *llmgen.Pipeline, user *entities.User, req *GenerateCharacterRequest, characterName string) (string, error) {
	result, err := pipeline.Generate(c.Request.Context(), llmgen.Request{
		Task:   llmgen.TaskCharacterDescription,
		UserID: user.ID,
		Vars: map[string]any{
			"characterName": characterName,
			"templateName":  req.TemplateName,
			"archetype":     req.Archetype,
			"race":          req.Race,
			"class":         req.Class,
			"originArea":    req.OriginArea,
			"backstory":     req.Backstory,
		},
		Parse: parseCharacterDescription,
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(result.Text), nil
}

func parseCharacterName(raw string) (json.RawMessage, error) {
	name := llmgen.CleanTextOutput(raw, 40)
	if name == "" {
		return nil, errors.New("model returned an empty name")
	}
	return json.RawMessage(name), nil
}

func parseCharacterDescription(raw string) (json.RawMessage, error) {
	desc := llmgen.CleanTextOutput(raw, 200)
	if desc == "" {
		return nil, errors.New("model returned an empty description")
	}
	return json.RawMessage(desc), nil
}

func (req *GenerateCharacterRequest) validateLengths() error {
	fields := []struct {
		name  string
		value string
		max   int
	}{
		{"templateName", req.TemplateName, groqFieldMaxRunes},
		{"archetype", req.Archetype, groqFieldMaxRunes},
		{"race", req.Race, groqFieldMaxRunes},
		{"class", req.Class, groqFieldMaxRunes},
		{"description", req.Description, groqFieldMaxRunes},
		{"originArea", req.OriginArea, groqFieldMaxRunes},
		{"currentName", req.CurrentName, groqFieldMaxRunes},
		{"backstory", req.Backstory, groqBackstoryMaxRunes},
	}
	for _, field := range fields {
		if utf8.RuneCountInString(field.value) > field.max {
			return fmt.Errorf("%s exceeds maximum length of %d characters", field.name, field.max)
		}
	}
	return nil
}
