package helpers

import (
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
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

func IsPermissionError(errStr string) bool {
	return strings.Contains(errStr, "not enough rights to send text messages") ||
		strings.Contains(errStr, "have no rights to send a message") ||
		strings.Contains(errStr, "CHAT_WRITE_FORBIDDEN") ||
		strings.Contains(errStr, "CHAT_RESTRICTED") ||
		strings.Contains(errStr, "need administrator rights in the channel chat")
}

func SendMessageWithErrorHandling(bot *gotgbot.Bot, chatId int64, text string, opts *gotgbot.SendMessageOpts) (*gotgbot.Message, error) {
	if cache.IsChatRestricted(chatId) {
		log.WithField("chat_id", chatId).Debug("[Helpers] Skipping send to restricted chat")
		return nil, nil
	}
	msg, err := bot.SendMessage(chatId, text, opts)
	if err != nil {
		errStr := err.Error()
		if IsPermissionError(errStr) {
			cache.MarkChatRestricted(chatId)
			log.WithFields(log.Fields{
				"chat_id": chatId,
				"error":   errStr,
			}).Warning("Bot lacks permission to send messages in this chat")
			return nil, nil
		}
		return nil, errors.Wrapf(err, "failed to send message to chat %d", chatId)
	}
	cache.MarkChatNotRestricted(chatId)
	return msg, nil
}

func IsExpectedTelegramError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	expectedErrors := []string{
		"CHAT_RESTRICTED",
		"bot was kicked from the",
		"bot was blocked by the user",
		"Forbidden: bot was kicked",
		"Forbidden: bot is not a member",

		"message thread not found",
		"thread not found",

		"group chat was deactivated",
		"chat not found",
		"group chat was upgraded to a supergroup",

		"timeout awaiting response headers",
		"http2: timeout",
		"context deadline exceeded",

		"not enough rights to restrict/unrestrict chat member",
		"not enough rights to send text messages",
		"not enough rights to",
		"bot lacks permission",

		"message can't be deleted",
		"message to delete not found",

		"TOPIC_CLOSED",
	}

	for _, expectedErr := range expectedErrors {
		if strings.Contains(errStr, expectedErr) {
			return true
		}
	}

	return false
}
