package server_test

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/talesmud/talesmud/pkg/db/sqlite"
	"github.com/talesmud/talesmud/pkg/entities"
	"github.com/talesmud/talesmud/pkg/repository"
	"github.com/talesmud/talesmud/pkg/server"
	"github.com/talesmud/talesmud/pkg/server/handler"
	"github.com/talesmud/talesmud/pkg/service"
)

// newNPCTemplatesHTTP mirrors the production mounting for /api/npc-templates*:
// reads on the authenticated group, writes behind CreatorMiddleware (server.go).
func newNPCTemplatesHTTP(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	client, err := sqlite.Open(filepath.Join(t.TempDir(), "npc-templates-http.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	facade := service.NewFacade(repository.NewSQLiteFactory(client), nil)
	h := &handler.NPCTemplatesHandler{Service: facade.NPCTemplatesService()}

	r := gin.New()
	api := r.Group("/api")
	api.Use(func(c *gin.Context) {
		role := c.GetHeader("X-Role")
		if role == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Set("user", &entities.User{
			Entity:   &entities.Entity{ID: "user-1"},
			Nickname: "Tester",
			Role:     role,
		})
		c.Next()
	})
	api.GET("npc-templates", h.List)
	api.GET("npc-templates/:id", h.Get)
	creator := api.Group("")
	creator.Use(server.CreatorMiddleware())
	creator.POST("npc-templates", h.Create)
	creator.PUT("npc-templates/:id", h.Update)
	creator.DELETE("npc-templates/:id", h.Delete)
	return r
}

func TestNPCTemplateHTTPCreatorGateAndLifecycle(t *testing.T) {
	r := newNPCTemplatesHTTP(t)
	body := gin.H{
		"name":           "Skeleton Warrior",
		"description":    "An animated bones soldier",
		"role":           "enemy",
		"creatureType":   "undead",
		"level":          3,
		"maxHp":          45,
		"damageMin":      4,
		"damageMax":      9,
		"goldMin":        5,
		"goldMax":        25,
		"spawnRoomId":    "room_crypt_1",
		"spawnCount":     3,
		"respawnSeconds": 60,
		"loot":           json.RawMessage(`[{"itemId":"item_bone","chance":80}]`),
		"behavior":       json.RawMessage(`{"stateMachine":[]}`),
		"scaling":        json.RawMessage(`{"targetLevel":null}`),
	}

	// Anonymous callers never reach the handlers.
	rec := doJSON(r, http.MethodGet, "/api/npc-templates", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous read: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(r, http.MethodPost, "/api/npc-templates", "", body)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous write: %d %s", rec.Code, rec.Body.String())
	}

	// Authenticated players may read but not write.
	rec = doJSON(r, http.MethodGet, "/api/npc-templates", entities.RolePlayer, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("player read: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(r, http.MethodGet, "/api/npc-templates/missing", entities.RolePlayer, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("player read missing: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(r, http.MethodPost, "/api/npc-templates", entities.RolePlayer, body)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("player write: %d %s", rec.Code, rec.Body.String())
	}

	// Creators own the full lifecycle.
	rec = doJSON(r, http.MethodPost, "/api/npc-templates", entities.RoleCreator, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("creator write: %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil || created.ID == "" {
		t.Fatalf("decode created: %s %v", rec.Body.String(), err)
	}

	update := gin.H{
		"name":      "Skeleton Lord",
		"level":     9,
		"maxHp":     80,
		"damageMin": 8,
		"damageMax": 18,
		"goldMin":   10,
		"goldMax":   50,
	}
	rec = doJSON(r, http.MethodPut, "/api/npc-templates/"+created.ID, entities.RolePlayer, update)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("player update: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(r, http.MethodPut, "/api/npc-templates/"+created.ID, entities.RoleCreator, update)
	if rec.Code != http.StatusOK {
		t.Fatalf("creator update: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(r, http.MethodGet, "/api/npc-templates/"+created.ID, entities.RolePlayer, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("player detail: %d %s", rec.Code, rec.Body.String())
	}
	var detail struct {
		Name  string          `json:"name"`
		Level int32           `json:"level"`
		Loot  json.RawMessage `json:"loot"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	if detail.Name != "Skeleton Lord" || detail.Level != 9 {
		t.Fatalf("unexpected detail: %s", rec.Body.String())
	}

	rec = doJSON(r, http.MethodDelete, "/api/npc-templates/"+created.ID, entities.RolePlayer, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("player delete: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(r, http.MethodDelete, "/api/npc-templates/"+created.ID, entities.RoleCreator, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("creator delete: %d %s", rec.Code, rec.Body.String())
	}
	// Admins share the creator gate.
	rec = doJSON(r, http.MethodDelete, "/api/npc-templates/"+created.ID, entities.RoleAdmin, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("admin delete missing: %d %s", rec.Code, rec.Body.String())
	}
}
