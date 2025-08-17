package helpers

import (
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/divkix/Alita_Robot/alita/utils/errors"
	log "github.com/sirupsen/logrus"
)

func DeleteMessageWithErrorHandling(bot *gotgbot.Bot, chatId, messageId int64) error {
	_, err := bot.DeleteMessage(chatId, messageId, nil)
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "message to delete not found") ||
			strings.Contains(errStr, "message can't be deleted") {
			log.WithFields(log.Fields{
				"chat_id":    chatId,
				"message_id": messageId,
				"error":      errStr,
			}).Debug("Message already deleted or can't be deleted")
			return nil
		}
		return errors.Wrapf(err, "failed to delete message %d in chat %d", messageId, chatId)
	}
	return nil
}
