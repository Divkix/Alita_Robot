// Package digestsettings stores each chat's weekly digest choice: whether it
// is on and where it goes. The rendering and sending live in alita/digest.
package digestsettings

import (
	"context"
	"errors"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

// Destinations an admin can choose, matching digest_settings.destination.
const (
	// DestinationAuto follows the default: the log channel when the chat has
	// one, otherwise the group.
	DestinationAuto = ""
	// DestinationLog posts the digest to the chat's log channel.
	DestinationLog = "log"
	// DestinationGroup posts the digest to the group itself.
	DestinationGroup = "group"
)

const cacheKeyPrefix = "digest_settings"

// ErrInvalidDestination is returned for a destination other than "", log or group.
var ErrInvalidDestination = errors.New("digestsettings: invalid destination")

// ValidDestination reports whether destination is one SetDestination accepts.
func ValidDestination(destination string) bool {
	switch destination {
	case DestinationAuto, DestinationLog, DestinationGroup:
		return true
	}
	return false
}

func invalidate(chatID int64) {
	cache.DeleteCache(cache.CacheKey(cacheKeyPrefix, chatID))
}

// Get returns the chat's settings. A missing row reads as disabled, following
// the default destination, and never sent.
func Get(chatID int64) (*models.DigestSettings, error) {
	return GetContext(context.Background(), chatID)
}

func GetContext(ctx context.Context, chatID int64) (*models.DigestSettings, error) {
	return cache.GetFromCacheOrLoad(ctx, cache.CacheKey(cacheKeyPrefix, chatID), cache.CacheTTLDigest,
		func(ctx context.Context) (*models.DigestSettings, error) {
			settings := &models.DigestSettings{}
			err := db.GetRecordContext(ctx, settings, models.DigestSettings{ChatID: chatID})
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return &models.DigestSettings{ChatID: chatID}, nil
			}
			if err != nil {
				return nil, err
			}
			return settings, nil
		})
}

// SetEnabled turns the weekly digest on or off for the chat, keeping its
// destination and send history.
func SetEnabled(chatID int64, enabled bool) error {
	return upsert(chatID, "enabled", map[string]any{"enabled": enabled})
}

// SetDestination sets where the digest goes, keeping the on/off switch and
// send history.
func SetDestination(chatID int64, destination string) error {
	if !ValidDestination(destination) {
		return ErrInvalidDestination
	}
	return upsert(chatID, "destination", map[string]any{"destination": destination})
}

// upsert inserts row, or on an existing row updates only column, so the
// switch and the destination never overwrite each other. row is a map because
// GORM swaps a struct's false value for the column default on insert, which
// would turn enabled=false into true.
func upsert(chatID int64, column string, row map[string]any) error {
	now := time.Now()
	row["chat_id"] = chatID
	row["created_at"] = now
	row["updated_at"] = now
	err := db.DB.Model(&models.DigestSettings{}).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}},
		DoUpdates: clause.AssignmentColumns([]string{column, "updated_at"}),
	}).Create(row).Error
	if err != nil {
		log.Errorf("[Database] digest settings %s: %v - %d", column, err, chatID)
		return err
	}

	invalidate(chatID)
	return nil
}

// ListDue returns every chat whose digest is on and whose last send is older
// than boundary, or that has never had one. It bypasses the cache: the
// scheduler needs the row each replica will claim against, not a copy.
func ListDue(boundary time.Time) ([]models.DigestSettings, error) {
	if db.DB == nil {
		return nil, errors.New("database not initialized")
	}

	var rows []models.DigestSettings
	err := db.DB.
		Where("enabled = ? AND (last_sent_at IS NULL OR last_sent_at < ?)", true, boundary.UTC()).
		Order("chat_id ASC").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list due digests: %w", err)
	}
	return rows, nil
}

// Claim takes ownership of this week's send by setting last_sent_at only when
// it still holds the value the caller read. It returns true when exactly one
// row was updated, which is the only case in which the caller may send: two
// replicas that race get true from one and false from the other.
func Claim(chatID int64, old *time.Time, claimedAt time.Time) (bool, error) {
	if db.DB == nil {
		return false, errors.New("database not initialized")
	}

	query := db.DB.Model(&models.DigestSettings{}).Where("chat_id = ?", chatID)
	if old == nil {
		query = query.Where("last_sent_at IS NULL")
	} else {
		query = query.Where("last_sent_at = ?", old.UTC())
	}

	res := query.Updates(map[string]any{
		"last_sent_at": claimedAt.UTC(),
		"updated_at":   time.Now().UTC(),
	})
	if res.Error != nil {
		return false, fmt.Errorf("claim digest send: %w", res.Error)
	}
	if res.RowsAffected == 1 {
		invalidate(chatID)
	}
	return res.RowsAffected == 1, nil
}

// Restore puts the pre-claim value back after a failed send, so the next tick
// retries. It only touches a row still holding claimedAt: a slower replica
// must not undo a later successful claim.
func Restore(chatID int64, claimedAt time.Time, previous *time.Time) error {
	if db.DB == nil {
		return errors.New("database not initialized")
	}

	var value any
	if previous != nil {
		value = previous.UTC()
	}
	res := db.DB.Model(&models.DigestSettings{}).
		Where("chat_id = ? AND last_sent_at = ?", chatID, claimedAt.UTC()).
		Updates(map[string]any{
			"last_sent_at": value,
			"updated_at":   time.Now().UTC(),
		})
	if res.Error != nil {
		return fmt.Errorf("restore digest claim: %w", res.Error)
	}
	invalidate(chatID)
	return nil
}
