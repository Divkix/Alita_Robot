package chats

import (
	"errors"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// GetChatBasicInfo retrieves only essential chat information with minimal column selection.
// Optimized for high-frequency calls by selecting only necessary fields.
func GetChatBasicInfo(chatID int64) (*models.Chat, error) {
	if db.DB == nil {
		return nil, errors.New("database not initialized")
	}

	var chat models.Chat
	err := db.DB.Model(&models.Chat{}).
		Select("id, chat_id, chat_name, language, users, is_inactive, last_activity").
		Where("chat_id = ?", chatID).
		First(&chat).Error

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Errorf("[OptimizedChatQueries] GetChatBasicInfo: %v", err)
	}

	return &chat, err
}

// GetChatBasicInfoCached retrieves chat information with caching layer for improved performance.
// Uses 30-minute cache TTL and falls back to direct query if cache fails.
func GetChatBasicInfoCached(chatID int64) (*models.Chat, error) {
	cacheKey := cache.CacheKey("chat", chatID)

	cached, err := cache.GetFromCacheOrLoad(cacheKey, 30*time.Minute, func() (*models.Chat, error) {
		return GetChatBasicInfo(chatID)
	})
	if err != nil {
		return GetChatBasicInfo(chatID)
	}

	return cached, nil
}
