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

	var denyReason, denyKey string
	var lookupErr error
	if !settings.AllowConnect {
		denyReason, denyKey = "allow_connect_disabled_non_admin", "connections_connect_connection_disabled"
	} else if isMember, err := chat_status.IsUserInChatWithError(b, &gotgbot.Chat{Id: chatID}, userID); err != nil {
		denyReason, denyKey, lookupErr = "member_lookup_failed", "error_generic", err
	} else if !isMember {
		denyReason, denyKey = "not_chat_member", "connections_connect_not_member"
	} else {
		return true, ""
	}

	log.WithFields(log.Fields{
		"chatId":       chatID,
		"userId":       userID,
		"allowConnect": settings.AllowConnect,
		"denyReason":   denyReason,
		"error":        lookupErr,
	}).Warn("[Connections] Connection request denied")
	return false, denyKey
}
