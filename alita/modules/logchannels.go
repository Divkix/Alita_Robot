package modules

import (
	"fmt"
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
	from := requireUser(b, ctx)
	if from == nil {
		return ext.EndGroups
	}
	tr := ctxTr(ctx)
	if chat == nil || msg == nil {
		return ext.EndGroups
	}
	if chat.Type == "channel" {
		if m := cache.GetMarshal(); m != nil {
			if err := m.Set(
				cache.Context,
				setlogPendingKey(chat.Id, msg.MessageId),
				[]byte("1"),
				store.WithExpiration(time.Hour),
			); err != nil {
				log.Debugf("[LogChannels] failed to store pending setlog: %v", err)
			}
			if err := m.Set(
				cache.Context,
				setlogPendingKey(chat.Id, 0),
				[]byte("1"),
				store.WithExpiration(time.Hour),
			); err != nil {
				log.Debugf("[LogChannels] failed to store pending setlog channel: %v", err)
			}
		}
		text, _ := tr.GetString("logs_forward_to_group")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	text, _ := tr.GetString("logs_need_channel")
	replyHTML(b, msg, text)
	return ext.EndGroups
}

func (moduleStruct) unsetLog(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	from := chat_status.RequireUser(b, ctx)
	if from == nil {
		return ext.EndGroups
	}
	tr := ctxTr(ctx)
	if chat == nil || chat.Type == "private" {
		text, _ := tr.GetString("logs_group_only")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(b, ctx, chat, from.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
		return ext.EndGroups
	}
	if err := logchannels.Unset(chat.Id); err != nil {
		text, _ := tr.GetString("logs_not_set")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	text, _ := tr.GetString("logs_unset_ok")
	replyHTML(b, msg, text)
	actionlog.Settings(b, chat, from, "unset log channel")
	return ext.EndGroups
}

func (moduleStruct) logChannel(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	if chat_status.RequireUser(b, ctx) == nil {
		return ext.EndGroups
	}
	tr := ctxTr(ctx)
	if chat == nil || chat.Type == "private" {
		text, _ := tr.GetString("logs_group_only")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	settings := logchannels.Get(chat.Id)
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
	chat := ctx.EffectiveChat
	from := chat_status.RequireUser(b, ctx)
	if from == nil {
		return ext.EndGroups
	}
	tr := ctxTr(ctx)
	if chat == nil || chat.Type == "private" {
		text, _ := tr.GetString("logs_group_only")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(b, ctx, chat, from.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
		return ext.EndGroups
	}
	if logchannels.Get(chat.Id) == nil {
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
	origin := msg.ForwardOrigin.MergeMessageOrigin()
	if origin.Chat == nil || origin.Chat.Type != "channel" {
		return ext.ContinueGroups
	}
	from := chat_status.RequireUser(b, ctx)
	if from == nil {
		return ext.ContinueGroups
	}
	if !chat_status.RequireUserAdmin(b, ctx, chat, from.Id) {
		return ext.ContinueGroups
	}
	keys := []string{setlogPendingKey(origin.Chat.Id, 0)}
	if origin.MessageId != 0 {
		keys = append([]string{setlogPendingKey(origin.Chat.Id, origin.MessageId)}, keys...)
	}
	pending := false
	if m := cache.GetMarshal(); m != nil {
		for _, key := range keys {
			var marker []byte
			if _, err := m.Get(cache.Context, key, &marker); err == nil {
				pending = true
				_ = m.Delete(cache.Context, key)
			}
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

// LoadLogChannels registers log-channel commands and the forward watcher.
func LoadLogChannels(dispatcher *ext.Dispatcher) {
	DefaultHelpRegistry().AbleMap[logChannelsModule.moduleName] = true

	dispatcher.AddHandler(handlers.NewCommand("setlog", logChannelsModule.setLog))
	dispatcher.AddHandler(handlers.NewCommand("unsetlog", logChannelsModule.unsetLog))
	dispatcher.AddHandler(handlers.NewCommand("logchannel", logChannelsModule.logChannel))
	dispatcher.AddHandler(handlers.NewCommand("logcategories", logChannelsModule.logCategories))
	dispatcher.AddHandler(handlers.NewCommand("log", logChannelsModule.enableLog))
	dispatcher.AddHandler(handlers.NewCommand("nolog", logChannelsModule.disableLog))

	dispatcher.AddHandlerToGroup(
		handlers.NewMessage(message.All, logChannelsModule.captureSetLogForward),
		logChannelsModule.handlerGroup,
	)
}

func init() {
	RegisterLegacyModule("LogChannels", 55, LoadLogChannels)
}
