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
	"github.com/divkix/Alita_Robot/alita/utils/extraction"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	"github.com/divkix/Alita_Robot/alita/utils/tracing"
)

var approvalsModule = moduleStruct{
	moduleName: "Approvals",
}

const approvedUsersInlineLimit = 50

func (m moduleStruct) approveUser(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	targetUserID, reason := extraction.ExtractUserAndText(b, ctx)
	switch targetUserID {
	case -1:
		return ext.EndGroups
	case 0:
		text := tr.String("common_no_user_specified")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			return err
		}
		return ext.EndGroups
	}

	if approvals.IsUserApproved(chat.Id, targetUserID) {
		text := tr.String(strings.ToLower(m.moduleName) + "_already_approved")
		_, err := msg.Reply(b, fmt.Sprintf(text, formatting.MentionHtml(targetUserID, extractDisplayName(targetUserID))), formatting.Shtml())
		if err != nil {
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
		text := tr.String(strings.ToLower(m.moduleName) + "_approve_error")
		_, _ = msg.Reply(b, text, nil)
		return ext.EndGroups
	}
	if err := settleMemberCaptcha(b, chat.Id, targetUserID, captchaReleased); err != nil {
		log.WithContext(tracing.UpdateContext(ctx)).Errorf("[Approvals] Approved user %d but captcha release will retry: %v", targetUserID, err)
	}

	reasonStr := ""
	if reason != "" {
		temp := tr.String(strings.ToLower(m.moduleName) + "_reason")
		reasonStr = fmt.Sprintf(temp, html.EscapeString(reason))
	}
	text := tr.String(strings.ToLower(m.moduleName) + "_user_approved")
	baseStr := fmt.Sprintf(text,
		formatting.MentionHtml(targetUserID, extractDisplayName(targetUserID)),
		html.EscapeString(approverName),
		reasonStr,
	)
	_, err := msg.Reply(b, baseStr, formatting.Shtml())
	if err != nil {
		return err
	}
	return ext.EndGroups
}

func (m moduleStruct) unapproveUser(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	targetUserID, _ := extraction.ExtractUserAndText(b, ctx)
	switch targetUserID {
	case -1:
		return ext.EndGroups
	case 0:
		text := tr.String("common_no_user_specified")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			return err
		}
		return ext.EndGroups
	}

	if !approvals.IsUserApproved(chat.Id, targetUserID) {
		text := tr.String(strings.ToLower(m.moduleName) + "_not_approved")
		_, err := msg.Reply(b, fmt.Sprintf(text, formatting.MentionHtml(targetUserID, extractDisplayName(targetUserID))), formatting.Shtml())
		if err != nil {
			return err
		}
		return ext.EndGroups
	}

	if err := approvals.RemoveApprovedUser(chat.Id, targetUserID); err != nil {
		log.Errorf("[Approvals] Failed to unapprove user %d in chat %d: %v", targetUserID, chat.Id, err)
		text := tr.String(strings.ToLower(m.moduleName) + "_unapprove_error")
		_, _ = msg.Reply(b, text, nil)
		return ext.EndGroups
	}

	text := tr.String(strings.ToLower(m.moduleName) + "_user_unapproved")
	_, err := msg.Reply(b, fmt.Sprintf(text,
		formatting.MentionHtml(targetUserID, extractDisplayName(targetUserID)),
	), formatting.Shtml())
	if err != nil {
		return err
	}
	return ext.EndGroups
}

func (m moduleStruct) checkApprovalStatus(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	targetUserID, _ := extraction.ExtractUserAndText(b, ctx)
	switch targetUserID {
	case -1:
		return ext.EndGroups
	case 0:
		text := tr.String("common_no_user_specified")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			return err
		}
		return ext.EndGroups
	}

	approvedUsers := approvals.GetApprovedUsersContext(tracing.UpdateContext(ctx), chat.Id)
	var foundUser *db.ApprovedUsers
	for _, a := range approvedUsers {
		if a.UserID == targetUserID {
			foundUser = a
			break
		}
	}

	if foundUser == nil {
		text := tr.String(strings.ToLower(m.moduleName) + "_check_not_approved")
		_, err := msg.Reply(b, fmt.Sprintf(text,
			html.EscapeString(extractDisplayName(targetUserID)),
		), formatting.Shtml())
		if err != nil {
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

	text := tr.String(strings.ToLower(m.moduleName) + "_check_status")
	dateStr := foundUser.CreatedAt.Format("2006-01-02")
	reasonStr := ""
	if foundUser.Reason != "" {
		temp := tr.String(strings.ToLower(m.moduleName) + "_reason")
		// The status template already puts the reason slot on its own line.
		reasonStr = strings.TrimPrefix(fmt.Sprintf(temp, html.EscapeString(foundUser.Reason)), "\n")
	}
	baseStr := fmt.Sprintf(text,
		html.EscapeString(targetName),
		html.EscapeString(dateStr),
		html.EscapeString(approverName),
		reasonStr,
	)
	_, err := msg.Reply(b, baseStr, formatting.Shtml())
	if err != nil {
		return err
	}
	return ext.EndGroups
}

func (m moduleStruct) listApprovedUsers(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	approvedUsers := approvals.GetApprovedUsersContext(tracing.UpdateContext(ctx), chat.Id)
	if len(approvedUsers) == 0 {
		text := tr.String(strings.ToLower(m.moduleName) + "_none_approved")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			return err
		}
		return ext.EndGroups
	}

	if len(approvedUsers) <= approvedUsersInlineLimit {
		listHeader := tr.String(strings.ToLower(m.moduleName) + "_list_header")
		listItem := tr.String(strings.ToLower(m.moduleName) + "_list_item")
		listReason := tr.String(strings.ToLower(m.moduleName) + "_list_reason")
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
			return err
		}
		return ext.EndGroups
	}

	tmpFile, err := os.CreateTemp("", "approved-*.txt")
	if err != nil {
		log.Errorf("[Approvals] Failed to create temp file: %v", err)
		text := tr.String(strings.ToLower(m.moduleName) + "_list_file_error")
		_, _ = msg.Reply(b, text, nil)
		return ext.EndGroups
	}
	defer func() { _ = tmpFile.Close() }()
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	fileHeader := tr.String(strings.ToLower(m.moduleName) + "_list_file_header")
	fileItem := tr.String(strings.ToLower(m.moduleName) + "_list_file_item")
	fileReason := tr.String(strings.ToLower(m.moduleName) + "_list_reason")
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

	text := tr.String(strings.ToLower(m.moduleName) + "_approved_list_file_caption")
	_, err = b.SendDocument(
		msg.Chat.Id,
		gotgbot.InputFileByReader("approved_users.txt", openedFile),
		&gotgbot.SendDocumentOpts{
			Caption:   fmt.Sprintf(text, html.EscapeString(chat.Title)),
			ParseMode: formatting.HTML,
			ReplyParameters: &gotgbot.ReplyParameters{
				MessageId:                msg.MessageId,
				AllowSendingWithoutReply: true,
			},
		},
	)
	if err != nil {
		return err
	}

	return ext.EndGroups
}

// approvalsClearAll is the /unapproveall owner confirmation: the shared
// skeleton asks, authorizes and answers; these closures hold only the
// approvals work.
var approvalsClearAll = ownerConfirmSpec{
	Namespace: "rmAllApprovals",
	AskKey:    strings.ToLower(approvalsModule.moduleName) + "_unapproveall_ask",
	Commit: func(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, tr *i18n.Translator, actor *gotgbot.User) (string, i18n.TranslationParams, error) {
		if err := approvals.RemoveAllApprovedUsers(chat.Id); err != nil {
			log.WithFields(log.Fields{"chatId": chat.Id, "error": err}).Error("Failed to remove all approved users")
			return "error_generic", nil, nil
		}
		return "", nil, nil
	},
	Keys: ownerConfirmKeys{
		Confirmed: strings.ToLower(approvalsModule.moduleName) + "_unapproveall_done",
		Cancelled: strings.ToLower(approvalsModule.moduleName) + "_unapproveall_cancel",
		Failed:    "error_generic",
	},
}

var approvalsClearAllMessage, approvalsClearAllCallback = registerOwnerConfirm(approvalsClearAll)

func (m moduleStruct) unapproveAllHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	return approvalsClearAllMessage(b, ctx)
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
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "approve", PMGroupArg: helpers.PMGroupArgOwn,
		RequiredChecks: []helpers.Check{helpers.RequireConnectedAdmin(helpers.BotAdminRequired), helpers.RequireUserAdmin()},
	}, pipelineHandler(approvalsModule.approveUser))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "unapprove", PMGroupArg: helpers.PMGroupArgOwn,
		RequiredChecks: []helpers.Check{helpers.RequireConnectedAdmin(helpers.BotAdminRequired), helpers.RequireUserAdmin()},
	}, pipelineHandler(approvalsModule.unapproveUser))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "approval", PMGroupArg: helpers.PMGroupArgOwn,
		RequiredChecks: []helpers.Check{helpers.RequireConnectedAdmin(helpers.BotAdminRequired), helpers.RequireUserAdmin()},
	}, pipelineHandler(approvalsModule.checkApprovalStatus))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "approved", PMGroupArg: helpers.PMGroupArgNone,
		RequiredChecks: []helpers.Check{helpers.RequireConnectedAdmin(helpers.BotAdminRequired), helpers.RequireUserAdmin()},
	}, pipelineHandler(approvalsModule.listApprovedUsers))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "unapproveall", PMGroupArg: helpers.PMGroupArgNone,
		RequiredChecks: []helpers.Check{groupOnlyChatCheck, helpers.RequireGroup(), helpers.RequireUserOwner()},
	}, pipelineHandler(approvalsModule.unapproveAllHandler))

	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("rmAllApprovals|"), approvalsClearAllCallback))
}

func init() {
	RegisterLegacyModule("Approvals", 40, LoadApprovals)
}
