package modules

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/connections"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
)

func canUserConnectToChat(b *gotgbot.Bot, chatID, userID int64) (bool, string) {
	settings := connections.GetChatConnectionSetting(chatID)
	if chat_status.IsUserAdmin(b, chatID, userID) {
		return true, ""
	}

	if settings.AllowConnect && chat_status.IsUserInChat(b, &gotgbot.Chat{Id: chatID}, userID) {
		return true, ""
	}

	log.WithFields(log.Fields{
		"chatId":       chatID,
		"userId":       userID,
		"allowConnect": settings.AllowConnect,
		"denyReason":   "allow_connect_disabled_non_admin",
	}).Warn("[Connections] Connection request denied")
	return false, "connections_connect_connection_disabled"
}
