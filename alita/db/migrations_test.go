package db

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	t.Parallel()

	got := SchemaMigration{}.TableName()
	if got != "schema_migrations" {
		t.Fatalf("SchemaMigration.TableName() = %q, want %q", got, "schema_migrations")
	}
}

// ---------------------------------------------------------------------------
// cleanSupabaseSQL
// ---------------------------------------------------------------------------

func TestCleanSupabaseSQL(t *testing.T) {
	t.Parallel()

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
			t.Parallel()

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
	t.Parallel()

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
			name:  "CREATE TYPE ENUM wrapped in DO block",
			input: `CREATE TYPE mood AS ENUM ('happy', 'sad', 'neutral');`,
			wantParts: []string{
				"DO $$",
				"CREATE TYPE mood AS ENUM",
				"EXCEPTION",
				"END $$",
			},
			wantGone: []string{"CREATE TYPE mood AS ENUM ('happy', 'sad', 'neutral');"},
		},
		{
			name:  "ALTER TABLE ADD CONSTRAINT wrapped in DO block",
			input: `ALTER TABLE orders ADD CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES users(id);`,
			wantParts: []string{
				"DO $$",
				"ALTER TABLE orders ADD CONSTRAINT fk_user",
				"EXCEPTION",
				"END $$",
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
			t.Parallel()

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
	t.Parallel()

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
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

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
	t.Parallel()

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
			name:      "only comments yields zero statements",
			input:     "-- just a comment\n/* another comment */",
			wantCount: 0,
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
			t.Parallel()

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
	t.Parallel()

	t.Run("empty directory returns empty slice no error", func(t *testing.T) {
		t.Parallel()

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
		t.Parallel()

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
		t.Parallel()

		runner := &MigrationRunner{db: nil, migrationsPath: "/tmp/nonexistent-migrations-dir-alita-test", cleanSQL: true}
		_, err := runner.getMigrationFiles()
		if err == nil {
			t.Fatalf("expected error for non-existent path, got nil")
		}
	})

	t.Run("mixed sql and txt files returns only sql files", func(t *testing.T) {
		t.Parallel()

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
