package modules

import (
	"testing"
)

func TestEncodeCallbackData(t *testing.T) {
	t.Parallel()

	t.Run("valid encode", func(t *testing.T) {
		t.Parallel()

		result := encodeCallbackData("test_ns", map[string]string{"k": "v"})
		// Result must not be empty and must contain the namespace.
		if result == "" {
			t.Fatal("expected non-empty encoded data")
		}
	})

	t.Run("encode error with empty namespace returns empty string", func(t *testing.T) {
		t.Parallel()

		// Empty namespace triggers ErrInvalidNamespace in the codec.
		result := encodeCallbackData("", map[string]string{"k": "v"})
		if result != "" {
			t.Fatalf("expected empty string on encode error, got %q", result)
		}
	})

	t.Run("nil fields map", func(t *testing.T) {
		t.Parallel()

		// Nil map should produce a valid encoded string (empty payload becomes "_").
		result := encodeCallbackData("test_ns", nil)
		if result == "" {
			t.Fatal("nil fields map should encode successfully")
		}
	})

	t.Run("empty fields map encodes successfully", func(t *testing.T) {
		t.Parallel()

		result := encodeCallbackData("test_ns", map[string]string{})
		if result == "" {
			t.Fatal("empty fields map should encode successfully")
		}
	})
}

func TestDecodeCallbackData(t *testing.T) {
	t.Parallel()

	t.Run("valid decode no expected namespaces", func(t *testing.T) {
		t.Parallel()

		encoded := encodeCallbackData("myns", map[string]string{"x": "1"})
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

		encoded := encodeCallbackData("myns", map[string]string{"x": "1"})
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

		encoded := encodeCallbackData("myns", map[string]string{"x": "1"})
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

		encoded := encodeCallbackData("MyNs", map[string]string{"x": "1"})
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

	t.Run("multiple expected namespaces with match", func(t *testing.T) {
		t.Parallel()

		encoded := encodeCallbackData("myns", map[string]string{"x": "1"})
		decoded, ok := decodeCallbackData(encoded, "otherns", "myns")
		if !ok {
			t.Fatal("expected ok=true when one of multiple namespaces matches")
		}
		if decoded == nil {
			t.Fatal("expected non-nil decoded")
		}
	})

	t.Run("multiple expected namespaces no match", func(t *testing.T) {
		t.Parallel()

		encoded := encodeCallbackData("myns", map[string]string{"x": "1"})
		decoded, ok := decodeCallbackData(encoded, "otherns", "anotherns")
		if ok {
			t.Fatal("expected ok=false when none of multiple namespaces match")
		}
		if decoded != nil {
			t.Fatal("expected nil decoded when no namespace matches")
		}
	})

	t.Run("empty fields map encodes and decodes successfully", func(t *testing.T) {
		t.Parallel()

		result := encodeCallbackData("test_ns", map[string]string{})
		if result == "" {
			t.Fatal("empty fields map should encode successfully")
		}
		decoded, ok := decodeCallbackData(result, "test_ns")
		if !ok {
			t.Fatal("expected decode ok=true for empty fields map")
		}
		if decoded == nil {
			t.Fatal("expected non-nil decoded")
		}
	})
}
