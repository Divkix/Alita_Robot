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

	handle, got := Acquire(key, time.Minute)
	if got != ClaimShared {
		t.Fatalf("first Acquire() = %v, want ClaimShared", got)
	}
	if _, got := Acquire(key, time.Minute); got != ClaimLost {
		t.Fatalf("second Acquire() = %v, want ClaimLost", got)
	}
	handle.Release()
	if _, got := Acquire(key, time.Minute); got != ClaimShared {
		t.Fatalf("Acquire() after Release = %v, want ClaimShared", got)
	}
}

func TestClaimWithoutRedis(t *testing.T) {
	cache.DisableRedisForTest()
	key := Key("test", "claim-local", t.Name())

	// Redis absent: the in-process claim wins and reports ClaimShared —
	// nothing is degraded, the claim simply cannot cross replicas.
	handle, got := Acquire(key, time.Minute)
	if got != ClaimShared {
		t.Fatalf("first Acquire() = %v, want ClaimShared", got)
	}
	if _, got := Acquire(key, time.Minute); got != ClaimLost {
		t.Fatalf("second Acquire() = %v, want ClaimLost", got)
	}
	handle.Release()
	if _, got := Acquire(key, time.Minute); got != ClaimShared {
		t.Fatalf("Acquire() after Release = %v, want ClaimShared", got)
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
	handle, got := Acquire(key, time.Minute)
	if got != ClaimLocal {
		t.Fatalf("Acquire() with dead Redis = %v, want ClaimLocal", got)
	}
	if _, got := Acquire(key, time.Minute); got != ClaimLost {
		t.Fatalf("second Acquire() = %v, want ClaimLost", got)
	}
	// Release logs the Redis failure and still drops the local token.
	handle.Release()
	if _, got := Acquire(key, time.Minute); got != ClaimLocal {
		t.Fatalf("Acquire() after Release = %v, want ClaimLocal", got)
	}
}

func TestAcquireIsAtomicUnderRace(t *testing.T) {
	withRedis(t)
	key := Key("test", "claim-race", t.Name())

	type outcome struct {
		handle ClaimHandle
		result ClaimResult
	}
	const n = 16
	var wg sync.WaitGroup
	results := make(chan outcome, n)
	wg.Add(n)
	for range n {
		go func() {
			defer wg.Done()
			handle, result := Acquire(key, time.Minute)
			results <- outcome{handle: handle, result: result}
		}()
	}
	wg.Wait()
	close(results)

	winners := 0
	var winner ClaimHandle
	for o := range results {
		if o.result == ClaimShared || o.result == ClaimLocal {
			winners++
			winner = o.handle
			continue
		}
		if o.handle != (ClaimHandle{}) {
			t.Fatal("a lost Acquire() returned a handle")
		}
	}
	if winners != 1 {
		t.Fatalf("claim winners = %d, want exactly 1", winners)
	}
	// Only the winner's handle frees the claim.
	winner.Release()
	if Held(key) {
		t.Fatal("Held() = true after the winner's Release()")
	}
}

func TestHeldFollowsClaim(t *testing.T) {
	withRedis(t)
	key := Key("test", "held", t.Name())

	if Held(key) {
		t.Fatal("Held() = true before any claim")
	}
	handle, _ := Acquire(key, time.Minute)
	if !Held(key) {
		t.Fatal("Held() = false after Acquire()")
	}
	handle.Release()
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

	stale, got := Acquire(key, 50*time.Millisecond)
	if got != ClaimShared {
		t.Fatalf("Acquire() = %v, want ClaimShared", got)
	}
	mr.FastForward(100 * time.Millisecond)
	owner, got := Acquire(key, time.Minute)
	if got != ClaimShared {
		t.Fatalf("Acquire() after TTL = %v, want ClaimShared", got)
	}
	// The expired owner's handle must not drop the claim it no longer owns.
	stale.Release()
	if _, got := Acquire(key, time.Minute); got != ClaimLost {
		t.Fatalf("Acquire() after stale Release = %v, want ClaimLost", got)
	}
	owner.Release()
	if Held(key) {
		t.Fatal("Held() = true after the owner's Release()")
	}
}

func TestReleaseAfterClearIsNoOp(t *testing.T) {
	withRedis(t)
	key := Key("test", "claim-clear", t.Name())

	stale, got := Acquire(key, time.Minute)
	if got != ClaimShared {
		t.Fatalf("first Acquire() = %v, want ClaimShared", got)
	}
	Clear(key)
	owner, got := Acquire(key, time.Minute)
	if got != ClaimShared {
		t.Fatalf("Acquire() after Clear = %v, want ClaimShared", got)
	}

	// A handle from before the reset no longer owns the key; nor does the
	// zero handle.
	stale.Release()
	ClaimHandle{}.Release()
	if _, got := Acquire(key, time.Minute); got != ClaimLost {
		t.Fatalf("Acquire() after stale Releases = %v, want ClaimLost", got)
	}
	owner.Release()
	if Held(key) {
		t.Fatal("Held() = true after the owner's Release()")
	}
}

func TestReleaseIsOwnerFencedWithoutRedis(t *testing.T) {
	cache.DisableRedisForTest()
	key := Key("test", "claim-local-fence", t.Name())

	stale, got := Acquire(key, 50*time.Millisecond)
	if got != ClaimShared {
		t.Fatalf("first Acquire() = %v, want ClaimShared", got)
	}
	time.Sleep(60 * time.Millisecond)
	owner, got := Acquire(key, time.Minute)
	if got != ClaimShared {
		t.Fatalf("Acquire() after TTL = %v, want ClaimShared", got)
	}

	// The local mirror is owner-fenced too.
	stale.Release()
	if !Held(key) {
		t.Fatal("stale Release() dropped the newer local claim")
	}
	owner.Release()
	if Held(key) {
		t.Fatal("Held() = true after the owner's Release()")
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
