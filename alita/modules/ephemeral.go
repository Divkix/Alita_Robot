package modules

import (
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"

	chatsettings "github.com/divkix/Alita_Robot/alita/settings"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	"github.com/divkix/Alita_Robot/alita/utils/tracing"
)

// ephemeralDesc is /ephemeral on|off: answers Alita would otherwise redirect
// to PM are shown only to the member who asked, in the group. From PM it
// takes the group as its first argument or from the connection.
var ephemeralDesc = helpers.CommandDescriptor{
	Name:           "ephemeral",
	PMGroupArg:     helpers.PMGroupArgOwn,
	RequiredChecks: declaredSettingChecks(chatsettings.EphemeralReplies),
}

// ephemeral handles /ephemeral. Without an argument it shows the switch.
func (moduleStruct) ephemeral(c *helpers.CommandContext) error {
	args := c.Ctx.Args()[1:]
	key := "ephemeral_status_off"
	if len(args) == 0 {
		if chatsettings.EphemeralReplies.ReadContext(tracing.UpdateContext(c.Ctx), c.Chat.Id).Value.(bool) {
			key = "ephemeral_status_on"
		}
	} else {
		var on bool
		switch strings.ToLower(args[0]) {
		case "on", "yes", "true":
			on, key = true, "ephemeral_enabled"
		case "off", "no", "false":
			key = "ephemeral_disabled"
		default:
			key = "pins_input_not_recognized"
		}
		if key != "pins_input_not_recognized" {
			err := chatsettings.EphemeralReplies.Write(chatsettings.WriteContext{Context: tracing.UpdateContext(c.Ctx), ChatID: c.Chat.Id, Title: c.Chat.Title, Bot: c.Bot}, on)
			if err != nil {
				log.WithContext(tracing.UpdateContext(c.Ctx)).WithFields(log.Fields{"chat_id": c.Chat.Id, "error": err}).
					Error("[Ephemeral] could not save the switch")
				key = "common_settings_save_failed"
			}
		}
	}
	text, _ := c.Tr.GetString(key)
	if _, err := c.Msg.Reply(c.Bot, withGroupLine(c.Tr, c.Msg, c.Chat, text), formatting.Shtml()); err != nil {
		log.WithFields(log.Fields{"chat_id": c.Chat.Id, "error": err}).Warn("[Ephemeral] could not reply")
		return err
	}
	return ext.EndGroups
}

// loadEphemeral registers /ephemeral as part of the Admin module (LoadAdmin).
func loadEphemeral(dispatcher *ext.Dispatcher) {
	helpers.WrapCommand(dispatcher, ephemeralDesc, adminModule.ephemeral)
}

// ephemeralReceiver is the one owner of when an answer may be shown only to
// its asker: /ephemeral is on for the chat, the update came from that group
// itself (not PM, not a PM group swap or connection, not a channel's direct
// messages), it is not a reply to someone else's message (a moderator pointing
// a newcomer at the rules answers publicly; the forum-topic implicit reply does
// not count), and a real user sent it (not an anonymous admin, a channel,
// Telegram or a bot). It returns that user's ID.
func ephemeralReceiver(ctx *ext.Context) (int64, bool) {
	msg, chat := ctx.EffectiveMessage, ctx.EffectiveChat
	if msg == nil || chat == nil || msg.Chat.Type == "private" || msg.Chat.IsDirectMessages || msg.Chat.Id != chat.Id {
		return 0, false
	}
	if reply := msg.ReplyToMessage; reply != nil && reply.ForumTopicCreated == nil {
		return 0, false
	}
	from := msg.From
	if msg.SenderChat != nil || from == nil || from.IsBot || from.Id <= 0 ||
		from.Id == chat_status.TelegramUserID || from.Id == chat_status.GroupAnonymousBotID {
		return 0, false
	}
	if !chatsettings.EphemeralReplies.ReadContext(tracing.UpdateContext(ctx), chat.Id).Value.(bool) {
		return 0, false
	}
	return from.Id, true
}

// sendEphemeral shows text only to the asker, in the group's topic and
// without a reply, when ephemeralReceiver allows it. It returns true only when
// Telegram accepted the message; on false the caller sends its public answer
// unchanged in the same update, so a send error (the bot is not an admin,
// a web_app or login_url button) degrades to today's behaviour. Telegram does
// not guarantee delivery of an accepted ephemeral message.
func sendEphemeral(b *gotgbot.Bot, ctx *ext.Context, text string, opts gotgbot.SendMessageOpts) bool {
	receiver, ok := ephemeralReceiver(ctx)
	if !ok {
		return false
	}
	msg := ctx.EffectiveMessage
	opts.ReplyParameters = nil
	opts.EphemeralMessageParameters = &gotgbot.EphemeralMessageParameters{ReceiverUserId: receiver}
	if msg.IsTopicMessage {
		opts.MessageThreadId = msg.MessageThreadId
	}
	if _, err := b.SendMessageWithContext(tracing.UpdateContext(ctx), msg.Chat.Id, text, &opts); err != nil {
		log.WithContext(tracing.UpdateContext(ctx)).WithFields(log.Fields{"chat_id": msg.Chat.Id, "error": err}).
			Debug("[Ephemeral] could not show the answer only to its asker; answering publicly")
		return false
	}
	return true
}
