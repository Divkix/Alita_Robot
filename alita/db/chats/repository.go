package chats

import (
	"errors"
	"fmt"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// GetChatSettings retrieves chat settings using optimized cached queries.
// Returns an empty Chat struct if not found or on error.
func GetChatSettings(chatId int64) (chatSrc *models.Chat) {
	// Use optimized cached query instead of SELECT *
	chat, err := GetChatBasicInfoCached(chatId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &models.Chat{}
		}
		log.Errorf("[Database] GetChatSettings: %v - %d", err, chatId)
		return &models.Chat{}
	}
	return chat
}

// EnsureChatInDb ensures that a chat exists in the database.
// Creates the chat record if it doesn't exist, or updates it if it does.
// This is essential for foreign key constraints that reference the chats table.
func EnsureChatInDb(chatId int64, chatName string) error {
	chatUpdate := &models.Chat{
		ChatId:   chatId,
		ChatName: chatName,
	}
	err := db.DB.Where("chat_id = ?", chatId).Assign(chatUpdate).FirstOrCreate(&models.Chat{}).Error
	if err != nil {
		log.Errorf("[Database] EnsureChatInDb: %v", err)
		return fmt.Errorf("failed to ensure chat %d in database: %w", chatId, err)
	}
	return nil
}

// UpdateChat updates or creates a chat record with the given information.
// Adds user to the chat's user list atomically if not already present, marks chat as active,
// and updates the last activity timestamp to track when messages are received.
// Returns error if database operation fails.
func UpdateChat(chatId int64, chatname string, userid int64) error {
	chatr := GetChatSettings(chatId)
	now := time.Now()

	if chatr.ChatId == 0 {
		// Create new chat record with the user already in the array
		newChat := &models.Chat{
			ChatId:       chatId,
			ChatName:     chatname,
			Users:        models.Int64Array{userid},
			IsInactive:   false,
			LastActivity: now,
		}
		err := db.DB.Create(newChat).Error
		if err != nil {
			log.Errorf("[Database] UpdateChat: %v - %d (%d)", err, chatId, userid)
			return err
		}
		cache.DeleteCache(cache.CacheKey("chat", chatId))
		return nil
	}

	// Update scalar fields (chat_name, is_inactive, last_activity)
	updates := map[string]any{
		"is_inactive":   false,
		"last_activity": now,
	}
	if chatr.ChatName != chatname {
		updates["chat_name"] = chatname
	}
	if err := db.DB.Model(&models.Chat{}).Where("chat_id = ?", chatId).Updates(updates).Error; err != nil {
		log.Errorf("[Database] UpdateChat (scalar): %d - %v", chatId, err)
		return err
	}

	// Atomically append userid only if not already present in the JSON array
	result := db.DB.Exec(
		`UPDATE chats SET users = users || to_jsonb(?::bigint) WHERE chat_id = ? AND NOT (users @> to_jsonb(?::bigint))`,
		userid, chatId, userid,
	)
	if result.Error != nil {
		log.Errorf("[Database] UpdateChat atomic append failed for chat %d user %d: %v", chatId, userid, result.Error)
	}

	// Invalidate cache after update
	cache.DeleteCache(cache.CacheKey("chat", chatId))
	log.Debugf("[Database] UpdateChat: %d", chatId)
	return nil
}

// GetAllChats retrieves all chat records and returns them as a map indexed by chat ID.
// Returns an empty map if an error occurs.
func GetAllChats() map[int64]models.Chat {
	var (
		chatArray []models.Chat
		chatMap   = make(map[int64]models.Chat)
	)
	err := db.DB.Find(&chatArray).Error
	if err != nil {
		log.Errorf("[Database] GetAllChats: %v", err)
		return chatMap
	}

	for _, i := range chatArray {
		chatMap[i.ChatId] = i
	}

	return chatMap
}

// LoadChatStats returns the count of active and inactive chats.
// Active chats have is_inactive = false, inactive chats have is_inactive = true.
func LoadChatStats() (activeChats, inactiveChats int) {
	var activeCount, inactiveCount int64

	// Count active chats
	err := db.DB.Model(&models.Chat{}).Where("is_inactive = ?", false).Count(&activeCount).Error
	if err != nil {
		log.Errorf("[Database][LoadChatStats] counting active chats: %v", err)
	}

	// Count inactive chats
	err = db.DB.Model(&models.Chat{}).Where("is_inactive = ?", true).Count(&inactiveCount).Error
	if err != nil {
		log.Errorf("[Database][LoadChatStats] counting inactive chats: %v", err)
	}

	activeChats = int(activeCount)
	inactiveChats = int(inactiveCount)
	return
}

// LoadActivityStats returns Daily Active Groups, Weekly Active Groups, and Monthly Active Groups.
// These metrics are based on last_activity timestamps within the respective time periods.
func LoadActivityStats() (dag, wag, mag int64) {
	now := time.Now()
	dayAgo := now.Add(-24 * time.Hour)
	weekAgo := now.Add(-7 * 24 * time.Hour)
	monthAgo := now.Add(-30 * 24 * time.Hour)

	// Count daily active groups
	err := db.DB.Model(&models.Chat{}).
		Where("is_inactive = ? AND last_activity >= ?", false, dayAgo).
		Count(&dag).Error
	if err != nil {
		log.Errorf("[Database][LoadActivityStats] counting daily active groups: %v", err)
	}

	// Count weekly active groups
	err = db.DB.Model(&models.Chat{}).
		Where("is_inactive = ? AND last_activity >= ?", false, weekAgo).
		Count(&wag).Error
	if err != nil {
		log.Errorf("[Database][LoadActivityStats] counting weekly active groups: %v", err)
	}

	// Count monthly active groups
	err = db.DB.Model(&models.Chat{}).
		Where("is_inactive = ? AND last_activity >= ?", false, monthAgo).
		Count(&mag).Error
	if err != nil {
		log.Errorf("[Database][LoadActivityStats] counting monthly active groups: %v", err)
	}

	return dag, wag, mag
}
