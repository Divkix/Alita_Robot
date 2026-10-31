//go:build testtools

package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	log "github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"go.opentelemetry.io/otel/trace"
)

func TestAdminCacheHelpersHandleNilMarshal(t *testing.T) {
	originalMarshal := GetMarshal()
	SetMarshal(nil)
	t.Cleanup(func() {
		SetMarshal(originalMarshal)
	})

	found, adminCache := GetAdminCacheList(-100123)
	if found {
		t.Fatalf("GetAdminCacheList() found = true, cache = %+v", adminCache)
	}

	found, member := GetAdminCacheUser(-100123, 42)
	if found {
		t.Fatalf("GetAdminCacheUser() found = true, member = %+v", member)
	}

	InvalidateAdminCache(-100123)
}

type adminCacheBotClient struct {
	responses           map[string]json.RawMessage
	errors              map[string]error
	lastAdminListParams map[string]any
}

func newAdminCacheBot(client *adminCacheBotClient) *gotgbot.Bot {
	return &gotgbot.Bot{
		Token:     "999:test",
		BotClient: client,
		User: gotgbot.User{
			Id:       999,
			IsBot:    true,
			Username: "AlitaTestBot",
		},
	}
}

func (c *adminCacheBotClient) RequestWithContext(_ context.Context, _ string, method string, params map[string]any, _ *gotgbot.RequestOpts) (json.RawMessage, error) {
	if err := c.errors[method]; err != nil {
		return nil, err
	}
	if method == "getChatAdministrators" {
		c.lastAdminListParams = params
	}
	if response, ok := c.responses[method+":"+fmt.Sprint(params["user_id"])]; ok {
		return response, nil
	}
	if response, ok := c.responses[method]; ok {
		return response, nil
	}
	return nil, fmt.Errorf("unexpected method %s", method)
}

func (c *adminCacheBotClient) GetAPIURL(*gotgbot.RequestOpts) string {
	return "https://api.telegram.org"
}

func (c *adminCacheBotClient) FileURL(token string, path string, _ *gotgbot.RequestOpts) string {
	return "https://api.telegram.org/file/bot" + token + "/" + path
}

func TestLoadAdminCacheFetchesAndStoresAdminMap(t *testing.T) {
	withMemoryMarshaler(t)

	client := &adminCacheBotClient{responses: map[string]json.RawMessage{
		"getChatMember:999": json.RawMessage(
			`{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"}}`,
		),
		"getChatAdministrators": json.RawMessage(
			`[` +
				`{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"}},` +
				`{"status":"creator","user":{"id":777000,"is_bot":false,"first_name":"Telegram"}}` +
				`]`,
		),
	}}
	got := LoadAdminCache(newAdminCacheBot(client), -100123)

	if client.lastAdminListParams["return_bots"] != true {
		t.Fatalf("getChatAdministrators return_bots = %#v, want true", client.lastAdminListParams["return_bots"])
	}
	if !got.Cached || len(got.UserInfo) != 2 {
		t.Fatalf("LoadAdminCache() = %+v, want cached admin list", got)
	}
	if !slices.ContainsFunc(got.UserInfo, func(m gotgbot.MergedChatMember) bool {
		return m.User.Id == 777000 && m.User.FirstName == "Telegram"
	}) {
		t.Fatalf("LoadAdminCache() = %+v, want Telegram admin 777000", got)
	}
	if found, _ := GetAdminCacheList(-100123); !found {
		t.Fatal("LoadAdminCache did not store the admin list before returning")
	}
}

func TestLoadAdminCacheHandlesNilBotNonAdminBotAndEmptyAdminList(t *testing.T) {
	withMemoryMarshaler(t)

	if got := LoadAdminCache(nil, -100123); got.Cached || len(got.UserInfo) != 0 {
		t.Fatalf("LoadAdminCache(nil) = %+v, want empty uncached result", got)
	}

	memberClient := &adminCacheBotClient{responses: map[string]json.RawMessage{
		"getChatMember:999": json.RawMessage(
			`{"status":"member","user":{"id":999,"is_bot":true,"first_name":"Alita"}}`,
		),
	}}
	if got := LoadAdminCache(newAdminCacheBot(memberClient), -100124); !got.Cached || len(got.UserInfo) != 0 {
		t.Fatalf("LoadAdminCache(non-admin bot) = %+v, want cached empty result", got)
	}
	if found, _ := GetAdminCacheList(-100124); !found {
		t.Fatal("LoadAdminCache did not store the non-admin result")
	}

	emptyClient := &adminCacheBotClient{responses: map[string]json.RawMessage{
		"getChatMember:999": json.RawMessage(
			`{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"}}`,
		),
		"getChatAdministrators": json.RawMessage(`[]`),
	}}
	if got := LoadAdminCache(newAdminCacheBot(emptyClient), -100125); !got.Cached || len(got.UserInfo) != 0 {
		t.Fatalf("LoadAdminCache(empty admins) = %+v, want cached empty result", got)
	}
	if found, _ := GetAdminCacheList(-100125); !found {
		t.Fatal("LoadAdminCache did not store the empty admin-list result")
	}
}

// Production asked Telegram again each time the 2-minute "bot is not admin"
// list expired. A recorded refusal must last as long as a stored admin list.
func TestMarkMemberLookupForbiddenOutlivesTheNegativeTTL(t *testing.T) {
	withMemoryMarshaler(t)
	const chatID = int64(-100126)
	client := &adminCacheBotClient{responses: map[string]json.RawMessage{
		"getChatMember:999": json.RawMessage(`{"status":"member","user":{"id":999,"is_bot":true,"first_name":"Alita"}}`),
	}}
	LoadAdminCache(newAdminCacheBot(client), chatID)
	MarkMemberLookupForbidden(chatID)

	if found, ac := GetAdminCacheList(chatID); !found || !ac.Negative || !ac.MemberLookupForbidden {
		t.Fatalf("stored list = (%v, %+v), want a Negative list marked forbidden", found, ac)
	}
	_, ttl, err := GetCacheManager().GetWithTTL(Context, adminCacheKey(chatID))
	if err != nil {
		t.Fatalf("GetWithTTL: %v", err)
	}
	if ttl <= negativeAdminCacheTTL {
		t.Fatalf("refusal TTL = %v, want longer than the %v negative TTL", ttl, negativeAdminCacheTTL)
	}
}

type delayedAdminCacheClient struct {
	inner            *adminCacheBotClient
	adminListStarted chan struct{}
	releaseAdminList chan struct{}
	adminListCalls   atomic.Int32
}

func (c *delayedAdminCacheClient) RequestWithContext(ctx context.Context, token, method string, params map[string]any, opts *gotgbot.RequestOpts) (json.RawMessage, error) {
	if method == "getChatAdministrators" {
		if c.adminListCalls.Add(1) == 1 {
			close(c.adminListStarted)
			<-c.releaseAdminList
		}
	}
	return c.inner.RequestWithContext(ctx, token, method, params, opts)
}

func (c *delayedAdminCacheClient) GetAPIURL(opts *gotgbot.RequestOpts) string {
	return c.inner.GetAPIURL(opts)
}

func (c *delayedAdminCacheClient) FileURL(token, path string, opts *gotgbot.RequestOpts) string {
	return c.inner.FileURL(token, path, opts)
}

func TestLoadAdminCacheCoalescesConcurrentFetches(t *testing.T) {
	withMemoryMarshaler(t)

	inner := &adminCacheBotClient{responses: map[string]json.RawMessage{
		"getChatMember:999": json.RawMessage(
			`{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"}}`,
		),
		"getChatAdministrators": json.RawMessage(
			`[{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"}}]`,
		),
	}}
	client := &delayedAdminCacheClient{
		inner:            inner,
		adminListStarted: make(chan struct{}),
		releaseAdminList: make(chan struct{}),
	}
	bot := &gotgbot.Bot{
		Token:     "999:test",
		BotClient: client,
		User: gotgbot.User{
			Id:       999,
			IsBot:    true,
			Username: "AlitaTestBot",
		},
	}

	const workers = 8
	var wg sync.WaitGroup
	start := make(chan struct{})
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			<-start
			got := LoadAdminCache(bot, -100777)
			if !got.Cached || len(got.UserInfo) != 1 {
				t.Errorf("LoadAdminCache() = %+v, want cached admin list", got)
			}
		}()
	}
	close(start)
	<-client.adminListStarted
	// Release the fetch only once every worker waits on it, so none can be
	// served from the cache it fills.
	waitForAdminLoadTest(t, "every worker to join the running load", func() bool {
		load := runningAdminLoad(adminCacheKey(-100777))
		return load != nil && adminLoadWaiters(load) == workers
	})
	close(client.releaseAdminList)
	wg.Wait()

	if got := client.adminListCalls.Load(); got != 1 {
		t.Fatalf("getChatAdministrators calls = %d, want 1 coalesced fetch", got)
	}
}

// hungBotStatusClient stalls the bot's own getChatMember like a hung Telegram:
// it returns when released, when the request context ends, or after stall.
type hungBotStatusClient struct {
	inner   *adminCacheBotClient
	started chan struct{}
	release chan struct{}
	stall   time.Duration
	once    sync.Once
}

func (c *hungBotStatusClient) RequestWithContext(ctx context.Context, token, method string, params map[string]any, opts *gotgbot.RequestOpts) (json.RawMessage, error) {
	if method == "getChatMember" && fmt.Sprint(params["user_id"]) == "999" {
		c.once.Do(func() { close(c.started) })
		select {
		case <-c.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(c.stall):
		}
	}
	return c.inner.RequestWithContext(ctx, token, method, params, opts)
}

func (c *hungBotStatusClient) GetAPIURL(opts *gotgbot.RequestOpts) string {
	return c.inner.GetAPIURL(opts)
}

func (c *hungBotStatusClient) FileURL(token, path string, opts *gotgbot.RequestOpts) string {
	return c.inner.FileURL(token, path, opts)
}

// A caller whose deadline ends while the admin list loads must neither cache
// a failure (an empty list) nor cut the load short for a concurrent caller.
func TestLoadAdminCacheWithContextDeadlineLeavesSharedLoadIntact(t *testing.T) {
	withMemoryMarshaler(t)
	const chatID = -100779
	client := &hungBotStatusClient{
		inner: &adminCacheBotClient{responses: map[string]json.RawMessage{
			"getChatMember:999":     json.RawMessage(`{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"}}`),
			"getChatAdministrators": json.RawMessage(`[{"status":"administrator","user":{"id":42,"is_bot":false,"first_name":"Admin"}}]`),
		}},
		started: make(chan struct{}),
		release: make(chan struct{}),
		stall:   2 * time.Second,
	}
	bot := newAdminCacheBot(client.inner)
	bot.BotClient = client
	release := sync.OnceFunc(func() { close(client.release) })
	t.Cleanup(release)

	other := make(chan AdminCache, 1)
	go func() {
		<-client.started
		other <- LoadAdminCache(bot, chatID)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	got, err := LoadAdminCacheWithContext(ctx, bot, chatID)
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("LoadAdminCacheWithContext returned after %v, want it to stop at the 50ms deadline", elapsed)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("LoadAdminCacheWithContext error = %v, want context.DeadlineExceeded", err)
	}
	if got.Cached || len(got.UserInfo) != 0 {
		t.Fatalf("LoadAdminCacheWithContext = %+v, want an empty uncached result", got)
	}
	if found, stored := GetAdminCacheList(chatID); found {
		t.Fatalf("deadline stored an admin list: %+v", stored)
	}

	release()
	select {
	case ac := <-other:
		if !slices.ContainsFunc(ac.UserInfo, func(m gotgbot.MergedChatMember) bool { return m.User.Id == 42 }) {
			t.Fatalf("concurrent LoadAdminCache = %+v, want admin 42", ac)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("concurrent LoadAdminCache did not finish")
	}
	if found, stored := GetAdminCacheList(chatID); !found || len(stored.UserInfo) != 1 {
		t.Fatalf("stored admin list = (%v, %+v), want the loaded list", found, stored)
	}
}

func TestAdminCacheInvalidationDiscardsPendingPermissions(t *testing.T) {
	withMemoryMarshaler(t)
	oldClient := &delayedAdminCacheClient{
		inner: &adminCacheBotClient{responses: map[string]json.RawMessage{
			"getChatMember:999":     json.RawMessage(`{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"}}`),
			"getChatAdministrators": json.RawMessage(`[{"status":"administrator","can_restrict_members":true,"user":{"id":42,"first_name":"Admin"}}]`),
		}},
		adminListStarted: make(chan struct{}),
		releaseAdminList: make(chan struct{}),
	}
	oldBot := newAdminCacheBot(oldClient.inner)
	oldBot.BotClient = oldClient
	finished := make(chan struct{})
	go func() {
		LoadAdminCache(oldBot, -100778)
		close(finished)
	}()
	release := sync.OnceFunc(func() { close(oldClient.releaseAdminList) })
	t.Cleanup(func() { release(); <-finished })
	<-oldClient.adminListStarted
	InvalidateAdminCache(-100778)
	newClient := &adminCacheBotClient{responses: map[string]json.RawMessage{
		"getChatMember:999":     json.RawMessage(`{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"}}`),
		"getChatAdministrators": json.RawMessage(`[]`),
	}}
	fresh := make(chan AdminCache, 1)
	go func() { fresh <- LoadAdminCache(newAdminCacheBot(newClient), -100778) }()
	select {
	case got := <-fresh:
		if len(got.UserInfo) != 0 {
			t.Fatalf("new lookup returned revoked administrators: %+v", got.UserInfo)
		}
	case <-time.After(time.Second):
		t.Fatal("lookup after invalidation joined an outdated request")
	}
	release()
	<-finished
	if found, got := GetAdminCacheList(-100778); !found || len(got.UserInfo) != 0 {
		t.Fatalf("delayed lookup replaced updated permissions: found=%v, admins=%+v", found, got.UserInfo)
	}
}

// panickingAdminListClient panics inside getChatAdministrators.
type panickingAdminListClient struct {
	inner *adminCacheBotClient
}

func (c *panickingAdminListClient) RequestWithContext(ctx context.Context, token, method string, params map[string]any, opts *gotgbot.RequestOpts) (json.RawMessage, error) {
	if method == "getChatAdministrators" {
		panic("admin list exploded")
	}
	return c.inner.RequestWithContext(ctx, token, method, params, opts)
}

func (c *panickingAdminListClient) GetAPIURL(opts *gotgbot.RequestOpts) string {
	return c.inner.GetAPIURL(opts)
}

func (c *panickingAdminListClient) FileURL(token, path string, opts *gotgbot.RequestOpts) string {
	return c.inner.FileURL(token, path, opts)
}

// A panicking load must not read as "no admins": LoadAdminCache passes the
// panic on to its caller as before, the context variant returns an error, and
// nothing is cached.
func TestLoadAdminCachePanicIsNotAnEmptyAdminList(t *testing.T) {
	withMemoryMarshaler(t)
	const chatID = -100780
	inner := &adminCacheBotClient{responses: map[string]json.RawMessage{
		"getChatMember:999": json.RawMessage(`{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"}}`),
	}}
	bot := newAdminCacheBot(inner)
	bot.BotClient = &panickingAdminListClient{inner: inner}

	recovered := func() (r any) {
		defer func() { r = recover() }()
		LoadAdminCache(bot, chatID)
		return nil
	}()
	if recovered == nil {
		t.Fatal("LoadAdminCache returned after the load panicked, want the panic passed on")
	}

	got, err := LoadAdminCacheWithContext(context.Background(), bot, chatID)
	if err == nil {
		t.Fatalf("LoadAdminCacheWithContext = %+v, nil error after the load panicked", got)
	}
	if found, stored := GetAdminCacheList(chatID); found {
		t.Fatalf("panicked load stored an admin list: %+v", stored)
	}
}

// captureErrorLogs records the standard logger's entries for the test.
func captureErrorLogs(t *testing.T) *logtest.Hook {
	t.Helper()
	logger := log.StandardLogger()
	previous := logger.ReplaceHooks(make(log.LevelHooks))
	for level, hooks := range previous {
		logger.Hooks[level] = append(logger.Hooks[level], hooks...)
	}
	t.Cleanup(func() { logger.ReplaceHooks(previous) })
	return logtest.NewLocal(logger)
}

func panicLogs(hook *logtest.Hook, text string) []*log.Entry {
	var found []*log.Entry
	for _, entry := range hook.AllEntries() {
		if entry.Level <= log.ErrorLevel && strings.Contains(entry.Message, text) {
			found = append(found, entry)
		}
	}
	return found
}

// A load panic that reaches a waiting caller is logged once, by that caller's
// handler: the loader stays quiet and the re-raised panic carries its stack.
func TestLoadAdminCachePanicIsLoggedOnceByItsCaller(t *testing.T) {
	withMemoryMarshaler(t)
	hook := captureErrorLogs(t)
	inner := &adminCacheBotClient{responses: map[string]json.RawMessage{
		"getChatMember:999": json.RawMessage(`{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"}}`),
	}}
	bot := newAdminCacheBot(inner)
	bot.BotClient = &panickingAdminListClient{inner: inner}

	recovered := func() (r any) {
		defer func() { r = recover() }()
		LoadAdminCache(bot, -100781)
		return nil
	}()
	if recovered == nil {
		t.Fatal("LoadAdminCache returned after the load panicked, want the panic passed on")
	}
	if got := fmt.Sprint(recovered); !strings.Contains(got, "admin list exploded") || !strings.Contains(got, "loadAdminCacheFromTelegram") {
		t.Fatalf("re-raised panic = %q, want the value and the loader's stack", got)
	}
	if logs := panicLogs(hook, "admin list exploded"); len(logs) != 0 {
		t.Fatalf("loader logged the panic its caller re-raises: %d entries", len(logs))
	}
}

// stalledPanickingAdminListClient panics in getChatAdministrators once released.
type stalledPanickingAdminListClient struct {
	inner   *adminCacheBotClient
	started chan struct{}
	release chan struct{}
	done    chan struct{}
}

func (c *stalledPanickingAdminListClient) RequestWithContext(ctx context.Context, token, method string, params map[string]any, opts *gotgbot.RequestOpts) (json.RawMessage, error) {
	if method == "getChatAdministrators" {
		close(c.started)
		<-c.release
		defer close(c.done)
		panic("orphaned admin list exploded")
	}
	return c.inner.RequestWithContext(ctx, token, method, params, opts)
}

func (c *stalledPanickingAdminListClient) GetAPIURL(opts *gotgbot.RequestOpts) string {
	return c.inner.GetAPIURL(opts)
}

func (c *stalledPanickingAdminListClient) FileURL(token, path string, opts *gotgbot.RequestOpts) string {
	return c.inner.FileURL(token, path, opts)
}

// A load panic after every caller stopped waiting still gets one log entry.
func TestLoadAdminCachePanicWithNoCallerLeftIsLoggedOnce(t *testing.T) {
	withMemoryMarshaler(t)
	hook := captureErrorLogs(t)
	inner := &adminCacheBotClient{responses: map[string]json.RawMessage{
		"getChatMember:999": json.RawMessage(`{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"}}`),
	}}
	client := &stalledPanickingAdminListClient{inner: inner, started: make(chan struct{}), release: make(chan struct{}), done: make(chan struct{})}
	bot := newAdminCacheBot(inner)
	bot.BotClient = client

	ctx, cancel := context.WithCancel(context.Background())
	go func() { <-client.started; cancel() }()
	if _, err := LoadAdminCacheWithContext(ctx, bot, -100782); !errors.Is(err, context.Canceled) {
		t.Fatalf("LoadAdminCacheWithContext error = %v, want context.Canceled", err)
	}
	close(client.release)
	<-client.done

	deadline := time.Now().Add(2 * time.Second)
	for len(panicLogs(hook, "orphaned admin list exploded")) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)
	logs := panicLogs(hook, "orphaned admin list exploded")
	if len(logs) != 1 || !strings.Contains(logs[0].Message, "loadAdminCacheFromTelegram") {
		t.Fatalf("panic log entries = %d, want one with the loader's stack", len(logs))
	}
}

// cancellingPanickingAdminListClient ends the caller's context and then
// panics in getChatAdministrators, so the caller can leave as the load fails.
type cancellingPanickingAdminListClient struct {
	inner  *adminCacheBotClient
	cancel context.CancelFunc
}

func (c *cancellingPanickingAdminListClient) RequestWithContext(ctx context.Context, token, method string, params map[string]any, opts *gotgbot.RequestOpts) (json.RawMessage, error) {
	if method == "getChatAdministrators" {
		c.cancel()
		panic("racing admin list exploded")
	}
	return c.inner.RequestWithContext(ctx, token, method, params, opts)
}

func (c *cancellingPanickingAdminListClient) GetAPIURL(opts *gotgbot.RequestOpts) string {
	return c.inner.GetAPIURL(opts)
}

func (c *cancellingPanickingAdminListClient) FileURL(token, path string, opts *gotgbot.RequestOpts) string {
	return c.inner.FileURL(token, path, opts)
}

// A caller whose context ends while the load panics either receives the panic
// (and reports it) or leaves; either way the panic is reported exactly once.
// One P keeps the loader running past the cancel, so the caller often finds
// both its context and the result ready.
func TestLoadAdminCachePanicIsReportedOnceWhenTheCallerLeavesAsItFails(t *testing.T) {
	withMemoryMarshaler(t)
	hook := captureErrorLogs(t)
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(1))
	inner := &adminCacheBotClient{responses: map[string]json.RawMessage{
		"getChatMember:999": json.RawMessage(`{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"}}`),
	}}

	for i := range 40 {
		hook.Reset()
		ctx, cancel := context.WithCancel(context.Background())
		bot := newAdminCacheBot(inner)
		bot.BotClient = &cancellingPanickingAdminListClient{inner: inner, cancel: cancel}
		chatID := int64(-100900 - i)

		_, err := LoadAdminCacheWithContext(ctx, bot, chatID)
		received := 0
		var loadPanic *AdminLoadPanicError
		if errors.As(err, &loadPanic) {
			received = 1
		}
		deadline := time.Now().Add(time.Second)
		for received+len(panicLogs(hook, "racing admin list exploded")) == 0 && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		time.Sleep(5 * time.Millisecond)
		if reports := received + len(panicLogs(hook, "racing admin list exploded")); reports != 1 {
			t.Fatalf("run %d: panic reported %d times (received by caller: %d), want once", i, reports, received)
		}
	}
}

// pendingAdminLoads counts the admin loads the shared table still tracks.
func pendingAdminLoads() int {
	adminLoadsMu.Lock()
	defer adminLoadsMu.Unlock()
	return len(adminLoads)
}

// Finished loads leave nothing behind, whether they succeed, panic or lose
// their only caller, so the table does not grow with every chat ever loaded.
func TestAdminLoadsAreForgottenOnceFinished(t *testing.T) {
	withMemoryMarshaler(t)
	ok := &adminCacheBotClient{responses: map[string]json.RawMessage{
		"getChatMember:999":     json.RawMessage(`{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"}}`),
		"getChatAdministrators": json.RawMessage(`[{"status":"administrator","user":{"id":42,"is_bot":false,"first_name":"Admin"}}]`),
	}}
	LoadAdminCache(newAdminCacheBot(ok), -100950)

	panicking := newAdminCacheBot(ok)
	panicking.BotClient = &panickingAdminListClient{inner: ok}
	func() {
		defer func() { _ = recover() }()
		LoadAdminCache(panicking, -100951)
	}()

	stalled := &hungBotStatusClient{inner: ok, started: make(chan struct{}), release: make(chan struct{}), stall: 2 * time.Second}
	stalledBot := newAdminCacheBot(ok)
	stalledBot.BotClient = stalled
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := LoadAdminCacheWithContext(ctx, stalledBot, -100952); err == nil {
		t.Fatal("stalled load returned before its caller's deadline")
	}
	close(stalled.release)

	deadline := time.Now().Add(2 * time.Second)
	for pendingAdminLoads() != 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if n := pendingAdminLoads(); n != 0 {
		t.Fatalf("admin load table holds %d finished loads, want 0", n)
	}
}

// adminListCountingClient counts getChatAdministrators calls on their way to
// the wrapped client.
type adminListCountingClient struct {
	gotgbot.BotClient
	adminListCalls atomic.Int32
}

func (c *adminListCountingClient) RequestWithContext(ctx context.Context, token, method string, params map[string]any, opts *gotgbot.RequestOpts) (json.RawMessage, error) {
	if method == "getChatAdministrators" {
		c.adminListCalls.Add(1)
	}
	return c.BotClient.RequestWithContext(ctx, token, method, params, opts)
}

func runningAdminLoad(key string) *adminLoad {
	adminLoadsMu.Lock()
	defer adminLoadsMu.Unlock()
	return adminLoads[key]
}

func adminLoadWaiters(load *adminLoad) int {
	adminLoadsMu.Lock()
	defer adminLoadsMu.Unlock()
	return load.waiters
}

func waitForAdminLoadTest(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

// ClearAdminCache during a load starts the next lookup on a fresh load. The
// cleared load, when it finishes, must neither store its stale list nor drop
// the fresh load from the table, so lookups arriving after it still join the
// fresh load instead of starting another one.
func TestClearAdminCacheMidLoadHandsLaterLookupsToAFreshLoad(t *testing.T) {
	withMemoryMarshaler(t)
	const chatID = int64(-100977)
	key := adminCacheKey(chatID)
	newStalledBot := func(adminID string) (*gotgbot.Bot, *hungBotStatusClient, *adminListCountingClient) {
		inner := &adminCacheBotClient{responses: map[string]json.RawMessage{
			"getChatMember:999":     json.RawMessage(`{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"}}`),
			"getChatAdministrators": json.RawMessage(`[{"status":"administrator","user":{"id":` + adminID + `,"is_bot":false,"first_name":"Admin"}}]`),
		}}
		hung := &hungBotStatusClient{inner: inner, started: make(chan struct{}), release: make(chan struct{}), stall: 10 * time.Second}
		counting := &adminListCountingClient{BotClient: hung}
		bot := newAdminCacheBot(inner)
		bot.BotClient = counting
		return bot, hung, counting
	}
	staleBot, staleHung, staleCalls := newStalledBot("41")
	freshBot, freshHung, freshCalls := newStalledBot("42")
	releaseStale := sync.OnceFunc(func() { close(staleHung.release) })
	releaseFresh := sync.OnceFunc(func() { close(freshHung.release) })
	t.Cleanup(releaseStale)
	t.Cleanup(releaseFresh)

	// The first caller starts the stale load and leaves on its deadline.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := LoadAdminCacheWithContext(ctx, staleBot, chatID); err == nil {
		t.Fatal("first lookup returned before its deadline")
	}
	<-staleHung.started
	stale := runningAdminLoad(key)
	if stale == nil {
		t.Fatal("stale load is not tracked")
	}

	if err := ClearAdminCache(chatID); err != nil {
		t.Fatalf("ClearAdminCache: %v", err)
	}
	if runningAdminLoad(key) != nil {
		t.Fatal("ClearAdminCache left the stale load tracked")
	}

	var wg sync.WaitGroup
	results := make([]AdminCache, 3)
	errs := make([]error, 3)
	lookup := func(i int, bot *gotgbot.Bot) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i], errs[i] = LoadAdminCacheWithContext(context.Background(), bot, chatID)
		}()
	}

	// One lookup starts the fresh load and another joins it.
	lookup(0, freshBot)
	<-freshHung.started
	fresh := runningAdminLoad(key)
	if fresh == nil || fresh == stale {
		t.Fatalf("lookup after ClearAdminCache joined load %p, want a fresh one (stale %p)", fresh, stale)
	}
	lookup(1, staleBot)
	waitForAdminLoadTest(t, "second lookup to join the fresh load", func() bool { return adminLoadWaiters(fresh) == 2 })

	// The stale load finishes while the fresh one still runs.
	releaseStale()
	waitForAdminLoadTest(t, "stale load to finish", func() bool {
		select {
		case <-stale.done:
			return true
		default:
			return false
		}
	})
	if found, stored := GetAdminCacheList(chatID); found {
		t.Fatalf("stale load stored its list after ClearAdminCache: %+v", stored)
	}

	// A lookup arriving now must still join the fresh load.
	lookup(2, staleBot)
	waitForAdminLoadTest(t, "third lookup to join the fresh load", func() bool { return adminLoadWaiters(fresh) == 3 })

	releaseFresh()
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("lookup %d error = %v", i, err)
		}
		if len(results[i].UserInfo) != 1 || results[i].UserInfo[0].User.Id != 42 {
			t.Fatalf("lookup %d got %+v, want admin 42 from the fresh load", i, results[i].UserInfo)
		}
	}
	if n := freshCalls.adminListCalls.Load(); n != 1 {
		t.Fatalf("fresh bot getChatAdministrators calls = %d, want 1", n)
	}
	if n := staleCalls.adminListCalls.Load(); n != 1 {
		t.Fatalf("stale bot getChatAdministrators calls = %d, want 1 (later lookups joined the fresh load)", n)
	}
	if found, stored := GetAdminCacheList(chatID); !found || len(stored.UserInfo) != 1 || stored.UserInfo[0].User.Id != 42 {
		t.Fatalf("stored list = %v %+v, want admin 42", found, stored)
	}
}

// goexitOnceAdminListClient ends the loading goroutine with runtime.Goexit on
// its first getChatAdministrators call and answers normally afterwards.
type goexitOnceAdminListClient struct {
	inner  *adminCacheBotClient
	exited atomic.Bool
}

func (c *goexitOnceAdminListClient) RequestWithContext(ctx context.Context, token, method string, params map[string]any, opts *gotgbot.RequestOpts) (json.RawMessage, error) {
	if method == "getChatAdministrators" && c.exited.CompareAndSwap(false, true) {
		runtime.Goexit()
	}
	return c.inner.RequestWithContext(ctx, token, method, params, opts)
}

func (c *goexitOnceAdminListClient) GetAPIURL(opts *gotgbot.RequestOpts) string {
	return c.inner.GetAPIURL(opts)
}

func (c *goexitOnceAdminListClient) FileURL(token, path string, opts *gotgbot.RequestOpts) string {
	return c.inner.FileURL(token, path, opts)
}

// A load that ends in runtime.Goexit neither returns nor panics. Its callers
// must still be woken with a failure rather than an empty list, and the chat's
// next lookup must start a new load instead of waiting on the dead one.
func TestLoadAdminCacheSurvivesALoadEndedByGoexit(t *testing.T) {
	withMemoryMarshaler(t)
	const chatID = int64(-100978)
	inner := &adminCacheBotClient{responses: map[string]json.RawMessage{
		"getChatMember:999":     json.RawMessage(`{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"}}`),
		"getChatAdministrators": json.RawMessage(`[{"status":"administrator","user":{"id":42,"is_bot":false,"first_name":"Admin"}}]`),
	}}
	bot := newAdminCacheBot(inner)
	bot.BotClient = &goexitOnceAdminListClient{inner: inner}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := LoadAdminCacheWithContext(ctx, bot, chatID)
	var loadPanic *AdminLoadPanicError
	if !errors.As(err, &loadPanic) {
		t.Fatalf("lookup whose load called Goexit: error = %v, want *AdminLoadPanicError", err)
	}

	ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	got, err := LoadAdminCacheWithContext(ctx, bot, chatID)
	if err != nil {
		t.Fatalf("next lookup error = %v, want a fresh load", err)
	}
	if len(got.UserInfo) != 1 || got.UserInfo[0].User.Id != 42 {
		t.Fatalf("next lookup got %+v, want admin 42", got.UserInfo)
	}
}

type adminRequestContextClient struct {
	gotgbot.BotClient
	contexts chan context.Context
}

func (c *adminRequestContextClient) RequestWithContext(ctx context.Context, token, method string, params map[string]any, opts *gotgbot.RequestOpts) (json.RawMessage, error) {
	c.contexts <- ctx
	return c.BotClient.RequestWithContext(ctx, token, method, params, opts)
}

func TestLoadAdminCacheKeepsTraceAfterInitiatingWaiterCancels(t *testing.T) {
	withMemoryMarshaler(t)
	const chatID = int64(-100979)
	parent := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: trace.TraceID{1}, SpanID: trace.SpanID{2}, TraceFlags: trace.FlagsSampled,
	})
	hung := &hungBotStatusClient{
		inner: &adminCacheBotClient{responses: map[string]json.RawMessage{
			"getChatMember:999":     json.RawMessage(`{"status":"administrator","user":{"id":999,"is_bot":true,"first_name":"Alita"}}`),
			"getChatAdministrators": json.RawMessage(`[{"status":"administrator","user":{"id":42,"is_bot":false,"first_name":"Admin"}}]`),
		}},
		started: make(chan struct{}),
		release: make(chan struct{}),
		stall:   10 * time.Second,
	}
	client := &adminRequestContextClient{BotClient: hung, contexts: make(chan context.Context, 2)}
	bot := newAdminCacheBot(hung.inner)
	bot.BotClient = client
	release := sync.OnceFunc(func() { close(hung.release) })
	t.Cleanup(release)

	ctx, cancel := context.WithCancel(trace.ContextWithSpanContext(context.Background(), parent))
	defer cancel()
	first := make(chan error, 1)
	go func() {
		_, err := LoadAdminCacheWithContext(ctx, bot, chatID)
		first <- err
	}()
	select {
	case <-hung.started:
	case <-time.After(3 * time.Second):
		t.Fatal("bot-status request did not start")
	}
	requestCtx := <-client.contexts
	cancel()
	select {
	case err := <-first:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("initiating waiter error = %v, want context.Canceled", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("initiating waiter did not stop after cancellation")
	}
	if requestCtx.Err() != nil {
		t.Fatalf("shared bot-status request canceled with its waiter: %v", requestCtx.Err())
	}
	if _, bounded := requestCtx.Deadline(); !bounded {
		t.Fatal("detached bot-status request lost its own timeout")
	}
	if got := trace.SpanContextFromContext(requestCtx); !got.Equal(parent) {
		t.Fatalf("bot-status trace parent = %v, want %v", got, parent)
	}

	load := runningAdminLoad(adminCacheKey(chatID))
	if load == nil {
		t.Fatal("cancellation forgot the shared load")
	}
	second := make(chan AdminCache, 1)
	go func() { second <- LoadAdminCache(bot, chatID) }()
	waitForAdminLoadTest(t, "second waiter to join detached load", func() bool { return adminLoadWaiters(load) == 1 })
	release()
	select {
	case result := <-second:
		if len(result.UserInfo) != 1 || result.UserInfo[0].User.Id != 42 {
			t.Fatalf("shared result = %+v, want admin 42", result)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("shared load did not finish for its remaining waiter")
	}
	if got := trace.SpanContextFromContext(<-client.contexts); !got.Equal(parent) {
		t.Fatalf("admin-list trace parent = %v, want initiating update %v", got, parent)
	}
	if found, stored := GetAdminCacheList(chatID); !found || len(stored.UserInfo) != 1 {
		t.Fatalf("completed shared load was not cached: %v %+v", found, stored)
	}
}

func TestLoadAdminCacheMissingBotIsQuietButUnexpectedErrorsWarn(t *testing.T) {
	for _, tc := range []struct {
		name        string
		code        int
		description string
		warn        bool
	}{
		{"missing", 400, "Bad Request: user not found", false},
		{"forbidden", 403, "Forbidden: bot was kicked from the supergroup chat", true},
		{"unknown bad request", 400, "Bad Request: unexpected", true},
		{"server failure", 500, "Internal Server Error", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withMemoryMarshaler(t)
			hook := captureErrorLogs(t)
			client := &adminCacheBotClient{errors: map[string]error{
				"getChatMember": &gotgbot.TelegramError{Method: "getChatMember", Code: tc.code, Description: tc.description},
			}}
			result := LoadAdminCache(newAdminCacheBot(client), -100980)
			if len(result.UserInfo) != 0 {
				t.Fatalf("failed bot membership returned admins: %+v", result)
			}
			warned := false
			for _, entry := range hook.AllEntries() {
				warned = warned || entry.Level <= log.WarnLevel
			}
			if warned != tc.warn {
				t.Errorf("warning/error logged = %v, want %v", warned, tc.warn)
			}
		})
	}
}

func TestLoadAdminCacheRejectsNonPositiveBotIDsWithoutRequests(t *testing.T) {
	for _, botID := range []int64{0, -1} {
		inner := &adminCacheBotClient{}
		client := &adminRequestContextClient{BotClient: inner, contexts: make(chan context.Context, 1)}
		bot := newAdminCacheBot(inner)
		bot.Id = botID
		bot.BotClient = client
		if result := LoadAdminCache(bot, -100981); result.Cached || len(result.UserInfo) != 0 {
			t.Errorf("bot ID %d returned a cached result: %+v", botID, result)
		}
		select {
		case <-client.contexts:
			t.Errorf("bot ID %d caused a membership request", botID)
		default:
		}
	}
}
