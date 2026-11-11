//go:build testtools

package modules

import (
	"errors"
	"fmt"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/approvals"
	"github.com/divkix/Alita_Robot/alita/db/captcha"
)

// pendingCaptchaAttempt stores an unexpired attempt for member 42 whose
// prompt is promptID (0: the prompt was never sent).
func pendingCaptchaAttempt(t *testing.T, chatID, promptID int64) *db.CaptchaAttempts {
	t.Helper()
	attempt, err := captcha.CreateCaptchaAttemptPreMessage(42, chatID, "7", 2)
	if err != nil {
		t.Fatalf("CreateCaptchaAttemptPreMessage() error = %v", err)
	}
	if promptID > 0 {
		if err := captcha.UpdateCaptchaAttemptMessageID(attempt.ID, promptID); err != nil {
			t.Fatalf("UpdateCaptchaAttemptMessageID() error = %v", err)
		}
		attempt.MessageID = promptID
	}
	t.Cleanup(func() {
		_ = captcha.DeleteCaptchaAttempt(42, chatID)
		_ = captcha.DeleteMutedUser(42, chatID)
	})
	return attempt
}

// assertCaptchaRows fails unless the attempt is gone and the chat has
// wantScheduled scheduled unmutes.
func assertCaptchaRows(t *testing.T, attempt *db.CaptchaAttempts, wantScheduled int) {
	t.Helper()
	if current, err := captcha.GetCaptchaAttemptByID(attempt.ID); err != nil || current != nil {
		t.Fatalf("attempt after settle = %#v, %v; want nil, nil", current, err)
	}
	users, err := captcha.GetMutedUsersForChat(attempt.ChatID)
	if err != nil || len(users) != wantScheduled {
		t.Fatalf("scheduled unmutes = (%+v, %v), want %d", users, err, wantScheduled)
	}
}

// tapCaptcha taps answer on attempt's prompt as member 42.
func tapCaptcha(bot *gotgbot.Bot, attempt *db.CaptchaAttempts, answer string) error {
	chat := gotgbot.Chat{Id: attempt.ChatID, Type: "supergroup", Title: "Captcha Chat"}
	member := gotgbot.User{Id: 42, FirstName: "Member"}
	data := encodeCallbackData("captcha_verify", map[string]string{"a": fmt.Sprint(attempt.ID), "u": "42", "s": answer})
	return captchaModule.captchaVerifyCallback(bot, newModuleCallbackContext(bot, chat, member, data))
}

// A pass lifts the mute by the same rule as a release; only an error retrying
// can fix tells the member the verification failed.
func TestCaptchaPassLiftsTheMuteByTheFailureRule(t *testing.T) {
	useRealLocales(t)
	for _, tt := range []struct {
		name          string
		unmuteErr     error
		wantErr       bool
		wantAnswer    string
		wantScheduled int
	}{
		{name: "unmute succeeds", wantAnswer: "✅ Verified successfully!"},
		{name: "member gone", unmuteErr: errors.New("Bad Request: USER_NOT_PARTICIPANT"), wantAnswer: "✅ Verified successfully!"},
		{name: "member is an admin", unmuteErr: errors.New("Bad Request: user is an administrator of the chat"), wantAnswer: "✅ Verified successfully!"},
		{name: "temporary failure", unmuteErr: errors.New("temporary Telegram failure"), wantErr: true, wantAnswer: "Failed to verify. Please contact an admin.", wantScheduled: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := newModuleBotClient()
			if tt.unmuteErr != nil {
				client.errors["restrictChatMember"] = tt.unmuteErr
			}
			bot := newModuleTestBot(client)
			attempt := pendingCaptchaAttempt(t, uniqueModuleChatID(), 123)

			if err := tapCaptcha(bot, attempt, "7"); (err != nil) != tt.wantErr {
				t.Fatalf("captchaVerifyCallback() error = %v, want error %t", err, tt.wantErr)
			}
			assertParams(t, client.onlyCall(t, "restrictChatMember"), targetParams(attempt.ChatID, 42))
			assertParams(t, client.onlyCall(t, "answerCallbackQuery"), map[string]any{"text": tt.wantAnswer})
			assertDeleted(t, client, attempt.ChatID, 123)
			assertCaptchaRows(t, attempt, tt.wantScheduled)
		})
	}
}

func TestSettleCaptchaReleaseLiftsTheMuteByTheFailureRule(t *testing.T) {
	temporary := errors.New("temporary Telegram failure")
	for _, tt := range []struct {
		name          string
		unmuteErr     error
		wantErr       error
		wantScheduled int
	}{
		{name: "unmute succeeds"},
		{name: "member gone", unmuteErr: errors.New("Bad Request: USER_NOT_PARTICIPANT")},
		{name: "chat gone", unmuteErr: errors.New("Forbidden: the group chat was deactivated")},
		{name: "member is an admin", unmuteErr: errors.New("Bad Request: user is an administrator of the chat")},
		{name: "temporary failure", unmuteErr: temporary, wantErr: temporary, wantScheduled: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := newModuleBotClient()
			if tt.unmuteErr != nil {
				client.errors["restrictChatMember"] = tt.unmuteErr
			}
			bot := newModuleTestBot(client)
			attempt := pendingCaptchaAttempt(t, uniqueModuleChatID(), 123)

			settled, err := settleCaptchaAttempt(bot, attempt, captchaReleased)
			if !settled || !errors.Is(err, tt.wantErr) || (tt.wantErr == nil && err != nil) {
				t.Fatalf("settleCaptchaAttempt() = (%v, %v), want (true, %v)", settled, err, tt.wantErr)
			}
			assertParams(t, client.onlyCall(t, "restrictChatMember"), targetParams(attempt.ChatID, 42))
			assertDeleted(t, client, attempt.ChatID, 123)
			assertCaptchaRows(t, attempt, tt.wantScheduled)
		})
	}
}

func TestSettleCaptchaReleaseWithPromptAlreadyRemoved(t *testing.T) {
	client := newModuleBotClient()
	client.errors["deleteMessage"] = errors.New("Bad Request: message to delete not found")
	bot := newModuleTestBot(client)
	attempt := pendingCaptchaAttempt(t, uniqueModuleChatID(), 123)

	if settled, err := settleCaptchaAttempt(bot, attempt, captchaReleased); !settled || err != nil {
		t.Fatalf("settleCaptchaAttempt() = (%v, %v), want (true, nil)", settled, err)
	}
	client.onlyCall(t, "restrictChatMember")
	assertCaptchaRows(t, attempt, 0)
}

func TestSettleCaptchaReleaseOfAnAttemptWhosePromptWasNeverSent(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	attempt := pendingCaptchaAttempt(t, uniqueModuleChatID(), 0)

	if settled, err := settleCaptchaAttempt(bot, attempt, captchaReleased); !settled || err != nil {
		t.Fatalf("settleCaptchaAttempt() = (%v, %v), want (true, nil)", settled, err)
	}
	client.onlyCall(t, "restrictChatMember")
	if calls := client.callsFor("deleteMessage"); len(calls) != 0 {
		t.Fatalf("deleteMessage calls = %d, want none for a prompt never sent", len(calls))
	}
	assertCaptchaRows(t, attempt, 0)
}

func TestSettleCaptchaSecondEndingDoesNothing(t *testing.T) {
	release := func(t *testing.T, bot *gotgbot.Bot, attempt *db.CaptchaAttempts) bool {
		t.Helper()
		settled, err := settleCaptchaAttempt(bot, attempt, captchaReleased)
		if err != nil {
			t.Fatalf("settleCaptchaAttempt() error = %v", err)
		}
		return settled
	}
	pass := func(t *testing.T, bot *gotgbot.Bot, attempt *db.CaptchaAttempts) bool {
		t.Helper()
		chat := gotgbot.Chat{Id: attempt.ChatID, Type: "supergroup", Title: "Captcha Chat"}
		member := gotgbot.User{Id: 42, FirstName: "Member"}
		data := encodeCallbackData("captcha_verify", map[string]string{"a": fmt.Sprint(attempt.ID), "u": "42", "s": "7"})
		before := len(bot.BotClient.(*moduleBotClient).callsFor("restrictChatMember"))
		if err := captchaModule.captchaVerifyCallback(bot, newModuleCallbackContext(bot, chat, member, data)); err != nil {
			t.Fatalf("captchaVerifyCallback() error = %v", err)
		}
		return len(bot.BotClient.(*moduleBotClient).callsFor("restrictChatMember")) > before
	}
	type ending func(*testing.T, *gotgbot.Bot, *db.CaptchaAttempts) bool
	for _, tt := range []struct {
		name          string
		first, second ending
	}{
		{"release then release", release, release},
		{"pass then release", pass, release},
		{"release then pass", release, pass},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			attempt := pendingCaptchaAttempt(t, uniqueModuleChatID(), 123)

			if !tt.first(t, bot, attempt) {
				t.Fatal("first ending did not settle the attempt")
			}
			restricts := len(client.callsFor("restrictChatMember"))
			deletes := len(client.callsFor("deleteMessage"))
			if tt.second(t, bot, attempt) {
				t.Fatal("second ending settled the attempt again")
			}
			if got := len(client.callsFor("restrictChatMember")); got != restricts {
				t.Fatalf("restrictChatMember calls after second ending = %d, want %d", got, restricts)
			}
			if got := len(client.callsFor("deleteMessage")); got != deletes {
				t.Fatalf("deleteMessage calls after second ending = %d, want %d", got, deletes)
			}
			assertCaptchaRows(t, attempt, 0)
		})
	}
}

func TestExpiredCaptchaEndsAsReleaseAfterApproveOrCaptchaOff(t *testing.T) {
	for _, approved := range []bool{true, false} {
		t.Run(fmt.Sprintf("approved=%t", approved), func(t *testing.T) {
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			chatID := uniqueModuleChatID()
			if approved {
				if err := captcha.SetCaptchaEnabled(chatID, "Captcha Chat", true); err != nil {
					t.Fatalf("SetCaptchaEnabled() error = %v", err)
				}
				if err := approvals.AddApprovedUser(chatID, 42, 777000, "trusted"); err != nil {
					t.Fatalf("AddApprovedUser() error = %v", err)
				}
				t.Cleanup(func() { _ = approvals.RemoveApprovedUser(chatID, 42) })
			}
			attempt := pendingCaptchaAttempt(t, chatID, 123)

			if handled, err := expireCaptchaAttempt(bot, attempt); !handled || err != nil {
				t.Fatalf("expireCaptchaAttempt() = (%v, %v), want (true, nil)", handled, err)
			}
			for _, method := range []string{"banChatMember", "unbanChatMember", "sendMessage"} {
				if calls := client.callsFor(method); len(calls) != 0 {
					t.Fatalf("%s calls = %d, want none for a release", method, len(calls))
				}
			}
			client.onlyCall(t, "restrictChatMember")
			assertDeleted(t, client, chatID, 123)
			assertCaptchaRows(t, attempt, 0)
		})
	}
}

// A newcomer who is gone by the time the undelivered challenge is rolled back
// leaves nothing to unmute: they are welcomed and no unmute is scheduled.
func TestUndeliveredCaptchaForAGoneMemberSchedulesNoUnmute(t *testing.T) {
	useRealLocales(t)
	client := newModuleBotClient()
	client.errors["sendPhoto"] = errors.New("http2: timeout awaiting response headers")
	client.errors["getChat"] = errors.New("Forbidden: the group chat was deactivated")
	bot := newModuleTestBot(client)
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Captcha Chat"}
	if err := captcha.SetCaptchaEnabled(chat.Id, chat.Title, true); err != nil {
		t.Fatalf("SetCaptchaEnabled() error = %v", err)
	}
	t.Cleanup(func() { _ = captcha.DeleteMutedUser(42, chat.Id) })

	err := SendCaptcha(bot, newModuleMessageContext(bot, chat, gotgbot.User{Id: 777000}, "join"), 42, "Member")
	if !errors.Is(err, errCaptchaDeliverySkipped) {
		t.Fatalf("SendCaptcha() error = %v, want the challenge skipped so the member is welcomed", err)
	}
	if attempt, getErr := captcha.GetCaptchaAttempt(42, chat.Id); getErr != nil || attempt != nil {
		t.Fatalf("GetCaptchaAttempt() = (%+v, %v), want no attempt", attempt, getErr)
	}
	if users, getErr := captcha.GetMutedUsersForChat(chat.Id); getErr != nil || len(users) != 0 {
		t.Fatalf("scheduled unmutes = (%+v, %v), want none for a member who is gone", users, getErr)
	}
}
