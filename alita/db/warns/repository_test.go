package warns

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/user"
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

	// Close DB handle before removing temp file.
	if db.DB != nil {
		sqlDB, err := db.DB.DB()
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
	if db.DB == nil {
		t.Skip("requires database connection")
	}
}

func TestCheckWarnSettings_Defaults(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	if err := chats.EnsureChatInDb(chatID, "test-warn-defaults"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.WarnSettings{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
	})

	settings := GetWarnSetting(chatID)
	if settings == nil {
		t.Fatalf("GetWarnSetting() returned nil")
	}
	if settings.WarnLimit != 3 {
		t.Fatalf("expected default WarnLimit=3, got %d", settings.WarnLimit)
	}
	if settings.WarnMode != "mute" {
		t.Fatalf("expected default WarnMode='mute', got %q", settings.WarnMode)
	}
}

func TestWarnUser(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	chatID := base
	userID := base + 1

	if err := chats.EnsureChatInDb(chatID, "test-warn-user"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ? AND user_id = ?", chatID, userID).Delete(&models.Warns{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.WarnSettings{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
	})

	numWarns, reasons, err := WarnUser(userID, chatID, "test reason")
	if err != nil {
		t.Fatalf("WarnUser() error = %v", err)
	}
	if numWarns != 1 {
		t.Fatalf("expected numWarns=1, got %d", numWarns)
	}
	if len(reasons) != 1 {
		t.Fatalf("expected 1 reason, got %d", len(reasons))
	}
	if reasons[0] != "test reason" {
		t.Fatalf("expected reason='test reason', got %q", reasons[0])
	}
}

func TestWarnUserCreatesMissingParentRows(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	chatID := base
	userID := base + 1
	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ? AND user_id = ?", chatID, userID).Delete(&models.Warns{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.WarnSettings{}).Error
		_ = db.DB.Where("user_id = ?", userID).Delete(&models.User{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
	})

	numWarns, reasons, err := WarnUser(userID, chatID, "first")
	if err != nil {
		t.Fatalf("WarnUser() error = %v", err)
	}
	if numWarns != 1 || len(reasons) != 1 {
		t.Fatalf("WarnUser() = (%d, %v), want one persisted warning", numWarns, reasons)
	}
	if err := db.DB.Where("chat_id = ?", chatID).First(&models.Chat{}).Error; err != nil {
		t.Fatalf("chat parent row missing: %v", err)
	}
	if err := db.DB.Where("user_id = ?", userID).First(&models.User{}).Error; err != nil {
		t.Fatalf("user parent row missing: %v", err)
	}
}

func TestWarnUserReachesLimit(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	chatID := base
	userID := base + 1
	limit := 3

	if err := chats.EnsureChatInDb(chatID, "test-warn-limit"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ? AND user_id = ?", chatID, userID).Delete(&models.Warns{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.WarnSettings{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
	})

	var lastCount int
	for i := 1; i <= limit; i++ {
		reason := fmt.Sprintf("reason %d", i)
		n, _, err := WarnUser(userID, chatID, reason)
		if err != nil {
			t.Fatalf("WarnUser(%d) error = %v", i, err)
		}
		lastCount = n
	}

	if lastCount != limit {
		t.Fatalf("expected warn count=%d after %d warns, got %d", limit, limit, lastCount)
	}

	setting := GetWarnSetting(chatID)
	if lastCount >= setting.WarnLimit {
		// Limit reached — this is expected behavior.
	} else {
		t.Fatalf("expected to reach warn limit %d, got %d", setting.WarnLimit, lastCount)
	}
}

func TestRemoveWarn(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	chatID := base
	userID := base + 1

	if err := chats.EnsureChatInDb(chatID, "test-remove-warn"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ? AND user_id = ?", chatID, userID).Delete(&models.Warns{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.WarnSettings{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
	})

	// Add two warns then remove one
	WarnUser(userID, chatID, "first")
	WarnUser(userID, chatID, "second")

	removed, err := RemoveWarn(userID, chatID)
	if err != nil {
		t.Fatalf("RemoveWarn() error = %v", err)
	}
	if !removed {
		t.Fatalf("RemoveWarn() returned false, expected true")
	}

	numWarns, reasons := GetWarns(userID, chatID)
	if numWarns != 1 {
		t.Fatalf("expected numWarns=1 after remove, got %d", numWarns)
	}
	if len(reasons) != 1 {
		t.Fatalf("expected 1 reason after remove, got %d", len(reasons))
	}
	if reasons[0] != "first" {
		t.Fatalf("expected remaining reason='first', got %q", reasons[0])
	}
}

func TestResetWarns(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	chatID := base
	userID := base + 1

	if err := chats.EnsureChatInDb(chatID, "test-reset-warns"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ? AND user_id = ?", chatID, userID).Delete(&models.Warns{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.WarnSettings{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
	})

	WarnUser(userID, chatID, "a")
	WarnUser(userID, chatID, "b")
	WarnUser(userID, chatID, "c")

	numBefore, _ := GetWarns(userID, chatID)
	if numBefore != 3 {
		t.Fatalf("expected 3 warns before reset, got %d", numBefore)
	}

	ok, err := ResetUserWarns(userID, chatID)
	if err != nil {
		t.Fatalf("ResetUserWarns() error = %v", err)
	}
	if !ok {
		t.Fatalf("ResetUserWarns() returned false")
	}

	numAfter, reasons := GetWarns(userID, chatID)
	if numAfter != 0 {
		t.Fatalf("expected 0 warns after reset, got %d", numAfter)
	}
	if len(reasons) != 0 {
		t.Fatalf("expected 0 reasons after reset, got %d", len(reasons))
	}
}

//nolint:dupl // Test functions intentionally similar for clarity
func TestSetWarnLimit(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()

	if err := chats.EnsureChatInDb(chatID, "test-set-warn-limit"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.WarnSettings{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
	})

	if err := SetWarnLimit(chatID, 5); err != nil {
		t.Fatalf("SetWarnLimit failed: %v", err)
	}

	settings := GetWarnSetting(chatID)
	if settings.WarnLimit != 5 {
		t.Fatalf("expected WarnLimit=5, got %d", settings.WarnLimit)
	}
}

//nolint:dupl // Test functions intentionally similar for clarity
func TestSetWarnMode(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()

	if err := chats.EnsureChatInDb(chatID, "test-set-warn-mode"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.WarnSettings{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
	})

	if err := SetWarnMode(chatID, "ban"); err != nil {
		t.Fatalf("SetWarnMode failed: %v", err)
	}

	settings := GetWarnSetting(chatID)
	if settings.WarnMode != "ban" {
		t.Fatalf("expected WarnMode='ban', got %q", settings.WarnMode)
	}
}

func TestWarnWithEmptyReason(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	chatID := base
	userID := base + 1

	if err := chats.EnsureChatInDb(chatID, "test-warn-empty-reason"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ? AND user_id = ?", chatID, userID).Delete(&models.Warns{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.WarnSettings{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
	})

	numWarns, reasons, err := WarnUser(userID, chatID, "")
	if err != nil {
		t.Fatalf("WarnUser() error = %v", err)
	}
	if numWarns != 1 {
		t.Fatalf("expected numWarns=1, got %d", numWarns)
	}
	if len(reasons) != 1 {
		t.Fatalf("expected 1 reason even for empty reason, got %d", len(reasons))
	}
	// Empty reason should be stored as a default "No Reason" placeholder
	if reasons[0] == "" {
		t.Fatalf("expected a non-empty placeholder for empty reason, got empty string")
	}
}

func TestWarnReasonTruncationPreservesUTF8(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	chatID := base
	userID := base + 1

	if err := chats.EnsureChatInDb(chatID, "test-warn-utf8-reason"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ? AND user_id = ?", chatID, userID).Delete(&models.Warns{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.WarnSettings{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
	})

	_, reasons, err := WarnUser(userID, chatID, strings.Repeat("a", 2999)+"界")
	if err != nil {
		t.Fatalf("WarnUser() error = %v", err)
	}
	if len(reasons) != 1 || len(reasons[0]) > 3000 || !utf8.ValidString(reasons[0]) {
		t.Fatalf("truncated reason = %q, want one valid UTF-8 reason of at most 3000 bytes", reasons)
	}
}

func TestResetWarns_NoWarns(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	chatID := base
	userID := base + 1

	if err := chats.EnsureChatInDb(chatID, "test-reset-no-warns"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ? AND user_id = ?", chatID, userID).Delete(&models.Warns{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.WarnSettings{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
	})

	// Reset when user has no warns — should return false (nothing was reset)
	ok, err := ResetUserWarns(userID, chatID)
	if err != nil {
		t.Fatalf("ResetUserWarns() error = %v", err)
	}
	if ok {
		t.Fatalf("ResetUserWarns() returned true when no warns exist, expected false")
	}
}

func TestSequentialMultipleWarns(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	chatID := base
	userID := base + 1

	if err := chats.EnsureChatInDb(chatID, "test-sequential-warns"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ? AND user_id = ?", chatID, userID).Delete(&models.Warns{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.WarnSettings{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
	})

	const total = 5
	for i := 0; i < total; i++ {
		WarnUser(userID, chatID, fmt.Sprintf("reason %d", i))
	}

	numWarns, reasons := GetWarns(userID, chatID)
	if numWarns != total {
		t.Fatalf("expected %d warns after sequential inserts, got %d", total, numWarns)
	}
	if len(reasons) != total {
		t.Fatalf("expected %d reasons after sequential inserts, got %d", total, len(reasons))
	}
}

func TestGetAllChatWarns(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	chatID := base
	userID1 := base + 1
	userID2 := base + 2

	if err := chats.EnsureChatInDb(chatID, "test-get-all-chat-warns"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Warns{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.WarnSettings{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
	})

	WarnUser(userID1, chatID, "reason1")
	WarnUser(userID2, chatID, "reason2")

	count := GetAllChatWarns(chatID)
	if count < 2 {
		t.Fatalf("expected at least 2 warned users, got %d", count)
	}
}

func TestResetAllChatWarns(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	chatID := base
	userID1 := base + 1
	userID2 := base + 2

	if err := chats.EnsureChatInDb(chatID, "test-reset-all-chat-warns"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Warns{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.WarnSettings{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
	})

	WarnUser(userID1, chatID, "a")
	WarnUser(userID2, chatID, "b")

	if err := ResetAllChatWarns(chatID); err != nil {
		t.Fatalf("ResetAllChatWarns() error = %v", err)
	}

	count := GetAllChatWarns(chatID)
	if count != 0 {
		t.Fatalf("expected 0 warns after ResetAllChatWarns, got %d", count)
	}
}

func TestWarnUserCreatesSettingsWithDefaultMode(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	userID := chatID + 1

	if err := chats.EnsureChatInDb(chatID, "test-warn-default-mode"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ? AND user_id = ?", chatID, userID).Delete(&models.Warns{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.WarnSettings{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
	})

	// WarnUser is the first operation — no prior GetWarnSetting call — so it
	// must create the WarnSettings row itself inside its transaction.
	WarnUser(userID, chatID, "trigger settings creation")

	// Load the persisted WarnSettings row directly from the DB.
	var settings models.WarnSettings
	if err := db.DB.Where("chat_id = ?", chatID).First(&settings).Error; err != nil {
		t.Fatalf("WarnSettings row not found after WarnUser: %v", err)
	}
	if settings.WarnMode != "mute" {
		t.Fatalf("expected WarnMode='mute' when WarnUser creates the settings row, got %q", settings.WarnMode)
	}
}

func TestRemoveWarn_NoWarns(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	chatID := base
	userID := base + 1

	if err := chats.EnsureChatInDb(chatID, "test-remove-warn-none"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ? AND user_id = ?", chatID, userID).Delete(&models.Warns{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.WarnSettings{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
	})

	// RemoveWarn on user with no prior warns should return false gracefully
	removed, err := RemoveWarn(userID, chatID)
	if err != nil {
		t.Fatalf("RemoveWarn() error = %v", err)
	}
	if removed {
		t.Fatalf("expected RemoveWarn=false when user has no warns")
	}
}

func TestConcurrentWarnAndRemovePreserveCount(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	chatID := base
	userID := base + 1
	if err := chats.EnsureChatInDb(chatID, "concurrent-warns"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	if err := user.EnsureUserInDb(userID, "concurrent-warns", "Concurrent"); err != nil {
		t.Fatalf("EnsureUserInDb() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ? AND user_id = ?", chatID, userID).Delete(&models.Warns{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.WarnSettings{}).Error
		_ = db.DB.Where("user_id = ?", userID).Delete(&models.User{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
	})

	const operations = 10
	for i := 0; i < operations; i++ {
		if _, _, err := WarnUser(userID, chatID, fmt.Sprintf("seed %d", i)); err != nil {
			t.Fatalf("WarnUser(seed %d) error = %v", i, err)
		}
	}

	start := make(chan struct{})
	removed := make(chan bool, operations)
	removeErrs := make(chan error, operations)
	warnErrs := make(chan error, operations)
	var workers sync.WaitGroup
	workers.Add(operations * 2)
	for i := 0; i < operations; i++ {
		go func(i int) {
			defer workers.Done()
			<-start
			_, _, err := WarnUser(userID, chatID, fmt.Sprintf("concurrent %d", i))
			warnErrs <- err
		}(i)
		go func() {
			defer workers.Done()
			<-start
			ok, err := RemoveWarn(userID, chatID)
			removed <- ok
			removeErrs <- err
		}()
	}
	close(start)
	workers.Wait()
	close(removed)
	close(removeErrs)
	close(warnErrs)

	for err := range warnErrs {
		if err != nil {
			t.Errorf("concurrent WarnUser() error = %v", err)
		}
	}
	for err := range removeErrs {
		if err != nil {
			t.Errorf("concurrent RemoveWarn() error = %v", err)
		}
	}

	for ok := range removed {
		if !ok {
			t.Fatal("concurrent RemoveWarn() returned false")
		}
	}
	numWarns, reasons := GetWarns(userID, chatID)
	if numWarns != operations || len(reasons) != operations {
		t.Fatalf("warn state = (%d, %d reasons), want (%d, %d)", numWarns, len(reasons), operations, operations)
	}
}
