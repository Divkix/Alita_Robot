package db

import (
	"context"
	"database/sql"
	"errors"
	"time"

	log "github.com/sirupsen/logrus"
)

// ErrDatabaseNotInitialized is returned when database operations are attempted
// before the database has been initialized
var ErrDatabaseNotInitialized = errors.New("database not initialized")

// DatabaseMetrics holds database performance metrics
type DatabaseMetrics struct {
	OpenConnections   int           `json:"open_connections"`
	InUse             int           `json:"in_use"`
	Idle              int           `json:"idle"`
	WaitCount         int64         `json:"wait_count"`
	WaitDuration      time.Duration `json:"wait_duration"`
	MaxIdleClosed     int64         `json:"max_idle_closed"`
	MaxLifetimeClosed int64         `json:"max_lifetime_closed"`
}

// StartMonitoring begins periodic database metrics collection
func StartMonitoring(ctx context.Context, interval time.Duration) {
	if DB == nil {
		log.Warn("[DBMonitoring] Database not initialized, skipping monitoring")
		return
	}

	sqlDB, err := DB.DB()
	if err != nil {
		log.Errorf("[DBMonitoring] Failed to get SQL DB: %v", err)
		return
	}

	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Info("[DBMonitoring] Stopping database monitoring")
				return
			case <-ticker.C:
				metrics := collectMetrics(sqlDB)
				logMetrics(metrics)
			}
		}
	}()

	log.Info("[DBMonitoring] Started database performance monitoring")
}

// collectMetrics gathers current database pool statistics
func collectMetrics(db *sql.DB) DatabaseMetrics {
	stats := db.Stats()
	return DatabaseMetrics{
		OpenConnections:   stats.OpenConnections,
		InUse:             stats.InUse,
		Idle:              stats.Idle,
		WaitCount:         stats.WaitCount,
		WaitDuration:      stats.WaitDuration,
		MaxIdleClosed:     stats.MaxIdleClosed,
		MaxLifetimeClosed: stats.MaxLifetimeClosed,
	}
}

// logMetrics logs database metrics in a structured format
func logMetrics(metrics DatabaseMetrics) {
	log.Debugf("[DBMonitoring] Connections: %d open, %d in-use, %d idle | Wait: %d calls, %v total | Closed: %d (max idle), %d (max lifetime)",
		metrics.OpenConnections,
		metrics.InUse,
		metrics.Idle,
		metrics.WaitCount,
		metrics.WaitDuration,
		metrics.MaxIdleClosed,
		metrics.MaxLifetimeClosed,
	)
}

// GetCurrentMetrics returns current database pool metrics
func GetCurrentMetrics() (*DatabaseMetrics, error) {
	if DB == nil {
		return nil, ErrDatabaseNotInitialized
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return nil, err
	}

	stats := sqlDB.Stats()
	metrics := DatabaseMetrics{
		OpenConnections:   stats.OpenConnections,
		InUse:             stats.InUse,
		Idle:              stats.Idle,
		WaitCount:         stats.WaitCount,
		WaitDuration:      stats.WaitDuration,
		MaxIdleClosed:     stats.MaxIdleClosed,
		MaxLifetimeClosed: stats.MaxLifetimeClosed,
	}

	return &metrics, nil
}
