package settings

import (
	"context"
	"fmt"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db/chattz"
	"github.com/divkix/Alita_Robot/alita/db/digestsettings"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/db/logchannels"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/reports"
	"github.com/divkix/Alita_Robot/alita/db/rules"
)

var ReportsEnabled = &Declaration{
	Key: "reports.enabled", Type: Switch, Default: true, Right: AdminBot,
	read: func(ctx context.Context, chatID int64) any {
		row := reports.GetChatReportSettings(chatID)
		if row.ID == 0 {
			return nil
		}
		return row.Status
	},
	write: func(input WriteContext, value any) error {
		return reports.SetChatReportStatus(input.ChatID, value.(bool))
	},
}

var ReportMuteThreshold = &Declaration{
	Key: "reports.mute_threshold", Type: Number, Default: 0, Right: Restrict,
	read: func(ctx context.Context, chatID int64) any {
		return reports.GetChatReportSettings(chatID).MuteThreshold
	},
	allowed: func(value any) error {
		if n := value.(int); n != 0 && (n < models.ReportMuteThresholdMin || n > models.ReportMuteThresholdMax) {
			return fmt.Errorf("must be 0 or between %d and %d", models.ReportMuteThresholdMin, models.ReportMuteThresholdMax)
		}
		return nil
	},
	write: func(input WriteContext, value any) error {
		return reports.SetChatReportMuteThreshold(input.ChatID, value.(int))
	},
}

// ReadReports normalizes a copy, leaving cached rows and persistence untouched.
func ReadReports(chatID int64) *models.ReportChatSettings {
	row := *reports.GetChatReportSettings(chatID)
	if row.ID == 0 {
		row.Status = ReportsEnabled.Default.(bool)
		row.MuteThreshold = ReportMuteThreshold.Default.(int)
	} else {
		row.Status = ReportsEnabled.effective(row.Status).(bool)
		row.MuteThreshold = ReportMuteThreshold.effective(row.MuteThreshold).(int)
	}
	row.Enabled = row.Status
	return &row
}

var DigestEnabled = &Declaration{
	Key: "digest.enabled", Type: Switch, Default: false, Right: AdminBot,
	read: func(ctx context.Context, chatID int64) any {
		row, err := digestsettings.GetContext(ctx, chatID)
		if err != nil || row == nil {
			return nil
		}
		return row.Enabled
	},
	precondition: digestAvailable,
	write: func(input WriteContext, value any) error {
		return digestsettings.SetEnabled(input.ChatID, value.(bool))
	},
}

var DigestDestination = &Declaration{
	Key: "digest.destination", Type: Choice, Default: digestsettings.DestinationAuto, Right: AdminBot,
	Options: []string{digestsettings.DestinationAuto, digestsettings.DestinationLog, digestsettings.DestinationGroup},
	read: func(ctx context.Context, chatID int64) any {
		row, err := digestsettings.GetContext(ctx, chatID)
		if err != nil || row == nil {
			return nil
		}
		return row.Destination
	},
	precondition: func(input WriteContext, value any) error {
		if err := digestAvailable(input, value); err != nil {
			return err
		}
		if value.(string) == digestsettings.DestinationLog {
			row := logchannels.GetContext(input.Context, input.ChatID)
			if row == nil || row.LogChannelID == 0 {
				return Refusal("digest_log_unavailable")
			}
		}
		return nil
	},
	write: func(input WriteContext, value any) error {
		return digestsettings.SetDestination(input.ChatID, value.(string))
	},
}

func digestAvailable(_ WriteContext, _ any) error {
	if config.AppConfig == nil || !config.AppConfig.EnableDigest {
		return Refusal("digest_globally_disabled")
	}
	return nil
}

// ReadDigest applies allowed-value defaults without applying write preconditions.
func ReadDigestContext(ctx context.Context, chatID int64) (*models.DigestSettings, error) {
	stored, err := digestsettings.GetContext(ctx, chatID)
	if err != nil {
		return nil, err
	}
	row := *stored
	row.Enabled = DigestEnabled.effective(row.Enabled).(bool)
	row.Destination = DigestDestination.effective(row.Destination).(string)
	return &row, nil
}

var Timezone = &Declaration{
	Key: "timezone.name", Type: Text, Default: chattz.DefaultName, Right: ChangeInfo,
	read: func(ctx context.Context, chatID int64) any {
		name, err := chattz.NameContext(ctx, chatID)
		if err != nil {
			return nil
		}
		return name
	},
	allowed: func(value any) error {
		_, _, err := chattz.Parse(value.(string))
		return err
	},
	write: func(input WriteContext, value any) error {
		_, err := chattz.Set(input.ChatID, value.(string))
		return err
	},
}

// ReadTimezone preserves repository errors while normalizing invalid stored names.
func ReadTimezoneContext(ctx context.Context, chatID int64) (string, error) {
	name, err := chattz.NameContext(ctx, chatID)
	if err != nil {
		return "", err
	}
	return Timezone.effective(name).(string), nil
}

var Language = &Declaration{
	Key: "language.code", Type: Choice, Default: "en", Right: Admin,
	Options: []string{"en", "es", "fr", "hi", "ru", "pt", "id"},
	read:    func(ctx context.Context, chatID int64) any { return lang.GetGroupLanguage(ctx, chatID) },
	write: func(input WriteContext, value any) error {
		return lang.ChangeGroupLanguage(input.ChatID, value.(string))
	},
}

var PrivateRules = &Declaration{
	Key: "rules.private_rules", Type: Switch, Default: false, Right: AdminBot,
	read: func(ctx context.Context, chatID int64) any { return rules.GetChatRulesInfo(chatID).Private },
	write: func(input WriteContext, value any) error {
		return rules.SetPrivateRules(input.ChatID, value.(bool))
	},
}
