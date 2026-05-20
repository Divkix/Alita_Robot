package cache

import (
	"fmt"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/divkix/Alita_Robot/alita/utils/constants"
)

func withMemoryMarshaler(t *testing.T) {
	SetupTestMemoryMarshaler(t)
}

func TestAdminCacheRoundTripWithMemoryStore(t *testing.T) {
	withMemoryMarshaler(t)

	const chatID = int64(-100123)
	adminViaMap := gotgbot.MergedChatMember{
		Status: "administrator",
		User:   gotgbot.User{Id: 10, FirstName: "Map Admin"},
	}
	adminViaList := gotgbot.MergedChatMember{
		Status: "administrator",
		User:   gotgbot.User{Id: 11, FirstName: "List Admin"},
	}
	adminCache := AdminCache{
		ChatId:   chatID,
		UserInfo: []gotgbot.MergedChatMember{adminViaList},
		UserMap:  map[int64]gotgbot.MergedChatMember{10: adminViaMap},
		Cached:   true,
	}

	if err := GetMarshal().Set(Context, fmt.Sprintf("alita:adminCache:%d", chatID), adminCache); err != nil {
		t.Fatalf("cache set: %v", err)
	}

	found, gotCache := GetAdminCacheList(chatID)
	if !found || !gotCache.Cached || gotCache.ChatId != chatID {
		t.Fatalf("GetAdminCacheList() = (%v, %+v), want cached chat %d", found, gotCache, chatID)
	}

	found, gotMember := GetAdminCacheUser(chatID, 10)
	if !found || gotMember.User.Id != 10 {
		t.Fatalf("GetAdminCacheUser(map user) = (%v, %+v), want user 10", found, gotMember)
	}

	found, gotMember = GetAdminCacheUser(chatID, 11)
	if !found || gotMember.User.Id != 11 {
		t.Fatalf("GetAdminCacheUser(list fallback) = (%v, %+v), want user 11", found, gotMember)
	}

	found, gotMember = GetAdminCacheUser(chatID, 42)
	if found || gotMember.User.Id != 0 {
		t.Fatalf("GetAdminCacheUser(missing) = (%v, %+v), want miss", found, gotMember)
	}

	InvalidateAdminCache(chatID)
	if found, _ := GetAdminCacheList(chatID); found {
		t.Fatal("GetAdminCacheList() found cache after InvalidateAdminCache")
	}
}

func TestRestrictedCacheUsesMemoryStoreWithoutRedis(t *testing.T) {
	withMemoryMarshaler(t)

	const chatID = int64(-100456)
	MarkChatRestricted(chatID)
	if !IsChatRestricted(chatID) {
		t.Fatal("IsChatRestricted() = false after MarkChatRestricted")
	}

	MarkChatNotRestricted(chatID)
	if IsChatRestricted(chatID) {
		t.Fatal("IsChatRestricted() = true after MarkChatNotRestricted")
	}
}

func TestRedisAccessorsWhenRedisIsNotInitialized(t *testing.T) {
	withMemoryMarshaler(t)

	if IsRedisAvailable() {
		t.Fatal("IsRedisAvailable() = true, want false without Redis client")
	}
	if got := GetRedisClient(); got != nil {
		t.Fatalf("GetRedisClient() = %#v, want nil", got)
	}
	if err := ClearAllCaches(); err == nil {
		t.Fatal("ClearAllCaches() error = nil, want redis client not initialized")
	}
}

func TestIsChatRestrictedAllowsMalformedAndStaleEntriesWithMemoryStore(t *testing.T) {
	withMemoryMarshaler(t)

	const malformedChatID = int64(-100457)
	if err := GetMarshal().Set(Context, restrictedChatKey(malformedChatID), "not-a-timestamp"); err != nil {
		t.Fatalf("cache set malformed: %v", err)
	}
	if IsChatRestricted(malformedChatID) {
		t.Fatal("IsChatRestricted(malformed timestamp) = true, want false")
	}

	const staleChatID = int64(-100458)
	stale := time.Now().Add(-constants.RestrictedProbeInterval - time.Second).Format(time.RFC3339)
	if err := GetMarshal().Set(Context, restrictedChatKey(staleChatID), stale); err != nil {
		t.Fatalf("cache set stale: %v", err)
	}
	if IsChatRestricted(staleChatID) {
		t.Fatal("IsChatRestricted(stale timestamp without Redis lock) = true, want false")
	}
}
