package modules

import (
	"fmt"
	"html"
	"strconv"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	log "github.com/sirupsen/logrus"

	"github.com/divideprojects/Alita_Robot/alita/db"
	"github.com/divideprojects/Alita_Robot/alita/i18n"
	"github.com/divideprojects/Alita_Robot/alita/utils/chat_status"
	"github.com/divideprojects/Alita_Robot/alita/utils/decorators/misc"
	"github.com/divideprojects/Alita_Robot/alita/utils/extraction"
	"github.com/divideprojects/Alita_Robot/alita/utils/helpers"
	"github.com/divideprojects/Alita_Robot/alita/utils/permissions"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

var warnsModule = moduleStruct{moduleName: "Warns"}

func (m moduleStruct) setWarnMode(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	// connection status
	connectedChat := helpers.IsUserConnected(b, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat
	args := ctx.Args()[1:]
	user := ctx.EffectiveSender.User

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
			go db.SetWarnMode(chat.Id, "ban")
			replyText = "Updated warn mode to: ban"
		case "kick":
			go db.SetWarnMode(chat.Id, "kick")
			replyText = "Updated warn mode to: kick"
		case "mute":
			go db.SetWarnMode(chat.Id, "mute")
			replyText = "Updated warn mode to: mute"
		default:
			replyText = fmt.Sprintf("Unknown type '%s'. Please use one of: ban/kick/mute", args[0])
		}
	} else {
		replyText = "You need to specify an action to take upon too many warns. Current modes are: ban/kick/mute"
	}

	_, err := msg.Reply(b, replyText, helpers.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}

func (m moduleStruct) warnThisUser(b *gotgbot.Bot, ctx *ext.Context, userId int64, reason, warnType string) (err error) {
	var (
		reply    string
		keyboard gotgbot.InlineKeyboardMarkup
	)

	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	tr := i18n.I18n{LangCode: db.GetLanguage(ctx)}

	// permissions check
	if chat_status.IsUserAdmin(b, chat.Id, userId) {
		_, err = msg.Reply(b, tr.GetString("strings."+m.moduleName+".errors.warn_admin"), nil)
		return err
	}

	switch warnType {
	case "dwarn":
		_, err := msg.ReplyToMessage.Delete(b, nil)
		if err != nil {
			log.Error(err)
			return err
		}
	case "swarn":
		_, err := msg.Delete(b, nil)
		if err != nil {
			log.Error(err)
			return err
		}
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
			reply = fmt.Sprintf(tr.GetString("strings."+m.moduleName+".warn.limit_kick"), numWarns, warnrc.WarnLimit, helpers.MentionHtml(u.Id, u.FirstName))
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
			reply = fmt.Sprintf(tr.GetString("strings."+m.moduleName+".warn.limit_mute"), numWarns, warnrc.WarnLimit, helpers.MentionHtml(u.Id, u.FirstName))
			if err != nil {
				log.Errorf("[warn] warnlimit: mute (%d) - %s", userId, err)
				return err
			}
		case "ban":
			_, err = chat.BanMember(b, userId, nil)
			reply = fmt.Sprintf(tr.GetString("strings."+m.moduleName+".warn.limit_ban"), numWarns, warnrc.WarnLimit, helpers.MentionHtml(u.Id, u.FirstName))
			if err != nil {
				log.Errorf("[warn] warnlimit: ban (%d) - %s", userId, err)
				return err
			}
		}
		for _, warnReason := range reasons {
			reply += fmt.Sprintf("\n - %s", html.EscapeString(warnReason))
		}
	} else {
		rules := db.GetChatRulesInfo(chat.Id)
		if len(rules.Rules) >= 1 {
			keyboard = gotgbot.InlineKeyboardMarkup{
				InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
					{
						{
							Text:         tr.GetString("strings." + m.moduleName + ".warn.remove_warn_button"),
							CallbackData: fmt.Sprintf("rmWarn.%d", u.Id),
						},
						{
							Text: tr.GetString("strings.CommonStrings.buttons.rules_button"),
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
							Text:         tr.GetString("strings." + m.moduleName + ".warn.remove_warn_button"),
							CallbackData: fmt.Sprintf("rmWarn.%d", u.Id),
						},
					},
				},
			}
		}

		reply = fmt.Sprintf(tr.GetString("strings."+m.moduleName+".warn.success"), helpers.MentionHtml(u.Id, u.FirstName), numWarns, warnrc.WarnLimit)

		if reason != "" {
			reply += fmt.Sprintf(tr.GetString("strings."+m.moduleName+".warn.reason"), html.EscapeString(reason))
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

func (m moduleStruct) warnUser(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Use helper for permission checks, user extraction, and protection validation
	userId, reason, ok := permissions.PerformCommonRestrictionChecks(b, ctx, permissions.CommonRestrictionPerms, true)
	if !ok {
		return ext.EndGroups
	}

	var warnusr int64
	if msg.ReplyToMessage != nil {
		warnusr = msg.ReplyToMessage.From.Id
	} else {
		warnusr = userId
	}

	return m.warnThisUser(b, ctx, warnusr, reason, "warn")
}

func (m moduleStruct) sWarnUser(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Use helper for permission checks, user extraction, and protection validation
	userId, reason, ok := permissions.PerformCommonRestrictionChecks(b, ctx, permissions.CommonRestrictionPerms, true)
	if !ok {
		return ext.EndGroups
	}

	var warnusr int64
	if msg.ReplyToMessage != nil && msg.ReplyToMessage.From.Id == userId {
		warnusr = msg.ReplyToMessage.From.Id
	} else {
		warnusr = userId
	}

	return m.warnThisUser(b, ctx, warnusr, reason, "swarn")
}

func (m moduleStruct) dWarnUser(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Use helper for permission checks, user extraction, and protection validation
	userId, reason, ok := permissions.PerformCommonRestrictionChecks(b, ctx, permissions.CommonRestrictionPerms, true)
	if !ok {
		return ext.EndGroups
	}

	var warnusr int64
	if msg.ReplyToMessage != nil && msg.ReplyToMessage.From.Id == userId {
		warnusr = msg.ReplyToMessage.From.Id
	} else {
		warnusr = userId
	}

	return m.warnThisUser(b, ctx, warnusr, reason, "dwarn")
}

func (m moduleStruct) warnings(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	user := ctx.EffectiveSender.User

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
	text := fmt.Sprintf(fmt.Sprint(
		"The group has the following settings:\n",
		"<b>Warn Limit:</b> <code>%d</code>\n",
		"<b>Warn Mode:</b> <code>%s</code>"),
		warnrc.WarnLimit,
		warnrc.WarnMode)
	_, err := msg.Reply(b, text, helpers.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}

func (m moduleStruct) warns(b *gotgbot.Bot, ctx *ext.Context) error {
	tr := i18n.I18n{LangCode: db.GetLanguage(ctx)}
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage

	// if command is disabled, return
	if chat_status.CheckDisabledCmd(b, msg, "warns") {
		return ext.EndGroups
	}

	userId := extraction.ExtractUser(b, ctx)
	if userId == -1 {
		userId = ctx.EffectiveUser.Id
	} else if strings.HasPrefix(fmt.Sprint(userId), "-100") {
		_, err := msg.Reply(b, tr.GetString("Warns.errors.anon_user"), nil)
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	} else if userId == 0 {
		_, err := msg.Reply(b, tr.GetString("strings.CommonStrings.errors.no_user_specified"),
			helpers.Shtml())
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
			text = fmt.Sprintf("This user has %d/%d warnings, for the following reasons:", numWarns, warnrc.WarnLimit)
			for _, reason := range reasons {
				text += fmt.Sprintf("\n - %s", reason)
			}
			msgs := helpers.SplitMessage(text)
			for _, msgText := range msgs {
				_, err := msg.Reply(b, msgText, nil)
				if err != nil {
					log.Error(err)
					return err
				}
			}
		} else {
			_, err := msg.Reply(b, fmt.Sprintf(tr.GetString("Warns.warns.list_no_reasons"),
				numWarns, warnrc.WarnLimit), nil)
			if err != nil {
				log.Error(err)
				return err
			}
		}
	} else {
		_, err := msg.Reply(b, tr.GetString("Warns.warns.no_warns"), nil)
		if err != nil {
			log.Error(err)
			return err
		}
	}
	return ext.EndGroups
}

func (m moduleStruct) rmWarnButton(b *gotgbot.Bot, ctx *ext.Context) error {
	query := ctx.Update.CallbackQuery
	user := ctx.EffectiveSender.User
	chat := ctx.EffectiveChat

	// Check permissions
	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireBotAdmin(b, ctx, nil, false) {
		return ext.EndGroups
	}

	args := strings.Split(query.Data, ".")
	userMatch := args[1]
	userId, _ := strconv.Atoi(userMatch)
	var replyText string

	res := db.RemoveWarn(int64(userId), chat.Id)
	if res {
		replyText = fmt.Sprintf("Warn removed by %s.", helpers.MentionHtml(user.Id, user.FirstName))
	} else {
		replyText = "User already has no warns!"
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

func (m moduleStruct) setWarnLimit(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	// connection status
	connectedChat := helpers.IsUserConnected(b, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat
	user := ctx.EffectiveSender.User
	args := ctx.Args()[1:]

	// Check permissions
	if !chat_status.RequireBotAdmin(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}

	var replyText string

	if len(args) == 0 {
		replyText = "Please specify how many warns a user should be allowed to receive before being acted upon. Eg. `/setwarnlimit 5`"
	} else {
		num, err := strconv.Atoi(args[0])
		if err != nil {
			replyText = fmt.Sprintf("%s is not a valid integer.", args[0])
		} else {
			if num < 1 || num > 100 {
				replyText = "The warn limit has to be set between 1 and 100."
			} else {
				go db.SetWarnLimit(chat.Id, num)
				replyText = fmt.Sprintf("Warn limit settings for this chat have been updated to %d.", num)
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

func (m moduleStruct) resetWarns(b *gotgbot.Bot, ctx *ext.Context) error {
	tr := i18n.I18n{LangCode: db.GetLanguage(ctx)}
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	user := ctx.EffectiveSender.User

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
	} else if strings.HasPrefix(fmt.Sprint(userId), "-100") {
		_, err := msg.Reply(b, tr.GetString("Warns.errors.anon_user"), nil)
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	} else if userId == 0 {
		_, err := msg.Reply(b, tr.GetString("strings.CommonStrings.errors.no_user_specified"),
			helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	db.ResetUserWarns(userId, chat.Id)
	_, err := msg.Reply(b, tr.GetString("Warns.reset.success"), helpers.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func (m moduleStruct) resetAllWarns(b *gotgbot.Bot, ctx *ext.Context) error {
	tr := i18n.I18n{LangCode: db.GetLanguage(ctx)}
	user := ctx.EffectiveSender.User
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat

	// Check if group or not
	if !chat_status.RequireGroup(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireUserOwner(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}

	warnrc := db.GetAllChatWarns(chat.Id)
	if warnrc == 0 {
		_, err := msg.Reply(b, tr.GetString("Warns.reset_all.no_warns"), helpers.Shtml())
		return err
	}

	if chat_status.RequireUserOwner(b, ctx, chat, user.Id, false) {
		_, err := msg.Reply(b, tr.GetString("Warns.reset_all.confirm"),
			&gotgbot.SendMessageOpts{
				ReplyMarkup: gotgbot.InlineKeyboardMarkup{
					InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
						{
							{Text: tr.GetString("strings.CommonStrings.buttons.yes"), CallbackData: "rmAllChatWarns.yes"},
							{Text: tr.GetString("strings.CommonStrings.buttons.no"), CallbackData: "rmAllChatWarns.no"},
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

func (m moduleStruct) warnsButtonHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	query := ctx.Update.CallbackQuery
	user := query.From

	if !chat_status.RequireUserOwner(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}

	args := strings.Split(query.Data, ".")
	response := args[1]
	var helpText string

	switch response {
	case "yes":
		go db.ResetAllChatWarns(query.Message.GetChat().Id)
		helpText = "Removed all warns of all the users in this chat !"
	case "no":
		helpText = "Cancelled the removal of all the warns of all the users in this chat !"
	}

	_, _, err := query.Message.EditText(
		b,
		"Removed all warns of all users in this chat.",
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

func LoadWarns(dispatcher *ext.Dispatcher) {
	HelpModule.AbleMap.Store(warnsModule.moduleName, true)

	dispatcher.AddHandler(handlers.NewCommand("warn", warnsModule.warnUser))
	dispatcher.AddHandler(handlers.NewCommand("swarn", warnsModule.sWarnUser))
	dispatcher.AddHandler(handlers.NewCommand("dwarn", warnsModule.dWarnUser))
	dispatcher.AddHandler(handlers.NewCommand("resetwarns", warnsModule.resetWarns))
	dispatcher.AddHandler(handlers.NewCommand("warns", warnsModule.warns))
	misc.AddCmdToDisableable("warns")
	dispatcher.AddHandler(handlers.NewCommand("setwarnlimit", warnsModule.setWarnLimit))
	dispatcher.AddHandler(handlers.NewCommand("setwarnmode", warnsModule.setWarnMode))
	dispatcher.AddHandler(handlers.NewCommand("resetallwarns", warnsModule.resetAllWarns))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("rmAllChatWarns"), warnsModule.warnsButtonHandler))
	dispatcher.AddHandler(handlers.NewCommand("warnings", warnsModule.warnings))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("rmWarn"), warnsModule.rmWarnButton))
}
