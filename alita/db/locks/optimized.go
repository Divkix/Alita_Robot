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
	cacheKey := LocksProjection.Key(chatID)

	cached, err := cache.GetFromCacheOrLoad(ctx, cacheKey, 1*time.Hour, func(ctx context.Context) (map[string]bool, error) {
		return GetChatLocksOptimizedContext(ctx, chatID)
	})
	if err != nil {
		return nil, err
	}

	return cached, nil
}

func GetChatAllowlistOptimizedContext(ctx context.Context, chatID int64) ([]models.LockAllowlist, error) {
	if db.DB == nil {
		return nil, errors.New("database not initialized")
	}

	var exceptions []models.LockAllowlist
	err := db.DB.WithContext(ctx).Where("chat_id = ?", chatID).
		Order("kind, value").Find(&exceptions).Error
	if err != nil {
		log.Errorf("[OptimizedLockQueries] GetChatAllowlistOptimized: %v", err)
		return nil, err
	}
	return exceptions, nil
}

func GetChatAllowlistCachedContext(ctx context.Context, chatID int64) ([]models.LockAllowlist, error) {
	cacheKey := LockAllowlistProjection.Key(chatID)

	return cache.GetFromCacheOrLoad(ctx, cacheKey, 1*time.Hour, func(ctx context.Context) ([]models.LockAllowlist, error) {
		return GetChatAllowlistOptimizedContext(ctx, chatID)
	})
}
