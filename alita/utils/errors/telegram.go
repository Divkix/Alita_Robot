package errors

import "strings"

// IsExpectedTelegramError reports whether err is a known Telegram failure that should be logged at Warn.
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
