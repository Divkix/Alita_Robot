package db

import (
	"errors"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
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
		timedBefore := queryDurationCount(t, op)
		fn()
		if timed := queryDurationCount(t, op) - timedBefore; timed != 1 {
			t.Fatalf("%s duration observations delta = %d, want 1", op, timed)
		}
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

func TestRegisterQueryMetricsCountsFailuresOnly(t *testing.T) {
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
	failures := metrics.DBQueryErrors.WithLabelValues("query")
	before := testutil.ToFloat64(failures)

	if err := database.First(&metricsProbe{}, 42).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("First() error = %v, want ErrRecordNotFound", err)
	}
	if err := database.Table("missing_table").Find(&[]metricsProbe{}).Error; err == nil {
		t.Fatal("query on a missing table succeeded")
	}
	if got := testutil.ToFloat64(failures) - before; got != 1 {
		t.Fatalf("query failures delta = %v, want 1 (a missing row is not a failure)", got)
	}
}

// queryDurationCount returns how many statements of operation
// alita_db_query_duration_seconds has observed.
func queryDurationCount(t *testing.T, operation string) uint64 {
	t.Helper()
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range families {
		if family.GetName() != "alita_db_query_duration_seconds" {
			continue
		}
		for _, m := range family.GetMetric() {
			for _, label := range m.GetLabel() {
				if label.GetName() == "operation" && label.GetValue() == operation {
					return m.GetHistogram().GetSampleCount()
				}
			}
		}
	}
	return 0
}
