//go:build testtools

package digest

import (
	"os"
	"testing"

	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/internal/testdb"
)

func TestMain(m *testing.M) {
	os.Exit(testdb.Run(m,
		&models.DigestSettings{},
		&models.ChatDailyStat{},
		&models.ChatHourlyStat{},
		&models.ChatMemberDailyStat{},
		&models.ChatMemberCount{},
		&models.ModerationEvent{},
		&models.LogChannel{},
	))
}
