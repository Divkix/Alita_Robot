package keyword_matcher

import (
	"strings"
	"sync/atomic"
	"time"

	"github.com/cloudflare/ahocorasick"
	log "github.com/sirupsen/logrus"
)

type KeywordMatcher struct {
	// Matching state is immutable once the cache publishes it.
	matcher       *ahocorasick.Matcher
	patterns      []string
	singlePattern string
	lastUsed      atomic.Int64
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
	switch len(km.patterns) {
	case 0:
		return
	case 1:
		km.singlePattern = strings.ToLower(km.patterns[0])
		return
	}

	start := time.Now()
	lowerPatterns := make([]string, len(km.patterns))
	for i, pattern := range km.patterns {
		lowerPatterns[i] = strings.ToLower(pattern)
	}
	km.matcher = ahocorasick.NewStringMatcher(lowerPatterns)

	log.WithFields(log.Fields{
		"patterns_count": len(km.patterns),
		"build_time":     time.Since(start),
	}).Debug("Built Aho-Corasick matcher")
}

func (km *KeywordMatcher) FirstMatch(text string) (string, bool) {
	if km.matcher == nil {
		if km.singlePattern != "" && strings.Contains(strings.ToLower(text), km.singlePattern) {
			return km.patterns[0], true
		}
		return "", false
	}

	// MatchThreadSafe handles concurrent calls without a lock around the matcher.
	hits := km.matcher.MatchThreadSafe([]byte(strings.ToLower(text)))
	if len(hits) == 0 {
		return "", false
	}
	firstIdx := hits[0]
	if firstIdx >= 0 && firstIdx < len(km.patterns) {
		return km.patterns[firstIdx], true
	}
	return "", false
}
