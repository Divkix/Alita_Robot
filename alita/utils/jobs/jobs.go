// Package jobs runs background work under its own trace.
package jobs

import (
	"context"

	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
	"github.com/divkix/Alita_Robot/alita/utils/tracing"
)

// Run runs one tick of the background job name as the root span of its own
// trace, so the database, Redis and Bot API calls it makes on ctx become
// child spans. It logs and records an error fn returns, and recovers a panic
// as error_handling.RecoverFromPanic does. name must be a fixed string: it
// names the span.
func Run(ctx context.Context, name string, fn func(context.Context) error) {
	ctx, span := tracing.StartSpan(ctx, "job "+name,
		trace.WithNewRoot(),
		trace.WithAttributes(attribute.String("job.name", name)),
	)
	defer span.End()
	defer error_handling.RecoverFromPanicContext(ctx, name, "jobs")

	if err := fn(ctx); err != nil {
		tracing.RecordError(span, err)
		log.WithContext(ctx).WithError(err).Errorf("[Jobs] %s failed", name)
	}
}
