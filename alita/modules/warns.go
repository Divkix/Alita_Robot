package modules

import (
	"fmt"
	"html"
	"strconv"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/db/rules"
	"github.com/divkix/Alita_Robot/alita/db/warns"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/extraction"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

var warnsModule = moduleStruct{moduleName: "Warns"}

// setWarnMode handles the /setwarnmode command to configure the action
// taken when users reach the warning limit (ban, kick, or mute).
func (moduleStruct) setWarnMode(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	// connection status
	connectedChat := chat_status.IsUserConnected(b, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat
	args := ctx.Args()[1:]
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	// permissions check
	if !chat_status.RequireBotAdmin(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_bot_not_admin", "", chat_status.WithReply())
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
		return ext.EndGroups
	}

	var replyText string

	if len(args) > 0 {
		switch strings.ToLower(args[0]) {
		case "ban":
			if err := warns.SetWarnMode(chat.Id, "ban"); err != nil {
				log.Errorf("[Warns] SetWarnMode failed for chat %d: %v", chat.Id, err)
				errText, _ := tr.GetString("common_settings_save_failed")
				_, _ = msg.Reply(b, errText, formatting.Shtml())
				return ext.EndGroups
			}
			replyText, _ = tr.GetString("warns_mode_updated_ban")
		case "kick":
			if err := warns.SetWarnMode(chat.Id, "kick"); err != nil {
				log.Errorf("[Warns] SetWarnMode failed for chat %d: %v", chat.Id, err)
				errText, _ := tr.GetString("common_settings_save_failed")
				_, _ = msg.Reply(b, errText, formatting.Shtml())
				return ext.EndGroups
			}
			replyText, _ = tr.GetString("warns_mode_updated_kick")
		case "mute":
			if err := warns.SetWarnMode(chat.Id, "mute"); err != nil {
				log.Errorf("[Warns] SetWarnMode failed for chat %d: %v", chat.Id, err)
				errText, _ := tr.GetString("common_settings_save_failed")
				_, _ = msg.Reply(b, errText, formatting.Shtml())
				return ext.EndGroups
			}
			replyText, _ = tr.GetString("warns_mode_updated_mute")
		default:
			temp, _ := tr.GetString("warns_mode_unknown")
			replyText = fmt.Sprintf(temp, args[0])
		}
	} else {
		replyText, _ = tr.GetString("warns_specify_action")
	}

	_, err := msg.Reply(b, replyText, formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}

// warnThisUser is a helper function that performs the actual warning process,
// including limit checking and enforcement of warn mode actions.
func (moduleStruct) warnThisUser(b *gotgbot.Bot, ctx *ext.Context, userId int64, reason, warnType string) (err error) {
	var (
		reply    string
		keyboard gotgbot.InlineKeyboardMarkup
	)

	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	// Get translated button texts
	removeWarnText, _ := tr.GetString("warns_remove_button")
	rulesButtonText, _ := tr.GetString("common_rules_button_emoji")

	// permissions check
	if chat_status.IsUserAdmin(b, chat.Id, userId) {
		text, _ := tr.GetString("warns_admin_warning_error")
		_, err = msg.Reply(b, text, nil)
		return err
	}

	switch warnType {
	case "dwarn":
		if msg.ReplyToMessage != nil {
			_, err := msg.ReplyToMessage.Delete(b, nil)
			if err != nil {
				log.Errorf("[Warns] Failed to delete message: %v", err)
			}
		}
	case "swarn":
		_ = helpers.DeleteMessageWithErrorHandling(b, chat.Id, msg.MessageId)
	}

	chatMember, err := chat.GetMember(b, userId, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	u := chatMember.MergeChatMember().User
	warnrc := warns.GetWarnSetting(chat.Id)
	numWarns, reasons := warns.WarnUser(userId, chat.Id, reason)

	if numWarns >= warnrc.WarnLimit {
		punished := false
		switch warnrc.WarnMode {
		case "kick":
			_, err = chat.BanMember(b, userId, nil)
			temp, _ := tr.GetString("warns_limit_kicked")
			reply = fmt.Sprintf(temp, numWarns, warnrc.WarnLimit, formatting.MentionHtml(u.Id, u.FirstName))
			if err != nil {
				log.Errorf("[warn] warnlimit: kick (%d) - %s", userId, err)
				return err
			}
			punished = true
		case "mute":
			_, err = chat.RestrictMember(b, userId,
				MutedPermissions,
				nil,
			)
			temp, _ := tr.GetString("warns_limit_muted")
			reply = fmt.Sprintf(temp, numWarns, warnrc.WarnLimit, formatting.MentionHtml(u.Id, u.FirstName))
			if err != nil {
				log.Errorf("[warn] warnlimit: mute (%d) - %s", userId, err)
				return err
			}
			punished = true
		case "ban":
			_, err = chat.BanMember(b, userId, nil)
			temp, _ := tr.GetString("warns_limit_banned")
			reply = fmt.Sprintf(temp, numWarns, warnrc.WarnLimit, formatting.MentionHtml(u.Id, u.FirstName))
			if err != nil {
				log.Errorf("[warn] warnlimit: ban (%d) - %s", userId, err)
				return err
			}
			punished = true
		default:
			log.Warnf("[Warns] Unknown warn mode: %s", warnrc.WarnMode)
		}

		if punished {
			warns.ResetUserWarns(userId, chat.Id)
		}
		var sb strings.Builder
		for _, warnReason := range reasons {
			fmt.Fprintf(&sb, "\n - %s", html.EscapeString(warnReason))
		}
		reply += sb.String()
	} else {
		rules := rules.GetChatRulesInfo(chat.Id)
		if len(rules.Rules) >= 1 {
			keyboard = gotgbot.InlineKeyboardMarkup{
				InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
					{
						{
							Text:         removeWarnText,
							CallbackData: encodeCallbackData("rmWarn", map[string]string{"u": fmt.Sprint(u.Id)}),
						},
						{
							Text: rulesButtonText,
							Url:  fmt.Sprintf("t.me/%s?start=rules_%d", b.Username, chat.Id),
						},
					},
				},
			}
		} else {
			keyboard = gotgbot.InlineKeyboardMarkup{
				InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
					{
						{
							Text:         removeWarnText,
							CallbackData: encodeCallbackData("rmWarn", map[string]string{"u": fmt.Sprint(u.Id)}),
						},
					},
				},
			}
		}

		temp, _ := tr.GetString("warns_user_warning")
		reply = fmt.Sprintf(temp, formatting.MentionHtml(u.Id, u.FirstName), numWarns, warnrc.WarnLimit)

		if reason != "" {
			temp, _ := tr.GetString("warns_warning_reason")
			reply += fmt.Sprintf(temp, html.EscapeString(reason))
		}
	}
	_, err = b.SendMessage(chat.Id, reply,
		&gotgbot.SendMessageOpts{
			ParseMode: formatting.HTML,
			LinkPreviewOptions: &gotgbot.LinkPreviewOptions{
				IsDisabled: true,
			},
			ReplyParameters: &gotgbot.ReplyParameters{
				MessageId:                msg.MessageId,
				AllowSendingWithoutReply: true,
			},
			ReplyMarkup: &keyboard,
		},
	)
	if err != nil {
		log.Errorf("[warn] sendMessage (%d) - %s", userId, err)
		return err
	}

	return ext.EndGroups
}

// warnUser handles the /warn command to issue warnings to users
// with optional reasons, requiring admin permissions.
func (m moduleStruct) warnUser(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	// Check permissions
	if !chat_status.RequireGroup(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_group_only_error", "", chat_status.WithReply())
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
		return ext.EndGroups
	}
	if !chat_status.RequireBotAdmin(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_bot_not_admin", "", chat_status.WithReply())
		return ext.EndGroups
	}
	if !chat_status.CanUserRestrict(b, ctx, nil, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_restrict_cmd_error", "chat_status_restrict_button_error")
		return ext.EndGroups
	}
	if !chat_status.CanBotRestrict(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_bot_restrict_group_error", "chat_status_bot_restrict_error")
		return ext.EndGroups
	}

	userId, reason := extraction.ExtractUserAndText(b, ctx)
	if userId == -1 {
		return ext.EndGroups
	} else if chat_status.IsChannelId(userId) {
		text, _ := tr.GetString("common_anonymous_user_error")
		_, err := msg.Reply(b, text, nil)
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	} else if userId == 0 {
		text, _ := tr.GetString("common_no_user_specified")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if !chat_status.IsUserInChat(b, chat, userId) {
		return ext.EndGroups
	}
	var warnusr int64
	if msg.ReplyToMessage != nil && msg.ReplyToMessage.From != nil && msg.ReplyToMessage.From.Id == userId {
		warnusr = msg.ReplyToMessage.From.Id
	} else {
		warnusr = userId
	}

	return m.warnThisUser(b, ctx, warnusr, reason, "warn")
}

// sWarnUser handles the /swarn command to silently warn users
// by deleting the command message, requiring admin permissions.
//
//nolint:dupl // sWarnUser has similar structure to dWarnUser
func (m moduleStruct) sWarnUser(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	// Check permissions
	if !chat_status.RequireGroup(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_group_only_error", "", chat_status.WithReply())
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
		return ext.EndGroups
	}
	if !chat_status.RequireBotAdmin(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_bot_not_admin", "", chat_status.WithReply())
		return ext.EndGroups
	}
	if !chat_status.CanUserRestrict(b, ctx, nil, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_restrict_cmd_error", "chat_status_restrict_button_error")
		return ext.EndGroups
	}
	if !chat_status.CanBotRestrict(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_bot_restrict_group_error", "chat_status_bot_restrict_error")
		return ext.EndGroups
	}

	userId, reason := extraction.ExtractUserAndText(b, ctx)
	if userId == -1 {
		return ext.EndGroups
	} else if chat_status.IsChannelId(userId) {
		text, _ := tr.GetString("common_anonymous_user_error")
		_, err := msg.Reply(b, text, nil)
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	} else if userId == 0 {
		text, _ := tr.GetString("common_no_user_specified")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if !chat_status.IsUserInChat(b, chat, userId) {
		return ext.EndGroups
	}
	var warnusr int64
	if msg.ReplyToMessage != nil && msg.ReplyToMessage.From != nil && msg.ReplyToMessage.From.Id == userId {
		warnusr = msg.ReplyToMessage.From.Id
	} else {
		warnusr = userId
	}

	return m.warnThisUser(b, ctx, warnusr, reason, "swarn")
}

// dWarnUser handles the /dwarn command to warn users and delete
// the message they replied to, requiring admin permissions.
//
//nolint:dupl // dWarnUser has similar structure to sWarnUser
func (m moduleStruct) dWarnUser(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	// Check permissions
	if !chat_status.RequireGroup(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_group_only_error", "", chat_status.WithReply())
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
		return ext.EndGroups
	}
	if !chat_status.RequireBotAdmin(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_bot_not_admin", "", chat_status.WithReply())
		return ext.EndGroups
	}
	if !chat_status.CanUserRestrict(b, ctx, nil, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_restrict_cmd_error", "chat_status_restrict_button_error")
		return ext.EndGroups
	}
	if !chat_status.CanBotRestrict(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_bot_restrict_group_error", "chat_status_bot_restrict_error")
		return ext.EndGroups
	}

	userId, reason := extraction.ExtractUserAndText(b, ctx)
	if userId == -1 {
		return ext.EndGroups
	} else if chat_status.IsChannelId(userId) {
		text, _ := tr.GetString("common_anonymous_user_error")
		_, err := msg.Reply(b, text, nil)
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	} else if userId == 0 {
		text, _ := tr.GetString("common_no_user_specified")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if !chat_status.IsUserInChat(b, chat, userId) {
		return ext.EndGroups
	}
	var warnusr int64
	if msg.ReplyToMessage != nil && msg.ReplyToMessage.From != nil && msg.ReplyToMessage.From.Id == userId {
		warnusr = msg.ReplyToMessage.From.Id
	} else {
		warnusr = userId
	}

	return m.warnThisUser(b, ctx, warnusr, reason, "dwarn")
}

// warnings handles the /warnings command to display current
// warning settings including limit and enforcement mode.
func (moduleStruct) warnings(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	// Check permissions
	if !chat_status.RequireGroup(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_group_only_error", "", chat_status.WithReply())
		return ext.EndGroups
	}
	if !chat_status.RequireBotAdmin(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_bot_not_admin", "", chat_status.WithReply())
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
		return ext.EndGroups
	}

	warnrc := warns.GetWarnSetting(chat.Id)
	temp, _ := tr.GetString("warns_settings_display")
	text := fmt.Sprintf(temp, warnrc.WarnLimit, warnrc.WarnMode)
	_, err := msg.Reply(b, text, formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}

// warns handles the /warns command to check the warning count
// and reasons for a specific user or the command sender.
func (moduleStruct) warns(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	// if command is disabled, return
	if chat_status.CheckDisabledCmd(b, msg, "warns") {
		return ext.EndGroups
	}

	userId := extraction.ExtractUser(b, ctx)
	if userId == -1 {
		if ctx.EffectiveUser == nil {
			text, _ := tr.GetString("common_anonymous_user_error")
			_, err := msg.Reply(b, text, nil)
			if err != nil {
				log.Error(err)
				return err
			}
			return ext.EndGroups
		}
		userId = ctx.EffectiveUser.Id
	} else if chat_status.IsChannelId(userId) {
		text, _ := tr.GetString("common_anonymous_user_error")
		_, err := msg.Reply(b, text, nil)
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	} else if userId == 0 {
		text, _ := tr.GetString("common_no_user_specified")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	numWarns, reasons := warns.GetWarns(userId, chat.Id)
	text := ""

	if numWarns != 0 {
		warnrc := warns.GetWarnSetting(chat.Id)
		if len(reasons) > 0 {
			temp, _ := tr.GetString("warns_user_warnings_list")
			text = fmt.Sprintf(temp, numWarns, warnrc.WarnLimit)
			var sb strings.Builder
			for _, reason := range reasons {
				fmt.Fprintf(&sb, "\n - %s", reason)
			}
			text += sb.String()
			msgs := formatting.SplitMessage(text)
			for _, msgText := range msgs {
				_, err := msg.Reply(b, msgText, nil)
				if err != nil {
					log.Error(err)
					return err
				}
			}
		} else {
			temp, _ := tr.GetString("warns_user_warnings_no_reasons")
			_, err := msg.Reply(b, fmt.Sprintf(temp, numWarns, warnrc.WarnLimit), nil)
			if err != nil {
				log.Error(err)
				return err
			}
		}
	} else {
		text, _ := tr.GetString("warns_user_no_warnings")
		_, err := msg.Reply(b, text, nil)
		if err != nil {
			log.Error(err)
			return err
		}
	}
	return ext.EndGroups
}

// rmWarnButton processes callback queries from remove warning buttons
// to remove the latest warning from a user, requiring admin permissions.
func (moduleStruct) rmWarnButton(b *gotgbot.Bot, ctx *ext.Context) error {
	query, ok := callbackQueryFromContext(ctx)
	if !ok {
		return ext.EndGroups
	}
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	chat := ctx.EffectiveChat
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	// Check permissions
	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
		return ext.EndGroups
	}
	if !chat_status.RequireBotAdmin(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_bot_not_admin", "", chat_status.WithReply())
		return ext.EndGroups
	}

	userMatch := ""
	if decoded, ok := decodeCallbackData(query.Data, "rmWarn"); ok {
		userMatch, _ = decoded.Field("u")
	}
	if userMatch == "" {
		log.Warnf("[Warns] Invalid callback data format: %s", query.Data)
		text, _ := tr.GetString("common_callback_invalid_request")
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return ext.EndGroups
	}
	userId, parseErr := strconv.Atoi(userMatch)
	if parseErr != nil {
		log.Errorf("[Warns] Failed to parse user ID from callback: %v", parseErr)
		text, _ := tr.GetString("common_callback_invalid_request")
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return ext.EndGroups
	}
	var replyText string

	res := warns.RemoveWarn(int64(userId), chat.Id)
	if res {
		temp, _ := tr.GetString("warns_removed_by")
		replyText = fmt.Sprintf(temp, formatting.MentionHtml(user.Id, user.FirstName))
	} else {
		replyText, _ = tr.GetString("warns_no_warns_to_remove")
	}

	if query.Message == nil {
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: replyText})
		return ext.EndGroups
	}

	_, _, err := query.Message.EditText(
		b,
		replyText,
		&gotgbot.EditMessageTextOpts{
			ParseMode: formatting.HTML,
		},
	)
	if err != nil {
		log.Error(err)
		return err
	}

	_, err = query.Answer(b, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// setWarnLimit handles the /setwarnlimit command to configure
// the maximum number of warnings before enforcement action.
func (moduleStruct) setWarnLimit(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	// connection status
	connectedChat := chat_status.IsUserConnected(b, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	args := ctx.Args()[1:]
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	// Check permissions
	if !chat_status.RequireBotAdmin(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_bot_not_admin", "", chat_status.WithReply())
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
		return ext.EndGroups
	}

	var replyText string

	if len(args) == 0 {
		replyText, _ = tr.GetString("warns_limit_set_help")
	} else {
		num, err := strconv.Atoi(args[0])
		if err != nil {
			temp, _ := tr.GetString("warns_invalid_number")
			replyText = fmt.Sprintf(temp, args[0])
		} else {
			if num < 1 || num > 100 {
				replyText, _ = tr.GetString("warns_limit_range_error")
			} else {
				if err := warns.SetWarnLimit(chat.Id, num); err != nil {
					log.Errorf("[Warns] SetWarnLimit failed for chat %d: %v", chat.Id, err)
					errText, _ := tr.GetString("common_settings_save_failed")
					_, _ = msg.Reply(b, errText, formatting.Smarkdown())
					return ext.EndGroups
				}
				temp, _ := tr.GetString("warns_limit_updated")
				replyText = fmt.Sprintf(temp, num)
			}
		}
	}

	_, err := msg.Reply(b, replyText, formatting.Smarkdown())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// resetWarns handles the /resetwarns command to clear all warnings
// for a specific user, requiring admin permissions.
func (moduleStruct) resetWarns(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	// Check permissions
	if !chat_status.RequireGroup(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_group_only_error", "", chat_status.WithReply())
		return ext.EndGroups
	}
	if !chat_status.RequireBotAdmin(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_bot_not_admin", "", chat_status.WithReply())
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
		return ext.EndGroups
	}

	userId := extraction.ExtractUser(b, ctx)
	if userId == -1 {
		return ext.EndGroups
	} else if chat_status.IsChannelId(userId) {
		text, _ := tr.GetString("common_anonymous_user_error")
		_, err := msg.Reply(b, text, nil)
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	} else if userId == 0 {
		text, _ := tr.GetString("common_no_user_specified")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	warns.ResetUserWarns(userId, chat.Id)
	text, _ := tr.GetString("warns_reset_success")
	_, err := msg.Reply(b, text, formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// resetAllWarns handles the /resetallwarns command to clear all warnings
// for all users in the chat with confirmation, restricted to owners.
func (moduleStruct) resetAllWarns(b *gotgbot.Bot, ctx *ext.Context) error {
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	// Get translated button texts
	yesText, _ := tr.GetString("common_yes")
	noText, _ := tr.GetString("common_no")

	// Check if group or not
	if !chat_status.RequireGroup(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_group_only_error", "", chat_status.WithReply())
		return ext.EndGroups
	}
	if !chat_status.RequireUserOwner(b, ctx, nil, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_owner_cmd_error", "chat_status_owner_button_error", chat_status.WithReply())
		return ext.EndGroups
	}

	warnrc := warns.GetAllChatWarns(chat.Id)
	if warnrc == 0 {
		text, _ := tr.GetString("warns_no_users_warned")
		_, err := msg.Reply(b, text, formatting.Shtml())
		return err
	}

	if chat_status.RequireUserOwner(b, ctx, chat, user.Id) {
		text, _ := tr.GetString("warns_reset_all_confirm")
		_, err := msg.Reply(b, text,
			&gotgbot.SendMessageOpts{
				ReplyMarkup: gotgbot.InlineKeyboardMarkup{
					InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
						{
							{
								Text:         yesText,
								CallbackData: encodeCallbackData("rmAllChatWarns", map[string]string{"a": "yes"}),
							},
							{
								Text:         noText,
								CallbackData: encodeCallbackData("rmAllChatWarns", map[string]string{"a": "no"}),
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
	}

	return ext.EndGroups
}

// warnsButtonHandler processes callback queries for the reset all warnings
// confirmation dialog, restricted to chat owners.
func (moduleStruct) warnsButtonHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	query, ok := callbackQueryFromContext(ctx)
	if !ok {
		return ext.EndGroups
	}
	user := query.From
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	if !chat_status.RequireUserOwner(b, ctx, nil, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_owner_cmd_error", "chat_status_owner_button_error", chat_status.WithReply())
		return ext.EndGroups
	}

	response := ""
	if decoded, ok := decodeCallbackData(query.Data, "rmAllChatWarns"); ok {
		response, _ = decoded.Field("a")
	}
	if response == "" {
		log.Warnf("[Warns] Invalid callback data format: %s", query.Data)
		text, _ := tr.GetString("common_callback_invalid_request")
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return ext.EndGroups
	}
	var helpText string

	var replyText string

	chat := ctx.EffectiveChat
	switch response {
	case "yes":
		if chat == nil {
			helpText, _ = tr.GetString("error_generic")
			replyText = helpText
			break
		}
		warns.ResetAllChatWarns(chat.Id)
		helpText, _ = tr.GetString("warns_reset_all_success")
		replyText, _ = tr.GetString("warns_reset_all_final")
	case "no":
		helpText, _ = tr.GetString("warns_reset_all_cancelled")
		replyText = helpText
	}

	if query.Message == nil {
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: helpText})
		return ext.EndGroups
	}

	_, _, err := query.Message.EditText(
		b,
		replyText,
		nil,
	)
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

// removeWarn handles /rmwarn and /unwarn commands to remove the latest warning
// from a specific user. Requires bot and user admin permissions.
func (moduleStruct) removeWarn(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	// Check permissions
	if !chat_status.RequireGroup(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_group_only_error", "", chat_status.WithReply())
		return ext.EndGroups
	}
	if !chat_status.RequireBotAdmin(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_bot_not_admin", "", chat_status.WithReply())
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
		return ext.EndGroups
	}

	userId := extraction.ExtractUser(b, ctx)
	if userId == -1 {
		return ext.EndGroups
	} else if chat_status.IsChannelId(userId) {
		text, _ := tr.GetString("common_anonymous_user_error")
		_, err := msg.Reply(b, text, nil)
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	} else if userId == 0 {
		text, _ := tr.GetString("common_no_user_specified")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	var replyText string
	if warns.RemoveWarn(userId, chat.Id) {
		temp, _ := tr.GetString("warns_removed_by")
		replyText = fmt.Sprintf(temp, formatting.MentionHtml(user.Id, user.FirstName))
	} else {
		replyText, _ = tr.GetString("warns_no_warns_to_remove")
	}

	_, err := msg.Reply(b, replyText, formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}

// LoadWarns registers all warns module handlers with the dispatcher,
// including warning commands and callback handlers.
func LoadWarns(dispatcher *ext.Dispatcher) {
	DefaultHelpRegistry().AbleMap.Store(warnsModule.moduleName, true)

	dispatcher.AddHandler(handlers.NewCommand("warn", warnsModule.warnUser))
	dispatcher.AddHandler(handlers.NewCommand("swarn", warnsModule.sWarnUser))
	dispatcher.AddHandler(handlers.NewCommand("dwarn", warnsModule.dWarnUser))
	// Aliases for reset warnings (docs mention /resetwarn as well)
	dispatcher.AddHandler(handlers.NewCommand("resetwarns", warnsModule.resetWarns))
	dispatcher.AddHandler(handlers.NewCommand("resetwarn", warnsModule.resetWarns))
	// Add commands to remove latest warn for a user
	dispatcher.AddHandler(handlers.NewCommand("rmwarn", warnsModule.removeWarn))
	dispatcher.AddHandler(handlers.NewCommand("unwarn", warnsModule.removeWarn))
	dispatcher.AddHandler(handlers.NewCommand("warns", warnsModule.warns))
	helpers.AddCmdToDisableable("warns")
	dispatcher.AddHandler(handlers.NewCommand("setwarnlimit", warnsModule.setWarnLimit))
	dispatcher.AddHandler(handlers.NewCommand("setwarnmode", warnsModule.setWarnMode))
	dispatcher.AddHandler(handlers.NewCommand("resetallwarns", warnsModule.resetAllWarns))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("rmAllChatWarns"), warnsModule.warnsButtonHandler))
	dispatcher.AddHandler(handlers.NewCommand("warnings", warnsModule.warnings))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("rmWarn"), warnsModule.rmWarnButton))
}

func init() {
	RegisterLegacyModule("Warns", 200, LoadWarns)
	RegisterAnonymousAdminHandler("warn", warnsModule.warnUser)
	RegisterAnonymousAdminHandler("swarn", warnsModule.sWarnUser)
	RegisterAnonymousAdminHandler("dwarn", warnsModule.dWarnUser)
}
