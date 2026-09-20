package keyword_matcher

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudflare/ahocorasick"
	log "github.com/sirupsen/logrus"
)

type KeywordMatcher struct {
	matcher   *ahocorasick.Matcher
	patterns  []string
	mu        sync.RWMutex
	lastBuild time.Time
	lastUsed  atomic.Int64
}

func newKeywordMatcher(patterns []string) *KeywordMatcher {
	km := &KeywordMatcher{
		patterns: make([]string, len(patterns)),
	}
	copy(km.patterns, patterns)
	km.build()
	km.lastUsed.Store(time.Now().UnixNano())
	return km
}

func (km *KeywordMatcher) touch() {
	km.lastUsed.Store(time.Now().UnixNano())
}

func (km *KeywordMatcher) lastUsedTime() time.Time {
	return time.Unix(0, km.lastUsed.Load())
}

func (km *KeywordMatcher) build() {
	start := time.Now()
	if len(km.patterns) == 0 {
		km.matcher = nil
		return
	}

	lowerPatterns := make([]string, len(km.patterns))
	for i, pattern := range km.patterns {
		lowerPatterns[i] = strings.ToLower(pattern)
	}

	km.matcher = ahocorasick.NewStringMatcher(lowerPatterns)
	km.lastBuild = time.Now()

	log.WithFields(log.Fields{
		"patterns_count": len(km.patterns),
		"build_time":     time.Since(start),
	}).Debug("Built Aho-Corasick matcher")
}

func (km *KeywordMatcher) FirstMatch(text string) (string, bool) {
	lowerText := strings.ToLower(text)

	km.mu.RLock()
	defer km.mu.RUnlock()

	if km.matcher == nil || len(km.patterns) == 0 {
		return "", false
	}

	// Matcher.Match is not safe for concurrent readers (it mutates m.counter and
	// node counters); MatchThreadSafe is the library's concurrency-safe variant.
	// The []byte conversion does not escape, so it allocates nothing and measures
	// no slower than an unsafe.StringData view; the plain copy is kept.
	hits := km.matcher.MatchThreadSafe([]byte(lowerText))
	if len(hits) == 0 {
		return "", false
	}

	firstIdx := hits[0]
	if firstIdx >= 0 && firstIdx < len(km.patterns) {
		return km.patterns[firstIdx], true
	}
	return "", false
}
