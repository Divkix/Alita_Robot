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

// upsertChatFields writes the supplied columns atomically, creating the row
// when missing. A created row starts with flood_limit 0 (disabled) instead of
// the column default 5, so only SetFlood can turn antiflood on. The insert
// goes through a map because GORM replaces a zero struct field with its
// `default` tag on create.
func upsertChatFields(ctx context.Context, chatID int64, fields map[string]any) error {
	columns := make([]string, 0, len(fields)+1)
	for column := range fields {
		columns = append(columns, column)
	}
	columns = append(columns, "updated_at")
	now := time.Now()
	fields["chat_id"] = chatID
	if _, ok := fields["flood_limit"]; !ok {
		fields["flood_limit"] = 0
	}
	fields["created_at"] = now
	fields["updated_at"] = now
	if err := db.DB.WithContext(ctx).Model(&models.AntifloodSettings{}).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}},
		DoUpdates: clause.AssignmentColumns(columns),
	}).Create(fields).Error; err != nil {
		log.Errorf("[Database] upsertChatFields: %v - %d", err, chatID)
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
	return upsertChatFields(ctx, chatID, map[string]any{"flood_limit": limit})
}

func SetFloodMode(chatID int64, mode string, actionTime int) error {
	return SetFloodModeContext(context.Background(), chatID, mode, actionTime)
}

func SetFloodModeContext(ctx context.Context, chatID int64, mode string, actionTime int) error {
	if mode != "tban" && mode != "tmute" {
		actionTime = 0
	}
	floodSrc, err := GetAntifloodSettingsCachedContext(ctx, chatID)
	if err != nil {
		return err
	}
	if floodSrc.Action == mode && floodSrc.FloodActionTime == actionTime {
		return nil
	}
	return upsertChatFields(ctx, chatID, map[string]any{"action": mode, "flood_action_time": actionTime})
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
	return upsertChatFields(ctx, chatID, map[string]any{"delete_antiflood_message": val})
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
