//go:build testtools

package probation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

func requireDB(t *testing.T) {
	t.Helper()
	if db.DB == nil {
		t.Fatal("test database was not initialized")
	}
}

func cleanupChat(t *testing.T, chatID int64) {
	t.Helper()
	t.Cleanup(func() {
		if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.ProbationSettings{}).Error; err != nil {
			t.Fatalf("cleanup settings: %v", err)
		}
		if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.ProbationMember{}).Error; err != nil {
			t.Fatalf("cleanup members: %v", err)
		}
	})
}

func mustSettings(t *testing.T, chatID int64) *models.ProbationSettings {
	t.Helper()
	s, err := GetSettings(context.Background(), chatID)
	if err != nil {
		t.Fatalf("GetSettings(%d) error = %v", chatID, err)
	}
	return s
}

func mustMember(t *testing.T, chatID, userID int64) Member {
	t.Helper()
	m, err := GetMember(context.Background(), chatID, userID)
	if err != nil {
		t.Fatalf("GetMember(%d, %d) error = %v", chatID, userID, err)
	}
	return m
}

func countSettingsRows(t *testing.T, chatID int64) int64 {
	t.Helper()
	var count int64
	if err := db.DB.Model(&models.ProbationSettings{}).Where("chat_id = ?", chatID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	return count
}

func TestMissingSettingsReadAsOffWithDefaults(t *testing.T) {
	requireDB(t)
	cache.SetupTestMemoryMarshaler(t)
	chatID := time.Now().UnixNano()
	cleanupChat(t, chatID)

	got := mustSettings(t, chatID)
	if got.Enabled || got.Duration() != 24*time.Hour || got.MessageCount != 5 || got.MinLength != 20 ||
		!got.BlockLinks || !got.BlockForwards || got.BlockMedia || got.Notice {
		t.Fatalf("missing row = %+v, want off, 24h, 5 messages, 20 chars, links+forwards, no notice", got)
	}
	if n := countSettingsRows(t, chatID); n != 0 {
		t.Fatalf("read created %d rows, want 0", n)
	}
}

// Each setter writes its own columns and the next cached read sees it.
func TestSettersAreIndependentAndInvalidateCache(t *testing.T) {
	requireDB(t)
	cache.SetupTestMemoryMarshaler(t)
	chatID := time.Now().UnixNano()
	cleanupChat(t, chatID)

	_ = mustSettings(t, chatID) // prime the cache
	if err := SetEnabled(chatID, true); err != nil {
		t.Fatal(err)
	}
	if got := mustSettings(t, chatID); !got.Enabled || !got.BlockLinks || got.MessageCount != 5 {
		t.Fatalf("after enable = %+v, want enabled with defaults kept", got)
	}
	if err := SetBlocked(chatID, false, true, true); err != nil {
		t.Fatal(err)
	}
	if err := SetDuration(chatID, 2*time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := SetMessageCount(chatID, 3); err != nil {
		t.Fatal(err)
	}
	if err := SetMinLength(chatID, 7); err != nil {
		t.Fatal(err)
	}
	if err := SetNotice(chatID, true); err != nil {
		t.Fatal(err)
	}
	got := mustSettings(t, chatID)
	if !got.Enabled || got.BlockLinks || !got.BlockForwards || !got.BlockMedia || got.Duration() != 2*time.Hour ||
		got.MessageCount != 3 || got.MinLength != 7 || !got.Notice {
		t.Fatalf("after setters = %+v", got)
	}
	if err := SetEnabled(chatID, false); err != nil {
		t.Fatal(err)
	}
	if got := mustSettings(t, chatID); got.Enabled || got.MessageCount != 3 {
		t.Fatalf("after disable = %+v, want off with the other settings kept", got)
	}
	if n := countSettingsRows(t, chatID); n != 1 {
		t.Fatalf("rows for chat = %d, want 1 (setters must upsert)", n)
	}
}

// SetBlock changes one kind and keeps the others.
func TestSetBlockChangesOneKind(t *testing.T) {
	requireDB(t)
	cache.SetupTestMemoryMarshaler(t)
	chatID := time.Now().UnixNano()
	cleanupChat(t, chatID)

	if err := SetBlock(chatID, BlockMedia, true); err != nil {
		t.Fatal(err)
	}
	if err := SetBlock(chatID, BlockLinks, false); err != nil {
		t.Fatal(err)
	}
	if got := mustSettings(t, chatID); got.BlockLinks || !got.BlockForwards || !got.BlockMedia {
		t.Fatalf("after SetBlock = %+v, want forwards and media only", got)
	}
	if err := SetBlock(chatID, "stickers", true); !errors.Is(err, ErrInvalidBlock) {
		t.Fatalf("SetBlock(stickers) error = %v, want ErrInvalidBlock", err)
	}
}

func TestSettersRejectOutOfRangeValues(t *testing.T) {
	requireDB(t)
	cache.SetupTestMemoryMarshaler(t)
	chatID := time.Now().UnixNano()
	cleanupChat(t, chatID)

	for name, err := range map[string]error{
		"duration low":  SetDuration(chatID, 59*time.Minute),
		"duration high": SetDuration(chatID, 31*24*time.Hour),
		"messages low":  SetMessageCount(chatID, 0),
		"messages high": SetMessageCount(chatID, 101),
		"minlen low":    SetMinLength(chatID, 0),
		"minlen high":   SetMinLength(chatID, 501),
	} {
		if !errors.Is(err, ErrOutOfRange) {
			t.Errorf("%s: error = %v, want ErrOutOfRange", name, err)
		}
	}
	if n := countSettingsRows(t, chatID); n != 0 {
		t.Fatalf("rejected values created %d rows, want 0", n)
	}
}

// A cached "not on probation" must not hide a later join, and an ended
// probation must not stay cached.
func TestMemberLifecycleInvalidatesCache(t *testing.T) {
	requireDB(t)
	cache.SetupTestMemoryMarshaler(t)
	chatID := time.Now().UnixNano()
	cleanupChat(t, chatID)
	const userID = 4242

	if mustMember(t, chatID, userID).Found {
		t.Fatal("member found before Start")
	}
	joined := time.Now().Add(-time.Minute).Truncate(time.Second)
	if err := Start(chatID, userID, joined); err != nil {
		t.Fatal(err)
	}
	m := mustMember(t, chatID, userID)
	if !m.Found || !m.JoinedAt.Equal(joined) {
		t.Fatalf("after Start = %+v, want found, joined %v", m, joined)
	}
	if err := End(chatID, userID); err != nil {
		t.Fatal(err)
	}
	if mustMember(t, chatID, userID).Found {
		t.Fatal("member still found after End")
	}
	if err := End(chatID, userID); err != nil {
		t.Fatalf("End without a row error = %v, want nil", err)
	}
}

// The notice is claimed once per probation; a new join claims it again.
func TestClaimNoticeOncePerProbation(t *testing.T) {
	requireDB(t)
	cache.SetupTestMemoryMarshaler(t)
	chatID := time.Now().UnixNano()
	cleanupChat(t, chatID)
	const userID = 4243

	if ok, err := ClaimNotice(chatID, userID); err != nil || ok {
		t.Fatalf("claim without a row = %v, %v; want false", ok, err)
	}
	if err := Start(chatID, userID, time.Now()); err != nil {
		t.Fatal(err)
	}
	if member, err := GetMember(context.Background(), chatID, userID); err != nil || member.Notified {
		t.Fatalf("member before the claim = %+v, %v; want not notified", member, err)
	}
	if ok, err := ClaimNotice(chatID, userID); err != nil || !ok {
		t.Fatalf("first claim = %v, %v; want true", ok, err)
	}
	if member, err := GetMember(context.Background(), chatID, userID); err != nil || !member.Notified {
		t.Fatalf("cached member after the claim = %+v, %v; want notified", member, err)
	}
	if ok, err := ClaimNotice(chatID, userID); err != nil || ok {
		t.Fatalf("second claim = %v, %v; want false", ok, err)
	}
	if err := Start(chatID, userID, time.Now()); err != nil {
		t.Fatal(err)
	}
	if ok, err := ClaimNotice(chatID, userID); err != nil || !ok {
		t.Fatalf("claim after rejoin = %v, %v; want true", ok, err)
	}
}

func TestDeleteJoinedBeforeRemovesOnlyOlderRows(t *testing.T) {
	requireDB(t)
	cache.SetupTestMemoryMarshaler(t)
	chatID := time.Now().UnixNano()
	cleanupChat(t, chatID)
	now := time.Now()

	if err := Start(chatID, 1, now.Add(-40*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := Start(chatID, 2, now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := DeleteJoinedBefore(now.Add(-30 * 24 * time.Hour)); err != nil {
		t.Fatal(err)
	}
	var users []int64
	if err := db.DB.Model(&models.ProbationMember{}).Where("chat_id = ?", chatID).Pluck("user_id", &users).Error; err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 || users[0] != 2 {
		t.Fatalf("remaining members = %v, want [2]", users)
	}
}
