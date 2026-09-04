package keyword_matcher

import (
	"slices"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
)

type Cache struct {
	matchers   map[int64]*KeywordMatcher
	mu         sync.RWMutex
	ttl        time.Duration
	lastUsed   map[int64]time.Time
	lastUsedMu sync.Mutex
}

func newCache(ttl time.Duration) *Cache {
	return &Cache{
		matchers: make(map[int64]*KeywordMatcher),
		lastUsed: make(map[int64]time.Time),
		ttl:      ttl,
	}
}

func (c *Cache) GetOrCreateMatcher(chatID int64, patterns []string) *KeywordMatcher {
	c.mu.RLock()
	matcher, exists := c.matchers[chatID]
	if exists {
		// ponytail: O(n) compare, patterns are typically tens per chat
		if slices.Equal(matcher.patterns, patterns) {
			c.mu.RUnlock()
			c.touchLastUsed(chatID)
			return matcher
		}
	}
	c.mu.RUnlock()

	c.mu.Lock()

	if matcher, exists := c.matchers[chatID]; exists {
		if slices.Equal(matcher.patterns, patterns) {
			c.mu.Unlock()
			c.touchLastUsed(chatID)
			return matcher
		}
	}

	matcher = newKeywordMatcher(patterns)
	c.matchers[chatID] = matcher
	c.mu.Unlock()
	c.touchLastUsed(chatID)

	log.WithFields(log.Fields{
		"chatID":        chatID,
		"pattern_count": len(patterns),
	}).Debug("Created/updated keyword matcher")

	return matcher
}

func (c *Cache) touchLastUsed(chatID int64) {
	c.lastUsedMu.Lock()
	c.lastUsed[chatID] = time.Now()
	c.lastUsedMu.Unlock()
}

func (c *Cache) cleanupExpired() {
	now := time.Now()

	c.lastUsedMu.Lock()
	expiredChats := make([]int64, 0)
	for chatID, lastUsed := range c.lastUsed {
		if now.Sub(lastUsed) > c.ttl {
			expiredChats = append(expiredChats, chatID)
		}
	}
	c.lastUsedMu.Unlock()

	if len(expiredChats) == 0 {
		return
	}

	c.mu.Lock()
	for _, chatID := range expiredChats {
		c.lastUsedMu.Lock()
		lu, ok := c.lastUsed[chatID]
		expired := ok && time.Since(lu) > c.ttl
		c.lastUsedMu.Unlock()
		if !expired {
			continue
		}
		delete(c.matchers, chatID)
	}
	c.mu.Unlock()

	c.lastUsedMu.Lock()
	for _, chatID := range expiredChats {
		if lu, ok := c.lastUsed[chatID]; ok && time.Since(lu) > c.ttl {
			delete(c.lastUsed, chatID)
		}
	}
	c.lastUsedMu.Unlock()

	log.WithField("expired_count", len(expiredChats)).Debug("Cleaned up expired keyword matchers")
}

var (
	namedCaches   = make(map[string]*Cache)
	namedCachesMu sync.Mutex
)

func GetNamedCache(name string) *Cache {
	namedCachesMu.Lock()
	c, ok := namedCaches[name]
	if ok {
		namedCachesMu.Unlock()
		return c
	}
	c = newCache(30 * time.Minute)
	namedCaches[name] = c
	namedCachesMu.Unlock()

	go func() {
		defer error_handling.RecoverFromPanic("GetNamedCache.cleanupRoutine["+name+"]", "keyword_matcher")
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			func() {
				defer error_handling.RecoverFromPanic("GetNamedCache.cleanupTick["+name+"]", "keyword_matcher")
				c.cleanupExpired()
			}()
		}
	}()

	return c
}
