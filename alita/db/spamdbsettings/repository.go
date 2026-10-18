// Package spamdbsettings stores each chat's CAS join check settings. The
// lookup client itself is alita/utils/spamdb.
package spamdbsettings

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

// What the join check does to a listed user.
const (
	ActionBan  = "ban"
	ActionMute = "mute"
	ActionLog  = "log"
)

const cacheKeyPrefix = "spam_db_settings"

// ErrInvalidAction is returned for an action other than ban, mute or log.
var ErrInvalidAction = errors.New("spamdbsettings: invalid action")

// ValidAction reports whether action is one SetSpamDBAction accepts.
func ValidAction(action string) bool {
	switch action {
	case ActionBan, ActionMute, ActionLog:
		return true
	}
	return false
}

// GetSpamDBSettingsContext returns the chat's settings. A missing row reads as
// disabled with the ban action.
func GetSpamDBSettingsContext(ctx context.Context, chatID int64) (*models.SpamDBSettings, error) {
	return cache.GetFromCacheOrLoad(ctx, cache.CacheKey(cacheKeyPrefix, chatID), cache.CacheTTLSpamDB,
		func(ctx context.Context) (*models.SpamDBSettings, error) {
			settings := &models.SpamDBSettings{}
			err := db.GetRecordContext(ctx, settings, models.SpamDBSettings{ChatID: chatID})
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return &models.SpamDBSettings{ChatID: chatID, Action: ActionBan}, nil
			}
			if err != nil {
				return nil, err
			}
			return settings, nil
		})
}

// SetSpamDBEnabled turns the join check on or off for the chat, keeping its
// action.
func SetSpamDBEnabled(chatID int64, enabled bool) error {
	return upsert(chatID, &models.SpamDBSettings{ChatID: chatID, Enabled: enabled, Action: ActionBan}, "enabled")
}

// SetSpamDBAction sets what the join check does to a listed user, keeping the
// on/off switch.
func SetSpamDBAction(chatID int64, action string) error {
	if !ValidAction(action) {
		return ErrInvalidAction
	}
	return upsert(chatID, &models.SpamDBSettings{ChatID: chatID, Action: action}, "action")
}

// upsert inserts row, or on an existing row updates only column, so the two
// settings never overwrite each other.
func upsert(chatID int64, row *models.SpamDBSettings, column string) error {
	err := db.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}},
		DoUpdates: clause.AssignmentColumns([]string{column, "updated_at"}),
	}).Create(row).Error
	if err != nil {
		log.Errorf("[Database] spam db settings %s: %v - %d", column, err, chatID)
		return err
	}

	cache.DeleteCache(cache.CacheKey(cacheKeyPrefix, chatID))
	return nil
}
