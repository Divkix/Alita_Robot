package chats

import (
	"context"
	"errors"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

type chatCacheEntry[T any] struct {
	Found bool
	Value T
}

func GetChatBasicInfo(chatID int64) (*models.Chat, error) {
	return GetChatBasicInfoContext(context.Background(), chatID)
}

func GetChatBasicInfoContext(ctx context.Context, chatID int64) (*models.Chat, error) {
	if db.DB == nil {
		return nil, errors.New("database not initialized")
	}

	var chat models.Chat
	err := db.DB.WithContext(ctx).Model(&models.Chat{}).
		Select("id, chat_id, chat_name, language, is_inactive, last_activity").
		Where("chat_id = ?", chatID).
		First(&chat).Error

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Errorf("[chats.GetChatBasicInfo] GetChatBasicInfo: %v", err)
	}

	return &chat, err
}

func GetChatBasicInfoCached(chatID int64) (*models.Chat, error) {
	return GetChatBasicInfoCachedContext(context.Background(), chatID)
}

func GetChatBasicInfoCachedContext(ctx context.Context, chatID int64) (*models.Chat, error) {
	return loadChatCache(ctx, cache.CacheKey("chat", chatID), chatID, GetChatBasicInfoContext)
}

func loadChatCache[T any](ctx context.Context, key string, chatID int64, loader func(context.Context, int64) (T, error)) (T, error) {
	var zero T
	cached, err := cache.GetFromCacheOrLoad(ctx, key, cache.CacheTTLChatSettings, func(ctx context.Context) (chatCacheEntry[T], error) {
		value, err := loader(ctx, chatID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return chatCacheEntry[T]{}, nil
		}
		if err != nil {
			return chatCacheEntry[T]{}, err
		}
		return chatCacheEntry[T]{Found: true, Value: value}, nil
	})
	if err != nil {
		return zero, err
	}
	if !cached.Found {
		return zero, gorm.ErrRecordNotFound
	}
	return cached.Value, nil
}

func GetChatUsers(chatID int64) (models.Int64Array, error) {
	return GetChatUsersContext(context.Background(), chatID)
}

func GetChatUsersContext(ctx context.Context, chatID int64) (models.Int64Array, error) {
	if db.DB == nil {
		return nil, errors.New("database not initialized")
	}

	var chat models.Chat
	err := db.DB.WithContext(ctx).Model(&models.Chat{}).
		Select("users").
		Where("chat_id = ?", chatID).
		First(&chat).Error

	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Errorf("[chats.GetChatUsers] GetChatUsers: %v", err)
		}
		return nil, err
	}

	return chat.Users, nil
}

func GetChatUsersCached(chatID int64) (models.Int64Array, error) {
	return GetChatUsersCachedContext(context.Background(), chatID)
}

func GetChatUsersCachedContext(ctx context.Context, chatID int64) (models.Int64Array, error) {
	return loadChatCache(ctx, cache.CacheKey("chat_users", chatID), chatID, GetChatUsersContext)
}

// ListChatsWithUser returns active chats whose tracked users include userID,
// most recently active first. It relies on Postgres jsonb containment, served
// by idx_chats_users_gin (jsonb_path_ops). That index was dropped once as
// unused and restored by 20261017120000_restore_chats_users_gin_index.sql for
// this query; without it every call scans and de-TOASTs all chats.users.
func ListChatsWithUser(ctx context.Context, userID int64, limit int) ([]models.Chat, error) {
	if db.DB == nil {
		return nil, errors.New("database not initialized")
	}
	var out []models.Chat
	err := db.DB.WithContext(ctx).Model(&models.Chat{}).
		Select("chat_id, chat_name, last_activity").
		Where("users @> to_jsonb(?::bigint) AND is_inactive = ?", userID, false).
		Order("last_activity DESC").
		Limit(limit).
		Find(&out).Error
	return out, err
}
