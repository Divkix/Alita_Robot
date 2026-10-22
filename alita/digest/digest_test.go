//go:build testtools

package digest

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/db/digestsettings"
	"github.com/divkix/Alita_Robot/alita/i18n"
)

// useDigestLocales serves the real locale files for the rest of the test, so
// the builder is checked against the strings users actually see.
func useDigestLocales(t *testing.T) {
	t.Helper()
	locales := map[string]string{}
	for _, code := range []string{"en", "es", "fr", "hi", "id", "pt", "ru"} {
		raw, err := os.ReadFile("../../locales/" + code + ".yml")
		if err != nil {
			t.Fatalf("read %s.yml: %v", code, err)
		}
		locales[code] = string(raw)
	}
	restore, err := i18n.OverrideLocalesForTest(locales)
	if err != nil {
		t.Fatalf("OverrideLocalesForTest: %v", err)
	}
	t.Cleanup(restore)
}

func TestWeekStartIsMondayMidnightUTC(t *testing.T) {
	cases := []struct {
		name string
		in   time.Time
		want time.Time
	}{
		{
			name: "monday afternoon",
			in:   time.Date(2026, 10, 19, 15, 30, 0, 0, time.UTC),
			want: time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "sunday night",
			in:   time.Date(2026, 10, 25, 23, 59, 0, 0, time.UTC),
			want: time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "wednesday",
			in:   time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC),
			want: time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC),
		},
		{
			// 01:30 in UTC+2 is still Sunday in UTC, so the completed week is
			// the one starting on the 12th.
			name: "local zone near midnight",
			in:   time.Date(2026, 10, 19, 1, 30, 0, 0, time.FixedZone("CEST", 2*60*60)),
			want: time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := WeekStart(tc.in)
			if !got.Equal(tc.want) {
				t.Fatalf("WeekStart(%v) = %v, want %v", tc.in, got, tc.want)
			}
			if _, offset := got.Zone(); offset != 0 {
				t.Fatalf("WeekStart(%v) has offset %d, want UTC", tc.in, offset)
			}
		})
	}
}

func TestResolveDestination(t *testing.T) {
	cases := []struct {
		name       string
		configured string
		hasLog     bool
		want       string
	}{
		{"auto with a log channel", digestsettings.DestinationAuto, true, digestsettings.DestinationLog},
		{"auto without a log channel", digestsettings.DestinationAuto, false, digestsettings.DestinationGroup},
		{"explicit log with a log channel", digestsettings.DestinationLog, true, digestsettings.DestinationLog},
		{"explicit log without a log channel", digestsettings.DestinationLog, false, digestsettings.DestinationGroup},
		{"explicit group with a log channel", digestsettings.DestinationGroup, true, digestsettings.DestinationGroup},
		{"explicit group without a log channel", digestsettings.DestinationGroup, false, digestsettings.DestinationGroup},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ResolveDestination(tc.configured, tc.hasLog); got != tc.want {
				t.Fatalf("ResolveDestination(%q, %v) = %q, want %q", tc.configured, tc.hasLog, got, tc.want)
			}
		})
	}
}

// TestBuildRendersCountsAndWeekOverWeekChanges pins the numbers a user sees:
// four counts, each with the localized change against the previous week, and
// one line naming the most frequent moderation action.
func TestBuildRendersCountsAndWeekOverWeekChanges(t *testing.T) {
	useDigestLocales(t)
	tr := i18n.MustNewTranslator("en")
	weekStart := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)

	current := Stats{Messages: 120, Joins: 5, Leaves: 2, Moderation: 3, TopAction: "ban"}
	previous := Stats{Messages: 100, Joins: 10, Leaves: 0, Moderation: 0}

	text, keyboard := Build(tr, current, previous, weekStart, "")
	if keyboard != nil {
		t.Fatalf("keyboard = %+v with no dashboard URL, want none", keyboard)
	}

	up, _ := tr.GetString("digest_change_up", i18n.TranslationParams{"percent": int64(20)})
	down, _ := tr.GetString("digest_change_down", i18n.TranslationParams{"percent": int64(50)})
	newChange, _ := tr.GetString("digest_change_new")

	for _, want := range []struct {
		key    string
		count  int64
		change string
	}{
		{"digest_message_messages", 120, up},
		{"digest_message_joins", 5, down},
		{"digest_message_leaves", 2, newChange},
		{"digest_message_moderation", 3, newChange},
	} {
		line, _ := tr.GetString(want.key, i18n.TranslationParams{"count": want.count, "change": want.change})
		if line == "" {
			t.Fatalf("locale returned an empty line for %q", want.key)
		}
		if !strings.Contains(text, line) {
			t.Errorf("digest %q does not contain %q", text, line)
		}
	}

	action, _ := tr.GetString("digest_action_ban")
	topLine, _ := tr.GetString("digest_message_top_action", i18n.TranslationParams{"action": action})
	if !strings.Contains(text, topLine) {
		t.Errorf("digest %q does not contain %q", text, topLine)
	}
	if !strings.Contains(text, weekStart.Format("2006-01-02")) {
		t.Errorf("digest %q does not name the reported week", text)
	}
	for _, invalid := range []string{"NaN", "+Inf", "-Inf"} {
		if strings.Contains(text, invalid) {
			t.Errorf("digest %q contains %q", text, invalid)
		}
	}
}

// A previous week of zero must read as new (activity) or an em dash (none),
// never as a division by zero.
func TestBuildHandlesAZeroPreviousWeek(t *testing.T) {
	useDigestLocales(t)
	tr := i18n.MustNewTranslator("en")
	weekStart := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)

	text, _ := Build(tr, Stats{Messages: 7}, Stats{}, weekStart, "")

	newChange, _ := tr.GetString("digest_change_new")
	noneChange, _ := tr.GetString("digest_change_none")
	if !strings.Contains(text, newChange) {
		t.Errorf("digest %q does not mark the new activity", text)
	}
	if !strings.Contains(text, noneChange) {
		t.Errorf("digest %q does not mark the unchanged zero metric", text)
	}
	for _, invalid := range []string{"NaN", "+Inf", "-Inf"} {
		if strings.Contains(text, invalid) {
			t.Errorf("digest %q contains %q", text, invalid)
		}
	}
}

func TestBuildEmptyWeekRendersTheEmptyMessage(t *testing.T) {
	useDigestLocales(t)
	tr := i18n.MustNewTranslator("en")
	weekStart := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)

	text, keyboard := Build(tr, Stats{}, Stats{Messages: 1000}, weekStart, "")
	want, _ := tr.GetString("digest_message_empty")
	if text != want {
		t.Fatalf("empty week text = %q, want %q", text, want)
	}
	if keyboard != nil {
		t.Fatalf("keyboard = %+v with no dashboard URL, want none", keyboard)
	}
}

func TestBuildAddsTheDashboardButtonOnlyWithAURL(t *testing.T) {
	useDigestLocales(t)
	tr := i18n.MustNewTranslator("en")
	weekStart := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
	const dashboardURL = "https://t.me/DigestTestBot?start=panel_-1001"

	_, keyboard := Build(tr, Stats{Messages: 1}, Stats{}, weekStart, dashboardURL)
	if keyboard == nil {
		t.Fatal("keyboard = nil with a dashboard URL, want a button")
	}
	if len(keyboard.InlineKeyboard) != 1 || len(keyboard.InlineKeyboard[0]) != 1 {
		t.Fatalf("keyboard = %+v, want one button", keyboard)
	}
	button := keyboard.InlineKeyboard[0][0]
	label, _ := tr.GetString("digest_dashboard_button")
	if button.Text != label {
		t.Fatalf("button text = %q, want %q", button.Text, label)
	}
	if button.Url != dashboardURL {
		t.Fatalf("button URL = %q, want %q", button.Url, dashboardURL)
	}
}

// The digest is sent in the chat's language, so a Spanish chat gets Spanish
// lines rather than English ones.
func TestBuildUsesTheGivenTranslator(t *testing.T) {
	useDigestLocales(t)
	es := i18n.MustNewTranslator("es")
	weekStart := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)

	text, _ := Build(es, Stats{Messages: 3}, Stats{}, weekStart, "")
	title, _ := es.GetString("digest_message_title")
	if !strings.Contains(text, title) {
		t.Fatalf("Spanish digest %q does not contain %q", text, title)
	}
}
