package keyword_matcher

import (
	"fmt"
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
	mu          sync.Mutex
	lastBuild   time.Time
}

// MatchResult contains information about a matched pattern
type MatchResult struct {
	Pattern string // The original pattern that matched
	Start   int    // Start position of match in text
	End     int    // End position of match in text
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

// NewKeywordMatcher creates a new keyword matcher with the given patterns
func NewKeywordMatcher(patterns []string) *KeywordMatcher {
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
// This is optimized for the common case where only the first match is needed,
// avoiding the expensive position-finding second scan used by FindMatches.
func (km *KeywordMatcher) FirstMatch(text string) (string, bool) {
	km.mu.Lock()
	defer km.mu.Unlock()

	if km.matcher == nil || len(km.patterns) == 0 {
		return "", false
	}

	lowerText := strings.ToLower(text)
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

// FindMatches returns all matches in the given text.
// NOTE: This performs an expensive second scan to find match positions.
// For performance-critical code that only needs the first match, use FirstMatch.
func (km *KeywordMatcher) FindMatches(text string) []MatchResult {
	km.mu.Lock()
	defer km.mu.Unlock()

	if km.matcher == nil {
		return nil
	}

	lowerText := strings.ToLower(text)
	matches := km.findMatchesWithPositions([]byte(lowerText))

	if len(matches) == 0 {
		return nil
	}

	results := make([]MatchResult, 0, len(matches))
	for _, match := range matches {
		if match.PatternIndex < len(km.patterns) {
			pattern := km.patterns[match.PatternIndex]
			results = append(results, MatchResult{
				Pattern: pattern,
				Start:   match.Start,
				End:     match.End,
			})
		}
	}

	return results
}

// matchInfo holds information about a match including position
type matchInfo struct {
	PatternIndex int
	Start        int
	End          int
}

// findMatchesWithPositions finds all matches with their positions in the text.
// Uses the Aho-Corasick matcher for efficient multi-pattern matching.
func (km *KeywordMatcher) findMatchesWithPositions(text []byte) []matchInfo {
	if len(text) == 0 || len(km.patterns) == 0 || km.matcher == nil {
		return nil
	}

	// Use the Aho-Corasick matcher to find all matches
	hits := km.matcher.Match(text)
	if len(hits) == 0 {
		return nil
	}

	var allMatches []matchInfo
	seen := make(map[string]bool, len(hits))

	// Convert hits to matchInfo with positions
	textStr := string(text)
	for _, hit := range hits {
		if hit >= len(km.patterns) {
			continue
		}

		pattern := strings.ToLower(km.patterns[hit])
		patternLen := len(pattern)

		// Find all occurrences of this pattern in the text
		searchStart := 0
		for {
			pos := strings.Index(textStr[searchStart:], pattern)
			if pos == -1 {
				break
			}

			actualPos := searchStart + pos
			key := fmt.Sprintf("%d:%d", hit, actualPos)
			if !seen[key] {
				seen[key] = true
				allMatches = append(allMatches, matchInfo{
					PatternIndex: hit,
					Start:        actualPos,
					End:          actualPos + patternLen,
				})
			}

			searchStart = actualPos + 1
		}
	}

	return allMatches
}

// HasMatch returns true if any pattern matches the text.
func (km *KeywordMatcher) HasMatch(text string) bool {
	km.mu.Lock()
	defer km.mu.Unlock()

	if km.matcher == nil || len(km.patterns) == 0 {
		return false
	}

	lowerText := strings.ToLower(text)
	hits := km.matcher.Match([]byte(lowerText))
	return len(hits) > 0
}

// GetPatterns returns a copy of the current patterns
func (km *KeywordMatcher) GetPatterns() []string {
	km.mu.Lock()
	defer km.mu.Unlock()

	patterns := make([]string, len(km.patterns))
	copy(patterns, km.patterns)
	return patterns
}
