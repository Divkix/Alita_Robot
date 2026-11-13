//go:build testtools

package modules

import (
	"fmt"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/stretchr/testify/require"

	"github.com/divkix/Alita_Robot/alita/db/captcha"
	"github.com/divkix/Alita_Robot/alita/db/greetings"
)

// heldCaptchaMember gives member 42 a pending attempt in a fresh chat with
// captcha on, as a join would leave it.
func heldCaptchaMember(t *testing.T) (gotgbot.Chat, gotgbot.User, uint) {
	t.Helper()
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Captcha Chat"}
	require.NoError(t, captcha.SetCaptchaEnabled(chat.Id, chat.Title, true))
	attempt := pendingCaptchaAttempt(t, chat.Id, 123)
	return chat, gotgbot.User{Id: 42, FirstName: "Member"}, attempt.ID
}

// The pending-captcha watcher holds what a held member sends, but their own
// join and leave notices are not theirs to hold: those go on to the later
// groups untouched.
func TestPendingCaptchaWatcherPassesJoinAndLeaveNotices(t *testing.T) {
	for _, tc := range []struct {
		name       string
		build      func(msg *gotgbot.Message, member *gotgbot.User)
		wantResult error
		wantStored int
	}{
		{name: "join notice", build: func(msg *gotgbot.Message, member *gotgbot.User) {
			msg.Text, msg.NewChatMembers = "", []gotgbot.User{*member}
		}, wantResult: ext.ContinueGroups},
		{name: "leave notice", build: func(msg *gotgbot.Message, member *gotgbot.User) {
			msg.Text, msg.LeftChatMember = "", member
		}, wantResult: ext.ContinueGroups},
		{name: "text", build: func(*gotgbot.Message, *gotgbot.User) {}, wantResult: ext.EndGroups, wantStored: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := newModuleBotClient()
			bot := newModuleTestBot(client)
			chat, member, attemptID := heldCaptchaMember(t)
			ctx := newModuleMessageContext(bot, chat, member, "hello")
			tc.build(ctx.EffectiveMessage, &member)

			require.Equal(t, tc.wantResult, captchaModule.handlePendingCaptchaMessage(bot, ctx))
			stored, err := captcha.GetStoredMessagesForAttempt(attemptID)
			require.NoError(t, err)
			require.Len(t, stored, tc.wantStored)
			require.Len(t, client.callsFor("deleteMessage"), tc.wantStored)
		})
	}
}

// A newcomer whose only message while held was their join notice passes with
// no summary of held messages.
func TestCaptchaPassAfterJoinNoticeReportsNothingHeld(t *testing.T) {
	useRealLocales(t)
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat, member, attemptID := heldCaptchaMember(t)
	ctx := newModuleMessageContext(bot, chat, member, "")
	ctx.EffectiveMessage.NewChatMembers = []gotgbot.User{member}
	_ = captchaModule.handlePendingCaptchaMessage(bot, ctx)

	attempt, err := captcha.GetCaptchaAttemptByID(attemptID)
	require.NoError(t, err)
	require.NoError(t, tapCaptcha(bot, attempt, "7"))
	for _, call := range client.callsFor("sendMessage") {
		if text := fmt.Sprint(call.Params["text"]); strings.Contains(text, "tried to send") {
			t.Fatalf("pass reported held messages: %q", text)
		}
	}
}

// With clean-service on, the held newcomer's join notice still reaches
// clean-service, which removes it.
func TestCleanServiceRemovesHeldNewcomersJoinNotice(t *testing.T) {
	client := newModuleBotClient()
	bot := newModuleTestBot(client)
	chat, member, attemptID := heldCaptchaMember(t)
	require.NoError(t, greetings.SetShouldCleanService(chat.Id, true))

	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadCaptcha(dispatcher)
	LoadGreetings(dispatcher)
	require.NoError(t, dispatcher.ProcessUpdate(bot, &gotgbot.Update{UpdateId: 1, Message: &gotgbot.Message{
		MessageId: 11, Date: 1, Chat: chat, From: &member, NewChatMembers: []gotgbot.User{member},
	}}, nil))

	assertDeleted(t, client, chat.Id, 11)
	stored, err := captcha.GetStoredMessagesForAttempt(attemptID)
	require.NoError(t, err)
	require.Empty(t, stored)
}
