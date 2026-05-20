package modules

import (
	"slices"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

func TestModuleEnabled_StoreAndLoad(t *testing.T) {
	t.Parallel()

	var m moduleEnabled
	m.Init()

	// Store true and load
	m.Store("admin", true)
	_, got := m.Load("admin")
	if !got {
		t.Fatalf("Load(\"admin\") = false, want true after Store(\"admin\", true)")
	}

	// Load non-existent key returns false
	_, got = m.Load("nonexistent")
	if got {
		t.Fatalf("Load(\"nonexistent\") = true, want false")
	}

	// Overwrite with false
	m.Store("admin", false)
	_, got = m.Load("admin")
	if got {
		t.Fatalf("Load(\"admin\") = true, want false after Store(\"admin\", false)")
	}

	// Empty string key
	m.Store("", true)
	_, got = m.Load("")
	if !got {
		t.Fatalf("Load(\"\") = false, want true after Store(\"\", true)")
	}
}

func TestModuleEnabled_LoadModules(t *testing.T) {
	t.Parallel()

	t.Run("no stores returns empty slice", func(t *testing.T) {
		t.Parallel()

		var m moduleEnabled
		m.Init()

		result := m.LoadModules()
		if len(result) != 0 {
			t.Fatalf("LoadModules() with no stores = %v (len %d), want empty slice", result, len(result))
		}
	})

	t.Run("enabled modules returned, disabled excluded", func(t *testing.T) {
		t.Parallel()

		var m moduleEnabled
		m.Init()
		m.Store("a", true)
		m.Store("b", true)
		m.Store("c", false)

		result := m.LoadModules()
		if len(result) != 2 {
			t.Fatalf("LoadModules() = %v (len %d), want 2 elements", result, len(result))
		}
		if !slices.Contains(result, "a") {
			t.Fatalf("LoadModules() = %v, want to contain \"a\"", result)
		}
		if !slices.Contains(result, "b") {
			t.Fatalf("LoadModules() = %v, want to contain \"b\"", result)
		}
		if slices.Contains(result, "c") {
			t.Fatalf("LoadModules() = %v, must not contain \"c\" (disabled)", result)
		}
	})
}

func TestListModules(t *testing.T) {
	t.Parallel()

	helpRegistry := NewHelpRegistry()
	helpRegistry.AbleMap.Store("admin", true)
	helpRegistry.AbleMap.Store("filters", true)
	helpRegistry.AbleMap.Store("help", true)

	result := listModulesFrom(helpRegistry)

	if len(result) != 3 {
		t.Fatalf("listModules() = %v (len %d), want 3 elements", result, len(result))
	}

	// Result must be sorted alphabetically.
	expected := []string{"admin", "filters", "help"}
	for i, name := range expected {
		if result[i] != name {
			t.Fatalf("listModules()[%d] = %q, want %q (not sorted); full result: %v", i, result[i], name, result)
		}
	}
}

func TestGetAltNamesOfModuleIncludesLowercaseModuleName(t *testing.T) {
	t.Parallel()

	got := getAltNamesOfModule("DefinitelyNotInConfig")
	if len(got) == 0 {
		t.Fatal("getAltNamesOfModule() returned empty slice")
	}
	if got[len(got)-1] != "definitelynotinconfig" {
		t.Fatalf("last alias = %q, want lowercase module name", got[len(got)-1])
	}
}

func TestInitHelpButtonsBuildsSortedKeyboard(t *testing.T) {
	registry := DefaultHelpRegistry()
	previousAbleMap := registry.AbleMap
	previousMarkup := markup
	registry.AbleMap.Init()
	t.Cleanup(func() {
		registry.AbleMap = previousAbleMap
		markup = previousMarkup
	})

	registry.AbleMap.Store("Warns", true)
	registry.AbleMap.Store("Admin", true)
	registry.AbleMap.Store("Filters", true)

	initHelpButtons()
	if len(markup.InlineKeyboard) == 0 {
		t.Fatal("initHelpButtons() produced no rows")
	}
	firstRow := markup.InlineKeyboard[0]
	if len(firstRow) != 3 {
		t.Fatalf("first keyboard row len = %d, want 3", len(firstRow))
	}
	if firstRow[0].Text != "Admin" || firstRow[1].Text != "Filters" || firstRow[2].Text != "Warns" {
		t.Fatalf("first keyboard row = %#v, want Admin/Filters/Warns sorted", firstRow)
	}
	for _, button := range firstRow {
		if button.CallbackData == "" {
			t.Fatalf("button %q has empty callback data", button.Text)
		}
	}
}

func TestModuleHelpLookupRenderingAndSend(t *testing.T) {
	previousRegistry := defaultHelpRegistry
	previousMarkup := markup
	defaultHelpRegistry = NewHelpRegistry()
	t.Cleanup(func() {
		defaultHelpRegistry = previousRegistry
		markup = previousMarkup
	})

	registry := DefaultHelpRegistry()
	registry.AbleMap.Store("Admin", true)
	registry.helpableKb["Admin"] = [][]gotgbot.InlineKeyboardButton{
		{{Text: "Admin", CallbackData: "admin-test"}},
	}
	initHelpButtons()

	if got := getModuleNameFromAltName("admin"); got != "Admin" {
		t.Fatalf("getModuleNameFromAltName() = %q, want Admin", got)
	}

	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "private", FirstName: "Tester"}
	user := gotgbot.User{Id: 42, FirstName: "Tester"}
	ctx := newModuleMessageContext(bot, chat, user, "/help admin")

	helpText, kb, parseMode := getHelpTextAndMarkup(ctx, "admin")
	if parseMode != helpers.HTML {
		t.Fatalf("parseMode = %q, want HTML", parseMode)
	}
	if !strings.Contains(helpText, "Admin") {
		t.Fatalf("helpText = %q, want Admin header", helpText)
	}
	if len(kb.InlineKeyboard) < 2 {
		t.Fatalf("inline keyboard rows = %d, want module row plus navigation", len(kb.InlineKeyboard))
	}

	if _, err := sendHelpkb(bot, ctx, "admin"); err != nil {
		t.Fatalf("sendHelpkb() error = %v", err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
}

func TestStartHelpPrefixHandlerRoutesHelpDeepLink(t *testing.T) {
	previousRegistry := defaultHelpRegistry
	previousMarkup := markup
	defaultHelpRegistry = NewHelpRegistry()
	t.Cleanup(func() {
		defaultHelpRegistry = previousRegistry
		markup = previousMarkup
	})

	registry := DefaultHelpRegistry()
	registry.AbleMap.Store("Admin", true)
	registry.helpableKb["Admin"] = [][]gotgbot.InlineKeyboardButton{
		{{Text: "Admin", CallbackData: "admin-test"}},
	}
	initHelpButtons()

	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "private", FirstName: "Tester"}
	user := gotgbot.User{Id: 42, FirstName: "Tester"}
	ctx := newModuleMessageContext(bot, chat, user, "/start help_admin")

	if err := startHelpPrefixHandler(bot, ctx, &user, "help_admin"); err != ext.EndGroups {
		t.Fatalf("startHelpPrefixHandler() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want help deep-link response", len(calls))
	}
}
