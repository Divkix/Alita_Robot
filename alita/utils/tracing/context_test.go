package tracing

import (
	"context"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func TestExtractContext_WithValidContext(t *testing.T) {
	expected := context.WithValue(context.Background(), contextTestKey("test"), "value")
	ctx := &ext.Context{
		Data: map[string]any{
			"context": expected,
		},
	}

	result := ExtractContext(ctx)
	if result != expected {
		t.Error("ExtractContext should return the context stored in Data[\"context\"]")
	}
	if result.Value(contextTestKey("test")) != "value" {
		t.Error("ExtractContext should preserve context values")
	}
}

func TestExtractContext_NilExtContext(t *testing.T) {
	result := ExtractContext(nil)
	if result == nil {
		t.Fatal("ExtractContext(nil) should return context.Background(), not nil")
	}
	// context.Background() has no deadline, values, or cancellation
	if result.Err() != nil {
		t.Error("ExtractContext(nil) should return a non-cancelled context")
	}
}

func TestExtractContext_NilData(t *testing.T) {
	ctx := &ext.Context{
		Data: nil,
	}

	result := ExtractContext(ctx)
	if result == nil {
		t.Fatal("ExtractContext with nil Data should return context.Background(), not nil")
	}
}

func TestExtractContext_MissingContextKey(t *testing.T) {
	ctx := &ext.Context{
		Data: map[string]any{
			"other_key": "other_value",
		},
	}

	result := ExtractContext(ctx)
	if result == nil {
		t.Fatal("ExtractContext with missing key should return context.Background(), not nil")
	}
}

func TestExtractContext_WrongType(t *testing.T) {
	ctx := &ext.Context{
		Data: map[string]any{
			"context": "not a context",
		},
	}

	result := ExtractContext(ctx)
	if result == nil {
		t.Fatal("ExtractContext with wrong type should return context.Background(), not nil")
	}
}

func TestExtractContext_EmptyData(t *testing.T) {
	ctx := &ext.Context{
		Data: map[string]any{},
	}

	result := ExtractContext(ctx)
	if result == nil {
		t.Fatal("ExtractContext with empty Data should return context.Background(), not nil")
	}
}

func TestExtractContext_CancelledContext(t *testing.T) {
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	ctx := &ext.Context{
		Data: map[string]any{
			"context": cancelCtx,
		},
	}

	result := ExtractContext(ctx)
	if result != cancelCtx {
		t.Error("ExtractContext should return the exact context, even if cancelled")
	}
	if result.Err() == nil {
		t.Error("ExtractContext should preserve the cancelled state")
	}
}

// contextTestKey is a custom type for context keys to avoid collisions.
type contextTestKey string
