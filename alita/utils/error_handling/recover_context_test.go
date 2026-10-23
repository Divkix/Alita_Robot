//go:build testtools

package error_handling

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/codes"

	"github.com/divkix/Alita_Robot/alita/utils/tracing"
)

func TestRecoverFromPanicContextRecordsOnSpan(t *testing.T) {
	recorder := tracing.RecordSpansForTest(t)
	ctx, span := tracing.StartSpan(context.Background(), "handler")
	func() {
		defer RecoverFromPanicContext(ctx, "handler", "test")
		panic("boom")
	}()
	span.End()

	spans := recorder.Ended()
	if len(spans) != 1 || spans[0].Status().Code != codes.Error {
		t.Fatalf("spans = %d, want one span with status Error", len(spans))
	}
	if events := spans[0].Events(); len(events) != 1 || events[0].Name != "exception" {
		t.Fatalf("events = %+v, want one exception", events)
	}
}
