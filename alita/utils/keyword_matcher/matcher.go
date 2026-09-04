package keyword_matcher

import (
	"strings"
	"sync"
	"time"

	"github.com/cloudflare/ahocorasick"
	log "github.com/sirupsen/logrus"
)

type KeywordMatcher struct {
	matcher   *ahocorasick.Matcher
	patterns  []string
	mu        sync.RWMutex
	lastBuild time.Time
}

func newKeywordMatcher(patterns []string) *KeywordMatcher {
	km := &KeywordMatcher{
		patterns: make([]string, len(patterns)),
	}
	copy(km.patterns, patterns)
	km.build()
	return km
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
	isNil := km.matcher == nil || len(km.patterns) == 0
	km.mu.RUnlock()
	if isNil {
		return "", false
	}

	km.mu.Lock()
	defer km.mu.Unlock()

	if km.matcher == nil || len(km.patterns) == 0 {
		return "", false
	}

	// ponytail: safe copy retained, switch to unsafe.Slice if pprof shows alloc
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
