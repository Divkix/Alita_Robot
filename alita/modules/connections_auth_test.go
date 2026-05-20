package modules

import (
	"fmt"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

func TestCanUserConnectToChatAllowsTelegramServiceAdmins(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chatID := uniqueModuleChatID()

	allowed, denyKey := canUserConnectToChat(bot, chatID, 777000)
	if !allowed {
		t.Fatalf("canUserConnectToChat() allowed = false, denyKey = %q", denyKey)
	}
	if denyKey != "" {
		t.Fatalf("denyKey = %q, want empty for admin bypass", denyKey)
	}
}

func TestCanUserConnectToChatRespectsAllowConnectMembership(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chatID := uniqueModuleChatID()
	_ = db.GetChatConnectionSetting(chatID)
	db.ToggleAllowConnect(chatID, true)

	allowed, denyKey := canUserConnectToChat(bot, chatID, 42)
	if !allowed {
		t.Fatalf("canUserConnectToChat() allowed = false, denyKey = %q", denyKey)
	}
	if denyKey != "" {
		t.Fatalf("denyKey = %q, want empty for member with allow-connect", denyKey)
	}
}

func TestCanUserConnectToChatDeniesNonAdminWhenDisabled(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chatID := uniqueModuleChatID()
	_ = db.GetChatConnectionSetting(chatID)
	db.ToggleAllowConnect(chatID, false)

	allowed, denyKey := canUserConnectToChat(bot, chatID, 42)
	if allowed {
		t.Fatal("canUserConnectToChat() allowed = true, want false when allow-connect is disabled")
	}
	if denyKey != "connections_connect_connection_disabled" {
		t.Fatalf("denyKey = %q, want connection disabled key", denyKey)
	}
}

func TestCanUserConnectToChatDeniesWhenMemberLookupFails(t *testing.T) {
	client := newModuleBotClient()
	client.errors["getChatMember"] = fmt.Errorf("telegram unavailable")
	bot := newModuleTestBot(client)
	chatID := uniqueModuleChatID()
	_ = db.GetChatConnectionSetting(chatID)
	db.ToggleAllowConnect(chatID, true)

	allowed, denyKey := canUserConnectToChat(bot, chatID, 42)
	if allowed {
		t.Fatal("canUserConnectToChat() allowed = true, want false on member lookup failure")
	}
	if denyKey != "connections_connect_connection_disabled" {
		t.Fatalf("denyKey = %q, want connection disabled key", denyKey)
	}
}

func TestAllowConnectTogglesSetting(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chatID := uniqueModuleChatID()
	chat := gotgbot.Chat{Id: chatID, Type: "supergroup", Title: "Connections Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}

	onCtx := newModuleMessageContext(bot, chat, user, "/allowconnect on")
	if err := ConnectionsModule.allowConnect(bot, onCtx); err != ext.EndGroups {
		t.Fatalf("allowConnect on error = %v, want EndGroups", err)
	}
	if !db.GetChatConnectionSetting(chatID).AllowConnect {
		t.Fatal("AllowConnect was not enabled")
	}

	currentCtx := newModuleMessageContext(bot, chat, user, "/allowconnect")
	if err := ConnectionsModule.allowConnect(bot, currentCtx); err != ext.EndGroups {
		t.Fatalf("allowConnect current error = %v, want EndGroups", err)
	}

	offCtx := newModuleMessageContext(bot, chat, user, "/allowconnect no")
	if err := ConnectionsModule.allowConnect(bot, offCtx); err != ext.EndGroups {
		t.Fatalf("allowConnect off error = %v, want EndGroups", err)
	}
	if db.GetChatConnectionSetting(chatID).AllowConnect {
		t.Fatal("AllowConnect stayed enabled after no")
	}
}

func TestDisconnectPrivateClearsConnection(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: 4242, FirstName: "Member"}
	chatID := uniqueModuleChatID()
	db.ConnectId(user.Id, chatID)

	privateChat := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Member"}
	ctx := newModuleMessageContext(bot, privateChat, user, "/disconnect")
	if err := ConnectionsModule.disconnect(bot, ctx); err != ext.EndGroups {
		t.Fatalf("disconnect() error = %v, want EndGroups", err)
	}
	if db.Connection(user.Id).Connected {
		t.Fatal("user remained connected after /disconnect")
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
}

func TestDisconnectInGroupDoesNotClearConnection(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: 4243, FirstName: "Member"}
	connectedChatID := uniqueModuleChatID()
	db.ConnectId(user.Id, connectedChatID)

	groupChat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Connections Chat"}
	ctx := newModuleMessageContext(bot, groupChat, user, "/disconnect")
	if err := ConnectionsModule.disconnect(bot, ctx); err != ext.EndGroups {
		t.Fatalf("disconnect() error = %v, want EndGroups", err)
	}
	if !db.Connection(user.Id).Connected {
		t.Fatal("group /disconnect cleared private connection")
	}
}

func TestConnectionReportsConnectedChat(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: 4244, FirstName: "Member"}
	chatID := uniqueModuleChatID()
	db.ConnectId(user.Id, chatID)

	privateChat := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Member"}
	ctx := newModuleMessageContext(bot, privateChat, user, "/connection")
	if err := ConnectionsModule.connection(bot, ctx); err != ext.EndGroups {
		t.Fatalf("connection() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("getChat"); len(calls) == 0 {
		t.Fatal("connection() did not fetch connected chat")
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
}

func TestConnectionCommandStringsIncludeRegisteredCommands(t *testing.T) {
	originalAdminCmds := helpers.AdminCmds
	originalUserCmds := helpers.UserCmds
	helpers.AdminCmds = []string{"ban", "mute"}
	helpers.UserCmds = []string{"rules", "notes"}
	t.Cleanup(func() {
		helpers.AdminCmds = originalAdminCmds
		helpers.UserCmds = originalUserCmds
	})

	adminCommands := ConnectionsModule.adminCmdConnString()
	userCommands := ConnectionsModule.userCmdConnString()
	if !strings.Contains(adminCommands, "/ban") {
		t.Fatalf("adminCmdConnString() = %q, want admin commands", adminCommands)
	}
	if !strings.Contains(userCommands, "/rules") {
		t.Fatalf("userCmdConnString() = %q, want user commands", userCommands)
	}
}
