package chats

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

// ChatCacheEntry is an explicit cache payload that avoids magic sentinel values.
type ChatCacheEntry struct {
	Found bool
	Chat  *models.Chat
}

// GetChatBasicInfo retrieves only essential chat information with minimal column selection.
// Optimized for high-frequency calls by selecting only necessary fields.
func GetChatBasicInfo(chatID int64) (*models.Chat, error) {
	if db.DB == nil {
		return nil, errors.New("database not initialized")
	}

	var chat models.Chat
	collection := db.DB.Collection("chats")
	err := collection.FindOne(context.Background(), bson.M{"chat_id": chatID}, options.FindOne().SetProjection(bson.M{
		"chat_id":       1,
		"chat_name":     1,
		"language":      1,
		"users":         1,
		"is_inactive":   1,
		"last_activity": 1,
	})).Decode(&chat)

	if err != nil && err != db.ErrRecordNotFound {
		log.Errorf("[chats.GetChatBasicInfo] GetChatBasicInfo: %v", err)
	}

	return &chat, err
}

// GetChatBasicInfoCached retrieves chat information with caching layer for improved performance.
// Uses cache.CacheTTLChatSettings TTL and falls back to direct query if cache fails.
func GetChatBasicInfoCached(chatID int64) (*models.Chat, error) {
	cacheKey := cache.CacheKey("chat", chatID)

	cached, err := cache.GetFromCacheOrLoad(cacheKey, cache.CacheTTLChatSettings, func() (ChatCacheEntry, error) {
		chat, err := GetChatBasicInfo(chatID)
		if errors.Is(err, db.ErrRecordNotFound) {
			return ChatCacheEntry{Found: false, Chat: nil}, nil
		}
		if err != nil {
			return ChatCacheEntry{}, err
		}
		return ChatCacheEntry{Found: true, Chat: chat}, nil
	})
	if err != nil {
		// Cache/serialization error: fall back to direct DB query.
		chat, dbErr := GetChatBasicInfo(chatID)
		if dbErr == nil {
			return chat, nil
		}
		if errors.Is(dbErr, db.ErrRecordNotFound) {
			return nil, db.ErrRecordNotFound
		}
		return nil, dbErr
	}

	if !cached.Found {
		return nil, db.ErrRecordNotFound
	}

	return cached.Chat, nil
}
