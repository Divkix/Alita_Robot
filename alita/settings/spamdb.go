package settings

import (
	"context"
	"errors"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/spamdbsettings"
)

var SpamDBEnabled = func() *Declaration {
	d := spamDBDeclaration("antispam.spamdb_enabled", Switch, false,
		func(row *models.SpamDBSettings) any { return row.Enabled },
		func(input WriteContext, value any) error {
			return spamdbsettings.SetSpamDBEnabled(input.ChatID, value.(bool))
		})
	d.precondition = func(_ WriteContext, value any) error {
		if value.(bool) && (config.AppConfig == nil || !config.AppConfig.EnableSpamDB) {
			return Refusal("spamdb_globally_disabled")
		}
		return nil
	}
	return d
}()

var SpamDBAction = func() *Declaration {
	d := spamDBDeclaration("antispam.spamdb_action", Choice, spamdbsettings.ActionBan,
		func(row *models.SpamDBSettings) any { return row.Action },
		func(input WriteContext, value any) error {
			return spamdbsettings.SetSpamDBAction(input.ChatID, value.(string))
		})
	d.Options = []string{spamdbsettings.ActionBan, spamdbsettings.ActionMute, spamdbsettings.ActionLog}
	return d
}()

var SpamDBCAS, SpamDBLols, SpamDBShared = spamDatabases()

// ErrLastSpamDatabase retains the panel's refusal while commands add the selected
// database's localized name. The repository constraint also protects concurrent writes.
var ErrLastSpamDatabase = errors.New("at least one spam database must stay on")

func spamDBDeclaration(key string, kind Type, def any, read func(*models.SpamDBSettings) any, write func(WriteContext, any) error) *Declaration {
	return &Declaration{
		Key: key, Type: kind, Default: def, Right: Admin,
		read:  storedValue(spamdbsettings.GetSpamDBSettingsContext, read),
		write: write,
	}
}

func spamDatabases() (*Declaration, *Declaration, *Declaration) {
	database := func(name string, def bool, read func(*models.SpamDBSettings) any) *Declaration {
		return spamDBDeclaration("antispam.spamdb_"+name, Switch, def, read,
			func(input WriteContext, value any) error {
				err := spamdbsettings.SetSpamDBDatabase(input.ChatID, name, value.(bool))
				if errors.Is(err, spamdbsettings.ErrLastDatabase) {
					return ErrLastSpamDatabase
				}
				return err
			})
	}
	cas := database(spamdbsettings.DatabaseCAS, true, func(row *models.SpamDBSettings) any {
		return row.CheckCAS || (!row.CheckLols && !row.CheckShared)
	})
	lols := database(spamdbsettings.DatabaseLols, false, func(row *models.SpamDBSettings) any { return row.CheckLols })
	shared := database(spamdbsettings.DatabaseShared, false, func(row *models.SpamDBSettings) any { return row.CheckShared })
	shared.precondition = func(_ WriteContext, value any) error {
		if value.(bool) && (config.AppConfig == nil || !config.AppConfig.EnableReputation) {
			return Refusal("spamdb_db_shared_disabled")
		}
		return nil
	}
	group := &sharedWrite{
		fields: []*Declaration{cas, lols, shared},
		validate: func(input WriteContext, changes map[string]any) error {
			row, err := spamdbsettings.GetSpamDBSettingsContext(input.Context, input.ChatID)
			if err != nil {
				return err
			}
			values := [3]bool{row.CheckCAS || (!row.CheckLols && !row.CheckShared), row.CheckLols, row.CheckShared}
			for i, field := range []*Declaration{cas, lols, shared} {
				if value, changed := changes[field.Key]; changed {
					values[i] = value.(bool)
				}
			}
			if !values[0] && !values[1] && !values[2] {
				return ErrLastSpamDatabase
			}
			return nil
		},
	}
	cas.shared, lols.shared, shared.shared = group, group, group
	return cas, lols, shared
}

// ReadSpamDBContext normalizes a copied cached row without inserting or repairing it.
func ReadSpamDBContext(ctx context.Context, chatID int64) (*models.SpamDBSettings, error) {
	stored, err := spamdbsettings.GetSpamDBSettingsContext(ctx, chatID)
	if err != nil || stored == nil {
		return nil, err
	}
	row := *stored
	row.Action = SpamDBAction.effective(row.Action).(string)
	if !row.CheckCAS && !row.CheckLols && !row.CheckShared {
		row.CheckCAS = SpamDBCAS.Default.(bool)
	}
	return &row, nil
}
