//go:build testtools

package ratelimit

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/eko/gocache/lib/v4/store"

	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

func TestFormatCooldown(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{"30 seconds", 30 * time.Second, "30 seconds"},
		{"1 minute 30 seconds", 90 * time.Second, "1 minute 30 seconds"},
		{"5 minutes", 5 * time.Minute, "5 minutes"},
		{"1 hour", 1 * time.Hour, "1 hour"},
		{"1 hour 30 minutes", 1*time.Hour + 30*time.Minute, "1 hour 30 minutes"},
		{"1 hour 1 minute", 1*time.Hour + time.Minute, "1 hour 1 minute"},
		{"1 second", time.Second, "1 second"},
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
	// Save original limiter and once for restoration.
	origBackupLimiter := backupLimiter
	origOnce := once

	t.Cleanup(func() {
		backupLimiter = origBackupLimiter
		once = origOnce
	})

	// Reset the singleton state for a clean test.
	once = &sync.Once{}
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

func TestBackupRateLimiter_CanMethods_NilCache(t *testing.T) {
	tests := []struct {
		name string
		fn   func(*BackupRateLimiter, int64) (bool, time.Duration)
	}{
		{"CanImport", func(l *BackupRateLimiter, id int64) (bool, time.Duration) { return l.CanImport(id) }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			limiter := &BackupRateLimiter{}
			allowed, remaining := tc.fn(limiter, 12345)
			if !allowed {
				t.Error("expected allowed=true when cache is nil")
			}
			if remaining != 0 {
				t.Errorf("expected remaining=0 when cache is nil, got %v", remaining)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// recordOperation / getLastOperation with in-memory cache
// ---------------------------------------------------------------------------

func TestBackupRateLimiter_recordOperationAndGetLastOperation(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)

	limiter := &BackupRateLimiter{}
	const chatID = int64(99901)
	cacheKey := exportRatePrefix + fmt.Sprint(chatID)

	// No operation recorded yet → getLastOperation should error.
	_, err := limiter.getLastOperation(cacheKey)
	if err == nil {
		t.Fatal("expected error for missing key")
	}

	// Record an operation.
	limiter.recordOperation(cacheKey, DefaultExportCooldown)

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
	limiter.recordOperation(cacheKey, DefaultExportCooldown)
	ts2, err := limiter.getLastOperation(cacheKey)
	if err != nil {
		t.Fatalf("unexpected error on second record: %v", err)
	}
	if ts2.Before(before) {
		t.Error("expected updated timestamp not to be before time of second record")
	}
}

// ---------------------------------------------------------------------------
// CanImport with working cache
// ---------------------------------------------------------------------------

func TestBackupRateLimiter_AcquireExportIsAtomic(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)

	limiter := &BackupRateLimiter{}
	const chatID = int64(99906)
	const contenders = 20
	start := make(chan struct{})
	results := make(chan bool, contenders)
	var wg sync.WaitGroup
	for range contenders {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			allowed, _ := limiter.AcquireExport(chatID)
			results <- allowed
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	acquired := 0
	for allowed := range results {
		if allowed {
			acquired++
		}
	}
	if acquired != 1 {
		t.Fatalf("successful acquisitions = %d, want 1", acquired)
	}

	if allowed, _ := limiter.AcquireExport(chatID); allowed {
		t.Fatal("AcquireExport allowed a second reservation")
	}
}

func TestBackupRateLimiter_CanImport_AllowedThenBlocked(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)

	limiter := &BackupRateLimiter{}
	const chatID = int64(99903)

	allowed, remaining := limiter.CanImport(chatID)
	if !allowed {
		t.Fatal("expected CanImport to be allowed on first call")
	}
	if remaining != 0 {
		t.Errorf("expected remaining=0 on first call, got %v", remaining)
	}

	limiter.AcquireImport(chatID)

	allowed, remaining = limiter.CanImport(chatID)
	if allowed {
		t.Fatal("expected CanImport to be blocked immediately after AcquireImport")
	}
	if remaining <= 0 || remaining > DefaultImportCooldown {
		t.Errorf("expected remaining in (0, %v], got %v", DefaultImportCooldown, remaining)
	}
}

func TestBackupRateLimiter_CanImport_AfterCooldown(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)

	limiter := &BackupRateLimiter{}
	const chatID = int64(99906)
	cacheKey := importRatePrefix + fmt.Sprint(chatID)

	past := time.Now().Add(-DefaultImportCooldown - time.Second)
	if err := cache.GetMarshal().Set(context.Background(), cacheKey, past, store.WithExpiration(DefaultImportCooldown)); err != nil {
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

func TestBackupRateLimiter_recordOperation_UnknownPrefix(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)

	limiter := &BackupRateLimiter{}
	cacheKey := "backup:unknown:12345"

	// Should not panic and should store with the provided 1-hour TTL.
	limiter.recordOperation(cacheKey, time.Hour)

	ts, err := limiter.getLastOperation(cacheKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ts.IsZero() {
		t.Error("expected non-zero timestamp for unknown-prefix key")
	}
}

func TestBackupRateLimiter_getLastOperation_CacheError(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)

	limiter := &BackupRateLimiter{}
	const chatID = int64(99908)
	cacheKey := exportRatePrefix + fmt.Sprint(chatID)

	// Seed cache with non-time data so unmarshalling fails.
	if err := cache.GetMarshal().Set(context.Background(), cacheKey, "not-a-time"); err != nil {
		t.Fatalf("failed to seed cache: %v", err)
	}

	_, err := limiter.getLastOperation(cacheKey)
	if err == nil {
		t.Fatal("expected error when cached value is not a time.Time")
	}
}
