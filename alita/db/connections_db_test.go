package db

import (
	"sync"
	"testing"
	"time"
)

func TestConnectChat(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	userID := base
	chatID := base + 1

	t.Cleanup(func() {
		DB.Where("user_id = ?", userID).Delete(&ConnectionSettings{})
		DB.Where("chat_id = ?", chatID).Delete(&ConnectionChatSettings{})
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

func TestDisconnectChat(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	userID := base + 10
	chatID := base + 11

	t.Cleanup(func() {
		DB.Where("user_id = ?", userID).Delete(&ConnectionSettings{})
		DB.Where("chat_id = ?", chatID).Delete(&ConnectionChatSettings{})
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
	t.Parallel()
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	userID := base + 20

	t.Cleanup(func() {
		DB.Where("user_id = ?", userID).Delete(&ConnectionSettings{})
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

func TestReconnect(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	userID := base + 30
	chatID := base + 31

	t.Cleanup(func() {
		DB.Where("user_id = ?", userID).Delete(&ConnectionSettings{})
		DB.Where("chat_id = ?", chatID).Delete(&ConnectionChatSettings{})
	})

	// Set up connection record
	_ = Connection(userID)
	ConnectId(userID, chatID)
	DisconnectId(userID)

	got := Connection(userID)
	if got.Connected {
		t.Fatal("expected Connected=false after DisconnectId")
	}

	// Reconnect
	reconnectedChat := ReconnectId(userID)
	if reconnectedChat != chatID {
		t.Fatalf("ReconnectId() returned chatID=%d, want %d", reconnectedChat, chatID)
	}

	got = Connection(userID)
	if !got.Connected {
		t.Fatal("expected Connected=true after ReconnectId")
	}
}

func TestSetAllowConnect(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	chatID := base + 40

	if err := EnsureChatInDb(chatID, "test_conn"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		DB.Where("chat_id = ?", chatID).Delete(&ConnectionChatSettings{})
		DB.Where("chat_id = ?", chatID).Delete(&Chat{})
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

func TestGetConnectedChats(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	base := time.Now().UnixNano()
	userID := base + 50
	chatID1 := base + 51
	chatID2 := base + 52

	t.Cleanup(func() {
		DB.Where("user_id = ?", userID).Delete(&ConnectionSettings{})
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
	t.Parallel()
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

func TestConcurrentConnect(t *testing.T) {
	t.Parallel()
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
				DB.Where("user_id = ?", uid).Delete(&ConnectionSettings{})
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
	t.Parallel()
	skipIfNoDb(t)

	userID := time.Now().UnixNano() + 9000

	t.Cleanup(func() {
		DB.Where("user_id = ?", userID).Delete(&ConnectionSettings{})
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
	t.Parallel()
	skipIfNoDb(t)

	base := time.Now().UnixNano() + 10000
	userID := base
	chatID := base + 1

	t.Cleanup(func() {
		DB.Where("user_id = ?", userID).Delete(&ConnectionSettings{})
		DB.Where("chat_id = ?", chatID).Delete(&ConnectionChatSettings{})
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
