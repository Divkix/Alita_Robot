package settings

import (
	"context"
	"fmt"
	"math"

	"github.com/divkix/Alita_Robot/alita/db/antiraid"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/utils/punishment/window"
)

var RaidTime = raidDuration("antiraid.raid_time", 21600, 1,
	func(row *models.AntiRaidSettings) int { return row.RaidTime },
	antiraid.SetRaidTime,
)

var RaidActionTime = raidDuration("antiraid.raid_action_time", 3600, window.MinSeconds,
	func(row *models.AntiRaidSettings) int { return row.RaidActionTime },
	antiraid.SetRaidActionTime,
)

func raidDuration(key string, def, minSeconds int, read func(*models.AntiRaidSettings) int, write func(int64, int) error) *Declaration {
	return &Declaration{
		Key: key, Type: Duration, Default: def, Right: AdminBot,
		read: func(ctx context.Context, chatID int64) any {
			row := antiraid.GetAntiRaidSettings(chatID)
			if row.ID == 0 {
				return nil
			}
			return read(row)
		},
		allowed: func(value any) error {
			if n := value.(int); n < minSeconds || n > window.MaxSeconds {
				return fmt.Errorf("must be between %d and %d seconds", minSeconds, window.MaxSeconds)
			}
			return nil
		},
		write: func(input WriteContext, value any) error { return write(input.ChatID, value.(int)) },
	}
}

var AutoAntiRaidThreshold = &Declaration{
	Key: "antiraid.auto_threshold", Type: Number, Default: 0, Right: AdminBot,
	read: func(ctx context.Context, chatID int64) any {
		row := antiraid.GetAntiRaidSettings(chatID)
		if row.ID == 0 {
			return nil
		}
		return row.AutoAntiRaidThreshold
	},
	allowed: func(value any) error {
		if n := value.(int); n < 0 || n > math.MaxInt32 {
			return fmt.Errorf("must be between 0 and %d", math.MaxInt32)
		}
		return nil
	},
	write: func(input WriteContext, value any) error {
		return antiraid.SetAutoAntiRaidThreshold(input.ChatID, value.(int))
	},
}

// ReadAntiRaid returns an effective snapshot for raid status and action paths.
// Copy the cached row before normalization; reads never modify stored values.
func ReadAntiRaid(chatID int64) models.AntiRaidSettings {
	row := *antiraid.GetAntiRaidSettings(chatID)
	if row.ID == 0 {
		row.RaidTime = RaidTime.Default.(int)
		row.RaidActionTime = RaidActionTime.Default.(int)
		row.AutoAntiRaidThreshold = AutoAntiRaidThreshold.Default.(int)
	} else {
		row.RaidTime = RaidTime.effective(row.RaidTime).(int)
		row.RaidActionTime = RaidActionTime.effective(row.RaidActionTime).(int)
		row.AutoAntiRaidThreshold = AutoAntiRaidThreshold.effective(row.AutoAntiRaidThreshold).(int)
	}
	return row
}
