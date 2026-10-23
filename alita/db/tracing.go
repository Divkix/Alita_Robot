package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"

	"github.com/divkix/Alita_Robot/alita/utils/tracing"
)

type querySpanKey struct{}

// registerQueryTracing gives every GORM statement run under a recorded span a
// child span named by operation and table. The SQL text and its bound values
// are never recorded.
func registerQueryTracing(database *gorm.DB) error {
	cb := database.Callback()
	return errors.Join(
		cb.Query().Before("gorm:query").Register("alita:trace_query", startQuerySpan("SELECT")),
		cb.Query().After("gorm:query").Register("alita:trace_query_end", endQuerySpan),
		cb.Create().Before("gorm:create").Register("alita:trace_create", startQuerySpan("INSERT")),
		cb.Create().After("gorm:create").Register("alita:trace_create_end", endQuerySpan),
		cb.Update().Before("gorm:update").Register("alita:trace_update", startQuerySpan("UPDATE")),
		cb.Update().After("gorm:update").Register("alita:trace_update_end", endQuerySpan),
		cb.Delete().Before("gorm:delete").Register("alita:trace_delete", startQuerySpan("DELETE")),
		cb.Delete().After("gorm:delete").Register("alita:trace_delete_end", endQuerySpan),
		cb.Row().Before("gorm:row").Register("alita:trace_row", startQuerySpan("")),
		cb.Row().After("gorm:row").Register("alita:trace_row_end", endQuerySpan),
		cb.Raw().Before("gorm:raw").Register("alita:trace_raw", startQuerySpan("")),
		cb.Raw().After("gorm:raw").Register("alita:trace_raw_end", endQuerySpan),
	)
}

// startQuerySpan returns a callback opening the statement's span. Row and Raw
// statements pass no operation: their SQL is free-form, so the span is named
// by table, or by the database system when there is none.
func startQuerySpan(operation string) func(*gorm.DB) {
	return func(tx *gorm.DB) {
		ctx := tx.Statement.Context
		if !trace.SpanFromContext(ctx).IsRecording() {
			return
		}
		table := tx.Statement.Table
		attrs := []attribute.KeyValue{semconv.DBSystemNamePostgreSQL}
		if operation != "" {
			attrs = append(attrs, semconv.DBOperationName(operation))
		}
		if table != "" {
			attrs = append(attrs, semconv.DBCollectionName(table))
		}
		_, span := tracing.StartClientSpan(ctx, querySpanName(operation, table), attrs...)
		tx.Statement.Settings.Store(querySpanKey{}, span)
	}
}

func querySpanName(operation, table string) string {
	switch {
	case operation != "" && table != "":
		return operation + " " + table
	case operation != "":
		return operation
	case table != "":
		return table
	default:
		return "postgresql"
	}
}

func endQuerySpan(tx *gorm.DB) {
	value, ok := tx.Statement.Settings.LoadAndDelete(querySpanKey{})
	if !ok {
		return
	}
	span, ok := value.(trace.Span)
	if !ok {
		return
	}
	if !errors.Is(tx.Error, gorm.ErrRecordNotFound) {
		recordQueryError(span, tx.Error)
	}
	span.End()
}

// recordQueryError marks span failed without the error's text, which can
// quote row values: Postgres messages and details, and pgx encode errors,
// include them. The span keeps the SQLSTATE, or the Go type when there is
// none.
func recordQueryError(span trace.Span, err error) {
	if err == nil {
		return
	}
	code, safe := safeQueryError(err)
	if code != "" {
		span.SetAttributes(semconv.DBResponseStatusCode(code))
	}
	tracing.RecordError(span, safe)
}

// safeQueryError returns the SQLSTATE of err, if any, and err with its text
// replaced by that SQLSTATE, or by its Go type when it has none. Errors with
// fixed text are kept.
func safeQueryError(err error) (string, error) {
	var pgErr *pgconn.PgError
	switch {
	case errors.As(err, &pgErr):
		return pgErr.Code, queryError("postgres error SQLSTATE " + pgErr.Code)
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled), errors.Is(err, gorm.ErrRecordNotFound):
		return "", err
	default:
		return "", queryError(fmt.Sprintf("database error %T", err))
	}
}

// queryError stands in for a database error on a span.
type queryError string

func (e queryError) Error() string {
	return string(e)
}
