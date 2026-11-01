// Package dungeons holds the entities backing the FASE 5.1 dungeon builder:
// the persisted dungeon definition produced by the frontend exportDefinition
// helper (public/mud-client/src/game/dungeonBuilder.js).
package dungeons

import (
	"encoding/json"
	"time"

	"github.com/talesmud/talesmud/pkg/entities"
)

// DungeonDefinition is a persisted dungeon design authored in the dungeon
// builder. Definition stores the exact JSON emitted by the frontend
// exportDefinition helper (version, grid, cells, rooms, corridors, entrances,
// exits, encounters, lootTables, meta) and is kept verbatim so the builder can
// reload and re-edit it losslessly.
type DungeonDefinition struct {
	*entities.Entity `json:",inline"`

	Name        string `json:"name"`
	Description string `json:"description"`

	// GridWidth/GridHeight mirror definition.grid for fast listing/filtering
	// without parsing the JSON payload.
	GridWidth  int `json:"gridWidth"`
	GridHeight int `json:"gridHeight"`

	// Definition is the raw exportDefinition JSON. json.RawMessage preserves
	// the payload byte-for-byte through marshal/unmarshal.
	Definition json.RawMessage `json:"definition"`

	Author string   `json:"author"`
	Tags   []string `json:"tags,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
