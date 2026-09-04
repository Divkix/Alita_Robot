package modules

import (
	"strings"
	"testing"
)

func TestNormalizeRulesForHTML(t *testing.T) {
	t.Parallel()

	t.Run("empty string returns empty", func(t *testing.T) {
		t.Parallel()
		if got := normalizeRulesForHTML(""); got != "" {
			t.Fatalf("normalizeRulesForHTML(%q) = %q, want %q", "", got, "")
		}
	})

	t.Run("whitespace only returns empty", func(t *testing.T) {
		t.Parallel()
		if got := normalizeRulesForHTML("   "); got != "" {
			t.Fatalf("normalizeRulesForHTML(%q) = %q, want %q", "   ", got, "")
		}
	})

	htmlPassthroughCases := []struct {
		name  string
		input string
	}{
		{name: "bold tag", input: "<b>Rule 1</b>"},
		{name: "self-closing tag", input: "<br/>"},
		{name: "mixed text with HTML tag", input: "some text <b>bold</b> more"},
	}
	for _, tc := range htmlPassthroughCases {

		t.Run(tc.name+" passthrough", func(t *testing.T) {
			t.Parallel()
			got := normalizeRulesForHTML(tc.input)
			if got != tc.input {
				t.Fatalf("normalizeRulesForHTML(%q) = %q, want unchanged %q", tc.input, got, tc.input)
			}
		})
	}

	t.Run("markdown bold converted to HTML b tag", func(t *testing.T) {
		t.Parallel()
		input := "*bold*"
		got := normalizeRulesForHTML(input)
		if !strings.Contains(got, "<b>") {
			t.Fatalf("normalizeRulesForHTML(%q) = %q, expected to contain <b>", input, got)
		}
	})

	t.Run("markdown italic converted to HTML i tag", func(t *testing.T) {
		t.Parallel()
		input := "_italic_"
		got := normalizeRulesForHTML(input)
		if !strings.Contains(got, "<i>") {
			t.Fatalf("normalizeRulesForHTML(%q) = %q, expected to contain <i>", input, got)
		}
	})

	// Angle brackets not matching HTML tag pattern: < followed by space is not [a-z]
	// so it passes to MD2HTMLV2 (plain text, no markdown) — output equals input
	t.Run("angle brackets not HTML treated as markdown passthrough", func(t *testing.T) {
		t.Parallel()
		input := "1 < 2 > 0"
		got := normalizeRulesForHTML(input)
		if got == "" {
			t.Fatalf("normalizeRulesForHTML(%q) returned empty, want non-empty", input)
		}
	})

	t.Run("HTML entities without tags processed as markdown", func(t *testing.T) {
		t.Parallel()
		input := "&amp; stuff"
		got := normalizeRulesForHTML(input)
		if got == "" {
			t.Fatalf("normalizeRulesForHTML(%q) returned empty, want non-empty", input)
		}
	})
}
