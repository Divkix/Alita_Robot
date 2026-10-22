//go:build testtools

package chattz

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

// newTZChat returns a chat ID no other test in this package uses.
func newTZChat(t *testing.T) int64 {
	t.Helper()
	chatID := -(time.Now().UnixNano() + testChatSeq.Add(1))
	t.Cleanup(func() {
		if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatTimezone{}).Error; err != nil {
			t.Errorf("cleanup timezone of chat %d: %v", chatID, err)
		}
	})
	return chatID
}

func countRows(t *testing.T, chatID int64) int64 {
	t.Helper()
	var n int64
	if err := db.DB.Model(&models.ChatTimezone{}).Where("chat_id = ?", chatID).Count(&n).Error; err != nil {
		t.Fatalf("count rows: %v", err)
	}
	return n
}

func TestParseAcceptsIANANamesAndRefusesTheRest(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		input, want string
	}{
		{"Asia/Kolkata", "Asia/Kolkata"},
		{"  Europe/London ", "Europe/London"},
		{"asia/kolkata", "Asia/Kolkata"},
		{"AMERICA/NEW_YORK", "America/New_York"},
		{"America/Port-au-Prince", "America/Port-au-Prince"},
		{"Etc/GMT+5", "Etc/GMT+5"},
		{"utc", "UTC"},
		{"UTC", "UTC"},
	} {
		got, loc, err := Parse(tt.input)
		if err != nil || got != tt.want || loc == nil {
			t.Errorf("Parse(%q) = %q, %v, %v; want %q", tt.input, got, loc, err, tt.want)
		}
	}

	for _, input := range []string{"", "   ", "Local", "local", "Mars/Olympus", "../etc/passwd", "/Asia/Kolkata",
		"Asia//Kolkata", "Asia/Kolkata ; drop", "+05:30", "Asia/Kolkata/"} {
		if got, _, err := Parse(input); !errors.Is(err, ErrUnknownTimezone) {
			t.Errorf("Parse(%q) = %q, %v; want ErrUnknownTimezone", input, got, err)
		}
	}
}

func TestChatWithoutRowIsOnUTCAndReadingInsertsNothing(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)
	chatID := newTZChat(t)

	name, err := Name(chatID)
	if err != nil || name != DefaultName {
		t.Fatalf("Name() = %q, %v; want %q", name, err, DefaultName)
	}
	if loc := Location(chatID); loc != time.UTC {
		t.Fatalf("Location() = %v, want UTC", loc)
	}
	if n := countRows(t, chatID); n != 0 {
		t.Fatalf("rows = %d, want 0: reading must not insert", n)
	}
}

func TestSetStoresTheCanonicalNameAndInvalidatesTheCache(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)
	chatID := newTZChat(t)

	// Warm the cache with the default first, so a missed invalidation shows.
	if loc := Location(chatID); loc != time.UTC {
		t.Fatalf("Location() before Set = %v, want UTC", loc)
	}

	stored, err := Set(chatID, "asia/kolkata")
	if err != nil || stored != "Asia/Kolkata" {
		t.Fatalf("Set() = %q, %v; want Asia/Kolkata", stored, err)
	}
	loc := Location(chatID)
	if loc.String() != "Asia/Kolkata" {
		t.Fatalf("Location() after Set = %v, want Asia/Kolkata", loc)
	}
	if _, offset := time.Date(2026, 1, 1, 0, 0, 0, 0, loc).Zone(); offset != 5*3600+30*60 {
		t.Fatalf("Asia/Kolkata offset = %d, want +05:30", offset)
	}

	if _, err := Set(chatID, "Europe/Berlin"); err != nil {
		t.Fatalf("Set(Europe/Berlin) error = %v", err)
	}
	if name, _ := Name(chatID); name != "Europe/Berlin" {
		t.Fatalf("Name() after second Set = %q, want Europe/Berlin", name)
	}
	if n := countRows(t, chatID); n != 1 {
		t.Fatalf("rows = %d, want 1 (writes must upsert)", n)
	}
}

func TestSetRefusesUnknownNamesWithoutWriting(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)
	chatID := newTZChat(t)

	if _, err := Set(chatID, "Mars/Olympus"); !errors.Is(err, ErrUnknownTimezone) {
		t.Fatalf("Set(Mars/Olympus) error = %v, want ErrUnknownTimezone", err)
	}
	if n := countRows(t, chatID); n != 0 {
		t.Fatalf("rows = %d, want 0 after a refused name", n)
	}
}

// A name the time zone database stops knowing (a stale row) must not break
// callers: Location falls back to UTC.
func TestLocationFallsBackToUTCForAStoredNameThatDoesNotLoad(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)
	chatID := newTZChat(t)

	if err := db.DB.Create(&models.ChatTimezone{ChatID: chatID, Timezone: "Mars/Olympus"}).Error; err != nil {
		t.Fatalf("seed row: %v", err)
	}
	if name, _ := Name(chatID); name != "Mars/Olympus" {
		t.Fatalf("Name() = %q, want the stored value", name)
	}
	if loc := Location(chatID); loc != time.UTC {
		t.Fatalf("Location() = %v, want the UTC fallback", loc)
	}
}
