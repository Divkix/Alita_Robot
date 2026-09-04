package cache

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/eko/gocache/lib/v4/store"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/utils/constants"
)

func TestMain(m *testing.M) {
	redisServer, err := miniredis.Run()
	if err != nil {
		fmt.Printf("[cache tests] start miniredis: %v\n", err)
		os.Exit(1)
	}
	config.AppConfig.RedisAddress = redisServer.Addr()
	config.AppConfig.RedisURL = ""
	config.AppConfig.RedisPassword = ""
	config.AppConfig.RedisDB = 0

	if err := InitCache(); err != nil {
		fmt.Printf("[cache tests] initialize cache: %v\n", err)
		redisServer.Close()
		os.Exit(1)
	}

	exitCode := m.Run()
	if client := GetRedisClient(); client != nil {
		_ = client.Close()
	}
	redisServer.Close()
	os.Exit(exitCode)
}

func skipIfNoCache(t *testing.T) {
	t.Helper()
	if GetMarshal() == nil {
		t.Skip("requires Redis connection")
	}
}

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

func TestRestrictedProbeKey_Format(t *testing.T) {
	t.Parallel()

	cases := []struct {
		chatID   int64
		expected string
	}{
		{-1001618764357, "alita:restricted_probe:-1001618764357"},
		{123456789, "alita:restricted_probe:123456789"},
		{0, "alita:restricted_probe:0"},
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("chatID=%d", tc.chatID), func(t *testing.T) {
			t.Parallel()
			got := restrictedProbeKey(tc.chatID)
			if got != tc.expected {
				t.Errorf("restrictedProbeKey(%d) = %q, want %q", tc.chatID, got, tc.expected)
			}
		})
	}
}

func TestGetRestrictedCacheStats_InitialZero(t *testing.T) {
	origHits := restrictedCacheHits.Load()
	origMisses := restrictedCacheMisses.Load()
	defer func() {
		restrictedCacheHits.Store(origHits)
		restrictedCacheMisses.Store(origMisses)
	}()

	restrictedCacheHits.Store(0)
	restrictedCacheMisses.Store(0)

	hits, misses := GetRestrictedCacheStats()
	if hits != 0 {
		t.Errorf("expected initial hits = 0, got %d", hits)
	}
	if misses != 0 {
		t.Errorf("expected initial misses = 0, got %d", misses)
	}
}

func TestGetRestrictedCacheStats_BothCounters(t *testing.T) {
	skipIfNoCache(t)

	const chatA = int64(-10099901)
	const chatB = int64(-10099902)

	baseHits, baseMisses := GetRestrictedCacheStats()

	MarkChatRestricted(chatA)
	defer MarkChatNotRestricted(chatA)

	if !IsChatRestricted(chatA) {
		t.Fatal("IsChatRestricted(chatA) should return true after MarkChatRestricted")
	}

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

func TestIsChatRestricted_WithinProbeTTL(t *testing.T) {
	skipIfNoCache(t)

	const chatID = int64(-10099921)
	defer MarkChatNotRestricted(chatID)

	err := GetMarshal().Set(
		Context,
		restrictedChatKey(chatID),
		time.Now().Add(-(constants.RestrictedProbeInterval / 2)).Format(time.RFC3339),
		store.WithExpiration(constants.RestrictedCacheTTL),
	)
	if err != nil {
		t.Fatalf("failed to seed restricted cache: %v", err)
	}

	if !IsChatRestricted(chatID) {
		t.Fatal("IsChatRestricted should return true within probe TTL")
	}
}

func TestIsChatRestricted_AfterProbeTTL(t *testing.T) {
	skipIfNoCache(t)

	const chatID = int64(-10099922)
	defer MarkChatNotRestricted(chatID)

	err := GetMarshal().Set(
		Context,
		restrictedChatKey(chatID),
		time.Now().Add(-constants.RestrictedProbeInterval-time.Second).Format(time.RFC3339),
		store.WithExpiration(constants.RestrictedCacheTTL),
	)
	if err != nil {
		t.Fatalf("failed to seed restricted cache: %v", err)
	}

	if IsChatRestricted(chatID) {
		t.Fatal("IsChatRestricted should return false after probe TTL to allow retry")
	}
}

func TestIsChatRestricted_ProbeSingleFlight(t *testing.T) {
	skipIfNoCache(t)

	const chatID = int64(-10099923)
	defer MarkChatNotRestricted(chatID)

	err := GetMarshal().Set(
		Context,
		restrictedChatKey(chatID),
		time.Now().Add(-constants.RestrictedProbeInterval-time.Second).Format(time.RFC3339),
		store.WithExpiration(constants.RestrictedCacheTTL),
	)
	if err != nil {
		t.Fatalf("failed to seed restricted cache: %v", err)
	}

	if IsChatRestricted(chatID) {
		t.Fatal("first check after probe interval should allow probe attempt")
	}

	if !IsChatRestricted(chatID) {
		t.Fatal("second check should be blocked while probe lock is active")
	}
}

func TestMarkChatRestricted_Idempotent(t *testing.T) {
	skipIfNoCache(t)

	const chatID = int64(-10099930)
	defer MarkChatNotRestricted(chatID)

	MarkChatRestricted(chatID)
	MarkChatRestricted(chatID)

	if !IsChatRestricted(chatID) {
		t.Errorf("IsChatRestricted(%d) = false after double MarkChatRestricted, want true", chatID)
	}
}

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

func TestRestrictedChatKey_NilSafe(t *testing.T) {
	t.Parallel()

	got := restrictedChatKey(12345)
	want := "alita:restricted:12345"
	if got != want {
		t.Errorf("restrictedChatKey(12345) = %q, want %q", got, want)
	}
}

func TestRestrictedProbeKey_NilSafe(t *testing.T) {
	t.Parallel()

	got := restrictedProbeKey(12345)
	want := "alita:restricted_probe:12345"
	if got != want {
		t.Errorf("restrictedProbeKey(12345) = %q, want %q", got, want)
	}
}

func TestGetRestrictedCacheStats_NilSafe(t *testing.T) {
	hits, misses := GetRestrictedCacheStats()
	if hits < 0 {
		t.Errorf("hits should be non-negative, got %d", hits)
	}
	if misses < 0 {
		t.Errorf("misses should be non-negative, got %d", misses)
	}
}

func TestMarkChatRestricted_NilMarshal_NoPanic(t *testing.T) {
	orig := GetMarshal()
	SetMarshal(nil)
	defer func() { SetMarshal(orig) }()

	MarkChatRestricted(-999)
}

func TestIsChatRestricted_NilMarshal_ReturnsFalse(t *testing.T) {
	orig := GetMarshal()
	SetMarshal(nil)
	defer func() { SetMarshal(orig) }()

	if IsChatRestricted(-999) {
		t.Error("IsChatRestricted with nil marshaler should return false")
	}
}

func TestMarkChatNotRestricted_NilMarshal_NoPanic(t *testing.T) {
	orig := GetMarshal()
	SetMarshal(nil)
	defer func() { SetMarshal(orig) }()

	MarkChatNotRestricted(-999)
}
