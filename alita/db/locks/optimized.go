package locks

import (
	"errors"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// GetLockStatus retrieves only the lock status for a specific lock type.
// Optimized for high-frequency lock status checks by selecting only the locked column.
// Returns false by default if no record is found.
func GetLockStatus(chatID int64, lockType string) (bool, error) {
	if db.DB == nil {
		return false, errors.New("database not initialized")
	}

	var locked bool
	err := db.DB.Model(&models.LockSettings{}).
		Select("locked").
		Where("chat_id = ? AND lock_type = ?", chatID, lockType).
		Scan(&locked).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil // Default to unlocked
	}
	if err != nil {
		log.Errorf("[OptimizedLockQueries] GetLockStatus: %v", err)
		return false, err
	}

	return locked, nil
}

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

// GetLockStatusCached retrieves lock status with caching layer for improved performance.
// Uses 1-hour cache TTL and falls back to direct query if cache fails.
func GetLockStatusCached(chatID int64, lockType string) (bool, error) {
	cacheKey := cache.CacheKey("lock", chatID, lockType)

	cached, err := cache.GetFromCacheOrLoad(cacheKey, 1*time.Hour, func() (bool, error) {
		return GetLockStatus(chatID, lockType)
	})
	if err != nil {
		// Fallback to direct query on cache error
		return GetLockStatus(chatID, lockType)
	}

	return cached, nil
}
