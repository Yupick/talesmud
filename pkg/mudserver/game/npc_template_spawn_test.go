package game

import (
	"strings"
	"testing"
	"time"

	"github.com/talesmud/talesmud/pkg/entities"
	npc "github.com/talesmud/talesmud/pkg/entities/npcs"
	"github.com/talesmud/talesmud/pkg/entities/rooms"
)

func testNPCTemplate() *npc.NPCTemplate {
	return &npc.NPCTemplate{
		Entity:         &entities.Entity{ID: "tpl-skeleton"},
		Name:           "Skeleton Warrior",
		Description:    "An animated bones soldier",
		CreatureType:   "undead",
		CombatStyle:    "melee",
		Difficulty:     "normal",
		Level:          3,
		MaxHp:          45,
		DamageMin:      4,
		DamageMax:      9,
		Aggressive:     true,
		AggroRadius:    2,
		XpReward:       120,
		GoldMin:        5,
		GoldMax:        25,
		LootTableID:    "loot_skeleton",
		SpawnRoomId:    "room-crypt",
		SpawnCount:     3,
		RespawnSeconds: 60,
	}
}

func TestSpawnNPCTemplateBridgesBlueprintToWorld(t *testing.T) {
	g, facade := newNPCTestGame(t)
	storeTestRoom(t, facade, "room-crypt", rooms.Exits{})

	tpl := testNPCTemplate()
	instanceID, name, roomID, err := g.SpawnNPCTemplate(tpl)
	if err != nil {
		t.Fatalf("spawn template: %v", err)
	}
	if instanceID == "" || roomID != "room-crypt" {
		t.Fatalf("unexpected instance result: id=%q name=%q room=%q", instanceID, name, roomID)
	}
	if !strings.HasPrefix(name, tpl.Name+"-") {
		t.Fatalf("expected instance target name with suffix, got %q", name)
	}

	runtimeTemplate, err := facade.NPCsService().FindByID(tpl.ID)
	if err != nil {
		t.Fatalf("runtime template not persisted: %v", err)
	}
	if !runtimeTemplate.IsTemplate {
		t.Fatal("runtime NPC must be flagged IsTemplate")
	}
	if runtimeTemplate.Name != tpl.Name || runtimeTemplate.Level != tpl.Level ||
		runtimeTemplate.MaxHitPoints != tpl.MaxHp || runtimeTemplate.CurrentHitPoints != tpl.MaxHp {
		t.Fatalf("runtime stats not mapped: %+v", runtimeTemplate)
	}
	if runtimeTemplate.SpawnRoomID != "room-crypt" || runtimeTemplate.RespawnTime != 60*time.Second {
		t.Fatalf("runtime spawn fields not mapped: %+v", runtimeTemplate)
	}
	trait := runtimeTemplate.EnemyTrait
	if trait == nil {
		t.Fatal("runtime template must have an EnemyTrait")
	}
	if trait.CreatureType != "undead" || trait.CombatStyle != "melee" || trait.Difficulty != "normal" ||
		trait.AttackPower != 6 || trait.Defense != 0 || trait.AggroRadius != 2 || !trait.AggroOnSight ||
		trait.XPReward != 120 || trait.GoldDrop.Min != 5 || trait.GoldDrop.Max != 25 ||
		trait.LootTableID != "loot_skeleton" {
		t.Fatalf("EnemyTrait not mapped: %+v", trait)
	}

	spawners, err := facade.NPCSpawnersService().FindByTemplate(tpl.ID)
	if err != nil {
		t.Fatalf("find spawners: %v", err)
	}
	if len(spawners) != 1 {
		t.Fatalf("expected exactly one spawner, got %d", len(spawners))
	}
	if spawners[0].RoomID != "room-crypt" || spawners[0].MaxInstances != 3 ||
		spawners[0].InitialCount != 1 || spawners[0].SpawnInterval != 0 ||
		spawners[0].RespawnDelay != 60*time.Second {
		t.Fatalf("spawner not configured from blueprint: %+v", spawners[0])
	}

	instance := g.NPCManager.GetInstance(instanceID)
	if instance == nil {
		t.Fatalf("instance %q not registered with the NPC manager", instanceID)
	}
	if instance.IsTemplate || instance.TemplateID != tpl.ID || instance.CurrentRoomID != "room-crypt" {
		t.Fatalf("instance not spawned from template: %+v", instance)
	}

	// Re-saving the blueprint must refresh the runtime template and reuse the
	// same spawner instead of creating a duplicate.
	tpl.Level = 5
	tpl.MaxHp = 70
	tpl.SpawnCount = 2
	tpl.RespawnSeconds = 30
	secondID, _, secondRoom, err := g.SpawnNPCTemplate(tpl)
	if err != nil {
		t.Fatalf("second spawn template: %v", err)
	}
	if secondID == "" || secondID == instanceID || secondRoom != "room-crypt" {
		t.Fatalf("unexpected second instance: id=%q first=%q room=%q", secondID, instanceID, secondRoom)
	}
	if g.NPCManager.GetInstance(secondID) == nil {
		t.Fatal("second instance not registered")
	}

	runtimeTemplate, err = facade.NPCsService().FindByID(tpl.ID)
	if err != nil {
		t.Fatalf("reload runtime template: %v", err)
	}
	if runtimeTemplate.Level != 5 || runtimeTemplate.MaxHitPoints != 70 {
		t.Fatalf("runtime template stats not refreshed: %+v", runtimeTemplate)
	}

	spawners, err = facade.NPCSpawnersService().FindByTemplate(tpl.ID)
	if err != nil {
		t.Fatalf("reload spawners: %v", err)
	}
	if len(spawners) != 1 {
		t.Fatalf("repeated saves must not duplicate spawners, got %d", len(spawners))
	}
	if spawners[0].MaxInstances != 2 || spawners[0].RespawnDelay != 30*time.Second {
		t.Fatalf("spawner not refreshed: %+v", spawners[0])
	}
}

func TestSpawnNPCTemplateValidation(t *testing.T) {
	g, facade := newNPCTestGame(t)
	storeTestRoom(t, facade, "room-crypt", rooms.Exits{})

	tests := []struct {
		name    string
		tpl     *npc.NPCTemplate
		mutate  func(*Game)
		wantErr string
	}{
		{name: "nil template", tpl: nil, wantErr: "npc template is required"},
		{
			name: "missing id",
			tpl: func() *npc.NPCTemplate {
				tpl := testNPCTemplate()
				tpl.Entity = nil
				return tpl
			}(),
			wantErr: "npc template id is required",
		},
		{
			name: "missing spawn room",
			tpl: func() *npc.NPCTemplate {
				tpl := testNPCTemplate()
				tpl.SpawnRoomId = ""
				return tpl
			}(),
			wantErr: "spawnRoomId is required",
		},
		{
			name: "unknown spawn room",
			tpl: func() *npc.NPCTemplate {
				tpl := testNPCTemplate()
				tpl.SpawnRoomId = "room-missing"
				return tpl
			}(),
			wantErr: "spawn room room-missing not found",
		},
		{
			name:    "missing npc manager",
			tpl:     testNPCTemplate(),
			mutate:  func(g *Game) { g.NPCManager = nil },
			wantErr: "npc manager is unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			game := g
			if tt.mutate != nil {
				game = New(facade)
				tt.mutate(game)
			}
			_, _, _, err := game.SpawnNPCTemplate(tt.tpl)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}
