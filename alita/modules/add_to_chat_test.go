//go:build testtools

package modules

import (
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/i18n"
)

// The rights the bot itself checks before acting (chat_status.CanBot*).
var botCheckedRights = []string{
	"change_info",      // CanBotChangeInfo
	"delete_messages",  // CanBotDelete
	"restrict_members", // CanBotRestrict
	"invite_users",     // CanBotInvite
	"pin_messages",     // CanBotPin
	"promote_members",  // CanBotPromote
}

func assertAddToChatRequestsAdminRights(t *testing.T, rawURL string) {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse %q: %v", rawURL, err)
	}
	if u.Host != "t.me" || u.Path != "/AlitaTestBot" {
		t.Fatalf("add-to-chat URL %q does not point at the bot", rawURL)
	}
	q := u.Query()
	if q.Get("startgroup") != "botstart" {
		t.Fatalf("add-to-chat URL %q lost startgroup=botstart", rawURL)
	}
	// Without admin= Telegram adds the bot as a plain member, so every
	// admin-only feature fails with "I'm not admin!" until rights are granted.
	granted := strings.Split(q.Get("admin"), " ") // "+" decodes to a space
	for _, right := range botCheckedRights {
		found := false
		for _, g := range granted {
			found = found || g == right
		}
		if !found {
			t.Errorf("add-to-chat URL %q does not request %s", rawURL, right)
		}
	}
}

func TestStartAddToChatButtonRequestsAdminRights(t *testing.T) {
	t.Parallel()

	startKb := getStartMarkup(helpTestTranslator(t), "AlitaTestBot")
	assertAddToChatRequestsAdminRights(t, startKb.InlineKeyboard[1][0].Url)
}

func TestConfigurationAddToChatButtonRequestsAdminRights(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: 4501, FirstName: "Owner"}
	chat := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Owner"}
	ctx := newModuleCallbackContext(bot, chat, user, encodeCallbackData("configuration", map[string]string{"s": "step1"}))

	if err := DefaultHelpRegistry().botConfig(bot, ctx); err != ext.EndGroups {
		t.Fatalf("botConfig(step1) error = %v, want EndGroups", err)
	}
	calls := client.callsFor("editMessageText")
	if len(calls) != 1 {
		t.Fatalf("editMessageText calls = %d, want 1", len(calls))
	}
	markup, ok := calls[0].Params["reply_markup"].(gotgbot.InlineKeyboardMarkup)
	if !ok {
		t.Fatalf("reply_markup = %T, want InlineKeyboardMarkup", calls[0].Params["reply_markup"])
	}
	assertAddToChatRequestsAdminRights(t, markup.InlineKeyboard[0][0].Url)
}

func TestBotJoinedWithoutAdminRightsAsksForThemInChatLanguage(t *testing.T) {
	restore, err := i18n.OverrideManagerForTest(`
bot_updates_thanks_for_adding: "Thanks!"
bot_updates_creators_plug: " Plug."
bot_updates_make_me_admin: " Localized: make me admin."
`)
	if err != nil {
		t.Fatalf("OverrideManagerForTest: %v", err)
	}
	t.Cleanup(restore)
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	bot.Id = 998 // the fake client reports every id but 999 as a plain member
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Fresh Group"}
	ctx := newModuleMessageContext(bot, chat, gotgbot.User{Id: 42, FirstName: "Adder"}, "bot joined")

	if err := botJoinedGroup(bot, ctx); err != ext.ContinueGroups {
		t.Fatalf("botJoinedGroup() error = %v, want ContinueGroups", err)
	}
	calls := client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
	text := fmt.Sprint(calls[0].Params["text"])
	if !strings.Contains(text, "Localized: make me admin.") {
		t.Fatalf("join message = %q, want the localized admin request", text)
	}
	if strings.Contains(text, "Make me admin to use me with my full abilities!") {
		t.Fatalf("join message = %q still carries the hardcoded English line", text)
	}
}
