//go:build testtools

package modules

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/connections"
	"github.com/divkix/Alita_Robot/alita/db/notes"
	"github.com/divkix/Alita_Robot/alita/db/rules"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
)

// deepLinkTestYAML holds the minimal translation strings needed to distinguish
// rejection messages from served content in the deep-link security tests.
// It is used by OverrideManagerForTest so that MustNewTranslator (called inside
// the production handlers) returns a real translator instead of a bare stub.
const deepLinkTestYAML = `
helpers_chat_not_found: "Could not find the chat"
helpers_invalid_deep_link: "Invalid deep link"
rules_for_chat: "Rules for <b>%s</b>:\n\n%s"
rules_not_set: "No rules set"
helpers_notes_current_header: "Current notes:\n"
notes_none_in_chat: "No notes in this chat"
helpers_note_not_exist: "Note does not exist"
helpers_note_admin_only: "Admin only note"
helpers_person_no_name: "PersonWithNoName"
`

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

func TestListModulesViaDefaultRegistry(t *testing.T) {
	previousRegistry := defaultHelpRegistry
	defaultHelpRegistry = NewHelpRegistry()
	defaultHelpRegistry.AbleMap.Store("Bans", true)
	defaultHelpRegistry.AbleMap.Store("Admin", true)
	defaultHelpRegistry.AbleMap.Store("Filters", true)
	defer func() {
		defaultHelpRegistry = previousRegistry
	}()

	result := listModules()
	if len(result) != 3 {
		t.Fatalf("listModules() = %v (len %d), want 3 elements", result, len(result))
	}

	expected := []string{"Admin", "Bans", "Filters"}
	for i, name := range expected {
		if result[i] != name {
			t.Fatalf("listModules()[%d] = %q, want %q; full result: %v", i, result[i], name, result)
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

	if got := getModuleNameFromAltName("admin", DefaultHelpRegistry()); got != "Admin" {
		t.Fatalf("getModuleNameFromAltName() = %q, want Admin", got)
	}

	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "private", FirstName: "Tester"}
	user := gotgbot.User{Id: 42, FirstName: "Tester"}
	ctx := newModuleMessageContext(bot, chat, user, "/help admin")

	helpText, kb, parseMode := getHelpTextAndMarkup(ctx, "admin", DefaultHelpRegistry())
	if parseMode != formatting.HTML {
		t.Fatalf("parseMode = %q, want HTML", parseMode)
	}
	if !strings.Contains(helpText, "Admin") {
		t.Fatalf("helpText = %q, want Admin header", helpText)
	}
	if len(kb.InlineKeyboard) < 2 {
		t.Fatalf("inline keyboard rows = %d, want module row plus navigation", len(kb.InlineKeyboard))
	}

	if _, err := sendHelpkb(bot, ctx, "admin", DefaultHelpRegistry()); err != nil {
		t.Fatalf("sendHelpkb() error = %v", err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
}

func TestHandleDeepLinkRoutesHelpDeepLink(t *testing.T) {
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

	if err := HandleDeepLink(bot, ctx, &user, "help_admin"); err != ext.EndGroups {
		t.Fatalf("HandleDeepLink() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want help deep-link response", len(calls))
	}
}

func TestHandleDeepLinkRoutesConnectAndRulesDeepLinks(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chatID := uniqueModuleChatID()
	client.responses["getChat"] = []byte(fmt.Sprintf(
		`{"id":%d,"type":"supergroup","title":"Deep Link Chat"}`,
		chatID,
	))
	chat := gotgbot.Chat{Id: 42, Type: "private", FirstName: "Tester"}
	user := gotgbot.User{Id: 42, FirstName: "Tester"}
	if err := chats.EnsureChatInDb(chatID, "Deep Link Chat"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	connections.ToggleAllowConnect(chatID, true)
	rules.SetChatRules(chatID, "Be kind.")
	t.Cleanup(func() {
		connections.ToggleAllowConnect(chatID, false)
		rules.SetChatRules(chatID, "")
	})

	connectCtx := newModuleMessageContext(bot, chat, user, fmt.Sprintf("/start connect_%d", chatID))
	if err := HandleDeepLink(bot, connectCtx, &user, fmt.Sprintf("connect_%d", chatID)); err != ext.EndGroups {
		t.Fatalf("HandleDeepLink(connect) error = %v, want EndGroups", err)
	}
	if connection := connections.Connection(user.Id); !connection.Connected || connection.ChatId != chatID {
		t.Fatalf("connection = %#v, want connected chat %d", connection, chatID)
	}

	rulesCtx := newModuleMessageContext(bot, chat, user, fmt.Sprintf("/start rules_%d", chatID))
	if err := HandleDeepLink(bot, rulesCtx, &user, fmt.Sprintf("rules_%d", chatID)); err != ext.EndGroups {
		t.Fatalf("HandleDeepLink(rules) error = %v, want EndGroups", err)
	}

	if calls := client.callsFor("sendMessage"); len(calls) != 2 {
		t.Fatalf("sendMessage calls = %d, want connect and rules responses", len(calls))
	}
}

func TestHandleDeepLinkRoutesNotesDeepLinks(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chatID := uniqueModuleChatID()
	client.responses["getChat"] = []byte(fmt.Sprintf(
		`{"id":%d,"type":"supergroup","title":"Notes Chat"}`,
		chatID,
	))
	chat := gotgbot.Chat{Id: 42, Type: "private", FirstName: "Tester"}
	user := gotgbot.User{Id: 42, FirstName: "Tester"}
	if err := chats.EnsureChatInDb(chatID, "Notes Chat"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	if err := notes.AddNote(chatID, "public", "Visible note", "", nil, db.TEXT, false, false, false, false, false, false); err != nil {
		t.Fatalf("AddNote(public) error = %v", err)
	}
	if err := notes.AddNote(chatID, "admin", "Hidden note", "", nil, db.TEXT, false, false, true, false, false, false); err != nil {
		t.Fatalf("AddNote(admin) error = %v", err)
	}

	listCtx := newModuleMessageContext(bot, chat, user, fmt.Sprintf("/start notes_%d", chatID))
	if err := HandleDeepLink(bot, listCtx, &user, fmt.Sprintf("notes_%d", chatID)); err != ext.EndGroups {
		t.Fatalf("HandleDeepLink(notes list) error = %v, want EndGroups", err)
	}

	noteCtx := newModuleMessageContext(bot, chat, user, fmt.Sprintf("/start note_%d_public", chatID))
	if err := HandleDeepLink(bot, noteCtx, &user, fmt.Sprintf("note_%d_public", chatID)); err != ext.EndGroups {
		t.Fatalf("HandleDeepLink(note) error = %v, want EndGroups", err)
	}

	missingCtx := newModuleMessageContext(bot, chat, user, fmt.Sprintf("/start note_%d_missing", chatID))
	if err := HandleDeepLink(bot, missingCtx, &user, fmt.Sprintf("note_%d_missing", chatID)); err != ext.EndGroups {
		t.Fatalf("HandleDeepLink(missing note) error = %v, want EndGroups", err)
	}

	if calls := client.callsFor("sendMessage"); len(calls) != 3 {
		t.Fatalf("sendMessage calls = %d, want list, note, and missing-note responses", len(calls))
	}
}

func TestHandleDeepLinkHandlesMissingChatsAndAdminOnlyNotes(t *testing.T) {
	user := gotgbot.User{Id: 42, FirstName: "Tester"}
	privateChat := gotgbot.Chat{Id: 42, Type: "private", FirstName: "Tester"}

	for _, arg := range []string{"connect_404", "rules_404", "notes_404"} {
		t.Run(arg, func(t *testing.T) {
			client := newModuleBotClient()
			client.errors["getChat"] = fmt.Errorf("chat not found")
			bot := newModuleTestBot(client)
			ctx := newModuleMessageContext(bot, privateChat, user, "/start "+arg)

			if err := HandleDeepLink(bot, ctx, &user, arg); err != ext.EndGroups {
				t.Fatalf("HandleDeepLink(%q) error = %v, want EndGroups", arg, err)
			}
			if calls := client.callsFor("sendMessage"); len(calls) != 1 {
				t.Fatalf("sendMessage calls = %d, want chat-not-found reply", len(calls))
			}
		})
	}

	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chatID := uniqueModuleChatID()
	client.responses["getChat"] = []byte(fmt.Sprintf(
		`{"id":%d,"type":"supergroup","title":"Private Notes Chat"}`,
		chatID,
	))
	if err := chats.EnsureChatInDb(chatID, "Private Notes Chat"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	if err := notes.AddNote(chatID, "adminonly", "Hidden", "", nil, db.TEXT, false, false, true, false, false, false); err != nil {
		t.Fatalf("AddNote(adminonly) error = %v", err)
	}

	ctx := newModuleMessageContext(bot, privateChat, user, fmt.Sprintf("/start note_%d_adminonly", chatID))
	if err := HandleDeepLink(bot, ctx, &user, fmt.Sprintf("note_%d_adminonly", chatID)); err != ext.ContinueGroups {
		t.Fatalf("HandleDeepLink(admin-only note) error = %v, want ContinueGroups", err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want admin-only notice", len(calls))
	}
}

func TestHandleDeepLinkRejectsInvalidDeepLinks(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "private", FirstName: "Tester"}
	user := gotgbot.User{Id: 42, FirstName: "Tester"}

	for _, arg := range []string{
		"connect_bad",
		"rules_bad",
		"note_bad",
		"note_123",
	} {
		t.Run(arg, func(t *testing.T) {
			ctx := newModuleMessageContext(bot, chat, user, "/start "+arg)
			if err := HandleDeepLink(bot, ctx, &user, arg); err != ext.EndGroups {
				t.Fatalf("HandleDeepLink(%q) error = %v, want EndGroups", arg, err)
			}
		})
	}

	if calls := client.callsFor("sendMessage"); len(calls) != 4 {
		t.Fatalf("sendMessage calls = %d, want one invalid-link reply per arg", len(calls))
	}
}

func TestHandleDeepLinkSendsAboutAndDefaultHelp(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "private", FirstName: "Tester"}
	user := gotgbot.User{Id: 42, FirstName: "Tester"}

	aboutCtx := newModuleMessageContext(bot, chat, user, "/start about")
	if err := HandleDeepLink(bot, aboutCtx, &user, "about"); err != ext.EndGroups {
		t.Fatalf("HandleDeepLink(about) error = %v, want EndGroups", err)
	}

	defaultCtx := newModuleMessageContext(bot, chat, user, "/start unknown")
	if err := HandleDeepLink(bot, defaultCtx, &user, "unknown"); err != ext.EndGroups {
		t.Fatalf("HandleDeepLink(default) error = %v, want EndGroups", err)
	}

	if calls := client.callsFor("sendMessage"); len(calls) != 2 {
		t.Fatalf("sendMessage calls = %d, want about and default help messages", len(calls))
	}
}

// TestGetModuleHelpAndKb_UsesPassedRegistry proves that getModuleHelpAndKb honors
// the passed *moduleStruct registry instead of reaching for the global HelpModule.
func TestGetModuleHelpAndKb_UsesPassedRegistry(t *testing.T) {
	localRegistry := NewHelpRegistry()
	localRegistry.AbleMap.Store("Admin", true)
	localRegistry.helpableKb["Admin"] = [][]gotgbot.InlineKeyboardButton{
		{{Text: "FakeAdminBtn", CallbackData: "test-admin"}},
		{{Text: "FakeAdminBtn2", CallbackData: "test-admin2"}},
	}

	// HelpModule has no "Admin" buttons, or different ones.
	_, kb := getModuleHelpAndKb("admin", "en", localRegistry)
	if len(kb.InlineKeyboard) < 2 {
		t.Fatalf("inline keyboard rows = %d, want at least module row + navigation", len(kb.InlineKeyboard))
	}

	// Verify the first row comes from the local registry.
	firstRow := kb.InlineKeyboard[0]
	if len(firstRow) != 1 {
		t.Fatalf("first module row len = %d, want 1", len(firstRow))
	}
	if firstRow[0].Text != "FakeAdminBtn" {
		t.Fatalf("button[0].Text = %q, want FakeAdminBtn (from local registry)", firstRow[0].Text)
	}
	if firstRow[0].CallbackData != "test-admin" {
		t.Fatalf("button[0].CallbackData = %q, want test-admin", firstRow[0].CallbackData)
	}

	// Verify the second module row.
	secondRow := kb.InlineKeyboard[1]
	if len(secondRow) != 1 {
		t.Fatalf("second module row len = %d, want 1", len(secondRow))
	}
	if secondRow[0].Text != "FakeAdminBtn2" {
		t.Fatalf("button[1].Text = %q, want FakeAdminBtn2", secondRow[0].Text)
	}

	// Verify that back + home navigation buttons are present in last row.
	lastRow := kb.InlineKeyboard[len(kb.InlineKeyboard)-1]
	if len(lastRow) != 2 {
		t.Fatalf("last row len = %d, want 2 (back + home)", len(lastRow))
	}
}

// TestGetModuleNameFromAltName_UsesPassedRegistry proves getModuleNameFromAltName
// resolves against the registry passed as parameter rather than the global HelpModule.
func TestGetModuleNameFromAltName_UsesPassedRegistry(t *testing.T) {
	localRegistry := NewHelpRegistry()
	localRegistry.AbleMap.Store("Filters", true)
	localRegistry.AbleMap.Store("Admin", true)

	cases := []struct {
		name     string
		altName  string
		expected string
	}{
		{"admin -> Admin", "admin", "Admin"},
		{"filters -> Filters", "filters", "Filters"},
		{"unknown -> empty", "unknown", ""},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := getModuleNameFromAltName(tc.altName, localRegistry)
			if got != tc.expected {
				t.Fatalf("getModuleNameFromAltName(%q) = %q, want %q", tc.altName, got, tc.expected)
			}
		})
	}
}

// TestGetHelpTextAndMarkup_UsesPassedRegistry proves getHelpTextAndMarkup resolves
// the module list and keyboard from the passed registry, not from the global.
func TestGetHelpTextAndMarkup_UsesPassedRegistry(t *testing.T) {
	localRegistry := NewHelpRegistry()
	localRegistry.AbleMap.Store("Admin", true)
	localRegistry.AbleMap.Store("Filters", true)
	// Do NOT store "Warns" — querying "warns" should miss in local registry.
	localRegistry.helpableKb["Admin"] = [][]gotgbot.InlineKeyboardButton{
		{{Text: "CustomAdmin", CallbackData: "custom-admin"}},
	}
	localRegistry.helpableKb["Filters"] = [][]gotgbot.InlineKeyboardButton{
		{{Text: "CustomFilters", CallbackData: "custom-filters"}},
	}

	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "private", FirstName: "Tester"}
	user := gotgbot.User{Id: 42, FirstName: "Tester"}

	cases := []struct {
		name          string
		moduleName    string
		wantContains  string
		wantBtn       string
		wantMinRows   int
		forbiddenBtns []string
	}{
		{"admin present in local registry", "admin", "Admin", "CustomAdmin", 2, nil},
		{"warns missing in local registry", "warns", "", "", 0, []string{"Warns", "CustomWarns"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctx := newModuleMessageContext(bot, chat, user, "/help "+tc.moduleName)
			helpText, kb, parseMode := getHelpTextAndMarkup(ctx, tc.moduleName, localRegistry)
			if parseMode != formatting.HTML {
				t.Fatalf("parseMode = %q, want HTML", parseMode)
			}
			if tc.wantContains != "" && !strings.Contains(helpText, tc.wantContains) {
				t.Fatalf("helpText = %q, want %q header", helpText, tc.wantContains)
			}
			if tc.wantMinRows > 0 && len(kb.InlineKeyboard) < tc.wantMinRows {
				t.Fatalf("inline keyboard rows = %d, want at least %d", len(kb.InlineKeyboard), tc.wantMinRows)
			}
			if tc.wantBtn != "" && kb.InlineKeyboard[0][0].Text != tc.wantBtn {
				t.Fatalf("button[0][0].Text = %q, want %q", kb.InlineKeyboard[0][0].Text, tc.wantBtn)
			}
			for _, forbidden := range tc.forbiddenBtns {
				for _, row := range kb.InlineKeyboard {
					for _, btn := range row {
						if btn.Text == forbidden {
							t.Fatalf("fallback kb contains local-only %q button", btn.Text)
						}
					}
				}
			}
		})
	}
}

func TestHandleDeepLinkPropagatesAboutAndDefaultSendErrors(t *testing.T) {
	for _, arg := range []string{"about", "unknown"} {
		t.Run(arg, func(t *testing.T) {
			client := newModuleBotClient()
			client.errors["sendMessage"] = fmt.Errorf("send failed")
			bot := newModuleTestBot(client)
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "private", FirstName: "Tester"}
			user := gotgbot.User{Id: 42, FirstName: "Tester"}
			ctx := newModuleMessageContext(bot, chat, user, "/start "+arg)

			if err := HandleDeepLink(bot, ctx, &user, arg); err == nil {
				t.Fatalf("HandleDeepLink(%q) error = nil, want send error", arg)
			}
		})
	}
}

// TestDeepLinkNonMemberDenied verifies that a user who has left (or was kicked
// from) the target chat is rejected by the rules and notes deep-link handlers
// before any content is served (IDOR fix).
//
// The test harness maps user ID 13 to {"status":"left"} and user ID 14 to
// {"status":"kicked"}, so both IDs represent non-members.
func TestDeepLinkNonMemberDenied(t *testing.T) {
	// Override the global i18n manager so MustNewTranslator returns real strings.
	// This lets us assert that the rejection text IS sent and content is NOT.
	restore, err := i18n.OverrideManagerForTest(deepLinkTestYAML)
	if err != nil {
		t.Fatalf("OverrideManagerForTest() error = %v", err)
	}
	t.Cleanup(restore)

	chatID := uniqueModuleChatID()

	// Seed the target chat and some content.
	if err := chats.EnsureChatInDb(chatID, "Secure Chat"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	rules.SetChatRules(chatID, "No spam.")
	if err := notes.AddNote(chatID, "secret", "Secret content", "", nil, db.TEXT, false, false, false, false, false, false); err != nil {
		t.Fatalf("AddNote(secret) error = %v", err)
	}
	t.Cleanup(func() {
		rules.SetChatRules(chatID, "")
		_ = notes.RemoveNote(chatID, "secret")
	})

	// Non-member: user ID 13 (status:"left" in test harness).
	nonMemberUser := gotgbot.User{Id: 13, FirstName: "Left User"}
	privateChat := gotgbot.Chat{Id: nonMemberUser.Id, Type: "private", FirstName: "Left User"}

	cases := []struct {
		name string
		arg  string
	}{
		{"rules denied for non-member", fmt.Sprintf("rules_%d", chatID)},
		{"notes list denied for non-member", fmt.Sprintf("notes_%d", chatID)},
		{"note denied for non-member", fmt.Sprintf("note_%d_secret", chatID)},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			client := newModuleBotClient()
			client.responses["getChat"] = []byte(fmt.Sprintf(
				`{"id":%d,"type":"supergroup","title":"Secure Chat"}`,
				chatID,
			))
			bot := newModuleTestBot(client)
			ctx := newModuleMessageContext(bot, privateChat, nonMemberUser, "/start "+tc.arg)

			if err := HandleDeepLink(bot, ctx, &nonMemberUser, tc.arg); err != ext.EndGroups {
				t.Fatalf("HandleDeepLink(%q) error = %v, want EndGroups (rejection)", tc.arg, err)
			}
			calls := client.callsFor("sendMessage")
			if len(calls) != 1 {
				t.Fatalf("sendMessage calls = %d, want 1 (rejection notice)", len(calls))
			}

			// The sent text MUST be the rejection message (not the served content).
			text, _ := calls[0].Params["text"].(string)
			if !strings.Contains(text, "Could not find the chat") {
				t.Fatalf("non-member received unexpected message (not the rejection): %q", text)
			}
			// The sent text MUST NOT contain any seeded content that would indicate
			// the security gate was bypassed and content was leaked.
			if strings.Contains(text, "No spam.") || strings.Contains(text, "Secret content") || strings.Contains(text, "secret") {
				t.Fatalf("non-member leaked content via deep link: %q", text)
			}
			// For the note subtest: verify no alternative content-delivery call fired
			// (e.g. sendDocument or sendPhoto that media.SendNote might use for media notes).
			for _, call := range client.calls {
				if call.Method == "sendDocument" || call.Method == "sendPhoto" || call.Method == "sendAudio" || call.Method == "sendVideo" {
					t.Fatalf("non-member triggered content-delivery call %q — gate was bypassed", call.Method)
				}
			}
		})
	}
}

// TestDeepLinkMemberAllowed verifies that a user who is a member of the target
// chat can still access rules, notes list, and individual notes via deep links.
//
// The test harness maps user ID 42 to {"status":"member"} — a legitimate member.
func TestDeepLinkMemberAllowed(t *testing.T) {
	// Override the global i18n manager so MustNewTranslator returns real strings.
	// This lets us assert that actual content (not a rejection) is delivered.
	restore, err := i18n.OverrideManagerForTest(deepLinkTestYAML)
	if err != nil {
		t.Fatalf("OverrideManagerForTest() error = %v", err)
	}
	t.Cleanup(restore)

	chatID := uniqueModuleChatID()

	if err := chats.EnsureChatInDb(chatID, "Open Chat"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	rules.SetChatRules(chatID, "Be nice.")
	if err := notes.AddNote(chatID, "welcome", "Welcome note", "", nil, db.TEXT, false, false, false, false, false, false); err != nil {
		t.Fatalf("AddNote(welcome) error = %v", err)
	}
	t.Cleanup(func() {
		rules.SetChatRules(chatID, "")
		_ = notes.RemoveNote(chatID, "welcome")
	})

	memberUser := gotgbot.User{Id: 42, FirstName: "Member"}
	privateChat := gotgbot.Chat{Id: 42, Type: "private", FirstName: "Member"}

	cases := []struct {
		name        string
		arg         string
		wantText    string // substring that must appear in the sent text
		forbidText  string // substring that must NOT appear (the rejection)
	}{
		{
			name:       "rules allowed for member",
			arg:        fmt.Sprintf("rules_%d", chatID),
			wantText:   "Be nice.",
			forbidText: "Could not find the chat",
		},
		{
			name:       "notes list allowed for member",
			arg:        fmt.Sprintf("notes_%d", chatID),
			wantText:   "welcome",
			forbidText: "Could not find the chat",
		},
		{
			name:       "note allowed for member",
			arg:        fmt.Sprintf("note_%d_welcome", chatID),
			wantText:   "Welcome note",
			forbidText: "Could not find the chat",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			client := newModuleBotClient()
			client.responses["getChat"] = []byte(fmt.Sprintf(
				`{"id":%d,"type":"supergroup","title":"Open Chat"}`,
				chatID,
			))
			bot := newModuleTestBot(client)
			ctx := newModuleMessageContext(bot, privateChat, memberUser, "/start "+tc.arg)

			if err := HandleDeepLink(bot, ctx, &memberUser, tc.arg); err != ext.EndGroups {
				t.Fatalf("HandleDeepLink(%q) error = %v, want EndGroups (content served)", tc.arg, err)
			}
			calls := client.callsFor("sendMessage")
			if len(calls) != 1 {
				t.Fatalf("sendMessage calls = %d, want 1 (content reply)", len(calls))
			}

			// The sent text MUST contain actual content, not the rejection message.
			text, _ := calls[0].Params["text"].(string)
			if strings.Contains(text, tc.forbidText) {
				t.Fatalf("member was wrongly denied — got rejection text in response: %q", text)
			}
			if !strings.Contains(text, tc.wantText) {
				t.Fatalf("member did not receive expected content %q; got: %q", tc.wantText, text)
			}
		})
	}
}
