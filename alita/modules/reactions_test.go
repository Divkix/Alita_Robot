package modules

import (
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/reactions"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

func TestReactionCommandsManageDBRows(t *testing.T) {
	if db.DB == nil {
		t.Skip("requires database connection")
	}

	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Reaction Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	t.Cleanup(func() {
		_ = reactions.ResetReactions(chat.Id)
	})

	addCtx := newModuleMessageContext(bot, chat, admin, "/addreaction hello 👍")
	if err := reactionsModule.addReaction(bot, addCtx); err != ext.EndGroups {
		t.Fatalf("addReaction() error = %v, want EndGroups", err)
	}
	if got := reactions.GetReactions(chat.Id)["hello"]; got != "👍" {
		t.Fatalf("stored reaction = %q, want 👍", got)
	}

	listCtx := newModuleMessageContext(bot, chat, admin, "/reactions")
	if err := reactionsModule.listReactions(bot, listCtx); err != ext.EndGroups {
		t.Fatalf("listReactions() error = %v, want EndGroups", err)
	}

	removeCtx := newModuleMessageContext(bot, chat, admin, "/removereaction hello")
	if err := reactionsModule.removeReaction(bot, removeCtx); err != ext.EndGroups {
		t.Fatalf("removeReaction() error = %v, want EndGroups", err)
	}
	if m := reactions.GetReactions(chat.Id); len(m) != 0 {
		t.Fatalf("reactions remained after removing final reaction: %v", m)
	}

	_ = reactions.AddReaction(chat.Id, "bye", "👍")
	resetCtx := newModuleMessageContext(bot, chat, admin, "/resetreactions")
	if err := reactionsModule.resetReactions(bot, resetCtx); err != ext.EndGroups {
		t.Fatalf("resetReactions() error = %v, want EndGroups", err)
	}
	if m := reactions.GetReactions(chat.Id); len(m) != 0 {
		t.Fatalf("reactions remained after reset: %v", m)
	}

	if calls := client.callsFor("sendMessage"); len(calls) != 4 {
		t.Fatalf("sendMessage calls = %d, want 4", len(calls))
	}
}

func TestReactionCommandsHandleUsageAndMissingEntries(t *testing.T) {
	if db.DB == nil {
		t.Skip("requires database connection")
	}

	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Reaction Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	t.Cleanup(func() {
		_ = reactions.ResetReactions(chat.Id)
	})

	for _, tt := range []struct {
		name string
		text string
		run  func(*gotgbot.Bot, *ext.Context) error
	}{
		{name: "add usage", text: "/addreaction keyword", run: reactionsModule.addReaction},
		{name: "remove usage", text: "/removereaction", run: reactionsModule.removeReaction},
		{name: "remove missing", text: "/removereaction hello", run: reactionsModule.removeReaction},
		{name: "list missing", text: "/reactions", run: reactionsModule.listReactions},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newModuleMessageContext(bot, chat, admin, tt.text)
			if err := tt.run(bot, ctx); err != ext.EndGroups {
				t.Fatalf("%s error = %v, want EndGroups", tt.name, err)
			}
		})
	}

	if err := reactions.AddReaction(chat.Id, "hello", "👍"); err != nil {
		t.Fatalf("seed reaction: %v", err)
	}
	missingKeywordCtx := newModuleMessageContext(bot, chat, admin, "/removereaction absent")
	if err := reactionsModule.removeReaction(bot, missingKeywordCtx); err != ext.EndGroups {
		t.Fatalf("removeReaction(missing keyword) error = %v, want EndGroups", err)
	}

	if calls := client.callsFor("sendMessage"); len(calls) != 5 {
		t.Fatalf("sendMessage calls = %d, want usage and missing-entry replies", len(calls))
	}
}

func TestReactionsHelpCallbackEditsAndAnswers(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Reaction Chat"}
	user := gotgbot.User{Id: 4306, FirstName: "Helper"}
	ctx := newModuleCallbackContext(
		bot,
		chat,
		user,
		encodeCallbackData("reactions_help", map[string]string{"action": "add"}),
	)

	if err := reactionsModule.reactionsHelpHandler(bot, ctx); err != ext.EndGroups {
		t.Fatalf("reactionsHelpHandler() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("editMessageText"); len(calls) != 1 {
		t.Fatalf("editMessageText calls = %d, want 1", len(calls))
	}
	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 1 {
		t.Fatalf("answerCallbackQuery calls = %d, want 1", len(calls))
	}
}

func TestReactionsHelpCallbackRejectsInvalidAndAnswersWithoutMessage(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Reaction Chat"}
	user := gotgbot.User{Id: 4306, FirstName: "Helper"}

	for _, data := range []string{"reactions_help", "reactions_help.unknown"} {
		ctx := newModuleCallbackContext(bot, chat, user, data)
		if err := reactionsModule.reactionsHelpHandler(bot, ctx); err != ext.EndGroups {
			t.Fatalf("reactionsHelpHandler(%q) error = %v, want EndGroups", data, err)
		}
	}

	noMessageCtx := newModuleCallbackContext(bot, chat, user, "reactions_help.remove")
	noMessageCtx.CallbackQuery.Message = nil
	if err := reactionsModule.reactionsHelpHandler(bot, noMessageCtx); err != ext.EndGroups {
		t.Fatalf("reactionsHelpHandler(no message) error = %v, want EndGroups", err)
	}

	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 3 {
		t.Fatalf("answerCallbackQuery calls = %d, want invalid and no-message answers", len(calls))
	}
	if calls := client.callsFor("editMessageText"); len(calls) != 0 {
		t.Fatalf("editMessageText calls = %d, want none for invalid/no-message callbacks", len(calls))
	}
}

func TestCheckReactionsSetsMessageReactionForMatchingKeyword(t *testing.T) {
	if db.DB == nil {
		t.Skip("requires database connection")
	}

	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Reaction Chat"}
	user := gotgbot.User{Id: 4307, FirstName: "Member"}
	t.Cleanup(func() {
		_ = reactions.ResetReactions(chat.Id)
	})

	if err := reactions.AddReaction(chat.Id, "hello", "👍"); err != nil {
		t.Fatalf("seed reaction: %v", err)
	}

	ctx := newModuleMessageContext(bot, chat, user, "well hello there")
	if err := reactionsModule.checkReactions(bot, ctx); err != ext.ContinueGroups {
		t.Fatalf("checkReactions() error = %v, want ContinueGroups", err)
	}
	if calls := client.callsFor("setMessageReaction"); len(calls) != 1 {
		t.Fatalf("setMessageReaction calls = %d, want 1", len(calls))
	}
}

func TestCheckReactionsNoopsForMissingMessageChatAndEmptyConfig(t *testing.T) {
	if db.DB == nil {
		t.Skip("requires database connection")
	}

	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Reaction Chat"}
	user := gotgbot.User{Id: 4307, FirstName: "Member"}
	t.Cleanup(func() {
		_ = reactions.ResetReactions(chat.Id)
	})

	emptyTextCtx := newModuleMessageContext(bot, chat, user, "")
	if err := reactionsModule.checkReactions(bot, emptyTextCtx); err != ext.ContinueGroups {
		t.Fatalf("checkReactions(empty text) error = %v, want ContinueGroups", err)
	}

	noChatCtx := newModuleMessageContext(bot, chat, user, "hello")
	noChatCtx.EffectiveChat = nil
	if err := reactionsModule.checkReactions(bot, noChatCtx); err != ext.ContinueGroups {
		t.Fatalf("checkReactions(no chat) error = %v, want ContinueGroups", err)
	}

	emptyConfigCtx := newModuleMessageContext(bot, chat, user, "hello")
	if err := reactionsModule.checkReactions(bot, emptyConfigCtx); err != ext.ContinueGroups {
		t.Fatalf("checkReactions(empty config) error = %v, want ContinueGroups", err)
	}

	if calls := client.callsFor("setMessageReaction"); len(calls) != 0 {
		t.Fatalf("setMessageReaction calls = %d, want none for no-op branches", len(calls))
	}
}

// TestReactionCommandsWorkWithNilCacheMarshal verifies the DB-backed repository
// still functions (bypassing cache) when the cache marshaler is unavailable,
// so reactions are not lost just because Redis is down.
func TestReactionCommandsWorkWithNilCacheMarshal(t *testing.T) {
	if db.DB == nil {
		t.Skip("requires database connection")
	}

	orig := cache.GetMarshal()
	cache.SetMarshal(nil)
	t.Cleanup(func() {
		cache.SetMarshal(orig)
	})

	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Reaction Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	t.Cleanup(func() {
		_ = reactions.ResetReactions(chat.Id)
	})

	addCtx := newModuleMessageContext(bot, chat, admin, "/addreaction hello 👍")
	if err := reactionsModule.addReaction(bot, addCtx); err != ext.EndGroups {
		t.Fatalf("addReaction(nil marshal) error = %v, want EndGroups", err)
	}
	if got := reactions.GetReactions(chat.Id)["hello"]; got != "👍" {
		t.Fatalf("stored reaction with nil marshal = %q, want 👍", got)
	}

	listCtx := newModuleMessageContext(bot, chat, admin, "/reactions")
	if err := reactionsModule.listReactions(bot, listCtx); err != ext.EndGroups {
		t.Fatalf("listReactions(nil marshal) error = %v, want EndGroups", err)
	}

	resetCtx := newModuleMessageContext(bot, chat, admin, "/resetreactions")
	if err := reactionsModule.resetReactions(bot, resetCtx); err != ext.EndGroups {
		t.Fatalf("resetReactions(nil marshal) error = %v, want EndGroups", err)
	}
	if m := reactions.GetReactions(chat.Id); len(m) != 0 {
		t.Fatalf("reactions remained after reset with nil marshal: %v", m)
	}

	checkCtx := newModuleMessageContext(bot, chat, admin, "hello")
	if err := reactionsModule.checkReactions(bot, checkCtx); err != ext.ContinueGroups {
		t.Fatalf("checkReactions(nil marshal) error = %v, want ContinueGroups", err)
	}
}

func TestLoadReactionsRegistersHelpAndHandlers(t *testing.T) {
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadReactions(dispatcher)

	if !DefaultHelpRegistry().AbleMap[reactionsModule.moduleName] {
		t.Fatal("reactions help registration = false, want enabled")
	}
	if got := DefaultHelpRegistry().AltHelpOptions["Reactions"]; len(got) != 1 || got[0] != "reaction" {
		t.Fatalf("reactions alt help = %v, want [reaction]", got)
	}
	if got := DefaultHelpRegistry().helpableKb["Reactions"]; len(got) != 1 || len(got[0]) != 2 {
		t.Fatalf("reactions help keyboard = %#v, want one row with two buttons", got)
	}
}
