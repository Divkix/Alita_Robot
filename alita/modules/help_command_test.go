package modules

import (
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func TestStartCommandRepliesInPrivateAndGroup(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: 4301, FirstName: "Helper"}
	privateChat := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Helper"}
	groupChat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Help Chat"}
	t.Cleanup(func() {
		cachedBotUsername = ""
	})

	privateCtx := newModuleMessageContext(bot, privateChat, user, "/start")
	if err := DefaultHelpRegistry().start(bot, privateCtx); err != ext.EndGroups {
		t.Fatalf("start(private) error = %v, want EndGroups", err)
	}

	groupCtx := newModuleMessageContext(bot, groupChat, user, "/start")
	if err := DefaultHelpRegistry().start(bot, groupCtx); err != ext.EndGroups {
		t.Fatalf("start(group) error = %v, want EndGroups", err)
	}

	if calls := client.callsFor("sendMessage"); len(calls) != 2 {
		t.Fatalf("sendMessage calls = %d, want 2", len(calls))
	}
}

func TestHelpCommandRepliesInPrivateAndGroup(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: 4302, FirstName: "Helper"}
	privateChat := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Helper"}
	groupChat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Help Chat"}

	privateCtx := newModuleMessageContext(bot, privateChat, user, "/help")
	if err := DefaultHelpRegistry().help(bot, privateCtx); err != ext.EndGroups {
		t.Fatalf("help(private) error = %v, want EndGroups", err)
	}

	groupCtx := newModuleMessageContext(bot, groupChat, user, "/help")
	if err := DefaultHelpRegistry().help(bot, groupCtx); err != ext.EndGroups {
		t.Fatalf("help(group) error = %v, want EndGroups", err)
	}

	if calls := client.callsFor("sendMessage"); len(calls) != 2 {
		t.Fatalf("sendMessage calls = %d, want 2", len(calls))
	}
}

func TestAboutCommandAndCallbacksSendOrEditMessages(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: 4303, FirstName: "Helper"}
	privateChat := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Helper"}
	groupChat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Help Chat"}
	t.Cleanup(func() {
		cachedBotUsername = ""
	})

	privateCtx := newModuleMessageContext(bot, privateChat, user, "/about")
	if err := DefaultHelpRegistry().about(bot, privateCtx); err != ext.EndGroups {
		t.Fatalf("about(private) error = %v, want EndGroups", err)
	}

	groupCtx := newModuleMessageContext(bot, groupChat, user, "/about")
	if err := DefaultHelpRegistry().about(bot, groupCtx); err != ext.EndGroups {
		t.Fatalf("about(group) error = %v, want EndGroups", err)
	}

	mainCtx := newModuleCallbackContext(
		bot,
		privateChat,
		user,
		encodeCallbackData("about", map[string]string{"a": "main"}, "about.main"),
	)
	if err := DefaultHelpRegistry().about(bot, mainCtx); err != ext.EndGroups {
		t.Fatalf("about(callback main) error = %v, want EndGroups", err)
	}

	meCtx := newModuleCallbackContext(bot, privateChat, user, "about.me")
	if err := DefaultHelpRegistry().about(bot, meCtx); err != ext.EndGroups {
		t.Fatalf("about(callback me) error = %v, want EndGroups", err)
	}

	if calls := client.callsFor("sendMessage"); len(calls) != 2 {
		t.Fatalf("sendMessage calls = %d, want 2", len(calls))
	}
	if calls := client.callsFor("editMessageText"); len(calls) != 2 {
		t.Fatalf("editMessageText calls = %d, want 2", len(calls))
	}
	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 2 {
		t.Fatalf("answerCallbackQuery calls = %d, want 2", len(calls))
	}
}

func TestBotConfigCallbackStepsEditAndAnswer(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: 4304, FirstName: "Helper"}
	privateChat := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Helper"}

	for _, data := range []string{"configuration.step1", "configuration.step2", "configuration.step3"} {
		ctx := newModuleCallbackContext(bot, privateChat, user, data)
		if err := DefaultHelpRegistry().botConfig(bot, ctx); err != ext.EndGroups {
			t.Fatalf("botConfig(%s) error = %v, want EndGroups", data, err)
		}
	}

	if calls := client.callsFor("editMessageText"); len(calls) != 3 {
		t.Fatalf("editMessageText calls = %d, want 3", len(calls))
	}
	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 3 {
		t.Fatalf("answerCallbackQuery calls = %d, want 3", len(calls))
	}
}

func TestDonateSendsMessage(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: 4305, FirstName: "Helper"}
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Help Chat"}
	ctx := newModuleMessageContext(bot, chat, user, "/donate")

	if err := DefaultHelpRegistry().donate(bot, ctx); err != ext.EndGroups {
		t.Fatalf("donate() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
}
