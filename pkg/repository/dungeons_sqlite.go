package repository

import (
	"database/sql"
	"errors"
	"time"

	dbpkg "github.com/talesmud/talesmud/pkg/db"
	dbsqlite "github.com/talesmud/talesmud/pkg/db/sqlite"
	e "github.com/talesmud/talesmud/pkg/entities"
	"github.com/talesmud/talesmud/pkg/entities/dungeons"
)

// sqliteDungeonsRepository persists dungeon definitions (FASE 5.1) in the
// dungeons table (id, data JSON), following the repo-wide JSON pattern and the
// timestamped Store/Update contract used by the clan repository.
//
// Definition is stored as raw JSON, so the exact exportDefinition payload
// round-trips without re-encoding.
type sqliteDungeonsRepository struct {
	*sqliteGenericRepo
	db *sql.DB
}

// NewSQLiteDungeonsRepository creates a new SQLite-backed dungeon definition repository.
func NewSQLiteDungeonsRepository(client *dbsqlite.Client) DungeonsRepository {
	conn := client.DB()
	return &sqliteDungeonsRepository{
		sqliteGenericRepo: newSQLiteGenericRepo(conn, "dungeons", func() interface{} {
			return &dungeons.DungeonDefinition{}
		}),
		db: conn,
	}
}

// ---------- CRUD ----------

func (repo *sqliteDungeonsRepository) Drop() error {
	return repo.sqliteGenericRepo.DropCollection()
}

// Create persists a new dungeon definition. It generates an ID and stamps
// CreatedAt/UpdatedAt when missing. As with Store elsewhere, it upserts on the
// entity id, so re-creating with the same id replaces the stored row.
func (repo *sqliteDungeonsRepository) Create(def *dungeons.DungeonDefinition) (*dungeons.DungeonDefinition, error) {
	if def == nil {
		return nil, errors.New("dungeon definition is nil")
	}
	if def.Entity == nil {
		def.Entity = e.NewEntity()
	}
	now := time.Now()
	if def.CreatedAt.IsZero() {
		def.CreatedAt = now
	}
	def.UpdatedAt = now
	result, err := repo.sqliteGenericRepo.Store(def)
	if err != nil {
		return nil, err
	}
	return result.(*dungeons.DungeonDefinition), nil
}

// Update replaces the definition stored under id and refreshes UpdatedAt.
// CreatedAt is preserved by callers that fetch, mutate and save back.
func (repo *sqliteDungeonsRepository) Update(id string, def *dungeons.DungeonDefinition) error {
	if def == nil {
		return errors.New("dungeon definition is nil")
	}
	if def.Entity == nil {
		def.Entity = e.NewEntity()
	}
	def.Entity.ID = id
	def.UpdatedAt = time.Now()
	return repo.sqliteGenericRepo.Update(def, id)
}

func (repo *sqliteDungeonsRepository) FindByID(id string) (*dungeons.DungeonDefinition, error) {
	result, err := repo.sqliteGenericRepo.FindByID(id)
	if err != nil {
		return nil, err
	}
	return result.(*dungeons.DungeonDefinition), nil
}

func (repo *sqliteDungeonsRepository) FindAll() ([]*dungeons.DungeonDefinition, error) {
	return collectRepo[dungeons.DungeonDefinition](repo.sqliteGenericRepo, nil)
}

func (repo *sqliteDungeonsRepository) FindByName(name string) ([]*dungeons.DungeonDefinition, error) {
	return collectRepo[dungeons.DungeonDefinition](repo.sqliteGenericRepo, dbpkg.NewQueryParams(
		dbpkg.QueryParam{Key: "name", Value: name},
	))
}

func (repo *sqliteDungeonsRepository) FindByAuthor(author string) ([]*dungeons.DungeonDefinition, error) {
	return collectRepo[dungeons.DungeonDefinition](repo.sqliteGenericRepo, dbpkg.NewQueryParams(
		dbpkg.QueryParam{Key: "author", Value: author},
	))
}

func (repo *sqliteDungeonsRepository) Delete(id string) error {
	return repo.sqliteGenericRepo.Delete(id)
}

var _ DungeonsRepository = (*sqliteDungeonsRepository)(nil)
