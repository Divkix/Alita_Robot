//go:build testtools

package keyword_matcher

import (
	"testing"
)

// TestNamedCachesDoNotCollide verifies that two different named caches sharing
// the same chatID store independent matchers and never evict each other's
// entries. It also confirms the "same cache, same patterns" fast-path returns
// the identical pointer (no rebuild).
func TestNamedCachesDoNotCollide(t *testing.T) {
	t.Parallel()

	const chatID = int64(123)
	patternsA := []string{"alpha", "apple"}
	patternsB := []string{"beta", "banana"}

	filtersCache := GetNamedCache("filters_test_collision")
	blacklistsCache := GetNamedCache("blacklists_test_collision")

	// Populate each named cache with its own patterns for the same chatID.
	mA1 := filtersCache.GetOrCreateMatcher(chatID, patternsA)
	mB1 := blacklistsCache.GetOrCreateMatcher(chatID, patternsB)

	if mA1 == nil {
		t.Fatal("filters cache returned nil matcher")
	}
	if mB1 == nil {
		t.Fatal("blacklists cache returned nil matcher")
	}

	// Re-fetch each with the same patterns: must get the same pointer (no rebuild).
	mA2 := filtersCache.GetOrCreateMatcher(chatID, patternsA)
	mB2 := blacklistsCache.GetOrCreateMatcher(chatID, patternsB)

	if mA1 != mA2 {
		t.Error("filters cache: re-fetch with same patterns returned a different pointer — unexpected rebuild")
	}
	if mB1 != mB2 {
		t.Error("blacklists cache: re-fetch with same patterns returned a different pointer — unexpected rebuild")
	}

	// Cross-check: the matchers in the two caches must be distinct objects,
	// confirming they don't share state.
	if mA1 == mB1 {
		t.Error("filters and blacklists caches returned the same matcher pointer for the same chatID — namespacing is broken")
	}

	// Verify pattern hashes differ (the core fix: each cache keeps its own hash).
	if mA1.patternHash == mB1.patternHash {
		t.Errorf("expected different patternHash for different pattern sets; got same hash %x", mA1.patternHash)
	}
}
