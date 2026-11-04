package db

import (
	"fmt"
	"os"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// A migration adds a column while statements prepared before it are still
// cached on a server session: the bot's own pool during a rollout, or
// PgBouncer's shared server connections for the process that starts next.
// Reading the table again must still work.
func TestPreparedSelectSurvivesAddedColumn(t *testing.T) {
	skipIfNoDb(t)
	if DB.Name() != "postgres" {
		t.Skip("prepared statement plans need Postgres")
	}
	database, err := gorm.Open(postgres.Open(os.Getenv("DATABASE_URL")), newGormConfig(logger.Default.LogMode(logger.Silent)))
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("sql db: %v", err)
	}
	// One session, so the second read runs the statement the first prepared.
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })

	type planProbe struct {
		ID   int64
		Name string
	}
	table := fmt.Sprintf("plan_probe_%d", time.Now().UnixNano())
	t.Cleanup(func() {
		if err := DB.Exec("DROP TABLE IF EXISTS " + table).Error; err != nil {
			t.Errorf("drop %s: %v", table, err)
		}
	})
	for _, stmt := range []string{
		"CREATE TABLE " + table + " (id bigint PRIMARY KEY, name text NOT NULL)",
		"INSERT INTO " + table + " VALUES (1, 'before')",
	} {
		if err := database.Exec(stmt).Error; err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}

	read := func() planProbe {
		t.Helper()
		var got planProbe
		if err := database.Table(table).Where("id = ?", 1).First(&got).Error; err != nil {
			t.Fatalf("read %s: %v", table, err)
		}
		return got
	}
	read()
	if err := database.Exec("ALTER TABLE " + table + " ADD COLUMN extra integer NOT NULL DEFAULT 0").Error; err != nil {
		t.Fatalf("add column: %v", err)
	}
	if got := read(); got != (planProbe{ID: 1, Name: "before"}) {
		t.Fatalf("read after ADD COLUMN = %+v", got)
	}
}

// A crash or failover resets the table statistics, after which n_live_tup
// counts only the rows inserted since: the count must still include the rows
// that were already there, whether or not the table was ever analyzed.
func TestTableRowCountSurvivesStatsReset(t *testing.T) {
	skipIfNoDb(t)
	if DB.Name() != "postgres" {
		t.Skip("table statistics need Postgres")
	}
	for _, analyzed := range []bool{true, false} {
		t.Run(fmt.Sprintf("analyzed=%t", analyzed), func(t *testing.T) {
			table := fmt.Sprintf("row_count_probe_%d", time.Now().UnixNano())
			t.Cleanup(func() {
				if err := DB.Exec("DROP TABLE IF EXISTS " + table).Error; err != nil {
					t.Errorf("drop %s: %v", table, err)
				}
			})
			stmts := []string{
				"CREATE TABLE " + table + " (id bigint PRIMARY KEY)",
				"INSERT INTO " + table + " SELECT generate_series(1, 1000)",
			}
			if analyzed {
				stmts = append(stmts, "ANALYZE "+table)
			}
			stmts = append(stmts,
				"SELECT pg_stat_force_next_flush()",
				"SELECT pg_stat_reset_single_table_counters('"+table+"'::regclass)",
				"INSERT INTO "+table+" SELECT generate_series(1001, 1005)",
				"SELECT pg_stat_force_next_flush()",
			)
			// One connection: pg_stat_force_next_flush only flushes its own backend's counters.
			err := DB.Connection(func(conn *gorm.DB) error {
				for _, stmt := range stmts {
					if err := conn.Exec(stmt).Error; err != nil {
						return fmt.Errorf("%s: %w", stmt, err)
					}
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if got := TableRowCount(table); got != 1005 {
				t.Fatalf("TableRowCount(%s) = %d after a stats reset, want 1005", table, got)
			}
		})
	}
}

func TestGetSpanAttributes(t *testing.T) {
	tests := []struct {
		name         string
		model        any
		wantLen      int
		wantKey      string
		wantTypeName string
	}{
		{
			name:    "nil input",
			model:   nil,
			wantLen: 0,
		},
		{
			name:         "struct pointer",
			model:        &User{},
			wantLen:      1,
			wantKey:      "db.model",
			wantTypeName: "*models.User",
		},
		{
			name:         "string value",
			model:        "hello",
			wantLen:      1,
			wantKey:      "db.model",
			wantTypeName: "string",
		},
		{
			name:         "int value",
			model:        42,
			wantLen:      1,
			wantKey:      "db.model",
			wantTypeName: "int",
		},
		{
			name:         "slice value",
			model:        []int{1, 2, 3},
			wantLen:      1,
			wantKey:      "db.model",
			wantTypeName: "[]int",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := getSpanAttributes(tc.model)

			if len(got) != tc.wantLen {
				t.Fatalf("getSpanAttributes() len = %d, want %d", len(got), tc.wantLen)
			}

			if tc.wantLen == 0 {
				return
			}

			if len(got) != 1 {
				t.Fatalf("expected exactly 1 attribute, got %d", len(got))
			}

			attr := got[0]
			if string(attr.Key) != tc.wantKey {
				t.Errorf("attribute key = %q, want %q", attr.Key, tc.wantKey)
			}

			val := attr.Value
			if val.AsString() != tc.wantTypeName {
				t.Errorf("attribute value = %q, want %q", val.AsString(), tc.wantTypeName)
			}
		})
	}
}
