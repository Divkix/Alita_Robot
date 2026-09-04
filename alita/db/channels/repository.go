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

func GetChannelSettings(channelId int64) (channelSrc *models.ChannelSettings) {
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
					Where("chat_id <> ? AND username <> '' AND LOWER(username) = ?", channelId, username).
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

func GetChannelInfoById(channelId int64) (username, name string, found bool) {
	channel := GetChannelSettings(channelId)
	if channel != nil && channel.ChatId != 0 {
		username = channel.Username
		name = channel.ChannelName
		found = true
	}
	return
}

func LoadChannelStats() (count int64) {
	err := db.DB.Model(&models.ChannelSettings{}).Count(&count).Error
	if err != nil {
		log.Errorf("[Database] loadChannelStats: %v", err)
		return 0
	}
	return
}
