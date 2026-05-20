package main

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/divkix/Alita_Robot/alita/db"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestFindOrphanReportsCleanDatabase(t *testing.T) {
	gdb := newValidationTestDB(t)
	reports, err := findOrphanReports(gdb, defaultOrphanChecks())
	if err != nil {
		t.Fatalf("findOrphanReports() error = %v", err)
	}
	if len(reports) != 0 {
		t.Fatalf("findOrphanReports() reports = %#v, want none", reports)
	}
}

func TestFindOrphanReportsIncludesCleanupSQL(t *testing.T) {
	gdb := newValidationTestDB(t)
	if err := gdb.Exec("INSERT INTO admin (chat_id) VALUES (?)", int64(999)).Error; err != nil {
		t.Fatalf("insert orphan admin: %v", err)
	}
	if err := gdb.Exec("INSERT INTO devs (user_id) VALUES (?)", int64(888)).Error; err != nil {
		t.Fatalf("insert orphan dev: %v", err)
	}

	reports, err := findOrphanReports(gdb, defaultOrphanChecks())
	if err != nil {
		t.Fatalf("findOrphanReports() error = %v", err)
	}
	if len(reports) != 2 {
		t.Fatalf("findOrphanReports() len = %d, want 2: %#v", len(reports), reports)
	}

	got := map[string]OrphanReport{}
	for _, report := range reports {
		got[report.Table] = report
	}
	for table, sqlSnippet := range map[string]string{
		"admin": "DELETE FROM admin",
		"devs":  "DELETE FROM devs",
	} {
		report, ok := got[table]
		if !ok {
			t.Fatalf("missing report for %s in %#v", table, reports)
		}
		if report.Count != 1 {
			t.Fatalf("%s count = %d, want 1", table, report.Count)
		}
		if !strings.Contains(report.SQL, sqlSnippet) {
			t.Fatalf("%s cleanup SQL = %q, want %q", table, report.SQL, sqlSnippet)
		}
	}
}

func TestRunOrphanValidationWritesCleanAndDirtyResults(t *testing.T) {
	cleanDB := newValidationTestDB(t)
	var cleanOut bytes.Buffer
	if code := runOrphanValidation(cleanDB, &cleanOut); code != 0 {
		t.Fatalf("runOrphanValidation(clean) code = %d, want 0", code)
	}
	if !strings.Contains(cleanOut.String(), "No orphaned records found") {
		t.Fatalf("clean output missing success message: %q", cleanOut.String())
	}

	dirtyDB := newValidationTestDB(t)
	if err := dirtyDB.Exec("INSERT INTO admin (chat_id) VALUES (?)", int64(999)).Error; err != nil {
		t.Fatalf("insert orphan admin: %v", err)
	}
	var dirtyOut bytes.Buffer
	if code := runOrphanValidation(dirtyDB, &dirtyOut); code != 1 {
		t.Fatalf("runOrphanValidation(dirty) code = %d, want 1", code)
	}
	for _, want := range []string{"Found 1 types of orphaned records", "DELETE FROM admin"} {
		if !strings.Contains(dirtyOut.String(), want) {
			t.Fatalf("dirty output missing %q: %q", want, dirtyOut.String())
		}
	}
}

func TestRunOrphanValidationReportsFailures(t *testing.T) {
	var out bytes.Buffer
	if code := runOrphanValidation(nil, &out); code != 1 {
		t.Fatalf("runOrphanValidation(nil) code = %d, want 1", code)
	}
	if out.Len() != 0 {
		t.Fatalf("runOrphanValidation(nil) output = %q, want empty", out.String())
	}

	gdb := newValidationTestDB(t)
	if code := runOrphanValidation(gdb, failingWriter{}); code != 1 {
		t.Fatalf("runOrphanValidation(write failure) code = %d, want 1", code)
	}
}

func TestFindOrphanReportsReturnsQueryError(t *testing.T) {
	gdb := newValidationTestDB(t)
	reports, err := findOrphanReports(gdb, []orphanCheck{
		{
			table:     "missing_table",
			condition: "chat_id NOT IN (SELECT chat_id FROM chats)",
			issue:     "missing table",
			cleanup:   "DELETE FROM missing_table;",
		},
	})
	if err == nil {
		t.Fatal("findOrphanReports() error = nil, want query error")
	}
	if reports != nil {
		t.Fatalf("findOrphanReports() reports = %#v, want nil on error", reports)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

func newValidationTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gdb, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "validation.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		t.Fatalf("sqlite DB handle: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	oldDB := db.DB
	db.DB = gdb
	t.Cleanup(func() {
		db.DB = oldDB
	})

	if err := gdb.Exec("CREATE TABLE chats (chat_id INTEGER PRIMARY KEY)").Error; err != nil {
		t.Fatalf("create chats: %v", err)
	}
	if err := gdb.Exec("CREATE TABLE users (user_id INTEGER PRIMARY KEY)").Error; err != nil {
		t.Fatalf("create users: %v", err)
	}
	for _, check := range defaultOrphanChecks() {
		columns := "chat_id INTEGER"
		switch check.table {
		case "devs", "report_user_settings":
			columns = "user_id INTEGER"
		case "chat_users", "connection", "warns_users":
			columns = "chat_id INTEGER, user_id INTEGER"
		case "channels":
			columns = "chat_id INTEGER, channel_id INTEGER"
		}
		if err := gdb.Exec("CREATE TABLE IF NOT EXISTS " + check.table + " (" + columns + ")").Error; err != nil {
			t.Fatalf("create %s: %v", check.table, err)
		}
	}
	return gdb
}
