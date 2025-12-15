package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// CommandsProcessed tracks total commands processed
	CommandsProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "alita_commands_processed_total",
			Help: "Total number of commands processed",
		},
		[]string{"command", "status"},
	)

	// MessagesProcessed tracks total messages processed
	MessagesProcessed = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "alita_messages_processed_total",
			Help: "Total number of messages processed",
		},
	)

	// DatabaseQueries tracks database query durations
	DatabaseQueries = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "alita_database_queries_duration_seconds",
			Help:    "Database query duration",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation", "table"},
	)

	// CacheHits tracks cache hit/miss rates
	CacheHits = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "alita_cache_hits_total",
			Help: "Total number of cache hits",
		},
		[]string{"cache_type", "result"},
	)

	// ActiveUsers tracks number of active users
	ActiveUsers = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "alita_active_users",
			Help: "Number of active users",
		},
	)

	// ActiveChats tracks number of active chats
	ActiveChats = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "alita_active_chats",
			Help: "Number of active chats",
		},
	)

	// ErrorRate tracks error occurrences
	ErrorRate = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "alita_errors_total",
			Help: "Total number of errors",
		},
		[]string{"error_type"},
	)

	// ResponseTime tracks API response times
	ResponseTime = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "alita_response_time_seconds",
			Help:    "API response time in seconds",
			Buckets: []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"endpoint"},
	)

	// GoroutineCount tracks current goroutine count
	GoroutineCount = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "alita_goroutines",
			Help: "Current number of goroutines",
		},
	)

	// MemoryUsage tracks memory usage in MB
	MemoryUsage = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "alita_memory_usage_mb",
			Help: "Current memory usage in MB",
		},
	)
)

// MetricsHandler returns the Prometheus HTTP handler for metrics endpoint
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}
