package keyword_matcher

import (
	"context"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
)

// Cache manages keyword matchers for different chats
type Cache struct {
	matchers   map[int64]*KeywordMatcher
	mu         sync.RWMutex
	ttl        time.Duration
	lastUsed   map[int64]time.Time
	lastUsedMu sync.Mutex
	stopChan   chan struct{}
	stopOnce   sync.Once
	cancel     context.CancelFunc
}

// NewCache creates a new keyword matcher cache
func NewCache(ttl time.Duration) *Cache {
	return &Cache{
		matchers: make(map[int64]*KeywordMatcher),
		lastUsed: make(map[int64]time.Time),
		ttl:      ttl,
		stopChan: make(chan struct{}),
	}
}

// GetOrCreateMatcher gets or creates a keyword matcher for the given chat.
// Uses RWMutex for concurrent read access and only takes write lock when
// creating a new matcher or when patterns have changed.
func (c *Cache) GetOrCreateMatcher(chatID int64, patterns []string) *KeywordMatcher {
	// Fast path: read-only check with RLock
	c.mu.RLock()
	matcher, exists := c.matchers[chatID]
	if exists {
		// O(1) hash comparison avoids copying patterns
		if matcher.patternHash == hashPatterns(patterns) {
			c.mu.RUnlock()
			c.touchLastUsed(chatID)
			return matcher
		}
	}
	c.mu.RUnlock()

	// Slow path: need write lock to create or update
	c.mu.Lock()

	// Double-check after acquiring write lock
	if matcher, exists := c.matchers[chatID]; exists {
		if matcher.patternHash == hashPatterns(patterns) {
			c.mu.Unlock()
			c.touchLastUsed(chatID)
			return matcher
		}
	}

	// Create new matcher
	matcher = NewKeywordMatcher(patterns)
	c.matchers[chatID] = matcher
	c.mu.Unlock()
	c.touchLastUsed(chatID)

	log.WithFields(log.Fields{
		"chatID":        chatID,
		"pattern_count": len(patterns),
	}).Debug("Created/updated keyword matcher")

	return matcher
}

// touchLastUsed records the current time for a chat under a separate mutex.
// The lastUsed map is read by the cleanup goroutine but written by many
// concurrent request handlers, so it needs its own lock.
func (c *Cache) touchLastUsed(chatID int64) {
	c.lastUsedMu.Lock()
	c.lastUsed[chatID] = time.Now()
	c.lastUsedMu.Unlock()
}

// CleanupExpired removes expired matchers based on TTL
func (c *Cache) CleanupExpired() {
	now := time.Now()

	// Step 1: snapshot expired IDs under lastUsedMu
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

	// Step 2: delete from matchers under mu
	c.mu.Lock()
	for _, chatID := range expiredChats {
		delete(c.matchers, chatID)
	}
	c.mu.Unlock()

	// Step 3: delete from lastUsed under lastUsedMu
	c.lastUsedMu.Lock()
	for _, chatID := range expiredChats {
		delete(c.lastUsed, chatID)
	}
	c.lastUsedMu.Unlock()

	log.WithField("expired_count", len(expiredChats)).Debug("Cleaned up expired keyword matchers")
}

// Stop stops the cleanup goroutine for this cache
// This should be called during shutdown or in tests to prevent goroutine leaks
func (c *Cache) Stop() {
	c.stopOnce.Do(func() {
		if c.cancel != nil {
			c.cancel()
		}
		if c.stopChan != nil {
			close(c.stopChan)
		}
	})
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
		// Create cancellable context for cleanup routine
		ctx, cancel := context.WithCancel(context.Background())
		globalCache.cancel = cancel
		// Start cleanup routine
		go func() {
			defer error_handling.RecoverFromPanic("GetGlobalCache.cleanupRoutine", "keyword_matcher")
			ticker := time.NewTicker(10 * time.Minute)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					globalCache.CleanupExpired()
				case <-ctx.Done():
					log.Debug("Keyword matcher cache cleanup routine stopped")
					return
				case <-globalCache.stopChan:
					log.Debug("Keyword matcher cache cleanup routine stopped via stopChan")
					return
				}
			}
		}()
	})
	return globalCache
}

// namedCaches is a registry of named caches, each isolated from the others.
var (
	namedCaches   = make(map[string]*Cache)
	namedCachesMu sync.Mutex
)

// GetNamedCache returns a per-name singleton keyword matcher cache.
// Each distinct name gets its own independent Cache instance with its own
// 30-minute TTL and cleanup goroutine, so consumers with different pattern
// sets (e.g. "filters" vs "blacklists") can never evict each other's entries.
func GetNamedCache(name string) *Cache {
	namedCachesMu.Lock()
	c, ok := namedCaches[name]
	if ok {
		namedCachesMu.Unlock()
		return c
	}
	c = NewCache(30 * time.Minute)
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	namedCaches[name] = c
	namedCachesMu.Unlock()

	// Start the cleanup goroutine outside the mutex to avoid holding it
	// for the goroutine's lifetime.
	go func() {
		defer error_handling.RecoverFromPanic("GetNamedCache.cleanupRoutine["+name+"]", "keyword_matcher")
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				c.CleanupExpired()
			case <-ctx.Done():
				log.WithField("cache_name", name).Debug("Named keyword matcher cache cleanup routine stopped")
				return
			case <-c.stopChan:
				log.WithField("cache_name", name).Debug("Named keyword matcher cache cleanup routine stopped via stopChan")
				return
			}
		}
	}()

	return c
}
