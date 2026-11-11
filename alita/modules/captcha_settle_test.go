//go:build testtools

package modules

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/approvals"
	"github.com/divkix/Alita_Robot/alita/db/captcha"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/i18n"
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
		settled, err := settleCaptchaAttempt(bot, attempt, captchaFailed(captchaFailedTimeout, "kick"))
		if err != nil {
			t.Fatalf("settleCaptchaAttempt() error = %v", err)
		}
		return settled
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
		{"timeout then timeout", timeout, timeout},
		{"release then timeout", release, timeout},
		{"timeout then release", timeout, release},
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

// memberCalls counts the calls that act on a member.
func memberCalls(client *moduleBotClient) map[string]int {
	got := map[string]int{}
	for _, method := range []string{"unbanChatMember", "banChatMember", "restrictChatMember"} {
		got[method] = len(client.callsFor(method))
	}
	return got
}

// A newcomer who does not answer in time gets the chat's failure action
// once, the group a notice and a ban its record; a 24-hour mute keeps its
// scheduled unmute, which is its timer.
func TestCaptchaTimeoutAppliesTheChatsFailureActionOnce(t *testing.T) {
	for _, tt := range []struct{ action, method string }{
		{"kick", "unbanChatMember"},
		{"ban", "banChatMember"},
		{"mute", "restrictChatMember"},
	} {
		t.Run(tt.action, func(t *testing.T) {
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			attempt := pendingCaptchaAttempt(t, uniqueModuleChatID(), 123)

			settled, err := settleCaptchaAttempt(bot, attempt, captchaFailed(captchaFailedTimeout, tt.action))
			if !settled || err != nil {
				t.Fatalf("settleCaptchaAttempt() = (%v, %v), want (true, nil)", settled, err)
			}
			want := map[string]int{"unbanChatMember": 0, "banChatMember": 0, "restrictChatMember": 0, tt.method: 1}
			if got := memberCalls(client); !reflect.DeepEqual(got, want) {
				t.Fatalf("member calls = %v, want %v", got, want)
			}
			assertParams(t, client.onlyCall(t, tt.method), targetParams(attempt.ChatID, 42))
			assertParams(t, client.onlyCall(t, "sendMessage"), map[string]any{"chat_id": attempt.ChatID})
			assertDeleted(t, client, attempt.ChatID, 123)

			records := banRecordsOf(t, attempt.ChatID)
			if tt.action == "ban" {
				if len(records) != 1 || records[0].UserId != 42 || records[0].Source != models.BanSourceCaptcha ||
					records[0].SourceDetail != captchaFailedTimeout || records[0].BannedBy != 0 || records[0].Until != nil {
					t.Fatalf("ban records = %+v, want one permanent captcha ban by the bot", records)
				}
			} else if len(records) != 0 {
				t.Fatalf("ban records = %+v, want none for %s", records, tt.action)
			}
			if tt.action != "mute" {
				assertCaptchaRows(t, attempt, 0)
				return
			}
			// The 24h end is captcha's own schedule; Telegram gets no until_date.
			if until := client.onlyCall(t, "restrictChatMember").Params["until_date"]; until != nil {
				t.Fatalf("failure mute until_date = %v, want none", until)
			}
			assertCaptchaRows(t, attempt, 1)
			users, _ := captcha.GetMutedUsersForChat(attempt.ChatID)
			if left := time.Until(users[0].UnmuteAt); left < 23*time.Hour || left > 24*time.Hour {
				t.Fatalf("scheduled unmute in %v, want 24h", left)
			}
		})
	}
}

// When the failure action fails the mute is lifted as for a release, and
// nothing claims the action happened.
func TestCaptchaFailureActionThatFailsLiftsTheMuteAsForARelease(t *testing.T) {
	temporary := errors.New("temporary Telegram failure")
	for _, tt := range []struct {
		name, action, method string
		actionErr            error
		// The mute and the unmute are both restrictChatMember calls.
		wantMember    map[string]int
		wantScheduled int
	}{
		{name: "kick", action: "kick", method: "unbanChatMember", actionErr: temporary,
			wantMember: map[string]int{"unbanChatMember": 2, "banChatMember": 0, "restrictChatMember": 1}},
		{name: "ban", action: "ban", method: "banChatMember", actionErr: temporary,
			wantMember: map[string]int{"unbanChatMember": 0, "banChatMember": 1, "restrictChatMember": 1}},
		{name: "mute, unmute fails too", action: "mute", method: "restrictChatMember", actionErr: temporary,
			wantMember: map[string]int{"unbanChatMember": 0, "banChatMember": 0, "restrictChatMember": 2}, wantScheduled: 1},
		{name: "mute, member gone meanwhile", action: "mute", method: "restrictChatMember", actionErr: errors.New("Bad Request: USER_NOT_PARTICIPANT"),
			wantMember: map[string]int{"unbanChatMember": 0, "banChatMember": 0, "restrictChatMember": 2}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := newModuleBotClient()
			client.errors[tt.method] = tt.actionErr
			bot := newModuleTestBot(client)
			attempt := pendingCaptchaAttempt(t, uniqueModuleChatID(), 123)

			settled, err := settleCaptchaAttempt(bot, attempt, captchaFailed(captchaFailedTimeout, tt.action))
			if !settled || !errors.Is(err, tt.actionErr) {
				t.Fatalf("settleCaptchaAttempt() = (%v, %v), want (true, the action's error)", settled, err)
			}
			if got := memberCalls(client); !reflect.DeepEqual(got, tt.wantMember) {
				t.Fatalf("member calls = %v, want %v", got, tt.wantMember)
			}
			if calls := client.callsFor("sendMessage"); len(calls) != 0 {
				t.Fatalf("sendMessage calls = %d, want no notice of an action that failed", len(calls))
			}
			if records := banRecordsOf(t, attempt.ChatID); len(records) != 0 {
				t.Fatalf("ban records = %+v, want none", records)
			}
			assertDeleted(t, client, attempt.ChatID, 123)
			assertCaptchaRows(t, attempt, tt.wantScheduled)
			if tt.wantScheduled == 1 {
				users, _ := captcha.GetMutedUsersForChat(attempt.ChatID)
				if time.Until(users[0].UnmuteAt) > time.Minute {
					t.Fatalf("scheduled unmute at %v, want the release's, not the 24h mute's", users[0].UnmuteAt)
				}
			}
		})
	}
}

// A newcomer who left before the timer fired, or whom someone else banned,
// gets no action, and that ban is not lifted.
func TestCaptchaFailureLeavesAGoneOrBannedMemberAlone(t *testing.T) {
	for _, state := range []string{"left", "kicked", "missing"} {
		for _, action := range []string{"kick", "ban", "mute"} {
			t.Run(state+"/"+action, func(t *testing.T) {
				client := newModuleBotClient()
				if state == "missing" {
					client.errors["getChatMember"] = &gotgbot.TelegramError{Method: "getChatMember", Code: 400, Description: "Bad Request: member not found"}
				} else {
					client.membersByUser = map[string]json.RawMessage{
						"42": json.RawMessage(fmt.Sprintf(`{"status":%q,"until_date":0,"user":{"id":42,"is_bot":false,"first_name":"Member"}}`, state)),
					}
				}
				bot := newModuleTestBot(client)
				attempt := pendingCaptchaAttempt(t, uniqueModuleChatID(), 123)

				settled, err := settleCaptchaAttempt(bot, attempt, captchaFailed(captchaFailedTimeout, action))
				if !settled || err != nil {
					t.Fatalf("settleCaptchaAttempt() = (%v, %v), want (true, nil)", settled, err)
				}
				if got, want := memberCalls(client), (map[string]int{"unbanChatMember": 0, "banChatMember": 0, "restrictChatMember": 0}); !reflect.DeepEqual(got, want) {
					t.Fatalf("member calls = %v, want none", got)
				}
				if calls := client.callsFor("sendMessage"); len(calls) != 0 {
					t.Fatalf("sendMessage calls = %d, want no notice", len(calls))
				}
				assertDeleted(t, client, attempt.ChatID, 123)
				assertCaptchaRows(t, attempt, 0)
			})
		}
	}
}

// A newcomer muted as the failure action is unmuted by the unmute worker
// once the 24 hours are up.
func TestCaptchaFailureMuteIsLiftedAfter24Hours(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	attempt := pendingCaptchaAttempt(t, uniqueModuleChatID(), 123)
	if settled, err := settleCaptchaAttempt(bot, attempt, captchaFailed(captchaFailedTimeout, "mute")); !settled || err != nil {
		t.Fatalf("settleCaptchaAttempt() = (%v, %v), want (true, nil)", settled, err)
	}
	unmuteExpiredCaptchaUsers(bot)
	// The worker also lifts other chats' due unmutes; count this chat's.
	if calls := callsToChat(client, "restrictChatMember", attempt.ChatID); len(calls) != 1 {
		t.Fatalf("restrictChatMember calls = %d, want the mute only before 24h", len(calls))
	}

	// 24 hours pass.
	if err := db.DB.Model(&models.CaptchaMutedUsers{}).Where("chat_id = ?", attempt.ChatID).
		Update("unmute_at", time.Now().Add(-time.Minute)).Error; err != nil {
		t.Fatalf("move unmute_at error = %v", err)
	}
	unmuteExpiredCaptchaUsers(bot)
	calls := callsToChat(client, "restrictChatMember", attempt.ChatID)
	if len(calls) != 2 {
		t.Fatalf("restrictChatMember calls = %d, want the mute and its lift", len(calls))
	}
	assertParams(t, calls[1], targetParams(attempt.ChatID, 42))
	assertCaptchaRows(t, attempt, 0)
}

// Wrong answers up to the limit fail the attempt: the chat's action is
// applied once and the alert names it.
func TestCaptchaWrongAnswersUpToTheLimitFailTheAttempt(t *testing.T) {
	useRealLocales(t)
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	attempt := pendingCaptchaAttempt(t, uniqueModuleChatID(), 123)
	if err := captcha.SetCaptchaFailureAction(attempt.ChatID, "Captcha Chat", "ban"); err != nil {
		t.Fatalf("SetCaptchaFailureAction() error = %v", err)
	}
	if err := captcha.SetCaptchaMaxAttempts(attempt.ChatID, "Captcha Chat", 2); err != nil {
		t.Fatalf("SetCaptchaMaxAttempts() error = %v", err)
	}
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadCaptcha(dispatcher)
	chat := gotgbot.Chat{Id: attempt.ChatID, Type: "supergroup", Title: "Captcha Chat"}
	data := encodeCallbackData("captcha_verify", map[string]string{"a": fmt.Sprint(attempt.ID), "u": "42", "s": "8"})
	tap := func() {
		t.Helper()
		ctx := newModuleCallbackContext(bot, chat, gotgbot.User{Id: 42, FirstName: "Member"}, data)
		if err := dispatcher.ProcessUpdate(bot, ctx.Update, nil); err != nil {
			t.Fatalf("ProcessUpdate() error = %v", err)
		}
	}
	tr := i18n.MustNewTranslator("en")

	tap()
	remaining, _ := tr.GetString("captcha_wrong_answer_remaining", i18n.TranslationParams{"d": 1})
	assertParams(t, client.onlyCall(t, "answerCallbackQuery"), map[string]any{"text": remaining})
	if calls := client.callsFor("banChatMember"); len(calls) != 0 {
		t.Fatalf("banChatMember calls = %d, want none before the limit", len(calls))
	}

	tap()
	assertParams(t, client.onlyCall(t, "banChatMember"), targetParams(attempt.ChatID, 42))
	banned, _ := tr.GetString("captcha_action_banned")
	final, _ := tr.GetString("captcha_wrong_answer_final", i18n.TranslationParams{"s": banned})
	answers := client.callsFor("answerCallbackQuery")
	if len(answers) != 2 {
		t.Fatalf("answerCallbackQuery calls = %d, want 2", len(answers))
	}
	assertParams(t, answers[1], map[string]any{"text": final, "show_alert": true})
	assertDeleted(t, client, attempt.ChatID, 123)
	if records := banRecordsOf(t, attempt.ChatID); len(records) != 1 || records[0].SourceDetail != captchaFailedWrongAnswers {
		t.Fatalf("ban records = %+v, want one for wrong answers", records)
	}
	assertCaptchaRows(t, attempt, 0)
}

// After a restart an expired attempt is settled at once and a running one
// keeps its timeout: it fails when that runs out, not before.
func TestCaptchaRestartSettlesExpiredAndKeepsRunningTimeouts(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	ctx, cancel := context.WithCancel(context.Background())
	captchaLifecycleMu.Lock()
	oldCtx, oldStop, oldDone := captchaLifecycleCtx, captchaLifecycleStop, captchaLifecycleDone
	captchaLifecycleCtx, captchaLifecycleStop, captchaLifecycleDone = ctx, cancel, false
	captchaLifecycleMu.Unlock()
	t.Cleanup(func() {
		cancel()
		captchaLifecycleWG.Wait()
		captchaLifecycleMu.Lock()
		captchaLifecycleCtx, captchaLifecycleStop, captchaLifecycleDone = oldCtx, oldStop, oldDone
		captchaLifecycleMu.Unlock()
	})
	now := time.Now()
	stored := map[string]*db.CaptchaAttempts{}
	for name, expiresAt := range map[string]time.Time{"expired": now.Add(-time.Minute), "running": now.Add(time.Second)} {
		attempt := &db.CaptchaAttempts{UserID: 42, ChatID: uniqueModuleChatID(), Answer: "7", MessageID: 123, CreatedAt: now.Add(-2 * time.Minute), ExpiresAt: expiresAt}
		if err := captcha.SetCaptchaEnabled(attempt.ChatID, "Captcha Chat", true); err != nil {
			t.Fatalf("SetCaptchaEnabled() error = %v", err)
		}
		if err := db.DB.Create(attempt).Error; err != nil {
			t.Fatalf("create %s attempt error = %v", name, err)
		}
		t.Cleanup(func() {
			_ = captcha.DeleteCaptchaAttempt(42, attempt.ChatID)
			_ = captcha.DeleteMutedUser(42, attempt.ChatID)
		})
		stored[name] = attempt
	}
	expired, running := stored["expired"], stored["running"]

	if err := runOrphanedCaptchaRecovery(bot); err != nil {
		t.Fatalf("runOrphanedCaptchaRecovery() error = %v", err)
	}
	if calls := callsToChat(client, "unbanChatMember", expired.ChatID); len(calls) != 1 {
		t.Fatalf("expired attempt kicks = %d, want 1 at once", len(calls))
	}
	assertCaptchaRows(t, expired, 0)
	if calls := callsToChat(client, "unbanChatMember", running.ChatID); len(calls) != 0 {
		t.Fatalf("running attempt kicks = %d, want none before its timeout", len(calls))
	}
	if current, err := captcha.GetCaptchaAttemptByID(running.ID); err != nil || current == nil {
		t.Fatalf("running attempt after restart = (%v, %v), want it kept", current, err)
	}

	waitForModuleCondition(t, func() bool { return len(callsToChat(client, "unbanChatMember", running.ChatID)) == 1 })
	cancel()
	captchaLifecycleWG.Wait()
	if time.Now().Before(running.ExpiresAt) {
		t.Fatalf("running attempt failed before its timeout %v", running.ExpiresAt)
	}
	assertDeleted(t, client, running.ChatID, 123)
	assertCaptchaRows(t, running, 0)
}
