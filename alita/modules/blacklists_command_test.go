package modules

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/divkix/Alita_Robot/alita/db/approvals"
	"github.com/divkix/Alita_Robot/alita/db/blacklists"
	logrustest "github.com/sirupsen/logrus/hooks/test"
)

func TestAddListActionAndRemoveBlacklistCommands(t *testing.T) {
	f := newAdminChatFixture(t)
	client, bot, chat, admin := f.client, f.bot, f.chat, f.admin

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
	f := newAdminChatFixture(t)
	bot, chat, admin := f.bot, f.chat, f.admin

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

	unicodeWord := strings.Repeat("界", 100)
	unicodeCtx := newModuleMessageContext(bot, chat, admin, "/addblacklist "+unicodeWord)
	if err := blacklistsModule.addBlacklist(bot, unicodeCtx); err != ext.EndGroups {
		t.Fatalf("addBlacklist 100-rune word error = %v, want EndGroups", err)
	}
	if got := blacklists.GetBlacklistSettings(chat.Id).Triggers(); len(got) != 1 || got[0] != unicodeWord {
		t.Fatalf("blacklist triggers after 100-rune word = %q, want accepted word", got)
	}
	if err := blacklists.RemoveAllBlacklist(chat.Id); err != nil {
		t.Fatalf("RemoveAllBlacklist cleanup error = %v", err)
	}

	addCtx := newModuleMessageContext(bot, chat, admin, "/addblacklist dup one two three four dup")
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
	assertParams(t, client.onlyCall(t, "deleteMessage"), map[string]any{"chat_id": chat.Id, "message_id": ctx.EffectiveMessage.MessageId})
	assertMuted(t, client.onlyCall(t, "restrictChatMember"), chat.Id, member.Id)
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want action notice", len(calls))
	}
}

func TestBlacklistWarnDoesNotLogNormalDispatcherStopAsError(t *testing.T) {
	hook := logrustest.NewGlobal()
	t.Cleanup(hook.Reset)
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Blacklist Chat"}
	member := gotgbot.User{Id: 42, FirstName: "Member"}
	if err := blacklists.AddBlacklist(chat.Id, "spam"); err != nil {
		t.Fatalf("AddBlacklist setup error = %v", err)
	}
	if err := blacklists.SetBlacklistAction(chat.Id, "warn"); err != nil {
		t.Fatalf("SetBlacklistAction setup error = %v", err)
	}

	ctx := newModuleMessageContext(bot, chat, member, "this has spam inside")
	if err := blacklistsModule.blacklistWatcher(bot, ctx); err != ext.EndGroups {
		t.Fatalf("blacklistWatcher error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) == 0 {
		t.Fatal("sendMessage calls = 0, want warn notice")
	}
	for _, entry := range hook.Entries {
		if entry.Message == ext.EndGroups.Error() {
			t.Fatal("normal blacklist warn completion logged as an error")
		}
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
			assertParams(t, client.onlyCall(t, "deleteMessage"), map[string]any{"chat_id": chat.Id, "message_id": ctx.EffectiveMessage.MessageId})
			if tc.wantMethod != "" {
				if calls := client.callsFor(tc.wantMethod); len(calls) == 0 {
					t.Fatalf("%s calls = 0, want watcher action", tc.wantMethod)
				}
			}
			if tc.action == "ban" {
				assertParams(t, client.onlyCall(t, "banChatMember"), targetParams(chat.Id, member.Id))
			} else if calls := client.callsFor("banChatMember"); len(calls) != 0 {
				t.Fatalf("banChatMember calls = %d, want none for %s", len(calls), tc.action)
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
	assertParams(t, client.onlyCall(t, "unbanChatMember"), targetParams(chat.Id, member.Id))

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
	assertParams(t, client.onlyCall(t, "banChatSenderChat"), map[string]any{"chat_id": chat.Id, "sender_chat_id": channel.Id})
}

func TestBlacklistWatcherSkipsSenderAndContentNoopBranches(t *testing.T) {
	f := newAdminChatFixture(t)
	client, bot, chat, member := f.client, f.bot, f.chat, f.member
	if err := blacklists.AddBlacklist(chat.Id, "spam"); err != nil {
		t.Fatalf("AddBlacklist setup error = %v", err)
	}
	if err := approvals.AddApprovedUser(chat.Id, member.Id, f.admin.Id, "trusted"); err != nil {
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

	adminCtx := newModuleMessageContext(bot, chat, f.admin, "spam")
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
	f := newAdminChatFixture(t)
	f.makeOwner()
	client, bot, chat, owner := f.client, f.bot, f.chat, f.admin
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

	data := encodeCallbackData("rmAllBlacklist", map[string]string{"a": "yes"})
	callbackCtx := newModuleCallbackContext(bot, chat, owner, data)
	if err := blacklistsModule.buttonHandler(bot, callbackCtx); err != ext.EndGroups {
		t.Fatalf("buttonHandler error = %v, want EndGroups", err)
	}
	waitForModuleCondition(t, func() bool {
		return len(blacklists.GetBlacklistSettings(chat.Id).Triggers()) == 0
	})
}

func TestRemoveAllBlacklistsRefusesNonOwnerAdmin(t *testing.T) {
	f := newAdminChatFixture(t)
	bot, chat, admin := f.bot, f.chat, f.admin
	if err := blacklists.AddBlacklist(chat.Id, "one"); err != nil {
		t.Fatalf("AddBlacklist setup error = %v", err)
	}

	callbackCtx := newModuleCallbackContext(bot, chat, admin, encodeCallbackData("rmAllBlacklist", map[string]string{"a": "yes"}))
	if err := blacklistsModule.buttonHandler(bot, callbackCtx); err != ext.EndGroups {
		t.Fatalf("buttonHandler(admin) error = %v, want EndGroups", err)
	}
	if got := len(blacklists.GetBlacklistSettings(chat.Id).Triggers()); got != 1 {
		t.Fatalf("blacklist triggers after a non-owner confirmed = %d, want 1", got)
	}
}

func TestBlacklistCommandsRefuseMembers(t *testing.T) {
	f := newAdminChatFixture(t)
	bot, chat, member := f.bot, f.chat, f.member
	if err := blacklists.AddBlacklist(chat.Id, "keep"); err != nil {
		t.Fatalf("AddBlacklist setup error = %v", err)
	}

	if err := blacklistsModule.addBlacklist(bot, newModuleMessageContext(bot, chat, member, "/addblacklist spam")); err != ext.EndGroups {
		t.Fatalf("addBlacklist(member) error = %v, want EndGroups", err)
	}
	if err := blacklistsModule.removeBlacklist(bot, newModuleMessageContext(bot, chat, member, "/rmblacklist keep")); err != ext.EndGroups {
		t.Fatalf("removeBlacklist(member) error = %v, want EndGroups", err)
	}
	if got := blacklists.GetBlacklistSettings(chat.Id).Triggers(); len(got) != 1 || got[0] != "keep" {
		t.Fatalf("blacklist triggers after member commands = %q, want only keep", got)
	}
}

func TestRemoveAllBlacklistsCancelAndInvalidCallbacks(t *testing.T) {
	f := newAdminChatFixture(t)
	f.makeOwner()
	client, bot, chat, owner := f.client, f.bot, f.chat, f.admin
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
	unknownCtx := newModuleCallbackContext(
		bot,
		chat,
		owner,
		encodeCallbackData("rmAllBlacklist", map[string]string{"a": "crafted"}),
	)
	if err := blacklistsModule.buttonHandler(bot, unknownCtx); err != ext.EndGroups {
		t.Fatalf("buttonHandler unknown action error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 3 {
		t.Fatalf("answerCallbackQuery calls = %d, want cancel and invalid answers", len(calls))
	}
}

func TestLoadBlacklistsRegistersHelpAndHandlers(t *testing.T) {
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadBlacklists(dispatcher)

	if !DefaultHelpRegistry().AbleMap[blacklistsModule.moduleName] {
		t.Fatal("blacklists help registration = false, want enabled")
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

func TestBlacklistWatcherSkipsBotAdminCallWhenNoTriggers(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "No Triggers Chat"}
	member := gotgbot.User{Id: 42, FirstName: "Member"}

	ctx := newModuleMessageContext(bot, chat, member, "some random message")
	if err := blacklistsModule.blacklistWatcher(bot, ctx); err != ext.ContinueGroups {
		t.Fatalf("blacklistWatcher(no triggers) error = %v, want ContinueGroups", err)
	}

	if calls := client.callsFor("getChatMember"); len(calls) != 0 {
		t.Fatalf("getChatMember calls = %d, want 0 when no triggers are set", len(calls))
	}
}

func TestBlacklistWatcherUsesMatchedTriggerAction(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Blacklist Chat"}
	if err := blacklists.AddBlacklist(chat.Id, "oldword"); err != nil {
		t.Fatalf("AddBlacklist(oldword) error = %v", err)
	}
	if err := blacklists.SetBlacklistAction(chat.Id, "mute"); err != nil {
		t.Fatalf("SetBlacklistAction error = %v", err)
	}
	if err := blacklists.AddBlacklist(chat.Id, "newword"); err != nil {
		t.Fatalf("AddBlacklist(newword) error = %v", err)
	}
	settings := blacklists.GetBlacklistSettings(chat.Id)
	if got := settings.Find("oldword"); got == nil || got.Action != "mute" {
		t.Fatalf("oldword settings = %+v, want mute", got)
	}
	if got := settings.Find("newword"); got == nil || got.Action != "warn" {
		t.Fatalf("newword settings = %+v, want warn", got)
	}

	warnMember := gotgbot.User{Id: 42, FirstName: "Member"}
	warnCtx := newModuleMessageContext(bot, chat, warnMember, "please newword here")
	if err := blacklistsModule.blacklistWatcher(bot, warnCtx); err != ext.EndGroups {
		t.Fatalf("blacklistWatcher(newword) error = %v, want EndGroups from warn", err)
	}
	if calls := client.callsFor("restrictChatMember"); len(calls) != 0 {
		t.Fatalf("restrictChatMember calls = %d after newword, want 0 (warn)", len(calls))
	}
	if calls := client.callsFor("sendMessage"); len(calls) == 0 {
		t.Fatal("newword match did not send a warn notice")
	}

	muteMember := gotgbot.User{Id: 43, FirstName: "Other"}
	muteCtx := newModuleMessageContext(bot, chat, muteMember, "please oldword here")
	if err := blacklistsModule.blacklistWatcher(bot, muteCtx); err != ext.ContinueGroups {
		t.Fatalf("blacklistWatcher(oldword) error = %v, want ContinueGroups", err)
	}
	if calls := client.callsFor("restrictChatMember"); len(calls) != 1 {
		t.Fatalf("restrictChatMember calls = %d after oldword, want mute action", len(calls))
	}
}

func TestBlacklistWatcherSkipsPermissionLookupsWhenNothingMatches(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Blacklist Chat"}
	member := gotgbot.User{Id: 42, FirstName: "Member"}
	if err := blacklists.AddBlacklist(chat.Id, "spam"); err != nil {
		t.Fatalf("AddBlacklist setup error = %v", err)
	}

	ctx := newModuleMessageContext(bot, chat, member, "hello there")
	if err := blacklistsModule.blacklistWatcher(bot, ctx); err != ext.ContinueGroups {
		t.Fatalf("blacklistWatcher(no match) error = %v, want ContinueGroups", err)
	}

	for _, method := range []string{"getChat", "getChatAdministrators", "getChatMember"} {
		if calls := client.callsFor(method); len(calls) != 0 {
			t.Fatalf("%s calls = %d, want 0 when no blacklist word matches", method, len(calls))
		}
	}
	if calls := client.callsFor("deleteMessage"); len(calls) != 0 {
		t.Fatalf("deleteMessage calls = %d, want 0 when no blacklist word matches", len(calls))
	}
}

func TestBlacklistWatcherDoesNothingWhenBotIsNotAdmin(t *testing.T) {
	client := newModuleBotClient()
	client.responses["getChatAdministrators"] = json.RawMessage(`[]`)
	client.errors["getChatMember"] = errors.New("forbidden")
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
		t.Fatalf("blacklistWatcher(bot not admin) error = %v, want ContinueGroups", err)
	}

	if calls := client.callsFor("deleteMessage"); len(calls) != 0 {
		t.Fatalf("deleteMessage calls = %d, want 0 when bot is not admin", len(calls))
	}
	if calls := client.callsFor("restrictChatMember"); len(calls) != 0 {
		t.Fatalf("restrictChatMember calls = %d, want 0 when bot is not admin", len(calls))
	}
}

// /blaction cannot store tban/tmute, but the action CHECK allows them, so rows
// restored from a backup or older versions carry them.
func TestBlacklistWatcherAppliesTimedActionsForAnHour(t *testing.T) {
	for _, tc := range []struct {
		action string
		method string
	}{
		{action: "tban", method: "banChatMember"},
		{action: "tmute", method: "restrictChatMember"},
	} {
		t.Run(tc.action, func(t *testing.T) {
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
			before := time.Now().Unix()
			if err := blacklistsModule.blacklistWatcher(bot, ctx); err != ext.ContinueGroups {
				t.Fatalf("blacklistWatcher error = %v, want ContinueGroups", err)
			}
			after := time.Now().Unix()

			if calls := client.callsFor("deleteMessage"); len(calls) != 1 {
				t.Fatalf("deleteMessage calls = %d, want message deletion", len(calls))
			}
			calls := client.callsFor(tc.method)
			if len(calls) != 1 {
				t.Fatalf("%s calls = %d, want timed action", tc.method, len(calls))
			}
			if until := untilDateParam(t, calls[0]); until < before+3600 || until > after+3600 {
				t.Fatalf("until_date = %d, want now+3600 (%d..%d)", until, before+3600, after+3600)
			}
			notices := client.callsFor("sendMessage")
			if len(notices) != 1 || !strings.Contains(fmt.Sprint(notices[0].Params["text"]), "1h") {
				t.Fatalf("blacklist notices = %#v, want duration 1h", notices)
			}
		})
	}
}

// banChatSenderChat has no until_date, so a timed ban would become permanent:
// the watcher only deletes the channel's message.
func TestBlacklistWatcherDoesNotBanChannelForTimedAction(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Blacklist Chat"}
	member := gotgbot.User{Id: 42, FirstName: "Member"}
	if err := blacklists.AddBlacklist(chat.Id, "spam"); err != nil {
		t.Fatalf("AddBlacklist setup error = %v", err)
	}
	if err := blacklists.SetBlacklistAction(chat.Id, "tban"); err != nil {
		t.Fatalf("SetBlacklistAction setup error = %v", err)
	}

	channel := gotgbot.Chat{Id: -1001234567891, Type: "channel", Title: "Spam Channel"}
	ctx := newModuleMessageContext(bot, chat, member, "channel says spam")
	ctx.EffectiveMessage.From = nil
	ctx.EffectiveMessage.SenderChat = &channel
	ctx.EffectiveSender = &gotgbot.Sender{Chat: &channel, ChatId: chat.Id}
	if err := blacklistsModule.blacklistWatcher(bot, ctx); err != ext.ContinueGroups {
		t.Fatalf("blacklistWatcher error = %v, want ContinueGroups", err)
	}
	if calls := client.callsFor("deleteMessage"); len(calls) != 1 {
		t.Fatalf("deleteMessage calls = %d, want message deletion", len(calls))
	}
	for _, method := range []string{"banChatSenderChat", "banChatMember", "sendMessage"} {
		if calls := client.callsFor(method); len(calls) != 0 {
			t.Fatalf("%s calls = %d, want none for a channel", method, len(calls))
		}
	}
}
