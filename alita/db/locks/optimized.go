package locks

import (
	"errors"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
	log "github.com/sirupsen/logrus"
)

// GetChatLocksOptimized retrieves all locks for a chat with minimal column selection.
// Returns a map of lock types to their boolean status for improved performance.
func GetChatLocksOptimized(chatID int64) (map[string]bool, error) {
	if db.DB == nil {
		return nil, errors.New("database not initialized")
	}

	type LockResult struct {
		LockType string
		Locked   bool
	}

	var locks []LockResult
	err := db.DB.Model(&models.LockSettings{}).
		Select("lock_type, locked").
		Where("chat_id = ?", chatID).
		Find(&locks).Error
	if err != nil {
		log.Errorf("[OptimizedLockQueries] GetChatLocksOptimized: %v", err)
		return nil, err
	}

	result := make(map[string]bool)
	for _, lock := range locks {
		result[lock.LockType] = lock.Locked
	}

	return result, nil
}

// GetChatLocksCached retrieves all locks for a chat with a caching layer for improved performance.
// Caches the full lock map under a single key, eliminating repeated DB round-trips per message.
// Uses 1-hour cache TTL and falls back to direct query if cache fails.
func GetChatLocksCached(chatID int64) (map[string]bool, error) {
	cacheKey := cache.CacheKey("locks_map", chatID)

	cached, err := cache.GetFromCacheOrLoad(cacheKey, 1*time.Hour, func() (map[string]bool, error) {
		return GetChatLocksOptimized(chatID)
	})
	if err != nil {
		return GetChatLocksOptimized(chatID)
	}

	return cached, nil
}
