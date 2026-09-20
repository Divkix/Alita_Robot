package keyword_matcher

import (
	"slices"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
)

type Cache struct {
	matchers map[int64]*KeywordMatcher
	mu       sync.RWMutex
	ttl      time.Duration
}

func newCache(ttl time.Duration) *Cache {
	return &Cache{
		matchers: make(map[int64]*KeywordMatcher),
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
			matcher.touch()
			return matcher
		}
	}
	c.mu.RUnlock()

	c.mu.Lock()

	if matcher, exists := c.matchers[chatID]; exists {
		if slices.Equal(matcher.patterns, patterns) {
			c.mu.Unlock()
			matcher.touch()
			return matcher
		}
	}

	matcher = newKeywordMatcher(patterns)
	c.matchers[chatID] = matcher
	c.mu.Unlock()

	log.WithFields(log.Fields{
		"chatID":        chatID,
		"pattern_count": len(patterns),
	}).Debug("Created/updated keyword matcher")

	return matcher
}

func (c *Cache) cleanupExpired() {
	now := time.Now()

	c.mu.RLock()
	expiredChats := make([]int64, 0)
	for chatID, matcher := range c.matchers {
		if now.Sub(matcher.lastUsedTime()) > c.ttl {
			expiredChats = append(expiredChats, chatID)
		}
	}
	c.mu.RUnlock()

	if len(expiredChats) == 0 {
		return
	}

	c.mu.Lock()
	for _, chatID := range expiredChats {
		matcher, ok := c.matchers[chatID]
		if !ok || time.Since(matcher.lastUsedTime()) <= c.ttl {
			continue
		}
		delete(c.matchers, chatID)
	}
	c.mu.Unlock()

	log.WithField("expired_count", len(expiredChats)).Debug("Cleaned up expired keyword matchers")
}

var namedCaches sync.Map // name -> *Cache

func GetNamedCache(name string) *Cache {
	if c, ok := namedCaches.Load(name); ok {
		return c.(*Cache)
	}

	c := newCache(30 * time.Minute)
	actual, loaded := namedCaches.LoadOrStore(name, c)
	if loaded {
		return actual.(*Cache)
	}

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
