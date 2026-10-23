//go:build testtools

package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/divkix/Alita_Robot/alita/utils/tracing"
)

type tracedRow struct {
	ID   int64
	Note string
}

func newTracedDB(t *testing.T) *gorm.DB {
	t.Helper()
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.AutoMigrate(&tracedRow{}); err != nil {
		t.Fatal(err)
	}
	if err := registerQueryTracing(database); err != nil {
		t.Fatal(err)
	}
	return database
}

func TestQueryTracingNamesOperationAndTableOnly(t *testing.T) {
	database := newTracedDB(t)
	recorder := tracing.RecordSpansForTest(t)

	if err := database.Create(&tracedRow{ID: 1, Note: "outside any trace"}).Error; err != nil {
		t.Fatal(err)
	}
	if spans := recorder.Ended(); len(spans) != 0 {
		t.Fatalf("statement without a parent span recorded %d spans", len(spans))
	}

	ctx, parent := tracing.StartSpan(context.Background(), "update")
	tx := database.WithContext(ctx)
	if err := tx.Create(&tracedRow{ID: 2, Note: "private text"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Where("note = ?", "private text").First(&tracedRow{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.First(&tracedRow{}, 99).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("missing row error = %v", err)
	}
	parent.End()

	var names []string
	for _, span := range recorder.Ended() {
		if span.SpanContext().SpanID() == parent.SpanContext().SpanID() {
			continue
		}
		names = append(names, span.Name())
		if span.Parent().SpanID() != parent.SpanContext().SpanID() {
			t.Fatalf("%s is not a child of the caller's span", span.Name())
		}
		if span.Status().Code == codes.Error {
			t.Fatalf("%s failed: %s", span.Name(), span.Status().Description)
		}
		for _, kv := range span.Attributes() {
			if strings.Contains(kv.Value.String(), "private text") {
				t.Fatalf("%s records a bound value in %s", span.Name(), kv.Key)
			}
		}
		if !hasAttribute(span.Attributes(), attribute.String("db.collection.name", "traced_rows")) {
			t.Fatalf("%s lacks db.collection.name: %v", span.Name(), span.Attributes())
		}
	}
	if got := strings.Join(names, ","); got != "INSERT traced_rows,SELECT traced_rows,SELECT traced_rows" {
		t.Fatalf("spans = %s", got)
	}
}

func TestQueryTracingDropsPostgresErrorDetail(t *testing.T) {
	database := newTracedDB(t)
	recorder := tracing.RecordSpansForTest(t)
	ctx, parent := tracing.StartSpan(context.Background(), "update")
	defer parent.End()

	tx := database.WithContext(ctx).Table("traced_rows")
	startQuerySpan("INSERT")(tx)
	_ = tx.AddError(&pgconn.PgError{Code: "23505", Message: "duplicate key", Detail: "Key (note)=(private text) already exists."})
	endQuerySpan(tx)

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("ended spans = %d, want 1", len(spans))
	}
	span := spans[0]
	if span.Status().Code != codes.Error || !hasAttribute(span.Attributes(), attribute.String("db.response.status_code", "23505")) {
		t.Fatalf("failure not recorded with its SQLSTATE: %v %v", span.Status(), span.Attributes())
	}
	for _, event := range span.Events() {
		for _, kv := range event.Attributes {
			if strings.Contains(kv.Value.String(), "private text") || strings.Contains(kv.Value.String(), "duplicate key") {
				t.Fatalf("span event leaks the Postgres message: %s", kv.Value.String())
			}
		}
	}
}

func TestQueryTracingDropsDriverErrorText(t *testing.T) {
	database := newTracedDB(t)
	recorder := tracing.RecordSpansForTest(t)
	ctx, parent := tracing.StartSpan(context.Background(), "update")
	defer parent.End()

	tx := database.WithContext(ctx).Table("traced_rows")
	startQuerySpan("")(tx)
	_ = tx.AddError(fmt.Errorf("failed to encode args[0]: unable to encode %q", "private text"))
	endQuerySpan(tx)

	spans := recorder.Ended()
	if len(spans) != 1 || spans[0].Name() != "traced_rows" || spans[0].Status().Code != codes.Error {
		t.Fatalf("want one failed traced_rows span, got %d", len(spans))
	}
	if strings.Contains(spans[0].Status().Description, "private text") {
		t.Fatalf("span status leaks the bound value: %s", spans[0].Status().Description)
	}
}

func hasAttribute(attrs []attribute.KeyValue, want attribute.KeyValue) bool {
	for _, kv := range attrs {
		if kv == want {
			return true
		}
	}
	return false
}
