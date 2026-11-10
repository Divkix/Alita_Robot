//go:build testtools

package modules

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db/connections"
	"github.com/divkix/Alita_Robot/alita/i18n"
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
	if err := connections.ToggleAllowConnect(chatID, true); err != nil {
		t.Fatalf("ToggleAllowConnect() error = %v", err)
	}

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
	if err := connections.ToggleAllowConnect(chatID, false); err != nil {
		t.Fatalf("connections.ToggleAllowConnect() error = %v", err)
	}

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
	if err := connections.ToggleAllowConnect(chatID, true); err != nil {
		t.Fatalf("ToggleAllowConnect() error = %v", err)
	}

	allowed, denyKey := canUserConnectToChat(bot, chatID, 42)
	if allowed {
		t.Fatal("canUserConnectToChat() allowed = true, want false on member lookup failure")
	}
	if denyKey != "error_generic" {
		t.Fatalf("denyKey = %q, want generic error key", denyKey)
	}
}

func TestCanUserConnectToChatDeniesRestrictedFormerMember(t *testing.T) {
	client := newModuleBotClient()
	client.responses["getChatMember"] = []byte(`{"status":"restricted","user":{"id":42,"is_bot":false,"first_name":"Former Member"},"is_member":false}`)
	bot := newModuleTestBot(client)
	chatID := uniqueModuleChatID()
	_ = connections.GetChatConnectionSetting(chatID)
	if err := connections.ToggleAllowConnect(chatID, true); err != nil {
		t.Fatalf("ToggleAllowConnect() error = %v", err)
	}

	allowed, denyKey := canUserConnectToChat(bot, chatID, 42)
	if allowed {
		t.Fatal("canUserConnectToChat() allowed = true, want false for restricted non-member")
	}
	if denyKey != "connections_connect_not_member" {
		t.Fatalf("denyKey = %q, want not-member key", denyKey)
	}
}

func TestAllowConnectTogglesSetting(t *testing.T) {
	useEnglishLocale(t)
	f := newAdminChatFixture(t)
	client, bot, chat, user := f.client, f.bot, f.chat, f.admin
	chatID := chat.Id

	onCtx := newModuleMessageContext(bot, chat, user, "/allowconnect on")
	if err := connectionsDispatcher().ProcessUpdate(bot, onCtx.Update, nil); err != nil {
		t.Fatalf("dispatch /allowconnect on: %v", err)
	}
	if !connections.GetChatConnectionSetting(chatID).AllowConnect {
		t.Fatal("AllowConnect was not enabled")
	}

	currentCtx := newModuleMessageContext(bot, chat, user, "/allowconnect")
	if err := connectionsDispatcher().ProcessUpdate(bot, currentCtx.Update, nil); err != nil {
		t.Fatalf("dispatch /allowconnect: %v", err)
	}

	offCtx := newModuleMessageContext(bot, chat, user, "/allowconnect no")
	if err := connectionsDispatcher().ProcessUpdate(bot, offCtx.Update, nil); err != nil {
		t.Fatalf("dispatch /allowconnect no: %v", err)
	}
	if connections.GetChatConnectionSetting(chatID).AllowConnect {
		t.Fatal("AllowConnect stayed enabled after no")
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 3 {
		t.Fatalf("sendMessage calls = %d, want on/current/off replies", len(calls))
	}
}

func TestAllowConnectHandlesInvalidOptionAndNonAdminNoop(t *testing.T) {
	useEnglishLocale(t)
	f := newAdminChatFixture(t)
	client, bot, chat := f.client, f.bot, f.chat
	chatID := chat.Id

	invalidCtx := newModuleMessageContext(bot, chat, f.admin, "/allowconnect maybe")
	if err := connectionsDispatcher().ProcessUpdate(bot, invalidCtx.Update, nil); err != nil {
		t.Fatalf("dispatch /allowconnect maybe: %v", err)
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want invalid-option reply", len(calls))
	}

	memberCtx := newModuleMessageContext(bot, chat, gotgbot.User{Id: 42, FirstName: "Member"}, "/allowconnect on")
	if err := connections.ToggleAllowConnect(chatID, false); err != nil {
		t.Fatalf("connections.ToggleAllowConnect() error = %v", err)
	}
	if err := connectionsDispatcher().ProcessUpdate(bot, memberCtx.Update, nil); err != nil {
		t.Fatalf("dispatch member /allowconnect: %v", err)
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
	if err := connections.ConnectId(user.Id, chatID); err != nil {
		t.Fatalf("connections.ConnectId() error = %v", err)
	}

	privateChat := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Member"}
	ctx := newModuleMessageContext(bot, privateChat, user, "/disconnect")
	if err := connectionsDispatcher().ProcessUpdate(bot, ctx.Update, nil); err != nil {
		t.Fatalf("dispatch /disconnect: %v", err)
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
	if err := connections.ConnectId(user.Id, connectedChatID); err != nil {
		t.Fatalf("connections.ConnectId() error = %v", err)
	}

	groupChat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Connections Chat"}
	ctx := newModuleMessageContext(bot, groupChat, user, "/disconnect")
	if err := connectionsDispatcher().ProcessUpdate(bot, ctx.Update, nil); err != nil {
		t.Fatalf("dispatch group /disconnect: %v", err)
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
	if err := connections.ConnectId(user.Id, chatID); err != nil {
		t.Fatalf("connections.ConnectId() error = %v", err)
	}

	privateChat := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Member"}
	ctx := newModuleMessageContext(bot, privateChat, user, "/connection")
	if err := connectionsDispatcher().ProcessUpdate(bot, ctx.Update, nil); err != nil {
		t.Fatalf("dispatch /connection: %v", err)
	}
	if calls := client.callsFor("getChat"); len(calls) == 0 {
		t.Fatal("connection() did not fetch connected chat")
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
}

func TestConnectionEscapesGroupTitle(t *testing.T) {
	useEnglishLocale(t)
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: 42448, FirstName: "Member"}
	chatID := uniqueModuleChatID()
	client.chatsByID = map[string]json.RawMessage{
		fmt.Sprint(chatID): json.RawMessage(fmt.Sprintf(`{"id":%d,"type":"supergroup","title":"Q&A <x>"}`, chatID)),
	}
	if err := connections.ConnectId(user.Id, chatID); err != nil {
		t.Fatalf("ConnectId() error = %v", err)
	}
	t.Cleanup(func() { _ = connections.DisconnectId(user.Id) })

	privateChat := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Member"}
	ctx := newModuleMessageContext(bot, privateChat, user, "/connection")
	if err := connectionsDispatcher().ProcessUpdate(bot, ctx.Update, nil); err != nil {
		t.Fatalf("dispatch /connection: %v", err)
	}
	text := fmt.Sprint(client.onlyCall(t, "sendMessage").Params["text"])
	if !strings.Contains(text, "Q&amp;A &lt;x&gt;") {
		t.Fatalf("connection reply = %q, want the group title HTML-escaped", text)
	}
}

func TestConnectionDisconnectsFormerMember(t *testing.T) {
	useEnglishLocale(t)
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: 13, FirstName: "Former Member"}
	if err := connections.ConnectId(user.Id, uniqueModuleChatID()); err != nil {
		t.Fatalf("ConnectId() error = %v", err)
	}

	privateChat := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Former Member"}
	ctx := newModuleMessageContext(bot, privateChat, user, "/connection")
	if err := connectionsDispatcher().ProcessUpdate(bot, ctx.Update, nil); err != nil {
		t.Fatalf("dispatch /connection: %v", err)
	}
	if connections.Connection(user.Id).Connected {
		t.Fatal("former member's stale connection remained active")
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want stale-connection reply", len(calls))
	}
	want, _ := i18n.MustNewTranslator("en").GetString("connections_stale_connection")
	assertParams(t, client.onlyCall(t, "sendMessage"), map[string]any{"text": want})
}

func TestConnectionMembershipLookupFailureDeniesWithoutDisconnecting(t *testing.T) {
	useEnglishLocale(t)
	client := newModuleBotClient()
	client.errors["getChatMember"] = fmt.Errorf("telegram unavailable")
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: 42449, FirstName: "Member"}
	if err := connections.ConnectId(user.Id, uniqueModuleChatID()); err != nil {
		t.Fatalf("ConnectId() error = %v", err)
	}

	privateChat := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Member"}
	ctx := newModuleMessageContext(bot, privateChat, user, "/connection")
	if err := connectionsDispatcher().ProcessUpdate(bot, ctx.Update, nil); err != nil {
		t.Fatalf("dispatch /connection: %v", err)
	}
	if !connections.Connection(user.Id).Connected {
		t.Fatal("transient membership lookup failure disconnected a valid connection")
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want generic-error reply", len(calls))
	}
	want, _ := i18n.MustNewTranslator("en").GetString("error_generic")
	assertParams(t, client.onlyCall(t, "sendMessage"), map[string]any{"text": want})
}

func TestConnectionReportsNotConnectedAndLookupErrors(t *testing.T) {
	useEnglishLocale(t)
	user := gotgbot.User{Id: 42440, FirstName: "Member"}
	privateChat := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Member"}

	noConnClient := newModuleBotClient()
	noConnBot := newModuleTestBot(noConnClient)
	noConnCtx := newModuleMessageContext(noConnBot, privateChat, user, "/connection")
	if err := connectionsDispatcher().ProcessUpdate(noConnBot, noConnCtx.Update, nil); err != nil {
		t.Fatalf("dispatch unconnected /connection: %v", err)
	}
	if calls := noConnClient.callsFor("getChat"); len(calls) != 0 {
		t.Fatalf("getChat calls = %d, want none without connection", len(calls))
	}
	if calls := noConnClient.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want not-connected reply", len(calls))
	}
	want, _ := i18n.MustNewTranslator("en").GetString("connections_not_connected")
	if text := noConnClient.callsFor("sendMessage")[0].Params["text"]; text != want {
		t.Fatalf("connection(not connected) reply = %q, want %q", text, want)
	}

	lookupClient := newModuleBotClient()
	lookupClient.errors["getChat"] = fmt.Errorf("telegram unavailable")
	lookupBot := newModuleTestBot(lookupClient)
	if err := connections.ConnectId(user.Id, uniqueModuleChatID()); err != nil {
		t.Fatalf("connections.ConnectId() error = %v", err)
	}
	lookupCtx := newModuleMessageContext(lookupBot, privateChat, user, "/connection")
	if err := connectionsDispatcher().ProcessUpdate(lookupBot, lookupCtx.Update, nil); err != nil {
		t.Fatalf("dispatch /connection lookup error: %v", err)
	}
	if !connections.Connection(user.Id).Connected {
		t.Fatal("transient connected-chat lookup cleared a valid connection")
	}
	if calls := lookupClient.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want generic-error reply", len(calls))
	}
	want, _ = i18n.MustNewTranslator("en").GetString("error_generic")
	assertParams(t, lookupClient.onlyCall(t, "sendMessage"), map[string]any{"text": want})
}

func TestConnectInGroupRepliesWithDeepLinkButton(t *testing.T) {
	f := newAdminChatFixture(t)
	client, bot, chat, user := f.client, f.bot, f.chat, f.admin
	ctx := newModuleMessageContext(bot, chat, user, "/connect")

	if err := connectionsDispatcher().ProcessUpdate(bot, ctx.Update, nil); err != nil {
		t.Fatalf("dispatch group /connect: %v", err)
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
	t.Cleanup(func() {
		if err := connections.DisconnectId(user.Id); err != nil {
			t.Errorf("connections.DisconnectId() error = %v", err)
		}
	})

	missingCtx := newModuleMessageContext(bot, privateChat, user, "/connect")
	if err := connectionsDispatcher().ProcessUpdate(bot, missingCtx.Update, nil); err != nil {
		t.Fatalf("dispatch /connect without target: %v", err)
	}
	if connections.Connection(user.Id).Connected {
		t.Fatal("missing chat id connected the user")
	}

	if err := connections.ToggleAllowConnect(-1001, true); err != nil {
		t.Fatalf("connections.ToggleAllowConnect() error = %v", err)
	}
	connectCtx := newModuleMessageContext(bot, privateChat, user, "/connect -1001")
	if err := connectionsDispatcher().ProcessUpdate(bot, connectCtx.Update, nil); err != nil {
		t.Fatalf("dispatch PM /connect: %v", err)
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
	if err := connections.ConnectId(user.Id, chatID); err != nil {
		t.Fatalf("connections.ConnectId() error = %v", err)
	}
	if err := connections.DisconnectId(user.Id); err != nil {
		t.Fatalf("connections.DisconnectId() error = %v", err)
	}
	if err := connections.ToggleAllowConnect(chatID, true); err != nil {
		t.Fatalf("ToggleAllowConnect() error = %v", err)
	}

	privateChat := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Member"}
	ctx := newModuleMessageContext(bot, privateChat, user, "/reconnect")
	if err := connectionsDispatcher().ProcessUpdate(bot, ctx.Update, nil); err != nil {
		t.Fatalf("dispatch /reconnect: %v", err)
	}
	if !connections.Connection(user.Id).Connected {
		t.Fatal("user was not reconnected to previous chat")
	}
	if calls := client.callsFor("getChat"); len(calls) == 0 {
		t.Fatal("reconnect skipped the connected-chat lookup")
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(calls))
	}
}

func TestReconnectPrivateHandlesNoLastChatAndLookupFailure(t *testing.T) {
	useEnglishLocale(t)
	user := gotgbot.User{Id: 42451, FirstName: "Member"}
	privateChat := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Member"}

	noLastClient := newModuleBotClient()
	noLastBot := newModuleTestBot(noLastClient)
	noLastCtx := newModuleMessageContext(noLastBot, privateChat, user, "/reconnect")
	if err := connectionsDispatcher().ProcessUpdate(noLastBot, noLastCtx.Update, nil); err != nil {
		t.Fatalf("dispatch /reconnect without last chat: %v", err)
	}
	if calls := noLastClient.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want no-last-chat reply", len(calls))
	}
	want, _ := i18n.MustNewTranslator("en").GetString("connections_reconnect_no_last_chat")
	assertParams(t, noLastClient.onlyCall(t, "sendMessage"), map[string]any{"text": want})

	lookupClient := newModuleBotClient()
	lookupClient.errors["getChat"] = fmt.Errorf("telegram unavailable")
	lookupBot := newModuleTestBot(lookupClient)
	if err := connections.ConnectId(user.Id, uniqueModuleChatID()); err != nil {
		t.Fatalf("connections.ConnectId() error = %v", err)
	}
	if err := connections.DisconnectId(user.Id); err != nil {
		t.Fatalf("connections.DisconnectId() error = %v", err)
	}
	lookupCtx := newModuleMessageContext(lookupBot, privateChat, user, "/reconnect")
	if err := connectionsDispatcher().ProcessUpdate(lookupBot, lookupCtx.Update, nil); err != nil {
		t.Fatalf("dispatch /reconnect lookup error: %v", err)
	}
	if connections.Connection(user.Id).Connected {
		t.Fatal("failed reconnect lookup changed Connected to true")
	}
	if calls := lookupClient.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want generic-error reply", len(calls))
	}
	want, _ = i18n.MustNewTranslator("en").GetString("error_generic")
	assertParams(t, lookupClient.onlyCall(t, "sendMessage"), map[string]any{"text": want})
}

func TestReconnectLookupFailurePreservesActiveConnection(t *testing.T) {
	useEnglishLocale(t)
	client := newModuleBotClient()
	client.errors["getChat"] = fmt.Errorf("telegram unavailable")
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: 42452, FirstName: "Member"}
	chatID := uniqueModuleChatID()
	if err := connections.ConnectId(user.Id, chatID); err != nil {
		t.Fatalf("ConnectId() error = %v", err)
	}
	t.Cleanup(func() { _ = connections.DisconnectId(user.Id) })

	privateChat := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Member"}
	ctx := newModuleMessageContext(bot, privateChat, user, "/reconnect")
	if err := connectionsDispatcher().ProcessUpdate(bot, ctx.Update, nil); err != nil {
		t.Fatalf("dispatch /reconnect lookup error: %v", err)
	}
	if conn := connections.Connection(user.Id); !conn.Connected || conn.ChatId != chatID {
		t.Fatalf("connection after transient lookup = %+v, want active chat %d preserved", conn, chatID)
	}
	want, _ := i18n.MustNewTranslator("en").GetString("error_generic")
	if text := client.onlyCall(t, "sendMessage").Params["text"]; text != want {
		t.Fatalf("reconnect(lookup error) reply = %q, want %q", text, want)
	}
}

func TestReconnectMembershipLookupFailurePreservesActiveConnection(t *testing.T) {
	useEnglishLocale(t)
	client := newModuleBotClient()
	client.errors["getChatMember"] = fmt.Errorf("telegram unavailable")
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: 42454, FirstName: "Member"}
	chatID := uniqueModuleChatID()
	if err := connections.ConnectId(user.Id, chatID); err != nil {
		t.Fatalf("ConnectId() error = %v", err)
	}
	t.Cleanup(func() { _ = connections.DisconnectId(user.Id) })

	privateChat := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Member"}
	ctx := newModuleMessageContext(bot, privateChat, user, "/reconnect")
	if err := connectionsDispatcher().ProcessUpdate(bot, ctx.Update, nil); err != nil {
		t.Fatalf("dispatch /reconnect membership error: %v", err)
	}
	if conn := connections.Connection(user.Id); !conn.Connected || conn.ChatId != chatID {
		t.Fatalf("connection after membership lookup failure = %+v, want active chat %d preserved", conn, chatID)
	}
	want, _ := i18n.MustNewTranslator("en").GetString("error_generic")
	if text := client.onlyCall(t, "sendMessage").Params["text"]; text != want {
		t.Fatalf("reconnect(membership error) reply = %q, want %q", text, want)
	}
}

func TestReconnectPrivateAuthorizesBeforeConnecting(t *testing.T) {
	useEnglishLocale(t)
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: 13, FirstName: "Former Member"}
	chatID := uniqueModuleChatID()
	if err := connections.ConnectId(user.Id, chatID); err != nil {
		t.Fatalf("ConnectId() error = %v", err)
	}
	if err := connections.DisconnectId(user.Id); err != nil {
		t.Fatalf("DisconnectId() error = %v", err)
	}

	privateChat := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Former Member"}
	ctx := newModuleMessageContext(bot, privateChat, user, "/reconnect")
	if err := connectionsDispatcher().ProcessUpdate(bot, ctx.Update, nil); err != nil {
		t.Fatalf("dispatch stale /reconnect: %v", err)
	}
	if connections.Connection(user.Id).Connected {
		t.Fatal("former member was reconnected before authorization")
	}
	if calls := client.callsFor("sendMessage"); len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want stale-connection reply", len(calls))
	}
	want, _ := i18n.MustNewTranslator("en").GetString("connections_stale_connection")
	assertParams(t, client.onlyCall(t, "sendMessage"), map[string]any{"text": want})
}

func TestReconnectPrivateDeniesWhenAllowConnectOff(t *testing.T) {
	restoreI18n, err := i18n.OverrideManagerForTest(`
connections_connect_connection_disabled: connect disabled
connections_connect_not_member: not member
connections_stale_connection: stale connection
connections_reconnect_reconnected: reconnected to %s
common_settings_save_failed: save failed
`)
	if err != nil {
		t.Fatalf("OverrideManagerForTest: %v", err)
	}
	t.Cleanup(restoreI18n)

	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: 42453, FirstName: "Member"}
	chatID := uniqueModuleChatID()
	if err := connections.ConnectId(user.Id, chatID); err != nil {
		t.Fatalf("ConnectId() error = %v", err)
	}
	if err := connections.DisconnectId(user.Id); err != nil {
		t.Fatalf("DisconnectId() error = %v", err)
	}
	if err := connections.ToggleAllowConnect(chatID, false); err != nil {
		t.Fatalf("ToggleAllowConnect() error = %v", err)
	}

	privateChat := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Member"}
	ctx := newModuleMessageContext(bot, privateChat, user, "/reconnect")
	if err := connectionsDispatcher().ProcessUpdate(bot, ctx.Update, nil); err != nil {
		t.Fatalf("dispatch disabled /reconnect: %v", err)
	}
	if connections.Connection(user.Id).Connected {
		t.Fatal("non-admin was reconnected while allowconnect is off")
	}
	calls := client.callsFor("sendMessage")
	if len(calls) != 1 {
		t.Fatalf("sendMessage calls = %d, want deny reply", len(calls))
	}
	if text := calls[0].Params["text"]; text != "connect disabled" {
		t.Fatalf("reconnect reply = %q, want deny key text, not stale-connection", text)
	}
}

func TestReconnectInGroupPromptsForPrivateChat(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Connections Chat"}
	user := gotgbot.User{Id: 4246, FirstName: "Member"}
	ctx := newModuleMessageContext(bot, chat, user, "/reconnect")

	if err := connectionsDispatcher().ProcessUpdate(bot, ctx.Update, nil); err != nil {
		t.Fatalf("dispatch group /reconnect: %v", err)
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
	if err := connections.ConnectId(user.Id, chatID); err != nil {
		t.Fatalf("connections.ConnectId() error = %v", err)
	}

	privateChat := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Member"}
	ctx := newModuleCallbackContext(bot, privateChat, user, encodeCallbackData("connbtns", map[string]string{"t": "Admin"}))
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
	if err := connections.ConnectId(user.Id, chatID); err != nil {
		t.Fatalf("connections.ConnectId() error = %v", err)
	}

	privateChat := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Member"}
	for _, data := range []string{
		encodeCallbackData("connbtns", map[string]string{"t": "User"}),
		encodeCallbackData("connbtns", map[string]string{"t": "Main"}),
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
	useEnglishLocale(t)
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	user := gotgbot.User{Id: 42471, FirstName: "Member"}
	privateChat := gotgbot.Chat{Id: user.Id, Type: "private", FirstName: "Member"}

	messageCtx := newModuleMessageContext(bot, privateChat, user, "/connection")
	if err := ConnectionsModule.connectionButtons(bot, messageCtx); err != ext.EndGroups {
		t.Fatalf("connectionButtons(message update) error = %v, want EndGroups", err)
	}

	callbackCtx := newModuleCallbackContext(bot, privateChat, user, encodeCallbackData("connbtns", map[string]string{"t": "User"}))
	if err := ConnectionsModule.connectionButtons(bot, callbackCtx); err != ext.EndGroups {
		t.Fatalf("connectionButtons(not connected) error = %v, want EndGroups", err)
	}
	if calls := client.callsFor("editMessageText"); len(calls) != 0 {
		t.Fatalf("editMessageText calls = %d, want none for missing context/connection", len(calls))
	}
	answers := client.callsFor("answerCallbackQuery")
	if len(answers) != 1 {
		t.Fatalf("answerCallbackQuery calls = %d, want not-connected callback answer", len(answers))
	}
	want, _ := i18n.MustNewTranslator("en").GetString("connections_not_connected")
	if text := answers[0].Params["text"]; text != want {
		t.Fatalf("not-connected callback answer = %q, want %q", text, want)
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

	for _, data := range []string{
		"connbtns",
		encodeCallbackData("connbtns", map[string]string{"t": "crafted"}),
	} {
		ctx := newModuleCallbackContext(bot, privateChat, user, data)
		if err := ConnectionsModule.connectionButtons(bot, ctx); err != ext.EndGroups {
			t.Fatalf("connectionButtons(%q) error = %v, want EndGroups", data, err)
		}
	}
	if calls := client.callsFor("editMessageText"); len(calls) != 0 {
		t.Fatalf("editMessageText calls = %d, want 0 for invalid data", len(calls))
	}
	if calls := client.callsFor("answerCallbackQuery"); len(calls) != 2 {
		t.Fatalf("answerCallbackQuery calls = %d, want 2", len(calls))
	}
}

func TestConnectionCommandsPreservePrivateOnlyReplies(t *testing.T) {
	useEnglishLocale(t)
	for _, tc := range []struct {
		command string
		key     string
	}{
		{"/connection", "chat_status_pm_only_error"},
		{"/disconnect", "connections_disconnect_need_pm"},
		{"/reconnect", "connections_reconnect_need_pm"},
	} {
		t.Run(tc.command, func(t *testing.T) {
			f := newAdminChatFixture(t)
			ctx := newModuleMessageContext(f.bot, f.chat, f.member, tc.command)
			if err := connectionsDispatcher().ProcessUpdate(f.bot, ctx.Update, nil); err != nil {
				t.Fatalf("dispatch %s: %v", tc.command, err)
			}
			want, _ := i18n.MustNewTranslator("en").GetString(tc.key)
			call := f.client.onlyCall(t, "sendMessage")
			assertParams(t, call, map[string]any{"chat_id": f.chat.Id, "text": want})
			if calls := f.client.callsFor("getChat"); len(calls) != 0 {
				t.Fatalf("getChat calls = %d, want no target lookup before PM refusal", len(calls))
			}
		})
	}
}

func TestConnectionCommandsPreserveMemberPolicy(t *testing.T) {
	useEnglishLocale(t)
	for _, command := range []string{"connect group", "connect PM", "reconnect"} {
		for _, allow := range []bool{false, true} {
			for _, admin := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/allow=%t/admin=%t", command, allow, admin), func(t *testing.T) {
					f := newAdminChatFixture(t)
					user := f.member
					if admin {
						user = f.admin
					}
					if err := connections.ToggleAllowConnect(f.chat.Id, allow); err != nil {
						t.Fatal(err)
					}
					if err := connections.DisconnectId(user.Id); err != nil {
						t.Fatal(err)
					}
					chat, text := privateChatOf(user), "/reconnect"
					switch command {
					case "connect group":
						chat, text = f.chat, "/connect"
					case "connect PM":
						text = fmt.Sprintf("/connect %d", f.chat.Id)
					case "reconnect":
						if err := connections.ConnectId(user.Id, f.chat.Id); err != nil {
							t.Fatal(err)
						}
						if err := connections.DisconnectId(user.Id); err != nil {
							t.Fatal(err)
						}
					}
					t.Cleanup(func() { _ = connections.DisconnectId(user.Id) })
					ctx := newModuleMessageContext(f.bot, chat, user, text)
					if err := connectionsDispatcher().ProcessUpdate(f.bot, ctx.Update, nil); err != nil {
						t.Fatalf("dispatch %s: %v", text, err)
					}
					allowed := admin || allow
					conn := connections.Connection(user.Id)
					if want := allowed && command != "connect group"; conn.Connected != want {
						t.Fatalf("connection = %+v, want connected=%t", conn, want)
					}
					if conn.Connected && conn.ChatId != f.chat.Id {
						t.Fatalf("connection target = %d, want %d", conn.ChatId, f.chat.Id)
					}
					call := f.client.onlyCall(t, "sendMessage")
					if !allowed {
						want, _ := i18n.MustNewTranslator("en").GetString("connections_connect_connection_disabled")
						assertParams(t, call, map[string]any{"text": want})
					} else if command == "connect group" {
						markup, ok := call.Params["reply_markup"].(gotgbot.InlineKeyboardMarkup)
						if !ok || len(markup.InlineKeyboard) == 0 || len(markup.InlineKeyboard[0]) == 0 {
							t.Fatalf("connect reply markup = %#v, want deep-link button", call.Params["reply_markup"])
						}
						want := fmt.Sprintf("https://t.me/%s?start=connect_%d", f.bot.Username, f.chat.Id)
						if url := markup.InlineKeyboard[0][0].Url; url != want {
							t.Fatalf("connect URL = %q, want %q", url, want)
						}
					} else if !strings.Contains(fmt.Sprint(call.Params["text"]), f.chat.Title) {
						t.Fatalf("connection reply = %q, want target title", call.Params["text"])
					}
				})
			}
		}
	}
}

func TestConnectPrivatePreservesMembershipRefusals(t *testing.T) {
	useEnglishLocale(t)
	for _, tc := range []struct {
		name   string
		member string
		err    error
		key    string
	}{
		{"former member", `{"status":"restricted","user":{"id":42,"is_bot":false,"first_name":"Former Member"},"is_member":false}`, nil, "connections_connect_not_member"},
		{"lookup failure", "", fmt.Errorf("telegram unavailable"), "error_generic"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newAdminChatFixture(t)
			if err := connections.ToggleAllowConnect(f.chat.Id, true); err != nil {
				t.Fatal(err)
			}
			if tc.err != nil {
				f.client.errors["getChatMember"] = tc.err
			} else {
				f.client.membersByUser[fmt.Sprint(f.member.Id)] = json.RawMessage(tc.member)
			}
			if err := connections.DisconnectId(f.member.Id); err != nil {
				t.Fatal(err)
			}
			ctx := newModuleMessageContext(f.bot, privateChatOf(f.member), f.member, fmt.Sprintf("/connect %d", f.chat.Id))
			if err := connectionsDispatcher().ProcessUpdate(f.bot, ctx.Update, nil); err != nil {
				t.Fatalf("dispatch /connect: %v", err)
			}
			if connections.Connection(f.member.Id).Connected {
				t.Fatal("denied /connect activated a connection")
			}
			want, _ := i18n.MustNewTranslator("en").GetString(tc.key)
			assertParams(t, f.client.onlyCall(t, "sendMessage"), map[string]any{"text": want})
		})
	}
}

func TestAllowConnectPrivateRequiresConnectedAdminWithoutBotAdmin(t *testing.T) {
	useEnglishLocale(t)
	for _, admin := range []bool{false, true} {
		t.Run(fmt.Sprintf("admin=%t", admin), func(t *testing.T) {
			f := newAdminChatFixture(t)
			f.client.adminsByChat[fmt.Sprint(f.chat.Id)] = json.RawMessage(fmt.Sprintf(`[{"status":"administrator","user":{"id":%d,"is_bot":false,"first_name":"Admin"},%s}]`, f.admin.Id, fixtureAdminRights))
			f.client.membersByUser[fmt.Sprint(f.bot.Id)] = json.RawMessage(`{"status":"member","user":{"id":999,"is_bot":true,"first_name":"Alita"}}`)
			user := f.member
			if admin {
				user = f.admin
			}
			if err := connections.ToggleAllowConnect(f.chat.Id, true); err != nil {
				t.Fatal(err)
			}
			if err := connections.ConnectId(user.Id, f.chat.Id); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = connections.DisconnectId(user.Id) })
			ctx := newModuleMessageContext(f.bot, privateChatOf(user), user, "/allowconnect off")
			if err := connectionsDispatcher().ProcessUpdate(f.bot, ctx.Update, nil); err != nil {
				t.Fatalf("dispatch /allowconnect: %v", err)
			}
			if got := connections.GetChatConnectionSetting(f.chat.Id).AllowConnect; got == admin {
				t.Fatalf("AllowConnect = %t, want %t", got, !admin)
			}
			key := "connections_is_user_connected_user_not_admin"
			if admin {
				key = "connections_allow_connect_turned_off"
			}
			want, _ := i18n.MustNewTranslator("en").GetString(key)
			assertParams(t, f.client.onlyCall(t, "sendMessage"), map[string]any{"chat_id": user.Id, "text": want})
		})
	}
}

func TestConnectionUsesSavedMemberTargetWithoutGroupParsing(t *testing.T) {
	useEnglishLocale(t)
	f := newAdminChatFixture(t)
	if err := connections.ToggleAllowConnect(f.chat.Id, false); err != nil {
		t.Fatal(err)
	}
	if err := connections.ConnectId(f.member.Id, f.chat.Id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connections.DisconnectId(f.member.Id) })

	ctx := newModuleMessageContext(f.bot, privateChatOf(f.member), f.member, "/connection ignored")
	if err := connectionsDispatcher().ProcessUpdate(f.bot, ctx.Update, nil); err != nil {
		t.Fatalf("dispatch /connection: %v", err)
	}
	call := f.client.onlyCall(t, "sendMessage")
	want, _ := i18n.MustNewTranslator("en").GetString("connections_connected")
	assertParams(t, call, map[string]any{
		"chat_id": f.member.Id,
		"text":    fmt.Sprintf(want, f.chat.Title),
	})
	markup, ok := call.Params["reply_markup"].(gotgbot.InlineKeyboardMarkup)
	if !ok || len(markup.InlineKeyboard) != 1 || len(markup.InlineKeyboard[0]) != 1 {
		t.Fatalf("connection keyboard = %#v, want member commands only", call.Params["reply_markup"])
	}
	if data := markup.InlineKeyboard[0][0].CallbackData; data != encodeCallbackData("connbtns", map[string]string{"t": "User"}) {
		t.Fatalf("member command callback = %q", data)
	}
	if conn := connections.Connection(f.member.Id); !conn.Connected || conn.ChatId != f.chat.Id {
		t.Fatalf("saved connection = %+v, want target preserved despite allow-connect being off", conn)
	}
}

func TestConnectionIdentityOnlyCommandsRefuseChannelSenders(t *testing.T) {
	useRealLocales(t)
	for _, command := range []string{"/connect", "/settings"} {
		t.Run(command, func(t *testing.T) {
			f := newAdminChatFixture(t)
			update := newModuleMessageContext(f.bot, f.chat, f.member, command).Update
			update.Message.From = nil
			update.Message.SenderChat = &gotgbot.Chat{Id: uniqueModuleChatID(), Type: "channel"}
			if err := connectionsDispatcher().ProcessUpdate(f.bot, update, nil); err != nil {
				t.Fatal(err)
			}
			if got := fmt.Sprint(f.client.onlyCall(t, "sendMessage").Params["text"]); got != "Cannot identify user. This command cannot be used by channels." {
				t.Fatalf("channel %s = %q, want the identity refusal", command, got)
			}
		})
	}
}
