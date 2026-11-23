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

func TestClaimWithoutRedis(t *testing.T) {
	cache.DisableRedisForTest()
	key := Key("test", "claim-local", t.Name())

	// Redis absent: the in-process claim wins and reports ClaimShared —
	// nothing is degraded, the claim simply cannot cross replicas.
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

func TestClaimLocalOnRedisError(t *testing.T) {
	// A configured but unreachable Redis degrades the claim to this
	// replica and reports ClaimLocal.
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{
		Addr:               mr.Addr(),
		MaxRetries:         -1,
		DialerRetries:      1,
		DialerRetryTimeout: time.Millisecond,
	})
	t.Cleanup(func() { _ = client.Close() })
	mr.Close()
	cache.SetRedisClientForTest(t, client)

	key := Key("test", "claim-err", t.Name())
	if got := Claim(key, time.Minute); got != ClaimLocal {
		t.Fatalf("Claim() with dead Redis = %v, want ClaimLocal", got)
	}
	if got := Claim(key, time.Minute); got != ClaimLost {
		t.Fatalf("second Claim() = %v, want ClaimLost", got)
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

	winners := 0
	for r := range wins {
		if r == ClaimShared || r == ClaimLocal {
			winners++
		}
	}
	if winners != 1 {
		t.Fatalf("claim winners = %d, want exactly 1", winners)
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

func TestCounterAddSharesThroughRedis(t *testing.T) {
	withRedis(t)
	key := Key("test", "counter", t.Name())

	if got, err := CounterAdd(key, time.Hour); err != nil || got != 1 {
		t.Fatalf("CounterAdd() = %d, %v; want 1", got, err)
	}
	if got, err := CounterAdd(key, time.Hour); err != nil || got != 2 {
		t.Fatalf("CounterAdd() = %d, %v; want 2", got, err)
	}
	// The TTL is set on the first increment only: a later Add must not
	// slide the expiry.
	if ttl, err := cache.GetRedisClient().TTL(cache.Context, key).Result(); err != nil || ttl <= 0 || ttl > time.Hour {
		t.Fatalf("counter TTL = %v, %v; want within (0, 1h]", ttl, err)
	}
}

func TestCounterAddWithoutRedis(t *testing.T) {
	cache.DisableRedisForTest()
	if _, err := CounterAdd(Key("test", "counter-local", t.Name()), time.Hour); err == nil {
		t.Fatal("CounterAdd() without Redis error = nil, want an error")
	}
}

func TestCounterAddExpires(t *testing.T) {
	mr := withRedis(t)
	key := Key("test", "counter-ttl", t.Name())

	if _, err := CounterAdd(key, 50*time.Millisecond); err != nil {
		t.Fatalf("CounterAdd() error = %v", err)
	}
	mr.FastForward(100 * time.Millisecond)
	if got, err := CounterAdd(key, time.Hour); err != nil || got != 1 {
		t.Fatalf("CounterAdd() after TTL = %d, %v; want 1", got, err)
	}
}

func TestWindowAddCountsAndRemembers(t *testing.T) {
	withRedis(t)
	key := Key("test", "window", t.Name())
	now := time.Now()

	prior, recent, err := WindowAdd(key, now, time.Hour, "m1", "first text", 10, 0)
	if err != nil || prior != 0 || len(recent) != 0 {
		t.Fatalf("WindowAdd() = %d, %v, %v; want 0, [], nil", prior, recent, err)
	}
	prior, recent, err = WindowAdd(key, now.Add(time.Minute), time.Hour, "m2", "second text", 10, 0)
	if err != nil || prior != 1 {
		t.Fatalf("WindowAdd() prior = %d, %v; want 1", prior, err)
	}
	if len(recent) != 1 || recent[0] != "first text" {
		t.Fatalf("WindowAdd() recent = %v, want [first text]", recent)
	}
}

func TestWindowAddDropsExpiredAndTrims(t *testing.T) {
	withRedis(t)
	key := Key("test", "window-trim", t.Name())
	now := time.Now()

	// Two members inside the hour, one older than the window.
	if _, _, err := WindowAdd(key, now.Add(-2*time.Hour), time.Hour, "old", "", 10, 0); err != nil {
		t.Fatalf("WindowAdd(old) error = %v", err)
	}
	if _, _, err := WindowAdd(key, now.Add(-time.Minute), time.Hour, "a", "", 10, 0); err != nil {
		t.Fatalf("WindowAdd(a) error = %v", err)
	}
	if _, _, err := WindowAdd(key, now.Add(-time.Second), time.Hour, "b", "", 10, 0); err != nil {
		t.Fatalf("WindowAdd(b) error = %v", err)
	}

	prior, _, err := WindowAdd(key, now, time.Hour, "c", "", 2, 0)
	if err != nil {
		t.Fatalf("WindowAdd() error = %v", err)
	}
	// "old" fell out of the window; prior counts only live members.
	if prior != 2 {
		t.Fatalf("WindowAdd() prior = %d, want 2", prior)
	}

	// max=2 keeps only the two newest members.
	prior, _, err = WindowAdd(key, now.Add(time.Second), time.Hour, "d", "", 2, 0)
	if err != nil || prior != 2 {
		t.Fatalf("WindowAdd() = %d, %v; want prior 2", prior, err)
	}
}

func TestWindowAddWithoutRedis(t *testing.T) {
	cache.DisableRedisForTest()
	_, _, err := WindowAdd(Key("test", "window-local", t.Name()), time.Now(), time.Hour, "m", "", 10, 0)
	if err == nil {
		t.Fatal("WindowAdd() without Redis error = nil, want an error")
	}
}

func TestMembersAddCountsDistinct(t *testing.T) {
	withRedis(t)
	key := Key("test", "members", t.Name())

	if got, err := MembersAdd(key, "a", time.Hour); err != nil || got != 1 {
		t.Fatalf("MembersAdd() = %d, %v; want 1", got, err)
	}
	if got, err := MembersAdd(key, "b", time.Hour); err != nil || got != 2 {
		t.Fatalf("MembersAdd() = %d, %v; want 2", got, err)
	}
	// A repeated member does not grow the set.
	if got, err := MembersAdd(key, "a", time.Hour); err != nil || got != 2 {
		t.Fatalf("MembersAdd(repeat) = %d, %v; want 2", got, err)
	}
}

func TestMembersAddWithoutRedis(t *testing.T) {
	cache.DisableRedisForTest()
	if _, err := MembersAdd(Key("test", "members-local", t.Name()), "a", time.Hour); err == nil {
		t.Fatal("MembersAdd() without Redis error = nil, want an error")
	}
}

func TestSwapReplacesAndDeletes(t *testing.T) {
	withRedis(t)
	key := Key("test", "swap", t.Name())

	type state struct {
		Active bool `msgpack:"a"`
	}

	// expected nil matches only an absent key.
	ok, err := Swap(key, state{Active: true}, nil, time.Hour)
	if err != nil || !ok {
		t.Fatalf("Swap(create) = %v, %v; want true", ok, err)
	}
	ok, err = Swap(key, state{Active: true}, nil, time.Hour)
	if err != nil || ok {
		t.Fatalf("Swap(create over existing) = %v, %v; want false", ok, err)
	}

	// A matching expected replaces the value.
	ok, err = Swap(key, state{Active: false}, state{Active: true}, time.Hour)
	if err != nil || !ok {
		t.Fatalf("Swap(replace) = %v, %v; want true", ok, err)
	}
	got, err := Peek[state](key)
	if err != nil || got == nil || got.Active {
		t.Fatalf("Peek() = %+v, %v; want inactive state", got, err)
	}

	// A stale expected changes nothing.
	ok, err = Swap(key, state{Active: true}, state{Active: true}, time.Hour)
	if err != nil || ok {
		t.Fatalf("Swap(stale) = %v, %v; want false", ok, err)
	}

	// to nil deletes when expected matches.
	ok, err = Swap(key, nil, state{Active: false}, time.Hour)
	if err != nil || !ok {
		t.Fatalf("Swap(delete) = %v, %v; want true", ok, err)
	}
	got, err = Peek[state](key)
	if err != nil || got != nil {
		t.Fatalf("Peek() after delete = %+v, %v; want nil", got, err)
	}
}

func TestSwapWithoutRedis(t *testing.T) {
	cache.DisableRedisForTest()
	key := Key("test", "swap-local", t.Name())
	type state struct {
		N int `msgpack:"n"`
	}

	ok, err := Swap(key, state{N: 1}, nil, time.Hour)
	if err != nil || !ok {
		t.Fatalf("Swap(create) = %v, %v; want true", ok, err)
	}
	ok, err = Swap(key, state{N: 2}, state{N: 1}, time.Hour)
	if err != nil || !ok {
		t.Fatalf("Swap(replace) = %v, %v; want true", ok, err)
	}
	ok, err = Swap(key, nil, state{N: 2}, time.Hour)
	if err != nil || !ok {
		t.Fatalf("Swap(delete) = %v, %v; want true", ok, err)
	}
	got, err := Peek[state](key)
	if err != nil || got != nil {
		t.Fatalf("Peek() after delete = %+v, %v; want nil", got, err)
	}
}
