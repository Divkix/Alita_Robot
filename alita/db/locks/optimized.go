package locks

import (
	"errors"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
	log "github.com/sirupsen/logrus"
)

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
