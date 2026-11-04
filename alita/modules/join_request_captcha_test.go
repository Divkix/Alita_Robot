//go:build testtools

package modules

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/approvals"
	"github.com/divkix/Alita_Robot/alita/db/captcha"
	"github.com/divkix/Alita_Robot/alita/db/greetings"
	"github.com/divkix/Alita_Robot/alita/db/logchannels"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/spamdbsettings"
)

type joinCaptchaFixture struct {
	adminChatFixture
	applicant  gotgbot.User
	userChatID int64
	logChatID  int64
}

func newJoinCaptchaFixture(t *testing.T, enabled bool) joinCaptchaFixture {
	t.Helper()
	useRealLocales(t)
	withMiniredis(t)
	f := joinCaptchaFixture{
		adminChatFixture: newAdminChatFixture(t),
		applicant:        gotgbot.User{Id: uniquePositiveUserID(), FirstName: "Applicant"},
		userChatID:       uniquePositiveUserID(),
		logChatID:        uniqueModuleChatID(),
	}
	pmResponse := json.RawMessage(fmt.Sprintf(
		`{"message_id":9003,"date":1,"chat":{"id":%d,"type":"private","first_name":"Applicant"},"photo":[{"file_id":"captcha-photo","file_unique_id":"captcha-photo-1","width":160,"height":80}]}`,
		f.userChatID,
	))
	for _, method := range []string{"sendPhoto", "editMessageCaption"} {
		f.client.responses[method] = pmResponse
	}
	if err := captcha.SetCaptchaMode(f.chat.Id, f.chat.Title, "math"); err != nil {
		t.Fatalf("SetCaptchaMode() error = %v", err)
	}
	if enabled {
		if err := captcha.SetCaptchaJoinRequests(f.chat.Id, f.chat.Title, true); err != nil {
			t.Fatalf("SetCaptchaJoinRequests() error = %v", err)
		}
	}
	t.Cleanup(func() {
		if err := db.DB.Where("chat_id = ?", f.chat.Id).Delete(&models.JoinRequestCaptcha{}).Error; err != nil {
			t.Errorf("clean join-request captcha rows: %v", err)
		}
		clearRecentJoinProcessing(f.chat.Id, f.applicant.Id)
	})
	return f
}

func (f joinCaptchaFixture) request(t *testing.T) {
	t.Helper()
	ctx := newJoinRequestContext(f.bot, f.chat, f.applicant)
	ctx.ChatJoinRequest.UserChatId = f.userChatID
	ctx.ChatJoinRequest.Date = time.Now().Unix()
	if err := greetingsModule.pendingJoins(f.bot, ctx); err != ext.ContinueGroups {
		t.Fatalf("pendingJoins() error = %v, want ContinueGroups", err)
	}
}

func (f joinCaptchaFixture) rows(t *testing.T) []models.JoinRequestCaptcha {
	t.Helper()
	var rows []models.JoinRequestCaptcha
	if err := db.DB.Where("chat_id = ?", f.chat.Id).Find(&rows).Error; err != nil {
		t.Fatalf("read join-request captcha rows: %v", err)
	}
	return rows
}

func (f joinCaptchaFixture) attempt(t *testing.T) models.JoinRequestCaptcha {
	t.Helper()
	rows := f.rows(t)
	if len(rows) != 1 {
		t.Fatalf("join-request captcha rows = %+v, want one", rows)
	}
	return rows[0]
}

func (f joinCaptchaFixture) enableLog(t *testing.T) {
	t.Helper()
	if err := logchannels.Set(f.chat.Id, f.chat.Title, f.logChatID); err != nil {
		t.Fatalf("logchannels.Set() error = %v", err)
	}
	if err := logchannels.SetCategory(f.chat.Id, logchannels.CategoryAutomated, true); err != nil {
		t.Fatalf("SetCategory(automated) error = %v", err)
	}
}

func (f joinCaptchaFixture) requireLog(t *testing.T, words ...string) {
	t.Helper()
	entries := spamDBMessagesTo(f.client, f.logChatID)
	if len(entries) != 1 {
		t.Fatalf("log entries = %v, want one", entries)
	}
	for _, word := range append([]string{f.chat.Title, f.applicant.FirstName, fmt.Sprint(f.applicant.Id)}, words...) {
		if !strings.Contains(strings.ToLower(entries[0]), strings.ToLower(word)) {
			t.Errorf("log entry %q does not contain %q", entries[0], word)
		}
	}
}

func (f joinCaptchaFixture) callbackContext(from gotgbot.User, attempt models.JoinRequestCaptcha, answer string) *ext.Context {
	data := encodeCallbackData("joincaptcha", map[string]string{
		"a": fmt.Sprint(attempt.ID),
		"c": fmt.Sprint(attempt.ChatID),
		"r": fmt.Sprint(attempt.RefreshCount),
		"s": answer,
	})
	return ext.NewContext(f.bot, &gotgbot.Update{
		UpdateId: 1134,
		CallbackQuery: &gotgbot.CallbackQuery{
			Id: "join-captcha-answer", From: from, Data: data,
			Message: gotgbot.Message{
				MessageId: attempt.MessageID, Date: 1,
				Chat: gotgbot.Chat{Id: f.userChatID, Type: "private", FirstName: "Applicant"},
				From: &f.bot.User, Caption: "Join-request captcha",
			},
		},
	}, nil)
}

func (f joinCaptchaFixture) pressContext(t *testing.T, ctx *ext.Context) {
	t.Helper()
	if err := captchaModule.joinCaptchaCallback(f.bot, ctx); err != nil && err != ext.EndGroups {
		t.Fatalf("joinCaptchaCallback() error = %v", err)
	}
}

func (f joinCaptchaFixture) answer(t *testing.T, attempt models.JoinRequestCaptcha, answer string) {
	t.Helper()
	f.pressContext(t, f.callbackContext(f.applicant, attempt, answer))
}

func (f joinCaptchaFixture) requireNoMemberActions(t *testing.T) {
	t.Helper()
	for _, method := range []string{"banChatMember", "unbanChatMember", "restrictChatMember"} {
		if calls := f.client.callsFor(method); len(calls) != 0 {
			t.Errorf("%s calls = %v, want applicants never moderated as members", method, calls)
		}
	}
}

func (f joinCaptchaFixture) requireTerminalCaption(t *testing.T, attempt models.JoinRequestCaptcha) {
	t.Helper()
	call := f.client.onlyCall(t, "editMessageCaption")
	assertParams(t, call, map[string]any{"chat_id": f.userChatID, "message_id": attempt.MessageID})
	if strings.TrimSpace(fmt.Sprint(call.Params["caption"])) == "" || call.Params["caption"] == nil {
		t.Error("terminal caption is empty")
	}
	if kb := markupOf(t, call); len(kb.InlineKeyboard) != 0 {
		t.Errorf("terminal keyboard = %+v, want the challenge buttons removed", kb)
	}
}

func requireJoinRequestAdminButtons(t *testing.T, f joinCaptchaFixture) {
	t.Helper()
	calls := callsToChat(f.client, "sendMessage", f.chat.Id)
	if len(calls) != 1 {
		t.Fatalf("group messages = %v, want one admin notice", calls)
	}
	text := fmt.Sprint(calls[0].Params["text"])
	if !strings.Contains(text, f.applicant.FirstName) || !strings.Contains(text, fmt.Sprint(f.applicant.Id)) {
		t.Errorf("admin notice = %q, want applicant name and ID", text)
	}
	actions := map[string]bool{}
	for _, row := range markupOf(t, calls[0]).InlineKeyboard {
		for _, button := range row {
			decoded, ok := decodeCallbackData(button.CallbackData, "join_request")
			if !ok {
				t.Fatalf("admin button has invalid callback data %q", button.CallbackData)
			}
			action, _ := decoded.Field("a")
			user, _ := decoded.Field("u")
			if user != fmt.Sprint(f.applicant.Id) {
				t.Errorf("admin button user = %q, want %d", user, f.applicant.Id)
			}
			actions[action] = true
		}
	}
	if len(actions) != 3 || !actions["accept"] || !actions["decline"] || !actions["ban"] {
		t.Errorf("admin actions = %v, want accept, decline and ban", actions)
	}
}

func TestJoinRequestCaptchaOffPreservesAdminAndAutoApproveFlows(t *testing.T) {
	for _, autoApprove := range []bool{false, true} {
		t.Run(fmt.Sprintf("autoapprove=%t", autoApprove), func(t *testing.T) {
			f := newJoinCaptchaFixture(t, false)
			if err := greetings.SetShouldAutoApprove(f.chat.Id, autoApprove); err != nil {
				t.Fatalf("SetShouldAutoApprove() error = %v", err)
			}
			f.request(t)
			if calls := f.client.callsFor("sendPhoto"); len(calls) != 0 {
				t.Fatalf("sendPhoto calls = %d, want no PM with join captcha off", len(calls))
			}
			if rows := f.rows(t); len(rows) != 0 {
				t.Fatalf("join captcha rows = %+v, want none", rows)
			}
			if autoApprove {
				assertParams(t, f.client.onlyCall(t, "approveChatJoinRequest"), targetParams(f.chat.Id, f.applicant.Id))
				if calls := f.client.callsFor("sendMessage"); len(calls) != 0 {
					t.Fatalf("sendMessage calls = %v, want no admin notice", calls)
				}
			} else {
				requireJoinRequestAdminButtons(t, f)
				if calls := f.client.callsFor("approveChatJoinRequest"); len(calls) != 0 {
					t.Fatalf("approvals = %v, want admin decision", calls)
				}
			}
		})
	}
}

func TestJoinRequestCaptchaSendsMathPhotoToUserChatAndCapsTimeout(t *testing.T) {
	for _, minutes := range []int{2, 10} {
		t.Run(fmt.Sprintf("timeout=%d", minutes), func(t *testing.T) {
			f := newJoinCaptchaFixture(t, true)
			if err := captcha.SetCaptchaTimeout(f.chat.Id, f.chat.Title, minutes); err != nil {
				t.Fatalf("SetCaptchaTimeout() error = %v", err)
			}
			before := time.Now()
			f.request(t)
			attempt := f.attempt(t)
			call := f.client.onlyCall(t, "sendPhoto")
			assertParams(t, call, map[string]any{"chat_id": f.userChatID})
			if !strings.Contains(fmt.Sprint(call.Params["caption"]), f.chat.Title) {
				t.Errorf("PM caption = %v, want group title %q", call.Params["caption"], f.chat.Title)
			}
			if attempt.ID == 0 || attempt.ChatID != f.chat.Id || attempt.UserID != f.applicant.Id || attempt.FirstName != f.applicant.FirstName || attempt.UserChatID != f.userChatID || attempt.MessageID != 9003 || attempt.Answer == "" || attempt.Attempts != 0 || attempt.RefreshCount != 0 {
				t.Fatalf("stored attempt = %+v, want the delivered initial PM challenge", attempt)
			}
			wantLifetime := time.Duration(min(minutes, 5)) * time.Minute
			if attempt.ExpiresAt.Before(before.Add(wantLifetime-time.Second)) || attempt.ExpiresAt.After(time.Now().Add(wantLifetime)) {
				t.Errorf("expires_at = %v, want %v from delivery", attempt.ExpiresAt, wantLifetime)
			}
			foundAnswer, foundRefresh := false, false
			for _, row := range markupOf(t, call).InlineKeyboard {
				for _, button := range row {
					decoded, ok := decodeCallbackData(button.CallbackData, "joincaptcha")
					if !ok {
						t.Fatalf("PM button has invalid callback data %q", button.CallbackData)
					}
					for key, want := range map[string]string{"a": fmt.Sprint(attempt.ID), "c": fmt.Sprint(f.chat.Id), "r": "0"} {
						if got, _ := decoded.Field(key); got != want {
							t.Errorf("PM button field %s = %q, want %q", key, got, want)
						}
					}
					selected, _ := decoded.Field("s")
					foundAnswer = foundAnswer || selected == attempt.Answer
					foundRefresh = foundRefresh || selected == ""
				}
			}
			if !foundAnswer || !foundRefresh {
				t.Errorf("PM keyboard has answer=%t, refresh=%t, want both", foundAnswer, foundRefresh)
			}
			for _, method := range []string{"approveChatJoinRequest", "declineChatJoinRequest", "sendMessage"} {
				if calls := f.client.callsFor(method); len(calls) != 0 {
					t.Errorf("%s calls = %v, want only a pending PM", method, calls)
				}
			}
			f.requireNoMemberActions(t)
		})
	}
}

func TestJoinRequestCaptchaCorrectAnswerApprovesWithoutAutoApprove(t *testing.T) {
	f := newJoinCaptchaFixture(t, true)
	f.enableLog(t)
	f.request(t)
	attempt := f.attempt(t)
	f.answer(t, attempt, attempt.Answer)

	assertParams(t, f.client.onlyCall(t, "approveChatJoinRequest"), targetParams(f.chat.Id, f.applicant.Id))
	f.requireTerminalCaption(t, attempt)
	f.requireLog(t)
	if rows := f.rows(t); len(rows) != 0 {
		t.Fatalf("attempts after approval = %+v, want none", rows)
	}
	for range 2 {
		if !joinRequestCaptchaPassed(f.chat.Id, f.applicant.Id) {
			t.Fatal("successful approval did not leave a nonconsuming recent-pass marker")
		}
	}
	if calls := callsToChat(f.client, "sendMessage", f.chat.Id); len(calls) != 0 {
		t.Errorf("group messages = %v, want no admin notice", calls)
	}
	if calls := f.client.callsFor("declineChatJoinRequest"); len(calls) != 0 {
		t.Errorf("declines = %v, want none", calls)
	}
	f.requireNoMemberActions(t)
}

func TestJoinRequestCaptchaWrongAnswersDeclineOnlyAtMaximum(t *testing.T) {
	f := newJoinCaptchaFixture(t, true)
	f.enableLog(t)
	if err := captcha.SetCaptchaMaxAttempts(f.chat.Id, f.chat.Title, 2); err != nil {
		t.Fatalf("SetCaptchaMaxAttempts() error = %v", err)
	}
	f.request(t)
	attempt := f.attempt(t)
	wrong := ""
	for _, row := range markupOf(t, f.client.onlyCall(t, "sendPhoto")).InlineKeyboard {
		for _, button := range row {
			decoded, ok := decodeCallbackData(button.CallbackData, "joincaptcha")
			if !ok {
				t.Fatalf("invalid PM callback %q", button.CallbackData)
			}
			if answer, _ := decoded.Field("s"); answer != "" && answer != attempt.Answer {
				wrong = answer
			}
		}
	}
	if wrong == "" {
		t.Fatal("captcha keyboard has no wrong answer")
	}
	f.answer(t, attempt, wrong)
	if got := f.attempt(t); got.Attempts != 1 {
		t.Fatalf("attempt after first wrong answer = %+v, want one attempt used", got)
	}
	if calls := f.client.callsFor("declineChatJoinRequest"); len(calls) != 0 {
		t.Fatalf("declines after one wrong answer = %v, want another chance", calls)
	}
	if entries := spamDBMessagesTo(f.client, f.logChatID); len(entries) != 0 {
		t.Fatalf("logs before terminal result = %v, want none", entries)
	}
	f.answer(t, attempt, wrong)
	assertParams(t, f.client.onlyCall(t, "declineChatJoinRequest"), targetParams(f.chat.Id, f.applicant.Id))
	if text, alert := lastAnswer(t, f.client); !alert || text != translated(t, "captcha_join_request_declined_alert") {
		t.Errorf("terminal wrong-answer response = %q (alert %t), want the declined alert", text, alert)
	}
	f.requireTerminalCaption(t, attempt)
	f.requireLog(t)
	if rows := f.rows(t); len(rows) != 0 {
		t.Fatalf("attempts after maximum = %+v, want none", rows)
	}
	if joinRequestCaptchaPassed(f.chat.Id, f.applicant.Id) {
		t.Error("wrong answers created a pass marker")
	}
	if calls := f.client.callsFor("approveChatJoinRequest"); len(calls) != 0 {
		t.Errorf("approvals = %v, want none", calls)
	}
	f.requireNoMemberActions(t)
}

func TestJoinRequestCaptchaTimeoutAndSweepDeclineApplicants(t *testing.T) {
	for _, sweep := range []bool{false, true} {
		t.Run(fmt.Sprintf("sweep=%t", sweep), func(t *testing.T) {
			f := newJoinCaptchaFixture(t, true)
			f.enableLog(t)
			attempt := models.JoinRequestCaptcha{
				ChatID: f.chat.Id, UserID: f.applicant.Id, FirstName: f.applicant.FirstName, UserChatID: f.userChatID,
				MessageID: 9003, Answer: "7", ExpiresAt: time.Now().Add(-time.Minute),
				CreatedAt: time.Now().Add(-2 * time.Minute),
			}
			if err := db.DB.Create(&attempt).Error; err != nil {
				t.Fatalf("create expired attempt: %v", err)
			}
			if sweep {
				future := models.JoinRequestCaptcha{
					ChatID: f.chat.Id, UserID: uniquePositiveUserID(), UserChatID: uniquePositiveUserID(),
					MessageID: 9004, Answer: "8", ExpiresAt: time.Now().Add(4 * time.Minute),
				}
				if err := db.DB.Create(&future).Error; err != nil {
					t.Fatalf("create live attempt: %v", err)
				}
				if err := cleanupExpiredJoinRequestCaptchas(context.Background(), f.bot); err != nil {
					t.Fatalf("cleanupExpiredJoinRequestCaptchas() error = %v", err)
				}
				if rows := f.rows(t); len(rows) != 1 || rows[0].ID != future.ID {
					t.Fatalf("rows after sweep = %+v, want only live attempt %+v", rows, future)
				}
			} else {
				if err := expireJoinRequestCaptcha(context.Background(), f.bot, &attempt); err != nil {
					t.Fatalf("expireJoinRequestCaptcha() error = %v", err)
				}
				if rows := f.rows(t); len(rows) != 0 {
					t.Fatalf("rows after expiry = %+v, want none", rows)
				}
			}
			assertParams(t, f.client.onlyCall(t, "declineChatJoinRequest"), targetParams(f.chat.Id, f.applicant.Id))
			f.requireTerminalCaption(t, attempt)
			f.requireLog(t)
			f.requireNoMemberActions(t)
			if calls := f.client.callsFor("approveChatJoinRequest"); len(calls) != 0 {
				t.Errorf("approvals = %v, want none", calls)
			}
		})
	}
}

func TestJoinRequestCaptchaApprovedUserBypassesCaptchaAndSpamChecks(t *testing.T) {
	_, cas := withSpamDBCAS(t, true, http.StatusOK, spamDBCASListed)
	f := newJoinCaptchaFixture(t, true)
	if err := approvals.AddApprovedUser(f.chat.Id, f.applicant.Id, f.admin.Id, "trusted"); err != nil {
		t.Fatalf("AddApprovedUser() error = %v", err)
	}
	if err := spamdbsettings.SetSpamDBEnabled(f.chat.Id, true); err != nil {
		t.Fatalf("SetSpamDBEnabled() error = %v", err)
	}
	f.request(t)
	assertParams(t, f.client.onlyCall(t, "approveChatJoinRequest"), targetParams(f.chat.Id, f.applicant.Id))
	if got := cas.hits.Load(); got != 0 {
		t.Errorf("CAS lookups = %d, want approved user bypass", got)
	}
	for _, method := range []string{"sendPhoto", "sendMessage", "declineChatJoinRequest"} {
		if calls := f.client.callsFor(method); len(calls) != 0 {
			t.Errorf("%s calls = %v, want immediate approval without captcha", method, calls)
		}
	}
	if rows := f.rows(t); len(rows) != 0 {
		t.Errorf("approved user's captcha rows = %+v, want none", rows)
	}
	f.requireNoMemberActions(t)
}

func TestJoinRequestCaptchaSpamDBActionsBeforePM(t *testing.T) {
	for _, action := range []string{spamdbsettings.ActionBan, spamdbsettings.ActionMute, spamdbsettings.ActionLog} {
		t.Run(action, func(t *testing.T) {
			_, cas := withSpamDBCAS(t, true, http.StatusOK, spamDBCASListed)
			f := newJoinCaptchaFixture(t, true)
			f.enableLog(t)
			if err := spamdbsettings.SetSpamDBEnabled(f.chat.Id, true); err != nil {
				t.Fatalf("SetSpamDBEnabled() error = %v", err)
			}
			if err := spamdbsettings.SetSpamDBAction(f.chat.Id, action); err != nil {
				t.Fatalf("SetSpamDBAction() error = %v", err)
			}
			f.request(t)
			if got := cas.hits.Load(); got != 1 {
				t.Fatalf("CAS lookups = %d, want the configured real checker used once", got)
			}
			if action == spamdbsettings.ActionLog {
				assertParams(t, f.client.onlyCall(t, "sendPhoto"), map[string]any{"chat_id": f.userChatID})
				_ = f.attempt(t)
				if calls := f.client.callsFor("declineChatJoinRequest"); len(calls) != 0 {
					t.Errorf("log-only declines = %v, want captcha to decide", calls)
				}
			} else {
				assertParams(t, f.client.onlyCall(t, "declineChatJoinRequest"), targetParams(f.chat.Id, f.applicant.Id))
				if calls := f.client.callsFor("sendPhoto"); len(calls) != 0 {
					t.Errorf("sendPhoto calls = %v, want no PM for rejected spammer", calls)
				}
				if rows := f.rows(t); len(rows) != 0 {
					t.Errorf("spam-rejected captcha rows = %+v, want none", rows)
				}
			}
			if action == spamdbsettings.ActionBan {
				assertParams(t, f.client.onlyCall(t, "banChatMember"), targetParams(f.chat.Id, f.applicant.Id))
				if record := singleBanRecord(t, f.chat.Id); record.UserId != f.applicant.Id || record.Source != models.BanSourceSpamDB {
					t.Errorf("ban record = %+v, want applicant with spamdb source", record)
				}
			} else {
				f.requireNoMemberActions(t)
			}
			if calls := f.client.callsFor("restrictChatMember"); len(calls) != 0 {
				t.Errorf("restrict calls = %v, want mute action to decline nonmembers", calls)
			}
			if calls := f.client.callsFor("approveChatJoinRequest"); len(calls) != 0 {
				t.Errorf("approvals = %v, want no spam bypass", calls)
			}
			if calls := callsToChat(f.client, "sendMessage", f.chat.Id); len(calls) != 0 {
				t.Errorf("group messages = %v, want no admin notice", calls)
			}
			f.requireLog(t, "CAS")
		})
	}
}

func TestJoinRequestCaptchaPMSendFailureFallsBackWithoutDeclining(t *testing.T) {
	for _, autoApprove := range []bool{false, true} {
		for _, sendErr := range []error{errors.New("Forbidden: bot was blocked by the user"), errors.New("http2: timeout awaiting response headers")} {
			t.Run(fmt.Sprintf("autoapprove=%t/%s", autoApprove, sendErr), func(t *testing.T) {
				f := newJoinCaptchaFixture(t, true)
				f.client.errors["sendPhoto"] = sendErr
				if err := greetings.SetShouldAutoApprove(f.chat.Id, autoApprove); err != nil {
					t.Fatalf("SetShouldAutoApprove() error = %v", err)
				}
				f.request(t)
				assertParams(t, f.client.onlyCall(t, "sendPhoto"), map[string]any{"chat_id": f.userChatID})
				if autoApprove {
					assertParams(t, f.client.onlyCall(t, "approveChatJoinRequest"), targetParams(f.chat.Id, f.applicant.Id))
					if calls := callsToChat(f.client, "sendMessage", f.chat.Id); len(calls) != 0 {
						t.Errorf("group messages = %v, want no admin notice", calls)
					}
				} else {
					requireJoinRequestAdminButtons(t, f)
					if calls := f.client.callsFor("approveChatJoinRequest"); len(calls) != 0 {
						t.Errorf("approvals = %v, want admin decision", calls)
					}
				}
				if calls := f.client.callsFor("declineChatJoinRequest"); len(calls) != 0 {
					t.Errorf("declines = %v, want no penalty for unreachable PM", calls)
				}
				if rows := f.rows(t); len(rows) != 0 {
					t.Errorf("failed-delivery attempts = %+v, want none left to time out", rows)
				}
				f.requireNoMemberActions(t)
			})
		}
	}
}

func TestJoinRequestCaptchaGoneCallbacksRemoveAttemptQuietly(t *testing.T) {
	for _, approve := range []bool{false, true} {
		for _, gone := range []string{"HIDE_REQUESTER_MISSING", "USER_ALREADY_PARTICIPANT"} {
			t.Run(fmt.Sprintf("approve=%t/%s", approve, gone), func(t *testing.T) {
				f := newJoinCaptchaFixture(t, true)
				f.enableLog(t)
				if err := captcha.SetCaptchaMaxAttempts(f.chat.Id, f.chat.Title, 1); err != nil {
					t.Fatalf("SetCaptchaMaxAttempts() error = %v", err)
				}
				f.request(t)
				attempt := f.attempt(t)
				answer, method := attempt.Answer+"0", "declineChatJoinRequest"
				if approve {
					answer, method = attempt.Answer, "approveChatJoinRequest"
				}
				f.client.errors[method] = &gotgbot.TelegramError{Code: 400, Description: "Bad Request: " + gone}
				f.answer(t, attempt, answer)
				assertParams(t, f.client.onlyCall(t, method), targetParams(f.chat.Id, f.applicant.Id))
				assertParams(t, f.client.onlyCall(t, "answerCallbackQuery"), map[string]any{"callback_query_id": "join-captcha-answer"})
				if approve {
					if text, alert := lastAnswer(t, f.client); !alert || text != translated(t, "captcha_join_request_expired") {
						t.Errorf("handled-elsewhere answer = %q (alert %t), want the expired-request answer", text, alert)
					}
				}
				if calls := callsToChat(f.client, "sendMessage", f.chat.Id); len(calls) != 0 {
					t.Errorf("handled-elsewhere admin notices = %v, want quiet completion", calls)
				}
				if rows := f.rows(t); len(rows) != 0 {
					t.Errorf("handled-elsewhere attempts = %+v, want none", rows)
				}
				if entries := spamDBMessagesTo(f.client, f.logChatID); len(entries) != 0 {
					t.Errorf("handled-elsewhere logs = %v, want quiet completion", entries)
				}
				if joinRequestCaptchaPassed(f.chat.Id, f.applicant.Id) {
					t.Error("request handled elsewhere received a captcha-pass marker")
				}
				f.requireNoMemberActions(t)
			})
		}
	}
}

func TestJoinRequestCaptchaRejectsWrongIdentityAndStaleButtons(t *testing.T) {
	for _, invalid := range []string{"other user", "other group", "other PM", "other message", "stale generation"} {
		t.Run(invalid, func(t *testing.T) {
			f := newJoinCaptchaFixture(t, true)
			f.request(t)
			attempt := f.attempt(t)
			callbackAttempt, from := attempt, f.applicant
			switch invalid {
			case "other user":
				from = f.admin
			case "other group":
				callbackAttempt.ChatID = uniqueModuleChatID()
			case "other message":
				callbackAttempt.MessageID++
			case "stale generation":
				callbackAttempt.RefreshCount++
			}
			callbackFixture := f
			if invalid == "other PM" {
				callbackFixture.userChatID++
			}
			f.pressContext(t, callbackFixture.callbackContext(from, callbackAttempt, attempt.Answer))
			if got := f.attempt(t); got.ID != attempt.ID || got.Answer != attempt.Answer || got.Attempts != 0 || got.RefreshCount != 0 {
				t.Errorf("attempt after rejected callback = %+v, want unchanged %+v", got, attempt)
			}
			for _, method := range []string{"approveChatJoinRequest", "declineChatJoinRequest", "editMessageCaption", "editMessageMedia"} {
				if calls := f.client.callsFor(method); len(calls) != 0 {
					t.Errorf("%s calls = %v, want invalid callback ignored", method, calls)
				}
			}
		})
	}
}

func TestJoinRequestCaptchaRefreshMakesOldAnswerButtonStale(t *testing.T) {
	f := newJoinCaptchaFixture(t, true)
	f.request(t)
	old := f.attempt(t)
	if err := db.DB.Model(&models.JoinRequestCaptcha{}).Where("id = ?", old.ID).
		UpdateColumn("updated_at", time.Now().Add(-time.Minute)).Error; err != nil {
		t.Fatalf("allow refresh after cooldown: %v", err)
	}
	f.client.responses["sendPhoto"] = json.RawMessage(fmt.Sprintf(
		`{"message_id":9004,"date":1,"chat":{"id":%d,"type":"private","first_name":"Applicant"},"photo":[{"file_id":"refreshed-photo","file_unique_id":"captcha-photo-2","width":160,"height":80}]}`,
		f.userChatID,
	))
	f.answer(t, old, "")
	refreshed := f.attempt(t)
	if refreshed.ID != old.ID || refreshed.RefreshCount != 1 || refreshed.Attempts != 0 || refreshed.MessageID != 9004 || refreshed.Answer == "" {
		t.Fatalf("refreshed attempt = %+v, want same attempt and next keyboard generation", refreshed)
	}
	if photos := f.client.callsFor("sendPhoto"); len(photos) != 2 {
		t.Fatalf("sendPhoto calls = %v, want the initial and refreshed PM", photos)
	} else {
		assertParams(t, photos[1], map[string]any{"chat_id": f.userChatID})
	}
	assertDeleted(t, f.client, f.userChatID, old.MessageID)
	f.answer(t, old, old.Answer)
	if got := f.attempt(t); got.Attempts != 0 || got.RefreshCount != 1 {
		t.Errorf("attempt after old button = %+v, want refresh retained without using an attempt", got)
	}
	if calls := f.client.callsFor("approveChatJoinRequest"); len(calls) != 0 {
		t.Fatalf("old keyboard approved = %v, want no action", calls)
	}
	f.answer(t, refreshed, refreshed.Answer)
	assertParams(t, f.client.onlyCall(t, "approveChatJoinRequest"), targetParams(f.chat.Id, f.applicant.Id))
}

func TestJoinRequestCaptchaRefreshSendFailureKeepsOriginalChallengeUsable(t *testing.T) {
	f := newJoinCaptchaFixture(t, true)
	f.request(t)
	old := f.attempt(t)
	if err := db.DB.Model(&models.JoinRequestCaptcha{}).Where("id = ?", old.ID).
		UpdateColumn("updated_at", time.Now().Add(-time.Minute)).Error; err != nil {
		t.Fatalf("allow refresh after cooldown: %v", err)
	}
	sendErr := errors.New("http2: timeout awaiting response headers")
	f.client.errors["sendPhoto"] = sendErr
	if err := captchaModule.joinCaptchaCallback(f.bot, f.callbackContext(f.applicant, old, "")); !errors.Is(err, sendErr) {
		t.Fatalf("failed refresh error = %v, want send failure", err)
	}
	if got := f.attempt(t); got.ID != old.ID || got.MessageID != old.MessageID || got.Answer != old.Answer || got.RefreshCount != old.RefreshCount || got.Attempts != old.Attempts || !got.ExpiresAt.Equal(old.ExpiresAt) {
		t.Fatalf("attempt after failed refresh = %+v, want original %+v", got, old)
	}
	for _, method := range []string{"deleteMessage", "approveChatJoinRequest", "declineChatJoinRequest"} {
		if calls := f.client.callsFor(method); len(calls) != 0 {
			t.Errorf("%s calls = %v, want the original challenge preserved", method, calls)
		}
	}
	delete(f.client.errors, "sendPhoto")
	f.answer(t, old, old.Answer)
	assertParams(t, f.client.onlyCall(t, "approveChatJoinRequest"), targetParams(f.chat.Id, f.applicant.Id))
}

func TestJoinRequestCaptchaPassedApplicantSkipsRegularCaptchaAndGetsWelcome(t *testing.T) {
	for _, serviceMessage := range []bool{false, true} {
		t.Run(fmt.Sprintf("serviceMessage=%t", serviceMessage), func(t *testing.T) {
			f := newJoinCaptchaFixture(t, true)
			if err := captcha.SetCaptchaEnabled(f.chat.Id, f.chat.Title, true); err != nil {
				t.Fatalf("SetCaptchaEnabled() error = %v", err)
			}
			if err := greetings.SetWelcomeText(f.chat.Id, "Welcome {first}", "", nil, db.TEXT); err != nil {
				t.Fatalf("SetWelcomeText() error = %v", err)
			}
			f.request(t)
			attempt := f.attempt(t)
			f.answer(t, attempt, attempt.Answer)

			serviceCtx := newServiceJoinContext(f.bot, f.chat, f.admin, []gotgbot.User{f.applicant})
			if err := SendCaptcha(f.bot, serviceCtx, f.applicant.Id, f.applicant.FirstName); !errors.Is(err, errCaptchaDisabled) {
				t.Fatalf("SendCaptcha() after PM pass error = %v, want errCaptchaDisabled", err)
			}
			clearRecentJoinProcessing(f.chat.Id, f.applicant.Id)
			if serviceMessage {
				if err := processSingleNewMember(f.bot, serviceCtx, f.applicant, true); err != nil {
					t.Fatalf("processSingleNewMember() error = %v", err)
				}
			} else {
				ctx := newChatMemberContext(f.bot, f.chat, f.admin,
					gotgbot.ChatMemberLeft{User: f.applicant}, gotgbot.ChatMemberMember{User: f.applicant})
				if err := greetingsModule.newMember(f.bot, ctx); err != ext.EndGroups {
					t.Fatalf("newMember() error = %v, want EndGroups", err)
				}
			}
			if calls := f.client.callsFor("sendPhoto"); len(calls) != 1 {
				t.Errorf("captcha photos = %v, want only the original PM challenge", calls)
			}
			if regular, err := captcha.GetCaptchaAttempt(f.applicant.Id, f.chat.Id); err != nil || regular != nil {
				t.Errorf("regular captcha attempt = %+v, %v, want none", regular, err)
			}
			messages := spamDBMessagesTo(f.client, f.chat.Id)
			if len(messages) != 1 || !strings.Contains(messages[0], "Welcome") || !strings.Contains(messages[0], f.applicant.FirstName) {
				t.Errorf("group messages = %v, want the normal welcome", messages)
			}
			f.requireNoMemberActions(t)
		})
	}
}

func TestJoinRequestCaptchaApprovalFailureFallsBackToAdminAndAnswersCallback(t *testing.T) {
	for _, noticeFails := range []bool{false, true} {
		t.Run(fmt.Sprintf("noticeFails=%t", noticeFails), func(t *testing.T) {
			f := newJoinCaptchaFixture(t, true)
			f.request(t)
			attempt := f.attempt(t)
			approveErr := &gotgbot.TelegramError{Code: 400, Description: "Bad Request: CHAT_ADMIN_REQUIRED"}
			f.client.errors["approveChatJoinRequest"] = approveErr
			noticeErr := errors.New("admin notice unavailable")
			if noticeFails {
				f.client.errorsByChat = map[string]error{"sendMessage|" + fmt.Sprint(f.chat.Id): noticeErr}
			}

			err := captchaModule.joinCaptchaCallback(f.bot, f.callbackContext(f.applicant, attempt, attempt.Answer))
			if !errors.Is(err, approveErr) {
				t.Errorf("approval failure = %v, want the original Telegram error", err)
			}
			if noticeFails && !errors.Is(err, noticeErr) {
				t.Errorf("approval failure = %v, want the admin-notice error too", err)
			}
			assertParams(t, f.client.onlyCall(t, "approveChatJoinRequest"), targetParams(f.chat.Id, f.applicant.Id))
			requireJoinRequestAdminButtons(t, f)
			assertParams(t, f.client.onlyCall(t, "answerCallbackQuery"), map[string]any{"callback_query_id": "join-captcha-answer"})
			if joinRequestCaptchaPassed(f.chat.Id, f.applicant.Id) {
				t.Error("failed approval left a captcha-pass marker")
			}
			if rows := f.rows(t); len(rows) != 0 {
				t.Errorf("attempts after failed approval = %+v, want none", rows)
			}
			if calls := f.client.callsFor("sendPhoto"); len(calls) != 1 {
				t.Errorf("captcha photos = %v, want only the initial PM challenge", calls)
			}
			if calls := f.client.callsFor("declineChatJoinRequest"); len(calls) != 0 {
				t.Errorf("declines = %v, want the pending request left for admins", calls)
			}
			f.requireNoMemberActions(t)
		})
	}
}

func TestJoinRequestCaptchaApprovedUserApprovalFailureFallsBackWithoutPM(t *testing.T) {
	f := newJoinCaptchaFixture(t, true)
	if err := approvals.AddApprovedUser(f.chat.Id, f.applicant.Id, f.admin.Id, "trusted"); err != nil {
		t.Fatalf("AddApprovedUser() error = %v", err)
	}
	approveErr := &gotgbot.TelegramError{Code: 400, Description: "Bad Request: CHAT_ADMIN_REQUIRED"}
	f.client.errors["approveChatJoinRequest"] = approveErr
	ctx := newJoinRequestContext(f.bot, f.chat, f.applicant)
	ctx.ChatJoinRequest.UserChatId = f.userChatID
	ctx.ChatJoinRequest.Date = time.Now().Unix()
	if err := greetingsModule.pendingJoins(f.bot, ctx); !errors.Is(err, approveErr) {
		t.Errorf("approved-user approval failure = %v, want the original Telegram error", err)
	}

	assertParams(t, f.client.onlyCall(t, "approveChatJoinRequest"), targetParams(f.chat.Id, f.applicant.Id))
	requireJoinRequestAdminButtons(t, f)
	for _, method := range []string{"sendPhoto", "declineChatJoinRequest"} {
		if calls := f.client.callsFor(method); len(calls) != 0 {
			t.Errorf("%s calls = %v, want the pending request left for admins without a PM", method, calls)
		}
	}
	if calls := callsToChat(f.client, "sendMessage", f.userChatID); len(calls) != 0 {
		t.Errorf("private messages = %v, want no PM for an approved user", calls)
	}
	if rows := f.rows(t); len(rows) != 0 {
		t.Errorf("approved-user captcha rows = %+v, want none", rows)
	}
	f.requireNoMemberActions(t)
}

func TestJoinRequestCaptchaSpamDBBanFailureStillDeclinesBeforePM(t *testing.T) {
	_, cas := withSpamDBCAS(t, true, http.StatusOK, spamDBCASListed)
	f := newJoinCaptchaFixture(t, true)
	f.enableLog(t)
	if err := spamdbsettings.SetSpamDBEnabled(f.chat.Id, true); err != nil {
		t.Fatalf("SetSpamDBEnabled() error = %v", err)
	}
	if err := spamdbsettings.SetSpamDBAction(f.chat.Id, spamdbsettings.ActionBan); err != nil {
		t.Fatalf("SetSpamDBAction() error = %v", err)
	}
	f.client.errors["banChatMember"] = &gotgbot.TelegramError{Code: 400, Description: "Bad Request: CHAT_ADMIN_REQUIRED"}
	f.request(t)

	if got := cas.hits.Load(); got != 1 {
		t.Errorf("CAS lookups = %d, want one real spam check", got)
	}
	assertParams(t, f.client.onlyCall(t, "banChatMember"), targetParams(f.chat.Id, f.applicant.Id))
	assertParams(t, f.client.onlyCall(t, "declineChatJoinRequest"), targetParams(f.chat.Id, f.applicant.Id))
	if records := banRecordsOf(t, f.chat.Id); len(records) != 0 {
		t.Errorf("ban records = %+v, want no record for the failed ban", records)
	}
	if rows := f.rows(t); len(rows) != 0 {
		t.Errorf("spam-rejected captcha rows = %+v, want none", rows)
	}
	for _, method := range []string{"sendPhoto", "approveChatJoinRequest", "restrictChatMember", "unbanChatMember"} {
		if calls := f.client.callsFor(method); len(calls) != 0 {
			t.Errorf("%s calls = %v, want only the failed ban and request decline", method, calls)
		}
	}
	for _, chatID := range []int64{f.chat.Id, f.userChatID} {
		if calls := callsToChat(f.client, "sendMessage", chatID); len(calls) != 0 {
			t.Errorf("messages to %d = %v, want neither PM nor admin fallback for a rejected spammer", chatID, calls)
		}
	}
	f.requireLog(t, "CAS", "ban (failed)")
}

func TestJoinRequestCaptchaRecoverySchedulesExpiredTimer(t *testing.T) {
	f := newJoinCaptchaFixture(t, true)
	f.enableLog(t)
	attempt := models.JoinRequestCaptcha{
		ChatID: f.chat.Id, UserID: f.applicant.Id, FirstName: f.applicant.FirstName, UserChatID: f.userChatID,
		MessageID: 9003, Answer: "7", ExpiresAt: time.Now().Add(-time.Minute),
		CreatedAt: time.Now().Add(-2 * time.Minute),
	}
	if err := db.DB.Create(&attempt).Error; err != nil {
		t.Fatalf("create expired attempt: %v", err)
	}

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

	if err := recoverJoinRequestCaptchas(f.bot); err != nil {
		t.Fatalf("recoverJoinRequestCaptchas() error = %v", err)
	}
	waitForModuleCondition(t, func() bool {
		return len(f.client.callsFor("declineChatJoinRequest")) != 0 && len(f.client.callsFor("editMessageCaption")) != 0
	})
	cancel()
	captchaLifecycleWG.Wait()

	assertParams(t, f.client.onlyCall(t, "declineChatJoinRequest"), targetParams(f.chat.Id, f.applicant.Id))
	f.requireTerminalCaption(t, attempt)
	f.requireLog(t, "timeout")
	if rows := f.rows(t); len(rows) != 0 {
		t.Errorf("attempts after timer expiry = %+v, want none", rows)
	}
	if calls := f.client.callsFor("approveChatJoinRequest"); len(calls) != 0 {
		t.Errorf("timer approvals = %v, want only a decline", calls)
	}
	f.requireNoMemberActions(t)
}
