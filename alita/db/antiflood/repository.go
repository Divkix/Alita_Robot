package antiflood

import (
	"errors"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// default mode is 'mute'
const defaultFloodsettingsMode string = "mute"

// GetFlood Get flood settings for a chat
func GetFlood(chatID int64) *models.AntifloodSettings {
	return checkFloodSetting(chatID)
}

// checkFloodSetting retrieves or returns default antiflood settings for a chat.
// Uses optimized cached queries and returns default settings if not found.
func checkFloodSetting(chatID int64) (floodSrc *models.AntifloodSettings) {
	// Use optimized cached query instead of SELECT *
	floodSrc, err := GetAntifloodSettingsCached(chatID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Return default settings
			return &models.AntifloodSettings{ChatId: chatID, Limit: 0, Action: defaultFloodsettingsMode}
		}
		log.Errorf("[Database][checkFloodSetting]: %v", err)
		return &models.AntifloodSettings{ChatId: chatID, Limit: 0, Action: defaultFloodsettingsMode}
	}
	return floodSrc
}

// upsertChatField upserts the given column updates for a chat's antiflood settings
// and invalidates the antiflood cache. Callers handle any pre-read short-circuits.
func upsertChatField(chatID int64, updates map[string]any) error {
	if err := db.DB.Where("chat_id = ?", chatID).
		Assign(updates).
		FirstOrCreate(&models.AntifloodSettings{}).Error; err != nil {
		log.Errorf("[Database] upsertChatField: %v - %d", err, chatID)
		return err
	}
	// Invalidate cache after update
	cache.DeleteCache(cache.CacheKey("antiflood", chatID))
	return nil
}

// SetFlood set Flood Setting for a Chat
func SetFlood(chatID int64, limit int) error {
	floodSrc := checkFloodSetting(chatID)

	// Check if update is actually needed
	if floodSrc.Limit == limit {
		return nil
	}

	action := floodSrc.Action
	if action == "" {
		action = defaultFloodsettingsMode
	}

	// Use map to ensure zero values (limit=0) are persisted
	updates := map[string]any{
		"chat_id":     chatID,
		"flood_limit": limit,
		"action":      action,
	}
	return upsertChatField(chatID, updates)
}

// SetFloodMode Set flood mode for a chat
func SetFloodMode(chatID int64, mode string) error {
	floodSrc := checkFloodSetting(chatID)
	// Check if update is actually needed
	if floodSrc.Action == mode {
		return nil
	}
	// Use map for consistency with other antiflood setters
	updates := map[string]any{
		"chat_id": chatID,
		"action":  mode,
	}
	return upsertChatField(chatID, updates)
}

// SetFloodMsgDel Set flood message deletion setting for a chat
func SetFloodMsgDel(chatID int64, val bool) error {
	floodSrc := checkFloodSetting(chatID)
	// Check if update is actually needed
	if floodSrc.DeleteAntifloodMessage == val {
		return nil
	}
	// Use map to ensure zero values (val=false) are persisted
	updates := map[string]any{
		"chat_id":                  chatID,
		"delete_antiflood_message": val,
	}
	return upsertChatField(chatID, updates)
}

// LoadAntifloodStats returns the count of chats with antiflood enabled (limit > 0).
func LoadAntifloodStats() (antiCount int64) {
	var totalCount int64
	var noAntiCount int64

	// Count total antiflood settings
	err := db.DB.Model(&models.AntifloodSettings{}).Count(&totalCount).Error
	if err != nil {
		log.Errorf("[Database] LoadAntifloodStats: %v", err)
		return 0
	}

	// Count settings with limit 0 (disabled)
	err = db.DB.Model(&models.AntifloodSettings{}).Where("flood_limit = ?", 0).Count(&noAntiCount).Error
	if err != nil {
		log.Errorf("[Database] LoadAntifloodStats: %v", err)
		return 0
	}

	antiCount = totalCount - noAntiCount // gives chats which have enabled anti flood

	return
}
