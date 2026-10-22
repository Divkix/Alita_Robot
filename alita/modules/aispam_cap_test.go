//go:build testtools

package modules

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db/aispam"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

// withAISpamDailyLimit sets AISPAM_DAILY_LIMIT, pins the cap's clock to a
// fixed time and starts from a fresh process count.
func withAISpamDailyLimit(t *testing.T, limit int, now time.Time) {
	t.Helper()

	previousLimit, previousNow := config.AppConfig.AISpamDailyLimit, aispamDailyNow
	config.AppConfig.AISpamDailyLimit = limit
	aispamDailyNow = func() time.Time { return now }
	resetAISpamDailyProcess()
	t.Cleanup(func() {
		config.AppConfig.AISpamDailyLimit = previousLimit
		aispamDailyNow = previousNow
		resetAISpamDailyProcess()
	})
}

// resetAISpamDailyProcess forgets this process's count, as a restart or
// another replica would see it.
func resetAISpamDailyProcess() {
	aispamLocalDaily.mu.Lock()
	aispamLocalDaily.day, aispamLocalDaily.count, aispamLocalDaily.warned = "", 0, ""
	aispamLocalDaily.mu.Unlock()
}

// withoutRedis runs the test with no Redis client configured.
func withoutRedis(t *testing.T) {
	t.Helper()

	previousMarshal, previousManager, previousClient := cache.GetCacheState()
	cache.SetCacheState(previousMarshal, previousManager, nil)
	t.Cleanup(func() {
		cache.SetCacheState(previousMarshal, previousManager, previousClient)
	})
}

// aispamCapFixture is a chat with the AI check on and a Jev server that keeps
// every message and counts its requests.
type aispamCapFixture struct {
	dispatcher *ext.Dispatcher
	bot        *gotgbot.Bot
	chat       gotgbot.Chat
	requests   *atomic.Int64
	nextID     int64
}

func newAISpamCapFixture(t *testing.T) *aispamCapFixture {
	t.Helper()

	resetAISpamRuntime()
	withAISpamTestConfig(t, "typesafe-test-key", true, 0)

	requests := &atomic.Int64{}
	withAISpamJevServer(t, func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		_, _ = fmt.Fprint(w, aispamDecisionBody(0.1, "none"))
	})

	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Capped Chat"}
	if err := aispam.SetAISpamEnabled(chat.Id, true); err != nil {
		t.Fatalf("SetAISpamEnabled() error = %v", err)
	}

	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadAISpam(dispatcher)

	return &aispamCapFixture{
		dispatcher: dispatcher,
		bot:        newModuleTestBot(newModuleBotClient()),
		chat:       chat,
		requests:   requests,
		nextID:     900,
	}
}

// send posts n messages and waits for their checks.
func (f *aispamCapFixture) send(t *testing.T, n int) {
	t.Helper()

	for range n {
		f.nextID++
		aispamProcessUpdate(t, f.dispatcher, f.bot, f.chat, gotgbot.User{Id: 42, FirstName: "Member"}, f.nextID, fmt.Sprintf("hello there number %d", f.nextID))
	}
	DrainAISpamChecks()
}

func TestAISpamDailyLimitStopsChecksUntilTheNextUTCDay(t *testing.T) {
	for _, tt := range []struct {
		name  string
		redis func(*testing.T)
	}{
		{name: "redis", redis: withMiniredis},
		{name: "no redis", redis: withoutRedis},
		{name: "redis unreachable", redis: withUnreachableRedis},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tt.redis(t)
			day := time.Date(2026, 10, 22, 23, 0, 0, 0, time.UTC)
			withAISpamDailyLimit(t, 2, day)
			f := newAISpamCapFixture(t)

			f.send(t, 4)
			if got := f.requests.Load(); got != 2 {
				t.Fatalf("Jev requests = %d, want 2 (the daily limit)", got)
			}
			checked, _, _, shed, failed := GetAISpamStats()
			if checked != 2 || shed != 2 || failed != 0 {
				t.Fatalf("stats = checked:%d shed:%d failed:%d, want 2/2/0", checked, shed, failed)
			}
			if aispamBreakerPaused(f.chat.Id) {
				t.Fatal("checks over the limit paused the chat's breaker")
			}

			aispamDailyNow = func() time.Time { return day.Add(2 * time.Hour) }
			f.send(t, 3)
			if got := f.requests.Load(); got != 4 {
				t.Fatalf("Jev requests after the UTC day changed = %d, want 4", got)
			}
		})
	}
}

// The count lives in Redis, so a replica that restarts, or another one,
// keeps to the limit the bot already used.
func TestAISpamDailyLimitIsSharedThroughRedis(t *testing.T) {
	withMiniredis(t)
	withAISpamDailyLimit(t, 2, time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC))
	f := newAISpamCapFixture(t)

	f.send(t, 2)
	resetAISpamDailyProcess()
	f.send(t, 2)

	if got := f.requests.Load(); got != 2 {
		t.Fatalf("Jev requests = %d, want 2 across both processes", got)
	}
}

func TestAISpamWithoutDailyLimitChecksEveryMessage(t *testing.T) {
	withMiniredis(t)
	withAISpamDailyLimit(t, 0, time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC))
	f := newAISpamCapFixture(t)

	f.send(t, 5)

	if got := f.requests.Load(); got != 5 {
		t.Fatalf("Jev requests = %d, want 5 (no cap)", got)
	}
	if _, _, _, shed, _ := GetAISpamStats(); shed != 0 {
		t.Fatalf("shed checks = %d, want 0", shed)
	}
}
