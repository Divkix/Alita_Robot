package tracing

import (
	"context"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// injectTraceContext replicates the context injection logic from TracingProcessor.ProcessUpdate.
// This is extracted so we can test the injection behavior without needing a full Dispatcher.
func injectTraceContext(ctx *ext.Context) (skipped bool) {
	if ctx != nil && ctx.Data != nil {
		if _, exists := ctx.Data["context"]; exists {
			return true
		}
	}

	traceCtx, span := StartSpan(context.Background(), "dispatcher.processUpdate")
	defer span.End()

	if ctx.Data == nil {
		ctx.Data = make(map[string]any)
	}
	ctx.Data["context"] = traceCtx
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

	// Assert ctx.Data has a "context" key of type context.Context
	raw, ok := ctx.Data["context"]
	if !ok {
		t.Fatal(`ctx.Data must contain a "context" entry after injection`)
	}
	if _, ok := raw.(context.Context); !ok {
		t.Fatalf(`ctx.Data["context"] must be of type context.Context (got %T)`, raw)
	}

	// Verify ExtractContext returns the same injected context
	extracted := ExtractContext(ctx)
	if extracted != raw {
		t.Fatal("ExtractContext should return the injected context from ctx.Data")
	}
}

func TestTracingProcessor_PreservesExistingContext(t *testing.T) {
	existingCtx := context.WithValue(context.Background(), contextTestKey("existing"), "yes")

	ctx := &ext.Context{
		Data: map[string]any{
			"context": existingCtx,
		},
	}

	skipped := injectTraceContext(ctx)
	if !skipped {
		t.Fatal("should skip injection when context already exists")
	}

	// The existing context should NOT be overwritten
	extracted := ExtractContext(ctx)
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
	if _, ok := ctx.Data["context"]; !ok {
		t.Fatal(`ctx.Data must contain a "context" key after injection`)
	}
}

func TestTracingProcessor_SkipsSpanForWebhookContext(t *testing.T) {
	webhookCtx := context.WithValue(context.Background(), contextTestKey("webhook"), "true")

	ctx := &ext.Context{
		Data: map[string]any{
			"context": webhookCtx,
		},
	}

	skipped := injectTraceContext(ctx)
	if !skipped {
		t.Fatal("should skip span creation when webhook context already exists")
	}

	// Verify the original webhook context is untouched
	raw := ctx.Data["context"]
	if raw != webhookCtx {
		t.Fatal("webhook context must not be replaced")
	}
}
