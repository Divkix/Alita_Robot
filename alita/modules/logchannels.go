package modules

import (
	"errors"
	"html"
	"strconv"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/divkix/Alita_Robot/alita/db/logchannels"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/actionlog"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	"github.com/divkix/Alita_Robot/alita/utils/opstate"
	"github.com/divkix/Alita_Robot/alita/utils/tracing"
)

const logHandlerGroup = 11

var logChannelsModule = moduleStruct{
	moduleName:   "LogChannels",
	handlerGroup: logHandlerGroup,
}

func setlogPendingKey(channelID, messageID int64) string {
	return opstate.Key("setlog", channelID, messageID)
}

// setLogChannelCommand is the senderless channel-post step, never a group or PM command.
type setLogChannelCommand struct {
	handlers.Command
}

func (h setLogChannelCommand) CheckUpdate(b *gotgbot.Bot, ctx *ext.Context) bool {
	return ctx.ChannelPost != nil && h.Command.CheckUpdate(b, ctx)
}

func (moduleStruct) setLogChannel(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	tr := ctxTr(ctx)
	if err := opstate.Stash(setlogPendingKey(msg.Chat.Id, msg.MessageId), []byte("1"), time.Hour); err != nil {
		log.Errorf("[LogChannels] failed to store pending setlog: %v", err)
		text := tr.String("common_settings_save_failed")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	text := tr.String("logs_forward_to_group")
	replyHTML(b, msg, text)
	return ext.EndGroups
}

func (moduleStruct) setLog(c *helpers.CommandContext) error {
	b, msg, tr := c.Bot, c.Msg, c.Tr
	if msg.Chat.Type == "private" {
		return setLogFromPM(c)
	}
	// The group-11 watcher owns the forwarded bind step.
	if msg.ForwardOrigin != nil {
		return ext.ContinueGroups
	}
	text := tr.String("logs_need_channel")
	replyHTML(b, msg, text)
	return ext.EndGroups
}

// setLogFromPM binds the channel named after the group: "/setlog <group>
// <channel>", with the group already cut by the descriptor's PM resolver or left out on a
// connection. No post in the channel proves who runs it, so Telegram must:
// the user is an admin of the group and of the channel, and the bot may post
// there.
func setLogFromPM(c *helpers.CommandContext) error {
	b, ctx, msg, from, group, tr := c.Bot, c.Ctx, c.Msg, c.User, c.Chat, c.Tr
	args := ctx.Args()[1:]
	switch {
	case len(args) == 0:
		text := tr.String("logs_pm_need_channel")
		replyHTML(b, msg, text)
		return ext.EndGroups
	case len(args) > 1:
		// On a connection a title is the command's own argument, so
		// /setlog "My Group" @chan gets here with the channel named.
		text := tr.String("logs_pm_setlog_usage")
		if err := replyCommandHelp(b, msg, tr, "setlog", text); err != nil {
			log.Error(err)
		}
		return ext.EndGroups
	}
	channel, err := chat_status.GetChat(b, args[0])
	if err != nil {
		log.WithFields(log.Fields{"channel": args[0], "error": err}).Debug("[LogChannels] channel lookup failed")
	}
	if err != nil || channel.Type != "channel" {
		text := tr.String("logs_pm_not_channel", i18n.TranslationParams{"channel": html.EscapeString(args[0])})
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	if !helpers.RunChecks(c, []helpers.Check{requireLogChannelPosting(channel.Id)}) {
		return ext.EndGroups
	}
	if err := logchannels.Set(group.Id, group.Title, channel.Id); err != nil {
		log.Errorf("[LogChannels] Set: %v", err)
		text := tr.String("common_settings_save_failed")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	// The reply goes to PM, where "this chat" would read as the PM.
	name := group.Title
	if strings.TrimSpace(name) == "" {
		name = strconv.FormatInt(group.Id, 10)
	}
	text := tr.String("logs_pm_set_ok", i18n.TranslationParams{"group": html.EscapeString(name)})
	replyHTML(b, msg, text)
	actionlog.Settings(b, group, from, "set log channel")
	return ext.EndGroups
}

// requireLogChannelPosting runs after the channel argument is resolved. Telegram
// requires bot posting rights before it can reliably answer the actor lookup.
func requireLogChannelPosting(channelID int64) helpers.Check {
	return helpers.NewCheck(helpers.CheckKindAny, func(c *helpers.CommandContext) bool {
		key := ""
		if !canPostInChannel(c.Bot, c.Ctx, channelID, c.Bot.Id) {
			key = "logs_pm_bot_cannot_post"
		} else if !canPostInChannel(c.Bot, c.Ctx, channelID, c.User.Id) {
			key = "logs_pm_user_not_channel_admin"
		}
		if key != "" {
			chat_status.NewPermissionResponder(c.Bot).Respond(c.Ctx, key, "", chat_status.WithReply())
			return false
		}
		return true
	})
}

// canPostInChannel reports whether userID owns the channel or is an admin of
// it that can post messages.
func canPostInChannel(b *gotgbot.Bot, ctx *ext.Context, channelID, userID int64) bool {
	if !chat_status.IsValidUserId(userID) || chat_status.IsSystemUser(userID) {
		return false
	}
	member, err := b.GetChatMemberWithContext(tracing.UpdateContext(ctx), channelID, userID, nil)
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

// logAdminChatCheck preserves group refusals and connected-PM admin checks.
var logAdminChatCheck = checkInPMOrGroup(
	helpers.RequireConnectedAdmin(helpers.BotAdminNotRequired),
	helpers.RequireUserAdmin(),
)

func (moduleStruct) unsetLog(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	from := ctx.EffectiveSender.User
	tr := ctxTr(ctx)
	chat := ctx.EffectiveChat
	// Read the settings first: Unset deletes them, and the change is logged
	// to the channel that was just unset.
	prev := logchannels.Get(chat.Id)
	if err := logchannels.Unset(chat.Id); err != nil {
		text := tr.String("logs_not_set")
		replyHTML(b, msg, withGroupLine(tr, msg, chat, text))
		return ext.EndGroups
	}
	text := tr.String("logs_unset_ok")
	replyHTML(b, msg, withGroupLine(tr, msg, chat, text))
	actionlog.SettingsTo(b, chat, prev, from, "unset log channel")
	return ext.EndGroups
}

func (moduleStruct) logChannel(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	tr := ctxTr(ctx)
	chat := ctx.EffectiveChat
	settings := logchannels.GetContext(tracing.UpdateContext(ctx), chat.Id)
	if settings == nil {
		text := tr.String("logs_not_set")
		replyHTML(b, msg, withGroupLine(tr, msg, chat, text))
		return ext.EndGroups
	}
	text := tr.String("logs_current", i18n.TranslationParams{
		"id": strconv.FormatInt(settings.LogChannelID, 10),
	})
	replyHTML(b, msg, withGroupLine(tr, msg, chat, text))
	return ext.EndGroups
}

func (moduleStruct) logCategories(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	tr := ctxTr(ctx)
	text := tr.String("logs_categories")
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
	from := ctx.EffectiveSender.User
	tr := ctxTr(ctx)
	chat := ctx.EffectiveChat
	if logchannels.GetContext(tracing.UpdateContext(ctx), chat.Id) == nil {
		text := tr.String("logs_not_set")
		replyHTML(b, msg, withGroupLine(tr, msg, chat, text))
		return ext.EndGroups
	}
	args := ctx.Args()[1:]
	if len(args) == 0 {
		text := tr.String("logs_usage")
		replyHTML(b, msg, withGroupLine(tr, msg, chat, text))
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
			if errors.Is(err, gorm.ErrRecordNotFound) {
				text := tr.String("logs_not_set")
				replyHTML(b, msg, withGroupLine(tr, msg, chat, text))
				return ext.EndGroups
			}
			log.Error(err)
			continue
		}
		changed = append(changed, name)
	}
	if len(unknown) > 0 && len(changed) == 0 {
		text := tr.String("logs_unknown_category")
		replyHTML(b, msg, withGroupLine(tr, msg, chat, text))
		return ext.EndGroups
	}
	key := "logs_disabled"
	if enable {
		key = "logs_enabled"
	}
	text := tr.String(key, i18n.TranslationParams{
		"cats": strings.Join(changed, ", "),
	})
	replyHTML(b, msg, withGroupLine(tr, msg, chat, text))
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
	// Atomic consume: concurrent forwards of the same channel message bind once.
	consumed, err := opstate.Consume[[]byte](setlogPendingKey(origin.Chat.Id, origin.MessageId))
	if err != nil {
		log.Debugf("[LogChannels] consume pending setlog: %v", err)
	}
	if consumed == nil {
		return ext.ContinueGroups
	}
	if err := logchannels.Set(chat.Id, chat.Title, origin.Chat.Id); err != nil {
		log.Errorf("[LogChannels] Set: %v", err)
		return ext.ContinueGroups
	}
	tr := ctxTr(ctx)
	text := tr.String("logs_set_ok")
	replyHTML(b, msg, text)
	actionlog.Settings(b, chat, from, "set log channel")
	return ext.ContinueGroups
}

func LoadLogChannels(dispatcher *ext.Dispatcher) {
	// Only this senderless channel-post step stays outside the pipeline.
	dispatcher.AddHandler(setLogChannelCommand{handlers.NewCommand("setlog", logChannelsModule.setLogChannel).SetAllowChannel(true)})
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "setlog", PMGroupArg: helpers.PMGroupArgOwn,
		// The group form only asks for a forwarded channel post; the watcher
		// that binds it requires group admin, so the menu keeps it admin-only.
		AdminOnlySet: true, AdminOnly: true,
		RequiredChecks: []helpers.Check{checkInPMOrGroup(helpers.RequireConnectedAdmin(helpers.BotAdminNotRequired), helpers.Check{})},
	}, logChannelsModule.setLog)
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "unsetlog", PMGroupArg: helpers.PMGroupArgNone,
		RequiredChecks: []helpers.Check{logAdminChatCheck},
	}, pipelineHandler(logChannelsModule.unsetLog))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "logchannel", PMGroupArg: helpers.PMGroupArgNone,
		RequiredChecks: []helpers.Check{helpers.RequireConnectedMember(helpers.BotAdminNotRequired)},
	}, pipelineHandler(logChannelsModule.logChannel))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "logcategories",
	}, pipelineHandler(logChannelsModule.logCategories))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "log", PMGroupArg: helpers.PMGroupArgOwn,
		RequiredChecks: []helpers.Check{logAdminChatCheck},
	}, pipelineHandler(logChannelsModule.enableLog))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "nolog", PMGroupArg: helpers.PMGroupArgOwn,
		RequiredChecks: []helpers.Check{logAdminChatCheck},
	}, pipelineHandler(logChannelsModule.disableLog))

	dispatcher.AddHandlerToGroup(
		handlers.NewMessage(message.All, logChannelsModule.captureSetLogForward),
		logChannelsModule.handlerGroup,
	)
}

func init() {
	RegisterLegacyModule("LogChannels", 55, LoadLogChannels)
}
