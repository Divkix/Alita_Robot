//go:build testtools

package modules

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/captcha"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/disabling"
	"github.com/divkix/Alita_Robot/alita/db/greetings"
	"github.com/divkix/Alita_Robot/alita/db/locks"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/warns"
	"github.com/divkix/Alita_Robot/alita/i18n"
)

func newSettingsTestChat(t *testing.T) int64 {
	t.Helper()
	chatID := -uniqueModuleChatID()
	if err := chats.EnsureChatInDb(chatID, "Settings Chat"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	return chatID
}

func changedSettingsCommands(sections []settingsSection) []string {
	var changed []string
	for _, section := range sections {
		for _, row := range section.Rows {
			if row.Changed {
				changed = append(changed, row.Command)
			}
		}
	}
	return changed
}

func TestSettingsSummaryFreshChatHasNoChangedRows(t *testing.T) {
	chatID := newSettingsTestChat(t)

	sections := collectChatSettings(context.Background(), chatID)
	if len(sections) == 0 {
		t.Fatal("collectChatSettings() returned no sections")
	}
	if changed := changedSettingsCommands(sections); len(changed) != 0 {
		t.Fatalf("fresh chat has changed rows %v, want none", changed)
	}

	for name, model := range map[string]any{
		"greetings":           &models.GreetingSettings{},
		"report_chat":         &models.ReportChatSettings{},
		"rules":               &models.RulesSettings{},
		"pins":                &models.PinSettings{},
		"admin":               &models.AdminSettings{},
		"connection_settings": &models.ConnectionChatSettings{},
		"notes_settings":      &models.NotesSettings{},
		"warn_settings":       &models.WarnSettings{},
	} {
		if n := db.CountRows(model, "chat_id = ?", chatID); n != 0 {
			t.Fatalf("summary created %d %s rows, want 0", n, name)
		}
	}
}

func TestSettingsSummaryMarksExactlyChangedFields(t *testing.T) {
	chatID := newSettingsTestChat(t)

	if err := captcha.SetCaptchaEnabled(chatID, true); err != nil {
		t.Fatalf("SetCaptchaEnabled() error = %v", err)
	}
	if err := warns.SetWarnLimit(chatID, 5); err != nil {
		t.Fatalf("SetWarnLimit() error = %v", err)
	}
	if err := greetings.SetGoodbyeToggle(chatID, false); err != nil {
		t.Fatalf("SetGoodbyeToggle() error = %v", err)
	}
	cache.ResetLocalForTest()

	got := changedSettingsCommands(collectChatSettings(context.Background(), chatID))
	want := []string{"/captcha", "/setwarnlimit", "/goodbye"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("changed rows = %v, want %v", got, want)
	}
}

func TestRenderSettingsSummaryEscapesAndFits(t *testing.T) {
	chatID := newSettingsTestChat(t)
	tr := i18n.MustNewTranslator("en")

	for perm := range lockMap {
		if err := locks.UpdateLock(chatID, perm, true); err != nil {
			t.Fatalf("UpdateLock(%s) error = %v", perm, err)
		}
	}
	for perm := range restrMap {
		if err := locks.UpdateLock(chatID, perm, true); err != nil {
			t.Fatalf("UpdateLock(%s) error = %v", perm, err)
		}
	}
	for i := range 60 {
		if err := disabling.DisableCMD(chatID, "longcommandname"+strings.Repeat("x", i%7)+string(rune('a'+i%26))+string(rune('a'+i/26))); err != nil {
			t.Fatalf("DisableCMD() error = %v", err)
		}
	}
	cache.ResetLocalForTest()

	text := renderSettingsSummary(tr, "a<b&c", collectChatSettings(context.Background(), chatID))
	if !strings.Contains(text, "a&lt;b&amp;c") {
		t.Fatalf("summary does not escape the chat title:\n%s", text)
	}
	if n := utf8.RuneCountInString(text); n > 4096 {
		t.Fatalf("summary length = %d runes, want <= 4096", n)
	}
	if !strings.Contains(text, "•") {
		t.Fatalf("summary with locks has no changed marker:\n%s", text)
	}
}

// warn_mode is nullable and both CHECKs on it pass NULL, so legacy rows can
// hold NULL; it loads as an empty WarnMode, for which warnUser takes no action
// at the limit, so the summary must not report the default mute for it.
func TestSettingsSummaryNullWarnModeIsNotMute(t *testing.T) {
	chatID := newSettingsTestChat(t)
	if err := db.DB.Create(&models.WarnSettings{ChatId: chatID, WarnLimit: 3, WarnMode: "mute"}).Error; err != nil {
		t.Fatalf("Create(WarnSettings) error = %v", err)
	}
	if err := db.DB.Exec("UPDATE warns_settings SET warn_mode = NULL WHERE chat_id = ?", chatID).Error; err != nil {
		t.Fatalf("set warn_mode NULL: %v", err)
	}
	cache.ResetLocalForTest()

	for _, section := range collectChatSettings(context.Background(), chatID) {
		for _, row := range section.Rows {
			if row.Command == "/setwarnmode" {
				if row.Value != "none" || !row.Changed {
					t.Fatalf("/setwarnmode row = %+v, want changed value none", row)
				}
				return
			}
		}
	}
	t.Fatal("summary has no /setwarnmode row")
}

// An admin who set /setwarntime must see the expiry in /settings, or they
// would read the summary as warns never expiring.
func TestSettingsSummaryShowsWarnExpiry(t *testing.T) {
	warnExpiryRow := func(chatID int64) *settingsRow {
		t.Helper()
		for _, section := range collectChatSettings(context.Background(), chatID) {
			for i := range section.Rows {
				if section.Rows[i].Command == "/setwarntime" {
					return &section.Rows[i]
				}
			}
		}
		t.Fatal("summary has no /setwarntime row")
		return nil
	}

	chatID := newSettingsTestChat(t)
	if got := warnExpiryRow(chatID); got.Value != "off" || got.Changed {
		t.Fatalf("fresh chat /setwarntime row = %+v, want unchanged value off", *got)
	}

	if err := warns.SetWarnExpiry(chatID, 30*24*60*60); err != nil {
		t.Fatalf("SetWarnExpiry() error = %v", err)
	}
	cache.ResetLocalForTest()

	if got := warnExpiryRow(chatID); got.Value != "30d" || !got.Changed {
		t.Fatalf("/setwarntime row = %+v, want changed value 30d", *got)
	}
}

// tban/tmute act for warn_action_time, so the summary shows it with the mode;
// a row without a usable duration shows the 1h warnUser falls back to.
func TestSettingsSummaryTimedWarnModeShowsDuration(t *testing.T) {
	for _, tc := range []struct {
		mode       string
		actionTime int
		want       string
	}{
		{"tban", 2 * 60 * 60, "tban 2h"},
		{"tmute", 0, "tmute 1h"},
		{"ban", 0, "ban"},
	} {
		chatID := newSettingsTestChat(t)
		if err := db.DB.Create(&models.WarnSettings{ChatId: chatID, WarnLimit: 3, WarnMode: tc.mode, WarnActionTime: tc.actionTime}).Error; err != nil {
			t.Fatalf("Create(WarnSettings) error = %v", err)
		}
		cache.ResetLocalForTest()

		var got *settingsRow
		for _, section := range collectChatSettings(context.Background(), chatID) {
			for i := range section.Rows {
				if section.Rows[i].Command == "/setwarnmode" {
					got = &section.Rows[i]
				}
			}
		}
		if got == nil {
			t.Fatal("summary has no /setwarnmode row")
		}
		if got.Value != tc.want || !got.Changed {
			t.Errorf("%s/%d: /setwarnmode row = %+v, want changed value %q", tc.mode, tc.actionTime, *got, tc.want)
		}
	}
}
