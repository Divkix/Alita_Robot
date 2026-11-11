//go:build testtools

package modules

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/disabling"
	"github.com/divkix/Alita_Robot/alita/db/locks"
	"github.com/divkix/Alita_Robot/alita/db/logchannels"
	"github.com/divkix/Alita_Robot/alita/db/models"
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

func TestSettingsSummaryReadsWithoutCreatingRows(t *testing.T) {
	chatID := newSettingsTestChat(t)

	sections := collectChatSettings(context.Background(), chatID)
	if len(sections) == 0 {
		t.Fatal("collectChatSettings() returned no sections")
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
		"chat_timezones":      &models.ChatTimezone{},
		"night_mode_settings": &models.NightModeSettings{},
	} {
		if n := db.CountRows(model, "chat_id = ?", chatID); n != 0 {
			t.Fatalf("summary created %d %s rows, want 0", n, name)
		}
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

func TestSettingsSummaryBlacklistModesShowEffectiveDuration(t *testing.T) {
	for _, tc := range []struct {
		mode       string
		actionTime int
		want       string
	}{
		{"tmute", 600, "tmute 10m"},
		{"tban", 86400, "tban 1d"},
		{"tmute", 0, "tmute 1h"},
		{"tban", 30, "tban 1h"},
		{"tmute", 366*86400 + 1, "tmute 1h"},
		{"ban", 0, "ban"},
	} {
		t.Run(tc.want, func(t *testing.T) {
			chatID := newSettingsTestChat(t)
			if err := db.DB.Create(&models.BlacklistSettings{ChatId: chatID, Word: "spam", Action: tc.mode, ActionTime: tc.actionTime}).Error; err != nil {
				t.Fatalf("Create(BlacklistSettings) error = %v", err)
			}
			for _, command := range []string{"/blaction"} {
				if got := settingsRowValue(t, chatID, command); got != tc.want {
					t.Errorf("%s row = %q, want %q", command, got, tc.want)
				}
			}
		})
	}
}

// An opt-in log category shows in /settings only once it is on, so a log
// channel that never turned one on reads as "all", as before it existed.
func TestSettingsSummaryShowsOptInLogCategoriesOnlyWhenOn(t *testing.T) {
	categoriesRow := func(chatID int64) settingsRow {
		t.Helper()
		cache.ResetLocalForTest()
		for _, row := range logChannelSettingsSection(chatID).Rows {
			if row.Command == "/logcategories" {
				return row
			}
		}
		t.Fatal("summary has no /logcategories row")
		return settingsRow{}
	}

	chatID := newSettingsTestChat(t)
	if err := logchannels.Set(chatID, "Settings Chat", chatID-1); err != nil {
		t.Fatalf("logchannels.Set() error = %v", err)
	}
	if got := categoriesRow(chatID); got.Value != "all" || got.Changed {
		t.Fatalf("fresh log channel /logcategories row = %+v, want unchanged value all", got)
	}

	if err := logchannels.SetCategory(chatID, logchannels.CategoryWarns, true); err != nil {
		t.Fatalf("SetCategory(warns) error = %v", err)
	}
	if got := categoriesRow(chatID); got.Value != "all + warns" || !got.Changed {
		t.Fatalf("/logcategories row = %+v, want changed value all + warns", got)
	}

	if err := logchannels.SetCategory(chatID, logchannels.CategoryAdmin, false); err != nil {
		t.Fatalf("SetCategory(admin) error = %v", err)
	}
	if got := categoriesRow(chatID); got.Value != "off: admin + warns" || !got.Changed {
		t.Fatalf("/logcategories row = %+v, want changed value off: admin + warns", got)
	}
}
