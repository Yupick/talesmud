package repository

import (
	"database/sql"
	"errors"
	"time"

	dbpkg "github.com/talesmud/talesmud/pkg/db"
	dbsqlite "github.com/talesmud/talesmud/pkg/db/sqlite"
	e "github.com/talesmud/talesmud/pkg/entities"
	npc "github.com/talesmud/talesmud/pkg/entities/npcs"
)

// sqliteNPCTemplatesRepository persists NPC/mob templates (FASE 5.3) in the
// npc_templates table (id, data JSON), following the repo-wide JSON pattern and
// the timestamped Store/Update contract used by the dungeon repository.
//
// Loot, Behavior and Scaling are stored as raw JSON, so the exact frontend
// payload round-trips without re-encoding.
type sqliteNPCTemplatesRepository struct {
	*sqliteGenericRepo
	db *sql.DB
}

// NewSQLiteNPCTemplatesRepository creates a new SQLite-backed NPC template repository.
func NewSQLiteNPCTemplatesRepository(client *dbsqlite.Client) NPCTemplatesRepository {
	conn := client.DB()
	return &sqliteNPCTemplatesRepository{
		sqliteGenericRepo: newSQLiteGenericRepo(conn, "npc_templates", func() interface{} {
			return &npc.NPCTemplate{}
		}),
		db: conn,
	}
}

// ---------- CRUD ----------

func (repo *sqliteNPCTemplatesRepository) Drop() error {
	return repo.sqliteGenericRepo.DropCollection()
}

// Create persists a new NPC template. It generates an ID and stamps
// CreatedAt/UpdatedAt when missing. As with Store elsewhere, it upserts on the
// entity id, so re-creating with the same id replaces the stored row.
func (repo *sqliteNPCTemplatesRepository) Create(tpl *npc.NPCTemplate) (*npc.NPCTemplate, error) {
	if tpl == nil {
		return nil, errors.New("npc template is nil")
	}
	if tpl.Entity == nil {
		tpl.Entity = e.NewEntity()
	}
	now := time.Now()
	if tpl.CreatedAt.IsZero() {
		tpl.CreatedAt = now
	}
	tpl.UpdatedAt = now
	result, err := repo.sqliteGenericRepo.Store(tpl)
	if err != nil {
		return nil, err
	}
	return result.(*npc.NPCTemplate), nil
}

// Update replaces the template stored under id and refreshes UpdatedAt.
// CreatedAt is preserved by callers that fetch, mutate and save back.
func (repo *sqliteNPCTemplatesRepository) Update(id string, tpl *npc.NPCTemplate) error {
	if tpl == nil {
		return errors.New("npc template is nil")
	}
	if tpl.Entity == nil {
		tpl.Entity = e.NewEntity()
	}
	tpl.Entity.ID = id
	tpl.UpdatedAt = time.Now()
	return repo.sqliteGenericRepo.Update(tpl, id)
}

func (repo *sqliteNPCTemplatesRepository) FindByID(id string) (*npc.NPCTemplate, error) {
	result, err := repo.sqliteGenericRepo.FindByID(id)
	if err != nil {
		return nil, err
	}
	return result.(*npc.NPCTemplate), nil
}

func (repo *sqliteNPCTemplatesRepository) FindAll() ([]*npc.NPCTemplate, error) {
	return collectRepo[npc.NPCTemplate](repo.sqliteGenericRepo, nil)
}

func (repo *sqliteNPCTemplatesRepository) FindByName(name string) ([]*npc.NPCTemplate, error) {
	return collectRepo[npc.NPCTemplate](repo.sqliteGenericRepo, dbpkg.NewQueryParams(
		dbpkg.QueryParam{Key: "name", Value: name},
	))
}

func (repo *sqliteNPCTemplatesRepository) FindByRole(role string) ([]*npc.NPCTemplate, error) {
	return collectRepo[npc.NPCTemplate](repo.sqliteGenericRepo, dbpkg.NewQueryParams(
		dbpkg.QueryParam{Key: "role", Value: role},
	))
}

func (repo *sqliteNPCTemplatesRepository) Delete(id string) error {
	return repo.sqliteGenericRepo.Delete(id)
}

var _ NPCTemplatesRepository = (*sqliteNPCTemplatesRepository)(nil)
