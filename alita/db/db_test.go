package db

import (
	"testing"
)

// TestGetSpanAttributes verifies span attribute generation for various inputs.
func TestGetSpanAttributes(t *testing.T) {
	tests := []struct {
		name         string
		model        any
		wantLen      int
		wantKey      string
		wantTypeName string
	}{
		{
			name:    "nil input",
			model:   nil,
			wantLen: 0,
		},
		{
			name:         "struct pointer",
			model:        &User{},
			wantLen:      1,
			wantKey:      "db.model",
			wantTypeName: "*models.User",
		},
		{
			name:         "string value",
			model:        "hello",
			wantLen:      1,
			wantKey:      "db.model",
			wantTypeName: "string",
		},
		{
			name:         "int value",
			model:        42,
			wantLen:      1,
			wantKey:      "db.model",
			wantTypeName: "int",
		},
		{
			name:         "slice value",
			model:        []int{1, 2, 3},
			wantLen:      1,
			wantKey:      "db.model",
			wantTypeName: "[]int",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := getSpanAttributes(tc.model)

			if len(got) != tc.wantLen {
				t.Fatalf("getSpanAttributes() len = %d, want %d", len(got), tc.wantLen)
			}

			if tc.wantLen == 0 {
				return
			}

			if len(got) != 1 {
				t.Fatalf("expected exactly 1 attribute, got %d", len(got))
			}

			attr := got[0]
			if string(attr.Key) != tc.wantKey {
				t.Errorf("attribute key = %q, want %q", attr.Key, tc.wantKey)
			}

			val := attr.Value
			if val.AsString() != tc.wantTypeName {
				t.Errorf("attribute value = %q, want %q", val.AsString(), tc.wantTypeName)
			}
		})
	}
}
