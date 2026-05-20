package chat_status

import (
	"fmt"
	"os"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/divkix/Alita_Robot/alita/db"
)

func TestMain(m *testing.M) {
	var dbFileName string
	if db.DB == nil {
		dbFile, err := os.CreateTemp("", "alita_chat_status_test_*.db")
		if err != nil {
			fmt.Printf("temp file creation failed: %v\n", err)
			os.Exit(1)
		}
		dbFileName = dbFile.Name()
		if closeErr := dbFile.Close(); closeErr != nil {
			fmt.Printf("temp file close failed: %v\n", closeErr)
			os.Exit(1)
		}

		sqliteDB, err := gorm.Open(
			sqlite.Open(dbFileName+"?_busy_timeout=10000&_journal_mode=WAL"),
			&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
		)
		if err != nil {
			fmt.Printf("SQLite init failed: %v\n", err)
			os.Exit(1)
		}
		db.DB = sqliteDB
	}

	if err := db.DB.AutoMigrate(
		&db.AdminSettings{},
		&db.ApprovedUsers{},
		&db.DisableSettings{},
		&db.DisableChatSettings{},
	); err != nil {
		fmt.Printf("AutoMigrate failed: %v\n", err)
		os.Exit(1)
	}

	exitCode := m.Run()

	if db.DB != nil {
		if sqlDB, err := db.DB.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}
	if dbFileName != "" {
		_ = os.Remove(dbFileName)
	}
	os.Exit(exitCode)
}
