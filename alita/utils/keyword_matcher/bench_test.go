package keyword_matcher

import (
	"sync/atomic"
	"testing"
)

const benchMessage = "hey everyone please check out https://example.com/deals for the " +
	"latest offers and remember to read the rules before posting anything here"

var benchPatterns = []string{
	"https://", "deals", "rules", "buy now", "free money", "crypto", "casino", "viagra",
}

func BenchmarkFirstMatch(b *testing.B) {
	km := newKeywordMatcher(benchPatterns)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, ok := km.FirstMatch(benchMessage); !ok {
			b.Fatal("expected a match")
		}
	}
}

// BenchmarkFirstMatchParallel guards the read-lock path: FirstMatch runs under
// km.mu.RLock via MatchThreadSafe, so concurrent readers must scale. Reverting to
// Matcher.Match under a write lock makes this benchmark collapse instead.
func BenchmarkFirstMatchParallel(b *testing.B) {
	km := newKeywordMatcher(benchPatterns)
	b.ReportAllocs()
	b.ResetTimer()
	var matched atomic.Bool
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, ok := km.FirstMatch(benchMessage); ok {
				matched.Store(true)
			}
		}
	})
	if !matched.Load() {
		b.Fatal("expected a match")
	}
}
