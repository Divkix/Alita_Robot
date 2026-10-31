package tracing

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"github.com/divkix/Alita_Robot/alita/utils/metrics"
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

func TestBotAPITransportCountsFailuresWithoutASpan(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()
	limited := metrics.TelegramRequestErrors.WithLabelValues("sendMessage", "429")
	refused := metrics.TelegramRequestErrors.WithLabelValues("getMe", "transport")
	limitedBefore, refusedBefore := testutil.ToFloat64(limited), testutil.ToFloat64(refused)

	resp, err := (&http.Client{Transport: BotAPITransport(http.DefaultTransport)}).Post(srv.URL+"/bot"+testBotToken+"/sendMessage", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	failing := roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, errors.New("refused") })
	req, _ := http.NewRequest(http.MethodPost, "https://api.telegram.org/bot"+testBotToken+"/getMe", nil)
	if _, err := BotAPITransport(failing).RoundTrip(req); err == nil {
		t.Fatal("want the transport error back")
	}

	if got := testutil.ToFloat64(limited) - limitedBefore; got != 1 {
		t.Fatalf("429 errors delta = %v, want 1", got)
	}
	if got := testutil.ToFloat64(refused) - refusedBefore; got != 1 {
		t.Fatalf("transport errors delta = %v, want 1", got)
	}
}

func TestBotAPITransportClassifiesErrorResponses(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		status      int
		description string
		reason      string
		expected    bool
	}{
		{"missing user", "getChatMember", 400, "Bad Request: user not found", "missing_member", true},
		{"missing member", "getChatMember", 400, "Bad Request: member not found", "missing_member", true},
		{"invalid participant", "getChatMember", 400, "Bad Request: PARTICIPANT_ID_INVALID", "missing_member", true},
		{"invalid user", "getChatMember", 400, "Bad Request: user_id_invalid", "missing_member", true},
		{"case insensitive method", "GETCHATMEMBER", 400, "Bad Request: user not found", "missing_member", true},
		{"missing chat", "getChatMember", 400, "Bad Request: chat not found", "chat_not_found", false},
		{"missing rights", "getChatMember", 400, "Bad Request: not enough rights to get chat member", "forbidden", false},
		{"different missing user", "getChatMember", 400, "Bad Request: user not found in another operation", "unknown", false},
		{"invalid lookup user id", "getChatMember", 400, "Bad Request: invalid user_id specified", "invalid_user", false},
		{"send missing user", "sendMessage", 400, "Bad Request: user not found", "missing_member", false},
		{"send missing member", "sendMessage", 400, "Bad Request: member not found", "missing_member", false},
		{"send invalid participant", "sendMessage", 400, "Bad Request: PARTICIPANT_ID_INVALID", "missing_member", false},
		{"send invalid user", "sendMessage", 400, "Bad Request: user_id_invalid", "missing_member", false},
		{"invalid send user id", "sendMessage", 400, "Bad Request: invalid user_id specified", "invalid_user", false},
		{"missing reply", "sendMessage", 400, "Bad Request: message to be replied not found", "reply_not_found", false},
		{"invalid entities", "sendMessage", 400, "Bad Request: can't parse entities: private-response-text at byte offset 42", "entity_parse", false},
		{"forbidden", "getChatMember", 403, "Forbidden: bot was blocked by the user", "forbidden", false},
		{"server failure", "getChatMember", 500, "Bad Request: user not found", "missing_member", false},
	}
	for _, recorded := range []bool{false, true} {
		for _, tt := range tests {
			t.Run(fmt.Sprintf("%s/recorded=%t", tt.name, recorded), func(t *testing.T) {
				recorder := recordSpans(t)
				const message = "private-request-text"
				body := fmt.Sprintf(" \n{\"ok\":false,\"error_code\":%d,\"description\":%q,\"extra\":%q}\n", tt.status, tt.description, message)
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(tt.status)
					_, _ = io.WriteString(w, body)
				}))
				defer srv.Close()
				base := http.DefaultTransport.(*http.Transport).Clone()
				defer base.CloseIdleConnections()
				closeErr := errors.New("original body close error")
				var observed *observedResponseBody
				client := &http.Client{Transport: BotAPITransport(roundTripFunc(func(r *http.Request) (*http.Response, error) {
					resp, err := base.RoundTrip(r)
					if err == nil {
						observed = &observedResponseBody{ReadCloser: resp.Body, closeErr: closeErr}
						resp.Body = observed
					}
					return resp, err
				}))}
				counter := metrics.TelegramRequestErrors.WithLabelValues(tt.method, fmt.Sprint(tt.status))
				before := testutil.ToFloat64(counter)
				reasonCounter := metrics.TelegramRequestErrorReasons.WithLabelValues(tt.method, fmt.Sprint(tt.status), tt.reason)
				reasonBefore := testutil.ToFloat64(reasonCounter)
				ctx := context.Background()
				var parent trace.Span
				if recorded {
					ctx, parent = StartSpan(ctx, "update")
					defer parent.End()
				}
				requestURL := srv.URL + "/bot" + testBotToken + "/" + tt.method + "?text=" + message
				req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, strings.NewReader(message))
				if err != nil {
					t.Fatal(err)
				}
				resp, err := client.Do(req)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = resp.Body.Close() })
				if observed.closes != 0 {
					t.Fatal("transport closed the response body before the decoder")
				}
				got, err := io.ReadAll(resp.Body)
				if err != nil || string(got) != body {
					t.Fatalf("response body changed: %q, error %v", got, err)
				}
				if err := resp.Body.Close(); err != closeErr || observed.closes != 1 {
					t.Fatalf("original Close not retained: error %v, closes %d", err, observed.closes)
				}
				wantErrors := float64(1)
				if tt.expected {
					wantErrors = 0
				}
				if delta := testutil.ToFloat64(counter) - before; delta != wantErrors {
					t.Fatalf("error counter delta = %v, want %v", delta, wantErrors)
				}
				if delta := testutil.ToFloat64(reasonCounter) - reasonBefore; delta != wantErrors {
					t.Fatalf("reason counter delta = %v, want %v", delta, wantErrors)
				}
				if !recorded {
					if spans := recorder.Ended(); len(spans) != 0 {
						t.Fatalf("request without a parent recorded %d spans", len(spans))
					}
					return
				}
				spans := childSpans(t, recorder, parent)
				if len(spans) != 1 {
					t.Fatalf("client spans = %d, want 1", len(spans))
				}
				span := spans[0]
				if span.Parent().SpanID() != parent.SpanContext().SpanID() {
					t.Fatal("error response span is not under the recorded parent")
				}
				if failed := span.Status().Code == codes.Error; failed == tt.expected {
					t.Fatalf("span status = %v, expected negative = %t", span.Status(), tt.expected)
				}
				var reason string
				for _, kv := range span.Attributes() {
					if kv.Key == "telegram.error.reason" {
						reason = kv.Value.AsString()
					}
					if tt.expected && kv.Key == "error.type" {
						t.Fatal("expected membership negative carries error.type")
					}
				}
				if reason != tt.reason {
					t.Fatalf("error reason = %q, want %q", reason, tt.reason)
				}
				for _, secret := range []string{requestURL, testBotToken, message, tt.description, "private-response-text"} {
					if strings.Contains(spanText(span), secret) {
						t.Fatalf("span leaks request or response content: %s", spanText(span))
					}
				}
			})
		}
	}
}

func TestBotAPITransportBoundsErrorInspection(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{"success", 200, `{"ok":true,"result":"` + strings.Repeat("x", 16384) + `"}`},
		{"oversized error", 400, `{"ok":false,"description":"Bad Request: user not found","padding":"` + strings.Repeat("x", 16384) + `"}`},
		{"truncated error", 400, `{"ok":false,"description":"Bad Request: user not found"`},
		{"missing ok", 400, `{"description":"Bad Request: user not found"}`},
		{"successful envelope", 400, `{"ok":true,"description":"Bad Request: user not found"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = io.WriteString(w, tt.body)
			}))
			defer srv.Close()
			base := http.DefaultTransport.(*http.Transport).Clone()
			defer base.CloseIdleConnections()
			var observed *observedResponseBody
			client := &http.Client{Transport: BotAPITransport(roundTripFunc(func(r *http.Request) (*http.Response, error) {
				resp, err := base.RoundTrip(r)
				if err == nil {
					observed = &observedResponseBody{ReadCloser: resp.Body}
					resp.Body = observed
				}
				return resp, err
			}))}
			counter := metrics.TelegramRequestErrors.WithLabelValues("getChatMember", fmt.Sprint(tt.status))
			before := testutil.ToFloat64(counter)
			resp, err := client.Post(srv.URL+"/bot"+testBotToken+"/getChatMember", "application/json", nil)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = resp.Body.Close() })
			if observed.bytesRead > maxTelegramErrorBody+1 {
				t.Fatalf("inspection read %d bytes, limit %d", observed.bytesRead, maxTelegramErrorBody+1)
			}
			if tt.status == http.StatusOK && observed.bytesRead != 0 {
				t.Fatalf("transport inspected %d success bytes", observed.bytesRead)
			}
			got, err := io.ReadAll(resp.Body)
			if err != nil || string(got) != tt.body {
				t.Fatalf("response body changed: %q, error %v", got, err)
			}
			wantErrors := float64(1)
			if tt.status == http.StatusOK {
				wantErrors = 0
			}
			if delta := testutil.ToFloat64(counter) - before; delta != wantErrors {
				t.Fatalf("error counter delta = %v, want %v", delta, wantErrors)
			}
		})
	}
}

func TestBotAPITransportDurationStopsAtHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"ok":false,"description":"Bad Request: chat not found"}`)
	}))
	defer srv.Close()
	base := http.DefaultTransport.(*http.Transport).Clone()
	defer base.CloseIdleConnections()
	duration := metrics.TelegramRequestDuration.WithLabelValues("getChatMember").(prometheus.Metric)
	var before dto.Metric
	if err := duration.Write(&before); err != nil {
		t.Fatal(err)
	}
	read := false
	client := &http.Client{Transport: BotAPITransport(roundTripFunc(func(r *http.Request) (*http.Response, error) {
		resp, err := base.RoundTrip(r)
		if err == nil {
			resp.Body = &observedResponseBody{ReadCloser: resp.Body, onRead: func() {
				read = true
				var after dto.Metric
				if err := duration.Write(&after); err != nil {
					t.Fatal(err)
				}
				if after.GetHistogram().GetSampleCount() != before.GetHistogram().GetSampleCount()+1 {
					t.Fatal("request duration was not observed before error body inspection")
				}
			}}
		}
		return resp, err
	}))}
	resp, err := client.Post(srv.URL+"/bot"+testBotToken+"/getChatMember", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if !read {
		t.Fatal("error body was not inspected")
	}
}

type observedResponseBody struct {
	io.ReadCloser
	bytesRead int
	closes    int
	closeErr  error
	onRead    func()
}

func (b *observedResponseBody) Read(p []byte) (int, error) {
	if b.onRead != nil {
		b.onRead()
	}
	n, err := b.ReadCloser.Read(p)
	b.bytesRead += n
	return n, err
}

func (b *observedResponseBody) Close() error {
	b.closes++
	if err := b.ReadCloser.Close(); err != nil {
		return err
	}
	return b.closeErr
}

func TestBotAPITransportPreservesErrorBodyReadFailure(t *testing.T) {
	const body = `{"ok":false,"description":"Bad Request: user not found"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprint(len(body)+10))
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, body)
	}))
	defer srv.Close()
	base := http.DefaultTransport.(*http.Transport).Clone()
	defer base.CloseIdleConnections()
	var observed *observedResponseBody
	client := &http.Client{Transport: BotAPITransport(roundTripFunc(func(r *http.Request) (*http.Response, error) {
		resp, err := base.RoundTrip(r)
		if err == nil {
			observed = &observedResponseBody{ReadCloser: resp.Body}
			resp.Body = observed
		}
		return resp, err
	}))}
	counter := metrics.TelegramRequestErrors.WithLabelValues("getChatMember", "400")
	before := testutil.ToFloat64(counter)
	resp, err := client.Post(srv.URL+"/bot"+testBotToken+"/getChatMember", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	got, err := io.ReadAll(resp.Body)
	if string(got) != body || !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("body read = %q, %v; want original bytes and unexpected EOF", got, err)
	}
	if delta := testutil.ToFloat64(counter) - before; delta != 1 {
		t.Fatalf("incomplete error response was exempted: error counter delta = %v", delta)
	}
	if observed.closes != 0 {
		t.Fatal("inspection closed the incomplete body")
	}
	if err := resp.Body.Close(); err != nil || observed.closes != 1 {
		t.Fatalf("original body was not closed: error %v, closes %d", err, observed.closes)
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
