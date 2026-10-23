//go:build testtools

package db

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.opentelemetry.io/otel/codes"
	"gorm.io/gorm"

	"github.com/divkix/Alita_Robot/alita/utils/tracing"
)

func TestRecordNotFoundLeavesDBSpanSuccessful(t *testing.T) {
	skipIfNoDb(t)
	recorder := tracing.RecordSpansForTest(t)
	missing := LockSettings{ChatId: time.Now().UnixNano(), LockType: "sticker"}

	ctx, parent := tracing.StartSpan(context.Background(), "update")

	err := updateRecordInternal(ctx, &LockSettings{}, missing, map[string]any{"locked": true}, "test")
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("update error = %v, want ErrRecordNotFound", err)
	}
	if err := GetRecordContext(ctx, &LockSettings{}, missing); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("get error = %v, want ErrRecordNotFound", err)
	}
	parent.End()

	spans := recorder.Ended()
	if len(spans) != 3 {
		t.Fatalf("ended spans = %d, want db.update, db.get and their parent", len(spans))
	}
	for _, span := range spans {
		if span.Status().Code == codes.Error {
			t.Fatalf("span %s status = Error for an expected miss", span.Name())
		}
	}
}
