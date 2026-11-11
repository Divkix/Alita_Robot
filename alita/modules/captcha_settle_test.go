//go:build testtools

package modules

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
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

// After a pass the chat sees how many messages of which kinds the member sent
// while held, never the messages, and the confirmation.
func TestCaptchaPassSummarisesWhatTheMemberSentWhileHeld(t *testing.T) {
	useRealLocales(t)
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	attempt := pendingCaptchaAttempt(t, uniqueModuleChatID(), 123)
	for _, kind := range []int{db.TEXT, db.PHOTO, db.TEXT} {
		if err := captcha.StoreMessageForCaptcha(42, attempt.ChatID, attempt.ID, kind, "secret words", "", ""); err != nil {
			t.Fatalf("StoreMessageForCaptcha() error = %v", err)
		}
	}

	if err := tapCaptcha(bot, attempt, "7"); err != nil {
		t.Fatalf("captchaVerifyCallback() error = %v", err)
	}
	var texts []string
	for _, call := range client.callsFor("sendMessage") {
		texts = append(texts, fmt.Sprint(call.Params["text"]))
	}
	joined := strings.Join(texts, "\n")
	for _, want := range []string{"tried to send 3 message(s) (text, photo)", "has been verified and can now chat"} {
		if !strings.Contains(joined, want) {
			t.Errorf("sent messages %q, want one containing %q", texts, want)
		}
	}
	if strings.Contains(joined, "secret words") {
		t.Errorf("sent messages %q replay a stored message", texts)
	}
	assertDeleted(t, client, attempt.ChatID, 123)
	assertCaptchaRows(t, attempt, 0)
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
		before := len(bot.BotClient.(*moduleBotClient).callsFor("restrictChatMember"))
		if err := tapCaptcha(bot, attempt, "7"); err != nil {
			t.Fatalf("captchaVerifyCallback() error = %v", err)
		}
		return len(bot.BotClient.(*moduleBotClient).callsFor("restrictChatMember")) > before
	}
	// timeout is the timer firing on a chat that kicks on failure.
	timeout := func(t *testing.T, bot *gotgbot.Bot, attempt *db.CaptchaAttempts) bool {
		t.Helper()
		if err := captcha.SetCaptchaEnabled(attempt.ChatID, "Captcha Chat", true); err != nil {
			t.Fatalf("SetCaptchaEnabled() error = %v", err)
		}
		claimed, err := expireCaptchaAttempt(bot, attempt)
		if err != nil {
			t.Fatalf("expireCaptchaAttempt() error = %v", err)
		}
		return claimed
	}
	type ending func(*testing.T, *gotgbot.Bot, *db.CaptchaAttempts) bool
	for _, tt := range []struct {
		name          string
		first, second ending
	}{
		{"release then release", release, release},
		{"pass then release", pass, release},
		{"release then pass", release, pass},
		{"pass then timeout", pass, timeout},
		{"timeout then pass", timeout, pass},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			attempt := pendingCaptchaAttempt(t, uniqueModuleChatID(), 123)

			if !tt.first(t, bot, attempt) {
				t.Fatal("first ending did not settle the attempt")
			}
			counts := func() map[string]int {
				got := map[string]int{}
				for _, method := range []string{"restrictChatMember", "banChatMember", "unbanChatMember", "deleteMessage"} {
					got[method] = len(client.callsFor(method))
				}
				return got
			}
			before := counts()
			if tt.second(t, bot, attempt) {
				t.Fatal("second ending settled the attempt again")
			}
			if after := counts(); !reflect.DeepEqual(after, before) {
				t.Fatalf("member and prompt calls after second ending = %v, want %v", after, before)
			}
			assertCaptchaRows(t, attempt, 0)
		})
	}
}

// A pass claims only with the right answer on the prompt the member is
// looking at: a wrong answer, an older image or another prompt settles nothing.
func TestSettleCaptchaPassClaimsOnlyTheRightAnswerOnTheCurrentPrompt(t *testing.T) {
	for _, tt := range []struct {
		name         string
		answer       string
		refreshCount int
		promptID     int64
		wantSettled  bool
	}{
		{name: "wrong answer", answer: "8", promptID: 123},
		{name: "older image", answer: "7", refreshCount: 1, promptID: 123},
		{name: "another prompt", answer: "7", promptID: 456},
		{name: "right answer on the current prompt", answer: "7", promptID: 123, wantSettled: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			attempt := pendingCaptchaAttempt(t, uniqueModuleChatID(), 123)
			seen := *attempt
			seen.MessageID = tt.promptID

			settled, err := settleCaptchaAttempt(bot, &seen, captchaPassed(tt.answer, tt.refreshCount, "Member", 3))
			if settled != tt.wantSettled || err != nil {
				t.Fatalf("settleCaptchaAttempt() = (%v, %v), want (%v, nil)", settled, err, tt.wantSettled)
			}
			if tt.wantSettled {
				client.onlyCall(t, "restrictChatMember")
				assertDeleted(t, client, attempt.ChatID, 123)
				assertCaptchaRows(t, attempt, 0)
				return
			}
			for _, method := range []string{"restrictChatMember", "deleteMessage"} {
				if calls := client.callsFor(method); len(calls) != 0 {
					t.Fatalf("%s calls = %d, want none when the claim fails", method, len(calls))
				}
			}
			if current, err := captcha.GetCaptchaAttemptByID(attempt.ID); err != nil || current == nil {
				t.Fatalf("attempt after a failed claim = (%v, %v), want it kept", current, err)
			}
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
