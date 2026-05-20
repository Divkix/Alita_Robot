package modules

import (
	"errors"
	"strings"
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

func TestHelpCommandRoutesSpecificModuleInPrivateAndGroup(t *testing.T) {
	previousRegistry := defaultHelpRegistry
	previousMarkup := markup
	defaultHelpRegistry = NewHelpRegistry()
	t.Cleanup(func() {
		defaultHelpRegistry = previousRegistry
		markup = previousMarkup
		cachedBotUsername = ""
	})
	registry := DefaultHelpRegistry()
	registry.AbleMap.Store("Admin", true)
	registry.AltHelpOptions["Admin"] = []string{"admin"}
	registry.helpableKb["Admin"] = [][]gotgbot.InlineKeyboardButton{{{Text: "Admin", CallbackData: "admin-test"}}}
	initHelpButtons()

	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: 4310, FirstName: "Helper"}
	privateChat := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Helper"}
	groupChat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Help Chat"}

	privateCtx := newModuleMessageContext(bot, privateChat, user, "/help admin")
	if err := DefaultHelpRegistry().help(bot, privateCtx); err != ext.EndGroups {
		t.Fatalf("help(private module) error = %v, want EndGroups", err)
	}

	groupCtx := newModuleMessageContext(bot, groupChat, user, "/help admin")
	groupCtx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{
		MessageId: 88,
		Date:      1,
		Chat:      groupChat,
		From:      &user,
		Text:      "question",
	}
	if err := DefaultHelpRegistry().help(bot, groupCtx); err != ext.EndGroups {
		t.Fatalf("help(group module) error = %v, want EndGroups", err)
	}

	if calls := client.callsFor("sendMessage"); len(calls) != 2 {
		t.Fatalf("sendMessage calls = %d, want private module and group PM link replies", len(calls))
	}
}

func TestStartCommandHandlesDeepLinkAndUnexpectedArgCount(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: 4311, FirstName: "Helper"}
	privateChat := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Helper"}

	deepLinkCtx := newModuleMessageContext(bot, privateChat, user, "/start rules_test")
	if err := DefaultHelpRegistry().start(bot, deepLinkCtx); err != ext.EndGroups {
		t.Fatalf("start(deep link) error = %v, want EndGroups", err)
	}

	unexpectedCtx := newModuleMessageContext(bot, privateChat, user, "/start a b")
	if err := DefaultHelpRegistry().start(bot, unexpectedCtx); err != ext.EndGroups {
		t.Fatalf("start(unexpected args) error = %v, want EndGroups", err)
	}
}

func TestHelpAndStartPropagateSendFailures(t *testing.T) {
	requestErr := errors.New("telegram unavailable")
	user := gotgbot.User{Id: 4312, FirstName: "Helper"}

	for _, tt := range []struct {
		name string
		text string
		run  func(*gotgbot.Bot, *ext.Context) error
	}{
		{name: "start private", text: "/start", run: DefaultHelpRegistry().start},
		{name: "start group", text: "/start", run: DefaultHelpRegistry().start},
		{name: "help private", text: "/help", run: DefaultHelpRegistry().help},
		{name: "help group", text: "/help", run: DefaultHelpRegistry().help},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := newModuleBotClient()
			client.errors["sendMessage"] = requestErr
			bot := newModuleTestBot(client)
			chat := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Helper"}
			if strings.Contains(tt.name, "group") {
				chat = gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Help Chat"}
			}
			ctx := newModuleMessageContext(bot, chat, user, tt.text)

			err := tt.run(bot, ctx)
			if !errors.Is(err, requestErr) {
				t.Fatalf("%s error = %v, want request error", tt.name, err)
			}
		})
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

func TestHelpButtonHandlerRoutesStartMainAndModuleCallbacks(t *testing.T) {
	previousRegistry := defaultHelpRegistry
	previousMarkup := markup
	defaultHelpRegistry = NewHelpRegistry()
	t.Cleanup(func() {
		defaultHelpRegistry = previousRegistry
		markup = previousMarkup
		cachedBotUsername = ""
	})

	registry := DefaultHelpRegistry()
	registry.AbleMap.Store("Admin", true)
	registry.helpableKb["Admin"] = [][]gotgbot.InlineKeyboardButton{
		{{Text: "Admin", CallbackData: "admin-test"}},
	}
	initHelpButtons()

	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: 4306, FirstName: "Helper"}
	chat := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Helper"}

	for _, data := range []string{
		encodeCallbackData("helpq", map[string]string{"m": "Help"}, "helpq.Help"),
		"helpq.BackStart",
		encodeCallbackData("helpq", map[string]string{"m": "Admin"}, "helpq.Admin"),
	} {
		ctx := newModuleCallbackContext(bot, chat, user, data)
		if err := DefaultHelpRegistry().helpButtonHandler(bot, ctx); err != ext.EndGroups {
			t.Fatalf("helpButtonHandler(%s) error = %v, want EndGroups", data, err)
		}
	}

	if calls := client.callsFor("editMessageText"); len(calls) != 3 {
		t.Fatalf("editMessageText calls = %d, want 3", len(calls))
	}
	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 3 {
		t.Fatalf("answerCallbackQuery calls = %d, want 3", len(calls))
	}
}

func TestHelpButtonAndConfigCallbacksRejectInvalidMessages(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: 4307, FirstName: "Helper"}
	privateChat := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Helper"}
	groupChat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Help Chat"}

	invalidHelp := newModuleCallbackContext(bot, privateChat, user, "helpq")
	if err := DefaultHelpRegistry().helpButtonHandler(bot, invalidHelp); err != ext.EndGroups {
		t.Fatalf("helpButtonHandler(invalid) error = %v, want EndGroups", err)
	}

	missingMessage := newModuleCallbackContext(bot, privateChat, user, "helpq.Help")
	missingMessage.CallbackQuery.Message = nil
	if err := DefaultHelpRegistry().helpButtonHandler(bot, missingMessage); err != ext.EndGroups {
		t.Fatalf("helpButtonHandler(nil message) error = %v, want EndGroups", err)
	}

	groupConfig := newModuleCallbackContext(bot, groupChat, user, "configuration.step1")
	if err := DefaultHelpRegistry().botConfig(bot, groupConfig); err != ext.EndGroups {
		t.Fatalf("botConfig(group) error = %v, want EndGroups", err)
	}

	invalidConfig := newModuleCallbackContext(bot, privateChat, user, "configuration")
	if err := DefaultHelpRegistry().botConfig(bot, invalidConfig); err != ext.EndGroups {
		t.Fatalf("botConfig(invalid) error = %v, want EndGroups", err)
	}

	if calls := client.callsFor("answerCallbackQuery"); len(calls) < 3 {
		t.Fatalf("answerCallbackQuery calls = %d, want at least invalid callbacks answered", len(calls))
	}
}

func TestLoadHelpRegistersHandlers(t *testing.T) {
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadHelp(dispatcher)
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
