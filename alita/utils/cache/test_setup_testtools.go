//go:build testtools

package cache

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	gocache "github.com/eko/gocache/lib/v4/cache"
	"github.com/eko/gocache/lib/v4/marshaler"
	"github.com/eko/gocache/lib/v4/store"
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

// InitTestMarshal installs an in-memory cache marshaler for package test suites.
// The returned function restores the previous cache state.
func InitTestMarshal() func() {
	previousMarshal := GetMarshal()
	previousManager := Manager
	previousRedisClient := redisClient

	manager := gocache.New[any](newCacheMemoryStore())
	Manager = manager
	SetMarshal(marshaler.New(manager))
	redisClient = nil

	return func() {
		SetMarshal(previousMarshal)
		Manager = previousManager
		redisClient = previousRedisClient
	}
}

// SetupTestMemoryMarshaler installs an in-memory cache marshaler for the duration of a test.
func SetupTestMemoryMarshaler(t *testing.T) {
	t.Helper()

	previousMarshal := GetMarshal()
	previousManager := Manager
	previousRedisClient := redisClient

	manager := gocache.New[any](newCacheMemoryStore())
	Manager = manager
	SetMarshal(marshaler.New(manager))
	redisClient = nil

	t.Cleanup(func() {
		SetMarshal(previousMarshal)
		Manager = previousManager
		redisClient = previousRedisClient
	})
}
