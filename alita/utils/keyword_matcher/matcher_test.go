package keyword_matcher

import (
	"sync"
	"testing"
)

func TestNewKeywordMatcher(t *testing.T) {
	t.Parallel()

	patterns := []string{"hello", "world", "foo"}
	km := newKeywordMatcher(patterns)
	if km == nil {
		t.Fatalf("NewKeywordMatcher() returned nil")
	}

	got, ok := km.FirstMatch("say hello")
	if !ok || got != "hello" {
		t.Fatalf("FirstMatch() = %q, %v, want %q, true", got, ok, "hello")
	}
}

func TestNewKeywordMatcherEmpty(t *testing.T) {
	t.Parallel()

	km := newKeywordMatcher([]string{})
	if km == nil {
		t.Fatalf("NewKeywordMatcher() returned nil for empty patterns")
	}
	if _, ok := km.FirstMatch("anything"); ok {
		t.Fatalf("FirstMatch() matched with no patterns")
	}
}

func TestNewKeywordMatcherNilPatterns(t *testing.T) {
	t.Parallel()

	km := newKeywordMatcher(nil)
	if km == nil {
		t.Fatalf("NewKeywordMatcher(nil) returned nil")
	}
	if _, ok := km.FirstMatch("anything"); ok {
		t.Fatalf("FirstMatch() matched with nil patterns")
	}
}

func TestFirstMatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		patterns    []string
		text        string
		wantPattern string
		wantOK      bool
	}{
		{
			name:        "first matching pattern returned",
			patterns:    []string{"alpha", "beta", "gamma"},
			text:        "xx beta gamma",
			wantPattern: "beta",
			wantOK:      true,
		},
		{
			name:        "case insensitive first match preserves original pattern",
			patterns:    []string{"HeLLo"},
			text:        "say hello",
			wantPattern: "HeLLo",
			wantOK:      true,
		},
		{
			name:     "no match",
			patterns: []string{"alpha"},
			text:     "omega",
			wantOK:   false,
		},
		{
			name:     "empty patterns",
			patterns: nil,
			text:     "alpha",
			wantOK:   false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			km := newKeywordMatcher(tc.patterns)
			gotPattern, gotOK := km.FirstMatch(tc.text)
			if gotOK != tc.wantOK {
				t.Fatalf("FirstMatch() ok = %v, want %v", gotOK, tc.wantOK)
			}
			if gotPattern != tc.wantPattern {
				t.Fatalf("FirstMatch() pattern = %q, want %q", gotPattern, tc.wantPattern)
			}
		})
	}
}

func TestConcurrentAccess(t *testing.T) {
	t.Parallel()

	km := newKeywordMatcher([]string{"hello", "world", "concurrent"})

	const goroutines = 10
	const callsEach = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for range goroutines {
		go func() {
			defer wg.Done()
			for range callsEach {
				_, _ = km.FirstMatch("hello world concurrent test")
			}
		}()
	}

	wg.Wait()
}

func TestSpecialCharacterPatterns(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		pattern string
		text    string
		want    bool
	}{
		{
			name:    "dot is matched literally",
			pattern: "foo.bar",
			text:    "foo.bar baz",
			want:    true,
		},
		{
			name:    "dot does not match arbitrary char",
			pattern: "foo.bar",
			text:    "fooXbar",
			want:    false,
		},
		{
			name:    "bracket expression matched literally",
			pattern: "[test]",
			text:    "this [test] value",
			want:    true,
		},
		{
			name:    "parentheses matched literally",
			pattern: "(abc)",
			text:    "value (abc) end",
			want:    true,
		},
	}

	for _, tc := range tests {

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			km := newKeywordMatcher([]string{tc.pattern})
			_, got := km.FirstMatch(tc.text)
			if got != tc.want {
				t.Fatalf("FirstMatch(%q) with pattern %q matched = %v, want %v", tc.text, tc.pattern, got, tc.want)
			}
		})
	}
}
