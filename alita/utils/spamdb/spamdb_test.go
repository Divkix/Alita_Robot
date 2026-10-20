package spamdb

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/alicebob/miniredis/v2/server"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
)

// Response bodies captured from the live services on 2026-10-17.
const (
	casNotFound = `{"ok":false,"description":"Record not found."}`
	// CAS answers in this form for some ids (seen for "abc" and -100123).
	casNotFoundErrorForm = `{"ok":false,"error":"Record not found."}`
	casListed            = `{"ok":true,"result":{"reasons":[1],"offenses":1,"messages":["spam"],"time_added":"2026-10-02T15:26:20.000Z"}}`
	casBadKey            = `{"ok":false,"error_code":401,"description":"Invalid API key"}`
	testUserID           = int64(1234567890)
	// lols.bot bodies follow its OpenAPI spec (https://api.lols.bot/lols-bot.json,
	// read 2026-10-18): ok, user_id and banned are always present.
	lolsBanned    = `{"ok":true,"user_id":1234567890,"banned":true,"when":"2024-06-28 17:18:05 UTC","offenses":300,"spam_factor":55.66}`
	lolsClean     = `{"ok":true,"user_id":1234567890,"banned":false}`
	testTimeout   = 200 * time.Millisecond
	slowerThanAny = 2 * time.Second
)

// fakeService is an httptest server answering the CAS API with a
// fixed status and body. It counts hits and records the last request.
type fakeService struct {
	srv   *httptest.Server
	hits  atomic.Int32
	mu    sync.Mutex
	last  *http.Request
	delay time.Duration
}

func newFakeService(t *testing.T, status int, body string) *fakeService {
	t.Helper()
	f := &fakeService{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.hits.Add(1)
		f.mu.Lock()
		f.last = r.Clone(context.Background())
		delay := f.delay
		f.mu.Unlock()
		if delay > 0 {
			select {
			case <-time.After(delay):
			case <-r.Context().Done():
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeService) setDelay(d time.Duration) {
	f.mu.Lock()
	f.delay = d
	f.mu.Unlock()
}

func (f *fakeService) lastRequest(t *testing.T) *http.Request {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.last == nil {
		t.Fatal("service was never called")
	}
	return f.last
}

func newTestChecker(cas *fakeService, opts Options) *Checker {
	opts.CASURL = cas.srv.URL + "/check"
	if opts.Timeout == 0 {
		opts.Timeout = testTimeout
	}
	return New(opts)
}

// newBothChecker points the Checker at a fake CAS and a fake lols.bot.
func newBothChecker(cas, lols *fakeService, opts Options) *Checker {
	opts.LolsURL = lols.srv.URL + "/account"
	return newTestChecker(cas, opts)
}

var bothSources = []Source{SourceCAS, SourceLols}

// checkCAS runs a CAS-only check and reports whether CAS lists the user.
func checkCAS(c *Checker, ctx context.Context, userID int64) (bool, error) {
	src, err := c.Check(ctx, userID, []Source{SourceCAS})
	if err == nil && src != "" && src != SourceCAS {
		return false, fmt.Errorf("CAS-only check reported source %q", src)
	}
	return src == SourceCAS, err
}

func newMiniredis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() error = %v", err)
	}
	t.Cleanup(mr.Close)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return mr, client
}

func TestCheckReportsCASListing(t *testing.T) {
	cas := newFakeService(t, http.StatusOK, casListed)
	c := newTestChecker(cas, Options{})

	listed, err := checkCAS(c, context.Background(), testUserID)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if !listed {
		t.Fatalf("Check() = %v, want listed by CAS", listed)
	}
	if got := cas.lastRequest(t).URL.Query().Get("user_id"); got != strconv.FormatInt(testUserID, 10) {
		t.Fatalf("CAS user_id = %q, want %d", got, testUserID)
	}
}

// Live CAS reports "not found" under either description or error.
func TestCheckCleanIsNotListed(t *testing.T) {
	for _, casBody := range []string{casNotFound, casNotFoundErrorForm} {
		t.Run(casBody, func(t *testing.T) {
			cas := newFakeService(t, http.StatusOK, casBody)
			c := newTestChecker(cas, Options{})

			listed, err := checkCAS(c, context.Background(), testUserID)
			if err != nil {
				t.Fatalf("Check() error = %v", err)
			}
			if listed {
				t.Fatalf("Check() = %v, want not listed", listed)
			}
		})
	}
}

// A failed lookup must never read as "clean": the caller logs it and lets the
// join through, but it must not be cached as a verdict.
func TestCheckFailureIsAnError(t *testing.T) {
	cases := []struct {
		name      string
		casStatus int
		casBody   string
	}{
		{"cas invalid key", http.StatusUnauthorized, casBadKey},
		{"cas error_code on 200", http.StatusOK, `{"ok":false,"error_code":429,"description":"Too many requests"}`},
		{"cas garbage", http.StatusOK, `<html>`},
		{"cas empty object", http.StatusOK, `{}`},
		{"cas null", http.StatusOK, `null`},
		{"cas bare ok false", http.StatusOK, `{"ok":false}`},
		{"cas error without code", http.StatusOK, `{"ok":false,"error":"Service temporarily unavailable"}`},
		{"cas description without code", http.StatusOK, `{"ok":false,"description":"Rate limit exceeded"}`},
		{"cas ok without result", http.StatusOK, `{"ok":true}`},
		{"cas ok with null result", http.StatusOK, `{"ok":true,"result":null}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cas := newFakeService(t, tc.casStatus, tc.casBody)
			c := newTestChecker(cas, Options{})

			listed, err := checkCAS(c, context.Background(), testUserID)
			if err == nil {
				t.Fatalf("Check() = %v, nil error; want an error", listed)
			}
			if listed {
				t.Fatalf("Check() = %v, want not listed on error", listed)
			}
		})
	}
}

// panickingTransport stands in for any bug below the lookup that panics.
type panickingTransport struct {
	calls atomic.Int32
}

func (p *panickingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	p.calls.Add(1)
	panic("transport bug")
}

// The shared lookup runs in a goroutine of its own, where singleflight
// re-raises a panic out of reach of any caller and the bot would exit. A
// panic there is a failed lookup instead: an error, nothing cached.
func TestCheckTurnsLookupPanicIntoError(t *testing.T) {
	transport := &panickingTransport{}
	c := New(Options{
		CASURL:     "http://cas.invalid/check",
		Timeout:    testTimeout,
		HTTPClient: &http.Client{Transport: transport},
	})

	for attempt := 1; attempt <= 2; attempt++ {
		source, err := c.Check(context.Background(), testUserID, []Source{SourceCAS})
		if err == nil {
			t.Fatalf("attempt %d: Check() = %q, nil error; want an error", attempt, source)
		}
		if source != "" {
			t.Fatalf("attempt %d: Check() = %q, want no source on error", attempt, source)
		}
	}
	if got := transport.calls.Load(); got != 2 {
		t.Fatalf("transport calls = %d, want 2: a panicked lookup must not be cached", got)
	}
}

func TestCheckGivesUpAtSharedDeadline(t *testing.T) {
	cas := newFakeService(t, http.StatusOK, casNotFound)
	cas.setDelay(slowerThanAny)
	c := newTestChecker(cas, Options{})

	start := time.Now()
	listed, err := checkCAS(c, context.Background(), testUserID)
	elapsed := time.Since(start)

	if err == nil || listed {
		t.Fatalf("Check() = %v, %v; want not listed with an error", listed, err)
	}
	if elapsed > testTimeout+time.Second/2 {
		t.Fatalf("Check() took %v, want about %v", elapsed, testTimeout)
	}
}

func TestCheckSendsCASAPIKeyOnlyWhenConfigured(t *testing.T) {
	cas := newFakeService(t, http.StatusOK, casNotFound)

	if _, err := checkCAS(newTestChecker(cas, Options{}), context.Background(), testUserID); err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if got := cas.lastRequest(t).Header.Get("Authorization"); got != "" {
		t.Fatalf("Authorization without key = %q, want empty", got)
	}

	if _, err := checkCAS(newTestChecker(cas, Options{CASAPIKey: "cas-test-key"}), context.Background(), testUserID+1); err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if got := cas.lastRequest(t).Header.Get("Authorization"); got != "Bearer cas-test-key" {
		t.Fatalf("Authorization = %q, want Bearer cas-test-key", got)
	}
}

// The key travels only in the Authorization header: a URL is logged by
// proxies and repeated in Go's transport errors, so the key must never be in
// the request URL, nor in the error a failed lookup returns.
func TestCheckKeepsCASAPIKeyOutOfURLAndErrors(t *testing.T) {
	const key = "cas-secret-key-7d41b2"

	cas := newFakeService(t, http.StatusOK, casNotFound)
	if _, err := checkCAS(newTestChecker(cas, Options{CASAPIKey: key}), context.Background(), testUserID); err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if uri := cas.lastRequest(t).RequestURI; strings.Contains(uri, key) {
		t.Fatalf("request URI %q carries the API key", uri)
	}

	bad := newFakeService(t, http.StatusBadGateway, `upstream down`)
	down := newFakeService(t, http.StatusOK, casNotFound)
	down.srv.Close()
	for name, svc := range map[string]*fakeService{"non-200": bad, "unreachable": down} {
		_, err := checkCAS(newTestChecker(svc, Options{CASAPIKey: key, Redis: func() *redis.Client { return nil }}), context.Background(), testUserID+1)
		if err == nil {
			t.Fatalf("%s: Check() error = nil, want a failure", name)
		}
		if strings.Contains(err.Error(), key) {
			t.Fatalf("%s: error %q carries the API key", name, err)
		}
	}
}

func TestCheckRejectsNonUserIDWithoutRequests(t *testing.T) {
	cas := newFakeService(t, http.StatusOK, casListed)
	c := newTestChecker(cas, Options{})

	for _, id := range []int64{0, -1001234567890} {
		if listed, err := checkCAS(c, context.Background(), id); err == nil || listed {
			t.Fatalf("Check(%d) = %v, %v; want an error", id, listed, err)
		}
	}
	if got := cas.hits.Load(); got != 0 {
		t.Fatalf("CAS hit %d times, want 0", got)
	}
}

func TestCheckCachesVerdictsInRedis(t *testing.T) {
	cases := []struct {
		name    string
		casBody string
		wantTTL time.Duration
		listed  bool
	}{
		{"listed", casListed, ListedTTL, true},
		{"clean", casNotFound, CleanTTL, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mr, client := newMiniredis(t)
			cas := newFakeService(t, http.StatusOK, tc.casBody)
			c := newTestChecker(cas, Options{Redis: func() *redis.Client { return client }})

			for i := 0; i < 2; i++ {
				listed, err := checkCAS(c, context.Background(), testUserID)
				if err != nil || listed != tc.listed {
					t.Fatalf("Check() #%d = %v, %v; want listed=%v", i, listed, err, tc.listed)
				}
			}
			if got := cas.hits.Load(); got != 1 {
				t.Fatalf("CAS hit %d times, want 1 (second check from cache)", got)
			}
			key := "alita:spamdb:cas:" + strconv.FormatInt(testUserID, 10)
			waitForRedisKey(t, mr, key)
			if ttl := mr.TTL(key); ttl != tc.wantTTL {
				t.Fatalf("TTL(%s) = %v, want %v", key, ttl, tc.wantTTL)
			}
		})
	}
}

func TestCheckDoesNotCacheFailures(t *testing.T) {
	mr, client := newMiniredis(t)
	cas := newFakeService(t, http.StatusUnauthorized, casBadKey)
	c := newTestChecker(cas, Options{Redis: func() *redis.Client { return client }})

	for i := 0; i < 2; i++ {
		if _, err := checkCAS(c, context.Background(), testUserID); err == nil {
			t.Fatalf("Check() #%d error = nil, want CAS failure", i)
		}
	}
	if got := cas.hits.Load(); got != 2 {
		t.Fatalf("CAS hit %d times, want 2 (failure not cached)", got)
	}
	if keys := mr.Keys(); len(keys) != 0 {
		t.Fatalf("Redis keys = %v, want none", keys)
	}
}

func TestCheckCachesInProcessWithoutRedis(t *testing.T) {
	cas := newFakeService(t, http.StatusOK, casListed)
	c := newTestChecker(cas, Options{Redis: func() *redis.Client { return nil }})

	for i := 0; i < 2; i++ {
		listed, err := checkCAS(c, context.Background(), testUserID)
		if err != nil || !listed {
			t.Fatalf("Check() #%d = %v, %v; want listed by CAS", i, listed, err)
		}
	}
	if got := cas.hits.Load(); got != 1 {
		t.Fatalf("CAS hit %d times, want 1 (second check from local cache)", got)
	}
}

// One join arrives as both a chat_member update and a service message; the
// two concurrent checks must share one lookup.
func TestCheckSharesConcurrentLookups(t *testing.T) {
	cas := newFakeService(t, http.StatusOK, casNotFound)
	cas.setDelay(100 * time.Millisecond)
	c := newTestChecker(cas, Options{Timeout: slowerThanAny, Redis: func() *redis.Client { return nil }})

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := checkCAS(c, context.Background(), testUserID); err != nil {
				t.Errorf("Check() error = %v", err)
			}
		}()
	}
	wg.Wait()

	if got := cas.hits.Load(); got != 1 {
		t.Fatalf("CAS hit %d times, want 1", got)
	}
}

func TestCheckHonoursCallerCancellation(t *testing.T) {
	cas := newFakeService(t, http.StatusOK, casNotFound)
	cas.setDelay(slowerThanAny)
	c := newTestChecker(cas, Options{Timeout: 4 * testTimeout})

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout/4)
	defer cancel()
	start := time.Now()
	if _, err := checkCAS(c, ctx, testUserID); err == nil {
		t.Fatal("Check() error = nil, want the caller's deadline")
	}
	if elapsed := time.Since(start); elapsed > 2*testTimeout {
		t.Fatalf("Check() took %v after the caller gave up", elapsed)
	}
}

// A lookup whose caller gave up still finishes and caches its verdict, so the
// second event of the same join does not ask again.
func TestCheckAbandonedLookupStillCaches(t *testing.T) {
	cas := newFakeService(t, http.StatusOK, casListed)
	cas.setDelay(testTimeout / 2)
	c := newTestChecker(cas, Options{Timeout: 4 * testTimeout, Redis: func() *redis.Client { return nil }})

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout/10)
	defer cancel()
	if _, err := checkCAS(c, ctx, testUserID); err == nil {
		t.Fatal("Check() error = nil, want the caller's deadline")
	}

	deadline := time.Now().Add(4 * testTimeout)
	for {
		if _, ok := c.cached(context.Background(), SourceCAS, testUserID); ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("abandoned lookup never cached its verdict")
		}
		time.Sleep(10 * time.Millisecond)
	}

	listed, err := checkCAS(c, context.Background(), testUserID)
	if err != nil || !listed {
		t.Fatalf("Check() = %v, %v; want listed by CAS from cache", listed, err)
	}
	if got := cas.hits.Load(); got != 1 {
		t.Fatalf("CAS hit %d times, want 1", got)
	}
}

// waitForRedisKey waits for the background Redis write that follows a lookup.
func waitForRedisKey(t *testing.T, mr *miniredis.Miniredis, key string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !mr.Exists(key) {
		if time.Now().After(deadline) {
			t.Fatalf("Redis key %s was never written", key)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// The verdict must reach the caller without waiting for the Redis write, and
// a repeat check must not repeat the lookup while that write is still pending.
func TestCheckDoesNotWaitForSlowRedisWrite(t *testing.T) {
	const redisSetDelay = 800 * time.Millisecond
	mr, client := newMiniredis(t)
	mr.Server().SetPreHook(func(_ *server.Peer, cmd string, _ ...string) bool {
		if cmd == "SET" {
			time.Sleep(redisSetDelay)
		}
		return false
	})
	cas := newFakeService(t, http.StatusOK, casListed)
	c := newTestChecker(cas, Options{Timeout: slowerThanAny, Redis: func() *redis.Client { return client }})

	start := time.Now()
	listed, err := checkCAS(c, context.Background(), testUserID)
	if elapsed := time.Since(start); elapsed > redisSetDelay/2 {
		t.Fatalf("Check() took %v; it waited for the Redis write", elapsed)
	}
	if err != nil || !listed {
		t.Fatalf("Check() = %v, %v; want listed by CAS", listed, err)
	}

	if listed, err = checkCAS(c, context.Background(), testUserID); err != nil || !listed {
		t.Fatalf("second Check() = %v, %v; want listed", listed, err)
	}
	if got := cas.hits.Load(); got != 1 {
		t.Fatalf("CAS hit %d times, want 1 (second check from the in-process copy)", got)
	}
	waitForRedisKey(t, mr, "alita:spamdb:cas:"+strconv.FormatInt(testUserID, 10))
}

// An unexpected CAS answer is an error and must leave the start of the body in
// the log, so an API change is visible.
func TestCheckLogsUnexpectedCASResponse(t *testing.T) {
	long := `{"ok":false,"description":"` + strings.Repeat("x", 500) + `"}`
	cases := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"unknown error", http.StatusOK, `{"ok":false,"error_code":429,"description":"Too many requests"}`, "Too many requests"},
		{"non-200", http.StatusBadGateway, `upstream exploded`, "upstream exploded"},
		{"non-200 empty body", http.StatusBadGateway, ``, "HTTP 502"},
		{"empty 200", http.StatusOK, ``, "HTTP 200"},
		{"not json", http.StatusOK, `<html>maintenance</html>`, "maintenance"},
		{"ok without result", http.StatusOK, `{"ok":true,"data":"moved"}`, "moved"},
		{"long body", http.StatusOK, long, strings.Repeat("x", 100)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hook := captureLogs(t)

			cas := newFakeService(t, tc.status, tc.body)
			c := newTestChecker(cas, Options{Redis: func() *redis.Client { return nil }})
			if _, err := checkCAS(c, context.Background(), testUserID); err == nil {
				t.Fatal("Check() error = nil, want a CAS failure")
			}

			var msg string
			for _, e := range hook.AllEntries() {
				if e.Level == logrus.WarnLevel && strings.Contains(e.Message, "unexpected CAS response") {
					msg = e.Message
				}
			}
			if msg == "" {
				t.Fatalf("no warning logged; entries = %v", hook.AllEntries())
			}
			if !strings.Contains(msg, tc.want) {
				t.Fatalf("warning %q does not carry the body %q", msg, tc.want)
			}
			if len(msg) > 400 {
				t.Fatalf("warning is %d bytes; the body was not truncated", len(msg))
			}
		})
	}
}

// A clean or listed answer is expected, and so is a transport failure: an
// unreachable CAS is an outage, not a changed API.
func TestCheckDoesNotWarnForExpectedCASAnswers(t *testing.T) {
	hook := captureLogs(t)

	for _, body := range []string{casNotFound, casNotFoundErrorForm, casListed} {
		cas := newFakeService(t, http.StatusOK, body)
		c := newTestChecker(cas, Options{Redis: func() *redis.Client { return nil }})
		if _, err := checkCAS(c, context.Background(), testUserID); err != nil {
			t.Fatalf("Check(%s) error = %v", body, err)
		}
	}

	cas := newFakeService(t, http.StatusOK, casNotFound)
	cas.srv.Close()
	c := newTestChecker(cas, Options{Redis: func() *redis.Client { return nil }})
	if _, err := checkCAS(c, context.Background(), testUserID); err == nil {
		t.Fatal("Check() with CAS unreachable: error = nil, want an error")
	}

	for _, e := range hook.AllEntries() {
		if e.Level <= logrus.WarnLevel {
			t.Fatalf("unexpected %s log: %q", e.Level, e.Message)
		}
	}
}

// A connection that drops mid-body is a transport failure, like an unreachable
// host: the check fails, but nothing says the CAS API changed.
func TestCheckDoesNotWarnWhenCASBodyIsCutOff(t *testing.T) {
	hook := captureLogs(t)

	cas := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "500")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":`))
		w.(http.Flusher).Flush()
		panic(http.ErrAbortHandler)
	}))
	t.Cleanup(cas.Close)
	c := New(Options{
		CASURL:  cas.URL + "/check",
		Timeout: testTimeout,
		Redis:   func() *redis.Client { return nil },
	})

	if _, err := checkCAS(c, context.Background(), testUserID); err == nil {
		t.Fatal("Check() error = nil, want a CAS failure")
	}
	for _, e := range hook.AllEntries() {
		if e.Level <= logrus.WarnLevel {
			t.Fatalf("unexpected %s log: %q", e.Level, e.Message)
		}
	}
}

// captureLogs records what the package logs to the standard logger.
func captureLogs(t *testing.T) *logtest.Hook {
	t.Helper()
	std := logrus.StandardLogger()
	old := std.ReplaceHooks(make(logrus.LevelHooks))
	hook := logtest.NewGlobal()
	t.Cleanup(func() { std.ReplaceHooks(old) })
	return hook
}
