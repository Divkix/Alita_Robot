package modules

import (
	"fmt"
	"html"
	"slices"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/blacklists"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	"github.com/divkix/Alita_Robot/alita/utils/keyword_matcher"
)

var blacklistsModule = moduleStruct{
	moduleName:   "Blacklists",
	handlerGroup: 7,
}

func (m moduleStruct) addBlacklist(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	connectedChat := chat_status.IsUserConnected(b, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	chat := connectedChat
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	args := ctx.Args()[1:]
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	var (
		alreadyBlacklisted, newBlacklist []string
		text                             string
	)

	if !chat_status.IsUserAdmin(b, chat.Id, user.Id) {
		return ext.EndGroups
	}
	if !chat_status.IsBotAdmin(b, ctx, chat) {
		return ext.EndGroups
	}
	if !chat_status.CanUserRestrict(b, ctx, chat, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_restrict_cmd_error", "chat_status_restrict_button_error")
		return ext.EndGroups
	}
	if !chat_status.CanBotRestrict(b, ctx, chat) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_bot_restrict_group_error", "chat_status_bot_restrict_error")
		return ext.EndGroups
	}

	if len(args) == 0 {
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_blacklist_give_bl_word")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	} else if len(args) >= 1 {
		allBlWords := blacklists.GetBlacklistSettings(chat.Id).Triggers()

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
				tooLong = append(tooLong, preview)
			} else {
				validArgs = append(validArgs, word)
			}
		}
		if len(tooLong) > 0 {
			text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_blacklist_word_too_long")
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
				alreadyBlacklisted = append(alreadyBlacklisted, blWord)
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
			temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_blacklist_already_blacklisted")
			text += temp + fmt.Sprintf("\n - %s\n\n", strings.Join(alreadyBlacklisted, "\n - "))
		}
		if len(newBlacklist) >= 1 {
			temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_blacklist_added_bl")
			text += temp + fmt.Sprintf("\n - %s\n\n", strings.Join(newBlacklist, "\n - "))
		}
		if saveFailed {
			temp, _ := tr.GetString("common_settings_save_failed")
			text += temp
		}

		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
	}

	return ext.EndGroups
}

func (m moduleStruct) removeBlacklist(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	connectedChat := chat_status.IsUserConnected(b, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	chat := connectedChat
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	args := ctx.Args()[1:]
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	var removedBlacklists []string

	if !chat_status.IsUserAdmin(b, chat.Id, user.Id) {
		return ext.EndGroups
	}
	if !chat_status.IsBotAdmin(b, ctx, chat) {
		return ext.EndGroups
	}
	if !chat_status.CanUserRestrict(b, ctx, chat, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_restrict_cmd_error", "chat_status_restrict_button_error")
		return ext.EndGroups
	}
	if !chat_status.CanBotRestrict(b, ctx, chat) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_bot_restrict_group_error", "chat_status_bot_restrict_error")
		return ext.EndGroups
	}

	if len(args) == 0 {
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_unblacklist_give_bl_word")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	} else {
		allBlWords := blacklists.GetBlacklistSettings(chat.Id).Triggers()
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
			text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_unblacklist_no_removed_bl")
			_, err := msg.Reply(b, text, nil)
			if err != nil {
				log.Error(err)
				return err
			}
		} else {
			temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_unblacklist_removed_bl")
			_, err := msg.Reply(b, fmt.Sprintf(temp, strings.Join(removedBlacklists, ", ")), nil)
			if err != nil {
				log.Error(err)
				return err
			}
		}
	}
	return ext.EndGroups
}

func (m moduleStruct) listBlacklists(b *gotgbot.Bot, ctx *ext.Context) error {
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	msg := ctx.EffectiveMessage
	if chat_status.CheckDisabledCmd(b, msg, "blacklists") {
		return ext.EndGroups
	}
	connectedChat := chat_status.IsUserConnected(b, ctx, false, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	chat := connectedChat

	var (
		replyMsgId     int64
		blacklistsText string
	)

	if reply := msg.ReplyToMessage; reply != nil {
		replyMsgId = reply.MessageId
	} else {
		replyMsgId = msg.MessageId
	}

	blSrc := blacklists.GetBlacklistSettings(chat.Id)
	triggers := blSrc.Triggers()
	slices.Sort(triggers)
	var sb strings.Builder
	for _, i := range triggers {
		fmt.Fprintf(&sb, "\n - <code>%s</code>", html.EscapeString(i))
	}
	blacklistsText += sb.String()

	if blacklistsText != "" {
		temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_ls_bl_list_bl")
		blacklistsText = temp + blacklistsText
	} else {
		blacklistsText, _ = tr.GetString(strings.ToLower(m.moduleName) + "_ls_bl_no_blacklisted")
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
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func (m moduleStruct) setBlacklistAction(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	connectedChat := chat_status.IsUserConnected(b, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	chat := connectedChat
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	args := ctx.Args()[1:]
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	var rMsg string

	if !chat_status.CanUserRestrict(b, ctx, chat, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_restrict_cmd_error", "chat_status_restrict_button_error")
		return ext.EndGroups
	}
	if !chat_status.CanBotRestrict(b, ctx, chat) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_bot_restrict_group_error", "chat_status_bot_restrict_error")
		return ext.EndGroups
	}

	if len(args) == 0 {
		currAction := blacklists.GetBlacklistSettings(chat.Id).Action()
		temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_set_bl_action_current_mode")
		rMsg = fmt.Sprintf(temp, currAction)
	} else if len(args) == 1 {
		action := strings.ToLower(args[0])
		if slices.Contains([]string{"mute", "kick", "warn", "ban", "none"}, action) {
			if err := blacklists.SetBlacklistAction(chat.Id, action); err != nil {
				log.WithFields(log.Fields{
					"chatId": chat.Id,
					"action": action,
					"error":  err,
				}).Error("[Blacklists] Failed to persist blacklist action")
				rMsg, _ = tr.GetString("blacklists_set_bl_action_update_failed")
			} else {
				temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_set_bl_action_changed_mode")
				rMsg = fmt.Sprintf(temp, action)
			}
		} else {
			rMsg, _ = tr.GetString(strings.ToLower(m.moduleName) + "_set_bl_action_choose_correct_option")
		}
	} else {
		rMsg, _ = tr.GetString(strings.ToLower(m.moduleName) + "_set_bl_action_choose_correct_option")
	}
	_, err := msg.Reply(b, rMsg, formatting.Smarkdown())
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}

func (m moduleStruct) rmAllBlacklists(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	if !chat_status.RequireGroup(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_group_only_error", "", chat_status.WithReply())
		return ext.EndGroups
	}
	if !chat_status.RequireUserOwner(b, ctx, chat, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_owner_cmd_error", "chat_status_owner_button_error", chat_status.WithReply())
		return ext.EndGroups
	}

	text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_rm_all_bl_ask")
	yesText, _ := tr.GetString("button_yes")
	noText, _ := tr.GetString("button_no")
	_, err := msg.Reply(b, text,
		&gotgbot.SendMessageOpts{
			ReplyMarkup: gotgbot.InlineKeyboardMarkup{
				InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
					{
						{
							Text:         yesText,
							CallbackData: encodeCallbackData("rmAllBlacklist", map[string]string{"a": "yes"}),
						},
						{
							Text:         noText,
							CallbackData: encodeCallbackData("rmAllBlacklist", map[string]string{"a": "no"}),
						},
					},
				},
			},
		},
	)
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func (m moduleStruct) buttonHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	query, ok := callbackQueryFromContext(ctx)
	if !ok {
		return ext.EndGroups
	}
	user := query.From
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	if query.Message == nil {
		text, _ := tr.GetString("common_callback_message_unavailable")
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return ext.EndGroups
	}

	if !chat_status.RequireUserOwner(b, ctx, nil, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_owner_cmd_error", "chat_status_owner_button_error", chat_status.WithReply())
		return ext.EndGroups
	}

	creatorAction := ""
	if decoded, ok := decodeCallbackData(query.Data, "rmAllBlacklist"); ok {
		creatorAction, _ = decoded.Field("a")
	}
	if creatorAction == "" {
		log.Warnf("[Blacklists] Invalid callback data format: %s", query.Data)
		return answerInvalidCallback(b, ctx, query)
	}
	var helpText string

	switch creatorAction {
	case "yes":
		chatID := query.Message.GetChat().Id
		if err := blacklists.RemoveAllBlacklist(chatID); err != nil {
			log.WithFields(log.Fields{
				"chatId": chatID,
				"error":  err,
			}).Error("Failed to remove all blacklists")
			helpText, _ = tr.GetString("common_settings_save_failed")
			break
		}
		helpText, _ = tr.GetString(strings.ToLower(m.moduleName) + "_rm_all_bl_button_handler_yes")
	case "no":
		helpText, _ = tr.GetString(strings.ToLower(m.moduleName) + "_rm_all_bl_button_handler_no")
	default:
		return answerInvalidCallback(b, ctx, query)
	}

	_, _, err := query.Message.EditText(b, &gotgbot.EditMessageTextOpts{Text: helpText, ParseMode: formatting.HTML})
	if err != nil {
		log.Error(err)
		return err
	}

	_, err = query.Answer(b,
		&gotgbot.AnswerCallbackQueryOpts{
			Text: helpText,
		},
	)
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
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

	blSettings := blacklists.GetBlacklistSettings(chat.Id)
	triggers := blSettings.Triggers()
	if len(triggers) == 0 {
		return ext.ContinueGroups
	}

	if !user.IsAnonymousChannel() && user.IsUser() && user.Id() > 0 && chat_status.IsUserAdmin(b, chat.Id, user.Id()) {
		return ext.ContinueGroups
	}
	if !user.IsAnonymousChannel() && user.IsUser() && user.Id() > 0 && chat_status.IsApproved(b, chat.Id, user.Id()) {
		return ext.ContinueGroups
	}

	if !chat_status.IsBotAdmin(b, ctx, chat) {
		return ext.ContinueGroups
	}

	msg := ctx.EffectiveMessage
	matchText := buildModerationMatchText(msg)
	if matchText == "" {
		return ext.ContinueGroups
	}
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

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
	reason := matched.Reason
	if reason == "" {
		reason = "Blacklisted word: '%s'"
	}

	_ = helpers.DeleteMessageWithErrorHandling(b, chat.Id, msg.MessageId)
	var err error
	switch matched.Action {
	case "mute":
		if user.IsAnonymousChannel() {
			return ext.ContinueGroups
		}

		_, err = b.RestrictChatMember(chat.Id, user.Id(), MutedPermissions, nil)
		if err != nil {
			log.Error(err)
			return err
		}

		_, err = b.SendMessage(chat.Id,
			func() string {
				temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_bl_watcher_muted_user")
				return fmt.Sprintf(temp, formatting.MentionHtml(user.Id(), user.Name()), fmt.Sprintf(reason, i))
			}(),
			formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
	case "ban":
		if user.IsAnonymousChannel() {
			_, err = b.BanChatSenderChat(chat.Id, user.Id(), nil)
		} else {
			_, err = b.BanChatMember(chat.Id, user.Id(), nil)
		}
		if err != nil {
			log.Error(err)
			return err
		}

		_, err = b.SendMessage(chat.Id,
			func() string {
				temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_bl_watcher_banned_user")
				return fmt.Sprintf(temp, formatting.MentionHtml(user.Id(), user.Name()), fmt.Sprintf(reason, i))
			}(),
			formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
	case "kick":
		if user.IsAnonymousChannel() {
			return ext.ContinueGroups
		}

		if err = kickMember(b, chat.Id, user.Id()); err != nil {
			log.Error(err)
			return err
		}

		_, err = b.SendMessage(chat.Id,
			func() string {
				temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_bl_watcher_kicked_user")
				return fmt.Sprintf(temp, formatting.MentionHtml(user.Id(), user.Name()), fmt.Sprintf(reason, i))
			}(),
			formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
	case "warn":
		if user.IsAnonymousChannel() {
			return ext.ContinueGroups
		}

		err = warnsModule.warnThisUser(b, ctx, user.Id(), fmt.Sprintf(reason, i), "warn")
		if err != nil {
			log.Error(err)
			return err
		}
	case "none":
		return ext.ContinueGroups
	}

	return ext.ContinueGroups
}

func LoadBlacklists(dispatcher *ext.Dispatcher) {
	DefaultHelpRegistry().AbleMap[blacklistsModule.moduleName] = true

	dispatcher.AddHandler(handlers.NewCommand("blacklists", blacklistsModule.listBlacklists))
	helpers.AddCmdToDisableable("blacklists")
	dispatcher.AddHandler(handlers.NewCommand("blocklists", blacklistsModule.listBlacklists))
	helpers.AddCmdToDisableable("blocklists")
	dispatcher.AddHandler(handlers.NewCommand("addblacklist", blacklistsModule.addBlacklist))
	dispatcher.AddHandler(handlers.NewCommand("blacklist", blacklistsModule.addBlacklist))
	dispatcher.AddHandler(handlers.NewCommand("addblocklist", blacklistsModule.addBlacklist))
	dispatcher.AddHandler(handlers.NewCommand("blocklist", blacklistsModule.addBlacklist))
	dispatcher.AddHandler(handlers.NewCommand("rmblacklist", blacklistsModule.removeBlacklist))
	dispatcher.AddHandler(handlers.NewCommand("rmblocklist", blacklistsModule.removeBlacklist))
	dispatcher.AddHandler(handlers.NewCommand("unblacklist", blacklistsModule.removeBlacklist))
	dispatcher.AddHandler(handlers.NewCommand("blaction", blacklistsModule.setBlacklistAction))
	dispatcher.AddHandler(handlers.NewCommand("blacklistaction", blacklistsModule.setBlacklistAction))
	helpers.MultiCommand(dispatcher, []string{"remallbl", "rmallbl", "rmblocklistall", "rmallblocklist"}, blacklistsModule.rmAllBlacklists)
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("rmAllBlacklist"), blacklistsModule.buttonHandler))
	dispatcher.AddHandlerToGroup(handlers.NewMessage(func(msg *gotgbot.Message) bool {
		return msg.Text != "" || msg.Caption != ""
	}, blacklistsModule.blacklistWatcher), blacklistsModule.handlerGroup)
}

func init() {
	RegisterLegacyModule("Blacklists", 240, LoadBlacklists)
}
