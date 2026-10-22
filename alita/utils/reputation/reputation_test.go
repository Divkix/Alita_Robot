//go:build testtools

package reputation

import (
	"context"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	gocache "github.com/eko/gocache/lib/v4/cache"
	"github.com/eko/gocache/lib/v4/marshaler"
	gocache_store "github.com/eko/gocache/store/redis/v4"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db"
	dbcache "github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/moderation"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/internal/testdb"
)

func TestMain(m *testing.M) {
	os.Exit(testdb.Run(m, &models.ModerationEvent{}))
}

var testSeq atomic.Int64

// testID returns an id no other test uses. Back-to-back clock reads can
// repeat, so the counter is seeded once from the clock.
func testID() int64 {
	testSeq.CompareAndSwap(0, time.Now().UnixNano())
	return testSeq.Add(1)
}

// enableReputation turns the feature on with explicit operator settings and
// restores the process config when the test ends.
func enableReputation(t *testing.T, minChats, windowDays int) {
	t.Helper()
	previous := *config.AppConfig
	t.Cleanup(func() { *config.AppConfig = previous })
	config.AppConfig.EnableReputation = true
	config.AppConfig.ReputationMinChats = minChats
	config.AppConfig.ReputationWindowDays = windowDays
}

// fixClock pins the window clock, so no test sleeps.
func fixClock(t *testing.T, at time.Time) {
	t.Helper()
	previous := now
	now = func() time.Time { return at }
	t.Cleanup(func() { now = previous })
}

// seedEvent records one moderation event the way the command pipeline does.
func seedEvent(t *testing.T, chatID, userID int64, action string, at time.Time) {
	t.Helper()
	if err := moderation.Record(models.ModerationEvent{
		ChatId: chatID, ActorId: 1, TargetUserId: userID, Action: action, CreatedAt: at,
	}); err != nil {
		t.Fatalf("record %s event in chat %d: %v", action, chatID, err)
	}
}

func TestLookupNeedsThreeDistinctChats(t *testing.T) {
	enableReputation(t, 3, 30)
	at := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	fixClock(t, at)

	listedUser := testID()
	for range 3 {
		seedEvent(t, testID(), listedUser, models.ModerationActionBan, at.Add(-5*24*time.Hour))
	}
	unlistedUser := testID()
	for range 2 {
		seedEvent(t, testID(), unlistedUser, models.ModerationActionKick, at.Add(-5*24*time.Hour))
	}

	listed, chats, err := Lookup(context.Background(), listedUser)
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	if !listed || chats != 3 {
		t.Fatalf("Lookup(three chats) = %v, %d; want listed with 3 chats", listed, chats)
	}

	listed, chats, err = Lookup(context.Background(), unlistedUser)
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	if listed || chats != 2 {
		t.Fatalf("Lookup(two chats) = %v, %d; want not listed with 2 chats", listed, chats)
	}

	// The threshold is operator config: the same two-chat signal lists the user
	// once the minimum drops to 2.
	config.AppConfig.ReputationMinChats = 2
	listed, chats, err = Lookup(context.Background(), unlistedUser)
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	if !listed || chats != 2 {
		t.Fatalf("Lookup(two chats, minimum 2) = %v, %d; want listed", listed, chats)
	}
}

func TestLookupIgnoresEventsOutsideTheWindow(t *testing.T) {
	enableReputation(t, 3, 30)
	at := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	fixClock(t, at)
	userID := testID()
	for range 3 {
		seedEvent(t, testID(), userID, models.ModerationActionBan, at.Add(-31*24*time.Hour))
	}

	listed, chats, err := Lookup(context.Background(), userID)
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	if listed || chats != 0 {
		t.Fatalf("Lookup() = %v, %d; want no signal from chats older than the window", listed, chats)
	}
}

func TestLookupCountsOneChatOnce(t *testing.T) {
	enableReputation(t, 3, 30)
	at := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	fixClock(t, at)
	userID := testID()
	chatID := testID()
	for range 10 {
		seedEvent(t, chatID, userID, models.ModerationActionBan, at.Add(-time.Hour))
	}

	listed, chats, err := Lookup(context.Background(), userID)
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	if listed || chats != 1 {
		t.Fatalf("Lookup(ten bans in one chat) = %v, %d; want not listed with 1 chat", listed, chats)
	}
}

func TestLookupUnbanInsideTheWindowClears(t *testing.T) {
	enableReputation(t, 3, 30)
	at := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	fixClock(t, at)
	userID := testID()
	chatIDs := []int64{testID(), testID(), testID()}
	for _, chatID := range chatIDs {
		seedEvent(t, chatID, userID, models.ModerationActionBan, at.Add(-5*24*time.Hour))
	}
	seedEvent(t, chatIDs[0], userID, models.ModerationActionUnban, at.Add(-time.Hour))

	listed, chats, err := Lookup(context.Background(), userID)
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	if listed {
		t.Fatalf("Lookup() = %v, %d; want an unban inside the window to clear the signal", listed, chats)
	}
	if chats != 3 {
		t.Fatalf("Lookup() chats = %d, want the 3 removals reported alongside the cleared verdict", chats)
	}
}

// A no-op /unban in a chat that never banned or kicked the user must not clear
// a signal other chats recorded (M7).
func TestLookupUnrelatedChatUnbanDoesNotClear(t *testing.T) {
	enableReputation(t, 3, 30)
	at := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	fixClock(t, at)
	userID := testID()
	for range 3 {
		seedEvent(t, testID(), userID, models.ModerationActionBan, at.Add(-5*24*time.Hour))
	}
	seedEvent(t, testID(), userID, models.ModerationActionUnban, at.Add(-time.Hour))

	listed, chats, err := Lookup(context.Background(), userID)
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	if !listed || chats != 3 {
		t.Fatalf("Lookup() = %v, %d; want an unban in a chat that never removed the user to leave the 3-chat signal", listed, chats)
	}
}

// An unban recorded before the chat's own ban does not reverse it (M7).
func TestLookupUnbanBeforeTheBanDoesNotClear(t *testing.T) {
	enableReputation(t, 3, 30)
	at := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	fixClock(t, at)
	userID := testID()
	chatIDs := []int64{testID(), testID(), testID()}
	for _, chatID := range chatIDs {
		seedEvent(t, chatID, userID, models.ModerationActionBan, at.Add(-5*24*time.Hour))
	}
	seedEvent(t, chatIDs[0], userID, models.ModerationActionUnban, at.Add(-6*24*time.Hour))

	listed, chats, err := Lookup(context.Background(), userID)
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	if !listed || chats != 3 {
		t.Fatalf("Lookup() = %v, %d; want an unban older than the chat's ban to leave the 3-chat signal", listed, chats)
	}
}

func TestLookupUnbanOutsideTheWindowDoesNotClear(t *testing.T) {
	enableReputation(t, 3, 30)
	at := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	fixClock(t, at)
	userID := testID()
	chatIDs := []int64{testID(), testID(), testID()}
	for _, chatID := range chatIDs {
		seedEvent(t, chatID, userID, models.ModerationActionBan, at.Add(-5*24*time.Hour))
	}
	seedEvent(t, chatIDs[0], userID, models.ModerationActionUnban, at.Add(-31*24*time.Hour))

	listed, chats, err := Lookup(context.Background(), userID)
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	if !listed || chats != 3 {
		t.Fatalf("Lookup() = %v, %d; want an old unban to leave the 3-chat signal", listed, chats)
	}
}

// /unmute records nothing (C1), and the action CHECK rejects it, so a user's
// only events can be mutes; mutes never count as removals.
func TestLookupIgnoresMuteEvents(t *testing.T) {
	enableReputation(t, 3, 30)
	at := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	fixClock(t, at)
	userID := testID()
	for range 3 {
		seedEvent(t, testID(), userID, models.ModerationActionMute, at.Add(-time.Hour))
	}

	listed, chats, err := Lookup(context.Background(), userID)
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	if listed || chats != 0 {
		t.Fatalf("Lookup(mutes only) = %v, %d; want no signal", listed, chats)
	}
}

// dropEventsTable breaks the aggregate's query for the rest of the test and
// restores the schema before the next one.
func dropEventsTable(t *testing.T) {
	t.Helper()
	if err := db.DB.Migrator().DropTable(&models.ModerationEvent{}); err != nil {
		t.Fatalf("drop moderation_events: %v", err)
	}
	t.Cleanup(func() {
		if err := db.DB.AutoMigrate(&models.ModerationEvent{}); err != nil {
			t.Errorf("restore moderation_events: %v", err)
		}
	})
}

func TestLookupFailsOpenOnDatabaseError(t *testing.T) {
	enableReputation(t, 3, 30)
	previousHooks := log.StandardLogger().ReplaceHooks(make(log.LevelHooks))
	t.Cleanup(func() { log.StandardLogger().ReplaceHooks(previousHooks) })
	hook := logtest.NewGlobal()
	dropEventsTable(t)

	listed, chats, err := Lookup(context.Background(), testID())
	if err == nil {
		t.Fatal("Lookup() error = nil, want the database failure")
	}
	if listed || chats != 0 {
		t.Fatalf("Lookup() = %v, %d; want not listed on a database error", listed, chats)
	}
	warned := false
	for _, entry := range hook.AllEntries() {
		if entry.Level == log.WarnLevel && strings.Contains(entry.Message, "not listed") {
			warned = true
		}
	}
	if !warned {
		t.Fatalf("lookup failure logged at warn level, got entries %+v", hook.AllEntries())
	}
}

func TestLookupDisabledNeverQueries(t *testing.T) {
	previous := *config.AppConfig
	t.Cleanup(func() { *config.AppConfig = previous })
	config.AppConfig.EnableReputation = false
	// With the table gone, any query would fail; a nil error proves the switch
	// returned before touching the database.
	dropEventsTable(t)

	listed, chats, err := Lookup(context.Background(), testID())
	if err != nil {
		t.Fatalf("Lookup() error = %v, want no query while the feature is disabled", err)
	}
	if listed || chats != 0 {
		t.Fatalf("Lookup() = %v, %d; want not listed while the feature is disabled", listed, chats)
	}
}

// withRedisCache points the cache stack at miniredis for the test's duration.
func withRedisCache(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	previousMarshal, previousManager, previousClient := cache.GetCacheState()
	manager := gocache.New[any](gocache_store.NewRedis(client))
	cache.SetCacheState(marshaler.New(manager), manager, client)
	t.Cleanup(func() {
		dbcache.ResetLocalForTest()
		cache.SetCacheState(previousMarshal, previousManager, previousClient)
	})
	dbcache.ResetLocalForTest()
	return server
}

func TestLookupCachesAVerdictForTenMinutes(t *testing.T) {
	enableReputation(t, 3, 30)
	server := withRedisCache(t)
	at := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	fixClock(t, at)
	userID := testID()
	for range 2 {
		seedEvent(t, testID(), userID, models.ModerationActionBan, at.Add(-5*24*time.Hour))
	}

	listed, chats, err := Lookup(context.Background(), userID)
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	if listed || chats != 2 {
		t.Fatalf("Lookup() = %v, %d; want not listed with 2 chats", listed, chats)
	}
	key := dbcache.CacheKey(cacheModule, userID)
	if ttl := server.TTL(key); ttl <= 9*time.Minute || ttl > 10*time.Minute {
		t.Fatalf("cache TTL = %v, want 10 minutes", ttl)
	}

	// A third removal written after the first verdict: nothing invalidates the
	// key, so both the in-process copy and Redis keep serving the old result.
	seedEvent(t, testID(), userID, models.ModerationActionBan, at.Add(-time.Hour))
	if listed, chats, err = Lookup(context.Background(), userID); err != nil || listed || chats != 2 {
		t.Fatalf("second Lookup() = %v, %d, %v; want the in-process cached verdict", listed, chats, err)
	}
	dbcache.ResetLocalForTest()
	if listed, chats, err = Lookup(context.Background(), userID); err != nil || listed || chats != 2 {
		t.Fatalf("Lookup() after dropping the local copy = %v, %d, %v; want the Redis cached verdict", listed, chats, err)
	}

	server.FastForward(9 * time.Minute)
	dbcache.ResetLocalForTest()
	if listed, chats, err = Lookup(context.Background(), userID); err != nil || listed || chats != 2 {
		t.Fatalf("Lookup() nine minutes later = %v, %d, %v; want the cached verdict still valid", listed, chats, err)
	}

	server.FastForward(2 * time.Minute)
	dbcache.ResetLocalForTest()
	if listed, chats, err = Lookup(context.Background(), userID); err != nil || !listed || chats != 3 {
		t.Fatalf("Lookup() after the TTL = %v, %d, %v; want a fresh verdict with 3 chats", listed, chats, err)
	}
}
