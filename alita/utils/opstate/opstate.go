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
//   - Error policy: Claim logs a Redis error and claims locally (degraded,
//     never silent); Release logs and still clears locally; Held reports
//     false on a Redis error after checking local; Stash/Consume/Peek return
//     the error to the caller, which owns the user-visible degrade.
//
// These keys sit outside alita:cache:, so CLEAR_CACHE_ON_STARTUP keeps them.
package opstate

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
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
	// ClaimShared: this call won the claim every replica sees.
	ClaimShared
	// ClaimLocal: Redis was absent or errored; this replica claimed the key
	// for itself. The claim still excludes duplicates on this replica, but
	// other replicas cannot see it.
	ClaimLocal
)

// localStore is the in-process fallback for every primitive. claimLocal
// writes an expiry marker; payloadLocal holds msgpack'd values.
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
		return ClaimLocal
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

// Consume reads and deletes the payload at key in one step: GETDEL against
// Redis, a mutex-serialized local take without it. A missing or expired key
// returns (nil, nil); a decode failure returns the error.
func Consume[T any](key string) (*T, error) {
	var raw []byte
	if rdb := cache.GetRedisClient(); rdb != nil {
		b, err := rdb.GetDel(cache.Context, key).Bytes()
		if err == redis.Nil {
			return nil, nil
		}
		if err != nil {
			return nil, fmt.Errorf("opstate consume %s: %w", key, err)
		}
		raw = b
	} else {
		raw = local.take(key)
		if raw == nil {
			return nil, nil
		}
	}
	var v T
	if err := msgpack.Unmarshal(raw, &v); err != nil {
		return nil, fmt.Errorf("opstate consume %s: %w", key, err)
	}
	return &v, nil
}

// Peek reads the payload at key without deleting it.
func Peek[T any](key string) (*T, error) {
	var raw []byte
	if rdb := cache.GetRedisClient(); rdb != nil {
		b, err := rdb.Get(cache.Context, key).Bytes()
		if err == redis.Nil {
			return nil, nil
		}
		if err != nil {
			return nil, fmt.Errorf("opstate peek %s: %w", key, err)
		}
		raw = b
	} else {
		raw = local.peek(key)
		if raw == nil {
			return nil, nil
		}
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

