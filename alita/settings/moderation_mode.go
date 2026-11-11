package settings

import (
	"context"
	"fmt"

	"github.com/divkix/Alita_Robot/alita/db/antiflood"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/warns"
)

var FloodMode, FloodActionTime = moderationMode("antiflood.action", "antiflood.action_time",
	func(ctx context.Context, chatID int64) (string, int) {
		row := antiflood.GetFloodContext(ctx, chatID)
		return row.Action, row.FloodActionTime
	}, func(input WriteContext, mode string, seconds int) error {
		return antiflood.SetFloodModeContext(input.Context, input.ChatID, mode, seconds)
	})

var WarnMode, WarnActionTime = moderationMode("warns.mode", "warns.action_time",
	func(ctx context.Context, chatID int64) (string, int) {
		row := warns.GetWarnSettingContext(ctx, chatID)
		return row.WarnMode, row.WarnActionTime
	}, func(input WriteContext, mode string, seconds int) error {
		return warns.SetWarnMode(input.ChatID, mode, seconds)
	})

// IsTimedMode reports whether moderation needs a duration supplied by the user.
func IsTimedMode(mode string) bool {
	return mode == "tban" || mode == "tmute"
}

func effectiveModeDuration(mode string, seconds, def int) int {
	if !IsTimedMode(mode) {
		return 0
	}
	if !models.ValidActionTime(seconds) {
		return def
	}
	return seconds
}

func moderationMode(modeKey, timeKey string, read func(context.Context, int64) (string, int), write func(WriteContext, string, int) error) (*Declaration, *Declaration) {
	mode := &Declaration{
		Key: modeKey, Type: Choice, Default: "mute", Right: AdminBot,
		Options:      []string{"ban", "kick", "mute", "tban", "tmute"},
		toggleValues: []string{"mute", "ban", "kick"},
		read: func(ctx context.Context, chatID int64) any {
			value, _ := read(ctx, chatID)
			return value
		},
	}
	duration := &Declaration{
		Key: timeKey, Type: Duration, Default: 3600, Right: AdminBot,
		allowed: func(value any) error {
			if n := value.(int); n != 0 && !models.ValidActionTime(n) {
				return fmt.Errorf("must be 0 (unset) or between 60 and 31622400 seconds")
			}
			return nil
		},
	}
	// Zero means inactive for an untimed mode, not a customized duration.
	duration.unchanged = func(value any) bool { return value == 0 || value == duration.Default }
	duration.read = func(ctx context.Context, chatID int64) any {
		storedMode, seconds := read(ctx, chatID)
		return effectiveModeDuration(mode.effective(storedMode).(string), seconds, duration.Default.(int))
	}
	values := func(input WriteContext, changes map[string]any) (string, int, error) {
		storedMode, seconds := read(input.Context, input.ChatID)
		selected := mode.effective(storedMode).(string)
		seconds = effectiveModeDuration(selected, seconds, duration.Default.(int))
		if value, changed := changes[mode.Key]; changed {
			selected = value.(string)
		}
		if value, changed := changes[duration.Key]; changed {
			seconds = value.(int)
		} else if !IsTimedMode(selected) {
			seconds = 0
		}
		if IsTimedMode(selected) {
			if !models.ValidActionTime(seconds) {
				return "", 0, fmt.Errorf("a timed action needs between 60 and 31622400 seconds")
			}
		} else if seconds != 0 {
			return "", 0, fmt.Errorf("action duration is only available for tban and tmute")
		}
		return selected, seconds, nil
	}
	group := &sharedWrite{
		fields: []*Declaration{mode, duration},
		validate: func(input WriteContext, changes map[string]any) error {
			_, _, err := values(input, changes)
			return err
		},
		write: func(input WriteContext, changes map[string]any) error {
			selected, seconds, err := values(input, changes)
			if err != nil {
				return err
			}
			return write(input, selected, seconds)
		},
	}
	mode.shared, duration.shared = group, group
	return mode, duration
}
