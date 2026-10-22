package modules

import (
	"fmt"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/i18n"
)

// panelEntryCase is one panel entry scenario shared by the group /settings
// and /help tests: off keeps the keyboard as it was, on appends one more row,
// and a short name switches that row to the direct Mini App link.
type panelEntryCase struct {
	name      string
	enabled   bool
	shortName string
}

var panelEntryCases = []panelEntryCase{
	{name: "disabled"},
	{name: "pm deep link", enabled: true},
	{name: "mini app short name", enabled: true, shortName: "alita_panel"},
}

// panelEntryURL is the URL the appended row must carry for a chat.
func panelEntryURL(tt panelEntryCase, chatID int64) string {
	if tt.shortName == "" {
		return fmt.Sprintf("https://t.me/AlitaTestBot?start=panel_%d", chatID)
	}
	return fmt.Sprintf("https://t.me/AlitaTestBot/%s?startapp=chat_%d", tt.shortName, chatID)
}

func TestGroupSettingsAddsPanelButtonWhenEnabled(t *testing.T) {
	useRealLocales(t)
	label, err := i18n.MustNewTranslator("en").GetString("panel_open_button")
	if err != nil || label == "" {
		t.Fatalf("panel_open_button = %q, %v; want a localized label", label, err)
	}

	for _, tt := range panelEntryCases {
		t.Run(tt.name, func(t *testing.T) {
			client, bot, groupA, _ := settingsFixture(t)
			withPanelConfig(t, tt.enabled, "https://panel.example.com", tt.shortName)
			group := gotgbot.Chat{Id: groupA, Type: "supergroup", Title: "Group A"}
			user := gotgbot.User{Id: settingsTestAdminID, FirstName: "Admin"}

			if err := ConnectionsModule.settings(bot, newModuleMessageContext(bot, group, user, "/settings")); err != ext.EndGroups {
				t.Fatalf("settings error = %v, want EndGroups", err)
			}

			markup := markupOf(t, client.onlyCall(t, "sendMessage"))
			if len(markup.InlineKeyboard) == 0 || len(markup.InlineKeyboard[0]) != 1 {
				t.Fatalf("first keyboard row = %+v, want the settings deep link alone", markup.InlineKeyboard)
			}
			wantSettingsURL := fmt.Sprintf("https://t.me/AlitaTestBot?start=settings_%d", groupA)
			if got := markup.InlineKeyboard[0][0].Url; got != wantSettingsURL {
				t.Errorf("settings button url = %q, want %q", got, wantSettingsURL)
			}

			if !tt.enabled {
				if len(markup.InlineKeyboard) != 1 {
					t.Fatalf("keyboard rows = %d, want only the settings row while the panel is disabled", len(markup.InlineKeyboard))
				}
				return
			}
			if len(markup.InlineKeyboard) != 2 || len(markup.InlineKeyboard[1]) != 1 {
				t.Fatalf("keyboard = %+v, want one appended panel row", markup.InlineKeyboard)
			}
			button := markup.InlineKeyboard[1][0]
			if want := panelEntryURL(tt, groupA); button.Url != want {
				t.Errorf("panel button url = %q, want %q", button.Url, want)
			}
			if button.Text != label {
				t.Errorf("panel button text = %q, want %q", button.Text, label)
			}
		})
	}
}

func TestGroupHelpAddsPanelButtonWhenEnabled(t *testing.T) {
	for _, tt := range panelEntryCases {
		t.Run(tt.name, func(t *testing.T) {
			withPanelConfig(t, tt.enabled, "https://panel.example.com", tt.shortName)
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			group := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Help Chat"}
			user := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Helper"}

			if err := DefaultHelpRegistry().help(bot, newModuleMessageContext(bot, group, user, "/help")); err != ext.EndGroups {
				t.Fatalf("help error = %v, want EndGroups", err)
			}

			markup := markupOf(t, client.onlyCall(t, "sendMessage"))
			if len(markup.InlineKeyboard) == 0 || len(markup.InlineKeyboard[0]) != 1 {
				t.Fatalf("first keyboard row = %+v, want the PM help link alone", markup.InlineKeyboard)
			}
			if got := markup.InlineKeyboard[0][0].Url; got != "https://t.me/AlitaTestBot?start=help_help" {
				t.Errorf("PM help button url = %q, want the /help deep link", got)
			}

			if !tt.enabled {
				if len(markup.InlineKeyboard) != 1 {
					t.Fatalf("keyboard rows = %d, want only the help row while the panel is disabled", len(markup.InlineKeyboard))
				}
				return
			}
			if len(markup.InlineKeyboard) != 2 || len(markup.InlineKeyboard[1]) != 1 {
				t.Fatalf("keyboard = %+v, want one appended panel row", markup.InlineKeyboard)
			}
			if want := panelEntryURL(tt, group.Id); markup.InlineKeyboard[1][0].Url != want {
				t.Errorf("panel button url = %q, want %q", markup.InlineKeyboard[1][0].Url, want)
			}
		})
	}
}
