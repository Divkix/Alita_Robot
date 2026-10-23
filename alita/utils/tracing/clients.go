package tracing

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/divkix/Alita_Robot/alita/utils/metrics"
)

var noopSpan = trace.SpanFromContext(context.Background())

// StartClientSpan starts a client span for an outgoing call, but only under a
// span ctx already records. Most Bot API, Postgres and Redis calls still run
// on context.Background(); a span for each would open a one-span trace per
// call. Hot-path callers check the parent first, to skip building the name and
// attributes.
func StartClientSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	if !trace.SpanFromContext(ctx).IsRecording() {
		return ctx, noopSpan
	}
	return StartSpan(ctx, name, trace.WithSpanKind(trace.SpanKindClient), trace.WithAttributes(attrs...))
}

// BotAPITransport measures every Bot API request and traces those made under
// a recorded span. The request URL carries the bot token, so metrics and
// spans name the method only and never record the URL, and no trace context
// is sent to Telegram.
func BotAPITransport(base http.RoundTripper) http.RoundTripper {
	return botAPITransport{base: base}
}

type botAPITransport struct {
	base http.RoundTripper
}

func (t botAPITransport) RoundTrip(r *http.Request) (*http.Response, error) {
	method := metrics.BotAPIMethod(r.URL.Path)
	start := time.Now()
	resp, err := t.traceRoundTrip(r, method)
	// getUpdates long-polls, so its latency is the poll timeout, not Telegram's.
	if method != "getUpdates" {
		metrics.TelegramRequestDuration.WithLabelValues(method).Observe(time.Since(start).Seconds())
	}
	switch {
	case err != nil:
		metrics.TelegramRequestErrors.WithLabelValues(method, "transport").Inc()
	case resp.StatusCode >= http.StatusBadRequest:
		metrics.TelegramRequestErrors.WithLabelValues(method, strconv.Itoa(resp.StatusCode)).Inc()
	}
	return resp, err
}

func (t botAPITransport) traceRoundTrip(r *http.Request, method string) (*http.Response, error) {
	if !trace.SpanFromContext(r.Context()).IsRecording() {
		return t.base.RoundTrip(r)
	}
	name := r.Method + " /bot{token}/" + method
	_, span := StartClientSpan(r.Context(), name,
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.URLTemplate(name[len(r.Method)+1:]),
		semconv.ServerAddress(r.URL.Hostname()),
		semconv.ServerPort(serverPort(r.URL)),
	)
	defer span.End()

	resp, err := t.base.RoundTrip(r)
	if err != nil {
		RecordError(span, err)
		return resp, err
	}
	span.SetAttributes(semconv.HTTPResponseStatusCode(resp.StatusCode))
	if resp.StatusCode >= http.StatusBadRequest {
		span.SetAttributes(semconv.ErrorTypeKey.String(strconv.Itoa(resp.StatusCode)))
		span.SetStatus(codes.Error, "")
	}
	return resp, nil
}

func serverPort(u *url.URL) int {
	if port, err := strconv.Atoi(u.Port()); err == nil {
		return port
	}
	if u.Scheme == "http" {
		return 80
	}
	return 443
}

// RedisHook traces Redis commands run under a recorded span. It records the
// command name only: keys and arguments carry chat and user data.
type RedisHook struct{}

var _ redis.Hook = RedisHook{}

func (RedisHook) DialHook(next redis.DialHook) redis.DialHook {
	return next
}

func (RedisHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		if !trace.SpanFromContext(ctx).IsRecording() {
			return next(ctx, cmd)
		}
		return traceRedis(ctx, cmd.Name(), 0, func(ctx context.Context) error { return next(ctx, cmd) })
	}
}

func (RedisHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		if !trace.SpanFromContext(ctx).IsRecording() {
			return next(ctx, cmds)
		}
		return traceRedis(ctx, "pipeline", len(cmds), func(ctx context.Context) error { return next(ctx, cmds) })
	}
}

func traceRedis(ctx context.Context, operation string, batchSize int, call func(context.Context) error) error {
	attrs := []attribute.KeyValue{semconv.DBSystemNameRedis, semconv.DBOperationName(operation)}
	if batchSize > 0 {
		attrs = append(attrs, semconv.DBOperationBatchSize(batchSize))
	}
	ctx, span := StartClientSpan(ctx, operation, attrs...)
	defer span.End()
	err := call(ctx)
	if err != nil && !errors.Is(err, redis.Nil) {
		RecordError(span, err)
	}
	return err
}
