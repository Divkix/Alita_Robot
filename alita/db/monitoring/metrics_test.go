package monitoring

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
)

func skipIfNoDb(t *testing.T) {
	t.Helper()
	if db.DB == nil {
		t.Skip("requires database connection")
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

func TestGetCurrentMetricsReturnsPoolStats(t *testing.T) {
	skipIfNoDb(t)

	metrics, err := GetCurrentMetrics()
	if err != nil {
		t.Fatalf("GetCurrentMetrics() error = %v", err)
	}
	if metrics == nil {
		t.Fatal("GetCurrentMetrics() metrics = nil, want values")
	}
	if metrics.OpenConnections < 0 {
		t.Fatalf("OpenConnections = %d, want non-negative", metrics.OpenConnections)
	}
	if metrics.InUse < 0 {
		t.Fatalf("InUse = %d, want non-negative", metrics.InUse)
	}
	if metrics.Idle < 0 {
		t.Fatalf("Idle = %d, want non-negative", metrics.Idle)
	}
}

func TestCollectMetricsAndLogMetrics(t *testing.T) {
	skipIfNoDb(t)

	sqlDB, err := db.DB.DB()
	if err != nil {
		t.Fatalf("DB.DB() error = %v", err)
	}

	metrics := collectMetrics(sqlDB)
	if metrics.OpenConnections < 0 {
		t.Fatalf("OpenConnections = %d, want non-negative", metrics.OpenConnections)
	}

	logMetrics(metrics)
}

func TestStartMonitoringHandlesNilAndCancellation(t *testing.T) {
	originalDB := db.DB
	db.DB = nil
	StartMonitoring(context.Background(), time.Millisecond)
	db.DB = originalDB

	skipIfNoDb(t)

	ctx, cancel := context.WithCancel(context.Background())
	StartMonitoring(ctx, time.Millisecond)
	time.Sleep(2 * time.Millisecond)
	cancel()
	time.Sleep(2 * time.Millisecond)
}
