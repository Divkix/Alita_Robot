package aispam

import (
	"context"
	"errors"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

// IsAISpamEnabled reports whether the AI spam filter is on for the chat.
// A missing row means disabled.
func IsAISpamEnabled(chatID int64) bool {
	return IsAISpamEnabledContext(context.Background(), chatID)
}

func IsAISpamEnabledContext(ctx context.Context, chatID int64) bool {
	settings, err := getAISpamSettingsCachedContext(ctx, chatID)
	if err != nil {
		log.Errorf("[Database] IsAISpamEnabled: %v - %d", err, chatID)
		return false
	}
	return settings.Enabled
}

// SetAISpamEnabled turns the filter on or off for the chat.
func SetAISpamEnabled(chatID int64, enabled bool) error {
	err := db.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"enabled", "updated_at"}),
	}).Create(&models.AISpamSettings{
		ChatID:  chatID,
		Enabled: enabled,
	}).Error
	if err != nil {
		log.Errorf("[Database] SetAISpamEnabled: %v - %d", err, chatID)
		return err
	}

	cache.DeleteCache(cache.CacheKey("ai_spam_settings", chatID))
	return nil
}

func getAISpamSettingsCachedContext(ctx context.Context, chatID int64) (*models.AISpamSettings, error) {
	return cache.GetFromCacheOrLoad(ctx, cache.CacheKey("ai_spam_settings", chatID), cache.CacheTTLAISpam,
		func(ctx context.Context) (*models.AISpamSettings, error) {
			settings := &models.AISpamSettings{}
			err := db.GetRecordContext(ctx, settings, models.AISpamSettings{ChatID: chatID})
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return &models.AISpamSettings{ChatID: chatID}, nil
			}
			if err != nil {
				return nil, err
			}
			return settings, nil
		})
}

func LoadAISpamStats() (enabledChats int64) {
	return db.CountRows(&models.AISpamSettings{}, "enabled = ?", true)
}
