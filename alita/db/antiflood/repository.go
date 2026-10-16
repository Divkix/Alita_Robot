package antiflood

import (
	"context"
	"errors"
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

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

// upsertChatField writes one column of the chat's settings row, creating the
// row when missing. A created row starts with flood_limit 0 (disabled) instead
// of the column default 5, so only SetFlood can turn antiflood on. The insert
// goes through a map because GORM replaces a zero struct field with its
// `default` tag on create.
func upsertChatField(ctx context.Context, chatID int64, column string, value any) error {
	now := time.Now()
	row := map[string]any{
		"chat_id":     chatID,
		"flood_limit": 0,
		"created_at":  now,
		"updated_at":  now,
	}
	row[column] = value
	if err := db.DB.WithContext(ctx).Model(&models.AntifloodSettings{}).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}},
		DoUpdates: clause.AssignmentColumns([]string{column, "updated_at"}),
	}).Create(row).Error; err != nil {
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
	return upsertChatField(ctx, chatID, "flood_limit", limit)
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
	return upsertChatField(ctx, chatID, "action", mode)
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
	return upsertChatField(ctx, chatID, "delete_antiflood_message", val)
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
