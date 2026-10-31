package tracing

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
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
// is sent to Telegram. Error envelopes are inspected within a fixed bound and
// replayed unchanged; only fixed reason categories, never descriptions, escape.
func BotAPITransport(base http.RoundTripper) http.RoundTripper {
	return botAPITransport{base: base}
}

type botAPITransport struct {
	base http.RoundTripper
}

func (t botAPITransport) RoundTrip(r *http.Request) (*http.Response, error) {
	method := metrics.BotAPIMethod(r.URL.Path)
	span := noopSpan
	if trace.SpanFromContext(r.Context()).IsRecording() {
		name := r.Method + " /bot{token}/" + method
		_, span = StartClientSpan(r.Context(), name,
			semconv.HTTPRequestMethodKey.String(r.Method),
			semconv.URLTemplate(name[len(r.Method)+1:]),
			semconv.ServerAddress(r.URL.Hostname()),
			semconv.ServerPort(serverPort(r.URL)),
		)
		defer span.End()
	}

	start := time.Now()
	resp, err := t.base.RoundTrip(r)
	// Observe headers before inspecting any error body. getUpdates long-polls,
	// so its latency is the poll timeout, not Telegram's.
	if method != "getUpdates" {
		metrics.TelegramRequestDuration.WithLabelValues(method).Observe(time.Since(start).Seconds())
	}
	if err != nil {
		metrics.TelegramRequestErrors.WithLabelValues(method, "transport").Inc()
		RecordError(span, err)
		return resp, err
	}
	span.SetAttributes(semconv.HTTPResponseStatusCode(resp.StatusCode))
	if resp.StatusCode >= http.StatusBadRequest {
		reason := telegramErrorReason(resp)
		span.SetAttributes(attribute.String("telegram.error.reason", reason))
		// getChatMember is a membership lookup: these known missing-member
		// replies are negative results, not failures. Missing chats and rights
		// still fail: Telegram only guarantees other-user lookups to chat admins.
		// https://core.telegram.org/bots/api/methods/getchatmember.md
		expected := strings.EqualFold(method, "getChatMember") &&
			resp.StatusCode == http.StatusBadRequest && reason == "missing_member"
		if !expected {
			code := strconv.Itoa(resp.StatusCode)
			metrics.TelegramRequestErrors.WithLabelValues(method, code).Inc()
			metrics.TelegramRequestErrorReasons.WithLabelValues(method, code, reason).Inc()
			span.SetAttributes(semconv.ErrorTypeKey.String(code))
			span.SetStatus(codes.Error, "")
		}
	}
	return resp, nil
}

const maxTelegramErrorBody = 8 * 1024

// telegramErrorReason reads only error envelopes, not successful payloads.
// Telegram documents ok=false plus a human-readable description; error_code
// may change, so only HTTP 400 and exact known descriptions can exempt a lookup.
// https://core.telegram.org/bots/api.md#making-requests
func telegramErrorReason(resp *http.Response) string {
	reason := "unknown"
	if resp.StatusCode == http.StatusForbidden {
		reason = "forbidden"
	}
	if resp.Body == nil {
		return reason
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxTelegramErrorBody+1))
	resp.Body = &replayedResponseBody{ReadCloser: resp.Body, prefix: bytes.NewReader(body), readErr: err}
	if err != nil || len(body) > maxTelegramErrorBody {
		return reason
	}
	var envelope struct {
		OK          *bool  `json:"ok"`
		Description string `json:"description"`
	}
	if json.Unmarshal(body, &envelope) != nil || envelope.OK == nil || *envelope.OK {
		return reason
	}
	if resp.StatusCode == http.StatusForbidden {
		return reason
	}
	description := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(envelope.Description), "bad request:"))
	switch description {
	case "user not found", "member not found", "participant_id_invalid", "user_id_invalid":
		return "missing_member"
	case "invalid user_id specified":
		return "invalid_user"
	case "message to be replied not found", "message to be replied to not found", "reply message not found":
		return "reply_not_found"
	case "chat not found":
		return "chat_not_found"
	case "chat_admin_required":
		return "forbidden"
	}
	switch {
	case strings.HasPrefix(description, "can't parse entities"):
		return "entity_parse"
	case strings.HasPrefix(description, "not enough rights"):
		return "forbidden"
	default:
		return reason
	}
}

// Retain both Close and any read failure consumed by inspection, so gotgbot's
// decoder sees the original stream rather than a truncated or successful body.
type replayedResponseBody struct {
	io.ReadCloser
	prefix  *bytes.Reader
	readErr error
}

func (b *replayedResponseBody) Read(p []byte) (int, error) {
	if b.prefix.Len() > 0 {
		return b.prefix.Read(p)
	}
	if len(p) > 0 && b.readErr != nil {
		err := b.readErr
		b.readErr = nil
		return 0, err
	}
	return b.ReadCloser.Read(p)
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
