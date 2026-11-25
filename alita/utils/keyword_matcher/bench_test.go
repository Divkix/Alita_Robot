package keyword_matcher

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	log "github.com/sirupsen/logrus"
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

func BenchmarkFirstMatchParallel(b *testing.B) {
	km := newKeywordMatcher(benchPatterns)
	b.ReportAllocs()
	b.ResetTimer()
	var matched atomic.Bool
	b.RunParallel(func(pb *testing.PB) {
		var ok bool
		for pb.Next() {
			_, ok = km.FirstMatch(benchMessage)
		}
		if ok {
			matched.Store(true)
		}
	})
	if !matched.Load() {
		b.Fatal("expected a match")
	}
}

func BenchmarkKeywordScan(b *testing.B) {
	manyPatterns := append([]string(nil), benchPatterns...)
	for i := range 56 {
		manyPatterns = append(manyPatterns, fmt.Sprintf("blocked phrase #%d", i))
	}

	cases := []struct {
		name     string
		patterns []string
		text     string
		want     string
	}{
		{"single-miss", benchPatterns[:1], "hello everyone, has anyone read the announcement?", ""},
		{"single-hit", benchPatterns[:1], benchMessage, "https://"},
		{"small-miss", benchPatterns, "hello everyone, has anyone read the announcement?", ""},
		{"small-hit", benchPatterns, benchMessage, "https://"},
		{"small-late-hit", benchPatterns, "Has anyone read the announcement about the new rules?", "rules"},
		{"small-unicode-hit", []string{"\u041f\u0420\u0418\u0412\u0415\u0422", "\u00c9T\u00c9", "\U0001f680"}, "BonJOUR, en \u00e9t\u00e9", "\u00c9T\u00c9"},
		{"small-long-miss", benchPatterns, strings.Repeat("hello everyone, has anyone read the announcement? ", 80), ""},
		{"small-repeated-prefix-miss", []string{"aaaab", "aaabb", "aabbb", "abbbb", "baaaa", "bbaaa", "bbbaa", "bbbba"}, strings.Repeat("a", 4096), ""},
		{"many-hit", manyPatterns, benchMessage, "https://"},
		{"many-miss", manyPatterns, "hello everyone, has anyone read the announcement?", ""},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			km := newKeywordMatcher(tc.patterns)
			b.ReportAllocs()
			for b.Loop() {
				got, ok := km.FirstMatch(tc.text)
				if got != tc.want || ok != (tc.want != "") {
					b.Fatalf("FirstMatch() = %q, %t; want %q", got, ok, tc.want)
				}
			}
		})
	}
}

func BenchmarkKeywordBuildSingle(b *testing.B) {
	level := log.GetLevel()
	log.SetLevel(log.InfoLevel)
	b.Cleanup(func() { log.SetLevel(level) })
	b.ReportAllocs()
	for b.Loop() {
		if km := newKeywordMatcher(benchPatterns[:1]); km == nil {
			b.Fatal("expected a matcher")
		}
	}
}
