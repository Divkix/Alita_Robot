package db

import (
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/utils/string_handling"
)

// DisableCMD disables a command in a specific chat.
// Creates a new disable setting record with disabled status set to true.
// Invalidates cache to ensure consistency.
// Returns an error if the database operation fails.
func DisableCMD(chatID int64, cmd string) error {
	// Create a new disable setting
	disableSetting := &DisableSettings{
		ChatId:   chatID,
		Command:  cmd,
		Disabled: true,
	}

	err := CreateRecord(disableSetting)
	if err != nil {
		log.Errorf("[Database][DisableCMD]: %v", err)
		return err
	}

	// Invalidate cache to ensure fresh data
	invalidateDisabledCommandsCache(chatID)
	return nil
}

// EnableCMD enables a command in a specific chat.
// Removes the disable setting record for the command.
// Invalidates cache to ensure consistency.
// Returns an error if the database operation fails.
func EnableCMD(chatID int64, cmd string) error {
	err := DB.Where("chat_id = ? AND command = ?", chatID, cmd).Delete(&DisableSettings{}).Error
	if err != nil {
		log.Errorf("[Database][EnableCMD]: %v", err)
		return err
	}

	// Invalidate cache to ensure fresh data
	invalidateDisabledCommandsCache(chatID)
	return nil
}

// GetChatDisabledCMDs retrieves all disabled commands for a chat.
// Returns an empty slice if no disabled commands are found or on error.
func GetChatDisabledCMDs(chatId int64) []string {
	var disableSettings []*DisableSettings
	err := GetRecords(&disableSettings, DisableSettings{ChatId: chatId, Disabled: true})
	if err != nil {
		log.Errorf("[Database] GetChatDisabledCMDs: %v - %d", err, chatId)
		return []string{}
	}

	commands := make([]string, len(disableSettings))
	for i, setting := range disableSettings {
		commands[i] = setting.Command
	}
	return commands
}

// GetChatDisabledCMDsCached retrieves all disabled commands for a chat with caching.
// Uses cache with TTL to avoid database queries on every command check.
func GetChatDisabledCMDsCached(chatId int64) []string {
	cacheKey := disabledCommandsCacheKey(chatId)
	result, err := getFromCacheOrLoad(cacheKey, CacheTTLDisabledCmds, func() ([]string, error) {
		return GetChatDisabledCMDs(chatId), nil
	})
	if err != nil {
		log.Errorf("[Cache] Failed to get disabled commands from cache for chat %d: %v", chatId, err)
		return GetChatDisabledCMDs(chatId) // Fallback to direct DB query
	}
	return result
}

// IsCommandDisabled checks if a specific command is disabled in a chat.
// Returns true if the command is in the chat's disabled commands list.
// Uses cached version for better performance.
func IsCommandDisabled(chatId int64, cmd string) bool {
	return string_handling.FindInStringSlice(GetChatDisabledCMDsCached(chatId), cmd)
}

// invalidateDisabledCommandsCache invalidates the disabled commands cache for a specific chat.
func invalidateDisabledCommandsCache(chatID int64) {
	deleteCache(disabledCommandsCacheKey(chatID))
}

// ToggleDel toggles the automatic deletion of disabled commands in a chat.
// Updates the DeleteCommands setting for the chat.
// Returns an error if the database operation fails.
func ToggleDel(chatId int64, pref bool) error {
	updates := map[string]any{
		"chat_id":         chatId,
		"delete_commands": pref,
	}
	err := DB.Where("chat_id = ?", chatId).
		Assign(updates).
		FirstOrCreate(&DisableChatSettings{}).Error
	if err != nil {
		log.Errorf("[Database] ToggleDel: %v", err)
		return err
	}
	return nil
}

// ShouldDel checks if automatic command deletion is enabled for a chat.
// Returns false if the setting is not found or on error.
func ShouldDel(chatId int64) bool {
	var settings DisableChatSettings
	err := GetRecord(&settings, DisableChatSettings{ChatId: chatId})
	if err != nil {
		log.Errorf("[Database] ShouldDel: %v", err)
		return false
	}
	return settings.DeleteCommands
}

// LoadDisableStats returns statistics about disabled commands.
// Returns the total number of disabled commands and distinct chats using command disabling.
func LoadDisableStats() (disabledCmds, disableEnabledChats int64) {
	// Count total disabled commands
	err := DB.Model(&DisableSettings{}).Where("disabled = ?", true).Count(&disabledCmds).Error
	if err != nil {
		log.Errorf("[Database] LoadDisableStats (commands): %v", err)
		return 0, 0
	}

	// Count distinct chats with disabled commands
	err = DB.Model(&DisableSettings{}).Where("disabled = ?", true).Distinct("chat_id").Count(&disableEnabledChats).Error
	if err != nil {
		log.Errorf("[Database] LoadDisableStats (chats): %v", err)
		return disabledCmds, 0
	}

	return
}
