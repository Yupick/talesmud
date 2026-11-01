package repository

import (
	"time"

	"github.com/talesmud/talesmud/pkg/entities"
)

// NPCMemory is the long-term memory an LLM-driven NPC keeps about one
// character (docs/design/PHASE6_LLM.md §6.3).
//
// It is the Phase 6 (Corte B) counterpart of the dm.Channel/dm.Message
// entities: the phase design places it in pkg/entities/npcs/memory.go as
// npcs.Memory, but it lives here for now because this cut only touches
// pkg/db and pkg/repository. The JSON field names are kept identical to the
// design so the type can be moved to pkg/entities/npcs without changes.
//
// The storage key is the deterministic composite "<npcTemplateID>::<characterID>",
// so a memory row always belongs to exactly one (npc, character) pair.
type NPCMemory struct {
	*entities.Entity `json:",inline"` // ID = "<npcTemplateID>::<characterID>"

	NPCID        string   `json:"npcId"`           // npcTemplateID the memory belongs to
	CharacterID  string   `json:"characterId"`     // character the memory is about
	Summary      string   `json:"summary"`         // 2-4 sentence summary of the relationship
	Facts        []string `json:"facts,omitempty"` // key facts: "killed the Warden", "owes 5 coins"
	Turns        int      `json:"turns"`           // recorded dialogue turns
	LastUserLine string   `json:"lastUserLine,omitempty"`
	// Relations stores the NPC's dynamic sentiment toward targets that have
	// interacted with it (normally "player"). It is keyed by target and
	// clamped to [-1,1]. Like SummarySource it lives inside the JSON payload,
	// so it needs no migration.
	Relations map[string]float64 `json:"relations,omitempty"`

	// SummarySource records who wrote Summary. An empty value means a legacy
	// row; "deterministic" is rebuilt by RecordTurn, while "llm" is written by
	// the async summariser and must never be clobbered by the deterministic
	// rebuild. It is stored inside the JSON payload, so no migration is needed.
	SummarySource string    `json:"summarySource,omitempty"`
	Updated       time.Time `json:"updated"`
}

// Summary provenance markers.
const (
	NPCMemorySummaryDeterministic = "deterministic"
	NPCMemorySummaryLLM           = "llm"
)

// MemoryID returns the deterministic storage key for a (npc, character) pair:
// "<npcTemplateID>::<characterID>".
func MemoryID(npcID, characterID string) string {
	return npcID + "::" + characterID
}
