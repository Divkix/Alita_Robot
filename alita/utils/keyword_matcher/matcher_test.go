package keyword_matcher

import (
	"sync"
	"testing"
)

func TestNewKeywordMatcher(t *testing.T) {
	t.Parallel()

	patterns := []string{"hello", "world", "foo"}
	km := NewKeywordMatcher(patterns)
	if km == nil {
		t.Fatalf("NewKeywordMatcher() returned nil")
	}

	got := km.GetPatterns()
	if len(got) != len(patterns) {
		t.Fatalf("GetPatterns() len = %d, want %d", len(got), len(patterns))
	}
	for i, p := range patterns {
		if got[i] != p {
			t.Fatalf("GetPatterns()[%d] = %q, want %q", i, got[i], p)
		}
	}
}

func TestNewKeywordMatcherEmpty(t *testing.T) {
	t.Parallel()

	km := NewKeywordMatcher([]string{})
	if km == nil {
		t.Fatalf("NewKeywordMatcher() returned nil for empty patterns")
	}
	if km.HasMatch("anything") {
		t.Fatalf("HasMatch() = true for matcher with no patterns, want false")
	}
}

func TestNewKeywordMatcherNilPatterns(t *testing.T) {
	t.Parallel()

	km := NewKeywordMatcher(nil)
	if km == nil {
		t.Fatalf("NewKeywordMatcher(nil) returned nil")
	}
	if km.HasMatch("anything") {
		t.Fatalf("HasMatch() = true for nil-initialized matcher, want false")
	}
}

func TestFindMatches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		patterns  []string
		text      string
		wantCount int
		wantNil   bool
	}{
		{
			name:      "two distinct patterns both present",
			patterns:  []string{"hello", "world"},
			text:      "hello world",
			wantCount: 2,
		},
		{
			name:      "case insensitive match",
			patterns:  []string{"hello"},
			text:      "HELLO",
			wantCount: 1,
		},
		{
			name:      "overlapping matches",
			patterns:  []string{"ab"},
			text:      "ababab",
			wantCount: 3,
		},
		{
			name:     "empty text returns nil",
			patterns: []string{"hello"},
			text:     "",
			wantNil:  true,
		},
		{
			name:     "empty patterns returns nil",
			patterns: []string{},
			text:     "hello",
			wantNil:  true,
		},
		{
			name:      "regex metachar matched literally - dot",
			patterns:  []string{"foo.bar"},
			text:      "foo.bar",
			wantCount: 1,
		},
		{
			name:     "no match returns nil",
			patterns: []string{"hello"},
			text:     "goodbye",
			wantNil:  true,
		},
		{
			name:      "pattern at start of text",
			patterns:  []string{"start"},
			text:      "start of text",
			wantCount: 1,
		},
		{
			name:      "pattern at end of text",
			patterns:  []string{"end"},
			text:      "text at end",
			wantCount: 1,
		},
	}

	for _, tc := range tests {

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			km := NewKeywordMatcher(tc.patterns)
			results := km.FindMatches(tc.text)

			if tc.wantNil {
				if results != nil {
					t.Fatalf("FindMatches() = %v, want nil", results)
				}
				return
			}

			if len(results) != tc.wantCount {
				t.Fatalf("FindMatches() count = %d, want %d; results = %v", len(results), tc.wantCount, results)
			}
		})
	}
}

func TestHasMatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		patterns []string
		text     string
		want     bool
	}{
		{
			name:     "pattern present in text",
			patterns: []string{"hello"},
			text:     "say hello there",
			want:     true,
		},
		{
			name:     "pattern not present",
			patterns: []string{"hello"},
			text:     "goodbye",
			want:     false,
		},
		{
			name:     "empty text",
			patterns: []string{"hello"},
			text:     "",
			want:     false,
		},
		{
			name:     "empty patterns",
			patterns: []string{},
			text:     "hello",
			want:     false,
		},
		{
			name:     "case insensitive",
			patterns: []string{"hello"},
			text:     "HELLO WORLD",
			want:     true,
		},
		{
			name:     "multiple patterns one matches",
			patterns: []string{"alpha", "beta", "gamma"},
			text:     "testing gamma now",
			want:     true,
		},
		{
			name:     "multiple patterns none match",
			patterns: []string{"alpha", "beta", "gamma"},
			text:     "testing delta now",
			want:     false,
		},
	}

	for _, tc := range tests {

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			km := NewKeywordMatcher(tc.patterns)
			got := km.HasMatch(tc.text)
			if got != tc.want {
				t.Fatalf("HasMatch(%q) = %v, want %v", tc.text, got, tc.want)
			}
		})
	}
}

func TestGetPatterns(t *testing.T) {
	t.Parallel()

	original := []string{"alpha", "beta", "gamma"}
	km := NewKeywordMatcher(original)

	got := km.GetPatterns()
	if len(got) != len(original) {
		t.Fatalf("GetPatterns() len = %d, want %d", len(got), len(original))
	}

	// Mutate the returned slice and verify internal state is unchanged
	got[0] = "mutated"

	second := km.GetPatterns()
	if second[0] != original[0] {
		t.Fatalf("GetPatterns() defensive copy failed: internal pattern changed to %q", second[0])
	}
}

func TestFindMatchesPositions(t *testing.T) {
	t.Parallel()

	km := NewKeywordMatcher([]string{"world"})
	results := km.FindMatches("hello world")

	if len(results) != 1 {
		t.Fatalf("FindMatches() count = %d, want 1", len(results))
	}

	r := results[0]
	if r.Pattern != "world" {
		t.Fatalf("MatchResult.Pattern = %q, want %q", r.Pattern, "world")
	}
	// "world" starts at index 6 in "hello world"
	if r.Start != 6 {
		t.Fatalf("MatchResult.Start = %d, want 6", r.Start)
	}
	// "world" ends at index 11
	if r.End != 11 {
		t.Fatalf("MatchResult.End = %d, want 11", r.End)
	}
}

func TestConcurrentAccess(t *testing.T) {
	t.Parallel()

	km := NewKeywordMatcher([]string{"hello", "world", "concurrent"})

	const goroutines = 10
	const callsEach = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for range goroutines {
		go func() {
			defer wg.Done()
			for range callsEach {
				_ = km.FindMatches("hello world concurrent test")
				_ = km.HasMatch("hello world concurrent test")
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

			km := NewKeywordMatcher([]string{tc.pattern})
			got := km.HasMatch(tc.text)
			if got != tc.want {
				t.Fatalf("HasMatch(%q) with pattern %q = %v, want %v", tc.text, tc.pattern, got, tc.want)
			}
		})
	}
}
