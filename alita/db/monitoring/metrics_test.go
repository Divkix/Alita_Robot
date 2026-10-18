package monitoring

import (
	"context"
	"errors"
	"testing"

	"github.com/divkix/Alita_Robot/alita/db"
)

func skipIfNoDb(t *testing.T) {
	t.Helper()
	if db.DB == nil {
		t.Fatal("test database was not initialized")
	}
}

func TestGetCurrentMetricsRequiresInitializedDatabase(t *testing.T) {
	originalDB := db.DB
	db.DB = nil
	t.Cleanup(func() {
		db.DB = originalDB
	})

	metrics, err := GetCurrentMetrics()
	if !errors.Is(err, ErrDatabaseNotInitialized) {
		t.Fatalf("GetCurrentMetrics() error = %v, want ErrDatabaseNotInitialized", err)
	}
	if metrics != nil {
		t.Fatalf("GetCurrentMetrics() metrics = %+v, want nil", metrics)
	}
}

func TestGetCurrentMetricsReportsHeldConnection(t *testing.T) {
	skipIfNoDb(t)

	sqlDB, err := db.DB.DB()
	if err != nil {
		t.Fatalf("DB.DB() error = %v", err)
	}
	conn, err := sqlDB.Conn(context.Background())
	if err != nil {
		t.Fatalf("sqlDB.Conn() error = %v", err)
	}
	defer func() { _ = conn.Close() }()

	metrics, err := GetCurrentMetrics()
	if err != nil {
		t.Fatalf("GetCurrentMetrics() error = %v", err)
	}
	if metrics.InUse < 1 {
		t.Fatalf("InUse = %d while a connection is held, want >= 1", metrics.InUse)
	}
	if metrics.OpenConnections != metrics.InUse+metrics.Idle {
		t.Fatalf("OpenConnections = %d, want InUse %d + Idle %d", metrics.OpenConnections, metrics.InUse, metrics.Idle)
	}
}
