package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const migrationLockTestFile = "20260101000000_lock_test.sql"

// TestMigrationAdvisoryLockPinnedConnection proves the lock is session-scoped:
// while a pinned connection holds it, another pool's session cannot take it,
// and releasing it on the owner session frees it for the other session.
func TestMigrationAdvisoryLockPinnedConnection(t *testing.T) {
	skipIfNoDb(t)
	schema := newMigrationLockSchema(t)

	owner := openMigrationLockPool(t, schema)
	other := openMigrationLockPool(t, schema)

	ownerConn := pinMigrationLockConn(t, owner)
	defer func() { _ = ownerConn.Close() }()

	if !tryMigrationLock(t, ownerConn) {
		t.Fatal("pinned connection could not acquire the migration lock")
	}

	otherConn := pinMigrationLockConn(t, other)
	defer func() { _ = otherConn.Close() }()

	if tryMigrationLock(t, otherConn) {
		t.Fatal("second session acquired the migration lock while the pinned connection held it")
	}

	if !unlockMigrationLock(t, ownerConn) {
		t.Fatal("owner session did not release the migration lock")
	}

	if !tryMigrationLock(t, otherConn) {
		t.Fatal("migration lock stayed held after the owner session released it")
	}
	if !unlockMigrationLock(t, otherConn) {
		t.Fatal("second session did not release the migration lock it acquired")
	}
}

// TestRunMigrationsSerializesConcurrentRunners runs two runners against separate
// pools on one empty schema. The loser must wait for the winner and observe its
// applied migration instead of applying the same file a second time.
func TestRunMigrationsSerializesConcurrentRunners(t *testing.T) {
	skipIfNoDb(t)
	schema := newMigrationLockSchema(t)

	pools := []*gorm.DB{
		openMigrationLockPool(t, schema),
		openMigrationLockPool(t, schema),
	}
	migrationsPath := newMigrationLockDir(t,
		"SELECT pg_sleep(0.3);\nCREATE TABLE migration_lock_widgets (id BIGINT PRIMARY KEY);\n")

	runners := make([]*MigrationRunner, len(pools))
	for i, pool := range pools {
		runners[i] = &MigrationRunner{db: pool, migrationsPath: migrationsPath}
	}

	start := make(chan struct{})
	errs := make([]error, len(runners))
	var wg sync.WaitGroup
	for i, runner := range runners {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs[i] = runner.RunMigrations()
		}()
	}
	close(start)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("concurrent RunMigrations() call %d error = %v", i+1, err)
		}
	}

	var records []SchemaMigration
	if err := pools[1].Order("version").Find(&records).Error; err != nil {
		t.Fatalf("load applied migrations from the second pool: %v", err)
	}
	if len(records) != 1 || records[0].Version != migrationLockTestFile {
		t.Fatalf("applied migrations from the second pool = %+v, want exactly %s", records, migrationLockTestFile)
	}
}

// TestMigrationLockReleasedAfterFailedMigration ensures the deferred unlock runs
// when migration work fails, so the next runner is not stuck behind a dead one.
func TestMigrationLockReleasedAfterFailedMigration(t *testing.T) {
	skipIfNoDb(t)
	schema := newMigrationLockSchema(t)

	database := openMigrationLockPool(t, schema)
	runner := &MigrationRunner{
		db:             database,
		migrationsPath: newMigrationLockDir(t, "SELECT * FROM migration_lock_missing_table;\n"),
	}

	if err := runner.RunMigrations(); err == nil {
		t.Fatal("RunMigrations() with a failing migration returned a nil error")
	}

	// Probe from a separate pool so the assertion cannot accidentally reuse the
	// session that ran RunMigrations.
	probe := openMigrationLockPool(t, schema)
	probeConn := pinMigrationLockConn(t, probe)
	defer func() { _ = probeConn.Close() }()

	if !tryMigrationLock(t, probeConn) {
		t.Fatal("migration lock is still held after a failed migration")
	}
	if !unlockMigrationLock(t, probeConn) {
		t.Fatal("probe session did not release the migration lock it acquired")
	}
}

// newMigrationLockSchema creates an empty schema for one test and drops it,
// with its objects, when the test finishes.
func newMigrationLockSchema(t *testing.T) string {
	t.Helper()
	schema := fmt.Sprintf("migration_lock_%d", time.Now().UnixNano())
	if err := getTestDB().Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	t.Cleanup(func() {
		if err := getTestDB().Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Errorf("drop test schema %s: %v", schema, err)
		}
	})
	return schema
}

// openMigrationLockPool opens an independent pool whose sessions resolve
// unqualified names to the given schema.
func openMigrationLockPool(t *testing.T, schema string) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if strings.Contains(dsn, "://") {
		parsed, err := url.Parse(dsn)
		if err != nil {
			t.Fatalf("parse DATABASE_URL: %v", err)
		}
		query := parsed.Query()
		query.Set("search_path", schema)
		parsed.RawQuery = query.Encode()
		dsn = parsed.String()
	} else {
		dsn += " search_path=" + schema
	}

	database, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open PostgreSQL pool for schema %s: %v", schema, err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("get PostgreSQL handle for schema %s: %v", schema, err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return database
}

func pinMigrationLockConn(t *testing.T, database *gorm.DB) *sql.Conn {
	t.Helper()
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("get database handle: %v", err)
	}
	conn, err := sqlDB.Conn(context.Background())
	if err != nil {
		t.Fatalf("pin migration lock connection: %v", err)
	}
	return conn
}

func newMigrationLockDir(t *testing.T, statements string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, migrationLockTestFile), []byte(statements), 0o600); err != nil {
		t.Fatalf("write test migration: %v", err)
	}
	return dir
}

func tryMigrationLock(t *testing.T, conn *sql.Conn) bool {
	t.Helper()
	var locked bool
	if err := conn.QueryRowContext(context.Background(), migrationLockSQL).Scan(&locked); err != nil {
		t.Fatalf("try migration lock: %v", err)
	}
	return locked
}

func unlockMigrationLock(t *testing.T, conn *sql.Conn) bool {
	t.Helper()
	var unlocked bool
	if err := conn.QueryRowContext(context.Background(), migrationUnlockSQL).Scan(&unlocked); err != nil {
		t.Fatalf("unlock migration lock: %v", err)
	}
	return unlocked
}
