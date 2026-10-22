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
