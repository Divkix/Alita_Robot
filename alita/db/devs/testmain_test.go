package devs

import (
	"os"
	"testing"

	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/internal/testdb"
)

func TestMain(m *testing.M) {
	os.Exit(testdb.Run(m,
		&models.DevSettings{},
		&models.PinSettings{},
		&models.ReportChatSettings{},
		&models.ReportUserSettings{},
		&models.AntifloodSettings{},
		&models.RulesSettings{},
		&models.BlacklistSettings{},
		&models.ConnectionSettings{},
		&models.ConnectionChatSettings{},
		&models.DisableSettings{},
		&models.DisableChatSettings{},
		&models.ChatFilters{},
		&models.GreetingSettings{},
		&models.Notes{},
		&models.ChannelSettings{},
		&models.Federation{},
		&models.FederationAdmin{},
		&models.FederationChat{},
		&models.FederationBan{},
		&models.FederationSub{},
		&models.CaptchaSettings{},
		&models.CaptchaAttempts{},
		&models.CaptchaMutedUsers{},
		&models.ApprovedUsers{},
		&models.Warns{},
		&models.WarnEntry{},
		&models.WarnSettings{},
		&models.LockSettings{},
		&models.AntiRaidSettings{},
		&models.LogChannel{},
		&models.Reactions{},
		&models.AISpamSettings{},
	))
}
