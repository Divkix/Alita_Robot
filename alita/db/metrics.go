package db

import (
	"errors"

	"gorm.io/gorm"

	"github.com/divkix/Alita_Robot/alita/utils/metrics"
)

// registerQueryMetrics counts every GORM statement in alita_db_queries_total,
// labelled by operation only.
func registerQueryMetrics(database *gorm.DB) error {
	count := func(operation string) func(*gorm.DB) {
		counter := metrics.DBQueries.WithLabelValues(operation)
		return func(*gorm.DB) { counter.Inc() }
	}
	cb := database.Callback()
	return errors.Join(
		cb.Query().After("gorm:query").Register("alita:metrics_query", count("query")),
		cb.Create().After("gorm:create").Register("alita:metrics_create", count("create")),
		cb.Update().After("gorm:update").Register("alita:metrics_update", count("update")),
		cb.Delete().After("gorm:delete").Register("alita:metrics_delete", count("delete")),
		cb.Row().After("gorm:row").Register("alita:metrics_row", count("row")),
		cb.Raw().After("gorm:raw").Register("alita:metrics_raw", count("raw")),
	)
}
