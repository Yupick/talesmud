package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/talesmud/talesmud/pkg/entities"
	npc "github.com/talesmud/talesmud/pkg/entities/npcs"
	"github.com/talesmud/talesmud/pkg/service"
)

// NPCTemplatesHandler exposes the FASE 5.3 NPC/mob template REST API. Reads are
// available to any authenticated user; writes are mounted behind
// CreatorMiddleware in server.go. The loot/behavior/scaling payloads are opaque
// JSON blobs owned by the frontend authoring tools.
//
// It is intentionally separate from NPCsHandler: those routes operate on the
// runtime NPC entity store (npcs table) while this one owns the dedicated
// npc_templates blueprints.
type NPCTemplatesHandler struct {
	Service service.NPCTemplatesService
}

// npcTemplateSummaryDTO is the list payload. The heavy loot/behavior/scaling
// JSON is omitted on purpose and only returned by the detail endpoint.
type npcTemplateSummaryDTO struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Role         string    `json:"role"`
	CreatureType string    `json:"creatureType"`
	Level        int32     `json:"level"`
	MaxHp        int32     `json:"maxHp"`
	DamageMin    int32     `json:"damageMin"`
	DamageMax    int32     `json:"damageMax"`
	Aggressive   bool      `json:"aggressive"`
	XpReward     int32     `json:"xpReward"`
	GoldMin      int32     `json:"goldMin"`
	GoldMax      int32     `json:"goldMax"`
	SpawnRoomId  string    `json:"spawnRoomId"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// npcTemplateDetailDTO extends the summary with the authoring-only fields,
// including the raw loot/behavior/scaling payloads.
type npcTemplateDetailDTO struct {
	npcTemplateSummaryDTO
	Description    string          `json:"description"`
	CombatStyle    string          `json:"combatStyle"`
	Difficulty     string          `json:"difficulty"`
	AggroRadius    int32           `json:"aggroRadius"`
	LootTableID    string          `json:"lootTableId"`
	Loot           json.RawMessage `json:"loot"`
	SpawnCount     int32           `json:"spawnCount"`
	RespawnSeconds int32           `json:"respawnSeconds"`
	Behavior       json.RawMessage `json:"behavior"`
	Scaling        json.RawMessage `json:"scaling"`
	LLM            *npc.LLMConfig  `json:"llm,omitempty"`
	IsTemplate     bool            `json:"isTemplate"`
	CreatedAt      time.Time       `json:"createdAt"`
}

func npcTemplateSummary(tpl *npc.NPCTemplate) npcTemplateSummaryDTO {
	summary := npcTemplateSummaryDTO{
		Name:         tpl.Name,
		Role:         tpl.Role,
		CreatureType: tpl.CreatureType,
		Level:        tpl.Level,
		MaxHp:        tpl.MaxHp,
		DamageMin:    tpl.DamageMin,
		DamageMax:    tpl.DamageMax,
		Aggressive:   tpl.Aggressive,
		XpReward:     tpl.XpReward,
		GoldMin:      tpl.GoldMin,
		GoldMax:      tpl.GoldMax,
		SpawnRoomId:  tpl.SpawnRoomId,
		UpdatedAt:    tpl.UpdatedAt,
	}
	if tpl.Entity != nil {
		summary.ID = tpl.ID
	}
	return summary
}

func npcTemplateDetail(tpl *npc.NPCTemplate, includeSecrets bool) npcTemplateDetailDTO {
	dto := npcTemplateDetailDTO{
		npcTemplateSummaryDTO: npcTemplateSummary(tpl),
		Description:           tpl.Description,
		CombatStyle:           tpl.CombatStyle,
		Difficulty:            tpl.Difficulty,
		AggroRadius:           tpl.AggroRadius,
		LootTableID:           tpl.LootTableID,
		Loot:                  tpl.Loot,
		SpawnCount:            tpl.SpawnCount,
		RespawnSeconds:        tpl.RespawnSeconds,
		Behavior:              tpl.Behavior,
		Scaling:               tpl.Scaling,
		LLM:                   tpl.LLM.Clone(),
		IsTemplate:            tpl.IsTemplate,
		CreatedAt:             tpl.CreatedAt,
	}
	// Secrets are trusted, internal prompt context. They are only returned to
	// creators/admins (who already have the write routes); everyone else gets
	// the LLM config without them.
	if dto.LLM != nil && !includeSecrets {
		dto.LLM.Secrets = nil
	}
	return dto
}

// canViewNPCTemplateSecrets reports whether the authenticated caller may see
// the authored secrets. Missing or unexpected user contexts fail closed.
func canViewNPCTemplateSecrets(c *gin.Context) bool {
	usr, exists := c.Get("user")
	if !exists {
		return false
	}
	user, ok := usr.(*entities.User)
	return ok && user.IsCreator()
}

// npcTemplateUpsertRequest is the shared body for POST /api/npc-templates and
// PUT /api/npc-templates/:id.
type npcTemplateUpsertRequest struct {
	Name           string          `json:"name"`
	Description    string          `json:"description"`
	Role           string          `json:"role"`
	CreatureType   string          `json:"creatureType"`
	CombatStyle    string          `json:"combatStyle"`
	Difficulty     string          `json:"difficulty"`
	Level          int32           `json:"level"`
	MaxHp          int32           `json:"maxHp"`
	DamageMin      int32           `json:"damageMin"`
	DamageMax      int32           `json:"damageMax"`
	Aggressive     bool            `json:"aggressive"`
	AggroRadius    int32           `json:"aggroRadius"`
	XpReward       int32           `json:"xpReward"`
	GoldMin        int32           `json:"goldMin"`
	GoldMax        int32           `json:"goldMax"`
	LootTableID    string          `json:"lootTableId"`
	Loot           json.RawMessage `json:"loot"`
	SpawnRoomId    string          `json:"spawnRoomId"`
	SpawnCount     int32           `json:"spawnCount"`
	RespawnSeconds int32           `json:"respawnSeconds"`
	Behavior       json.RawMessage `json:"behavior"`
	Scaling        json.RawMessage `json:"scaling"`
	LLM            *npc.LLMConfig  `json:"llm,omitempty"`
}

func (req *npcTemplateUpsertRequest) toTemplate() *npc.NPCTemplate {
	return &npc.NPCTemplate{
		Name:           strings.TrimSpace(req.Name),
		Description:    req.Description,
		Role:           strings.TrimSpace(req.Role),
		CreatureType:   strings.TrimSpace(req.CreatureType),
		CombatStyle:    req.CombatStyle,
		Difficulty:     req.Difficulty,
		Level:          req.Level,
		MaxHp:          req.MaxHp,
		DamageMin:      req.DamageMin,
		DamageMax:      req.DamageMax,
		Aggressive:     req.Aggressive,
		AggroRadius:    req.AggroRadius,
		XpReward:       req.XpReward,
		GoldMin:        req.GoldMin,
		GoldMax:        req.GoldMax,
		LootTableID:    req.LootTableID,
		Loot:           req.Loot,
		SpawnRoomId:    req.SpawnRoomId,
		SpawnCount:     req.SpawnCount,
		RespawnSeconds: req.RespawnSeconds,
		Behavior:       req.Behavior,
		Scaling:        req.Scaling,
		LLM:            req.LLM.Clone(),
	}
}

// writeNPCTemplateServiceError maps NPC template service errors onto stable
// HTTP status codes: unknown ids are 404, validation problems 400, anything
// else 500.
func writeNPCTemplateServiceError(c *gin.Context, err error) {
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "not found"):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case strings.Contains(msg, "required"), strings.Contains(msg, "must be"):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

// List handles GET /api/npc-templates?name=&role=&limit=.
func (h *NPCTemplatesHandler) List(c *gin.Context) {
	filter := service.NPCTemplateListFilter{
		Name: c.Query("name"),
		Role: c.Query("role"),
	}
	if raw := c.Query("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be a positive integer"})
			return
		}
		filter.Limit = limit
	}

	templates, err := h.Service.List(filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]npcTemplateSummaryDTO, 0, len(templates))
	for _, tpl := range templates {
		if tpl == nil {
			continue
		}
		out = append(out, npcTemplateSummary(tpl))
	}
	c.JSON(http.StatusOK, out)
}

// Get handles GET /api/npc-templates/:id and returns the full DTO, 404 when
// the id is unknown.
func (h *NPCTemplatesHandler) Get(c *gin.Context) {
	tpl, err := h.Service.FindByID(c.Param("id"))
	if err != nil || tpl == nil {
		if err != nil && !npcTemplateErrIsMissing(err) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "npc template not found"})
		return
	}
	c.JSON(http.StatusOK, npcTemplateDetail(tpl, canViewNPCTemplateSecrets(c)))
}

// npcTemplateErrIsMissing reports whether err means "there is nothing under
// that id" (as opposed to a storage failure).
func npcTemplateErrIsMissing(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not found") || strings.Contains(msg, "required")
}

// Create handles POST /api/npc-templates (creator-gated) and answers 201 with
// the created detail DTO.
func (h *NPCTemplatesHandler) Create(c *gin.Context) {
	var req npcTemplateUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	created, err := h.Service.Create(req.toTemplate())
	if err != nil {
		writeNPCTemplateServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, npcTemplateDetail(created, true))
}

// Update handles PUT /api/npc-templates/:id (creator-gated) and answers 200
// with the updated detail DTO, or 404 when the id is unknown.
func (h *NPCTemplatesHandler) Update(c *gin.Context) {
	var req npcTemplateUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tpl := req.toTemplate()
	if err := h.Service.Update(c.Param("id"), tpl); err != nil {
		writeNPCTemplateServiceError(c, err)
		return
	}
	// repo.Update stamps UpdatedAt, keeps the id and service.Update preserved
	// CreatedAt on tpl.
	c.JSON(http.StatusOK, npcTemplateDetail(tpl, true))
}

// Delete handles DELETE /api/npc-templates/:id (creator-gated) and answers 204,
// or 404 when the id is unknown.
func (h *NPCTemplatesHandler) Delete(c *gin.Context) {
	if err := h.Service.Delete(c.Param("id")); err != nil {
		writeNPCTemplateServiceError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
