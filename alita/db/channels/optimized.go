package channels

import (
	"context"
	"errors"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

func getChannelSettingsRaw(chatID int64) (*models.ChannelSettings, error) {
	return getChannelSettingsRawContext(context.Background(), chatID)
}

func getChannelSettingsRawContext(ctx context.Context, chatID int64) (*models.ChannelSettings, error) {
	if db.DB == nil {
		return nil, errors.New("database not initialized")
	}

	var settings models.ChannelSettings
	err := db.DB.WithContext(ctx).Model(&models.ChannelSettings{}).
		Select("id, chat_id, channel_id, channel_name, username").
		Where("chat_id = ?", chatID).
		First(&settings).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		log.Errorf("[OptimizedChannelQueries] getChannelSettingsRaw: %v", err)
		return nil, err
	}

	return &settings, nil
}

func GetChannelSettingsCached(chatID int64) (*models.ChannelSettings, error) {
	return GetChannelSettingsCachedContext(context.Background(), chatID)
}

func GetChannelSettingsCachedContext(ctx context.Context, chatID int64) (*models.ChannelSettings, error) {
	cacheKey := cache.CacheKey("channel", chatID)

	cached, err := cache.GetFromCacheOrLoad(ctx, cacheKey, cache.CacheTTLChannels, func(ctx context.Context) (*models.ChannelSettings, error) {
		return getChannelSettingsRawContext(ctx, chatID)
	})
	if err != nil {
		return nil, err
	}

	return cached, nil
}
