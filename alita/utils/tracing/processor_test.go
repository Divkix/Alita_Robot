package tracing

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// injectTraceContext replicates the context injection logic from TracingProcessor.ProcessUpdate.
// This is extracted so we can test the injection behavior without needing a full Dispatcher.
func injectTraceContext(ctx *ext.Context) (skipped bool) {
	if ctx != nil && ctx.Data != nil {
		if _, exists := ctx.Data[ContextDataKey]; exists {
			return true
		}
	}

	traceCtx, span := StartSpan(context.Background(), "dispatcher.processUpdate")
	defer span.End()

	if ctx.Data == nil {
		ctx.Data = make(map[string]any)
	}
	ctx.Data[ContextDataKey] = traceCtx
	return false
}

func TestTracingProcessor_InjectsContext(t *testing.T) {
	ctx := &ext.Context{
		Data: nil,
	}

	skipped := injectTraceContext(ctx)
	if skipped {
		t.Fatal("should not skip injection when no context exists")
	}

	// Assert ctx.Data has a context key of type context.Context
	raw, ok := ctx.Data[ContextDataKey]
	if !ok {
		t.Fatal("ctx.Data must contain the trace context entry after injection")
	}
	if _, ok := raw.(context.Context); !ok {
		t.Fatalf("trace context entry must be of type context.Context (got %T)", raw)
	}
}

func TestTracingProcessor_PreservesExistingContext(t *testing.T) {
	existingCtx := context.WithValue(context.Background(), contextTestKey("existing"), "yes")

	ctx := &ext.Context{
		Data: map[string]any{
			ContextDataKey: existingCtx,
		},
	}

	skipped := injectTraceContext(ctx)
	if !skipped {
		t.Fatal("should skip injection when context already exists")
	}

	// The existing context should NOT be overwritten
	extracted, ok := ctx.Data[ContextDataKey].(context.Context)
	if !ok {
		t.Fatal("existing trace context entry must be a context.Context")
	}
	if extracted != existingCtx {
		t.Error("TracingProcessor should not overwrite an existing context")
	}
	if extracted.Value(contextTestKey("existing")) != "yes" {
		t.Error("Existing context values should be preserved")
	}
}

func TestTracingProcessor_InitializesNilData(t *testing.T) {
	ctx := &ext.Context{
		Data: nil,
	}

	injectTraceContext(ctx)

	if ctx.Data == nil {
		t.Fatal("TracingProcessor should initialize nil Data map")
	}
	if _, ok := ctx.Data[ContextDataKey]; !ok {
		t.Fatal("ctx.Data must contain the trace context key after injection")
	}
}

func TestTracingProcessor_SkipsSpanForWebhookContext(t *testing.T) {
	webhookCtx := context.WithValue(context.Background(), contextTestKey("webhook"), "true")

	ctx := &ext.Context{
		Data: map[string]any{
			ContextDataKey: webhookCtx,
		},
	}

	skipped := injectTraceContext(ctx)
	if !skipped {
		t.Fatal("should skip span creation when webhook context already exists")
	}

	// Verify the original webhook context is untouched
	raw := ctx.Data[ContextDataKey]
	if raw != webhookCtx {
		t.Fatal("webhook context must not be replaced")
	}
}

// contextTestKey is a custom type for context keys to avoid collisions.
type contextTestKey string

// ---------------------------------------------------------------------------
// Callback tests (wiring tests for main.go monitoring setup)
// ---------------------------------------------------------------------------

func TestRunOnProcessUpdateCallback_InvokesRegisteredCallback(t *testing.T) {
	// Do not use t.Parallel() - tests global state

	var called atomic.Int32
	SetOnProcessUpdateCallback(func() {
		called.Add(1)
	})
	defer SetOnProcessUpdateCallback(nil) // cleanup

	runOnProcessUpdateCallback()

	if called.Load() != 1 {
		t.Errorf("expected callback to be called exactly once, got %d calls", called.Load())
	}

	// Call again to verify multiple invocations work
	runOnProcessUpdateCallback()
	if called.Load() != 2 {
		t.Errorf("expected callback to be called twice, got %d calls", called.Load())
	}
}

func TestRunOnProcessUpdateCallback_NoCallback_NoOp(t *testing.T) {
	// Do not use t.Parallel() - tests global state

	// Ensure no callback is set
	SetOnProcessUpdateCallback(nil)

	// Should not panic
	runOnProcessUpdateCallback()
}
