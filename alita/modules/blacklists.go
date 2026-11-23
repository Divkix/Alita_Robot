package modules

import (
	"errors"
	"fmt"
	"html"
	"slices"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/blacklists"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/enforce"
	"github.com/divkix/Alita_Robot/alita/i18n"
	chatsettings "github.com/divkix/Alita_Robot/alita/settings"
	"github.com/divkix/Alita_Robot/alita/utils/actionlog"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/extraction"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	"github.com/divkix/Alita_Robot/alita/utils/keyword_matcher"
	"github.com/divkix/Alita_Robot/alita/utils/punishment"
	"github.com/divkix/Alita_Robot/alita/utils/tracing"
)

var blacklistsModule = moduleStruct{
	moduleName:   "Blacklists",
	handlerGroup: 7,
}

// blacklistTimedActionTime is the fallback for tban/tmute rows without a usable
// duration, such as backups taken before action_time existed.
const blacklistTimedActionTime = 60 * 60

func blacklistActionTime(settings *models.BlacklistSettings) int {
	if !models.ValidActionTime(settings.ActionTime) {
		return blacklistTimedActionTime
	}
	return settings.ActionTime
}

// blacklistModeLabel includes the effective duration for timed actions.
func blacklistModeLabel(settings models.BlacklistSettingsSlice) string {
	action := settings.Action()
	if action == "tban" || action == "tmute" {
		return action + " " + formatDuration(blacklistActionTime(settings[0]))
	}
	return action
}

func (m moduleStruct) addBlacklist(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	args := ctx.Args()[1:]
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	var (
		alreadyBlacklisted, newBlacklist []string
		text                             string
	)

	if len(args) == 0 {
		text := tr.String(strings.ToLower(m.moduleName) + "_blacklist_give_bl_word")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			return err
		}
		return ext.EndGroups
	} else if len(args) >= 1 {
		allBlWords := blacklists.GetBlacklistSettingsContext(tracing.UpdateContext(ctx), chat.Id).Triggers()

		blWordSet := make(map[string]struct{}, len(allBlWords))
		for _, w := range allBlWords {
			blWordSet[w] = struct{}{}
		}

		var tooLong []string
		validArgs := make([]string, 0, len(args))
		for _, word := range args {
			runes := []rune(word)
			if len(runes) > 100 {
				preview := word
				if len(runes) > 20 {
					preview = string(runes[:20]) + "..."
				}
				tooLong = append(tooLong, html.EscapeString(preview))
			} else {
				validArgs = append(validArgs, word)
			}
		}
		if len(tooLong) > 0 {
			text := tr.String(strings.ToLower(m.moduleName) + "_blacklist_word_too_long")
			_, err := msg.Reply(b, fmt.Sprintf(text, strings.Join(tooLong, ", ")), formatting.Shtml())
			if err != nil {
				log.Error(err)
			}
		}
		if len(validArgs) == 0 {
			return ext.EndGroups
		}
		args = validArgs

		saveFailed := false
		for _, blWord := range args {
			blWord = strings.ToLower(blWord)
			if _, exists := blWordSet[blWord]; exists {
				alreadyBlacklisted = append(alreadyBlacklisted, fmt.Sprintf("<code>%s</code>", html.EscapeString(blWord)))
				continue
			}
			if err := blacklists.AddBlacklist(chat.Id, blWord); err != nil {
				log.WithFields(log.Fields{
					"chatId": chat.Id,
					"word":   blWord,
					"error":  err,
				}).Error("Failed to add blacklist")
				saveFailed = true
				continue
			}
			blWordSet[blWord] = struct{}{}
			newBlacklist = append(newBlacklist, fmt.Sprintf("<code>%s</code>", html.EscapeString(blWord)))
		}

		if len(alreadyBlacklisted) >= 1 {
			temp := tr.String(strings.ToLower(m.moduleName) + "_blacklist_already_blacklisted")
			text += temp + fmt.Sprintf("\n - %s\n\n", strings.Join(alreadyBlacklisted, "\n - "))
		}
		if len(newBlacklist) >= 1 {
			temp := tr.String(strings.ToLower(m.moduleName) + "_blacklist_added_bl")
			text += temp + fmt.Sprintf("\n - %s\n\n", strings.Join(newBlacklist, "\n - "))
		}
		if saveFailed {
			temp := tr.String("common_settings_save_failed")
			text += temp
		}

		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			return err
		}
	}

	return ext.EndGroups
}

func (m moduleStruct) removeBlacklist(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	args := ctx.Args()[1:]
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	var removedBlacklists []string

	if len(args) == 0 {
		text := tr.String(strings.ToLower(m.moduleName) + "_unblacklist_give_bl_word")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			return err
		}
		return ext.EndGroups
	} else {
		allBlWords := blacklists.GetBlacklistSettingsContext(tracing.UpdateContext(ctx), chat.Id).Triggers()
		for _, blWord := range args {
			blWord = strings.ToLower(blWord)
			if slices.Contains(allBlWords, blWord) {
				if err := blacklists.RemoveBlacklist(chat.Id, blWord); err != nil {
					log.WithFields(log.Fields{
						"chatId": chat.Id,
						"word":   blWord,
						"error":  err,
					}).Error("Failed to remove blacklist")
					continue
				}
				removedBlacklists = append(removedBlacklists, blWord)
			}
		}
		if len(removedBlacklists) <= 0 {
			text := tr.String(strings.ToLower(m.moduleName) + "_unblacklist_no_removed_bl")
			_, err := msg.Reply(b, text, nil)
			if err != nil {
				return err
			}
		} else {
			temp := tr.String(strings.ToLower(m.moduleName) + "_unblacklist_removed_bl")
			_, err := msg.Reply(b, fmt.Sprintf(temp, strings.Join(removedBlacklists, ", ")), nil)
			if err != nil {
				return err
			}
		}
	}
	return ext.EndGroups
}

func (m moduleStruct) listBlacklists(b *gotgbot.Bot, ctx *ext.Context) error {
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat

	var (
		replyMsgId     int64
		blacklistsText string
	)

	if reply := msg.ReplyToMessage; reply != nil {
		replyMsgId = reply.MessageId
	} else {
		replyMsgId = msg.MessageId
	}

	blSrc := blacklists.GetBlacklistSettingsContext(tracing.UpdateContext(ctx), chat.Id)
	triggers := blSrc.Triggers()
	slices.Sort(triggers)
	var sb strings.Builder
	for _, i := range triggers {
		fmt.Fprintf(&sb, "\n - <code>%s</code>", html.EscapeString(i))
	}
	blacklistsText += sb.String()

	if blacklistsText != "" {
		temp := tr.String(strings.ToLower(m.moduleName) + "_ls_bl_list_bl")
		blacklistsText = temp + blacklistsText
	} else {
		blacklistsText = tr.String(strings.ToLower(m.moduleName) + "_ls_bl_no_blacklisted")
	}

	_, err := msg.Reply(b,
		blacklistsText,
		&gotgbot.SendMessageOpts{
			ReplyParameters: &gotgbot.ReplyParameters{
				MessageId:                replyMsgId,
				AllowSendingWithoutReply: true,
			},
			ParseMode: formatting.HTML,
		},
	)
	if err != nil {
		return err
	}

	return ext.EndGroups
}

func (m moduleStruct) setBlacklistAction(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	args := ctx.Args()[1:]
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	var rMsg string

	if len(args) == 0 {
		currAction := blacklistModeLabel(blacklists.GetBlacklistSettingsContext(tracing.UpdateContext(ctx), chat.Id))
		temp := tr.String(strings.ToLower(m.moduleName) + "_set_bl_action_current_mode")
		rMsg = fmt.Sprintf(temp, currAction)
	} else {
		action := strings.ToLower(args[0])
		timed := action == "tban" || action == "tmute"
		if slices.Contains([]string{"mute", "kick", "warn", "ban", "none", "tban", "tmute"}, action) && (len(args) == 1 || timed) {
			actionTime := 0
			if timed {
				seconds := extraction.ExtractDuration(b, ctx, strings.Join(args[1:], " "))
				if seconds == -1 {
					return ext.EndGroups
				}
				actionTime = int(seconds)
			}
			if err := blacklists.SetBlacklistAction(chat.Id, action, actionTime); errors.Is(err, blacklists.ErrNoBlacklistWords) {
				rMsg = tr.String("blacklists_set_bl_action_no_words")
			} else if err != nil {
				log.WithFields(log.Fields{
					"chatId": chat.Id,
					"action": action,
					"error":  err,
				}).Error("[Blacklists] Failed to persist blacklist action")
				rMsg = tr.String("blacklists_set_bl_action_update_failed")
			} else {
				key := strings.ToLower(m.moduleName) + "_set_bl_action_changed_mode"
				value := action
				if timed {
					key = "blist_action_" + action
					value = formatDuration(actionTime)
				}
				temp := tr.String(key)
				rMsg = fmt.Sprintf(temp, value)
			}
		} else {
			rMsg = tr.String(strings.ToLower(m.moduleName) + "_set_bl_action_choose_correct_option")
		}
	}
	_, err := msg.Reply(b, rMsg, formatting.Smarkdown())
	if err != nil {
		return err
	}
	return ext.EndGroups
}

// blacklistsClearAll is the /remallbl owner confirmation: the shared
// skeleton asks, authorizes and answers; these closures hold only the
// blacklists work.
var blacklistsClearAll = ownerConfirmSpec{
	Namespace: "rmAllBlacklist",
	AskKey:    strings.ToLower(blacklistsModule.moduleName) + "_rm_all_bl_ask",
	Commit: func(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, tr *i18n.Translator, actor *gotgbot.User) (string, i18n.TranslationParams, error) {
		if err := blacklists.RemoveAllBlacklist(chat.Id); err != nil {
			log.WithFields(log.Fields{"chatId": chat.Id, "error": err}).Error("Failed to remove all blacklists")
			return "error_generic", nil, nil
		}
		return "", nil, nil
	},
	Keys: ownerConfirmKeys{
		Confirmed: strings.ToLower(blacklistsModule.moduleName) + "_rm_all_bl_done",
		Cancelled: strings.ToLower(blacklistsModule.moduleName) + "_rm_all_bl_cancel",
		Failed:    "error_generic",
	},
}

var blacklistsClearAllMessage, blacklistsClearAllCallback = registerOwnerConfirm(blacklistsClearAll)

func (m moduleStruct) rmAllBlacklists(b *gotgbot.Bot, ctx *ext.Context) error {
	return blacklistsClearAllMessage(b, ctx)
}

// blacklistNoticeKeys names each punishing action's notice.
var blacklistNoticeKeys = map[string]string{
	"mute":  "_bl_watcher_muted_user",
	"ban":   "_bl_watcher_banned_user",
	"kick":  "_bl_watcher_kicked_user",
	"tban":  "_bl_watcher_tbanned_user",
	"tmute": "_bl_watcher_tmuted_user",
}

func (m moduleStruct) blacklistWatcher(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	user := ctx.EffectiveSender
	if user == nil {
		return ext.ContinueGroups
	}
	if user.IsAnonymousAdmin() {
		return ext.ContinueGroups
	}

	blSettings := blacklists.GetBlacklistSettingsContext(tracing.UpdateContext(ctx), chat.Id)
	triggers := blSettings.Triggers()
	if len(triggers) == 0 {
		return ext.ContinueGroups
	}

	msg := ctx.EffectiveMessage
	matchText := buildModerationMatchText(msg)
	if matchText == "" {
		return ext.ContinueGroups
	}

	cache := keyword_matcher.GetNamedCache("blacklists")
	matcher := cache.GetOrCreateMatcher(chat.Id, triggers)

	firstPattern, found := matcher.FirstMatch(matchText)
	if !found {
		return ext.ContinueGroups
	}
	i := firstPattern
	matched := blSettings.Find(i)
	if matched == nil {
		return ext.ContinueGroups
	}

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	reason := matched.Reason
	if reason == "" || reason == models.DefaultBlacklistReason {
		reason = models.DefaultBlacklistReason
		if translated, _ := tr.GetString("db_blacklist_default_reason"); translated != "" {
			reason = translated
		}
	}

	if matched.Action == "warn" {
		// The warn delegate is not a punishment spec: it keeps this watcher's
		// own exemption and bot-admin guards (declared divergence — an
		// approved or exempt sender must not be warned either), deletes the
		// trigger, then hands off to warns' own chain.
		if user.IsUser() && chat_status.IsProtectedForUpdate(b, ctx, chat.Id, user.Id()) {
			return ext.ContinueGroups
		}
		if !chat_status.IsBotAdminForUpdate(b, ctx, chat) {
			return ext.ContinueGroups
		}
		_ = helpers.DeleteMessageWithErrorHandling(b, chat.Id, msg.MessageId)
		if user.IsAnonymousChannel() {
			return ext.ContinueGroups
		}
		err := warnsModule.warnThisUser(b, ctx, user.Id(), fmt.Sprintf(reason, i), "warn", models.WarnEntry{WarnedBy: b.Id})
		if err != nil && !errors.Is(err, ext.EndGroups) && !errors.Is(err, ext.ContinueGroups) {
			log.Error(err)
		}
		if err != nil {
			return err
		}
		return ext.ContinueGroups
	}

	key, ok := blacklistNoticeKeys[matched.Action]

	// banReason is the reason a ban's record keeps: plain text, unlike the
	// notice's.
	banReason := fmt.Sprintf(reason, i)

	// A channel is banned for good or left alone: the module refuses it any
	// other mode, and banChatSenderChat takes no until_date.
	mode, timed := chatsettings.PunishmentMode(matched.Action)
	request := punishment.Request{
		ChatID: chat.Id, Target: punishment.Target{ID: user.Id(), SenderChat: user.IsAnonymousChannel()},
		Mode: mode, Source: models.BanSourceBlacklist, Reason: banReason,
	}
	action := matched.Action
	params := i18n.TranslationParams{"0": formatting.MentionHtml(user.Id(), user.Name())}
	if timed {
		actionTime := blacklistActionTime(matched)
		request.Duration = time.Duration(actionTime) * time.Second
		action += " " + formatDuration(actionTime)
		params["1"] = formatDuration(actionTime)
		params["2"] = fmt.Sprintf(reason, html.EscapeString(i))
	} else {
		params["1"] = fmt.Sprintf(reason, html.EscapeString(i))
	}

	spec := enforce.Spec{
		Request:         request,
		PermissionCheck: chat_status.IsBotAdminForUpdate,
		BeforeApply: func() {
			_ = helpers.DeleteMessageWithErrorHandling(b, chat.Id, msg.MessageId)
		},
	}
	if ok {
		spec.Log = &enforce.LogSpec{
			Tag: "BLACKLIST",
			// The matched trigger is a log field of its own.
			Fields: []actionlog.Field{
				{Name: "Action", Value: action},
				{Name: "Trigger", Value: i},
			},
		}
		spec.Notice = &enforce.NoticeSpec{
			Key:    strings.ToLower(m.moduleName) + key,
			Params: params,
			Raw:    true,
		}
	}
	if _, err := enforce.Automated(b, ctx, spec); err != nil {
		return err
	}
	return ext.ContinueGroups
}

func LoadBlacklists(dispatcher *ext.Dispatcher) {
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:           "blacklists",
		Aliases:        []string{"blocklists"},
		PMGroupArg:     helpers.PMGroupArgNone,
		Disableable:    true,
		RequiredChecks: []helpers.Check{helpers.RequireConnectedMember(helpers.BotAdminRequired)},
	}, pipelineHandler(blacklistsModule.listBlacklists))
	restrictChecks := []helpers.Check{helpers.RequireConnectedAdmin(helpers.BotAdminRequired), helpers.CanUserRestrict(), helpers.CanBotRestrict()}
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:           "addblacklist",
		Aliases:        []string{"blacklist", "addblocklist", "blocklist"},
		PMGroupArg:     helpers.PMGroupArgOwn,
		RequiredChecks: restrictChecks,
	}, pipelineHandler(blacklistsModule.addBlacklist))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:           "rmblacklist",
		Aliases:        []string{"rmblocklist", "unblacklist"},
		PMGroupArg:     helpers.PMGroupArgOwn,
		RequiredChecks: restrictChecks,
	}, pipelineHandler(blacklistsModule.removeBlacklist))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:           "blaction",
		Aliases:        []string{"blacklistaction"},
		PMGroupArg:     helpers.PMGroupArgOwn,
		RequiredChecks: restrictChecks,
	}, pipelineHandler(blacklistsModule.setBlacklistAction))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:           "remallbl",
		Aliases:        []string{"rmallbl", "rmblocklistall", "rmallblocklist"},
		PMGroupArg:     helpers.PMGroupArgNone,
		RequiredChecks: []helpers.Check{groupOnlyChatCheck, helpers.RequireGroup(), helpers.RequireUserOwner()},
	}, pipelineHandler(blacklistsModule.rmAllBlacklists))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("rmAllBlacklist|"), blacklistsClearAllCallback))
	dispatcher.AddHandlerToGroup(handlers.NewMessage(func(msg *gotgbot.Message) bool {
		return msg.Text != "" || msg.Caption != ""
	}, blacklistsModule.blacklistWatcher), blacklistsModule.handlerGroup)
}

func init() {
	RegisterLegacyModule("Blacklists", 240, LoadBlacklists)
}
