//go:build testtools

package jobs

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/divkix/Alita_Robot/alita/utils/tracing"
)

func TestRunStartsARootSpanPerTick(t *testing.T) {
	recorder := tracing.RecordSpansForTest(t)
	parentCtx, parent := tracing.StartSpan(context.Background(), "caller")
	defer parent.End()

	var childParent trace.SpanContext
	Run(parentCtx, "sweep", func(ctx context.Context) error {
		_, child := tracing.StartClientSpan(ctx, "SELECT chats")
		childParent = trace.SpanContextFromContext(ctx)
		child.End()
		return nil
	})

	spans := recorder.Ended()
	if len(spans) != 2 {
		t.Fatalf("ended spans = %d, want the job and its child", len(spans))
	}
	job := spans[1]
	if job.Name() != "job sweep" || job.Parent().IsValid() {
		t.Fatalf("job span %q has parent %v, want a root span", job.Name(), job.Parent())
	}
	if childParent.SpanID() != job.SpanContext().SpanID() || spans[0].Parent().SpanID() != job.SpanContext().SpanID() {
		t.Fatal("work inside the job is not a child of its span")
	}
}

func TestRunRecordsErrorsAndPanics(t *testing.T) {
	recorder := tracing.RecordSpansForTest(t)

	Run(context.Background(), "failing", func(context.Context) error { return errors.New("boom") })
	Run(context.Background(), "panicking", func(context.Context) error { panic("bad tick") })

	spans := recorder.Ended()
	if len(spans) != 2 {
		t.Fatalf("ended spans = %d, want 2", len(spans))
	}
	for _, span := range spans {
		if span.Status().Code != codes.Error {
			t.Fatalf("%s status = %v, want Error", span.Name(), span.Status())
		}
	}
	if got := spans[1].Status().Description; got != "panic: bad tick" {
		t.Fatalf("panic status = %q", got)
	}
}
