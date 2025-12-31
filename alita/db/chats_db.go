package db

import (
	"errors"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/divkix/Alita_Robot/alita/utils/string_handling"
)

// GetChatSettings retrieves chat settings using optimized cached queries.
// Returns an empty Chat struct if not found or on error.
func GetChatSettings(chatId int64) (chatSrc *Chat) {
	// Use optimized cached query instead of SELECT *
	chat, err := GetOptimizedQueries().GetChatBasicInfoCached(chatId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &Chat{}
		}
		log.Errorf("[Database] GetChatSettings: %v - %d", err, chatId)
		return &Chat{}
	}
	return chat
}

// EnsureChatInDb ensures that a chat exists in the database.
// Creates the chat record if it doesn't exist, or updates it if it does.
// This is essential for foreign key constraints that reference the chats table.
func EnsureChatInDb(chatId int64, chatName string) error {
	chatUpdate := &Chat{
		ChatId:   chatId,
		ChatName: chatName,
	}
	err := DB.Where("chat_id = ?", chatId).Assign(chatUpdate).FirstOrCreate(&Chat{}).Error
	if err != nil {
		log.Errorf("[Database] EnsureChatInDb: %v", err)
		return fmt.Errorf("failed to ensure chat %d in database: %w", chatId, err)
	}
	return nil
}

// UpdateChat updates or creates a chat record with the given information.
// Adds user to the chat's user list if not already present, marks chat as active,
// and updates the last activity timestamp to track when messages are received.
// Returns error if database operation fails.
func UpdateChat(chatId int64, chatname string, userid int64) error {
	chatr := GetChatSettings(chatId)
	foundUser := string_handling.FindInInt64Slice(chatr.Users, userid)
	now := time.Now()

	// Always update last_activity to track message activity
	if chatr.ChatName == chatname && foundUser {
		// Only update last_activity and is_inactive
		err := DB.Model(&Chat{}).Where("chat_id = ?", chatId).Updates(map[string]any{
			"last_activity": now,
			"is_inactive":   false,
		}).Error
		if err != nil {
			log.Errorf("[Database] UpdateChat (activity only): %d - %v", chatId, err)
			return err
		}
		// Invalidate cache after update
		deleteCache(chatCacheKey(chatId))
		return nil
	}

	// Prepare updates for all fields
	updates := make(map[string]any)
	if chatr.ChatName != chatname {
		updates["chat_name"] = chatname
	}
	if !foundUser {
		newUsers := chatr.Users
		newUsers = append(newUsers, userid)
		updates["users"] = newUsers
	}
	updates["is_inactive"] = false
	updates["last_activity"] = now

	if chatr.ChatId == 0 {
		// Create new chat
		newChat := &Chat{
			ChatId:       chatId,
			ChatName:     chatname,
			Users:        Int64Array{userid},
			IsInactive:   false,
			LastActivity: now,
		}
		err := DB.Create(newChat).Error
		if err != nil {
			log.Errorf("[Database] UpdateChat: %v - %d (%d)", err, chatId, userid)
			return err
		}
	} else if len(updates) > 0 {
		// Update existing chat with all changes
		err := DB.Model(&Chat{}).Where("chat_id = ?", chatId).Updates(updates).Error
		if err != nil {
			log.Errorf("[Database] UpdateChat: %v - %d (%d)", err, chatId, userid)
			return err
		}
	}

	// Invalidate cache after update
	deleteCache(chatCacheKey(chatId))
	log.Debugf("[Database] UpdateChat: %d", chatId)
	return nil
}

// GetAllChats retrieves all chat records and returns them as a map indexed by chat ID.
// Returns an empty map if an error occurs.
func GetAllChats() map[int64]Chat {
	var (
		chatArray []Chat
		chatMap   = make(map[int64]Chat)
	)
	err := DB.Find(&chatArray).Error
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
	err := DB.Model(&Chat{}).Where("is_inactive = ?", false).Count(&activeCount).Error
	if err != nil {
		log.Errorf("[Database][LoadChatStats] counting active chats: %v", err)
	}

	// Count inactive chats
	err = DB.Model(&Chat{}).Where("is_inactive = ?", true).Count(&inactiveCount).Error
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
	err := DB.Model(&Chat{}).
		Where("is_inactive = ? AND last_activity >= ?", false, dayAgo).
		Count(&dag).Error
	if err != nil {
		log.Errorf("[Database][LoadActivityStats] counting daily active groups: %v", err)
	}

	// Count weekly active groups
	err = DB.Model(&Chat{}).
		Where("is_inactive = ? AND last_activity >= ?", false, weekAgo).
		Count(&wag).Error
	if err != nil {
		log.Errorf("[Database][LoadActivityStats] counting weekly active groups: %v", err)
	}

	// Count monthly active groups
	err = DB.Model(&Chat{}).
		Where("is_inactive = ? AND last_activity >= ?", false, monthAgo).
		Count(&mag).Error
	if err != nil {
		log.Errorf("[Database][LoadActivityStats] counting monthly active groups: %v", err)
	}

	return dag, wag, mag
}
