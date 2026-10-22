//go:build testtools

package chatstats

import (
	"testing"

	"github.com/divkix/Alita_Robot/alita/db/models"
)

func mustGetHourlyRange(t *testing.T, chatID int64, from, to string) []models.ChatHourlyStat {
	t.Helper()
	rows, err := GetHourlyRange(chatID, from, to)
	if err != nil {
		t.Fatalf("GetHourlyRange(%d, %q, %q) error = %v", chatID, from, to, err)
	}
	return rows
}

func TestAddSumsHourlyRowsAcrossReplicaFlushes(t *testing.T) {
	chatID := testChatID(t)

	// Each call stands in for one replica flushing the same hours.
	if err := Add(Batch{Hourly: []models.ChatHourlyStat{
		{ChatID: chatID, Day: "2026-10-21", Hour: 9, Messages: 2},
		{ChatID: chatID, Day: "2026-10-21", Hour: 23, Messages: 1},
	}}); err != nil {
		t.Fatalf("first Add() error = %v", err)
	}
	if err := Add(Batch{Hourly: []models.ChatHourlyStat{
		{ChatID: chatID, Day: "2026-10-21", Hour: 9, Messages: 3},
		{ChatID: chatID, Day: "2026-10-22", Hour: 0, Messages: 4},
	}}); err != nil {
		t.Fatalf("second Add() error = %v", err)
	}

	rows := mustGetHourlyRange(t, chatID, "2026-10-21", "2026-10-22")
	want := []models.ChatHourlyStat{
		{ChatID: chatID, Day: "2026-10-21", Hour: 9, Messages: 5},
		{ChatID: chatID, Day: "2026-10-21", Hour: 23, Messages: 1},
		{ChatID: chatID, Day: "2026-10-22", Hour: 0, Messages: 4},
	}
	if len(rows) != len(want) {
		t.Fatalf("rows = %v, want %v", rows, want)
	}
	for i := range want {
		if rows[i] != want[i] {
			t.Fatalf("row %d = %+v, want %+v (oldest first, summed)", i, rows[i], want[i])
		}
	}
}

func TestAddWritesNothingWhenAnyTableFails(t *testing.T) {
	chatID := testChatID(t)

	// Hour 24 breaks the hour check, after the daily rows are already
	// inserted in the same transaction.
	err := Add(Batch{
		Daily:  []models.ChatDailyStat{{ChatID: chatID, Day: "2026-10-21", Messages: 1}},
		Hourly: []models.ChatHourlyStat{{ChatID: chatID, Day: "2026-10-21", Hour: 24, Messages: 1}},
	})
	if err == nil {
		t.Fatal("Add() with an invalid hour = nil, want an error")
	}

	if rows := mustGetRange(t, chatID, "2026-10-21", "2026-10-21"); len(rows) != 0 {
		t.Fatalf("daily rows after a failed batch = %v, want none (rolled back)", rows)
	}
}

func TestDeleteHourlyBeforeKeepsRowsInsideRetention(t *testing.T) {
	chatID := testChatID(t)
	if err := Add(Batch{Hourly: []models.ChatHourlyStat{
		{ChatID: chatID, Day: "2026-07-01", Hour: 5, Messages: 1},
		{ChatID: chatID, Day: "2026-10-20", Hour: 5, Messages: 1},
	}}); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	if _, err := DeleteHourlyBefore("2026-07-24"); err != nil {
		t.Fatalf("DeleteHourlyBefore() error = %v", err)
	}

	rows := mustGetHourlyRange(t, chatID, "2026-01-01", "2026-12-31")
	if len(rows) != 1 || rows[0].Day != "2026-10-20" {
		t.Fatalf("remaining rows = %v, want only 2026-10-20", rows)
	}
}
