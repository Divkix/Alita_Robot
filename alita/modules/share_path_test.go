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
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

// The private /about screen is where a member can hand the bot to a group
// admin, so it carries the docs page's add-to-group URL plus a share sheet.
func TestAboutOffersAddToGroupAndShare(t *testing.T) {
	useRealLocales(t)
	tr := i18n.MustNewTranslator("en")
	shareButtonText, err := tr.GetString("help_button_share")
	if err != nil || shareButtonText == "" {
		t.Fatalf("help_button_share = %q, %v; want a localized label", shareButtonText, err)
	}
	shareText, err := tr.GetString("help_share_text")
	if err != nil || shareText == "" {
		t.Fatalf("help_share_text = %q, %v; want a localized pitch", shareText, err)
	}

	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Admin"}
	chat := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Admin"}

	if err := DefaultHelpRegistry().about(bot, newModuleMessageContext(bot, chat, user, "/about")); err != ext.EndGroups {
		t.Fatalf("about error = %v, want EndGroups", err)
	}

	markup := markupOf(t, client.onlyCall(t, "sendMessage"))
	if len(markup.InlineKeyboard) == 0 || len(markup.InlineKeyboard[0]) != 2 {
		t.Fatalf("first keyboard row = %+v, want the add-to-group and share buttons", markup.InlineKeyboard)
	}

	add := markup.InlineKeyboard[0][0]
	if want := helpers.AddToGroupURL("AlitaTestBot"); add.Url != want {
		t.Errorf("add-to-group url = %q, want %q", add.Url, want)
	}
	assertAddToChatRequestsAdminRights(t, add.Url)

	share := markup.InlineKeyboard[0][1]
	if share.Text != shareButtonText {
		t.Errorf("share button text = %q, want %q", share.Text, shareButtonText)
	}
	shared, err := url.Parse(share.Url)
	if err != nil {
		t.Fatalf("parse share url %q: %v", share.Url, err)
	}
	if shared.Scheme != "https" || shared.Host != "t.me" || shared.Path != "/share/url" {
		t.Fatalf("share url = %q, want a t.me share link", share.Url)
	}
	if got := shared.Query().Get("url"); got != "https://t.me/AlitaTestBot" {
		t.Errorf("shared url = %q, want the bot's t.me link", got)
	}
	if got := shared.Query().Get("text"); got != shareText {
		t.Errorf("share text = %q, want %q", got, shareText)
	}
}

// The dashboard is optional, so its mention follows PANEL_ENABLED while the
// add-to-group and share buttons stay on both variants.
func TestAboutMentionsDashboardOnlyWhenPanelEnabled(t *testing.T) {
	useRealLocales(t)
	hint, err := i18n.MustNewTranslator("en").GetString("help_about_panel_hint")
	if err != nil || hint == "" {
		t.Fatalf("help_about_panel_hint = %q, %v; want a localized hint", hint, err)
	}

	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("panel enabled=%v", enabled), func(t *testing.T) {
			withPanelConfig(t, enabled, "https://panel.example.com", "")

			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			user := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Admin"}
			chat := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Admin"}

			if err := DefaultHelpRegistry().about(bot, newModuleMessageContext(bot, chat, user, "/about")); err != ext.EndGroups {
				t.Fatalf("about error = %v, want EndGroups", err)
			}

			call := client.onlyCall(t, "sendMessage")
			text := fmt.Sprint(call.Params["text"])
			if got := strings.Contains(text, hint); got != enabled {
				t.Errorf("about text mentions the dashboard = %v, want %v; text = %q", got, enabled, text)
			}
			markup := markupOf(t, call)
			if len(markup.InlineKeyboard) == 0 || len(markup.InlineKeyboard[0]) != 2 {
				t.Fatalf("first keyboard row = %+v, want the add-to-group and share buttons", markup.InlineKeyboard)
			}
		})
	}
}

// A group /help reply sends the reader to the bot's PM. It must never carry a
// raw add-to-group link: that would add the bot without its admin rights.
func TestGroupHelpLinksThePrivateHelpDeepLink(t *testing.T) {
	withPanelConfig(t, false, "", "")

	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Helper"}
	group := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Help Chat"}

	if err := DefaultHelpRegistry().help(bot, newModuleMessageContext(bot, group, user, "/help")); err != ext.EndGroups {
		t.Fatalf("help error = %v, want EndGroups", err)
	}

	markup := markupOf(t, client.onlyCall(t, "sendMessage"))
	if len(markup.InlineKeyboard) == 0 || len(markup.InlineKeyboard[0]) != 1 {
		t.Fatalf("first keyboard row = %+v, want the private help button alone", markup.InlineKeyboard)
	}
	button := markup.InlineKeyboard[0][0]
	if want := "https://t.me/AlitaTestBot?start=help_help"; button.Url != want {
		t.Fatalf("group /help button url = %q, want the PM deep link %q", button.Url, want)
	}
	for _, row := range markup.InlineKeyboard {
		for _, button := range row {
			if strings.Contains(button.Url, "startgroup") {
				t.Errorf("group /help offers %q; adding the bot must go through the rights link, not a group reply", button.Url)
			}
		}
	}
}
