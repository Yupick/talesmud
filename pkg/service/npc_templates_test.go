package service

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/talesmud/talesmud/pkg/db/sqlite"
	npc "github.com/talesmud/talesmud/pkg/entities/npcs"
	"github.com/talesmud/talesmud/pkg/repository"
)

func newNPCTemplatesTestService(t *testing.T) NPCTemplatesService {
	t.Helper()
	client, err := sqlite.Open(filepath.Join(t.TempDir(), "npc-templates-service.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return NewNPCTemplatesService(repository.NewSQLiteFactory(client).NPCTemplates())
}

func validNPCTemplate(name string) *npc.NPCTemplate {
	return &npc.NPCTemplate{
		Name:           name,
		Description:    "An animated bones soldier",
		Role:           "enemy",
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
		Loot:           json.RawMessage(`[{"itemId":"item_bone","chance":80,"quantity":1}]`),
		SpawnRoomId:    "room_crypt_1",
		SpawnCount:     3,
		RespawnSeconds: 60,
		Behavior:       json.RawMessage(`{"stateMachine":[]}`),
		Scaling:        json.RawMessage(`{"targetLevel":null}`),
	}
}

func TestValidateNPCTemplate(t *testing.T) {
	base := func() *npc.NPCTemplate { return validNPCTemplate("Skeleton") }

	tests := []struct {
		name    string
		mutate  func(*npc.NPCTemplate)
		wantErr bool
	}{
		{name: "valid", mutate: func(*npc.NPCTemplate) {}},
		{name: "nil", mutate: nil, wantErr: true},
		{name: "empty name", mutate: func(tpl *npc.NPCTemplate) { tpl.Name = "" }, wantErr: true},
		{name: "blank name", mutate: func(tpl *npc.NPCTemplate) { tpl.Name = "   " }, wantErr: true},
		{name: "level zero", mutate: func(tpl *npc.NPCTemplate) { tpl.Level = 0 }, wantErr: true},
		{name: "maxHp zero", mutate: func(tpl *npc.NPCTemplate) { tpl.MaxHp = 0 }, wantErr: true},
		{name: "negative damageMin", mutate: func(tpl *npc.NPCTemplate) { tpl.DamageMin = -1 }, wantErr: true},
		{name: "negative damageMax", mutate: func(tpl *npc.NPCTemplate) { tpl.DamageMax = -1 }, wantErr: true},
		{name: "damageMax below damageMin", mutate: func(tpl *npc.NPCTemplate) {
			tpl.DamageMin, tpl.DamageMax = 10, 3
		}, wantErr: true},
		{name: "goldMax below goldMin", mutate: func(tpl *npc.NPCTemplate) {
			tpl.GoldMin, tpl.GoldMax = 30, 5
		}, wantErr: true},
		{name: "invalid loot json", mutate: func(tpl *npc.NPCTemplate) {
			tpl.Loot = json.RawMessage(`{not json`)
		}, wantErr: true},
		{name: "behavior json array allowed", mutate: func(tpl *npc.NPCTemplate) {
			tpl.Behavior = json.RawMessage(`[]`)
		}},
		{name: "invalid behavior text", mutate: func(tpl *npc.NPCTemplate) {
			tpl.Behavior = json.RawMessage(`oops`)
		}, wantErr: true},
		{name: "invalid scaling json", mutate: func(tpl *npc.NPCTemplate) {
			tpl.Scaling = json.RawMessage(`{`)
		}, wantErr: true},
		{name: "empty raw payloads allowed", mutate: func(tpl *npc.NPCTemplate) {
			tpl.Loot, tpl.Behavior, tpl.Scaling = nil, nil, nil
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var tpl *npc.NPCTemplate
			if tt.mutate != nil {
				tpl = base()
				tt.mutate(tpl)
			}
			err := ValidateNPCTemplate(tpl)
			if tt.wantErr && err == nil {
				t.Fatal("expected validation error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestNPCTemplatesServiceLifecycle(t *testing.T) {
	svc := newNPCTemplatesTestService(t)

	created, err := svc.Create(validNPCTemplate("Skeleton Warrior"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.ID == "" || !created.IsTemplate {
		t.Fatalf("expected generated id and template flag: %+v", created)
	}
	if created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Fatalf("expected timestamps: %+v", created)
	}

	// Client-supplied ids are ignored on create.
	forged := validNPCTemplate("Forged")
	forged.Entity = nil
	if _, err := svc.Create(forged); err != nil {
		t.Fatalf("create forged: %v", err)
	}

	got, err := svc.FindByID(created.ID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.Name != "Skeleton Warrior" || string(got.Loot) == "" {
		t.Fatalf("unexpected stored template: %+v", got)
	}

	// List filters.
	all, err := svc.List(NPCTemplateListFilter{})
	if err != nil || len(all) != 2 {
		t.Fatalf("list all: %d %v", len(all), err)
	}
	limited, err := svc.List(NPCTemplateListFilter{Limit: 1})
	if err != nil || len(limited) != 1 {
		t.Fatalf("list limited: %d %v", len(limited), err)
	}
	if byName, _ := svc.List(NPCTemplateListFilter{Name: "Skeleton Warrior"}); len(byName) != 1 {
		t.Fatalf("list by name: %v", byName)
	}
	if byRole, _ := svc.List(NPCTemplateListFilter{Role: "enemy"}); len(byRole) != 2 {
		t.Fatalf("list by role: %v", byRole)
	}
	if both, _ := svc.List(NPCTemplateListFilter{Name: "Skeleton Warrior", Role: "boss"}); len(both) != 0 {
		t.Fatalf("list contradictory: %v", both)
	}

	// Update preserves CreatedAt and replaces data.
	update := validNPCTemplate("Skeleton Lord")
	update.Level = 9
	if err := svc.Update(created.ID, update); err != nil {
		t.Fatalf("update: %v", err)
	}
	reloaded, err := svc.FindByID(created.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Name != "Skeleton Lord" || reloaded.Level != 9 {
		t.Fatalf("update not stored: %+v", reloaded)
	}
	if !reloaded.CreatedAt.Equal(created.CreatedAt) {
		t.Fatalf("CreatedAt not preserved: %v != %v", reloaded.CreatedAt, created.CreatedAt)
	}

	if err := svc.Update("missing", update); err == nil {
		t.Fatal("expected update of unknown id to fail")
	}

	if err := svc.Delete(created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := svc.FindByID(created.ID); err == nil {
		t.Fatal("expected deleted template to be gone")
	}
	if err := svc.Delete(created.ID); err == nil {
		t.Fatal("expected second delete to fail")
	}
}

func TestSpawnFromTemplateInheritsLLM(t *testing.T) {
	client, err := sqlite.Open(filepath.Join(t.TempDir(), "npc-spawn.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	svc := NewNPCsService(repository.NewSQLiteFactory(client).NPCs())
	stored, err := svc.Store(&npc.NPC{
		Name:             "Crypt Keeper",
		MaxHitPoints:     40,
		CurrentHitPoints: 40,
		IsTemplate:       true,
		LLM: &npc.LLMConfig{
			Enabled:         true,
			Persona:         "Soy el guardián de la cripta.",
			MemoryTurns:     6,
			CooldownSeconds: 5,
			Goals:           []string{"Proteger la cripta."},
			Secrets:         []string{"El rey duerme bajo el altar."},
			Relations:       []npc.LLMRelation{{Target: "player", Kind: "mentor", Sentiment: 0.7}},
		},
	})
	if err != nil {
		t.Fatalf("store template: %v", err)
	}

	instance, err := svc.SpawnFromTemplate(stored.ID, "room_crypt")
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	if instance.LLM == nil || !instance.LLM.Enabled {
		t.Fatalf("spawned instance did not inherit llm: %+v", instance.LLM)
	}
	if instance.LLM.Persona != stored.LLM.Persona || instance.LLM.MemoryTurns != 6 ||
		instance.LLM.CooldownSeconds != 5 {
		t.Fatalf("spawned llm fields differ: %+v", instance.LLM)
	}
	if instance.LLM == stored.LLM {
		t.Fatal("spawned instance shares the template llm pointer")
	}

	// Mutating the instance must not leak back into the stored template.
	instance.LLM.Goals[0] = "mutated"
	instance.LLM.Relations[0].Kind = "rival"
	reloaded, err := svc.FindByID(stored.ID)
	if err != nil {
		t.Fatalf("reload template: %v", err)
	}
	if reloaded.LLM.Goals[0] != "Proteger la cripta." || reloaded.LLM.Relations[0].Kind != "mentor" {
		t.Fatalf("instance mutated template llm: %+v", reloaded.LLM)
	}
}
