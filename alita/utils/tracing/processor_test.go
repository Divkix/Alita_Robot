package tracing

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/divkix/Alita_Robot/alita/utils/metrics"
	"github.com/divkix/Alita_Robot/alita/utils/updatememo"
)

func TestTracingProcessorProcessUpdateInjectsContext(t *testing.T) {
	ctx := &ext.Context{}
	processor := TracingProcessor{}
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})

	if err := processor.ProcessUpdate(dispatcher, &gotgbot.Bot{}, ctx); err != nil {
		t.Fatalf("ProcessUpdate() error = %v", err)
	}

	raw, ok := ctx.Data[ContextDataKey]
	if !ok {
		t.Fatal("ProcessUpdate() did not inject tracing context")
	}
	if _, ok := raw.(context.Context); !ok {
		t.Fatalf("ProcessUpdate() context entry type = %T, want context.Context", raw)
	}
}

func TestTracingProcessorProcessUpdateInjectsUpdateMemo(t *testing.T) {
	ctx := &ext.Context{}
	processor := TracingProcessor{}
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})

	if err := processor.ProcessUpdate(dispatcher, &gotgbot.Bot{}, ctx); err != nil {
		t.Fatalf("ProcessUpdate() error = %v", err)
	}

	if updatememo.From(ctx) == nil {
		t.Fatal("ProcessUpdate() did not inject a per-update memo")
	}
}

func TestTracingProcessorProcessUpdatePreservesExistingContext(t *testing.T) {
	existingCtx := context.WithValue(context.Background(), contextTestKey("existing"), "true")
	ctx := &ext.Context{
		Data: map[string]any{
			ContextDataKey: existingCtx,
		},
	}
	processor := TracingProcessor{}
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})

	if err := processor.ProcessUpdate(dispatcher, &gotgbot.Bot{}, ctx); err != nil {
		t.Fatalf("ProcessUpdate() error = %v", err)
	}

	if got := ctx.Data[ContextDataKey]; got != existingCtx {
		t.Fatalf("ProcessUpdate() replaced existing context: got %v, want %v", got, existingCtx)
	}
	if updatememo.From(ctx) == nil {
		t.Fatal("ProcessUpdate() did not inject a per-update memo alongside an existing context")
	}
}

func TestTracingProcessorProcessUpdateRunsCallback(t *testing.T) {
	// Do not use t.Parallel() - tests global state.
	var called atomic.Int32
	SetOnProcessUpdateCallback(func() {
		called.Add(1)
	})
	defer SetOnProcessUpdateCallback(nil)

	processor := TracingProcessor{}
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})

	if err := processor.ProcessUpdate(dispatcher, &gotgbot.Bot{}, &ext.Context{}); err != nil {
		t.Fatalf("ProcessUpdate() error = %v", err)
	}
	if called.Load() != 1 {
		t.Fatalf("ProcessUpdate() callback calls = %d, want 1", called.Load())
	}

	// A cleared callback is skipped, not called as a nil func.
	SetOnProcessUpdateCallback(nil)
	if err := processor.ProcessUpdate(dispatcher, &gotgbot.Bot{}, &ext.Context{}); err != nil {
		t.Fatalf("ProcessUpdate() after clearing callback error = %v", err)
	}
	if called.Load() != 1 {
		t.Fatalf("cleared callback still ran: calls = %d, want 1", called.Load())
	}
}

type contextTestKey string

func TestTracingProcessorProcessUpdateCountsEveryUpdate(t *testing.T) {
	processor := TracingProcessor{}
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	processed := metrics.UpdatesProcessed.WithLabelValues("unknown", metrics.OutcomeOK.String(), "")

	before := testutil.ToFloat64(processed)
	if err := processor.ProcessUpdate(dispatcher, &gotgbot.Bot{}, &ext.Context{}); err != nil {
		t.Fatalf("ProcessUpdate() error = %v", err)
	}
	if got := testutil.ToFloat64(processed) - before; got != 1 {
		t.Fatalf("updates processed delta = %v, want 1", got)
	}

	// Webhook updates arrive with the server's context already set.
	webhook := &ext.Context{Data: map[string]any{ContextDataKey: context.Background()}}
	before = testutil.ToFloat64(processed)
	if err := processor.ProcessUpdate(dispatcher, &gotgbot.Bot{}, webhook); err != nil {
		t.Fatalf("webhook ProcessUpdate() error = %v", err)
	}
	if got := testutil.ToFloat64(processed) - before; got != 1 {
		t.Fatalf("webhook updates processed delta = %v, want 1", got)
	}
}

func TestTracingProcessorLabelsOutcomeAndKnownCommand(t *testing.T) {
	// Do not use t.Parallel() - sets the global command set.
	metrics.SetCommandModules(map[string]string{"ban": "bans"})
	defer metrics.SetCommandModules(nil)

	command := func(text string) *ext.Context {
		return ext.NewContext(&gotgbot.Bot{}, &gotgbot.Update{Message: &gotgbot.Message{
			Chat:     gotgbot.Chat{Id: -1001, Type: "supergroup"},
			Text:     text,
			Entities: []gotgbot.MessageEntity{{Type: "bot_command", Offset: 0, Length: int64(len(text))}},
		}}, nil)
	}
	failing := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	failing.AddHandler(handlers.NewMessage(nil, func(_ *gotgbot.Bot, ctx *ext.Context) error {
		RecordUpdateError(ctx, errors.New("boom"), false)
		return nil
	}))
	panicking := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	panicking.AddHandler(handlers.NewMessage(nil, func(*gotgbot.Bot, *ext.Context) error { panic("boom") }))

	for _, tc := range []struct {
		name       string
		dispatcher *ext.Dispatcher
		text       string
		outcome    metrics.Outcome
		command    string
	}{
		{"known command error", failing, "/ban@AlitaBot 42", metrics.OutcomeError, "ban"},
		{"unknown command", failing, "/whatever", metrics.OutcomeError, metrics.Other},
		{"panic", panicking, "/ban", metrics.OutcomePanic, "ban"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			processed := metrics.UpdatesProcessed.WithLabelValues("message", tc.outcome.String(), tc.command)
			before := testutil.ToFloat64(processed)
			func() {
				defer func() { _ = recover() }()
				_ = TracingProcessor{}.ProcessUpdate(tc.dispatcher, &gotgbot.Bot{}, command(tc.text))
			}()
			if got := testutil.ToFloat64(processed) - before; got != 1 {
				t.Fatalf("updates processed{outcome=%s,command=%s} delta = %v, want 1", tc.outcome, tc.command, got)
			}
		})
	}
}

func TestRecordUpdateErrorCountsByModule(t *testing.T) {
	metrics.SetCommandModules(map[string]string{"ban": "bans"})
	defer metrics.SetCommandModules(nil)

	ctx := ext.NewContext(&gotgbot.Bot{}, &gotgbot.Update{Message: &gotgbot.Message{
		Text:     "/ban",
		Entities: []gotgbot.MessageEntity{{Type: "bot_command", Offset: 0, Length: 4}},
	}}, map[string]any{})
	errs := metrics.HandlerErrors.WithLabelValues("bans", "true")
	before := testutil.ToFloat64(errs)
	RecordUpdateError(ctx, errors.New("message is not modified"), true)
	if got := testutil.ToFloat64(errs) - before; got != 1 {
		t.Fatalf("handler errors{module=bans,expected=true} delta = %v, want 1", got)
	}

	// The outcome only ever gets worse.
	for _, step := range []struct {
		mark func()
		want metrics.Outcome
	}{
		{func() { RecordUpdateError(ctx, errors.New("boom"), false) }, metrics.OutcomeError},
		{func() { RecordUpdateError(ctx, errors.New("not modified"), true) }, metrics.OutcomeError},
		{func() { MarkUpdatePanic(ctx) }, metrics.OutcomePanic},
		{func() { RecordUpdateError(ctx, errors.New("boom"), false) }, metrics.OutcomePanic},
	} {
		step.mark()
		if got := ctx.Data[outcomeDataKey]; got != step.want {
			t.Fatalf("outcome = %v, want %v", got, step.want)
		}
	}
}
