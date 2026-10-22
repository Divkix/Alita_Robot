//go:build testtools

package nightmode

import (
	"errors"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

var testChatSeq atomic.Int64

func newNightChat(t *testing.T) int64 {
	t.Helper()
	chatID := -(time.Now().UnixNano() + testChatSeq.Add(1))
	t.Cleanup(func() {
		if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.NightModeSettings{}).Error; err != nil {
			t.Errorf("cleanup night mode of chat %d: %v", chatID, err)
		}
	})
	return chatID
}

func mustGet(t *testing.T, chatID int64) *models.NightModeSettings {
	t.Helper()
	settings, err := Get(chatID)
	if err != nil {
		t.Fatalf("Get(%d) error = %v", chatID, err)
	}
	return settings
}

func countRows(t *testing.T, chatID int64) int64 {
	t.Helper()
	var n int64
	if err := db.DB.Model(&models.NightModeSettings{}).Where("chat_id = ?", chatID).Count(&n).Error; err != nil {
		t.Fatalf("count rows: %v", err)
	}
	return n
}

func TestChatWithoutRowHasNightModeOffAndReadingInsertsNothing(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)
	chatID := newNightChat(t)

	if settings := mustGet(t, chatID); settings.Enabled || ActiveAt(settings, time.Now()) {
		t.Fatalf("Get() = %+v, want night mode off", settings)
	}
	if err := Disable(chatID); err != nil {
		t.Fatalf("Disable() error = %v", err)
	}
	if n := countRows(t, chatID); n != 0 {
		t.Fatalf("rows = %d, want 0: reading and disabling must not insert", n)
	}
}

func TestEnableStoresAMidnightWindowAndInvalidatesTheCache(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)
	chatID := newNightChat(t)
	_ = mustGet(t, chatID) // warm the cache with the default

	// 00:00 is minute 0, which a struct insert would drop.
	if err := Enable(chatID, 0, 7*60, []string{"url", "media", "url"}); err != nil {
		t.Fatalf("Enable() error = %v", err)
	}
	settings := mustGet(t, chatID)
	if !settings.Enabled || settings.StartMinute != 0 || settings.EndMinute != 420 {
		t.Fatalf("Get() = %+v, want enabled 0..420", settings)
	}
	if got := settings.Types(); !slices.Equal(got, []string{"media", "url"}) {
		t.Fatalf("Types() = %v, want [media url] (sorted, no duplicates)", got)
	}

	if err := Enable(chatID, 23*60, 6*60, []string{"all"}); err != nil {
		t.Fatalf("second Enable() error = %v", err)
	}
	if settings := mustGet(t, chatID); settings.StartMinute != 1380 || settings.LockTypes != "all" {
		t.Fatalf("Get() after second Enable = %+v, want 1380 and all", settings)
	}

	if err := Disable(chatID); err != nil {
		t.Fatalf("Disable() error = %v", err)
	}
	settings = mustGet(t, chatID)
	if settings.Enabled || settings.StartMinute != 1380 || settings.LockTypes != "all" {
		t.Fatalf("Get() after Disable = %+v, want off with the window kept", settings)
	}
	if n := countRows(t, chatID); n != 1 {
		t.Fatalf("rows = %d, want 1 (writes must upsert)", n)
	}
}

func TestEnableRefusesBadWindowsAndEmptyTypes(t *testing.T) {
	cache.SetupTestMemoryMarshaler(t)
	chatID := newNightChat(t)

	for _, tt := range []struct {
		start, end int
		types      []string
		want       error
	}{
		{60, 60, []string{"url"}, ErrInvalidWindow},
		{-1, 60, []string{"url"}, ErrInvalidWindow},
		{60, MinutesPerDay, []string{"url"}, ErrInvalidWindow},
		{60, 120, nil, ErrNoLockTypes},
		{60, 120, []string{""}, ErrNoLockTypes},
	} {
		if err := Enable(chatID, tt.start, tt.end, tt.types); !errors.Is(err, tt.want) {
			t.Errorf("Enable(%d, %d, %v) error = %v, want %v", tt.start, tt.end, tt.types, err, tt.want)
		}
	}
	if n := countRows(t, chatID); n != 0 {
		t.Fatalf("rows = %d, want 0 after refused writes", n)
	}
}

func TestInWindowHandlesSameDayAndMidnightCrossingWindows(t *testing.T) {
	t.Parallel()
	at := func(h, m int) time.Time { return time.Date(2026, 6, 1, h, m, 0, 0, time.UTC) }

	for _, tt := range []struct {
		name       string
		start, end int
		now        time.Time
		want       bool
	}{
		{"same day, before", 9 * 60, 17 * 60, at(8, 59), false},
		{"same day, start is inside", 9 * 60, 17 * 60, at(9, 0), true},
		{"same day, end is outside", 9 * 60, 17 * 60, at(17, 0), false},
		{"crossing, evening", 23 * 60, 7 * 60, at(23, 30), true},
		{"crossing, after midnight", 23 * 60, 7 * 60, at(0, 0), true},
		{"crossing, last minute", 23 * 60, 7 * 60, at(6, 59), true},
		{"crossing, end is outside", 23 * 60, 7 * 60, at(7, 0), false},
		{"crossing, afternoon", 23 * 60, 7 * 60, at(15, 0), false},
		{"empty window", 60, 60, at(1, 0), false},
	} {
		if got := InWindow(tt.start, tt.end, tt.now); got != tt.want {
			t.Errorf("%s: InWindow(%d, %d, %s) = %v, want %v", tt.name, tt.start, tt.end, tt.now.Format("15:04"), got, tt.want)
		}
	}
}

// On 2026-03-08 New York's clocks jump from 02:00 to 03:00, and on
// 2026-11-01 they repeat 01:00-02:00. A start or end inside the skipped hour
// takes effect at 03:00, the next minute that exists; a window in the
// repeated hour applies both times through.
func TestInWindowFollowsTheWallClockAcrossDST(t *testing.T) {
	t.Parallel()
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}
	instant := func(utc string) time.Time {
		u, err := time.Parse(time.RFC3339, utc)
		if err != nil {
			t.Fatalf("parse %s: %v", utc, err)
		}
		return u.In(ny)
	}

	// 02:30-06:00 starts in the gap.
	start, end := 2*60+30, 6*60
	if InWindow(start, end, instant("2026-03-08T06:59:00Z")) { // 01:59 EST
		t.Fatal("window starting 02:30 is active at 01:59 before the jump")
	}
	if !InWindow(start, end, instant("2026-03-08T07:00:00Z")) { // 03:00 EDT
		t.Fatal("window starting 02:30 is not active at 03:00, the first minute after the gap")
	}

	// 22:00-02:30 ends in the gap.
	start, end = 22*60, 2*60+30
	if !InWindow(start, end, instant("2026-03-08T06:59:00Z")) { // 01:59 EST
		t.Fatal("window ending 02:30 is not active at 01:59")
	}
	if InWindow(start, end, instant("2026-03-08T07:00:00Z")) { // 03:00 EDT
		t.Fatal("window ending 02:30 is still active at 03:00, after the gap")
	}

	// 01:00-01:30 lies in the repeated hour.
	start, end = 1*60, 1*60+30
	for _, utc := range []string{"2026-11-01T05:15:00Z", "2026-11-01T06:15:00Z"} { // 01:15 EDT, 01:15 EST
		if !InWindow(start, end, instant(utc)) {
			t.Fatalf("window 01:00-01:30 is not active at %s (%s)", utc, instant(utc).Format("15:04 MST"))
		}
	}
}

func TestActiveAtNeedsEnabledAndTypes(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 1, 0, 30, 0, 0, time.UTC)
	on := &models.NightModeSettings{Enabled: true, StartMinute: 23 * 60, EndMinute: 7 * 60, LockTypes: "url"}
	if !ActiveAt(on, now) {
		t.Fatal("enabled window at 00:30 is not active")
	}
	off := *on
	off.Enabled = false
	noTypes := *on
	noTypes.LockTypes = ""
	for name, s := range map[string]*models.NightModeSettings{"disabled": &off, "no types": &noTypes, "nil": nil} {
		if ActiveAt(s, now) {
			t.Errorf("%s settings are active", name)
		}
	}
}
