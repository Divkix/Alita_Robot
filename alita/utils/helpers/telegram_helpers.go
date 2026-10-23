package helpers

import (
	"fmt"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/errors"
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

// IsChatPermissionError reports whether err means the bot may not send to the
// chat. It marks the chat restricted so later sends skip it.
func IsChatPermissionError(err error, chatId int64) bool {
	if err == nil || !IsPermissionError(err.Error()) {
		return false
	}
	cache.MarkChatRestricted(chatId)
	return true
}

// IsPermanentSendError reports whether err means a send can never succeed
// until someone changes the chat's state: the bot was removed or is not a
// member, the chat is gone or was deactivated, the group was upgraded, or the
// bot may not send messages there. Retrying the same send cannot fix any of
// them; a network error or a flood wait is not permanent.
func IsPermanentSendError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	if IsPermissionError(errStr) {
		return true
	}

	permanentErrors := []string{
		"bot was kicked",
		"bot is not a member",
		"chat not found",
		"group chat was deactivated",
		"group chat was upgraded to a supergroup",
	}
	for _, permanentErr := range permanentErrors {
		if strings.Contains(errStr, permanentErr) {
			return true
		}
	}

	return false
}

func SendMessageWithErrorHandling(bot *gotgbot.Bot, chatId int64, text string, opts *gotgbot.SendMessageOpts) (*gotgbot.Message, error) {
	restricted, restrictedFlagPresent := cache.IsChatRestrictedState(chatId)
	if restricted {
		log.WithField("chat_id", chatId).Debug("[Helpers] Skipping send to restricted chat")
		return nil, nil
	}
	sentAt := time.Now()
	msg, err := bot.SendMessage(chatId, text, opts)
	if err != nil {
		if IsChatPermissionError(err, chatId) {
			log.WithFields(log.Fields{
				"chat_id": chatId,
				"error":   err.Error(),
			}).Debug("Bot lacks permission to send messages in this chat")
			return nil, nil
		}
		return nil, errors.Wrapf(err, "failed to send message to chat %d", chatId)
	}
	if restrictedFlagPresent {
		cache.MarkChatNotRestrictedIfOlder(chatId, sentAt)
	}
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

// LogHandlerError logs an error a handler returned: Warn for an expected
// Telegram API error, Error otherwise, with the update id, the error type and,
// for a WrappedError, where it was wrapped. The dispatcher's error handler and
// WrapCommand (after a chat swap, when it ends groups instead) both log
// through it, so the two paths produce the same entry.
func LogHandlerError(ctx *ext.Context, err error) {
	updateID := int64(-1)
	if ctx != nil && ctx.UpdateId != 0 {
		updateID = ctx.UpdateId
	}
	fields := log.Fields{
		"update_id":  updateID,
		"error_type": fmt.Sprintf("%T", err),
	}
	if wrappedErr, ok := err.(*errors.WrappedError); ok {
		fields["file"] = wrappedErr.File
		fields["line"] = wrappedErr.Line
		fields["function"] = wrappedErr.Function
	}

	if IsExpectedTelegramError(err) {
		log.WithFields(fields).Warnf("Expected Telegram API error: %v", err)
		return
	}
	log.WithFields(fields).Errorf("Handler error occurred: %v", err)
}
