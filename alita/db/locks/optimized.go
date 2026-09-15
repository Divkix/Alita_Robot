package locks

import (
	"context"
	"errors"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

func GetChatLocksOptimized(chatID int64) (map[string]bool, error) {
	return GetChatLocksOptimizedContext(context.Background(), chatID)
}

func GetChatLocksOptimizedContext(ctx context.Context, chatID int64) (map[string]bool, error) {
	if db.DB == nil {
		return nil, errors.New("database not initialized")
	}

	type LockResult struct {
		LockType string
		Locked   bool
	}

	var locks []LockResult
	err := db.DB.WithContext(ctx).Model(&models.LockSettings{}).
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
	return GetChatLocksCachedContext(context.Background(), chatID)
}

func GetChatLocksCachedContext(ctx context.Context, chatID int64) (map[string]bool, error) {
	cacheKey := cache.CacheKey("locks_map", chatID)

	cached, err := cache.GetFromCacheOrLoad(ctx, cacheKey, 1*time.Hour, func(ctx context.Context) (map[string]bool, error) {
		return GetChatLocksOptimizedContext(ctx, chatID)
	})
	if err != nil {
		return nil, err
	}

	return cached, nil
}
