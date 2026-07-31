package connections

import (
	"fmt"
	"os"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

func TestMain(m *testing.M) {
	var dbFileName string
	if db.DB == nil {
		dbFile, err := os.CreateTemp("", "alita_connections_test_*.db")
		if err != nil {
			fmt.Printf("temp file creation failed: %v\n", err)
			os.Exit(1)
		}
		dbFileName = dbFile.Name()
		if err := dbFile.Close(); err != nil {
			fmt.Printf("temp file close failed: %v\n", err)
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
		sqlDB, err := sqliteDB.DB()
		if err != nil {
			fmt.Printf("SQLite handle failed: %v\n", err)
			os.Exit(1)
		}
		sqlDB.SetMaxOpenConns(1)
		db.DB = sqliteDB

		if err := db.DB.AutoMigrate(
			&models.User{},
			&models.Chat{},
			&models.ConnectionSettings{},
			&models.ConnectionChatSettings{},
		); err != nil {
			fmt.Printf("AutoMigrate failed: %v\n", err)
			os.Exit(1)
		}
	}

	exitCode := m.Run()
	if dbFileName != "" {
		if sqlDB, err := db.DB.DB(); err == nil {
			_ = sqlDB.Close()
		}
		_ = os.Remove(dbFileName)
	}
	os.Exit(exitCode)
}
