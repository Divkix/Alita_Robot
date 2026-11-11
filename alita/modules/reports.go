package modules

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/reports"
	"github.com/divkix/Alita_Robot/alita/i18n"
	chatsettings "github.com/divkix/Alita_Robot/alita/settings"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	"github.com/divkix/Alita_Robot/alita/utils/punishment"
	"github.com/divkix/Alita_Robot/alita/utils/tracing"
)

var reportsModule = moduleStruct{
	moduleName:   "Reports",
	handlerGroup: 8,
}

var reportDesc = helpers.CommandDescriptor{
	Name:           "report",
	Disableable:    true,
	RequiredChecks: []helpers.CheckFunc{helpers.RequireIdentifiedUser()},
}

var adminMentionRegex = regexp.MustCompile(`(?i)(^|\s)@admins?(\s|$)`)

// maxReportReasonRunes caps the stored reason so one report cannot store an
// arbitrarily long message.
const maxReportReasonRunes = 500

// reportReason is the text after /report, or the whole @admin message.
func reportReason(text string) string {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "/") {
		if i := strings.IndexFunc(text, unicode.IsSpace); i >= 0 {
			text = strings.TrimSpace(text[i:])
		} else {
			text = ""
		}
	}
	if r := []rune(text); len(r) > maxReportReasonRunes {
		text = string(r[:maxReportReasonRunes])
	}
	return text
}

// reportSnippet is the start of the reported message's text or caption. The
// log channel keeps the copy; the stored row only needs to recognise it.
func reportSnippet(m *gotgbot.Message) string {
	text := m.Text
	if text == "" {
		text = m.Caption
	}
	if r := []rune(text); len(r) > models.MaxReportSnippetRunes {
		text = string(r[:models.MaxReportSnippetRunes])
	}
	return text
}

func (moduleStruct) report(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	sender := ctx.EffectiveSender
	if sender == nil || sender.User == nil {
		return ext.EndGroups
	}
	user := sender.User
	msg := ctx.EffectiveMessage

	reportprefs := chatsettings.ReadReports(chat.Id)
	if !reportprefs.Status {
		return ext.EndGroups
	}

	if msg.ReplyToMessage == nil {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("reports_reply_to_report")
		_, _ = msg.Reply(b, text, nil)
		return ext.EndGroups
	}

	if msg.ReplyToMessage.From == nil {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("reports_cannot_report_channel")
		_, _ = msg.Reply(b, text, nil)
		return ext.EndGroups
	}

	var (
		replyMsgId int64
		adminArray []int64
		err        error
	)

	if msg.ReplyToMessage.From.Id == user.Id {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("reports_cannot_report_self")
		_, _ = msg.Reply(b, text, nil)
		return ext.EndGroups
	}

	if replyMsg := msg.ReplyToMessage; replyMsg != nil {
		replyMsgId = replyMsg.MessageId
	} else {
		replyMsgId = msg.MessageId
	}

	if slices.Contains(reportprefs.BlockedList, user.Id) {
		if chat_status.CanBotDelete(b, ctx, nil) {
			_, err := msg.Delete(b, nil)
			if err != nil {
				log.Error(err)
			}

		}
		return ext.EndGroups
	}

	if chat_status.IsSystemUser(user.Id) {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("reports_expose_yourself")
		_, _ = msg.Reply(b, text, nil)
		return ext.EndGroups
	}
	if chat_status.IsSystemUser(msg.ReplyToMessage.From.Id) {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("reports_special_account")
		_, _ = msg.Reply(b, text, nil)
		return ext.EndGroups
	}

	admins, _ := loadAdminCache(b, chat.Id)
	for i := range admins.UserInfo {
		admin := &admins.UserInfo[i]
		adminArray = append(adminArray, admin.User.Id)
	}
	if slices.Contains(adminArray, user.Id) ||
		(len(adminArray) == 0 && chat_status.IsUserAdmin(b, chat.Id, user.Id)) {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("reports_admin_report")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	reportedUser := msg.ReplyToMessage.From
	reportedMsgId := msg.ReplyToMessage.MessageId

	if reportedUser.Id == b.Id {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("reports_why_report_myself")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}
	if slices.Contains(adminArray, reportedUser.Id) {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("reports_why_report_admin")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	reason := reportReason(msg.GetText())
	created, storeErr := reports.Create(&models.Report{
		ChatId:            chat.Id,
		ReportedUserId:    reportedUser.Id,
		ReportedMessageId: reportedMsgId,
		ReporterId:        user.Id,
		Reason:            reason,
		Snippet:           reportSnippet(msg.ReplyToMessage),
	})
	if storeErr != nil {
		log.Errorf("[Reports] failed to store report in chat %d: %v", chat.Id, storeErr)
	}

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	if storeErr == nil && !created && reporterDecided(chat.Id, reportedMsgId, user.Id) {
		// The admins decided this member's report of the message: repeating
		// it neither pings them again nor counts on a card.
		text, _ := tr.GetString("reports_already_reviewed")
		_, _ = msg.Reply(b, text, nil)
		return ext.EndGroups
	}
	reportTemplate, _ := tr.GetString("reports_message_template")
	reported := fmt.Sprintf(
		reportTemplate,
		formatting.MentionHtml(user.Id, user.FirstName),
		formatting.MentionHtml(reportedUser.Id, reportedUser.FirstName),
	)
	reportStatuses := reports.GetUserReportStatuses(adminArray)
	var sb strings.Builder
	for _, adminUserId := range adminArray {
		if !reportStatuses[adminUserId] {
			continue
		}
		sb.WriteString(formatting.MentionHtml(adminUserId, "\u2063"))
	}
	reported += sb.String()

	actionButton := func(labelKey, action string) gotgbot.InlineKeyboardButton {
		return gotgbot.InlineKeyboardButton{
			Text: trS(ctxTr(ctx), labelKey),
			CallbackData: encodeCallbackData("report", map[string]string{
				"a": action,
				"u": fmt.Sprint(reportedUser.Id),
				"m": fmt.Sprint(reportedMsgId),
			}),
		}
	}

	_, err = msg.Reply(b,
		reported,
		&gotgbot.SendMessageOpts{
			ParseMode: formatting.HTML,
			ReplyParameters: &gotgbot.ReplyParameters{
				MessageId:                replyMsgId,
				AllowSendingWithoutReply: true,
			},
			ReplyMarkup: gotgbot.InlineKeyboardMarkup{
				InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
					{
						{
							Text: trS(ctxTr(ctx), "reports_button_message"),
							Url:  chat_status.GetMessageLinkFromMessageId(chat, reportedMsgId),
						},
					},
					{
						actionButton("reports_button_kick", "kick"),
						actionButton("reports_button_ban", "ban"),
					},
					{actionButton("reports_button_delete", "delete")},
					{actionButton("reports_button_resolved", "resolved")},
				},
			},
		},
	)
	if err != nil {
		log.Error(err)
	}
	// The group gets the report with its admin mentions and buttons whether
	// or not the card is posted; the card goes second so that a slow log
	// channel does not hold up the group reply, and still goes when that
	// reply failed.
	deliverReport(b, tr, chat, user, msg.ReplyToMessage, reason, storeErr == nil, created)
	if storeErr == nil && created {
		autoMuteReported(b, ctx, tr, chat, msg.ReplyToMessage, reportprefs.MuteThreshold)
	}
	if err != nil {
		return err
	}
	return ext.EndGroups
}

// reporterDecided reports whether a member's stored report of a message was
// decided. On a lookup failure it says no, so the report goes out as usual.
func reporterDecided(chatID, messageID, reporterID int64) bool {
	rep, err := reports.GetByReporter(chatID, messageID, reporterID)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Errorf("[Reports] failed to load report of %d in chat %d: %v", reporterID, chatID, err)
		}
		return false
	}
	return rep.Status != models.ReportStatusOpen
}

// reportWatcher uses the command's pipeline so /disable report still covers
// @admin mentions, then lets the later watcher groups see the message.
func (m moduleStruct) reportWatcher(b *gotgbot.Bot, ctx *ext.Context) error {
	if sender := ctx.EffectiveSender; sender != nil && sender.User != nil {
		_ = helpers.RunCommand(b, ctx, reportDesc.Name)
	}
	return ext.ContinueGroups
}

func (moduleStruct) reports(c *helpers.CommandContext) error {
	args := c.Ctx.Args()[1:]
	tr := i18n.MustNewTranslator(lang.GetLanguage(c.Ctx))
	inPM := c.Msg.Chat.Type == "private"

	action := ""
	if len(args) > 0 {
		action = strings.ToLower(args[0])
	}

	var replyText string
	switch action {
	case "":
		replyText = reportsStatusText(c, tr, inPM)
	case "on", "yes", "true":
		replyText = reportsToggleText(c, tr, inPM, true)
	case "off", "no", "false":
		replyText = reportsToggleText(c, tr, inPM, false)
	case "block", "unblock", "mute", "showblocklist":
		if inPM {
			replyText = trS(tr, "reports_group_only")
			break
		}
		switch action {
		case "mute":
			if len(args) > 1 && !helpers.RunChecks(c, declaredSettingChecks(chatsettings.ReportMuteThreshold)) {
				return ext.EndGroups
			}
			replyText = reportsMuteSetting(tracing.UpdateContext(c.Ctx), tr, c.Chat.Id, args[1:])
		case "showblocklist":
			replyText = reportsBlocklistText(c, tr)
		default:
			replyText = reportsBlockText(c, tr, action == "block")
		}
	default:
		replyText = trS(tr, "reports_invalid_input")
	}

	if _, err := c.Msg.Reply(c.Bot, replyText, formatting.Shtml()); err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}

// reportsStatusText is /reports without arguments: the user's own preference
// in PM, the group's setting and auto-mute line elsewhere.
func reportsStatusText(c *helpers.CommandContext, tr *i18n.Translator, inPM bool) string {
	var text string
	if inPM {
		if reports.GetUserReportSettings(c.User.Id).Status {
			text = trS(tr, "reports_preference_enabled_private")
		} else {
			text = trS(tr, "reports_preference_disabled_private")
		}
	} else {
		settings := chatsettings.ReadReports(c.Chat.Id)
		if settings.Status {
			text = trS(tr, "reports_status_enabled_group")
		} else {
			text = trS(tr, "reports_status_disabled_group")
		}
		text += "\n" + reportsMuteStatus(tr, settings.MuteThreshold)
	}
	return text + trS(tr, "reports_change_settings_hint")
}

// reportsToggleText turns reports on or off: the user's own preference in PM,
// the group's setting elsewhere.
func reportsToggleText(c *helpers.CommandContext, tr *i18n.Translator, inPM, on bool) string {
	var err error
	onKey, offKey := "reports_turned_on_group", "reports_turned_off_group"
	if inPM {
		err = reports.SetUserReportSettings(c.User.Id, on)
		onKey, offKey = "reports_turned_on_personal", "reports_turned_off_personal"
	} else {
		err = chatsettings.ReportsEnabled.Write(chatsettings.WriteContext{Context: tracing.UpdateContext(c.Ctx), ChatID: c.Chat.Id, Title: c.Chat.Title, Bot: c.Bot}, on)
	}
	switch {
	case err != nil:
		return trS(tr, "common_settings_save_failed")
	case on:
		return trS(tr, onKey)
	default:
		return trS(tr, offKey)
	}
}

// reportsBlockText blocks or unblocks the author of the replied-to message
// from reporting in the group.
func reportsBlockText(c *helpers.CommandContext, tr *i18n.Translator, block bool) string {
	usageKey, doneKey, apply := "reports_reply_to_unblock", "reports_user_unblocked", reports.UnblockReportUser
	if block {
		usageKey, doneKey, apply = "reports_reply_to_block", "reports_user_blocked", reports.BlockReportUser
	}
	reply := c.Msg.ReplyToMessage
	if reply == nil {
		return trS(tr, usageKey)
	}
	target := reply.From
	if target == nil {
		return trS(tr, "reports_cannot_report_channel")
	}
	if err := apply(c.Chat.Id, target.Id); err != nil {
		return trS(tr, "common_settings_save_failed")
	}
	text, _ := tr.GetString(doneKey, i18n.TranslationParams{
		"s": formatting.MentionHtml(target.Id, target.FirstName),
	})
	return text
}

// reportsBlocklistText lists the members blocked from reporting in the group.
func reportsBlocklistText(c *helpers.CommandContext, tr *i18n.Translator) string {
	blockedUsers := chatsettings.ReadReports(c.Chat.Id).BlockedList
	if len(blockedUsers) == 0 {
		return trS(tr, "reports_no_blocked_users")
	}
	var builder strings.Builder
	builder.Grow(256)
	builder.WriteString(trS(tr, "reports_blocked_users_header"))
	for _, blockUserId := range blockedUsers {
		bUser, err := c.Bot.GetChat(blockUserId, nil)
		if err != nil {
			log.Error(err)
			continue
		}
		builder.WriteString("\n - ")
		builder.WriteString(formatting.MentionHtml(blockUserId, bUser.FirstName))
	}
	return builder.String()
}

// legacyReportStatus maps the in-group report buttons to report decisions.
var legacyReportStatus = map[string]string{
	"kick":     models.ReportStatusKicked,
	"ban":      models.ReportStatusBanned,
	"delete":   models.ReportStatusDeleted,
	"resolved": models.ReportStatusDismissed,
}

// applyReportPunishment punishes userID in chatID on admin actorID's report
// decision. A ban is recorded with reason; a refusal is an error.
func applyReportPunishment(ctx context.Context, b *gotgbot.Bot, chatID, userID int64, mode punishment.Mode, actorID int64, reason string) error {
	result, err := punishment.Apply(ctx, b, punishment.Request{
		ChatID: chatID, Target: punishment.Target{ID: userID}, Mode: mode,
		Source: models.BanSourceReport, Reason: reason, ActorID: actorID,
	})
	if err == nil && !result.Applied {
		err = errors.New(result.Reason)
	}
	return err
}

func (moduleStruct) markResolvedButtonHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	query, ok := callbackQueryFromContext(ctx)
	if !ok {
		return ext.EndGroups
	}
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	msg := query.Message
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	if msg == nil {
		return answerInvalidCallback(b, ctx, query)
	}
	var replyQuery, replyText string

	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
		return ext.EndGroups
	}

	invalidActionText, _ := tr.GetString("reports_invalid_action")

	action := ""
	userIDRaw := ""
	msgIDRaw := ""
	if decoded, ok := decodeCallbackData(query.Data, "report"); ok {
		action, _ = decoded.Field("a")
		userIDRaw, _ = decoded.Field("u")
		msgIDRaw, _ = decoded.Field("m")
	}
	if action == "" || userIDRaw == "" || msgIDRaw == "" {
		log.Warnf("[Reports] Invalid callback data format: %s", query.Data)
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: invalidActionText})
		return ext.EndGroups
	}
	switch action {
	case "kick", "ban", "delete", "resolved":
	default:
		log.Warnf("[Reports] Invalid callback action: %s", action)
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: invalidActionText})
		return ext.EndGroups
	}
	userId, err := strconv.ParseInt(userIDRaw, 10, 64)
	if err != nil {
		log.Warnf("[Reports] Invalid user ID in callback: %s", userIDRaw)
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: invalidActionText})
		return ext.EndGroups
	}
	msgId, err := strconv.ParseInt(msgIDRaw, 10, 64)
	if err != nil {
		log.Warnf("[Reports] Invalid message ID in callback: %s", msgIDRaw)
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: invalidActionText})
		return ext.EndGroups
	}

	switch action {
	case "kick", "ban":
		if !chat_status.CanUserRestrict(b, ctx, chat, user.Id) {
			chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_restrict_cmd_error", "chat_status_restrict_button_error")
			return ext.EndGroups
		}
		if !chat_status.CanBotRestrict(b, ctx, chat) {
			chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_bot_restrict_error", "chat_status_bot_restrict_error")
			return ext.EndGroups
		}
	case "delete":
		if !chat_status.CanUserDelete(b, ctx, chat, user.Id) {
			chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_delete_cmd_error", "chat_status_delete_button_error")
			return ext.EndGroups
		}
		if !chat_status.CanBotDelete(b, ctx, chat) {
			chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_bot_delete_error", "chat_status_bot_delete_error")
			return ext.EndGroups
		}
	}

	// A message with stored reports is decided once: they are closed before
	// acting, so a decision already taken on the card or by another admin
	// stops this press, and opened again if the action fails. A message
	// reported before reports were stored has none and acts as before.
	status := legacyReportStatus[action]
	closed, err := reports.Resolve(chat.Id, msgId, status, user.Id)
	legacy := false
	if err == nil && len(closed) == 0 {
		var stored bool
		stored, err = reports.HasReports(chat.Id, msgId)
		if err == nil && stored {
			handledText, _ := tr.GetString("reports_already_handled")
			_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: handledText})
			return ext.EndGroups
		}
		legacy = true
	}
	if err != nil {
		log.Errorf("[Reports] failed to close reports of message %d in %d for %s: %v", msgId, chat.Id, action, err)
		failedText, _ := tr.GetString("reports_action_failed")
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: failedText})
		return err
	}
	reopen := func() {
		if reopenErr := reports.Reopen(closed, status); reopenErr != nil {
			log.Errorf("[Reports] failed to reopen reports %v after a failed %s: %v", closed, action, reopenErr)
		}
	}

	switch action {
	case "kick":
		replyQuery, _ = tr.GetString("reports_success_kick")
		kickedText, _ := tr.GetString("reports_user_kicked")
		actionBy, _ := tr.GetString("reports_action_by", i18n.TranslationParams{"s": formatting.MentionHtml(user.Id, user.FirstName)})
		replyText = fmt.Sprintf("%s\n%s", kickedText, actionBy)
		if err := applyReportPunishment(tracing.UpdateContext(ctx), b, chat.Id, userId, punishment.Kick, user.Id, ""); err != nil {
			reopen()
			log.Error(err)
			return err
		}
		recordModerationEvent(chat.Id, user.Id, userId, models.ModerationActionKick, "")
	case "ban":
		replyQuery, _ = tr.GetString("reports_success_ban")
		bannedText, _ := tr.GetString("reports_user_banned")
		actionBy, _ := tr.GetString("reports_action_by", i18n.TranslationParams{"s": formatting.MentionHtml(user.Id, user.FirstName)})
		replyText = fmt.Sprintf("%s\n%s", bannedText, actionBy)
		if err := applyReportPunishment(tracing.UpdateContext(ctx), b, chat.Id, userId, punishment.Ban, user.Id, ""); err != nil {
			reopen()
			log.Error(err)
			return err
		}
		recordModerationEvent(chat.Id, user.Id, userId, models.ModerationActionBan, "")

	case "delete":
		replyQuery, _ = tr.GetString("reports_success_delete")
		deletedText, _ := tr.GetString("reports_message_deleted")
		actionBy, _ := tr.GetString("reports_action_by", i18n.TranslationParams{"s": formatting.MentionHtml(user.Id, user.FirstName)})
		replyText = fmt.Sprintf("%s\n%s", deletedText, actionBy)
		if err := helpers.DeleteMessageWithErrorHandling(b, chat.Id, msgId); err != nil {
			reopen()
			log.Error(err)
			return err
		}
	case "resolved":
		replyQuery, _ = tr.GetString("reports_resolved_success")
		replyText, _ = tr.GetString("reports_resolved_by", i18n.TranslationParams{"s": formatting.MentionHtml(user.Id, user.FirstName)})
	}
	settleReportAutoMute(b, ctx, chat.Id, userId, status)
	if legacy {
		// Close any report stored while the action ran, so its card stops
		// acting.
		if _, err := reports.Resolve(chat.Id, msgId, status, user.Id); err != nil {
			log.Errorf("[Reports] failed to close reports of message %d in %d after %s: %v", msgId, chat.Id, action, err)
		}
	}
	_, _, err = msg.EditText(b, &gotgbot.EditMessageTextOpts{
		Text: replyText, ChatId: chat.Id,
		ParseMode: formatting.HTML,
	})
	if err != nil {
		log.Error(err)
		return err
	}

	_, err = query.Answer(b,
		&gotgbot.AnswerCallbackQueryOpts{
			Text: replyQuery,
		},
	)
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func LoadReports(dispatcher *ext.Dispatcher) {
	SetModuleEnabled(reportsModule.moduleName, true)

	dispatcher.AddHandlerToGroup(
		handlers.NewMessage(
			func(msg *gotgbot.Message) bool {
				return adminMentionRegex.MatchString(msg.Text)
			},
			reportsModule.reportWatcher,
		),
		reportsModule.handlerGroup,
	)
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("report"), reportsModule.markResolvedButtonHandler))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix(reportCardNamespace+"|"), reportsModule.reportCardHandler))
	helpers.WrapCommand(dispatcher, reportDesc, pipelineHandler(reportsModule.report))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:           "reports",
		RequiredChecks: declaredSettingChecks(chatsettings.ReportsEnabled),
	}, reportsModule.reports)
	startReportRetentionSweeper()
}

func init() {
	RegisterLegacyModule("Reports", 110, LoadReports)
}
