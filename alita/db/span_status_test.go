//go:build testtools

package db

import (
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

	err := UpdateRecord(&LockSettings{}, missing, map[string]any{"locked": true})
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("update error = %v, want ErrRecordNotFound", err)
	}
	if err := GetRecord(&LockSettings{}, missing); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("get error = %v, want ErrRecordNotFound", err)
	}

	spans := recorder.Ended()
	if len(spans) != 2 {
		t.Fatalf("ended spans = %d, want 2", len(spans))
	}
	for _, span := range spans {
		if span.Status().Code == codes.Error {
			t.Fatalf("span %s status = Error for an expected miss", span.Name())
		}
	}
}
