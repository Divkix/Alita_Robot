package antiraid

import (
	"context"
	"errors"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// getAntiRaidSettingsRaw retrieves anti-raid settings with minimal column selection.
func getAntiRaidSettingsRaw(chatID int64) (*models.AntiRaidSettings, error) {
	if db.DB == nil {
		return defaultAntiRaidSettings(chatID), errors.New("database not initialized")
	}

	var settings models.AntiRaidSettings
	collection := db.DB.Collection("antiraid_settings")
	err := collection.FindOne(context.Background(), bson.M{"chat_id": chatID}, options.FindOne().SetProjection(bson.M{
		"chat_id":                 1,
		"raid_time":               1,
		"raid_action_time":        1,
		"auto_antiraid_threshold": 1,
	})).Decode(&settings)

	if err == db.ErrRecordNotFound {
		return defaultAntiRaidSettings(chatID), nil
	}
	if err != nil {
		log.Errorf("[OptimizedAntiRaidQueries] getAntiRaidSettingsRaw: %v", err)
		return nil, err
	}

	return &settings, nil
}

// GetAntiRaidSettingsCached retrieves anti-raid settings with caching layer for improved performance.
// Uses 30-minute cache TTL and falls back to direct query if cache fails.
func GetAntiRaidSettingsCached(chatID int64) (*models.AntiRaidSettings, error) {
	cacheKey := cache.CacheKey("antiraid", chatID)

	cached, err := cache.GetFromCacheOrLoad(cacheKey, cache.CacheTTLAntiRaid, func() (*models.AntiRaidSettings, error) {
		return getAntiRaidSettingsRaw(chatID)
	})
	if err != nil {
		return getAntiRaidSettingsRaw(chatID)
	}

	return cached, nil
}
