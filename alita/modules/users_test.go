package modules

import (
	"testing"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
)

func TestAsyncUserUpdateWrappersPersistRecords(t *testing.T) {
	userID := time.Now().UnixNano()
	chatID := uniqueModuleChatID()
	channelID := -1000000000000 - userID%1000000

	asyncUpdateUser(userID, "user_name", "User Name")
	username, name, found := db.GetUserInfoById(userID)
	if !found || username != "user_name" || name != "User Name" {
		t.Fatalf("GetUserInfoById() = (%q, %q, %v), want persisted user", username, name, found)
	}

	asyncUpdateChat(chatID, "Users Chat", userID)
	chat := db.GetChatSettings(chatID)
	if chat.ChatId != chatID || chat.ChatName != "Users Chat" {
		t.Fatalf("GetChatSettings() = %+v, want persisted chat", chat)
	}

	asyncUpdateChannel(channelID, "Updates", "updates")
	channelUsername, channelName, found := db.GetChannelInfoById(channelID)
	if !found || channelUsername != "updates" || channelName != "Updates" {
		t.Fatalf(
			"GetChannelInfoById() = (%q, %q, %v), want persisted channel",
			channelUsername,
			channelName,
			found,
		)
	}
}
