package channels

import (
	"errors"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// getChannelSettingsRaw retrieves channel settings with all relevant columns.
// Returns channel settings for the specified chat or nil if not found.
func getChannelSettingsRaw(chatID int64) (*models.ChannelSettings, error) {
	if db.DB == nil {
		return nil, errors.New("database not initialized")
	}

	var settings models.ChannelSettings
	err := db.DB.Model(&models.ChannelSettings{}).
		Select("id, chat_id, channel_id, channel_name, username").
		Where("chat_id = ?", chatID).
		First(&settings).Error

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Errorf("[OptimizedChannelQueries] getChannelSettingsRaw: %v", err)
	}

	return &settings, err
}

// GetChannelSettingsCached retrieves channel settings with caching layer for improved performance.
// Uses 30-minute cache TTL and falls back to direct query if cache fails.
func GetChannelSettingsCached(chatID int64) (*models.ChannelSettings, error) {
	cacheKey := cache.CacheKey("channel", chatID)

	cached, err := cache.GetFromCacheOrLoad(cacheKey, 30*time.Minute, func() (*models.ChannelSettings, error) {
		return getChannelSettingsRaw(chatID)
	})
	if err != nil {
		return getChannelSettingsRaw(chatID)
	}

	return cached, nil
}
