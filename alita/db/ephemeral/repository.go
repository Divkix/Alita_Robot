// Package ephemeral stores each chat's /ephemeral switch: whether answers
// Alita would otherwise redirect to PM are shown only to the asker in the
// group. A chat that never set it is off. Read it through
// settings.EphemeralReplies, which owns the default.
package ephemeral

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

const cacheKeyPrefix = "ephemeral_settings"

func cacheKey(chatID int64) string {
	return cache.CacheKey(cacheKeyPrefix, chatID)
}

// GetContext returns the chat's row; a chat without one reads as off and
// nothing is inserted.
func GetContext(ctx context.Context, chatID int64) (*models.EphemeralSettings, error) {
	return cache.GetFromCacheOrLoad(ctx, cacheKey(chatID), cache.CacheTTLEphemeral,
		func(ctx context.Context) (*models.EphemeralSettings, error) {
			row := &models.EphemeralSettings{}
			err := db.GetRecordContext(ctx, row, models.EphemeralSettings{ChatID: chatID})
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return &models.EphemeralSettings{ChatID: chatID}, nil
			}
			if err != nil {
				return nil, err
			}
			return row, nil
		})
}

// SetEnabled stores the switch. The row is a map because GORM swaps a
// struct's false for the column default on insert.
func SetEnabled(chatID int64, enabled bool) error {
	if err := chats.EnsureChatInDb(chatID, ""); err != nil {
		return err
	}
	now := time.Now()
	err := db.DB.Model(&models.EphemeralSettings{}).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"enabled", "updated_at"}),
	}).Create(map[string]any{"chat_id": chatID, "enabled": enabled, "created_at": now, "updated_at": now}).Error
	if err != nil {
		return err
	}
	cache.DeleteCache(cacheKey(chatID))
	return nil
}
