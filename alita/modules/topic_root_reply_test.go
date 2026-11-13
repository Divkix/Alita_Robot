//go:build testtools

package modules

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// forumTopicID is the forum topic the tests' commands are sent in; its
// creation service message has the same ID.
const forumTopicID = 7

// topicCreator opened forum topic forumTopicID.
var topicCreator = gotgbot.User{Id: 5151, FirstName: "Topic Creator"}

// sentInTopic puts ctx's message in forum topic forumTopicID and gives it
// reply, or, for a nil reply, Telegram's implicit reply to the topic's
// creation message.
func sentInTopic(ctx *ext.Context, reply *gotgbot.Message) *ext.Context {
	msg := ctx.EffectiveMessage
	msg.IsTopicMessage, msg.MessageThreadId = true, forumTopicID
	if reply == nil {
		reply = &gotgbot.Message{
			MessageId: forumTopicID, Date: 1, Chat: msg.Chat, From: &topicCreator,
			ForumTopicCreated: &gotgbot.ForumTopicCreated{Name: "Topic"},
		}
	}
	msg.ReplyToMessage = reply
	return ctx
}

// botActions lists, in order, each request the handler sent other than a
// lookup, as its method and parameters.
func botActions(client *moduleBotClient) []string {
	client.mu.Lock()
	defer client.mu.Unlock()
	var actions []string
	for _, call := range client.calls {
		if strings.HasPrefix(call.Method, "get") {
			continue
		}
		action := call.Method
		for _, key := range slices.Sorted(maps.Keys(call.Params)) {
			action += fmt.Sprintf(" %s=%v", key, call.Params[key])
		}
		actions = append(actions, action)
	}
	return actions
}

// A command in a forum topic that names nobody and has only Telegram's
// implicit reply to the topic's creation message acts as it does in the topic
// with no reply at all: it neither targets the topic's creator nor deletes,
// purges, pins or reports the creation message.
func TestTopicRootReplyIsNoReply(t *testing.T) {
	useRealLocales(t)
	for _, tt := range []struct {
		text     string
		byMember bool
		run      func(*gotgbot.Bot, *ext.Context) error
	}{
		{text: "/ban", run: bansModule.ban},
		{text: "/ban spamming links", run: bansModule.ban},
		{text: "/mute", run: mutesModule.mute},
		{text: "/mute being loud", run: mutesModule.mute},
		{text: "/warn", run: warnsModule.warnUser},
		{text: "/warn rude", run: warnsModule.warnUser},
		{text: "/kick", run: bansModule.kick},
		{text: "/kick flooding", run: bansModule.kick},
		{text: "/dban 42", run: bansModule.dBan},
		{text: "/dmute 42", run: mutesModule.dMute},
		{text: "/dkick", run: bansModule.dkick},
		{text: "/dwarn", run: warnsModule.dWarnUser},
		{text: "/spam", run: spamFilterModule.spamCommand},
		{text: "/purge", run: purgesModule.purge},
		{text: "/purgefrom", run: purgesModule.purgeFrom},
		{text: "/del", run: purgesModule.delCmd},
		{text: "/report", byMember: true, run: reportsModule.report},
		{text: "/pin", run: commandResponse(pinsModule.pin)},
		{text: "/unpin", run: commandResponse(pinsModule.unpin)},
		{text: "/id", run: miscModule.getId},
		{text: "/id 42", run: miscModule.getId},
	} {
		t.Run(tt.text, func(t *testing.T) {
			f := newAdminChatFixture(t)
			f.chat.IsForum = true
			sender := f.admin
			if tt.byMember {
				sender = f.member
			}
			send := func(reply bool) []string {
				resetCalls(f.client)
				ctx := newModuleMessageContext(f.bot, f.chat, sender, tt.text)
				ctx.EffectiveMessage.IsTopicMessage, ctx.EffectiveMessage.MessageThreadId = true, forumTopicID
				if reply {
					sentInTopic(ctx, nil)
				}
				if err := tt.run(f.bot, ctx); err != nil && err != ext.EndGroups {
					t.Fatalf("%s error = %v", tt.text, err)
				}
				return botActions(f.client)
			}

			withoutReply := send(false)
			if withRootReply := send(true); !slices.Equal(withRootReply, withoutReply) {
				t.Fatalf("%s with the topic's implicit reply sent\n%s\nwant what it sends with no reply\n%s",
					tt.text, strings.Join(withRootReply, "\n"), strings.Join(withoutReply, "\n"))
			}
		})
	}
}

// A deliberate reply to an ordinary message in a topic still names its author.
func TestTopicReplyToOrdinaryMessageTargetsItsAuthor(t *testing.T) {
	f := newAdminChatFixture(t)
	f.chat.IsForum = true
	ctx := sentInTopic(newModuleMessageContext(f.bot, f.chat, f.admin, "/ban spam"), &gotgbot.Message{
		MessageId: 50, Date: 1, Chat: f.chat, From: &f.member, MessageThreadId: forumTopicID, IsTopicMessage: true,
	})
	if err := bansModule.ban(f.bot, ctx); err != ext.EndGroups {
		t.Fatalf("ban() error = %v, want EndGroups", err)
	}
	assertParams(t, f.client.onlyCall(t, "banChatMember"), targetParams(f.chat.Id, f.member.Id))

	resetCalls(f.client)
	ctx = sentInTopic(newModuleMessageContext(f.bot, f.chat, f.admin, "/pin"), &gotgbot.Message{
		MessageId: 50, Date: 1, Chat: f.chat, From: &f.member, MessageThreadId: forumTopicID, IsTopicMessage: true,
	})
	if err := commandResponse(pinsModule.pin)(f.bot, ctx); err != ext.EndGroups {
		t.Fatalf("pin() error = %v, want EndGroups", err)
	}
	assertParams(t, f.client.onlyCall(t, "pinChatMessage"), map[string]any{"chat_id": f.chat.Id, "message_id": 50})
}
