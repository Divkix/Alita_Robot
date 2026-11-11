package settings

import (
	"context"
	"fmt"

	"github.com/divkix/Alita_Robot/alita/db/antiflood"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/warns"
)

var FloodLimit = &Declaration{
	Key: "antiflood.limit", Type: Number, Default: 0, Right: AdminBot,
	read: func(chatID int64) any { return antiflood.GetFlood(chatID).Limit },
	allowed: func(value any) error {
		if n := value.(int); n != 0 && (n < 3 || n > 100) {
			return fmt.Errorf("must be 0 (off) or between 3 and 100")
		}
		return nil
	},
	write: func(input WriteContext, value any) error { return antiflood.SetFlood(input.ChatID, value.(int)) },
}

var FloodDelete = &Declaration{
	Key: "antiflood.delete_messages", Type: Switch, Default: false, Right: AdminBot,
	read:  func(chatID int64) any { return antiflood.GetFlood(chatID).DeleteAntifloodMessage },
	write: func(input WriteContext, value any) error { return antiflood.SetFloodMsgDel(input.ChatID, value.(bool)) },
}

var WarnLimit = &Declaration{
	Key: "warns.limit", Type: Number, Default: 3, Right: AdminBot,
	read: func(chatID int64) any { return warns.GetWarnSetting(chatID).WarnLimit },
	allowed: func(value any) error {
		if n := value.(int); n < 1 || n > 100 {
			return fmt.Errorf("must be between 1 and 100")
		}
		return nil
	},
	write: func(input WriteContext, value any) error { return warns.SetWarnLimit(input.ChatID, value.(int)) },
}

var WarnExpiry = &Declaration{
	Key: "warns.expiry", Type: Duration, Default: 0, Right: AdminBot,
	read: func(chatID int64) any { return warns.GetWarnSetting(chatID).WarnExpiry },
	allowed: func(value any) error {
		if !models.ValidWarnExpiry(value.(int)) {
			return fmt.Errorf("must be 0 (never) or between 60 and 31622400 seconds")
		}
		return nil
	},
	write: func(input WriteContext, value any) error { return warns.SetWarnExpiry(input.ChatID, value.(int)) },
}

// ReadFlood returns a normalized copy without changing the cached or stored row.
func ReadFlood(chatID int64) *models.AntifloodSettings {
	return ReadFloodContext(context.Background(), chatID)
}

func ReadFloodContext(ctx context.Context, chatID int64) *models.AntifloodSettings {
	row := *antiflood.GetFloodContext(ctx, chatID)
	row.Limit = FloodLimit.effective(row.Limit).(int)
	row.Action = FloodMode.effective(row.Action).(string)
	row.FloodActionTime = effectiveModeDuration(row.Action, row.FloodActionTime, FloodActionTime.Default.(int))
	return &row
}

// ReadWarns returns a normalized copy without creating or repairing a row.
func ReadWarnsContext(ctx context.Context, chatID int64) *models.WarnSettings {
	row := *warns.GetWarnSettingContext(ctx, chatID)
	row.WarnLimit = WarnLimit.effective(row.WarnLimit).(int)
	row.WarnExpiry = WarnExpiry.effective(row.WarnExpiry).(int)
	row.WarnMode = WarnMode.effective(row.WarnMode).(string)
	row.WarnActionTime = effectiveModeDuration(row.WarnMode, row.WarnActionTime, WarnActionTime.Default.(int))
	return &row
}
