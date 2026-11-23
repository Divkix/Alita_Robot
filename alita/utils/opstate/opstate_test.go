//go:build testtools

package opstate

import (
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

type opstatePayload struct {
	ChatID int64  `msgpack:"c"`
	Search string `msgpack:"s"`
}

func withRedis(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	cache.SetRedisClientForTest(t, client)
	return mr
}

func TestClaimSharedAndLost(t *testing.T) {
	withRedis(t)
	key := Key("test", "claim", t.Name())

	if got := Claim(key, time.Minute); got != ClaimShared {
		t.Fatalf("first Claim() = %v, want ClaimShared", got)
	}
	if got := Claim(key, time.Minute); got != ClaimLost {
		t.Fatalf("second Claim() = %v, want ClaimLost", got)
	}
	Release(key)
	if got := Claim(key, time.Minute); got != ClaimShared {
		t.Fatalf("Claim() after Release = %v, want ClaimShared", got)
	}
}

func TestClaimLocalWithoutRedis(t *testing.T) {
	cache.DisableRedisForTest()
	key := Key("test", "claim-local", t.Name())

	if got := Claim(key, time.Minute); got != ClaimLocal {
		t.Fatalf("first Claim() = %v, want ClaimLocal", got)
	}
	if got := Claim(key, time.Minute); got != ClaimLost {
		t.Fatalf("second Claim() = %v, want ClaimLost", got)
	}
	Release(key)
	if got := Claim(key, time.Minute); got != ClaimLocal {
		t.Fatalf("Claim() after Release = %v, want ClaimLocal", got)
	}
}

func TestClaimIsAtomicUnderRace(t *testing.T) {
	withRedis(t)
	key := Key("test", "claim-race", t.Name())

	const n = 16
	var wg sync.WaitGroup
	wins := make(chan ClaimResult, n)
	wg.Add(n)
	for range n {
		go func() {
			defer wg.Done()
			wins <- Claim(key, time.Minute)
		}()
	}
	wg.Wait()
	close(wins)

	shared, localWins := 0, 0
	for r := range wins {
		switch r {
		case ClaimShared:
			shared++
		case ClaimLocal:
			localWins++
		}
	}
	if shared+localWins != 1 {
		t.Fatalf("claim winners = %d shared + %d local, want exactly 1", shared, localWins)
	}
}

func TestHeldFollowsClaim(t *testing.T) {
	withRedis(t)
	key := Key("test", "held", t.Name())

	if Held(key) {
		t.Fatal("Held() = true before any claim")
	}
	Claim(key, time.Minute)
	if !Held(key) {
		t.Fatal("Held() = false after Claim()")
	}
	Release(key)
	if Held(key) {
		t.Fatal("Held() = true after Release()")
	}
}

func TestStashConsumePeekRoundTrip(t *testing.T) {
	withRedis(t)
	key := Key("test", "stash", t.Name())
	payload := opstatePayload{ChatID: -1001, Search: "spam"}

	if err := Stash(key, payload, time.Minute); err != nil {
		t.Fatalf("Stash() error = %v", err)
	}
	got, err := Peek[opstatePayload](key)
	if err != nil || got == nil {
		t.Fatalf("Peek() = %v, %v; want the payload", got, err)
	}
	if got.ChatID != payload.ChatID || got.Search != payload.Search {
		t.Fatalf("Peek() = %+v, want %+v", got, payload)
	}

	consumed, err := Consume[opstatePayload](key)
	if err != nil || consumed == nil {
		t.Fatalf("Consume() = %v, %v; want the payload", consumed, err)
	}
	if consumed.ChatID != payload.ChatID || consumed.Search != payload.Search {
		t.Fatalf("Consume() = %+v, want %+v", consumed, payload)
	}
	// Consume is one-shot.
	again, err := Consume[opstatePayload](key)
	if err != nil || again != nil {
		t.Fatalf("second Consume() = %v, %v; want nil, nil", again, err)
	}
}

func TestStashConsumePeekWithoutRedis(t *testing.T) {
	cache.DisableRedisForTest()
	key := Key("test", "stash-local", t.Name())
	payload := opstatePayload{ChatID: -1002, Search: "local"}

	if err := Stash(key, payload, time.Minute); err != nil {
		t.Fatalf("Stash() error = %v", err)
	}
	got, err := Consume[opstatePayload](key)
	if err != nil || got == nil {
		t.Fatalf("Consume() = %v, %v; want the payload", got, err)
	}
	if got.ChatID != payload.ChatID {
		t.Fatalf("Consume() = %+v, want %+v", got, payload)
	}
	if again, _ := Consume[opstatePayload](key); again != nil {
		t.Fatal("second Consume() returned a payload, want nil")
	}
}

func TestConsumeMissReturnsNil(t *testing.T) {
	withRedis(t)
	got, err := Consume[opstatePayload](Key("test", "consume-miss", t.Name()))
	if err != nil || got != nil {
		t.Fatalf("Consume() on missing key = %v, %v; want nil, nil", got, err)
	}
}

func TestClaimExpires(t *testing.T) {
	mr := withRedis(t)
	key := Key("test", "claim-ttl", t.Name())

	if got := Claim(key, 50*time.Millisecond); got != ClaimShared {
		t.Fatalf("Claim() = %v, want ClaimShared", got)
	}
	mr.FastForward(100 * time.Millisecond)
	if got := Claim(key, time.Minute); got != ClaimShared {
		t.Fatalf("Claim() after TTL = %v, want ClaimShared", got)
	}
}
