package sqlite

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func setupTestDB(t *testing.T) (*Client, string) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	client, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	t.Cleanup(func() { client.Close() })
	return client, dbPath
}

func TestOpen_AppliesPragmas(t *testing.T) {
	client, _ := setupTestDB(t)

	// Verify WAL mode
	var journalMode string
	if err := client.db.QueryRow("PRAGMA journal_mode;").Scan(&journalMode); err != nil {
		t.Fatalf("Failed to query journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Errorf("Expected journal_mode=wal, got %s", journalMode)
	}

	// Verify busy_timeout
	var busyTimeout int
	if err := client.db.QueryRow("PRAGMA busy_timeout;").Scan(&busyTimeout); err != nil {
		t.Fatalf("Failed to query busy_timeout: %v", err)
	}
	if busyTimeout != 5000 {
		t.Errorf("Expected busy_timeout=5000, got %d", busyTimeout)
	}

	// Verify synchronous
	var synchronous int
	if err := client.db.QueryRow("PRAGMA synchronous;").Scan(&synchronous); err != nil {
		t.Fatalf("Failed to query synchronous: %v", err)
	}
	// synchronous=NORMAL = 1
	if synchronous != 1 {
		t.Errorf("Expected synchronous=1 (NORMAL), got %d", synchronous)
	}
}

func TestOpen_PoolConfiguration(t *testing.T) {
	client, _ := setupTestDB(t)

	stats := client.db.Stats()
	// MaxOpenConns should be 4 by default
	if stats.MaxOpenConnections != 4 {
		t.Errorf("Expected MaxOpenConnections=4, got %d", stats.MaxOpenConnections)
	}
}

func TestOpen_PoolConfiguration_EnvOverride(t *testing.T) {
	os.Setenv("DB_MAX_OPEN_CONNS", "8")
	defer os.Unsetenv("DB_MAX_OPEN_CONNS")

	client, _ := setupTestDB(t)
	stats := client.db.Stats()
	if stats.MaxOpenConnections != 8 {
		t.Errorf("Expected MaxOpenConnections=8, got %d", stats.MaxOpenConnections)
	}
}

func TestOpen_PoolConfiguration_InvalidEnv(t *testing.T) {
	os.Setenv("DB_MAX_OPEN_CONNS", "invalid")
	defer os.Unsetenv("DB_MAX_OPEN_CONNS")

	client, _ := setupTestDB(t)
	stats := client.db.Stats()
	// Should fall back to default 4
	if stats.MaxOpenConnections != 4 {
		t.Errorf("Expected MaxOpenConnections=4 (fallback), got %d", stats.MaxOpenConnections)
	}
}

func TestMigration_Idempotency(t *testing.T) {
	client, dbPath := setupTestDB(t)
	client.Close()

	// Re-open the same database — migrations should run again without error
	client2, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Re-open failed (migration not idempotent): %v", err)
	}
	defer client2.Close()
}

func TestMigration_IndexesExist(t *testing.T) {
	client, _ := setupTestDB(t)

	expectedIndexes := []string{
		"idx_characters_user",
		"idx_characters_name",
		"idx_rooms_name",
		"idx_rooms_area",
		"idx_npc_spawners_room",
		"idx_npc_spawners_template",
		"idx_users_online",
		"idx_users_refid",
		"idx_users_username",
	}

	for _, idx := range expectedIndexes {
		var count int
		err := client.db.QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?",
			idx,
		).Scan(&count)
		if err != nil {
			t.Fatalf("Failed to check index %s: %v", idx, err)
		}
		if count == 0 {
			t.Errorf("Expected index %s to exist", idx)
		}
	}
}

func TestMigration_IdxCharactersUserUsesCorrectPath(t *testing.T) {
	client, _ := setupTestDB(t)

	// Verify the index definition uses $.belongsUser (not $.belongsUserID)
	var sql string
	err := client.db.QueryRow(
		"SELECT sql FROM sqlite_master WHERE type='index' AND name='idx_characters_user'",
	).Scan(&sql)
	if err != nil {
		t.Fatalf("Failed to get index definition: %v", err)
	}

	if sql == "" {
		t.Fatal("Index definition is empty")
	}
	// Should contain belongsUser but NOT belongsUserID
	if !contains(sql, "$.belongsUser") {
		t.Errorf("Index should use $.belongsUser, got: %s", sql)
	}
	if contains(sql, "$.belongsUserID") {
		t.Errorf("Index should NOT use $.belongsUserID (dead path), got: %s", sql)
	}
}

func TestMigration_IdxCharactersUser_EffectiveForFindAllForUser(t *testing.T) {
	client, _ := setupTestDB(t)

	// Insert a test character with belongsUser field
	testData := `{"id":"char1","name":"TestChar","belongsUser":"user123","currentRoomID":"room1"}`
	_, err := client.db.Exec("INSERT INTO characters (id, data) VALUES (?, ?)", "char1", testData)
	if err != nil {
		t.Fatalf("Failed to insert test character: %v", err)
	}

	// EXPLAIN QUERY PLAN should show the index is used
	rows, err := client.db.Query("EXPLAIN QUERY PLAN SELECT * FROM characters WHERE json_extract(data, '$.belongsUser') = 'user123'")
	if err != nil {
		t.Fatalf("EXPLAIN QUERY PLAN failed: %v", err)
	}
	defer rows.Close()

	var plan string
	for rows.Next() {
		var id, parent, notused int
		var detail string
		if err := rows.Scan(&id, &parent, &notused, &detail); err != nil {
			t.Fatalf("Failed to scan plan row: %v", err)
		}
		plan += detail + "\n"
	}

	if !contains(plan, "USING INDEX") && !contains(plan, "USING COVERING INDEX") {
		t.Errorf("Expected query plan to use index, got:\n%s", plan)
	}
}

func TestMigration_IdxRoomsName_EffectiveForFindByName(t *testing.T) {
	client, _ := setupTestDB(t)

	testData := `{"id":"room1","name":"TestRoom","area":"testarea"}`
	_, err := client.db.Exec("INSERT INTO rooms (id, data) VALUES (?, ?)", "room1", testData)
	if err != nil {
		t.Fatalf("Failed to insert test room: %v", err)
	}

	rows, err := client.db.Query("EXPLAIN QUERY PLAN SELECT * FROM rooms WHERE json_extract(data, '$.name') = 'TestRoom'")
	if err != nil {
		t.Fatalf("EXPLAIN QUERY PLAN failed: %v", err)
	}
	defer rows.Close()

	var plan string
	for rows.Next() {
		var id, parent, notused int
		var detail string
		if err := rows.Scan(&id, &parent, &notused, &detail); err != nil {
			t.Fatalf("Failed to scan plan row: %v", err)
		}
		plan += detail + "\n"
	}

	if !contains(plan, "USING INDEX") && !contains(plan, "USING COVERING INDEX") {
		t.Errorf("Expected query plan to use index for rooms.name, got:\n%s", plan)
	}
}

func TestMigration_IdxNPCSpawnersRoom_EffectiveForFindByRoom(t *testing.T) {
	client, _ := setupTestDB(t)

	testData := `{"id":"spawner1","roomId":"room1","templateId":"npc1"}`
	_, err := client.db.Exec("INSERT INTO npc_spawners (id, data) VALUES (?, ?)", "spawner1", testData)
	if err != nil {
		t.Fatalf("Failed to insert test spawner: %v", err)
	}

	rows, err := client.db.Query("EXPLAIN QUERY PLAN SELECT * FROM npc_spawners WHERE json_extract(data, '$.roomId') = 'room1'")
	if err != nil {
		t.Fatalf("EXPLAIN QUERY PLAN failed: %v", err)
	}
	defer rows.Close()

	var plan string
	for rows.Next() {
		var id, parent, notused int
		var detail string
		if err := rows.Scan(&id, &parent, &notused, &detail); err != nil {
			t.Fatalf("Failed to scan plan row: %v", err)
		}
		plan += detail + "\n"
	}

	if !contains(plan, "USING INDEX") && !contains(plan, "USING COVERING INDEX") {
		t.Errorf("Expected query plan to use index for npc_spawners.roomId, got:\n%s", plan)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Ensure sql import is used (for potential future test helpers)
var _ = sql.ErrNoRows
