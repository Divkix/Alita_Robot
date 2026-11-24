// Package opstate owns the alita:* operational keyspace: the short-lived
// Redis state behind "do this once" claims, consume-once payloads behind
// callback tokens, and held flags. It replaces the per-feature hand-rolled
// SetNX/sync.Map/GetDel plumbing.
//
// Declared policies, once for every primitive:
//
//   - Key naming: callers pass parts to Key; the "alita:" prefix and the
//     colon joining live here, so feature code never formats a key string.
//   - Redis absent (GetRedisClient() == nil): every primitive falls back to
//     an in-process store local to this replica. Claims degrade to
//     per-replica; stashed payloads are per-replica too.
//   - Atomicity: claims are SET NX PX (or an in-process CAS); Consume is
//     GETDEL (or a mutex-serialized local read+delete); Stash/Peek are plain
//     writes/reads.
//   - Error policy: Claim reports ClaimLocal (logged, never silent) when a
//     configured Redis errors; Release logs and still clears locally; Held
//     reports the local answer on a Redis error; Stash/Consume/Peek return
//     the error to the caller, which owns the user-visible degrade.
//
// Counter, Window, Members and Swap cover the remaining operational shapes:
// an incrementing count with a lifetime, a sorted-set sliding window, a
// set of distinct members with a lifetime, and a compare-and-set on a
// stashed payload. Redis absent they report an error and the caller owns
// the degrade, matching the fail-open sites they replace; Swap alone also
// runs against this replica's local payload store.
//
// These keys sit outside alita:cache:, so CLEAR_CACHE_ON_STARTUP keeps them.
package opstate

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
	"github.com/vmihailenco/msgpack/v5"

	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

// Key builds an operational key: "alita:" + parts joined by ":".
func Key(parts ...any) string {
	strs := make([]string, len(parts))
	for i, p := range parts {
		strs[i] = fmt.Sprint(p)
	}
	return "alita:" + strings.Join(strs, ":")
}

// ClaimResult is the outcome of Claim.
type ClaimResult int

const (
	// ClaimLost: the key is already held (by this or another replica).
	ClaimLost ClaimResult = iota
	// ClaimShared: this call won and nothing is degraded — the shared claim
	// through Redis, or the in-process claim when Redis is not configured.
	ClaimShared
	// ClaimLocal: Redis is configured but errored; this replica claimed the
	// key for itself. The claim still excludes duplicates on this replica,
	// but callers that must not act degraded treat it as lost.
	ClaimLocal
)

// localStore is the in-process fallback for every primitive that declares
// one: claimLocal writes an expiry marker; payloadLocal holds msgpack'd
// values for Stash/Consume/Peek/Swap.
type localStore struct {
	mu       sync.Mutex
	claims   map[string]time.Time
	payloads map[string]localPayload
}

type localPayload struct {
	data []byte
	exp  time.Time
}

var local = &localStore{
	claims:   make(map[string]time.Time),
	payloads: make(map[string]localPayload),
}

func (s *localStore) claim(key string, ttl time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if exp, ok := s.claims[key]; ok && now.Before(exp) {
		return false
	}
	s.claims[key] = now.Add(ttl)
	return true
}

func (s *localStore) release(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.claims, key)
}

func (s *localStore) held(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	exp, ok := s.claims[key]
	if !ok {
		return false
	}
	if time.Now().Before(exp) {
		return true
	}
	delete(s.claims, key)
	return false
}

func (s *localStore) put(key string, data []byte, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.payloads[key] = localPayload{data: data, exp: time.Now().Add(ttl)}
}

// take removes and returns the payload, or nil when absent or expired.
func (s *localStore) take(key string) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.payloads[key]
	if !ok {
		return nil
	}
	delete(s.payloads, key)
	if time.Now().After(p.exp) {
		return nil
	}
	return p.data
}

func (s *localStore) peek(key string) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.payloads[key]
	if !ok {
		return nil
	}
	if time.Now().After(p.exp) {
		delete(s.payloads, key)
		return nil
	}
	return p.data
}

// swap replaces the payload at key with data when the stored bytes equal
// expectedRaw, or deletes it when data is nil. expectedRaw nil matches only
// an absent or expired key. It mirrors the swapScript compare-and-set.
func (s *localStore) swap(key string, data, expectedRaw []byte, ttl time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.payloads[key]
	live := ok && time.Now().Before(p.exp)
	if expectedRaw == nil {
		if live {
			return false
		}
	} else if !live || string(p.data) != string(expectedRaw) {
		return false
	}
	if data == nil {
		delete(s.payloads, key)
		return true
	}
	s.payloads[key] = localPayload{data: data, exp: time.Now().Add(ttl)}
	return true
}

// Claim takes key for ttl: one winner across replicas through Redis SET NX,
// or this replica alone when Redis is absent. A Redis error is logged and
// degrades to the local claim rather than failing the caller silently.
func Claim(key string, ttl time.Duration) ClaimResult {
	// The local mirror keeps Held accurate while Redis is healthy and is the
	// claim itself when it is not.
	mirror := func() {
		local.mu.Lock()
		local.claims[key] = time.Now().Add(ttl)
		local.mu.Unlock()
	}

	if rdb := cache.GetRedisClient(); rdb != nil {
		claimed, err := rdb.SetNX(cache.Context, key, 1, ttl).Result()
		if err != nil {
			log.WithError(err).Debugf("[opstate] claim %s failed, claiming locally", key)
			if local.claim(key, ttl) {
				return ClaimLocal
			}
			return ClaimLost
		}
		if claimed {
			mirror()
			return ClaimShared
		}
		return ClaimLost
	}
	if local.claim(key, ttl) {
		return ClaimShared
	}
	return ClaimLost
}

// Release drops a claim on both stores; a Redis error is logged and the
// local claim is still cleared.
func Release(key string) {
	if rdb := cache.GetRedisClient(); rdb != nil {
		if err := rdb.Del(cache.Context, key).Err(); err != nil {
			log.WithError(err).Debugf("[opstate] release %s failed", key)
		}
	}
	local.release(key)
}

// Held reports whether key is currently claimed, checking this replica's
// mirror first and Redis when the mirror misses. A Redis error reports the
// local answer.
func Held(key string) bool {
	if local.held(key) {
		return true
	}
	if rdb := cache.GetRedisClient(); rdb != nil {
		if n, err := rdb.Exists(cache.Context, key).Result(); err == nil {
			return n > 0
		}
	}
	return false
}

// Stash stores v behind key for ttl. Without Redis the payload lives in
// this replica's local store.
func Stash[T any](key string, v T, ttl time.Duration) error {
	data, err := msgpack.Marshal(v)
	if err != nil {
		return fmt.Errorf("opstate stash %s: %w", key, err)
	}
	if rdb := cache.GetRedisClient(); rdb != nil {
		if err := rdb.Set(cache.Context, key, data, ttl).Err(); err != nil {
			return fmt.Errorf("opstate stash %s: %w", key, err)
		}
		return nil
	}
	local.put(key, data, ttl)
	return nil
}

// Delete drops the state at key on Redis and this replica's local payload
// store, best-effort: Redis errors are ignored. Claims are untouched —
// Release owns those. For cleanup paths, not for consume semantics.
func Delete(key string) {
	if rdb := cache.GetRedisClient(); rdb != nil {
		_ = rdb.Del(cache.Context, key).Err()
	}
	local.mu.Lock()
	delete(local.payloads, key)
	local.mu.Unlock()
}

// readPayload fetches the raw bytes at key for op ("consume" or "peek"),
// taking it through getdel when op is consume. A missing or expired key
// returns (nil, nil); storage failures return the error.
func readPayload(key, op string) ([]byte, error) {
	consume := op == "consume"
	if rdb := cache.GetRedisClient(); rdb != nil {
		var b []byte
		var err error
		if consume {
			b, err = rdb.GetDel(cache.Context, key).Bytes()
		} else {
			b, err = rdb.Get(cache.Context, key).Bytes()
		}
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		if err != nil {
			return nil, fmt.Errorf("opstate %s %s: %w", op, key, err)
		}
		return b, nil
	}
	if consume {
		return local.take(key), nil
	}
	return local.peek(key), nil
}

// Consume reads and deletes the payload at key in one step: GETDEL against
// Redis, a mutex-serialized local take without it. A missing or expired key
// returns (nil, nil); a decode failure returns the error.
func Consume[T any](key string) (*T, error) {
	raw, err := readPayload(key, "consume")
	if err != nil || raw == nil {
		return nil, err
	}
	var v T
	if err := msgpack.Unmarshal(raw, &v); err != nil {
		return nil, fmt.Errorf("opstate consume %s: %w", key, err)
	}
	return &v, nil
}

// Peek reads the payload at key without deleting it.
func Peek[T any](key string) (*T, error) {
	raw, err := readPayload(key, "peek")
	if err != nil || raw == nil {
		return nil, err
	}
	var v T
	if err := msgpack.Unmarshal(raw, &v); err != nil {
		return nil, fmt.Errorf("opstate peek %s: %w", key, err)
	}
	return &v, nil
}

// NewToken returns a short random token for stash keys.
func NewToken() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// CounterAdd increments the counter at key through Redis and returns the
// new count — the INCR + EXPIRE-on-first shape: the first increment after
// the key appears or expires sets its TTL. Without Redis, or on a Redis
// error, it returns an error and the caller owns the degrade.
func CounterAdd(key string, ttl time.Duration) (int64, error) {
	rdb := cache.GetRedisClient()
	if rdb == nil {
		return 0, errors.New("opstate: counter requires Redis")
	}
	n, err := counterScript.Run(cache.Context, rdb, []string{key}, int64(ttl/time.Second)).Int64()
	if err != nil {
		return 0, fmt.Errorf("opstate counter %s: %w", key, err)
	}
	return n, nil
}

var counterScript = redis.NewScript(`
local n = redis.call('INCR', KEYS[1])
if n == 1 then
	redis.call('EXPIRE', KEYS[1], ARGV[1])
end
return n
`)

// WindowAdd records member at score now in the sorted set at key, drops
// entries older than now-window and, when max is positive, trims the set
// to max members, all in one atomic step. It returns the number of
// members already inside the window (before this add) and the payloads of
// the members still in the window, oldest first, excluding the member just
// added — at most recentMax of them when recentMax is positive, all of
// them otherwise. payload may be empty; it is what recent carries back.
//
// Members are stored as "<score>\x00<member>\x00<payload>" so one set serves
// both the count and the remembered payloads. Without Redis, or on a Redis
// error, it returns an error: the callers it replaces failed open
// (untracked joins, unjudged sender history), so Window does not invent a
// per-replica window.
func WindowAdd(key string, now time.Time, window time.Duration, member, payload string, max, recentMax int) (int64, []string, error) {
	rdb := cache.GetRedisClient()
	if rdb == nil {
		return 0, nil, errors.New("opstate: window requires Redis")
	}
	score := now.UnixMilli()
	stored := strconv.FormatInt(score, 10) + "\x00" + member
	if payload != "" {
		stored += "\x00" + payload
	}
	out, err := windowScript.Run(
		cache.Context, rdb, []string{key},
		score, stored, now.Add(-window).UnixMilli(), int64(window/time.Second), max, recentMax,
	).Result()
	if err != nil {
		return 0, nil, fmt.Errorf("opstate window %s: %w", key, err)
	}
	values, ok := out.([]any)
	if !ok || len(values) == 0 {
		return 0, nil, nil
	}
	prior, _ := values[0].(int64)
	recent := make([]string, 0, len(values)-1)
	for _, v := range values[1:] {
		if s, ok := v.(string); ok {
			recent = append(recent, s)
		}
	}
	return prior, recent, nil
}

var windowScript = redis.NewScript(`
local cutoff = tonumber(ARGV[3])
redis.call('ZREMRANGEBYSCORE', KEYS[1], 0, cutoff)
local prior = redis.call('ZCARD', KEYS[1])
redis.call('ZADD', KEYS[1], ARGV[1], ARGV[2])
if tonumber(ARGV[5]) > 0 then
	redis.call('ZREMRANGEBYRANK', KEYS[1], 0, -tonumber(ARGV[5]) - 1)
end
redis.call('EXPIRE', KEYS[1], ARGV[4])
local members = redis.call('ZRANGE', KEYS[1], 0, -1, 'REV')
local recent = {}
for i = 1, #members do
	local m = members[i]
	if m ~= ARGV[2] then
		local sep = string.find(m, '\0', 1, true)
		if sep then
			local rest = string.sub(m, sep + 1)
			local psep = string.find(rest, '\0', 1, true)
			if psep then
				recent[#recent + 1] = string.sub(rest, psep + 1)
			end
		end
	end
	if tonumber(ARGV[6]) > 0 and #recent >= tonumber(ARGV[6]) then
		break
	end
end
local out = {prior}
for i = #recent, 1, -1 do out[#out + 1] = recent[i] end
return out
`)

// MembersAdd records member in the set at key and returns the set's size
// after the add, refreshing the set's TTL to ttl on every call — the shape
// of "count distinct things seen during a window that slides with use".
// Without Redis or on a Redis error it returns an error: the probation
// counter it replaces failed open, so Members does not count in-process.
func MembersAdd(key, member string, ttl time.Duration) (int64, error) {
	rdb := cache.GetRedisClient()
	if rdb == nil {
		return 0, errors.New("opstate: members requires Redis")
	}
	pipe := rdb.TxPipeline()
	pipe.SAdd(cache.Context, key, member)
	pipe.Expire(cache.Context, key, ttl)
	card := pipe.SCard(cache.Context, key)
	if _, err := pipe.Exec(cache.Context); err != nil {
		return 0, fmt.Errorf("opstate members %s: %w", key, err)
	}
	return card.Val(), nil
}

// Swap is a compare-and-set on the payload at key: when the stored bytes
// equal the msgpack encoding of expected it stores the msgpack encoding of
// to (deleting the key when to is nil) and returns true; otherwise it
// changes nothing and returns false. expected nil matches only an absent
// key. ttl applies to the stored value; ignored when to is nil.
//
// Without Redis the swap runs against this replica's local payload store,
// serialized by the store's mutex.
func Swap(key string, to, expected any, ttl time.Duration) (bool, error) {
	var expectedRaw, data []byte
	var err error
	if expected != nil {
		if expectedRaw, err = msgpack.Marshal(expected); err != nil {
			return false, fmt.Errorf("opstate swap %s: %w", key, err)
		}
	}
	if to != nil {
		if data, err = msgpack.Marshal(to); err != nil {
			return false, fmt.Errorf("opstate swap %s: %w", key, err)
		}
	}

	if rdb := cache.GetRedisClient(); rdb != nil {
		argv := []any{""}
		if expectedRaw != nil {
			argv[0] = expectedRaw
		}
		argv = append(argv, "")
		if data != nil {
			argv[1] = data
		}
		argv = append(argv, int64(ttl/time.Millisecond))
		n, err := swapScript.Run(cache.Context, rdb, []string{key}, argv...).Int()
		if err != nil {
			return false, fmt.Errorf("opstate swap %s: %w", key, err)
		}
		return n == 1, nil
	}
	return local.swap(key, data, expectedRaw, ttl), nil
}

var swapScript = redis.NewScript(`
local current = redis.call('GET', KEYS[1])
if ARGV[1] == '' then
	if current ~= false then return 0 end
elseif current ~= ARGV[1] then
	return 0
end
if ARGV[2] == '' then
	redis.call('DEL', KEYS[1])
else
	redis.call('SET', KEYS[1], ARGV[2], 'PX', ARGV[3])
end
return 1
`)
