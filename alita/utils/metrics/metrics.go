// Package metrics defines the bot's Prometheus metrics. It imports only
// external packages so any internal package can depend on it without cycles.
//
// Labels must never carry chat IDs, user IDs or cache keys: each distinct
// value creates a new time series.
package metrics

import (
	"context"
	"path"
	"strings"
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/redis/go-redis/v9"
)

var (
	// UpdatesProcessed counts top-level Telegram updates handled by the
	// dispatcher, by update type, outcome and command (see CommandLabel).
	UpdatesProcessed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "alita_updates_processed_total",
		Help: "Telegram updates processed by the dispatcher, by update type, outcome and command.",
	}, []string{"update_type", "outcome", "command"})

	// HandlerErrors counts errors handlers returned, by the module of the
	// command that failed ("other" for non-command updates) and whether the
	// error is an expected Telegram one.
	HandlerErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "alita_handler_errors_total",
		Help: "Errors returned by update handlers, by module and whether the error was expected.",
	}, []string{"module", "expected"})

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

	// DBQueryDuration observes GORM statement latency by callback operation.
	DBQueryDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "alita_db_query_duration_seconds",
		Help:    "Database statement latency, by GORM operation.",
		Buckets: prometheus.DefBuckets,
	}, []string{"operation"})

	// DBQueryErrors counts failed GORM statements by callback operation,
	// including errors GORM raises before running SQL and cancelled
	// contexts. A lookup that finds no row is not a failure.
	DBQueryErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "alita_db_query_errors_total",
		Help: "Database statements that failed, by GORM operation.",
	}, []string{"operation"})

	// TelegramRequestDuration observes Bot API request latency, up to the
	// response headers (before any error-envelope inspection), by method
	// (see BotAPIMethod). getUpdates is left out: it long-polls.
	TelegramRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "alita_telegram_api_request_duration_seconds",
		Help:    "Bot API request latency, by method.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method"})

	// TelegramRequestErrors counts failed Bot API requests by method and
	// code: the HTTP status (429 is a rate limit), or "transport" when no
	// response came back. Known getChatMember HTTP 400 missing-member replies
	// are expected negative lookups and excluded; missing chats, missing rights
	// and the same descriptions from other methods remain failures.
	TelegramRequestErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "alita_telegram_api_errors_total",
		Help: "Failed Bot API requests, by method and HTTP status or \"transport\".",
	}, []string{"method", "code"})

	// WebhookQueueDepth is the number of webhook updates accepted but not yet
	// picked up by a dispatcher worker.
	WebhookQueueDepth = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "alita_webhook_queue_depth",
		Help: "Webhook updates waiting for a dispatcher worker.",
	})

	// WebhookRejected counts webhook updates answered with 503, by reason:
	// "queue_full" or "stopped".
	WebhookRejected = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "alita_webhook_rejected_total",
		Help: "Webhook updates rejected with 503, by reason.",
	}, []string{"reason"})

	// WebhookQueueFull and WebhookStopped are WebhookRejected's two series,
	// created up front so alerts see a zero before the first rejection.
	WebhookQueueFull = WebhookRejected.WithLabelValues("queue_full")
	WebhookStopped   = WebhookRejected.WithLabelValues("stopped")

	// JobRuns counts background job ticks by job name (a fixed string).
	JobRuns = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "alita_job_runs_total",
		Help: "Background job ticks run, by job.",
	}, []string{"job"})

	// JobFailures counts job ticks that returned an error or panicked.
	JobFailures = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "alita_job_failures_total",
		Help: "Background job ticks that failed or panicked, by job.",
	}, []string{"job"})

	// CacheLocalHits counts read-through lookups served from process memory.
	CacheLocalHits = promauto.NewCounter(prometheus.CounterOpts{
		Name: "alita_cache_local_hits_total",
		Help: "Read-through cache lookups served by the in-process layer.",
	})

	// CacheLocalMisses counts read-through lookups the in-process layer could
	// not serve (they fall through to Redis).
	CacheLocalMisses = promauto.NewCounter(prometheus.CounterOpts{
		Name: "alita_cache_local_misses_total",
		Help: "Read-through cache lookups that missed the in-process layer.",
	})
)

// Outcome is how an update's handling ended, ordered from best to worst.
type Outcome int

const (
	OutcomeOK Outcome = iota
	OutcomeExpectedError
	OutcomeError
	OutcomePanic
)

var outcomeLabels = [...]string{"ok", "expected_error", "error", "panic"}

// String returns the outcome's label value.
func (o Outcome) String() string {
	if o < 0 || int(o) >= len(outcomeLabels) {
		return Other
	}
	return outcomeLabels[o]
}

// Other is the label for a command or module outside the known set: label
// values must not come straight from user text.
const Other = "other"

var commandModules atomic.Pointer[map[string]string]

// SetCommandModules sets the known commands (without "/") and the module of
// each. Commands outside it are labelled Other.
func SetCommandModules(modules map[string]string) {
	commandModules.Store(&modules)
}

// CommandLabel returns cmd if it is a known command, "" for no command, and
// Other for anything else.
func CommandLabel(cmd string) string {
	if cmd == "" {
		return ""
	}
	if _, ok := lookupModule(cmd); ok {
		return cmd
	}
	return Other
}

// CommandModule returns the module of a known command, or Other.
func CommandModule(cmd string) string {
	if module, ok := lookupModule(cmd); ok {
		return module
	}
	return Other
}

func lookupModule(cmd string) (string, bool) {
	modules := commandModules.Load()
	if modules == nil {
		return "", false
	}
	module, ok := (*modules)[cmd]
	return module, ok
}

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

// BotAPIMethod returns the method from a /bot<token>/<method> path, or
// "unknown" when the last segment is not a plain method name, so a malformed
// path can neither leak the token nor grow a label's cardinality.
func BotAPIMethod(p string) string {
	method := path.Base(p)
	if len(method) > 64 {
		return "unknown"
	}
	for _, c := range method {
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') {
			return "unknown"
		}
	}
	return method
}
