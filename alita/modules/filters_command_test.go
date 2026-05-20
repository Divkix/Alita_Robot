package modules

import (
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
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
	if !db.DoesFilterExists(chat.Id, "hello") {
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
	if db.DoesFilterExists(chat.Id, "hello") {
		t.Fatal("filter still exists after remove")
	}
}

func TestFilterOverwriteCallbackReplacesExistingFilter(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Filter Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	if err := db.AddFilter(chat.Id, "hello", "old reply", "", nil, db.TEXT); err != nil {
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
		"filters_overwrite.hello",
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

func TestRemoveAllFiltersConfirmationAndCallback(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Filter Chat"}
	owner := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	if err := db.AddFilter(chat.Id, "one", "1", "", nil, db.TEXT); err != nil {
		t.Fatalf("AddFilter setup error = %v", err)
	}
	if err := db.AddFilter(chat.Id, "two", "2", "", nil, db.TEXT); err != nil {
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

	data := encodeCallbackData("rmAllFilters", map[string]string{"a": "yes"}, "rmAllFilters.yes")
	callbackCtx := newModuleCallbackContext(bot, chat, owner, data)
	if err := filtersModule.filtersButtonHandler(bot, callbackCtx); err != ext.EndGroups {
		t.Fatalf("filtersButtonHandler error = %v, want EndGroups", err)
	}
	waitForModuleCondition(t, func() bool {
		return len(db.GetFiltersList(chat.Id)) == 0
	})
}
