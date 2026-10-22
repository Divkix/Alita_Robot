package bans

import (
	"slices"
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

// seedUser stores a users row for id and removes it when the test ends.
func seedUser(t *testing.T, id int64, username, name string) {
	t.Helper()
	if err := db.DB.Create(&models.User{UserId: id, UserName: username, Name: name}).Error; err != nil {
		t.Fatalf("create user %d: %v", id, err)
	}
	t.Cleanup(func() {
		if err := db.DB.Where("user_id = ?", id).Delete(&models.User{}).Error; err != nil {
			t.Errorf("delete user %d: %v", id, err)
		}
	})
}

func listedUsers(t *testing.T, chatID int64, search string, page, size int) ([]int64, int64) {
	t.Helper()
	rows, total, _, err := ListOpen(chatID, search, time.Now(), page, size)
	if err != nil {
		t.Fatalf("ListOpen(%q) error = %v", search, err)
	}
	ids := make([]int64, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.UserId)
	}
	return ids, total
}

func TestListOpenListsBansInForceNewestFirst(t *testing.T) {
	chatID := newBanChat(t)
	base := time.Now().Add(-time.Hour)
	for i, userID := range []int64{1001, 1002, 1003, 1004} {
		record(t, models.BanRecord{
			ChatId: chatID, UserId: userID, Source: models.BanSourceManual,
			CreatedAt: base.Add(time.Duration(i) * time.Minute),
		})
	}
	// Lifted, and a temporary ban past its end: neither is in force.
	if _, err := Lift(chatID, 1002, 7); err != nil {
		t.Fatalf("Lift() error = %v", err)
	}
	past := time.Now().Add(-time.Minute)
	record(t, models.BanRecord{ChatId: chatID, UserId: 1005, Source: models.BanSourceManual, Until: &past})
	// Another chat's ban.
	record(t, models.BanRecord{ChatId: newBanChat(t), UserId: 1006, Source: models.BanSourceManual})

	ids, total := listedUsers(t, chatID, "", 0, 10)
	if want := []int64{1004, 1003, 1001}; !slices.Equal(ids, want) || total != 3 {
		t.Fatalf("ListOpen() = %v (total %d), want %v (total 3)", ids, total, want)
	}

	ids, total = listedUsers(t, chatID, "", 1, 1)
	if want := []int64{1003}; !slices.Equal(ids, want) || total != 3 {
		t.Fatalf("second page of one = %v (total %d), want %v (total 3)", ids, total, want)
	}

	// A page past the end is the last one.
	rows, _, page, err := ListOpen(chatID, "", time.Now(), 7, 2)
	if err != nil || page != 1 || len(rows) != 1 || rows[0].UserId != 1001 {
		t.Fatalf("ListOpen(page 7 of 2) = %v, page %d, %v; want the last page (1) with 1001", rows, page, err)
	}
}

func TestListOpenMatchesIDUsernameAndName(t *testing.T) {
	chatID := newBanChat(t)
	seedUser(t, 2001, "SpamKing", "Totally Real")
	seedUser(t, 2002, "helper", "Spam_Bot 50%")
	for _, userID := range []int64{2001, 2002, 2003} {
		record(t, models.BanRecord{ChatId: chatID, UserId: userID, Source: models.BanSourceManual})
	}

	for _, tc := range []struct {
		search string
		want   []int64
	}{
		{"@spamking", []int64{2001}},
		{" SPAMKING ", []int64{2001}},
		{"real", []int64{2001}},
		{"spam", []int64{2002, 2001}},
		{"2003", []int64{2003}},
		{"50%", []int64{2002}},
		{"m_b", []int64{2002}},
		{"nobody", nil},
	} {
		ids, total := listedUsers(t, chatID, tc.search, 0, 10)
		slices.Sort(ids)
		want := slices.Clone(tc.want)
		slices.Sort(want)
		if !slices.Equal(ids, want) || total != int64(len(want)) {
			t.Errorf("ListOpen(%q) = %v (total %d), want %v", tc.search, ids, total, want)
		}
	}
}
