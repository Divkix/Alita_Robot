package settings

import (
	"github.com/divkix/Alita_Robot/alita/db/greetings"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

var Welcome = greetingSwitch("greetings.welcome_enabled", true, Admin,
	func(row *models.GreetingSettings) bool { return row.WelcomeSettings.ShouldWelcome }, greetings.SetWelcomeToggle)

var Goodbye = greetingSwitch("greetings.goodbye_enabled", true, Admin,
	func(row *models.GreetingSettings) bool { return row.GoodbyeSettings.ShouldGoodbye }, greetings.SetGoodbyeToggle)

var CleanWelcome = greetingSwitch("greetings.clean_welcome", false, ChangeInfo,
	func(row *models.GreetingSettings) bool { return row.WelcomeSettings.CleanWelcome }, greetings.SetCleanWelcomeSetting)

var CleanGoodbye = greetingSwitch("greetings.clean_goodbye", false, ChangeInfo,
	func(row *models.GreetingSettings) bool { return row.GoodbyeSettings.CleanGoodbye }, greetings.SetCleanGoodbyeSetting)

var CleanService = greetingSwitch("greetings.clean_service", false, ChangeInfoBot,
	func(row *models.GreetingSettings) bool { return row.ShouldCleanService }, greetings.SetShouldCleanService)

var AutoApprove = greetingSwitch("greetings.auto_approve", false, ChangeInfoBot,
	func(row *models.GreetingSettings) bool { return row.ShouldAutoApprove }, greetings.SetShouldAutoApprove)

func greetingSwitch(key string, def bool, right Right, read func(*models.GreetingSettings) bool, write func(int64, bool) error) *Declaration {
	return &Declaration{
		Key: key, Type: Switch, Default: def, Right: right,
		read: func(chatID int64) any {
			row := greetings.GetGreetingSettings(chatID)
			if row.ID == 0 {
				return nil
			}
			return read(row)
		},
		write: func(input WriteContext, value any) error { return write(input.ChatID, value.(bool)) },
	}
}

// ReadGreetings returns a copied, effective snapshot for status and action paths.
// Missing embedded settings do not discard unrelated stored preferences.
func ReadGreetings(chatID int64) models.GreetingSettings {
	row := *greetings.GetGreetingSettings(chatID)
	welcome, goodbye := *row.WelcomeSettings, *row.GoodbyeSettings
	if row.ID == 0 {
		welcome.ShouldWelcome = Welcome.Default.(bool)
		welcome.CleanWelcome = CleanWelcome.Default.(bool)
		goodbye.ShouldGoodbye = Goodbye.Default.(bool)
		goodbye.CleanGoodbye = CleanGoodbye.Default.(bool)
		row.ShouldCleanService = CleanService.Default.(bool)
		row.ShouldAutoApprove = AutoApprove.Default.(bool)
	}
	row.WelcomeSettings, row.GoodbyeSettings = &welcome, &goodbye
	return row
}
