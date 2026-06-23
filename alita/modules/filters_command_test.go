package modules

import (
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/filters"
)

func waitForModuleCondition(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition was not met before timeout")
}

func TestAddListWatchAndRemoveTextFilter(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Filter Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	member := gotgbot.User{Id: 42, FirstName: "Member"}

	addCtx := newModuleMessageContext(bot, chat, admin, "/filter hello Hi there")
	if err := filtersModule.addFilter(bot, addCtx); err != ext.EndGroups {
		t.Fatalf("addFilter error = %v, want EndGroups", err)
	}
	if !filters.DoesFilterExists(chat.Id, "hello") {
		t.Fatal("filter was not stored")
	}

	listCtx := newModuleMessageContext(bot, chat, admin, "/filters")
	if err := filtersModule.filtersList(bot, listCtx); err != ext.EndGroups {
		t.Fatalf("filtersList error = %v, want EndGroups", err)
	}

	watchCtx := newModuleMessageContext(bot, chat, member, "well hello everyone")
	if err := filtersModule.filtersWatcher(bot, watchCtx); err != ext.ContinueGroups {
		t.Fatalf("filtersWatcher error = %v, want ContinueGroups", err)
	}
	calls := client.callsFor("sendMessage")
	if len(calls) < 3 {
		t.Fatalf("sendMessage calls = %d, want add/list/watch replies", len(calls))
	}
	lastText := calls[len(calls)-1].Params["text"].(string)
	if !strings.Contains(lastText, "Hi there") {
		t.Fatalf("filter watcher text = %q, want stored reply", lastText)
	}

	removeCtx := newModuleMessageContext(bot, chat, admin, "/stop hello")
	if err := filtersModule.rmFilter(bot, removeCtx); err != ext.EndGroups {
		t.Fatalf("rmFilter error = %v, want EndGroups", err)
	}
	if filters.DoesFilterExists(chat.Id, "hello") {
		t.Fatal("filter still exists after remove")
	}
}

func TestFilterCommandValidationBranches(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Filter Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	replyWithoutKeyword := newModuleMessageContext(bot, chat, admin, "/filter")
	replyWithoutKeyword.EffectiveMessage.ReplyToMessage = &gotgbot.Message{
		MessageId: 222,
		Date:      1,
		Chat:      chat,
		From:      &gotgbot.User{Id: 42, FirstName: "Member"},
		Text:      "reply text",
	}
	if err := filtersModule.addFilter(bot, replyWithoutKeyword); err != ext.EndGroups {
		t.Fatalf("addFilter reply without keyword error = %v, want EndGroups", err)
	}

	for _, text := range []string{
		"/filter",
		"/filter " + strings.Repeat("x", 101) + " value",
	} {
		ctx := newModuleMessageContext(bot, chat, admin, text)
		if err := filtersModule.addFilter(bot, ctx); err != ext.EndGroups {
			t.Fatalf("addFilter(%q) error = %v, want EndGroups", text, err)
		}
	}

	if err := filters.AddFilter(chat.Id, "dupe", "old", "", nil, db.TEXT); err != nil {
		t.Fatalf("AddFilter setup error = %v", err)
	}
	overwriteCtx := newModuleMessageContext(bot, chat, admin, "/filter dupe new")
	if err := filtersModule.addFilter(bot, overwriteCtx); err != ext.EndGroups {
		t.Fatalf("addFilter duplicate error = %v, want EndGroups", err)
	}
	lastCall := client.callsFor("sendMessage")[len(client.callsFor("sendMessage"))-1]
	if lastCall.Params["reply_markup"] == nil {
		t.Fatal("duplicate filter prompt did not include overwrite buttons")
	}
}

func TestAddFilterRejectsLimitAndNonAdmin(t *testing.T) {
	t.Run("limit exceeded", func(t *testing.T) {
		client := newModuleBotClient()
		bot := newModuleTestBot(client)
		chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Filter Chat"}
		admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
		for i := 0; i < 150; i++ {
			if err := filters.AddFilter(chat.Id, "word"+strconv.Itoa(i), "reply", "", nil, db.TEXT); err != nil {
				t.Fatalf("AddFilter setup %d error = %v", i, err)
			}
		}

		ctx := newModuleMessageContext(bot, chat, admin, "/filter overflow nope")
		if err := filtersModule.addFilter(bot, ctx); err != ext.EndGroups {
			t.Fatalf("addFilter limit error = %v, want EndGroups", err)
		}
		if filters.DoesFilterExists(chat.Id, "overflow") {
			t.Fatal("filter was stored despite limit")
		}
	})

	t.Run("non admin", func(t *testing.T) {
		client := newModuleBotClient()
		bot := newModuleTestBot(client)
		chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Filter Chat"}
		member := gotgbot.User{Id: 42, FirstName: "Member"}
		ctx := newModuleMessageContext(bot, chat, member, "/filter hello nope")

		if err := filtersModule.addFilter(bot, ctx); err != ext.EndGroups {
			t.Fatalf("addFilter non-admin error = %v, want EndGroups", err)
		}
		if filters.DoesFilterExists(chat.Id, "hello") {
			t.Fatal("filter was stored by non-admin")
		}
	})
}

func TestRemoveAndListFilterValidationBranches(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Filter Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	for _, text := range []string{"/stop", "/stop missing"} {
		ctx := newModuleMessageContext(bot, chat, admin, text)
		if err := filtersModule.rmFilter(bot, ctx); err != ext.EndGroups {
			t.Fatalf("rmFilter(%q) error = %v, want EndGroups", text, err)
		}
	}

	listCtx := newModuleMessageContext(bot, chat, admin, "/filters")
	listCtx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{
		MessageId: 303,
		Date:      1,
		Chat:      chat,
		From:      &gotgbot.User{Id: 42, FirstName: "Member"},
		Text:      "thread source",
	}
	if err := filtersModule.filtersList(bot, listCtx); err != ext.EndGroups {
		t.Fatalf("filtersList empty with reply error = %v, want EndGroups", err)
	}
	calls := client.callsFor("sendMessage")
	if len(calls) != 3 {
		t.Fatalf("sendMessage calls = %d, want remove missing and empty list replies", len(calls))
	}
	if calls[len(calls)-1].Params["reply_parameters"] == nil {
		t.Fatal("filtersList did not anchor reply to replied message")
	}
}

func TestFiltersWatcherNoFormatPathRequiresAdmin(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Filter Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	member := gotgbot.User{Id: 42, FirstName: "Member"}
	if err := filters.AddFilter(chat.Id, "raw", "<b>Raw</b>", "", nil, db.TEXT); err != nil {
		t.Fatalf("AddFilter setup error = %v", err)
	}

	memberCtx := newModuleMessageContext(bot, chat, member, "raw noformat")
	if err := filtersModule.filtersWatcher(bot, memberCtx); err != ext.EndGroups {
		t.Fatalf("filtersWatcher member noformat error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want admin denial", len(calls))
	}

	adminCtx := newModuleMessageContext(bot, chat, admin, "raw noformat")
	if err := filtersModule.filtersWatcher(bot, adminCtx); err != ext.ContinueGroups {
		t.Fatalf("filtersWatcher admin noformat error = %v, want ContinueGroups", err)
	}
	calls := client.callsFor("sendMessage")
	if len(calls) != 2 {
		t.Fatalf("sendMessage calls = %d, want denial plus raw filter reply", len(calls))
	}
	if text := calls[len(calls)-1].Params["text"].(string); !strings.Contains(text, "Raw") {
		t.Fatalf("raw filter reply = %q, want stored content", text)
	}
}

func TestFilterOverwriteCallbackReplacesExistingFilter(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Filter Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	if err := filters.AddFilter(chat.Id, "hello", "old reply", "", nil, db.TEXT); err != nil {
		t.Fatalf("AddFilter setup error = %v", err)
	}
	if err := setFilterOverwriteCache("token-1", overwriteFilter{
		overwriteBase: overwriteBase{
			ChatID:   chat.Id,
			ItemName: "hello",
			Text:     "new reply",
			DataType: db.TEXT,
		},
	}); err != nil {
		t.Fatalf("setFilterOverwriteCache setup error = %v", err)
	}

	data := encodeCallbackData(
		"filters_overwrite",
		map[string]string{"a": "yes", "t": "token-1"},
	)
	ctx := newModuleCallbackContext(bot, chat, admin, data)
	if err := filtersModule.filterOverWriteHandler(bot, ctx); err != ext.EndGroups {
		t.Fatalf("filterOverWriteHandler error = %v, want EndGroups", err)
	}

	watchCtx := newModuleMessageContext(bot, chat, gotgbot.User{Id: 42, FirstName: "Member"}, "hello")
	if err := filtersModule.filtersWatcher(bot, watchCtx); err != ext.ContinueGroups {
		t.Fatalf("filtersWatcher error = %v, want ContinueGroups", err)
	}
	calls := client.callsFor("sendMessage")
	lastText := calls[len(calls)-1].Params["text"].(string)
	if !strings.Contains(lastText, "new reply") {
		t.Fatalf("filter reply after overwrite = %q, want new reply", lastText)
	}
}

func TestFilterOverwriteCallbackCancelAndExpired(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Filter Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	cancelData := encodeCallbackData("filters_overwrite", map[string]string{"a": "cancel"})
	cancelCtx := newModuleCallbackContext(bot, chat, admin, cancelData)
	if err := filtersModule.filterOverWriteHandler(bot, cancelCtx); err != ext.EndGroups {
		t.Fatalf("filterOverWriteHandler cancel error = %v, want EndGroups", err)
	}

	expiredData := encodeCallbackData("filters_overwrite", map[string]string{"a": "yes", "t": "missing"})
	expiredCtx := newModuleCallbackContext(bot, chat, admin, expiredData)
	if err := filtersModule.filterOverWriteHandler(bot, expiredCtx); err != ext.EndGroups {
		t.Fatalf("filterOverWriteHandler expired error = %v, want EndGroups", err)
	}

	if calls := client.callsFor("editMessageText"); len(calls) != 2 {
		t.Fatalf("editMessageText calls = %d, want one edit per callback", len(calls))
	}
	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 2 {
		t.Fatalf("answerCallbackQuery calls = %d, want one answer per callback", len(calls))
	}
}

func TestRemoveAllFiltersConfirmationAndCallback(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Filter Chat"}
	owner := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	if err := filters.AddFilter(chat.Id, "one", "1", "", nil, db.TEXT); err != nil {
		t.Fatalf("AddFilter setup error = %v", err)
	}
	if err := filters.AddFilter(chat.Id, "two", "2", "", nil, db.TEXT); err != nil {
		t.Fatalf("AddFilter setup error = %v", err)
	}

	confirmCtx := newModuleMessageContext(bot, chat, owner, "/stopall")
	if err := filtersModule.rmAllFilters(bot, confirmCtx); err != ext.EndGroups {
		t.Fatalf("rmAllFilters error = %v, want EndGroups", err)
	}
	calls := client.callsFor("sendMessage")
	if len(calls) != 1 || calls[0].Params["reply_markup"] == nil {
		t.Fatalf("rmAllFilters confirmation calls = %+v, want reply markup", calls)
	}

	data := encodeCallbackData("rmAllFilters", map[string]string{"a": "yes"})
	callbackCtx := newModuleCallbackContext(bot, chat, owner, data)
	if err := filtersModule.filtersButtonHandler(bot, callbackCtx); err != ext.EndGroups {
		t.Fatalf("filtersButtonHandler error = %v, want EndGroups", err)
	}
	waitForModuleCondition(t, func() bool {
		return len(filters.GetFiltersList(chat.Id)) == 0
	})
}

func TestRemoveAllFiltersEmptyCancelAndInvalidCallback(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Filter Chat"}
	owner := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	emptyCtx := newModuleMessageContext(bot, chat, owner, "/stopall")
	if err := filtersModule.rmAllFilters(bot, emptyCtx); err != ext.EndGroups {
		t.Fatalf("rmAllFilters empty error = %v, want EndGroups", err)
	}

	if err := filters.AddFilter(chat.Id, "keep", "reply", "", nil, db.TEXT); err != nil {
		t.Fatalf("AddFilter setup error = %v", err)
	}
	cancelCtx := newModuleCallbackContext(bot, chat, owner, "rmAllFilters.no")
	if err := filtersModule.filtersButtonHandler(bot, cancelCtx); err != ext.EndGroups {
		t.Fatalf("filtersButtonHandler cancel error = %v, want EndGroups", err)
	}
	if !filters.DoesFilterExists(chat.Id, "keep") {
		t.Fatal("filter was removed after cancel callback")
	}

	invalidCtx := newModuleCallbackContext(bot, chat, owner, "rmAllFilters")
	if err := filtersModule.filtersButtonHandler(bot, invalidCtx); err != ext.EndGroups {
		t.Fatalf("filtersButtonHandler invalid error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 2 {
		t.Fatalf("answerCallbackQuery calls = %d, want cancel and invalid acknowledgements", len(calls))
	}
}

func TestFilterCommandsPropagateGotgbotRequestErrors(t *testing.T) {
	requestErr := errors.New("telegram request failed")
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	member := gotgbot.User{Id: 42, FirstName: "Member"}

	for _, tt := range []struct {
		name  string
		text  string
		setup func(t *testing.T, chat gotgbot.Chat)
		run   func(*gotgbot.Bot, *ext.Context) error
		user  gotgbot.User
	}{
		{name: "add filter validation reply", text: "/filter", run: filtersModule.addFilter, user: admin},
		{name: "add filter success reply", text: "/filter hello Hi", run: filtersModule.addFilter, user: admin},
		{
			name: "add filter overwrite confirmation reply",
			text: "/filter hello New",
			setup: func(t *testing.T, chat gotgbot.Chat) {
				t.Helper()
				if err := filters.AddFilter(chat.Id, "hello", "old", "", nil, db.TEXT); err != nil {
					t.Fatalf("AddFilter setup error = %v", err)
				}
			},
			run:  filtersModule.addFilter,
			user: admin,
		},
		{name: "remove filter missing keyword reply", text: "/stop", run: filtersModule.rmFilter, user: admin},
		{name: "remove filter missing filter reply", text: "/stop missing", run: filtersModule.rmFilter, user: admin},
		{
			name: "remove filter success reply",
			text: "/stop hello",
			setup: func(t *testing.T, chat gotgbot.Chat) {
				t.Helper()
				if err := filters.AddFilter(chat.Id, "hello", "old", "", nil, db.TEXT); err != nil {
					t.Fatalf("AddFilter setup error = %v", err)
				}
			},
			run:  filtersModule.rmFilter,
			user: admin,
		},
		{name: "filters empty list reply", text: "/filters", run: filtersModule.filtersList, user: member},
		{
			name: "filters populated list reply",
			text: "/filters",
			setup: func(t *testing.T, chat gotgbot.Chat) {
				t.Helper()
				if err := filters.AddFilter(chat.Id, "hello", "old", "", nil, db.TEXT); err != nil {
					t.Fatalf("AddFilter setup error = %v", err)
				}
			},
			run:  filtersModule.filtersList,
			user: member,
		},
		{name: "remove all empty reply", text: "/stopall", run: filtersModule.rmAllFilters, user: admin},
		{
			name: "remove all confirmation reply",
			text: "/stopall",
			setup: func(t *testing.T, chat gotgbot.Chat) {
				t.Helper()
				if err := filters.AddFilter(chat.Id, "hello", "old", "", nil, db.TEXT); err != nil {
					t.Fatalf("AddFilter setup error = %v", err)
				}
			},
			run:  filtersModule.rmAllFilters,
			user: admin,
		},
		{
			name: "watcher formatted reply",
			text: "hello",
			setup: func(t *testing.T, chat gotgbot.Chat) {
				t.Helper()
				if err := filters.AddFilter(chat.Id, "hello", "old", "", nil, db.TEXT); err != nil {
					t.Fatalf("AddFilter setup error = %v", err)
				}
			},
			run:  filtersModule.filtersWatcher,
			user: member,
		},
		{
			name: "watcher noformat reply",
			text: "hello noformat",
			setup: func(t *testing.T, chat gotgbot.Chat) {
				t.Helper()
				if err := filters.AddFilter(chat.Id, "hello", "old", "", nil, db.TEXT); err != nil {
					t.Fatalf("AddFilter setup error = %v", err)
				}
			},
			run:  filtersModule.filtersWatcher,
			user: admin,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			client.errors["sendMessage"] = requestErr
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Filter Chat"}
			if tt.setup != nil {
				tt.setup(t, chat)
			}
			ctx := newModuleMessageContext(bot, chat, tt.user, tt.text)

			err := tt.run(bot, ctx)
			if !errors.Is(err, requestErr) {
				t.Fatalf("%s returned error %v, want request error", tt.text, err)
			}
		})
	}
}

func TestFilterCallbackHandlersPropagateGotgbotRequestErrors(t *testing.T) {
	requestErr := errors.New("telegram request failed")
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	for _, tt := range []struct {
		name   string
		method string
		run    func(*gotgbot.Bot, *ext.Context) error
		data   string
		setup  func(t *testing.T, chat gotgbot.Chat)
	}{
		{
			name:   "remove all edit failure",
			method: "editMessageText",
			run:    filtersModule.filtersButtonHandler,
			data:   encodeCallbackData("rmAllFilters", map[string]string{"a": "yes"}),
			setup: func(t *testing.T, chat gotgbot.Chat) {
				t.Helper()
				if err := filters.AddFilter(chat.Id, "hello", "old", "", nil, db.TEXT); err != nil {
					t.Fatalf("AddFilter setup error = %v", err)
				}
			},
		},
		{
			name:   "remove all answer failure",
			method: "answerCallbackQuery",
			run:    filtersModule.filtersButtonHandler,
			data:   encodeCallbackData("rmAllFilters", map[string]string{"a": "no"}),
			setup: func(t *testing.T, chat gotgbot.Chat) {
				t.Helper()
				if err := filters.AddFilter(chat.Id, "hello", "old", "", nil, db.TEXT); err != nil {
					t.Fatalf("AddFilter setup error = %v", err)
				}
			},
		},
		{
			name:   "overwrite edit failure",
			method: "editMessageText",
			run:    filtersModule.filterOverWriteHandler,
			data:   encodeCallbackData("filters_overwrite", map[string]string{"a": "yes", "t": "token-edit"}),
			setup: func(t *testing.T, chat gotgbot.Chat) {
				t.Helper()
				if err := filters.AddFilter(chat.Id, "hello", "old", "", nil, db.TEXT); err != nil {
					t.Fatalf("AddFilter setup error = %v", err)
				}
				if err := setFilterOverwriteCache("token-edit", overwriteFilter{
					overwriteBase: overwriteBase{
						ChatID:   chat.Id,
						ItemName: "hello",
						Text:     "new",
						DataType: db.TEXT,
					},
				}); err != nil {
					t.Fatalf("setFilterOverwriteCache setup error = %v", err)
				}
			},
		},
		{
			name:   "overwrite answer failure",
			method: "answerCallbackQuery",
			run:    filtersModule.filterOverWriteHandler,
			data:   encodeCallbackData("filters_overwrite", map[string]string{"a": "yes", "t": "token-answer"}),
			setup: func(t *testing.T, chat gotgbot.Chat) {
				t.Helper()
				if err := filters.AddFilter(chat.Id, "hello", "old", "", nil, db.TEXT); err != nil {
					t.Fatalf("AddFilter setup error = %v", err)
				}
				if err := setFilterOverwriteCache("token-answer", overwriteFilter{
					overwriteBase: overwriteBase{
						ChatID:   chat.Id,
						ItemName: "hello",
						Text:     "new",
						DataType: db.TEXT,
					},
				}); err != nil {
					t.Fatalf("setFilterOverwriteCache setup error = %v", err)
				}
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			client.errors[tt.method] = requestErr
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Filter Chat"}
			if tt.setup != nil {
				tt.setup(t, chat)
			}
			ctx := newModuleCallbackContext(bot, chat, admin, tt.data)

			err := tt.run(bot, ctx)
			if !errors.Is(err, requestErr) {
				t.Fatalf("%s error = %v, want request error", tt.name, err)
			}
		})
	}
}

func TestFilterCallbackHandlersReturnEarlyWithoutChat(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Filter Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	rmAllCtx := newModuleCallbackContext(
		bot,
		chat,
		admin,
		encodeCallbackData("rmAllFilters", map[string]string{"a": "yes"}),
	)
	rmAllCtx.EffectiveChat = nil
	if err := filtersModule.filtersButtonHandler(bot, rmAllCtx); err != ext.EndGroups {
		t.Fatalf("filtersButtonHandler(no chat) error = %v, want EndGroups", err)
	}

	overwriteCtx := newModuleCallbackContext(bot, chat, admin, encodeCallbackData("filters_overwrite", map[string]string{"a": "cancel"}))
	overwriteCtx.EffectiveChat = nil
	if err := filtersModule.filterOverWriteHandler(bot, overwriteCtx); err != ext.EndGroups {
		t.Fatalf("filterOverWriteHandler(no chat) error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 0 {
		t.Fatalf("answerCallbackQuery calls = %d, want none when chat is missing", len(calls))
	}
}
