package error_handling

import (
	"context"
	"runtime/debug"
	"sync/atomic"

	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/trace"

	"github.com/divkix/Alita_Robot/alita/utils/tracing"
)

var onErrorCallback atomic.Value

func SetOnErrorCallback(cb func()) {
	onErrorCallback.Store(cb)
}

func RecoverFromPanic(funcName, modName string) {
	if r := recover(); r != nil {
		handlePanic(context.Background(), funcName, modName, r)
	}
}

// RecoverFromPanicContext is RecoverFromPanic for code running under a span:
// it also records the panic, with its stack, on the span in ctx.
func RecoverFromPanicContext(ctx context.Context, funcName, modName string) {
	if r := recover(); r != nil {
		tracing.RecordPanic(trace.SpanFromContext(ctx), r)
		handlePanic(ctx, funcName, modName, r)
	}
}

func handlePanic(ctx context.Context, funcName, modName string, r any) {
	stackTrace := string(debug.Stack())

	log.WithContext(ctx).Errorf("[%s][%s] Recovered from panic: %v\nStack trace:\n%s",
		modName, funcName, r, stackTrace)

	if cb, ok := onErrorCallback.Load().(func()); ok && cb != nil {
		cb()
	}
}
