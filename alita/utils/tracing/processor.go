package tracing

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"go.opentelemetry.io/otel/trace"

	"github.com/divkix/Alita_Robot/alita/utils/metrics"
	"github.com/divkix/Alita_Robot/alita/utils/updatememo"
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

	start := time.Now()
	defer func() {
		metrics.UpdatesProcessed.Inc()
		metrics.UpdateDuration.Observe(time.Since(start).Seconds())
	}()
	if ctx.Data == nil {
		ctx.Data = make(map[string]any)
	}
	ctx.Data[updatememo.DataKey] = updatememo.New()

	// Webhook updates arrive with the server's deadline and span already set.
	if _, exists := ctx.Data[ContextDataKey]; exists {
		span := trace.SpanFromContext(UpdateContext(ctx))
		defer recordPanic(span)
		annotateUpdate(span, ctx)
		err = tp.BaseProcessor.ProcessUpdate(d, b, ctx)
		RecordError(span, err)
		return err
	}

	// The 30-second deadline is load-bearing: tracing.UpdateContext hands this
	// context to context-aware repositories, and
	// docs/src/content/docs/architecture/caching.md documents that polling and
	// webhook updates carry it. It is created unconditionally, not only when
	// tracing is on.
	baseCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	traceCtx, span := StartSpan(baseCtx, "dispatcher.processUpdate")
	// Not deferred directly: End recovers and records a panic itself, which
	// recordPanic has already done.
	defer func() { span.End() }()
	defer recordPanic(span)
	ctx.Data[ContextDataKey] = traceCtx
	annotateUpdate(span, ctx)

	err = tp.BaseProcessor.ProcessUpdate(d, b, ctx)
	RecordError(span, err)
	return err
}

// recordPanic records a panic on its way to the dispatcher's recover on the
// update's span, with the stack that panicked, then lets it continue.
func recordPanic(span trace.Span) {
	if r := recover(); r != nil {
		RecordPanic(span, r)
		panic(r)
	}
}
