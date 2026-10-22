//go:build testtools

package chatstats

import (
	"slices"
	"testing"

	"github.com/divkix/Alita_Robot/alita/db/models"
)

func TestClaimMemberCountHasOneWinnerPerChatAndDay(t *testing.T) {
	chatID := testChatID(t)

	first, err := ClaimMemberCount(chatID, "2026-10-22")
	if err != nil {
		t.Fatalf("first ClaimMemberCount() error = %v", err)
	}
	second, err := ClaimMemberCount(chatID, "2026-10-22")
	if err != nil {
		t.Fatalf("second ClaimMemberCount() error = %v", err)
	}
	if !first || second {
		t.Fatalf("claims = %v, %v, want the first to win and the second to lose", first, second)
	}
	nextDay, err := ClaimMemberCount(chatID, "2026-10-23")
	if err != nil || !nextDay {
		t.Fatalf("claim for the next day = %v, %v, want a win", nextDay, err)
	}
}

func TestMemberCountClaimReleaseAndRead(t *testing.T) {
	chatID := testChatID(t)

	if _, err := ClaimMemberCount(chatID, "2026-10-21"); err != nil {
		t.Fatalf("ClaimMemberCount() error = %v", err)
	}
	if err := SetMemberCount(chatID, "2026-10-21", 120); err != nil {
		t.Fatalf("SetMemberCount() error = %v", err)
	}
	// A written count is final: releasing it does nothing and writing again fails.
	if err := ReleaseMemberCount(chatID, "2026-10-21"); err != nil {
		t.Fatalf("ReleaseMemberCount() error = %v", err)
	}
	if err := SetMemberCount(chatID, "2026-10-21", 999); err == nil {
		t.Fatal("SetMemberCount() over a written count = nil, want an error")
	}

	// A claim without a count is not a point of the series.
	if _, err := ClaimMemberCount(chatID, "2026-10-22"); err != nil {
		t.Fatalf("ClaimMemberCount() error = %v", err)
	}
	rows, err := GetMemberCounts(chatID, "2026-10-01", "2026-10-31")
	if err != nil {
		t.Fatalf("GetMemberCounts() error = %v", err)
	}
	if len(rows) != 1 || rows[0].Day != "2026-10-21" || rows[0].Count == nil || *rows[0].Count != 120 {
		t.Fatalf("rows = %+v, want only 2026-10-21 with 120", rows)
	}

	// A released claim can be claimed again.
	if err := ReleaseMemberCount(chatID, "2026-10-22"); err != nil {
		t.Fatalf("ReleaseMemberCount() error = %v", err)
	}
	again, err := ClaimMemberCount(chatID, "2026-10-22")
	if err != nil || !again {
		t.Fatalf("claim after release = %v, %v, want a win", again, err)
	}
}

func TestListChatsDueMemberCountSkipsIdleAndClaimedChats(t *testing.T) {
	active := testChatID(t)
	yesterday := active + 1
	stale := active + 2
	idle := active + 3
	claimed := active + 4
	mustAddDaily(t,
		models.ChatDailyStat{ChatID: active, Day: "2026-10-22", Messages: 1},
		models.ChatDailyStat{ChatID: yesterday, Day: "2026-10-21", Joins: 1},
		models.ChatDailyStat{ChatID: stale, Day: "2026-10-19", Messages: 5},
		models.ChatDailyStat{ChatID: idle, Day: "2026-10-22"},
		models.ChatDailyStat{ChatID: claimed, Day: "2026-10-22", Leaves: 1},
	)
	if _, err := ClaimMemberCount(claimed, "2026-10-22"); err != nil {
		t.Fatalf("ClaimMemberCount() error = %v", err)
	}

	due, err := ListChatsDueMemberCount("2026-10-21", "2026-10-22", 10_000)
	if err != nil {
		t.Fatalf("ListChatsDueMemberCount() error = %v", err)
	}
	for _, want := range []int64{active, yesterday} {
		if !slices.Contains(due, want) {
			t.Fatalf("due = %v, want it to include %d", due, want)
		}
	}
	for _, unwanted := range []int64{stale, idle, claimed} {
		if slices.Contains(due, unwanted) {
			t.Fatalf("due = %v, want it to exclude %d", due, unwanted)
		}
	}
	if !slices.IsSorted(due) {
		t.Fatalf("due = %v, want ascending chat IDs", due)
	}
}
