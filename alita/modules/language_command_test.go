package modules

import (
	"fmt"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db/connections"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/i18n"
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
	data := encodeCallbackData("change_language", map[string]string{"l": "es"})

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
		Data:         encodeCallbackData("change_language", map[string]string{"l": "fr"}),
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

	for _, data := range []string{
		"change_language",
		encodeCallbackData("change_language", map[string]string{"l": "bogus"}),
		encodeCallbackData("change_language", map[string]string{"l": "config"}),
	} {
		ctx := newModuleCallbackContext(bot, chat, user, data)
		if err := languagesModule.langBtnHandler(bot, ctx); err != ext.EndGroups {
			t.Fatalf("langBtnHandler(%q) error = %v, want EndGroups", data, err)
		}
	}
	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 3 {
		t.Fatalf("answerCallbackQuery calls = %d, want 3", len(calls))
	}
	if calls := client.callsFor("editMessageText"); len(calls) != 0 {
		t.Fatalf("editMessageText calls = %d, want 0 for invalid selection", len(calls))
	}
}

func TestLanguageCallbackRejectsInvalidUser(t *testing.T) {
	t.Run("invalid callback user", func(t *testing.T) {
		client := newModuleBotClient()
		bot := newModuleTestBot(client)
		user := gotgbot.User{Id: 0, FirstName: "Invalid"}
		chat := gotgbot.Chat{Id: 1000, Type: "private", FirstName: "Invalid"}
		data := encodeCallbackData("change_language", map[string]string{"l": "es"})

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
			data := encodeCallbackData("change_language", map[string]string{"l": "fr"})
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

// The "help translate" button on /lang is labelled in the reader's language.
func TestChangeLanguageHelpTranslateButtonFollowsReadersLanguage(t *testing.T) {
	useRealLocales(t)
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Lector"}
	chat := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Lector"}
	if err := lang.ChangeUserLanguage(user.Id, "es"); err != nil {
		t.Fatalf("ChangeUserLanguage: %v", err)
	}
	want, _ := i18n.MustNewTranslator("es").GetString("language_help_translate")
	en, _ := i18n.MustNewTranslator("en").GetString("language_help_translate")
	if want == "" || want == en {
		t.Fatal("the test needs an es translation of language_help_translate")
	}

	if err := languagesModule.changeLanguage(bot, newModuleMessageContext(bot, chat, user, "/lang")); err != ext.EndGroups {
		t.Fatalf("changeLanguage() error = %v, want EndGroups", err)
	}
	calls := client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
	kb, ok := calls[0].Params["reply_markup"].(gotgbot.InlineKeyboardMarkup)
	if !ok {
		t.Fatalf("reply_markup = %#v, want an inline keyboard", calls[0].Params["reply_markup"])
	}
	var label string
	for _, row := range kb.InlineKeyboard {
		for _, button := range row {
			if button.Url != "" {
				label = button.Text
			}
		}
	}
	if label != want {
		t.Errorf("help translate button = %q, want the es language_help_translate %q", label, want)
	}
}

func languageDispatcher() *ext.Dispatcher {
	d := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadLanguage(d)
	return d
}

func TestLanguageDispatcherKeepsPMAndGroupGates(t *testing.T) {
	useRealLocales(t)
	for _, mode := range []string{"group", "private", "connected-group"} {
		for _, asAdmin := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/admin=%t", mode, asAdmin), func(t *testing.T) {
				f := newAdminChatFixture(t)
				user := f.member
				if asAdmin {
					user = f.admin
				}
				chat, text := f.chat, "/lang"
				want := "language_current_group"
				if mode != "group" {
					chat = privateChatOf(user)
					want = "language_current_user"
					if mode == "connected-group" {
						if err := connections.ConnectId(user.Id, f.chat.Id); err != nil {
							t.Fatal(err)
						}
						t.Cleanup(func() { _ = connections.DisconnectId(user.Id) })
						text += " group"
						want = "language_current_group"
					}
				}
				if !asAdmin && mode == "group" {
					want = "chat_status_user_admin_cmd_error"
				} else if !asAdmin && mode == "connected-group" {
					want = "connections_is_user_connected_user_not_admin"
				}
				update := newModuleMessageContext(f.bot, chat, user, text).Update
				if err := languageDispatcher().ProcessUpdate(f.bot, update, nil); err != nil {
					t.Fatal(err)
				}
				calls := f.client.callsFor("sendMessage")
				prefix, _ := i18n.MustNewTranslator("en").GetString(want)
				if prefix == "" {
					t.Fatalf("missing translation %s", want)
				}
				prefix = strings.Split(prefix, "%")[0]
				if len(calls) != 1 || !strings.HasPrefix(fmt.Sprint(calls[0].Params["text"]), prefix) {
					t.Fatalf("replies = %v, want %s", sentTexts(f.client), want)
				}
				if asAdmin && calls[0].Params["reply_markup"] == nil {
					t.Fatal("authorized command did not show its language keyboard")
				}
			})
		}
	}
}
