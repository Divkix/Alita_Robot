//go:build testtools

package tracing

import (
	"context"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// RecordSpansForTest turns tracing on with an in-memory recorder for one test
// and restores the previous state when it ends. Tests using it must not run in
// parallel.
func RecordSpansForTest(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	origProvider, origTracer, origEnabled := tracerProvider, tracer, enabled
	recorder := tracetest.NewSpanRecorder()
	tracerProvider = sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	provider := tracerProvider
	t.Cleanup(func() {
		_ = provider.Shutdown(context.Background())
		tracerProvider, tracer, enabled = origProvider, origTracer, origEnabled
	})
	tracer = tracerProvider.Tracer("test")
	enabled = true
	return recorder
}
