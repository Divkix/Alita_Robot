package tracing

import (
	"fmt"
	"runtime/debug"

	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/divkix/Alita_Robot/alita/utils/logredact"
)

// RecordError marks span failed and attaches err as an exception event with
// the caller's stack, so the trace shows where it ended in error.
func RecordError(span trace.Span, err error) {
	if err == nil || !span.IsRecording() {
		return
	}
	typ := fmt.Sprintf("%T", err)
	recordException(span, typ, typ, err.Error())
}

// RecordPanic records a recovered panic value on span as RecordError does.
// Call it from the deferred function that recovered, so the stack still holds
// the frames that panicked.
func RecordPanic(span trace.Span, r any) {
	if !span.IsRecording() {
		return
	}
	recordException(span, "panic", fmt.Sprintf("%T", r), fmt.Sprintf("panic: %v", r))
}

// recordException writes the exception event span.RecordError would, but
// with the message scrubbed like a log line before it is exported.
func recordException(span trace.Span, errorType, exceptionType, message string) {
	message = logredact.Scrub(message)
	span.AddEvent(semconv.ExceptionEventName, trace.WithAttributes(
		semconv.ExceptionType(exceptionType),
		semconv.ExceptionMessage(message),
		semconv.ExceptionStacktrace(string(debug.Stack())),
	))
	span.SetAttributes(semconv.ErrorTypeKey.String(errorType))
	span.SetStatus(codes.Error, message)
}
