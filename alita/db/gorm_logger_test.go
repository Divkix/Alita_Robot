package db

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestGormLoggerOmitsRowValues(t *testing.T) {
	log, hook := logrustest.NewNullLogger()
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: newGormLogger(log)})
	if err != nil {
		t.Fatal(err)
	}
	var injected error
	if err := database.Callback().Raw().Before("gorm:raw").Register("test:fail", func(tx *gorm.DB) {
		if injected != nil {
			_ = tx.AddError(injected)
		}
	}); err != nil {
		t.Fatal(err)
	}

	for _, fail := range []error{
		nil, // a plain failure: the bound value must not be inlined into the SQL
		fmt.Errorf("failed to encode args[0]: unable to encode %q", "private text"),
		&pgconn.PgError{Code: "22P02", Message: `invalid input syntax for type integer: "private text"`},
	} {
		injected = fail
		hook.Reset()
		_ = database.Exec("SELECT * FROM missing_table WHERE note = ?", "private text").Error
		entries := hook.AllEntries()
		if len(entries) == 0 {
			t.Fatalf("failed statement (%v) was not logged", fail)
		}
		for _, entry := range entries {
			if strings.Contains(entry.Message, "private text") {
				t.Fatalf("log line carries a row value: %s", entry.Message)
			}
			if !strings.Contains(entry.Message, "gorm_logger_test.go") {
				t.Fatalf("log line lost the caller's file: %s", entry.Message)
			}
		}
	}
}
