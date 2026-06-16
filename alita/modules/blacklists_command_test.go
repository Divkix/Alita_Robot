package modules

import (
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/divkix/Alita_Robot/alita/db/approvals"
	"github.com/divkix/Alita_Robot/alita/db/blacklists"
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
		triggers := blacklists.GetBlacklistSettings(chat.Id).Triggers()
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
	if got := blacklists.GetBlacklistSettings(chat.Id).Action(); got != "mute" {
		t.Fatalf("blacklist action = %q, want mute", got)
	}

	removeCtx := newModuleMessageContext(bot, chat, admin, "/rmblacklist spam")
	if err := blacklistsModule.removeBlacklist(bot, removeCtx); err != ext.EndGroups {
		t.Fatalf("removeBlacklist error = %v, want EndGroups", err)
	}
	waitForModuleCondition(t, func() bool {
		return !slicesContains(blacklists.GetBlacklistSettings(chat.Id).Triggers(), "spam")
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
	if got := len(blacklists.GetBlacklistSettings(chat.Id).Triggers()); got != 0 {
		t.Fatalf("blacklist triggers after too-long word = %d, want none", got)
	}

	addCtx := newModuleMessageContext(bot, chat, admin, "/addblacklist dup one two three four")
	if err := blacklistsModule.addBlacklist(bot, addCtx); err != ext.EndGroups {
		t.Fatalf("addBlacklist bulk error = %v, want EndGroups", err)
	}
	waitForModuleCondition(t, func() bool {
		return len(blacklists.GetBlacklistSettings(chat.Id).Triggers()) == 5
	})
	duplicateCtx := newModuleMessageContext(bot, chat, admin, "/addblacklist dup")
	if err := blacklistsModule.addBlacklist(bot, duplicateCtx); err != ext.EndGroups {
		t.Fatalf("addBlacklist duplicate error = %v, want EndGroups", err)
	}
	if got := len(blacklists.GetBlacklistSettings(chat.Id).Triggers()); got != 5 {
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
	if err := blacklists.AddBlacklist(chat.Id, "spam"); err != nil {
		t.Fatalf("AddBlacklist setup error = %v", err)
	}
	if err := blacklists.SetBlacklistAction(chat.Id, "mute"); err != nil {
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
			if err := blacklists.AddBlacklist(chat.Id, "spam"); err != nil {
				t.Fatalf("AddBlacklist setup error = %v", err)
			}
			if err := blacklists.SetBlacklistAction(chat.Id, tc.action); err != nil {
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

func TestBlacklistWatcherAppliesKickAndAnonymousChannelBan(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Blacklist Chat"}
	member := gotgbot.User{Id: 42, FirstName: "Member"}
	if err := blacklists.AddBlacklist(chat.Id, "spam"); err != nil {
		t.Fatalf("AddBlacklist setup error = %v", err)
	}
	if err := blacklists.SetBlacklistAction(chat.Id, "kick"); err != nil {
		t.Fatalf("SetBlacklistAction(kick) setup error = %v", err)
	}

	kickCtx := newModuleMessageContext(bot, chat, member, "this has spam inside")
	if err := blacklistsModule.blacklistWatcher(bot, kickCtx); err != ext.ContinueGroups {
		t.Fatalf("blacklistWatcher(kick) error = %v, want ContinueGroups", err)
	}
	if calls := client.callsFor("banChatMember"); len(calls) != 1 {
		t.Fatalf("banChatMember calls = %d, want kick action", len(calls))
	}

	if err := blacklists.SetBlacklistAction(chat.Id, "ban"); err != nil {
		t.Fatalf("SetBlacklistAction(ban) setup error = %v", err)
	}
	channel := gotgbot.Chat{Id: -1001234567890, Type: "channel", Title: "Spam Channel"}
	channelCtx := newModuleMessageContext(bot, chat, member, "channel says spam")
	channelCtx.EffectiveMessage.From = nil
	channelCtx.EffectiveMessage.SenderChat = &channel
	channelCtx.EffectiveSender = &gotgbot.Sender{Chat: &channel, ChatId: chat.Id}
	if err := blacklistsModule.blacklistWatcher(bot, channelCtx); err != ext.ContinueGroups {
		t.Fatalf("blacklistWatcher(channel ban) error = %v, want ContinueGroups", err)
	}
	if calls := client.callsFor("banChatSenderChat"); len(calls) != 1 {
		t.Fatalf("banChatSenderChat calls = %d, want anonymous channel ban", len(calls))
	}
}

func TestBlacklistWatcherSkipsSenderAndContentNoopBranches(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Blacklist Chat"}
	member := gotgbot.User{Id: 42, FirstName: "Member"}
	if err := blacklists.AddBlacklist(chat.Id, "spam"); err != nil {
		t.Fatalf("AddBlacklist setup error = %v", err)
	}
	if err := approvals.AddApprovedUser(chat.Id, member.Id, 777000, "trusted"); err != nil {
		t.Fatalf("AddApprovedUser setup error = %v", err)
	}

	nilSenderCtx := newModuleMessageContext(bot, chat, member, "spam")
	nilSenderCtx.EffectiveSender = nil
	if err := blacklistsModule.blacklistWatcher(bot, nilSenderCtx); err != ext.ContinueGroups {
		t.Fatalf("blacklistWatcher(nil sender) error = %v, want ContinueGroups", err)
	}

	anonAdminChat := gotgbot.Chat{Id: chat.Id, Type: "supergroup", Title: "Anon Admin"}
	anonAdminCtx := newModuleMessageContext(bot, chat, member, "spam")
	anonAdminCtx.EffectiveSender = &gotgbot.Sender{Chat: &anonAdminChat, ChatId: chat.Id}
	if err := blacklistsModule.blacklistWatcher(bot, anonAdminCtx); err != ext.ContinueGroups {
		t.Fatalf("blacklistWatcher(anonymous admin) error = %v, want ContinueGroups", err)
	}

	adminCtx := newModuleMessageContext(bot, chat, gotgbot.User{Id: 777000, FirstName: "Telegram"}, "spam")
	if err := blacklistsModule.blacklistWatcher(bot, adminCtx); err != ext.ContinueGroups {
		t.Fatalf("blacklistWatcher(admin) error = %v, want ContinueGroups", err)
	}

	approvedCtx := newModuleMessageContext(bot, chat, member, "spam")
	if err := blacklistsModule.blacklistWatcher(bot, approvedCtx); err != ext.ContinueGroups {
		t.Fatalf("blacklistWatcher(approved) error = %v, want ContinueGroups", err)
	}

	emptyTextCtx := newModuleMessageContext(bot, chat, gotgbot.User{Id: 43, FirstName: "Other"}, "")
	if err := blacklistsModule.blacklistWatcher(bot, emptyTextCtx); err != ext.ContinueGroups {
		t.Fatalf("blacklistWatcher(empty text) error = %v, want ContinueGroups", err)
	}

	noMatchCtx := newModuleMessageContext(bot, chat, gotgbot.User{Id: 44, FirstName: "Clean"}, "clean message")
	if err := blacklistsModule.blacklistWatcher(bot, noMatchCtx); err != ext.ContinueGroups {
		t.Fatalf("blacklistWatcher(no match) error = %v, want ContinueGroups", err)
	}

	if calls := client.callsFor("deleteMessage"); len(calls) != 0 {
		t.Fatalf("deleteMessage calls = %d, want no moderation side effects", len(calls))
	}
	if calls := client.callsFor("banChatMember"); len(calls) != 0 {
		t.Fatalf("banChatMember calls = %d, want no moderation side effects", len(calls))
	}
}

func TestRemoveAllBlacklistsConfirmationAndCallback(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Blacklist Chat"}
	owner := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	if err := blacklists.AddBlacklist(chat.Id, "one"); err != nil {
		t.Fatalf("AddBlacklist setup error = %v", err)
	}
	if err := blacklists.AddBlacklist(chat.Id, "two"); err != nil {
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
		return len(blacklists.GetBlacklistSettings(chat.Id).Triggers()) == 0
	})
}

func TestRemoveAllBlacklistsCancelAndInvalidCallbacks(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Blacklist Chat"}
	owner := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	if err := blacklists.AddBlacklist(chat.Id, "one"); err != nil {
		t.Fatalf("AddBlacklist setup error = %v", err)
	}

	cancelCtx := newModuleCallbackContext(bot, chat, owner, "rmAllBlacklist.no")
	if err := blacklistsModule.buttonHandler(bot, cancelCtx); err != ext.EndGroups {
		t.Fatalf("buttonHandler cancel error = %v, want EndGroups", err)
	}
	if got := len(blacklists.GetBlacklistSettings(chat.Id).Triggers()); got != 1 {
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

// TestBlacklistWatcherSkipsBotAdminCallWhenNoTriggers asserts that when a chat
// has no blacklist triggers the watcher returns before issuing any getChatMember
// API call — verifying the reorder from Step 1 of plan 007.
func TestBlacklistWatcherSkipsBotAdminCallWhenNoTriggers(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	// Use a fresh chat ID with no blacklist triggers configured.
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "No Triggers Chat"}
	member := gotgbot.User{Id: 42, FirstName: "Member"}

	ctx := newModuleMessageContext(bot, chat, member, "some random message")
	if err := blacklistsModule.blacklistWatcher(bot, ctx); err != ext.ContinueGroups {
		t.Fatalf("blacklistWatcher(no triggers) error = %v, want ContinueGroups", err)
	}

	// getChatMember must NOT have been called — the fast-path returns before any
	// admin-status lookup when no triggers are configured.
	if calls := client.callsFor("getChatMember"); len(calls) != 0 {
		t.Fatalf("getChatMember calls = %d, want 0 when no triggers are set", len(calls))
	}
}
