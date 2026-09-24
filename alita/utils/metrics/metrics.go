// Package metrics defines the bot's Prometheus metrics. It imports only
// external packages so any internal package can depend on it without cycles.
//
// Labels must never carry chat IDs, user IDs or cache keys: each distinct
// value creates a new time series.
package metrics

import (
	"context"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/redis/go-redis/v9"
)

var (
	// UpdatesProcessed counts top-level Telegram updates handled by the dispatcher.
	UpdatesProcessed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "alita_updates_processed_total",
		Help: "Telegram updates processed by the dispatcher.",
	})

	// UpdateDuration observes how long one update took to process.
	UpdateDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "alita_update_duration_seconds",
		Help:    "Time spent processing one Telegram update.",
		Buckets: prometheus.DefBuckets,
	})

	// RedisCommands counts Redis commands sent, labelled by lower-case command name.
	RedisCommands = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "alita_redis_commands_total",
		Help: "Redis commands sent, by command name.",
	}, []string{"command"})

	// DBQueries counts GORM statements, labelled by callback operation.
	DBQueries = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "alita_db_queries_total",
		Help: "Database statements executed, by GORM operation.",
	}, []string{"operation"})
)

// RedisHook is a go-redis hook that counts every command, including each
// command inside a pipeline.
type RedisHook struct{}

var _ redis.Hook = RedisHook{}

func (RedisHook) DialHook(next redis.DialHook) redis.DialHook {
	return next
}

func (RedisHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		RedisCommands.WithLabelValues(strings.ToLower(cmd.Name())).Inc()
		return next(ctx, cmd)
	}
}

func (RedisHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		for _, cmd := range cmds {
			RedisCommands.WithLabelValues(strings.ToLower(cmd.Name())).Inc()
		}
		return next(ctx, cmds)
	}
}
