package modules

import (
	"fmt"
	"testing"

	"github.com/divkix/Alita_Robot/alita/db"
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
