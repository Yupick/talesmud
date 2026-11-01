package importer_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/talesmud/talesmud/pkg/contenthealth"
	"github.com/talesmud/talesmud/pkg/importer"
	"github.com/talesmud/talesmud/pkg/service"
)

// findAdventureRoot locates the import/adventure pack from the package working
// directory, walking up until the folder is found.
func findAdventureRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		candidate := filepath.Join(dir, "import", "adventure")
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatal("import/adventure not found from " + dir)
	return ""
}

func adventureWorld(t *testing.T) contenthealth.World {
	t.Helper()
	converted, err := importer.LoadConverted(findAdventureRoot(t))
	if err != nil {
		t.Fatalf("LoadConverted: %v", err)
	}
	return contenthealth.World{
		StartRoomID: service.DefaultStartRoomID,
		Rooms:       converted.Rooms,
		NPCs:        converted.NPCs,
		Items:       converted.Items,
		LootTables:  converted.LootTables,
		Spawners:    converted.Spawners,
		Dialogs:     converted.Dialogs,
		Quests:      converted.Quests,
		Scripts:     converted.Scripts,
		Skills:      converted.Skills,
		Classes:     contenthealth.CatalogClasses(),
	}
}

func TestAdventurePackLoads(t *testing.T) {
	w := adventureWorld(t)
	if len(w.Rooms) < 30 {
		t.Errorf("rooms = %d, want >= 30", len(w.Rooms))
	}
	if len(w.Items) < 20 {
		t.Errorf("items = %d, want >= 20", len(w.Items))
	}
	if len(w.NPCs) < 15 {
		t.Errorf("npcs = %d, want >= 15", len(w.NPCs))
	}
	if len(w.LootTables) < 5 {
		t.Errorf("loot tables = %d, want >= 5", len(w.LootTables))
	}
	if len(w.Spawners) < 5 {
		t.Errorf("spawners = %d, want >= 5", len(w.Spawners))
	}
	if len(w.Scripts) == 0 {
		t.Error("scripts = 0, want >= 1")
	}
}

func TestAdventurePackNoDuplicateIDs(t *testing.T) {
	w := adventureWorld(t)
	rooms := map[string]bool{}
	for _, r := range w.Rooms {
		if rooms[r.ID] {
			t.Errorf("duplicate room id %s", r.ID)
		}
		rooms[r.ID] = true
	}
	items := map[string]bool{}
	for _, i := range w.Items {
		if items[i.ID] {
			t.Errorf("duplicate item id %s", i.ID)
		}
		items[i.ID] = true
	}
	npcs := map[string]bool{}
	for _, n := range w.NPCs {
		if npcs[n.ID] {
			t.Errorf("duplicate npc id %s", n.ID)
		}
		npcs[n.ID] = true
	}
	tables := map[string]bool{}
	for _, lt := range w.LootTables {
		if tables[lt.ID] {
			t.Errorf("duplicate loot table id %s", lt.ID)
		}
		tables[lt.ID] = true
	}
	spawners := map[string]bool{}
	for _, s := range w.Spawners {
		if spawners[s.ID] {
			t.Errorf("duplicate spawner id %s", s.ID)
		}
		spawners[s.ID] = true
	}
	scripts := map[string]bool{}
	for _, s := range w.Scripts {
		if scripts[s.ID] {
			t.Errorf("duplicate script id %s", s.ID)
		}
		scripts[s.ID] = true
	}
	quests := map[string]bool{}
	for _, q := range w.Quests {
		if quests[q.ID] {
			t.Errorf("duplicate quest id %s", q.ID)
		}
		quests[q.ID] = true
	}
}

func TestAdventurePackHealth(t *testing.T) {
	w := adventureWorld(t)
	report := contenthealth.Run(w, contenthealth.Options{})
	if report.Summary.Errors > 0 {
		for _, rule := range report.Rules {
			for _, hit := range rule.Hits {
				t.Errorf("ERROR %s: %s (%s): %s", rule.ID, hit.EntityType, hit.EntityID, hit.Message)
			}
		}
	}
	if report.Summary.Warnings > 0 {
		for _, rule := range report.Rules {
			for _, hit := range rule.Hits {
				t.Logf("WARNING %s: %s (%s): %s", rule.ID, hit.EntityType, hit.EntityID, hit.Message)
			}
		}
	}
	if report.Summary.Warnings > 0 {
		t.Errorf("health report has %d warnings, want 0", report.Summary.Warnings)
	}
}

func TestAdventurePackReachability(t *testing.T) {
	w := adventureWorld(t)
	ix := w.Graph()
	reach := ix.Reachability()
	if len(reach.Islands) > 0 {
		for _, island := range reach.Islands {
			t.Errorf("unreachable island (%s): %v", island.Reason, island.RoomIDs)
		}
	}
	// Every authored room must be reachable from the start room.
	for _, r := range w.Rooms {
		if !reach.Reachable[r.ID] {
			t.Errorf("room %s (%s) is not reachable from %s", r.ID, r.Name, service.DefaultStartRoomID)
		}
	}
}

func TestAdventurePackStartRoomExists(t *testing.T) {
	w := adventureWorld(t)
	var found bool
	for _, r := range w.Rooms {
		if r.ID == service.DefaultStartRoomID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("start room %s is missing from the pack", service.DefaultStartRoomID)
	}
}
