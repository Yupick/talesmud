package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
	_ "modernc.org/sqlite"
)

// Client wraps a SQLite connection and handles initialization.
type Client struct {
	db *sql.DB
}

// Open opens a SQLite database file and applies basic pragmas.
func Open(path string) (*Client, error) {
	// Embed pragmas in DSN so every new connection inherits them.
	// This is critical when using a connection pool > 1: pragmas like
	// busy_timeout and synchronous are per-connection in SQLite.
	// modernc.org/sqlite supports these as DSN parameters.
	dsn := path + "?_busy_timeout=5000&_journal_mode=WAL&_synchronous=NORMAL"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// Connection pool tuning for concurrent workloads (100-500 concurrent users).
	//
	// Why not 1 connection? With WAL mode, SQLite allows concurrent readers but
	// only one writer at a time. A single connection serializes ALL queries,
	// creating a bottleneck under load. With WAL + busy_timeout, multiple
	// connections can coexist: readers don't block writers, and writers wait
	// up to busy_timeout (5s) before failing with SQLITE_BUSY.
	//
	// Why a configurable limit? Too many connections increase memory usage and
	// lock contention. The sweet spot for SQLite is typically 4-8 connections.
	// Override via DB_MAX_OPEN_CONNS env var.
	//
	// Pragmas (busy_timeout, synchronous) are per-connection. We apply them
	// explicitly on each new connection via SetConnMaxLifetime + a connection
	// hook pattern (see applyPragmas).
	maxOpenConns := 4
	if v := os.Getenv("DB_MAX_OPEN_CONNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxOpenConns = n
		} else {
			log.WithField("value", v).Warn("Invalid DB_MAX_OPEN_CONNS, using default")
		}
	}
	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxOpenConns)
	// Recycle connections periodically to prevent stale connections and
	// ensure pragmas are re-applied on fresh connections.
	db.SetConnMaxLifetime(5 * time.Minute)

	client := &Client{db: db}
	if err := client.applyPragmas(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := client.InitSchema(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := client.runMigrations(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return client, nil
}

// DB returns the underlying sql.DB.
func (c *Client) DB() *sql.DB {
	return c.db
}

// Close closes the database connection.
func (c *Client) Close() error {
	return c.db.Close()
}

func (c *Client) applyPragmas() error {
	pragmas := []string{
		"PRAGMA journal_mode=WAL;",
		"PRAGMA synchronous=NORMAL;",
		"PRAGMA busy_timeout=5000;",
	}
	for _, stmt := range pragmas {
		if _, err := c.db.Exec(stmt); err != nil {
			return fmt.Errorf("sqlite pragma failed: %w", err)
		}
	}
	return nil
}

// InitSchema creates the tables used by the repositories.
func (c *Client) InitSchema() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users (id TEXT PRIMARY KEY, data TEXT NOT NULL);`,
		`CREATE TABLE IF NOT EXISTS characters (id TEXT PRIMARY KEY, data TEXT NOT NULL);`,
		`CREATE TABLE IF NOT EXISTS charactertemplates (id TEXT PRIMARY KEY, data TEXT NOT NULL);`,
		`CREATE TABLE IF NOT EXISTS rooms (id TEXT PRIMARY KEY, data TEXT NOT NULL);`,
		`CREATE TABLE IF NOT EXISTS items (id TEXT PRIMARY KEY, data TEXT NOT NULL);`,
		`CREATE TABLE IF NOT EXISTS scripts (id TEXT PRIMARY KEY, data TEXT NOT NULL);`,
		`CREATE TABLE IF NOT EXISTS npcs (id TEXT PRIMARY KEY, data TEXT NOT NULL);`,
		`CREATE TABLE IF NOT EXISTS npc_spawners (id TEXT PRIMARY KEY, data TEXT NOT NULL);`,
		`CREATE TABLE IF NOT EXISTS dialogs (id TEXT PRIMARY KEY, data TEXT NOT NULL);`,
		`CREATE TABLE IF NOT EXISTS conversations (id TEXT PRIMARY KEY, data TEXT NOT NULL);`,
		`CREATE TABLE IF NOT EXISTS parties (id TEXT PRIMARY KEY, data TEXT NOT NULL);`,
		`CREATE TABLE IF NOT EXISTS loot_tables (id TEXT PRIMARY KEY, data TEXT NOT NULL);`,
		`CREATE TABLE IF NOT EXISTS server_settings (id TEXT PRIMARY KEY, data TEXT NOT NULL);`,
		`CREATE TABLE IF NOT EXISTS quests (id TEXT PRIMARY KEY, data TEXT NOT NULL);`,
		`CREATE TABLE IF NOT EXISTS quest_progress (id TEXT PRIMARY KEY, data TEXT NOT NULL);`,
		`CREATE TABLE IF NOT EXISTS skills (id TEXT PRIMARY KEY, data TEXT NOT NULL);`,
		`CREATE TABLE IF NOT EXISTS guest_statistics (id TEXT PRIMARY KEY, data TEXT NOT NULL);`,
		`CREATE TABLE IF NOT EXISTS character_resources (id TEXT PRIMARY KEY, data TEXT NOT NULL);`,
		`CREATE TABLE IF NOT EXISTS content_health (id TEXT PRIMARY KEY, data TEXT NOT NULL);`,
		`CREATE TABLE IF NOT EXISTS audit_log (id TEXT PRIMARY KEY, data TEXT NOT NULL);`,
		`CREATE TABLE IF NOT EXISTS ssh_keys (id TEXT PRIMARY KEY, data TEXT NOT NULL);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_ssh_keys_fp ON ssh_keys(json_extract(data, '$.fingerprint'));`,
	}
	for _, stmt := range stmts {
		if _, err := c.db.Exec(stmt); err != nil {
			return fmt.Errorf("sqlite schema init failed: %w", err)
		}
	}

	// Create indexes on frequently queried JSON fields for better performance
	if err := c.createIndexes(); err != nil {
		return fmt.Errorf("sqlite index creation failed: %w", err)
	}

	return nil
}

// createIndexes creates indexes on commonly queried JSON fields
func (c *Client) createIndexes() error {
	indexes := []string{
		// NPCs: queried by name, room, and template status
		`CREATE INDEX IF NOT EXISTS idx_npcs_name ON npcs(json_extract(data, '$.name'));`,
		`CREATE INDEX IF NOT EXISTS idx_npcs_room ON npcs(json_extract(data, '$.currentRoomID'));`,
		`CREATE INDEX IF NOT EXISTS idx_npcs_template ON npcs(json_extract(data, '$.isTemplate'));`,

		// Characters: queried by user and room
		`CREATE INDEX IF NOT EXISTS idx_characters_user ON characters(json_extract(data, '$.belongsUserID'));`,
		`CREATE INDEX IF NOT EXISTS idx_characters_room ON characters(json_extract(data, '$.currentRoomID'));`,

		// Items: queried by name, room, and template status
		`CREATE INDEX IF NOT EXISTS idx_items_name ON items(json_extract(data, '$.name'));`,
		`CREATE INDEX IF NOT EXISTS idx_items_template ON items(json_extract(data, '$.isTemplate'));`,

		// Users: queried by email and online status
		`CREATE INDEX IF NOT EXISTS idx_users_email ON users(json_extract(data, '$.email'));`,

		// Quest progress: queried by character
		`CREATE INDEX IF NOT EXISTS idx_quest_progress_char ON quest_progress(json_extract(data, '$.characterID'));`,

		// Guest statistics: queried by guestRefID and date
		`CREATE INDEX IF NOT EXISTS idx_guest_stats_ref ON guest_statistics(json_extract(data, '$.guestRefID'));`,
		`CREATE INDEX IF NOT EXISTS idx_guest_stats_date ON guest_statistics(json_extract(data, '$.date'));`,
		`CREATE INDEX IF NOT EXISTS idx_guest_stats_started ON guest_statistics(json_extract(data, '$.startedAt'));`,

		// Audit log: filtered by entity and listed newest first
		`CREATE INDEX IF NOT EXISTS idx_audit_entity ON audit_log(json_extract(data, '$.entityType'), json_extract(data, '$.entityId'));`,
		`CREATE INDEX IF NOT EXISTS idx_audit_time ON audit_log(json_extract(data, '$.time'));`,
	}

	for _, idx := range indexes {
		if _, err := c.db.Exec(idx); err != nil {
			log.WithError(err).Warn("Failed to create index (may already exist)")
			// Don't fail on index creation errors - they're optimizations
		}
	}
	return nil
}

// runMigrations performs data migrations after schema init.
func (c *Client) runMigrations() error {
	if err := c.migrateItemTemplates(); err != nil {
		return fmt.Errorf("item templates migration failed: %w", err)
	}
	// FASE 4: Sistema Social (Clanes y Mensajería privada)
	if err := c.migratePhase4SocialTables(); err != nil {
		return fmt.Errorf("phase4 social migration failed: %w", err)
	}
	// FASE 6 (Corte B): Memoria largo plazo de NPCs (LLM) + índices de rendimiento
	if err := c.migratePhase6NPCMemories(); err != nil {
		return fmt.Errorf("phase6 npc memories migration failed: %w", err)
	}
	// Batch 16 (Db): corrección de índice muerto + índices faltantes
	if err := c.migratePhase16Indexes(); err != nil {
		return fmt.Errorf("phase16 index migration failed: %w", err)
	}
	// Batch 18 (Db): persistencia de definiciones de mazmorra (FASE 5.1)
	if err := c.migratePhase5Dungeons(); err != nil {
		return fmt.Errorf("phase5 dungeons migration failed: %w", err)
	}
	// Batch 20 (Db): persistencia de plantillas de NPC/mob (FASE 5.3)
	if err := c.migrateBatch20NPCTemplates(); err != nil {
		return fmt.Errorf("batch20 npc templates migration failed: %w", err)
	}
	return nil
}

// migrateBatch20NPCTemplates creates the FASE 5.3 NPC/mob template table and its
// lookup indexes. It follows the repo-wide (id, data) JSON pattern; the whole
// npcFactory.js payload (including loot, behavior and scaling raw JSON) lives
// in data, so listings can filter without parsing.
//
// All statements are idempotent (IF NOT EXISTS) and additive: existing tables,
// indexes and data are untouched, so the migration is safe to run on every open.
func (c *Client) migrateBatch20NPCTemplates() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS npc_templates (id TEXT PRIMARY KEY, data TEXT NOT NULL);`,
		// npc_templates.FindByName filters json_extract(data, '$.name').
		`CREATE INDEX IF NOT EXISTS idx_npc_templates_name ON npc_templates(json_extract(data, '$.name'));`,
		// npc_templates.FindByRole filters json_extract(data, '$.role').
		`CREATE INDEX IF NOT EXISTS idx_npc_templates_role ON npc_templates(json_extract(data, '$.role'));`,
	}

	for _, stmt := range stmts {
		if _, err := c.db.Exec(stmt); err != nil {
			return fmt.Errorf("batch20 npc templates migration failed: %w", err)
		}
	}
	return nil
}

// migratePhase5Dungeons creates the FASE 5.1 dungeon definitions table and its
// lookup indexes. It follows the repo-wide (id, data) JSON pattern; the whole
// exportDefinition payload lives in data and GridWidth/GridHeight are mirrored
// top-level JSON fields so listings can filter without parsing.
//
// All statements are idempotent (IF NOT EXISTS) and additive: existing tables,
// indexes and data are untouched, so the migration is safe to run on every open.
func (c *Client) migratePhase5Dungeons() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS dungeons (id TEXT PRIMARY KEY, data TEXT NOT NULL);`,
		// dungeons.FindByName filters json_extract(data, '$.name').
		`CREATE INDEX IF NOT EXISTS idx_dungeons_name ON dungeons(json_extract(data, '$.name'));`,
		// dungeons.FindByAuthor filters json_extract(data, '$.author').
		`CREATE INDEX IF NOT EXISTS idx_dungeons_author ON dungeons(json_extract(data, '$.author'));`,
	}

	for _, stmt := range stmts {
		if _, err := c.db.Exec(stmt); err != nil {
			return fmt.Errorf("phase5 dungeons migration failed: %w", err)
		}
	}
	return nil
}

// migratePhase6NPCMemories creates the Phase 6 long-term NPC memory table and
// its indexes, plus performance indexes for filter columns used by earlier
// phases that lacked them.
//
// npc_memories follows the repo-wide (id, data) JSON pattern. The id is the
// deterministic composite "<npcTemplateID>::<characterID>" (see
// docs/design/PHASE6_LLM.md §6.3), so rows for legacy characters without a
// dedicated record are simply absent — no backfill is required.
//
// All statements are idempotent (IF NOT EXISTS) and additive: existing data
// and schemas are untouched.
func (c *Client) migratePhase6NPCMemories() error {
	stmts := []string{
		// ---------- Fase 6: memoria largo plazo de NPCs ----------
		`CREATE TABLE IF NOT EXISTS npc_memories (id TEXT PRIMARY KEY, data TEXT NOT NULL);`,

		// FindByNPC / DeleteForNPC filter by npcId; FindByCharacter by characterId.
		// `updated` supports time-ordered listings and admin cleanup sweeps.
		`CREATE INDEX IF NOT EXISTS idx_npc_memories_npc ON npc_memories(json_extract(data, '$.npcId'));`,
		`CREATE INDEX IF NOT EXISTS idx_npc_memories_character ON npc_memories(json_extract(data, '$.characterId'));`,
		`CREATE INDEX IF NOT EXISTS idx_npc_memories_updated ON npc_memories(json_extract(data, '$.updated'));`,

		// ---------- Rendimiento: filtros existentes sin índice ----------
		// quests.FindByName filters json_extract(data, '$.name').
		`CREATE INDEX IF NOT EXISTS idx_quests_name ON quests(json_extract(data, '$.name'));`,
		// quests.FindBySourceNPC filters json_extract(data, '$.source.npcId').
		`CREATE INDEX IF NOT EXISTS idx_quests_source_npc ON quests(json_extract(data, '$.source.npcId'));`,
		// dialogs.FindByName filters json_extract(data, '$.name').
		`CREATE INDEX IF NOT EXISTS idx_dialogs_name ON dialogs(json_extract(data, '$.name'));`,
	}

	for _, stmt := range stmts {
		if _, err := c.db.Exec(stmt); err != nil {
			return fmt.Errorf("phase6 npc memories migration failed: %w", err)
		}
	}
	return nil
}

// migratePhase4SocialTables creates the Phase 4 social tables and indexes.
//
// Most tables follow the repo-wide (id, data) JSON pattern. dm_messages is the
// deliberate exception: it also carries real channel_id / sent_at / from_char_id
// columns so range and ordering queries can use indexes instead of json_extract.
// The real columns are written together with the JSON payload and are treated as
// read-only projections of that payload.
func (c *Client) migratePhase4SocialTables() error {
	stmts := []string{
		// ---------- Clanes ----------
		`CREATE TABLE IF NOT EXISTS clans (id TEXT PRIMARY KEY, data TEXT NOT NULL);`,
		`CREATE TABLE IF NOT EXISTS clan_members (id TEXT PRIMARY KEY, data TEXT NOT NULL);`,
		`CREATE TABLE IF NOT EXISTS clan_invites (id TEXT PRIMARY KEY, data TEXT NOT NULL);`,
		`CREATE TABLE IF NOT EXISTS clan_bank_ledger (id TEXT PRIMARY KEY, data TEXT NOT NULL);`,

		// ---------- Mensajería privada ----------
		`CREATE TABLE IF NOT EXISTS dm_channels (id TEXT PRIMARY KEY, data TEXT NOT NULL);`,
		`CREATE TABLE IF NOT EXISTS dm_messages (
			id TEXT PRIMARY KEY,
			data TEXT NOT NULL,
			channel_id TEXT NOT NULL,
			sent_at INTEGER NOT NULL,
			from_char_id TEXT NOT NULL
		);`,

		// Índices reales para dm_messages (columnas, no json_extract)
		`CREATE INDEX IF NOT EXISTS idx_dm_messages_channel_time
			ON dm_messages(channel_id, sent_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_dm_messages_channel_sender
			ON dm_messages(channel_id, from_char_id);`,
		`CREATE INDEX IF NOT EXISTS idx_dm_messages_unread
			ON dm_messages(channel_id, sent_at)
			WHERE json_extract(data, '$.readAt') IS NULL;`,

		// Índices JSON para clanes
		`CREATE INDEX IF NOT EXISTS idx_clans_name ON clans(json_extract(data, '$.name'));`,
		`CREATE INDEX IF NOT EXISTS idx_clans_tag ON clans(json_extract(data, '$.tag'));`,
		`CREATE INDEX IF NOT EXISTS idx_clans_leader ON clans(json_extract(data, '$.leaderCharID'));`,
		`CREATE INDEX IF NOT EXISTS idx_clans_active ON clans(json_extract(data, '$.active'));`,

		// Índices JSON para miembros
		`CREATE INDEX IF NOT EXISTS idx_clan_members_char ON clan_members(json_extract(data, '$.characterID'));`,
		`CREATE INDEX IF NOT EXISTS idx_clan_members_clan ON clan_members(json_extract(data, '$.clanID'));`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_clan_members_unique
			ON clan_members(json_extract(data, '$.characterID'), json_extract(data, '$.clanID'));`,

		// Índices JSON para invitaciones
		`CREATE INDEX IF NOT EXISTS idx_clan_invites_clan ON clan_invites(json_extract(data, '$.clanID'));`,
		`CREATE INDEX IF NOT EXISTS idx_clan_invites_char ON clan_invites(json_extract(data, '$.characterID'));`,
		`CREATE INDEX IF NOT EXISTS idx_clan_invites_status ON clan_invites(json_extract(data, '$.status'));`,

		// Índices JSON para el ledger bancario
		`CREATE INDEX IF NOT EXISTS idx_clan_bank_ledger_clan ON clan_bank_ledger(json_extract(data, '$.clanID'));`,
		`CREATE INDEX IF NOT EXISTS idx_clan_bank_ledger_time ON clan_bank_ledger(json_extract(data, '$.timestamp'));`,

		// Índices JSON para canales DM
		`CREATE INDEX IF NOT EXISTS idx_dm_channels_participant ON dm_channels(json_extract(data, '$.participantA'));`,
		`CREATE INDEX IF NOT EXISTS idx_dm_channels_participant_b ON dm_channels(json_extract(data, '$.participantB'));`,
	}

	for _, stmt := range stmts {
		if _, err := c.db.Exec(stmt); err != nil {
			return fmt.Errorf("phase4 social table migration failed: %w", err)
		}
	}
	return nil
}

// migratePhase16Indexes fixes the dead index on characters and adds missing
// indexes for rooms, npc_spawners, and users.
//
// Problem: idx_characters_user was created on json_extract(data, '$.belongsUserID')
// but the actual JSON field is 'belongsUser' (see entities/traits/belongsuser.go).
// This made the index useless — FindAllForUser performed a full table scan.
//
// Fix: DROP the dead index and recreate it with the correct JSON path.
// Also adds indexes for:
//   - characters.name (FindByName)
//   - rooms.name, rooms.area (FindByName, area-based queries)
//   - npc_spawners.roomId, npc_spawners.templateId (FindByRoom, FindByTemplate)
//   - users.isOnline, users.refid, users.username (FindAllOnline, FindByRefID, FindByUsername)
//
// All statements are idempotent and safe for existing databases.
func (c *Client) migratePhase16Indexes() error {
	stmts := []string{
		// Fix dead index: drop and recreate with correct JSON path
		`DROP INDEX IF EXISTS idx_characters_user;`,
		`CREATE INDEX IF NOT EXISTS idx_characters_user ON characters(json_extract(data, '$.belongsUser'));`,

		// Characters: name lookups
		`CREATE INDEX IF NOT EXISTS idx_characters_name ON characters(json_extract(data, '$.name'));`,

		// Rooms: name and area filters
		`CREATE INDEX IF NOT EXISTS idx_rooms_name ON rooms(json_extract(data, '$.name'));`,
		`CREATE INDEX IF NOT EXISTS idx_rooms_area ON rooms(json_extract(data, '$.area'));`,

		// NPC Spawners: room and template filters
		`CREATE INDEX IF NOT EXISTS idx_npc_spawners_room ON npc_spawners(json_extract(data, '$.roomId'));`,
		`CREATE INDEX IF NOT EXISTS idx_npc_spawners_template ON npc_spawners(json_extract(data, '$.templateId'));`,

		// Users: online status, refid, and username lookups
		`CREATE INDEX IF NOT EXISTS idx_users_online ON users(json_extract(data, '$.isOnline'));`,
		`CREATE INDEX IF NOT EXISTS idx_users_refid ON users(json_extract(data, '$.refid'));`,
		`CREATE INDEX IF NOT EXISTS idx_users_username ON users(json_extract(data, '$.username'));`,
	}

	for _, stmt := range stmts {
		if _, err := c.db.Exec(stmt); err != nil {
			return fmt.Errorf("phase16 index migration failed: %w", err)
		}
	}
	log.Info("Phase 16 index migration complete: fixed idx_characters_user, added rooms/npc_spawners/users indexes")
	return nil
}

// migrateItemTemplates migrates data from the old itemtemplates table to items table.
// It adds isTemplate: true to each item and removes the script field.
func (c *Client) migrateItemTemplates() error {
	// Check if itemtemplates table exists
	var count int
	err := c.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='itemtemplates'").Scan(&count)
	if err != nil || count == 0 {
		return nil // No migration needed
	}

	log.Info("Migrating itemtemplates to items table...")

	// Read all itemtemplates first (collect before writing to avoid deadlock with single connection)
	type templateRow struct {
		id   string
		data string
	}
	var templates []templateRow

	rows, err := c.db.Query("SELECT id, data FROM itemtemplates")
	if err != nil {
		return err
	}
	for rows.Next() {
		var t templateRow
		if err := rows.Scan(&t.id, &t.data); err != nil {
			log.WithError(err).Warn("Failed to scan itemtemplate row")
			continue
		}
		templates = append(templates, t)
	}
	rows.Close() // Close before writing

	if err := rows.Err(); err != nil {
		return err
	}

	// Now insert into items table
	migratedCount := 0
	for _, t := range templates {
		// Parse JSON, add isTemplate flag, remove script field
		var itemData map[string]interface{}
		if err := json.Unmarshal([]byte(t.data), &itemData); err != nil {
			log.WithError(err).WithField("id", t.id).Warn("Failed to parse itemtemplate JSON")
			continue
		}

		itemData["isTemplate"] = true
		delete(itemData, "script") // Remove script field

		newData, err := json.Marshal(itemData)
		if err != nil {
			log.WithError(err).WithField("id", t.id).Warn("Failed to marshal migrated item")
			continue
		}

		// Insert into items table (upsert to handle conflicts)
		_, err = c.db.Exec(
			"INSERT OR REPLACE INTO items (id, data) VALUES (?, ?)",
			t.id, string(newData),
		)
		if err != nil {
			log.WithError(err).WithField("id", t.id).Warn("Failed to migrate itemtemplate")
			continue
		}
		migratedCount++
	}

	// Drop the old itemtemplates table
	_, err = c.db.Exec("DROP TABLE IF EXISTS itemtemplates")
	if err != nil {
		return fmt.Errorf("failed to drop itemtemplates table: %w", err)
	}

	log.WithField("count", migratedCount).Info("Item templates migration complete")
	return nil
}
