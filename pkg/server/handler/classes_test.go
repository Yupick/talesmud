package handler

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/talesmud/talesmud/pkg/classkit"
)

func newClassesTestHandler(t *testing.T) *ClassesHandler {
	t.Helper()
	dir := t.TempDir()
	seed := &classkit.Def{
		ID: "seed", Name: "Seed", Description: "seed class", Order: 99,
		Races:   []string{"human"},
		Balance: classkit.Row{DamageDealt: 1, DamageTaken: 1, BehindDealt: 1, Swings: 1},
		Skills: []classkit.SkillSpec{{
			ID: "seed_slam", Name: "Slam", Effect: "slam", Level: 1, Target: "enemy",
		}},
		Template: &classkit.Template{
			ID: "tpl-seed", Str: 10, Dex: 10, Int: 10, Wis: 10, Sta: 10,
		},
	}
	if err := classkit.SaveDef(dir, seed); err != nil {
		t.Fatalf("seed class: %v", err)
	}
	t.Cleanup(func() {
		// Restore the repo world pack so later tests do not see the temp dir.
		classkit.LoadWorld()
	})
	return &ClassesHandler{}
}

func classTestBody(id, name string) gin.H {
	return gin.H{
		"id":            id,
		"name":          name,
		"description":   "A test caster",
		"school":        "arcane",
		"armorType":     "Cloth",
		"combatType":    "Magic",
		"primary":       "INT",
		"skillClass":    id,
		"aliases":       []string{id + "_alias"},
		"races":         []string{"human", "elf"},
		"portraitClass": "mage",
		"hpMultiplier":  0.75,
		"caster":        true,
		"hotbarCap":     4,
		"order":         7,
		"balance": gin.H{
			"damageDealt": 1.4, "damageTaken": 1.25, "behindDealt": 1.15, "swings": 1,
		},
		"attributes": gin.H{"str": 4, "dex": 6, "int": 18, "wis": 14, "sta": 8},
		"mana":       41,
		"hotbar":     []string{id + "_sear"},
		"templateId": "tpl-" + id,
		"backstory":  "scratch a mark",
		"startingItems": []gin.H{
			{"slot": "main_hand", "name": "Apprentice Staff"},
		},
		"skills": []gin.H{{
			"id": id + "_sear", "name": "Sear", "effect": "sear", "level": 4,
			"cooldown": 4, "manaCost": 12, "multiplier": 1.8, "duration": 0,
			"oncePerFight": false, "keepsSwing": false, "target": "enemy",
			"tooltip": "A cast at 1.80x.",
		}},
	}
}

func TestClassesCRUD(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newClassesTestHandler(t)

	rec := performHandlerRequest(http.MethodPost, "/api/classes", classTestBody("ember", "Ember"), nil, nil, h.Create)
	if rec.Code != http.StatusOK {
		t.Fatalf("create: expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		Attributes struct {
			Str int32 `json:"str"`
			Int int32 `json:"int"`
		} `json:"attributes"`
		Balance struct {
			DamageDealt float64 `json:"damageDealt"`
			Swings      int     `json:"swings"`
		} `json:"balance"`
		Mana   int32    `json:"mana"`
		Hotbar []string `json:"hotbar"`
		Skills []struct {
			ID     string `json:"id"`
			Effect string `json:"effect"`
		} `json:"skills"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create body=%s: %v", rec.Body.String(), err)
	}
	if created.ID != "ember" || created.Name != "Ember" {
		t.Fatalf("unexpected create dto: %+v", created)
	}
	if created.Attributes.Int != 18 || created.Mana != 41 {
		t.Fatalf("template fields missing: %+v", created)
	}
	if created.Balance.DamageDealt != 1.4 || created.Balance.Swings != 1 {
		t.Fatalf("balance missing: %+v", created)
	}
	if len(created.Hotbar) != 1 || created.Hotbar[0] != "ember_sear" {
		t.Fatalf("hotbar missing: %+v", created.Hotbar)
	}
	if len(created.Skills) != 1 || created.Skills[0].Effect != "sear" {
		t.Fatalf("skills missing: %+v", created.Skills)
	}
	if classkit.FindDef("ember") == nil {
		t.Fatal("created class not in catalog")
	}

	rec = performHandlerRequest(http.MethodGet, "/api/classes/ember", nil, nil,
		gin.Params{{Key: "id", Value: "ember"}}, h.GetClassByID)
	if rec.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	// The path id wins over the body id.
	update := classTestBody("ignored", "Ember II")
	rec = performHandlerRequest(http.MethodPut, "/api/classes/ember", update, nil,
		gin.Params{{Key: "id", Value: "ember"}}, h.Update)
	if rec.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	updated := classkit.FindDef("ember")
	if updated == nil || updated.Name != "Ember II" {
		t.Fatalf("update did not apply path id: %+v", updated)
	}
	if classkit.FindDef("ignored") != nil {
		t.Fatal("update created a class with the body id")
	}

	rec = performHandlerRequest(http.MethodPost, "/api/classes/ember/duplicate", gin.H{
		"id": "ember_copy", "name": "Ember Copy",
	}, nil, gin.Params{{Key: "id", Value: "ember"}}, h.Duplicate)
	if rec.Code != http.StatusOK {
		t.Fatalf("duplicate: expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var dup struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Skills []struct {
			ID string `json:"id"`
		} `json:"skills"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &dup); err != nil {
		t.Fatalf("decode duplicate: %v", err)
	}
	if dup.ID != "ember_copy" || dup.Name != "Ember Copy" || len(dup.Skills) != 1 {
		t.Fatalf("unexpected duplicate: %+v", dup)
	}

	rec = performHandlerRequest(http.MethodDelete, "/api/classes/ember_copy", nil, nil,
		gin.Params{{Key: "id", Value: "ember_copy"}}, h.Delete)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete: expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	rec = performHandlerRequest(http.MethodGet, "/api/classes/ember_copy", nil, nil,
		gin.Params{{Key: "id", Value: "ember_copy"}}, h.GetClassByID)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("deleted class: expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}

	rec = performHandlerRequest(http.MethodGet, "/api/classes", nil, nil, nil, h.List)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var list []map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list) != 2 || list[0]["id"] != "ember" || list[1]["id"] != "seed" {
		t.Fatalf("unexpected list: %v", list)
	}
}

func TestClassesErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newClassesTestHandler(t)

	emptyID := classTestBody("", "No Id")
	rec := performHandlerRequest(http.MethodPost, "/api/classes", emptyID, nil, nil, h.Create)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty id: expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}

	badEffect := classTestBody("bad_effect", "Bad Effect")
	badEffect["skills"] = []gin.H{{
		"id": "bad_effect_unknown", "name": "Unknown", "effect": "not_real", "level": 1,
	}}
	rec = performHandlerRequest(http.MethodPost, "/api/classes", badEffect, nil, nil, h.Create)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad effect: expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}

	rec = performHandlerRequest(http.MethodGet, "/api/classes/missing", nil, nil,
		gin.Params{{Key: "id", Value: "missing"}}, h.GetClassByID)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("get missing: expected 404, got %d", rec.Code)
	}

	rec = performHandlerRequest(http.MethodPut, "/api/classes/missing", classTestBody("missing", "Missing"), nil,
		gin.Params{{Key: "id", Value: "missing"}}, h.Update)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("update missing: expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}

	rec = performHandlerRequest(http.MethodDelete, "/api/classes/missing", nil, nil,
		gin.Params{{Key: "id", Value: "missing"}}, h.Delete)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("delete missing: expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}

	rec = performHandlerRequest(http.MethodPost, "/api/classes/missing/duplicate", gin.H{}, nil,
		gin.Params{{Key: "id", Value: "missing"}}, h.Duplicate)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("duplicate missing: expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}
}
