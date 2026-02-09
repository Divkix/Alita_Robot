package modules

import (
	"fmt"
	"html"
	"strconv"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/decorators/misc"
	"github.com/divkix/Alita_Robot/alita/utils/extraction"
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
	connectedChat := helpers.IsUserConnected(b, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat
	args := ctx.Args()[1:]
	user := chat_status.RequireUser(b, ctx, false)
	if user == nil {
		return ext.EndGroups
	}
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	// permissions check
	if !chat_status.RequireBotAdmin(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}

	var replyText string

	if len(args) > 0 {
		switch strings.ToLower(args[0]) {
		case "ban":
			db.SetWarnMode(chat.Id, "ban")
			replyText, _ = tr.GetString("warns_mode_updated_ban")
		case "kick":
			db.SetWarnMode(chat.Id, "kick")
			replyText, _ = tr.GetString("warns_mode_updated_kick")
		case "mute":
			db.SetWarnMode(chat.Id, "mute")
			replyText, _ = tr.GetString("warns_mode_updated_mute")
		default:
			temp, _ := tr.GetString("warns_mode_unknown")
			replyText = fmt.Sprintf(temp, args[0])
		}
	} else {
		replyText, _ = tr.GetString("warns_specify_action")
	}

	_, err := msg.Reply(b, replyText, helpers.Shtml())
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
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

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
	warnrc := db.GetWarnSetting(chat.Id)
	numWarns, reasons := db.WarnUser(userId, chat.Id, reason)

	if numWarns >= warnrc.WarnLimit {
		db.ResetUserWarns(userId, chat.Id)
		switch warnrc.WarnMode {
		case "kick":
			_, err = chat.BanMember(b, userId, nil)
			temp, _ := tr.GetString("warns_limit_kicked")
			reply = fmt.Sprintf(temp, numWarns, warnrc.WarnLimit, helpers.MentionHtml(u.Id, u.FirstName))
			if err != nil {
				log.Errorf("[warn] warnlimit: kick (%d) - %s", userId, err)
				return err
			}
		case "mute":
			_, err = chat.RestrictMember(b, userId,
				gotgbot.ChatPermissions{
					CanSendMessages:       false,
					CanSendPhotos:         false,
					CanSendVideos:         false,
					CanSendAudios:         false,
					CanSendDocuments:      false,
					CanSendVideoNotes:     false,
					CanSendVoiceNotes:     false,
					CanAddWebPagePreviews: false,
					CanChangeInfo:         false,
					CanInviteUsers:        false,
					CanPinMessages:        false,
					CanManageTopics:       false,
					CanSendPolls:          false,
					CanSendOtherMessages:  false,
				},
				nil,
			)
			temp, _ := tr.GetString("warns_limit_muted")
			reply = fmt.Sprintf(temp, numWarns, warnrc.WarnLimit, helpers.MentionHtml(u.Id, u.FirstName))
			if err != nil {
				log.Errorf("[warn] warnlimit: mute (%d) - %s", userId, err)
				return err
			}
		case "ban":
			_, err = chat.BanMember(b, userId, nil)
			temp, _ := tr.GetString("warns_limit_banned")
			reply = fmt.Sprintf(temp, numWarns, warnrc.WarnLimit, helpers.MentionHtml(u.Id, u.FirstName))
			if err != nil {
				log.Errorf("[warn] warnlimit: ban (%d) - %s", userId, err)
				return err
			}
		}
		var sb strings.Builder
		for _, warnReason := range reasons {
			sb.WriteString(fmt.Sprintf("\n - %s", html.EscapeString(warnReason)))
		}
		reply += sb.String()
	} else {
		rules := db.GetChatRulesInfo(chat.Id)
		if len(rules.Rules) >= 1 {
			keyboard = gotgbot.InlineKeyboardMarkup{
				InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
					{
						{
							Text:         removeWarnText,
							CallbackData: fmt.Sprintf("rmWarn.%d", u.Id),
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
							CallbackData: fmt.Sprintf("rmWarn.%d", u.Id),
						},
					},
				},
			}
		}

		temp, _ := tr.GetString("warns_user_warning")
		reply = fmt.Sprintf(temp, helpers.MentionHtml(u.Id, u.FirstName), numWarns, warnrc.WarnLimit)

		if reason != "" {
			temp, _ := tr.GetString("warns_warning_reason")
			reply += fmt.Sprintf(temp, html.EscapeString(reason))
		}
	}
	_, err = b.SendMessage(chat.Id, reply,
		&gotgbot.SendMessageOpts{
			ParseMode: helpers.HTML,
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
	user := chat_status.RequireUser(b, ctx, false)
	if user == nil {
		return ext.EndGroups
	}
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	// Check permissions
	if !chat_status.RequireGroup(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireBotAdmin(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.CanUserRestrict(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}
	if !chat_status.CanBotRestrict(b, ctx, nil, false) {
		return ext.EndGroups
	}

	userId, reason := extraction.ExtractUserAndText(b, ctx)
	if userId == -1 {
		return ext.EndGroups
	} else if helpers.IsChannelID(userId) {
		text, _ := tr.GetString("common_anonymous_user_error")
		_, err := msg.Reply(b, text, nil)
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	} else if userId == 0 {
		text, _ := tr.GetString("common_no_user_specified")
		_, err := msg.Reply(b, text, helpers.Shtml())
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
	if msg.ReplyToMessage != nil && msg.ReplyToMessage.From != nil {
		warnusr = msg.ReplyToMessage.From.Id
	} else {
		warnusr = userId
	}

	return m.warnThisUser(b, ctx, warnusr, reason, "warn")
}

// sWarnUser handles the /swarn command to silently warn users
// by deleting the command message, requiring admin permissions.
func (m moduleStruct) sWarnUser(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	user := chat_status.RequireUser(b, ctx, false)
	if user == nil {
		return ext.EndGroups
	}
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	// Check permissions
	if !chat_status.RequireGroup(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireBotAdmin(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.CanUserRestrict(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}
	if !chat_status.CanBotRestrict(b, ctx, nil, false) {
		return ext.EndGroups
	}

	userId, reason := extraction.ExtractUserAndText(b, ctx)
	if userId == -1 {
		return ext.EndGroups
	} else if helpers.IsChannelID(userId) {
		text, _ := tr.GetString("common_anonymous_user_error")
		_, err := msg.Reply(b, text, nil)
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	} else if userId == 0 {
		text, _ := tr.GetString("common_no_user_specified")
		_, err := msg.Reply(b, text, helpers.Shtml())
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
func (m moduleStruct) dWarnUser(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	user := chat_status.RequireUser(b, ctx, false)
	if user == nil {
		return ext.EndGroups
	}
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	// Check permissions
	if !chat_status.RequireGroup(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireBotAdmin(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.CanUserRestrict(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}
	if !chat_status.CanBotRestrict(b, ctx, nil, false) {
		return ext.EndGroups
	}

	userId, reason := extraction.ExtractUserAndText(b, ctx)
	if userId == -1 {
		return ext.EndGroups
	} else if helpers.IsChannelID(userId) {
		text, _ := tr.GetString("common_anonymous_user_error")
		_, err := msg.Reply(b, text, nil)
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	} else if userId == 0 {
		text, _ := tr.GetString("common_no_user_specified")
		_, err := msg.Reply(b, text, helpers.Shtml())
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
	user := chat_status.RequireUser(b, ctx, false)
	if user == nil {
		return ext.EndGroups
	}
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	// Check permissions
	if !chat_status.RequireGroup(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireBotAdmin(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}

	warnrc := db.GetWarnSetting(chat.Id)
	temp, _ := tr.GetString("warns_settings_display")
	text := fmt.Sprintf(temp, warnrc.WarnLimit, warnrc.WarnMode)
	_, err := msg.Reply(b, text, helpers.Shtml())
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
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	// if command is disabled, return
	if chat_status.CheckDisabledCmd(b, msg, "warns") {
		return ext.EndGroups
	}

	userId := extraction.ExtractUser(b, ctx)
	if userId == -1 {
		userId = ctx.EffectiveUser.Id
	} else if helpers.IsChannelID(userId) {
		text, _ := tr.GetString("common_anonymous_user_error")
		_, err := msg.Reply(b, text, nil)
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	} else if userId == 0 {
		text, _ := tr.GetString("common_no_user_specified")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	numWarns, reasons := db.GetWarns(userId, chat.Id)
	text := ""

	if numWarns != 0 {
		warnrc := db.GetWarnSetting(chat.Id)
		if len(reasons) > 0 {
			temp, _ := tr.GetString("warns_user_warnings_list")
			text = fmt.Sprintf(temp, numWarns, warnrc.WarnLimit)
			var sb strings.Builder
			for _, reason := range reasons {
				sb.WriteString(fmt.Sprintf("\n - %s", reason))
			}
			text += sb.String()
			msgs := helpers.SplitMessage(text)
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
	query := ctx.CallbackQuery
	user := chat_status.RequireUser(b, ctx, false)
	if user == nil {
		return ext.EndGroups
	}
	chat := ctx.EffectiveChat
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	// Check permissions
	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireBotAdmin(b, ctx, nil, false) {
		return ext.EndGroups
	}

	args := strings.Split(query.Data, ".")
	if len(args) < 2 {
		log.Warnf("[Warns] Invalid callback data format: %s", query.Data)
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "Invalid request."})
		return ext.EndGroups
	}
	userMatch := args[1]
	userId, _ := strconv.Atoi(userMatch)
	var replyText string

	res := db.RemoveWarn(int64(userId), chat.Id)
	if res {
		temp, _ := tr.GetString("warns_removed_by")
		replyText = fmt.Sprintf(temp, helpers.MentionHtml(user.Id, user.FirstName))
	} else {
		replyText, _ = tr.GetString("warns_no_warns_to_remove")
	}

	_, _, err := query.Message.EditText(
		b,
		replyText,
		&gotgbot.EditMessageTextOpts{
			ParseMode: helpers.HTML,
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
	connectedChat := helpers.IsUserConnected(b, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx, false)
	if user == nil {
		return ext.EndGroups
	}
	args := ctx.Args()[1:]
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	// Check permissions
	if !chat_status.RequireBotAdmin(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id, false) {
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
				db.SetWarnLimit(chat.Id, num)
				temp, _ := tr.GetString("warns_limit_updated")
				replyText = fmt.Sprintf(temp, num)
			}
		}
	}

	_, err := msg.Reply(b, replyText, helpers.Smarkdown())
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
	user := chat_status.RequireUser(b, ctx, false)
	if user == nil {
		return ext.EndGroups
	}
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	// Check permissions
	if !chat_status.RequireGroup(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireBotAdmin(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}

	userId := extraction.ExtractUser(b, ctx)
	if userId == -1 {
		return ext.EndGroups
	} else if helpers.IsChannelID(userId) {
		text, _ := tr.GetString("common_anonymous_user_error")
		_, err := msg.Reply(b, text, nil)
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	} else if userId == 0 {
		text, _ := tr.GetString("common_no_user_specified")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	db.ResetUserWarns(userId, chat.Id)
	text, _ := tr.GetString("warns_reset_success")
	_, err := msg.Reply(b, text, helpers.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// resetAllWarns handles the /resetallwarns command to clear all warnings
// for all users in the chat with confirmation, restricted to owners.
func (moduleStruct) resetAllWarns(b *gotgbot.Bot, ctx *ext.Context) error {
	user := chat_status.RequireUser(b, ctx, false)
	if user == nil {
		return ext.EndGroups
	}
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	// Get translated button texts
	yesText, _ := tr.GetString("common_yes")
	noText, _ := tr.GetString("common_no")

	// Check if group or not
	if !chat_status.RequireGroup(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireUserOwner(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}

	warnrc := db.GetAllChatWarns(chat.Id)
	if warnrc == 0 {
		text, _ := tr.GetString("warns_no_users_warned")
		_, err := msg.Reply(b, text, helpers.Shtml())
		return err
	}

	if chat_status.RequireUserOwner(b, ctx, chat, user.Id, false) {
		text, _ := tr.GetString("warns_reset_all_confirm")
		_, err := msg.Reply(b, text,
			&gotgbot.SendMessageOpts{
				ReplyMarkup: gotgbot.InlineKeyboardMarkup{
					InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
						{
							{Text: yesText, CallbackData: "rmAllChatWarns.yes"},
							{Text: noText, CallbackData: "rmAllChatWarns.no"},
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
	query := ctx.CallbackQuery
	user := query.From
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	if !chat_status.RequireUserOwner(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}

	args := strings.Split(query.Data, ".")
	if len(args) < 2 {
		log.Warnf("[Warns] Invalid callback data format: %s", query.Data)
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "Invalid request."})
		return ext.EndGroups
	}
	response := args[1]
	var helpText string

	var replyText string

	switch response {
	case "yes":
		db.ResetAllChatWarns(query.Message.GetChat().Id)
		helpText, _ = tr.GetString("warns_reset_all_success")
		replyText, _ = tr.GetString("warns_reset_all_final")
	case "no":
		helpText, _ = tr.GetString("warns_reset_all_cancelled")
		replyText = helpText
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
	user := chat_status.RequireUser(b, ctx, false)
	if user == nil {
		return ext.EndGroups
	}
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	// Check permissions
	if !chat_status.RequireGroup(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireBotAdmin(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}

	userId := extraction.ExtractUser(b, ctx)
	if userId == -1 {
		return ext.EndGroups
	} else if helpers.IsChannelID(userId) {
		text, _ := tr.GetString("common_anonymous_user_error")
		_, err := msg.Reply(b, text, nil)
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	} else if userId == 0 {
		text, _ := tr.GetString("common_no_user_specified")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	var replyText string
	if db.RemoveWarn(userId, chat.Id) {
		temp, _ := tr.GetString("warns_removed_by")
		replyText = fmt.Sprintf(temp, helpers.MentionHtml(user.Id, user.FirstName))
	} else {
		replyText, _ = tr.GetString("warns_no_warns_to_remove")
	}

	_, err := msg.Reply(b, replyText, helpers.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}

// LoadWarns registers all warns module handlers with the dispatcher,
// including warning commands and callback handlers.
func LoadWarns(dispatcher *ext.Dispatcher) {
	HelpModule.AbleMap.Store(warnsModule.moduleName, true)

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
	misc.AddCmdToDisableable("warns")
	dispatcher.AddHandler(handlers.NewCommand("setwarnlimit", warnsModule.setWarnLimit))
	dispatcher.AddHandler(handlers.NewCommand("setwarnmode", warnsModule.setWarnMode))
	dispatcher.AddHandler(handlers.NewCommand("resetallwarns", warnsModule.resetAllWarns))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("rmAllChatWarns"), warnsModule.warnsButtonHandler))
	dispatcher.AddHandler(handlers.NewCommand("warnings", warnsModule.warnings))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("rmWarn"), warnsModule.rmWarnButton))
}
