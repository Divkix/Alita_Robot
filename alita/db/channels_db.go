package db

import (
	"errors"
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// GetChannelSettings retrieves channel settings from cache or database.
// Returns nil if the channel is not found or an error occurs.
func GetChannelSettings(channelId int64) (channelSrc *ChannelSettings) {
	// Use optimized cached query instead of SELECT *
	channelSrc, err := GetOptimizedQueries().GetChannelSettingsCached(channelId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		log.Errorf("[Database] GetChannelSettings: %v - %d", err, channelId)
		return nil
	}
	return channelSrc
}

// EnsureChatExists ensures a chat record exists before creating related records.
// Creates a minimal chat record with default settings if it doesn't exist.
func EnsureChatExists(chatId int64, chatName string) error {
	if ChatExists(chatId) {
		return nil
	}

	// Create minimal chat record
	chat := &Chat{
		ChatId:     chatId,
		ChatName:   chatName,
		Language:   "en", // default language
		Users:      Int64Array{},
		IsInactive: false,
	}

	err := CreateRecord(chat)
	if err != nil {
		log.Errorf("[Database] EnsureChatExists: Failed to create chat %d: %v", chatId, err)
		return err
	}

	log.Infof("[Database] EnsureChatExists: Created chat record for %d", chatId)
	return nil
}

// UpdateChannel updates or creates a channel record with full metadata.
// Stores channel name and username, and invalidates cache after updates.
// Returns error if database operation fails.
func UpdateChannel(channelId int64, channelName, username string) error {
	// Check if channel already exists
	channelSrc := GetChannelSettings(channelId)
	now := time.Now()

	if channelSrc != nil && channelSrc.ChatId != 0 {
		// Channel exists - check if update is needed
		needsUpdate := false
		updates := make(map[string]any)

		if channelSrc.ChannelName != channelName && channelName != "" {
			updates["channel_name"] = channelName
			needsUpdate = true
		}
		if channelSrc.Username != username && username != "" {
			updates["username"] = username
			needsUpdate = true
		}

		if needsUpdate {
			updates["updated_at"] = now
			err := DB.Model(&ChannelSettings{}).Where("chat_id = ?", channelId).Updates(updates).Error
			if err != nil {
				log.Errorf("[Database] UpdateChannel: failed to update %d: %v", channelId, err)
				return err
			}
			deleteCache(channelCacheKey(channelId))
			log.Debugf("[Database] UpdateChannel: updated channel %d", channelId)
		}
		return nil
	}

	// Create new channel with full metadata
	channelSrc = &ChannelSettings{
		ChatId:      channelId,
		ChannelId:   channelId,
		ChannelName: channelName,
		Username:    username,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	err := DB.Create(channelSrc).Error
	if err != nil {
		log.Errorf("[Database] UpdateChannel: failed to create %d (%s): %v", channelId, username, err)
		return err
	}
	deleteCache(channelCacheKey(channelId))
	log.Infof("[Database] UpdateChannel: created channel %d (%s)", channelId, channelName)
	return nil
}

// GetChannelIdByUserName finds a channel ID by username.
// Returns 0 if the channel is not found or an error occurs.
func GetChannelIdByUserName(username string) int64 {
	if username == "" {
		return 0
	}

	var chatId int64
	err := DB.Model(&ChannelSettings{}).
		Select("chat_id").
		Where("username = ?", username).
		Scan(&chatId).Error

	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Errorf("[Database] GetChannelIdByUserName: %v - %s", err, username)
		}
		return 0
	}
	return chatId
}

// GetChannelInfoById retrieves channel information by channel ID.
// Returns username, name, and whether the channel was found.
func GetChannelInfoById(channelId int64) (username, name string, found bool) {
	channel := GetChannelSettings(channelId)
	if channel != nil && channel.ChatId != 0 {
		username = channel.Username
		name = channel.ChannelName
		found = true
	}
	return
}

// LoadChannelStats returns the total count of channel settings records.
func LoadChannelStats() (count int64) {
	err := DB.Model(&ChannelSettings{}).Count(&count).Error
	if err != nil {
		log.Errorf("[Database] loadChannelStats: %v", err)
		return 0
	}
	return
}
