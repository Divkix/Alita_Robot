package db

import (
	"strings"

	log "github.com/sirupsen/logrus"
)

// AddBlacklist adds a new blacklist word to a chat with default 'warn' action.
// The trigger is converted to lowercase before storage.
func AddBlacklist(chatId int64, trigger string) {
	// Create a new blacklist entry
	blacklist := &BlacklistSettings{
		ChatId: chatId,
		Word:   strings.ToLower(trigger),
		Action: "warn",                   // default action (intentionally 'warn' for safety)
		Reason: "Blacklisted word: '%s'", // default format string with placeholder for trigger word
	}

	err := CreateRecord(blacklist)
	if err != nil {
		log.Errorf("[Database] AddBlacklist: %v - %d", err, chatId)
	}

	// Invalidate cache after adding blacklist
	deleteCache(blacklistCacheKey(chatId))
}

// RemoveBlacklist removes a specific blacklist word from a chat.
// The trigger is converted to lowercase before removal.
func RemoveBlacklist(chatId int64, trigger string) {
	result := DB.Where("chat_id = ? AND word = ?", chatId, strings.ToLower(trigger)).Delete(&BlacklistSettings{})
	if result.Error != nil {
		log.Errorf("[Database] RemoveBlacklist: %v - %d", result.Error, chatId)
	}

	// Invalidate cache if something was deleted
	if result.RowsAffected > 0 {
		deleteCache(blacklistCacheKey(chatId))
	}
}

// RemoveAllBlacklist removes all blacklist entries for a specific chat.
func RemoveAllBlacklist(chatId int64) {
	err := DB.Where("chat_id = ?", chatId).Delete(&BlacklistSettings{}).Error
	if err != nil {
		log.Errorf("[Database] RemoveAllBlacklist: %v - %d", err, chatId)
	}

	// Invalidate cache after removing all blacklist entries
	deleteCache(blacklistCacheKey(chatId))
}

// SetBlacklistAction updates the action for all blacklist entries in a chat.
// The action is converted to lowercase before storage.
func SetBlacklistAction(chatId int64, action string) {
	err := DB.Model(&BlacklistSettings{}).Where("chat_id = ?", chatId).Update("action", strings.ToLower(action)).Error
	if err != nil {
		log.Errorf("[Database] SetBlacklistAction: %v - %d", err, chatId)
	}

	// Invalidate cache after updating action
	deleteCache(blacklistCacheKey(chatId))
}

// GetBlacklistSettings retrieves all blacklist settings for a chat with caching support.
// Returns an empty slice if no blacklists are found or on error.
func GetBlacklistSettings(chatId int64) BlacklistSettingsSlice {
	// Try to get from cache first
	cacheKey := blacklistCacheKey(chatId)
	result, err := getFromCacheOrLoad(cacheKey, CacheTTLBlacklist, func() (BlacklistSettingsSlice, error) {
		var blacklists []*BlacklistSettings
		err := GetRecords(&blacklists, BlacklistSettings{ChatId: chatId})
		if err != nil {
			log.Errorf("[Database] GetBlacklistSettings: %v - %d", err, chatId)
			return BlacklistSettingsSlice{}, err
		}
		return BlacklistSettingsSlice(blacklists), nil
	})
	if err != nil {
		return BlacklistSettingsSlice{}
	}
	return result
}

// LoadBlacklistsStats returns statistics about blacklist usage.
// Returns the total number of blacklist entries and distinct chats using blacklists.
func LoadBlacklistsStats() (blacklistTriggers, blacklistChats int64) {
	// Count total blacklist entries
	err := DB.Model(&BlacklistSettings{}).Count(&blacklistTriggers).Error
	if err != nil {
		log.Errorf("[Database] LoadBlacklistsStats (triggers): %v", err)
		return 0, 0
	}

	// Count distinct chats with blacklists
	err = DB.Model(&BlacklistSettings{}).Distinct("chat_id").Count(&blacklistChats).Error
	if err != nil {
		log.Errorf("[Database] LoadBlacklistsStats (chats): %v", err)
		return blacklistTriggers, 0
	}

	return
}
