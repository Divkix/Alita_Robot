package cache

import (
	"fmt"
	"os"
	"testing"

	"github.com/divkix/Alita_Robot/alita/config"
)

// TestMain for the cache package: tries to initialize the cache with a real Redis
// connection. If Redis is not available, tests that call skipIfNoCache are skipped,
// but pure-logic tests (key format, nil safety) still run.
func TestMain(m *testing.M) {
	// Attempt to initialize the cache using any available Redis configuration.
	// Allow override via environment (REDIS_ADDRESS).
	if addr := os.Getenv("REDIS_ADDRESS"); addr != "" {
		config.AppConfig.RedisAddress = addr
	}
	if config.AppConfig.RedisAddress == "" {
		config.AppConfig.RedisAddress = "localhost:6379"
	}

	if err := InitCache(); err != nil {
		fmt.Printf("[cache tests] Redis not available (%v) — Redis-dependent tests will be skipped\n", err)
		// Marshal remains nil; skipIfNoCache() will skip Redis-dependent tests.
		// Pure-logic tests (key format, nil safety) still run.
	}

	os.Exit(m.Run())
}

// skipIfNoCache skips the current test when the cache is not initialized.
func skipIfNoCache(t *testing.T) {
	t.Helper()
	if Marshal == nil {
		t.Skip("requires Redis connection")
	}
}

// ---------------------------------------------------------------------------
// restrictedChatKey
// ---------------------------------------------------------------------------

func TestRestrictedCacheKey_Format(t *testing.T) {
	t.Parallel()

	cases := []struct {
		chatID   int64
		expected string
	}{
		{-1001618764357, "alita:restricted:-1001618764357"},
		{123456789, "alita:restricted:123456789"},
		{0, "alita:restricted:0"},
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("chatID=%d", tc.chatID), func(t *testing.T) {
			t.Parallel()
			got := restrictedChatKey(tc.chatID)
			if got != tc.expected {
				t.Errorf("restrictedChatKey(%d) = %q, want %q", tc.chatID, got, tc.expected)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GetRestrictedCacheStats — no Redis needed (atomic counters)
// ---------------------------------------------------------------------------

func TestGetRestrictedCacheStats_BothCounters(t *testing.T) {
	skipIfNoCache(t)

	// Use a unique chat ID to avoid collisions with parallel tests.
	const chatA = int64(-10099901)
	const chatB = int64(-10099902)

	// Record baseline to isolate this test from others.
	baseHits, baseMisses := GetRestrictedCacheStats()

	// Mark chatA restricted, then check it → hit.
	MarkChatRestricted(chatA)
	defer MarkChatNotRestricted(chatA)

	if !IsChatRestricted(chatA) {
		t.Fatal("IsChatRestricted(chatA) should return true after MarkChatRestricted")
	}

	// Check chatB (never marked) → miss.
	if IsChatRestricted(chatB) {
		t.Fatal("IsChatRestricted(chatB) should return false for unknown chat")
	}

	hits, misses := GetRestrictedCacheStats()
	if hits-baseHits < 1 {
		t.Errorf("expected at least 1 new hit, got delta=%d", hits-baseHits)
	}
	if misses-baseMisses < 1 {
		t.Errorf("expected at least 1 new miss, got delta=%d", misses-baseMisses)
	}
}

// ---------------------------------------------------------------------------
// MarkChatRestricted / IsChatRestricted
// ---------------------------------------------------------------------------

func TestMarkChatRestricted(t *testing.T) {
	skipIfNoCache(t)

	const chatID = int64(-1001618764357)
	defer MarkChatNotRestricted(chatID)

	MarkChatRestricted(chatID)

	if !IsChatRestricted(chatID) {
		t.Errorf("IsChatRestricted(%d) = false, want true after MarkChatRestricted", chatID)
	}
}

func TestMarkChatNotRestricted(t *testing.T) {
	skipIfNoCache(t)

	const chatID = int64(-10099910)

	MarkChatRestricted(chatID)
	MarkChatNotRestricted(chatID)

	if IsChatRestricted(chatID) {
		t.Errorf("IsChatRestricted(%d) = true after MarkChatNotRestricted, want false", chatID)
	}
}

func TestIsChatRestricted_Miss(t *testing.T) {
	skipIfNoCache(t)

	const chatID = int64(-10099920) // never marked

	if IsChatRestricted(chatID) {
		t.Errorf("IsChatRestricted(%d) = true for never-marked chat, want false", chatID)
	}
}

func TestMarkChatRestricted_Idempotent(t *testing.T) {
	skipIfNoCache(t)

	const chatID = int64(-10099930)
	defer MarkChatNotRestricted(chatID)

	MarkChatRestricted(chatID)
	MarkChatRestricted(chatID) // second call must not cause an error or flip state

	if !IsChatRestricted(chatID) {
		t.Errorf("IsChatRestricted(%d) = false after double MarkChatRestricted, want true", chatID)
	}
}

// ---------------------------------------------------------------------------
// Stats counters
// ---------------------------------------------------------------------------

func TestIsChatRestricted_StatsIncrementHit(t *testing.T) {
	skipIfNoCache(t)

	const chatID = int64(-10099940)
	MarkChatRestricted(chatID)
	defer MarkChatNotRestricted(chatID)

	baseHits, _ := GetRestrictedCacheStats()

	if !IsChatRestricted(chatID) {
		t.Fatal("IsChatRestricted should return true for marked chat")
	}

	hits, _ := GetRestrictedCacheStats()
	if hits-baseHits != 1 {
		t.Errorf("expected hits delta=1, got %d", hits-baseHits)
	}
}

func TestIsChatRestricted_StatsIncrementMiss(t *testing.T) {
	skipIfNoCache(t)

	const chatID = int64(-10099950) // never marked

	_, baseMisses := GetRestrictedCacheStats()

	if IsChatRestricted(chatID) {
		t.Fatal("IsChatRestricted should return false for unknown chat")
	}

	_, misses := GetRestrictedCacheStats()
	if misses-baseMisses != 1 {
		t.Errorf("expected misses delta=1, got %d", misses-baseMisses)
	}
}

// ---------------------------------------------------------------------------
// Nil-safety: functions must not panic when Marshal is nil
// ---------------------------------------------------------------------------

func TestNilMarshal_NoOp(t *testing.T) {
	t.Parallel()

	// Save and restore Marshal.
	orig := Marshal
	Marshal = nil
	defer func() { Marshal = orig }()

	// None of these should panic.
	MarkChatRestricted(-999)
	MarkChatNotRestricted(-999)

	if IsChatRestricted(-999) {
		t.Error("IsChatRestricted with nil Marshal should return false")
	}
}
