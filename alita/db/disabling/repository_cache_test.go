//go:build testtools

package disabling

import (
	"fmt"
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/models"
	utilsCache "github.com/divkix/Alita_Robot/alita/utils/cache"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestGetChatDisabledCMDsCachedDoesNotCacheDatabaseErrors(t *testing.T) {
	originalDB := db.DB
	testDB, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:disabling-%d?mode=memory&cache=shared", time.Now().UnixNano())),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
	)
	if err != nil {
		t.Fatalf("open SQLite: %v", err)
	}
	sqlDB, err := testDB.DB()
	if err != nil {
		t.Fatalf("get SQLite handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	db.DB = testDB
	utilsCache.SetupTestMemoryMarshaler(t)
	t.Cleanup(func() {
		_ = sqlDB.Close()
		db.DB = originalDB
	})

	const chatID = int64(-100123)
	if got := GetChatDisabledCMDsCached(chatID); len(got) != 0 {
		t.Fatalf("disabled commands with missing table = %v, want empty", got)
	}

	if err := db.DB.AutoMigrate(&models.DisableSettings{}); err != nil {
		t.Fatalf("AutoMigrate DisableSettings: %v", err)
	}
	if err := db.DB.Create(&models.DisableSettings{
		ChatId:   chatID,
		Command:  "rules",
		Disabled: true,
	}).Error; err != nil {
		t.Fatalf("insert disabled command: %v", err)
	}

	got := GetChatDisabledCMDsCached(chatID)
	if len(got) != 1 || got[0] != "rules" {
		t.Fatalf("disabled commands after DB recovery = %v, want [rules]", got)
	}
}
