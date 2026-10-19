//go:build testtools

package spamdbsettings

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

func skipIfNoDb(t *testing.T) {
	if db.DB == nil {
		t.Fatal("test database was not initialized")
	}
}

func cleanupChat(t *testing.T, chatID int64) {
	t.Helper()
	t.Cleanup(func() {
		if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.SpamDBSettings{}).Error; err != nil {
			t.Fatalf("cleanup failed: %v", err)
		}
	})
}

func mustGet(t *testing.T, chatID int64) *models.SpamDBSettings {
	t.Helper()
	settings, err := GetSpamDBSettingsContext(context.Background(), chatID)
	if err != nil {
		t.Fatalf("GetSpamDBSettingsContext(%d) error = %v", chatID, err)
	}
	return settings
}

func TestSpamDBEnabledRoundTrip(t *testing.T) {
	skipIfNoDb(t)
	cache.SetupTestMemoryMarshaler(t)

	chatID := time.Now().UnixNano()
	cleanupChat(t, chatID)

	if mustGet(t, chatID).Enabled {
		t.Fatal("missing row = enabled, want disabled")
	}

	if err := SetSpamDBEnabled(chatID, true); err != nil {
		t.Fatalf("SetSpamDBEnabled(true) error = %v", err)
	}
	if !mustGet(t, chatID).Enabled {
		t.Fatal("after enable = disabled, want enabled")
	}

	if err := SetSpamDBEnabled(chatID, false); err != nil {
		t.Fatalf("SetSpamDBEnabled(false) error = %v", err)
	}
	if mustGet(t, chatID).Enabled {
		t.Fatal("after disable = enabled, want disabled")
	}

	var count int64
	if err := db.DB.Model(&models.SpamDBSettings{}).Where("chat_id = ?", chatID).Count(&count).Error; err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("rows for chat = %d, want 1 (toggle must upsert, not insert)", count)
	}
}

func TestSpamDBEnabledWriteInvalidatesCache(t *testing.T) {
	skipIfNoDb(t)
	cache.SetupTestMemoryMarshaler(t)

	chatID := time.Now().UnixNano()
	cleanupChat(t, chatID)

	if err := SetSpamDBEnabled(chatID, false); err != nil {
		t.Fatalf("SetSpamDBEnabled(false) error = %v", err)
	}
	// Prime the cache with the disabled value.
	if mustGet(t, chatID).Enabled {
		t.Fatal("primed read = enabled, want disabled")
	}

	// The next read must see the write even though the old value is cached.
	if err := SetSpamDBEnabled(chatID, true); err != nil {
		t.Fatalf("SetSpamDBEnabled(true) error = %v", err)
	}
	if !mustGet(t, chatID).Enabled {
		t.Fatal("read after enable returned the stale cached value")
	}
}

// One chat's switch must not leak into another chat.
func TestSpamDBEnabledIsPerChat(t *testing.T) {
	skipIfNoDb(t)
	cache.SetupTestMemoryMarshaler(t)

	chatID := time.Now().UnixNano()
	otherChat := chatID + 1
	cleanupChat(t, chatID)
	cleanupChat(t, otherChat)

	if err := SetSpamDBEnabled(chatID, true); err != nil {
		t.Fatalf("SetSpamDBEnabled(true) error = %v", err)
	}
	if mustGet(t, otherChat).Enabled {
		t.Fatal("other chat = enabled, want disabled")
	}
}

func TestSpamDBActionDefaultsToBan(t *testing.T) {
	skipIfNoDb(t)
	cache.SetupTestMemoryMarshaler(t)

	chatID := time.Now().UnixNano()
	cleanupChat(t, chatID)

	if got := mustGet(t, chatID).Action; got != ActionBan {
		t.Fatalf("missing row action = %q, want %q", got, ActionBan)
	}
	if err := SetSpamDBEnabled(chatID, true); err != nil {
		t.Fatalf("SetSpamDBEnabled(true) error = %v", err)
	}
	if got := mustGet(t, chatID).Action; got != ActionBan {
		t.Fatalf("new row action = %q, want %q", got, ActionBan)
	}
}

// The action and the switch are set by separate commands; neither may reset
// the other, and each write must reach the next cached read.
func TestSpamDBActionAndSwitchAreIndependent(t *testing.T) {
	skipIfNoDb(t)
	cache.SetupTestMemoryMarshaler(t)

	chatID := time.Now().UnixNano()
	cleanupChat(t, chatID)

	if err := SetSpamDBAction(chatID, ActionMute); err != nil {
		t.Fatalf("SetSpamDBAction(mute) error = %v", err)
	}
	if got := mustGet(t, chatID); got.Action != ActionMute || got.Enabled {
		t.Fatalf("after action = %+v, want mute and still disabled", got)
	}

	if err := SetSpamDBEnabled(chatID, true); err != nil {
		t.Fatalf("SetSpamDBEnabled(true) error = %v", err)
	}
	if got := mustGet(t, chatID); got.Action != ActionMute || !got.Enabled {
		t.Fatalf("after enable = %+v, want mute and enabled", got)
	}

	if err := SetSpamDBAction(chatID, ActionLog); err != nil {
		t.Fatalf("SetSpamDBAction(log) error = %v", err)
	}
	if got := mustGet(t, chatID); got.Action != ActionLog || !got.Enabled {
		t.Fatalf("after second action = %+v, want log and enabled", got)
	}
}

func TestSetSpamDBActionRejectsUnknownAction(t *testing.T) {
	skipIfNoDb(t)
	cache.SetupTestMemoryMarshaler(t)

	chatID := time.Now().UnixNano()
	cleanupChat(t, chatID)

	if err := SetSpamDBAction(chatID, "kick"); !errors.Is(err, ErrInvalidAction) {
		t.Fatalf("SetSpamDBAction(kick) error = %v, want ErrInvalidAction", err)
	}
	var count int64
	if err := db.DB.Model(&models.SpamDBSettings{}).Where("chat_id = ?", chatID).Count(&count).Error; err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("rows for chat = %d, want 0", count)
	}
}

// CAS is checked by default and lols.bot only after an admin opts in.
func TestSpamDBDatabasesDefaultToCASOnly(t *testing.T) {
	skipIfNoDb(t)
	cache.SetupTestMemoryMarshaler(t)

	chatID := time.Now().UnixNano()
	cleanupChat(t, chatID)

	if got := mustGet(t, chatID); !got.CheckCAS || got.CheckLols {
		t.Fatalf("missing row = cas %v lols %v, want CAS only", got.CheckCAS, got.CheckLols)
	}
	if err := SetSpamDBEnabled(chatID, true); err != nil {
		t.Fatalf("SetSpamDBEnabled(true) error = %v", err)
	}
	if got := mustGet(t, chatID); !got.CheckCAS || got.CheckLols {
		t.Fatalf("new row = cas %v lols %v, want CAS only", got.CheckCAS, got.CheckLols)
	}
}

// Each database is switched on its own, a switched-off database stays off
// (false must reach the row, not the column default), and the next cached
// read sees every write.
func TestSetSpamDBDatabaseRoundTrip(t *testing.T) {
	skipIfNoDb(t)
	cache.SetupTestMemoryMarshaler(t)

	chatID := time.Now().UnixNano()
	cleanupChat(t, chatID)

	steps := []struct {
		database  string
		on        bool
		cas, lols bool
	}{
		{DatabaseLols, false, true, false},
		{DatabaseLols, true, true, true},
		{DatabaseCAS, false, false, true},
		{DatabaseCAS, true, true, true},
		{DatabaseLols, false, true, false},
	}
	for _, s := range steps {
		if err := SetSpamDBDatabase(chatID, s.database, s.on); err != nil {
			t.Fatalf("SetSpamDBDatabase(%s, %v) error = %v", s.database, s.on, err)
		}
		if got := mustGet(t, chatID); got.CheckCAS != s.cas || got.CheckLols != s.lols {
			t.Fatalf("after %s=%v: cas %v lols %v, want cas %v lols %v", s.database, s.on, got.CheckCAS, got.CheckLols, s.cas, s.lols)
		}
	}
}

// A chat with the check on must always ask some database, so the last one
// cannot be switched off.
func TestSetSpamDBDatabaseKeepsOneDatabase(t *testing.T) {
	skipIfNoDb(t)
	cache.SetupTestMemoryMarshaler(t)

	chatID := time.Now().UnixNano()
	cleanupChat(t, chatID)

	if err := SetSpamDBDatabase(chatID, DatabaseCAS, false); !errors.Is(err, ErrLastDatabase) {
		t.Fatalf("switching off the only database: error = %v, want ErrLastDatabase", err)
	}
	if got := mustGet(t, chatID); !got.CheckCAS {
		t.Fatal("CAS was switched off although it was the only database")
	}

	if err := SetSpamDBDatabase(chatID, DatabaseLols, true); err != nil {
		t.Fatalf("SetSpamDBDatabase(lols, true) error = %v", err)
	}
	if err := SetSpamDBDatabase(chatID, DatabaseCAS, false); err != nil {
		t.Fatalf("SetSpamDBDatabase(cas, false) error = %v", err)
	}
	if err := SetSpamDBDatabase(chatID, DatabaseLols, false); !errors.Is(err, ErrLastDatabase) {
		t.Fatalf("switching off the last database: error = %v, want ErrLastDatabase", err)
	}
	if got := mustGet(t, chatID); got.CheckCAS || !got.CheckLols {
		t.Fatalf("after refusal: cas %v lols %v, want lols only", got.CheckCAS, got.CheckLols)
	}
}

// The database choice, the switch and the action are separate commands;
// none may reset another.
func TestSpamDBDatabasesSwitchAndActionAreIndependent(t *testing.T) {
	skipIfNoDb(t)
	cache.SetupTestMemoryMarshaler(t)

	chatID := time.Now().UnixNano()
	cleanupChat(t, chatID)

	if err := SetSpamDBEnabled(chatID, true); err != nil {
		t.Fatalf("SetSpamDBEnabled(true) error = %v", err)
	}
	if err := SetSpamDBAction(chatID, ActionLog); err != nil {
		t.Fatalf("SetSpamDBAction(log) error = %v", err)
	}
	if err := SetSpamDBDatabase(chatID, DatabaseLols, true); err != nil {
		t.Fatalf("SetSpamDBDatabase(lols, true) error = %v", err)
	}
	if err := SetSpamDBDatabase(chatID, DatabaseCAS, false); err != nil {
		t.Fatalf("SetSpamDBDatabase(cas, false) error = %v", err)
	}
	if got := mustGet(t, chatID); !got.Enabled || got.Action != ActionLog {
		t.Fatalf("after database changes = %+v, want enabled with log", got)
	}

	if err := SetSpamDBEnabled(chatID, false); err != nil {
		t.Fatalf("SetSpamDBEnabled(false) error = %v", err)
	}
	if err := SetSpamDBAction(chatID, ActionMute); err != nil {
		t.Fatalf("SetSpamDBAction(mute) error = %v", err)
	}
	if got := mustGet(t, chatID); got.CheckCAS || !got.CheckLols {
		t.Fatalf("after switch and action = cas %v lols %v, want lols only", got.CheckCAS, got.CheckLols)
	}
}

func TestSetSpamDBDatabaseRejectsUnknownDatabase(t *testing.T) {
	skipIfNoDb(t)
	cache.SetupTestMemoryMarshaler(t)

	chatID := time.Now().UnixNano()
	cleanupChat(t, chatID)

	if err := SetSpamDBDatabase(chatID, "spamwatch", true); !errors.Is(err, ErrInvalidDatabase) {
		t.Fatalf("SetSpamDBDatabase(spamwatch) error = %v, want ErrInvalidDatabase", err)
	}
	var count int64
	if err := db.DB.Model(&models.SpamDBSettings{}).Where("chat_id = ?", chatID).Count(&count).Error; err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("rows for chat = %d, want 0", count)
	}
}

// The schema itself refuses a row with no database, so two admins switching
// off different databases at once cannot leave the check asking nothing.
func TestSpamDBSettingsRowNeedsADatabase(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	cleanupChat(t, chatID)

	// A map, because GORM would swap the struct's false for the default.
	err := db.DB.Model(&models.SpamDBSettings{}).
		Create(map[string]any{"chat_id": chatID, "check_cas": false, "check_lols": false}).Error
	if err == nil {
		t.Fatal("a row with no database was stored")
	}
}

// Two admins switching off different databases at once: the second write was
// decided on a read that is already stale, so the schema's check refuses it.
// That refusal is the "last database" answer, not a failed save.
func TestStaleSwitchOffReadsAsLastDatabase(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	cleanupChat(t, chatID)
	for _, step := range []struct {
		database string
		on       bool
	}{{DatabaseLols, true}, {DatabaseCAS, false}} {
		if err := SetSpamDBDatabase(chatID, step.database, step.on); err != nil {
			t.Fatalf("SetSpamDBDatabase(%s, %v) error = %v", step.database, step.on, err)
		}
	}

	// What SetSpamDBDatabase(lols, off) writes after reading "CAS is on".
	stale := &models.SpamDBSettings{ChatID: chatID, CheckCAS: true, CheckLols: true}
	if err := writeDatabase(chatID, DatabaseLols, false, stale); !errors.Is(err, ErrLastDatabase) {
		t.Fatalf("writeDatabase() error = %v, want ErrLastDatabase", err)
	}
	if got := mustGet(t, chatID); got.CheckCAS || !got.CheckLols {
		t.Fatalf("stored cas %v lols %v, want only lols", got.CheckCAS, got.CheckLols)
	}

	// The raw upsert error is the schema's check, which is what that
	// answer rests on.
	err := upsert(chatID, "check_lols", map[string]any{"check_lols": false, "check_cas": true})
	if err == nil || !isLastDatabaseViolation(err) {
		t.Fatalf("upsert() error = %v, want the databases check violation", err)
	}
}

func TestIsLastDatabaseViolation(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"postgres check on the databases", &pgconn.PgError{Code: "23514", ConstraintName: "chk_spam_db_databases"}, true},
		{"postgres check wrapped", fmt.Errorf("save: %w", &pgconn.PgError{Code: "23514", ConstraintName: "chk_spam_db_databases"}), true},
		{"postgres check on the action", &pgconn.PgError{Code: "23514", ConstraintName: "chk_spam_db_action"}, false},
		{"postgres other violation of that name", &pgconn.PgError{Code: "23505", ConstraintName: "chk_spam_db_databases"}, false},
		{"postgres connection failure", &pgconn.PgError{Code: "08006"}, false},
		{"other error", errors.New("connection refused"), false},
	}
	for _, tc := range cases {
		if got := isLastDatabaseViolation(tc.err); got != tc.want {
			t.Errorf("%s: isLastDatabaseViolation() = %v, want %v", tc.name, got, tc.want)
		}
	}
}
