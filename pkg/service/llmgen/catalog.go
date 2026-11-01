package llmgen

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/talesmud/talesmud/pkg/service/llmprompt"
)

// CatalogRef is a compact reference to a world entity, used both to build
// the catalog prompt block and to drive deterministic fallbacks.
type CatalogRef struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Kind  string `json:"kind,omitempty"`
	Area  string `json:"area,omitempty"`
	Level int    `json:"level,omitempty"`
}

// QuestCatalog is the read-only world view the quest generator reasons about.
// It is fetched fresh per request so server-side edits are picked up.
type QuestCatalog struct {
	Areas   []CatalogRef
	NPCs    []CatalogRef
	Rooms   []CatalogRef
	Items   []CatalogRef
	MaxXP   int
	MaxGold int
}

// BuildCatalog renders the catalog prompt block. Only ids from this block
// are considered valid by validation.
func BuildCatalog(cat QuestCatalog) string {
	var b strings.Builder
	writeGroup := func(title string, refs []CatalogRef) {
		if len(refs) == 0 {
			return
		}
		fmt.Fprintf(&b, "%s (%d):\n", title, len(refs))
		for _, r := range refs {
			fmt.Fprintf(&b, "  - %s", r.ID)
			if r.Name != "" && !strings.EqualFold(r.Name, r.ID) {
				fmt.Fprintf(&b, " (%s)", r.Name)
			}
			if r.Area != "" {
				fmt.Fprintf(&b, " [area:%s]", r.Area)
			}
			if r.Level > 0 {
				fmt.Fprintf(&b, " [lvl:%d]", r.Level)
			}
			b.WriteByte('\n')
		}
	}
	writeGroup("AREAS", cat.Areas)
	writeGroup("NPCS", cat.NPCs)
	writeGroup("ROOMS", cat.Rooms)
	writeGroup("ITEMS", cat.Items)
	return b.String()
}

func catalogRefsVar(vars map[string]any, key string) []CatalogRef {
	raw, ok := vars[key]
	if !ok {
		return nil
	}
	if refs, ok := raw.([]CatalogRef); ok {
		return refs
	}
	return nil
}

// deterministicQuestJSON builds a valid quest envelope from the catalog
// carried in the request vars. It is used when the provider is disabled or
// when the model output cannot be validated at all.
func deterministicQuestJSON(req Request, _ llmprompt.Rendered) (json.RawMessage, error) {
	npcs := catalogRefsVar(req.Vars, "_catalogNPCs")
	rooms := catalogRefsVar(req.Vars, "_catalogRooms")
	area := stringVar(req.Vars, "area")

	src := map[string]any{"type": "auto"}
	turnIn := "anywhere"
	if len(npcs) > 0 {
		src = map[string]any{"type": "npc", "npcId": npcs[0].ID}
		turnIn = "npc:" + npcs[0].ID
	}

	var objectives []map[string]any
	if len(npcs) > 0 {
		objectives = append(objectives, map[string]any{
			"id":          "obj1",
			"type":        "kill",
			"description": fmt.Sprintf("Derrota a %s.", displayName(npcs[0])),
			"targetId":    npcs[0].ID,
			"amount":      1,
		})
	} else if len(rooms) > 0 {
		objectives = append(objectives, map[string]any{
			"id":          "obj1",
			"type":        "visit",
			"description": fmt.Sprintf("Explora %s.", displayName(rooms[0])),
			"targetId":    rooms[0].ID,
			"amount":      1,
		})
	}
	if len(objectives) == 0 {
		return nil, fmt.Errorf("llmgen: catalog has no NPCs or rooms for deterministic quest")
	}

	name := "Una tarea pendiente"
	if area != "" {
		name = "Tarea en " + area
	}
	envelope := map[string]any{
		"quests": []map[string]any{{
			"name":        name,
			"description": "La zona reclama ayuda; alguien debe encargarse.",
			"category":    "side",
			"area":        area,
			"source":      src,
			"turnIn":      turnIn,
			"objectives":  objectives,
			"rewards":     map[string]any{"xp": 50, "gold": 10},
		}},
	}
	return json.Marshal(envelope)
}

func displayName(r CatalogRef) string {
	if strings.TrimSpace(r.Name) != "" {
		return r.Name
	}
	return r.ID
}
