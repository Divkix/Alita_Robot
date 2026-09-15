package testdb

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

// Run gives each repository test process an isolated database unless PostgreSQL was explicitly enabled.
func Run(m *testing.M, tables ...any) int {
	if db.DB != nil {
		return m.Run()
	}
	dir, err := os.MkdirTemp("", "alita-repository-tests-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}()
	database, err := gorm.Open(sqlite.Open(dir+"/database.db?_busy_timeout=10000&_journal_mode=WAL"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	sqlDB, err := database.DB()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer func() {
		if err := sqlDB.Close(); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}()
	tables = append([]any{&models.User{}, &models.Chat{}}, tables...)
	if err := database.AutoMigrate(tables...); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	db.DB = database
	defer func() { db.DB = nil }()
	return m.Run()
}
