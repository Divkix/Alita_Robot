// Package spamdb looks a Telegram user up in the CAS (cas.chat) and lols.bot
// spam databases and in the bot's own shared reputation aggregate, so a chat
// can act on a known spammer when they join.
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
	"github.com/divkix/Alita_Robot/alita/utils/reputation"
)

// Source is a spam database a chat can check new members against.
type Source string

const (
	SourceCAS    Source = "cas"
	SourceLols   Source = "lols"
	SourceShared Source = "shared"
)

// Name is how the source is named to admins and in log-channel notices.
func (s Source) Name() string {
	switch s {
	case SourceCAS:
		return "CAS"
	case SourceLols:
		return "lols.bot"
	case SourceShared:
		return "Shared bans"
	}
	return string(s)
}

const (
	DefaultCASURL  = "https://api.cas.chat/check"
	DefaultLolsURL = "https://api.lols.bot/account"

	// DefaultTimeout bounds each database's lookup; the join handler waits on
	// the slowest selected one.
	DefaultTimeout = 3 * time.Second

	// A listing is rarely lifted, so it is kept longer than a clean verdict,
	// which goes stale as soon as the account starts spamming elsewhere.
	ListedTTL = 6 * time.Hour
	CleanTTL  = 30 * time.Minute

	redisKeyPrefix   = "alita:spamdb:"
	listedValue      = "listed"
	cleanValue       = "clean"
	maxResponseBytes = 64 << 10
	// maxLoggedBodyBytes caps how much of an unexpected body is logged.
	maxLoggedBodyBytes = 200
	localCacheSize     = 10_000
	storeTimeout       = time.Second
)

var (
	// ErrInvalidUserID is returned for chat and channel IDs, which neither
	// database indexes.
	ErrInvalidUserID = errors.New("spamdb: not a user id")
	// ErrNoSources is returned when a check names no known database.
	ErrNoSources = errors.New("spamdb: no database selected")
)

var errLookupPanicked = errors.New("spamdb: lookup panicked")

// Options configures a Checker. Zero values select the production defaults.
type Options struct {
	// CASAPIKey is optional for now; CAS has announced it will become required.
	CASAPIKey  string
	CASURL     string
	LolsURL    string
	Timeout    time.Duration
	HTTPClient *http.Client
	// Redis returns the shared cache client, or nil to use the in-process
	// cache. It is called per lookup so a client set up later is picked up.
	Redis func() *redis.Client
}

// verdictKey identifies one database's verdict on one user, so a chat that
// checks only CAS never reads a lols.bot verdict and the reverse.
type verdictKey struct {
	source Source
	userID int64
}

// Checker answers whether a user is listed in the selected databases.
type Checker struct {
	casURL  string
	casKey  string
	lolsURL string
	timeout time.Duration
	client  *http.Client
	redis   func() *redis.Client

	listedLocal *expirable.LRU[verdictKey, struct{}]
	cleanLocal  *expirable.LRU[verdictKey, struct{}]
	flight      singleflight.Group
}

// New builds a Checker from opts.
func New(opts Options) *Checker {
	c := &Checker{
		casURL:      opts.CASURL,
		casKey:      opts.CASAPIKey,
		lolsURL:     opts.LolsURL,
		timeout:     opts.Timeout,
		client:      opts.HTTPClient,
		redis:       opts.Redis,
		listedLocal: expirable.NewLRU[verdictKey, struct{}](localCacheSize, nil, ListedTTL),
		cleanLocal:  expirable.NewLRU[verdictKey, struct{}](localCacheSize, nil, CleanTTL),
	}
	if c.casURL == "" {
		c.casURL = DefaultCASURL
	}
	if c.lolsURL == "" {
		c.lolsURL = DefaultLolsURL
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

type answer struct {
	source Source
	listed bool
	err    error
}

// Check asks each of sources in parallel and returns the first one that lists
// userID, without waiting for the others. It returns "" only when every source
// answered clean; if none lists the user and any failed, it returns the
// failures. ctx only bounds this caller's wait: each lookup runs under the
// Checker's timeout and still caches its verdict after the caller has gone.
func (c *Checker) Check(ctx context.Context, userID int64, sources []Source) (Source, error) {
	if userID <= 0 {
		return "", ErrInvalidUserID
	}
	for _, src := range sources {
		if src != SourceCAS && src != SourceLols && src != SourceShared {
			return "", fmt.Errorf("%w: unknown %q", ErrNoSources, src)
		}
	}
	if len(sources) == 0 {
		return "", ErrNoSources
	}

	answers := make(chan answer, len(sources))
	for _, src := range sources {
		go func() {
			// The answer is sent even after a recovered panic, so Check
			// never waits on a lookup that is gone.
			a := answer{source: src, err: fmt.Errorf("spamdb: %s: lookup panicked", src)}
			defer func() { answers <- a }()
			defer error_handling.RecoverFromPanic("checkSource", "spamdb")
			a.listed, a.err = c.checkSource(ctx, src, userID)
		}()
	}

	var errs []error
	for range sources {
		select {
		case a := <-answers:
			if a.err != nil {
				errs = append(errs, a.err)
				continue
			}
			if a.listed {
				return a.source, nil
			}
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	return "", errors.Join(errs...)
}

// checkSource reports whether src lists userID. Concurrent checks of the same
// user in the same database share one lookup.
func (c *Checker) checkSource(ctx context.Context, src Source, userID int64) (bool, error) {
	if src == SourceShared {
		// reputation.Lookup keeps its own ten-minute verdict cache and fails
		// open, so caching or deduplicating it here would only serve a second
		// copy of the same answer. The count is not used by a join check.
		listed, _, err := reputation.Lookup(ctx, userID)
		return listed, err
	}
	if listed, ok := c.cached(ctx, src, userID); ok {
		return listed, nil
	}

	ch := c.flight.DoChan(string(src)+":"+strconv.FormatInt(userID, 10), func() (any, error) {
		// singleflight re-raises a panic here in a goroutine no caller can
		// recover, which would exit the bot; a panic is a failed lookup instead.
		listed, err := false, errLookupPanicked
		func() {
			defer error_handling.RecoverFromPanic("lookup", "spamdb")
			lookupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), c.timeout)
			defer cancel()
			lookup := c.lookupCAS
			if src == SourceLols {
				lookup = c.lookupLols
			}
			listed, err = lookup(lookupCtx, userID)
			if err == nil {
				c.store(ctx, src, userID, listed)
			}
		}()
		if err != nil {
			return false, err
		}
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
	raw, status, err := c.getJSON(ctx, c.casURL, "user_id", userID, c.casKey, &body)
	if err != nil {
		// status is zero when no response arrived (timeout, refused
		// connection); that is an outage, not a changed API.
		if status != 0 {
			warnUnexpected(SourceCAS, userID, status, raw)
		}
		return false, fmt.Errorf("spamdb: cas: %w", err)
	}
	if body.OK {
		if len(body.Result) == 0 || string(body.Result) == "null" {
			warnUnexpected(SourceCAS, userID, status, raw)
			return false, errors.New("spamdb: cas: ok without a result")
		}
		return true, nil
	}
	if body.ErrorCode == 0 && (body.Description == casNotFoundText || body.Error == casNotFoundText) {
		return false, nil
	}
	warnUnexpected(SourceCAS, userID, status, raw)
	return false, fmt.Errorf("spamdb: cas: error %d: %s%s", body.ErrorCode, body.Description, body.Error)
}

// lols.bot answers 200 {"ok":true,"user_id":N,"banned":bool,...}, where
// banned says whether the user is on its ban list; 400 and 500 mean a bad
// request. It takes no API key. Its OpenAPI spec is
// https://api.lols.bot/lols-bot.json. An answer without a verdict, or about
// another user, is an error and is logged like an unexpected CAS answer.
func (c *Checker) lookupLols(ctx context.Context, userID int64) (bool, error) {
	var body struct {
		OK     bool  `json:"ok"`
		UserID int64 `json:"user_id"`
		Banned *bool `json:"banned"`
	}
	raw, status, err := c.getJSON(ctx, c.lolsURL, "id", userID, "", &body)
	if err != nil {
		if status != 0 {
			warnUnexpected(SourceLols, userID, status, raw)
		}
		return false, fmt.Errorf("spamdb: lols: %w", err)
	}
	if !body.OK || body.Banned == nil || body.UserID != userID {
		warnUnexpected(SourceLols, userID, status, raw)
		return false, errors.New("spamdb: lols: response carried no verdict for the user")
	}
	return *body.Banned, nil
}

func warnUnexpected(src Source, userID int64, status int, raw []byte) {
	if len(raw) > maxLoggedBodyBytes {
		raw = raw[:maxLoggedBodyBytes]
	}
	log.Warnf("[SpamDB] unexpected %s response for %d: HTTP %d, body %q", src.Name(), userID, status, raw)
}

// getJSON queries base with param set to userID and decodes the response into
// out. apiKey, when set, goes only in the Authorization header. Once a
// response has arrived it also returns its status code and raw body (capped at
// maxResponseBytes), even on a non-200 status or a decode failure, so callers
// can log what the service actually said. The status is zero when the exchange
// failed in transit, including a body cut off after the headers.
func (c *Checker) getJSON(ctx context.Context, base, param string, userID int64, apiKey string, out any) ([]byte, int, error) {
	u, err := url.Parse(base)
	if err != nil {
		return nil, 0, fmt.Errorf("parse url: %w", err)
	}
	q := u.Query()
	q.Set(param, strconv.FormatInt(userID, 10))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, 0, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
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

func redisKey(src Source, userID int64) string {
	return redisKeyPrefix + string(src) + ":" + strconv.FormatInt(userID, 10)
}

// cached returns a stored verdict of src. The in-process copy is checked
// first, so a verdict this replica just stored never waits on Redis. A Redis
// failure reads as a miss so the lookup still runs.
//
// The shared client sets ContextTimeoutEnabled, so a deadline on ctx also stops
// the socket read, not just the wait for a pooled connection and the pause
// between go-redis retries. A ctx without a deadline falls back to the
// client's ReadTimeout.
func (c *Checker) cached(ctx context.Context, src Source, userID int64) (listed, ok bool) {
	key := verdictKey{source: src, userID: userID}
	if _, ok := c.listedLocal.Get(key); ok {
		return true, true
	}
	if _, ok := c.cleanLocal.Get(key); ok {
		return false, true
	}

	rdb := c.redisClient()
	if rdb == nil {
		return false, false
	}
	v, err := rdb.Get(ctx, redisKey(src, userID)).Result()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			log.Debugf("[SpamDB] cache read for %s:%d: %v", src, userID, err)
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

// store remembers a verdict of src. The in-process copy is written before
// store returns; the Redis write runs in the background so a slow or hung
// Redis never delays the verdict the join handler is waiting for.
func (c *Checker) store(ctx context.Context, src Source, userID int64, listed bool) {
	key := verdictKey{source: src, userID: userID}
	value, ttl := cleanValue, CleanTTL
	if listed {
		value, ttl = listedValue, ListedTTL
		c.listedLocal.Add(key, struct{}{})
		c.cleanLocal.Remove(key)
	} else {
		c.cleanLocal.Add(key, struct{}{})
		c.listedLocal.Remove(key)
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
		if err := rdb.Set(ctx, redisKey(src, userID), value, ttl).Err(); err != nil {
			log.Debugf("[SpamDB] cache write for %s:%d: %v", src, userID, err)
		}
	}()
}
