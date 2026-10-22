//go:build testtools

package membersnapshot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/chatstats"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

var chatSeq atomic.Int64

// testNow is far past any day other packages' tests write, because a tick
// measures every chat active since yesterday and a shared PostgreSQL test
// database keeps their rows.
var testNow = time.Date(2099, 3, 10, 9, 0, 0, 0, time.UTC)

// newActiveChat returns a chat ID no other test uses, with activity on day,
// and removes its rows when the test ends.
func newActiveChat(t *testing.T, day string) int64 {
	t.Helper()
	chatID := -(time.Now().UnixNano() + chatSeq.Add(1))
	if err := chatstats.AddDaily([]models.ChatDailyStat{{ChatID: chatID, Day: day, Messages: 1}}); err != nil {
		t.Fatalf("AddDaily() error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatDailyStat{}).Error; err != nil {
			t.Errorf("cleanup daily stats: %v", err)
		}
		if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatMemberCount{}).Error; err != nil {
			t.Errorf("cleanup member counts: %v", err)
		}
	})
	return chatID
}

// countClient is a hand-written gotgbot.BotClient that answers
// getChatMemberCount with 100 + the number of earlier calls for that chat, and
// can fail chosen chats a set number of times with a set error. Tests set its
// fields before the snapshotter runs.
type countClient struct {
	mu       sync.Mutex
	calls    map[int64]int
	failures map[int64]int
	failErr  error
	// onCall runs inside each call, before it answers.
	onCall func()
}

func newCountClient() *countClient {
	return &countClient{calls: map[int64]int{}, failures: map[int64]int{}}
}

func (c *countClient) RequestWithContext(ctx context.Context, _ string, method string, params map[string]any, _ *gotgbot.RequestOpts) (json.RawMessage, error) {
	if method != "getChatMemberCount" {
		return nil, fmt.Errorf("unexpected method %s", method)
	}
	chatID, err := strconv.ParseInt(fmt.Sprint(params["chat_id"]), 10, 64)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls[chatID]++
	if c.onCall != nil {
		c.onCall()
	}
	if c.failures[chatID] > 0 {
		c.failures[chatID]--
		return nil, c.failErr
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return json.RawMessage(strconv.Itoa(100 + c.calls[chatID] - 1)), nil
}

func (c *countClient) GetAPIURL(*gotgbot.RequestOpts) string { return "https://api.telegram.org" }

func (c *countClient) FileURL(token, path string, _ *gotgbot.RequestOpts) string {
	return "https://api.telegram.org/file/bot" + token + "/" + path
}

func (c *countClient) callsFor(chatID int64) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls[chatID]
}

func newTestSnapshotter(client *countClient, now time.Time) *Snapshotter {
	s := New(&gotgbot.Bot{Token: "999:test", BotClient: client, User: gotgbot.User{Id: 999, IsBot: true}})
	s.pace = 0
	s.now = func() time.Time { return now }
	return s
}

func storedCount(t *testing.T, chatID int64, day string) (int64, bool) {
	t.Helper()
	rows, err := chatstats.GetMemberCounts(chatID, day, day)
	if err != nil {
		t.Fatalf("GetMemberCounts() error = %v", err)
	}
	if len(rows) == 0 {
		return 0, false
	}
	return *rows[0].Count, true
}

func TestRunDueMeasuresEachActiveChatOncePerDay(t *testing.T) {
	now := testNow
	today := newActiveChat(t, "2099-03-10")
	yesterday := newActiveChat(t, "2099-03-09")
	stale := newActiveChat(t, "2099-03-06")
	client := newCountClient()
	s := newTestSnapshotter(client, now)

	s.runDue(context.Background(), now)
	s.runDue(context.Background(), now.Add(time.Hour))

	for _, chatID := range []int64{today, yesterday} {
		if calls := client.callsFor(chatID); calls != 1 {
			t.Fatalf("chat %d calls = %d, want 1 across two ticks of one day", chatID, calls)
		}
		if count, ok := storedCount(t, chatID, "2099-03-10"); !ok || count != 100 {
			t.Fatalf("chat %d count = %d (stored %v), want 100", chatID, count, ok)
		}
	}
	if calls := client.callsFor(stale); calls != 0 {
		t.Fatalf("chat idle since 2099-03-06 calls = %d, want 0", calls)
	}

	// The next UTC day measures again.
	s.runDue(context.Background(), now.AddDate(0, 0, 1))
	if count, ok := storedCount(t, today, "2099-03-11"); !ok || count != 101 {
		t.Fatalf("next-day count = %d (stored %v), want 101", count, ok)
	}
}

func TestTwoReplicasMeasureAChatOnce(t *testing.T) {
	now := testNow
	chatID := newActiveChat(t, "2099-03-10")
	client := newCountClient()
	replicas := []*Snapshotter{newTestSnapshotter(client, now), newTestSnapshotter(client, now)}

	var wg sync.WaitGroup
	for _, replica := range replicas {
		wg.Add(1)
		go func() {
			defer wg.Done()
			replica.runDue(context.Background(), now)
		}()
	}
	wg.Wait()

	if calls := client.callsFor(chatID); calls != 1 {
		t.Fatalf("calls = %d, want 1: the claim must stop the second replica", calls)
	}
	if _, ok := storedCount(t, chatID, "2099-03-10"); !ok {
		t.Fatal("no count stored")
	}
}

func TestTransientFailureIsRetriedOnTheNextTick(t *testing.T) {
	now := testNow
	chatID := newActiveChat(t, "2099-03-10")
	client := newCountClient()
	client.failures[chatID] = 1
	client.failErr = errors.New("telegram is unreachable")
	s := newTestSnapshotter(client, now)

	s.runDue(context.Background(), now)
	if _, ok := storedCount(t, chatID, "2099-03-10"); ok {
		t.Fatal("a count was stored after a failed call")
	}

	s.runDue(context.Background(), now.Add(10*time.Minute))
	if calls := client.callsFor(chatID); calls != 2 {
		t.Fatalf("calls = %d, want 2 (the failure released the claim)", calls)
	}
	if _, ok := storedCount(t, chatID, "2099-03-10"); !ok {
		t.Fatal("the retry stored no count")
	}
}

func TestPermanentFailureIsNotRetriedThatDay(t *testing.T) {
	now := testNow
	chatID := newActiveChat(t, "2099-03-10")
	client := newCountClient()
	client.failures[chatID] = 5
	client.failErr = errors.New("Forbidden: bot was kicked from the supergroup chat")
	s := newTestSnapshotter(client, now)

	s.runDue(context.Background(), now)
	s.runDue(context.Background(), now.Add(10*time.Minute))

	if calls := client.callsFor(chatID); calls != 1 {
		t.Fatalf("calls = %d, want 1: a permanent failure keeps the day's claim", calls)
	}
	if _, ok := storedCount(t, chatID, "2099-03-10"); ok {
		t.Fatal("a count was stored for a chat Telegram refused")
	}
}

func TestShutdownDuringACallReleasesItsClaim(t *testing.T) {
	now := testNow
	chatID := newActiveChat(t, "2099-03-10")
	client := newCountClient()
	s := newTestSnapshotter(client, now)

	// Shutdown lands while the call is in flight: the call returns the
	// context error, a transient failure.
	ctx, cancel := context.WithCancel(context.Background())
	client.onCall = cancel
	s.runDue(ctx, now)
	if calls := client.callsFor(chatID); calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
	if _, ok := storedCount(t, chatID, "2099-03-10"); ok {
		t.Fatal("a count was stored by a cancelled call")
	}

	// The cancelled call left no claim behind: a live tick measures the chat.
	client.onCall = nil
	s.runDue(context.Background(), now)
	if _, ok := storedCount(t, chatID, "2099-03-10"); !ok {
		t.Fatal("the chat was not measured after a cancelled call")
	}

	// Stop is safe twice and on a snapshotter that never started.
	s.Stop()
	s.Stop()
}
