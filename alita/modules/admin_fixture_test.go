package modules

import (
	"fmt"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db/admin"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
)

// The fixture's admin must be authorised by the admin-cache load, not by the
// service-account short-circuit that 777000 and 1087968824 take.
func TestAdminChatFixtureAuthorisesThroughAdminCache(t *testing.T) {
	f := newAdminChatFixture(t)

	if _, known := chat_status.IsUserAdminCached(f.chat.Id, 777000); !known {
		t.Fatal("777000 is not answered without a lookup; the guard below proves nothing")
	}
	if _, known := chat_status.IsUserAdminCached(f.chat.Id, f.admin.Id); known {
		t.Fatalf("admin %d answered before any admin-cache load; it is a hardcoded admin", f.admin.Id)
	}

	if !chat_status.IsUserAdmin(f.bot, f.chat.Id, f.admin.Id) {
		t.Fatalf("IsUserAdmin(%d) = false, want true from the admin list", f.admin.Id)
	}
	f.client.onlyCall(t, "getChatAdministrators")
	if chat_status.IsUserAdmin(f.bot, f.chat.Id, f.member.Id) {
		t.Fatalf("IsUserAdmin(%d) = true, want false for a member", f.member.Id)
	}
}

func TestAdminChatFixtureMemberCannotModerate(t *testing.T) {
	f := newAdminChatFixture(t)
	other := f.member
	other.Id = uniquePositiveUserID()

	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadBans(dispatcher)
	ctx := newBanReplyContext(f.bot, f.chat, f.member, other, "/ban")
	if err := dispatcher.ProcessUpdate(f.bot, ctx.Update, nil); err != nil {
		t.Fatalf("ban(by member) error = %v", err)
	}
	if calls := f.client.callsFor("banChatMember"); len(calls) != 0 {
		t.Fatalf("member's /ban sent %d banChatMember calls, want none", len(calls))
	}
	f.client.onlyCall(t, "getChatAdministrators")

	ctx = newBanReplyContext(f.bot, f.chat, f.admin, other, "/ban")
	if err := dispatcher.ProcessUpdate(f.bot, ctx.Update, nil); err != nil {
		t.Fatalf("ban(by admin) error = %v", err)
	}
	assertParams(t, f.client.onlyCall(t, "banChatMember"), targetParams(f.chat.Id, other.Id))
}

func anonymousAdminProof(t *testing.T, dispatcher *ext.Dispatcher, f adminChatFixture, command *ext.Context) *gotgbot.Update {
	t.Helper()
	if err := admin.SetAnonAdminMode(f.chat.Id, false); err != nil {
		t.Fatalf("SetAnonAdminMode() error = %v", err)
	}
	msg := command.Message
	msg.From = &gotgbot.User{Id: chat_status.GroupAnonymousBotID, IsBot: true, FirstName: "Group", Username: "GroupAnonymousBot"}
	msg.SenderChat = &f.chat
	msg.Entities = []gotgbot.MessageEntity{{Type: "bot_command", Offset: 0, Length: int64(len(strings.Fields(msg.Text)[0]))}}
	key := chat_status.AnonAdminCacheKey(f.chat.Id, msg.MessageId)
	t.Cleanup(func() { _ = cache.GetMarshal().Delete(cache.Context, key) })

	before := len(f.client.callsFor("sendMessage"))
	if err := dispatcher.ProcessUpdate(f.bot, command.Update, nil); err != nil {
		t.Fatalf("anonymous %s error = %v", msg.Text, err)
	}
	calls := f.client.callsFor("sendMessage")[before:]
	if len(calls) != 1 {
		t.Fatalf("anonymous %s sent %d replies, want only the proof", msg.Text, len(calls))
	}
	proof := calls[0]
	assertParams(t, proof, map[string]any{"chat_id": f.chat.Id})
	markup, ok := proof.Params["reply_markup"].(gotgbot.InlineKeyboardMarkup)
	if !ok || len(markup.InlineKeyboard) != 1 || len(markup.InlineKeyboard[0]) != 1 ||
		!strings.HasPrefix(markup.InlineKeyboard[0][0].CallbackData, "anon_admin|") {
		t.Fatalf("anonymous %s reply = %#v, want the proof button", msg.Text, proof.Params)
	}
	tap := newModuleCallbackContext(f.bot, f.chat, f.admin, markup.InlineKeyboard[0][0].CallbackData).Update
	tap.CallbackQuery.Message = gotgbot.Message{
		MessageId: 9001,
		Date:      1,
		Chat:      f.chat,
		From:      &f.bot.User,
		Text:      fmt.Sprint(proof.Params["text"]),
	}
	return tap
}

func dispatchAnonymousAdminCommand(t *testing.T, dispatcher *ext.Dispatcher, f adminChatFixture, command *ext.Context) []moduleBotCall {
	t.Helper()
	tap := anonymousAdminProof(t, dispatcher, f, command)
	before := len(f.client.callsFor("sendMessage"))
	if err := dispatcher.ProcessUpdate(f.bot, tap, nil); err != nil {
		t.Fatalf("anonymous proof confirmation error = %v", err)
	}
	return f.client.callsFor("sendMessage")[before:]
}
