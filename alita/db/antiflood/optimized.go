package antiflood

import (
	"context"
	"errors"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

func GetAntifloodSettings(chatID int64) (*models.AntifloodSettings, error) {
	return GetAntifloodSettingsContext(context.Background(), chatID)
}

func GetAntifloodSettingsContext(ctx context.Context, chatID int64) (*models.AntifloodSettings, error) {
	if db.DB == nil {
		return &models.AntifloodSettings{
			ChatId: chatID,
			Limit:  0,
			Action: "mute",
		}, errors.New("database not initialized")
	}

	var settings models.AntifloodSettings
	err := db.DB.WithContext(ctx).Model(&models.AntifloodSettings{}).
		Select("id, chat_id, flood_limit, action, delete_antiflood_message").
		Where("chat_id = ?", chatID).
		First(&settings).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &models.AntifloodSettings{
			ChatId: chatID,
			Limit:  0,
			Action: "mute",
		}, nil
	}
	if err != nil {
		log.Errorf("[OptimizedAntifloodQueries] GetAntifloodSettings: %v", err)
		return nil, err
	}

	return &settings, nil
}

func GetAntifloodSettingsCached(chatID int64) (*models.AntifloodSettings, error) {
	return GetAntifloodSettingsCachedContext(context.Background(), chatID)
}

func GetAntifloodSettingsCachedContext(ctx context.Context, chatID int64) (*models.AntifloodSettings, error) {
	cacheKey := cache.CacheKey("antiflood", chatID)

	return cache.GetFromCacheOrLoad(ctx, cacheKey, cache.CacheTTLAntiflood, func(ctx context.Context) (*models.AntifloodSettings, error) {
		return GetAntifloodSettingsContext(ctx, chatID)
	})
}
