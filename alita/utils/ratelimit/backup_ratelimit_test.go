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
	origBackupLimiter := backupLimiter
	origOnce := once

	t.Cleanup(func() {
		backupLimiter = origBackupLimiter
		once = origOnce
	})

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

func TestBackupRateLimiter_AcquireMethods_NilCache(t *testing.T) {
	tests := []struct {
		name string
		fn   func(*BackupRateLimiter, int64) (bool, time.Duration)
	}{
		{"AcquireImport", func(l *BackupRateLimiter, id int64) (bool, time.Duration) { return l.AcquireImport(id) }},
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

func TestBackupRateLimiter_AcquireImport_AllowedThenBlocked(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)

	limiter := &BackupRateLimiter{}
	const chatID = int64(99903)

	allowed, remaining := limiter.AcquireImport(chatID)
	if !allowed {
		t.Fatal("expected AcquireImport to be allowed on first call")
	}
	if remaining != 0 {
		t.Errorf("expected remaining=0 on first call, got %v", remaining)
	}

	allowed, remaining = limiter.AcquireImport(chatID)
	if allowed {
		t.Fatal("expected AcquireImport to be blocked immediately after a reservation")
	}
	if remaining <= 0 || remaining > DefaultImportCooldown {
		t.Errorf("expected remaining in (0, %v], got %v", DefaultImportCooldown, remaining)
	}
}

func TestBackupRateLimiter_AcquireImport_AfterCooldown(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)

	limiter := &BackupRateLimiter{}
	const chatID = int64(99906)
	cacheKey := importRatePrefix + fmt.Sprint(chatID)

	past := time.Now().Add(-DefaultImportCooldown - time.Second)
	if err := cache.GetMarshal().Set(context.Background(), cacheKey, past, store.WithExpiration(DefaultImportCooldown)); err != nil {
		t.Fatalf("failed to seed cache: %v", err)
	}

	allowed, remaining := limiter.AcquireImport(chatID)
	if !allowed {
		t.Fatalf("expected AcquireImport to be allowed after cooldown, got remaining=%v", remaining)
	}
	if remaining != 0 {
		t.Errorf("expected remaining=0 after cooldown, got %v", remaining)
	}
}
