package callbackcodec

import (
	"errors"
	"strings"
	"testing"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	t.Parallel()

	encoded, err := Encode("notes.overwrite", map[string]string{
		"a": "yes",
		"t": "ab12cd34",
	})
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if len(encoded) > MaxCallbackDataLen {
		t.Fatalf("encoded callback exceeds max length: %d", len(encoded))
	}

	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if decoded.Namespace != "notes.overwrite" {
		t.Fatalf("unexpected namespace: %s", decoded.Namespace)
	}

	if got, _ := decoded.Field("a"); got != "yes" {
		t.Fatalf("unexpected action field: %q", got)
	}
	if got, _ := decoded.Field("t"); got != "ab12cd34" {
		t.Fatalf("unexpected token field: %q", got)
	}
}

func TestEncodeRejectsInvalidNamespace(t *testing.T) {
	t.Parallel()

	if _, err := Encode("", map[string]string{"a": "x"}); !errors.Is(err, ErrInvalidNamespace) {
		t.Fatalf("expected ErrInvalidNamespace for empty namespace, got %v", err)
	}
	if _, err := Encode("bad|namespace", map[string]string{"a": "x"}); !errors.Is(err, ErrInvalidNamespace) {
		t.Fatalf("expected ErrInvalidNamespace for pipe namespace, got %v", err)
	}
}

func TestEncodeRejectsOversizedPayload(t *testing.T) {
	t.Parallel()

	_, err := Encode("filters_overwrite", map[string]string{
		"w": strings.Repeat("x", 120),
	})
	if !errors.Is(err, ErrDataTooLong) {
		t.Fatalf("expected ErrDataTooLong, got %v", err)
	}
}

func TestDecodeRejectsMalformedPayloads(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data string
		err  error
	}{
		{name: "missing separators", data: "notes.overwrite", err: ErrInvalidFormat},
		{name: "unsupported version", data: "notes.overwrite|v2|a=yes", err: ErrUnsupportedVersion},
		{name: "missing namespace", data: "|v1|a=yes", err: ErrInvalidNamespace},
		{name: "invalid query", data: "notes.overwrite|v1|%%%", err: ErrInvalidFormat},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := Decode(tc.data)
			if !errors.Is(err, tc.err) {
				t.Fatalf("expected %v, got %v", tc.err, err)
			}
		})
	}
}

func TestEncodeEmptyFields(t *testing.T) {
	t.Parallel()

	// Empty fields map — empty key is skipped, payload becomes "_"
	encoded, err := Encode("ns", map[string]string{})
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if !strings.Contains(encoded, "|v1|_") {
		t.Fatalf("empty fields payload should be '_', got %q", encoded)
	}
}

func TestEncodeSpecialCharacters(t *testing.T) {
	t.Parallel()

	encoded, err := Encode("ns", map[string]string{"q": "hello world&foo=bar"})
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if v, _ := decoded.Field("q"); v != "hello world&foo=bar" {
		t.Errorf("Field(q) = %q, want %q", v, "hello world&foo=bar")
	}
}

func TestDecodeEmptyFieldSection(t *testing.T) {
	t.Parallel()

	// Payload "_" means no fields
	decoded, err := Decode("ns|v1|_")
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if len(decoded.Fields) != 0 {
		t.Errorf("expected empty fields, got %v", decoded.Fields)
	}
}

func TestEncodeOrFallbackValid(t *testing.T) {
	t.Parallel()

	result := EncodeOrFallback("ns", map[string]string{"k": "v"}, "fallback")
	if result == "fallback" {
		t.Fatal("expected encoded result, got fallback")
	}
	if _, err := Decode(result); err != nil {
		t.Errorf("EncodeOrFallback result not decodable: %v", err)
	}
}

func TestEncodeOrFallbackInvalid(t *testing.T) {
	t.Parallel()

	// Invalid namespace triggers fallback
	result := EncodeOrFallback("bad|ns", map[string]string{}, "fallback")
	if result != "fallback" {
		t.Errorf("expected fallback, got %q", result)
	}
}

func TestEncodeMaxLengthBoundary(t *testing.T) {
	t.Parallel()

	// Build a payload that is exactly at the limit
	// Format: "ns|v1|k=<value>" — "ns|v1|k=" is 8 chars, leaving 56 for value
	val := strings.Repeat("a", 56)
	encoded, err := Encode("ns", map[string]string{"k": val})
	if err != nil {
		t.Fatalf("Encode() at boundary error = %v", err)
	}
	if len(encoded) > MaxCallbackDataLen {
		t.Fatalf("encoded length %d exceeds max %d", len(encoded), MaxCallbackDataLen)
	}

	// One byte over should fail
	val2 := strings.Repeat("a", 57)
	_, err2 := Encode("ns", map[string]string{"k": val2})
	if !errors.Is(err2, ErrDataTooLong) {
		t.Errorf("expected ErrDataTooLong for oversized payload, got %v", err2)
	}
}

func TestDecodeFieldWithPipe(t *testing.T) {
	t.Parallel()

	// A pipe in the payload section is valid (SplitN stops at 3 parts)
	decoded, err := Decode("ns|v1|a=1&b=2")
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if v, _ := decoded.Field("a"); v != "1" {
		t.Errorf("Field(a) = %q, want %q", v, "1")
	}
}

func TestDecodedFieldNilReceiver(t *testing.T) {
	t.Parallel()

	var d *Decoded
	v, ok := d.Field("anything")
	if ok {
		t.Error("nil Decoded.Field() should return ok=false")
	}
	if v != "" {
		t.Errorf("nil Decoded.Field() value = %q, want empty", v)
	}
}
