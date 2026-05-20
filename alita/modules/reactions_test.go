package modules

import (
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

func TestReactionKey(t *testing.T) {
	t.Parallel()

	if got := reactionKey(-1001234567890); got != "alita:reactions:-1001234567890" {
		t.Fatalf("reactionKey() = %q", got)
	}
}

func TestReactionCommandsManageCache(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Reaction Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	key := reactionKey(chat.Id)
	t.Cleanup(func() {
		_ = cache.Marshal.Delete(cache.Context, key)
	})

	addCtx := newModuleMessageContext(bot, chat, admin, "/addreaction hello ok")
	if err := reactionsModule.addReaction(bot, addCtx); err != ext.EndGroups {
		t.Fatalf("addReaction() error = %v, want EndGroups", err)
	}
	existing, err := cache.Marshal.Get(cache.Context, key, new(map[string]string))
	if err != nil {
		t.Fatalf("cached reactions missing after add: %v", err)
	}
	if got := (*existing.(*map[string]string))["hello"]; got != "ok" {
		t.Fatalf("cached reaction = %q, want ok", got)
	}

	listCtx := newModuleMessageContext(bot, chat, admin, "/reactions")
	if err := reactionsModule.listReactions(bot, listCtx); err != ext.EndGroups {
		t.Fatalf("listReactions() error = %v, want EndGroups", err)
	}

	removeCtx := newModuleMessageContext(bot, chat, admin, "/removereaction hello")
	if err := reactionsModule.removeReaction(bot, removeCtx); err != ext.EndGroups {
		t.Fatalf("removeReaction() error = %v, want EndGroups", err)
	}
	if _, err := cache.Marshal.Get(cache.Context, key, new(map[string]string)); err == nil {
		t.Fatal("reaction cache remained after removing final reaction")
	}

	_ = cache.Marshal.Set(cache.Context, key, map[string]string{"bye": "ok"})
	resetCtx := newModuleMessageContext(bot, chat, admin, "/resetreactions")
	if err := reactionsModule.resetReactions(bot, resetCtx); err != ext.EndGroups {
		t.Fatalf("resetReactions() error = %v, want EndGroups", err)
	}
	if _, err := cache.Marshal.Get(cache.Context, key, new(map[string]string)); err == nil {
		t.Fatal("reaction cache remained after reset")
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 4 {
		t.Fatalf("sendMessage calls = %d, want 4", len(calls))
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
		encodeCallbackData("reactions_help", map[string]string{"action": "add"}, "reactions_help.add"),
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

func TestCheckReactionsSetsMessageReactionForMatchingKeyword(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Reaction Chat"}
	user := gotgbot.User{Id: 4307, FirstName: "Member"}
	key := reactionKey(chat.Id)
	t.Cleanup(func() {
		_ = cache.Marshal.Delete(cache.Context, key)
	})

	DefaultHelpRegistry().AbleMap.Store(reactionsModule.moduleName, true)
	if err := cache.Marshal.Set(cache.Context, key, map[string]string{"hello": "ok"}); err != nil {
		t.Fatalf("seed reaction cache: %v", err)
	}

	ctx := newModuleMessageContext(bot, chat, user, "well hello there")
	if err := reactionsModule.checkReactions(bot, ctx); err != ext.ContinueGroups {
		t.Fatalf("checkReactions() error = %v, want ContinueGroups", err)
	}
	if calls := client.callsFor("setMessageReaction"); len(calls) != 1 {
		t.Fatalf("setMessageReaction calls = %d, want 1", len(calls))
	}
}
