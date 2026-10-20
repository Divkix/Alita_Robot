package backup

import (
	"os"
	"testing"

	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/internal/testdb"
)

func TestMain(m *testing.M) {
	os.Exit(testdb.Run(m,
		&models.WarnSettings{},
		&models.Warns{},
		&models.WarnEntry{},
		&models.GreetingSettings{},
		&models.ChatFilters{},
		&models.AdminSettings{},
		&models.BlacklistSettings{},
		&models.PinSettings{},
		&models.ReportChatSettings{},
		&models.ReportUserSettings{},
		&models.DevSettings{},
		&models.ChannelSettings{},
		&models.AntifloodSettings{},
		&models.ConnectionSettings{},
		&models.ConnectionChatSettings{},
		&models.DisableSettings{},
		&models.DisableChatSettings{},
		&models.RulesSettings{},
		&models.LockSettings{},
		&models.NotesSettings{},
		&models.Notes{},
		&models.CaptchaSettings{},
		&models.CaptchaAttempts{},
		&models.StoredMessages{},
		&models.CaptchaMutedUsers{},
		&models.ApprovedUsers{},
		&models.AntiRaidSettings{},
		&models.Reactions{},
		&models.Federation{},
		&models.FederationAdmin{},
		&models.FederationChat{},
		&models.FederationBan{},
		&models.FederationSub{},
		&models.LogChannel{},
	))
}
