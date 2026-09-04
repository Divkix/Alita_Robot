package channels

import (
	"fmt"
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/models"
	utilsCache "github.com/divkix/Alita_Robot/alita/utils/cache"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func skipIfNoDb(t *testing.T) {
	t.Helper()
	if db.DB == nil {
		t.Skip("requires database connection")
	}
}

func withChannelSQLite(t *testing.T) {
	t.Helper()
	if db.DB != nil && db.DB.Name() == "postgres" {
		return
	}

	originalDB := db.DB
	originalMarshal := utilsCache.GetMarshal()
	testDB, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:channels-%d?mode=memory&cache=shared", time.Now().UnixNano())),
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
	utilsCache.SetMarshal(nil)
	if err := db.DB.AutoMigrate(&models.ChannelSettings{}); err != nil {
		t.Fatalf("AutoMigrate ChannelSettings: %v", err)
	}

	t.Cleanup(func() {
		_ = sqlDB.Close()
		db.DB = originalDB
		utilsCache.SetMarshal(originalMarshal)
	})
}

func TestEnsureChannelInDb(t *testing.T) {
	skipIfNoDb(t)

	channelID := -(time.Now().UnixNano() % 1_000_000_000_000)
	if channelID > 0 {
		channelID = -channelID
	}
	channelName := fmt.Sprintf("channel-%d", channelID)
	username := fmt.Sprintf("chan_%d", -channelID)

	err := UpdateChannel(channelID, channelName, username)
	if err != nil {
		t.Fatalf("UpdateChannel() error = %v", err)
	}
	t.Cleanup(func() { db.DB.Where("chat_id = ?", channelID).Delete(&models.ChannelSettings{}) })

	var ch models.ChannelSettings
	if err := db.DB.Where("chat_id = ?", channelID).First(&ch).Error; err != nil {
		t.Fatalf("expected channel %d to exist: %v", channelID, err)
	}
	if ch.ChannelName != channelName {
		t.Errorf("channel name = %q, want %q", ch.ChannelName, channelName)
	}
}

func TestGetChannelIdByUserName(t *testing.T) {
	skipIfNoDb(t)

	channelID := -(time.Now().UnixNano()%1_000_000_000_000 + 2000)
	if channelID > 0 {
		channelID = -channelID
	}
	username := fmt.Sprintf("chanun_%d", -channelID)

	if err := UpdateChannel(channelID, "test-channel", username); err != nil {
		t.Fatalf("UpdateChannel() error = %v", err)
	}
	t.Cleanup(func() { db.DB.Where("chat_id = ?", channelID).Delete(&models.ChannelSettings{}) })

	gotID := GetChannelIdByUserName(username)
	if gotID != channelID {
		t.Errorf("GetChannelIdByUserName(%q) = %d, want %d", username, gotID, channelID)
	}
}

func TestGetChannelIdByUserName_NotFound(t *testing.T) {
	skipIfNoDb(t)

	gotID := GetChannelIdByUserName("nonexistent_channel_xyzabc999")
	if gotID != 0 {
		t.Errorf("GetChannelIdByUserName() = %d for non-existent channel, want 0", gotID)
	}
}

func TestGetChannelIdByUserName_Empty(t *testing.T) {
	skipIfNoDb(t)

	gotID := GetChannelIdByUserName("")
	if gotID != 0 {
		t.Errorf("GetChannelIdByUserName(\"\") = %d, want 0", gotID)
	}
}

func TestGetChannelInfoById(t *testing.T) {
	skipIfNoDb(t)

	channelID := -(time.Now().UnixNano()%1_000_000_000_000 + 3000)
	if channelID > 0 {
		channelID = -channelID
	}
	channelName := fmt.Sprintf("info-channel-%d", channelID)
	username := fmt.Sprintf("infochan_%d", -channelID)

	if err := UpdateChannel(channelID, channelName, username); err != nil {
		t.Fatalf("UpdateChannel() error = %v", err)
	}
	t.Cleanup(func() { db.DB.Where("chat_id = ?", channelID).Delete(&models.ChannelSettings{}) })

	gotUsername, gotName, found := GetChannelInfoById(channelID)
	if !found {
		t.Fatalf("GetChannelInfoById(%d) found=false, want true", channelID)
	}
	if gotUsername != username {
		t.Errorf("username = %q, want %q", gotUsername, username)
	}
	if gotName != channelName {
		t.Errorf("name = %q, want %q", gotName, channelName)
	}
}

func TestGetChannelInfoById_NotFound(t *testing.T) {
	skipIfNoDb(t)

	_, _, found := GetChannelInfoById(-9999999999999)
	if found {
		t.Error("GetChannelInfoById() found=true for non-existent channel, want false")
	}
}

func TestUpdateChannel(t *testing.T) {
	skipIfNoDb(t)

	channelID := -(time.Now().UnixNano()%1_000_000_000_000 + 4000)
	if channelID > 0 {
		channelID = -channelID
	}
	username := fmt.Sprintf("updatechan_%d", -channelID)

	if err := UpdateChannel(channelID, "original-name", username); err != nil {
		t.Fatalf("initial UpdateChannel() error = %v", err)
	}
	t.Cleanup(func() { db.DB.Where("chat_id = ?", channelID).Delete(&models.ChannelSettings{}) })

	updatedName := "updated-name"
	if err := UpdateChannel(channelID, updatedName, username); err != nil {
		t.Fatalf("UpdateChannel() update error = %v", err)
	}

	var ch models.ChannelSettings
	if err := db.DB.Where("chat_id = ?", channelID).First(&ch).Error; err != nil {
		t.Fatalf("expected channel to exist: %v", err)
	}
	if ch.ChannelName != updatedName {
		t.Errorf("channel name = %q, want %q", ch.ChannelName, updatedName)
	}
}

func TestUpdateChannelClearsAndReassignsNormalizedUsername(t *testing.T) {
	withChannelSQLite(t)

	const (
		firstChannelID  = int64(-1000000000001)
		secondChannelID = int64(-1000000000002)
		thirdChannelID  = int64(-1000000000003)
	)
	if db.DB.Name() == "postgres" {
		for _, chatID := range []int64{firstChannelID, secondChannelID, thirdChannelID} {
			if err := chats.EnsureChatInDb(chatID, "channel ownership test"); err != nil {
				t.Fatalf("EnsureChatInDb(%d) error = %v", chatID, err)
			}
		}
		t.Cleanup(func() {
			_ = db.DB.Where("chat_id IN ?", []int64{firstChannelID, secondChannelID, thirdChannelID}).
				Delete(&models.ChannelSettings{}).Error
			_ = db.DB.Where("chat_id IN ?", []int64{firstChannelID, secondChannelID, thirdChannelID}).
				Delete(&models.Chat{}).Error
		})
	}
	if err := UpdateChannel(firstChannelID, "First", "@NewsRoom"); err != nil {
		t.Fatalf("UpdateChannel(first) error = %v", err)
	}
	if got := GetChannelIdByUserName("NEWSROOM"); got != firstChannelID {
		t.Fatalf("case-insensitive lookup = %d, want %d", got, firstChannelID)
	}

	if err := UpdateChannel(firstChannelID, "First", ""); err != nil {
		t.Fatalf("UpdateChannel(clear username) error = %v", err)
	}
	if got := GetChannelIdByUserName("newsroom"); got != 0 {
		t.Fatalf("lookup after username removal = %d, want 0", got)
	}

	if err := UpdateChannel(firstChannelID, "First", "newsroom"); err != nil {
		t.Fatalf("UpdateChannel(restore first username) error = %v", err)
	}
	if err := UpdateChannel(secondChannelID, "Second", "NEWSROOM"); err != nil {
		t.Fatalf("UpdateChannel(reassign username) error = %v", err)
	}
	if got := GetChannelIdByUserName("@NewsRoom"); got != secondChannelID {
		t.Fatalf("lookup after reassignment = %d, want %d", got, secondChannelID)
	}
	first := GetChannelSettings(firstChannelID)
	if first == nil {
		t.Fatal("first channel settings = nil")
	}
	if first.Username != "" {
		t.Fatalf("first channel username = %q, want cleared", first.Username)
	}
	if err := db.DB.Create(&models.ChannelSettings{
		ChatId:    thirdChannelID,
		ChannelId: thirdChannelID,
		Username:  "NewsRoom",
	}).Error; err == nil {
		t.Fatal("case-insensitive username uniqueness was not enforced")
	}
}

func TestLoadChannelStats_ErrorBranch(t *testing.T) {
	skipIfNoDb(t)

	_ = db.DB.Migrator().DropTable(&models.ChannelSettings{})
	t.Cleanup(func() {
		_ = db.DB.AutoMigrate(&models.ChannelSettings{})
	})

	count := LoadChannelStats()
	if count != 0 {
		t.Fatalf("LoadChannelStats() = %d, want 0 on error", count)
	}
}
