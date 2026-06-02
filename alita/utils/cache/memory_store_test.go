//go:build testtools

package cache

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/eko/gocache/lib/v4/store"

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

func TestCacheMemoryStore_GetWithTTLAndLifecycle(t *testing.T) {
	ctx := context.Background()
	mem := newCacheMemoryStore()
	const key = "lifecycle-key"

	if _, _, err := mem.GetWithTTL(ctx, key); err == nil {
		t.Fatal("GetWithTTL() error = nil, want not found")
	}

	if err := mem.Set(ctx, key, []byte("payload"), store.WithExpiration(time.Minute)); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if err := mem.Set(ctx, "unsupported", 123, nil); err == nil {
		t.Fatal("Set(unsupported type) error = nil, want unsupported type error")
	}

	got, ttl, err := mem.GetWithTTL(ctx, key)
	if err != nil {
		t.Fatalf("GetWithTTL() error = %v", err)
	}
	if string(got.([]byte)) != "payload" {
		t.Fatalf("GetWithTTL() value = %v, want payload", got)
	}
	if ttl <= 0 {
		t.Fatalf("GetWithTTL() ttl = %v, want positive duration", ttl)
	}

	const expiredKey = "expired-key"
	if err := mem.Set(ctx, expiredKey, []byte("old"), store.WithExpiration(time.Millisecond)); err != nil {
		t.Fatalf("Set(expired) error = %v", err)
	}
	time.Sleep(2 * time.Millisecond)
	if _, _, err := mem.GetWithTTL(ctx, expiredKey); err == nil {
		t.Fatal("GetWithTTL(expired) error = nil, want expired error")
	}
	if _, err := mem.Get(ctx, expiredKey); err == nil {
		t.Fatal("Get(expired) error = nil, want expired error")
	}

	if err := mem.Invalidate(ctx); err != nil {
		t.Fatalf("Invalidate() error = %v", err)
	}
	if err := mem.Clear(ctx); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}
	if _, err := mem.Get(ctx, key); err == nil {
		t.Fatal("Get() after Clear() error = nil, want not found")
	}
	if mem.GetType() != "cache-memory-test" {
		t.Fatalf("GetType() = %q, want cache-memory-test", mem.GetType())
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
