package modules

import (
	"testing"
)

func TestNormalizeRulesForHTML(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		input        string
		wantExact    string // exact expected output; empty means "not checked"
		wantEmpty    bool   // expect empty string
		wantNonEmpty bool   // expect non-empty string (exact output not checked)
		wantSame     bool   // expect output == input (HTML passthrough)
	}{
		{
			name:      "empty string",
			input:     "",
			wantEmpty: true,
		},
		{
			name:      "whitespace only",
			input:     "   ",
			wantEmpty: true,
		},
		{
			name:     "html bold tag",
			input:    "<b>bold</b>",
			wantSame: true,
		},
		{
			name:     "html italic tag",
			input:    "<i>text</i>",
			wantSame: true,
		},
		{
			name:     "html uppercase tag",
			input:    "<B>bold</B>",
			wantSame: true,
		},
		{
			name:     "closing tag only",
			input:    "</b>",
			wantSame: true,
		},
		{
			name:         "plain text",
			input:        "plain text",
			wantNonEmpty: true,
		},
		{
			name:         "markdown bold",
			input:        "**bold**",
			wantNonEmpty: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := normalizeRulesForHTML(tc.input)

			switch {
			case tc.wantEmpty:
				if got != "" {
					t.Errorf("normalizeRulesForHTML(%q) = %q, want empty string", tc.input, got)
				}
			case tc.wantSame:
				if got != tc.input {
					t.Errorf("normalizeRulesForHTML(%q) = %q, want same as input (HTML passthrough)", tc.input, got)
				}
			case tc.wantNonEmpty:
				if got == "" {
					t.Errorf("normalizeRulesForHTML(%q) = empty string, want non-empty", tc.input)
				}
			case tc.wantExact != "":
				if got != tc.wantExact {
					t.Errorf("normalizeRulesForHTML(%q) = %q, want %q", tc.input, got, tc.wantExact)
				}
			}
		})
	}
}
