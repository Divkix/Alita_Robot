package disabling

import (
	"context"
	"slices"
	"time"

	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

// DisableCMD disables a command in a specific chat.
func DisableCMD(chatID int64, cmd string) error {
	// Create a new disable setting
	disableSetting := &models.DisableSettings{
		ChatId:    chatID,
		Command:   cmd,
		Disabled:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := db.CreateRecord(disableSetting)
	if err != nil {
		log.Errorf("[Database][DisableCMD]: %v", err)
		return err
	}

	// Invalidate cache to ensure fresh data
	invalidateDisabledCommandsCache(chatID)
	return nil
}

// EnableCMD enables a command in a specific chat.
func EnableCMD(chatID int64, cmd string) error {
	collection := db.DB.Collection("disable")
	_, err := collection.DeleteOne(context.Background(), bson.M{"chat_id": chatID, "command": cmd})
	if err != nil {
		log.Errorf("[Database][EnableCMD]: %v", err)
		return err
	}

	// Invalidate cache to ensure fresh data
	invalidateDisabledCommandsCache(chatID)
	return nil
}

// GetChatDisabledCMDs retrieves all disabled commands for a chat.
func GetChatDisabledCMDs(chatId int64) []string {
	var disableSettings []*models.DisableSettings
	err := db.GetRecords(&disableSettings, bson.M{"chat_id": chatId, "disabled": true})
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
func GetChatDisabledCMDsCached(chatId int64) []string {
	cacheKey := cache.CacheKey("disabled_cmds", chatId)
	result, err := cache.GetFromCacheOrLoad(cacheKey, cache.CacheTTLDisabledCmds, func() ([]string, error) {
		return GetChatDisabledCMDs(chatId), nil
	})
	if err != nil {
		log.Errorf("[Cache] Failed to get disabled commands from cache for chat %d: %v", chatId, err)
		return GetChatDisabledCMDs(chatId) // Fallback to direct DB query
	}
	return result
}

// IsCommandDisabled checks if a specific command is disabled in a chat.
func IsCommandDisabled(chatId int64, cmd string) bool {
	return slices.Contains(GetChatDisabledCMDsCached(chatId), cmd)
}

// invalidateDisabledCommandsCache invalidates the disabled commands cache for a specific chat.
func invalidateDisabledCommandsCache(chatID int64) {
	cache.DeleteCache(cache.CacheKey("disabled_cmds", chatID))
}

// ToggleDel toggles the automatic deletion of disabled commands in a chat.
func ToggleDel(chatId int64, pref bool) error {
	updates := bson.M{
		"delete_commands": pref,
		"updated_at":      time.Now(),
	}

	collection := db.DB.Collection("disable_chat_settings")
	opts := options.Update().SetUpsert(true)
	_, err := collection.UpdateOne(context.Background(), bson.M{"chat_id": chatId}, bson.M{"$set": updates}, opts)

	if err != nil {
		log.Errorf("[Database] ToggleDel: %v", err)
		return err
	}
	return nil
}

// ShouldDel checks if automatic command deletion is enabled for a chat.
func ShouldDel(chatId int64) bool {
	var settings models.DisableChatSettings
	err := db.GetRecord(&settings, bson.M{"chat_id": chatId})
	if err != nil {
		if err != db.ErrRecordNotFound {
			log.Errorf("[Database] ShouldDel: %v", err)
		}
		return false
	}
	return settings.DeleteCommands
}

// LoadDisableStats returns statistics about disabled commands.
func LoadDisableStats() (disabledCmds, disableEnabledChats int64) {
	collection := db.DB.Collection("disable")

	// Count total disabled commands
	var err error
	disabledCmds, err = collection.CountDocuments(context.Background(), bson.M{"disabled": true})
	if err != nil {
		log.Errorf("[Database] LoadDisableStats (commands): %v", err)
		return 0, 0
	}

	// Count distinct chats with disabled commands
	distinctChats, err := collection.Distinct(context.Background(), "chat_id", bson.M{"disabled": true})
	if err != nil {
		log.Errorf("[Database] LoadDisableStats (chats): %v", err)
		return disabledCmds, 0
	}
	disableEnabledChats = int64(len(distinctChats))

	return
}
