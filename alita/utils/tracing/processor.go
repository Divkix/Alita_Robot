package tracing

import (
	"context"
	"sync/atomic"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"go.opentelemetry.io/otel/codes"
)

var onProcessUpdateCallback atomic.Value

const ContextDataKey = "context"

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
