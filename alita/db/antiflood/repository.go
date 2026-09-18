package antiflood

import (
	"context"
	"errors"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

const defaultFloodsettingsMode string = "mute"

func GetFlood(chatID int64) *models.AntifloodSettings {
	return GetFloodContext(context.Background(), chatID)
}

func GetFloodContext(ctx context.Context, chatID int64) (floodSrc *models.AntifloodSettings) {
	floodSrc, err := GetAntifloodSettingsCachedContext(ctx, chatID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &models.AntifloodSettings{ChatId: chatID, Limit: 0, Action: defaultFloodsettingsMode}
		}
		log.Errorf("[Database][GetFlood]: %v", err)
		return &models.AntifloodSettings{ChatId: chatID, Limit: 0, Action: defaultFloodsettingsMode}
	}
	return floodSrc
}

func upsertChatField(ctx context.Context, chatID int64, updates map[string]any) error {
	if err := db.DB.WithContext(ctx).Where("chat_id = ?", chatID).
		Assign(updates).
		FirstOrCreate(&models.AntifloodSettings{}).Error; err != nil {
		log.Errorf("[Database] upsertChatField: %v - %d", err, chatID)
		return err
	}
	cache.DeleteCache(cache.CacheKey("antiflood", chatID))
	return nil
}

func SetFlood(chatID int64, limit int) error {
	return SetFloodContext(context.Background(), chatID, limit)
}

func SetFloodContext(ctx context.Context, chatID int64, limit int) error {
	floodSrc, err := GetAntifloodSettingsCachedContext(ctx, chatID)
	if err != nil {
		return err
	}

	if floodSrc.Limit == limit {
		return nil
	}

	// Update only flood_limit: writing back the cached action would clobber a
	// concurrent SetFloodMode with a stale value.
	updates := map[string]any{
		"chat_id":     chatID,
		"flood_limit": limit,
	}
	return upsertChatField(ctx, chatID, updates)
}

func SetFloodMode(chatID int64, mode string) error {
	return SetFloodModeContext(context.Background(), chatID, mode)
}

func SetFloodModeContext(ctx context.Context, chatID int64, mode string) error {
	floodSrc, err := GetAntifloodSettingsCachedContext(ctx, chatID)
	if err != nil {
		return err
	}
	if floodSrc.Action == mode {
		return nil
	}
	updates := map[string]any{
		"chat_id": chatID,
		"action":  mode,
	}
	return upsertChatField(ctx, chatID, updates)
}

func SetFloodMsgDel(chatID int64, val bool) error {
	return SetFloodMsgDelContext(context.Background(), chatID, val)
}

func SetFloodMsgDelContext(ctx context.Context, chatID int64, val bool) error {
	floodSrc, err := GetAntifloodSettingsCachedContext(ctx, chatID)
	if err != nil {
		return err
	}
	if floodSrc.DeleteAntifloodMessage == val {
		return nil
	}
	updates := map[string]any{
		"chat_id":                  chatID,
		"delete_antiflood_message": val,
	}
	return upsertChatField(ctx, chatID, updates)
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
