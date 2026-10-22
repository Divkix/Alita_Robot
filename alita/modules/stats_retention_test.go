//go:build testtools

package modules

import (
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/db/chatstats"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/utils/stats"
)

func TestActivityRetentionSweepsHourlyRowsOlderThan90Days(t *testing.T) {
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	chatID := uniqueModuleChatID()
	old := stats.Day(now.AddDate(0, 0, -(activityStatsRetentionDays + 1)))
	kept := stats.Day(now.AddDate(0, 0, -(activityStatsRetentionDays - 1)))
	if err := chatstats.Add(chatstats.Batch{Hourly: []models.ChatHourlyStat{
		{ChatID: chatID, Day: old, Hour: 3, Messages: 1},
		{ChatID: chatID, Day: kept, Hour: 3, Messages: 1},
	}}); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	sweepExpiredActivityStats(now)

	rows, err := chatstats.GetHourlyRange(chatID, old, stats.Day(now))
	if err != nil {
		t.Fatalf("GetHourlyRange() error = %v", err)
	}
	if len(rows) != 1 || rows[0].Day != kept {
		t.Fatalf("rows = %v, want the %s row only", rows, kept)
	}
}

func TestActivityRetentionSweepsMemberRowsOlderThan90Days(t *testing.T) {
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	chatID := uniqueModuleChatID()
	old := stats.Day(now.AddDate(0, 0, -(activityStatsRetentionDays + 1)))
	kept := stats.Day(now.AddDate(0, 0, -(activityStatsRetentionDays - 1)))
	if err := chatstats.Add(chatstats.Batch{Members: []models.ChatMemberDailyStat{
		{ChatID: chatID, Day: old, UserID: 7, Messages: 10},
		{ChatID: chatID, Day: kept, UserID: 7, Messages: 1},
	}}); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	sweepExpiredActivityStats(now)

	top, err := chatstats.TopMembers(chatID, old, stats.Day(now), 10)
	if err != nil {
		t.Fatalf("TopMembers() error = %v", err)
	}
	if len(top) != 1 || top[0].Messages != 1 {
		t.Fatalf("top = %+v, want user 7 with only the kept day's message", top)
	}
}

func TestActivityRetentionKeepsMemberCountsFor400Days(t *testing.T) {
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	chatID := uniqueModuleChatID()
	old := stats.Day(now.AddDate(0, 0, -(digestStatsRetentionDays + 1)))
	kept := stats.Day(now.AddDate(0, 0, -(activityStatsRetentionDays + 1)))
	for _, day := range []string{old, kept} {
		if _, err := chatstats.ClaimMemberCount(chatID, day); err != nil {
			t.Fatalf("ClaimMemberCount(%s) error = %v", day, err)
		}
		if err := chatstats.SetMemberCount(chatID, day, 10); err != nil {
			t.Fatalf("SetMemberCount(%s) error = %v", day, err)
		}
	}

	sweepExpiredActivityStats(now)

	rows, err := chatstats.GetMemberCounts(chatID, old, stats.Day(now))
	if err != nil {
		t.Fatalf("GetMemberCounts() error = %v", err)
	}
	if len(rows) != 1 || rows[0].Day != kept {
		t.Fatalf("rows = %+v, want only %s (older than 90 days, inside 400)", rows, kept)
	}
}
