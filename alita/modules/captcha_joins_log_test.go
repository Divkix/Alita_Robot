package modules

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/divkix/Alita_Robot/alita/db/captcha"
	"github.com/divkix/Alita_Robot/alita/db/logchannels"
)

const joinsLogChannel = int64(-1009876543214)

// captchaJoinsFixture is a chat with a log channel and a captcha pending for
// member 42 whose answer is 7; joinsOn turns the opt-in joins category on.
func captchaJoinsFixture(t *testing.T, joinsOn bool) (*moduleBotClient, *gotgbot.Bot, gotgbot.Chat, uint) {
	t.Helper()
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Joins Log Chat"}
	if err := logchannels.Set(chat.Id, chat.Title, joinsLogChannel); err != nil {
		t.Fatalf("logchannels.Set() error = %v", err)
	}
	if joinsOn {
		if err := logchannels.SetCategory(chat.Id, logchannels.CategoryJoins, true); err != nil {
			t.Fatalf("SetCategory(joins) error = %v", err)
		}
	}
	if err := captcha.SetCaptchaMaxAttempts(chat.Id, 3); err != nil {
		t.Fatalf("SetCaptchaMaxAttempts() error = %v", err)
	}
	attempt, err := captcha.CreateCaptchaAttemptPreMessage(42, chat.Id, "7", 2)
	if err != nil {
		t.Fatalf("CreateCaptchaAttemptPreMessage() error = %v", err)
	}
	return client, bot, chat, attempt.ID
}

// answerCaptcha presses the captcha button for answer as member 42.
func answerCaptcha(bot *gotgbot.Bot, chat gotgbot.Chat, attemptID uint, answer string) {
	member := gotgbot.User{Id: 42, FirstName: "Member"}
	data := encodeCallbackData("captcha_verify", map[string]string{"a": fmt.Sprint(attemptID), "u": "42", "s": answer})
	_ = captchaModule.captchaVerifyCallback(bot, newModuleCallbackContext(bot, chat, member, data))
}

func TestCaptchaPassLogsUnderTheJoinsCategory(t *testing.T) {
	requireEntry := func(t *testing.T, client *moduleBotClient, want ...string) {
		t.Helper()
		entries := spamDBMessagesTo(client, joinsLogChannel)
		if len(entries) != 1 {
			t.Fatalf("log channel entries = %d, want 1: %v", len(entries), entries)
		}
		for _, w := range append([]string{"Joins Log Chat", "#CAPTCHA_PASSED\n", "<code>42</code>"}, want...) {
			if !strings.Contains(entries[0], w) {
				t.Fatalf("log entry %q does not contain %q", entries[0], w)
			}
		}
	}
	requireNone := func(t *testing.T, client *moduleBotClient) {
		t.Helper()
		if entries := spamDBMessagesTo(client, joinsLogChannel); len(entries) != 0 {
			t.Fatalf("log channel entries = %d, want none: %v", len(entries), entries)
		}
	}

	t.Run("logged", func(t *testing.T) {
		client, bot, chat, attemptID := captchaJoinsFixture(t, true)
		answerCaptcha(bot, chat, attemptID, "7")
		requireEntry(t, client, "Attempts: 1/3")
	})

	t.Run("logged after a wrong answer", func(t *testing.T) {
		client, bot, chat, attemptID := captchaJoinsFixture(t, true)
		answerCaptcha(bot, chat, attemptID, "8")
		requireNone(t, client)
		answerCaptcha(bot, chat, attemptID, "7")
		requireEntry(t, client, "Attempts: 2/3")
	})

	t.Run("category off by default", func(t *testing.T) {
		client, bot, chat, attemptID := captchaJoinsFixture(t, false)
		answerCaptcha(bot, chat, attemptID, "7")
		requireNone(t, client)
	})

	// A member the bot could not unmute has not passed.
	t.Run("unmute failed", func(t *testing.T) {
		client, bot, chat, attemptID := captchaJoinsFixture(t, true)
		client.errors["restrictChatMember"] = errors.New("Bad Request: not enough rights")
		answerCaptcha(bot, chat, attemptID, "7")
		requireNone(t, client)
	})
}
