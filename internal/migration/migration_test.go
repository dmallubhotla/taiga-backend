package migration_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"

	"gitea.deepak.science/deepak/taiga/internal/migration"
)

func TestNewWithSQLite(t *testing.T) {
	// Create temp directory for test migrations
	tempDir := t.TempDir()
	migrationsPath := filepath.Join(tempDir, "migrations")
	err := os.MkdirAll(migrationsPath, 0755)
	require.NoError(t, err)

	// Create a simple migration file
	migrationContent := `CREATE TABLE test_table (id INTEGER PRIMARY KEY);`
	err = os.WriteFile(filepath.Join(migrationsPath, "000001_test.up.sql"), []byte(migrationContent), 0644)
	require.NoError(t, err)

	downContent := `DROP TABLE test_table;`
	err = os.WriteFile(filepath.Join(migrationsPath, "000001_test.down.sql"), []byte(downContent), 0644)
	require.NoError(t, err)

	// Create in-memory SQLite database
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Test creating migrator
	migrator, err := migration.New(db, "sqlite", migrationsPath)
	require.NoError(t, err)
	assert.NotNil(t, migrator)

	// Clean up
	err = migrator.Close()
	assert.NoError(t, err)
}

func TestNewWithPostgres(t *testing.T) {
	// Create temp directory for test migrations
	tempDir := t.TempDir()
	migrationsPath := filepath.Join(tempDir, "migrations")
	err := os.MkdirAll(migrationsPath, 0755)
	require.NoError(t, err)

	// Note: This test requires a mock or real postgres connection
	// For this test, we'll skip if postgres isn't available
	t.Skip("Postgres test requires actual postgres connection - skipping for unit tests")
}

func TestNewWithUnsupportedDriver(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	migrator, err := migration.New(db, "unsupported-driver", "./migrations")
	assert.Nil(t, migrator)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported database driver")
}

func TestNewWithInvalidPath(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	migrator, err := migration.New(db, "sqlite", "/nonexistent/path")
	assert.Nil(t, migrator)
	assert.Error(t, err)
}

func TestMigrationOperations(t *testing.T) {
	// Create temp directory for test migrations
	tempDir := t.TempDir()
	migrationsPath := filepath.Join(tempDir, "migrations")
	err := os.MkdirAll(migrationsPath, 0755)
	require.NoError(t, err)

	// Create migration files
	upMigration := `CREATE TABLE users (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		email TEXT UNIQUE NOT NULL
	);`
	err = os.WriteFile(filepath.Join(migrationsPath, "000001_create_users.up.sql"), []byte(upMigration), 0644)
	require.NoError(t, err)

	downMigration := `DROP TABLE users;`
	err = os.WriteFile(filepath.Join(migrationsPath, "000001_create_users.down.sql"), []byte(downMigration), 0644)
	require.NoError(t, err)

	// Second migration
	upMigration2 := `CREATE TABLE posts (
		id INTEGER PRIMARY KEY,
		user_id INTEGER,
		title TEXT NOT NULL,
		content TEXT
	);`
	err = os.WriteFile(filepath.Join(migrationsPath, "000002_create_posts.up.sql"), []byte(upMigration2), 0644)
	require.NoError(t, err)

	downMigration2 := `DROP TABLE posts;`
	err = os.WriteFile(filepath.Join(migrationsPath, "000002_create_posts.down.sql"), []byte(downMigration2), 0644)
	require.NoError(t, err)

	// Create in-memory SQLite database
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Create migrator
	migrator, err := migration.New(db, "sqlite", migrationsPath)
	require.NoError(t, err)
	defer func() { _ = migrator.Close() }()

	// Test initial version (should be no migrations applied)
	_, _, err = migrator.Version()
	if err != nil {
		// No migrations applied yet, this is expected
		assert.Contains(t, err.Error(), "no migration")
	}

	// Test Up migration
	err = migrator.Up()
	assert.NoError(t, err)

	// Check version after up migration
	version, dirty, err := migrator.Version()
	assert.NoError(t, err)
	assert.Equal(t, uint(2), version) // Should be at version 2 (latest)
	assert.False(t, dirty)

	// Verify tables were created
	var tableCount int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name IN ('users', 'posts')").Scan(&tableCount)
	assert.NoError(t, err)
	assert.Equal(t, 2, tableCount)

	// Test Steps (go down 1 step)
	err = migrator.Steps(-1)
	assert.NoError(t, err)

	// Check version after step down
	version, dirty, err = migrator.Version()
	assert.NoError(t, err)
	assert.Equal(t, uint(1), version)
	assert.False(t, dirty)

	// Test Migrate to specific version
	err = migrator.Migrate(2)
	assert.NoError(t, err)

	version, dirty, err = migrator.Version()
	assert.NoError(t, err)
	assert.Equal(t, uint(2), version)
	assert.False(t, dirty)

	// Test Down migration
	err = migrator.Down()
	assert.NoError(t, err)

	// Check that tables were dropped
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name IN ('users', 'posts')").Scan(&tableCount)
	assert.NoError(t, err)
	assert.Equal(t, 0, tableCount)
}

func TestStepsWithInvalidStep(t *testing.T) {
	tempDir := t.TempDir()
	migrationsPath := filepath.Join(tempDir, "migrations")
	err := os.MkdirAll(migrationsPath, 0755)
	require.NoError(t, err)

	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	migrator, err := migration.New(db, "sqlite", migrationsPath)
	require.NoError(t, err)
	defer func() { _ = migrator.Close() }()

	// Try to step when no migrations exist
	err = migrator.Steps(1)
	assert.Error(t, err)
}

func TestMigrateToInvalidVersion(t *testing.T) {
	tempDir := t.TempDir()
	migrationsPath := filepath.Join(tempDir, "migrations")
	err := os.MkdirAll(migrationsPath, 0755)
	require.NoError(t, err)

	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	migrator, err := migration.New(db, "sqlite", migrationsPath)
	require.NoError(t, err)
	defer func() { _ = migrator.Close() }()

	// Try to migrate to a version that doesn't exist
	err = migrator.Migrate(999)
	assert.Error(t, err)
}

func TestClose(t *testing.T) {
	tempDir := t.TempDir()
	migrationsPath := filepath.Join(tempDir, "migrations")
	err := os.MkdirAll(migrationsPath, 0755)
	require.NoError(t, err)

	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	migrator, err := migration.New(db, "sqlite", migrationsPath)
	require.NoError(t, err)

	// Test Close
	err = migrator.Close()
	assert.NoError(t, err)

	// Calling close again should still work
	_ = migrator.Close()
	// May or may not error depending on implementation, but shouldn't panic
}

func TestSQLite3DriverAlias(t *testing.T) {
	tempDir := t.TempDir()
	migrationsPath := filepath.Join(tempDir, "migrations")
	err := os.MkdirAll(migrationsPath, 0755)
	require.NoError(t, err)

	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Test that "sqlite3" also works as driver name
	migrator, err := migration.New(db, "sqlite3", migrationsPath)
	require.NoError(t, err)
	assert.NotNil(t, migrator)

	err = migrator.Close()
	assert.NoError(t, err)
}
