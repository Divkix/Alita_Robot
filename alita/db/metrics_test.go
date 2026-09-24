package db

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/divkix/Alita_Robot/alita/utils/metrics"
)

type metricsProbe struct {
	ID   uint
	Name string
}

func TestRegisterQueryMetricsCountsStatements(t *testing.T) {
	database, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := registerQueryMetrics(database); err != nil {
		t.Fatalf("registerQueryMetrics() error = %v", err)
	}
	if err := database.AutoMigrate(&metricsProbe{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	delta := func(op string, fn func()) float64 {
		before := testutil.ToFloat64(metrics.DBQueries.WithLabelValues(op))
		fn()
		return testutil.ToFloat64(metrics.DBQueries.WithLabelValues(op)) - before
	}

	if got := delta("create", func() {
		if err := database.Create(&metricsProbe{Name: "a"}).Error; err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}); got != 1 {
		t.Fatalf("create delta = %v, want 1", got)
	}
	if got := delta("query", func() {
		var rows []metricsProbe
		if err := database.Find(&rows).Error; err != nil {
			t.Fatalf("Find() error = %v", err)
		}
	}); got != 1 {
		t.Fatalf("query delta = %v, want 1", got)
	}
	if got := delta("update", func() {
		if err := database.Model(&metricsProbe{}).Where("name = ?", "a").Update("name", "b").Error; err != nil {
			t.Fatalf("Update() error = %v", err)
		}
	}); got != 1 {
		t.Fatalf("update delta = %v, want 1", got)
	}
	if got := delta("raw", func() {
		if err := database.Exec("DELETE FROM metrics_probes WHERE name = ?", "none").Error; err != nil {
			t.Fatalf("Exec() error = %v", err)
		}
	}); got != 1 {
		t.Fatalf("raw delta = %v, want 1", got)
	}
	if got := delta("delete", func() {
		if err := database.Where("name = ?", "b").Delete(&metricsProbe{}).Error; err != nil {
			t.Fatalf("Delete() error = %v", err)
		}
	}); got != 1 {
		t.Fatalf("delete delta = %v, want 1", got)
	}
}
