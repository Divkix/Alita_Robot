package antiraid

import (
	"context"
	"errors"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

func getAntiRaidSettingsRaw(chatID int64) (*models.AntiRaidSettings, error) {
	return getAntiRaidSettingsRawContext(context.Background(), chatID)
}

func getAntiRaidSettingsRawContext(ctx context.Context, chatID int64) (*models.AntiRaidSettings, error) {
	if db.DB == nil {
		return defaultAntiRaidSettings(chatID), errors.New("database not initialized")
	}

	var settings models.AntiRaidSettings
	err := db.DB.WithContext(ctx).Model(&models.AntiRaidSettings{}).
		Select("id, chat_id, raid_time, raid_action_time, auto_antiraid_threshold").
		Where("chat_id = ?", chatID).
		First(&settings).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return defaultAntiRaidSettings(chatID), nil
	}
	if err != nil {
		log.Errorf("[OptimizedAntiRaidQueries] getAntiRaidSettingsRaw: %v", err)
		return nil, err
	}

	return &settings, nil
}

func GetAntiRaidSettingsCached(chatID int64) (*models.AntiRaidSettings, error) {
	return GetAntiRaidSettingsCachedContext(context.Background(), chatID)
}

func GetAntiRaidSettingsCachedContext(ctx context.Context, chatID int64) (*models.AntiRaidSettings, error) {
	cacheKey := cache.CacheKey("antiraid", chatID)

	cached, err := cache.GetFromCacheOrLoad(ctx, cacheKey, cache.CacheTTLAntiRaid, func(ctx context.Context) (*models.AntiRaidSettings, error) {
		return getAntiRaidSettingsRawContext(ctx, chatID)
	})
	if err != nil {
		return nil, err
	}

	return cached, nil
}
