package cache

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/eko/gocache/lib/v4/store"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/utils/constants"
)

var (
	restrictedCacheHits   atomic.Int64
	restrictedCacheMisses atomic.Int64
)

// restrictedChatKey returns the Redis key for a restricted chat.
func restrictedChatKey(chatID int64) string {
	return fmt.Sprintf("alita:restricted:%d", chatID)
}

// MarkChatRestricted marks a chat as restricted (bot can't send messages).
// The restriction expires after RestrictedCacheTTL (30 min).
func MarkChatRestricted(chatID int64) {
	if Marshal == nil {
		return
	}
	err := Marshal.Set(
		Context,
		restrictedChatKey(chatID),
		time.Now().Format(time.RFC3339),
		store.WithExpiration(constants.RestrictedCacheTTL),
	)
	if err != nil {
		log.WithField("chat_id", chatID).Debugf("[RestrictedCache] Failed to mark chat restricted: %v", err)
	} else {
		log.WithField("chat_id", chatID).Info("[RestrictedCache] Marked chat as restricted")
	}
}

// IsChatRestricted checks if a chat is currently in the restricted cache.
// Returns true if the bot should skip sending to this chat.
// Increments hit/miss counters for monitoring.
func IsChatRestricted(chatID int64) bool {
	if Marshal == nil {
		return false
	}
	var ts string
	_, err := Marshal.Get(Context, restrictedChatKey(chatID), &ts)
	if err != nil {
		restrictedCacheMisses.Add(1)
		return false
	}
	restrictedCacheHits.Add(1)
	log.WithField("chat_id", chatID).Debugf("[RestrictedCache] Cache hit — skipping send to restricted chat (since %s)", ts)
	return true
}

// MarkChatNotRestricted removes the restricted flag for a chat.
// Called when the bot's permissions are upgraded (e.g., admin cache load detects bot is admin).
func MarkChatNotRestricted(chatID int64) {
	if Marshal == nil {
		return
	}
	if err := Marshal.Delete(Context, restrictedChatKey(chatID)); err != nil {
		log.WithField("chat_id", chatID).Debugf("[RestrictedCache] Failed to clear restricted flag: %v", err)
	} else {
		log.WithField("chat_id", chatID).Info("[RestrictedCache] Cleared restricted flag — bot can now send")
	}
}

// GetRestrictedCacheStats returns cumulative hit/miss counters for monitoring.
func GetRestrictedCacheStats() (hits, misses int64) {
	return restrictedCacheHits.Load(), restrictedCacheMisses.Load()
}
