package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/talesmud/talesmud/pkg/service"
)

// dungeonTestDefinition mirrors the frontend exportDefinition shape
// (public/mud-client/src/game/dungeonBuilder.js).
const dungeonTestDefinition = `{
  "version": 1,
  "grid": {"width": 3, "height": 2},
  "cells": [[{"type":"empty","roomId":null,"encounter":null,"loot":null}]],
  "rooms": [{"id":"room_1","x":0,"y":0,"w":2,"h":2}],
  "meta": {"author":"gandalf","notes":"first draft"}
}`

func newDungeonsTestHandler(t *testing.T) (*DungeonsHandler, service.Facade) {
	t.Helper()
	facade := testFacade(t)
	return &DungeonsHandler{Service: facade.DungeonsService()}, facade
}

func dungeonCreateBody() gin.H {
	return gin.H{
		"name":        "Cripta olvidada",
		"description": "Una cripta bajo la colina",
		"gridWidth":   4,
		"gridHeight":  3,
		"definition":  json.RawMessage(dungeonTestDefinition),
		"author":      "gandalf",
		"tags":        []string{"undead", "crypt"},
	}
}

func decodeDungeonSummary(t *testing.T, rec *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body %s: %v", rec.Body.String(), err)
	}
	return body
}

func TestDungeonCreateGetRoundTrip(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, _ := newDungeonsTestHandler(t)

	rec := performHandlerRequest(http.MethodPost, "/api/dungeons", dungeonCreateBody(), nil, nil, h.Create)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	created := decodeDungeonSummary(t, rec)
	id, _ := created["id"].(string)
	if id == "" {
		t.Fatalf("expected generated id, body=%s", rec.Body.String())
	}
	if created["name"] != "Cripta olvidada" || created["author"] != "gandalf" {
		t.Fatalf("unexpected dto: %v", created)
	}
	if created["gridWidth"].(float64) != 4 || created["gridHeight"].(float64) != 3 {
		t.Fatalf("unexpected grid: %v", created)
	}
	if _, ok := created["definition"]; ok {
		t.Fatalf("summary dto must not carry definition: %v", created)
	}
	updatedAt, _ := created["updatedAt"].(string)
	if parsed, err := time.Parse(time.RFC3339Nano, updatedAt); err != nil || parsed.IsZero() {
		t.Fatalf("expected stamped updatedAt, got %q", updatedAt)
	}

	rec = performHandlerRequest(http.MethodGet, "/api/dungeons/"+id, nil, nil,
		gin.Params{{Key: "id", Value: id}}, h.Get)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var detail struct {
		ID         string          `json:"id"`
		Name       string          `json:"name"`
		Definition json.RawMessage `json:"definition"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	if detail.ID != id || detail.Name != "Cripta olvidada" {
		t.Fatalf("unexpected detail: %s", rec.Body.String())
	}
	if !json.Valid(detail.Definition) {
		t.Fatalf("definition is not valid JSON: %s", detail.Definition)
	}
	// json.Marshal compacts RawMessage payloads on store, so compare modulo
	// insignificant whitespace (same contract as the repository round-trip).
	var want, gotDef bytes.Buffer
	if err := json.Compact(&want, []byte(dungeonTestDefinition)); err != nil {
		t.Fatal(err)
	}
	if err := json.Compact(&gotDef, detail.Definition); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(want.Bytes(), gotDef.Bytes()) {
		t.Fatalf("definition did not round-trip:\nwant %s\ngot  %s", want.String(), gotDef.String())
	}

	rec = performHandlerRequest(http.MethodGet, "/api/dungeons", nil, nil, nil, h.List)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if bytes.Contains(rec.Body.Bytes(), []byte(`"definition"`)) {
		t.Fatalf("list must not embed definitions: %s", rec.Body.String())
	}
	var list []map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list) != 1 || list[0]["id"] != id {
		t.Fatalf("unexpected list: %v", list)
	}
}

func TestDungeonCreateValidatesBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, _ := newDungeonsTestHandler(t)

	tests := []struct {
		name string
		body interface{}
	}{
		{name: "empty name", body: gin.H{
			"gridWidth": 3, "gridHeight": 2,
			"definition": json.RawMessage(dungeonTestDefinition),
		}},
		{name: "blank name", body: gin.H{
			"name": "   ", "gridWidth": 3, "gridHeight": 2,
			"definition": json.RawMessage(dungeonTestDefinition),
		}},
		{name: "missing definition", body: gin.H{
			"name": "Cripta", "gridWidth": 3, "gridHeight": 2,
		}},
		{name: "empty definition", body: gin.H{
			"name": "Cripta", "gridWidth": 3, "gridHeight": 2,
			"definition": json.RawMessage(""),
		}},
		{name: "invalid definition json", body: gin.H{
			"name": "Cripta", "gridWidth": 3, "gridHeight": 2,
			"definition": json.RawMessage("{not json"),
		}},
		{name: "zero grid width", body: gin.H{
			"name": "Cripta", "gridWidth": 0, "gridHeight": 2,
			"definition": json.RawMessage(dungeonTestDefinition),
		}},
		{name: "negative grid height", body: gin.H{
			"name": "Cripta", "gridWidth": 3, "gridHeight": -1,
			"definition": json.RawMessage(dungeonTestDefinition),
		}},
		{name: "malformed body", body: "{"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := performHandlerRequest(http.MethodPost, "/api/dungeons", tt.body, nil, nil, h.Create)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestDungeonGetNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, _ := newDungeonsTestHandler(t)

	rec := performHandlerRequest(http.MethodGet, "/api/dungeons/missing", nil, nil,
		gin.Params{{Key: "id", Value: "missing"}}, h.Get)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestDungeonUpdateAndDelete(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, facade := newDungeonsTestHandler(t)

	rec := performHandlerRequest(http.MethodPost, "/api/dungeons", dungeonCreateBody(), nil, nil, h.Create)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	created := decodeDungeonSummary(t, rec)
	id, _ := created["id"].(string)

	updatedBody := dungeonCreateBody()
	updatedBody["name"] = "Cripta revisada"
	updatedBody["gridWidth"] = 5
	rec = performHandlerRequest(http.MethodPut, "/api/dungeons/"+id, updatedBody, nil,
		gin.Params{{Key: "id", Value: id}}, h.Update)
	if rec.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	body := decodeDungeonSummary(t, rec)
	if body["name"] != "Cripta revisada" || body["gridWidth"].(float64) != 5 {
		t.Fatalf("unexpected updated dto: %v", body)
	}

	stored, err := facade.DungeonsService().FindByID(id)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if stored.Name != "Cripta revisada" || stored.GridWidth != 5 {
		t.Fatalf("store not updated: %+v", stored)
	}
	if stored.CreatedAt.IsZero() {
		t.Fatal("update must preserve CreatedAt")
	}

	rec = performHandlerRequest(http.MethodPut, "/api/dungeons/missing", updatedBody, nil,
		gin.Params{{Key: "id", Value: "missing"}}, h.Update)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("update missing: expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}

	rec = performHandlerRequest(http.MethodDelete, "/api/dungeons/"+id, nil, nil,
		gin.Params{{Key: "id", Value: id}}, h.Delete)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: expected 204, got %d body=%s", rec.Code, rec.Body.String())
	}

	rec = performHandlerRequest(http.MethodDelete, "/api/dungeons/"+id, nil, nil,
		gin.Params{{Key: "id", Value: id}}, h.Delete)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("delete again: expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}

	rec = performHandlerRequest(http.MethodGet, "/api/dungeons/"+id, nil, nil,
		gin.Params{{Key: "id", Value: id}}, h.Get)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("get after delete: expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestDungeonListFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, _ := newDungeonsTestHandler(t)

	seed := []gin.H{
		{"name": "Cripta", "author": "gandalf", "gridWidth": 3, "gridHeight": 2,
			"definition": json.RawMessage(dungeonTestDefinition)},
		{"name": "Mazmorra", "author": "samwise", "gridWidth": 2, "gridHeight": 2,
			"definition": json.RawMessage(dungeonTestDefinition)},
		{"name": "Cripta profunda", "author": "samwise", "gridWidth": 5, "gridHeight": 4,
			"definition": json.RawMessage(dungeonTestDefinition)},
	}
	for _, body := range seed {
		rec := performHandlerRequest(http.MethodPost, "/api/dungeons", body, nil, nil, h.Create)
		if rec.Code != http.StatusCreated {
			t.Fatalf("seed %v: %d %s", body["name"], rec.Code, rec.Body.String())
		}
	}

	list := func(query string) []map[string]interface{} {
		t.Helper()
		rec := performHandlerRequest(http.MethodGet, "/api/dungeons"+query, nil, nil, nil, h.List)
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
	if got := list("?name=Cripta"); len(got) != 1 || got[0]["name"] != "Cripta" {
		t.Fatalf("name filter: %v", got)
	}
	if got := list("?author=samwise"); len(got) != 2 {
		t.Fatalf("author filter: %v", got)
	}
	if got := list("?name=Cripta%20profunda&author=samwise"); len(got) != 1 {
		t.Fatalf("name+author filter: %v", got)
	}
	if got := list("?name=Cripta%20profunda&author=gandalf"); len(got) != 0 {
		t.Fatalf("contradictory filters: %v", got)
	}
	if got := list("?limit=2"); len(got) != 2 {
		t.Fatalf("limit filter: %d items", len(got))
	}
	for _, bad := range []string{"?limit=0", "?limit=-3", "?limit=abc"} {
		rec := performHandlerRequest(http.MethodGet, "/api/dungeons"+bad, nil, nil, nil, h.List)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("bad limit %q: expected 400, got %d", bad, rec.Code)
		}
	}
}

func TestDungeonCreateDoesNotTrustClientID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, _ := newDungeonsTestHandler(t)

	body := dungeonCreateBody()
	body["id"] = "forged-id"
	rec := performHandlerRequest(http.MethodPost, "/api/dungeons", body, nil, nil, h.Create)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	created := decodeDungeonSummary(t, rec)
	if id, _ := created["id"].(string); id == "forged-id" || strings.Contains(rec.Body.String(), "forged-id") {
		t.Fatalf("client-supplied id leaked into create: %s", rec.Body.String())
	}
}
