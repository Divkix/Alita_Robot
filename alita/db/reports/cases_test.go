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

// setCreatedAt backdates when a report was made.
func setCreatedAt(t *testing.T, chatID, messageID int64, at time.Time) {
	t.Helper()
	if err := db.DB.Model(&models.Report{}).
		Where("chat_id = ? AND reported_message_id = ?", chatID, messageID).
		Update("created_at", at).Error; err != nil {
		t.Fatalf("backdate creation of report of message %d: %v", messageID, err)
	}
}

func TestDeleteExpiredBeforeDropsOldReports(t *testing.T) {
	chatID := newReportCaseChat(t)
	now := time.Now()
	cutoff := now.Add(-90 * 24 * time.Hour)

	// 1: closed long ago; 2: closed recently; 3: open and old; 4: open and
	// recent; 5: made long ago but closed recently.
	for _, msgID := range []int64{1, 2, 3, 4, 5} {
		mustCreateReport(t, &models.Report{ChatId: chatID, ReportedUserId: 9, ReportedMessageId: msgID, ReporterId: 100})
	}
	for _, msgID := range []int64{1, 2, 5} {
		if _, err := Resolve(chatID, msgID, models.ReportStatusDismissed, 7); err != nil {
			t.Fatalf("Resolve(%d) error = %v", msgID, err)
		}
	}
	setResolvedAt(t, chatID, 1, cutoff.Add(-time.Hour))
	setResolvedAt(t, chatID, 2, cutoff.Add(time.Hour))
	setCreatedAt(t, chatID, 3, cutoff.Add(-time.Hour))
	setCreatedAt(t, chatID, 4, cutoff.Add(time.Hour))
	setCreatedAt(t, chatID, 5, cutoff.Add(-time.Hour))
	setResolvedAt(t, chatID, 5, cutoff.Add(time.Hour))

	deleted, err := DeleteExpiredBefore(cutoff)
	if err != nil {
		t.Fatalf("DeleteExpiredBefore() error = %v", err)
	}
	if deleted != 2 {
		t.Fatalf("DeleteExpiredBefore() deleted = %d, want the old closed and the old open report", deleted)
	}

	var left []int64
	if err := db.DB.Model(&models.Report{}).Where("chat_id = ?", chatID).
		Order("reported_message_id").Pluck("reported_message_id", &left).Error; err != nil {
		t.Fatalf("list reports: %v", err)
	}
	if len(left) != 3 || left[0] != 2 || left[1] != 4 || left[2] != 5 {
		t.Fatalf("reports left = %v, want [2 4 5]: those closed or made within the retention", left)
	}
}

func TestResolveLeavesOtherMessagesOpen(t *testing.T) {
	chatID := newReportCaseChat(t)
	mustCreateReport(t, &models.Report{ChatId: chatID, ReportedUserId: 43, ReportedMessageId: 505, ReporterId: 42})
	mustCreateReport(t, &models.Report{ChatId: chatID, ReportedUserId: 43, ReportedMessageId: 506, ReporterId: 42})

	if _, err := Resolve(chatID, 505, models.ReportStatusDismissed, 7); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	for msgID, want := range map[int64]int{505: 0, 506: 1} {
		rows, err := OpenForMessage(chatID, msgID)
		if err != nil {
			t.Fatalf("OpenForMessage(%d) error = %v", msgID, err)
		}
		if len(rows) != want {
			t.Fatalf("open reports of message %d = %d, want %d: only message 505 is resolved", msgID, len(rows), want)
		}
	}
}

func TestResolveClosesOpenReportsOnce(t *testing.T) {
	chatID := newReportCaseChat(t)

	mustCreateReport(t, &models.Report{ChatId: chatID, ReportedUserId: 43, ReportedMessageId: 505, ReporterId: 42})
	mustCreateReport(t, &models.Report{ChatId: chatID, ReportedUserId: 43, ReportedMessageId: 505, ReporterId: 44})

	closed, err := Resolve(chatID, 505, models.ReportStatusBanned, 7)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(closed) != 2 {
		t.Fatalf("Resolve() closed = %v, want 2 reports", closed)
	}
	closed, err = Resolve(chatID, 505, models.ReportStatusDismissed, 8)
	if err != nil {
		t.Fatalf("second Resolve() error = %v", err)
	}
	if len(closed) != 0 {
		t.Fatalf("second Resolve() closed = %v, want none", closed)
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

// linkCard puts report id on card adminMessageID in channel -100777.
func linkCard(t *testing.T, id, adminMessageID int64) *models.Report {
	t.Helper()
	if err := db.DB.Model(&models.Report{}).Where("id = ?", id).
		Updates(map[string]any{"admin_chat_id": -100777, "admin_message_id": adminMessageID}).Error; err != nil {
		t.Fatalf("link report %d to card %d: %v", id, adminMessageID, err)
	}
	r, err := GetByID(id)
	if err != nil {
		t.Fatalf("GetByID(%d) error = %v", id, err)
	}
	return r
}

func TestResolveCardClosesOnlyReportsOnTheCard(t *testing.T) {
	chatID := newReportCaseChat(t)
	a := &models.Report{ChatId: chatID, ReportedUserId: 43, ReportedMessageId: 505, ReporterId: 42}
	b := &models.Report{ChatId: chatID, ReportedUserId: 43, ReportedMessageId: 505, ReporterId: 44}
	c := &models.Report{ChatId: chatID, ReportedUserId: 43, ReportedMessageId: 505, ReporterId: 45}
	for _, r := range []*models.Report{a, b, c} {
		mustCreateReport(t, r)
	}
	card := linkCard(t, a.ID, 31)
	linkCard(t, b.ID, 31)
	linkCard(t, c.ID, 32)

	closed, err := ResolveCard(card, models.ReportStatusBanned, 7)
	if err != nil {
		t.Fatalf("ResolveCard() error = %v", err)
	}
	if len(closed) != 2 || closed[0] != a.ID || closed[1] != b.ID {
		t.Fatalf("ResolveCard() closed = %v, want reports %d and %d of card 31", closed, a.ID, b.ID)
	}
	if left, err := GetByID(c.ID); err != nil || left.Status != models.ReportStatusOpen {
		t.Fatalf("report on card 32 = %+v, %v; want it open", left, err)
	}
}

// The pressed card's report was decided after it was loaded, and a newer
// report of the message is open on a card of its own.
func TestResolveCardIgnoresClosedReport(t *testing.T) {
	chatID := newReportCaseChat(t)
	a := &models.Report{ChatId: chatID, ReportedUserId: 43, ReportedMessageId: 505, ReporterId: 42}
	mustCreateReport(t, a)
	stale := linkCard(t, a.ID, 31)
	if _, err := Resolve(chatID, 505, models.ReportStatusDismissed, 8); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	c := &models.Report{ChatId: chatID, ReportedUserId: 43, ReportedMessageId: 505, ReporterId: 44}
	mustCreateReport(t, c)
	linkCard(t, c.ID, 31) // even on the same card

	closed, err := ResolveCard(stale, models.ReportStatusBanned, 7)
	if err != nil || len(closed) != 0 {
		t.Fatalf("ResolveCard() = %v, %v; want nothing closed for a decided report", closed, err)
	}
	if left, err := GetByID(c.ID); err != nil || left.Status != models.ReportStatusOpen {
		t.Fatalf("newer report = %+v, %v; want it open", left, err)
	}

	if _, err := ResolveCard(&models.Report{ID: c.ID, ChatId: chatID, ReportedMessageId: 505}, models.ReportStatusBanned, 7); err == nil {
		t.Fatal("ResolveCard() without a card error = nil, want rejection")
	}
}

func TestHasReportsCountsDecidedReports(t *testing.T) {
	chatID := newReportCaseChat(t)
	mustCreateReport(t, &models.Report{ChatId: chatID, ReportedUserId: 43, ReportedMessageId: 505, ReporterId: 42})
	if _, err := Resolve(chatID, 505, models.ReportStatusDismissed, 7); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	for msgID, want := range map[int64]bool{505: true, 606: false} {
		got, err := HasReports(chatID, msgID)
		if err != nil || got != want {
			t.Fatalf("HasReports(%d) = %v, %v; want %v", msgID, got, err, want)
		}
	}
}

func TestReopenRestoresOnlyTheFailedDecision(t *testing.T) {
	chatID := newReportCaseChat(t)
	mustCreateReport(t, &models.Report{ChatId: chatID, ReportedUserId: 43, ReportedMessageId: 505, ReporterId: 42})
	earlier, err := Resolve(chatID, 505, models.ReportStatusBanned, 7)
	if err != nil || len(earlier) != 1 {
		t.Fatalf("first Resolve() = %v, %v; want one report", earlier, err)
	}
	mustCreateReport(t, &models.Report{ChatId: chatID, ReportedUserId: 43, ReportedMessageId: 505, ReporterId: 44})
	failed, err := Resolve(chatID, 505, models.ReportStatusBanned, 7)
	if err != nil || len(failed) != 1 {
		t.Fatalf("second Resolve() = %v, %v; want the new report", failed, err)
	}

	if err := Reopen(failed, models.ReportStatusBanned); err != nil {
		t.Fatalf("Reopen() error = %v", err)
	}

	rows, err := OpenForMessage(chatID, 505)
	if err != nil {
		t.Fatalf("OpenForMessage() error = %v", err)
	}
	if len(rows) != 1 || rows[0].ID != failed[0] || rows[0].ResolvedBy != nil || rows[0].ResolvedAt != nil {
		t.Fatalf("open reports = %+v, want only report %d reopened without a resolver", rows, failed[0])
	}
	kept, err := GetByID(earlier[0])
	if err != nil || kept.Status != models.ReportStatusBanned {
		t.Fatalf("earlier report = %+v, %v; want it still banned", kept, err)
	}
}

func TestReopenLeavesOtherDecisionsAlone(t *testing.T) {
	chatID := newReportCaseChat(t)
	mustCreateReport(t, &models.Report{ChatId: chatID, ReportedUserId: 43, ReportedMessageId: 505, ReporterId: 42})
	mustCreateReport(t, &models.Report{ChatId: chatID, ReportedUserId: 43, ReportedMessageId: 505, ReporterId: 44})
	failed, err := Resolve(chatID, 505, models.ReportStatusBanned, 7)
	if err != nil || len(failed) != 2 {
		t.Fatalf("Resolve() = %v, %v; want both reports", failed, err)
	}
	// Before the failed ban is undone, one of its reports gets another decision.
	if err := db.DB.Model(&models.Report{}).Where("id = ?", failed[1]).
		Updates(map[string]any{"status": models.ReportStatusDismissed, "resolved_by": 8}).Error; err != nil {
		t.Fatalf("redecide report: %v", err)
	}

	if err := Reopen(failed, models.ReportStatusBanned); err != nil {
		t.Fatalf("Reopen() error = %v", err)
	}

	rows, err := OpenForMessage(chatID, 505)
	if err != nil {
		t.Fatalf("OpenForMessage() error = %v", err)
	}
	if len(rows) != 1 || rows[0].ID != failed[0] {
		t.Fatalf("open reports = %+v, want only report %d reopened", rows, failed[0])
	}
	other, err := GetByID(failed[1])
	if err != nil || other.Status != models.ReportStatusDismissed || other.ResolvedBy == nil || *other.ResolvedBy != 8 {
		t.Fatalf("redecided report = %+v, %v; want it still dismissed by 8", other, err)
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
			closed, err := Resolve(chatID, 505, models.ReportStatusBanned, adminID)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err)
				return
			}
			if len(closed) > 0 {
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

	if recorded, err := SetAdminMessage(chatID, 505, -100777, 31, 0); err != nil || recorded != 31 {
		t.Fatalf("SetAdminMessage() = %d, %v; want card 31", recorded, err)
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

// Two first reports of a message post a card each at once: the card
// recorded first keeps the reports, and the later one is told it lost.
func TestSetAdminMessageKeepsCardRecordedFirst(t *testing.T) {
	chatID := newReportCaseChat(t)
	a := &models.Report{ChatId: chatID, ReportedUserId: 43, ReportedMessageId: 505, ReporterId: 42}
	mustCreateReport(t, a)
	if recorded, err := SetAdminMessage(chatID, 505, -100777, 32, 0); err != nil || recorded != 32 {
		t.Fatalf("first SetAdminMessage() = %d, %v; want card 32", recorded, err)
	}
	mustCreateReport(t, &models.Report{ChatId: chatID, ReportedUserId: 43, ReportedMessageId: 505, ReporterId: 44})

	recorded, err := SetAdminMessage(chatID, 505, -100777, 31, 0)
	if err != nil || recorded != 32 {
		t.Fatalf("second SetAdminMessage() = %d, %v; want card 32 kept", recorded, err)
	}
	rows, err := OpenForMessage(chatID, 505)
	if err != nil || len(rows) != 2 {
		t.Fatalf("OpenForMessage() = %+v, %v; want 2 rows", rows, err)
	}
	for _, row := range rows {
		if row.AdminMessageId == nil || *row.AdminMessageId != 32 {
			t.Fatalf("report %d card = %v, want 32 on every open report", row.ID, row.AdminMessageId)
		}
	}
}

// A card posted to replace one that can no longer be edited takes the
// reports over, as does a card in a new log channel.
func TestSetAdminMessageReplacesCard(t *testing.T) {
	chatID := newReportCaseChat(t)
	mustCreateReport(t, &models.Report{ChatId: chatID, ReportedUserId: 43, ReportedMessageId: 505, ReporterId: 42})
	if _, err := SetAdminMessage(chatID, 505, -100777, 31, 0); err != nil {
		t.Fatalf("SetAdminMessage() error = %v", err)
	}
	if recorded, err := SetAdminMessage(chatID, 505, -100777, 40, 31); err != nil || recorded != 40 {
		t.Fatalf("replacing SetAdminMessage() = %d, %v; want card 40", recorded, err)
	}
	if recorded, err := SetAdminMessage(chatID, 505, -100888, 50, 0); err != nil || recorded != 50 {
		t.Fatalf("SetAdminMessage() in another channel = %d, %v; want card 50", recorded, err)
	}
	rows, err := OpenForMessage(chatID, 505)
	if err != nil || len(rows) != 1 || *rows[0].AdminChatId != -100888 || *rows[0].AdminMessageId != 50 {
		t.Fatalf("OpenForMessage() = %+v, %v; want the report on card 50 in -100888", rows, err)
	}
	if recorded, err := SetAdminMessage(chatID, 606, -100777, 60, 0); err != nil || recorded != 0 {
		t.Fatalf("SetAdminMessage() without open reports = %d, %v; want 0", recorded, err)
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
			_, cardErr = SetAdminMessage(chatID, messageID, -100777, 31, 0)
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
