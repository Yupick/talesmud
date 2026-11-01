package repository

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/talesmud/talesmud/pkg/db/sqlite"
	npc "github.com/talesmud/talesmud/pkg/entities/npcs"
)

// newNPCTemplateTestFactory opens a SQLite database in a temp file and returns
// a factory wired to it. A file (instead of :memory:) guarantees a single
// database across the connection pool.
func newNPCTemplateTestFactory(t *testing.T) Factory {
	t.Helper()
	client, err := sqlite.Open(filepath.Join(t.TempDir(), "npc_templates.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return NewSQLiteFactory(client)
}

// sampleTemplate mirrors the frontend npcFactory.js normalized shape.
const sampleTemplate = `{
  "name": "Skeleton Warrior",
  "description": "An animated bones soldier",
  "role": "enemy",
  "creatureType": "undead",
  "combatStyle": "melee",
  "difficulty": "normal",
  "level": 3,
  "maxHp": 45,
  "damageMin": 4,
  "damageMax": 9,
  "aggressive": true,
  "aggroRadius": 2,
  "xpReward": 120,
  "goldMin": 5,
  "goldMax": 25,
  "lootTableId": "loot_skeleton",
  "spawnRoomId": "room_crypt_1",
  "spawnCount": 3,
  "respawnSeconds": 60,
  "isTemplate": true
}`

// sampleLoot mirrors the frontend loot editor payload.
const sampleLoot = `[
  {"itemId": "item_bone", "chance": 80, "quantity": 1},
  {"itemId": "item_rusty_sword", "chance": 15, "quantity": 1}
]`

// sampleBehavior mirrors npcBehavior.js defaultBehavior().
const sampleBehavior = `{
  "stateMachine": [
    {"from": "idle", "event": "aggro", "to": "combat"},
    {"from": "combat", "event": "target_lost", "to": "idle"}
  ],
  "aggro": {"radius": 2, "conditions": ["player_in_room"]},
  "abilities": [{"id": "slash", "cooldown": 3, "range": 1}]
}`

// sampleScaling mirrors npcBehavior.js defaultScaling().
const sampleScaling = `{"targetLevel": null}`

func TestNPCTemplateCRUD(t *testing.T) {
	repo := newNPCTemplateTestFactory(t).NPCTemplates()

	created, err := repo.Create(&npc.NPCTemplate{
		Name:          "Skeleton Warrior",
		Description:   "An animated bones soldier",
		Role:          "enemy",
		CreatureType:   "undead",
		CombatStyle:   "melee",
		Difficulty:    "normal",
		Level:         3,
		MaxHp:         45,
		DamageMin:     4,
		DamageMax:     9,
		Aggressive:    true,
		AggroRadius:   2,
		XpReward:      120,
		GoldMin:       5,
		GoldMax:       25,
		LootTableID:   "loot_skeleton",
		Loot:          json.RawMessage(sampleLoot),
		SpawnRoomId:   "room_crypt_1",
		SpawnCount:    3,
		RespawnSeconds: 60,
		Behavior:      json.RawMessage(sampleBehavior),
		Scaling:       json.RawMessage(sampleScaling),
		IsTemplate:    true,
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
	if got.Name != "Skeleton Warrior" || got.Role != "enemy" {
		t.Fatalf("unexpected template: %+v", got)
	}
	if got.CreatureType != "undead" || got.CombatStyle != "melee" {
		t.Fatalf("unexpected creature/combat: %+v", got)
	}
	if got.Level != 3 || got.MaxHp != 45 || got.DamageMin != 4 || got.DamageMax != 9 {
		t.Fatalf("unexpected stats: %+v", got)
	}
	if !got.Aggressive || got.AggroRadius != 2 {
		t.Fatalf("unexpected aggro: %+v", got)
	}
	if got.XpReward != 120 || got.GoldMin != 5 || got.GoldMax != 25 {
		t.Fatalf("unexpected rewards: %+v", got)
	}
	if got.LootTableID != "loot_skeleton" || got.SpawnRoomId != "room_crypt_1" {
		t.Fatalf("unexpected loot/spawn: %+v", got)
	}
	if got.SpawnCount != 3 || got.RespawnSeconds != 60 {
		t.Fatalf("unexpected spawn config: %+v", got)
	}
	if !got.IsTemplate {
		t.Fatal("expected isTemplate=true")
	}

	byName, err := repo.FindByName("Skeleton Warrior")
	if err != nil || len(byName) != 1 {
		t.Fatalf("find by name: %v len=%d", err, len(byName))
	}
	byRole, err := repo.FindByRole("enemy")
	if err != nil || len(byRole) != 1 {
		t.Fatalf("find by role: %v len=%d", err, len(byRole))
	}
	all, err := repo.FindAll()
	if err != nil || len(all) != 1 {
		t.Fatalf("find all: %v len=%d", err, len(all))
	}

	// Update: mutate a fetched entity and save it back.
	got.Description = "An elite animated bones soldier"
	got.MaxHp = 60
	if err := repo.Update(got.ID, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	updated, err := repo.FindByID(got.ID)
	if err != nil {
		t.Fatalf("find after update: %v", err)
	}
	if updated.Description != "An elite animated bones soldier" {
		t.Fatalf("unexpected description: %q", updated.Description)
	}
	if updated.MaxHp != 60 {
		t.Fatalf("unexpected maxHp after update: %d", updated.MaxHp)
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
		t.Fatal("expected deleting an absent template to fail")
	}
}

// TestNPCTemplateJSONRoundTrip verifies the loot, behavior and scaling raw
// JSON payloads survive a write/read cycle unchanged (modulo insignificant
// whitespace).
func TestNPCTemplateJSONRoundTrip(t *testing.T) {
	repo := newNPCTemplateTestFactory(t).NPCTemplates()

	created, err := repo.Create(&npc.NPCTemplate{
		Name:        "JSON Round Trip",
		Role:        "boss",
		Loot:        json.RawMessage(sampleLoot),
		Behavior:    json.RawMessage(sampleBehavior),
		Scaling:     json.RawMessage(sampleScaling),
		IsTemplate:  true,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := repo.FindByID(created.ID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if !sameJSON(sampleLoot, string(got.Loot)) {
		t.Fatalf("loot JSON did not round-trip:\nwant %s\ngot  %s", sampleLoot, got.Loot)
	}
	if !sameJSON(sampleBehavior, string(got.Behavior)) {
		t.Fatalf("behavior JSON did not round-trip:\nwant %s\ngot  %s", sampleBehavior, got.Behavior)
	}
	if !sameJSON(sampleScaling, string(got.Scaling)) {
		t.Fatalf("scaling JSON did not round-trip:\nwant %s\ngot  %s", sampleScaling, got.Scaling)
	}
}

// TestNPCTemplateMigrationIsIdempotent opens the same database file several
// times; every open re-runs the FASE 5.3 migration and must succeed without
// data loss.
func TestNPCTemplateMigrationIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "npc_templates-idempotent.db")

	client, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	factory := NewSQLiteFactory(client)
	created, err := factory.NPCTemplates().Create(&npc.NPCTemplate{
		Name:       "Persistent",
		Role:       "enemy",
		IsTemplate: true,
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

	got, err := factory2.NPCTemplates().FindByID(created.ID)
	if err != nil {
		t.Fatalf("find after reopen: %v", err)
	}
	if got.Name != "Persistent" {
		t.Fatalf("data lost across reopen: %+v", got)
	}

	// A third open confirms repeated migrations stay no-ops.
	client3, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("third open: %v", err)
	}
	t.Cleanup(func() { _ = client3.Close() })
}

// TestNPCTemplateFindByRoleFilters verifies role-based filtering returns only
// matching templates.
func TestNPCTemplateFindByRoleFilters(t *testing.T) {
	repo := newNPCTemplateTestFactory(t).NPCTemplates()

	roles := []string{"enemy", "boss", "merchant"}
	for _, role := range roles {
		_, err := repo.Create(&npc.NPCTemplate{
			Name:       "Template " + role,
			Role:       role,
			IsTemplate: true,
		})
		if err != nil {
			t.Fatalf("create %s: %v", role, err)
		}
	}

	enemies, err := repo.FindByRole("enemy")
	if err != nil {
		t.Fatalf("find enemy: %v", err)
	}
	if len(enemies) != 1 || enemies[0].Role != "enemy" {
		t.Fatalf("unexpected enemies: %+v", enemies)
	}

	// A role with no templates returns an empty slice, not an error.
	none, err := repo.FindByRole("nonexistent")
	if err != nil {
		t.Fatalf("find nonexistent: %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("expected empty slice, got %d", len(none))
	}
}
