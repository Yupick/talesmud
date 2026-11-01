package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/talesmud/talesmud/pkg/entities/characters"
	"github.com/talesmud/talesmud/pkg/service"
)

// ClansHandler exposes the clan HTTP API. Every mutating call takes the acting
// character in the request body and verifies that the authenticated user owns
// that character before the service is invoked.
type ClansHandler struct {
	Service    service.ClansService
	Characters service.CharactersService
}

type clanCreateRequest struct {
	CharacterID string `json:"characterId"`
	Name        string `json:"name"`
	Tag         string `json:"tag"`
	Description string `json:"description"`
}

type clanActorRequest struct {
	CharacterID string `json:"characterId"`
}

type clanInviteRequest struct {
	CharacterID       string `json:"characterId"`
	TargetCharacterID string `json:"targetCharacterId"`
}

type clanRankRequest struct {
	CharacterID       string `json:"characterId"`
	TargetCharacterID string `json:"targetCharacterId"`
	Rank              string `json:"rank"`
}

type clanBankRequest struct {
	CharacterID string `json:"characterId"`
	Amount      int64  `json:"amount"`
}

// characterIDFromRequest reads the acting character from the query string
// first (GET endpoints) and falls back to a JSON body (POST endpoints).
func characterIDFromRequest(c *gin.Context) string {
	if id := strings.TrimSpace(c.Query("characterId")); id != "" {
		return id
	}
	var body clanActorRequest
	if err := c.ShouldBindJSON(&body); err == nil {
		return strings.TrimSpace(body.CharacterID)
	}
	return ""
}

// resolveOwnedCharacter loads a character and enforces that the authenticated
// user owns it (admins may access any character). It writes the error response
// and returns ok=false on failure.
func resolveOwnedCharacter(c *gin.Context, characters service.CharactersService, characterID string) (*characters.Character, bool) {
	if strings.TrimSpace(characterID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "characterId is required"})
		return nil, false
	}
	if characters == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "character service unavailable"})
		return nil, false
	}
	character, err := characters.FindByID(characterID)
	if err != nil || character == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "character not found"})
		return nil, false
	}
	if !canAccessCharacter(c, character) {
		c.JSON(http.StatusForbidden, gin.H{"error": "character access denied"})
		return nil, false
	}
	return character, true
}

// resolveExistingCharacter loads a character referenced by another character
// (invite target, DM peer) without requiring ownership.
func resolveExistingCharacter(c *gin.Context, characters service.CharactersService, characterID string) bool {
	if strings.TrimSpace(characterID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "targetCharacterId is required"})
		return false
	}
	if characters == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "character service unavailable"})
		return false
	}
	character, err := characters.FindByID(characterID)
	if err != nil || character == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "target character not found"})
		return false
	}
	return true
}

// writeServiceError maps service-layer errors onto stable HTTP status codes.
func writeServiceError(c *gin.Context, err error) {
	if err == nil {
		return
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "not found"):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case strings.Contains(msg, "already"), strings.Contains(msg, "pending invite"):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case strings.Contains(msg, "insufficient rank"),
		strings.Contains(msg, "only the leader"),
		strings.Contains(msg, "only leader"),
		strings.Contains(msg, "not a member"),
		strings.Contains(msg, "not authorized"),
		strings.Contains(msg, "cannot kick"),
		strings.Contains(msg, "not a participant"),
		strings.Contains(msg, "access denied"):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
}

// Create handles POST /api/clans.
func (h *ClansHandler) Create(c *gin.Context) {
	var req clanCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if _, ok := resolveOwnedCharacter(c, h.Characters, req.CharacterID); !ok {
		return
	}

	cl, err := h.Service.CreateClan(req.CharacterID, req.Name, req.Tag, req.Description)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, cl)
}

// GetMine handles GET /api/clans/mine and returns the character's clan with its
// members plus any pending invites for that character.
func (h *ClansHandler) GetMine(c *gin.Context) {
	characterID := characterIDFromRequest(c)
	if _, ok := resolveOwnedCharacter(c, h.Characters, characterID); !ok {
		return
	}

	pending, err := h.Service.GetPendingInvitesForCharacter(characterID)
	if err != nil {
		writeServiceError(c, err)
		return
	}

	inClan, err := h.Service.IsCharacterInClan(characterID)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	if !inClan {
		c.JSON(http.StatusOK, gin.H{"clan": nil, "members": []interface{}{}, "invites": pending})
		return
	}

	cl, err := h.Service.GetClanForCharacter(characterID)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	members, err := h.Service.ListMembers(cl.ID)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"clan": cl, "members": members, "invites": pending})
}

// Invite handles POST /api/clans/:id/invite.
func (h *ClansHandler) Invite(c *gin.Context) {
	var req clanInviteRequest
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

	inv, err := h.Service.Invite(c.Param("id"), req.CharacterID, req.TargetCharacterID)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, inv)
}

// AcceptInvite handles POST /api/clans/invites/:inviteId/accept.
func (h *ClansHandler) AcceptInvite(c *gin.Context) {
	var req clanActorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if _, ok := resolveOwnedCharacter(c, h.Characters, req.CharacterID); !ok {
		return
	}
	if err := h.Service.AcceptInvite(c.Param("inviteId"), req.CharacterID); err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "accepted"})
}

// DeclineInvite handles POST /api/clans/invites/:inviteId/decline.
func (h *ClansHandler) DeclineInvite(c *gin.Context) {
	var req clanActorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if _, ok := resolveOwnedCharacter(c, h.Characters, req.CharacterID); !ok {
		return
	}
	if err := h.Service.DeclineInvite(c.Param("inviteId"), req.CharacterID); err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "declined"})
}

// Kick handles POST /api/clans/:id/kick.
func (h *ClansHandler) Kick(c *gin.Context) {
	var req clanInviteRequest
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
	if err := h.Service.Kick(c.Param("id"), req.CharacterID, req.TargetCharacterID); err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "kicked"})
}

// ChangeRank handles POST /api/clans/:id/rank.
func (h *ClansHandler) ChangeRank(c *gin.Context) {
	var req clanRankRequest
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

	rank, err := service.ParseClanRank(req.Rank)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.Service.ChangeRank(c.Param("id"), req.CharacterID, req.TargetCharacterID, rank); err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "rank updated"})
}

// Deposit handles POST /api/clans/:id/deposit.
func (h *ClansHandler) Deposit(c *gin.Context) {
	var req clanBankRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if _, ok := resolveOwnedCharacter(c, h.Characters, req.CharacterID); !ok {
		return
	}
	if err := h.Service.Deposit(c.Param("id"), req.CharacterID, req.Amount); err != nil {
		writeServiceError(c, err)
		return
	}
	balance, _ := h.Service.GetBankBalance(c.Param("id"))
	c.JSON(http.StatusOK, gin.H{"status": "deposited", "balance": balance})
}

// Withdraw handles POST /api/clans/:id/withdraw.
func (h *ClansHandler) Withdraw(c *gin.Context) {
	var req clanBankRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if _, ok := resolveOwnedCharacter(c, h.Characters, req.CharacterID); !ok {
		return
	}
	if err := h.Service.Withdraw(c.Param("id"), req.CharacterID, req.Amount); err != nil {
		writeServiceError(c, err)
		return
	}
	balance, _ := h.Service.GetBankBalance(c.Param("id"))
	c.JSON(http.StatusOK, gin.H{"status": "withdrawn", "balance": balance})
}

// Disband handles POST /api/clans/:id/disband.
func (h *ClansHandler) Disband(c *gin.Context) {
	var req clanActorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if _, ok := resolveOwnedCharacter(c, h.Characters, req.CharacterID); !ok {
		return
	}
	if err := h.Service.Disband(c.Param("id"), req.CharacterID); err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "disbanded"})
}
