package filters

import (
	"errors"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
	log "github.com/sirupsen/logrus"
)

// GetChatFiltersOptimized retrieves filters with minimal column selection.
// Optimized for high-frequency calls (34K+ calls) by selecting only essential filter fields.
// Includes all fields needed by filtersWatcher: keyword, filter_reply, msgtype, fileid, filter_buttons, nonotif.
func GetChatFiltersOptimized(chatID int64) ([]*models.ChatFilters, error) {
	if db.DB == nil {
		return nil, errors.New("database not initialized")
	}

	var filters []*models.ChatFilters
	err := db.DB.Model(&models.ChatFilters{}).
		Select("id, chat_id, keyword, filter_reply, msgtype, fileid, filter_buttons, nonotif").
		Where("chat_id = ?", chatID).
		Find(&filters).Error
	if err != nil {
		log.Errorf("[OptimizedFilterQueries] GetChatFiltersOptimized: %v", err)
		return nil, err
	}

	return filters, nil
}

// GetChatFiltersCached retrieves filters with caching layer for improved performance.
// Uses 15-minute cache TTL and falls back to direct query if cache fails.
func GetChatFiltersCached(chatID int64) ([]*models.ChatFilters, error) {
	cacheKey := cache.CacheKey("filters_optimized", chatID)

	cached, err := cache.GetFromCacheOrLoad(cacheKey, 15*time.Minute, func() ([]*models.ChatFilters, error) {
		return GetChatFiltersOptimized(chatID)
	})
	if err != nil {
		return GetChatFiltersOptimized(chatID)
	}

	return cached, nil
}
