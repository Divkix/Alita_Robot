package modules

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

func TestJoinClaimNoCacheFallback(t *testing.T) {
	withNilCacheMarshal(t)
	t.Cleanup(func() {
		clearJoinClaim(-100123, 456)
	})

	if got := claimJoin(-100123, 456); got != ClaimShared {
		t.Fatalf("first claimJoin() = %v, want ClaimShared", got)
	}
	if got := claimJoin(-100123, 456); got != ClaimLost {
		t.Fatalf("second claimJoin() = %v, want ClaimLost duplicate", got)
	}

	clearJoinClaim(-100123, 456)
	if got := claimJoin(-100123, 456); got != ClaimShared {
		t.Fatalf("claim after clear = %v, want ClaimShared", got)
	}
}

// TestJoinClaimIsAtomic verifies that exactly one caller wins the join claim
// for a given (chat, user) pair — both in the sequential case and when N
// goroutines race concurrently.
func TestJoinClaimIsAtomic(t *testing.T) {
	chatID := -time.Now().UnixNano() / 1e6 // negative to avoid valid Telegram chat IDs
	userID := time.Now().UnixNano()/1e6 + 1

	t.Cleanup(func() {
		clearJoinClaim(chatID, userID)
	})

	if got := claimJoin(chatID, userID); got != ClaimShared {
		t.Fatalf("first claimJoin() = %v, want ClaimShared", got)
	}
	if got := claimJoin(chatID, userID); got != ClaimLost {
		t.Fatalf("second claimJoin() = %v, want ClaimLost (duplicate)", got)
	}

	clearJoinClaim(chatID, userID)

	const N = 20
	var wg sync.WaitGroup
	var wins atomic.Int64

	wg.Add(N)
	for range N {
		go func() {
			defer wg.Done()
			if claimJoin(chatID, userID) == ClaimShared {
				wins.Add(1)
			}
		}()
	}
	wg.Wait()

	if got := wins.Load(); got != 1 {
		t.Fatalf("concurrent claimJoin() winners = %d, want exactly 1", got)
	}
}

// Without a cache the pending flag cannot be stored, so every join request
// must still be treated as new rather than silently suppressed.
func TestPendingJoinsCacheNilMarshalNeverReportsPending(t *testing.T) {
	withNilCacheMarshal(t)

	greetingsModule.setPendingJoins(-100123, 456)
	if greetingsModule.loadPendingJoins(-100123, 456) {
		t.Fatal("loadPendingJoins(nil marshal) = true after setPendingJoins, want false")
	}
}

func TestPendingJoinsCacheRoundTrip(t *testing.T) {
	m := cache.GetMarshal()
	if m == nil {
		t.Skip("requires cache marshal")
	}

	chatID := uniqueModuleChatID()
	const userID int64 = 456789
	key := fmt.Sprintf("alita:pendingJoins:%d:%d", chatID, userID)
	t.Cleanup(func() {
		_ = m.Delete(cache.Context, key)
	})

	greetingsModule.setPendingJoins(chatID, userID)
	if !greetingsModule.loadPendingJoins(chatID, userID) {
		t.Fatal("loadPendingJoins() = false after setPendingJoins, want true")
	}
}
