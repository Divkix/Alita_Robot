package ratelimit

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/utils/cache"
	gocache "github.com/eko/gocache/lib/v4/cache"
	"github.com/eko/gocache/lib/v4/marshaler"
	"github.com/eko/gocache/lib/v4/store"
)

func TestFormatCooldown(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{"30 seconds", 30 * time.Second, "30 seconds"},
		{"1 minute 30 seconds", 90 * time.Second, "1 minutes 30 seconds"},
		{"5 minutes", 5 * time.Minute, "5 minutes"},
		{"1 hour", 1 * time.Hour, "1 hours"},
		{"1 hour 30 minutes", 1*time.Hour + 30*time.Minute, "1 hours 30 minutes"},
		{"0 seconds", 0, "0 seconds"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatCooldown(tt.duration)
			if got != tt.want {
				t.Errorf("FormatCooldown(%v) = %q, want %q", tt.duration, got, tt.want)
			}
		})
	}
}

func TestGetBackupRateLimiter_Singleton(t *testing.T) {
	// Reset the singleton state for a clean test.
	once = sync.Once{}
	backupLimiter = nil

	first := GetBackupRateLimiter()
	second := GetBackupRateLimiter()

	if first == nil {
		t.Fatal("GetBackupRateLimiter() returned nil")
	}
	if first != second {
		t.Error("GetBackupRateLimiter() returned different instances")
	}
}

func TestBackupRateLimiter_CanExport_NilCache(t *testing.T) {
	limiter := &BackupRateLimiter{}
	allowed, remaining := limiter.CanExport(12345)
	if !allowed {
		t.Error("expected allowed=true when cache is nil")
	}
	if remaining != 0 {
		t.Errorf("expected remaining=0 when cache is nil, got %v", remaining)
	}
}

func TestBackupRateLimiter_CanImport_NilCache(t *testing.T) {
	limiter := &BackupRateLimiter{}
	allowed, remaining := limiter.CanImport(12345)
	if !allowed {
		t.Error("expected allowed=true when cache is nil")
	}
	if remaining != 0 {
		t.Errorf("expected remaining=0 when cache is nil, got %v", remaining)
	}
}

func TestBackupRateLimiter_CanReset_NilCache(t *testing.T) {
	limiter := &BackupRateLimiter{}
	allowed, remaining := limiter.CanReset(12345)
	if !allowed {
		t.Error("expected allowed=true when cache is nil")
	}
	if remaining != 0 {
		t.Errorf("expected remaining=0 when cache is nil, got %v", remaining)
	}
}

// ---------------------------------------------------------------------------
// Record methods with nil cache — must not panic
// ---------------------------------------------------------------------------

func TestBackupRateLimiter_RecordExport_NilCache_NoPanic(t *testing.T) {
	limiter := &BackupRateLimiter{}
	limiter.RecordExport(12345)
}

func TestBackupRateLimiter_RecordImport_NilCache_NoPanic(t *testing.T) {
	limiter := &BackupRateLimiter{}
	limiter.RecordImport(12345)
}

func TestBackupRateLimiter_RecordReset_NilCache_NoPanic(t *testing.T) {
	limiter := &BackupRateLimiter{}
	limiter.RecordReset(12345)
}

// ---------------------------------------------------------------------------
// In-memory mock store for cache-backed tests
// ---------------------------------------------------------------------------

// memoryStore is a simple in-memory implementation of store.StoreInterface.
type memoryStore struct {
	mu    sync.RWMutex
	data  map[string][]byte
	ttls  map[string]time.Time
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		data: make(map[string][]byte),
		ttls: make(map[string]time.Time),
	}
}

func (m *memoryStore) Get(_ context.Context, key any) (any, error) {
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

func (m *memoryStore) GetWithTTL(_ context.Context, key any) (any, time.Duration, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	k := fmt.Sprint(key)
	v, ok := m.data[k]
	if !ok {
		return nil, 0, fmt.Errorf("key not found")
	}
	var ttl time.Duration
	if expiry, ok := m.ttls[k]; ok {
		ttl = time.Until(expiry)
		if ttl < 0 {
			return nil, 0, fmt.Errorf("key expired")
		}
	}
	return v, ttl, nil
}

func (m *memoryStore) Set(_ context.Context, key, value any, options ...store.Option) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := fmt.Sprint(key)
	opts := store.ApplyOptions(options...)
	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return fmt.Errorf("unsupported value type %T", value)
	}
	m.data[k] = data
	if opts.Expiration > 0 {
		m.ttls[k] = time.Now().Add(opts.Expiration)
	} else {
		delete(m.ttls, k)
	}
	return nil
}

func (m *memoryStore) Delete(_ context.Context, key any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := fmt.Sprint(key)
	delete(m.data, k)
	delete(m.ttls, k)
	return nil
}

func (m *memoryStore) Invalidate(_ context.Context, _ ...store.InvalidateOption) error {
	return nil
}

func (m *memoryStore) Clear(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data = make(map[string][]byte)
	m.ttls = make(map[string]time.Time)
	return nil
}

func (m *memoryStore) GetType() string {
	return "memory"
}

// setupTestCache sets cache.Marshal to an in-memory marshaler and returns a cleanup func.
func setupTestCache(t *testing.T) func() {
	t.Helper()
	orig := cache.Marshal
	mem := newMemoryStore()
	cm := gocache.New[any](mem)
	cache.Marshal = marshaler.New(cm)
	return func() {
		cache.Marshal = orig
	}
}

// ---------------------------------------------------------------------------
// recordOperation / getLastOperation with in-memory cache
// ---------------------------------------------------------------------------

func TestBackupRateLimiter_recordOperationAndGetLastOperation(t *testing.T) {
	cleanup := setupTestCache(t)
	defer cleanup()

	limiter := &BackupRateLimiter{}
	const chatID = int64(99901)
	cacheKey := exportRatePrefix + fmt.Sprint(chatID)

	// No operation recorded yet → getLastOperation should error.
	_, err := limiter.getLastOperation(cacheKey)
	if err == nil {
		t.Fatal("expected error for missing key")
	}

	// Record an operation.
	limiter.recordOperation(cacheKey)

	// Now getLastOperation should succeed.
	ts, err := limiter.getLastOperation(cacheKey)
	if err != nil {
		t.Fatalf("unexpected error after record: %v", err)
	}
	if ts.IsZero() {
		t.Error("expected non-zero timestamp")
	}

	// Record again; timestamp should be updated (or at least not older).
	before := time.Now()
	limiter.recordOperation(cacheKey)
	ts2, err := limiter.getLastOperation(cacheKey)
	if err != nil {
		t.Fatalf("unexpected error on second record: %v", err)
	}
	if ts2.Before(before) {
		t.Error("expected updated timestamp not to be before time of second record")
	}
}

// ---------------------------------------------------------------------------
// CanExport / CanImport / CanReset with working cache
// ---------------------------------------------------------------------------

func TestBackupRateLimiter_CanExport_AllowedThenBlocked(t *testing.T) {
	cleanup := setupTestCache(t)
	defer cleanup()

	limiter := &BackupRateLimiter{}
	const chatID = int64(99902)

	// First call — should be allowed (no previous export).
	allowed, remaining := limiter.CanExport(chatID)
	if !allowed {
		t.Fatal("expected CanExport to be allowed on first call")
	}
	if remaining != 0 {
		t.Errorf("expected remaining=0 on first call, got %v", remaining)
	}

	// Record the export.
	limiter.RecordExport(chatID)

	// Second call immediately — should be blocked.
	allowed, remaining = limiter.CanExport(chatID)
	if allowed {
		t.Fatal("expected CanExport to be blocked immediately after RecordExport")
	}
	if remaining <= 0 || remaining > DefaultExportCooldown {
		t.Errorf("expected remaining in (0, %v], got %v", DefaultExportCooldown, remaining)
	}
}

func TestBackupRateLimiter_CanImport_AllowedThenBlocked(t *testing.T) {
	cleanup := setupTestCache(t)
	defer cleanup()

	limiter := &BackupRateLimiter{}
	const chatID = int64(99903)

	allowed, remaining := limiter.CanImport(chatID)
	if !allowed {
		t.Fatal("expected CanImport to be allowed on first call")
	}
	if remaining != 0 {
		t.Errorf("expected remaining=0 on first call, got %v", remaining)
	}

	limiter.RecordImport(chatID)

	allowed, remaining = limiter.CanImport(chatID)
	if allowed {
		t.Fatal("expected CanImport to be blocked immediately after RecordImport")
	}
	if remaining <= 0 || remaining > DefaultImportCooldown {
		t.Errorf("expected remaining in (0, %v], got %v", DefaultImportCooldown, remaining)
	}
}

func TestBackupRateLimiter_CanReset_AllowedThenBlocked(t *testing.T) {
	cleanup := setupTestCache(t)
	defer cleanup()

	limiter := &BackupRateLimiter{}
	const chatID = int64(99904)

	allowed, remaining := limiter.CanReset(chatID)
	if !allowed {
		t.Fatal("expected CanReset to be allowed on first call")
	}
	if remaining != 0 {
		t.Errorf("expected remaining=0 on first call, got %v", remaining)
	}

	limiter.RecordReset(chatID)

	allowed, remaining = limiter.CanReset(chatID)
	if allowed {
		t.Fatal("expected CanReset to be blocked immediately after RecordReset")
	}
	if remaining <= 0 || remaining > DefaultResetCooldown {
		t.Errorf("expected remaining in (0, %v], got %v", DefaultResetCooldown, remaining)
	}
}

func TestBackupRateLimiter_CanExport_AfterCooldown(t *testing.T) {
	cleanup := setupTestCache(t)
	defer cleanup()

	limiter := &BackupRateLimiter{}
	const chatID = int64(99905)
	cacheKey := exportRatePrefix + fmt.Sprint(chatID)

	// Manually seed cache with a timestamp well in the past.
	past := time.Now().Add(-DefaultExportCooldown - time.Second)
	// marshaler stores time values via msgpack, so we must store via marshaler
	if err := cache.Marshal.Set(context.Background(), cacheKey, past, store.WithExpiration(DefaultExportCooldown)); err != nil {
		t.Fatalf("failed to seed cache: %v", err)
	}

	allowed, remaining := limiter.CanExport(chatID)
	if !allowed {
		t.Fatalf("expected CanExport to be allowed after cooldown, got remaining=%v", remaining)
	}
	if remaining != 0 {
		t.Errorf("expected remaining=0 after cooldown, got %v", remaining)
	}
}

func TestBackupRateLimiter_CanImport_AfterCooldown(t *testing.T) {
	cleanup := setupTestCache(t)
	defer cleanup()

	limiter := &BackupRateLimiter{}
	const chatID = int64(99906)
	cacheKey := importRatePrefix + fmt.Sprint(chatID)

	past := time.Now().Add(-DefaultImportCooldown - time.Second)
	if err := cache.Marshal.Set(context.Background(), cacheKey, past, store.WithExpiration(DefaultImportCooldown)); err != nil {
		t.Fatalf("failed to seed cache: %v", err)
	}

	allowed, remaining := limiter.CanImport(chatID)
	if !allowed {
		t.Fatalf("expected CanImport to be allowed after cooldown, got remaining=%v", remaining)
	}
	if remaining != 0 {
		t.Errorf("expected remaining=0 after cooldown, got %v", remaining)
	}
}

func TestBackupRateLimiter_CanReset_AfterCooldown(t *testing.T) {
	cleanup := setupTestCache(t)
	defer cleanup()

	limiter := &BackupRateLimiter{}
	const chatID = int64(99907)
	cacheKey := resetRatePrefix + fmt.Sprint(chatID)

	past := time.Now().Add(-DefaultResetCooldown - time.Second)
	if err := cache.Marshal.Set(context.Background(), cacheKey, past, store.WithExpiration(DefaultResetCooldown)); err != nil {
		t.Fatalf("failed to seed cache: %v", err)
	}

	allowed, remaining := limiter.CanReset(chatID)
	if !allowed {
		t.Fatalf("expected CanReset to be allowed after cooldown, got remaining=%v", remaining)
	}
	if remaining != 0 {
		t.Errorf("expected remaining=0 after cooldown, got %v", remaining)
	}
}

func TestBackupRateLimiter_recordOperation_UnknownPrefix(t *testing.T) {
	cleanup := setupTestCache(t)
	defer cleanup()

	limiter := &BackupRateLimiter{}
	cacheKey := "backup:unknown:12345"

	// Should not panic and should store with default 1-hour TTL.
	limiter.recordOperation(cacheKey)

	ts, err := limiter.getLastOperation(cacheKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ts.IsZero() {
		t.Error("expected non-zero timestamp for unknown-prefix key")
	}
}

func TestBackupRateLimiter_getLastOperation_CacheError(t *testing.T) {
	cleanup := setupTestCache(t)
	defer cleanup()

	limiter := &BackupRateLimiter{}
	const chatID = int64(99908)
	cacheKey := exportRatePrefix + fmt.Sprint(chatID)

	// Seed cache with non-time data so unmarshalling fails.
	if err := cache.Marshal.Set(context.Background(), cacheKey, "not-a-time"); err != nil {
		t.Fatalf("failed to seed cache: %v", err)
	}

	_, err := limiter.getLastOperation(cacheKey)
	if err == nil {
		t.Fatal("expected error when cached value is not a time.Time")
	}
}
