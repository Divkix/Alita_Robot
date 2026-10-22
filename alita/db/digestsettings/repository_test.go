//go:build testtools

package digestsettings

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

var testChatSeq atomic.Int64

// newDigestChat returns a chat ID no other test in this package uses, so rows
// from an earlier test cannot reach this one's assertions.
func newDigestChat(t *testing.T) int64 {
	t.Helper()
	chatID := time.Now().UnixNano() + testChatSeq.Add(1)
	t.Cleanup(func() {
		if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.DigestSettings{}).Error; err != nil {
			t.Errorf("cleanup digest settings of chat %d: %v", chatID, err)
		}
	})
	return chatID
}

func mustGetSettings(t *testing.T, chatID int64) *models.DigestSettings {
	t.Helper()
	settings, err := Get(chatID)
	if err != nil {
		t.Fatalf("Get(%d) error = %v", chatID, err)
	}
	return settings
}

func mustSetEnabled(t *testing.T, chatID int64, enabled bool) {
	t.Helper()
	if err := SetEnabled(chatID, enabled); err != nil {
		t.Fatalf("SetEnabled(%d, %v) error = %v", chatID, enabled, err)
	}
}

func mustSetDestination(t *testing.T, chatID int64, destination string) {
	t.Helper()
	if err := SetDestination(chatID, destination); err != nil {
		t.Fatalf("SetDestination(%d, %q) error = %v", chatID, destination, err)
	}
}

func mustClaim(t *testing.T, chatID int64, old *time.Time, claimedAt time.Time) bool {
	t.Helper()
	won, err := Claim(chatID, old, claimedAt)
	if err != nil {
		t.Fatalf("Claim(%d, %v, %v) error = %v", chatID, old, claimedAt, err)
	}
	return won
}

func TestGetDefaultsToDisabledAutoAndNeverSent(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)
	chatID := newDigestChat(t)

	settings := mustGetSettings(t, chatID)
	if settings.Enabled {
		t.Fatal("missing row = enabled, want disabled")
	}
	if settings.Destination != DestinationAuto {
		t.Fatalf("destination = %q, want the automatic default", settings.Destination)
	}
	if settings.LastSentAt != nil {
		t.Fatalf("last_sent_at = %v, want nil", settings.LastSentAt)
	}

	var count int64
	if err := db.DB.Model(&models.DigestSettings{}).Where("chat_id = ?", chatID).Count(&count).Error; err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 0 {
		t.Fatalf("rows = %d, want 0: reading must not insert", count)
	}
}

// The switch and the destination are separate commands; neither may reset the
// other, and false must reach the row rather than the column default.
func TestSetEnabledAndDestinationAreIndependent(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)
	chatID := newDigestChat(t)

	mustSetDestination(t, chatID, DestinationLog)
	mustSetEnabled(t, chatID, true)
	if got := mustGetSettings(t, chatID); !got.Enabled || got.Destination != DestinationLog {
		t.Fatalf("after enable = %+v, want enabled with log", got)
	}

	mustSetEnabled(t, chatID, false)
	if got := mustGetSettings(t, chatID); got.Enabled || got.Destination != DestinationLog {
		t.Fatalf("after disable = %+v, want disabled with log", got)
	}

	mustSetDestination(t, chatID, DestinationGroup)
	if got := mustGetSettings(t, chatID); got.Enabled || got.Destination != DestinationGroup {
		t.Fatalf("after destination change = %+v, want disabled with group", got)
	}

	var count int64
	if err := db.DB.Model(&models.DigestSettings{}).Where("chat_id = ?", chatID).Count(&count).Error; err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("rows = %d, want 1 (writes must upsert, not insert)", count)
	}
}

func TestSetDestinationRejectsUnknownValue(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)
	chatID := newDigestChat(t)

	if err := SetDestination(chatID, "channel"); !errors.Is(err, ErrInvalidDestination) {
		t.Fatalf("SetDestination(channel) error = %v, want ErrInvalidDestination", err)
	}
	var count int64
	if err := db.DB.Model(&models.DigestSettings{}).Where("chat_id = ?", chatID).Count(&count).Error; err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 0 {
		t.Fatalf("rows = %d, want 0 after a refused destination", count)
	}
}

// ListDue is the scheduler's query: on, and either never sent or sent before
// this week's Monday.
func TestListDueSelectsOnlyEnabledChatsBeforeTheBoundary(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)
	boundary := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)

	neverSent := newDigestChat(t)
	mustSetEnabled(t, neverSent, true)

	sentLastWeek := newDigestChat(t)
	mustSetEnabled(t, sentLastWeek, true)
	if won := mustClaim(t, sentLastWeek, nil, boundary.AddDate(0, 0, -7)); !won {
		t.Fatal("claim before the boundary did not win")
	}

	alreadySent := newDigestChat(t)
	mustSetEnabled(t, alreadySent, true)
	if won := mustClaim(t, alreadySent, nil, boundary); !won {
		t.Fatal("claim at the boundary did not win")
	}

	disabled := newDigestChat(t)
	mustSetDestination(t, disabled, DestinationGroup)

	due, err := ListDue(boundary)
	if err != nil {
		t.Fatalf("ListDue() error = %v", err)
	}
	got := make(map[int64]bool, len(due))
	for _, row := range due {
		got[row.ChatID] = true
	}
	if !got[neverSent] {
		t.Error("a chat that never had a digest is not due")
	}
	if !got[sentLastWeek] {
		t.Error("a chat last sent before the boundary is not due")
	}
	if got[alreadySent] {
		t.Error("a chat already sent this week is due again")
	}
	if got[disabled] {
		t.Error("a disabled chat is due")
	}
}

// The claim is the exactly-once mechanism: one replica wins, the second gets
// zero rows. A restore releases the claim so the next tick retries.
func TestClaimLetsOneReplicaWinAndRestoreReleasesIt(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)
	chatID := newDigestChat(t)
	mustSetEnabled(t, chatID, true)

	claimedAt := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
	if won := mustClaim(t, chatID, nil, claimedAt); !won {
		t.Fatal("the first claim lost")
	}
	if won := mustClaim(t, chatID, nil, claimedAt.Add(time.Minute)); won {
		t.Fatal("a second replica claimed an already claimed digest")
	}
	settings := mustGetSettings(t, chatID)
	if settings.LastSentAt == nil || !settings.LastSentAt.Equal(claimedAt) {
		t.Fatalf("last_sent_at = %v, want %v", settings.LastSentAt, claimedAt)
	}

	// The failed send restores the null claim, and the next tick wins again.
	if err := Restore(chatID, claimedAt, nil); err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	if got := mustGetSettings(t, chatID); got.LastSentAt != nil {
		t.Fatalf("last_sent_at = %v after restore, want nil", got.LastSentAt)
	}
	retriedAt := claimedAt.Add(time.Hour)
	if won := mustClaim(t, chatID, nil, retriedAt); !won {
		t.Fatal("the retry after a restore lost")
	}

	// A later claim only wins against the value it read, so a stale replica
	// cannot take the send away.
	if won := mustClaim(t, chatID, &claimedAt, retriedAt.Add(time.Hour)); won {
		t.Fatal("a claim with a stale last_sent_at won")
	}
	if won := mustClaim(t, chatID, &retriedAt, retriedAt.Add(2*time.Hour)); !won {
		t.Fatal("a claim with the current last_sent_at lost")
	}
}

// Restoring a claim whose row moved on must not undo the newer send.
func TestRestoreLeavesANewerClaimAlone(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)
	chatID := newDigestChat(t)
	mustSetEnabled(t, chatID, true)

	failed := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
	if won := mustClaim(t, chatID, nil, failed); !won {
		t.Fatal("the first claim lost")
	}
	newer := failed.Add(time.Hour)
	if err := Restore(chatID, failed, nil); err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	if won := mustClaim(t, chatID, nil, newer); !won {
		t.Fatal("the claim after restore lost")
	}
	if err := Restore(chatID, failed, nil); err != nil {
		t.Fatalf("Restore() of the stale claim error = %v", err)
	}
	settings := mustGetSettings(t, chatID)
	if settings.LastSentAt == nil || !settings.LastSentAt.Equal(newer) {
		t.Fatalf("last_sent_at = %v, want the newer claim %v", settings.LastSentAt, newer)
	}
}
