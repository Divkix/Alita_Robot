package blacklists

import (
	"strings"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
	log "github.com/sirupsen/logrus"
)

func AddBlacklist(chatId int64, trigger string) error {
	blacklist := &models.BlacklistSettings{
		ChatId: chatId,
		Word:   strings.ToLower(trigger),
		Action: "warn",
		Reason: "Blacklisted word: '%s'",
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

func SetBlacklistAction(chatId int64, action string) error {
	err := db.DB.Model(&models.BlacklistSettings{}).Where("chat_id = ?", chatId).Update("action", strings.ToLower(action)).Error
	if err != nil {
		log.Errorf("[Database] SetBlacklistAction: %v - %d", err, chatId)
		return err
	}

	cache.DeleteCache(cache.CacheKey("blacklist", chatId))
	return nil
}

func GetBlacklistSettings(chatId int64) models.BlacklistSettingsSlice {
	cacheKey := cache.CacheKey("blacklist", chatId)
	result, err := cache.GetFromCacheOrLoad(cacheKey, cache.CacheTTLBlacklist, func() (models.BlacklistSettingsSlice, error) {
		var blacklists []*models.BlacklistSettings
		err := db.GetRecords(&blacklists, models.BlacklistSettings{ChatId: chatId})
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
