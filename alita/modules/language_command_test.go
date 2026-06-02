package modules

import (
	"fmt"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/divkix/Alita_Robot/alita/db/lang"
)

func TestChangeLanguagePrivateShowsLanguageKeyboard(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: 5201, FirstName: "Member"}
	chat := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Member"}

	ctx := newModuleMessageContext(bot, chat, user, "/lang")
	if err := languagesModule.changeLanguage(bot, ctx); err != ext.EndGroups {
		t.Fatalf("changeLanguage() error = %v, want EndGroups", err)
	}
	calls := client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
	if calls[0].Params["reply_markup"] == nil {
		t.Fatal("language keyboard reply_markup was not sent")
	}
}

func TestLanguageCallbackChangesUserLanguageAndEditsMessage(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: 5202, FirstName: "Member"}
	chat := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Member"}
	data := encodeCallbackData("change_language", map[string]string{"l": "es"}, "change_language.es")

	ctx := newModuleCallbackContext(bot, chat, user, data)
	if err := languagesModule.langBtnHandler(bot, ctx); err != ext.EndGroups {
		t.Fatalf("langBtnHandler() error = %v, want EndGroups", err)
	}
	if lang := lang.GetLanguage(ctx); lang != "es" {
		t.Fatalf("user language = %q, want es", lang)
	}
	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 1 {
		t.Fatalf("answerCallbackQuery calls = %d, want 1", len(calls))
	}
	if calls := client.callsFor("editMessageText"); len(calls) != 1 {
		t.Fatalf("editMessageText calls = %d, want 1", len(calls))
	}
}

func TestLanguageCallbackWithoutMessageAnswersInsteadOfEditing(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: 5203, FirstName: "Member"}
	query := &gotgbot.CallbackQuery{
		Id:           "callback-2",
		From:         user,
		Data:         "change_language.fr",
		ChatInstance: "test-chat-instance",
	}
	ctx := ext.NewContext(bot, &gotgbot.Update{UpdateId: 3, CallbackQuery: query}, nil)
	ctx.EffectiveChat = &gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Member"}

	if err := languagesModule.langBtnHandler(bot, ctx); err != ext.EndGroups {
		t.Fatalf("langBtnHandler() error = %v, want EndGroups", err)
	}
	if lang := lang.GetLanguage(ctx); lang != "fr" {
		t.Fatalf("user language = %q, want fr", lang)
	}
	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 1 {
		t.Fatalf("answerCallbackQuery calls = %d, want 1", len(calls))
	}
	if calls := client.callsFor("editMessageText"); len(calls) != 0 {
		t.Fatalf("editMessageText calls = %d, want 0 without callback message", len(calls))
	}
}

func TestLanguageCallbackRejectsInvalidSelection(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: 5204, FirstName: "Member"}
	chat := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Member"}

	ctx := newModuleCallbackContext(bot, chat, user, "change_language")
	if err := languagesModule.langBtnHandler(bot, ctx); err != ext.EndGroups {
		t.Fatalf("langBtnHandler() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 1 {
		t.Fatalf("answerCallbackQuery calls = %d, want 1", len(calls))
	}
	if calls := client.callsFor("editMessageText"); len(calls) != 0 {
		t.Fatalf("editMessageText calls = %d, want 0 for invalid selection", len(calls))
	}
}

func TestLanguageCallbackHandlesGroupPermissionsAndInvalidUser(t *testing.T) {
	t.Run("group admin changes group language", func(t *testing.T) {
		client := newModuleBotClient()
		bot := newModuleTestBot(client)
		admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
		chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Language Chat"}
		data := encodeCallbackData("change_language", map[string]string{"l": "hi"}, "change_language.hi")

		ctx := newModuleCallbackContext(bot, chat, admin, data)
		if err := languagesModule.langBtnHandler(bot, ctx); err != ext.EndGroups {
			t.Fatalf("langBtnHandler(group admin) error = %v, want EndGroups", err)
		}
		if lang := lang.GetLanguage(ctx); lang != "hi" {
			t.Fatalf("group language = %q, want hi", lang)
		}
		if calls := client.callsFor("editMessageText"); len(calls) != 1 {
			t.Fatalf("editMessageText calls = %d, want group language edit", len(calls))
		}
	})

	t.Run("group member rejected", func(t *testing.T) {
		client := newModuleBotClient()
		bot := newModuleTestBot(client)
		member := gotgbot.User{Id: 42, FirstName: "Member"}
		chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Language Chat"}
		data := encodeCallbackData("change_language", map[string]string{"l": "es"}, "change_language.es")

		ctx := newModuleCallbackContext(bot, chat, member, data)
		if err := languagesModule.langBtnHandler(bot, ctx); err != ext.EndGroups {
			t.Fatalf("langBtnHandler(group member) error = %v, want EndGroups", err)
		}
		if calls := client.callsFor("editMessageText"); len(calls) != 0 {
			t.Fatalf("editMessageText calls = %d, want no edit for non-admin", len(calls))
		}
	})

	t.Run("invalid callback user", func(t *testing.T) {
		client := newModuleBotClient()
		bot := newModuleTestBot(client)
		user := gotgbot.User{Id: 0, FirstName: "Invalid"}
		chat := gotgbot.Chat{Id: 1000, Type: "private", FirstName: "Invalid"}
		data := encodeCallbackData("change_language", map[string]string{"l": "es"}, "change_language.es")

		ctx := newModuleCallbackContext(bot, chat, user, data)
		if err := languagesModule.langBtnHandler(bot, ctx); err != ext.EndGroups {
			t.Fatalf("langBtnHandler(invalid user) error = %v, want EndGroups", err)
		}
		if calls := client.callsFor("answerCallbackQuery"); len(calls) != 1 {
			t.Fatalf("answerCallbackQuery calls = %d, want invalid-user answer", len(calls))
		}
	})
}

func TestLanguageCallbackPropagatesAnswerAndEditErrors(t *testing.T) {
	for _, tt := range []struct {
		name   string
		method string
		noMsg  bool
	}{
		{name: "answer without message", method: "answerCallbackQuery", noMsg: true},
		{name: "edit message", method: "editMessageText"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := newModuleBotClient()
			client.errors[tt.method] = fmt.Errorf("telegram request failed")
			bot := newModuleTestBot(client)
			user := gotgbot.User{Id: uniqueModuleChatID(), FirstName: "Member"}
			data := encodeCallbackData("change_language", map[string]string{"l": "fr"}, "change_language.fr")
			var ctx *ext.Context
			if tt.noMsg {
				query := &gotgbot.CallbackQuery{
					Id:           "callback-error",
					From:         user,
					Data:         data,
					ChatInstance: "test-chat-instance",
				}
				ctx = ext.NewContext(bot, &gotgbot.Update{UpdateId: 4, CallbackQuery: query}, nil)
				ctx.EffectiveChat = &gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Member"}
			} else {
				ctx = newModuleCallbackContext(bot, gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Member"}, user, data)
			}

			if err := languagesModule.langBtnHandler(bot, ctx); err == nil {
				t.Fatalf("langBtnHandler(%s) error = nil, want request error", tt.name)
			}
		})
	}
}
