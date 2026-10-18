package reports

import (
	"sync"
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

func newReportCaseChat(t *testing.T) int64 {
	t.Helper()
	skipIfNoDb(t)
	chatID := -time.Now().UnixNano()
	if err := chats.EnsureChatInDb(chatID, "reports case chat"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Report{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
	})
	return chatID
}

func mustCreateReport(t *testing.T, r *models.Report) bool {
	t.Helper()
	created, err := Create(r)
	if err != nil {
		t.Fatalf("Create(%+v) error = %v", r, err)
	}
	return created
}

func TestCreateDedupesSameReporterOnSameMessage(t *testing.T) {
	chatID := newReportCaseChat(t)

	first := &models.Report{ChatId: chatID, ReportedUserId: 43, ReportedMessageId: 505, ReporterId: 42, Reason: "spam"}
	if !mustCreateReport(t, first) {
		t.Fatal("first Create() created = false, want true")
	}
	if first.ID == 0 {
		t.Fatal("first Create() did not assign an ID")
	}
	again := &models.Report{ChatId: chatID, ReportedUserId: 43, ReportedMessageId: 505, ReporterId: 42, Reason: "again"}
	if mustCreateReport(t, again) {
		t.Fatal("duplicate Create() created = true, want false")
	}

	rows, err := OpenForMessage(chatID, 505)
	if err != nil {
		t.Fatalf("OpenForMessage() error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("open rows = %d, want 1", len(rows))
	}
	if rows[0].Reason != "spam" || rows[0].Status != models.ReportStatusOpen {
		t.Fatalf("stored row = %+v, want first reason and open status", rows[0])
	}
	got, err := GetByID(first.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.ReporterId != 42 || got.ReportedUserId != 43 || got.ChatId != chatID {
		t.Fatalf("GetByID() = %+v, want the first report", got)
	}
}

func TestCreateStoresParentChat(t *testing.T) {
	skipIfNoDb(t)
	chatID := -time.Now().UnixNano()
	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Report{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
	})

	if !mustCreateReport(t, &models.Report{ChatId: chatID, ReportedUserId: 43, ReportedMessageId: 505, ReporterId: 42}) {
		t.Fatal("Create() created = false, want true")
	}

	var parents int64
	if err := db.DB.Model(&models.Chat{}).Where("chat_id = ?", chatID).Count(&parents).Error; err != nil {
		t.Fatalf("count chat parent: %v", err)
	}
	if parents != 1 {
		t.Fatalf("chat parent rows = %d, want 1", parents)
	}
}

func TestCreateStoresSnippet(t *testing.T) {
	chatID := newReportCaseChat(t)

	mustCreateReport(t, &models.Report{
		ChatId: chatID, ReportedUserId: 43, ReportedMessageId: 505, ReporterId: 42, Snippet: "buy cheap now",
	})
	rows, err := OpenForMessage(chatID, 505)
	if err != nil {
		t.Fatalf("OpenForMessage() error = %v", err)
	}
	if len(rows) != 1 || rows[0].Snippet != "buy cheap now" {
		t.Fatalf("stored rows = %+v, want the snippet kept", rows)
	}
}

func TestCreateRejectsUnknownStatus(t *testing.T) {
	chatID := newReportCaseChat(t)

	_, err := Create(&models.Report{
		ChatId: chatID, ReportedUserId: 43, ReportedMessageId: 505, ReporterId: 42, Status: "lost",
	})
	if err == nil {
		t.Fatal("Create() with an unknown status error = nil, want the status CHECK to reject it")
	}
}

// setResolvedAt backdates a closed report.
func setResolvedAt(t *testing.T, chatID, messageID int64, at time.Time) {
	t.Helper()
	if err := db.DB.Model(&models.Report{}).
		Where("chat_id = ? AND reported_message_id = ?", chatID, messageID).
		Update("resolved_at", at).Error; err != nil {
		t.Fatalf("backdate report of message %d: %v", messageID, err)
	}
}

func TestDeleteResolvedBeforeKeepsOpenAndRecentReports(t *testing.T) {
	chatID := newReportCaseChat(t)
	now := time.Now()
	cutoff := now.Add(-90 * 24 * time.Hour)

	// 1: closed long ago; 2: closed recently; 3: open and old.
	for _, msgID := range []int64{1, 2, 3} {
		mustCreateReport(t, &models.Report{ChatId: chatID, ReportedUserId: 9, ReportedMessageId: msgID, ReporterId: 100})
	}
	for _, msgID := range []int64{1, 2} {
		if _, err := Resolve(chatID, msgID, models.ReportStatusDismissed, 7); err != nil {
			t.Fatalf("Resolve(%d) error = %v", msgID, err)
		}
	}
	setResolvedAt(t, chatID, 1, cutoff.Add(-time.Hour))
	setResolvedAt(t, chatID, 2, cutoff.Add(time.Hour))
	if err := db.DB.Model(&models.Report{}).
		Where("chat_id = ? AND reported_message_id = ?", chatID, 3).
		Update("created_at", cutoff.Add(-time.Hour)).Error; err != nil {
		t.Fatalf("backdate open report: %v", err)
	}

	deleted, err := DeleteResolvedBefore(cutoff)
	if err != nil {
		t.Fatalf("DeleteResolvedBefore() error = %v", err)
	}
	if deleted < 1 {
		t.Fatalf("DeleteResolvedBefore() deleted = %d, want the old closed report", deleted)
	}

	var left []int64
	if err := db.DB.Model(&models.Report{}).Where("chat_id = ?", chatID).
		Order("reported_message_id").Pluck("reported_message_id", &left).Error; err != nil {
		t.Fatalf("list reports: %v", err)
	}
	if len(left) != 2 || left[0] != 2 || left[1] != 3 {
		t.Fatalf("reports left = %v, want [2 3]: the recent closed and the open one", left)
	}
}

func TestResolveClosesOpenReportsOnce(t *testing.T) {
	chatID := newReportCaseChat(t)

	mustCreateReport(t, &models.Report{ChatId: chatID, ReportedUserId: 43, ReportedMessageId: 505, ReporterId: 42})
	mustCreateReport(t, &models.Report{ChatId: chatID, ReportedUserId: 43, ReportedMessageId: 505, ReporterId: 44})

	affected, err := Resolve(chatID, 505, models.ReportStatusBanned, 7)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if affected != 2 {
		t.Fatalf("Resolve() affected = %d, want 2", affected)
	}
	affected, err = Resolve(chatID, 505, models.ReportStatusDismissed, 8)
	if err != nil {
		t.Fatalf("second Resolve() error = %v", err)
	}
	if affected != 0 {
		t.Fatalf("second Resolve() affected = %d, want 0", affected)
	}

	var rows []models.Report
	if err := db.DB.Where("chat_id = ? AND reported_message_id = ?", chatID, 505).Find(&rows).Error; err != nil {
		t.Fatalf("load rows: %v", err)
	}
	for _, row := range rows {
		if row.Status != models.ReportStatusBanned {
			t.Fatalf("row status = %q, want %q (first resolver wins)", row.Status, models.ReportStatusBanned)
		}
		if row.ResolvedBy == nil || *row.ResolvedBy != 7 {
			t.Fatalf("row resolved_by = %v, want 7", row.ResolvedBy)
		}
		if row.ResolvedAt == nil {
			t.Fatal("row resolved_at = nil, want a timestamp")
		}
	}
}

func TestResolveRejectsUnknownStatus(t *testing.T) {
	chatID := newReportCaseChat(t)
	mustCreateReport(t, &models.Report{ChatId: chatID, ReportedUserId: 43, ReportedMessageId: 505, ReporterId: 42})

	for _, status := range []string{models.ReportStatusOpen, "", "nuked"} {
		if _, err := Resolve(chatID, 505, status, 7); err == nil {
			t.Fatalf("Resolve(status %q) error = nil, want rejection", status)
		}
	}
	rows, err := OpenForMessage(chatID, 505)
	if err != nil {
		t.Fatalf("OpenForMessage() error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("open rows after rejected resolves = %d, want 1", len(rows))
	}
}

func TestConcurrentResolveHasOneWinner(t *testing.T) {
	chatID := newReportCaseChat(t)
	mustCreateReport(t, &models.Report{ChatId: chatID, ReportedUserId: 43, ReportedMessageId: 505, ReporterId: 42})

	const presses = 8
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		winners int
		errs    []error
	)
	for i := range presses {
		wg.Add(1)
		go func(adminID int64) {
			defer wg.Done()
			affected, err := Resolve(chatID, 505, models.ReportStatusBanned, adminID)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err)
				return
			}
			if affected > 0 {
				winners++
			}
		}(int64(1000 + i))
	}
	wg.Wait()

	if len(errs) > 0 {
		t.Fatalf("concurrent Resolve() errors = %v", errs)
	}
	if winners != 1 {
		t.Fatalf("concurrent Resolve() winners = %d, want exactly 1", winners)
	}
}

func TestSetAdminMessageStoresCardOnOpenReports(t *testing.T) {
	chatID := newReportCaseChat(t)
	mustCreateReport(t, &models.Report{ChatId: chatID, ReportedUserId: 43, ReportedMessageId: 505, ReporterId: 42})
	mustCreateReport(t, &models.Report{ChatId: chatID, ReportedUserId: 43, ReportedMessageId: 505, ReporterId: 44})
	mustCreateReport(t, &models.Report{ChatId: chatID, ReportedUserId: 43, ReportedMessageId: 606, ReporterId: 42})

	if err := SetAdminMessage(chatID, 505, -100777, 31); err != nil {
		t.Fatalf("SetAdminMessage() error = %v", err)
	}

	rows, err := OpenForMessage(chatID, 505)
	if err != nil {
		t.Fatalf("OpenForMessage() error = %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("open rows = %d, want 2", len(rows))
	}
	for _, row := range rows {
		if row.AdminChatId == nil || *row.AdminChatId != -100777 || row.AdminMessageId == nil || *row.AdminMessageId != 31 {
			t.Fatalf("row card = (%v, %v), want (-100777, 31)", row.AdminChatId, row.AdminMessageId)
		}
	}
	other, err := OpenForMessage(chatID, 606)
	if err != nil {
		t.Fatalf("OpenForMessage(606) error = %v", err)
	}
	if len(other) != 1 || other[0].AdminChatId != nil {
		t.Fatalf("other message rows = %+v, want untouched card fields", other)
	}
}

// Resolve and SetAdminMessage both lock a message's open rows; they must take
// them in the same order, or on Postgres they deadlock and one of them fails.
func TestConcurrentResolveAndSetAdminMessage(t *testing.T) {
	const (
		rounds    = 20
		reporters = 4
	)
	for round := range rounds {
		chatID := newReportCaseChat(t)
		messageID := int64(700 + round)
		for i := range reporters {
			mustCreateReport(t, &models.Report{
				ChatId: chatID, ReportedUserId: 43, ReportedMessageId: messageID, ReporterId: int64(100 + i),
			})
		}

		var (
			wg         sync.WaitGroup
			resolveErr error
			cardErr    error
		)
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, resolveErr = Resolve(chatID, messageID, models.ReportStatusBanned, 1000)
		}()
		go func() {
			defer wg.Done()
			cardErr = SetAdminMessage(chatID, messageID, -100777, 31)
		}()
		wg.Wait()

		if resolveErr != nil || cardErr != nil {
			t.Fatalf("round %d: Resolve() error = %v, SetAdminMessage() error = %v", round, resolveErr, cardErr)
		}
		var rows []models.Report
		if err := db.DB.Where("chat_id = ? AND reported_message_id = ?", chatID, messageID).Order("id").Find(&rows).Error; err != nil {
			t.Fatalf("round %d: load rows: %v", round, err)
		}
		carded := 0
		for _, row := range rows {
			if row.Status != models.ReportStatusBanned {
				t.Fatalf("round %d: row %d status = %q, want banned", round, row.ID, row.Status)
			}
			if row.AdminMessageId != nil {
				carded++
			}
		}
		if carded != 0 && carded != len(rows) {
			t.Fatalf("round %d: card stored on %d of %d rows, want all or none", round, carded, len(rows))
		}
	}
}
