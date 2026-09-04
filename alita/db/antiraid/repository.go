package antiraid

import (
	"errors"
	"fmt"
	"math"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func defaultAntiRaidSettings(chatID int64) *models.AntiRaidSettings {
	return &models.AntiRaidSettings{
		ChatID:                chatID,
		RaidTime:              21600,
		RaidActionTime:        3600,
		AutoAntiRaidThreshold: 0,
	}
}

func GetAntiRaidSettings(chatID int64) *models.AntiRaidSettings {
	settings, err := GetAntiRaidSettingsCached(chatID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return defaultAntiRaidSettings(chatID)
		}
		log.Errorf("[Database][GetAntiRaidSettings]: %v", err)
		return defaultAntiRaidSettings(chatID)
	}
	return settings
}

func upsertChatField(chatID int64, updates map[string]any) error {
	if err := db.DB.Where("chat_id = ?", chatID).
		Assign(updates).
		FirstOrCreate(&models.AntiRaidSettings{}).Error; err != nil {
		log.Errorf("[Database] upsertChatField: %v - %d", err, chatID)
		return err
	}
	cache.DeleteCache(cache.CacheKey("antiraid", chatID))
	return nil
}

func SetRaidTime(chatID int64, seconds int) error {
	if seconds < 0 {
		return fmt.Errorf("raid time must be non-negative, got %d", seconds)
	}
	if int64(seconds) > math.MaxInt32 {
		return fmt.Errorf("raid time exceeds a PostgreSQL integer, got %d", seconds)
	}

	updates := map[string]any{
		"chat_id":   chatID,
		"raid_time": seconds,
	}
	return upsertChatField(chatID, updates)
}

func SetRaidActionTime(chatID int64, seconds int) error {
	if seconds < 0 {
		return fmt.Errorf("raid action time must be non-negative, got %d", seconds)
	}
	if int64(seconds) > math.MaxInt32 {
		return fmt.Errorf("raid action time exceeds a PostgreSQL integer, got %d", seconds)
	}

	updates := map[string]any{
		"chat_id":          chatID,
		"raid_action_time": seconds,
	}
	return upsertChatField(chatID, updates)
}

func SetAutoAntiRaidThreshold(chatID int64, threshold int) error {
	if threshold < 0 {
		return fmt.Errorf("threshold must be non-negative, got %d", threshold)
	}

	updates := map[string]any{
		"chat_id":                 chatID,
		"auto_antiraid_threshold": threshold,
	}
	return upsertChatField(chatID, updates)
}
