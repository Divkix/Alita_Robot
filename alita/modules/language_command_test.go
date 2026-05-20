package modules

import (
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
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
	if lang := db.GetLanguage(ctx); lang != "es" {
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
	if lang := db.GetLanguage(ctx); lang != "fr" {
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
