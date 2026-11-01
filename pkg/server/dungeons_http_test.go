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

// newDungeonsHTTP mirrors the production mounting for /api/dungeons*: reads on
// the authenticated group, writes behind CreatorMiddleware (server.go).
func newDungeonsHTTP(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	client, err := sqlite.Open(filepath.Join(t.TempDir(), "dungeons-http.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	facade := service.NewFacade(repository.NewSQLiteFactory(client), nil)
	h := &handler.DungeonsHandler{Service: facade.DungeonsService()}

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
	api.GET("dungeons", h.List)
	api.GET("dungeons/:id", h.Get)
	creator := api.Group("")
	creator.Use(server.CreatorMiddleware())
	creator.POST("dungeons", h.Create)
	creator.PUT("dungeons/:id", h.Update)
	creator.DELETE("dungeons/:id", h.Delete)
	return r
}

func TestDungeonHTTPCreatorGateAndLifecycle(t *testing.T) {
	r := newDungeonsHTTP(t)
	body := gin.H{
		"name":        "Cripta",
		"description": "Bajo la colina",
		"gridWidth":   3,
		"gridHeight":  2,
		"definition":  json.RawMessage(`{"version":1,"grid":{"width":3,"height":2}}`),
		"author":      "gandalf",
	}

	// Anonymous callers never reach the handlers.
	rec := doJSON(r, http.MethodGet, "/api/dungeons", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous read: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(r, http.MethodPost, "/api/dungeons", "", body)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous write: %d %s", rec.Code, rec.Body.String())
	}

	// Authenticated players may read but not write.
	rec = doJSON(r, http.MethodGet, "/api/dungeons", entities.RolePlayer, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("player read: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(r, http.MethodGet, "/api/dungeons/missing", entities.RolePlayer, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("player read missing: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(r, http.MethodPost, "/api/dungeons", entities.RolePlayer, body)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("player write: %d %s", rec.Code, rec.Body.String())
	}

	// Creators own the full lifecycle.
	rec = doJSON(r, http.MethodPost, "/api/dungeons", entities.RoleCreator, body)
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
		"name":       "Cripta revisada",
		"gridWidth":  4,
		"gridHeight": 2,
		"definition": json.RawMessage(`{"version":1,"grid":{"width":4,"height":2}}`),
	}
	rec = doJSON(r, http.MethodPut, "/api/dungeons/"+created.ID, entities.RolePlayer, update)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("player update: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(r, http.MethodPut, "/api/dungeons/"+created.ID, entities.RoleCreator, update)
	if rec.Code != http.StatusOK {
		t.Fatalf("creator update: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(r, http.MethodGet, "/api/dungeons/"+created.ID, entities.RolePlayer, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("player detail: %d %s", rec.Code, rec.Body.String())
	}
	var detail struct {
		Name       string          `json:"name"`
		Definition json.RawMessage `json:"definition"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	if detail.Name != "Cripta revisada" || !json.Valid(detail.Definition) {
		t.Fatalf("unexpected detail: %s", rec.Body.String())
	}

	rec = doJSON(r, http.MethodDelete, "/api/dungeons/"+created.ID, entities.RolePlayer, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("player delete: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(r, http.MethodDelete, "/api/dungeons/"+created.ID, entities.RoleCreator, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("creator delete: %d %s", rec.Code, rec.Body.String())
	}
	// Admins share the creator gate.
	rec = doJSON(r, http.MethodDelete, "/api/dungeons/"+created.ID, entities.RoleAdmin, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("admin delete missing: %d %s", rec.Code, rec.Body.String())
	}
}
