package modules

import (
	"fmt"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db/connections"
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
	_ = connections.GetChatConnectionSetting(chatID)
	connections.ToggleAllowConnect(chatID, true)

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
	_ = connections.GetChatConnectionSetting(chatID)
	connections.ToggleAllowConnect(chatID, false)

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
	_ = connections.GetChatConnectionSetting(chatID)
	connections.ToggleAllowConnect(chatID, true)

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
	if !connections.GetChatConnectionSetting(chatID).AllowConnect {
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
	if connections.GetChatConnectionSetting(chatID).AllowConnect {
		t.Fatal("AllowConnect stayed enabled after no")
	}
}

func TestAllowConnectHandlesInvalidOptionAndNonAdminNoop(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chatID := uniqueModuleChatID()
	chat := gotgbot.Chat{Id: chatID, Type: "supergroup", Title: "Connections Chat"}

	invalidCtx := newModuleMessageContext(bot, chat, gotgbot.User{Id: 777000, FirstName: "Telegram"}, "/allowconnect maybe")
	if err := ConnectionsModule.allowConnect(bot, invalidCtx); err != ext.EndGroups {
		t.Fatalf("allowConnect(invalid) error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want invalid-option reply", len(calls))
	}

	memberCtx := newModuleMessageContext(bot, chat, gotgbot.User{Id: 42, FirstName: "Member"}, "/allowconnect on")
	connections.ToggleAllowConnect(chatID, false)
	if err := ConnectionsModule.allowConnect(bot, memberCtx); err != ext.EndGroups {
		t.Fatalf("allowConnect(non-admin) error = %v, want EndGroups", err)
	}
	if connections.GetChatConnectionSetting(chatID).AllowConnect {
		t.Fatal("non-admin /allowconnect changed the chat setting")
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want no reply for non-admin noop", len(calls))
	}
}

func TestDisconnectPrivateClearsConnection(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: 4242, FirstName: "Member"}
	chatID := uniqueModuleChatID()
	connections.ConnectId(user.Id, chatID)

	privateChat := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Member"}
	ctx := newModuleMessageContext(bot, privateChat, user, "/disconnect")
	if err := ConnectionsModule.disconnect(bot, ctx); err != ext.EndGroups {
		t.Fatalf("disconnect() error = %v, want EndGroups", err)
	}
	if connections.Connection(user.Id).Connected {
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
	connections.ConnectId(user.Id, connectedChatID)

	groupChat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Connections Chat"}
	ctx := newModuleMessageContext(bot, groupChat, user, "/disconnect")
	if err := ConnectionsModule.disconnect(bot, ctx); err != ext.EndGroups {
		t.Fatalf("disconnect() error = %v, want EndGroups", err)
	}
	if !connections.Connection(user.Id).Connected {
		t.Fatal("group /disconnect cleared private connection")
	}
}

func TestConnectionReportsConnectedChat(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: 4244, FirstName: "Member"}
	chatID := uniqueModuleChatID()
	connections.ConnectId(user.Id, chatID)

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

func TestConnectionReportsNotConnectedAndLookupErrors(t *testing.T) {
	user := gotgbot.User{Id: 42440, FirstName: "Member"}
	privateChat := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Member"}

	noConnClient := newModuleBotClient()
	noConnBot := newModuleTestBot(noConnClient)
	noConnCtx := newModuleMessageContext(noConnBot, privateChat, user, "/connection")
	if err := ConnectionsModule.connection(noConnBot, noConnCtx); err != ext.EndGroups {
		t.Fatalf("connection(not connected) error = %v, want EndGroups", err)
	}
	if calls := noConnClient.callsFor("getChat"); len(calls) != 0 {
		t.Fatalf("getChat calls = %d, want none without connection", len(calls))
	}
	if calls := noConnClient.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want not-connected reply", len(calls))
	}

	lookupClient := newModuleBotClient()
	lookupClient.errors["getChat"] = fmt.Errorf("telegram unavailable")
	lookupBot := newModuleTestBot(lookupClient)
	connections.ConnectId(user.Id, uniqueModuleChatID())
	lookupCtx := newModuleMessageContext(lookupBot, privateChat, user, "/connection")
	if err := ConnectionsModule.connection(lookupBot, lookupCtx); err == nil {
		t.Fatal("connection(lookup error) error = nil, want request error")
	}
	if connections.Connection(user.Id).Connected {
		t.Fatal("failed connected-chat lookup did not clear stale connection")
	}
}

func TestConnectInGroupRepliesWithDeepLinkButton(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Connections Chat"}
	user := gotgbot.User{Id: 777000, FirstName: "Telegram"}
	ctx := newModuleMessageContext(bot, chat, user, "/connect")

	if err := ConnectionsModule.connect(bot, ctx); err != ext.EndGroups {
		t.Fatalf("connect() error = %v, want EndGroups", err)
	}
	if connections.Connection(user.Id).Connected {
		t.Fatal("group /connect should not create a direct DB connection")
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
}

func TestConnectPrivateEstablishesConnectionAndHandlesMissingChat(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: 42441, FirstName: "Member"}
	privateChat := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Member"}

	missingCtx := newModuleMessageContext(bot, privateChat, user, "/connect")
	if err := ConnectionsModule.connect(bot, missingCtx); err != ext.EndGroups {
		t.Fatalf("connect(missing chat) error = %v, want EndGroups", err)
	}
	if connections.Connection(user.Id).Connected {
		t.Fatal("missing chat id connected the user")
	}

	connections.ToggleAllowConnect(-1001, true)
	connectCtx := newModuleMessageContext(bot, privateChat, user, "/connect -1001")
	if err := ConnectionsModule.connect(bot, connectCtx); err != ext.EndGroups {
		t.Fatalf("connect(private) error = %v, want EndGroups", err)
	}
	if conn := connections.Connection(user.Id); !conn.Connected || conn.ChatId != -1001 {
		t.Fatalf("connection = %+v, want connected to -1001", conn)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 2 {
		t.Fatalf("sendMessage calls = %d, want missing-chat and success replies", len(calls))
	}
}

func TestReconnectPrivateRestoresPreviousChat(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: 4245, FirstName: "Member"}
	chatID := uniqueModuleChatID()
	connections.ConnectId(user.Id, chatID)
	connections.DisconnectId(user.Id)

	privateChat := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Member"}
	ctx := newModuleMessageContext(bot, privateChat, user, "/reconnect")
	if err := ConnectionsModule.reconnect(bot, ctx); err != ext.EndGroups {
		t.Fatalf("reconnect() error = %v, want EndGroups", err)
	}
	if !connections.Connection(user.Id).Connected {
		t.Fatal("user was not reconnected to previous chat")
	}
	if calls := client.callsFor("getChat"); len(calls) != 1 {
		t.Fatalf("getChat calls = %d, want 1", len(calls))
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
}

func TestReconnectPrivateHandlesNoLastChatAndLookupFailure(t *testing.T) {
	user := gotgbot.User{Id: 42451, FirstName: "Member"}
	privateChat := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Member"}

	noLastClient := newModuleBotClient()
	noLastBot := newModuleTestBot(noLastClient)
	noLastCtx := newModuleMessageContext(noLastBot, privateChat, user, "/reconnect")
	if err := ConnectionsModule.reconnect(noLastBot, noLastCtx); err != ext.EndGroups {
		t.Fatalf("reconnect(no last) error = %v, want EndGroups", err)
	}
	if calls := noLastClient.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want no-last-chat reply", len(calls))
	}

	lookupClient := newModuleBotClient()
	lookupClient.errors["getChat"] = fmt.Errorf("telegram unavailable")
	lookupBot := newModuleTestBot(lookupClient)
	connections.ConnectId(user.Id, uniqueModuleChatID())
	connections.DisconnectId(user.Id)
	lookupCtx := newModuleMessageContext(lookupBot, privateChat, user, "/reconnect")
	if err := ConnectionsModule.reconnect(lookupBot, lookupCtx); err == nil {
		t.Fatal("reconnect(lookup error) error = nil, want request error")
	}
}

func TestReconnectInGroupPromptsForPrivateChat(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Connections Chat"}
	user := gotgbot.User{Id: 4246, FirstName: "Member"}
	ctx := newModuleMessageContext(bot, chat, user, "/reconnect")

	if err := ConnectionsModule.reconnect(bot, ctx); err != ext.EndGroups {
		t.Fatalf("reconnect() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
}

func TestConnectionButtonsRenderAdminCommandsAndAnswerCallback(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: 4247, FirstName: "Member"}
	chatID := uniqueModuleChatID()
	connections.ConnectId(user.Id, chatID)

	privateChat := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Member"}
	ctx := newModuleCallbackContext(bot, privateChat, user, "connbtns.Admin")
	if err := ConnectionsModule.connectionButtons(bot, ctx); err != ext.EndGroups {
		t.Fatalf("connectionButtons() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("editMessageText"); len(calls) != 1 {
		t.Fatalf("editMessageText calls = %d, want 1", len(calls))
	}
	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 1 {
		t.Fatalf("answerCallbackQuery calls = %d, want 1", len(calls))
	}
}

func TestConnectionButtonsRenderUserAndMainViews(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: 42470, FirstName: "Member"}
	chatID := uniqueModuleChatID()
	connections.ConnectId(user.Id, chatID)

	privateChat := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Member"}
	for _, data := range []string{
		encodeCallbackData("connbtns", map[string]string{"t": "User"}, "connbtns.User"),
		encodeCallbackData("connbtns", map[string]string{"t": "Main"}, "connbtns.Main"),
	} {
		ctx := newModuleCallbackContext(bot, privateChat, user, data)
		if err := ConnectionsModule.connectionButtons(bot, ctx); err != ext.EndGroups {
			t.Fatalf("connectionButtons(%q) error = %v, want EndGroups", data, err)
		}
	}
	if calls := client.callsFor("editMessageText"); len(calls) != 2 {
		t.Fatalf("editMessageText calls = %d, want two button view edits", len(calls))
	}
	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 2 {
		t.Fatalf("answerCallbackQuery calls = %d, want two callback answers", len(calls))
	}
}

func TestConnectionButtonsSkipMissingCallbackOrConnection(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: 42471, FirstName: "Member"}
	privateChat := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Member"}

	messageCtx := newModuleMessageContext(bot, privateChat, user, "/connection")
	if err := ConnectionsModule.connectionButtons(bot, messageCtx); err != ext.EndGroups {
		t.Fatalf("connectionButtons(message update) error = %v, want EndGroups", err)
	}

	callbackCtx := newModuleCallbackContext(bot, privateChat, user, "connbtns.User")
	if err := ConnectionsModule.connectionButtons(bot, callbackCtx); err != ext.EndGroups {
		t.Fatalf("connectionButtons(not connected) error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("editMessageText"); len(calls) != 0 {
		t.Fatalf("editMessageText calls = %d, want none for missing context/connection", len(calls))
	}
	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 1 {
		t.Fatalf("answerCallbackQuery calls = %d, want not-connected callback answer", len(calls))
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 0 {
		t.Fatalf("sendMessage calls = %d, want callback path without chat message", len(calls))
	}
}

func TestConnectionButtonsRejectInvalidData(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: 4248, FirstName: "Member"}
	privateChat := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Member"}
	ctx := newModuleCallbackContext(bot, privateChat, user, "connbtns")

	if err := ConnectionsModule.connectionButtons(bot, ctx); err != ext.EndGroups {
		t.Fatalf("connectionButtons() error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("editMessageText"); len(calls) != 0 {
		t.Fatalf("editMessageText calls = %d, want 0 for invalid data", len(calls))
	}
	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 1 {
		t.Fatalf("answerCallbackQuery calls = %d, want 1", len(calls))
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
