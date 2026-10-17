package modules

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/federations"
)

// gatedBotClient holds the first call to one Bot API method until the test
// releases it, so a test can start a second federation walk while the first one
// is still in flight.
type gatedBotClient struct {
	*moduleBotClient
	method  string
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func newGatedBotClient(method string) *gatedBotClient {
	return &gatedBotClient{
		moduleBotClient: newModuleBotClient(),
		method:          method,
		entered:         make(chan struct{}),
		release:         make(chan struct{}),
	}
}

func (c *gatedBotClient) RequestWithContext(ctx context.Context, token, method string, params map[string]any, opts *gotgbot.RequestOpts) (json.RawMessage, error) {
	if method == c.method {
		first := false
		c.once.Do(func() { first = true })
		if first {
			close(c.entered)
			<-c.release
		}
	}
	return c.moduleBotClient.RequestWithContext(ctx, token, method, params, opts)
}

// lastBanActions returns, per chat, the last ban or unban the bot sent for the user.
func lastBanActions(client *moduleBotClient, userID int64) map[int64]string {
	client.mu.Lock()
	defer client.mu.Unlock()
	last := make(map[int64]string)
	for _, call := range client.calls {
		if call.Method != "banChatMember" && call.Method != "unbanChatMember" {
			continue
		}
		if uid, _ := call.Params["user_id"].(int64); uid != userID {
			continue
		}
		if cid, ok := call.Params["chat_id"].(int64); ok {
			last[cid] = call.Method
		}
	}
	return last
}

func TestFedBanWalksEndInTheLatestFederationState(t *testing.T) {
	cases := []struct {
		name       string
		gate       string
		preBanned  bool
		first      string
		second     string
		wantMethod string
	}{
		{name: "fban then unfban", gate: "banChatMember", first: "/fban", second: "/unfban", wantMethod: "unbanChatMember"},
		{name: "unfban then fban", gate: "unbanChatMember", preBanned: true, first: "/unfban", second: "/fban", wantMethod: "banChatMember"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := newGatedBotClient(tc.gate)
			bot := newModuleTestBot(client.moduleBotClient)
			bot.BotClient = client
			owner := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Owner"}
			fed, err := federations.CreateFederation(owner.Id, "Walk Order Fed")
			require.NoError(t, err)
			t.Cleanup(func() { _ = federations.DeleteFederation(fed.FedID) })

			target := uniquePositiveUserID()
			chatA := seedFedChat(t, fed.FedID, target)
			chatB := seedFedChat(t, fed.FedID, target)
			if tc.preBanned {
				_, _, err = federations.Fban(fed.FedID, target, owner.Id, "spam")
				require.NoError(t, err)
			}
			run := func(command string) {
				handler := federationsModule.fban
				if command == "/unfban" {
					handler = federationsModule.unfban
				}
				ctx := newModuleMessageContext(bot, chatA, owner, command+" "+strconv.FormatInt(target, 10)+" spam")
				require.Equal(t, ext.EndGroups, handler(bot, ctx))
			}

			run(tc.first)
			select {
			case <-client.entered:
			case <-time.After(2 * time.Second):
				t.Fatal("the first walk never reached Telegram")
			}
			run(tc.second)
			// Give an unserialised second walk the time to finish before the first resumes.
			time.Sleep(200 * time.Millisecond)
			close(client.release)

			require.Eventually(t, func() bool {
				return len(unbanTargets(client.moduleBotClient, target)) == 2 &&
					len(client.callsFor("banChatMember")) == 2
			}, 2*time.Second, 10*time.Millisecond, "both walks must reach both chats")
			want := map[int64]string{chatA.Id: tc.wantMethod, chatB.Id: tc.wantMethod}
			require.Equal(t, want, lastBanActions(client.moduleBotClient, target),
				"every chat must end in the state of the last command")
		})
	}
}

func TestFbanWalkBansNobodyOnceTheBanRowIsGone(t *testing.T) {
	cases := []struct {
		name   string
		queued bool
	}{
		{name: "no ban row"},
		// sync.Mutex is not FIFO, so a fban walk can wait behind another walk (one
		// sleeping on a 429, say) until after the unfban deleted the row.
		{name: "queued until after the unfban", queued: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			owner := uniquePositiveUserID()
			fed, err := federations.CreateFederation(owner, "Gone Ban Fed")
			require.NoError(t, err)
			t.Cleanup(func() { _ = federations.DeleteFederation(fed.FedID) })

			target := uniquePositiveUserID()
			seedFedChat(t, fed.FedID, target)
			if tc.queued {
				_, _, err = federations.Fban(fed.FedID, target, owner, "spam")
				require.NoError(t, err)
				unlock := lockFedWalk(fed.FedID, target)
				done := make(chan struct{})
				go func() { defer close(done); applyActiveFban(bot, fed.FedID, target) }()
				require.NoError(t, federations.Unfban(fed.FedID, target))
				unlock()
				select {
				case <-done:
				case <-time.After(2 * time.Second):
					t.Fatal("the queued fban walk never finished")
				}
			} else {
				applyActiveFban(bot, fed.FedID, target)
			}
			require.Empty(t, lastBanActions(client, target),
				"a fban walk that finds no ban row must not ban the user anywhere")
		})
	}
}

// failFedBanLookups makes every federation_bans query for fedID fail until the
// test ends. Register and Remove write gorm's callback list without a lock while
// queries iterate it, so tests that use this must not run with t.Parallel() and
// must let every walk finish first; the unique name does not make it safe.
func failFedBanLookups(t *testing.T, fedID string) {
	t.Helper()
	name := "test:fail_fed_ban_lookup:" + fedID
	injected := errors.New("injected federation_bans failure")
	require.NoError(t, db.DB.Callback().Query().Before("gorm:query").Register(name, func(tx *gorm.DB) {
		if tx.Statement.Table == "federation_bans" && strings.Contains(fmt.Sprintf("%v", tx.Statement.Clauses["WHERE"]), fedID) {
			_ = tx.AddError(injected)
		}
	}))
	t.Cleanup(func() { _ = db.DB.Callback().Query().Remove(name) })
}

func TestUnfbanKeepsBansWhenTheFedTreeLookupFails(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	ownerA := uniquePositiveUserID()
	ownerB := uniquePositiveUserID()
	fedA, err := federations.CreateFederation(ownerA, "Lookup Subscriber Fed")
	require.NoError(t, err)
	t.Cleanup(func() { _ = federations.DeleteFederation(fedA.FedID) })
	fedB, err := federations.CreateFederation(ownerB, "Lookup Source Fed")
	require.NoError(t, err)
	t.Cleanup(func() { _ = federations.DeleteFederation(fedB.FedID) })
	require.NoError(t, federations.SubscribeFed(fedA.FedID, fedB.FedID))

	target := uniquePositiveUserID()
	seedFedChat(t, fedA.FedID, target)
	_, _, err = federations.Fban(fedB.FedID, target, ownerB, "upstream")
	require.NoError(t, err)
	failFedBanLookups(t, fedB.FedID)

	applyActiveUnfban(bot, fedA.FedID, target)
	require.Empty(t, unbanTargets(client, target),
		"a failed lookup of a subscribed federation's ban must not lift the ban it may still hold")
}

func TestFbanWalkStillBansWhenTheBanLookupFails(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	owner := uniquePositiveUserID()
	fed, err := federations.CreateFederation(owner, "Lookup Fban Fed")
	require.NoError(t, err)
	t.Cleanup(func() { _ = federations.DeleteFederation(fed.FedID) })

	target := uniquePositiveUserID()
	chat := seedFedChat(t, fed.FedID, target)
	_, _, err = federations.Fban(fed.FedID, target, owner, "spam")
	require.NoError(t, err)
	failFedBanLookups(t, fed.FedID)

	applyActiveFban(bot, fed.FedID, target)
	var chatIDs []int64
	for _, call := range client.callsFor("banChatMember") {
		if uid, _ := call.Params["user_id"].(int64); uid == target {
			chatIDs = append(chatIDs, call.Params["chat_id"].(int64))
		}
	}
	require.Equal(t, []int64{chat.Id}, chatIDs,
		"a failed re-read of the ban must not skip the fban walk; only a confirmed missing ban may")
	require.Empty(t, unbanTargets(client, target),
		"while the ban cannot be read at all, the walk must keep the bans it placed")
}

// failFirstFedBanLookups makes the first n federation_bans queries for fedID
// fail and returns how many failed so far. Not safe with t.Parallel(), for the
// same reason as failFedBanLookups.
func failFirstFedBanLookups(t *testing.T, fedID string, n int32) *atomic.Int32 {
	t.Helper()
	name := "test:fail_first_fed_ban_lookups:" + fedID
	injected := errors.New("injected federation_bans failure")
	var failed atomic.Int32
	require.NoError(t, db.DB.Callback().Query().Before("gorm:query").Register(name, func(tx *gorm.DB) {
		if tx.Statement.Table != "federation_bans" || !strings.Contains(fmt.Sprintf("%v", tx.Statement.Clauses["WHERE"]), fedID) {
			return
		}
		if failed.Add(1) <= n {
			_ = tx.AddError(injected)
		}
	}))
	t.Cleanup(func() { _ = db.DB.Callback().Query().Remove(name) })
	return &failed
}

func TestFbanWalkLiftsItsBansWhenTheReReadFindsNoBan(t *testing.T) {
	cases := []struct {
		name        string
		banned      bool
		upstreamBan bool
		wantUnban   bool
	}{
		{name: "ban row gone", wantUnban: true},
		{name: "ban row still there", banned: true},
		{name: "subscribed federation still bans", upstreamBan: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			owner := uniquePositiveUserID()
			fed, err := federations.CreateFederation(owner, "Reread Fban Fed")
			require.NoError(t, err)
			t.Cleanup(func() { _ = federations.DeleteFederation(fed.FedID) })

			target := uniquePositiveUserID()
			chatA := seedFedChat(t, fed.FedID, target)
			chatB := seedFedChat(t, fed.FedID, target)
			if tc.banned {
				_, _, err = federations.Fban(fed.FedID, target, owner, "spam")
				require.NoError(t, err)
			}
			if tc.upstreamBan {
				upstreamOwner := uniquePositiveUserID()
				upstream, err := federations.CreateFederation(upstreamOwner, "Reread Upstream Fed")
				require.NoError(t, err)
				t.Cleanup(func() { _ = federations.DeleteFederation(upstream.FedID) })
				require.NoError(t, federations.SubscribeFed(fed.FedID, upstream.FedID))
				_, _, err = federations.Fban(upstream.FedID, target, upstreamOwner, "upstream")
				require.NoError(t, err)
			}
			failed := failFirstFedBanLookups(t, fed.FedID, 1)

			applyActiveFban(bot, fed.FedID, target)
			require.Positive(t, failed.Load(), "the injected lookup failure must have fired")
			require.Len(t, client.callsFor("banChatMember"), 2, "a failed first read must still ban")
			if tc.wantUnban {
				require.ElementsMatch(t, []int64{chatA.Id, chatB.Id}, unbanTargets(client, target),
					"a re-read that confirms no ban must lift the bans the walk placed by mistake")
				for _, call := range client.callsFor("unbanChatMember") {
					require.Equal(t, true, call.Params["only_if_banned"], "lifting must not kick members who are not banned")
				}
			} else {
				require.Empty(t, unbanTargets(client, target), "a re-read that finds a ban anywhere in the federation tree must keep it")
			}
		})
	}
}

// floodOnceBotClient answers the first call to method with Telegram's 429 and
// the given retry_after, in seconds.
type floodOnceBotClient struct {
	*moduleBotClient
	method     string
	retryAfter int64
	once       sync.Once
}

func (c *floodOnceBotClient) RequestWithContext(ctx context.Context, token, method string, params map[string]any, opts *gotgbot.RequestOpts) (json.RawMessage, error) {
	if method == c.method {
		flooded := false
		c.once.Do(func() { flooded = true })
		if flooded {
			// Record the refused call so the test sees both attempts.
			_, _ = c.moduleBotClient.RequestWithContext(ctx, token, method, params, opts)
			return nil, &gotgbot.TelegramError{
				Method:         method,
				Params:         params,
				Code:           429,
				Description:    "Too Many Requests: retry after " + strconv.FormatInt(c.retryAfter, 10),
				ResponseParams: &gotgbot.ResponseParameters{RetryAfter: c.retryAfter},
			}
		}
	}
	return c.moduleBotClient.RequestWithContext(ctx, token, method, params, opts)
}

func TestFedBanWalksRetryFloodWaitsUpToTheCap(t *testing.T) {
	const overCap = 31 // one second past the 30 s fedWalkMaxFloodWait
	cases := []struct {
		name       string
		method     string
		walk       func(b *gotgbot.Bot, fedID string, userID int64)
		retryAfter int64
		attempts   int
	}{
		{name: "ban retried", method: "banChatMember", walk: applyActiveFban, attempts: 2},
		{name: "unban retried", method: "unbanChatMember", walk: applyActiveUnfban, attempts: 2},
		// Over the cap the walk gives up before time.Sleep, so these cases do not wait.
		{name: "ban over the cap", method: "banChatMember", walk: applyActiveFban, retryAfter: overCap, attempts: 1},
		{name: "unban over the cap", method: "unbanChatMember", walk: applyActiveUnfban, retryAfter: overCap, attempts: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := &floodOnceBotClient{moduleBotClient: newModuleBotClient(), method: tc.method, retryAfter: tc.retryAfter}
			bot := newModuleTestBot(client.moduleBotClient)
			bot.BotClient = client
			owner := uniquePositiveUserID()
			fed, err := federations.CreateFederation(owner, "Flood Fed")
			require.NoError(t, err)
			t.Cleanup(func() { _ = federations.DeleteFederation(fed.FedID) })

			target := uniquePositiveUserID()
			chat := seedFedChat(t, fed.FedID, target)
			if tc.method == "banChatMember" {
				_, _, err = federations.Fban(fed.FedID, target, owner, "spam")
				require.NoError(t, err)
			}

			tc.walk(bot, fed.FedID, target)
			var chatIDs []int64
			for _, call := range client.callsFor(tc.method) {
				if uid, _ := call.Params["user_id"].(int64); uid == target {
					chatIDs = append(chatIDs, call.Params["chat_id"].(int64))
				}
			}
			require.Equal(t, slices.Repeat([]int64{chat.Id}, tc.attempts), chatIDs,
				"a 429 is retried once after retry_after, and not at all when retry_after exceeds the cap")
		})
	}
}

func TestFedWalkLocksAreReleasedAfterTheLastWalk(t *testing.T) {
	fedID := "walk-lock-" + strconv.FormatInt(uniquePositiveUserID(), 10)
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() { lockFedWalk(fedID, 42)() })
	}
	wg.Wait()

	fedWalkLocks.Lock()
	defer fedWalkLocks.Unlock()
	require.NotContains(t, fedWalkLocks.byKey, fedWalkKey{fedID: fedID, userID: 42},
		"a (federation, user) lock must not outlive its last walk")
}
