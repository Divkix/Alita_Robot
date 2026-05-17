package db

import (
	"fmt"
	"os"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestMain(m *testing.M) {
	if DB == nil {
		dbPath := os.TempDir() + "/alita_test.db?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)"
		sqliteDB, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		if err != nil {
			fmt.Printf("SQLite init failed: %v\n", err)
			os.Exit(1)
		}
		DB = sqliteDB
	}

	err := DB.AutoMigrate(
		&User{},
		&Chat{},
		&WarnSettings{},
		&Warns{},
		&GreetingSettings{},
		&ChatFilters{},
		&AdminSettings{},
		&BlacklistSettings{},
		&PinSettings{},
		&ReportChatSettings{},
		&ReportUserSettings{},
		&DevSettings{},
		&ChannelSettings{},
		&AntifloodSettings{},
		&ConnectionSettings{},
		&ConnectionChatSettings{},
		&DisableSettings{},
		&DisableChatSettings{},
		&RulesSettings{},
		&LockSettings{},
		&NotesSettings{},
		&Notes{},
		&CaptchaSettings{},
		&CaptchaAttempts{},
		&StoredMessages{},
		&CaptchaMutedUsers{},
	)
	if err != nil {
		fmt.Printf("AutoMigrate failed: %v\n", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

func skipIfNoDb(t *testing.T) {
	t.Helper()
	if DB == nil {
		t.Skip("requires database connection")
	}
}
