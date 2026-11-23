//go:build testtools

package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	log "github.com/sirupsen/logrus"
	logrustest "github.com/sirupsen/logrus/hooks/test"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/i18n"
)

// initTestLocales serves the real locale files from disk through the i18n
// test override, as Run's locale init does in production, so a test compares
// against the real user-visible labels instead of empty strings.
func initTestLocales(t *testing.T) {
	t.Helper()
	entries, err := os.ReadDir("../../locales")
	if err != nil {
		t.Fatalf("read locales dir: %v", err)
	}
	locales := make(map[string]string, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yml") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join("../../locales", entry.Name()))
		if err != nil {
			t.Fatalf("read locale %s: %v", entry.Name(), err)
		}
		locales[strings.TrimSuffix(entry.Name(), ".yml")] = string(raw)
	}
	restore, err := i18n.OverrideLocalesForTest(locales)
	if err != nil {
		t.Fatalf("override locales: %v", err)
	}
	t.Cleanup(restore)
}

// With the panel enabled the default menu button becomes a Mini App button in
// every private chat; with it disabled the button is left untouched.
func TestPanelMenuButtonFollowsPanelConfig(t *testing.T) {
	initTestLocales(t)
	label, err := i18n.MustNewTranslator("en").GetString("panel_open_button")
	if err != nil || label == "" {
		t.Fatalf("panel_open_button = %q, %v; want a label", label, err)
	}

	previous := config.AppConfig
	config.AppConfig.PanelEnabled = false
	config.AppConfig.PanelPublicURL = "https://panel.example.com"
	t.Cleanup(func() { config.AppConfig = previous })

	disabled := &appBotClient{}
	setPanelMenuButton(&gotgbot.Bot{Token: "999:test", BotClient: disabled})
	if calls := disabled.callsFor("setChatMenuButton"); len(calls) != 0 {
		t.Fatalf("disabled panel sent %d setChatMenuButton calls, want none", len(calls))
	}

	config.AppConfig.PanelEnabled = true
	enabled := &appBotClient{}
	setPanelMenuButton(&gotgbot.Bot{Token: "999:test", BotClient: enabled})
	calls := enabled.callsFor("setChatMenuButton")
	if len(calls) != 1 {
		t.Fatalf("setChatMenuButton calls = %d, want 1", len(calls))
	}
	if got, ok := calls[0].params["chat_id"]; ok {
		t.Errorf("chat_id = %v, want no chat_id so the default button of every private chat changes", got)
	}
	button, ok := calls[0].params["menu_button"].(gotgbot.MenuButtonWebApp)
	if !ok {
		t.Fatalf("menu_button type = %T, want MenuButtonWebApp", calls[0].params["menu_button"])
	}
	if button.Text != label {
		t.Errorf("menu button text = %q, want %q", button.Text, label)
	}
	if button.WebApp.Url != "https://panel.example.com" {
		t.Errorf("menu button url = %q, want the configured panel URL", button.WebApp.Url)
	}

	// A restart repeats the same write, so the button never drifts.
	restart := &appBotClient{}
	setPanelMenuButton(&gotgbot.Bot{Token: "999:test", BotClient: restart})
	again := restart.callsFor("setChatMenuButton")
	if len(again) != 1 {
		t.Fatalf("restart setChatMenuButton calls = %d, want 1", len(again))
	}
	if got := again[0].params["menu_button"]; got != button {
		t.Errorf("restart menu button = %#v, want the same value %#v", got, button)
	}
}

// A refusal from Telegram is a warning, never a startup failure: the command
// menu and the group buttons keep working.
func TestPanelMenuButtonRefusalOnlyWarns(t *testing.T) {
	initTestLocales(t)
	previous := config.AppConfig
	config.AppConfig.PanelEnabled = true
	config.AppConfig.PanelPublicURL = "https://panel.example.com"
	t.Cleanup(func() { config.AppConfig = previous })

	previousLevel := log.GetLevel()
	log.SetLevel(log.DebugLevel)
	t.Cleanup(func() { log.SetLevel(previousLevel) })
	hook := logrustest.NewGlobal()
	t.Cleanup(hook.Reset)

	client := &appBotClient{failMethods: map[string]error{
		"setChatMenuButton": errors.New("Bad Request: BUTTON_URL_INVALID"),
	}}
	setPanelMenuButton(&gotgbot.Bot{Token: "999:test", BotClient: client})

	var warned bool
	for _, entry := range hook.AllEntries() {
		if entry.Level <= log.ErrorLevel {
			t.Fatalf("refusal logged at %s: %q", entry.Level, entry.Message)
		}
		if entry.Level == log.WarnLevel && strings.Contains(entry.Message, "menu button") {
			warned = true
		}
	}
	if !warned {
		t.Fatalf("no warning about the menu button; entries = %v", hook.AllEntries())
	}
}
