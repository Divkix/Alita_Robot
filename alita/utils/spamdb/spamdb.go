// Package spamdb looks a Telegram user up in the CAS (cas.chat) spam database,
// so a chat can act on a known spammer when they join.
//
// A failed lookup is an error, never a "clean" verdict: callers let the join
// through and nothing is cached, so the next join asks again.
package spamdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/singleflight"

	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
)

const (
	DefaultCASURL = "https://api.cas.chat/check"

	// DefaultTimeout bounds the whole lookup; the join handler waits on it.
	DefaultTimeout = 3 * time.Second

	// A listing is rarely lifted, so it is kept longer than a clean verdict,
	// which goes stale as soon as the account starts spamming elsewhere.
	ListedTTL = 6 * time.Hour
	CleanTTL  = 30 * time.Minute

	redisKeyPrefix   = "alita:spamdb:"
	listedValue      = "listed"
	cleanValue       = "clean"
	maxResponseBytes = 64 << 10
	// maxLoggedBodyBytes caps how much of an unexpected CAS body is logged.
	maxLoggedBodyBytes = 200
	localCacheSize     = 10_000
	storeTimeout       = time.Second
)

// ErrInvalidUserID is returned for chat and channel IDs, which CAS does not
// index.
var ErrInvalidUserID = errors.New("spamdb: not a user id")

// Options configures a Checker. Zero values select the production defaults.
type Options struct {
	// CASAPIKey is optional for now; CAS has announced it will become required.
	CASAPIKey  string
	CASURL     string
	Timeout    time.Duration
	HTTPClient *http.Client
	// Redis returns the shared cache client, or nil to use the in-process
	// cache. It is called per lookup so a client set up later is picked up.
	Redis func() *redis.Client
}

// Checker answers whether a user is listed in CAS.
type Checker struct {
	casURL  string
	casKey  string
	timeout time.Duration
	client  *http.Client
	redis   func() *redis.Client

	listedLocal *expirable.LRU[int64, struct{}]
	cleanLocal  *expirable.LRU[int64, struct{}]
	flight      singleflight.Group
}

// New builds a Checker from opts.
func New(opts Options) *Checker {
	c := &Checker{
		casURL:      opts.CASURL,
		casKey:      opts.CASAPIKey,
		timeout:     opts.Timeout,
		client:      opts.HTTPClient,
		redis:       opts.Redis,
		listedLocal: expirable.NewLRU[int64, struct{}](localCacheSize, nil, ListedTTL),
		cleanLocal:  expirable.NewLRU[int64, struct{}](localCacheSize, nil, CleanTTL),
	}
	if c.casURL == "" {
		c.casURL = DefaultCASURL
	}
	if c.timeout <= 0 {
		c.timeout = DefaultTimeout
	}
	if c.client == nil {
		c.client = &http.Client{
			Timeout: c.timeout,
			Transport: &http.Transport{
				MaxIdleConns:    10,
				IdleConnTimeout: 90 * time.Second,
			},
		}
	}
	return c
}

// Check reports whether userID is listed in CAS. Concurrent checks for the
// same user share one lookup, which runs under the Checker's timeout even if
// the first caller gives up; ctx only bounds this caller's wait.
func (c *Checker) Check(ctx context.Context, userID int64) (bool, error) {
	if userID <= 0 {
		return false, ErrInvalidUserID
	}
	if listed, ok := c.cached(ctx, userID); ok {
		return listed, nil
	}

	ch := c.flight.DoChan(strconv.FormatInt(userID, 10), func() (any, error) {
		lookupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), c.timeout)
		defer cancel()
		listed, err := c.lookupCAS(lookupCtx, userID)
		if err != nil {
			return false, err
		}
		c.store(ctx, userID, listed)
		return listed, nil
	})

	select {
	case r := <-ch:
		if r.Err != nil {
			return false, r.Err
		}
		return r.Val.(bool), nil
	case <-ctx.Done():
		return false, ctx.Err()
	}
}

// casNotFoundText is the only ok:false answer that means "clean". CAS puts it in
// description for most ids and in error for some; anything else saying
// ok:false (rate limits, outages, bad keys) is a failure.
const casNotFoundText = "Record not found."

// CAS answers 200 {"ok":true,"result":{...}} for a listed user and
// 200 {"ok":false,"description":"Record not found."} for a clean one. Any other
// answer is an error and is logged with the start of the body, so an API change
// shows up in the logs instead of the check quietly going dark.
func (c *Checker) lookupCAS(ctx context.Context, userID int64) (bool, error) {
	var body struct {
		OK          bool            `json:"ok"`
		Result      json.RawMessage `json:"result"`
		ErrorCode   int             `json:"error_code"`
		Description string          `json:"description"`
		Error       string          `json:"error"`
	}
	raw, status, err := c.getJSON(ctx, userID, &body)
	if err != nil {
		// status is zero when no response arrived (timeout, refused
		// connection); that is an outage, not a changed API.
		if status != 0 {
			warnUnexpectedCAS(userID, status, raw)
		}
		return false, fmt.Errorf("spamdb: cas: %w", err)
	}
	if body.OK {
		if len(body.Result) == 0 || string(body.Result) == "null" {
			warnUnexpectedCAS(userID, status, raw)
			return false, errors.New("spamdb: cas: ok without a result")
		}
		return true, nil
	}
	if body.ErrorCode == 0 && (body.Description == casNotFoundText || body.Error == casNotFoundText) {
		return false, nil
	}
	warnUnexpectedCAS(userID, status, raw)
	return false, fmt.Errorf("spamdb: cas: error %d: %s%s", body.ErrorCode, body.Description, body.Error)
}

func warnUnexpectedCAS(userID int64, status int, raw []byte) {
	if len(raw) > maxLoggedBodyBytes {
		raw = raw[:maxLoggedBodyBytes]
	}
	log.Warnf("[SpamDB] unexpected CAS response for %d: HTTP %d, body %q", userID, status, raw)
}

// getJSON decodes the response into out. Once a response has arrived it also
// returns its status code and raw body (capped at maxResponseBytes), even on a
// non-200 status or a decode failure, so callers can log what the service
// actually said. The status is zero when the exchange failed in transit,
// including a body cut off after the headers.
func (c *Checker) getJSON(ctx context.Context, userID int64, out any) ([]byte, int, error) {
	u, err := url.Parse(c.casURL)
	if err != nil {
		return nil, 0, fmt.Errorf("parse url: %w", err)
	}
	q := u.Query()
	q.Set("user_id", strconv.FormatInt(userID, 10))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, 0, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if c.casKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.casKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseBytes))
		_ = resp.Body.Close()
	}()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		// The connection broke mid-body: a transport failure, so no status.
		return nil, 0, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return raw, resp.StatusCode, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return raw, resp.StatusCode, fmt.Errorf("decode response: %w", err)
	}
	return raw, resp.StatusCode, nil
}

func (c *Checker) redisClient() *redis.Client {
	if c.redis == nil {
		return nil
	}
	return c.redis()
}

func redisKey(userID int64) string {
	return redisKeyPrefix + strconv.FormatInt(userID, 10)
}

// cached returns a stored verdict. The in-process copy is checked first, so a
// verdict this replica just stored never waits on Redis. A Redis failure reads
// as a miss so the lookup still runs.
//
// The shared client sets ContextTimeoutEnabled, so a deadline on ctx also stops
// the socket read, not just the wait for a pooled connection and the pause
// between go-redis retries. A ctx without a deadline falls back to the
// client's ReadTimeout.
func (c *Checker) cached(ctx context.Context, userID int64) (listed, ok bool) {
	if _, ok := c.listedLocal.Get(userID); ok {
		return true, true
	}
	if _, ok := c.cleanLocal.Get(userID); ok {
		return false, true
	}

	rdb := c.redisClient()
	if rdb == nil {
		return false, false
	}
	v, err := rdb.Get(ctx, redisKey(userID)).Result()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			log.Debugf("[SpamDB] cache read for %d: %v", userID, err)
		}
		return false, false
	}
	switch v {
	case cleanValue:
		return false, true
	case listedValue:
		return true, true
	default:
		return false, false
	}
}

// store remembers a verdict. The in-process copy is written before store
// returns; the Redis write runs in the background so a slow or hung Redis never
// delays the verdict the join handler is waiting for.
func (c *Checker) store(ctx context.Context, userID int64, listed bool) {
	value, ttl := cleanValue, CleanTTL
	if listed {
		value, ttl = listedValue, ListedTTL
		c.listedLocal.Add(userID, struct{}{})
		c.cleanLocal.Remove(userID)
	} else {
		c.cleanLocal.Add(userID, struct{}{})
		c.listedLocal.Remove(userID)
	}

	rdb := c.redisClient()
	if rdb == nil {
		return
	}
	ctx = context.WithoutCancel(ctx)
	go func() {
		defer error_handling.RecoverFromPanic("storeRedis", "spamdb")
		// The shared client sets ContextTimeoutEnabled, so this deadline also
		// stops the socket write, not just the pool wait and the pause
		// between go-redis retries.
		ctx, cancel := context.WithTimeout(ctx, storeTimeout)
		defer cancel()
		if err := rdb.Set(ctx, redisKey(userID), value, ttl).Err(); err != nil {
			log.Debugf("[SpamDB] cache write for %d: %v", userID, err)
		}
	}()
}
