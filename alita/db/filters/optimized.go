package filters

import (
	"context"
	"errors"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

func GetChatFiltersOptimized(chatID int64) ([]*models.ChatFilters, error) {
	return GetChatFiltersOptimizedContext(context.Background(), chatID)
}

func GetChatFiltersOptimizedContext(ctx context.Context, chatID int64) ([]*models.ChatFilters, error) {
	if db.DB == nil {
		return nil, errors.New("database not initialized")
	}

	var filters []*models.ChatFilters
	err := db.DB.WithContext(ctx).Model(&models.ChatFilters{}).
		Select("id, chat_id, keyword, filter_reply, msgtype, fileid, filter_buttons, nonotif").
		Where("chat_id = ?", chatID).
		Find(&filters).Error
	if err != nil {
		log.Errorf("[OptimizedFilterQueries] GetChatFiltersOptimized: %v", err)
		return nil, err
	}

	return filters, nil
}

func GetChatFiltersCached(chatID int64) ([]*models.ChatFilters, error) {
	return GetChatFiltersCachedContext(context.Background(), chatID)
}

func GetChatFiltersCachedContext(ctx context.Context, chatID int64) ([]*models.ChatFilters, error) {
	cacheKey := cache.CacheKey("filters_optimized", chatID)

	cached, err := cache.GetFromCacheOrLoad(ctx, cacheKey, 15*time.Minute, func(ctx context.Context) ([]*models.ChatFilters, error) {
		return GetChatFiltersOptimizedContext(ctx, chatID)
	})
	if err != nil {
		return nil, err
	}

	return cached, nil
}
