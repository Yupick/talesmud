package repository

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/talesmud/talesmud/pkg/db/sqlite"
	"github.com/talesmud/talesmud/pkg/entities/dungeons"
)

// newDungeonTestFactory opens a SQLite database in a temp file and returns a
// factory wired to it. A file (instead of :memory:) guarantees a single
// database across the connection pool.
func newDungeonTestFactory(t *testing.T) Factory {
	t.Helper()
	client, err := sqlite.Open(filepath.Join(t.TempDir(), "dungeons.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return NewSQLiteFactory(client)
}

// sampleDefinition mirrors the frontend exportDefinition shape (dungeonBuilder.js).
const sampleDefinition = `{
  "version": 1,
  "grid": {"width": 3, "height": 2},
  "cells": [[{"type":"empty","roomId":null,"encounter":null,"loot":null}]],
  "rooms": [{"id":"room_1","x":0,"y":0,"w":2,"h":2}],
  "corridors": [{"points":[[2,0],[2,1]]}],
  "entrances": [{"x":0,"y":1}],
  "exits": [{"x":2,"y":0}],
  "encounters": [{"id":"enc_1","cell":{"x":2,"y":1},"npcTemplateId":"npc-skeleton","count":2,"level":3}],
  "lootTables": [{"name":"bones","entries":[{"itemId":"item-bone","chance":50,"qty":2}]}],
  "meta": {"author":"gandalf","notes":"first draft"}
}`

func TestDungeonDefinitionCRUD(t *testing.T) {
	repo := newDungeonTestFactory(t).Dungeons()

	created, err := repo.Create(&dungeons.DungeonDefinition{
		Name:        "Cripta olvidada",
		Description: "Una cripta bajo la colina",
		GridWidth:   4,
		GridHeight:  3,
		Definition:  json.RawMessage(sampleDefinition),
		Author:      "gandalf",
		Tags:        []string{"undead", "crypt"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected generated id")
	}
	if created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Fatal("expected timestamps to be stamped")
	}

	got, err := repo.FindByID(created.ID)
	if err != nil {
		t.Fatalf("find by id: %v", err)
	}
	if got.Name != "Cripta olvidada" || got.Author != "gandalf" {
		t.Fatalf("unexpected definition: %+v", got)
	}
	if got.GridWidth != 4 || got.GridHeight != 3 {
		t.Fatalf("unexpected grid: %+v", got)
	}
	if len(got.Tags) != 2 || got.Tags[0] != "undead" {
		t.Fatalf("unexpected tags: %v", got.Tags)
	}

	byName, err := repo.FindByName("Cripta olvidada")
	if err != nil || len(byName) != 1 {
		t.Fatalf("find by name: %v len=%d", err, len(byName))
	}
	byAuthor, err := repo.FindByAuthor("gandalf")
	if err != nil || len(byAuthor) != 1 {
		t.Fatalf("find by author: %v len=%d", err, len(byAuthor))
	}
	all, err := repo.FindAll()
	if err != nil || len(all) != 1 {
		t.Fatalf("find all: %v len=%d", err, len(all))
	}

	// Update: mutate a fetched entity and save it back.
	got.Description = "Una cripta ampliada"
	got.Tags = []string{"undead", "crypt", "boss"}
	if err := repo.Update(got.ID, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	updated, err := repo.FindByID(got.ID)
	if err != nil {
		t.Fatalf("find after update: %v", err)
	}
	if updated.Description != "Una cripta ampliada" {
		t.Fatalf("unexpected description: %q", updated.Description)
	}
	if len(updated.Tags) != 3 || updated.Tags[2] != "boss" {
		t.Fatalf("unexpected tags after update: %v", updated.Tags)
	}
	if updated.CreatedAt.IsZero() {
		t.Fatal("expected createdAt to be preserved")
	}
	if !updated.UpdatedAt.After(updated.CreatedAt) && !updated.UpdatedAt.Equal(updated.CreatedAt) {
		t.Fatalf("expected updatedAt >= createdAt: %v / %v", updated.UpdatedAt, updated.CreatedAt)
	}

	// Delete removes the row and reports the missing row on a second call.
	if err := repo.Delete(got.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.FindByID(got.ID); err == nil {
		t.Fatal("expected find after delete to fail")
	}
	if err := repo.Delete(got.ID); err == nil {
		t.Fatal("expected deleting an absent definition to fail")
	}
}

// TestDungeonDefinitionJSONRoundTrip verifies the exportDefinition payload
// survives a write/read cycle unchanged (modulo insignificant whitespace).
func TestDungeonDefinitionJSONRoundTrip(t *testing.T) {
	repo := newDungeonTestFactory(t).Dungeons()

	created, err := repo.Create(&dungeons.DungeonDefinition{
		Name:       "Round trip",
		GridWidth:  3,
		GridHeight: 2,
		Definition: json.RawMessage(sampleDefinition),
		Author:     "gandalf",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := repo.FindByID(created.ID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if !sameJSON(sampleDefinition, string(got.Definition)) {
		t.Fatalf("definition JSON did not round-trip:\nwant %s\ngot  %s", sampleDefinition, got.Definition)
	}
	if got.GridWidth != 3 || got.GridHeight != 2 {
		t.Fatalf("unexpected grid: %+v", got)
	}
}

// sameJSON compares two JSON documents after compacting, ignoring whitespace.
func sameJSON(a, b string) bool {
	var ab, bb bytes.Buffer
	if err := json.Compact(&ab, []byte(a)); err != nil {
		return false
	}
	if err := json.Compact(&bb, []byte(b)); err != nil {
		return false
	}
	return bytes.Equal(ab.Bytes(), bb.Bytes())
}

// TestDungeonMigrationIsIdempotent opens the same database file several times;
// every open re-runs the FASE 5.1 migration and must succeed without data loss.
func TestDungeonMigrationIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dungeons-idempotent.db")

	client, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	factory := NewSQLiteFactory(client)
	created, err := factory.Dungeons().Create(&dungeons.DungeonDefinition{
		Name:       "Persistent",
		GridWidth:  2,
		GridHeight: 2,
		Definition: json.RawMessage(`{"version":1}`),
		Author:     "gandalf",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := factory.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// Reopen: the migration runs again on an existing database.
	client2, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("second open (migration idempotency): %v", err)
	}
	t.Cleanup(func() { _ = client2.Close() })
	factory2 := NewSQLiteFactory(client2)

	got, err := factory2.Dungeons().FindByID(created.ID)
	if err != nil {
		t.Fatalf("find after reopen: %v", err)
	}
	if got.Name != "Persistent" || got.Author != "gandalf" {
		t.Fatalf("data lost across reopen: %+v", got)
	}

	// A third open confirms repeated migrations stay no-ops.
	client3, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("third open: %v", err)
	}
	t.Cleanup(func() { _ = client3.Close() })
}
