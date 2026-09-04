package chats

import (
	"errors"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type ChatCacheEntry struct {
	Found bool
	Chat  *models.Chat
}

func GetChatBasicInfo(chatID int64) (*models.Chat, error) {
	if db.DB == nil {
		return nil, errors.New("database not initialized")
	}

	var chat models.Chat
	err := db.DB.Model(&models.Chat{}).
		Select("id, chat_id, chat_name, language, is_inactive, last_activity").
		Where("chat_id = ?", chatID).
		First(&chat).Error

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Errorf("[chats.GetChatBasicInfo] GetChatBasicInfo: %v", err)
	}

	return &chat, err
}

func GetChatBasicInfoCached(chatID int64) (*models.Chat, error) {
	cacheKey := cache.CacheKey("chat", chatID)

	cached, err := cache.GetFromCacheOrLoad(cacheKey, cache.CacheTTLChatSettings, func() (ChatCacheEntry, error) {
		chat, err := GetChatBasicInfo(chatID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ChatCacheEntry{Found: false, Chat: nil}, nil
		}
		if err != nil {
			return ChatCacheEntry{}, err
		}
		return ChatCacheEntry{Found: true, Chat: chat}, nil
	})
	if err != nil {
		chat, dbErr := GetChatBasicInfo(chatID)
		if dbErr == nil {
			return chat, nil
		}
		if errors.Is(dbErr, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, dbErr
	}

	if !cached.Found {
		return nil, gorm.ErrRecordNotFound
	}

	return cached.Chat, nil
}

type ChatUsersCacheEntry struct {
	Found bool
	Users models.Int64Array
}

func GetChatUsers(chatID int64) (models.Int64Array, error) {
	if db.DB == nil {
		return nil, errors.New("database not initialized")
	}

	var chat models.Chat
	err := db.DB.Model(&models.Chat{}).
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
	cacheKey := cache.CacheKey("chat_users", chatID)

	cached, err := cache.GetFromCacheOrLoad(cacheKey, cache.CacheTTLChatSettings, func() (ChatUsersCacheEntry, error) {
		users, err := GetChatUsers(chatID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ChatUsersCacheEntry{Found: false, Users: nil}, nil
		}
		if err != nil {
			return ChatUsersCacheEntry{}, err
		}
		return ChatUsersCacheEntry{Found: true, Users: users}, nil
	})
	if err != nil {
		users, dbErr := GetChatUsers(chatID)
		if dbErr == nil {
			return users, nil
		}
		if errors.Is(dbErr, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, dbErr
	}

	if !cached.Found {
		return nil, gorm.ErrRecordNotFound
	}

	return cached.Users, nil
}
