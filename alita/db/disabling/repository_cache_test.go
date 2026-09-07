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

func setupDisablingCacheDB(t *testing.T) {
	t.Helper()
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

}

func TestGetChatDisabledCMDsCachedDoesNotCacheDatabaseErrors(t *testing.T) {
	setupDisablingCacheDB(t)
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

func TestDisableCMDFlipsImportedFalseRow(t *testing.T) {
	setupDisablingCacheDB(t)
	if err := db.DB.AutoMigrate(&models.DisableSettings{}); err != nil {
		t.Fatalf("AutoMigrate DisableSettings: %v", err)
	}

	chatID := time.Now().UnixNano() + 9000
	cmd := "ban"

	// Backup imports preserve disabled=false rows because replaceChatRows
	// inserts maps (all keys included), so seed one the same way. A struct
	// Create would silently apply the column default of true instead.
	if err := db.DB.Model(&models.DisableSettings{}).Create(map[string]any{
		"chat_id":  chatID,
		"command":  cmd,
		"disabled": false,
	}).Error; err != nil {
		t.Fatalf("seed disabled=false row error = %v", err)
	}
	if IsCommandDisabled(chatID, cmd) {
		t.Fatal("IsCommandDisabled() = true before DisableCMD, want false")
	}

	if err := DisableCMD(chatID, cmd); err != nil {
		t.Fatalf("DisableCMD() error = %v", err)
	}

	var row models.DisableSettings
	if err := db.DB.Where("chat_id = ? AND command = ?", chatID, cmd).Take(&row).Error; err != nil {
		t.Fatalf("read back DisableSettings error = %v", err)
	}
	if !row.Disabled {
		t.Fatal("DisableSettings.Disabled = false after DisableCMD, want true")
	}
	if !IsCommandDisabled(chatID, cmd) {
		t.Fatal("IsCommandDisabled() = false after DisableCMD, want true")
	}
}
