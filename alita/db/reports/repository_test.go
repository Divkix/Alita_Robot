package reports

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

func TestMain(m *testing.M) {
	var dbFileName string
	if db.DB == nil {
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
		sqlDB, err := sqliteDB.DB()
		if err != nil {
			fmt.Printf("SQLite handle failed: %v\n", err)
			os.Exit(1)
		}
		sqlDB.SetMaxOpenConns(1)
		db.DB = sqliteDB
	}

	if dbFileName != "" {
		err := db.DB.AutoMigrate(
			&models.User{},
			&models.Chat{},
			&models.WarnSettings{},
			&models.Warns{},
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
		)
		if err != nil {
			fmt.Printf("AutoMigrate failed: %v\n", err)
			os.Exit(1)
		}
	}

	exitCode := m.Run()

	if db.DB != nil {
		sqlDB, err := db.DB.DB()
		if err != nil {
			fmt.Printf("failed to get underlying DB: %v\n", err)
		} else if closeErr := sqlDB.Close(); closeErr != nil {
			fmt.Printf("DB close failed: %v\n", closeErr)
		}
	}

	if dbFileName != "" {
		if rmErr := os.Remove(dbFileName); rmErr != nil {
			fmt.Printf("temp file remove failed: %v\n", rmErr)
		}
	}

	os.Exit(exitCode)
}

func skipIfNoDb(t *testing.T) {
	t.Helper()
	if db.DB == nil {
		t.Skip("requires database connection")
	}
}

func TestGetChatReportSettings_Defaults(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()

	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.ReportChatSettings{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
	})

	if err := chats.EnsureChatInDb(chatID, ""); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}

	settings := GetChatReportSettings(chatID)
	if settings == nil {
		t.Fatal("expected non-nil ReportChatSettings")
	}
	if !settings.Enabled {
		t.Fatal("expected default Enabled=true for chat report settings")
	}
	if len(settings.BlockedList) != 0 {
		t.Fatalf("expected empty BlockedList by default, got %v", settings.BlockedList)
	}
}

func TestSetChatReportEnabled_BooleanRoundTrip(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()

	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.ReportChatSettings{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
	})

	if err := chats.EnsureChatInDb(chatID, ""); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}

	_ = GetChatReportSettings(chatID)

	if err := SetChatReportStatus(chatID, false); err != nil {
		t.Fatalf("SetChatReportStatus() error = %v", err)
	}
	settings := GetChatReportSettings(chatID)
	if settings.Enabled {
		t.Fatal("expected Enabled=false after SetChatReportStatus(false)")
	}

	if err := SetChatReportStatus(chatID, true); err != nil {
		t.Fatalf("SetChatReportStatus() error = %v", err)
	}
	settings = GetChatReportSettings(chatID)
	if !settings.Enabled {
		t.Fatal("expected Enabled=true after SetChatReportStatus(true)")
	}
}

func TestGetUserReportSettings_Defaults(t *testing.T) {
	skipIfNoDb(t)

	userID := time.Now().UnixNano()

	t.Cleanup(func() {
		_ = db.DB.Where("user_id = ?", userID).Delete(&models.ReportUserSettings{}).Error
		_ = db.DB.Where("user_id = ?", userID).Delete(&models.User{}).Error
	})

	settings := GetUserReportSettings(userID)
	if settings == nil {
		t.Fatal("expected non-nil ReportUserSettings")
	}
	if !settings.Enabled {
		t.Fatal("expected default Enabled=true for user report settings")
	}
	var parentCount int64
	if err := db.DB.Model(&models.User{}).Where("user_id = ?", userID).Count(&parentCount).Error; err != nil {
		t.Fatalf("count user parent: %v", err)
	}
	if parentCount != 1 {
		t.Fatalf("user parent rows = %d, want 1", parentCount)
	}
}

func TestSetUserReportEnabled_BooleanRoundTrip(t *testing.T) {
	skipIfNoDb(t)

	userID := time.Now().UnixNano()

	t.Cleanup(func() {
		_ = db.DB.Where("user_id = ?", userID).Delete(&models.ReportUserSettings{}).Error
		_ = db.DB.Where("user_id = ?", userID).Delete(&models.User{}).Error
	})

	_ = GetUserReportSettings(userID)

	if err := SetUserReportSettings(userID, false); err != nil {
		t.Fatalf("SetUserReportSettings() error = %v", err)
	}
	settings := GetUserReportSettings(userID)
	if settings.Enabled {
		t.Fatal("expected Enabled=false after SetUserReportSettings(false)")
	}

	if err := SetUserReportSettings(userID, true); err != nil {
		t.Fatalf("SetUserReportSettings() error = %v", err)
	}
	settings = GetUserReportSettings(userID)
	if !settings.Enabled {
		t.Fatal("expected Enabled=true after SetUserReportSettings(true)")
	}
}

func TestGetBlockedReportsList_EmptyForNewChat(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()

	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.ReportChatSettings{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
	})

	if err := chats.EnsureChatInDb(chatID, ""); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}

	settings := GetChatReportSettings(chatID)
	if len(settings.BlockedList) != 0 {
		t.Fatalf("expected empty blocked list for new chat, got %v", settings.BlockedList)
	}
}

func TestAddBlockedReport_AddAndVerify(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	chatID := base
	userID := base + 1

	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.ReportChatSettings{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
	})

	if err := chats.EnsureChatInDb(chatID, ""); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}

	_ = GetChatReportSettings(chatID)

	if err := BlockReportUser(chatID, userID); err != nil {
		t.Fatalf("BlockReportUser() error = %v", err)
	}

	settings := GetChatReportSettings(chatID)
	found := false
	for _, id := range settings.BlockedList {
		if id == userID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected userID %d in blocked list, got %v", userID, settings.BlockedList)
	}
}

func TestRemoveBlockedReport_UnblockOneOfTwo(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	chatID := base
	userID1 := base + 1
	userID2 := base + 2

	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.ReportChatSettings{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
	})

	if err := chats.EnsureChatInDb(chatID, ""); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}

	_ = GetChatReportSettings(chatID)

	// Block two users, then unblock one — list stays non-nil so GORM persists it
	if err := BlockReportUser(chatID, userID1); err != nil {
		t.Fatalf("BlockReportUser() error = %v", err)
	}
	if err := BlockReportUser(chatID, userID2); err != nil {
		t.Fatalf("BlockReportUser() error = %v", err)
	}
	if err := UnblockReportUser(chatID, userID1); err != nil {
		t.Fatalf("UnblockReportUser() error = %v", err)
	}

	settings := GetChatReportSettings(chatID)
	foundUser1 := false
	foundUser2 := false
	for _, id := range settings.BlockedList {
		if id == userID1 {
			foundUser1 = true
		}
		if id == userID2 {
			foundUser2 = true
		}
	}
	if foundUser1 {
		t.Fatalf("expected userID1 %d to be removed from blocked list, but it's still present", userID1)
	}
	if !foundUser2 {
		t.Fatalf("expected userID2 %d to remain in blocked list, but it's missing", userID2)
	}
}

func TestBlockReportUser_IdempotentAdd(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	chatID := base
	userID := base + 1

	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.ReportChatSettings{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
	})

	if err := chats.EnsureChatInDb(chatID, ""); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}

	_ = GetChatReportSettings(chatID)

	if err := BlockReportUser(chatID, userID); err != nil {
		t.Fatalf("BlockReportUser() error = %v", err)
	}
	if err := BlockReportUser(chatID, userID); err != nil {
		t.Fatalf("BlockReportUser() second call error = %v", err)
	}

	settings := GetChatReportSettings(chatID)
	count := 0
	for _, id := range settings.BlockedList {
		if id == userID {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected userID to appear exactly once in blocked list, got %d occurrences", count)
	}
}

func TestLoadReportStats_Returns(t *testing.T) {
	skipIfNoDb(t)

	uRCount, gRCount := LoadReportStats()
	if uRCount < 0 {
		t.Fatalf("expected non-negative uRCount, got %d", uRCount)
	}
	if gRCount < 0 {
		t.Fatalf("expected non-negative gRCount, got %d", gRCount)
	}
}

func TestConcurrentReportBlockListUpdates(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	if err := chats.EnsureChatInDb(chatID, "concurrent-reports"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	_ = GetChatReportSettings(chatID)
	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.ReportChatSettings{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
	})

	const users = 12
	runConcurrent := func(operation func(int64) error) {
		t.Helper()
		errs := make(chan error, users)
		var workers sync.WaitGroup
		workers.Add(users)
		for i := 0; i < users; i++ {
			go func(userID int64) {
				defer workers.Done()
				errs <- operation(userID)
			}(int64(i + 1))
		}
		workers.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Errorf("concurrent report-list update: %v", err)
			}
		}
	}

	runConcurrent(func(userID int64) error {
		return BlockReportUser(chatID, userID)
	})
	settings := GetChatReportSettings(chatID)
	if len(settings.BlockedList) != users {
		t.Fatalf("blocked users = %v, want %d entries", settings.BlockedList, users)
	}

	runConcurrent(func(userID int64) error {
		return UnblockReportUser(chatID, userID)
	})
	if blocked := GetChatReportSettings(chatID).BlockedList; len(blocked) != 0 {
		t.Fatalf("blocked users after concurrent unblocks = %v, want empty", blocked)
	}
}
