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

// SendMessageWithErrorHandling wraps bot.SendMessage with graceful error handling for expected permission errors.
// This prevents Sentry spam when the bot lacks send message permissions in a chat.
// Returns (*Message, nil) for suppressed permission errors to allow callers to continue execution.
func SendMessageWithErrorHandling(bot *gotgbot.Bot, chatId int64, text string, opts *gotgbot.SendMessageOpts) (*gotgbot.Message, error) {
	msg, err := bot.SendMessage(chatId, text, opts)
	if err != nil {
		errStr := err.Error()
		// Check for expected permission-related errors
		if strings.Contains(errStr, "not enough rights to send text messages") ||
			strings.Contains(errStr, "have no rights to send a message") ||
			strings.Contains(errStr, "CHAT_WRITE_FORBIDDEN") ||
			strings.Contains(errStr, "CHAT_RESTRICTED") ||
			strings.Contains(errStr, "need administrator rights in the channel chat") {
			log.WithFields(log.Fields{
				"chat_id": chatId,
				"error":   errStr,
			}).Warning("Bot lacks permission to send messages in this chat")
			return nil, nil
		}
		return nil, errors.Wrapf(err, "failed to send message to chat %d", chatId)
	}
	return msg, nil
}

// ShouldSuppressFromSentry checks if an error should be suppressed from Sentry reporting.
// Returns true for expected Telegram API errors that occur during normal bot operations.
func ShouldSuppressFromSentry(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Check for expected Telegram API errors
	expectedErrors := []string{
		// Bot access errors (kicked, banned, or restricted)
		"CHAT_RESTRICTED",
		"bot was kicked from the",
		"bot was blocked by the user",
		"Forbidden: bot was kicked",
		"Forbidden: bot is not a member",

		// Thread/topic errors
		"message thread not found",
		"thread not found",

		// Chat state errors
		"group chat was deactivated",
		"chat not found",
		"group chat was upgraded to a supergroup",
	}

	for _, expectedErr := range expectedErrors {
		if strings.Contains(errStr, expectedErr) {
			return true
		}
	}

	return false
}
