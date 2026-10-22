//go:build testtools

package chatstats

import (
	"testing"

	"github.com/divkix/Alita_Robot/alita/db/models"
)

func mustAddMembers(t *testing.T, rows ...models.ChatMemberDailyStat) {
	t.Helper()
	if err := Add(Batch{Members: rows}); err != nil {
		t.Fatalf("Add(members %v) error = %v", rows, err)
	}
}

func TestTopMembersSumsDaysRanksAndLimits(t *testing.T) {
	chatID := testChatID(t)
	otherChatID := chatID + 1

	// Two flushes of the same day add up; another chat and a day outside the
	// range do not count.
	mustAddMembers(t,
		models.ChatMemberDailyStat{ChatID: chatID, Day: "2026-10-20", UserID: 1, Messages: 2},
		models.ChatMemberDailyStat{ChatID: chatID, Day: "2026-10-20", UserID: 2, Messages: 5},
		models.ChatMemberDailyStat{ChatID: chatID, Day: "2026-10-20", UserID: 3, Messages: 4},
		models.ChatMemberDailyStat{ChatID: chatID, Day: "2026-10-01", UserID: 3, Messages: 100},
		models.ChatMemberDailyStat{ChatID: otherChatID, Day: "2026-10-20", UserID: 1, Messages: 50},
	)
	mustAddMembers(t,
		models.ChatMemberDailyStat{ChatID: chatID, Day: "2026-10-20", UserID: 1, Messages: 1},
		models.ChatMemberDailyStat{ChatID: chatID, Day: "2026-10-21", UserID: 1, Messages: 2},
	)

	top, err := TopMembers(chatID, "2026-10-15", "2026-10-21", 2)
	if err != nil {
		t.Fatalf("TopMembers() error = %v", err)
	}
	want := []MemberTotal{{UserID: 1, Messages: 5}, {UserID: 2, Messages: 5}}
	if len(top) != len(want) || top[0] != want[0] || top[1] != want[1] {
		t.Fatalf("top = %+v, want %+v (summed, ties by user ID, limited to 2)", top, want)
	}
}

func TestDeleteMemberDailyBeforeKeepsRowsInsideRetention(t *testing.T) {
	chatID := testChatID(t)
	mustAddMembers(t,
		models.ChatMemberDailyStat{ChatID: chatID, Day: "2026-07-01", UserID: 1, Messages: 9},
		models.ChatMemberDailyStat{ChatID: chatID, Day: "2026-10-20", UserID: 1, Messages: 1},
	)

	if _, err := DeleteMemberDailyBefore("2026-07-24"); err != nil {
		t.Fatalf("DeleteMemberDailyBefore() error = %v", err)
	}

	top, err := TopMembers(chatID, "2026-01-01", "2026-12-31", 10)
	if err != nil {
		t.Fatalf("TopMembers() error = %v", err)
	}
	if len(top) != 1 || top[0].Messages != 1 {
		t.Fatalf("top after the sweep = %+v, want user 1 with only the kept day's message", top)
	}
}
