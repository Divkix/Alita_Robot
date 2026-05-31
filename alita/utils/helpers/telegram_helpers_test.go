package helpers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/divkix/Alita_Robot/alita/utils/media"
)

type telegramHelperBotClient struct {
	errors map[string]error
	calls  map[string]int
}

func newTelegramHelperBotClient() *telegramHelperBotClient {
	return &telegramHelperBotClient{
		errors: make(map[string]error),
		calls:  make(map[string]int),
	}
}

func (c *telegramHelperBotClient) RequestWithContext(_ context.Context, _ string, method string, _ map[string]any, _ *gotgbot.RequestOpts) (json.RawMessage, error) {
	c.calls[method]++
	if err := c.errors[method]; err != nil {
		return nil, err
	}
	switch method {
	case "sendMessage":
		return json.RawMessage(`{"message_id":1,"date":1,"chat":{"id":-1001,"type":"supergroup","title":"Helpers"}}`), nil
	default:
		return json.RawMessage(`true`), nil
	}
}

func (c *telegramHelperBotClient) GetAPIURL(*gotgbot.RequestOpts) string {
	return gotgbot.DefaultAPIURL
}

func (c *telegramHelperBotClient) FileURL(token string, path string, _ *gotgbot.RequestOpts) string {
	return gotgbot.DefaultAPIURL + "/file/bot" + token + "/" + path
}

func newTelegramHelperBot(client *telegramHelperBotClient) *gotgbot.Bot {
	return &gotgbot.Bot{
		Token:     "999:test",
		BotClient: client,
		User:      gotgbot.User{Id: 999, IsBot: true, Username: "HelperBot"},
	}
}

func TestDeleteMessageWithErrorHandlingSuppressesExpectedTelegramErrors(t *testing.T) {
	for _, errText := range []string{
		"Bad Request: message to delete not found",
		"Bad Request: message can't be deleted",
	} {
		t.Run(errText, func(t *testing.T) {
			client := newTelegramHelperBotClient()
			client.errors["deleteMessage"] = fmt.Errorf("%s", errText)
			bot := newTelegramHelperBot(client)

			if err := DeleteMessageWithErrorHandling(bot, -1001, 55); err != nil {
				t.Fatalf("DeleteMessageWithErrorHandling() error = %v, want nil", err)
			}
			if client.calls["deleteMessage"] != 1 {
				t.Fatalf("deleteMessage calls = %d, want 1", client.calls["deleteMessage"])
			}
		})
	}
}

func TestDeleteMessageWithErrorHandlingWrapsUnexpectedError(t *testing.T) {
	client := newTelegramHelperBotClient()
	client.errors["deleteMessage"] = fmt.Errorf("Internal Server Error")
	bot := newTelegramHelperBot(client)

	err := DeleteMessageWithErrorHandling(bot, -1001, 55)
	if err == nil {
		t.Fatal("DeleteMessageWithErrorHandling() error = nil, want wrapped error")
	}
	if !strings.Contains(err.Error(), "failed to delete message 55 in chat -1001") {
		t.Fatalf("DeleteMessageWithErrorHandling() error = %v, want context", err)
	}
}

func TestSendMessageWithErrorHandlingSuppressesPermissionErrors(t *testing.T) {
	for _, errText := range []string{
		"Forbidden: not enough rights to send text messages",
		"Forbidden: have no rights to send a message",
		"Bad Request: CHAT_WRITE_FORBIDDEN",
		"Bad Request: CHAT_RESTRICTED",
		"Bad Request: need administrator rights in the channel chat",
	} {
		t.Run(errText, func(t *testing.T) {
			client := newTelegramHelperBotClient()
			client.errors["sendMessage"] = fmt.Errorf("%s", errText)
			bot := newTelegramHelperBot(client)

			msg, err := SendMessageWithErrorHandling(bot, -1001, "hello", nil)
			if err != nil {
				t.Fatalf("SendMessageWithErrorHandling() error = %v, want nil", err)
			}
			if msg != nil {
				t.Fatalf("SendMessageWithErrorHandling() msg = %#v, want nil for suppressed error", msg)
			}
		})
	}
}

func TestSendMessageWithErrorHandlingWrapsUnexpectedError(t *testing.T) {
	client := newTelegramHelperBotClient()
	client.errors["sendMessage"] = fmt.Errorf("Internal Server Error")
	bot := newTelegramHelperBot(client)

	msg, err := SendMessageWithErrorHandling(bot, -1001, "hello", nil)
	if msg != nil {
		t.Fatalf("SendMessageWithErrorHandling() msg = %#v, want nil on error", msg)
	}
	if err == nil {
		t.Fatal("SendMessageWithErrorHandling() error = nil, want wrapped error")
	}
	if !strings.Contains(err.Error(), "failed to send message to chat -1001") {
		t.Fatalf("SendMessageWithErrorHandling() error = %v, want context", err)
	}
}

func TestSendMessageWithErrorHandlingReturnsSentMessage(t *testing.T) {
	client := newTelegramHelperBotClient()
	bot := newTelegramHelperBot(client)

	msg, err := SendMessageWithErrorHandling(bot, -1001, "hello", nil)
	if err != nil {
		t.Fatalf("SendMessageWithErrorHandling() error = %v, want nil", err)
	}
	if msg == nil || msg.MessageId != 1 {
		t.Fatalf("SendMessageWithErrorHandling() msg = %#v, want sent message", msg)
	}
}

func TestIsExpectedTelegramErrorClassifiesNilExpectedAndUnexpected(t *testing.T) {
	if IsExpectedTelegramError(nil) {
		t.Fatal("IsExpectedTelegramError(nil) = true, want false")
	}
	for _, errText := range []string{
		"Forbidden: bot was kicked",
		"message thread not found",
		"group chat was deactivated",
		"context deadline exceeded",
		"not enough rights to restrict/unrestrict chat member",
		"message to delete not found",
		"TOPIC_CLOSED",
	} {
		if !IsExpectedTelegramError(fmt.Errorf("%s", errText)) {
			t.Fatalf("IsExpectedTelegramError(%q) = false, want true", errText)
		}
	}
	if IsExpectedTelegramError(fmt.Errorf("database connection failed")) {
		t.Fatal("IsExpectedTelegramError(unexpected) = true, want false")
	}
}

func TestIsExpectedTelegramErrorKnown(t *testing.T) {
	t.Parallel()

	knownErrors := []string{
		"bot was kicked from the group",
		"bot was blocked by the user",
		"chat not found",
		"message can't be deleted",
		"message to delete not found",
		"group chat was deactivated",
		"not enough rights to restrict/unrestrict chat member",
		"context deadline exceeded",
		"message thread not found",
	}
	for _, msg := range knownErrors {
		t.Run(msg, func(t *testing.T) {
			t.Parallel()
			err := fmt.Errorf("%s", msg)
			if !IsExpectedTelegramError(err) {
				t.Fatalf("IsExpectedTelegramError(%q) expected true", msg)
			}
		})
	}
}

func TestIsExpectedTelegramErrorUnknown(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("some unknown telegram error xyz")
	if IsExpectedTelegramError(err) {
		t.Fatalf("IsExpectedTelegramError for unknown error expected false")
	}
}

func TestIsExpectedTelegramErrorAllStrings(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		errMsg string
	}{
		{"CHAT_RESTRICTED", "CHAT_RESTRICTED"},
		{"bot was kicked from the", "bot was kicked from the group"},
		{"bot was blocked by the user", "bot was blocked by the user"},
		{"Forbidden: bot was kicked", "Forbidden: bot was kicked"},
		{"Forbidden: bot is not a member", "Forbidden: bot is not a member"},
		{"message thread not found", "message thread not found"},
		{"thread not found", "thread not found"},
		{"group chat was deactivated", "group chat was deactivated"},
		{"chat not found", "chat not found"},
		{"group chat was upgraded to a supergroup", "group chat was upgraded to a supergroup"},
		{"timeout awaiting response headers", "timeout awaiting response headers"},
		{"http2: timeout", "http2: timeout"},
		{"context deadline exceeded", "context deadline exceeded"},
		{"not enough rights to restrict/unrestrict chat member", "not enough rights to restrict/unrestrict chat member"},
		{"not enough rights to send text messages", "not enough rights to send text messages"},
		{"not enough rights to", "not enough rights to pin"},
		{"message can't be deleted", "message can't be deleted"},
		{"message to delete not found", "message to delete not found"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := errors.New(tc.errMsg)
			if !IsExpectedTelegramError(err) {
				t.Fatalf("IsExpectedTelegramError(%q) expected true", tc.errMsg)
			}
		})
	}
}

func TestIsExpectedTelegramErrorSubstring(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("failed: bot was kicked from the group chat")
	if !IsExpectedTelegramError(err) {
		t.Fatalf("IsExpectedTelegramError with extra context expected true, got false")
	}
}

func TestIsExpectedTelegramErrorWrapped(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("wrap: %w", errors.New("chat not found"))
	if !IsExpectedTelegramError(err) {
		t.Fatalf("IsExpectedTelegramError with wrapped error expected true, got false")
	}
}

func TestIsExpectedTelegramErrorEmptyError(t *testing.T) {
	t.Parallel()

	err := errors.New("")
	if IsExpectedTelegramError(err) {
		t.Fatalf("IsExpectedTelegramError(\"\") expected false")
	}
}

func TestIsPermissionError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		errStr   string
		expected bool
	}{
		{"not enough rights to send text messages", true},
		{"have no rights to send a message", true},
		{"Bad Request: CHAT_WRITE_FORBIDDEN", true},
		{"Forbidden: CHAT_RESTRICTED", true},
		{"need administrator rights in the channel chat", true},
		{"some other error", false},
		{"Bad Request: message is not modified", false},
		{"", false},
	}

	for _, tc := range cases {
		t.Run(tc.errStr, func(t *testing.T) {
			t.Parallel()
			got := IsPermissionError(tc.errStr)
			if got != tc.expected {
				t.Errorf("IsPermissionError(%q) = %v, want %v", tc.errStr, got, tc.expected)
			}
		})
	}
}

// TestIsExpectedTelegramError_ErrNoPermission verifies that the ErrNoPermission
// sentinel value from the media package is classified as an expected Telegram error
// so the dispatcher logs it at Warn instead of Error.
func TestIsExpectedTelegramError_ErrNoPermission(t *testing.T) {
	t.Parallel()

	if !IsExpectedTelegramError(media.ErrNoPermission) {
		t.Fatalf("IsExpectedTelegramError(media.ErrNoPermission) expected true (ErrNoPermission should be suppressed); got false for %q", media.ErrNoPermission.Error())
	}
}
