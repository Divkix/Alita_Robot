package tracing

import (
	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/trace"
)

// traceFieldsHook adds the trace and span ids of a log call's context to the
// line, so stdout logs can be matched to traces. Exported log records carry
// them natively; this hook runs after the export hook, so they don't get
// them twice.
type traceFieldsHook struct{}

func (traceFieldsHook) Levels() []log.Level {
	return log.AllLevels
}

func (traceFieldsHook) Fire(entry *log.Entry) error {
	if entry.Context == nil {
		return nil
	}
	sc := trace.SpanContextFromContext(entry.Context)
	if !sc.IsValid() {
		return nil
	}
	entry.Data["trace_id"] = sc.TraceID().String()
	entry.Data["span_id"] = sc.SpanID().String()
	return nil
}
