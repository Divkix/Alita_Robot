//go:build testtools

package modules

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/connections"
	"github.com/divkix/Alita_Robot/alita/db/logchannels"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
)

// logChannelPMFixture is oneOffFixture's group (@fixturegroup) and a channel
// (@logchan) where the fixture's admin is an admin and the bot may post.
type logChannelPMFixture struct {
	adminChatFixture
	channel gotgbot.Chat
}

func newLogChannelPMFixture(t *testing.T) logChannelPMFixture {
	t.Helper()
	f := logChannelPMFixture{
		adminChatFixture: oneOffFixture(t),
		channel:          gotgbot.Chat{Id: uniqueModuleChatID(), Type: "channel", Title: "Log Channel", Username: "logchan"},
	}
	if err := chats.EnsureChatInDb(f.chat.Id, f.chat.Title); err != nil {
		t.Fatalf("EnsureChatInDb(group) error = %v", err)
	}
	if err := chats.EnsureChatInDb(f.channel.Id, f.channel.Title); err != nil {
		t.Fatalf("EnsureChatInDb(channel) error = %v", err)
	}
	channelJSON := json.RawMessage(fmt.Sprintf(`{"id":%d,"type":"channel","title":%q,"username":"logchan"}`, f.channel.Id, f.channel.Title))
	f.client.chatsByID["@logchan"] = channelJSON
	f.client.chatsByID[fmt.Sprint(f.channel.Id)] = channelJSON
	f.client.membersByChat = map[string]json.RawMessage{}
	f.setMember(f.channel.Id, f.admin.Id, `"status":"administrator","can_post_messages":true`)
	f.setMember(f.channel.Id, 999, `"status":"administrator","can_post_messages":true`)
	return f
}

// setMember makes getChatMember(chatID, userID) answer with fields.
func (f logChannelPMFixture) setMember(chatID, userID int64, fields string) {
	f.client.membersByChat[fmt.Sprint(chatID)+"|"+fmt.Sprint(userID)] = json.RawMessage(
		fmt.Sprintf(`{%s,"user":{"id":%d,"is_bot":%t,"first_name":"Someone"}}`, fields, userID, userID == 999),
	)
}

func logChannelsDispatcher() *ext.Dispatcher {
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	LoadLogChannels(dispatcher)
	return dispatcher
}

// sendPM sends text to the bot in user's PM through the dispatcher.
func (f logChannelPMFixture) sendPM(t *testing.T, user gotgbot.User, text string) {
	t.Helper()
	update := &gotgbot.Update{UpdateId: 1, Message: &gotgbot.Message{MessageId: 101, Date: 1, Chat: privateChatOf(user), From: &user, Text: text}}
	if err := logChannelsDispatcher().ProcessUpdate(f.bot, update, nil); err != nil {
		t.Fatalf("ProcessUpdate(%q) error = %v", text, err)
	}
}

// requireReplyTo fails unless a message containing want went to chatID.
func (f logChannelPMFixture) requireReplyTo(t *testing.T, chatID int64, want string) {
	t.Helper()
	for _, call := range f.client.callsFor("sendMessage") {
		if fmt.Sprint(call.Params["chat_id"]) == fmt.Sprint(chatID) && strings.Contains(fmt.Sprint(call.Params["text"]), want) {
			return
		}
	}
	t.Fatalf("no message to %d contains %q; sent:\n%s", chatID, want, sentText(f.client))
}

func (f logChannelPMFixture) requireNotBound(t *testing.T) {
	t.Helper()
	if got := logchannels.Get(f.chat.Id); got != nil {
		t.Fatalf("log channel bound to %d, want none", got.LogChannelID)
	}
}

func (f logChannelPMFixture) bindChannel(t *testing.T) {
	t.Helper()
	if err := logchannels.Set(f.chat.Id, f.chat.Title, f.channel.Id); err != nil {
		t.Fatalf("logchannels.Set() error = %v", err)
	}
}

// /setlog from PM names the group and the channel; the group's admin, who
// also administers the channel, binds it without posting in the channel.
func TestSetLogInPMBindsChannel(t *testing.T) {
	f := newLogChannelPMFixture(t)

	f.sendPM(t, f.admin, "/setlog @fixturegroup @logchan")

	got := logchannels.Get(f.chat.Id)
	if got == nil || got.LogChannelID != f.channel.Id {
		t.Fatalf("log channel = %+v, want %d", got, f.channel.Id)
	}
	f.requireReplyTo(t, f.admin.Id, translated(t, "logs_set_ok"))
	// The change is logged to the new channel, under the group.
	f.requireReplyTo(t, f.channel.Id, fmt.Sprintf(
		"<b>%s</b> (<code>%d</code>)\n#SETTINGS\nAdmin: %s\nset log channel",
		html.EscapeString(f.chat.Title), f.chat.Id, formatting.MentionHtml(f.admin.Id, f.admin.FirstName),
	))
}

func TestSetLogInPMBindsChannelByIDs(t *testing.T) {
	f := newLogChannelPMFixture(t)

	f.sendPM(t, f.admin, fmt.Sprintf("/setlog %d %d", f.chat.Id, f.channel.Id))

	got := logchannels.Get(f.chat.Id)
	if got == nil || got.LogChannelID != f.channel.Id {
		t.Fatalf("log channel = %+v, want %d", got, f.channel.Id)
	}
}

// With a saved connection the channel alone is enough: it names no group.
func TestSetLogInPMOnConnectionBindsChannel(t *testing.T) {
	f := newLogChannelPMFixture(t)
	f.connectAdmin(t)

	f.sendPM(t, f.admin, "/setlog @logchan")

	got := logchannels.Get(f.chat.Id)
	if got == nil || got.LogChannelID != f.channel.Id {
		t.Fatalf("log channel = %+v, want %d", got, f.channel.Id)
	}
}

func TestSetLogInPMWithoutChannelAsksForIt(t *testing.T) {
	f := newLogChannelPMFixture(t)

	f.sendPM(t, f.admin, "/setlog @fixturegroup")

	f.requireNotBound(t)
	f.requireReplyTo(t, f.admin.Id, translated(t, "logs_pm_need_channel"))
}

func TestSetLogInPMRefusesWhenUserNotChannelAdmin(t *testing.T) {
	f := newLogChannelPMFixture(t)
	f.setMember(f.channel.Id, f.admin.Id, `"status":"member"`)

	f.sendPM(t, f.admin, "/setlog @fixturegroup @logchan")

	f.requireNotBound(t)
	f.requireReplyTo(t, f.admin.Id, translated(t, "logs_pm_user_not_channel_admin"))
}

// The old flow needed a /setlog post in the channel, so a channel admin
// without the right to post could not bind it; from PM they cannot either.
func TestSetLogInPMRefusesChannelAdminWhoCannotPost(t *testing.T) {
	f := newLogChannelPMFixture(t)
	f.setMember(f.channel.Id, f.admin.Id, `"status":"administrator","can_post_messages":false`)

	f.sendPM(t, f.admin, "/setlog @fixturegroup @logchan")

	f.requireNotBound(t)
	f.requireReplyTo(t, f.admin.Id, translated(t, "logs_pm_user_not_channel_admin"))
}

// Telegram answers getChatMember in a channel only to an admin bot, so when
// the bot is not one, every lookup there fails. The user hears about the
// bot's rights, the thing to fix, not about their own.
func TestSetLogInPMWhenBotIsNotChannelAdminAnswersAboutBot(t *testing.T) {
	f := newLogChannelPMFixture(t)
	f.setMember(f.channel.Id, f.admin.Id, `"status":"creator"`)
	f.client.errorsByChat = map[string]error{
		"getChatMember|" + fmt.Sprint(f.channel.Id): errors.New("Bad Request: member list is inaccessible"),
	}

	f.sendPM(t, f.admin, "/setlog @fixturegroup @logchan")

	f.requireNotBound(t)
	f.requireReplyTo(t, f.admin.Id, translated(t, "logs_pm_bot_cannot_post"))
}

func TestSetLogInPMRefusesWhenBotCannotPost(t *testing.T) {
	f := newLogChannelPMFixture(t)
	f.setMember(f.channel.Id, 999, `"status":"administrator","can_post_messages":false`)

	f.sendPM(t, f.admin, "/setlog @fixturegroup @logchan")

	f.requireNotBound(t)
	f.requireReplyTo(t, f.admin.Id, translated(t, "logs_pm_bot_cannot_post"))
}

// A group as the second argument is not a channel, even where the user and
// the bot could post to it.
func TestSetLogInPMRefusesNonChannel(t *testing.T) {
	f := newLogChannelPMFixture(t)
	f.setMember(f.chat.Id, f.admin.Id, `"status":"creator"`)
	f.setMember(f.chat.Id, 999, `"status":"administrator","can_post_messages":true`)

	f.sendPM(t, f.admin, fmt.Sprintf("/setlog @fixturegroup %d", f.chat.Id))

	f.requireNotBound(t)
	f.requireReplyTo(t, f.admin.Id, translatedWith(t, "logs_pm_not_channel", i18n.TranslationParams{"channel": fmt.Sprint(f.chat.Id)}))
}

// Binding from PM needs the same group rights as forwarding the post in the
// group: an admin. A member who may connect and owns the channel is refused.
func TestSetLogInPMRefusesGroupMemberWhoIsNotAdmin(t *testing.T) {
	f := newLogChannelPMFixture(t)
	if err := connections.ToggleAllowConnect(f.chat.Id, true); err != nil {
		t.Fatalf("ToggleAllowConnect() error = %v", err)
	}
	f.setMember(f.channel.Id, f.member.Id, `"status":"creator"`)

	f.sendPM(t, f.member, "/setlog @fixturegroup @logchan")

	f.requireNotBound(t)
	f.requireReplyTo(t, f.member.Id, translated(t, "connections_is_user_connected_user_not_admin"))
}

func TestLogInPMWithGroupTogglesCategory(t *testing.T) {
	f := newLogChannelPMFixture(t)
	f.bindChannel(t)

	f.sendPM(t, f.admin, "/nolog @fixturegroup admin")

	if got := logchannels.Get(f.chat.Id); got == nil || got.CatAdmin {
		t.Fatalf("after /nolog: %+v, want admin category off", got)
	}
	f.requireReplyTo(t, f.admin.Id, translatedWith(t, "logs_disabled", i18n.TranslationParams{"cats": "admin"}))

	f.sendPM(t, f.admin, "/log @fixturegroup admin")

	if got := logchannels.Get(f.chat.Id); got == nil || !got.CatAdmin {
		t.Fatalf("after /log: %+v, want admin category on", got)
	}
}

func TestNologInPMRefusesGroupMemberWhoIsNotAdmin(t *testing.T) {
	f := newLogChannelPMFixture(t)
	f.bindChannel(t)
	if err := connections.ToggleAllowConnect(f.chat.Id, true); err != nil {
		t.Fatalf("ToggleAllowConnect() error = %v", err)
	}

	f.sendPM(t, f.member, "/nolog @fixturegroup admin")

	if got := logchannels.Get(f.chat.Id); got == nil || !got.CatAdmin {
		t.Fatalf("a member turned the admin category off: %+v", got)
	}
	f.requireReplyTo(t, f.member.Id, translated(t, "connections_is_user_connected_user_not_admin"))
}

func TestUnsetLogInPMWithGroup(t *testing.T) {
	f := newLogChannelPMFixture(t)
	f.bindChannel(t)

	f.sendPM(t, f.admin, "/unsetlog @fixturegroup")

	f.requireNotBound(t)
	f.requireReplyTo(t, f.admin.Id, translated(t, "logs_unset_ok"))
}

func TestLogChannelInPMWithGroup(t *testing.T) {
	f := newLogChannelPMFixture(t)
	f.bindChannel(t)

	f.sendPM(t, f.admin, "/logchannel @fixturegroup")

	f.requireReplyTo(t, f.admin.Id, fmt.Sprint(f.channel.Id))
}

// Without a group and without a connection, /setlog in PM answers with its
// help and the user's groups instead of asking for a channel post.
func TestSetLogInPMWithoutGroupAnswersWithHelp(t *testing.T) {
	f := titleFixture(t, "My Group")

	call := sendPMCommand(t, logChannelsDispatcher(), f.bot, f.client, f.admin, "/setlog", translated(t, "connections_pm_group_pick"))

	assertPMGroupHelp(t, fmt.Sprint(call.Params["text"]), "setlog", f.chat)
	if got := logchannels.Get(f.chat.Id); got != nil {
		t.Fatalf("log channel bound to %d, want none", got.LogChannelID)
	}
}

// In the group, /setlog without a forward still points at the channel step.
func TestSetLogInGroupStillAsksForChannelPost(t *testing.T) {
	f := newLogChannelPMFixture(t)
	update := &gotgbot.Update{UpdateId: 1, Message: &gotgbot.Message{
		MessageId: 101, Date: 1, Chat: f.chat, From: &f.admin, Text: "/setlog @logchan",
		Entities: []gotgbot.MessageEntity{{Type: "bot_command", Offset: 0, Length: 7}},
	}}
	if err := logChannelsDispatcher().ProcessUpdate(f.bot, update, nil); err != nil {
		t.Fatalf("ProcessUpdate() error = %v", err)
	}

	f.requireNotBound(t)
	f.requireReplyTo(t, f.chat.Id, translated(t, "logs_need_channel"))
}
