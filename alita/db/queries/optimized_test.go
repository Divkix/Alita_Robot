package queries

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"gorm.io/gorm"
)

func skipIfNoDb(t *testing.T) {
	t.Helper()
	if db.DB == nil {
		t.Skip("requires database connection")
	}
}

func TestOptimizedLockQueries_NilDB(t *testing.T) {

	q := &OptimizedLockQueries{db: nil}
	_, err := q.GetLockStatus(123, "text")
	if err == nil {
		t.Fatal("GetLockStatus() with nil db expected error, got nil")
	}
	if err.Error() != "database not initialized" {
		t.Fatalf("GetLockStatus() error = %q, want %q", err.Error(), "database not initialized")
	}
}

func TestOptimizedLockQueries_GetLockStatus(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	lockType := "text"

	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ? AND lock_type = ?", chatID, lockType).Delete(&models.LockSettings{}).Error
	})

	q := NewOptimizedLockQueries()

	// No record -> default unlocked (false)
	locked, err := q.GetLockStatus(chatID, lockType)
	if err != nil {
		t.Fatalf("GetLockStatus() no record error = %v", err)
	}
	if locked {
		t.Fatalf("GetLockStatus() no record = %v, want false", locked)
	}

	// Insert a locked record
	if err := db.DB.Create(&models.LockSettings{ChatId: chatID, LockType: lockType, Locked: true}).Error; err != nil {
		t.Fatalf("DB.Create() lock error = %v", err)
	}

	locked, err = q.GetLockStatus(chatID, lockType)
	if err != nil {
		t.Fatalf("GetLockStatus() after insert error = %v", err)
	}
	if !locked {
		t.Fatalf("GetLockStatus() after insert = %v, want true", locked)
	}
}

func TestOptimizedLockQueries_GetChatLocksOptimized(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()

	lockTypes := []string{"text", "photo", "video"}
	t.Cleanup(func() {
		for _, lt := range lockTypes {
			_ = db.DB.Where("chat_id = ? AND lock_type = ?", chatID, lt).Delete(&models.LockSettings{}).Error
		}
	})

	// Insert 3 locks
	for _, lt := range lockTypes {
		if err := db.DB.Create(&models.LockSettings{ChatId: chatID, LockType: lt, Locked: true}).Error; err != nil {
			t.Fatalf("DB.Create() lock %q error = %v", lt, err)
		}
	}

	q := NewOptimizedLockQueries()

	// Get all locks for chatID -> map with 3 entries
	result, err := q.GetChatLocksOptimized(chatID)
	if err != nil {
		t.Fatalf("GetChatLocksOptimized() error = %v", err)
	}
	if len(result) != 3 {
		t.Fatalf("GetChatLocksOptimized() len = %d, want 3", len(result))
	}
	for _, lt := range lockTypes {
		if !result[lt] {
			t.Fatalf("GetChatLocksOptimized() lock %q = false, want true", lt)
		}
	}

	// Different chatID -> empty map
	differentChatID := chatID + 1
	emptyResult, err := q.GetChatLocksOptimized(differentChatID)
	if err != nil {
		t.Fatalf("GetChatLocksOptimized() different chat error = %v", err)
	}
	if len(emptyResult) != 0 {
		t.Fatalf("GetChatLocksOptimized() different chat len = %d, want 0", len(emptyResult))
	}
}

func TestOptimizedUserQueries_NilDB(t *testing.T) {

	q := &OptimizedUserQueries{db: nil}
	_, err := q.GetUserBasicInfo(123)
	if err == nil {
		t.Fatal("GetUserBasicInfo() with nil db expected error, got nil")
	}
	if err.Error() != "database not initialized" {
		t.Fatalf("GetUserBasicInfo() error = %q, want %q", err.Error(), "database not initialized")
	}
}

func TestOptimizedUserQueries_GetUserBasicInfo(t *testing.T) {
	skipIfNoDb(t)

	userID := time.Now().UnixNano()
	username := fmt.Sprintf("testuser_%d", userID)
	name := "Test User"

	t.Cleanup(func() {
		_ = db.DB.Where("user_id = ?", userID).Delete(&models.User{}).Error
	})

	// Insert a user
	if err := db.EnsureUserInDb(userID, username, name); err != nil {
		t.Fatalf("EnsureUserInDb() error = %v", err)
	}

	q := NewOptimizedUserQueries()

	// Get user by ID
	user, err := q.GetUserBasicInfo(userID)
	if err != nil {
		t.Fatalf("GetUserBasicInfo() error = %v", err)
	}
	if user.UserId != userID {
		t.Fatalf("GetUserBasicInfo() UserId = %d, want %d", user.UserId, userID)
	}

	// Non-existent user -> ErrRecordNotFound
	nonExistentID := userID + 999999
	_, err = q.GetUserBasicInfo(nonExistentID)
	if err == nil {
		t.Fatal("GetUserBasicInfo() nonexistent expected error, got nil")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("GetUserBasicInfo() nonexistent error = %v, want gorm.ErrRecordNotFound", err)
	}
}

func TestOptimizedChatQueries_NilDB(t *testing.T) {

	q := &OptimizedChatQueries{db: nil}
	_, err := q.GetChatBasicInfo(123)
	if err == nil {
		t.Fatal("GetChatBasicInfo() with nil db expected error, got nil")
	}
	if err.Error() != "database not initialized" {
		t.Fatalf("GetChatBasicInfo() error = %q, want %q", err.Error(), "database not initialized")
	}
}

func TestOptimizedChatQueries_GetChatBasicInfo(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
	})

	if err := db.EnsureChatInDb(chatID, "Optimized Chat"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}

	q := NewOptimizedChatQueries()
	chat, err := q.GetChatBasicInfo(chatID)
	if err != nil {
		t.Fatalf("GetChatBasicInfo() error = %v", err)
	}
	if chat.ChatId != chatID {
		t.Fatalf("GetChatBasicInfo() ChatId = %d, want %d", chat.ChatId, chatID)
	}

	_, err = q.GetChatBasicInfo(chatID + 999999)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("GetChatBasicInfo() missing error = %v, want gorm.ErrRecordNotFound", err)
	}
}

func TestOptimizedAntifloodQueries_DefaultAndRecord(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.AntifloodSettings{}).Error
	})

	q := NewOptimizedAntifloodQueries()
	defaults, err := q.GetAntifloodSettings(chatID)
	if err != nil {
		t.Fatalf("GetAntifloodSettings(default) error = %v", err)
	}
	if defaults.ChatId != chatID || defaults.Limit != 0 || defaults.Action != "mute" {
		t.Fatalf("default antiflood settings = %+v, want chat %d disabled mute", defaults, chatID)
	}

	if err := db.DB.Create(&models.AntifloodSettings{
		ChatId:                 chatID,
		Limit:                  7,
		Action:                 "ban",
		DeleteAntifloodMessage: true,
	}).Error; err != nil {
		t.Fatalf("DB.Create() antiflood error = %v", err)
	}

	got, err := q.GetAntifloodSettings(chatID)
	if err != nil {
		t.Fatalf("GetAntifloodSettings(record) error = %v", err)
	}
	if got.Limit != 7 || got.Action != "ban" || !got.DeleteAntifloodMessage {
		t.Fatalf("antiflood settings = %+v, want limit 7 ban delete", got)
	}
}

func TestOptimizedFilterQueries_GetChatFiltersOptimized(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatFilters{}).Error
	})

	filters := []*models.ChatFilters{
		{ChatId: chatID, KeyWord: "hello", FilterReply: "world", MsgType: 1, FileID: "file-1", NoNotif: true},
		{ChatId: chatID, KeyWord: "bye", FilterReply: "later", MsgType: 2, FileID: "file-2"},
	}
	for _, filter := range filters {
		if err := db.DB.Create(filter).Error; err != nil {
			t.Fatalf("DB.Create() filter %q error = %v", filter.KeyWord, err)
		}
	}

	q := NewOptimizedFilterQueries()
	got, err := q.GetChatFiltersOptimized(chatID)
	if err != nil {
		t.Fatalf("GetChatFiltersOptimized() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("GetChatFiltersOptimized() len = %d, want 2", len(got))
	}
	keywords := map[string]bool{}
	for _, filter := range got {
		keywords[filter.KeyWord] = true
	}
	if !keywords["hello"] || !keywords["bye"] {
		t.Fatalf("GetChatFiltersOptimized() keywords = %v, want hello and bye", keywords)
	}

	empty, err := q.GetChatFiltersOptimized(chatID + 1)
	if err != nil {
		t.Fatalf("GetChatFiltersOptimized(empty) error = %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("GetChatFiltersOptimized(empty) len = %d, want 0", len(empty))
	}
}

func TestOptimizedChannelQueries_GetChannelSettings(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.ChannelSettings{}).Error
	})

	if err := db.DB.Create(&models.ChannelSettings{
		ChatId:      chatID,
		ChannelId:   chatID + 10,
		ChannelName: "News",
		Username:    "news_channel",
	}).Error; err != nil {
		t.Fatalf("DB.Create() channel error = %v", err)
	}

	q := NewOptimizedChannelQueries()
	got, err := q.GetChannelSettings(chatID)
	if err != nil {
		t.Fatalf("GetChannelSettings() error = %v", err)
	}
	if got.ChannelId != chatID+10 || got.ChannelName != "News" || got.Username != "news_channel" {
		t.Fatalf("GetChannelSettings() = %+v, want inserted channel fields", got)
	}

	_, err = q.GetChannelSettings(chatID + 999999)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("GetChannelSettings(missing) error = %v, want gorm.ErrRecordNotFound", err)
	}
}

func TestOptimizedQueriesNilDatabaseConstructors(t *testing.T) {
	originalDB := db.DB
	db.DB = nil
	t.Cleanup(func() {
		db.DB = originalDB
		optimizedQueriesMu.Lock()
		optimizedQueries = nil
		optimizedQueriesMu.Unlock()
	})

	if NewOptimizedLockQueries().db != nil {
		t.Fatal("NewOptimizedLockQueries() with nil DB returned non-nil db")
	}
	if NewOptimizedUserQueries().db != nil {
		t.Fatal("NewOptimizedUserQueries() with nil DB returned non-nil db")
	}
	if NewOptimizedChatQueries().db != nil {
		t.Fatal("NewOptimizedChatQueries() with nil DB returned non-nil db")
	}
	if NewOptimizedAntifloodQueries().db != nil {
		t.Fatal("NewOptimizedAntifloodQueries() with nil DB returned non-nil db")
	}
	if NewOptimizedFilterQueries().db != nil {
		t.Fatal("NewOptimizedFilterQueries() with nil DB returned non-nil db")
	}
	if NewOptimizedBlacklistQueries().db != nil {
		t.Fatal("NewOptimizedBlacklistQueries() with nil DB returned non-nil db")
	}
	if NewOptimizedChannelQueries().db != nil {
		t.Fatal("NewOptimizedChannelQueries() with nil DB returned non-nil db")
	}
	if NewOptimizedAntiRaidQueries().db != nil {
		t.Fatal("NewOptimizedAntiRaidQueries() with nil DB returned non-nil db")
	}
	if isOptimizedQueriesValid() {
		t.Fatal("isOptimizedQueriesValid() with nil DB = true, want false")
	}
}

func TestCachedOptimizedQueriesRejectsMissingQueryAdapters(t *testing.T) {
	tests := []struct {
		name string
		call func() error
	}{
		{name: "lock", call: func() error {
			_, err := (*CachedOptimizedQueries)(nil).GetLockStatusCached(1, "text")
			return err
		}},
		{name: "user", call: func() error {
			_, err := (&CachedOptimizedQueries{}).GetUserBasicInfoCached(1)
			return err
		}},
		{name: "chat", call: func() error {
			_, err := (&CachedOptimizedQueries{}).GetChatBasicInfoCached(1)
			return err
		}},
		{name: "antiflood", call: func() error {
			_, err := (&CachedOptimizedQueries{}).GetAntifloodSettingsCached(1)
			return err
		}},
		{name: "antiraid", call: func() error {
			_, err := (&CachedOptimizedQueries{}).GetAntiRaidSettingsCached(1)
			return err
		}},
		{name: "filter", call: func() error {
			_, err := (&CachedOptimizedQueries{}).GetChatFiltersCached(1)
			return err
		}},
		{name: "channel", call: func() error {
			_, err := (&CachedOptimizedQueries{}).GetChannelSettingsCached(1)
			return err
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); err == nil {
				t.Fatal("cached query error = nil, want missing adapter error")
			}
		})
	}
}

func TestGetOptimizedQueries_Singleton(t *testing.T) {
	skipIfNoDb(t)

	// Call twice and verify same pointer is returned
	first := GetOptimizedQueries()
	second := GetOptimizedQueries()

	if first == nil {
		t.Fatal("GetOptimizedQueries() returned nil on first call")
	}
	if second == nil {
		t.Fatal("GetOptimizedQueries() returned nil on second call")
	}
	if first != second {
		t.Fatalf("GetOptimizedQueries() returned different pointers: %p != %p", first, second)
	}
}

func TestCacheKeyFormats(t *testing.T) {

	tests := []struct {
		name     string
		fn       func() string
		expected string
	}{
		{
			name:     "CacheKey with two IDs (lock style)",
			fn:       func() string { return cache.CacheKey("lock", int64(123), "text") },
			expected: "alita:lock:123:text",
		},
		{
			name:     "CacheKey user",
			fn:       func() string { return cache.CacheKey("user", int64(456)) },
			expected: "alita:user:456",
		},
		{
			name:     "CacheKey chat",
			fn:       func() string { return cache.CacheKey("chat", int64(789)) },
			expected: "alita:chat:789",
		},
		{
			name:     "CacheKey antiflood",
			fn:       func() string { return cache.CacheKey("antiflood", int64(101)) },
			expected: "alita:antiflood:101",
		},
		{
			name:     "CacheKey channel",
			fn:       func() string { return cache.CacheKey("channel", int64(202)) },
			expected: "alita:channel:202",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.fn()
			if got != tc.expected {
				t.Fatalf("%s() = %q, want %q", tc.name, got, tc.expected)
			}
		})
	}
}
