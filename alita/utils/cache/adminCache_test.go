//go:build testtools

package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	log "github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
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
	if admin, ok := got.UserMap[777000]; !ok || admin.User.FirstName != "Telegram" {
		t.Fatalf("UserMap[777000] = (%+v, %v), want Telegram admin", admin, ok)
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
	time.Sleep(50 * time.Millisecond)
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
		if _, ok := ac.UserMap[42]; !ok {
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
