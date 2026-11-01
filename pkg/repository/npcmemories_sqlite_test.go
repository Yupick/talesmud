package repository

import (
	"testing"
	"time"

	e "github.com/talesmud/talesmud/pkg/entities"
)

func TestNPCMemoryUpsertGet(t *testing.T) {
	repo := newSocialTestFactory(t).NPCMemories()

	// No memory yet: Get returns (nil, nil), not an error.
	got, err := repo.Get("npc-warden", "char-legacy")
	if err != nil {
		t.Fatalf("get absent memory: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil memory, got %+v", got)
	}

	memory, err := repo.Upsert(&NPCMemory{
		NPCID:        "npc-warden",
		CharacterID:  "char-legacy",
		Summary:      "The warden distrusts the player.",
		Facts:        []string{"stole a coin", "fled the dungeon"},
		Turns:        3,
		LastUserLine: "I mean no harm",
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if memory.ID != MemoryID("npc-warden", "char-legacy") {
		t.Fatalf("expected composite id, got %q", memory.ID)
	}
	if memory.Updated.IsZero() {
		t.Fatal("expected Updated to be stamped")
	}

	got, err = repo.Get("npc-warden", "char-legacy")
	if err != nil {
		t.Fatalf("get after upsert: %v", err)
	}
	if got == nil {
		t.Fatal("expected stored memory, got nil")
	}
	if got.NPCID != "npc-warden" || got.CharacterID != "char-legacy" {
		t.Fatalf("unexpected pair: %+v", got)
	}
	if got.Summary != "The warden distrusts the player." {
		t.Fatalf("unexpected summary: %q", got.Summary)
	}
	if len(got.Facts) != 2 || got.Facts[0] != "stole a coin" {
		t.Fatalf("unexpected facts: %v", got.Facts)
	}
	if got.Turns != 3 || got.LastUserLine != "I mean no harm" {
		t.Fatalf("unexpected fields: %+v", got)
	}
	if got.Updated.IsZero() {
		t.Fatal("expected Updated persisted")
	}
}

func TestNPCMemoryUpsertUpdatesExisting(t *testing.T) {
	repo := newSocialTestFactory(t).NPCMemories()

	if _, err := repo.Upsert(&NPCMemory{
		NPCID:       "npc-warden",
		CharacterID: "char-a",
		Summary:     "first summary",
		Turns:       1,
	}); err != nil {
		t.Fatalf("initial upsert: %v", err)
	}

	// Second upsert on the same pair replaces the row (no duplicates).
	updated, err := repo.Upsert(&NPCMemory{
		NPCID:       "npc-warden",
		CharacterID: "char-a",
		Summary:     "revised summary",
		Facts:       []string{"returned the coin"},
		Turns:       5,
	})
	if err != nil {
		t.Fatalf("update upsert: %v", err)
	}
	if updated.ID != MemoryID("npc-warden", "char-a") {
		t.Fatalf("id must stay the composite key, got %q", updated.ID)
	}

	got, err := repo.Get("npc-warden", "char-a")
	if err != nil || got == nil {
		t.Fatalf("get: %v memory=%+v", err, got)
	}
	if got.Summary != "revised summary" || got.Turns != 5 {
		t.Fatalf("expected updated fields: %+v", got)
	}
	if len(got.Facts) != 1 || got.Facts[0] != "returned the coin" {
		t.Fatalf("expected replaced facts: %v", got.Facts)
	}

	// Update with explicit pair arguments behaves the same.
	if err := repo.Update("npc-warden", "char-a", &NPCMemory{
		Entity:       &e.Entity{ID: "ignored-stale-id"},
		Summary:      "via Update",
		LastUserLine: "hello",
	}); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ = repo.Get("npc-warden", "char-a")
	if got.Summary != "via Update" || got.ID != MemoryID("npc-warden", "char-a") {
		t.Fatalf("unexpected update result: %+v", got)
	}
}

func TestNPCMemoryUpsertValidation(t *testing.T) {
	repo := newSocialTestFactory(t).NPCMemories()

	if _, err := repo.Upsert(nil); err == nil {
		t.Fatal("expected nil memory to fail")
	}
	if _, err := repo.Upsert(&NPCMemory{CharacterID: "char-a"}); err == nil {
		t.Fatal("expected missing npcId to fail")
	}
	if _, err := repo.Upsert(&NPCMemory{NPCID: "npc-warden"}); err == nil {
		t.Fatal("expected missing characterId to fail")
	}
	if _, err := repo.Get("", "char-a"); err == nil {
		t.Fatal("expected get without npcId to fail")
	}
	if err := repo.Delete("", "char-a"); err == nil {
		t.Fatal("expected delete without npcId to fail")
	}
	if err := repo.DeleteForNPC(""); err == nil {
		t.Fatal("expected deleteForNPC without npcId to fail")
	}
}

func TestNPCMemoryFindBy(t *testing.T) {
	repo := newSocialTestFactory(t).NPCMemories()

	// Two characters with the warden, one with another NPC.
	if _, err := repo.Upsert(&NPCMemory{NPCID: "npc-warden", CharacterID: "char-a", Summary: "a", Turns: 1}); err != nil {
		t.Fatalf("upsert a: %v", err)
	}
	if _, err := repo.Upsert(&NPCMemory{NPCID: "npc-warden", CharacterID: "char-b", Summary: "b", Turns: 2}); err != nil {
		t.Fatalf("upsert b: %v", err)
	}
	if _, err := repo.Upsert(&NPCMemory{NPCID: "npc-merchant", CharacterID: "char-a", Summary: "m", Turns: 1}); err != nil {
		t.Fatalf("upsert merchant: %v", err)
	}

	byNPC, err := repo.FindByNPC("npc-warden")
	if err != nil || len(byNPC) != 2 {
		t.Fatalf("find by npc: %v len=%d", err, len(byNPC))
	}
	// Most recently updated first: char-b was stored after char-a.
	if byNPC[0].CharacterID != "char-b" || byNPC[1].CharacterID != "char-a" {
		t.Fatalf("unexpected order: %s, %s", byNPC[0].CharacterID, byNPC[1].CharacterID)
	}

	byChar, err := repo.FindByCharacter("char-a")
	if err != nil || len(byChar) != 2 {
		t.Fatalf("find by character: %v len=%d", err, len(byChar))
	}

	if _, err := repo.FindByNPC(""); err == nil {
		t.Fatal("expected find by npc without id to fail")
	}
	if _, err := repo.FindByCharacter(""); err == nil {
		t.Fatal("expected find by character without id to fail")
	}
}

func TestNPCMemoryDelete(t *testing.T) {
	repo := newSocialTestFactory(t).NPCMemories()

	if _, err := repo.Upsert(&NPCMemory{NPCID: "npc-warden", CharacterID: "char-a", Summary: "bye"}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	if err := repo.Delete("npc-warden", "char-a"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	got, err := repo.Get("npc-warden", "char-a")
	if err != nil || got != nil {
		t.Fatalf("expected memory gone: %v memory=%+v", err, got)
	}

	// Deleting again reports the missing row (same contract as dm DeleteMessage).
	if err := repo.Delete("npc-warden", "char-a"); err == nil {
		t.Fatal("expected deleting absent memory to fail")
	}
}

func TestNPCMemoryDeleteForNPC(t *testing.T) {
	repo := newSocialTestFactory(t).NPCMemories()

	if _, err := repo.Upsert(&NPCMemory{NPCID: "npc-warden", CharacterID: "char-a"}); err != nil {
		t.Fatalf("upsert a: %v", err)
	}
	if _, err := repo.Upsert(&NPCMemory{NPCID: "npc-warden", CharacterID: "char-b"}); err != nil {
		t.Fatalf("upsert b: %v", err)
	}
	if _, err := repo.Upsert(&NPCMemory{NPCID: "npc-merchant", CharacterID: "char-a"}); err != nil {
		t.Fatalf("upsert merchant: %v", err)
	}

	if err := repo.DeleteForNPC("npc-warden"); err != nil {
		t.Fatalf("delete for npc: %v", err)
	}

	byNPC, _ := repo.FindByNPC("npc-warden")
	if len(byNPC) != 0 {
		t.Fatalf("expected warden memories gone, got %d", len(byNPC))
	}
	// Other NPCs are untouched.
	merchant, err := repo.Get("npc-merchant", "char-a")
	if err != nil || merchant == nil {
		t.Fatalf("expected merchant memory intact: %v memory=%+v", err, merchant)
	}

	// Deleting an NPC with no memories is not an error.
	if err := repo.DeleteForNPC("npc-ghost"); err != nil {
		t.Fatalf("delete for npc without memories: %v", err)
	}
}

func TestNPCMemoryDrop(t *testing.T) {
	repo := newSocialTestFactory(t).NPCMemories()

	if _, err := repo.Upsert(&NPCMemory{NPCID: "npc-warden", CharacterID: "char-a"}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := repo.Drop(); err != nil {
		t.Fatalf("drop: %v", err)
	}
	got, err := repo.Get("npc-warden", "char-a")
	if err != nil || got != nil {
		t.Fatalf("expected table emptied: %v memory=%+v", err, got)
	}
}

// TestNPCMemoryTimestampsAreIndependent guards the Updated ordering used by
// FindByNPC/FindByCharacter: two upserts in the same millisecond must still
// round-trip their timestamps.
func TestNPCMemoryTimestampsAreIndependent(t *testing.T) {
	repo := newSocialTestFactory(t).NPCMemories()

	base := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	m1, err := repo.Upsert(&NPCMemory{NPCID: "npc-x", CharacterID: "char-1", Updated: base})
	if err != nil {
		t.Fatalf("upsert m1: %v", err)
	}
	m2, err := repo.Upsert(&NPCMemory{NPCID: "npc-x", CharacterID: "char-2", Updated: base.Add(time.Hour)})
	if err != nil {
		t.Fatalf("upsert m2: %v", err)
	}
	if !m1.Updated.Equal(base) || !m2.Updated.Equal(base.Add(time.Hour)) {
		t.Fatalf("timestamps not preserved: %v / %v", m1.Updated, m2.Updated)
	}
}

var _ NPCMemoriesRepository = (*sqliteNPCMemoryRepository)(nil)
