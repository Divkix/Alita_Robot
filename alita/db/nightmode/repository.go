// Package nightmode stores each chat's night mode: lock types the locks
// delete path enforces every day between two times in the chat's timezone.
// Which lock types exist is the locks module's business; this package only
// stores them and answers whether a local time falls inside the window.
package nightmode

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

// MinutesPerDay bounds StartMinute and EndMinute: both lie in [0, MinutesPerDay).
const MinutesPerDay = 24 * 60

const cacheKeyPrefix = "night_mode"

var (
	// ErrInvalidWindow is returned for a time outside the day, or a start
	// equal to the end (an empty or all-day window).
	ErrInvalidWindow = errors.New("nightmode: invalid window")
	// ErrNoLockTypes is returned when Enable gets no lock type.
	ErrNoLockTypes = errors.New("nightmode: no lock types")
)

func cacheKey(chatID int64) string {
	return cache.CacheKey(cacheKeyPrefix, chatID)
}

// Get returns the chat's night mode. A missing row reads as disabled.
func Get(chatID int64) (*models.NightModeSettings, error) {
	return GetContext(context.Background(), chatID)
}

func GetContext(ctx context.Context, chatID int64) (*models.NightModeSettings, error) {
	return cache.GetFromCacheOrLoad(ctx, cacheKey(chatID), cache.CacheTTLNightMode,
		func(ctx context.Context) (*models.NightModeSettings, error) {
			settings := &models.NightModeSettings{}
			err := db.GetRecordContext(ctx, settings, models.NightModeSettings{ChatID: chatID})
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return &models.NightModeSettings{ChatID: chatID}, nil
			}
			if err != nil {
				return nil, err
			}
			return settings, nil
		})
}

// Enable turns night mode on with the given window and lock types, replacing
// any earlier ones. The types are stored sorted and without duplicates.
func Enable(chatID int64, startMinute, endMinute int, lockTypes []string) error {
	if !validMinute(startMinute) || !validMinute(endMinute) || startMinute == endMinute {
		return ErrInvalidWindow
	}
	types := slices.Clone(lockTypes)
	slices.Sort(types)
	types = slices.Compact(types)
	types = slices.DeleteFunc(types, func(s string) bool { return s == "" })
	if len(types) == 0 {
		return ErrNoLockTypes
	}
	if err := chats.EnsureChatInDb(chatID, ""); err != nil {
		return err
	}

	// A map, not the struct: GORM drops zero values on insert, and 00:00 is
	// minute 0.
	now := time.Now()
	row := map[string]any{
		"chat_id":      chatID,
		"enabled":      true,
		"start_minute": startMinute,
		"end_minute":   endMinute,
		"lock_types":   strings.Join(types, ","),
		"created_at":   now,
		"updated_at":   now,
	}
	err := db.DB.Model(&models.NightModeSettings{}).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"enabled", "start_minute", "end_minute", "lock_types", "updated_at"}),
	}).Create(row).Error
	if err != nil {
		return err
	}
	cache.DeleteCache(cacheKey(chatID))
	return nil
}

// Disable turns night mode off, keeping the window and types. A chat without
// a row stays without one.
func Disable(chatID int64) error {
	err := db.DB.Model(&models.NightModeSettings{}).
		Where("chat_id = ?", chatID).
		Updates(map[string]any{"enabled": false, "updated_at": time.Now()}).Error
	if err != nil {
		return err
	}
	cache.DeleteCache(cacheKey(chatID))
	return nil
}

// ActiveAt reports whether night mode applies at local, a time already in the
// chat's timezone.
func ActiveAt(settings *models.NightModeSettings, local time.Time) bool {
	if settings == nil || !settings.Enabled || settings.LockTypes == "" {
		return false
	}
	return InWindow(settings.StartMinute, settings.EndMinute, local)
}

// InWindow reports whether local's wall-clock time falls in [start, end),
// both in minutes after midnight. A window whose end is before its start
// crosses midnight.
//
// It reads the wall clock, so DST needs no special case: when the clock
// skips a start or end time (02:30 on a spring-forward night), the change
// takes effect at the first minute after the gap (03:00); in a repeated hour
// the window follows the clock both times through.
func InWindow(start, end int, local time.Time) bool {
	minute := local.Hour()*60 + local.Minute()
	switch {
	case start < end:
		return minute >= start && minute < end
	case start > end:
		return minute >= start || minute < end
	default:
		return false
	}
}

func validMinute(m int) bool {
	return m >= 0 && m < MinutesPerDay
}
