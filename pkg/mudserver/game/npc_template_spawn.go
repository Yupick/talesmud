package game

import (
	"errors"
	"fmt"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/talesmud/talesmud/pkg/entities"
	npc "github.com/talesmud/talesmud/pkg/entities/npcs"
)

// SpawnNPCTemplate bridges the persisted NPC/mob blueprints (npc_templates,
// FASE 5.3) with the runtime world. It makes sure the blueprint has a matching
// runtime NPC marked IsTemplate, keeps exactly one spawner for its spawn room,
// and immediately instantiates it through the live NPC manager.
//
// The operation is intentionally best-effort at the handler boundary: a failed
// instance spawn must not roll back the authored template. Callers that need
// strict behaviour can inspect the returned error.
func (g *Game) SpawnNPCTemplate(tpl *npc.NPCTemplate) (instanceID, name, roomID string, err error) {
	if tpl == nil {
		return "", "", "", errors.New("npc template is required")
	}
	if tpl.Entity == nil || strings.TrimSpace(tpl.ID) == "" {
		return "", "", "", errors.New("npc template id is required")
	}
	roomID = strings.TrimSpace(tpl.SpawnRoomId)
	if roomID == "" {
		return "", "", "", errors.New("spawnRoomId is required")
	}
	if g == nil || g.Facade == nil {
		return "", "", "", errors.New("game facade is unavailable")
	}
	if g.NPCManager == nil {
		return "", "", "", errors.New("npc manager is unavailable")
	}
	if _, err := g.Facade.RoomsService().FindByID(roomID); err != nil {
		return "", "", "", fmt.Errorf("spawn room %s not found: %w", roomID, err)
	}

	if err := g.ensureRuntimeTemplateNPC(tpl, roomID); err != nil {
		return "", "", "", err
	}
	if err := g.ensureNPCTemplateSpawner(tpl, roomID); err != nil {
		return "", "", "", err
	}

	instance, err := g.NPCManager.SpawnInstanceDirect(tpl.ID, roomID)
	if err != nil {
		return "", "", "", fmt.Errorf("spawn npc instance: %w", err)
	}
	if instance == nil || instance.Entity == nil {
		return "", "", "", errors.New("npc manager returned an empty instance")
	}

	log.WithFields(log.Fields{
		"template": tpl.ID,
		"instance": instance.Entity.ID,
		"name":     instance.GetTargetName(),
		"room":     instance.CurrentRoomID,
	}).Info("Spawned NPC template into the world")

	return instance.Entity.ID, instance.GetTargetName(), instance.CurrentRoomID, nil
}

// ensureRuntimeTemplateNPC upserts the runtime NPC row used by
// NPCsService.SpawnFromTemplate. The blueprint id is deliberately reused as the
// runtime template id (the two entities live in different tables).
func (g *Game) ensureRuntimeTemplateNPC(tpl *npc.NPCTemplate, roomID string) error {
	existing, findErr := g.Facade.NPCsService().FindByID(tpl.ID)
	if findErr == nil && existing != nil && existing.IsTemplate {
		applyNPCTemplateStats(existing, tpl, roomID)
		existing.IsTemplate = true
		existing.TemplateID = ""
		if err := g.Facade.NPCsService().Update(tpl.ID, existing); err != nil {
			return fmt.Errorf("update runtime npc template: %w", err)
		}
		return nil
	}

	runtimeTemplate := &npc.NPC{
		Entity:     &entities.Entity{ID: tpl.ID},
		IsTemplate: true,
	}
	applyNPCTemplateStats(runtimeTemplate, tpl, roomID)
	if _, err := g.Facade.NPCsService().Import(runtimeTemplate); err != nil {
		return fmt.Errorf("store runtime npc template: %w", err)
	}
	return nil
}

// applyNPCTemplateStats maps the authored blueprint numbers onto the runtime
// NPC fields that combat and the spawner read. It is idempotent so repeated
// saves simply refresh the runtime template.
func applyNPCTemplateStats(dst *npc.NPC, tpl *npc.NPCTemplate, roomID string) {
	dst.Name = tpl.Name
	dst.Description = tpl.Description
	dst.Level = tpl.Level
	dst.MaxHitPoints = tpl.MaxHp
	dst.CurrentHitPoints = tpl.MaxHp
	dst.SpawnRoomID = roomID
	dst.RespawnTime = time.Duration(tpl.RespawnSeconds) * time.Second
	dst.EnemyTrait = npcTemplateEnemyTrait(tpl)
}

func npcTemplateEnemyTrait(tpl *npc.NPCTemplate) *npc.EnemyTrait {
	return &npc.EnemyTrait{
		CreatureType: tpl.CreatureType,
		CombatStyle:  tpl.CombatStyle,
		Difficulty:   tpl.Difficulty,
		AttackPower:  (tpl.DamageMin + tpl.DamageMax) / 2,
		Defense:      0,
		AggroRadius:  int(tpl.AggroRadius),
		AggroOnSight: tpl.Aggressive,
		XPReward:     int64(tpl.XpReward),
		GoldDrop:     npc.Range{Min: tpl.GoldMin, Max: tpl.GoldMax},
		LootTableID:  strings.TrimSpace(tpl.LootTableID),
	}
}

// ensureNPCTemplateSpawner keeps a single spawner per (template, room) pair so
// restarts and re-saves do not fan out duplicate spawners.
func (g *Game) ensureNPCTemplateSpawner(tpl *npc.NPCTemplate, roomID string) error {
	spawners, err := g.Facade.NPCSpawnersService().FindByTemplate(tpl.ID)
	if err != nil {
		return fmt.Errorf("find npc spawners: %w", err)
	}
	for _, spawner := range spawners {
		if spawner == nil || strings.TrimSpace(spawner.RoomID) != roomID {
			continue
		}
		spawner.MaxInstances = npcTemplateMaxInstances(tpl)
		spawner.RespawnDelay = time.Duration(tpl.RespawnSeconds) * time.Second
		if err := g.Facade.NPCSpawnersService().Update(spawner.ID, spawner); err != nil {
			return fmt.Errorf("update npc spawner: %w", err)
		}
		return nil
	}

	spawner := &npc.NPCSpawner{
		Entity:        entities.NewEntity(),
		Name:          tpl.Name,
		TemplateID:    tpl.ID,
		RoomID:        roomID,
		MaxInstances:  npcTemplateMaxInstances(tpl),
		SpawnInterval: 0,
		InitialCount:  1,
		RespawnDelay:  time.Duration(tpl.RespawnSeconds) * time.Second,
	}
	if _, err := g.Facade.NPCSpawnersService().Store(spawner); err != nil {
		return fmt.Errorf("store npc spawner: %w", err)
	}
	return nil
}

func npcTemplateMaxInstances(tpl *npc.NPCTemplate) int {
	if tpl.SpawnCount < 1 {
		return 1
	}
	return int(tpl.SpawnCount)
}
