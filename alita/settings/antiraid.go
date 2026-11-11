package settings

import (
	"fmt"
	"math"

	"github.com/divkix/Alita_Robot/alita/db/antiraid"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

const maxRaidDuration = 366 * 24 * 60 * 60

var RaidTime = &Declaration{
	Key: "antiraid.raid_time", Type: Duration, Default: 21600, Right: AdminBot,
	read: func(chatID int64) any {
		row := antiraid.GetAntiRaidSettings(chatID)
		if row.ID == 0 {
			return nil
		}
		return row.RaidTime
	},
	allowed: func(value any) error {
		if n := value.(int); n < 1 || n > maxRaidDuration {
			return fmt.Errorf("must be between 1 and %d seconds", maxRaidDuration)
		}
		return nil
	},
	write: func(input WriteContext, value any) error { return antiraid.SetRaidTime(input.ChatID, value.(int)) },
}

var RaidActionTime = &Declaration{
	Key: "antiraid.raid_action_time", Type: Duration, Default: 3600, Right: AdminBot,
	read: func(chatID int64) any {
		row := antiraid.GetAntiRaidSettings(chatID)
		if row.ID == 0 {
			return nil
		}
		return row.RaidActionTime
	},
	allowed: func(value any) error {
		if n := value.(int); n < 30 || n > maxRaidDuration {
			return fmt.Errorf("must be between 30 and %d seconds", maxRaidDuration)
		}
		return nil
	},
	write: func(input WriteContext, value any) error {
		return antiraid.SetRaidActionTime(input.ChatID, value.(int))
	},
}

var AutoAntiRaidThreshold = &Declaration{
	Key: "antiraid.auto_threshold", Type: Number, Default: 0, Right: AdminBot,
	read: func(chatID int64) any {
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
