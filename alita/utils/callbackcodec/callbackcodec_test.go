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

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := Decode(tc.data)
			if !errors.Is(err, tc.err) {
				t.Fatalf("expected %v, got %v", tc.err, err)
			}
		})
	}
}

func TestEncodeOrFallbackSuccess(t *testing.T) {
	t.Parallel()

	result := EncodeOrFallback("ns", map[string]string{"k": "v"}, "fallback")
	if result == "fallback" {
		t.Fatalf("expected encoded data, got fallback")
	}
	if !strings.HasPrefix(result, "ns|v1|") {
		t.Fatalf("encoded data missing expected prefix: %q", result)
	}
}

func TestEncodeOrFallbackInvalidNamespace(t *testing.T) {
	t.Parallel()

	result := EncodeOrFallback("", map[string]string{"k": "v"}, "fallback")
	if result != "fallback" {
		t.Fatalf("expected fallback for empty namespace, got %q", result)
	}
}

func TestEncodeOrFallbackOversized(t *testing.T) {
	t.Parallel()

	result := EncodeOrFallback("ns", map[string]string{"w": strings.Repeat("x", 120)}, "fallback")
	if result != "fallback" {
		t.Fatalf("expected fallback for oversized payload, got %q", result)
	}
}

func TestEncodeOrFallbackEmptyFallback(t *testing.T) {
	t.Parallel()

	result := EncodeOrFallback("", map[string]string{"k": "v"}, "")
	if result != "" {
		t.Fatalf("expected empty string fallback, got %q", result)
	}
}

func TestFieldNilReceiver(t *testing.T) {
	t.Parallel()

	var d *Decoded
	val, ok := d.Field("x")
	if val != "" || ok {
		t.Fatalf("nil receiver Field() expected (\"\", false), got (%q, %v)", val, ok)
	}
}

func TestFieldExistingKey(t *testing.T) {
	t.Parallel()

	d := &Decoded{
		Namespace: "test",
		Fields:    map[string]string{"a": "yes"},
	}
	val, ok := d.Field("a")
	if val != "yes" || !ok {
		t.Fatalf("Field(\"a\") expected (\"yes\", true), got (%q, %v)", val, ok)
	}
}

func TestFieldMissingKey(t *testing.T) {
	t.Parallel()

	d := &Decoded{
		Namespace: "test",
		Fields:    map[string]string{"a": "yes"},
	}
	val, ok := d.Field("missing")
	if val != "" || ok {
		t.Fatalf("Field(\"missing\") expected (\"\", false), got (%q, %v)", val, ok)
	}
}

func TestEncodeEmptyFields(t *testing.T) {
	t.Parallel()

	result, err := Encode("ns", map[string]string{})
	if err != nil {
		t.Fatalf("Encode() with empty fields error = %v", err)
	}
	if result != "ns|v1|_" {
		t.Fatalf("expected \"ns|v1|_\", got %q", result)
	}
}

func TestEncodeNilFields(t *testing.T) {
	t.Parallel()

	result, err := Encode("ns", nil)
	if err != nil {
		t.Fatalf("Encode() with nil fields error = %v", err)
	}
	if result != "ns|v1|_" {
		t.Fatalf("expected \"ns|v1|_\", got %q", result)
	}
}

func TestEncodeSkipsEmptyKey(t *testing.T) {
	t.Parallel()

	result, err := Encode("ns", map[string]string{"": "val"})
	if err != nil {
		t.Fatalf("Encode() with empty key error = %v", err)
	}
	// empty key skipped -> payload collapses to "_"
	if result != "ns|v1|_" {
		t.Fatalf("expected \"ns|v1|_\" (empty key skipped), got %q", result)
	}
}

func TestDecodeUnderscorePayload(t *testing.T) {
	t.Parallel()

	d, err := Decode("ns|v1|_")
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if len(d.Fields) != 0 {
		t.Fatalf("expected empty Fields for underscore payload, got %v", d.Fields)
	}
}

func TestDecodeRejectsEmptyString(t *testing.T) {
	t.Parallel()

	_, err := Decode("")
	if !errors.Is(err, ErrInvalidFormat) {
		t.Fatalf("Decode(\"\") expected ErrInvalidFormat, got %v", err)
	}
}

func TestDecodeRejectsBoundaryUnderscoreOnly(t *testing.T) {
	t.Parallel()

	d, err := Decode("ns|v1|_")
	if err != nil {
		t.Fatalf("Decode(\"ns|v1|_\") unexpected error: %v", err)
	}
	if len(d.Fields) != 0 {
		t.Fatalf("expected empty Fields for underscore payload, got %v", d.Fields)
	}
}

func TestDecodeRejectsWrongVersionV2(t *testing.T) {
	t.Parallel()

	_, err := Decode("ns|v2|a=yes")
	if !errors.Is(err, ErrUnsupportedVersion) {
		t.Fatalf("Decode with v2 expected ErrUnsupportedVersion, got %v", err)
	}
}

func TestDecodeRejectsWrongVersionV0(t *testing.T) {
	t.Parallel()

	_, err := Decode("ns|v0|a=yes")
	if !errors.Is(err, ErrUnsupportedVersion) {
		t.Fatalf("Decode with v0 expected ErrUnsupportedVersion, got %v", err)
	}
}

func TestDecodeRejectsCorruptBase64(t *testing.T) {
	t.Parallel()

	tests := []string{
		"ns|v1|%%%malformed%%%",
		"ns|v1|a=%FF%GG",
		"ns|v1|a=%ZZ",
	}
	for _, data := range tests {
		_, err := Decode(data)
		if !errors.Is(err, ErrInvalidFormat) {
			t.Fatalf("expected ErrInvalidFormat for %q, got %v", data, err)
		}
	}
}

func TestDecodeRejectsMissingVersionField(t *testing.T) {
	t.Parallel()

	// Only 2 pipe-separated parts instead of 3
	_, err := Decode("ns|v1")
	if !errors.Is(err, ErrInvalidFormat) {
		t.Fatalf("expected ErrInvalidFormat for missing payload segment, got %v", err)
	}
}

func TestDecodeRejectsEmptyVersion(t *testing.T) {
	t.Parallel()

	_, err := Decode("ns||a=b")
	if !errors.Is(err, ErrUnsupportedVersion) {
		t.Fatalf("expected ErrUnsupportedVersion for empty version, got %v", err)
	}
}

func TestDecodeRejectsPipeOnlyPayload(t *testing.T) {
	t.Parallel()

	// Too few parts after splitting
	_, err := Decode("|")
	if !errors.Is(err, ErrInvalidFormat) {
		t.Fatalf("expected ErrInvalidFormat for single pipe, got %v", err)
	}
	_, err = Decode("||")
	if !errors.Is(err, ErrInvalidNamespace) {
		t.Fatalf("expected ErrInvalidNamespace for two pipes (empty namespace), got %v", err)
	}
}

func TestEncodeNearMaxLength(t *testing.T) {
	t.Parallel()

	// Construct a payload that approaches MaxCallbackDataLen
	// Namespace "test" + version separator "|v1|" = 5 chars for "|v1|"
	// So max field payload = 64 - len("test") - 5 = 55
	// The payload is URL-encoded, so we need to stay within bounds
	result, err := Encode("ns", map[string]string{"k": "v"})
	if err != nil {
		t.Fatalf("Encode() for small payload error = %v", err)
	}
	if len(result) > MaxCallbackDataLen {
		t.Fatalf("encoded data exceeds max length: %d > %d", len(result), MaxCallbackDataLen)
	}
}

func TestRoundTripURLSpecialChars(t *testing.T) {
	t.Parallel()

	fields := map[string]string{
		"q": "a&b=c%25",
	}
	encoded, err := Encode("ns", fields)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	got, ok := decoded.Field("q")
	if !ok {
		t.Fatalf("Field \"q\" not found after round-trip")
	}
	if got != "a&b=c%25" {
		t.Fatalf("Field \"q\" value mismatch: expected %q, got %q", "a&b=c%25", got)
	}
}
