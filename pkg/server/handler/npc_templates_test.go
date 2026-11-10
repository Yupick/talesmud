package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/talesmud/talesmud/pkg/entities"
	npc "github.com/talesmud/talesmud/pkg/entities/npcs"
	"github.com/talesmud/talesmud/pkg/service"
)

func newNPCTemplatesTestHandler(t *testing.T) (*NPCTemplatesHandler, service.Facade) {
	t.Helper()
	facade := testFacade(t)
	return &NPCTemplatesHandler{Service: facade.NPCTemplatesService()}, facade
}

func npcTemplateCreateBody() gin.H {
	return gin.H{
		"name":           "Skeleton Warrior",
		"description":    "An animated bones soldier",
		"role":           "enemy",
		"creatureType":   "undead",
		"combatStyle":    "melee",
		"difficulty":     "normal",
		"level":          3,
		"maxHp":          45,
		"damageMin":      4,
		"damageMax":      9,
		"aggressive":     true,
		"aggroRadius":    2,
		"xpReward":       120,
		"goldMin":        5,
		"goldMax":        25,
		"lootTableId":    "loot_skeleton",
		"loot":           json.RawMessage(`[{"itemId":"item_bone","chance":80}]`),
		"spawnRoomId":    "room_crypt_1",
		"spawnCount":     3,
		"respawnSeconds": 60,
		"behavior":       json.RawMessage(`{"stateMachine":[]}`),
		"scaling":        json.RawMessage(`{"targetLevel":null}`),
	}
}

func decodeNPCTemplateDTO(t *testing.T, rec *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body %s: %v", rec.Body.String(), err)
	}
	return body
}

func TestNPCTemplateCreateGetRoundTrip(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, _ := newNPCTemplatesTestHandler(t)

	rec := performHandlerRequest(http.MethodPost, "/api/npc-templates", npcTemplateCreateBody(), nil, nil, h.Create)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	created := decodeNPCTemplateDTO(t, rec)
	id, _ := created["id"].(string)
	if id == "" {
		t.Fatalf("expected generated id, body=%s", rec.Body.String())
	}
	if created["name"] != "Skeleton Warrior" || created["role"] != "enemy" {
		t.Fatalf("unexpected dto: %v", created)
	}
	if created["description"] != "An animated bones soldier" {
		t.Fatalf("detail dto must carry description: %v", created)
	}
	if _, ok := created["loot"]; !ok {
		t.Fatalf("detail dto must carry loot: %v", created)
	}
	if _, ok := created["behavior"]; !ok {
		t.Fatalf("detail dto must carry behavior: %v", created)
	}
	updatedAt, _ := created["updatedAt"].(string)
	if parsed, err := time.Parse(time.RFC3339Nano, updatedAt); err != nil || parsed.IsZero() {
		t.Fatalf("expected stamped updatedAt, got %q", updatedAt)
	}

	rec = performHandlerRequest(http.MethodGet, "/api/npc-templates/"+id, nil, nil,
		gin.Params{{Key: "id", Value: id}}, h.Get)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var detail struct {
		ID         string          `json:"id"`
		Name       string          `json:"name"`
		MaxHp      int32           `json:"maxHp"`
		Loot       json.RawMessage `json:"loot"`
		Scaling    json.RawMessage `json:"scaling"`
		SpawnCount int32           `json:"spawnCount"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	if detail.ID != id || detail.Name != "Skeleton Warrior" || detail.MaxHp != 45 || detail.SpawnCount != 3 {
		t.Fatalf("unexpected detail: %s", rec.Body.String())
	}
	if !json.Valid(detail.Loot) || !json.Valid(detail.Scaling) {
		t.Fatalf("raw payloads invalid: %s", rec.Body.String())
	}

	// The list payload must stay lightweight.
	rec = performHandlerRequest(http.MethodGet, "/api/npc-templates", nil, nil, nil, h.List)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	for _, heavy := range []string{`"loot"`, `"behavior"`, `"scaling"`, `"description"`} {
		if bytes.Contains(rec.Body.Bytes(), []byte(heavy)) {
			t.Fatalf("list must not embed %s: %s", heavy, rec.Body.String())
		}
	}
	var list []map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list) != 1 || list[0]["id"] != id {
		t.Fatalf("unexpected list: %v", list)
	}
}

func TestNPCTemplateLLMRoundTrip(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, facade := newNPCTemplatesTestHandler(t)

	body := npcTemplateCreateBody()
	body["llm"] = map[string]any{
		"enabled":          true,
		"persona":          "Soy el guardián de la cripta.",
		"voice":            "grave",
		"temperature":      0.7,
		"memoryTurns":      6,
		"fallbackDialogId": "DLG_CRYPT",
		"cooldownSeconds":  5,
		"personality": map[string]any{
			"openness":          0.4,
			"conscientiousness": 0.9,
			"extraversion":      0.6,
			"agreeableness":     0.5,
			"neuroticism":       0.3,
		},
		"goals":   []string{"Proteger la cripta."},
		"secrets": []string{"El rey duerme bajo el altar."},
		"relations": []map[string]any{
			{"target": "player", "kind": "mentor", "sentiment": 0.7},
		},
	}

	rec := performHandlerRequest(http.MethodPost, "/api/npc-templates", body, nil, nil, h.Create)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	created := decodeNPCTemplateDTO(t, rec)
	id, _ := created["id"].(string)
	if _, ok := created["llm"]; !ok {
		t.Fatalf("detail dto must carry llm: %v", created)
	}

	stored, err := facade.NPCTemplatesService().FindByID(id)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if stored.LLM == nil || !stored.LLM.Enabled {
		t.Fatalf("llm not persisted: %+v", stored.LLM)
	}
	if stored.LLM.MemoryTurns != 6 || stored.LLM.CooldownSeconds != 5 ||
		stored.LLM.FallbackDialogID != "DLG_CRYPT" {
		t.Fatalf("llm fields not persisted: %+v", stored.LLM)
	}
	if stored.LLM.Personality == nil || stored.LLM.Personality.Conscientiousness != 0.9 {
		t.Fatalf("personality not persisted: %+v", stored.LLM.Personality)
	}
	if len(stored.LLM.Goals) != 1 || stored.LLM.Goals[0] != "Proteger la cripta." {
		t.Fatalf("goals not persisted: %+v", stored.LLM.Goals)
	}
	if len(stored.LLM.Secrets) != 1 || len(stored.LLM.Relations) != 1 {
		t.Fatalf("secrets/relations not persisted: %+v", stored.LLM)
	}

	rec = performHandlerRequest(http.MethodGet, "/api/npc-templates/"+id, nil, nil,
		gin.Params{{Key: "id", Value: id}}, h.Get)
	if rec.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var detail struct {
		LLM *npc.LLMConfig `json:"llm"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	if detail.LLM == nil || detail.LLM.Persona != "Soy el guardián de la cripta." ||
		detail.LLM.MemoryTurns != 6 {
		t.Fatalf("detail llm not round-tripped: %+v", detail.LLM)
	}
}

func TestNPCTemplateGetRedactsSecretsForNonCreators(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, facade := newNPCTemplatesTestHandler(t)

	secret := "El rey duerme bajo el altar."
	body := npcTemplateCreateBody()
	body["llm"] = map[string]any{
		"enabled": true,
		"persona": "Soy el guardián de la cripta.",
		"secrets": []string{secret},
	}

	rec := performHandlerRequest(http.MethodPost, "/api/npc-templates", body, nil, nil, h.Create)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(secret)) {
		t.Fatalf("creator create response must keep authored secrets: %s", rec.Body.String())
	}
	created := decodeNPCTemplateDTO(t, rec)
	id, _ := created["id"].(string)
	if id == "" {
		t.Fatalf("expected generated id, body=%s", rec.Body.String())
	}

	tests := []struct {
		name        string
		user        *entities.User
		wantSecrets bool
	}{
		{name: "anonymous context fails closed", user: nil, wantSecrets: false},
		{name: "player", user: testUser("player-1", "ref-player", entities.RolePlayer), wantSecrets: false},
		{name: "creator", user: testUser("creator-1", "ref-creator", entities.RoleCreator), wantSecrets: true},
		{name: "admin", user: testUser("admin-1", "ref-admin", entities.RoleAdmin), wantSecrets: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := performHandlerRequest(http.MethodGet, "/api/npc-templates/"+id, nil, tt.user,
				gin.Params{{Key: "id", Value: id}}, h.Get)
			if rec.Code != http.StatusOK {
				t.Fatalf("get: expected 200, got %d body=%s", rec.Code, rec.Body.String())
			}
			var detail struct {
				LLM *npc.LLMConfig `json:"llm"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
				t.Fatalf("decode detail: %v", err)
			}
			if tt.wantSecrets {
				if detail.LLM == nil || len(detail.LLM.Secrets) != 1 || detail.LLM.Secrets[0] != secret {
					t.Fatalf("expected secrets for %s: %s", tt.name, rec.Body.String())
				}
				return
			}
			if detail.LLM != nil && len(detail.LLM.Secrets) != 0 {
				t.Fatalf("secrets exposed to %s: %s", tt.name, rec.Body.String())
			}
			if bytes.Contains(rec.Body.Bytes(), []byte(secret)) {
				t.Fatalf("secret leaked to %s: %s", tt.name, rec.Body.String())
			}
		})
	}

	// Redaction must not mutate the stored template.
	stored, err := facade.NPCTemplatesService().FindByID(id)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if stored.LLM == nil || len(stored.LLM.Secrets) != 1 || stored.LLM.Secrets[0] != secret {
		t.Fatalf("stored secrets were mutated: %+v", stored.LLM)
	}
}

func TestNPCTemplateCreateValidatesBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, _ := newNPCTemplatesTestHandler(t)

	base := func() gin.H { return npcTemplateCreateBody() }

	tests := []struct {
		name string
		body interface{}
	}{
		{name: "empty name", body: func() gin.H { b := base(); b["name"] = ""; return b }()},
		{name: "blank name", body: func() gin.H { b := base(); b["name"] = "   "; return b }()},
		{name: "level below one", body: func() gin.H { b := base(); b["level"] = 0; return b }()},
		{name: "maxHp below one", body: func() gin.H { b := base(); b["maxHp"] = 0; return b }()},
		{name: "negative damage", body: func() gin.H { b := base(); b["damageMin"] = -1; return b }()},
		{name: "damageMax below damageMin", body: func() gin.H { b := base(); b["damageMin"] = 12; b["damageMax"] = 3; return b }()},
		{name: "goldMax below goldMin", body: func() gin.H { b := base(); b["goldMin"] = 40; b["goldMax"] = 4; return b }()},
		{name: "invalid loot", body: func() gin.H { b := base(); b["loot"] = json.RawMessage(`{oops`); return b }()},
		{name: "invalid behavior", body: func() gin.H { b := base(); b["behavior"] = json.RawMessage(`nope`); return b }()},
		{name: "invalid scaling", body: func() gin.H { b := base(); b["scaling"] = json.RawMessage(`[`); return b }()},
		{name: "malformed body", body: "{"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := performHandlerRequest(http.MethodPost, "/api/npc-templates", tt.body, nil, nil, h.Create)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestNPCTemplateGetNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, _ := newNPCTemplatesTestHandler(t)

	rec := performHandlerRequest(http.MethodGet, "/api/npc-templates/missing", nil, nil,
		gin.Params{{Key: "id", Value: "missing"}}, h.Get)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestNPCTemplateUpdateAndDelete(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, facade := newNPCTemplatesTestHandler(t)

	rec := performHandlerRequest(http.MethodPost, "/api/npc-templates", npcTemplateCreateBody(), nil, nil, h.Create)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	created := decodeNPCTemplateDTO(t, rec)
	id, _ := created["id"].(string)

	updatedBody := npcTemplateCreateBody()
	updatedBody["name"] = "Skeleton Lord"
	updatedBody["level"] = 9
	rec = performHandlerRequest(http.MethodPut, "/api/npc-templates/"+id, updatedBody, nil,
		gin.Params{{Key: "id", Value: id}}, h.Update)
	if rec.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	body := decodeNPCTemplateDTO(t, rec)
	if body["name"] != "Skeleton Lord" || body["level"].(float64) != 9 || body["id"] != id {
		t.Fatalf("unexpected updated dto: %v", body)
	}

	stored, err := facade.NPCTemplatesService().FindByID(id)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if stored.Name != "Skeleton Lord" || stored.Level != 9 {
		t.Fatalf("store not updated: %+v", stored)
	}
	if stored.CreatedAt.IsZero() {
		t.Fatal("update must preserve CreatedAt")
	}

	rec = performHandlerRequest(http.MethodPut, "/api/npc-templates/missing", updatedBody, nil,
		gin.Params{{Key: "id", Value: "missing"}}, h.Update)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("update missing: expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}

	rec = performHandlerRequest(http.MethodDelete, "/api/npc-templates/"+id, nil, nil,
		gin.Params{{Key: "id", Value: id}}, h.Delete)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: expected 204, got %d body=%s", rec.Code, rec.Body.String())
	}
	rec = performHandlerRequest(http.MethodDelete, "/api/npc-templates/"+id, nil, nil,
		gin.Params{{Key: "id", Value: id}}, h.Delete)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("delete again: expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}

	rec = performHandlerRequest(http.MethodGet, "/api/npc-templates/"+id, nil, nil,
		gin.Params{{Key: "id", Value: id}}, h.Get)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("get after delete: expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestNPCTemplateListFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, _ := newNPCTemplatesTestHandler(t)

	seed := []gin.H{
		{"name": "Skeleton", "role": "enemy", "level": 2, "maxHp": 30},
		{"name": "Zombie", "role": "enemy", "level": 4, "maxHp": 55},
		{"name": "Merchant", "role": "merchant", "level": 1, "maxHp": 10},
	}
	for _, body := range seed {
		rec := performHandlerRequest(http.MethodPost, "/api/npc-templates", body, nil, nil, h.Create)
		if rec.Code != http.StatusCreated {
			t.Fatalf("seed %v: %d %s", body["name"], rec.Code, rec.Body.String())
		}
	}

	list := func(query string) []map[string]interface{} {
		t.Helper()
		rec := performHandlerRequest(http.MethodGet, "/api/npc-templates"+query, nil, nil, nil, h.List)
		if rec.Code != http.StatusOK {
			t.Fatalf("list %q: %d %s", query, rec.Code, rec.Body.String())
		}
		var out []map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode %q: %v", query, err)
		}
		return out
	}

	if got := list(""); len(got) != 3 {
		t.Fatalf("unfiltered list: %d items", len(got))
	}
	if got := list("?name=Skeleton"); len(got) != 1 || got[0]["name"] != "Skeleton" {
		t.Fatalf("name filter: %v", got)
	}
	if got := list("?role=enemy"); len(got) != 2 {
		t.Fatalf("role filter: %v", got)
	}
	if got := list("?name=Skeleton&role=merchant"); len(got) != 0 {
		t.Fatalf("contradictory filters: %v", got)
	}
	if got := list("?limit=2"); len(got) != 2 {
		t.Fatalf("limit filter: %d items", len(got))
	}
	for _, bad := range []string{"?limit=0", "?limit=-3", "?limit=abc"} {
		rec := performHandlerRequest(http.MethodGet, "/api/npc-templates"+bad, nil, nil, nil, h.List)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("bad limit %q: expected 400, got %d", bad, rec.Code)
		}
	}
}

func TestNPCTemplateCreateDoesNotTrustClientID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, _ := newNPCTemplatesTestHandler(t)

	body := npcTemplateCreateBody()
	body["id"] = "forged-id"
	rec := performHandlerRequest(http.MethodPost, "/api/npc-templates", body, nil, nil, h.Create)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	created := decodeNPCTemplateDTO(t, rec)
	if id, _ := created["id"].(string); id == "forged-id" || strings.Contains(rec.Body.String(), "forged-id") {
		t.Fatalf("client-supplied id leaked into create: %s", rec.Body.String())
	}
}

// fakeNPCTemplateSpawner records the templates handed to the live-world bridge
// and returns canned results/errors.
type fakeNPCTemplateSpawner struct {
	calls      []*npc.NPCTemplate
	instanceID string
	name       string
	roomID     string
	err        error
}

func (f *fakeNPCTemplateSpawner) spawn(tpl *npc.NPCTemplate) (string, string, string, error) {
	f.calls = append(f.calls, tpl)
	if f.err != nil {
		return "", "", "", f.err
	}
	name := f.name
	if name == "" && tpl != nil {
		name = tpl.Name
	}
	return f.instanceID, name, f.roomID, nil
}

func TestNPCTemplateCreateAndUpdateTriggerSpawner(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, _ := newNPCTemplatesTestHandler(t)
	fake := &fakeNPCTemplateSpawner{instanceID: "instance-1", roomID: "room_crypt_1"}
	h.Spawner = fake.spawn

	rec := performHandlerRequest(http.MethodPost, "/api/npc-templates", npcTemplateCreateBody(), nil, nil, h.Create)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	if len(fake.calls) != 1 {
		t.Fatalf("create must call the spawner once, got %d", len(fake.calls))
	}
	created := decodeNPCTemplateDTO(t, rec)
	spawn, ok := created["spawn"].(map[string]interface{})
	if !ok {
		t.Fatalf("create response must carry spawn result: %s", rec.Body.String())
	}
	if spawn["instanceId"] != "instance-1" || spawn["roomId"] != "room_crypt_1" || spawn["name"] != "Skeleton Warrior" {
		t.Fatalf("unexpected spawn result: %v", spawn)
	}
	id, _ := created["id"].(string)
	if id == "" {
		t.Fatalf("expected created id: %s", rec.Body.String())
	}

	fake.calls = nil
	fake.instanceID = "instance-2"
	updatedBody := npcTemplateCreateBody()
	updatedBody["name"] = "Skeleton Lord"
	rec = performHandlerRequest(http.MethodPut, "/api/npc-templates/"+id, updatedBody, nil,
		gin.Params{{Key: "id", Value: id}}, h.Update)
	if rec.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if len(fake.calls) != 1 {
		t.Fatalf("update must call the spawner once, got %d", len(fake.calls))
	}
	if fake.calls[0].ID != id || fake.calls[0].SpawnRoomId != "room_crypt_1" {
		t.Fatalf("update handed wrong template to spawner: %+v", fake.calls[0])
	}
	updated := decodeNPCTemplateDTO(t, rec)
	if spawn, ok := updated["spawn"].(map[string]interface{}); !ok || spawn["instanceId"] != "instance-2" {
		t.Fatalf("update response must carry spawn result: %s", rec.Body.String())
	}
}

func TestNPCTemplateCreateWithoutSpawnConfigSkipsSpawner(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, _ := newNPCTemplatesTestHandler(t)
	fake := &fakeNPCTemplateSpawner{instanceID: "instance-1", roomID: "room_crypt_1"}
	h.Spawner = fake.spawn

	body := npcTemplateCreateBody()
	body["spawnRoomId"] = ""
	body["spawnCount"] = 0
	rec := performHandlerRequest(http.MethodPost, "/api/npc-templates", body, nil, nil, h.Create)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	if len(fake.calls) != 0 {
		t.Fatalf("create without a spawn room must not call the spawner, got %d", len(fake.calls))
	}
	if _, ok := decodeNPCTemplateDTO(t, rec)["spawn"]; ok {
		t.Fatalf("create without a spawn attempt must omit spawn: %s", rec.Body.String())
	}
}

func TestNPCTemplateCreateSpawnerFailureIsBestEffort(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, _ := newNPCTemplatesTestHandler(t)
	fake := &fakeNPCTemplateSpawner{err: errors.New("npc manager is unavailable")}
	h.Spawner = fake.spawn

	rec := performHandlerRequest(http.MethodPost, "/api/npc-templates", npcTemplateCreateBody(), nil, nil, h.Create)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create must survive a spawn failure: %d body=%s", rec.Code, rec.Body.String())
	}
	if len(fake.calls) != 1 {
		t.Fatalf("spawner must have been attempted, got %d", len(fake.calls))
	}
	if _, ok := decodeNPCTemplateDTO(t, rec)["spawn"]; ok {
		t.Fatalf("failed spawn must be omitted from the response: %s", rec.Body.String())
	}
}

func TestNPCTemplateSpawnEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, _ := newNPCTemplatesTestHandler(t)
	fake := &fakeNPCTemplateSpawner{instanceID: "instance-1", name: "Skeleton Warrior-abc123", roomID: "room_crypt_1"}
	h.Spawner = fake.spawn

	rec := performHandlerRequest(http.MethodPost, "/api/npc-templates", npcTemplateCreateBody(), nil, nil, h.Create)
	if rec.Code != http.StatusCreated {
		t.Fatalf("seed create: %d %s", rec.Code, rec.Body.String())
	}
	id, _ := decodeNPCTemplateDTO(t, rec)["id"].(string)
	fake.calls = nil

	rec = performHandlerRequest(http.MethodPost, "/api/npc-templates/"+id+"/spawn", nil, nil,
		gin.Params{{Key: "id", Value: id}}, h.Spawn)
	if rec.Code != http.StatusOK {
		t.Fatalf("spawn: expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var result struct {
		InstanceID string `json:"instanceId"`
		Name       string `json:"name"`
		RoomID     string `json:"roomId"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode spawn: %v", err)
	}
	if result.InstanceID != "instance-1" || result.Name != "Skeleton Warrior-abc123" || result.RoomID != "room_crypt_1" {
		t.Fatalf("unexpected spawn response: %s", rec.Body.String())
	}
	if len(fake.calls) != 1 || fake.calls[0].ID != id {
		t.Fatalf("spawn must pass the persisted template, got %+v", fake.calls)
	}

	rec = performHandlerRequest(http.MethodPost, "/api/npc-templates/missing/spawn", nil, nil,
		gin.Params{{Key: "id", Value: "missing"}}, h.Spawn)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("spawn missing: expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}

	unavailable := &NPCTemplatesHandler{Service: h.Service}
	rec = performHandlerRequest(http.MethodPost, "/api/npc-templates/"+id+"/spawn", nil, nil,
		gin.Params{{Key: "id", Value: id}}, unavailable.Spawn)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("spawn without bridge: expected 503, got %d body=%s", rec.Code, rec.Body.String())
	}

	fake.err = errors.New("spawnRoomId is required")
	rec = performHandlerRequest(http.MethodPost, "/api/npc-templates/"+id+"/spawn", nil, nil,
		gin.Params{{Key: "id", Value: id}}, h.Spawn)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("spawn config error: expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}

	fake.err = errors.New("npc manager is unavailable")
	rec = performHandlerRequest(http.MethodPost, "/api/npc-templates/"+id+"/spawn", nil, nil,
		gin.Params{{Key: "id", Value: id}}, h.Spawn)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("spawn infrastructure error: expected 500, got %d body=%s", rec.Code, rec.Body.String())
	}
}
