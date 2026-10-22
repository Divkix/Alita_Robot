package bans

import (
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

var lastBanChat atomic.Int64

// newBanChat returns a chat id no other test uses and removes its rows when
// the test ends.
func newBanChat(t *testing.T) int64 {
	t.Helper()
	lastBanChat.CompareAndSwap(0, -time.Now().UnixNano())
	chatID := lastBanChat.Add(-1)
	t.Cleanup(func() {
		if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.BanRecord{}).Error; err != nil {
			t.Errorf("delete ban records of chat %d: %v", chatID, err)
		}
	})
	return chatID
}

func record(t *testing.T, r models.BanRecord) int64 {
	t.Helper()
	id, err := Record(r)
	if err != nil {
		t.Fatalf("Record(%+v) error = %v", r, err)
	}
	if id == 0 {
		t.Fatalf("Record(%+v) returned id 0", r)
	}
	return id
}

func rowsOf(t *testing.T, chatID int64) []models.BanRecord {
	t.Helper()
	var rows []models.BanRecord
	if err := db.DB.Where("chat_id = ?", chatID).Order("id").Find(&rows).Error; err != nil {
		t.Fatalf("load ban records of chat %d: %v", chatID, err)
	}
	return rows
}

func TestRecordStoresTheBan(t *testing.T) {
	chatID := newBanChat(t)
	until := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	id := record(t, models.BanRecord{
		ChatId: chatID, UserId: 42, BannedBy: 7, Reason: "spam",
		Source: models.BanSourceManual, Until: &until,
	})

	got, err := Latest(chatID, 42)
	if err != nil || got == nil {
		t.Fatalf("Latest() = %v, %v; want the record", got, err)
	}
	if got.ID != id || got.BannedBy != 7 || got.Reason != "spam" || got.Source != models.BanSourceManual {
		t.Fatalf("Latest() = %+v, want id %d by 7 for spam from manual", got, id)
	}
	if got.Until == nil || !got.Until.Equal(until) {
		t.Fatalf("Until = %v, want %v", got.Until, until)
	}
	if !got.IsOpen(time.Now()) {
		t.Fatal("a new temporary ban is not open")
	}
	if got.CreatedAt.IsZero() {
		t.Fatal("record has no time")
	}
}

func TestRepeatBanUpdatesTheOpenRow(t *testing.T) {
	chatID := newBanChat(t)
	first := record(t, models.BanRecord{ChatId: chatID, UserId: 42, BannedBy: 7, Reason: "spam", Source: models.BanSourceManual})
	second := record(t, models.BanRecord{ChatId: chatID, UserId: 42, BannedBy: 0, Reason: "flood", Source: models.BanSourceAntiflood})

	if first != second {
		t.Fatalf("repeat ban got id %d, want the open row %d", second, first)
	}
	rows := rowsOf(t, chatID)
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if rows[0].BannedBy != 0 || rows[0].Reason != "flood" || rows[0].Source != models.BanSourceAntiflood {
		t.Fatalf("row = %+v, want the repeat ban's values", rows[0])
	}
}

func TestBanAfterLiftMakesANewRow(t *testing.T) {
	chatID := newBanChat(t)
	first := record(t, models.BanRecord{ChatId: chatID, UserId: 42, BannedBy: 7, Source: models.BanSourceManual})
	lifted, err := Lift(chatID, 42, 8)
	if err != nil || !lifted {
		t.Fatalf("Lift() = %v, %v; want true", lifted, err)
	}
	second := record(t, models.BanRecord{ChatId: chatID, UserId: 42, BannedBy: 9, Source: models.BanSourceManual})
	if second == first {
		t.Fatal("ban after lift reused the lifted row")
	}

	rows := rowsOf(t, chatID)
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if rows[0].LiftedAt == nil || rows[0].LiftedBy == nil || *rows[0].LiftedBy != 8 {
		t.Fatalf("first row = %+v, want lifted by 8", rows[0])
	}
	latest, err := Latest(chatID, 42)
	if err != nil || latest == nil || latest.ID != second {
		t.Fatalf("Latest() = %+v, %v; want row %d", latest, err, second)
	}
}

func TestLiftWithoutAnOpenBanClosesNothing(t *testing.T) {
	chatID := newBanChat(t)
	lifted, err := Lift(chatID, 42, 8)
	if err != nil || lifted {
		t.Fatalf("Lift() = %v, %v; want false", lifted, err)
	}
}

func TestLiftLeavesAnExpiredTemporaryBan(t *testing.T) {
	chatID := newBanChat(t)
	ended := time.Now().Add(-time.Minute)
	record(t, models.BanRecord{ChatId: chatID, UserId: 42, Source: models.BanSourceAntiflood, Until: &ended})

	lifted, err := Lift(chatID, 42, 8)
	if err != nil || lifted {
		t.Fatalf("Lift() = %v, %v; want false for an expired ban", lifted, err)
	}
	got, err := Latest(chatID, 42)
	if err != nil || got == nil || got.LiftedAt != nil || got.IsOpen(time.Now()) {
		t.Fatalf("Latest() = %+v, %v; want an expired, unlifted row", got, err)
	}
}

func TestLiftUsersClosesOnlyTheNamedBans(t *testing.T) {
	chatID := newBanChat(t)
	for _, userID := range []int64{1, 2, 3} {
		record(t, models.BanRecord{ChatId: chatID, UserId: userID, Source: models.BanSourceManual})
	}
	closed, err := LiftUsers(chatID, []int64{1, 3, 4}, 8)
	if err != nil || closed != 2 {
		t.Fatalf("LiftUsers() = %d, %v; want 2", closed, err)
	}
	if got, err := Latest(chatID, 2); err != nil || got == nil || got.LiftedAt != nil {
		t.Fatalf("user 2 = %+v, %v; want still open", got, err)
	}
}

func TestLatestWithoutRecordIsNil(t *testing.T) {
	chatID := newBanChat(t)
	got, err := Latest(chatID, 42)
	if err != nil || got != nil {
		t.Fatalf("Latest() = %+v, %v; want nil, nil", got, err)
	}
}

func TestRecordCutsLongText(t *testing.T) {
	chatID := newBanChat(t)
	record(t, models.BanRecord{
		ChatId: chatID, UserId: 42, Source: models.BanSourceFed,
		Reason:       strings.Repeat("é", models.MaxModerationReasonRunes+10),
		SourceDetail: strings.Repeat("f", models.MaxBanSourceDetailRunes+10),
	})
	got, err := Latest(chatID, 42)
	if err != nil || got == nil {
		t.Fatalf("Latest() = %v, %v", got, err)
	}
	if n := utf8.RuneCountInString(got.Reason); n != models.MaxModerationReasonRunes {
		t.Fatalf("reason runes = %d, want %d", n, models.MaxModerationReasonRunes)
	}
	if n := utf8.RuneCountInString(got.SourceDetail); n != models.MaxBanSourceDetailRunes {
		t.Fatalf("detail runes = %d, want %d", n, models.MaxBanSourceDetailRunes)
	}
}

func TestDeleteExpiredBeforeKeepsOpenPermanentBans(t *testing.T) {
	chatID := newBanChat(t)
	now := time.Now()
	old := now.Add(-100 * 24 * time.Hour)
	recent := now.Add(-time.Hour)
	cutoff := now.Add(-90 * 24 * time.Hour)

	// An open permanent ban made long ago stays.
	record(t, models.BanRecord{ChatId: chatID, UserId: 1, Source: models.BanSourceManual, CreatedAt: old})
	// A temporary ban that ended long ago goes; one that ended lately stays.
	record(t, models.BanRecord{ChatId: chatID, UserId: 2, Source: models.BanSourceManual, CreatedAt: old, Until: &old})
	record(t, models.BanRecord{ChatId: chatID, UserId: 3, Source: models.BanSourceManual, CreatedAt: old, Until: &recent})
	// A ban lifted long ago goes; one lifted lately stays.
	record(t, models.BanRecord{ChatId: chatID, UserId: 4, Source: models.BanSourceManual, CreatedAt: old})
	record(t, models.BanRecord{ChatId: chatID, UserId: 5, Source: models.BanSourceManual, CreatedAt: old})
	if err := db.DB.Model(&models.BanRecord{}).Where("chat_id = ? AND user_id = ?", chatID, 4).
		Updates(map[string]any{"lifted_at": old.UTC(), "lifted_by": 8}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := Lift(chatID, 5, 8); err != nil {
		t.Fatal(err)
	}

	if _, err := DeleteExpiredBefore(cutoff); err != nil {
		t.Fatalf("DeleteExpiredBefore() error = %v", err)
	}
	var kept []int64
	for _, r := range rowsOf(t, chatID) {
		kept = append(kept, r.UserId)
	}
	if want := []int64{1, 3, 5}; !slices.Equal(kept, want) {
		t.Fatalf("kept users %v, want %v", kept, want)
	}
}

func TestRecordRejectsUnknownSource(t *testing.T) {
	chatID := newBanChat(t)
	if _, err := Record(models.BanRecord{ChatId: chatID, UserId: 42, Source: "telegram"}); err == nil {
		t.Fatal("Record() with an unknown source succeeded")
	}
}
