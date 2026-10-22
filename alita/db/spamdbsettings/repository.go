// Package spamdbsettings stores each chat's spam database join check
// settings. The lookup client itself is alita/utils/spamdb.
package spamdbsettings

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
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

// The databases the join check can ask, as admins name them. Shared is the
// bot's own reputation aggregate, not a third-party service.
const (
	DatabaseCAS    = "cas"
	DatabaseLols   = "lols"
	DatabaseShared = "shared"
)

const cacheKeyPrefix = "spam_db_settings"

var (
	// ErrInvalidAction is returned for an action other than ban, mute or log.
	ErrInvalidAction = errors.New("spamdbsettings: invalid action")
	// ErrInvalidDatabase is returned for a database other than cas, lols or
	// shared.
	ErrInvalidDatabase = errors.New("spamdbsettings: invalid database")
	// ErrLastDatabase is returned when switching off the only database left.
	ErrLastDatabase = errors.New("spamdbsettings: at least one database must stay on")
)

// databaseColumns maps each database to its column.
var databaseColumns = map[string]string{
	DatabaseCAS:    "check_cas",
	DatabaseLols:   "check_lols",
	DatabaseShared: "check_shared",
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
	if _, ok := databaseColumns[database]; !ok {
		return ErrInvalidDatabase
	}
	var current *models.SpamDBSettings
	if !on {
		var err error
		if current, err = loadSettings(context.Background(), chatID); err != nil {
			log.Errorf("[Database] spam db settings %s: %v - %d", databaseColumns[database], err, chatID)
			return err
		}
	}
	return writeDatabase(chatID, database, on, current)
}

// writeDatabase stores the switch. current is the chat's settings as read
// before the write, needed to switch a database off; it may be stale by then.
func writeDatabase(chatID int64, database string, on bool, current *models.SpamDBSettings) error {
	column := databaseColumns[database]
	row := map[string]any{column: on}
	if !on {
		otherOn := false
		for other, otherColumn := range databaseColumns {
			if other == database {
				continue
			}
			otherOn = otherOn || databaseOn(current, other)
			// The check constraint applies to the row an upsert proposes,
			// even when it then updates an existing row, and a first write
			// inserts it as is, so the proposal carries every other database
			// as it was read: at least one of them is on.
			row[otherColumn] = databaseOn(current, other)
		}
		if !otherOn {
			return ErrLastDatabase
		}
	}

	err := upsert(chatID, column, row)
	if isLastDatabaseViolation(err) {
		// Another admin switched every other database off after current was
		// read.
		return ErrLastDatabase
	}
	return err
}

// databaseOn reports whether settings has database switched on. A nil row
// counts as every database off.
func databaseOn(settings *models.SpamDBSettings, database string) bool {
	if settings == nil {
		return false
	}
	switch database {
	case DatabaseCAS:
		return settings.CheckCAS
	case DatabaseLols:
		return settings.CheckLols
	case DatabaseShared:
		return settings.CheckShared
	}
	return false
}

// databasesCheck is the constraint that keeps at least one database on.
const databasesCheck = "chk_spam_db_databases"

// isLastDatabaseViolation reports whether err is the table refusing a row with
// no database on: Postgres SQLSTATE 23514 naming databasesCheck, or SQLite's
// "CHECK constraint failed" message naming it.
func isLastDatabaseViolation(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23514" && pgErr.ConstraintName == databasesCheck
	}
	return strings.Contains(err.Error(), "CHECK constraint failed: "+databasesCheck)
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
		if !isLastDatabaseViolation(err) {
			log.Errorf("[Database] spam db settings %s: %v - %d", column, err, chatID)
		}
		return err
	}

	cache.DeleteCache(cache.CacheKey(cacheKeyPrefix, chatID))
	return nil
}
