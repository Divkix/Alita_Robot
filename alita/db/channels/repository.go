package channels

import (
	"errors"
	"strings"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// GetChannelSettings retrieves channel settings from cache or database.
// Returns nil if the channel is not found or an error occurs.
func GetChannelSettings(channelId int64) (channelSrc *models.ChannelSettings) {
	// Use optimized cached query instead of SELECT *
	channelSrc, err := GetChannelSettingsCached(channelId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		log.Errorf("[Database] GetChannelSettings: %v - %d", err, channelId)
		return nil
	}
	return channelSrc
}

// UpdateChannel updates or creates a channel record with full metadata.
// Stores channel name and username, and invalidates cache after updates.
// Returns error if database operation fails.
func UpdateChannel(channelId int64, channelName, username string) error {
	username = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(username), "@"))
	now := time.Now()
	updates := map[string]any{
		"channel_id": channelId,
		"username":   username,
		"updated_at": now,
	}
	if channelName != "" {
		updates["channel_name"] = channelName
	}

	var reassignedChatIDs []int64
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		reassignedChatIDs = nil
		err = db.DB.Transaction(func(tx *gorm.DB) error {
			if username != "" {
				if err := tx.Model(&models.ChannelSettings{}).
					Where("chat_id <> ? AND LOWER(username) = ?", channelId, username).
					Pluck("chat_id", &reassignedChatIDs).Error; err != nil {
					return err
				}
				if len(reassignedChatIDs) > 0 {
					if err := tx.Model(&models.ChannelSettings{}).
						Where("chat_id IN ?", reassignedChatIDs).
						Updates(map[string]any{"username": "", "updated_at": now}).Error; err != nil {
						return err
					}
				}
			}

			channelSrc := &models.ChannelSettings{ChatId: channelId}
			return tx.Where("chat_id = ?", channelId).Assign(updates).FirstOrCreate(channelSrc).Error
		})
		if err == nil || username == "" {
			break
		}
		// A concurrent first observation can win the unique username insert
		// after our ownership lookup. Retrying clears that owner transactionally.
	}
	if err != nil {
		log.Errorf("[Database] UpdateChannel: failed to store %d (%s): %v", channelId, username, err)
		return err
	}

	cache.DeleteCache(cache.CacheKey("channel", channelId))
	for _, reassignedChatID := range reassignedChatIDs {
		cache.DeleteCache(cache.CacheKey("channel", reassignedChatID))
	}
	log.Debugf("[Database] UpdateChannel: stored channel %d", channelId)
	return nil
}

// GetChannelIdByUserName finds a channel ID by username.
// Returns 0 if the channel is not found or an error occurs.
func GetChannelIdByUserName(username string) int64 {
	username = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(username), "@"))
	if username == "" {
		return 0
	}

	var chatId int64
	err := db.DB.Model(&models.ChannelSettings{}).
		Select("chat_id").
		Where("LOWER(username) = ?", username).
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
	err := db.DB.Model(&models.ChannelSettings{}).Count(&count).Error
	if err != nil {
		log.Errorf("[Database] loadChannelStats: %v", err)
		return 0
	}
	return
}
