package keyword_matcher

import (
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
)

// Cache manages keyword matchers for different chats
type Cache struct {
	matchers map[int64]*KeywordMatcher
	mu       sync.RWMutex
	ttl      time.Duration
	lastUsed map[int64]time.Time
}

// NewCache creates a new keyword matcher cache
func NewCache(ttl time.Duration) *Cache {
	return &Cache{
		matchers: make(map[int64]*KeywordMatcher),
		lastUsed: make(map[int64]time.Time),
		ttl:      ttl,
	}
}

// GetOrCreateMatcher gets or creates a keyword matcher for the given chat
func (c *Cache) GetOrCreateMatcher(chatID int64, patterns []string) *KeywordMatcher {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Update last used time
	c.lastUsed[chatID] = time.Now()

	// Check if matcher exists
	if matcher, exists := c.matchers[chatID]; exists {
		// Check if patterns have changed
		existingPatterns := matcher.GetPatterns()
		if patternsEqual(existingPatterns, patterns) {
			return matcher
		}
	}

	// Create new matcher
	matcher := NewKeywordMatcher(patterns)
	c.matchers[chatID] = matcher

	log.WithFields(log.Fields{
		"chatID":        chatID,
		"pattern_count": len(patterns),
	}).Debug("Created/updated keyword matcher")

	return matcher
}

// CleanupExpired removes expired matchers based on TTL
func (c *Cache) CleanupExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	expiredChats := make([]int64, 0)

	for chatID, lastUsed := range c.lastUsed {
		if now.Sub(lastUsed) > c.ttl {
			expiredChats = append(expiredChats, chatID)
		}
	}

	for _, chatID := range expiredChats {
		delete(c.matchers, chatID)
		delete(c.lastUsed, chatID)
	}

	if len(expiredChats) > 0 {
		log.WithField("expired_count", len(expiredChats)).Debug("Cleaned up expired keyword matchers")
	}
}

// patternsEqual checks if two pattern slices are equal
func patternsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	// Create maps for efficient comparison
	aMap := make(map[string]bool)
	for _, pattern := range a {
		aMap[pattern] = true
	}

	for _, pattern := range b {
		if !aMap[pattern] {
			return false
		}
	}

	return true
}

// Global cache instance
var (
	globalCache *Cache
	once        sync.Once
)

// GetGlobalCache returns the singleton keyword matcher cache
func GetGlobalCache() *Cache {
	once.Do(func() {
		globalCache = NewCache(30 * time.Minute) // 30 minute TTL
		// Start cleanup routine
		go func() {
			defer error_handling.RecoverFromPanic("GetGlobalCache.cleanupRoutine", "keyword_matcher")
			ticker := time.NewTicker(10 * time.Minute)
			defer ticker.Stop()
			for range ticker.C {
				globalCache.CleanupExpired()
			}
		}()
	})
	return globalCache
}
