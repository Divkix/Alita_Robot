package db

import (
	"errors"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// default mode is 'mute'
const defaultFloodsettingsMode string = "mute"

// GetFlood Get flood settings for a chat
func GetFlood(chatID int64) *AntifloodSettings {
	return checkFloodSetting(chatID)
}

// checkFloodSetting retrieves or returns default antiflood settings for a chat.
// Uses optimized cached queries and returns default settings if not found.
func checkFloodSetting(chatID int64) (floodSrc *AntifloodSettings) {
	// Use optimized cached query instead of SELECT *
	floodSrc, err := GetOptimizedQueries().GetAntifloodSettingsCached(chatID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Return default settings
			return &AntifloodSettings{ChatId: chatID, Limit: 0, Action: defaultFloodsettingsMode}
		}
		log.Errorf("[Database][checkFloodSetting]: %v", err)
		return &AntifloodSettings{ChatId: chatID, Limit: 0, Action: defaultFloodsettingsMode}
	}
	return floodSrc
}

// SetFlood set Flood Setting for a Chat
func SetFlood(chatID int64, limit int) {
	floodSrc := checkFloodSetting(chatID)

	// Check if update is actually needed
	if floodSrc.Limit == limit {
		return
	}

	action := floodSrc.Action
	if action == "" {
		action = defaultFloodsettingsMode
	}

	// update the value in db
	err := UpdateRecord(&AntifloodSettings{}, AntifloodSettings{ChatId: chatID}, AntifloodSettings{Limit: limit, Action: action})
	if err != nil {
		log.Errorf("[Database] SetFlood: %v - %d", err, chatID)
	}
	// Invalidate cache after update
	deleteCache(optimizedAntifloodCacheKey(chatID))
}

// SetFloodMode Set flood mode for a chat
func SetFloodMode(chatID int64, mode string) {
	floodSrc := checkFloodSetting(chatID)
	// Check if update is actually needed
	if floodSrc.Action == mode {
		return
	}
	err := UpdateRecord(&AntifloodSettings{}, AntifloodSettings{ChatId: chatID}, AntifloodSettings{Action: mode})
	if err != nil {
		log.Errorf("[Database] SetFloodMode: %v - %d", err, chatID)
	}
	// Invalidate cache after update
	deleteCache(optimizedAntifloodCacheKey(chatID))
}

// SetFloodMsgDel Set flood message deletion setting for a chat
func SetFloodMsgDel(chatID int64, val bool) {
	floodSrc := checkFloodSetting(chatID)
	// Check if update is actually needed
	if floodSrc.DeleteAntifloodMessage == val {
		return
	}
	err := UpdateRecord(&AntifloodSettings{}, AntifloodSettings{ChatId: chatID}, AntifloodSettings{DeleteAntifloodMessage: val})
	if err != nil {
		log.Errorf("[Database] SetFloodMsgDel: %v", err)
		return
	}
	// Invalidate cache after update
	deleteCache(optimizedAntifloodCacheKey(chatID))
}

// LoadAntifloodStats returns the count of chats with antiflood enabled (limit > 0).
func LoadAntifloodStats() (antiCount int64) {
	var totalCount int64
	var noAntiCount int64

	// Count total antiflood settings
	err := DB.Model(&AntifloodSettings{}).Count(&totalCount).Error
	if err != nil {
		log.Errorf("[Database] LoadAntifloodStats: %v", err)
		return 0
	}

	// Count settings with limit 0 (disabled)
	err = DB.Model(&AntifloodSettings{}).Where("flood_limit = ?", 0).Count(&noAntiCount).Error
	if err != nil {
		log.Errorf("[Database] LoadAntifloodStats: %v", err)
		return 0
	}

	antiCount = totalCount - noAntiCount // gives chats which have enabled anti flood

	return
}
