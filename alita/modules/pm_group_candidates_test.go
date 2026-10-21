//go:build testtools

package modules

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/alicebob/miniredis/v2"
	gocache "github.com/eko/gocache/lib/v4/cache"
	"github.com/eko/gocache/lib/v4/marshaler"
	gocache_store "github.com/eko/gocache/store/redis/v4"
	"github.com/redis/go-redis/v9"

	"github.com/divkix/Alita_Robot/alita/db/logchannels"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/rules"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

// candidatesRedis points the cache at a miniredis server, as in production,
// and turns the picker pause on, so tests move the server's clock past the
// pause instead of sleeping.
func candidatesRedis(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	previousMarshal, previousManager, previousClient := cache.GetCacheState()
	manager := gocache.New[any](gocache_store.NewRedis(client))
	cache.SetCacheState(marshaler.New(manager), manager, client)
	t.Cleanup(func() { cache.SetCacheState(previousMarshal, previousManager, previousClient) })
	originalCooldown := settingsPickerCooldown
	settingsPickerCooldown = 5 * time.Second
	t.Cleanup(func() { settingsPickerCooldown = originalCooldown })
	return server
}

// runOwnInPM runs a PM command with arguments of its own and returns the
// arguments its handler saw, or nil when it did not run.
func runOwnInPM(t *testing.T, f adminChatFixture, text string) []string {
	t.Helper()
	ctx := newModuleMessageContext(f.bot, privateChatOf(f.admin), f.admin, text)
	var args []string
	handler := withPMGroupArg("cmd", pmArgsOwn, func(_ *gotgbot.Bot, ctx *ext.Context) error {
		args = ctx.Args()
		return ext.EndGroups
	})
	if err := handler(f.bot, ctx); err != ext.EndGroups {
		t.Fatalf("handler(%q) error = %v, want ext.EndGroups", text, err)
	}
	return args
}

// With a connection, the groups looked up for a title are kept for the
// picker's pause: a second command inside it does not scan them again, one
// after it does, and neither takes the picker's pause.
func TestPMGroupOwnArgCandidatesKeptForThePause(t *testing.T) {
	f := titleFixture(t, "My Group")
	scans := setPMGroupCandidates(t, models.Chat{ChatId: f.chat.Id, ChatName: "My Group"})
	server := candidatesRedis(t)
	f.connectAdmin(t)

	runOwnInPM(t, f, "/cmd hello on")
	runOwnInPM(t, f, "/cmd world on")
	if n := scans.Load(); n != 1 {
		t.Fatalf("candidate scans inside the pause = %d, want 1", n)
	}

	server.FastForward(settingsPickerCooldown)
	runOwnInPM(t, f, "/cmd hello on")
	if n := scans.Load(); n != 2 {
		t.Fatalf("candidate scans after the pause = %d, want 2", n)
	}
	if _, wait := settingsPickerWait(i18n.MustNewTranslator("en"), f.admin.Id); wait {
		t.Fatal("the commands took the picker's pause")
	}
}

// The kept groups still serve the title check: a title among them is refused
// inside the pause without a second scan.
func TestPMGroupOwnArgKeptCandidatesStillRefuseTitle(t *testing.T) {
	f := titleFixture(t, "My Group")
	other := connectToOtherGroup(t, f)
	scans := setPMGroupCandidates(t, models.Chat{ChatId: f.chat.Id, ChatName: "My Group"})
	candidatesRedis(t)

	if args := runOwnInPM(t, f, "/cmd hello on"); fmt.Sprint(args) != "[/cmd hello on]" {
		t.Fatalf("handler saw args %q, want [/cmd hello on]", args)
	}
	if args := runOwnInPM(t, f, `/cmd "my group" on`); args != nil {
		t.Fatalf("handler ran with args %q, want the title refused", args)
	}
	requireTitleRefused(t, f, "my group", other)
	if n := scans.Load(); n != 1 {
		t.Fatalf("candidate scans = %d, want 1", n)
	}
}

// A failed scan is not kept: the next command scans again and refuses a
// title the failed scan could not see.
func TestPMGroupOwnArgCandidateErrorNotKept(t *testing.T) {
	f := titleFixture(t, "My Group")
	other := connectToOtherGroup(t, f)
	candidatesRedis(t)
	var scans atomic.Int32
	original := settingsCandidateChats
	settingsCandidateChats = func(context.Context, int64, int) ([]models.Chat, error) {
		if scans.Add(1) == 1 {
			return nil, errors.New("db down")
		}
		return []models.Chat{{ChatId: f.chat.Id, ChatName: "My Group"}}, nil
	}
	t.Cleanup(func() { settingsCandidateChats = original })

	if args := runOwnInPM(t, f, `/cmd "My Group" on`); fmt.Sprint(args) != `[/cmd "My Group" on]` {
		t.Fatalf("handler saw args %q after a failed scan, want the command run as before", args)
	}
	resetCalls(f.client)
	if args := runOwnInPM(t, f, `/cmd "My Group" on`); args != nil {
		t.Fatalf("handler ran with args %q, want the title refused after a new scan", args)
	}
	requireTitleRefused(t, f, "My Group", other)
	if n := scans.Load(); n != 2 {
		t.Fatalf("candidate scans = %d, want 2", n)
	}
}

// One user's kept groups are never served to another user inside the pause.
func TestPMGroupCandidatesKeptPerUser(t *testing.T) {
	candidatesRedis(t)
	alice, bob := uniquePositiveUserID(), uniquePositiveUserID()
	original := settingsCandidateChats
	settingsCandidateChats = func(_ context.Context, userID int64, _ int) ([]models.Chat, error) {
		return []models.Chat{{ChatId: -userID, ChatName: fmt.Sprintf("G%d", userID)}}, nil
	}
	t.Cleanup(func() { settingsCandidateChats = original })

	for _, user := range []int64{alice, bob, alice, bob} {
		got, err := pmGroupCandidates(context.Background(), user)
		if want := fmt.Sprintf("[{%d G%d}]", -user, user); err != nil || fmt.Sprint(got) != want {
			t.Fatalf("pmGroupCandidates(%d) = %v, %v; want %s", user, got, err, want)
		}
	}
}

// Concurrent commands of one user inside the pause share one scan.
func TestPMGroupCandidatesConcurrentCallsScanOnce(t *testing.T) {
	candidatesRedis(t)
	userID := uniquePositiveUserID()
	var scans atomic.Int32
	release := make(chan struct{})
	original := settingsCandidateChats
	settingsCandidateChats = func(context.Context, int64, int) ([]models.Chat, error) {
		scans.Add(1)
		<-release
		return []models.Chat{{ChatId: -1001, ChatName: "Alpha"}}, nil
	}
	t.Cleanup(func() { settingsCandidateChats = original })

	const callers = 16
	var wg sync.WaitGroup
	results := make([][]settingsPickerEntry, callers)
	errs := make([]error, callers)
	for i := range callers {
		wg.Go(func() { results[i], errs[i] = pmGroupCandidates(context.Background(), userID) })
	}
	for scans.Load() == 0 {
		time.Sleep(time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond) // let every caller join the running scan
	close(release)
	wg.Wait()

	if n := scans.Load(); n != 1 {
		t.Fatalf("candidate scans for %d concurrent calls = %d, want 1", callers, n)
	}
	for i := range callers {
		if errs[i] != nil || fmt.Sprint(results[i]) != "[{-1001 Alpha}]" {
			t.Fatalf("call %d = %v, %v; want [{-1001 Alpha}]", i, results[i], errs[i])
		}
	}
}

// On a connection, a usual argument of a settings command costs one scan of
// the user's groups and no Telegram lookups for them; inside the pause the
// title of one of them is still refused without another scan.
func TestSetRulesInPMOnConnectionUsualArgumentCostsOneScan(t *testing.T) {
	f := titleFixture(t, "My Group")
	other := connectToOtherGroup(t, f)
	if err := rules.SetChatRules(other, "B rules"); err != nil {
		t.Fatalf("SetChatRules() error = %v", err)
	}
	alpha, beta := uniqueModuleChatID(), uniqueModuleChatID()
	f.addGroup(t, alpha, "Alpha", true)
	f.addGroup(t, beta, "Beta", true)
	scans := setPMGroupCandidates(t, models.Chat{ChatId: alpha, ChatName: "Alpha"}, models.Chat{ChatId: beta, ChatName: "Beta"})
	candidatesRedis(t)

	processPM(t, pmSettingsDispatcher(), f, f.admin.Id, `/setrules "hello world" text`)

	if got := rules.GetChatRulesInfo(other).Rules; got != `&#34;hello world&#34; text` && got != `"hello world" text` {
		t.Fatalf("connected rules = %q, want the quoted argument kept; sent:\n%s", got, sentText(f.client))
	}
	if n := scans.Load(); n != 1 {
		t.Fatalf("candidate scans = %d, want 1", n)
	}
	for _, method := range []string{"getChatAdministrators", "getChatMember"} {
		for _, call := range f.client.callsFor(method) {
			if id := fmt.Sprint(call.Params["chat_id"]); id == fmt.Sprint(alpha) || id == fmt.Sprint(beta) {
				t.Fatalf("%s for group %s, whose title does not match", method, id)
			}
		}
	}

	resetCalls(f.client)
	before := rules.GetChatRulesInfo(other).Rules
	processPM(t, pmSettingsDispatcher(), f, f.admin.Id, "/setrules Alpha text")

	want := fmt.Sprintf(translated(t, "connections_pm_group_title_on_connection"), "Alpha", other)
	if sent := sentText(f.client); !strings.Contains(sent, want) || !strings.Contains(sent, fmt.Sprint(alpha)) {
		t.Fatalf("title Alpha not refused with group %d listed; sent:\n%s", alpha, sent)
	}
	if got := rules.GetChatRulesInfo(other).Rules; got != before {
		t.Fatalf("connected rules = %q after the refused title, want %q", got, before)
	}
	if n := scans.Load(); n != 1 {
		t.Fatalf("candidate scans inside the pause = %d, want 1", n)
	}
}

// With a connection, /log with the title of one of the user's groups as its
// first argument enables nothing in either group.
func TestLogInPMOnConnectionRefusesGroupTitle(t *testing.T) {
	f := newLogChannelPMFixture(t)
	f.bindChannel(t)
	other := connectToOtherGroup(t, f.adminChatFixture)
	if err := logchannels.Set(other, "Group B", f.channel.Id); err != nil {
		t.Fatalf("logchannels.Set() error = %v", err)
	}
	for _, id := range []int64{f.chat.Id, other} {
		if err := logchannels.SetCategory(id, "admin", false); err != nil {
			t.Fatalf("SetCategory(%d) error = %v", id, err)
		}
	}

	f.sendPM(t, f.admin, fmt.Sprintf("/log %q admin", f.chat.Title))

	for _, id := range []int64{f.chat.Id, other} {
		if got := logchannels.Get(id); got == nil || got.CatAdmin {
			t.Fatalf("group %d after /log by title: %+v, want admin category still off", id, got)
		}
	}
	requireTitleRefused(t, f.adminChatFixture, f.chat.Title, other)
}
