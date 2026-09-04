package antiflood

import (
	"errors"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

const defaultFloodsettingsMode string = "mute"

func GetFlood(chatID int64) *models.AntifloodSettings {
	return checkFloodSetting(chatID)
}

func checkFloodSetting(chatID int64) (floodSrc *models.AntifloodSettings) {
	floodSrc, err := GetAntifloodSettingsCached(chatID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &models.AntifloodSettings{ChatId: chatID, Limit: 0, Action: defaultFloodsettingsMode}
		}
		log.Errorf("[Database][checkFloodSetting]: %v", err)
		return &models.AntifloodSettings{ChatId: chatID, Limit: 0, Action: defaultFloodsettingsMode}
	}
	return floodSrc
}

func upsertChatField(chatID int64, updates map[string]any) error {
	if err := db.DB.Where("chat_id = ?", chatID).
		Assign(updates).
		FirstOrCreate(&models.AntifloodSettings{}).Error; err != nil {
		log.Errorf("[Database] upsertChatField: %v - %d", err, chatID)
		return err
	}
	cache.DeleteCache(cache.CacheKey("antiflood", chatID))
	return nil
}

func SetFlood(chatID int64, limit int) error {
	floodSrc := checkFloodSetting(chatID)

	if floodSrc.Limit == limit {
		return nil
	}

	action := floodSrc.Action
	if action == "" {
		action = defaultFloodsettingsMode
	}

	updates := map[string]any{
		"chat_id":     chatID,
		"flood_limit": limit,
		"action":      action,
	}
	return upsertChatField(chatID, updates)
}

func SetFloodMode(chatID int64, mode string) error {
	floodSrc := checkFloodSetting(chatID)
	if floodSrc.Action == mode {
		return nil
	}
	updates := map[string]any{
		"chat_id": chatID,
		"action":  mode,
	}
	return upsertChatField(chatID, updates)
}

func SetFloodMsgDel(chatID int64, val bool) error {
	floodSrc := checkFloodSetting(chatID)
	if floodSrc.DeleteAntifloodMessage == val {
		return nil
	}
	updates := map[string]any{
		"chat_id":                  chatID,
		"delete_antiflood_message": val,
	}
	return upsertChatField(chatID, updates)
}

func LoadAntifloodStats() (antiCount int64) {
	var totalCount int64
	var noAntiCount int64

	err := db.DB.Model(&models.AntifloodSettings{}).Count(&totalCount).Error
	if err != nil {
		log.Errorf("[Database] LoadAntifloodStats: %v", err)
		return 0
	}

	err = db.DB.Model(&models.AntifloodSettings{}).Where("flood_limit = ?", 0).Count(&noAntiCount).Error
	if err != nil {
		log.Errorf("[Database] LoadAntifloodStats: %v", err)
		return 0
	}

	antiCount = totalCount - noAntiCount

	return
}
