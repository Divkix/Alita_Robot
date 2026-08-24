package connections

import (
	"sync"
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

func skipIfNoDb(t *testing.T) {
	if db.DB == nil {
		t.Skip("DB not initialized")
	}
}

func TestConnectChat(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	userID := base
	chatID := base + 1

	t.Cleanup(func() {
		db.DB.Where("user_id = ?", userID).Delete(&models.ConnectionSettings{})
		db.DB.Where("chat_id = ?", chatID).Delete(&models.ConnectionChatSettings{})
	})

	// Ensure user connection record exists
	conn := Connection(userID)
	if conn == nil {
		t.Fatal("Connection() returned nil")
	}

	ConnectId(userID, chatID)

	got := Connection(userID)
	if got == nil {
		t.Fatal("Connection() returned nil after ConnectId")
	}
	if !got.Connected {
		t.Fatalf("expected Connected=true, got %v", got.Connected)
	}
	if got.ChatId != chatID {
		t.Fatalf("expected ChatId=%d, got %d", chatID, got.ChatId)
	}
}

func TestConnectIdAcceptsTelegramGroupIDs(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	userID := base + 5
	chatID := -1000000000000 - base%1_000_000

	t.Cleanup(func() {
		db.DB.Where("user_id = ?", userID).Delete(&models.ConnectionSettings{})
		db.DB.Where("chat_id = ?", chatID).Delete(&models.ConnectionChatSettings{})
	})

	ConnectId(userID, chatID)

	got := Connection(userID)
	if got == nil || !got.Connected {
		t.Fatalf("Connection(%d) = %+v, want connected for negative Telegram group ID", userID, got)
	}
	if got.ChatId != chatID {
		t.Fatalf("ChatId = %d, want %d", got.ChatId, chatID)
	}
}

func TestConnectIdRejectsZeroChatID(t *testing.T) {
	if err := ConnectId(time.Now().UnixNano(), 0); err == nil {
		t.Fatal("ConnectId() error = nil for zero chat ID")
	}
}

func TestDisconnectChat(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	userID := base + 10
	chatID := base + 11

	t.Cleanup(func() {
		db.DB.Where("user_id = ?", userID).Delete(&models.ConnectionSettings{})
		db.DB.Where("chat_id = ?", chatID).Delete(&models.ConnectionChatSettings{})
	})

	// Connect first
	_ = Connection(userID)
	ConnectId(userID, chatID)

	got := Connection(userID)
	if !got.Connected {
		t.Fatal("expected Connected=true after ConnectId")
	}

	// Now disconnect
	DisconnectId(userID)

	got = Connection(userID)
	if got.Connected {
		t.Fatalf("expected Connected=false after DisconnectId, got %v", got.Connected)
	}
}

func TestGetConnection(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	userID := base + 20

	t.Cleanup(func() {
		db.DB.Where("user_id = ?", userID).Delete(&models.ConnectionSettings{})
	})

	conn := Connection(userID)
	if conn == nil {
		t.Fatal("Connection() returned nil")
	}
	if conn.UserId != userID {
		t.Fatalf("expected UserId=%d, got %d", userID, conn.UserId)
	}
	if conn.Connected {
		t.Fatalf("expected Connected=false by default")
	}
}

func TestSetAllowConnect(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	chatID := base + 40

	if err := chats.EnsureChatInDb(chatID, "test_conn"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		db.DB.Where("chat_id = ?", chatID).Delete(&models.ConnectionChatSettings{})
		db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{})
	})

	// Get default settings first (creates the record)
	settings := GetChatConnectionSetting(chatID)
	if settings == nil {
		t.Fatal("GetChatConnectionSetting() returned nil")
	}

	// Toggle to true
	ToggleAllowConnect(chatID, true)
	settings = GetChatConnectionSetting(chatID)
	if !settings.AllowConnect {
		t.Fatal("expected AllowConnect=true after ToggleAllowConnect(true)")
	}

	// Toggle to false -- zero-value boolean round-trip
	ToggleAllowConnect(chatID, false)
	settings = GetChatConnectionSetting(chatID)
	if settings.AllowConnect {
		t.Fatal("expected AllowConnect=false after ToggleAllowConnect(false)")
	}
}

func TestToggleAllowConnectCreatesMissingSettings(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano() + 45
	t.Cleanup(func() {
		db.DB.Where("chat_id = ?", chatID).Delete(&models.ConnectionChatSettings{})
		db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{})
	})

	ToggleAllowConnect(chatID, true)

	settings := GetChatConnectionSetting(chatID)
	if settings == nil {
		t.Fatal("GetChatConnectionSetting() returned nil")
	}
	if !settings.AllowConnect {
		t.Fatal("AllowConnect = false, want true after toggling missing settings row")
	}
}

func TestGetConnectedChats(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	userID := base + 50
	chatID1 := base + 51
	chatID2 := base + 52

	t.Cleanup(func() {
		db.DB.Where("user_id = ?", userID).Delete(&models.ConnectionSettings{})
	})

	// Connect to two separate chats sequentially (each ConnectId updates the single record)
	_ = Connection(userID)
	ConnectId(userID, chatID1)
	got := Connection(userID)
	if got.ChatId != chatID1 {
		t.Fatalf("expected ChatId=%d, got %d", chatID1, got.ChatId)
	}

	ConnectId(userID, chatID2)
	got = Connection(userID)
	if got.ChatId != chatID2 {
		t.Fatalf("expected ChatId=%d after second connect, got %d", chatID2, got.ChatId)
	}
	if !got.Connected {
		t.Fatal("expected Connected=true after second ConnectId")
	}
}

func TestLoadConnectionStats(t *testing.T) {
	skipIfNoDb(t)

	connectedUsers, connectedChats := LoadConnectionStats()
	// We can't assert exact values since other tests share the DB,
	// but the function must not panic and should return non-negative values.
	if connectedUsers < 0 {
		t.Fatalf("LoadConnectionStats() connectedUsers=%d, want >= 0", connectedUsers)
	}
	if connectedChats < 0 {
		t.Fatalf("LoadConnectionStats() connectedChats=%d, want >= 0", connectedChats)
	}
}

func TestLoadConnectionStatsErrorBranch(t *testing.T) {
	skipIfNoDb(t)

	_ = db.DB.Migrator().DropTable(&models.ConnectionChatSettings{})
	t.Cleanup(func() {
		_ = db.DB.AutoMigrate(&models.ConnectionChatSettings{})
	})

	users, chats := LoadConnectionStats()
	if users != 0 || chats != 0 {
		t.Fatalf("LoadConnectionStats() = (%d, %d), want (0, 0) on error", users, chats)
	}
}

func TestConcurrentConnect(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	chatID := base + 60

	const workers = 5
	var wg sync.WaitGroup
	wg.Add(workers)

	// Each goroutine uses its own unique userID to avoid races on shared rows.
	for i := 0; i < workers; i++ {
		userID := base + 70 + int64(i)
		go func(uid int64) {
			defer wg.Done()
			t.Cleanup(func() {
				db.DB.Where("user_id = ?", uid).Delete(&models.ConnectionSettings{})
			})
			_ = Connection(uid)
			ConnectId(uid, chatID)
			got := Connection(uid)
			if got == nil || !got.Connected {
				t.Errorf("goroutine uid=%d: expected Connected=true", uid)
			}
		}(userID)
	}

	wg.Wait()
}

func TestConnectionForNewUser(t *testing.T) {
	skipIfNoDb(t)

	userID := time.Now().UnixNano() + 9000

	t.Cleanup(func() {
		db.DB.Where("user_id = ?", userID).Delete(&models.ConnectionSettings{})
	})

	// Connection() for a brand-new user must return a non-nil record with Connected=false
	conn := Connection(userID)
	if conn == nil {
		t.Fatal("Connection() returned nil for new user")
	}
	if conn.Connected {
		t.Fatalf("expected Connected=false for new user, got true")
	}
	if conn.UserId != userID {
		t.Fatalf("expected UserId=%d, got %d", userID, conn.UserId)
	}
}

func TestDisconnectId(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano() + 10000
	userID := base
	chatID := base + 1

	t.Cleanup(func() {
		db.DB.Where("user_id = ?", userID).Delete(&models.ConnectionSettings{})
		db.DB.Where("chat_id = ?", chatID).Delete(&models.ConnectionChatSettings{})
	})

	// Establish a connection
	_ = Connection(userID)
	ConnectId(userID, chatID)

	got := Connection(userID)
	if !got.Connected {
		t.Fatal("expected Connected=true after ConnectId")
	}
	if got.ChatId != chatID {
		t.Fatalf("expected ChatId=%d after ConnectId, got %d", chatID, got.ChatId)
	}

	// Disconnect and verify
	DisconnectId(userID)

	got = Connection(userID)
	if got == nil {
		t.Fatal("Connection() returned nil after DisconnectId")
	}
	if got.Connected {
		t.Fatalf("expected Connected=false after DisconnectId, got true")
	}
}

func TestConcurrentConnectKeepsOneRowPerUser(t *testing.T) {
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	userID := base
	const workers = 8
	chatIDs := make([]int64, workers)
	if err := db.DB.Create(&models.User{UserId: userID}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	for i := range chatIDs {
		chatIDs[i] = base + int64(i) + 1
		if err := db.DB.Create(&models.Chat{ChatId: chatIDs[i]}).Error; err != nil {
			t.Fatalf("create chat %d: %v", chatIDs[i], err)
		}
	}
	t.Cleanup(func() {
		_ = db.DB.Where("user_id = ?", userID).Delete(&models.ConnectionSettings{}).Error
		_ = db.DB.Where("chat_id IN ?", chatIDs).Delete(&models.Chat{}).Error
		_ = db.DB.Where("user_id = ?", userID).Delete(&models.User{}).Error
	})

	start := make(chan struct{})
	errs := make(chan error, workers)
	var wait sync.WaitGroup
	wait.Add(workers)
	for _, chatID := range chatIDs {
		go func(chatID int64) {
			defer wait.Done()
			<-start
			errs <- ConnectId(userID, chatID)
		}(chatID)
	}
	close(start)
	wait.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Errorf("ConnectId() error = %v", err)
		}
	}

	var count int64
	if err := db.DB.Model(&models.ConnectionSettings{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		t.Fatalf("count connections: %v", err)
	}
	if count != 1 {
		t.Fatalf("connection rows for user = %d, want 1", count)
	}
	connection := Connection(userID)
	if connection == nil || !connection.Connected {
		t.Fatalf("connection = %+v, want connected", connection)
	}
}
