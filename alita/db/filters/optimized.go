package filters

import (
	"context"
	"errors"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// GetChatFiltersOptimized retrieves filters with minimal column selection.
// Optimized for high-frequency calls (34K+ calls) by selecting only essential filter fields.
func GetChatFiltersOptimized(chatID int64) ([]*models.ChatFilters, error) {
	if db.DB == nil {
		return nil, errors.New("database not initialized")
	}

	var filters []*models.ChatFilters
	collection := db.DB.Collection("filters")
	cursor, err := collection.Find(context.Background(), bson.M{"chat_id": chatID}, options.Find().SetProjection(bson.M{
		"chat_id":        1,
		"keyword":        1,
		"filter_reply":   1,
		"msgtype":        1,
		"fileid":         1,
		"filter_buttons": 1,
		"nonotif":        1,
	}))

	if err != nil {
		log.Errorf("[OptimizedFilterQueries] GetChatFiltersOptimized: %v", err)
		return nil, err
	}
	defer cursor.Close(context.Background())

	if err := cursor.All(context.Background(), &filters); err != nil {
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
