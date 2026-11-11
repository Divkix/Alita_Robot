//go:build testtools

package modules

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/ephemeral"
	"github.com/divkix/Alita_Robot/alita/db/models"
	chatsettings "github.com/divkix/Alita_Robot/alita/settings"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
)

// enableEphemeral turns /ephemeral on for chatID for the rest of the test.
func enableEphemeral(t *testing.T, chatID int64) {
	t.Helper()
	if err := ephemeral.SetEnabled(chatID, true); err != nil {
		t.Fatalf("ephemeral.SetEnabled(%d) error = %v", chatID, err)
	}
	cleanupEphemeral(t, chatID)
}

func cleanupEphemeral(t *testing.T, chatID int64) {
	t.Helper()
	t.Cleanup(func() {
		if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.EphemeralSettings{}).Error; err != nil {
			t.Errorf("cleanup ephemeral settings of %d: %v", chatID, err)
		}
	})
}

// ephemeralSends are the sends that asked Telegram to show the message only
// to one user.
func ephemeralSends(client *moduleBotClient) []moduleBotCall {
	var sends []moduleBotCall
	client.mu.Lock()
	defer client.mu.Unlock()
	for _, call := range client.calls {
		if _, ok := call.Params["ephemeral_message_parameters"]; ok {
			sends = append(sends, call)
		}
	}
	return sends
}

// ephemeralReceiverOf is the user a send was shown to, 0 for a public send.
func ephemeralReceiverOf(call moduleBotCall) int64 {
	params, _ := call.Params["ephemeral_message_parameters"].(*gotgbot.EphemeralMessageParameters)
	if params == nil {
		return 0
	}
	return params.ReceiverUserId
}

// ephemeralRefusingClient answers every ephemeral send with an error, as
// Telegram does when the bot is not an admin, and records it like any call.
type ephemeralRefusingClient struct{ *moduleBotClient }

func (c ephemeralRefusingClient) RequestWithContext(ctx context.Context, token, method string, params map[string]any, opts *gotgbot.RequestOpts) (json.RawMessage, error) {
	response, err := c.moduleBotClient.RequestWithContext(ctx, token, method, params, opts)
	if _, ok := params["ephemeral_message_parameters"]; ok {
		return nil, errors.New("Bad Request: not enough rights to send ephemeral messages")
	}
	return response, err
}

func TestEphemeralCommandStoresTheSwitchForAdminsOnly(t *testing.T) {
	useRealLocales(t)
	f := newAdminChatFixture(t)
	cleanupEphemeral(t, f.chat.Id)
	dispatcher := adminPMDispatcher()
	on := func() bool {
		return chatsettings.EphemeralReplies.ReadContext(t.Context(), f.chat.Id).Value.(bool)
	}

	aispamProcessUpdate(t, dispatcher, f.bot, f.chat, f.member, 1101, "/ephemeral on")
	if on() || db.CountRows(&models.EphemeralSettings{}, "chat_id = ?", f.chat.Id) != 0 {
		t.Fatal("/ephemeral on from a member changed the switch")
	}
	if reply := lastMessageTo(t, f.client, f.chat.Id); reply != translated(t, "connections_is_user_connected_user_not_admin") {
		t.Fatalf("member reply = %q, want the admin refusal", reply)
	}

	aispamProcessUpdate(t, dispatcher, f.bot, f.chat, f.admin, 1102, "/ephemeral on")
	if !on() {
		t.Fatal("/ephemeral on from an admin left the switch off")
	}
	if reply := lastMessageTo(t, f.client, f.chat.Id); reply != translated(t, "ephemeral_enabled") {
		t.Fatalf("on reply = %q", reply)
	}
	aispamProcessUpdate(t, dispatcher, f.bot, f.chat, f.admin, 1103, "/ephemeral")
	if reply := lastMessageTo(t, f.client, f.chat.Id); reply != translated(t, "ephemeral_status_on") {
		t.Fatalf("status reply = %q", reply)
	}
	aispamProcessUpdate(t, dispatcher, f.bot, f.chat, f.admin, 1104, "/ephemeral maybe")
	if reply := lastMessageTo(t, f.client, f.chat.Id); reply != translated(t, "pins_input_not_recognized") || !on() {
		t.Fatalf("bad input reply = %q, switch on = %t", reply, on())
	}
	aispamProcessUpdate(t, dispatcher, f.bot, f.chat, f.admin, 1105, "/ephemeral off")
	if on() {
		t.Fatal("/ephemeral off left the switch on")
	}
}

// Only a real user's own message in the group itself, not a reply, may get
// an answer shown only to them.
func TestEphemeralReceiverAcceptsOnlyAUsersOwnGroupMessage(t *testing.T) {
	chat := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Ephemeral Chat"}
	enableEphemeral(t, chat.Id)
	off := gotgbot.Chat{Id: uniqueModuleChatID(), Type: "supergroup", Title: "Off Chat"}
	user := gotgbot.User{Id: 4242, FirstName: "Asker"}

	for _, tc := range []struct {
		name   string
		mutate func(msg *gotgbot.Message, ctx *ext.Context)
		want   bool
	}{
		{"own message", func(*gotgbot.Message, *ext.Context) {}, true},
		{"forum topic implicit reply", func(msg *gotgbot.Message, _ *ext.Context) {
			msg.IsTopicMessage, msg.MessageThreadId = true, 7
			msg.ReplyToMessage = &gotgbot.Message{MessageId: 7, Chat: chat, ForumTopicCreated: &gotgbot.ForumTopicCreated{Name: "t"}}
		}, true},
		{"reply to another message", func(msg *gotgbot.Message, _ *ext.Context) {
			msg.ReplyToMessage = &gotgbot.Message{MessageId: 50, Chat: chat}
		}, false},
		{"setting off", func(msg *gotgbot.Message, ctx *ext.Context) {
			msg.Chat, ctx.EffectiveChat = off, &off
		}, false},
		{"anonymous admin", func(msg *gotgbot.Message, _ *ext.Context) {
			msg.From = &gotgbot.User{Id: chat_status.GroupAnonymousBotID, IsBot: true, FirstName: "Group"}
			msg.SenderChat = &chat
		}, false},
		{"channel sender", func(msg *gotgbot.Message, _ *ext.Context) {
			msg.From = &gotgbot.User{Id: chat_status.SendAsChannelBotID, IsBot: true, FirstName: "Channel"}
			msg.SenderChat = &gotgbot.Chat{Id: -100555, Type: "channel"}
		}, false},
		{"telegram", func(msg *gotgbot.Message, _ *ext.Context) {
			msg.From = &gotgbot.User{Id: chat_status.TelegramUserID, FirstName: "Telegram"}
		}, false},
		{"bot", func(msg *gotgbot.Message, _ *ext.Context) {
			msg.From = &gotgbot.User{Id: 5151, IsBot: true, FirstName: "Other bot"}
		}, false},
		{"private chat", func(msg *gotgbot.Message, ctx *ext.Context) {
			pm := privateChatOf(user)
			msg.Chat, ctx.EffectiveChat = pm, &pm
		}, false},
		{"PM group swap", func(msg *gotgbot.Message, _ *ext.Context) {
			msg.Chat = privateChatOf(user)
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := newModuleMessageContext(newModuleTestBot(newModuleBotClient()), chat, user, "/rules")
			tc.mutate(ctx.EffectiveMessage, ctx)
			got, ok := ephemeralReceiver(ctx)
			if ok != tc.want || (ok && got != user.Id) {
				t.Fatalf("ephemeralReceiver() = %d, %t; want %t for the asker %d", got, ok, tc.want, user.Id)
			}
		})
	}
}

func TestSendEphemeralShowsTextOnlyToTheAskerInItsTopic(t *testing.T) {
	f := newAdminChatFixture(t)
	enableEphemeral(t, f.chat.Id)
	ctx := newModuleMessageContext(f.bot, f.chat, f.member, "/rules")
	ctx.EffectiveMessage.IsTopicMessage, ctx.EffectiveMessage.MessageThreadId = true, 7

	if !sendEphemeral(f.bot, ctx, "only for you", gotgbot.SendMessageOpts{ReplyParameters: &gotgbot.ReplyParameters{MessageId: 101}}) {
		t.Fatal("sendEphemeral() = false for an eligible message")
	}
	call := f.client.onlyCall(t, "sendMessage")
	assertParams(t, call, map[string]any{"chat_id": f.chat.Id, "message_thread_id": 7, "text": "only for you"})
	if got := ephemeralReceiverOf(call); got != f.member.Id {
		t.Fatalf("receiver = %d, want the asker %d", got, f.member.Id)
	}
	if _, ok := call.Params["reply_parameters"]; ok {
		t.Fatal("an ephemeral send carried reply_parameters")
	}

	f.bot.BotClient = ephemeralRefusingClient{f.client}
	if sendEphemeral(f.bot, ctx, "refused", gotgbot.SendMessageOpts{}) {
		t.Fatal("sendEphemeral() = true for a send Telegram refused")
	}
	if n := len(f.client.callsFor("sendMessage")); n != 2 {
		t.Fatalf("sendMessage calls = %d, want the two attempts and nothing else", n)
	}
}
