//go:build testtools

package modules

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db/connections"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

// helpMenuModules are the shown names of every module the help menu lists.
var helpMenuModules = []string{
	"Admin", "AISpam", "Antiflood", "AntiRaid", "Approvals", "Backup", "Bans", "Blacklists", "Captcha",
	"Connections", "Digest", "Disabling", "Federations", "Filters", "Formatting", "Greetings", "Languages",
	"Locks", "LogChannels", "Misc", "Mutes", "Notes", "Pins", "Purges", "Reactions", "Reports", "Rules",
	"SpamDB", "Warns",
}

// commandsMenuFixture loads the real locales and help registry, and gives
// the picker no candidate groups unless a test sets some.
func commandsMenuFixture(t *testing.T) adminChatFixture {
	t.Helper()
	useRealLocales(t)
	withExtendRegistry(t, helpMenuModules...)
	f := newAdminChatFixture(t)
	original, originalCooldown := settingsCandidateChats, settingsPickerCooldown
	settingsCandidateChats = func(context.Context, int64, int) ([]models.Chat, error) { return nil, nil }
	settingsPickerCooldown = 0
	t.Cleanup(func() { settingsCandidateChats, settingsPickerCooldown = original, originalCooldown })
	return f
}

func runCommandsCommand(t *testing.T, f adminChatFixture, chat gotgbot.Chat, user gotgbot.User) {
	t.Helper()
	c, err := helpers.BuildCommandContext(f.bot, newModuleMessageContext(f.bot, chat, user, "/commands"))
	if err != nil {
		t.Fatalf("BuildCommandContext() error = %v", err)
	}
	if err := DefaultHelpRegistry().commandsMenu(c); err != ext.EndGroups {
		t.Fatalf("commandsMenu error = %v, want EndGroups", err)
	}
}

func pressCommandsButton(t *testing.T, f adminChatFixture, chat gotgbot.Chat, user gotgbot.User, data string) {
	t.Helper()
	if data == "" {
		t.Fatal("button has no callback data")
	}
	ctx := newModuleCallbackContext(f.bot, chat, user, data)
	if err := DefaultHelpRegistry().commandsMenuCallback(f.bot, ctx); err != ext.EndGroups {
		t.Fatalf("commandsMenuCallback error = %v, want EndGroups", err)
	}
}

func lastCall(t *testing.T, client *moduleBotClient, method string) moduleBotCall {
	t.Helper()
	calls := client.callsFor(method)
	if len(calls) == 0 {
		t.Fatalf("no %s call", method)
	}
	return calls[len(calls)-1]
}

// buttonsByText indexes a keyboard's buttons by their label.
func buttonsByText(t *testing.T, call moduleBotCall) map[string]gotgbot.InlineKeyboardButton {
	t.Helper()
	out := map[string]gotgbot.InlineKeyboardButton{}
	for _, row := range markupOf(t, call).InlineKeyboard {
		for _, button := range row {
			out[button.Text] = button
		}
	}
	return out
}

func TestCommandsInGroupLinksToPMWithTheGroup(t *testing.T) {
	f := commandsMenuFixture(t)

	runCommandsCommand(t, f, f.chat, f.member)

	call := f.client.onlyCall(t, "sendMessage")
	wantURL := fmt.Sprintf("https://t.me/AlitaTestBot?start=commands_%d", f.chat.Id)
	var found bool
	for _, button := range buttonsByText(t, call) {
		found = found || button.Url == wantURL
	}
	if !found {
		t.Fatalf("group reply buttons = %v, want one linking to %s", markupOf(t, call), wantURL)
	}
	if text := fmt.Sprint(call.Params["text"]); strings.Contains(text, "/kickme") {
		t.Fatalf("group reply lists commands: %q", text)
	}
}

// A member sees only sections with a command a member can use, and in them
// only those commands.
func TestCommandsMenuHidesAdminCommandsFromAMember(t *testing.T) {
	f := commandsMenuFixture(t)
	pm := privateChatOf(f.member)

	runCommandsCommand(t, f, pm, f.member)

	home := lastCall(t, f.client, "sendMessage")
	sections := buttonsByText(t, home)
	for _, adminOnly := range []string{"Captcha", "Purges", "Mutes", "AISpam"} {
		if _, ok := sections[adminOnly]; ok {
			t.Errorf("member sees section %s, which has no member command", adminOnly)
		}
	}
	bans, ok := sections["Bans"]
	if !ok {
		t.Fatalf("member sections = %v, want Bans (/kickme, /banme)", sections)
	}

	pressCommandsButton(t, f, pm, f.member, bans.CallbackData)
	section := lastCall(t, f.client, "editMessageText")
	text := fmt.Sprint(section.Params["text"])
	if !strings.Contains(text, "/kickme") || strings.Contains(text, "/ban —") {
		t.Fatalf("member Bans section = %q, want /kickme and no /ban", text)
	}
	commands := buttonsByText(t, section)
	if _, ok := commands["/ban"]; ok {
		t.Fatal("member Bans section has a /ban button")
	}
	if _, ok := commands["/kickme"]; !ok {
		t.Fatalf("member Bans section buttons = %v, want /kickme", commands)
	}
}

// The deep link names the group: its admin sees every command, and examples
// name that group.
func TestCommandsDeepLinkShowsAdminCommandsAndNamesTheGroup(t *testing.T) {
	f := commandsMenuFixture(t)
	pm := privateChatOf(f.admin)

	ctx := newModuleMessageContext(f.bot, pm, f.admin, fmt.Sprintf("/start commands_%d", f.chat.Id))
	if err := HandleDeepLink(f.bot, ctx, &f.admin, fmt.Sprintf("commands_%d", f.chat.Id)); err != ext.EndGroups {
		t.Fatalf("HandleDeepLink error = %v, want EndGroups", err)
	}

	captcha, ok := buttonsByText(t, lastCall(t, f.client, "sendMessage"))["Captcha"]
	if !ok {
		t.Fatal("admin does not see the Captcha section")
	}
	pressCommandsButton(t, f, pm, f.admin, captcha.CallbackData)
	command, ok := buttonsByText(t, lastCall(t, f.client, "editMessageText"))["/captchamode"]
	if !ok {
		t.Fatal("admin Captcha section has no /captchamode button")
	}

	pressCommandsButton(t, f, pm, f.admin, command.CallbackData)
	screen := lastCall(t, f.client, "editMessageText")
	example := fmt.Sprintf("/captchamode %d text", f.chat.Id)
	if text := fmt.Sprint(screen.Params["text"]); !strings.Contains(text, "/captchamode") || !strings.Contains(text, example) {
		t.Fatalf("command screen = %q, want the /captchamode entry and example %q", text, example)
	}
	var copied, back bool
	for _, button := range buttonsByText(t, screen) {
		if button.CopyText != nil && button.CopyText.Text == example {
			copied = true
		}
		if decoded, ok := decodeCallbackData(button.CallbackData, commandsMenuNamespace); ok {
			action, _ := decoded.Field("a")
			module, _ := decoded.Field("m")
			back = back || (action == "s" && module == "captcha")
		}
	}
	if !copied || !back {
		t.Fatalf("command screen buttons = %v, want Copy example (%q) and Back to Captcha", markupOf(t, screen), example)
	}
}

func TestCommandsMenuUsesTheConnectedGroup(t *testing.T) {
	f := commandsMenuFixture(t)
	pm := f.connectAdmin(t)

	runCommandsCommand(t, f, pm, f.admin)

	if _, ok := buttonsByText(t, lastCall(t, f.client, "sendMessage"))["Captcha"]; !ok {
		t.Fatal("admin of the connected group does not see the Captcha section")
	}
}

// Without a linked or connected group, any group of the /settings picker's
// list where the user is an admin shows the admin commands.
func TestCommandsMenuFallsBackToThePickerGroups(t *testing.T) {
	f := commandsMenuFixture(t)
	settingsCandidateChats = func(context.Context, int64, int) ([]models.Chat, error) {
		return []models.Chat{{ChatId: f.chat.Id, ChatName: f.chat.Title}}, nil
	}
	if err := connections.DisconnectId(f.admin.Id); err != nil {
		t.Fatalf("DisconnectId() error = %v", err)
	}

	runCommandsCommand(t, f, privateChatOf(f.admin), f.admin)

	if _, ok := buttonsByText(t, lastCall(t, f.client, "sendMessage"))["Captcha"]; !ok {
		t.Fatal("admin of a picker group does not see the Captcha section")
	}
}

// Every button of every screen fits the callback cap, with the longest group
// ID, and every command has its own help entry in every section screen.
func TestCommandsMenuButtonsAllEncode(t *testing.T) {
	useRealLocales(t)
	withExtendRegistry(t, helpMenuModules...)
	tr := i18n.MustNewTranslator("en")
	view := commandsView{admin: true, group: -1009999999999}

	_, home := commandsHomeScreen(tr, view)
	assertButtonsEncode(t, home)
	modules, _ := commandsSections()
	for _, module := range modules {
		_, kb, ok := commandsSectionScreen(tr, view, module)
		if !ok {
			t.Errorf("section %s has no screen", module)
			continue
		}
		assertButtonsEncode(t, kb)
		for _, cmd := range sectionCommands(module, view) {
			_, kb, ok := commandsCommandScreen(tr, view, cmd)
			if !ok {
				t.Errorf("/%s has no command screen", cmd)
				continue
			}
			assertButtonsEncode(t, kb)
		}
	}
}

func assertButtonsEncode(t *testing.T, kb gotgbot.InlineKeyboardMarkup) {
	t.Helper()
	for _, row := range kb.InlineKeyboard {
		for _, button := range row {
			if button.CallbackData == "" && button.CopyText == nil && button.Url == "" {
				t.Errorf("button %q has no action (callback data over the cap?)", button.Text)
			}
		}
	}
}

func TestCommandsMenuButtonOutsidePMIsRefused(t *testing.T) {
	f := commandsMenuFixture(t)
	data := commandsView{}.callbackData(map[string]string{"a": "h"})

	pressCommandsButton(t, f, f.chat, f.member, data)

	if calls := f.client.callsFor("editMessageText"); len(calls) != 0 {
		t.Fatalf("a menu button in a group edited the message: %v", calls[0].Params)
	}
}

func TestExampleNamingGroup(t *testing.T) {
	tests := []struct{ example, want string }{
		{"/lock @mygroup sticker", "/lock -1005 sticker"},
		{`/rules "My Group"`, "/rules -1005"},
		{"/approve -1001234567890 @username", "/approve -1005 @username"},
		{"/digest now", "/digest now"},
		{"/ban @username spam", "/ban @username spam"},
		{"/stopall @mygroupx", "/stopall @mygroupx"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := exampleNamingGroup(tt.example, -1005); got != tt.want {
			t.Errorf("exampleNamingGroup(%q) = %q, want %q", tt.example, got, tt.want)
		}
	}
}
