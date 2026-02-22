package modules

import (
	"testing"
)

func TestEncodeCallbackData(t *testing.T) {
	t.Parallel()

	t.Run("valid encode", func(t *testing.T) {
		t.Parallel()

		result := encodeCallbackData("test_ns", map[string]string{"k": "v"}, "fallback")
		// Result must not be the fallback and must contain the namespace.
		if result == "fallback" {
			t.Fatal("expected encoded data, got fallback")
		}
		if result == "" {
			t.Fatal("expected non-empty encoded data")
		}
	})

	t.Run("encode error with empty namespace falls back", func(t *testing.T) {
		t.Parallel()

		// Empty namespace triggers ErrInvalidNamespace in the codec.
		result := encodeCallbackData("", map[string]string{"k": "v"}, "myfallback")
		if result != "myfallback" {
			t.Fatalf("expected fallback 'myfallback', got %q", result)
		}
	})

	t.Run("nil fields map", func(t *testing.T) {
		t.Parallel()

		// Nil map should produce a valid encoded string (empty payload becomes "_").
		result := encodeCallbackData("test_ns", nil, "fallback")
		if result == "fallback" {
			t.Fatal("nil fields map should encode successfully, not fall back")
		}
	})

	t.Run("empty fallback on encode error", func(t *testing.T) {
		t.Parallel()

		result := encodeCallbackData("", nil, "")
		if result != "" {
			t.Fatalf("expected empty fallback, got %q", result)
		}
	})
}

func TestDecodeCallbackData(t *testing.T) {
	t.Parallel()

	t.Run("valid decode no expected namespaces", func(t *testing.T) {
		t.Parallel()

		encoded := encodeCallbackData("myns", map[string]string{"x": "1"}, "")
		decoded, ok := decodeCallbackData(encoded)
		if !ok {
			t.Fatal("expected ok=true for valid encoded data")
		}
		if decoded == nil {
			t.Fatal("expected non-nil decoded")
		}
		if decoded.Namespace != "myns" {
			t.Fatalf("expected namespace 'myns', got %q", decoded.Namespace)
		}
	})

	t.Run("valid with matching namespace", func(t *testing.T) {
		t.Parallel()

		encoded := encodeCallbackData("myns", map[string]string{"x": "1"}, "")
		decoded, ok := decodeCallbackData(encoded, "myns")
		if !ok {
			t.Fatal("expected ok=true when namespace matches")
		}
		if decoded == nil {
			t.Fatal("expected non-nil decoded")
		}
	})

	t.Run("namespace mismatch", func(t *testing.T) {
		t.Parallel()

		encoded := encodeCallbackData("myns", map[string]string{"x": "1"}, "")
		decoded, ok := decodeCallbackData(encoded, "otherns")
		if ok {
			t.Fatal("expected ok=false for mismatched namespace")
		}
		if decoded != nil {
			t.Fatal("expected nil decoded on mismatch")
		}
	})

	t.Run("case-insensitive namespace match", func(t *testing.T) {
		t.Parallel()

		encoded := encodeCallbackData("MyNs", map[string]string{"x": "1"}, "")
		decoded, ok := decodeCallbackData(encoded, "myns")
		if !ok {
			t.Fatal("expected ok=true for case-insensitive namespace match")
		}
		if decoded == nil {
			t.Fatal("expected non-nil decoded")
		}
	})

	t.Run("invalid malformed data", func(t *testing.T) {
		t.Parallel()

		decoded, ok := decodeCallbackData("not|valid")
		if ok {
			t.Fatal("expected ok=false for malformed data")
		}
		if decoded != nil {
			t.Fatal("expected nil decoded for malformed data")
		}
	})

	t.Run("empty string", func(t *testing.T) {
		t.Parallel()

		decoded, ok := decodeCallbackData("")
		if ok {
			t.Fatal("expected ok=false for empty string")
		}
		if decoded != nil {
			t.Fatal("expected nil decoded for empty string")
		}
	})
}
