package tracing

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/divkix/Alita_Robot/alita/utils/callbackcodec"
)

// recordSpans turns tracing on with an in-memory recorder for one test.
func recordSpans(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	resetTracingState(t)
	recorder := tracetest.NewSpanRecorder()
	tracerProvider = sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	tracer = tracerProvider.Tracer("test")
	enabled = true
	return recorder
}

func spanAttributes(t *testing.T, recorder *tracetest.SpanRecorder) map[attribute.Key]attribute.Value {
	t.Helper()
	attrs := make(map[attribute.Key]attribute.Value)
	for _, kv := range onlySpan(t, recorder).Attributes() {
		attrs[kv.Key] = kv.Value
	}
	return attrs
}

func processUpdate(t *testing.T, update *gotgbot.Update, handler ext.Handler) {
	t.Helper()
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{
		MaxRoutines: -1,
		Error: func(_ *gotgbot.Bot, ctx *ext.Context, err error) ext.DispatcherAction {
			RecordUpdateError(ctx, err, false)
			return ext.DispatcherActionNoop
		},
	})
	if handler != nil {
		dispatcher.AddHandler(handler)
	}
	ctx := ext.NewContext(&gotgbot.Bot{}, update, nil)
	if err := (TracingProcessor{}).ProcessUpdate(dispatcher, &gotgbot.Bot{}, ctx); err != nil {
		t.Fatalf("ProcessUpdate() error = %v", err)
	}
}

func TestProcessUpdateTagsCommandChatAndUser(t *testing.T) {
	recorder := recordSpans(t)

	processUpdate(t, &gotgbot.Update{Message: &gotgbot.Message{
		Text:     "/Ban@AlitaBot @spammer",
		Entities: []gotgbot.MessageEntity{{Type: "bot_command", Offset: 0, Length: 13}},
		Chat:     gotgbot.Chat{Id: -100123, Type: "supergroup"},
		From:     &gotgbot.User{Id: 42},
	}}, nil)

	attrs := spanAttributes(t, recorder)
	want := map[attribute.Key]attribute.Value{
		"update.type": attribute.StringValue("message"),
		"bot.command": attribute.StringValue("/ban"),
		"chat.id":     attribute.Int64Value(-100123),
		"chat.type":   attribute.StringValue("supergroup"),
		"user.id":     attribute.Int64Value(42),
	}
	for key, value := range want {
		if attrs[key] != value {
			t.Errorf("attribute %s = %v, want %v", key, attrs[key].String(), value.String())
		}
	}
}

func TestProcessUpdateLeavesPlainMessagesWithoutCommand(t *testing.T) {
	command := []gotgbot.MessageEntity{{Type: "bot_command", Offset: 0, Length: 1}}
	for _, msg := range []*gotgbot.Message{
		{Text: "/home/x is a path"}, // no bot_command entity
		{Text: "hello /ban", Entities: []gotgbot.MessageEntity{{Type: "bot_command", Offset: 6, Length: 4}}},
		{Text: "/" + strings.Repeat("a", maxCommandLen), Entities: command},
	} {
		recorder := recordSpans(t)
		msg.Chat = gotgbot.Chat{Id: 1, Type: "private"}
		processUpdate(t, &gotgbot.Update{Message: msg}, nil)
		if value, ok := spanAttributes(t, recorder)["bot.command"]; ok {
			t.Errorf("text %q tagged bot.command = %q, want none", msg.Text, value.AsString())
		}
	}
}

func TestProcessUpdateTagsCallbackNamespace(t *testing.T) {
	recorder := recordSpans(t)
	data, err := callbackcodec.Encode("warns", map[string]string{"a": "rm"})
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	processUpdate(t, &gotgbot.Update{CallbackQuery: &gotgbot.CallbackQuery{
		Data: data,
		From: gotgbot.User{Id: 7},
	}}, nil)

	attrs := spanAttributes(t, recorder)
	if got := attrs["update.type"].AsString(); got != "callback_query" {
		t.Errorf("update.type = %q, want callback_query", got)
	}
	if got := attrs["callback.namespace"].AsString(); got != "warns" {
		t.Errorf("callback.namespace = %q, want warns", got)
	}
}

type failingHandler struct{ err error }

func (h failingHandler) CheckUpdate(*gotgbot.Bot, *ext.Context) bool   { return true }
func (h failingHandler) HandleUpdate(*gotgbot.Bot, *ext.Context) error { return h.err }
func (h failingHandler) Name() string                                  { return "failing" }

func TestHandlerErrorMarksUpdateSpanFailed(t *testing.T) {
	recorder := recordSpans(t)

	processUpdate(t, &gotgbot.Update{Message: &gotgbot.Message{
		Text: "/warn",
		Chat: gotgbot.Chat{Id: -1, Type: "group"},
	}}, failingHandler{err: errors.New("db down")})

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("ended spans = %d, want 1", len(spans))
	}
	if status := spans[0].Status(); status.Code != codes.Error || status.Description != "db down" {
		t.Fatalf("span status = %+v, want Error with the handler's message", status)
	}
}

func TestExpectedErrorLeavesUpdateSpanSuccessful(t *testing.T) {
	recorder := recordSpans(t)
	ctx, span := StartSpan(context.Background(), "update")
	extCtx := &ext.Context{Data: map[string]any{ContextDataKey: ctx}}

	RecordUpdateError(extCtx, errors.New("message is not modified"), true)
	span.End()

	ended := recorder.Ended()
	if len(ended) != 1 {
		t.Fatalf("ended spans = %d, want 1", len(ended))
	}
	if code := ended[0].Status().Code; code == codes.Error {
		t.Fatal("expected error set the span status to Error")
	}
	expected := false
	for _, kv := range ended[0].Attributes() {
		if kv.Key == "error.expected" {
			expected = kv.Value.AsBool()
		}
	}
	if !expected {
		t.Fatal("expected error not tagged error.expected=true")
	}
}

func TestProcessUpdateMarksSpanOnPanic(t *testing.T) {
	recorder := recordSpans(t)
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1, Processor: TracingProcessor{}})
	dispatcher.AddHandler(panickingHandler{})

	// The dispatcher's own recover turns the panic into an error.
	err := dispatcher.ProcessUpdate(&gotgbot.Bot{}, &gotgbot.Update{Message: &gotgbot.Message{Text: "/ping"}}, nil)
	if !errors.Is(err, ext.ErrPanicRecovered) {
		t.Fatalf("ProcessUpdate() error = %v, want ErrPanicRecovered", err)
	}

	spans := recorder.Ended()
	if len(spans) != 1 || spans[0].Status().Code != codes.Error {
		t.Fatalf("spans = %d, want one span with status Error", len(spans))
	}
}

type panickingHandler struct{}

func (panickingHandler) CheckUpdate(*gotgbot.Bot, *ext.Context) bool   { return true }
func (panickingHandler) HandleUpdate(*gotgbot.Bot, *ext.Context) error { panic("boom") }
func (panickingHandler) Name() string                                  { return "panicking" }
