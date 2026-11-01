package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	dbsqlite "github.com/talesmud/talesmud/pkg/db/sqlite"
	e "github.com/talesmud/talesmud/pkg/entities"
)

// sqliteNPCMemoryRepository persists long-term NPC memories in the
// npc_memories table (id, data JSON), following the exact pattern of
// sqliteDMChannelRepository (dm_sqlite.go).
//
// The table key is the deterministic composite "<npcTemplateID>::<characterID>"
// (see MemoryID), so a missing row simply means the NPC has no memory of that
// character yet — no legacy backfill is needed.
type sqliteNPCMemoryRepository struct {
	db *sql.DB
}

// NewSQLiteNPCMemoryRepository creates a new SQLite-backed NPC memory repository.
func NewSQLiteNPCMemoryRepository(client *dbsqlite.Client) NPCMemoriesRepository {
	return &sqliteNPCMemoryRepository{db: client.DB()}
}

func (repo *sqliteNPCMemoryRepository) scanMemory(rows *sql.Rows) ([]*NPCMemory, error) {
	out := make([]*NPCMemory, 0)
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		memory := &NPCMemory{}
		if err := json.Unmarshal([]byte(payload), memory); err != nil {
			continue
		}
		out = append(out, memory)
	}
	return out, rows.Err()
}

func (repo *sqliteNPCMemoryRepository) findByID(id string) (*NPCMemory, error) {
	row := repo.db.QueryRow(`SELECT data FROM npc_memories WHERE id = ?`, id)
	var payload string
	if err := row.Scan(&payload); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // No memory yet for this pair — not an error.
		}
		return nil, err
	}
	memory := &NPCMemory{}
	if err := json.Unmarshal([]byte(payload), memory); err != nil {
		return nil, err
	}
	return memory, nil
}

func (repo *sqliteNPCMemoryRepository) store(m *NPCMemory) error {
	payload, err := json.Marshal(m)
	if err != nil {
		return err
	}
	_, err = repo.db.Exec(
		`INSERT INTO npc_memories (id, data) VALUES (?, ?)
		 ON CONFLICT(id) DO UPDATE SET data = excluded.data`,
		m.ID,
		string(payload),
	)
	return err
}

// ---------- CRUD ----------

func (repo *sqliteNPCMemoryRepository) Drop() error {
	_, err := repo.db.Exec(`DELETE FROM npc_memories`)
	return err
}

// Get returns the memory of npcID about characterID, or (nil, nil) when the
// NPC has no memory of that character yet.
func (repo *sqliteNPCMemoryRepository) Get(npcID, characterID string) (*NPCMemory, error) {
	if npcID == "" || characterID == "" {
		return nil, errors.New("npcId and characterId are required")
	}
	return repo.findByID(MemoryID(npcID, characterID))
}

// Upsert creates or replaces the memory of npcID about characterID. The
// storage key is always derived from the pair (MemoryID), stamping Entity
// and Updated when missing, so callers cannot orphan a row under a stale id.
func (repo *sqliteNPCMemoryRepository) Upsert(m *NPCMemory) (*NPCMemory, error) {
	if m == nil {
		return nil, errors.New("memory is nil")
	}
	if m.NPCID == "" || m.CharacterID == "" {
		return nil, errors.New("memory requires npcId and characterId")
	}
	if m.Entity == nil {
		m.Entity = e.NewEntity()
	}
	m.ID = MemoryID(m.NPCID, m.CharacterID)
	if m.Updated.IsZero() {
		m.Updated = time.Now()
	}
	if err := repo.store(m); err != nil {
		return nil, err
	}
	return m, nil
}

// Update replaces the memory stored under the pair's key. It is Upsert with
// explicit semantics for callers that fetched, mutated and saved back.
func (repo *sqliteNPCMemoryRepository) Update(npcID, characterID string, m *NPCMemory) error {
	if m == nil {
		return errors.New("memory is nil")
	}
	m.NPCID = npcID
	m.CharacterID = characterID
	if m.Entity == nil {
		m.Entity = e.NewEntity()
	}
	m.ID = MemoryID(npcID, characterID)
	if m.Updated.IsZero() {
		m.Updated = time.Now()
	}
	return repo.store(m)
}

// Delete removes the memory of npcID about characterID. It returns an error
// when no such memory exists (same contract as DeleteMessage in dm_sqlite.go).
func (repo *sqliteNPCMemoryRepository) Delete(npcID, characterID string) error {
	if npcID == "" || characterID == "" {
		return errors.New("npcId and characterId are required")
	}
	result, err := repo.db.Exec(`DELETE FROM npc_memories WHERE id = ?`, MemoryID(npcID, characterID))
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("npc memory not found for npc %s character %s", npcID, characterID)
	}
	return nil
}

// DeleteForNPC forgets every memory about any character kept for npcID
// (admin operation). Deleting zero rows is not an error.
func (repo *sqliteNPCMemoryRepository) DeleteForNPC(npcID string) error {
	if npcID == "" {
		return errors.New("npcId is required")
	}
	_, err := repo.db.Exec(
		`DELETE FROM npc_memories WHERE json_extract(data, '$.npcId') = ?`,
		npcID,
	)
	return err
}

// ---------- List ----------

// FindByNPC returns every memory kept for npcID, most recently updated first.
func (repo *sqliteNPCMemoryRepository) FindByNPC(npcID string) ([]*NPCMemory, error) {
	if npcID == "" {
		return nil, errors.New("npcId is required")
	}
	rows, err := repo.db.Query(
		`SELECT data FROM npc_memories
		 WHERE json_extract(data, '$.npcId') = ?
		 ORDER BY json_extract(data, '$.updated') DESC`,
		npcID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return repo.scanMemory(rows)
}

// FindByCharacter returns every memory any NPC keeps about characterID, most
// recently updated first.
func (repo *sqliteNPCMemoryRepository) FindByCharacter(characterID string) ([]*NPCMemory, error) {
	if characterID == "" {
		return nil, errors.New("characterId is required")
	}
	rows, err := repo.db.Query(
		`SELECT data FROM npc_memories
		 WHERE json_extract(data, '$.characterId') = ?
		 ORDER BY json_extract(data, '$.updated') DESC`,
		characterID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return repo.scanMemory(rows)
}
