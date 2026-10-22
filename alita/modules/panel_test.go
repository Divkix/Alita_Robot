//go:build testtools

package modules

import (
	"fmt"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/config"
)

func withPanelConfig(t *testing.T, enabled bool, publicURL string) {
	t.Helper()
	previous := config.AppConfig
	config.AppConfig = &config.Config{
		PanelEnabled:   enabled,
		PanelPublicURL: publicURL,
	}
	t.Cleanup(func() { config.AppConfig = previous })
}

func TestPanelDeepLinkDisabledFallsBackToUnknownPayload(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Admin"}
	ctx := newModuleMessageContext(bot, privateChatOf(user), user, "/start panel_-1001234567890")
	withPanelConfig(t, false, "")

	if err := HandleDeepLink(bot, ctx, &user, "some_unknown_payload"); err != nil && err != ext.EndGroups {
		t.Fatalf("HandleDeepLink(unknown) error = %v", err)
	}
	if err := HandleDeepLink(bot, ctx, &user, "panel_-1001234567890"); err != nil && err != ext.EndGroups {
		t.Fatalf("HandleDeepLink(panel_) error = %v", err)
	}

	calls := client.callsFor("sendMessage")
	if len(calls) != 2 {
		t.Fatalf("sendMessage calls = %d, want 2", len(calls))
	}
	if calls[0].Params["text"] != calls[1].Params["text"] {
		t.Fatalf("disabled panel reply = %v, want the unknown-payload help", calls[1].Params["text"])
	}
	if fmt.Sprint(calls[0].Params["reply_markup"]) != fmt.Sprint(calls[1].Params["reply_markup"]) {
		t.Fatalf("disabled panel keyboard = %v, want the unknown-payload keyboard", calls[1].Params["reply_markup"])
	}
}

func TestPanelDeepLinkRepliesWithWebAppButton(t *testing.T) {
	useRealLocales(t)
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Admin"}
	ctx := newModuleMessageContext(bot, privateChatOf(user), user, "/start panel_-1001234567890")
	withPanelConfig(t, true, "https://panel.example.com")

	if err := HandleDeepLink(bot, ctx, &user, "panel_-1001234567890"); err != nil && err != ext.EndGroups {
		t.Fatalf("HandleDeepLink() error = %v", err)
	}

	calls := client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
	markup, ok := calls[0].Params["reply_markup"].(gotgbot.InlineKeyboardMarkup)
	if !ok || len(markup.InlineKeyboard) == 0 || len(markup.InlineKeyboard[0]) == 0 {
		t.Fatalf("reply_markup = %#v, want a web_app keyboard", calls[0].Params["reply_markup"])
	}
	button := markup.InlineKeyboard[0][0]
	if button.WebApp == nil {
		t.Fatalf("button = %#v, want web_app", button)
	}
	if button.WebApp.Url != "https://panel.example.com?chat=-1001234567890" {
		t.Fatalf("web_app url = %q", button.WebApp.Url)
	}
	if button.Text != "Open admin panel" {
		t.Fatalf("button text = %q, want the localized label", button.Text)
	}
	if calls[0].Params["text"] != "Tap the button below to open the admin panel for this chat." {
		t.Fatalf("reply text = %v, want the localized message", calls[0].Params["text"])
	}
}

func TestPanelDeepLinkInvalidChatIDFallsBack(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Admin"}
	ctx := newModuleMessageContext(bot, privateChatOf(user), user, "/start panel_notanumber")
	withPanelConfig(t, true, "https://panel.example.com")

	if err := HandleDeepLink(bot, ctx, &user, "unknown_payload"); err != nil && err != ext.EndGroups {
		t.Fatalf("HandleDeepLink(unknown) error = %v", err)
	}
	if err := HandleDeepLink(bot, ctx, &user, "panel_notanumber"); err != nil && err != ext.EndGroups {
		t.Fatalf("HandleDeepLink() error = %v", err)
	}

	calls := client.callsFor("sendMessage")
	if len(calls) != 2 {
		t.Fatalf("sendMessage calls = %d, want 2", len(calls))
	}
	if calls[0].Params["text"] != calls[1].Params["text"] {
		t.Fatalf("invalid chat id reply = %v, want the unknown-payload help", calls[1].Params["text"])
	}
	if fmt.Sprint(calls[0].Params["reply_markup"]) != fmt.Sprint(calls[1].Params["reply_markup"]) {
		t.Fatalf("invalid chat id keyboard = %v, want the unknown-payload keyboard", calls[1].Params["reply_markup"])
	}
}
