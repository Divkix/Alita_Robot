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

func slicesContains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
