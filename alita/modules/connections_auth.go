package modules

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
)

// canUserConnectToChat enforces the same authorization gate for /connect and deep-link connect.
func canUserConnectToChat(b *gotgbot.Bot, chatID, userID int64) (bool, string) {
	settings := db.GetChatConnectionSetting(chatID)
	if chat_status.IsUserAdmin(b, chatID, userID) {
		return true, ""
	}

	if settings.AllowConnect {
		member, err := b.GetChatMember(chatID, userID, nil)
		if err != nil {
			log.WithFields(log.Fields{
				"chatId":     chatID,
				"userId":     userID,
				"dbSetting":  settings.AllowConnect,
				"denyReason": "member_lookup_failed",
				"error":      err,
			}).Warn("[Connections] Connection request denied")
			return false, "connections_connect_connection_disabled"
		}

		memberStatus := member.MergeChatMember().Status
		if memberStatus != "left" && memberStatus != "kicked" {
			return true, ""
		}
	}

	log.WithFields(log.Fields{
		"chatId":       chatID,
		"userId":       userID,
		"allowConnect": settings.AllowConnect,
		"denyReason":   "allow_connect_disabled_non_admin",
	}).Warn("[Connections] Connection request denied")
	return false, "connections_connect_connection_disabled"
}
