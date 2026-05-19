package modules

import (
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

func TestParseDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		wantSec  int
		wantOk   bool
	}{
		{"minutes", "30m", 30 * 60, true},
		{"hours", "2h", 2 * 60 * 60, true},
		{"days", "1d", 24 * 60 * 60, true},
		{"weeks", "1w", 7 * 24 * 60 * 60, true},
		{"raw seconds", "3600", 3600, true},
		{"empty", "", 0, false},
		{"garbage", "abc", 0, false},
		{"negative minutes", "-5m", 0, false},
		{"uppercase", "1H", 3600, true},
		{"whitespace", "  5m  ", 5 * 60, true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := parseDuration(tc.input)
			if got != tc.wantSec || ok != tc.wantOk {
				t.Errorf("parseDuration(%q) = (%d, %v), want (%d, %v)", tc.input, got, ok, tc.wantSec, tc.wantOk)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    int
		expected string
	}{
		{60, "1m"},
		{3600, "1h"},
		{86400, "1d"},
		{604800, "1w"},
		{30, "30s"},
		{7200, "2h"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.expected, func(t *testing.T) {
			t.Parallel()
			got := formatDuration(tc.input)
			if got != tc.expected {
				t.Errorf("formatDuration(%d) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestAntiRaidStateMachine(t *testing.T) {
	if cache.Marshal == nil {
		t.Skip("requires Redis cache")
	}

	chatID := time.Now().UnixNano()

	// Initial state
	if antiRaidModule.isRaidActive(chatID) {
		t.Fatal("expected raid to be inactive initially")
	}

	// Enable
	antiRaidModule.enableRaid(chatID, 3600)
	if !antiRaidModule.isRaidActive(chatID) {
		t.Fatal("expected raid to be active after enable")
	}

	// Disable
	if !antiRaidModule.disableRaid(chatID) {
		t.Fatal("expected disableRaid to return true for active raid")
	}
	if antiRaidModule.isRaidActive(chatID) {
		t.Fatal("expected raid to be inactive after disable")
	}

	// Disable when already disabled
	if antiRaidModule.disableRaid(chatID) {
		t.Fatal("expected disableRaid to return false for already-inactive raid")
	}
}

func TestAntiRaidAutoExpiry(t *testing.T) {
	if cache.Marshal == nil {
		t.Skip("requires Redis cache")
	}

	chatID := time.Now().UnixNano() + 1

	antiRaidModule.enableRaid(chatID, 1) // 1 second
	if !antiRaidModule.isRaidActive(chatID) {
		t.Fatal("expected raid active immediately")
	}

	time.Sleep(2 * time.Second)
	if antiRaidModule.isRaidActive(chatID) {
		t.Fatal("expected raid expired after 1s duration")
	}
}

func TestAntiRaidExtend(t *testing.T) {
	if cache.Marshal == nil {
		t.Skip("requires Redis cache")
	}

	chatID := time.Now().UnixNano() + 2

	antiRaidModule.enableRaid(chatID, 3600)
	st := getRaidState(chatID)
	originalExpiry := st.ExpiresAt

	time.Sleep(100 * time.Millisecond)
	st.ExpiresAt = time.Now().Unix() + 7200
	if err := setRaidState(chatID, st); err != nil {
		t.Fatalf("setRaidState failed: %v", err)
	}

	st2 := getRaidState(chatID)
	if st2.ExpiresAt <= originalExpiry {
		t.Fatalf("expected extended expiry > original, got %d vs %d", st2.ExpiresAt, originalExpiry)
	}

	antiRaidModule.disableRaid(chatID)
}
