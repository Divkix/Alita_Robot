package migrations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestRepositoryMigrationChain(t *testing.T) {
	if !strings.EqualFold(strings.TrimSpace(os.Getenv("ALITA_TEST_MIGRATION_CHAIN")), "true") {
		t.Skip("set ALITA_TEST_MIGRATION_CHAIN=true to test the repository migrations against an empty PostgreSQL database")
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Fatal("DATABASE_URL is required when ALITA_TEST_MIGRATION_CHAIN=true")
	}

	database, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("get PostgreSQL handle: %v", err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close PostgreSQL: %v", err)
		}
	})

	var relations int64
	if err := database.Raw(`
		SELECT COUNT(*)
		FROM pg_catalog.pg_class AS c
		JOIN pg_catalog.pg_namespace AS n ON n.oid = c.relnamespace
		WHERE n.nspname = current_schema()
		  AND c.relkind IN ('r', 'p', 'v', 'm', 'S', 'f')
	`).Scan(&relations).Error; err != nil {
		t.Fatalf("inspect PostgreSQL schema: %v", err)
	}
	if relations != 0 {
		t.Fatalf("refusing to run repository migrations against a nonempty current schema (%d relations)", relations)
	}

	migrationsPath, err := filepath.Abs(filepath.Join("..", "..", "..", "migrations"))
	if err != nil {
		t.Fatalf("resolve repository migrations path: %v", err)
	}
	runner := &MigrationRunner{db: database, migrationsPath: migrationsPath}
	files, err := runner.getMigrationFiles()
	if err != nil {
		t.Fatalf("list repository migrations: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("repository migrations directory contains no SQL files")
	}

	if err := runner.RunMigrations(); err != nil {
		t.Fatalf("apply repository migration chain: %v", err)
	}

	var timezoneAwareCaptchaColumns int64
	if err := database.Raw(`
		SELECT COUNT(*)
		FROM information_schema.columns
		WHERE table_schema = current_schema()
		  AND table_name = 'captcha_attempts'
		  AND column_name IN ('created_at', 'updated_at', 'expires_at')
		  AND data_type = 'timestamp with time zone'
	`).Scan(&timezoneAwareCaptchaColumns).Error; err != nil {
		t.Fatalf("inspect captcha attempt timestamp types: %v", err)
	}
	if timezoneAwareCaptchaColumns != 3 {
		t.Fatalf("timezone-aware captcha attempt timestamp columns = %d, want 3", timezoneAwareCaptchaColumns)
	}

	var records []SchemaMigration
	if err := database.Order("version").Find(&records).Error; err != nil {
		t.Fatalf("load applied migration records: %v", err)
	}
	if len(records) != len(files) {
		t.Fatalf("recorded migrations = %d, repository files = %d", len(records), len(files))
	}

	recorded := make(map[string]bool, len(records))
	for _, record := range records {
		recorded[record.Version] = true
		if record.Checksum == "" {
			t.Errorf("migration %s has an empty checksum", record.Version)
		}
	}
	for _, file := range files {
		version := filepath.Base(file)
		if !recorded[version] {
			t.Errorf("migration %s was not recorded", version)
		}
	}
}
