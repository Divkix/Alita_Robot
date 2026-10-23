package tracing

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// exception returns the message and stack of the span's one exception event.
func exception(t *testing.T, span sdktrace.ReadOnlySpan) (message, stack string) {
	t.Helper()
	var found int
	for _, event := range span.Events() {
		if event.Name != "exception" {
			continue
		}
		found++
		for _, kv := range event.Attributes {
			switch kv.Key {
			case "exception.message":
				message = kv.Value.AsString()
			case "exception.stacktrace":
				stack = kv.Value.AsString()
			}
		}
	}
	if found != 1 {
		t.Fatalf("exception events = %d, want 1", found)
	}
	return message, stack
}

func onlySpan(t *testing.T, recorder *tracetest.SpanRecorder) sdktrace.ReadOnlySpan {
	t.Helper()
	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("ended spans = %d, want 1", len(spans))
	}
	return spans[0]
}

func TestRecordErrorAddsStackAndScrubsSecrets(t *testing.T) {
	recorder := recordSpans(t)
	_, span := StartSpan(context.Background(), "op")
	token := "123456789:AAEhBP0av28tj9l6fsa7nyOTaVtnhcjpRBE"
	RecordError(span, errors.New("Post https://api.telegram.org/bot"+token+"/getMe: timeout"))
	span.End()

	ended := onlySpan(t, recorder)
	if ended.Status().Code != codes.Error || strings.Contains(ended.Status().Description, token) {
		t.Fatalf("status = %+v, want Error without the token", ended.Status())
	}
	message, stack := exception(t, ended)
	if strings.Contains(message, token) || !strings.Contains(message, "getMe: timeout") {
		t.Fatalf("exception.message = %q, want the error with the token scrubbed", message)
	}
	if !strings.Contains(stack, "TestRecordErrorAddsStackAndScrubsSecrets") {
		t.Fatalf("exception.stacktrace does not show the caller:\n%s", stack)
	}
}

func TestRecordErrorIgnoresNil(t *testing.T) {
	recorder := recordSpans(t)
	_, span := StartSpan(context.Background(), "op")
	RecordError(span, nil)
	span.End()

	ended := onlySpan(t, recorder)
	if ended.Status().Code == codes.Error || len(ended.Events()) != 0 {
		t.Fatalf("nil error changed the span: status %+v, events %d", ended.Status(), len(ended.Events()))
	}
}

func TestProcessUpdateRecordsPanicStack(t *testing.T) {
	for _, webhook := range []bool{false, true} {
		recorder := recordSpans(t)
		var data map[string]any
		end := func() {}
		if webhook {
			// Webhook updates bring the server's span; the panic lands on it.
			ctx, span := StartSpan(context.Background(), "webhook")
			data, end = map[string]any{ContextDataKey: ctx}, func() { span.End() }
		}
		dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1, Processor: TracingProcessor{}})
		dispatcher.AddHandler(panickingHandler{})

		err := dispatcher.ProcessUpdate(&gotgbot.Bot{}, &gotgbot.Update{Message: &gotgbot.Message{Text: "/ping"}}, data)
		end()
		if !errors.Is(err, ext.ErrPanicRecovered) {
			t.Fatalf("webhook=%v: ProcessUpdate() error = %v, want ErrPanicRecovered", webhook, err)
		}
		message, stack := exception(t, onlySpan(t, recorder))
		if message != "panic: boom" || !strings.Contains(stack, "panickingHandler.HandleUpdate") {
			t.Fatalf("webhook=%v: exception = %q, want the panic with the handler's frame in:\n%s", webhook, message, stack)
		}
	}
}
