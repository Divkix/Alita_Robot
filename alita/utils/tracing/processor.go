package tracing

import (
	"context"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"go.opentelemetry.io/otel/codes"
)

// TracingProcessor wraps ext.BaseProcessor to inject trace context into every
// update processed by the dispatcher. This ensures that polling mode updates
// get a root trace span, matching the behavior of the webhook handler.
type TracingProcessor struct {
	ext.BaseProcessor
}

// ProcessUpdate starts a trace span for the update and injects the trace context
// into ctx.Data["context"] before delegating to the base processor.
// If a context already exists in ctx.Data (e.g., from webhook handler), it is preserved.
func (tp TracingProcessor) ProcessUpdate(d *ext.Dispatcher, b *gotgbot.Bot, ctx *ext.Context) error {
	traceCtx, span := StartSpan(context.Background(), "dispatcher.processUpdate")
	defer span.End()

	if ctx.Data == nil {
		ctx.Data = make(map[string]any)
	}
	if _, exists := ctx.Data["context"]; !exists {
		ctx.Data["context"] = traceCtx
	}

	err := tp.BaseProcessor.ProcessUpdate(d, b, ctx)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
	}
	return err
}
