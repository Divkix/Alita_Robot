package db

import (
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
