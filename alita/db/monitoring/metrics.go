package monitoring

import (
	"context"
	"errors"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db"
)

// ErrDatabaseNotInitialized is returned when database operations are attempted
// before the database has been initialized
var ErrDatabaseNotInitialized = errors.New("database not initialized")

// DatabaseMetrics holds database performance metrics (dummy for MongoDB)
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
	log.Info("[DBMonitoring] Database metrics monitoring is not implemented for MongoDB")
}

// GetCurrentMetrics returns current database pool metrics
func GetCurrentMetrics() (*DatabaseMetrics, error) {
	if db.DB == nil {
		return nil, ErrDatabaseNotInitialized
	}

	return &DatabaseMetrics{}, nil
}
