package modules

import (
	"fmt"
	"html"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/approvals"
	dbcaptcha "github.com/divkix/Alita_Robot/alita/db/captcha"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
	"github.com/divkix/Alita_Robot/alita/utils/extraction"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

var approvalsModule = moduleStruct{
	moduleName: "Approvals",
}

const approvedUsersInlineLimit = 50

func (m moduleStruct) approveUser(b *gotgbot.Bot, ctx *ext.Context) error {
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
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	if !chat_status.RequireUserAdmin(b, ctx, chat, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
		return ext.EndGroups
	}

	targetUserID, reason := extraction.ExtractUserAndText(b, ctx)
	switch targetUserID {
	case -1:
		return ext.EndGroups
	case 0:
		text, _ := tr.GetString("common_no_user_specified")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if approvals.IsUserApproved(chat.Id, targetUserID) {
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_already_approved")
		_, err := msg.Reply(b, fmt.Sprintf(text, formatting.MentionHtml(targetUserID, "")), formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	_, approverName, found := extraction.GetUserInfo(user.Id)
	if !found {
		approverName = user.FirstName
	}

	if err := approvals.AddApprovedUser(chat.Id, targetUserID, user.Id, reason); err != nil {
		log.Errorf("[Approvals] Failed to approve user %d in chat %d: %v", targetUserID, chat.Id, err)
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_approve_error")
		_, _ = msg.Reply(b, text, nil)
		return ext.EndGroups
	}
	if attempt, err := dbcaptcha.GetCaptchaAttemptIncludingExpired(targetUserID, chat.Id); err != nil {
		log.Errorf("[Approvals] Failed to load captcha attempt for approved user %d: %v", targetUserID, err)
	} else if attempt != nil {
		released, releaseErr := releaseIncompleteCaptchaAttempt(b, attempt)
		if released && attempt.MessageID > 0 {
			_ = helpers.DeleteMessageWithErrorHandling(b, chat.Id, attempt.MessageID)
		}
		if releaseErr != nil {
			log.Errorf("[Approvals] Approved user %d but captcha release will retry: %v", targetUserID, releaseErr)
		}
	}

	text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_user_approved")
	baseStr := fmt.Sprintf(text,
		formatting.MentionHtml(targetUserID, extractDisplayName(targetUserID)),
		html.EscapeString(approverName),
	)
	if reason != "" {
		temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_reason")
		baseStr += fmt.Sprintf(temp, html.EscapeString(reason))
	}
	_, err := msg.Reply(b, baseStr, formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}

func (m moduleStruct) unapproveUser(b *gotgbot.Bot, ctx *ext.Context) error {
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
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	if !chat_status.RequireUserAdmin(b, ctx, chat, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
		return ext.EndGroups
	}

	targetUserID, _ := extraction.ExtractUserAndText(b, ctx)
	switch targetUserID {
	case -1:
		return ext.EndGroups
	case 0:
		text, _ := tr.GetString("common_no_user_specified")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if !approvals.IsUserApproved(chat.Id, targetUserID) {
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_not_approved")
		_, err := msg.Reply(b, fmt.Sprintf(text, formatting.MentionHtml(targetUserID, "")), formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if err := approvals.RemoveApprovedUser(chat.Id, targetUserID); err != nil {
		log.Errorf("[Approvals] Failed to unapprove user %d in chat %d: %v", targetUserID, chat.Id, err)
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_unapprove_error")
		_, _ = msg.Reply(b, text, nil)
		return ext.EndGroups
	}

	text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_user_unapproved")
	_, err := msg.Reply(b, fmt.Sprintf(text,
		formatting.MentionHtml(targetUserID, extractDisplayName(targetUserID)),
	), formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}

func (m moduleStruct) checkApprovalStatus(b *gotgbot.Bot, ctx *ext.Context) error {
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
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	if !chat_status.RequireUserAdmin(b, ctx, chat, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
		return ext.EndGroups
	}

	targetUserID, _ := extraction.ExtractUserAndText(b, ctx)
	switch targetUserID {
	case -1:
		return ext.EndGroups
	case 0:
		text, _ := tr.GetString("common_no_user_specified")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	approvedUsers := approvals.GetApprovedUsers(chat.Id)
	var foundUser *db.ApprovedUsers
	for _, a := range approvedUsers {
		if a.UserID == targetUserID {
			foundUser = a
			break
		}
	}

	if foundUser == nil {
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_check_not_approved")
		_, err := msg.Reply(b, fmt.Sprintf(text,
			html.EscapeString(extractDisplayName(targetUserID)),
		), formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	_, approverName, approverFound := extraction.GetUserInfo(foundUser.ApprovedBy)
	if !approverFound {
		approverName = strconv.FormatInt(foundUser.ApprovedBy, 10)
	}

	_, targetName, targetFound := extraction.GetUserInfo(foundUser.UserID)
	if !targetFound {
		targetName = strconv.FormatInt(foundUser.UserID, 10)
	}

	text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_check_status")
	dateStr := foundUser.CreatedAt.Format("2006-01-02")
	baseStr := fmt.Sprintf(text,
		html.EscapeString(targetName),
		html.EscapeString(dateStr),
		html.EscapeString(approverName),
	)
	if foundUser.Reason != "" {
		temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_reason")
		baseStr += fmt.Sprintf(temp, html.EscapeString(foundUser.Reason))
	}
	_, err := msg.Reply(b, baseStr, formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}

func (m moduleStruct) listApprovedUsers(b *gotgbot.Bot, ctx *ext.Context) error {
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
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	if !chat_status.RequireUserAdmin(b, ctx, chat, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
		return ext.EndGroups
	}

	approvedUsers := approvals.GetApprovedUsers(chat.Id)
	if len(approvedUsers) == 0 {
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_none_approved")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if len(approvedUsers) <= approvedUsersInlineLimit {
		listHeader, _ := tr.GetString(strings.ToLower(m.moduleName) + "_list_header")
		listItem, _ := tr.GetString(strings.ToLower(m.moduleName) + "_list_item")
		listReason, _ := tr.GetString(strings.ToLower(m.moduleName) + "_list_reason")
		var sb strings.Builder
		sb.WriteString(listHeader)
		for _, a := range approvedUsers {
			_, name, found := extraction.GetUserInfo(a.UserID)
			if !found {
				name = strconv.FormatInt(a.UserID, 10)
			}
			item := fmt.Sprintf(listItem, html.EscapeString(name))
			if a.Reason != "" {
				item += fmt.Sprintf(listReason, html.EscapeString(a.Reason))
			}
			fmt.Fprintf(&sb, "\n%s", item)
		}
		_, err := msg.Reply(b, sb.String(), formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	tmpFile, err := os.CreateTemp("", "approved-*.txt")
	if err != nil {
		log.Errorf("[Approvals] Failed to create temp file: %v", err)
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_list_file_error")
		_, _ = msg.Reply(b, text, nil)
		return ext.EndGroups
	}
	defer func() { _ = tmpFile.Close() }()
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	fileHeader, _ := tr.GetString(strings.ToLower(m.moduleName) + "_list_file_header")
	fileItem, _ := tr.GetString(strings.ToLower(m.moduleName) + "_list_file_item")
	fileReason, _ := tr.GetString(strings.ToLower(m.moduleName) + "_list_reason")
	var fileSb strings.Builder
	fmt.Fprintf(&fileSb, fileHeader, chat.Id)
	fmt.Fprintf(&fileSb, "%s\n\n", time.Now().Format(time.RFC3339))
	for i, a := range approvedUsers {
		_, name, found := extraction.GetUserInfo(a.UserID)
		if !found {
			name = strconv.FormatInt(a.UserID, 10)
		}
		item := fmt.Sprintf(fileItem, i+1, name)
		if a.Reason != "" {
			item += fmt.Sprintf(fileReason, a.Reason)
		}
		fmt.Fprintf(&fileSb, "%s\n", item)
	}

	if _, err := tmpFile.WriteString(fileSb.String()); err != nil {
		log.Errorf("[Approvals] Failed to write temp file: %v", err)
		return ext.EndGroups
	}
	_ = tmpFile.Close()

	openedFile, err := os.Open(tmpFile.Name())
	if err != nil {
		log.Errorf("[Approvals] Failed to open temp file: %v", err)
		return ext.EndGroups
	}
	defer func() { _ = openedFile.Close() }()

	text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_approved_list_file_caption")
	_, err = b.SendDocument(
		chat.Id,
		gotgbot.InputFileByReader("approved_users.txt", openedFile),
		&gotgbot.SendDocumentOpts{
			Caption: func() string { return fmt.Sprintf(text, html.EscapeString(chat.Title)) }(),
			ReplyParameters: &gotgbot.ReplyParameters{
				MessageId:                msg.MessageId,
				AllowSendingWithoutReply: true,
			},
		},
	)
	if err != nil {
		log.Errorf("[Approvals] Failed to send document: %v", err)
		return err
	}

	return ext.EndGroups
}

//nolint:dupl // Similar to other rmAll handlers with distinct callback data and messages
func (m moduleStruct) unapproveAllHandler(b *gotgbot.Bot, ctx *ext.Context) error {
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

	text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_unapproveall_ask")
	yesText, _ := tr.GetString("button_yes")
	noText, _ := tr.GetString("button_no")
	_, err := msg.Reply(b, text,
		&gotgbot.SendMessageOpts{
			ReplyMarkup: gotgbot.InlineKeyboardMarkup{
				InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
					{
						{
							Text:         yesText,
							CallbackData: encodeCallbackData("rmAllApprovals", map[string]string{"a": "yes"}),
						},
						{
							Text:         noText,
							CallbackData: encodeCallbackData("rmAllApprovals", map[string]string{"a": "no"}),
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

func (m moduleStruct) unapproveAllCallback(b *gotgbot.Bot, ctx *ext.Context) error {
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

	action := ""
	if decoded, ok := decodeCallbackData(query.Data, "rmAllApprovals"); ok {
		action, _ = decoded.Field("a")
	}
	if action == "" {
		log.Warnf("[Approvals] Invalid callback data format: %s", query.Data)
		return answerInvalidCallback(b, ctx, query)
	}

	var helpText string
	switch action {
	case "yes":
		if query.Message == nil {
			log.Warn("[Approvals] Cannot remove all approved users: message was deleted")
			text, _ := tr.GetString("common_callback_message_unavailable")
			_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: text})
			return ext.EndGroups
		}
		defer error_handling.RecoverFromPanic("rmAllApprovals", "approvals")
		if err := approvals.RemoveAllApprovedUsers(query.Message.GetChat().Id); err != nil {
			log.WithFields(log.Fields{
				"chatId": query.Message.GetChat().Id,
				"error":  err,
			}).Error("Failed to remove all approved users")
			helpText, _ = tr.GetString(strings.ToLower(m.moduleName)+"_unapproveall_error", i18n.TranslationParams{"error": err.Error()})
		} else {
			helpText, _ = tr.GetString(strings.ToLower(m.moduleName) + "_unapproveall_done")
		}
	case "no":
		if query.Message == nil {
			log.Warn("[Approvals] Cannot cancel unapproveall: message was deleted")
			text, _ := tr.GetString("common_callback_message_unavailable")
			_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: text})
			return ext.EndGroups
		}
		helpText, _ = tr.GetString(strings.ToLower(m.moduleName) + "_unapproveall_cancel")
	default:
		if query.Message == nil {
			log.Warnf("[Approvals] Unexpected action '%s' with nil message", action)
			text, _ := tr.GetString("common_callback_message_unavailable")
			_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: text})
			return ext.EndGroups
		}
		log.WithFields(log.Fields{
			"action": action,
			"chatId": query.Message.GetChat().Id,
		}).Warn("[Approvals] Unexpected callback action")
		helpText, _ = tr.GetString(strings.ToLower(m.moduleName) + "_unapproveall_cancel")
	}

	_, _, err := query.Message.EditText(b, &gotgbot.EditMessageTextOpts{Text: helpText, ParseMode: formatting.HTML})
	if err != nil {
		log.Error(err)
		return err
	}

	_, err = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: helpText})
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func extractDisplayName(userID int64) string {
	_, name, found := extraction.GetUserInfo(userID)
	if found && name != "" {
		return name
	}
	return strconv.FormatInt(userID, 10)
}

//nolint:dupl // Pattern matches other LoadXxx functions
func LoadApprovals(dispatcher *ext.Dispatcher) {
	DefaultHelpRegistry().AbleMap[approvalsModule.moduleName] = true

	dispatcher.AddHandler(handlers.NewCommand("approve", approvalsModule.approveUser))
	dispatcher.AddHandler(handlers.NewCommand("unapprove", approvalsModule.unapproveUser))
	dispatcher.AddHandler(handlers.NewCommand("approval", approvalsModule.checkApprovalStatus))
	dispatcher.AddHandler(handlers.NewCommand("approved", approvalsModule.listApprovedUsers))
	dispatcher.AddHandler(handlers.NewCommand("unapproveall", approvalsModule.unapproveAllHandler))

	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("rmAllApprovals"), approvalsModule.unapproveAllCallback))
}

func init() {
	RegisterLegacyModule("Approvals", 40, LoadApprovals)
}
