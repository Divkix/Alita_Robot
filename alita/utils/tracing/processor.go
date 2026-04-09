package tracing

import (
	"context"
	"sync/atomic"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"go.opentelemetry.io/otel/codes"
)

var onProcessUpdateCallback atomic.Value

// ContextDataKey is the ext.Context.Data key used to carry tracing context.
const ContextDataKey = "context"

// SetOnProcessUpdateCallback registers a callback to be called when an update is processed
func SetOnProcessUpdateCallback(cb func()) {
	onProcessUpdateCallback.Store(cb)
}

// TracingProcessor wraps ext.BaseProcessor to inject trace context into every
// update processed by the dispatcher. This ensures that polling mode updates
// get a root trace span, matching the behavior of the webhook handler.
type TracingProcessor struct {
	ext.BaseProcessor
}

// runOnProcessUpdateCallback executes the registered callback if set.
// Extracted for testability without requiring full dispatcher integration.
func runOnProcessUpdateCallback() {
	if cb, ok := onProcessUpdateCallback.Load().(func()); ok && cb != nil {
		cb()
	}
}

// ProcessUpdate starts a trace span for the update (in polling mode) and injects the
// trace context into ctx.Data[ContextDataKey] before delegating to the base processor.
// If a context already exists in ctx.Data (e.g., from webhook handler), it is reused and
// no new span is created here to avoid duplicating dispatcher.processUpdate spans.
func (tp TracingProcessor) ProcessUpdate(d *ext.Dispatcher, b *gotgbot.Bot, ctx *ext.Context) (err error) {
	// Record message for monitoring
	runOnProcessUpdateCallback()

	// If an existing context is present (e.g., webhook request), just delegate without
	// creating a new root span so that we don't break trace parenting or duplicate spans.
	if ctx != nil && ctx.Data != nil {
		if _, exists := ctx.Data[ContextDataKey]; exists {
			return tp.BaseProcessor.ProcessUpdate(d, b, ctx)
		}
	}

	// No existing context: create a new root span and propagate its context.
	traceCtx, span := StartSpan(context.Background(), "dispatcher.processUpdate")
	defer func() {
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}()

	if ctx.Data == nil {
		ctx.Data = make(map[string]any)
	}
	ctx.Data[ContextDataKey] = traceCtx

	err = tp.BaseProcessor.ProcessUpdate(d, b, ctx)
	return err
}
