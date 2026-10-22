//go:build testtools

package spamfilter

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

var testChatSeq atomic.Int64

const spamText = "Earn $500 a day from home! DM me for the crypto signals group"

// newChat returns a chat ID no other test in this package uses.
func newChat(t *testing.T) int64 {
	t.Helper()
	chatID := -(time.Now().UnixNano() + testChatSeq.Add(1))
	t.Cleanup(func() {
		if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.SpamSample{}).Error; err != nil {
			t.Errorf("cleanup spam samples of chat %d: %v", chatID, err)
		}
		if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.SpamFilterSettings{}).Error; err != nil {
			t.Errorf("cleanup spam filter settings of chat %d: %v", chatID, err)
		}
		cache.DeleteCache(samplesKey(chatID))
		cache.DeleteCache(settingsKey(chatID))
		cache.DeleteCache(sharedKey())
	})
	return chatID
}

func mustSettings(t *testing.T, chatID int64) *models.SpamFilterSettings {
	t.Helper()
	settings, err := GetSettings(context.Background(), chatID)
	if err != nil {
		t.Fatalf("GetSettings(%d) error = %v", chatID, err)
	}
	return settings
}

func mustAdd(t *testing.T, s NewSample) *models.SpamSample {
	t.Helper()
	sample, err := Add(s)
	if err != nil {
		t.Fatalf("Add(%+v) error = %v", s, err)
	}
	return sample
}

func mustList(t *testing.T, chatID int64) []models.SpamSample {
	t.Helper()
	rows, err := List(context.Background(), chatID)
	if err != nil {
		t.Fatalf("List(%d) error = %v", chatID, err)
	}
	return rows
}

func mustShared(t *testing.T) []SharedSample {
	t.Helper()
	shared, err := Shared(context.Background())
	if err != nil {
		t.Fatalf("Shared() error = %v", err)
	}
	return shared
}

func sharedHas(shared []SharedSample, normalized string) bool {
	for _, s := range shared {
		if s.Normalized == normalized {
			return true
		}
	}
	return false
}

func TestSettingsDefaultToOff(t *testing.T) {
	chatID := newChat(t)
	settings := mustSettings(t, chatID)
	if settings.Mode != models.SpamFilterOff || settings.Threshold != models.DefaultSpamFilterThreshold {
		t.Fatalf("GetSettings(new chat) = %+v, want off at %.2f", settings, models.DefaultSpamFilterThreshold)
	}
	if Acting(settings, time.Now()) {
		t.Fatal("Acting(default settings) = true, want false")
	}
}

func TestSetModeKeepsTheDryRunAlreadyDone(t *testing.T) {
	chatID := newChat(t)
	if err := SetMode(chatID, models.SpamFilterDryRun); err != nil {
		t.Fatalf("SetMode(dryrun) error = %v", err)
	}
	dry := mustSettings(t, chatID)
	if dry.Mode != models.SpamFilterDryRun || dry.EnabledAt == nil {
		t.Fatalf("after dryrun settings = %+v, want dryrun with EnabledAt", dry)
	}
	started := *dry.EnabledAt

	if err := SetMode(chatID, models.SpamFilterOn); err != nil {
		t.Fatalf("SetMode(on) error = %v", err)
	}
	on := mustSettings(t, chatID)
	if on.Mode != models.SpamFilterOn || on.EnabledAt == nil || !on.EnabledAt.Equal(started) {
		t.Fatalf("after on settings = %+v, want on with EnabledAt %v kept", on, started)
	}
	if Acting(on, started.Add(DryRunPeriod-time.Minute)) {
		t.Fatal("Acting(on, inside the dry run) = true, want false")
	}
	if !Acting(on, started.Add(DryRunPeriod)) {
		t.Fatal("Acting(on, after the dry run) = false, want true")
	}

	if err := SetMode(chatID, models.SpamFilterOff); err != nil {
		t.Fatalf("SetMode(off) error = %v", err)
	}
	off := mustSettings(t, chatID)
	if off.Mode != models.SpamFilterOff || off.EnabledAt != nil {
		t.Fatalf("after off settings = %+v, want off without EnabledAt", off)
	}
	if err := SetMode(chatID, models.SpamFilterOn); err != nil {
		t.Fatalf("SetMode(on) again error = %v", err)
	}
	again := mustSettings(t, chatID)
	if again.EnabledAt == nil || Acting(again, time.Now()) {
		t.Fatalf("on after off settings = %+v, want a new dry run", again)
	}
}

func TestDryRunNeverActs(t *testing.T) {
	chatID := newChat(t)
	if err := SetMode(chatID, models.SpamFilterDryRun); err != nil {
		t.Fatalf("SetMode(dryrun) error = %v", err)
	}
	if Acting(mustSettings(t, chatID), time.Now().Add(365*24*time.Hour)) {
		t.Fatal("Acting(dryrun, a year later) = true, want false")
	}
}

func TestSetThresholdValidatesAndKeepsTheMode(t *testing.T) {
	chatID := newChat(t)
	if err := SetMode(chatID, models.SpamFilterDryRun); err != nil {
		t.Fatalf("SetMode(dryrun) error = %v", err)
	}
	for _, bad := range []float64{0.49, 1.01, -1} {
		if err := SetThreshold(chatID, bad); !errors.Is(err, ErrInvalidThreshold) {
			t.Fatalf("SetThreshold(%v) error = %v, want ErrInvalidThreshold", bad, err)
		}
	}
	if err := SetThreshold(chatID, 0.7); err != nil {
		t.Fatalf("SetThreshold(0.7) error = %v", err)
	}
	settings := mustSettings(t, chatID)
	if settings.Threshold != 0.7 || settings.Mode != models.SpamFilterDryRun {
		t.Fatalf("settings = %+v, want dryrun at 0.7", settings)
	}
	if err := SetMode(chatID, "maybe"); !errors.Is(err, ErrInvalidMode) {
		t.Fatalf("SetMode(maybe) error = %v, want ErrInvalidMode", err)
	}
}

func TestAddStoresOneRowPerTextAndTheLatestLabelWins(t *testing.T) {
	chatID := newChat(t)
	first := mustAdd(t, NewSample{ChatID: chatID, Label: models.SpamLabelSpam, Text: spamText, SenderID: 5, AddedBy: 9})
	if first.ID == 0 || first.Normalized == "" || first.ContentHash != Hash(first.Normalized) {
		t.Fatalf("Add() = %+v, want a stored, normalised row", first)
	}
	if rows := mustList(t, chatID); len(rows) != 1 || rows[0].Label != models.SpamLabelSpam {
		t.Fatalf("List() = %+v, want the one spam row", rows)
	}

	// The same text, differently decorated, normalises to the same row.
	second := mustAdd(t, NewSample{ChatID: chatID, Label: models.SpamLabelHam, Text: "EARN $500 a day from home!!! dm me for the crypto signals group 🔥", AddedBy: 9})
	if second.ID != first.ID {
		t.Fatalf("Add(same text) ID = %d, want %d", second.ID, first.ID)
	}
	rows := mustList(t, chatID)
	if len(rows) != 1 || rows[0].Label != models.SpamLabelHam {
		t.Fatalf("List() after relabel = %+v, want the one ham row", rows)
	}
}

func TestAddRefusesShortTexts(t *testing.T) {
	chatID := newChat(t)
	if _, err := Add(NewSample{ChatID: chatID, Label: models.SpamLabelSpam, Text: "hi 👋 ok"}); !errors.Is(err, ErrTooShort) {
		t.Fatalf("Add(short) error = %v, want ErrTooShort", err)
	}
	if rows := mustList(t, chatID); len(rows) != 0 {
		t.Fatalf("List() = %+v, want nothing stored", rows)
	}
}

func TestAddKeepsTheNewestSamplesOfEachLabel(t *testing.T) {
	chatID := newChat(t)
	ham := mustAdd(t, NewSample{ChatID: chatID, Label: models.SpamLabelHam, Text: "the weekly meeting moved to thursday"})
	for i := range MaxSamplesPerLabel + 3 {
		mustAdd(t, NewSample{ChatID: chatID, Label: models.SpamLabelSpam, Text: fmt.Sprintf("buy followers now offer number %d", i)})
	}
	var spam, hams int64
	db.DB.Model(&models.SpamSample{}).Where("chat_id = ? AND label = ?", chatID, models.SpamLabelSpam).Count(&spam)
	db.DB.Model(&models.SpamSample{}).Where("chat_id = ? AND label = ?", chatID, models.SpamLabelHam).Count(&hams)
	if spam != MaxSamplesPerLabel || hams != 1 {
		t.Fatalf("stored %d spam and %d ham, want %d and 1", spam, hams, MaxSamplesPerLabel)
	}
	if _, err := Get(ham.ID); err != nil {
		t.Fatalf("Get(ham) error = %v, want the ham kept", err)
	}
	var oldest int64
	db.DB.Model(&models.SpamSample{}).Where("chat_id = ? AND text = ?", chatID, "buy followers now offer number 0").Count(&oldest)
	if oldest != 0 {
		t.Fatal("the oldest spam sample survived the cap")
	}
}

func TestDeleteRemovesOnlyTheChatsOwnSample(t *testing.T) {
	chatID, other := newChat(t), newChat(t)
	sample := mustAdd(t, NewSample{ChatID: other, Label: models.SpamLabelSpam, Text: spamText})
	if removed, err := Delete(chatID, sample.ID); err != nil || removed {
		t.Fatalf("Delete(other chat's sample) = %v, %v; want false, nil", removed, err)
	}
	if _, err := Get(sample.ID); err != nil {
		t.Fatalf("Get() after foreign delete error = %v", err)
	}
	if removed, err := Delete(other, sample.ID); err != nil || !removed {
		t.Fatalf("Delete(own sample) = %v, %v; want true, nil", removed, err)
	}
	if _, err := Get(sample.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("Get() after delete error = %v, want not found", err)
	}
	if rows := mustList(t, other); len(rows) != 0 {
		t.Fatalf("List() after delete = %+v, want empty", rows)
	}
}

func TestMarkHamReplacesTheSampleWithHam(t *testing.T) {
	chatID := newChat(t)
	sample := mustAdd(t, NewSample{ChatID: chatID, Label: models.SpamLabelSpam, Text: spamText})
	message := "Earn credits a day from home by reviewing the crypto docs group"
	if err := MarkHam(chatID, sample.ID, message, 42); err != nil {
		t.Fatalf("MarkHam() error = %v", err)
	}
	rows := mustList(t, chatID)
	if len(rows) != 1 || rows[0].Label != models.SpamLabelHam || rows[0].Text != message || rows[0].AddedBy != 42 {
		t.Fatalf("List() after MarkHam = %+v, want only the ham message", rows)
	}
}

func TestRelabelSenderAsHamTurnsOnlyThatSendersSpam(t *testing.T) {
	chatID := newChat(t)
	mustAdd(t, NewSample{ChatID: chatID, Label: models.SpamLabelSpam, Text: spamText, SenderID: 5})
	other := mustAdd(t, NewSample{ChatID: chatID, Label: models.SpamLabelSpam, Text: "cheap followers and likes, message me", SenderID: 6})
	turned, err := RelabelSenderAsHam(chatID, 5, 9)
	if err != nil || turned != 1 {
		t.Fatalf("RelabelSenderAsHam() = %d, %v; want 1, nil", turned, err)
	}
	for _, row := range mustList(t, chatID) {
		want := models.SpamLabelHam
		if row.ID == other.ID {
			want = models.SpamLabelSpam
		}
		if row.Label != want {
			t.Fatalf("row %d (sender %d) label = %q, want %q", row.ID, row.SenderID, row.Label, want)
		}
	}
	if turned, err := RelabelSenderAsHam(chatID, 77, 9); err != nil || turned != 0 {
		t.Fatalf("RelabelSenderAsHam(no samples) = %d, %v; want 0, nil", turned, err)
	}
}

func TestDeleteExpiredBeforeRemovesOldSamples(t *testing.T) {
	chatID := newChat(t)
	old := mustAdd(t, NewSample{ChatID: chatID, Label: models.SpamLabelSpam, Text: spamText})
	fresh := mustAdd(t, NewSample{ChatID: chatID, Label: models.SpamLabelSpam, Text: "cheap followers and likes, message me"})
	mustList(t, chatID) // cache the list, which the sweep must clear
	if err := db.DB.Model(&models.SpamSample{}).Where("id = ?", old.ID).
		Update("created_at", time.Now().UTC().Add(-Retention-time.Hour)).Error; err != nil {
		t.Fatalf("age sample: %v", err)
	}
	removed, err := DeleteExpiredBefore(time.Now().Add(-Retention))
	if err != nil || removed < 1 {
		t.Fatalf("DeleteExpiredBefore() = %d, %v; want at least 1", removed, err)
	}
	rows := mustList(t, chatID)
	if len(rows) != 1 || rows[0].ID != fresh.ID {
		t.Fatalf("List() after sweep = %+v, want only the fresh sample", rows)
	}
}

func TestSharedNeedsTwoUnrelatedChats(t *testing.T) {
	text := spamText + fmt.Sprintf(" %d", time.Now().UnixNano())
	normalized := mustAdd(t, NewSample{ChatID: newChat(t), Label: models.SpamLabelSpam, Text: text, OwnerID: 1001, AddedBy: 2001, AdminIDs: []int64{3001}}).Normalized

	if sharedHas(mustShared(t), normalized) {
		t.Fatal("a text one chat flagged is shared")
	}

	// A second chat of the same owner does not count.
	mustAdd(t, NewSample{ChatID: newChat(t), Label: models.SpamLabelSpam, Text: text, OwnerID: 1001, AddedBy: 2002})
	if sharedHas(mustShared(t), normalized) {
		t.Fatal("a text two chats of one owner flagged is shared")
	}

	// Nor does a chat with another owner but an admin in common.
	mustAdd(t, NewSample{ChatID: newChat(t), Label: models.SpamLabelSpam, Text: text, OwnerID: 1002, AddedBy: 2003, AdminIDs: []int64{3001, 2002}})
	if sharedHas(mustShared(t), normalized) {
		t.Fatal("a text two chats sharing an admin flagged is shared")
	}

	// Nor a chat whose owner is unknown.
	mustAdd(t, NewSample{ChatID: newChat(t), Label: models.SpamLabelSpam, Text: text, AddedBy: 2004})
	if sharedHas(mustShared(t), normalized) {
		t.Fatal("a text a chat with an unknown owner flagged is shared")
	}

	// An unrelated chat puts it in.
	unrelatedChat := newChat(t)
	mustAdd(t, NewSample{ChatID: unrelatedChat, Label: models.SpamLabelSpam, Text: text, OwnerID: 1003, AddedBy: 2005, AdminIDs: []int64{3005}})
	if !sharedHas(mustShared(t), normalized) {
		t.Fatal("a text two unrelated chats flagged is not shared")
	}

	// Ham in that chat takes it out again.
	mustAdd(t, NewSample{ChatID: unrelatedChat, Label: models.SpamLabelHam, Text: text})
	if sharedHas(mustShared(t), normalized) {
		t.Fatal("a text is still shared after the unrelated chat called it ham")
	}
}

func TestAdminSetIsSortedAndSkipsNonUsers(t *testing.T) {
	if got := adminSet([]int64{30, -100123, 10, 30, 0}, 20, 10); got != "10,20,30" {
		t.Fatalf("adminSet() = %q, want %q", got, "10,20,30")
	}
}
