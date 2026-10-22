package reports

import (
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

func TestCountOpenReportersCountsEachMemberOnce(t *testing.T) {
	chatID := newReportCaseChat(t)
	for _, r := range []models.Report{
		{ReportedMessageId: 505, ReporterId: 1},
		{ReportedMessageId: 506, ReporterId: 1},
		{ReportedMessageId: 505, ReporterId: 2},
		{ReportedMessageId: 507, ReporterId: 3},
	} {
		r.ChatId, r.ReportedUserId = chatID, 43
		mustCreateReport(t, &r)
	}
	// Another target's report does not count.
	mustCreateReport(t, &models.Report{ChatId: chatID, ReportedUserId: 44, ReportedMessageId: 600, ReporterId: 4})

	if n, err := CountOpenReporters(chatID, 43); err != nil || n != 3 {
		t.Fatalf("CountOpenReporters() = %d, %v, want 3", n, err)
	}
	if _, err := Resolve(chatID, 507, models.ReportStatusDismissed, 9); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if n, err := CountOpenReporters(chatID, 43); err != nil || n != 2 {
		t.Fatalf("CountOpenReporters() after a dismissal = %d, %v, want 2", n, err)
	}
}

func TestAutoMuteClaimAndRelease(t *testing.T) {
	chatID := newReportCaseChat(t)
	t.Cleanup(func() { _ = db.DB.Where("chat_id = ?", chatID).Delete(&models.ReportAutoMute{}).Error })
	until := time.Now().Add(24 * time.Hour).Truncate(time.Second)

	if ok, err := ClaimAutoMute(chatID, 43, until); err != nil || !ok {
		t.Fatalf("first ClaimAutoMute() = %v, %v, want true", ok, err)
	}
	if ok, err := ClaimAutoMute(chatID, 43, until.Add(time.Hour)); err != nil || ok {
		t.Fatalf("second ClaimAutoMute() = %v, %v, want false", ok, err)
	}
	m, err := ReleaseAutoMute(chatID, 43)
	if err != nil || m == nil || !m.MutedUntil.Equal(until) {
		t.Fatalf("ReleaseAutoMute() = %+v, %v, want the mute until %v", m, err, until)
	}
	if m, err := ReleaseAutoMute(chatID, 43); err != nil || m != nil {
		t.Fatalf("second ReleaseAutoMute() = %+v, %v, want nil", m, err)
	}
}

func TestDeleteAutoMutesBefore(t *testing.T) {
	chatID := newReportCaseChat(t)
	t.Cleanup(func() { _ = db.DB.Where("chat_id = ?", chatID).Delete(&models.ReportAutoMute{}).Error })
	now := time.Now()
	if _, err := ClaimAutoMute(chatID, 1, now.Add(-100*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := ClaimAutoMute(chatID, 2, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := DeleteAutoMutesBefore(now.Add(-90 * 24 * time.Hour)); err != nil {
		t.Fatalf("DeleteAutoMutesBefore() error = %v", err)
	}
	if m, _ := GetAutoMute(chatID, 1); m != nil {
		t.Fatalf("old auto-mute kept: %+v", m)
	}
	if m, _ := GetAutoMute(chatID, 2); m == nil {
		t.Fatal("current auto-mute removed")
	}
}

func TestSetChatReportMuteThresholdRejectsOutOfRange(t *testing.T) {
	chatID := newReportCaseChat(t)
	t.Cleanup(func() { _ = db.DB.Where("chat_id = ?", chatID).Delete(&models.ReportChatSettings{}).Error })
	for _, n := range []int{1, 21, -1} {
		if err := SetChatReportMuteThreshold(chatID, n); err == nil {
			t.Fatalf("SetChatReportMuteThreshold(%d) error = nil, want out of range", n)
		}
	}
	if err := SetChatReportMuteThreshold(chatID, 5); err != nil {
		t.Fatalf("SetChatReportMuteThreshold(5) error = %v", err)
	}
	if got := GetChatReportSettings(chatID).MuteThreshold; got != 5 {
		t.Fatalf("MuteThreshold = %d, want 5", got)
	}
	if err := SetChatReportMuteThreshold(chatID, 0); err != nil {
		t.Fatalf("SetChatReportMuteThreshold(0) error = %v", err)
	}
	if got := GetChatReportSettings(chatID).MuteThreshold; got != 0 {
		t.Fatalf("MuteThreshold = %d, want 0", got)
	}
}
