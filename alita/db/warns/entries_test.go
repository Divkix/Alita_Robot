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
	"gorm.io/gorm/clause"

	"github.com/divkix/Alita_Robot/alita/db"
	dbcache "github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/user"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

func newEntriesFixture(t *testing.T) (chatID, userID int64) {
	t.Helper()
	skipIfNoDb(t)
	chatID = time.Now().UnixNano()
	userID = chatID + 1
	if err := chats.EnsureChatInDb(chatID, "warn-entries"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	// Fixtures below insert warns_users and warn_entries rows directly, so the
	// users row their foreign keys need must exist up front.
	if err := user.EnsureUserInDb(userID, "", "warn-entries"); err != nil {
		t.Fatalf("EnsureUserInDb() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.WarnEntry{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Warns{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.WarnSettings{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
		_ = db.DB.Where("user_id = ?", userID).Delete(&models.User{}).Error
	})
	return chatID, userID
}

func storedEntries(t *testing.T, chatID, userID int64) []models.WarnEntry {
	t.Helper()
	var entries []models.WarnEntry
	if err := db.DB.Where("chat_id = ? AND user_id = ?", chatID, userID).Order("id").Find(&entries).Error; err != nil {
		t.Fatalf("load warn entries: %v", err)
	}
	return entries
}

func reasonsOf(entries []models.WarnEntry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Reason
	}
	return out
}

func TestWarnUserRecordsIssuerAndTime(t *testing.T) {
	chatID, userID := newEntriesFixture(t)

	before := time.Now().Add(-time.Second)
	_, entries, err := WarnUser(userID, chatID, models.WarnEntry{Reason: "spam", WarnedBy: 77, Anonymous: true})
	after := time.Now().Add(time.Second)
	if err != nil {
		t.Fatalf("WarnUser() error = %v", err)
	}

	_, listed := GetWarns(userID, chatID)
	for name, got := range map[string][]models.WarnEntry{"returned": entries, "listed": listed, "stored": storedEntries(t, chatID, userID)} {
		if len(got) != 1 {
			t.Fatalf("%s entries = %+v, want one", name, got)
		}
		e := got[0]
		if e.Reason != "spam" || e.WarnedBy != 77 || !e.Anonymous {
			t.Fatalf("%s entry = %+v, want spam by 77, anonymous", name, e)
		}
		if e.WarnedAt == nil || e.WarnedAt.Before(before) || e.WarnedAt.After(after) {
			t.Fatalf("%s warned_at = %v, want between %v and %v", name, e.WarnedAt, before, after)
		}
	}
}

func TestWarnUserKeepsLegacyReasonsColumn(t *testing.T) {
	chatID, userID := newEntriesFixture(t)

	for _, reason := range []string{"one", "two"} {
		if _, _, err := WarnUser(userID, chatID, models.WarnEntry{Reason: reason, WarnedBy: 77}); err != nil {
			t.Fatalf("WarnUser(%s) error = %v", reason, err)
		}
	}
	if _, err := RemoveWarn(userID, chatID); err != nil {
		t.Fatalf("RemoveWarn() error = %v", err)
	}

	var row models.Warns
	if err := db.DB.Where("chat_id = ? AND user_id = ?", chatID, userID).Take(&row).Error; err != nil {
		t.Fatalf("load warns row: %v", err)
	}
	if len(row.Reasons) != 1 || row.Reasons[0] != "one" {
		t.Fatalf("warns column = %v, want [one] so an older bot still reads it", row.Reasons)
	}
}

func TestRemoveWarnDropsNewestEntry(t *testing.T) {
	chatID, userID := newEntriesFixture(t)
	for _, reason := range []string{"first", "second"} {
		if _, _, err := WarnUser(userID, chatID, models.WarnEntry{Reason: reason, WarnedBy: 77}); err != nil {
			t.Fatalf("WarnUser(%s) error = %v", reason, err)
		}
	}

	if removed, err := RemoveWarn(userID, chatID); err != nil || !removed {
		t.Fatalf("RemoveWarn() = %v, %v; want true, nil", removed, err)
	}

	if got := reasonsOf(storedEntries(t, chatID, userID)); len(got) != 1 || got[0] != "first" {
		t.Fatalf("stored entries = %v, want [first]", got)
	}
	if _, listed := GetWarns(userID, chatID); len(listed) != 1 || listed[0].Reason != "first" {
		t.Fatalf("listed entries = %+v, want [first]", listed)
	}
}

func TestResetUserWarnsDeletesEntries(t *testing.T) {
	chatID, userID := newEntriesFixture(t)
	if _, _, err := WarnUser(userID, chatID, models.WarnEntry{Reason: "gone"}); err != nil {
		t.Fatalf("WarnUser() error = %v", err)
	}

	if ok, err := ResetUserWarns(userID, chatID); err != nil || !ok {
		t.Fatalf("ResetUserWarns() = %v, %v; want true, nil", ok, err)
	}

	if got := storedEntries(t, chatID, userID); len(got) != 0 {
		t.Fatalf("stored entries = %+v, want none", got)
	}
}

func TestResetAllChatWarnsDeletesEntries(t *testing.T) {
	chatID, userID := newEntriesFixture(t)
	for _, uid := range []int64{userID, userID + 1} {
		if _, _, err := WarnUser(uid, chatID, models.WarnEntry{Reason: "gone"}); err != nil {
			t.Fatalf("WarnUser(%d) error = %v", uid, err)
		}
	}

	if err := ResetAllChatWarns(chatID); err != nil {
		t.Fatalf("ResetAllChatWarns() error = %v", err)
	}

	var count int64
	if err := db.DB.Model(&models.WarnEntry{}).Where("chat_id = ?", chatID).Count(&count).Error; err != nil {
		t.Fatalf("count warn entries: %v", err)
	}
	if count != 0 {
		t.Fatalf("warn entries left = %d, want 0", count)
	}
}

func TestWarnUserDropsEntriesLeftWithoutWarnRow(t *testing.T) {
	chatID, userID := newEntriesFixture(t)
	// An older bot deletes only the warns_users row, leaving its entries behind.
	if err := db.DB.Create(&models.WarnEntry{UserId: userID, ChatId: chatID, Reason: "stale"}).Error; err != nil {
		t.Fatalf("create stale entry: %v", err)
	}

	_, entries, err := WarnUser(userID, chatID, models.WarnEntry{Reason: "fresh"})
	if err != nil {
		t.Fatalf("WarnUser() error = %v", err)
	}

	if got := reasonsOf(entries); len(got) != 1 || got[0] != "fresh" {
		t.Fatalf("entries = %v, want [fresh]", got)
	}
	if got := reasonsOf(storedEntries(t, chatID, userID)); len(got) != 1 || got[0] != "fresh" {
		t.Fatalf("stored entries = %v, want [fresh]", got)
	}
}

func TestGetWarnsFallsBackToLegacyReasons(t *testing.T) {
	chatID, userID := newEntriesFixture(t)
	if err := db.DB.Create(&models.Warns{
		UserId: userID, ChatId: chatID, NumWarns: 2, Reasons: models.StringArray{"old one", "old two"},
	}).Error; err != nil {
		t.Fatalf("create legacy warns: %v", err)
	}

	num, entries := GetWarns(userID, chatID)

	if num != 2 || len(entries) != 2 || entries[0].Reason != "old one" || entries[1].Reason != "old two" {
		t.Fatalf("GetWarns() = %d, %+v; want both legacy reasons", num, entries)
	}
	for _, e := range entries {
		if e.WarnedBy != 0 || e.WarnedAt != nil || e.Anonymous {
			t.Fatalf("legacy entry = %+v, want no issuer and no time", e)
		}
	}
}

func TestGetWarnsReloadsLegacyCacheShape(t *testing.T) {
	chatID, userID := newEntriesFixture(t)
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() error = %v", err)
	}
	t.Cleanup(mr.Close)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	prevMarshal, prevManager, prevClient := cache.GetCacheState()
	m := marshaler.New(gocache.New[any](gocache_store.NewRedis(client)))
	cache.SetCacheState(m, gocache.New[any](gocache_store.NewRedis(client)), client)
	t.Cleanup(func() { cache.SetCacheState(prevMarshal, prevManager, prevClient) })
	dbcache.ResetLocalForTest()

	if _, _, err := WarnUser(userID, chatID, models.WarnEntry{Reason: "fresh", WarnedBy: 77}); err != nil {
		t.Fatalf("WarnUser() error = %v", err)
	}
	// What a release before warn entries left in Redis.
	legacy := struct {
		NumWarns int
		Reasons  []string
	}{NumWarns: 1, Reasons: []string{"cached"}}
	if err := m.Set(cache.Context, dbcache.CacheKey("warns", userID, chatID), legacy); err != nil {
		t.Fatalf("seed legacy cache: %v", err)
	}
	dbcache.ResetLocalForTest()

	_, entries := GetWarns(userID, chatID)

	if len(entries) != 1 || entries[0].Reason != "fresh" || entries[0].WarnedBy != 77 {
		t.Fatalf("GetWarns() entries = %+v, want the stored entry, not the legacy cache", entries)
	}
}

// oldReleaseWrite stores warns_users the way a release before warn entries
// does: num_warns and reasons only, entries untouched.
func oldReleaseWrite(t *testing.T, chatID, userID int64, reasons ...string) {
	t.Helper()
	var row models.Warns
	if err := db.DB.Where("chat_id = ? AND user_id = ?", chatID, userID).Take(&row).Error; err != nil {
		t.Fatalf("load warns row: %v", err)
	}
	row.NumWarns = len(reasons)
	row.Reasons = models.StringArray(reasons)
	if err := db.DB.Save(&row).Error; err != nil {
		t.Fatalf("old release write: %v", err)
	}
	dbcache.DeleteCache(dbcache.CacheKey("warns", userID, chatID))
}

func checkIssued(t *testing.T, what string, got []models.WarnEntry, want []string, issuedBy []int64) {
	t.Helper()
	if r := reasonsOf(got); len(r) != len(want) {
		t.Fatalf("%s reasons = %v, want %v", what, r, want)
	}
	for i, e := range got {
		if e.Reason != want[i] || e.WarnedBy != issuedBy[i] || (e.WarnedAt == nil) != (issuedBy[i] == 0) {
			t.Fatalf("%s entry %d = %+v, want %q issued by %d", what, i, e, want[i], issuedBy[i])
		}
	}
}

func TestEntriesFollowReasonsAfterOldReleaseWrites(t *testing.T) {
	chatID, userID := newEntriesFixture(t)
	for _, reason := range []string{"a", "b"} {
		if _, _, err := WarnUser(userID, chatID, models.WarnEntry{Reason: reason, WarnedBy: 77}); err != nil {
			t.Fatalf("WarnUser(%s) error = %v", reason, err)
		}
	}
	oldReleaseWrite(t, chatID, userID, "a")      // old /rmwarn
	oldReleaseWrite(t, chatID, userID, "a", "c") // old /warn c

	if num, listed := GetWarns(userID, chatID); num != 2 {
		t.Fatalf("GetWarns() num = %d, want 2", num)
	} else {
		checkIssued(t, "listed", listed, []string{"a", "c"}, []int64{77, 0})
	}

	_, entries, err := WarnUser(userID, chatID, models.WarnEntry{Reason: "d", WarnedBy: 78})
	if err != nil {
		t.Fatalf("WarnUser(d) error = %v", err)
	}
	checkIssued(t, "returned", entries, []string{"a", "c", "d"}, []int64{77, 0, 78})
	checkIssued(t, "stored", storedEntries(t, chatID, userID), []string{"a", "c", "d"}, []int64{77, 0, 78})

	if removed, err := RemoveWarn(userID, chatID); err != nil || !removed {
		t.Fatalf("RemoveWarn() = %v, %v; want true, nil", removed, err)
	}
	checkIssued(t, "stored after rmwarn", storedEntries(t, chatID, userID), []string{"a", "c"}, []int64{77, 0})
}

func TestRemoveWarnDropsTheEntryOfThePoppedReason(t *testing.T) {
	chatID, userID := newEntriesFixture(t)
	for _, reason := range []string{"a", "b"} {
		if _, _, err := WarnUser(userID, chatID, models.WarnEntry{Reason: reason, WarnedBy: 77}); err != nil {
			t.Fatalf("WarnUser(%s) error = %v", reason, err)
		}
	}
	oldReleaseWrite(t, chatID, userID, "a", "b", "c") // old /warn c, no entry

	if removed, err := RemoveWarn(userID, chatID); err != nil || !removed {
		t.Fatalf("RemoveWarn() = %v, %v; want true, nil", removed, err)
	}

	checkIssued(t, "stored", storedEntries(t, chatID, userID), []string{"a", "b"}, []int64{77, 77})
}

func TestEntriesFollowReasonsAfterOldReleaseReset(t *testing.T) {
	chatID, userID := newEntriesFixture(t)
	for _, reason := range []string{"a", "b", "c"} {
		if _, _, err := WarnUser(userID, chatID, models.WarnEntry{Reason: reason, WarnedBy: 77}); err != nil {
			t.Fatalf("WarnUser(%s) error = %v", reason, err)
		}
	}
	// Old /resetwarns deletes warns_users alone; old /warn spam recreates it.
	oldReleaseWrite(t, chatID, userID, "spam")

	if num, listed := GetWarns(userID, chatID); num != 1 {
		t.Fatalf("GetWarns() num = %d, want 1", num)
	} else {
		checkIssued(t, "listed", listed, []string{"spam"}, []int64{0})
	}

	_, entries, err := WarnUser(userID, chatID, models.WarnEntry{Reason: "e", WarnedBy: 78})
	if err != nil {
		t.Fatalf("WarnUser(e) error = %v", err)
	}
	checkIssued(t, "returned", entries, []string{"spam", "e"}, []int64{0, 78})
	checkIssued(t, "stored", storedEntries(t, chatID, userID), []string{"spam", "e"}, []int64{0, 78})
}

// The resets take the chat-row lock WarnUser holds, so a user's first warn,
// whose warns_users row is still an uncommitted INSERT, cannot commit between
// their two DELETEs and leave its entry deleted under a surviving warn row.
func TestResetsWaitForTheChatLock(t *testing.T) {
	if db.DB.Name() != "postgres" {
		t.Skip("row locks need Postgres")
	}
	resets := map[string]func(chatID, userID int64) error{
		"ResetUserWarns": func(chatID, userID int64) error {
			_, err := ResetUserWarns(userID, chatID)
			return err
		},
		"ResetAllChatWarns": func(chatID, _ int64) error { return ResetAllChatWarns(chatID) },
	}
	for name, reset := range resets {
		t.Run(name, func(t *testing.T) {
			chatID, userID := newEntriesFixture(t)
			if _, _, err := WarnUser(userID, chatID, models.WarnEntry{Reason: "a"}); err != nil {
				t.Fatalf("WarnUser() error = %v", err)
			}
			holder := db.DB.Begin()
			var chat models.Chat
			if err := holder.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("chat_id = ?", chatID).Take(&chat).Error; err != nil {
				holder.Rollback()
				t.Fatalf("lock chat row: %v", err)
			}

			done := make(chan error, 1)
			go func() { done <- reset(chatID, userID) }()
			select {
			case err := <-done:
				holder.Rollback()
				t.Fatalf("%s returned (%v) while the chat row was locked", name, err)
			case <-time.After(300 * time.Millisecond):
			}
			holder.Rollback()
			select {
			case err := <-done:
				if err != nil {
					t.Fatalf("%s error = %v", name, err)
				}
			case <-time.After(5 * time.Second):
				t.Fatalf("%s did not finish after the lock was released", name)
			}
		})
	}
}

func TestWarnUserBackfillsLegacyReasons(t *testing.T) {
	chatID, userID := newEntriesFixture(t)
	if err := db.DB.Create(&models.Warns{
		UserId: userID, ChatId: chatID, NumWarns: 2, Reasons: models.StringArray{"old one", "old two"},
	}).Error; err != nil {
		t.Fatalf("create legacy warns: %v", err)
	}

	_, entries, err := WarnUser(userID, chatID, models.WarnEntry{Reason: "fresh", WarnedBy: 77})
	if err != nil {
		t.Fatalf("WarnUser() error = %v", err)
	}

	want, by := []string{"old one", "old two", "fresh"}, []int64{0, 0, 77}
	checkIssued(t, "returned", entries, want, by)
	checkIssued(t, "stored", storedEntries(t, chatID, userID), want, by)
}
