//go:build testtools

package warns

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	gocache "github.com/eko/gocache/lib/v4/cache"
	"github.com/eko/gocache/lib/v4/marshaler"
	gocache_store "github.com/eko/gocache/store/redis/v4"
	"github.com/redis/go-redis/v9"

	"github.com/divkix/Alita_Robot/alita/db"
	dbcache "github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

const thirtyDays = 30 * 24 * 60 * 60

func timeAgo(d time.Duration) *time.Time {
	at := time.Now().UTC().Add(-d).Truncate(time.Second)
	return &at
}

// seedWarns stores entries as the user's warns, with a matching count and
// reasons column, the way WarnUser leaves them.
func seedWarns(t *testing.T, chatID, userID int64, entries ...models.WarnEntry) {
	t.Helper()
	reasons := make(models.StringArray, len(entries))
	for i := range entries {
		entries[i].UserId = userID
		entries[i].ChatId = chatID
		reasons[i] = entries[i].Reason
	}
	if err := db.DB.Create(&models.Warns{
		UserId: userID, ChatId: chatID, NumWarns: len(entries), Reasons: reasons,
	}).Error; err != nil {
		t.Fatalf("create warns row: %v", err)
	}
	if err := db.DB.Create(&entries).Error; err != nil {
		t.Fatalf("create warn entries: %v", err)
	}
	dbcache.ResetLocalForTest()
}

func setExpiry(t *testing.T, chatID int64, seconds int) {
	t.Helper()
	if err := SetWarnExpiry(chatID, seconds); err != nil {
		t.Fatalf("SetWarnExpiry(%d) error = %v", seconds, err)
	}
}

func storedRow(t *testing.T, chatID, userID int64) models.Warns {
	t.Helper()
	var row models.Warns
	if err := db.DB.Where("chat_id = ? AND user_id = ?", chatID, userID).Take(&row).Error; err != nil {
		t.Fatalf("load warns row: %v", err)
	}
	return row
}

func useMiniredis(t *testing.T) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() error = %v", err)
	}
	t.Cleanup(mr.Close)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	prevMarshal, prevManager, prevClient := cache.GetCacheState()
	cache.SetCacheState(marshaler.New(gocache.New[any](gocache_store.NewRedis(client))), gocache.New[any](gocache_store.NewRedis(client)), client)
	t.Cleanup(func() { cache.SetCacheState(prevMarshal, prevManager, prevClient) })
	dbcache.ResetLocalForTest()
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestWarnUserDropsExpiredWarns(t *testing.T) {
	chatID, userID := newEntriesFixture(t)
	setExpiry(t, chatID, thirtyDays)
	seedWarns(t, chatID, userID,
		models.WarnEntry{Reason: "old", WarnedBy: 77, WarnedAt: timeAgo(40 * 24 * time.Hour)},
		models.WarnEntry{Reason: "recent", WarnedBy: 77, WarnedAt: timeAgo(24 * time.Hour)},
	)

	num, entries, err := WarnUser(userID, chatID, models.WarnEntry{Reason: "new", WarnedBy: 77})
	if err != nil {
		t.Fatalf("WarnUser() error = %v", err)
	}

	want := []string{"recent", "new"}
	if num != 2 || !equalStrings(reasonsOf(entries), want) {
		t.Fatalf("WarnUser() = %d, %v; want 2, %v", num, reasonsOf(entries), want)
	}
	if got := reasonsOf(storedEntries(t, chatID, userID)); !equalStrings(got, want) {
		t.Fatalf("stored entries = %v, want %v", got, want)
	}
	if row := storedRow(t, chatID, userID); row.NumWarns != 2 || !equalStrings(row.Reasons, want) {
		t.Fatalf("warns row = %d %v, want 2 %v so an older bot reads the same", row.NumWarns, row.Reasons, want)
	}
}

func TestWarnUserKeepsUndatedWarnsWhenExpiring(t *testing.T) {
	chatID, userID := newEntriesFixture(t)
	setExpiry(t, chatID, 60)
	// Copied from warns_users by the entries migration: no issuer, no time.
	seedWarns(t, chatID, userID, models.WarnEntry{Reason: "legacy"})

	num, entries, err := WarnUser(userID, chatID, models.WarnEntry{Reason: "new", WarnedBy: 77})
	if err != nil {
		t.Fatalf("WarnUser() error = %v", err)
	}

	if want := []string{"legacy", "new"}; num != 2 || !equalStrings(reasonsOf(entries), want) {
		t.Fatalf("WarnUser() = %d, %v; want 2, %v", num, reasonsOf(entries), want)
	}
}

func TestWarnExpiresOnceItsExpiryHasPassed(t *testing.T) {
	chatID, userID := newEntriesFixture(t)
	setExpiry(t, chatID, thirtyDays)
	now := time.Now().UTC().Truncate(time.Second)
	prev := timeNow
	timeNow = func() time.Time { return now }
	t.Cleanup(func() { timeNow = prev })
	exactly := now.Add(-thirtyDays * time.Second)
	justUnder := exactly.Add(time.Second)
	seedWarns(t, chatID, userID,
		models.WarnEntry{Reason: "exactly", WarnedBy: 77, WarnedAt: &exactly},
		models.WarnEntry{Reason: "just under", WarnedBy: 77, WarnedAt: &justUnder},
	)

	num, entries := GetWarns(userID, chatID)

	if want := []string{"just under"}; num != 1 || !equalStrings(reasonsOf(entries), want) {
		t.Fatalf("GetWarns() = %d, %v; want 1, %v", num, reasonsOf(entries), want)
	}
}

func TestGetWarnsHidesExpiredWithoutWriting(t *testing.T) {
	chatID, userID := newEntriesFixture(t)
	setExpiry(t, chatID, thirtyDays)
	seedWarns(t, chatID, userID,
		models.WarnEntry{Reason: "old", WarnedBy: 77, WarnedAt: timeAgo(40 * 24 * time.Hour)},
		models.WarnEntry{Reason: "recent", WarnedBy: 77, WarnedAt: timeAgo(time.Hour)},
	)

	num, entries := GetWarns(userID, chatID)

	if want := []string{"recent"}; num != 1 || !equalStrings(reasonsOf(entries), want) {
		t.Fatalf("GetWarns() = %d, %v; want 1, %v", num, reasonsOf(entries), want)
	}
	if row := storedRow(t, chatID, userID); row.NumWarns != 2 || len(storedEntries(t, chatID, userID)) != 2 {
		t.Fatalf("reading changed the stored warns: num_warns = %d", row.NumWarns)
	}
}

func TestGetWarnsAppliesExpiryToCachedWarns(t *testing.T) {
	chatID, userID := newEntriesFixture(t)
	useMiniredis(t)
	seedWarns(t, chatID, userID,
		models.WarnEntry{Reason: "old", WarnedBy: 77, WarnedAt: timeAgo(40 * 24 * time.Hour)},
		models.WarnEntry{Reason: "recent", WarnedBy: 77, WarnedAt: timeAgo(time.Hour)},
	)
	if num, _ := GetWarns(userID, chatID); num != 2 {
		t.Fatalf("GetWarns() without expiry = %d, want 2", num)
	}

	// Leaves the cached warns in place, as a warn ageing past the expiry does.
	setExpiry(t, chatID, thirtyDays)
	num, entries := GetWarns(userID, chatID)

	if want := []string{"recent"}; num != 1 || !equalStrings(reasonsOf(entries), want) {
		t.Fatalf("GetWarns() = %d, %v; want 1, %v", num, reasonsOf(entries), want)
	}
}

func TestRemoveWarnDropsNewestLiveWarn(t *testing.T) {
	chatID, userID := newEntriesFixture(t)
	setExpiry(t, chatID, thirtyDays)
	seedWarns(t, chatID, userID,
		models.WarnEntry{Reason: "old", WarnedBy: 77, WarnedAt: timeAgo(40 * 24 * time.Hour)},
		models.WarnEntry{Reason: "recent", WarnedBy: 77, WarnedAt: timeAgo(24 * time.Hour)},
		models.WarnEntry{Reason: "newest", WarnedBy: 77, WarnedAt: timeAgo(time.Hour)},
	)

	if removed, err := RemoveWarn(userID, chatID); err != nil || !removed {
		t.Fatalf("RemoveWarn() = %v, %v; want true, nil", removed, err)
	}

	want := []string{"recent"}
	if got := reasonsOf(storedEntries(t, chatID, userID)); !equalStrings(got, want) {
		t.Fatalf("stored entries = %v, want %v", got, want)
	}
	if row := storedRow(t, chatID, userID); row.NumWarns != 1 || !equalStrings(row.Reasons, want) {
		t.Fatalf("warns row = %d %v, want 1 %v", row.NumWarns, row.Reasons, want)
	}
}

func TestRemoveWarnWithOnlyExpiredWarnsRemovesNothing(t *testing.T) {
	chatID, userID := newEntriesFixture(t)
	useMiniredis(t)
	setExpiry(t, chatID, thirtyDays)
	seedWarns(t, chatID, userID,
		models.WarnEntry{Reason: "old", WarnedBy: 77, WarnedAt: timeAgo(40 * 24 * time.Hour)},
	)
	if num, _ := GetWarns(userID, chatID); num != 0 {
		t.Fatalf("GetWarns() before = %d, want 0", num)
	}

	if removed, err := RemoveWarn(userID, chatID); err != nil || removed {
		t.Fatalf("RemoveWarn() = %v, %v; want false, nil", removed, err)
	}

	if row := storedRow(t, chatID, userID); row.NumWarns != 0 || len(storedEntries(t, chatID, userID)) != 0 {
		t.Fatalf("warns row = %d with %d entries, want the expired warn deleted", row.NumWarns, len(storedEntries(t, chatID, userID)))
	}
	// With expiry off again, a cache left from before the cleanup would bring
	// the deleted warn back.
	setExpiry(t, chatID, 0)
	if num, entries := GetWarns(userID, chatID); num != 0 || len(entries) != 0 {
		t.Fatalf("GetWarns() after = %d, %+v; want no warns", num, entries)
	}
}

func TestSetWarnExpiryKeepsLimitAndMode(t *testing.T) {
	chatID, _ := newEntriesFixture(t)

	setExpiry(t, chatID, 86400)
	if s := GetWarnSetting(chatID); s.WarnExpiry != 86400 || s.WarnLimit != 3 || s.WarnMode != "mute" {
		t.Fatalf("settings = %+v, want expiry 86400 with default limit and mode", s)
	}
	if err := SetWarnLimit(chatID, 6); err != nil {
		t.Fatalf("SetWarnLimit() error = %v", err)
	}
	if err := SetWarnMode(chatID, "tban", 3600); err != nil {
		t.Fatalf("SetWarnMode() error = %v", err)
	}
	if s := GetWarnSetting(chatID); s.WarnExpiry != 86400 {
		t.Fatalf("expiry after limit and mode = %d, want 86400", s.WarnExpiry)
	}

	setExpiry(t, chatID, 0)
	if s := GetWarnSetting(chatID); s.WarnExpiry != 0 || s.WarnLimit != 6 || s.WarnMode != "tban" || s.WarnActionTime != 3600 {
		t.Fatalf("settings = %+v, want expiry off with limit 6, tban 3600", s)
	}
}

func TestWarnUserKeepsReasonsWithoutEntriesWhenPruning(t *testing.T) {
	chatID, userID := newEntriesFixture(t)
	setExpiry(t, chatID, thirtyDays)
	seedWarns(t, chatID, userID,
		models.WarnEntry{Reason: "old", WarnedBy: 77, WarnedAt: timeAgo(40 * 24 * time.Hour)},
		models.WarnEntry{Reason: "recent", WarnedBy: 77, WarnedAt: timeAgo(24 * time.Hour)},
	)
	// After a rollback an older release warned the user again: a reason and a
	// count without an entry.
	if err := db.DB.Model(&models.Warns{}).
		Where("chat_id = ? AND user_id = ?", chatID, userID).
		Updates(map[string]any{"num_warns": 3, "warns": models.StringArray{"old", "recent", "older release"}}).Error; err != nil {
		t.Fatalf("add warn without entry: %v", err)
	}
	dbcache.ResetLocalForTest()

	if _, _, err := WarnUser(userID, chatID, models.WarnEntry{Reason: "new", WarnedBy: 77}); err != nil {
		t.Fatalf("WarnUser() error = %v", err)
	}

	want := []string{"recent", "older release", "new"}
	if row := storedRow(t, chatID, userID); row.NumWarns != 3 || !equalStrings(row.Reasons, want) {
		t.Fatalf("warns row = %d %v, want 3 %v", row.NumWarns, row.Reasons, want)
	}
}

func TestPruneToNoWarnsDeletesStrayEntries(t *testing.T) {
	chatID, userID := newEntriesFixture(t)
	setExpiry(t, chatID, thirtyDays)
	seedWarns(t, chatID, userID,
		models.WarnEntry{Reason: "old", WarnedBy: 77, WarnedAt: timeAgo(40 * 24 * time.Hour)},
		models.WarnEntry{Reason: "stray", WarnedBy: 77, WarnedAt: timeAgo(24 * time.Hour)},
	)
	// An older release's /rmwarn lowered the count but left the entry.
	if err := db.DB.Model(&models.Warns{}).
		Where("chat_id = ? AND user_id = ?", chatID, userID).
		Updates(map[string]any{"num_warns": 1, "warns": models.StringArray{"old"}}).Error; err != nil {
		t.Fatalf("lower warn count: %v", err)
	}
	dbcache.ResetLocalForTest()

	if removed, err := RemoveWarn(userID, chatID); err != nil || removed {
		t.Fatalf("RemoveWarn() = %v, %v; want false, nil", removed, err)
	}

	if got := storedEntries(t, chatID, userID); len(got) != 0 {
		t.Fatalf("stored entries = %v, want none once the user has no warns", reasonsOf(got))
	}
	if row := storedRow(t, chatID, userID); row.NumWarns != 0 || len(row.Reasons) != 0 {
		t.Fatalf("warns row = %d %v, want 0 []", row.NumWarns, row.Reasons)
	}
}
