package npc

import (
	"encoding/json"
	"time"

	"github.com/talesmud/talesmud/pkg/entities"
)

// NPCTemplate is a persisted NPC/mob blueprint authored in the Creator
// (FASE 5.3). It mirrors the shape emitted by the frontend npcFactory.js
// (public/mud-client/src/game/npcFactory.js) so the Creator can push/pull
// templates losslessly via the best-effort REST helpers in api/npcs.js.
//
// Loot, Behavior and Scaling are stored as raw JSON (json.RawMessage) so the
// exact frontend payload round-trips byte-for-byte, mirroring the dungeons
// repository pattern (pkg/entities/dungeons/dungeon.go).
type NPCTemplate struct {
	*entities.Entity `json:",inline"`

	Name         string `json:"name"`
	Description  string `json:"description"`
	Role         string `json:"role"`
	CreatureType string `json:"creatureType"`
	CombatStyle  string `json:"combatStyle"`
	Difficulty   string `json:"difficulty"`

	Level     int32 `json:"level"`
	MaxHp     int32 `json:"maxHp"`
	DamageMin int32 `json:"damageMin"`
	DamageMax int32 `json:"damageMax"`

	Aggressive  bool  `json:"aggressive"`
	AggroRadius int32 `json:"aggroRadius"`

	XpReward int32 `json:"xpReward"`
	GoldMin  int32 `json:"goldMin"`
	GoldMax  int32 `json:"goldMax"`

	LootTableID string          `json:"lootTableId,omitempty"`
	Loot        json.RawMessage `json:"loot,omitempty"`

	SpawnRoomId    string `json:"spawnRoomId,omitempty"`
	SpawnCount     int32  `json:"spawnCount"`
	RespawnSeconds int32  `json:"respawnSeconds"`

	// Behavior is the raw npcBehavior.js payload (stateMachine, aggro, abilities).
	Behavior json.RawMessage `json:"behavior,omitempty"`
	// Scaling is the raw npcBehavior.js scaling bookkeeping (targetLevel).
	Scaling json.RawMessage `json:"scaling,omitempty"`

	// LLM enables opt-in free-form dialogue for this template.
	LLM *LLMConfig `json:"llm,omitempty"`

	IsTemplate bool `json:"isTemplate"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
