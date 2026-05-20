package modules

import (
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
)

func TestAddListActionAndRemoveBlacklistCommands(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Blacklist Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	addCtx := newModuleMessageContext(bot, chat, admin, "/addblacklist spam eggs")
	if err := blacklistsModule.addBlacklist(bot, addCtx); err != ext.EndGroups {
		t.Fatalf("addBlacklist error = %v, want EndGroups", err)
	}
	waitForModuleCondition(t, func() bool {
		triggers := db.GetBlacklistSettings(chat.Id).Triggers()
		return len(triggers) == 2
	})

	listCtx := newModuleMessageContext(bot, chat, admin, "/blacklists")
	if err := blacklistsModule.listBlacklists(bot, listCtx); err != ext.EndGroups {
		t.Fatalf("listBlacklists error = %v, want EndGroups", err)
	}
	calls := client.callsFor("sendMessage")
	lastText := calls[len(calls)-1].Params["text"].(string)
	if !strings.Contains(lastText, "spam") || !strings.Contains(lastText, "eggs") {
		t.Fatalf("blacklist list text = %q, want stored words", lastText)
	}

	actionCtx := newModuleMessageContext(bot, chat, admin, "/blaction mute")
	if err := blacklistsModule.setBlacklistAction(bot, actionCtx); err != ext.EndGroups {
		t.Fatalf("setBlacklistAction error = %v, want EndGroups", err)
	}
	if got := db.GetBlacklistSettings(chat.Id).Action(); got != "mute" {
		t.Fatalf("blacklist action = %q, want mute", got)
	}

	removeCtx := newModuleMessageContext(bot, chat, admin, "/rmblacklist spam")
	if err := blacklistsModule.removeBlacklist(bot, removeCtx); err != ext.EndGroups {
		t.Fatalf("removeBlacklist error = %v, want EndGroups", err)
	}
	waitForModuleCondition(t, func() bool {
		return !slicesContains(db.GetBlacklistSettings(chat.Id).Triggers(), "spam")
	})
}

func TestBlacklistCommandsHandleValidationBranches(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Blacklist Chat"}
	admin := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	emptyListCtx := newModuleMessageContext(bot, chat, admin, "/blacklists")
	if err := blacklistsModule.listBlacklists(bot, emptyListCtx); err != ext.EndGroups {
		t.Fatalf("listBlacklists empty error = %v, want EndGroups", err)
	}

	missingAddCtx := newModuleMessageContext(bot, chat, admin, "/addblacklist")
	if err := blacklistsModule.addBlacklist(bot, missingAddCtx); err != ext.EndGroups {
		t.Fatalf("addBlacklist missing error = %v, want EndGroups", err)
	}

	tooLongWord := strings.Repeat("x", 101)
	tooLongCtx := newModuleMessageContext(bot, chat, admin, "/addblacklist "+tooLongWord)
	if err := blacklistsModule.addBlacklist(bot, tooLongCtx); err != ext.EndGroups {
		t.Fatalf("addBlacklist too-long error = %v, want EndGroups", err)
	}
	if got := len(db.GetBlacklistSettings(chat.Id).Triggers()); got != 0 {
		t.Fatalf("blacklist triggers after too-long word = %d, want none", got)
	}

	addCtx := newModuleMessageContext(bot, chat, admin, "/addblacklist dup one two three four")
	if err := blacklistsModule.addBlacklist(bot, addCtx); err != ext.EndGroups {
		t.Fatalf("addBlacklist bulk error = %v, want EndGroups", err)
	}
	waitForModuleCondition(t, func() bool {
		return len(db.GetBlacklistSettings(chat.Id).Triggers()) == 5
	})
	duplicateCtx := newModuleMessageContext(bot, chat, admin, "/addblacklist dup")
	if err := blacklistsModule.addBlacklist(bot, duplicateCtx); err != ext.EndGroups {
		t.Fatalf("addBlacklist duplicate error = %v, want EndGroups", err)
	}
	if got := len(db.GetBlacklistSettings(chat.Id).Triggers()); got != 5 {
		t.Fatalf("blacklist triggers after duplicate = %d, want unchanged 5", got)
	}

	missingRemoveCtx := newModuleMessageContext(bot, chat, admin, "/rmblacklist")
	if err := blacklistsModule.removeBlacklist(bot, missingRemoveCtx); err != ext.EndGroups {
		t.Fatalf("removeBlacklist missing error = %v, want EndGroups", err)
	}
	absentRemoveCtx := newModuleMessageContext(bot, chat, admin, "/rmblacklist absent")
	if err := blacklistsModule.removeBlacklist(bot, absentRemoveCtx); err != ext.EndGroups {
		t.Fatalf("removeBlacklist absent error = %v, want EndGroups", err)
	}

	currentActionCtx := newModuleMessageContext(bot, chat, admin, "/blaction")
	if err := blacklistsModule.setBlacklistAction(bot, currentActionCtx); err != ext.EndGroups {
		t.Fatalf("setBlacklistAction current error = %v, want EndGroups", err)
	}
	invalidActionCtx := newModuleMessageContext(bot, chat, admin, "/blaction freeze")
	if err := blacklistsModule.setBlacklistAction(bot, invalidActionCtx); err != ext.EndGroups {
		t.Fatalf("setBlacklistAction invalid error = %v, want EndGroups", err)
	}
	tooManyActionCtx := newModuleMessageContext(bot, chat, admin, "/blaction mute ban")
	if err := blacklistsModule.setBlacklistAction(bot, tooManyActionCtx); err != ext.EndGroups {
		t.Fatalf("setBlacklistAction too-many error = %v, want EndGroups", err)
	}
}

func TestBlacklistWatcherAppliesMuteAction(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Blacklist Chat"}
	member := gotgbot.User{Id: 42, FirstName: "Member"}
	if err := db.AddBlacklist(chat.Id, "spam"); err != nil {
		t.Fatalf("AddBlacklist setup error = %v", err)
	}
	if err := db.SetBlacklistAction(chat.Id, "mute"); err != nil {
		t.Fatalf("SetBlacklistAction setup error = %v", err)
	}

	ctx := newModuleMessageContext(bot, chat, member, "this has spam inside")
	if err := blacklistsModule.blacklistWatcher(bot, ctx); err != ext.ContinueGroups {
		t.Fatalf("blacklistWatcher error = %v, want ContinueGroups", err)
	}
	if calls := client.callsFor("deleteMessage"); len(calls) != 1 {
		t.Fatalf("deleteMessage calls = %d, want 1", len(calls))
	}
	if calls := client.callsFor("restrictChatMember"); len(calls) != 1 {
		t.Fatalf("restrictChatMember calls = %d, want mute action", len(calls))
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want action notice", len(calls))
	}
}

func TestBlacklistWatcherAppliesBanWarnAndNoneActions(t *testing.T) {
	tests := []struct {
		name       string
		action     string
		wantMethod string
		wantErr    error
	}{
		{name: "ban", action: "ban", wantMethod: "banChatMember", wantErr: ext.ContinueGroups},
		{name: "warn", action: "warn", wantMethod: "sendMessage", wantErr: ext.EndGroups},
		{name: "none", action: "none", wantMethod: "", wantErr: ext.ContinueGroups},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Blacklist Chat"}
			member := gotgbot.User{Id: 42, FirstName: "Member"}
			if err := db.AddBlacklist(chat.Id, "spam"); err != nil {
				t.Fatalf("AddBlacklist setup error = %v", err)
			}
			if err := db.SetBlacklistAction(chat.Id, tc.action); err != nil {
				t.Fatalf("SetBlacklistAction setup error = %v", err)
			}

			ctx := newModuleMessageContext(bot, chat, member, "this has spam inside")
			if err := blacklistsModule.blacklistWatcher(bot, ctx); err != tc.wantErr {
				t.Fatalf("blacklistWatcher error = %v, want %v", err, tc.wantErr)
			}
			if calls := client.callsFor("deleteMessage"); len(calls) != 1 {
				t.Fatalf("deleteMessage calls = %d, want message deletion", len(calls))
			}
			if tc.wantMethod != "" {
				if calls := client.callsFor(tc.wantMethod); len(calls) == 0 {
					t.Fatalf("%s calls = 0, want watcher action", tc.wantMethod)
				}
			}
		})
	}
}

func TestRemoveAllBlacklistsConfirmationAndCallback(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Blacklist Chat"}
	owner := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	if err := db.AddBlacklist(chat.Id, "one"); err != nil {
		t.Fatalf("AddBlacklist setup error = %v", err)
	}
	if err := db.AddBlacklist(chat.Id, "two"); err != nil {
		t.Fatalf("AddBlacklist setup error = %v", err)
	}

	confirmCtx := newModuleMessageContext(bot, chat, owner, "/rmallbl")
	if err := blacklistsModule.rmAllBlacklists(bot, confirmCtx); err != ext.EndGroups {
		t.Fatalf("rmAllBlacklists error = %v, want EndGroups", err)
	}
	calls := client.callsFor("sendMessage")
	if len(calls) != 1 || calls[0].Params["reply_markup"] == nil {
		t.Fatalf("rmAllBlacklists confirmation calls = %+v, want reply markup", calls)
	}

	data := encodeCallbackData("rmAllBlacklist", map[string]string{"a": "yes"}, "rmAllBlacklist.yes")
	callbackCtx := newModuleCallbackContext(bot, chat, owner, data)
	if err := blacklistsModule.buttonHandler(bot, callbackCtx); err != ext.EndGroups {
		t.Fatalf("buttonHandler error = %v, want EndGroups", err)
	}
	waitForModuleCondition(t, func() bool {
		return len(db.GetBlacklistSettings(chat.Id).Triggers()) == 0
	})
}

func TestRemoveAllBlacklistsCancelAndInvalidCallbacks(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Blacklist Chat"}
	owner := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	if err := db.AddBlacklist(chat.Id, "one"); err != nil {
		t.Fatalf("AddBlacklist setup error = %v", err)
	}

	cancelCtx := newModuleCallbackContext(bot, chat, owner, "rmAllBlacklist.no")
	if err := blacklistsModule.buttonHandler(bot, cancelCtx); err != ext.EndGroups {
		t.Fatalf("buttonHandler cancel error = %v, want EndGroups", err)
	}
	if got := len(db.GetBlacklistSettings(chat.Id).Triggers()); got != 1 {
		t.Fatalf("blacklist triggers after cancel = %d, want retained trigger", got)
	}

	invalidCtx := newModuleCallbackContext(bot, chat, owner, "rmAllBlacklist")
	if err := blacklistsModule.buttonHandler(bot, invalidCtx); err != ext.EndGroups {
		t.Fatalf("buttonHandler invalid error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 2 {
		t.Fatalf("answerCallbackQuery calls = %d, want cancel and invalid answers", len(calls))
	}
}

func TestLoadBlacklistsRegistersHelpAndHandlers(t *testing.T) {
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadBlacklists(dispatcher)

	if moduleName, enabled := DefaultHelpRegistry().AbleMap.Load(blacklistsModule.moduleName); moduleName != blacklistsModule.moduleName || !enabled {
		t.Fatalf("blacklists help registration = (%q, %v), want enabled", moduleName, enabled)
	}
}

func slicesContains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
