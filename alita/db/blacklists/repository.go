package blacklists

import (
	"context"
	"errors"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

// AddBlacklist stores a new word with the chat's current action. /blaction
// writes the action into every word row, so a new word copies it from the
// oldest existing row; a chat without words starts at "warn".
func AddBlacklist(chatId int64, trigger string) error {
	var actions []string
	if err := db.DB.Model(&models.BlacklistSettings{}).
		Where("chat_id = ?", chatId).
		Order("id").
		Limit(1).
		Pluck("action", &actions).Error; err != nil {
		log.Errorf("[Database] AddBlacklist: read chat action: %v - %d", err, chatId)
		return err
	}
	action := "warn"
	if len(actions) > 0 && actions[0] != "" {
		action = actions[0]
	}

	blacklist := &models.BlacklistSettings{
		ChatId: chatId,
		Word:   strings.ToLower(trigger),
		Action: action,
		Reason: models.DefaultBlacklistReason,
	}

	err := db.CreateRecord(blacklist)
	if err != nil {
		log.Errorf("[Database] AddBlacklist: %v - %d", err, chatId)
		return err
	}

	cache.DeleteCache(cache.CacheKey("blacklist", chatId))
	return nil
}

func RemoveBlacklist(chatId int64, trigger string) error {
	result := db.DB.Where("chat_id = ? AND word = ?", chatId, strings.ToLower(trigger)).Delete(&models.BlacklistSettings{})
	if result.Error != nil {
		log.Errorf("[Database] RemoveBlacklist: %v - %d", result.Error, chatId)
		return result.Error
	}

	if result.RowsAffected > 0 {
		cache.DeleteCache(cache.CacheKey("blacklist", chatId))
	}
	return nil
}

func RemoveAllBlacklist(chatId int64) error {
	err := db.DB.Where("chat_id = ?", chatId).Delete(&models.BlacklistSettings{}).Error
	if err != nil {
		log.Errorf("[Database] RemoveAllBlacklist: %v - %d", err, chatId)
		return err
	}

	cache.DeleteCache(cache.CacheKey("blacklist", chatId))
	return nil
}

// ErrNoBlacklistWords means the chat has no words to store an action on.
var ErrNoBlacklistWords = errors.New("chat has no blacklisted words")

// SetBlacklistAction writes the action into every word row of the chat. With
// no rows there is nowhere to keep it, so it returns ErrNoBlacklistWords.
func SetBlacklistAction(chatId int64, action string) error {
	result := db.DB.Model(&models.BlacklistSettings{}).Where("chat_id = ?", chatId).Update("action", strings.ToLower(action))
	if result.Error != nil {
		log.Errorf("[Database] SetBlacklistAction: %v - %d", result.Error, chatId)
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNoBlacklistWords
	}

	cache.DeleteCache(cache.CacheKey("blacklist", chatId))
	return nil
}

func GetBlacklistSettings(chatId int64) models.BlacklistSettingsSlice {
	return GetBlacklistSettingsContext(context.Background(), chatId)
}

func GetBlacklistSettingsContext(ctx context.Context, chatId int64) models.BlacklistSettingsSlice {
	cacheKey := cache.CacheKey("blacklist", chatId)
	result, err := cache.GetFromCacheOrLoad(ctx, cacheKey, cache.CacheTTLBlacklist, func(ctx context.Context) (models.BlacklistSettingsSlice, error) {
		var blacklists []*models.BlacklistSettings
		err := db.GetRecordsContext(ctx, &blacklists, models.BlacklistSettings{ChatId: chatId})
		if err != nil {
			log.Errorf("[Database] GetBlacklistSettings: %v - %d", err, chatId)
			return models.BlacklistSettingsSlice{}, err
		}
		return models.BlacklistSettingsSlice(blacklists), nil
	})
	if err != nil {
		return models.BlacklistSettingsSlice{}
	}
	return result
}

func LoadBlacklistsStats() (blacklistTriggers, blacklistChats int64) {
	err := db.DB.Model(&models.BlacklistSettings{}).Count(&blacklistTriggers).Error
	if err != nil {
		log.Errorf("[Database] LoadBlacklistsStats (triggers): %v", err)
		return 0, 0
	}

	err = db.DB.Model(&models.BlacklistSettings{}).Distinct("chat_id").Count(&blacklistChats).Error
	if err != nil {
		log.Errorf("[Database] LoadBlacklistsStats (chats): %v", err)
		return blacklistTriggers, 0
	}

	return
}
