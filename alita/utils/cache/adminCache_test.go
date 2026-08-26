//go:build testtools

package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
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
	responses map[string]json.RawMessage
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
