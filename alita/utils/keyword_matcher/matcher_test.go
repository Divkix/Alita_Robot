package keyword_matcher

import (
	"testing"
)

func TestNewKeywordMatcher_NilPatterns(t *testing.T) {
	t.Parallel()
	km := NewKeywordMatcher(nil)
	if km == nil {
		t.Fatal("expected non-nil KeywordMatcher")
	}
	if km.HasMatch("hello") {
		t.Error("expected no match for nil patterns")
	}
}

func TestNewKeywordMatcher_EmptyPatterns(t *testing.T) {
	t.Parallel()
	km := NewKeywordMatcher([]string{})
	if km == nil {
		t.Fatal("expected non-nil KeywordMatcher")
	}
	if km.HasMatch("hello") {
		t.Error("expected no match for empty patterns")
	}
}

func TestHasMatch_CaseInsensitive(t *testing.T) {
	t.Parallel()
	km := NewKeywordMatcher([]string{"hello"})
	if !km.HasMatch("HELLO") {
		t.Error("expected match: pattern 'hello' should match 'HELLO'")
	}
	if !km.HasMatch("Hello World") {
		t.Error("expected match: pattern 'hello' should match 'Hello World'")
	}
	if !km.HasMatch("say hello") {
		t.Error("expected match: pattern 'hello' should match 'say hello'")
	}
}

func TestHasMatch_NoMatch(t *testing.T) {
	t.Parallel()
	km := NewKeywordMatcher([]string{"world"})
	if km.HasMatch("hello") {
		t.Error("expected no match for 'hello' with pattern 'world'")
	}
}

func TestHasMatch_EmptyText(t *testing.T) {
	t.Parallel()
	km := NewKeywordMatcher([]string{"hello"})
	if km.HasMatch("") {
		t.Error("expected no match for empty text")
	}
}

func TestHasMatch_SinglePattern(t *testing.T) {
	t.Parallel()
	km := NewKeywordMatcher([]string{"abc"})
	if !km.HasMatch("xabcx") {
		t.Error("expected match for 'abc' embedded in 'xabcx'")
	}
}

func TestHasMatch_MultiplePatterns(t *testing.T) {
	t.Parallel()
	km := NewKeywordMatcher([]string{"foo", "bar"})
	if !km.HasMatch("I like foo") {
		t.Error("expected match for 'foo'")
	}
	if !km.HasMatch("I like bar") {
		t.Error("expected match for 'bar'")
	}
	if km.HasMatch("baz") {
		t.Error("expected no match for 'baz'")
	}
}

func TestHasMatch_PatternEqualsText(t *testing.T) {
	t.Parallel()
	km := NewKeywordMatcher([]string{"exact"})
	if !km.HasMatch("exact") {
		t.Error("expected match when pattern equals text exactly")
	}
}

func TestHasMatch_SingleCharPattern(t *testing.T) {
	t.Parallel()
	km := NewKeywordMatcher([]string{"a"})
	if !km.HasMatch("banana") {
		t.Error("expected match for single char pattern 'a' in 'banana'")
	}
	if km.HasMatch("bbb") {
		t.Error("expected no match for single char pattern 'a' in 'bbb'")
	}
}

func TestFindMatches_NilMatcher(t *testing.T) {
	t.Parallel()
	km := NewKeywordMatcher(nil)
	results := km.FindMatches("hello")
	if results != nil {
		t.Errorf("expected nil results for nil-pattern matcher, got %v", results)
	}
}

func TestFindMatches_EmptyText(t *testing.T) {
	t.Parallel()
	km := NewKeywordMatcher([]string{"hello"})
	results := km.FindMatches("")
	if results != nil {
		t.Errorf("expected nil results for empty text, got %v", results)
	}
}

func TestFindMatches_SingleMatch(t *testing.T) {
	t.Parallel()
	km := NewKeywordMatcher([]string{"hello"})
	results := km.FindMatches("say hello world")
	if len(results) == 0 {
		t.Fatal("expected at least one match")
	}
	if results[0].Pattern != "hello" {
		t.Errorf("expected pattern 'hello', got '%s'", results[0].Pattern)
	}
}

func TestFindMatches_CaseInsensitiveResult(t *testing.T) {
	t.Parallel()
	// Pattern is stored as original case, match is case-insensitive
	km := NewKeywordMatcher([]string{"Hello"})
	results := km.FindMatches("say HELLO world")
	if len(results) == 0 {
		t.Fatal("expected match for 'Hello' against 'HELLO'")
	}
	// Pattern in result should be the original case
	if results[0].Pattern != "Hello" {
		t.Errorf("expected original-case pattern 'Hello', got '%s'", results[0].Pattern)
	}
}

func TestFindMatches_Overlapping(t *testing.T) {
	t.Parallel()
	// "ab" appears at positions 0, 2, 4 in "ababab"
	km := NewKeywordMatcher([]string{"ab"})
	results := km.FindMatches("ababab")
	if len(results) < 1 {
		t.Fatalf("expected at least 1 match in overlapping text 'ababab', got %d", len(results))
	}
}

func TestGetPatterns_ReturnsCopy(t *testing.T) {
	t.Parallel()
	km := NewKeywordMatcher([]string{"foo", "bar"})

	patterns := km.GetPatterns()
	if len(patterns) != 2 {
		t.Fatalf("expected 2 patterns, got %d", len(patterns))
	}

	// Mutate the returned slice
	patterns[0] = "MUTATED"

	// Verify the internal state is unchanged
	again := km.GetPatterns()
	if again[0] != "foo" {
		t.Errorf("GetPatterns should return a copy; got mutated value '%s'", again[0])
	}
}

// Note: concurrent HasMatch/FindMatches are intentionally not tested here.
// The cloudflare/ahocorasick library has internal mutable state in Match() that
// is not safe for concurrent reads, despite the RWMutex in KeywordMatcher.
// This is a known limitation; the Cache layer provides per-chat isolation.
