package db

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gorm.io/gorm"

	"github.com/divkix/Alita_Robot/alita/config"
)

// newTestRunner returns a MigrationRunner with nil db suitable for testing
// pure functions like cleanSupabaseSQL and splitSQLStatements.
func newTestRunner() *MigrationRunner {
	return &MigrationRunner{db: nil, migrationsPath: "", cleanSQL: true}
}

// ---------------------------------------------------------------------------
// SchemaMigration.TableName
// ---------------------------------------------------------------------------

func TestSchemaMigrationTableName(t *testing.T) {

	got := SchemaMigration{}.TableName()
	if got != "schema_migrations" {
		t.Fatalf("SchemaMigration.TableName() = %q, want %q", got, "schema_migrations")
	}
}

// ---------------------------------------------------------------------------
// cleanSupabaseSQL
// ---------------------------------------------------------------------------

func TestCleanSupabaseSQL(t *testing.T) {

	runner := newTestRunner()

	tests := []struct {
		name      string
		input     string
		wantParts []string // substrings that must be present after cleaning
		wantGone  []string // substrings that must NOT be present after cleaning
	}{
		{
			name:     "GRANT removal -- anon role",
			input:    `GRANT SELECT ON some_table TO anon;`,
			wantGone: []string{"GRANT SELECT"},
		},
		{
			name:     "GRANT removal -- authenticated role",
			input:    `GRANT INSERT ON some_table TO authenticated;`,
			wantGone: []string{"GRANT INSERT"},
		},
		{
			name:     "GRANT removal -- service_role",
			input:    `GRANT ALL ON some_table TO service_role;`,
			wantGone: []string{"GRANT ALL"},
		},
		{
			name:     "policy removal",
			input:    `CREATE POLICY my_policy ON some_table TO anon;`,
			wantGone: []string{"CREATE POLICY my_policy"},
		},
		{
			name:     "schema extensions removal",
			input:    `CREATE EXTENSION IF NOT EXISTS pgcrypto with schema "extensions";`,
			wantGone: []string{`with schema "extensions"`},
		},
		{
			name:      "Supabase-only extension skipped",
			input:     `CREATE EXTENSION IF NOT EXISTS hypopg;`,
			wantGone:  []string{"CREATE EXTENSION IF NOT EXISTS hypopg"},
			wantParts: []string{"Skipped Supabase-specific extension: hypopg"},
		},
		{
			name:      "non-Supabase extension normalised to IF NOT EXISTS",
			input:     `CREATE EXTENSION pgcrypto;`,
			wantParts: []string{"CREATE EXTENSION IF NOT EXISTS"},
			wantGone:  []string{"CREATE EXTENSION pgcrypto;"},
		},
		{
			name:      "CREATE TABLE made idempotent",
			input:     `CREATE TABLE users (id SERIAL PRIMARY KEY);`,
			wantParts: []string{"CREATE TABLE IF NOT EXISTS"},
		},
		{
			name:      "empty SQL returns empty",
			input:     "",
			wantParts: []string{},
			wantGone:  []string{},
		},
		{
			name:      "clean SQL passes through unchanged semantics",
			input:     `SELECT 1;`,
			wantParts: []string{"SELECT 1"},
		},
		{
			name:      "idempotency -- already idempotent CREATE TABLE unchanged",
			input:     `CREATE TABLE IF NOT EXISTS foo (id SERIAL);`,
			wantParts: []string{"CREATE TABLE IF NOT EXISTS"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			got := runner.cleanSupabaseSQL(tc.input)

			for _, want := range tc.wantParts {
				if want != "" && !strings.Contains(got, want) {
					t.Errorf("cleanSupabaseSQL() result missing expected substring %q\nresult: %q", want, got)
				}
			}

			for _, gone := range tc.wantGone {
				if gone != "" && strings.Contains(got, gone) {
					t.Errorf("cleanSupabaseSQL() result still contains removed substring %q\nresult: %q", gone, got)
				}
			}
		})
	}
}

func TestCleanSupabaseSQL_AdditionalCases(t *testing.T) {

	runner := newTestRunner()

	tests := []struct {
		name      string
		input     string
		wantParts []string
		wantGone  []string
	}{
		{
			name:      "CREATE INDEX adds IF NOT EXISTS",
			input:     `CREATE INDEX idx_users_email ON users(email);`,
			wantParts: []string{"CREATE INDEX IF NOT EXISTS"},
			wantGone:  []string{"CREATE INDEX idx_users_email ON"},
		},
		{
			name:      "CREATE UNIQUE INDEX adds IF NOT EXISTS",
			input:     `CREATE UNIQUE INDEX idx_users_email ON users(email);`,
			wantParts: []string{"CREATE UNIQUE INDEX IF NOT EXISTS"},
		},
		{
			name:      "Already IF NOT EXISTS in CREATE INDEX not doubled",
			input:     `CREATE INDEX IF NOT EXISTS idx_foo ON bar(col);`,
			wantParts: []string{"CREATE INDEX IF NOT EXISTS"},
			wantGone:  []string{"IF NOT EXISTS IF NOT EXISTS"},
		},
		{
			// The DO block wraps the original CREATE TYPE statement, so the inner SQL
			// still appears as a substring. Verify the outer DO block wrapper is added.
			name:  "CREATE TYPE ENUM wrapped in DO block",
			input: `CREATE TYPE mood AS ENUM ('happy', 'sad', 'neutral');`,
			wantParts: []string{
				"DO $$",
				"CREATE TYPE mood AS ENUM",
				"EXCEPTION",
				"END $$;",
			},
		},
		{
			name:  "ALTER TABLE ADD CONSTRAINT wrapped in DO block",
			input: `ALTER TABLE orders ADD CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES users(id);`,
			wantParts: []string{
				"DO $$",
				"ALTER TABLE orders ADD CONSTRAINT fk_user",
				"EXCEPTION",
				"END $$;",
			},
		},
		{
			name: "ALTER TABLE ADD CONSTRAINT inside existing DO block is NOT double-wrapped",
			input: `DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'chats') THEN
        ALTER TABLE disable_chat_settings
        ADD CONSTRAINT fk_disable_chat_settings_chat
        FOREIGN KEY (chat_id) REFERENCES chats(chat_id) ON DELETE CASCADE ON UPDATE CASCADE;
    END IF;
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;`,
			wantParts: []string{
				"DO $$",
				"ALTER TABLE disable_chat_settings",
				"ADD CONSTRAINT fk_disable_chat_settings_chat",
				"EXCEPTION",
				"WHEN duplicate_object THEN NULL",
			},
			wantGone: []string{
				"DO $$ BEGIN\n    ALTER TABLE disable_chat_settings ADD CONSTRAINT fk_disable_chat_settings_chat", // must not inject inner DO block
			},
		},
		{
			name:  "standalone ALTER TABLE ADD CONSTRAINT still gets DO-wrapped",
			input: `ALTER TABLE orders ADD CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES users(id);`,
			wantParts: []string{
				"DO $$ BEGIN",
				"ALTER TABLE orders ADD CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES users(id);",
				"EXCEPTION",
				"WHEN OTHERS THEN null;",
				"END $$;",
			},
		},
		{
			name: "Mixed GRANT and CREATE TABLE - GRANTs removed, CREATE TABLE preserved",
			input: `GRANT SELECT ON users TO anon;
CREATE TABLE profiles (id SERIAL PRIMARY KEY);
GRANT INSERT ON profiles TO authenticated;`,
			wantParts: []string{"CREATE TABLE IF NOT EXISTS"},
			wantGone:  []string{"GRANT SELECT", "GRANT INSERT"},
		},
		{
			name:      "Empty string returns empty output",
			input:     "",
			wantParts: []string{},
			wantGone:  []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			got := runner.cleanSupabaseSQL(tc.input)

			for _, want := range tc.wantParts {
				if want != "" && !strings.Contains(got, want) {
					t.Errorf("cleanSupabaseSQL() result missing expected substring %q\nresult: %q", want, got)
				}
			}

			for _, gone := range tc.wantGone {
				if gone != "" && strings.Contains(got, gone) {
					t.Errorf("cleanSupabaseSQL() result still contains removed substring %q\nresult: %q", gone, got)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// splitSQLStatements
// ---------------------------------------------------------------------------

func TestSplitSQLStatements(t *testing.T) {

	runner := newTestRunner()

	tests := []struct {
		name      string
		input     string
		wantCount int
		wantFirst string // optional: first statement content check (substring)
	}{
		{
			name:      "simple split",
			input:     "SELECT 1; SELECT 2;",
			wantCount: 2,
		},
		{
			name: "dollar-quoted string not split",
			input: `CREATE FUNCTION f() RETURNS void AS $$
BEGIN
  RAISE NOTICE 'hello; world';
END;
$$ LANGUAGE plpgsql;`,
			wantCount: 1,
		},
		{
			name: "block comment preserved",
			input: `/* this is a comment; not a statement */
SELECT 1;`,
			wantCount: 1,
		},
		{
			name: "line comment preserved",
			input: `-- this is a comment; not a statement
SELECT 1;`,
			wantCount: 1,
		},
		{
			name:      "quoted semicolons not split",
			input:     `SELECT 'hello; world'; SELECT 2;`,
			wantCount: 2,
		},
		{
			name:      "empty input returns nothing",
			input:     "",
			wantCount: 0,
		},
		{
			name:      "whitespace-only returns nothing",
			input:     "   \n\t  ",
			wantCount: 0,
		},
		{
			name:      "single statement no semicolon",
			input:     "SELECT 1",
			wantCount: 1,
			wantFirst: "SELECT 1",
		},
		{
			name:      "three statements",
			input:     "SELECT 1; SELECT 2; SELECT 3;",
			wantCount: 3,
		},
		{
			// Critical test: DO block with trailing semicolon followed by another statement
			// This is the exact bug scenario that caused the migration crash
			name:      "DO block with semicolon followed by ALTER statement splits correctly",
			input:     "DO $$ BEGIN\n    ALTER TABLE t ADD CONSTRAINT c CHECK (x > 0);\nEXCEPTION\n    WHEN OTHERS THEN null;\nEND $$;\nALTER TABLE t VALIDATE CONSTRAINT c;",
			wantCount: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			got := runner.splitSQLStatements(tc.input)
			if len(got) != tc.wantCount {
				t.Fatalf("splitSQLStatements() returned %d statements, want %d\nstatements: %v", len(got), tc.wantCount, got)
			}
			if tc.wantFirst != "" && len(got) > 0 {
				if !strings.Contains(got[0], tc.wantFirst) {
					t.Errorf("first statement = %q, want it to contain %q", got[0], tc.wantFirst)
				}
			}
		})
	}
}

func TestSplitSQLStatements_AdditionalCases(t *testing.T) {

	runner := newTestRunner()

	tests := []struct {
		name      string
		input     string
		wantCount int
	}{
		{
			name:      "nested single quotes (escaped) not split",
			input:     `SELECT 'it''s a test';`,
			wantCount: 1,
		},
		{
			// The implementation preserves comment text as part of statement content;
			// comment-only input without a trailing semicolon yields 1 "statement" (the comment text).
			name:      "only comments without semicolons yields one statement",
			input:     "-- just a comment\n/* another comment */",
			wantCount: 1,
		},
		{
			name:      "empty string yields zero statements",
			input:     "",
			wantCount: 0,
		},
		{
			name:      "mixed dollar-quoted and regular statements",
			input:     "SELECT 1; DO $$ BEGIN NULL; END $$; SELECT 2;",
			wantCount: 3,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			got := runner.splitSQLStatements(tc.input)
			if len(got) != tc.wantCount {
				t.Fatalf("splitSQLStatements(%q) returned %d statements, want %d\nstatements: %v",
					tc.input, len(got), tc.wantCount, got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// getMigrationFiles
// ---------------------------------------------------------------------------

func TestGetMigrationFiles(t *testing.T) {

	t.Run("empty directory returns empty slice no error", func(t *testing.T) {

		dir := t.TempDir()
		runner := &MigrationRunner{db: nil, migrationsPath: dir, cleanSQL: true}

		files, err := runner.getMigrationFiles()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(files) != 0 {
			t.Fatalf("expected empty files slice, got %v", files)
		}
	})

	t.Run("directory with 3 sql files returns 3 sorted entries", func(t *testing.T) {

		dir := t.TempDir()
		names := []string{"003_c.sql", "001_a.sql", "002_b.sql"}
		for _, name := range names {
			if err := os.WriteFile(filepath.Join(dir, name), []byte("SELECT 1;"), 0o600); err != nil {
				t.Fatalf("failed to create test file: %v", err)
			}
		}

		runner := &MigrationRunner{db: nil, migrationsPath: dir, cleanSQL: true}
		files, err := runner.getMigrationFiles()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(files) != 3 {
			t.Fatalf("expected 3 files, got %d: %v", len(files), files)
		}
		// Verify sorted order
		if !strings.HasSuffix(files[0], "001_a.sql") {
			t.Errorf("expected first file to be 001_a.sql, got %s", files[0])
		}
		if !strings.HasSuffix(files[1], "002_b.sql") {
			t.Errorf("expected second file to be 002_b.sql, got %s", files[1])
		}
		if !strings.HasSuffix(files[2], "003_c.sql") {
			t.Errorf("expected third file to be 003_c.sql, got %s", files[2])
		}
	})

	t.Run("non-existent path returns error", func(t *testing.T) {

		runner := &MigrationRunner{db: nil, migrationsPath: "/tmp/nonexistent-migrations-dir-alita-test", cleanSQL: true}
		_, err := runner.getMigrationFiles()
		if err == nil {
			t.Fatalf("expected error for non-existent path, got nil")
		}
	})

	t.Run("mixed sql and txt files returns only sql files", func(t *testing.T) {

		dir := t.TempDir()
		sqlFiles := []string{"001_migration.sql", "002_migration.sql"}
		otherFiles := []string{"readme.txt", "notes.md"}

		for _, name := range sqlFiles {
			if err := os.WriteFile(filepath.Join(dir, name), []byte("SELECT 1;"), 0o600); err != nil {
				t.Fatalf("failed to create sql file: %v", err)
			}
		}
		for _, name := range otherFiles {
			if err := os.WriteFile(filepath.Join(dir, name), []byte("not sql"), 0o600); err != nil {
				t.Fatalf("failed to create non-sql file: %v", err)
			}
		}

		runner := &MigrationRunner{db: nil, migrationsPath: dir, cleanSQL: true}
		files, err := runner.getMigrationFiles()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(files) != 2 {
			t.Fatalf("expected 2 sql files, got %d: %v", len(files), files)
		}
		for _, f := range files {
			if !strings.HasSuffix(f, ".sql") {
				t.Errorf("expected only .sql files, got %s", f)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// RunMigrations and applyMigration
// ---------------------------------------------------------------------------

func TestRunMigrations_RecordsAndSkipsAppliedFiles(t *testing.T) {
	skipIfNoDb(t)

	dir := t.TempDir()
	migrationPath := filepath.Join(dir, "001_create_test_table.sql")
	if err := os.WriteFile(migrationPath, []byte(`
CREATE TABLE migration_runner_test_items (
	id INTEGER PRIMARY KEY,
	name TEXT
);
INSERT INTO migration_runner_test_items (id, name) VALUES (1, 'first');
`), 0o600); err != nil {
		t.Fatalf("failed to write migration file: %v", err)
	}

	runner := &MigrationRunner{db: DB, migrationsPath: dir, cleanSQL: true}
	if err := runner.RunMigrations(); err != nil {
		t.Fatalf("RunMigrations() first run error = %v", err)
	}

	var itemCount int64
	if err := DB.Table("migration_runner_test_items").Count(&itemCount).Error; err != nil {
		t.Fatalf("failed to count migrated rows: %v", err)
	}
	if itemCount != 1 {
		t.Fatalf("migrated row count = %d, want 1", itemCount)
	}

	if !runner.isMigrationApplied(filepath.Base(migrationPath)) {
		t.Fatalf("migration %s was not recorded as applied", filepath.Base(migrationPath))
	}

	if err := runner.RunMigrations(); err != nil {
		t.Fatalf("RunMigrations() second run error = %v", err)
	}
	if err := DB.Table("migration_runner_test_items").Count(&itemCount).Error; err != nil {
		t.Fatalf("failed to count migrated rows after second run: %v", err)
	}
	if itemCount != 1 {
		t.Fatalf("migrated row count after skipped run = %d, want 1", itemCount)
	}

	t.Cleanup(func() {
		_ = DB.Exec("DROP TABLE IF EXISTS migration_runner_test_items").Error
		_ = DB.Where("version = ?", filepath.Base(migrationPath)).Delete(&SchemaMigration{}).Error
	})
}

func TestApplyMigration_EmptyFileDoesNotRecordVersion(t *testing.T) {
	skipIfNoDb(t)

	dir := t.TempDir()
	version := "002_empty.sql"
	migrationPath := filepath.Join(dir, version)
	if err := os.WriteFile(migrationPath, []byte("   \n\t  "), 0o600); err != nil {
		t.Fatalf("failed to write empty migration file: %v", err)
	}

	runner := &MigrationRunner{db: DB, migrationsPath: dir, cleanSQL: true}
	if err := runner.ensureMigrationsTable(); err != nil {
		t.Fatalf("ensureMigrationsTable() error = %v", err)
	}
	if err := runner.applyMigration(migrationPath, version); err != nil {
		t.Fatalf("applyMigration(empty) error = %v", err)
	}
	if runner.isMigrationApplied(version) {
		t.Fatalf("empty migration %s was recorded as applied", version)
	}
}

func TestApplyMigration_RollsBackFailedStatement(t *testing.T) {
	skipIfNoDb(t)

	dir := t.TempDir()
	version := "003_rollback.sql"
	migrationPath := filepath.Join(dir, version)
	if err := os.WriteFile(migrationPath, []byte(`
CREATE TABLE migration_runner_rollback_items (
	id INTEGER PRIMARY KEY
);
INSERT INTO migration_runner_missing_table (id) VALUES (1);
`), 0o600); err != nil {
		t.Fatalf("failed to write rollback migration file: %v", err)
	}

	runner := &MigrationRunner{db: DB, migrationsPath: dir, cleanSQL: true}
	if err := runner.ensureMigrationsTable(); err != nil {
		t.Fatalf("ensureMigrationsTable() error = %v", err)
	}
	err := runner.applyMigration(migrationPath, version)
	if err == nil {
		t.Fatal("applyMigration(failing) error = nil, want statement failure")
	}
	if !strings.Contains(err.Error(), "Statement preview") {
		t.Fatalf("applyMigration(failing) error %q missing statement preview", err)
	}
	if runner.isMigrationApplied(version) {
		t.Fatalf("failing migration %s was recorded as applied", version)
	}
	if DB.Migrator().HasTable("migration_runner_rollback_items") {
		t.Fatalf("failed migration left migration_runner_rollback_items table behind")
	}
}

func TestApplyMigration_RejectsUnsafePath(t *testing.T) {
	skipIfNoDb(t)

	dir := t.TempDir()
	runner := &MigrationRunner{db: DB, migrationsPath: dir, cleanSQL: true}

	tests := []struct {
		name string
		path string
	}{
		{name: "outside directory", path: filepath.Join(t.TempDir(), "001_outside.sql")},
		{name: "parent directory segment", path: dir + "/../001_parent.sql"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := runner.applyMigration(tc.path, filepath.Base(tc.path))
			if err == nil {
				t.Fatal("applyMigration() error = nil, want unsafe path rejection")
			}
			if !strings.Contains(err.Error(), "migration file path") {
				t.Fatalf("applyMigration() error = %q, want path validation error", err)
			}
		})
	}
}

func TestNewMigrationRunnerUsesConfiguredPath(t *testing.T) {
	previousConfig := config.AppConfig
	t.Cleanup(func() {
		config.AppConfig = previousConfig
	})
	config.AppConfig = &config.Config{MigrationsPath: "custom-migrations"}

	runner := NewMigrationRunner(&gorm.DB{})
	if runner.migrationsPath != "custom-migrations" {
		t.Fatalf("migrationsPath = %q, want custom-migrations", runner.migrationsPath)
	}
	if !runner.cleanSQL {
		t.Fatal("cleanSQL = false, want true")
	}
}

// ---------------------------------------------------------------------------
// findDollarQuoteBlocks
// ---------------------------------------------------------------------------

func TestFindDollarQuoteBlocks(t *testing.T) {

	tests := []struct {
		name      string
		input     string
		wantCount int
		wantStart []int
	}{
		{
			name:      "empty string returns empty",
			input:     "",
			wantCount: 0,
		},
		{
			name:      "simple dollar-quoted block",
			input:     `DO $$ BEGIN NULL; END $$;`,
			wantCount: 1,
			wantStart: []int{3},
		},
		{
			name:      "multiple dollar-quoted blocks",
			input:     `DO $$ a $$; DO $$ b $$;`,
			wantCount: 2,
			wantStart: []int{3, 15},
		},
		{
			name:      "dollar quote inside single quotes is ignored",
			input:     `SELECT '$$ not a block $$';`,
			wantCount: 0,
		},
		{
			name:      "dollar quote inside double quotes is ignored",
			input:     `SELECT "$$ not a block $$";`,
			wantCount: 0,
		},
		{
			name: "dollar quote inside line comment is ignored",
			input: `-- $$ not a block $$
SELECT 1;`,
			wantCount: 0,
		},
		{
			name:      "dollar quote inside block comment is ignored",
			input:     `/* $$ not a block $$ */ SELECT 1;`,
			wantCount: 0,
		},
		{
			name:      "tagged dollar-quoted block",
			input:     `$func$ BEGIN RAISE NOTICE 'hello'; END $func$;`,
			wantCount: 1,
			wantStart: []int{0},
		},
		{
			name:      "mismatched tags not closed",
			input:     `$a$ content $b$;`,
			wantCount: 0,
		},
		{
			name:      "dollar quote after real block still works",
			input:     `DO $$ BEGIN END $$; SELECT $$x$$;`,
			wantCount: 2,
		},
		{
			name:      "semicolon inside dollar block does not split",
			input:     `DO $$ BEGIN; END $$; SELECT 1;`,
			wantCount: 1,
			wantStart: []int{3},
		},
		{
			name:      "escaped single quote inside dollar block",
			input:     `DO $$ BEGIN RAISE NOTICE ''it''''s''; END $$;`,
			wantCount: 1,
			wantStart: []int{3},
		},
		{
			name:      "no closing tag returns empty",
			input:     `DO $$ BEGIN NULL;`,
			wantCount: 0,
		},
		{
			name:      "plain SQL without dollar quotes returns empty",
			input:     `SELECT 1; SELECT 2;`,
			wantCount: 0,
		},
		{
			name:      "nested-looking single and double quotes do not interfere",
			input:     `SELECT "'$$'"; DO $$ x $$;`,
			wantCount: 1,
			wantStart: []int{18},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			got := findDollarQuoteBlocks(tc.input)
			if len(got) != tc.wantCount {
				t.Fatalf("findDollarQuoteBlocks(%q) returned %d blocks, want %d", tc.input, len(got), tc.wantCount)
			}
			for i := range got {
				if tc.wantStart != nil && got[i].start != tc.wantStart[i] {
					t.Errorf("block[%d].start = %d, want %d", i, got[i].start, tc.wantStart[i])
				}
				// Verify end > start and within input bounds
				if got[i].end <= got[i].start {
					t.Errorf("block[%d].end (%d) should be > start (%d)", i, got[i].end, got[i].start)
				}
				if got[i].end > len(tc.input) {
					t.Errorf("block[%d].end (%d) should be <= len(input) (%d)", i, got[i].end, len(tc.input))
				}
			}
		})
	}
}

func TestFindDollarQuoteBlocks_ByteOffsets(t *testing.T) {

	// Verify that byte offsets (not rune offsets) are returned for non-ASCII content.
	input := `DO $$ 日本語 $$;`
	got := findDollarQuoteBlocks(input)
	if len(got) != 1 {
		t.Fatalf("expected 1 block, got %d", len(got))
	}

	// start should be byte offset of first '$' after "DO "
	wantStart := 3 // byte offset of first '$'
	if got[0].start != wantStart {
		t.Errorf("start = %d, want %d", got[0].start, wantStart)
	}

	// Verify end > start and end <= len(input)
	if got[0].end <= got[0].start {
		t.Errorf("end (%d) should be > start (%d)", got[0].end, got[0].start)
	}
	if got[0].end > len(input) {
		t.Errorf("end (%d) should be <= len(input) (%d)", got[0].end, len(input))
	}
}
