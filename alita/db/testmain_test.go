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
	var dbFileName string
	if DB == nil {
		dbFile, err := os.CreateTemp("", "alita_test_*.db")
		if err != nil {
			fmt.Printf("temp file creation failed: %v\n", err)
			os.Exit(1)
		}
		dbFileName = dbFile.Name()
		if closeErr := dbFile.Close(); closeErr != nil {
			fmt.Printf("temp file close failed: %v\n", closeErr)
			os.Exit(1)
		}
		dbPath := dbFileName + "?_busy_timeout=10000&_journal_mode=WAL"
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
		&ApprovedUsers{},
		&AntiRaidSettings{},
		&Federation{},
		&FederationAdmin{},
		&FederationChat{},
		&FederationBan{},
		&FederationSubscription{},
		&FederationSettings{},
	)
	if err != nil {
		fmt.Printf("AutoMigrate failed: %v\n", err)
		os.Exit(1)
	}

	exitCode := m.Run()

	// Close DB handle before removing temp file.
	if DB != nil {
		sqlDB, err := DB.DB()
		if err != nil {
			fmt.Printf("failed to get underlying DB: %v\n", err)
		} else if closeErr := sqlDB.Close(); closeErr != nil {
			fmt.Printf("DB close failed: %v\n", closeErr)
		}
	}

	// Remove temp file before exit.
	if dbFileName != "" {
		if rmErr := os.Remove(dbFileName); rmErr != nil {
			fmt.Printf("temp file remove failed: %v\n", rmErr)
		}
	}

	os.Exit(exitCode)
}

func skipIfNoDb(t *testing.T) {
	t.Helper()
	if DB == nil {
		t.Skip("requires database connection")
	}
}
