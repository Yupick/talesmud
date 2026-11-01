package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/talesmud/talesmud/pkg/service"
)

// DMsHandler exposes the private messaging HTTP API. Every call takes the
// acting character and verifies ownership before the service validates channel
// participation.
type DMsHandler struct {
	Service    service.DMService
	Characters service.CharactersService
}

type dmChannelRequest struct {
	CharacterID       string `json:"characterId"`
	TargetCharacterID string `json:"targetCharacterId"`
}

type dmSendRequest struct {
	CharacterID string `json:"characterId"`
	Body        string `json:"body"`
}

type dmActorRequest struct {
	CharacterID string `json:"characterId"`
}

// GetChannels handles GET /api/dm/channels.
func (h *DMsHandler) GetChannels(c *gin.Context) {
	characterID := characterIDFromRequest(c)
	if _, ok := resolveOwnedCharacter(c, h.Characters, characterID); !ok {
		return
	}
	channels, err := h.Service.GetChannels(characterID)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, channels)
}

// CreateChannel handles POST /api/dm/channels (find-or-create for a pair).
func (h *DMsHandler) CreateChannel(c *gin.Context) {
	var req dmChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if _, ok := resolveOwnedCharacter(c, h.Characters, req.CharacterID); !ok {
		return
	}
	if !resolveExistingCharacter(c, h.Characters, req.TargetCharacterID) {
		return
	}
	channel, err := h.Service.FindOrCreateChannel(req.CharacterID, req.TargetCharacterID)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, channel)
}

// GetMessages handles GET /api/dm/channels/:channelId/messages.
func (h *DMsHandler) GetMessages(c *gin.Context) {
	channelID := c.Param("channelId")
	characterID := characterIDFromRequest(c)
	if _, ok := resolveOwnedCharacter(c, h.Characters, characterID); !ok {
		return
	}

	limit := 50
	if raw := c.Query("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be a positive integer"})
			return
		}
		limit = value
	}

	var since *time.Time
	if raw := c.Query("since"); raw != "" {
		value, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "since must be an RFC3339 timestamp"})
			return
		}
		since = &value
	}

	messages, err := h.Service.GetMessages(channelID, characterID, limit, since)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, messages)
}

// SendMessage handles POST /api/dm/channels/:channelId/messages.
func (h *DMsHandler) SendMessage(c *gin.Context) {
	var req dmSendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if _, ok := resolveOwnedCharacter(c, h.Characters, req.CharacterID); !ok {
		return
	}
	message, err := h.Service.SendMessage(c.Param("channelId"), req.CharacterID, req.Body)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, message)
}

// MarkRead handles POST /api/dm/channels/:channelId/read.
func (h *DMsHandler) MarkRead(c *gin.Context) {
	var req dmActorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if _, ok := resolveOwnedCharacter(c, h.Characters, req.CharacterID); !ok {
		return
	}
	if err := h.Service.MarkRead(c.Param("channelId"), req.CharacterID); err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "read"})
}
