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
