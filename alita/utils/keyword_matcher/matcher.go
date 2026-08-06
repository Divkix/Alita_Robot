package keyword_matcher

import (
	"hash/fnv"
	"strings"
	"sync"
	"time"

	"github.com/cloudflare/ahocorasick"
	log "github.com/sirupsen/logrus"
)

// KeywordMatcher provides efficient multi-pattern matching using Aho-Corasick algorithm
type KeywordMatcher struct {
	matcher     *ahocorasick.Matcher
	patterns    []string
	patternHash uint64
	mu          sync.RWMutex
	lastBuild   time.Time
}

// hashPatterns computes a hash of the pattern slice for fast comparison.
func hashPatterns(patterns []string) uint64 {
	h := fnv.New64a()
	for _, p := range patterns {
		_, _ = h.Write([]byte(p))
		_, _ = h.Write([]byte{0}) // separator
	}
	return h.Sum64()
}

// newKeywordMatcher creates a keyword matcher with the given patterns.
func newKeywordMatcher(patterns []string) *KeywordMatcher {
	km := &KeywordMatcher{
		patterns:    make([]string, len(patterns)),
		patternHash: hashPatterns(patterns),
	}
	copy(km.patterns, patterns)
	km.build()
	return km
}

// build compiles the patterns into an Aho-Corasick matcher
func (km *KeywordMatcher) build() {
	if len(km.patterns) == 0 {
		km.matcher = nil
		return
	}

	// Convert patterns to lowercase for case-insensitive matching
	lowerPatterns := make([]string, len(km.patterns))
	for i, pattern := range km.patterns {
		lowerPatterns[i] = strings.ToLower(pattern)
	}

	km.matcher = ahocorasick.NewStringMatcher(lowerPatterns)
	km.lastBuild = time.Now()

	log.WithFields(log.Fields{
		"patterns_count": len(km.patterns),
		"build_time":     time.Since(km.lastBuild),
	}).Debug("Built Aho-Corasick matcher")
}

// FirstMatch returns the first pattern that matches the given text.
// ahocorasick.Matcher.Match is not safe for concurrent use, so we hold an
// exclusive lock across the Match call. ToLower is done outside the lock to
// reduce contention.
func (km *KeywordMatcher) FirstMatch(text string) (string, bool) {
	lowerText := strings.ToLower(text)

	km.mu.Lock()
	defer km.mu.Unlock()

	if km.matcher == nil || len(km.patterns) == 0 {
		return "", false
	}

	hits := km.matcher.Match([]byte(lowerText))
	if len(hits) == 0 {
		return "", false
	}

	firstIdx := hits[0]
	if firstIdx >= 0 && firstIdx < len(km.patterns) {
		return km.patterns[firstIdx], true
	}
	return "", false
}
