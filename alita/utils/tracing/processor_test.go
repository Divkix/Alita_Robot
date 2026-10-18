package tracing

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
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

func TestTracingProcessorProcessUpdateCountsTopLevelUpdatesOnly(t *testing.T) {
	processor := TracingProcessor{}
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})

	before := testutil.ToFloat64(metrics.UpdatesProcessed)
	if err := processor.ProcessUpdate(dispatcher, &gotgbot.Bot{}, &ext.Context{}); err != nil {
		t.Fatalf("ProcessUpdate() error = %v", err)
	}
	if got := testutil.ToFloat64(metrics.UpdatesProcessed) - before; got != 1 {
		t.Fatalf("updates processed delta = %v, want 1", got)
	}

	nested := &ext.Context{Data: map[string]any{ContextDataKey: context.Background()}}
	before = testutil.ToFloat64(metrics.UpdatesProcessed)
	if err := processor.ProcessUpdate(dispatcher, &gotgbot.Bot{}, nested); err != nil {
		t.Fatalf("nested ProcessUpdate() error = %v", err)
	}
	if got := testutil.ToFloat64(metrics.UpdatesProcessed) - before; got != 0 {
		t.Fatalf("nested updates processed delta = %v, want 0", got)
	}
}
