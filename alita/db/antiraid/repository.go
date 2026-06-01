package antiraid

import (
	"errors"
	"fmt"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/queries"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// defaultAntiRaidSettings returns default settings for a chat when no record exists.
// Raid time: 6h (21600s), action time: 1h (3600s), auto threshold: 0 (disabled).
func defaultAntiRaidSettings(chatID int64) *models.AntiRaidSettings {
	return &models.AntiRaidSettings{
		ChatID:                chatID,
		RaidTime:              21600,
		RaidActionTime:        3600,
		AutoAntiRaidThreshold: 0,
	}
}

// GetAntiRaidSettings retrieves anti-raid settings for a chat.
// Returns defaults if no record exists.
func GetAntiRaidSettings(chatID int64) *models.AntiRaidSettings {
	settings, err := queries.GetOptimizedQueries().GetAntiRaidSettingsCached(chatID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return defaultAntiRaidSettings(chatID)
		}
		log.Errorf("[Database][GetAntiRaidSettings]: %v", err)
		return defaultAntiRaidSettings(chatID)
	}
	return settings
}

// SetRaidTime sets the raid duration (in seconds) for a chat.
func SetRaidTime(chatID int64, seconds int) error {
	if seconds < 0 {
		return fmt.Errorf("raid time must be non-negative, got %d", seconds)
	}

	updates := map[string]any{
		"chat_id":    chatID,
		"raid_time":  seconds,
	}
	err := db.DB.Where("chat_id = ?", chatID).
		Assign(updates).
		FirstOrCreate(&models.AntiRaidSettings{}).Error
	if err != nil {
		log.Errorf("[Database] SetRaidTime: %v - %d", err, chatID)
		return err
	}
	cache.DeleteCache(cache.CacheKey("antiraid", chatID))
	return nil
}

// SetRaidActionTime sets the ban/restriction duration during a raid (in seconds).
func SetRaidActionTime(chatID int64, seconds int) error {
	if seconds < 0 {
		return fmt.Errorf("raid action time must be non-negative, got %d", seconds)
	}

	updates := map[string]any{
		"chat_id":           chatID,
		"raid_action_time":  seconds,
	}
	err := db.DB.Where("chat_id = ?", chatID).
		Assign(updates).
		FirstOrCreate(&models.AntiRaidSettings{}).Error
	if err != nil {
		log.Errorf("[Database] SetRaidActionTime: %v - %d", err, chatID)
		return err
	}
	cache.DeleteCache(cache.CacheKey("antiraid", chatID))
	return nil
}

// SetAutoAntiRaidThreshold sets the auto-trigger join-rate threshold.
// 0 disables auto-trigger.
func SetAutoAntiRaidThreshold(chatID int64, threshold int) error {
	if threshold < 0 {
		return fmt.Errorf("threshold must be non-negative, got %d", threshold)
	}

	updates := map[string]any{
		"chat_id":                   chatID,
		"auto_antiraid_threshold":   threshold,
	}
	err := db.DB.Where("chat_id = ?", chatID).
		Assign(updates).
		FirstOrCreate(&models.AntiRaidSettings{}).Error
	if err != nil {
		log.Errorf("[Database] SetAutoAntiRaidThreshold: %v - %d", err, chatID)
		return err
	}
	cache.DeleteCache(cache.CacheKey("antiraid", chatID))
	return nil
}
