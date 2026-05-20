package cache

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	gocache "github.com/eko/gocache/lib/v4/cache"
	"github.com/eko/gocache/lib/v4/marshaler"
	"github.com/eko/gocache/lib/v4/store"

	"github.com/divkix/Alita_Robot/alita/utils/constants"
)

type cacheMemoryStore struct {
	mu   sync.RWMutex
	data map[string][]byte
	ttls map[string]time.Time
}

func newCacheMemoryStore() *cacheMemoryStore {
	return &cacheMemoryStore{
		data: make(map[string][]byte),
		ttls: make(map[string]time.Time),
	}
}

func (m *cacheMemoryStore) Get(_ context.Context, key any) (any, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	k := fmt.Sprint(key)
	if expiry, ok := m.ttls[k]; ok && time.Now().After(expiry) {
		return nil, fmt.Errorf("key expired")
	}
	v, ok := m.data[k]
	if !ok {
		return nil, fmt.Errorf("key not found")
	}
	return v, nil
}

func (m *cacheMemoryStore) GetWithTTL(_ context.Context, key any) (any, time.Duration, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	k := fmt.Sprint(key)
	v, ok := m.data[k]
	if !ok {
		return nil, 0, fmt.Errorf("key not found")
	}
	if expiry, ok := m.ttls[k]; ok {
		ttl := time.Until(expiry)
		if ttl < 0 {
			return nil, 0, fmt.Errorf("key expired")
		}
		return v, ttl, nil
	}
	return v, 0, nil
}

func (m *cacheMemoryStore) Set(_ context.Context, key, value any, options ...store.Option) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	k := fmt.Sprint(key)
	switch v := value.(type) {
	case []byte:
		m.data[k] = v
	case string:
		m.data[k] = []byte(v)
	default:
		return fmt.Errorf("unsupported value type %T", value)
	}
	if expiration := store.ApplyOptions(options...).Expiration; expiration > 0 {
		m.ttls[k] = time.Now().Add(expiration)
	} else {
		delete(m.ttls, k)
	}
	return nil
}

func (m *cacheMemoryStore) Delete(_ context.Context, key any) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	k := fmt.Sprint(key)
	delete(m.data, k)
	delete(m.ttls, k)
	return nil
}

func (m *cacheMemoryStore) Invalidate(context.Context, ...store.InvalidateOption) error {
	return nil
}

func (m *cacheMemoryStore) Clear(context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.data = make(map[string][]byte)
	m.ttls = make(map[string]time.Time)
	return nil
}

func (m *cacheMemoryStore) GetType() string {
	return "cache-memory-test"
}

func withMemoryMarshaler(t *testing.T) {
	t.Helper()

	previousMarshal := Marshal
	previousManager := Manager
	previousRedisClient := redisClient

	manager := gocache.New[any](newCacheMemoryStore())
	Manager = manager
	Marshal = marshaler.New(manager)
	redisClient = nil

	t.Cleanup(func() {
		Marshal = previousMarshal
		Manager = previousManager
		redisClient = previousRedisClient
	})
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

	if err := Marshal.Set(Context, fmt.Sprintf("alita:adminCache:%d", chatID), adminCache); err != nil {
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

func TestIsChatRestrictedAllowsMalformedAndStaleEntriesWithMemoryStore(t *testing.T) {
	withMemoryMarshaler(t)

	const malformedChatID = int64(-100457)
	if err := Marshal.Set(Context, restrictedChatKey(malformedChatID), "not-a-timestamp"); err != nil {
		t.Fatalf("cache set malformed: %v", err)
	}
	if IsChatRestricted(malformedChatID) {
		t.Fatal("IsChatRestricted(malformed timestamp) = true, want false")
	}

	const staleChatID = int64(-100458)
	stale := time.Now().Add(-constants.RestrictedProbeInterval - time.Second).Format(time.RFC3339)
	if err := Marshal.Set(Context, restrictedChatKey(staleChatID), stale); err != nil {
		t.Fatalf("cache set stale: %v", err)
	}
	if IsChatRestricted(staleChatID) {
		t.Fatal("IsChatRestricted(stale timestamp without Redis lock) = true, want false")
	}
}
