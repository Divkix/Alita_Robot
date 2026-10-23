package tracing

import (
	"context"
	"testing"

	log "github.com/sirupsen/logrus"
)

func TestTraceFieldsHookAddsIDsOfTheLogContext(t *testing.T) {
	recordSpans(t)
	ctx, span := StartSpan(context.Background(), "update")
	defer span.End()

	entry := log.NewEntry(log.StandardLogger()).WithContext(ctx)
	if err := (traceFieldsHook{}).Fire(entry); err != nil {
		t.Fatal(err)
	}
	if entry.Data["trace_id"] != span.SpanContext().TraceID().String() || entry.Data["span_id"] != span.SpanContext().SpanID().String() {
		t.Fatalf("fields = %v", entry.Data)
	}

	plain := log.NewEntry(log.StandardLogger()).WithContext(context.Background())
	if err := (traceFieldsHook{}).Fire(plain); err != nil {
		t.Fatal(err)
	}
	if _, ok := plain.Data["trace_id"]; ok {
		t.Fatal("trace_id added without a span")
	}
}
