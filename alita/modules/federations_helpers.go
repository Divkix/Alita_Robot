package modules

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db/federations"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/utils/extraction"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	log "github.com/sirupsen/logrus"
)

// resolveUserIDFromArg parses a user ID from a string argument.
func resolveUserIDFromArg(s string) (int64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	uid, err := strconv.ParseInt(s, 10, 64)
	if err == nil && uid > 0 {
		return uid, true
	}
	return 0, false
}

// sendFederationBanReason sends the ban reason for a user in a specific federation.
func sendFederationBanReason(c *helpers.CommandContext, fedID string, userID int64) error {
	fed, err := federations.GetFederationByIDCached(fedID)
	if err != nil {
		if err == federations.ErrFederationNotFound {
			_ = sendSimpleReply(c, "federations_fed_not_found")
		} else {
			_ = sendSimpleReply(c, "common_db_error")
		}
		return ext.EndGroups
	}

	if userID == 0 {
		_ = sendSimpleReply(c, "federations_fban_no_user_specified")
		return ext.EndGroups
	}

	reason, bannedAt, err := federations.GetBanReason(fed.FedID, userID)
	if err != nil {
		if err == federations.ErrFederationNotFound {
			targetName := fmt.Sprintf("%d", userID)
			chat, chatErr := c.Bot.GetChat(userID, nil)
			if chatErr == nil {
				targetName = formatting.MentionHtml(chat.Id, chat.FirstName)
			}
			_ = sendSimpleReply(c, "federations_fbanstat_not_banned", targetName, fed.Name)
			return ext.EndGroups
		}
		_ = sendSimpleReply(c, "common_db_error")
		return ext.EndGroups
	}

	targetName := fmt.Sprintf("%d", userID)
	chat, chatErr := c.Bot.GetChat(userID, nil)
	if chatErr == nil {
		targetName = formatting.MentionHtml(chat.Id, chat.FirstName)
	}

	dateStr := bannedAt.Format(time.RFC1123)
	if reason == "" {
		reason = "No reason provided"
	}

	_ = sendSimpleReply(c, "federations_fbanstat_reason", targetName, fed.Name, reason, dateStr)
	return ext.EndGroups
}

// resolveTargetUser extracts a target user ID from a reply or command argument.
// Returns 0, false when no valid target could be determined.
func resolveTargetUser(c *helpers.CommandContext) (int64, bool) {
	args := c.Ctx.Args()

	if c.Msg.ReplyToMessage != nil && len(args) == 1 {
		sender := c.Msg.ReplyToMessage.GetSender()
		if sender != nil {
			return sender.Id(), true
		}
	}

	uid, _ := extraction.ExtractUserAndText(c.Bot, c.Ctx)
	if uid == -1 {
		return 0, false
	}
	if uid == 0 {
		_ = sendSimpleReply(c, "federations_fban_no_user_specified")
		return 0, false
	}
	return uid, true
}

// getDisplayName returns an HTML mention for a user, falling back to their ID.
func getDisplayName(b *gotgbot.Bot, userID int64) string {
	name := fmt.Sprintf("%d", userID)
	chat, err := b.GetChat(userID, nil)
	if err != nil {
		return name
	}
	chatInfo := chat.ToChat()
	return formatting.MentionHtml(chatInfo.Id, chatInfo.FirstName)
}

// actionType is a federation action used for notifications/logging.
type actionType string

const (
	actionBanned          actionType = "banned"
	actionUnbanned        actionType = "unbanned"
	actionPromoted        actionType = "promoted"
	actionDemoted         actionType = "demoted"
	actionSelfDemoted     actionType = "self-demoted"
	actionSettingsChanged actionType = "changed settings"
)

// notifyFederationAction sends a notification to the federation owner and log chat
// after a federation action, respecting notification and log settings.
func notifyFederationAction(c *helpers.CommandContext, fed *models.Federation, action actionType, targetID int64, detail string) {
	settings, err := federations.GetSettingsCached(fed.FedID)
	if err != nil {
		log.Errorf("[Federations] notifyFederationAction settings: %v", err)
		return
	}

	text := buildNotificationText(c, fed, action, targetID, detail)

	if settings.NotificationsEnabled {
		_, _ = helpers.SendMessageWithErrorHandling(c.Bot, fed.OwnerID, text, formatting.Shtml())
	}

	if settings.LogChatID > 0 {
		_, _ = helpers.SendMessageWithErrorHandling(c.Bot, settings.LogChatID, text, formatting.Shtml())
	}
}

// buildNotificationText builds the HTML notification body for a federation action.
func buildNotificationText(c *helpers.CommandContext, fed *models.Federation, action actionType, targetID int64, detail string) string {
	actor := getDisplayName(c.Bot, c.User.Id)
	var target string
	if targetID > 0 {
		target = getDisplayName(c.Bot, targetID)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "<b>Federation action</b> in <b>%s</b>\n", formatting.HtmlEscape(fed.Name))
	if targetID > 0 {
		fmt.Fprintf(&sb, "%s has been %s by %s.", target, action, actor)
	} else {
		fmt.Fprintf(&sb, "%s %s.", actor, action)
	}
	if detail != "" {
		fmt.Fprintf(&sb, "\nDetail: %s", formatting.HtmlEscape(detail))
	}
	fmt.Fprintf(&sb, "\nDate: %s", time.Now().Format(time.RFC1123))
	return sb.String()
}

// parseOnOff converts common boolean strings to a bool and reports validity.
func parseOnOff(s string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "on", "yes", "true", "1":
		return true, true
	case "off", "no", "false", "0":
		return false, true
	default:
		return false, false
	}
}

// downloadRepliedDocument downloads a document replied to the current message.
// It enforces a maximum file size and only allows the configured extensions.
func downloadRepliedDocument(c *helpers.CommandContext, allowedExts []string, maxBytes int64) ([]byte, string, error) {
	if c.Msg.ReplyToMessage == nil || c.Msg.ReplyToMessage.Document == nil {
		return nil, "federations_import_no_document", fmt.Errorf("no replied document")
	}

	doc := c.Msg.ReplyToMessage.Document
	fileName := strings.ToLower(doc.FileName)

	valid := false
	for _, ext := range allowedExts {
		if strings.HasSuffix(fileName, ext) {
			valid = true
			break
		}
	}
	if !valid {
		return nil, "federations_import_invalid_file", fmt.Errorf("invalid file type")
	}

	if doc.FileSize > maxBytes {
		return nil, "federations_import_file_too_large", fmt.Errorf("file too large")
	}

	file, err := c.Bot.GetFile(doc.FileId, nil)
	if err != nil {
		log.Errorf("[Federations] GetFile failed: %v", err)
		return nil, "federations_import_download_failed", err
	}

	const fileBaseURL = "https://api.telegram.org/file/bot"
	base, err := url.Parse(fileBaseURL)
	if err != nil {
		return nil, "federations_import_download_failed", err
	}
	parsed, err := url.Parse(fmt.Sprintf("%s%s/%s", fileBaseURL, c.Bot.Token, file.FilePath))
	if err != nil {
		return nil, "federations_import_download_failed", err
	}
	if parsed.Scheme != base.Scheme || parsed.Host != base.Host {
		return nil, "federations_import_download_failed", fmt.Errorf("unexpected host")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, "federations_import_download_failed", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Errorf("[Federations] download file: %v", err)
		return nil, "federations_import_download_failed", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, "federations_import_download_failed", fmt.Errorf("status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("[Federations] read file body: %v", err)
		return nil, "federations_import_download_failed", err
	}

	if int64(len(data)) > maxBytes {
		return nil, "federations_import_file_too_large", fmt.Errorf("file too large after download")
	}

	return data, "", nil
}
