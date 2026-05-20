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
	waitForModuleCondition(t, func() bool {
		username, name, found := db.GetUserInfoById(userID)
		return found && username == "user_name" && name == "User Name"
	})

	asyncUpdateChat(chatID, "Users Chat", userID)
	waitForModuleCondition(t, func() bool {
		chat := db.GetChatSettings(chatID)
		return chat.ChatId == chatID && chat.ChatName == "Users Chat"
	})

	asyncUpdateChannel(channelID, "Updates", "updates")
	waitForModuleCondition(t, func() bool {
		channelUsername, channelName, found := db.GetChannelInfoById(channelID)
		return found && channelUsername == "updates" && channelName == "Updates"
	})
}
