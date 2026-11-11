package settings

import (
	"context"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db/aispam"
)

var AISpamEnabled = &Declaration{
	Key: "antispam.aispam_enabled", Type: Switch, Default: false, Right: Admin,
	read: func(ctx context.Context, chatID int64) any { return aispam.IsAISpamEnabledContext(ctx, chatID) },
	precondition: func(_ WriteContext, value any) error {
		if !value.(bool) {
			return nil
		}
		if config.AppConfig == nil || config.AppConfig.TypeSafeAPIKey == "" {
			return Refusal("aispam_no_key")
		}
		if !config.AppConfig.EnableAISpam {
			return Refusal("aispam_globally_disabled")
		}
		return nil
	},
	write: func(input WriteContext, value any) error { return aispam.SetAISpamEnabled(input.ChatID, value.(bool)) },
}
