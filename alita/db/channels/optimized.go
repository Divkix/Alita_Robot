package channels

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

// getChannelSettingsRaw retrieves channel settings with all relevant columns.
// Returns channel settings for the specified chat or nil if not found.
func getChannelSettingsRaw(chatID int64) (*models.ChannelSettings, error) {
	if db.DB == nil {
		return nil, errors.New("database not initialized")
	}

	var settings models.ChannelSettings
	collection := db.DB.Collection("channels")
	err := collection.FindOne(context.Background(), bson.M{"chat_id": chatID}, options.FindOne().SetProjection(bson.M{
		"chat_id":      1,
		"channel_id":   1,
		"channel_name": 1,
		"username":     1,
	})).Decode(&settings)

	if err != nil {
		if err == db.ErrRecordNotFound {
			return nil, nil
		}
		log.Errorf("[OptimizedChannelQueries] getChannelSettingsRaw: %v", err)
		return nil, err
	}

	return &settings, nil
}

// GetChannelSettingsCached retrieves channel settings with caching layer for improved performance.
// Uses 30-minute cache TTL and falls back to direct query if cache fails.
func GetChannelSettingsCached(chatID int64) (*models.ChannelSettings, error) {
	cacheKey := cache.CacheKey("channel", chatID)

	cached, err := cache.GetFromCacheOrLoad(cacheKey, cache.CacheTTLChannels, func() (*models.ChannelSettings, error) {
		return getChannelSettingsRaw(chatID)
	})
	if err != nil {
		return getChannelSettingsRaw(chatID)
	}

	return cached, nil
}
