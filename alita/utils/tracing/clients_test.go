package tracing

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

const testBotToken = "123456789:AAEhBP0av28tj9l6fsa7nyOTaVtnhcjpRBE"

// spanText flattens everything a span exports, to search it for leaks.
func spanText(span sdktrace.ReadOnlySpan) string {
	var b strings.Builder
	fmt.Fprintln(&b, span.Name(), span.Status().Description)
	for _, kv := range span.Attributes() {
		fmt.Fprintln(&b, kv.Key, kv.Value.String())
	}
	for _, event := range span.Events() {
		for _, kv := range event.Attributes {
			fmt.Fprintln(&b, kv.Key, kv.Value.String())
		}
	}
	return b.String()
}

// childSpans returns the ended spans other than the test's own parent span.
func childSpans(t *testing.T, recorder interface {
	Ended() []sdktrace.ReadOnlySpan
}, parent trace.Span) []sdktrace.ReadOnlySpan {
	t.Helper()
	parent.End()
	var children []sdktrace.ReadOnlySpan
	for _, span := range recorder.Ended() {
		if span.SpanContext().SpanID() != parent.SpanContext().SpanID() {
			children = append(children, span)
		}
	}
	return children
}

func TestBotAPITransportHidesTokenAndTraceContext(t *testing.T) {
	recorder := recordSpans(t)
	var gotTraceparent string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTraceparent = r.Header.Get("traceparent")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()
	client := &http.Client{Transport: BotAPITransport(http.DefaultTransport)}

	ctx, parent := StartSpan(context.Background(), "update")
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL+"/bot"+testBotToken+"/sendMessage", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()

	spans := childSpans(t, recorder, parent)
	if len(spans) != 1 {
		t.Fatalf("client spans = %d, want 1", len(spans))
	}
	span := spans[0]
	if span.Name() != "POST /bot{token}/sendMessage" || span.SpanKind() != trace.SpanKindClient {
		t.Fatalf("span = %q kind %v", span.Name(), span.SpanKind())
	}
	if span.Status().Code != codes.Error || !strings.Contains(spanText(span), "http.response.status_code 429") {
		t.Fatalf("429 not recorded as an error: %s", spanText(span))
	}
	if strings.Contains(spanText(span), testBotToken) {
		t.Fatalf("span leaks the bot token: %s", spanText(span))
	}
	if gotTraceparent != "" {
		t.Fatalf("traceparent sent to Telegram: %q", gotTraceparent)
	}
}

func TestBotAPITransportScrubsTransportErrors(t *testing.T) {
	recorder := recordSpans(t)
	failing := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("dial %s: refused", r.URL)
	})
	ctx, parent := StartSpan(context.Background(), "update")
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.telegram.org/bot"+testBotToken+"/getMe", nil)
	if _, err := BotAPITransport(failing).RoundTrip(req); err == nil {
		t.Fatal("want the transport error back")
	}

	spans := childSpans(t, recorder, parent)
	if len(spans) != 1 || spans[0].Status().Code != codes.Error {
		t.Fatalf("want one failed client span, got %d", len(spans))
	}
	if strings.Contains(spanText(spans[0]), testBotToken) {
		t.Fatalf("span leaks the bot token: %s", spanText(spans[0]))
	}
}

func TestClientSpansNeedARecordedParent(t *testing.T) {
	recorder := recordSpans(t)
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer srv.Close()
	client := &http.Client{Transport: BotAPITransport(http.DefaultTransport)}
	resp, err := client.Post(srv.URL+"/bot"+testBotToken+"/getMe", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()

	rdb := newTracedRedis(t)
	if err := rdb.Set(context.Background(), "k", "v", 0).Err(); err != nil {
		t.Fatal(err)
	}
	if spans := recorder.Ended(); len(spans) != 0 {
		t.Fatalf("calls without a parent recorded %d spans", len(spans))
	}
}

func TestRedisHookRecordsCommandsWithoutArguments(t *testing.T) {
	recorder := recordSpans(t)
	rdb := newTracedRedis(t)
	ctx, parent := StartSpan(context.Background(), "update")
	if err := rdb.Set(ctx, "user:42", "secret-value", 0).Err(); err != nil {
		t.Fatal(err)
	}
	if err := rdb.Get(ctx, "missing").Err(); err != redis.Nil {
		t.Fatalf("get missing = %v, want redis.Nil", err)
	}
	pipe := rdb.Pipeline()
	pipe.Incr(ctx, "n")
	pipe.Expire(ctx, "n", 0)
	if _, err := pipe.Exec(ctx); err != nil {
		t.Fatal(err)
	}

	names := map[string]sdktrace.ReadOnlySpan{}
	for _, span := range childSpans(t, recorder, parent) {
		names[span.Name()] = span
		if text := spanText(span); strings.Contains(text, "user:42") || strings.Contains(text, "secret-value") {
			t.Fatalf("span records command arguments: %s", text)
		}
	}
	for _, want := range []string{"set", "get", "pipeline"} {
		if names[want] == nil {
			t.Fatalf("no %q span in %v", want, names)
		}
		if status := names[want].Status(); status.Code == codes.Error {
			t.Fatalf("%s failed: %s", want, status.Description)
		}
	}
	if !strings.Contains(spanText(names["pipeline"]), "db.operation.batch.size 2") {
		t.Fatalf("pipeline span lacks its batch size: %s", spanText(names["pipeline"]))
	}
}

func TestBotAPIMethod(t *testing.T) {
	for path, want := range map[string]string{
		"/bot" + testBotToken + "/sendMessage":     "sendMessage",
		"/bot" + testBotToken + "/test/getUpdates": "getUpdates",
		"/bot" + testBotToken + "/":                "unknown",
		"/bot" + testBotToken + "/send%20message":  "unknown",
		"/bot" + testBotToken + "/" + testBotToken: "unknown",
		"/prefix/bot" + testBotToken + "/getChat":  "getChat",
	} {
		if got := botAPIMethod(path); got != want {
			t.Errorf("botAPIMethod(%q) = %q, want %q", path, got, want)
		}
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func newTracedRedis(t *testing.T) *redis.Client {
	t.Helper()
	rdb := redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	rdb.AddHook(RedisHook{})
	return rdb
}
