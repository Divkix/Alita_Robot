package modules

import (
	"fmt"
	"html"
	"strconv"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
	"github.com/eko/gocache/lib/v4/store"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/divkix/Alita_Robot/alita/db/logchannels"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/actionlog"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	"github.com/divkix/Alita_Robot/alita/utils/tracing"
)

const logHandlerGroup = 11

var logChannelsModule = moduleStruct{
	moduleName:   "LogChannels",
	handlerGroup: logHandlerGroup,
}

func setlogPendingKey(channelID, messageID int64) string {
	return fmt.Sprintf("alita:setlog:%d:%d", channelID, messageID)
}

func (moduleStruct) setLog(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	if chat == nil || msg == nil {
		return ext.EndGroups
	}
	tr := ctxTr(ctx)
	if chat.Type == "channel" {
		// Channel posts have no user sender; only channel admins can post here.
		m := cache.GetMarshal()
		if m == nil {
			text, _ := tr.GetString("common_settings_save_failed")
			replyHTML(b, msg, text)
			return ext.EndGroups
		}
		if err := m.Set(
			cache.Context,
			setlogPendingKey(chat.Id, msg.MessageId),
			[]byte("1"),
			store.WithExpiration(time.Hour),
		); err != nil {
			log.Errorf("[LogChannels] failed to store pending setlog: %v", err)
			text, _ := tr.GetString("common_settings_save_failed")
			replyHTML(b, msg, text)
			return ext.EndGroups
		}
		text, _ := tr.GetString("logs_forward_to_group")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	if chat.Type == "private" {
		return setLogFromPM(b, ctx, tr)
	}
	// A forwarded /setlog post is the bind step; let the group-11 watcher
	// (captureSetLogForward) handle it instead of answering logs_need_channel.
	if msg.ForwardOrigin != nil {
		return ext.ContinueGroups
	}
	if requireUser(b, ctx) == nil {
		return ext.EndGroups
	}
	text, _ := tr.GetString("logs_need_channel")
	replyHTML(b, msg, text)
	return ext.EndGroups
}

// setLogFromPM binds the channel named after the group: "/setlog <group>
// <channel>", with the group already cut by withPMGroupArg or left out on a
// connection. No post in the channel proves who runs it, so Telegram must:
// the user is an admin of the group and of the channel, and the bot may post
// there.
func setLogFromPM(b *gotgbot.Bot, ctx *ext.Context, tr *i18n.Translator) error {
	msg := ctx.EffectiveMessage
	from := chat_status.RequireUser(b, ctx)
	if from == nil {
		return ext.EndGroups
	}
	group := resolveAdminChat(b, ctx, false)
	if group == nil {
		return ext.EndGroups
	}
	args := ctx.Args()[1:]
	if len(args) != 1 {
		text, _ := tr.GetString("logs_pm_need_channel")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	channel, err := chat_status.GetChat(b, args[0])
	if err != nil {
		log.WithFields(log.Fields{"channel": args[0], "error": err}).Debug("[LogChannels] channel lookup failed")
	}
	if err != nil || channel.Type != "channel" {
		text, _ := tr.GetString("logs_pm_not_channel", i18n.TranslationParams{"channel": html.EscapeString(args[0])})
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	// The bot first: Telegram answers getChatMember in a channel only to an
	// admin bot, so without that the user lookup below would fail too.
	if !canPostInChannel(b, channel.Id, b.Id) {
		text, _ := tr.GetString("logs_pm_bot_cannot_post")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	// As with a /setlog post in the channel, the user must be able to post there.
	if !canPostInChannel(b, channel.Id, from.Id) {
		text, _ := tr.GetString("logs_pm_user_not_channel_admin")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	if err := logchannels.Set(group.Id, group.Title, channel.Id); err != nil {
		log.Errorf("[LogChannels] Set: %v", err)
		text, _ := tr.GetString("common_settings_save_failed")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	text, _ := tr.GetString("logs_set_ok")
	replyHTML(b, msg, text)
	actionlog.Settings(b, group, from, "set log channel")
	return ext.EndGroups
}

// canPostInChannel reports whether userID owns the channel or is an admin of
// it that can post messages.
func canPostInChannel(b *gotgbot.Bot, channelID, userID int64) bool {
	member, err := b.GetChatMember(channelID, userID, nil)
	if err != nil {
		log.WithFields(log.Fields{"channelId": channelID, "userId": userID, "error": err}).Debug("[LogChannels] channel member lookup failed")
		return false
	}
	switch member.GetStatus() {
	case "creator":
		return true
	case "administrator":
		return member.MergeChatMember().CanPostMessages
	}
	return false
}

// logsChat returns the group a log-channel command acts on: the chat it was
// sent in or, in PM, the group withPMGroupArg named or the connected one. A
// non-nil admin must be an admin of that group, checked once: in PM by
// IsUserConnected, which answers any refusal. nil means the call has been
// answered.
func logsChat(b *gotgbot.Bot, ctx *ext.Context, admin *gotgbot.User) *gotgbot.Chat {
	chat := ctx.EffectiveChat
	if chat.Type != "private" {
		if admin != nil && !chat_status.RequireUserAdmin(b, ctx, chat, admin.Id) {
			chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
			return nil
		}
		return chat
	}
	if admin != nil {
		return resolveAdminChat(b, ctx, false)
	}
	if chat = chat_status.IsUserConnected(b, ctx, false, false); chat != nil {
		ctx.EffectiveChat = chat
	}
	return chat
}

func (moduleStruct) unsetLog(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	from := chat_status.RequireUser(b, ctx)
	if from == nil {
		return ext.EndGroups
	}
	tr := ctxTr(ctx)
	chat := logsChat(b, ctx, from)
	if chat == nil {
		return ext.EndGroups
	}
	// Read the settings first: Unset deletes them, and the change is logged
	// to the channel that was just unset.
	prev := logchannels.Get(chat.Id)
	if err := logchannels.Unset(chat.Id); err != nil {
		text, _ := tr.GetString("logs_not_set")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	text, _ := tr.GetString("logs_unset_ok")
	replyHTML(b, msg, text)
	actionlog.SettingsTo(b, chat, prev, from, "unset log channel")
	return ext.EndGroups
}

func (moduleStruct) logChannel(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	if chat_status.RequireUser(b, ctx) == nil {
		return ext.EndGroups
	}
	tr := ctxTr(ctx)
	chat := logsChat(b, ctx, nil)
	if chat == nil {
		return ext.EndGroups
	}
	settings := logchannels.GetContext(tracing.UpdateContext(ctx), chat.Id)
	if settings == nil {
		text, _ := tr.GetString("logs_not_set")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	text, _ := tr.GetString("logs_current", i18n.TranslationParams{
		"id": strconv.FormatInt(settings.LogChannelID, 10),
	})
	replyHTML(b, msg, text)
	return ext.EndGroups
}

func (moduleStruct) logCategories(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	if chat_status.RequireUser(b, ctx) == nil {
		return ext.EndGroups
	}
	tr := ctxTr(ctx)
	text, _ := tr.GetString("logs_categories")
	replyHTML(b, msg, text)
	return ext.EndGroups
}

func (m moduleStruct) enableLog(b *gotgbot.Bot, ctx *ext.Context) error {
	return m.setLogCategories(b, ctx, true)
}

func (m moduleStruct) disableLog(b *gotgbot.Bot, ctx *ext.Context) error {
	return m.setLogCategories(b, ctx, false)
}

func (moduleStruct) setLogCategories(b *gotgbot.Bot, ctx *ext.Context, enable bool) error {
	msg := ctx.EffectiveMessage
	from := chat_status.RequireUser(b, ctx)
	if from == nil {
		return ext.EndGroups
	}
	tr := ctxTr(ctx)
	chat := logsChat(b, ctx, from)
	if chat == nil {
		return ext.EndGroups
	}
	if logchannels.GetContext(tracing.UpdateContext(ctx), chat.Id) == nil {
		text, _ := tr.GetString("logs_not_set")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	args := ctx.Args()[1:]
	if len(args) == 0 {
		text, _ := tr.GetString("logs_usage")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	var unknown []string
	var changed []string
	for _, raw := range args {
		name := strings.ToLower(strings.TrimSpace(raw))
		if !logchannels.IsValidCategory(name) {
			unknown = append(unknown, name)
			continue
		}
		if err := logchannels.SetCategory(chat.Id, name, enable); err != nil {
			if err == gorm.ErrRecordNotFound {
				text, _ := tr.GetString("logs_not_set")
				replyHTML(b, msg, text)
				return ext.EndGroups
			}
			log.Error(err)
			continue
		}
		changed = append(changed, name)
	}
	if len(unknown) > 0 && len(changed) == 0 {
		text, _ := tr.GetString("logs_unknown_category")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	key := "logs_disabled"
	if enable {
		key = "logs_enabled"
	}
	text, _ := tr.GetString(key, i18n.TranslationParams{
		"cats": strings.Join(changed, ", "),
	})
	replyHTML(b, msg, text)
	actionlog.Settings(b, chat, from, key+" "+strings.Join(changed, ", "))
	return ext.EndGroups
}

func (moduleStruct) captureSetLogForward(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	if msg == nil || chat == nil || chat.Type == "private" || chat.Type == "channel" {
		return ext.ContinueGroups
	}
	if msg.ForwardOrigin == nil {
		return ext.ContinueGroups
	}
	// Telegram copies every post of a channel into its linked discussion
	// group on its own; a /setlog posted there arrives as such a copy, and no
	// group admin sent it.
	if msg.IsAutomaticForward {
		return ext.ContinueGroups
	}
	origin := msg.ForwardOrigin.MergeMessageOrigin()
	if origin.Chat == nil || origin.Chat.Type != "channel" {
		return ext.ContinueGroups
	}
	from := chat_status.RequireUser(b, ctx)
	// Telegram's own account is counted as an admin; it must not stand in for
	// a group admin binding the channel.
	if from == nil || from.Id == chat_status.TelegramUserID {
		return ext.ContinueGroups
	}
	if !chat_status.RequireUserAdmin(b, ctx, chat, from.Id) {
		return ext.ContinueGroups
	}
	if origin.MessageId == 0 {
		return ext.ContinueGroups
	}
	pending := false
	key := setlogPendingKey(origin.Chat.Id, origin.MessageId)
	if rdb := cache.GetRedisClient(); rdb != nil {
		// Atomic consume: concurrent forwards of the same channel message bind once.
		if err := rdb.GetDel(cache.Context, key).Err(); err == nil {
			pending = true
		}
	} else if m := cache.GetMarshal(); m != nil {
		var marker []byte
		if _, err := m.Get(cache.Context, key, &marker); err == nil {
			pending = true
			_ = m.Delete(cache.Context, key)
		}
	}
	if !pending {
		return ext.ContinueGroups
	}
	if err := logchannels.Set(chat.Id, chat.Title, origin.Chat.Id); err != nil {
		log.Errorf("[LogChannels] Set: %v", err)
		return ext.ContinueGroups
	}
	tr := ctxTr(ctx)
	text, _ := tr.GetString("logs_set_ok")
	replyHTML(b, msg, text)
	actionlog.Settings(b, chat, from, "set log channel")
	return ext.ContinueGroups
}

func LoadLogChannels(dispatcher *ext.Dispatcher) {
	SetModuleEnabled(logChannelsModule.moduleName, true)

	// Channel posts must reach /setlog: posting it in the channel starts the bind.
	// From PM each command but /logcategories takes the group first.
	dispatcher.AddHandler(handlers.NewCommand("setlog", withPMGroupArg("setlog", helpers.PMGroupArgOwn, logChannelsModule.setLog)).SetAllowChannel(true))
	dispatcher.AddHandler(handlers.NewCommand("unsetlog", withPMGroupArg("unsetlog", helpers.PMGroupArgNone, logChannelsModule.unsetLog)))
	dispatcher.AddHandler(handlers.NewCommand("logchannel", withPMGroupArg("logchannel", helpers.PMGroupArgNone, logChannelsModule.logChannel)))
	dispatcher.AddHandler(handlers.NewCommand("logcategories", logChannelsModule.logCategories))
	dispatcher.AddHandler(handlers.NewCommand("log", withPMGroupArg("log", helpers.PMGroupArgOwn, logChannelsModule.enableLog)))
	dispatcher.AddHandler(handlers.NewCommand("nolog", withPMGroupArg("nolog", helpers.PMGroupArgOwn, logChannelsModule.disableLog)))

	dispatcher.AddHandlerToGroup(
		handlers.NewMessage(message.All, logChannelsModule.captureSetLogForward),
		logChannelsModule.handlerGroup,
	)
}

func init() {
	RegisterLegacyModule("LogChannels", 55, LoadLogChannels)
}
