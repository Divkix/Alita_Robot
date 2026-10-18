// Package spamdbsettings stores each chat's spam database join check
// settings. The lookup client itself is alita/utils/spamdb.
package spamdbsettings

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

// What the join check does to a listed user.
const (
	ActionBan  = "ban"
	ActionMute = "mute"
	ActionLog  = "log"
)

// The databases the join check can ask, as admins name them.
const (
	DatabaseCAS  = "cas"
	DatabaseLols = "lols"
)

const cacheKeyPrefix = "spam_db_settings"

var (
	// ErrInvalidAction is returned for an action other than ban, mute or log.
	ErrInvalidAction = errors.New("spamdbsettings: invalid action")
	// ErrInvalidDatabase is returned for a database other than cas or lols.
	ErrInvalidDatabase = errors.New("spamdbsettings: invalid database")
	// ErrLastDatabase is returned when switching off the only database left.
	ErrLastDatabase = errors.New("spamdbsettings: at least one database must stay on")
)

// databaseColumns maps each database to its column.
var databaseColumns = map[string]string{
	DatabaseCAS:  "check_cas",
	DatabaseLols: "check_lols",
}

// ValidAction reports whether action is one SetSpamDBAction accepts.
func ValidAction(action string) bool {
	switch action {
	case ActionBan, ActionMute, ActionLog:
		return true
	}
	return false
}

// GetSpamDBSettingsContext returns the chat's settings. A missing row reads as
// disabled, CAS only, with the ban action.
func GetSpamDBSettingsContext(ctx context.Context, chatID int64) (*models.SpamDBSettings, error) {
	return cache.GetFromCacheOrLoad(ctx, cache.CacheKey(cacheKeyPrefix, chatID), cache.CacheTTLSpamDB,
		func(ctx context.Context) (*models.SpamDBSettings, error) {
			return loadSettings(ctx, chatID)
		})
}

func loadSettings(ctx context.Context, chatID int64) (*models.SpamDBSettings, error) {
	settings := &models.SpamDBSettings{}
	err := db.GetRecordContext(ctx, settings, models.SpamDBSettings{ChatID: chatID})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &models.SpamDBSettings{ChatID: chatID, Action: ActionBan, CheckCAS: true}, nil
	}
	if err != nil {
		return nil, err
	}
	return settings, nil
}

// SetSpamDBEnabled turns the join check on or off for the chat, keeping its
// action.
func SetSpamDBEnabled(chatID int64, enabled bool) error {
	return upsert(chatID, "enabled", map[string]any{"enabled": enabled})
}

// SetSpamDBAction sets what the join check does to a listed user, keeping the
// on/off switch.
func SetSpamDBAction(chatID int64, action string) error {
	if !ValidAction(action) {
		return ErrInvalidAction
	}
	return upsert(chatID, "action", map[string]any{"action": action})
}

// SetSpamDBDatabase switches one database on or off for the chat, keeping the
// other settings. Switching off the only database left is refused with
// ErrLastDatabase; the table's check constraint backs this up against two
// concurrent writes.
func SetSpamDBDatabase(chatID int64, database string, on bool) error {
	column, ok := databaseColumns[database]
	if !ok {
		return ErrInvalidDatabase
	}
	row := map[string]any{column: on}
	if !on {
		current, err := loadSettings(context.Background(), chatID)
		if err != nil {
			log.Errorf("[Database] spam db settings %s: %v - %d", column, err, chatID)
			return err
		}
		other, otherOn := "check_lols", current.CheckLols
		if database == DatabaseLols {
			other, otherOn = "check_cas", current.CheckCAS
		}
		if !otherOn {
			return ErrLastDatabase
		}
		// The check constraint applies to the row an upsert proposes, even
		// when it then updates an existing row, so the proposal carries the
		// other database as it is: on.
		row[other] = true
	}

	return upsert(chatID, column, row)
}

// upsert inserts row, or on an existing row updates only column, so the
// settings never overwrite each other; any column row leaves out takes its
// default in a new row. row is a map because GORM swaps a struct's zero value
// for the column's default on insert, which would turn check_cas=false into
// true.
func upsert(chatID int64, column string, row map[string]any) error {
	now := time.Now()
	row["chat_id"] = chatID
	row["created_at"] = now
	row["updated_at"] = now
	err := db.DB.Model(&models.SpamDBSettings{}).Clauses(clause.OnConflict{
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
