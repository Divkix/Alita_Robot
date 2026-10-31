package modules

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"
	logrustest "github.com/sirupsen/logrus/hooks/test"

	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
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
	defaultHelpRegistry = newHelpRegistry()
	t.Cleanup(func() {
		defaultHelpRegistry = previousRegistry
		cachedBotUsername = ""
	})
	registry := DefaultHelpRegistry()
	registry.AbleMap["Admin"] = true
	registry.AltHelpOptions["Admin"] = []string{"admin"}
	setHelpKb(registry, "Admin", [][]gotgbot.InlineKeyboardButton{{{Text: "Admin", CallbackData: "admin-test"}}})

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

func TestStartDeepLinkDoesNotLogNormalDispatcherStopAsError(t *testing.T) {
	hook := logrustest.NewGlobal()
	t.Cleanup(hook.Reset)
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: 4313, FirstName: "Helper"}
	chat := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Helper"}
	ctx := newModuleMessageContext(bot, chat, user, "/start unknown_deeplink_test")
	if err := DefaultHelpRegistry().start(bot, ctx); err != ext.EndGroups {
		t.Fatalf("start(deep link) error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
	for _, entry := range hook.Entries {
		if entry.Message == ext.EndGroups.Error() {
			t.Fatal("normal deep-link completion logged as an error")
		}
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

func TestStartBlockedUserIsLoggedOnceByDispatcher(t *testing.T) {
	previousHooks := log.StandardLogger().ReplaceHooks(make(log.LevelHooks))
	t.Cleanup(func() { log.StandardLogger().ReplaceHooks(previousHooks) })
	hook := logrustest.NewGlobal()
	for _, text := range []string{"/start", "/start unknown_deeplink_test"} {
		t.Run(text, func(t *testing.T) {
			hook.Reset()
			client := newModuleBotClient()
			blocked := &gotgbot.TelegramError{
				Method: "sendMessage", Code: 403,
				Description: "Forbidden: bot was blocked by the user",
			}
			client.errors["sendMessage"] = blocked
			bot := newModuleTestBot(client)
			user := gotgbot.User{Id: 4314, FirstName: "Helper"}
			chat := gotgbot.Chat{Id: user.Id, Type: "private"}
			ctx := newModuleMessageContext(bot, chat, user, text)

			err := DefaultHelpRegistry().start(bot, ctx)
			if !errors.Is(err, blocked) {
				t.Fatalf("start error = %v, want blocked error", err)
			}
			if entries := hook.AllEntries(); len(entries) != 0 {
				t.Fatalf("handler logged before dispatcher: %v", entries)
			}
			helpers.LogHandlerError(ctx, err)
			entries := hook.AllEntries()
			if len(entries) != 1 || entries[0].Level != log.WarnLevel ||
				!strings.Contains(entries[0].Message, "Expected Telegram API error:") {
				t.Fatalf("want one expected warning, got %v", entries)
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
		encodeCallbackData("about", map[string]string{"a": "main"}),
	)
	if err := DefaultHelpRegistry().about(bot, mainCtx); err != ext.EndGroups {
		t.Fatalf("about(callback main) error = %v, want EndGroups", err)
	}

	meCtx := newModuleCallbackContext(bot, privateChat, user, encodeCallbackData("about", map[string]string{"a": "me"}))
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

	for _, data := range []string{
		encodeCallbackData("configuration", map[string]string{"s": "step1"}),
		encodeCallbackData("configuration", map[string]string{"s": "step2"}),
		encodeCallbackData("configuration", map[string]string{"s": "step3"}),
	} {
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
	defaultHelpRegistry = newHelpRegistry()
	t.Cleanup(func() {
		defaultHelpRegistry = previousRegistry
		cachedBotUsername = ""
	})

	registry := DefaultHelpRegistry()
	registry.AbleMap["Admin"] = true
	setHelpKb(registry, "Admin", [][]gotgbot.InlineKeyboardButton{
		{{Text: "Admin", CallbackData: "admin-test"}},
	})

	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: 4306, FirstName: "Helper"}
	chat := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Helper"}

	for _, data := range []string{
		encodeCallbackData("helpq", map[string]string{"m": "Help"}),
		encodeCallbackData("helpq", map[string]string{"m": "BackStart"}),
		encodeCallbackData("helpq", map[string]string{"m": "Admin"}),
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
	unknownConfig := newModuleCallbackContext(
		bot,
		privateChat,
		user,
		encodeCallbackData("configuration", map[string]string{"s": "step4"}),
	)
	if err := DefaultHelpRegistry().botConfig(bot, unknownConfig); err != ext.EndGroups {
		t.Fatalf("botConfig(unknown step) error = %v, want EndGroups", err)
	}
	unknownAbout := newModuleCallbackContext(
		bot,
		privateChat,
		user,
		encodeCallbackData("about", map[string]string{"a": "crafted"}),
	)
	if err := DefaultHelpRegistry().about(bot, unknownAbout); err != ext.EndGroups {
		t.Fatalf("about(unknown action) error = %v, want EndGroups", err)
	}

	if calls := client.callsFor("answerCallbackQuery"); len(calls) < 5 {
		t.Fatalf("answerCallbackQuery calls = %d, want all invalid callbacks answered", len(calls))
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

// Module help buttons follow the viewer's language, like the help text.
func TestModuleHelpButtonsFollowTheViewersLanguage(t *testing.T) {
	useRealLocales(t)
	previousRegistry := defaultHelpRegistry
	defaultHelpRegistry = newHelpRegistry()
	t.Cleanup(func() { defaultHelpRegistry = previousRegistry })

	LoadFilters(ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1}))

	en, err := i18n.MustNewTranslator("en").GetString("common_formatting_button")
	if err != nil {
		t.Fatalf("en common_formatting_button: %v", err)
	}
	fr, err := i18n.MustNewTranslator("fr").GetString("common_formatting_button")
	if err != nil {
		t.Fatalf("fr common_formatting_button: %v", err)
	}
	if fr == en {
		t.Fatalf("fr and en labels are both %q; the test needs a translated label", fr)
	}
	for lang, want := range map[string]string{"en": en, "fr": fr} {
		_, kb := getModuleHelpAndKb("filters", lang, DefaultHelpRegistry())
		if len(kb.InlineKeyboard) == 0 || len(kb.InlineKeyboard[0]) == 0 {
			t.Fatalf("%s: help keyboard is empty", lang)
		}
		if got := kb.InlineKeyboard[0][0].Text; got != want {
			t.Errorf("%s: first help button = %q, want %q", lang, got, want)
		}
	}
}

// /donate and the PM /help module list answer in the reader's language.
func TestDonateAndHelpListFollowTheReadersLanguage(t *testing.T) {
	useRealLocales(t)
	previousRegistry := defaultHelpRegistry
	defaultHelpRegistry = newHelpRegistry()
	t.Cleanup(func() {
		defaultHelpRegistry = previousRegistry
		cachedBotUsername = ""
	})
	DefaultHelpRegistry().AbleMap["Admin"] = true

	es := i18n.MustNewTranslator("es")
	en := i18n.MustNewTranslator("en")
	wantDonate, _ := es.GetString("help_donatetext")
	enDonate, _ := en.GetString("help_donatetext")
	wantBack, _ := es.GetString("helpers_back_button")
	enBack, _ := en.GetString("helpers_back_button")
	if wantDonate == "" || wantDonate == enDonate || wantBack == "" || wantBack == enBack {
		t.Fatal("the test needs es translations of help_donatetext and helpers_back_button")
	}

	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Lector"}
	group := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Grupo"}
	if err := lang.ChangeGroupLanguage(group.Id, "es"); err != nil {
		t.Fatalf("ChangeGroupLanguage: %v", err)
	}
	if err := lang.ChangeUserLanguage(user.Id, "es"); err != nil {
		t.Fatalf("ChangeUserLanguage: %v", err)
	}

	if err := DefaultHelpRegistry().donate(bot, newModuleMessageContext(bot, group, user, "/donate")); err != ext.EndGroups {
		t.Fatalf("donate() error = %v, want EndGroups", err)
	}
	sent := client.callsFor("sendMessage")
	if len(sent) != 1 {
		t.Fatalf("donate sent %d messages, want 1", len(sent))
	}
	if got, _ := sent[0].Params["text"].(string); got != wantDonate {
		t.Errorf("donate text = %q, want the es help_donatetext", got)
	}

	private := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: user.FirstName}
	if err := DefaultHelpRegistry().help(bot, newModuleMessageContext(bot, private, user, "/help")); err != ext.EndGroups {
		t.Fatalf("help() error = %v, want EndGroups", err)
	}
	sent = client.callsFor("sendMessage")
	if len(sent) != 2 {
		t.Fatalf("help sent %d messages, want 1", len(sent)-1)
	}
	kb, ok := sent[1].Params["reply_markup"].(*gotgbot.InlineKeyboardMarkup)
	if !ok || kb == nil || len(kb.InlineKeyboard) == 0 {
		t.Fatalf("help reply_markup = %#v, want the module keyboard", sent[1].Params["reply_markup"])
	}
	last := kb.InlineKeyboard[len(kb.InlineKeyboard)-1]
	if len(last) != 1 || last[0].Text != wantBack {
		t.Errorf("help keyboard last row = %#v, want the es helpers_back_button", last)
	}
}

// Extra words after the module name in PM still get the module's help.
func TestPrivateHelpWithExtraWordsAnswers(t *testing.T) {
	previousRegistry := defaultHelpRegistry
	defaultHelpRegistry = newHelpRegistry()
	t.Cleanup(func() {
		defaultHelpRegistry = previousRegistry
		cachedBotUsername = ""
	})
	registry := DefaultHelpRegistry()
	registry.AbleMap["Admin"] = true
	registry.AltHelpOptions["Admin"] = []string{"admin"}
	setHelpKb(registry, "Admin", [][]gotgbot.InlineKeyboardButton{{{Text: "Admin", CallbackData: "admin-test"}}})

	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Helper"}
	privateChat := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Helper"}

	for _, text := range []string{"/help admin", "/help admin extra"} {
		if err := DefaultHelpRegistry().help(bot, newModuleMessageContext(bot, privateChat, user, text)); err != ext.EndGroups {
			t.Fatalf("help(%q) error = %v, want EndGroups", text, err)
		}
	}
	sent := client.callsFor("sendMessage")
	if len(sent) != 2 {
		t.Fatalf("sendMessage calls = %d, want one reply per /help", len(sent))
	}
	for _, call := range sent {
		if got := call.Params["chat_id"]; got != privateChat.Id {
			t.Errorf("reply chat_id = %v, want %d", got, privateChat.Id)
		}
	}
	if sent[0].Params["text"] != sent[1].Params["text"] {
		t.Errorf("/help admin extra text = %q, want the /help admin reply %q", sent[1].Params["text"], sent[0].Params["text"])
	}
}

// Group /help <module> names the module in every language.
func TestGroupModuleHelpNamesTheModuleInChatLanguage(t *testing.T) {
	useRealLocales(t)
	previousRegistry := defaultHelpRegistry
	defaultHelpRegistry = newHelpRegistry()
	t.Cleanup(func() {
		defaultHelpRegistry = previousRegistry
		cachedBotUsername = ""
	})
	registry := DefaultHelpRegistry()
	registry.AbleMap["Admin"] = true
	registry.AltHelpOptions["Admin"] = []string{"admin"}

	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Helper"}
	group := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Grupo"}
	if err := lang.ChangeGroupLanguage(group.Id, "es"); err != nil {
		t.Fatalf("ChangeGroupLanguage: %v", err)
	}

	if err := DefaultHelpRegistry().help(bot, newModuleMessageContext(bot, group, user, "/help admin")); err != ext.EndGroups {
		t.Fatalf("help() error = %v, want EndGroups", err)
	}
	sent := client.callsFor("sendMessage")
	if len(sent) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(sent))
	}
	want, _ := i18n.MustNewTranslator("es").GetString("help_contact_pm_module", i18n.TranslationParams{"module": "Admin"})
	if got, _ := sent[0].Params["text"].(string); got != want || !strings.Contains(got, "Admin") {
		t.Errorf("group /help admin text = %q, want %q", got, want)
	}
}

// Extra words after the module name still get that module's PM deep link.
func TestGroupHelpWithExtraWordsLinksTheModule(t *testing.T) {
	useRealLocales(t)
	previousRegistry := defaultHelpRegistry
	defaultHelpRegistry = newHelpRegistry()
	t.Cleanup(func() {
		defaultHelpRegistry = previousRegistry
		cachedBotUsername = ""
	})
	registry := DefaultHelpRegistry()
	registry.AbleMap["Admin"] = true

	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Helper"}
	group := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Help Chat"}

	if err := DefaultHelpRegistry().help(bot, newModuleMessageContext(bot, group, user, "/help admin extra")); err != ext.EndGroups {
		t.Fatalf("help() error = %v, want EndGroups", err)
	}
	sent := client.callsFor("sendMessage")
	if len(sent) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(sent))
	}
	want, _ := i18n.MustNewTranslator("en").GetString("help_contact_pm_module", i18n.TranslationParams{"module": "Admin"})
	if got, _ := sent[0].Params["text"].(string); got != want {
		t.Errorf("group /help admin extra text = %q, want %q", got, want)
	}
	markup, ok := sent[0].Params["reply_markup"].(gotgbot.InlineKeyboardMarkup)
	if !ok || len(markup.InlineKeyboard) == 0 || len(markup.InlineKeyboard[0]) == 0 {
		t.Fatalf("reply_markup = %#v, want the PM link button", sent[0].Params["reply_markup"])
	}
	if url := markup.InlineKeyboard[0][0].Url; !strings.HasSuffix(url, "?start=help_admin") {
		t.Errorf("PM link = %q, want the admin module deep link", url)
	}
}

// The main help text links the docs site; no reply showing it may expand
// that link into a preview card.
func TestHelpRepliesDisableLinkPreviews(t *testing.T) {
	previousRegistry := defaultHelpRegistry
	defaultHelpRegistry = newHelpRegistry()
	t.Cleanup(func() {
		defaultHelpRegistry = previousRegistry
		cachedBotUsername = ""
	})
	registry := DefaultHelpRegistry()
	registry.AbleMap["Admin"] = true

	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Helper"}
	privateChat := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Helper"}

	for _, text := range []string{"/help", "/help admin", "/help nosuchmodule"} {
		if err := DefaultHelpRegistry().help(bot, newModuleMessageContext(bot, privateChat, user, text)); err != ext.EndGroups {
			t.Fatalf("help(%q) error = %v, want EndGroups", text, err)
		}
	}
	for _, module := range []string{"help", "admin"} {
		if _, err := sendHelpkb(bot, newModuleMessageContext(bot, privateChat, user, "/start help_"+module), module, registry); err != nil {
			t.Fatalf("sendHelpkb(%q) error = %v", module, err)
		}
	}
	sent := client.callsFor("sendMessage")
	if len(sent) != 5 {
		t.Fatalf("sendMessage calls = %d, want 5", len(sent))
	}
	for i, call := range sent {
		var raw []byte
		if encoded, ok := call.Params["link_preview_options"].(string); ok {
			raw = []byte(encoded)
		} else {
			var err error
			if raw, err = json.Marshal(call.Params["link_preview_options"]); err != nil {
				t.Fatalf("marshal link_preview_options: %v", err)
			}
		}
		var opts gotgbot.LinkPreviewOptions
		if err := json.Unmarshal(raw, &opts); err != nil || !opts.IsDisabled {
			t.Errorf("reply %d link_preview_options = %s, want previews disabled", i, raw)
		}
	}
}
