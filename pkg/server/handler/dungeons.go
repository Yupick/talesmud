package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/talesmud/talesmud/pkg/entities/dungeons"
	"github.com/talesmud/talesmud/pkg/service"
)

// DungeonsHandler exposes the FASE 5.1 dungeon builder REST API. Reads are
// available to any authenticated user; writes are mounted behind
// CreatorMiddleware in server.go. The Definition payload is an opaque
// exportDefinition JSON blob owned by the frontend builder.
type DungeonsHandler struct {
	Service service.DungeonsService
}

// dungeonSummaryDTO is the list/create/update payload. Definition is omitted
// on purpose: it can be large and only the detail endpoint needs it.
type dungeonSummaryDTO struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	GridWidth   int       `json:"gridWidth"`
	GridHeight  int       `json:"gridHeight"`
	Author      string    `json:"author"`
	Tags        []string  `json:"tags,omitempty"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// dungeonDetailDTO extends the summary with the raw builder payload.
type dungeonDetailDTO struct {
	dungeonSummaryDTO
	Definition json.RawMessage `json:"definition"`
}

func dungeonSummary(def *dungeons.DungeonDefinition) dungeonSummaryDTO {
	summary := dungeonSummaryDTO{
		Name:        def.Name,
		Description: def.Description,
		GridWidth:   def.GridWidth,
		GridHeight:  def.GridHeight,
		Author:      def.Author,
		Tags:        def.Tags,
		UpdatedAt:   def.UpdatedAt,
	}
	if def.Entity != nil {
		summary.ID = def.ID
	}
	return summary
}

func dungeonDetail(def *dungeons.DungeonDefinition) dungeonDetailDTO {
	return dungeonDetailDTO{
		dungeonSummaryDTO: dungeonSummary(def),
		Definition:        def.Definition,
	}
}

// dungeonUpsertRequest is the shared body for POST /api/dungeons and
// PUT /api/dungeons/:id. Definition is the opaque exportDefinition JSON.
type dungeonUpsertRequest struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	GridWidth   int             `json:"gridWidth"`
	GridHeight  int             `json:"gridHeight"`
	Definition  json.RawMessage `json:"definition"`
	Author      string          `json:"author"`
	Tags        []string        `json:"tags"`
}

func (req *dungeonUpsertRequest) toDefinition() *dungeons.DungeonDefinition {
	return &dungeons.DungeonDefinition{
		Name:        strings.TrimSpace(req.Name),
		Description: req.Description,
		GridWidth:   req.GridWidth,
		GridHeight:  req.GridHeight,
		Definition:  req.Definition,
		Author:      strings.TrimSpace(req.Author),
		Tags:        req.Tags,
	}
}

// writeDungeonServiceError maps dungeon service errors onto stable HTTP
// status codes: validation problems are 400, unknown ids 404, anything
// else 500.
func writeDungeonServiceError(c *gin.Context, err error) {
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

// List handles GET /api/dungeons?name=&author=&limit=.
func (h *DungeonsHandler) List(c *gin.Context) {
	filter := service.DungeonListFilter{
		Name:   c.Query("name"),
		Author: c.Query("author"),
	}
	if raw := c.Query("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be a positive integer"})
			return
		}
		filter.Limit = limit
	}

	defs, err := h.Service.List(filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]dungeonSummaryDTO, 0, len(defs))
	for _, def := range defs {
		if def == nil {
			continue
		}
		out = append(out, dungeonSummary(def))
	}
	c.JSON(http.StatusOK, out)
}

// Get handles GET /api/dungeons/:id and returns the summary plus the raw
// definition payload.
func (h *DungeonsHandler) Get(c *gin.Context) {
	def, err := h.Service.FindByID(c.Param("id"))
	if err != nil || def == nil {
		if err != nil && !dungeonErrIsMissing(err) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "dungeon not found"})
		return
	}
	c.JSON(http.StatusOK, dungeonDetail(def))
}

// dungeonErrIsMissing reports whether err means "there is nothing under that
// id" (as opposed to a storage failure).
func dungeonErrIsMissing(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not found") || strings.Contains(msg, "required")
}

// Create handles POST /api/dungeons (creator-gated) and answers 201 with the
// created DTO.
func (h *DungeonsHandler) Create(c *gin.Context) {
	var req dungeonUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	created, err := h.Service.Create(req.toDefinition())
	if err != nil {
		writeDungeonServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, dungeonSummary(created))
}

// Update handles PUT /api/dungeons/:id (creator-gated) and answers 200 with
// the updated DTO, or 404 when the id is unknown.
func (h *DungeonsHandler) Update(c *gin.Context) {
	var req dungeonUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	def := req.toDefinition()
	if err := h.Service.Update(c.Param("id"), def); err != nil {
		writeDungeonServiceError(c, err)
		return
	}
	// repo.Update stamps UpdatedAt (and keeps CreatedAt) on def and sets the id.
	c.JSON(http.StatusOK, dungeonSummary(def))
}

// Delete handles DELETE /api/dungeons/:id (creator-gated) and answers 204, or
// 404 when the id is unknown.
func (h *DungeonsHandler) Delete(c *gin.Context) {
	if err := h.Service.Delete(c.Param("id")); err != nil {
		writeDungeonServiceError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
