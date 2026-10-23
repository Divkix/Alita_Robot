package db

import (
	"database/sql"
	"errors"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"gorm.io/gorm"

	"github.com/divkix/Alita_Robot/alita/utils/metrics"
)

type queryStartKey struct{}

// registerQueryMetrics counts, times and counts failures of every GORM
// statement, labelled by operation only.
func registerQueryMetrics(database *gorm.DB) error {
	start := func(tx *gorm.DB) { tx.Statement.Settings.Store(queryStartKey{}, time.Now()) }
	end := func(operation string) func(*gorm.DB) {
		count := metrics.DBQueries.WithLabelValues(operation)
		duration := metrics.DBQueryDuration.WithLabelValues(operation)
		failures := metrics.DBQueryErrors.WithLabelValues(operation)
		return func(tx *gorm.DB) {
			count.Inc()
			if started, ok := tx.Statement.Settings.LoadAndDelete(queryStartKey{}); ok {
				if t, ok := started.(time.Time); ok {
					duration.Observe(time.Since(t).Seconds())
				}
			}
			if tx.Error != nil && !errors.Is(tx.Error, gorm.ErrRecordNotFound) {
				failures.Inc()
			}
		}
	}
	cb := database.Callback()
	return errors.Join(
		cb.Query().Before("gorm:query").Register("alita:metrics_query_start", start),
		cb.Query().After("gorm:query").Register("alita:metrics_query", end("query")),
		cb.Create().Before("gorm:create").Register("alita:metrics_create_start", start),
		cb.Create().After("gorm:create").Register("alita:metrics_create", end("create")),
		cb.Update().Before("gorm:update").Register("alita:metrics_update_start", start),
		cb.Update().After("gorm:update").Register("alita:metrics_update", end("update")),
		cb.Delete().Before("gorm:delete").Register("alita:metrics_delete_start", start),
		cb.Delete().After("gorm:delete").Register("alita:metrics_delete", end("delete")),
		cb.Row().Before("gorm:row").Register("alita:metrics_row_start", start),
		cb.Row().After("gorm:row").Register("alita:metrics_row", end("row")),
		cb.Raw().Before("gorm:raw").Register("alita:metrics_raw_start", start),
		cb.Raw().After("gorm:raw").Register("alita:metrics_raw", end("raw")),
	)
}

// registerPoolMetrics exports the connection pool's stats as go_sql_*
// metrics with db_name="alita".
func registerPoolMetrics(sqlDB *sql.DB) error {
	err := prometheus.Register(collectors.NewDBStatsCollector(sqlDB, "alita"))
	if errors.As(err, new(prometheus.AlreadyRegisteredError)) {
		return nil
	}
	return err
}
