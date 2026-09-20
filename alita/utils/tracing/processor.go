package tracing

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"go.opentelemetry.io/otel/codes"
)

var onProcessUpdateCallback atomic.Value

const ContextDataKey = "context"

// UpdateContext returns the update's cancellation, deadline, and trace context.
func UpdateContext(ctx *ext.Context) context.Context {
	if ctx != nil && ctx.Data != nil {
		if updateCtx, ok := ctx.Data[ContextDataKey].(context.Context); ok {
			return updateCtx
		}
	}
	return context.Background()
}

func SetOnProcessUpdateCallback(cb func()) {
	onProcessUpdateCallback.Store(cb)
}

type TracingProcessor struct {
	ext.BaseProcessor
}

func runOnProcessUpdateCallback() {
	if cb, ok := onProcessUpdateCallback.Load().(func()); ok && cb != nil {
		cb()
	}
}

func (tp TracingProcessor) ProcessUpdate(d *ext.Dispatcher, b *gotgbot.Bot, ctx *ext.Context) (err error) {
	if ctx == nil {
		return tp.BaseProcessor.ProcessUpdate(d, b, ctx)
	}
	runOnProcessUpdateCallback()

	if ctx.Data != nil {
		if _, exists := ctx.Data[ContextDataKey]; exists {
			return tp.BaseProcessor.ProcessUpdate(d, b, ctx)
		}
	}

	// The 30-second deadline is load-bearing: tracing.UpdateContext hands this
	// context to context-aware repositories, and
	// docs/src/content/docs/architecture/caching.md documents that polling and
	// webhook updates carry it. It is created unconditionally, not only when
	// tracing is on.
	baseCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	traceCtx, span := StartSpan(baseCtx, "dispatcher.processUpdate")
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
