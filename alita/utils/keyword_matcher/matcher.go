package keyword_matcher

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cloudflare/ahocorasick"
	log "github.com/sirupsen/logrus"
)

// KeywordMatcher provides efficient multi-pattern matching using Aho-Corasick algorithm
type KeywordMatcher struct {
	matcher   *ahocorasick.Matcher
	patterns  []string
	mu        sync.RWMutex
	lastBuild time.Time
}

// MatchResult contains information about a matched pattern
type MatchResult struct {
	Pattern string // The original pattern that matched
	Start   int    // Start position of match in text
	End     int    // End position of match in text
}

// NewKeywordMatcher creates a new keyword matcher with the given patterns
func NewKeywordMatcher(patterns []string) *KeywordMatcher {
	km := &KeywordMatcher{
		patterns: make([]string, len(patterns)),
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

// FindMatches returns all matches in the given text
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

// findMatchesWithPositions finds all matches with their positions in the text
// This implementation uses the Aho-Corasick matcher for efficient multi-pattern matching
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
	seen := make(map[string]bool)

	// Convert hits to matchInfo
	for _, hit := range hits {
		// hit contains the pattern index and end position
		patternIdx := hit
		if patternIdx >= len(km.patterns) {
			continue
		}

		pattern := strings.ToLower(km.patterns[patternIdx])
		patternLen := len(pattern)

		// Find all occurrences of this pattern in the text
		textStr := string(text)
		lowerTextStr := strings.ToLower(textStr)
		searchStart := 0

		for {
			pos := strings.Index(lowerTextStr[searchStart:], pattern)
			if pos == -1 {
				break
			}

			actualPos := searchStart + pos
			key := fmt.Sprintf("%d:%d", patternIdx, actualPos)
			if !seen[key] {
				seen[key] = true
				allMatches = append(allMatches, matchInfo{
					PatternIndex: patternIdx,
					Start:        actualPos,
					End:          actualPos + patternLen,
				})
			}

			searchStart = actualPos + 1
		}
	}

	return allMatches
}

// HasMatch returns true if any pattern matches the text
func (km *KeywordMatcher) HasMatch(text string) bool {
	km.mu.Lock()
	defer km.mu.Unlock()

	if km.matcher == nil {
		return false
	}

	lowerText := strings.ToLower(text)
	hits := km.matcher.Match([]byte(lowerText))
	return len(hits) > 0
}

// GetPatterns returns a copy of the current patterns
func (km *KeywordMatcher) GetPatterns() []string {
	km.mu.RLock()
	defer km.mu.RUnlock()

	patterns := make([]string, len(km.patterns))
	copy(patterns, km.patterns)
	return patterns
}
