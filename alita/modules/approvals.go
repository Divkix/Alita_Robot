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
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
	"github.com/divkix/Alita_Robot/alita/utils/extraction"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
)

var approvalsModule = moduleStruct{
	moduleName: "Approvals",
}

const approvedUsersInlineLimit = 50

/*
	Used to approve a user in the group!

Connection - true, true
Admin can approve a user in the chat
*/
// approveUser handles the /approve command to add a user to the approved list.
// Approved users are immune to anti-spam measures (antiflood, blacklists, locks, captcha, antispam).
func (m moduleStruct) approveUser(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	// connection status
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

	// Permission checks
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

	// Check if already approved
	if approvals.IsUserApproved(chat.Id, targetUserID) {
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_already_approved")
		_, err := msg.Reply(b, fmt.Sprintf(text, formatting.MentionHtml(targetUserID, "")), formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	// Resolve approved_by user name for display
	_, approverName, found := extraction.GetUserInfo(user.Id)
	if !found {
		approverName = user.FirstName
	}

	// Reason is optional; default to empty string
	if err := approvals.AddApprovedUser(chat.Id, targetUserID, user.Id, reason); err != nil {
		log.Errorf("[Approvals] Failed to approve user %d in chat %d: %v", targetUserID, chat.Id, err)
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_approve_error")
		_, _ = msg.Reply(b, text, nil)
		return ext.EndGroups
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

/*
	Used to remove a user from the approved list!

Connection - true, true
Admin can unapprove a user in the chat
*/
// unapproveUser handles the /unapprove command to remove a user from the approved list.
func (m moduleStruct) unapproveUser(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	// connection status
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

	// Permission checks
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

	// Check if actually approved
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

/*
	Used to check a user's approval status!

Connection - true, true
Admin can check a user's approval status in the chat
*/
// checkApprovalStatus handles the /approval command to check if a user is approved.
func (m moduleStruct) checkApprovalStatus(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	// connection status
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

	// Permission checks
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

	// Resolve names for display
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

/*
	Used to list all approved users in the chat!

Connection - true, true
Admin can list all approved users in the chat
*/
// listApprovedUsers handles the /approved command to list all approved users.
// If the list exceeds 50 users, sends a .txt file instead of an inline message.
func (m moduleStruct) listApprovedUsers(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	// connection status
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

	// Permission checks
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

	// If the list is small, send it inline
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

	// Large list: send as .txt file
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

/*
	Used to remove all approved users from the chat!

Only chat creator can use this command with a confirmation button.
*/
// unapproveAllHandler handles the /unapproveall command.
// Sends an inline keyboard for confirmation before bulk removal.
//nolint:dupl // Similar to other rmAll handlers with distinct callback data and messages
func (m moduleStruct) unapproveAllHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	// Permission checks
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

// unapproveAllCallback processes the confirmation callback for /unapproveall.
func (m moduleStruct) unapproveAllCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	query, ok := callbackQueryFromContext(ctx)
	if !ok {
		return ext.EndGroups
	}
	user := query.From
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	// Permission checks
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
		text, _ := tr.GetString("common_callback_invalid_request")
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return ext.EndGroups
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

	_, _, err := query.Message.EditText(b,
		helpText,
		&gotgbot.EditMessageTextOpts{ParseMode: formatting.HTML},
	)
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

// extractDisplayName returns a display name for a user ID by looking up in DB.
// Falls back to the raw numeric ID if not found in database.
func extractDisplayName(userID int64) string {
	_, name, found := extraction.GetUserInfo(userID)
	if found && name != "" {
		return name
	}
	return strconv.FormatInt(userID, 10)
}

// LoadApprovals registers all approvals module handlers with the dispatcher.
//
//nolint:dupl // Pattern matches other LoadXxx functions
func LoadApprovals(dispatcher *ext.Dispatcher) {
	DefaultHelpRegistry().AbleMap.Store(approvalsModule.moduleName, true)

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
