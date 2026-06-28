package modules

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/federations"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/extraction"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	"github.com/divkix/Alita_Robot/alita/utils/ratelimit"
)

var (
	federationsModule = moduleStruct{moduleName: "Federations"}

	// federationEnforceGroup is the watcher group for passive ban enforcement.
	federationEnforceGroup = 8

	// pendingDeleteFed tracks users who must send /delfed again to confirm.
	pendingDeleteFed   = make(map[int64]time.Time)
	pendingDeleteFedMu sync.Mutex
)

// requirePrivate returns a check that ensures the command is used in PM.
func requirePrivate() helpers.CheckFunc {
	return func(c *helpers.CommandContext) bool {
		if c.Chat == nil || c.Chat.Type != "private" {
			text, _ := c.Tr.GetString("federations_pm_only")
			_, err := c.Msg.Reply(c.Bot, text, formatting.Shtml())
			if err != nil {
				log.Error(err)
			}
			return false
		}
		return true
	}
}

// requireGroup returns a check that ensures the command is used in a group.
func requireGroup() helpers.CheckFunc {
	return func(c *helpers.CommandContext) bool {
		if c.Chat == nil || (c.Chat.Type != "group" && c.Chat.Type != "supergroup") {
			text, _ := c.Tr.GetString("chat_status_group_only_error")
			_, err := c.Msg.Reply(c.Bot, text, formatting.Shtml())
			if err != nil {
				log.Error(err)
			}
			return false
		}
		return true
	}
}

// sendSimpleReply sends a localized reply and returns ext.EndGroups.
func sendSimpleReply(c *helpers.CommandContext, key string, args ...any) error {
	base, _ := c.Tr.GetString(key)
	if base == "" {
		base = key
	}
	if len(args) > 0 {
		base = fmt.Sprintf(base, args...)
	}
	_, err := c.Msg.Reply(c.Bot, base, formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// resolveFederation determines the federation to operate on.
// In PM with no args it returns the user's own federation.
// In a group with no args it returns the chat's federation.
// With an arg it looks up the provided fed_id.
func resolveFederation(c *helpers.CommandContext, args []string) (*models.Federation, error) {
	if len(args) > 0 {
		fed, err := federations.GetFederationByIDCached(args[0])
		if err != nil {
			if err == federations.ErrFederationNotFound {
				_ = sendSimpleReply(c, "federations_fed_not_found")
				return nil, err
			}
			_ = sendSimpleReply(c, "common_db_error")
			return nil, err
		}
		return fed, nil
	}

	if c.Chat != nil && c.Chat.Type == "private" {
		fed, err := federations.GetFederationByOwnerCached(c.User.Id)
		if err != nil {
			if err == federations.ErrFederationNotFound {
				_ = sendSimpleReply(c, "federations_fed_not_found")
				return nil, err
			}
			_ = sendSimpleReply(c, "common_db_error")
			return nil, err
		}
		return fed, nil
	}

	if c.Chat == nil {
		_ = sendSimpleReply(c, "federations_fed_not_found")
		return nil, federations.ErrFederationNotFound
	}

	fed, err := federations.GetChatFederationCached(c.Chat.Id)
	if err != nil {
		if err == federations.ErrFederationNotFound {
			_ = sendSimpleReply(c, "federations_chatfed_none")
			return nil, err
		}
		_ = sendSimpleReply(c, "common_db_error")
		return nil, err
	}
	return fed, nil
}

// newFed handles /newfed to create a federation.
func (m moduleStruct) newFed(b *gotgbot.Bot, ctx *ext.Context) error {
	c, err := helpers.BuildCommandContext(b, ctx)
	if err != nil {
		return ext.EndGroups
	}
	if c.Chat == nil || c.Chat.Type != "private" {
		_ = sendSimpleReply(c, "federations_newfed_pm_only")
		return ext.EndGroups
	}

	args := strings.TrimSpace(strings.Join(ctx.Args()[1:], " "))
	if args == "" {
		_ = sendSimpleReply(c, "federations_need_fed_id")
		return ext.EndGroups
	}

	fed, err := federations.CreateFederation(c.User.Id, args)
	if err != nil {
		if err == federations.ErrAlreadyFederationOwner {
			_ = sendSimpleReply(c, "federations_newfed_already_exists")
		} else if strings.Contains(err.Error(), "name cannot be empty") {
			_ = sendSimpleReply(c, "federations_need_fed_id")
		} else {
			_ = sendSimpleReply(c, "common_db_error")
		}
		return ext.EndGroups
	}

	_ = sendSimpleReply(c, "federations_newfed_created", fed.Name, fed.FedID)
	return ext.EndGroups
}

// renameFed handles /renamefed.
func (m moduleStruct) renameFed(c *helpers.CommandContext) error {
	args := strings.TrimSpace(strings.Join(c.Ctx.Args()[1:], " "))
	if args == "" {
		_ = sendSimpleReply(c, "federations_need_fed_id")
		return ext.EndGroups
	}

	fed, err := federations.GetFederationByOwnerCached(c.User.Id)
	if err != nil {
		if err == federations.ErrFederationNotFound {
			_ = sendSimpleReply(c, "federations_fed_not_found")
		} else {
			_ = sendSimpleReply(c, "common_db_error")
		}
		return ext.EndGroups
	}

	if err := federations.RenameFederation(fed.FedID, args); err != nil {
		if strings.Contains(err.Error(), "name cannot be empty") {
			_ = sendSimpleReply(c, "federations_need_fed_id")
		} else {
			_ = sendSimpleReply(c, "common_db_error")
		}
		return ext.EndGroups
	}

	_ = sendSimpleReply(c, "federations_renamefed_success", args)
	return ext.EndGroups
}

// delFed handles /delfed with double-confirm.
func (m moduleStruct) delFed(c *helpers.CommandContext) error {
	fed, err := federations.GetFederationByOwnerCached(c.User.Id)
	if err != nil {
		if err == federations.ErrFederationNotFound {
			_ = sendSimpleReply(c, "federations_fed_not_found")
		} else {
			_ = sendSimpleReply(c, "common_db_error")
		}
		return ext.EndGroups
	}

	pendingDeleteFedMu.Lock()
	last, pending := pendingDeleteFed[c.User.Id]
	pendingDeleteFedMu.Unlock()

	if !pending || time.Since(last) > 1*time.Minute {
		pendingDeleteFedMu.Lock()
		pendingDeleteFed[c.User.Id] = time.Now()
		pendingDeleteFedMu.Unlock()
		_ = sendSimpleReply(c, "federations_delfed_confirm")
		return ext.EndGroups
	}

	if err := federations.DeleteFederation(fed.FedID); err != nil {
		_ = sendSimpleReply(c, "common_db_error")
		return ext.EndGroups
	}

	pendingDeleteFedMu.Lock()
	delete(pendingDeleteFed, c.User.Id)
	pendingDeleteFedMu.Unlock()

	_ = sendSimpleReply(c, "federations_delfed_done", fed.Name)
	return ext.EndGroups
}

// fedInfo handles /fedinfo.
func (m moduleStruct) fedInfo(c *helpers.CommandContext) error {
	args := c.Ctx.Args()[1:]
	fed, err := resolveFederation(c, args)
	if err != nil {
		return ext.EndGroups
	}

	chats, _ := federations.ListFederationChatsCached(fed.FedID)
	admins, _ := federations.ListAdminsCached(fed.FedID)
	banCount, _ := federations.CountBans(fed.FedID)

	ownerName := fmt.Sprintf("%d", fed.OwnerID)
	ownerChat, ownerErr := c.Bot.GetChat(fed.OwnerID, nil)
	if ownerErr == nil {
		ownerName = formatting.MentionHtml(ownerChat.Id, ownerChat.FirstName)
	}

	_ = sendSimpleReply(c, "federations_fedinfo_header", fed.Name, ownerName, fed.FedID, len(chats), len(admins), banCount)
	return ext.EndGroups
}

// fedAdmins handles /fedadmins.
func (m moduleStruct) fedAdmins(c *helpers.CommandContext) error {
	args := c.Ctx.Args()[1:]
	fed, err := resolveFederation(c, args)
	if err != nil {
		return ext.EndGroups
	}

	admins, err := federations.ListAdminsCached(fed.FedID)
	if err != nil {
		_ = sendSimpleReply(c, "common_db_error")
		return ext.EndGroups
	}

	if len(admins) == 0 {
		_ = sendSimpleReply(c, "federations_fedadmins_none")
		return ext.EndGroups
	}

	var sb strings.Builder
	header, _ := c.Tr.GetString("federations_fedadmins_header")
	fmt.Fprintf(&sb, header, fed.Name)
	for _, admin := range admins {
		name := fmt.Sprintf("%d", admin.UserID)
		chat, adminErr := c.Bot.GetChat(admin.UserID, nil)
		if adminErr == nil {
			name = formatting.MentionHtml(chat.Id, chat.FirstName)
		}
		sb.WriteString("\n - ")
		sb.WriteString(name)
	}

	_, err = c.Msg.Reply(c.Bot, sb.String(), formatting.Shtml())
	if err != nil {
		log.Error(err)
	}
	return ext.EndGroups
}

// chatFed handles /chatfed.
func (m moduleStruct) chatFed(c *helpers.CommandContext) error {
	if c.Chat == nil || (c.Chat.Type != "group" && c.Chat.Type != "supergroup") {
		_ = sendSimpleReply(c, "chat_status_group_only_error")
		return ext.EndGroups
	}

	fed, err := federations.GetChatFederationCached(c.Chat.Id)
	if err != nil {
		if err == federations.ErrFederationNotFound {
			_ = sendSimpleReply(c, "federations_chatfed_none")
		} else {
			_ = sendSimpleReply(c, "common_db_error")
		}
		return ext.EndGroups
	}

	_ = sendSimpleReply(c, "federations_chatfed_info", fed.Name, fed.FedID)
	return ext.EndGroups
}

// myFeds handles /myfeds.
func (m moduleStruct) myFeds(c *helpers.CommandContext) error {
	feds, err := federations.GetFederationsByAdmin(c.User.Id)
	if err != nil {
		_ = sendSimpleReply(c, "common_db_error")
		return ext.EndGroups
	}

	if len(feds) == 0 {
		_ = sendSimpleReply(c, "federations_myfeds_none")
		return ext.EndGroups
	}

	var sb strings.Builder
	header, _ := c.Tr.GetString("federations_myfeds_header")
	sb.WriteString(header)
	for _, fed := range feds {
		if fed.OwnerID == c.User.Id {
			fmt.Fprintf(&sb, "\n - %s (Owner) — %s", fed.Name, fed.FedID)
		} else {
			fmt.Fprintf(&sb, "\n - %s — %s", fed.Name, fed.FedID)
		}
	}

	_, err = c.Msg.Reply(c.Bot, sb.String(), formatting.Shtml())
	if err != nil {
		log.Error(err)
	}
	return ext.EndGroups
}

// joinFed handles /joinfed.
func (m moduleStruct) joinFed(c *helpers.CommandContext) error {
	args := c.Ctx.Args()[1:]
	if len(args) == 0 {
		_ = sendSimpleReply(c, "federations_need_fed_id")
		return ext.EndGroups
	}

	fedID := args[0]
	fed, err := federations.GetFederationByIDCached(fedID)
	if err != nil {
		if err == federations.ErrFederationNotFound {
			_ = sendSimpleReply(c, "federations_fed_not_found")
		} else {
			_ = sendSimpleReply(c, "common_db_error")
		}
		return ext.EndGroups
	}

	if err := federations.JoinChat(fed.FedID, c.Chat.Id, c.User.Id); err != nil {
		if err == federations.ErrChatAlreadyInFederation {
			_ = sendSimpleReply(c, "federations_joinfed_already_in_fed")
		} else {
			_ = sendSimpleReply(c, "common_db_error")
		}
		return ext.EndGroups
	}

	_ = sendSimpleReply(c, "federations_joinfed_success", fed.Name)
	return ext.EndGroups
}

// leaveFed handles /leavefed.
func (m moduleStruct) leaveFed(c *helpers.CommandContext) error {
	fed, err := federations.GetChatFederationCached(c.Chat.Id)
	if err != nil {
		if err == federations.ErrFederationNotFound {
			_ = sendSimpleReply(c, "federations_leavefed_not_in_fed")
		} else {
			_ = sendSimpleReply(c, "common_db_error")
		}
		return ext.EndGroups
	}

	if err := federations.LeaveChat(c.Chat.Id); err != nil {
		_ = sendSimpleReply(c, "common_db_error")
		return ext.EndGroups
	}

	_ = sendSimpleReply(c, "federations_leavefed_success", fed.Name)
	return ext.EndGroups
}

// quietFed handles /quietfed on|off for the current group.
func (m moduleStruct) quietFed(c *helpers.CommandContext) error {
	if c.Chat == nil || (c.Chat.Type != "group" && c.Chat.Type != "supergroup") {
		_ = sendSimpleReply(c, "chat_status_group_only_error")
		return ext.EndGroups
	}

	args := c.Ctx.Args()[1:]
	if len(args) == 0 {
		_ = sendSimpleReply(c, "federations_invalid_on_off")
		return ext.EndGroups
	}

	enabled, ok := parseOnOff(args[0])
	if !ok {
		_ = sendSimpleReply(c, "federations_invalid_on_off")
		return ext.EndGroups
	}

	if _, err := federations.GetChatFederationCached(c.Chat.Id); err != nil {
		if err == federations.ErrFederationNotFound {
			_ = sendSimpleReply(c, "federations_chatfed_none")
		} else {
			_ = sendSimpleReply(c, "common_db_error")
		}
		return ext.EndGroups
	}

	if err := federations.SetQuiet(c.Chat.Id, enabled); err != nil {
		_ = sendSimpleReply(c, "common_db_error")
		return ext.EndGroups
	}

	if enabled {
		_ = sendSimpleReply(c, "federations_quietfed_on")
	} else {
		_ = sendSimpleReply(c, "federations_quietfed_off")
	}
	return ext.EndGroups
}

// resolveFederationForBan resolves the federation and verifies the user is an admin.
func resolveFederationForBan(c *helpers.CommandContext) (*models.Federation, error) {
	var fed *models.Federation
	var err error

	if c.Chat != nil && (c.Chat.Type == "group" || c.Chat.Type == "supergroup") {
		fed, err = federations.GetChatFederationCached(c.Chat.Id)
	} else {
		fed, err = federations.GetFederationByOwnerCached(c.User.Id)
	}

	if err != nil {
		if err == federations.ErrFederationNotFound {
			_ = sendSimpleReply(c, "federations_fed_not_found")
		} else {
			_ = sendSimpleReply(c, "common_db_error")
		}
		return nil, err
	}

	isAdmin, err := federations.IsAdminCached(fed.FedID, c.User.Id)
	if err != nil {
		log.Errorf("[Federations] resolveFederationForBan: IsAdminCached(%s, %d) error: %v", fed.FedID, c.User.Id, err)
		_ = sendSimpleReply(c, "common_db_error")
		return nil, err
	}
	if !isAdmin && fed.OwnerID != c.User.Id {
		log.Debugf("[Federations] resolveFederationForBan: user %d is not admin (fed=%s, owner=%d)", c.User.Id, fed.FedID, fed.OwnerID)
		_ = sendSimpleReply(c, "federations_fban_not_admin")
		return nil, fmt.Errorf("not admin")
	}

	return fed, nil
}

// extractFederationTarget extracts the target user and remaining text from the command.
func extractFederationTarget(c *helpers.CommandContext) (int64, string, bool) {
	args := c.Ctx.Args()

	// If the command is a reply with no extra args, use the replied user.
	if c.Msg.ReplyToMessage != nil && len(args) == 1 {
		sender := c.Msg.ReplyToMessage.GetSender()
		if sender != nil {
			return sender.Id(), "", true
		}
	}

	// Try standard extraction.
	uid, text := extraction.ExtractUserAndText(c.Bot, c.Ctx)
	if uid == -1 {
		return 0, "", false
	}
	if uid == 0 {
		_ = sendSimpleReply(c, "federations_fban_no_user_specified")
		return 0, "", false
	}

	return uid, text, true
}

// banInChat attempts to ban a user in a single chat when the bot has the
// right to restrict members. It is used by both /fban and active enforcement.
// Returns nil on success (including expected USER_NOT_PARTICIPANT in supergroups,
// where the ban still blocks re-entry). Returns a descriptive error on actual failure.
func banInChat(b *gotgbot.Bot, chatID int64, userID int64) error {
	if b == nil || chatID == 0 || userID <= 0 {
		return fmt.Errorf("invalid parameters: bot=%v chatID=%d userID=%d", b != nil, chatID, userID)
	}

	chat, err := b.GetChat(chatID, nil)
	if err != nil {
		log.Errorf("[Federations] banInChat: failed to get chat %d: %v", chatID, err)
		return fmt.Errorf("GetChat(%d) failed: %w", chatID, err)
	}
	chatInfo := chat.ToChat()

	if !chat_status.CanBotRestrict(b, nil, &chatInfo) {
		log.Errorf("[Federations] banInChat: bot cannot restrict in chat %d (missing can_restrict_members admin right)", chatID)
		return fmt.Errorf("bot cannot restrict in chat %d: missing can_restrict_members right", chatID)
	}

	_, err = b.BanChatMember(chatID, userID, nil)
	if err != nil {
		errDesc := err.Error()
		// USER_NOT_PARTICIPANT is expected in supergroups — the ban still applies
		// to block re-entry via invite links. This is not a failure.
		if chat.Type == "supergroup" && strings.Contains(errDesc, "USER_NOT_PARTICIPANT") {
			log.Infof("[Federations] banInChat: user %d not participant in supergroup %d, ban applied (blocks re-entry)", userID, chatID)
			return nil
		}
		log.Errorf("[Federations] banInChat: failed to ban user %d in chat %d: %v", userID, chatID, err)
		return fmt.Errorf("BanChatMember(%d, %d) failed: %w", chatID, userID, err)
	}

	log.Infof("[Federations] banInChat: successfully banned user %d in chat %d", userID, chatID)
	return nil
}

// fban handles /fban.
func (m moduleStruct) fban(c *helpers.CommandContext) error {
	fed, err := resolveFederationForBan(c)
	if err != nil {
		return ext.EndGroups
	}

	userID, reason, ok := extractFederationTarget(c)
	if !ok {
		return ext.EndGroups
	}

	if userID == c.Bot.Id {
		_ = sendSimpleReply(c, "federations_fban_is_bot_itself")
		return ext.EndGroups
	}

	// Check require_reason setting.
	settings, err := federations.GetSettings(fed.FedID)
	if err == nil && settings.RequireReason && strings.TrimSpace(reason) == "" {
		_ = sendSimpleReply(c, "federations_fban_require_reason")
		return ext.EndGroups
	}

	// Prevent banning federation admins.
	targetIsAdmin, err := federations.IsAdminCached(fed.FedID, userID)
	if err == nil && targetIsAdmin {
		_ = sendSimpleReply(c, "federations_cannot_target_admin")
		return ext.EndGroups
	}

	if err := federations.BanUser(fed.FedID, userID, reason, c.User.Id); err != nil {
		_ = sendSimpleReply(c, "common_db_error")
		return ext.EndGroups
	}

	// If the command was issued in a group that belongs to the federation,
	// ban the target there immediately. This avoids the race with the async
	// chat-user tracker and guarantees the issuing chat is enforced.
	if c.Chat != nil && (c.Chat.Type == "group" || c.Chat.Type == "supergroup") {
		chatFed, err := federations.GetChatFederationCached(c.Chat.Id)
		if err == nil && chatFed != nil && chatFed.ID == fed.ID {
			log.Infof("[Federations] /fban: immediately banning user %d in issuing chat %d", userID, c.Chat.Id)
			if banErr := banInChat(c.Bot, c.Chat.Id, userID); banErr != nil {
				log.Errorf("[Federations] /fban: immediate ban failed for user %d in chat %d: %v", userID, c.Chat.Id, banErr)
			}
		}
	}

	// Active enforcement in other federation chats where the user has been seen.
	go federations.ActiveBanInChats(c.Bot, fed.FedID, userID)

	targetName := fmt.Sprintf("%d", userID)
	chat, chatErr := c.Bot.GetChat(userID, nil)
	if chatErr == nil {
		targetName = formatting.MentionHtml(chat.Id, chat.FirstName)
	}

	_ = sendSimpleReply(c, "federations_fban_done", targetName, fed.Name, reason)
	notifyFederationAction(c, fed, actionBanned, userID, reason)
	return ext.EndGroups
}

// unfban handles /unfban — unbans the user from all federation groups via
// Telegram API and removes the DB ban record so passive enforcement stops.
func (m moduleStruct) unfban(c *helpers.CommandContext) error {
	fed, err := resolveFederationForBan(c)
	if err != nil {
		return ext.EndGroups
	}

	userID, _, ok := extractFederationTarget(c)
	if !ok {
		return ext.EndGroups
	}

	if userID == c.Bot.Id {
		_ = sendSimpleReply(c, "federations_fban_is_bot_itself")
		return ext.EndGroups
	}

	isBanned, _, err := federations.IsBanned(fed.FedID, userID)
	if err != nil {
		_ = sendSimpleReply(c, "common_db_error")
		return ext.EndGroups
	}
	if !isBanned {
		_ = sendSimpleReply(c, "federations_fban_user_not_banned")
		return ext.EndGroups
	}

	// Remove the DB ban record so passive enforcement stops.
	if err := federations.UnbanUser(fed.FedID, userID); err != nil {
		_ = sendSimpleReply(c, "common_db_error")
		return ext.EndGroups
	}

	// Unban in all federation chats via Telegram API (active unban).
	go unfbanInAllChats(c.Bot, fed.FedID, userID)

	targetName := fmt.Sprintf("%d", userID)
	chat, chatErr := c.Bot.GetChat(userID, nil)
	if chatErr == nil {
		targetName = formatting.MentionHtml(chat.Id, chat.FirstName)
	}

	_ = sendSimpleReply(c, "federations_unfban_done", targetName, fed.Name)
	notifyFederationAction(c, fed, actionUnbanned, userID, "")
	return ext.EndGroups
}

// unfbanInAllChats unbans a user from every chat in the federation via Telegram API.
func unfbanInAllChats(b *gotgbot.Bot, fedID string, userID int64) {
	chatIDs, err := federations.ListFederationChatsCached(fedID)
	if err != nil {
		log.Errorf("[Federations] unfbanInAllChats: list chats for fed %s: %v", fedID, err)
		return
	}

	for _, chatID := range chatIDs {
		chatFull, err := b.GetChat(chatID, nil)
		if err != nil {
			log.Errorf("[Federations] unfbanInAllChats: GetChat(%d) failed: %v", chatID, err)
			continue
		}
		chatInfo := chatFull.ToChat()

		if !chat_status.CanBotRestrict(b, nil, &chatInfo) {
			log.Warnf("[Federations] unfbanInAllChats: bot cannot restrict in chat %d, skipping", chatID)
			continue
		}

		_, err = b.UnbanChatMember(chatID, userID, nil)
		if err != nil {
			// USER_NOT_PARTICIPANT is expected in supergroups — not a failure.
			if chatFull.Type == "supergroup" && strings.Contains(err.Error(), "USER_NOT_PARTICIPANT") {
				log.Infof("[Federations] unfbanInAllChats: user %d not participant in supergroup %d, unban applied", userID, chatID)
				continue
			}
			log.Errorf("[Federations] unfbanInAllChats: UnbanChatMember(%d, %d) failed: %v", chatID, userID, err)
			continue
		}

		log.Infof("[Federations] unfbanInAllChats: unbanned user %d in chat %d", userID, chatID)
	}
}

// fedStat handles /fedstat.
func (m moduleStruct) fedStat(c *helpers.CommandContext) error {
	args := c.Ctx.Args()[1:]

	if len(args) >= 2 {
		var userID int64
		if c.Msg.ReplyToMessage != nil {
			sender := c.Msg.ReplyToMessage.GetSender()
			if sender != nil {
				userID = sender.Id()
			}
		}
		if userID == 0 {
			if uid, ok := resolveUserIDFromArg(args[0]); ok {
				userID = uid
			} else {
				uid, _ := extraction.ExtractUserAndText(c.Bot, c.Ctx)
				if uid > 0 {
					userID = uid
				}
			}
		}
		return sendFederationBanReason(c, args[1], userID)
	}

	var userID int64
	if len(args) > 0 {
		if uid, ok := resolveUserIDFromArg(args[0]); ok {
			userID = uid
		} else {
			uid, _ := extraction.ExtractUserAndText(c.Bot, c.Ctx)
			if uid > 0 {
				userID = uid
			} else {
				userID = c.User.Id
			}
		}
	} else if c.Msg.ReplyToMessage != nil {
		sender := c.Msg.ReplyToMessage.GetSender()
		if sender != nil {
			userID = sender.Id()
		}
	} else {
		userID = c.User.Id
	}

	bans, err := federations.GetUserFederationBans(userID)
	if err != nil {
		_ = sendSimpleReply(c, "common_db_error")
		return ext.EndGroups
	}

	targetName := fmt.Sprintf("%d", userID)
	chat, chatErr := c.Bot.GetChat(userID, nil)
	if chatErr == nil {
		targetName = formatting.MentionHtml(chat.Id, chat.FirstName)
	}

	if len(bans) == 0 {
		_ = sendSimpleReply(c, "federations_fedstat_none", targetName)
		return ext.EndGroups
	}

	var sb strings.Builder
	header, _ := c.Tr.GetString("federations_fedstat_header")
	fmt.Fprintf(&sb, header, targetName)
	for _, ban := range bans {
		fmt.Fprintf(&sb, "\n - %s (%s)", ban.FedName, ban.FedID)
	}

	_, err = c.Msg.Reply(c.Bot, sb.String(), formatting.Shtml())
	if err != nil {
		log.Error(err)
	}
	return ext.EndGroups
}

// fbanStat handles /fbanstat.
func (m moduleStruct) fbanStat(c *helpers.CommandContext) error {
	args := c.Ctx.Args()[1:]
	if len(args) == 0 {
		_ = sendSimpleReply(c, "federations_need_fed_id")
		return ext.EndGroups
	}

	fedID := args[0]
	var userID int64
	if c.Msg.ReplyToMessage != nil {
		sender := c.Msg.ReplyToMessage.GetSender()
		if sender != nil {
			userID = sender.Id()
		}
	} else if len(args) > 1 {
		if uid, ok := resolveUserIDFromArg(args[1]); ok {
			userID = uid
		} else {
			uid, _ := extraction.ExtractUserAndText(c.Bot, c.Ctx)
			if uid > 0 {
				userID = uid
			}
		}
	} else {
		userID = c.User.Id
	}

	return sendFederationBanReason(c, fedID, userID)
}

// fedPromote handles /fedpromote.
func (m moduleStruct) fedPromote(c *helpers.CommandContext) error {
	fed, err := federations.GetFederationByOwnerCached(c.User.Id)
	if err != nil {
		if err == federations.ErrFederationNotFound {
			_ = sendSimpleReply(c, "federations_fed_not_found")
		} else {
			_ = sendSimpleReply(c, "common_db_error")
		}
		return ext.EndGroups
	}

	targetID, ok := resolveTargetUser(c)
	if !ok {
		return ext.EndGroups
	}

	if targetID == c.Bot.Id {
		_ = sendSimpleReply(c, "federations_fedpromote_is_bot")
		return ext.EndGroups
	}
	if targetID == fed.OwnerID {
		_ = sendSimpleReply(c, "federations_fedpromote_is_owner")
		return ext.EndGroups
	}

	isAdmin, err := federations.IsAdminCached(fed.FedID, targetID)
	if err == nil && isAdmin {
		_ = sendSimpleReply(c, "federations_fedpromote_already_admin")
		return ext.EndGroups
	}

	if err := federations.AddAdmin(fed.FedID, targetID, c.User.Id); err != nil {
		_ = sendSimpleReply(c, "common_db_error")
		return ext.EndGroups
	}

	targetName := getDisplayName(c.Bot, targetID)
	_ = sendSimpleReply(c, "federations_fedpromote_success", targetName)
	notifyFederationAction(c, fed, actionPromoted, targetID, "")
	return ext.EndGroups
}

// fedDemote handles /feddemote.
func (m moduleStruct) fedDemote(c *helpers.CommandContext) error {
	fed, err := federations.GetFederationByOwnerCached(c.User.Id)
	if err != nil {
		if err == federations.ErrFederationNotFound {
			_ = sendSimpleReply(c, "federations_fed_not_found")
		} else {
			_ = sendSimpleReply(c, "common_db_error")
		}
		return ext.EndGroups
	}

	targetID, ok := resolveTargetUser(c)
	if !ok {
		return ext.EndGroups
	}

	if targetID == fed.OwnerID {
		_ = sendSimpleReply(c, "federations_feddemote_is_owner")
		return ext.EndGroups
	}

	isAdmin, err := federations.IsAdminCached(fed.FedID, targetID)
	if err != nil {
		_ = sendSimpleReply(c, "common_db_error")
		return ext.EndGroups
	}
	if !isAdmin {
		_ = sendSimpleReply(c, "federations_feddemote_not_admin")
		return ext.EndGroups
	}

	if err := federations.RemoveAdmin(fed.FedID, targetID); err != nil {
		_ = sendSimpleReply(c, "common_db_error")
		return ext.EndGroups
	}

	targetName := getDisplayName(c.Bot, targetID)
	_ = sendSimpleReply(c, "federations_feddemote_success", targetName)
	notifyFederationAction(c, fed, actionDemoted, targetID, "")
	return ext.EndGroups
}

// fedDemoteMe handles /feddemoteme.
func (m moduleStruct) fedDemoteMe(c *helpers.CommandContext) error {
	args := c.Ctx.Args()[1:]
	if len(args) == 0 {
		_ = sendSimpleReply(c, "federations_need_fed_id")
		return ext.EndGroups
	}

	fed, err := federations.GetFederationByIDCached(args[0])
	if err != nil {
		if err == federations.ErrFederationNotFound {
			_ = sendSimpleReply(c, "federations_fed_not_found")
		} else {
			log.Errorf("[Federations] /feddemoteme GetFederationByIDCached: %v", err)
			_ = sendSimpleReply(c, "common_db_error")
		}
		return ext.EndGroups
	}

	if fed.OwnerID == c.User.Id {
		_ = sendSimpleReply(c, "federations_feddemoteme_owner_cannot")
		return ext.EndGroups
	}

	isAdmin, err := federations.IsAdminCached(fed.FedID, c.User.Id)
	if err != nil {
		log.Errorf("[Federations] /feddemoteme IsAdminCached: %v", err)
		_ = sendSimpleReply(c, "common_db_error")
		return ext.EndGroups
	}
	if !isAdmin {
		_ = sendSimpleReply(c, "federations_feddemoteme_not_admin")
		return ext.EndGroups
	}

	if err := federations.RemoveAdmin(fed.FedID, c.User.Id); err != nil {
		if err == federations.ErrFederationNotFound {
			_ = sendSimpleReply(c, "federations_feddemoteme_not_admin")
		} else {
			log.Errorf("[Federations] /feddemoteme RemoveAdmin: %v", err)
			_ = sendSimpleReply(c, "common_db_error")
		}
		return ext.EndGroups
	}

	_ = sendSimpleReply(c, "federations_feddemoteme_success", fed.Name)
	notifyFederationAction(c, fed, actionSelfDemoted, 0, "")
	return ext.EndGroups
}

// federationBoolSetter updates a boolean federation setting.
type federationBoolSetter func(fedID string, enabled bool) error

// setFederationToggle handles /fedreason and /fednotif toggles.
func (m moduleStruct) setFederationToggle(
	c *helpers.CommandContext,
	setter federationBoolSetter,
	onKey, offKey, detailFormat string,
) error {
	fed, err := federations.GetFederationByOwnerCached(c.User.Id)
	if err != nil {
		if err == federations.ErrFederationNotFound {
			_ = sendSimpleReply(c, "federations_fed_not_found")
		} else {
			_ = sendSimpleReply(c, "common_db_error")
		}
		return ext.EndGroups
	}

	args := c.Ctx.Args()[1:]
	if len(args) == 0 {
		_ = sendSimpleReply(c, "federations_invalid_on_off")
		return ext.EndGroups
	}

	enabled, ok := parseOnOff(args[0])
	if !ok {
		_ = sendSimpleReply(c, "federations_invalid_on_off")
		return ext.EndGroups
	}

	if err := setter(fed.FedID, enabled); err != nil {
		_ = sendSimpleReply(c, "common_db_error")
		return ext.EndGroups
	}

	key := onKey
	if !enabled {
		key = offKey
	}
	_ = sendSimpleReply(c, key)
	notifyFederationAction(c, fed, actionSettingsChanged, 0, fmt.Sprintf(detailFormat, enabled))
	return ext.EndGroups
}

// fedReason handles /fedreason.
func (m moduleStruct) fedReason(c *helpers.CommandContext) error {
	return m.setFederationToggle(
		c,
		federations.SetRequireReason,
		"federations_fedreason_on",
		"federations_fedreason_off",
		"require_reason=%v",
	)
}

// fedNotif handles /fednotif.
func (m moduleStruct) fedNotif(c *helpers.CommandContext) error {
	return m.setFederationToggle(
		c,
		federations.SetNotifications,
		"federations_fednotif_on",
		"federations_fednotif_off",
		"notifications_enabled=%v",
	)
}

// setFedLog handles /setfedlog.
func (m moduleStruct) setFedLog(c *helpers.CommandContext) error {
	if c.Chat == nil {
		_ = sendSimpleReply(c, "federations_setfedlog_group_or_channel")
		return ext.EndGroups
	}

	// In a channel the user must provide the federation ID and confirm ownership.
	if c.Chat.Type == "channel" {
		args := c.Ctx.Args()[1:]
		if len(args) == 0 {
			_ = sendSimpleReply(c, "federations_need_fed_id")
			return ext.EndGroups
		}

		fedID := args[0]
		fed, err := federations.GetFederationByIDCached(fedID)
		if err != nil {
			if err == federations.ErrFederationNotFound {
				_ = sendSimpleReply(c, "federations_fed_not_found")
			} else {
				_ = sendSimpleReply(c, "common_db_error")
			}
			return ext.EndGroups
		}

		confirmText, _ := c.Tr.GetString("federations_setfedlog_confirm")
		buttonText, _ := c.Tr.GetString("federations_setfedlog_button_confirm")
		if buttonText == "" {
			buttonText = "Confirm"
		}

		callbackData := encodeCallbackData("setfedlog", map[string]string{
			"f": fed.FedID,
			"c": fmt.Sprintf("%d", c.Chat.Id),
		})
		if callbackData == "" {
			_ = sendSimpleReply(c, "common_db_error")
			return ext.EndGroups
		}

		keyboard := gotgbot.InlineKeyboardMarkup{
			InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
				{
					{Text: buttonText, CallbackData: callbackData},
				},
			},
		}

		_, err = c.Bot.SendMessage(c.Chat.Id, fmt.Sprintf(confirmText, fed.Name), &gotgbot.SendMessageOpts{
			ParseMode:   formatting.HTML,
			ReplyMarkup: keyboard,
		})
		if err != nil {
			log.Error(err)
		}
		return ext.EndGroups
	}

	// In a group, set the user's own federation log to this chat.
	if c.Chat.Type != "group" && c.Chat.Type != "supergroup" {
		_ = sendSimpleReply(c, "federations_setfedlog_group_or_channel")
		return ext.EndGroups
	}

	fed, err := federations.GetFederationByOwnerCached(c.User.Id)
	if err != nil {
		if err == federations.ErrFederationNotFound {
			_ = sendSimpleReply(c, "federations_fed_not_found")
		} else {
			log.Errorf("[Federations] /setfedlog: %v", err)
			_ = sendSimpleReply(c, "common_db_error")
		}
		return ext.EndGroups
	}

	if err := federations.SetLogChat(fed.FedID, c.Chat.Id, c.Chat.Title); err != nil {
		log.Errorf("[Federations] /setfedlog SetLogChat: %v", err)
		_ = sendSimpleReply(c, "common_db_error")
		return ext.EndGroups
	}

	_ = sendSimpleReply(c, "federations_setfedlog_done", c.Chat.Title)
	notifyFederationAction(c, fed, actionSettingsChanged, 0, fmt.Sprintf("log_chat_id=%d", c.Chat.Id))
	return ext.EndGroups
}

// setFedLogCallback handles the confirmation button for channel federation logs.
func (m moduleStruct) setFedLogCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	query, ok := callbackQueryFromContext(ctx)
	if !ok {
		return ext.EndGroups
	}

	decoded, ok := decodeCallbackData(query.Data, "setfedlog")
	if !ok {
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "Invalid callback"})
		return ext.EndGroups
	}

	fedID, _ := decoded.Field("f")
	chatIDStr, _ := decoded.Field("c")
	chatID, perr := strconv.ParseInt(chatIDStr, 10, 64)
	if perr != nil {
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "Invalid callback"})
		return ext.EndGroups
	}

	fed, err := federations.GetFederationByIDCached(fedID)
	if err != nil {
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "Federation not found"})
		return ext.EndGroups
	}

	if fed.OwnerID != query.From.Id {
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "Only the federation owner can confirm this."})
		return ext.EndGroups
	}

	// Fetch chat name for the FK constraint.
	chatName := ""
	if chat, gErr := b.GetChat(chatID, nil); gErr == nil {
		chatName = chat.Title
	}

	if err := federations.SetLogChat(fed.FedID, chatID, chatName); err != nil {
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "Failed to set log chat."})
		return ext.EndGroups
	}

	_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "Federation log set."})

	if query.Message != nil {
		c := &helpers.CommandContext{Bot: b, Ctx: ctx, User: &query.From}
		notifyFederationAction(c, fed, actionSettingsChanged, 0, fmt.Sprintf("log_chat_id=%d", chatID))
		_, _, _ = query.Message.EditText(b, fmt.Sprintf("Federation log set for <b>%s</b>.", formatting.HtmlEscape(fed.Name)), &gotgbot.EditMessageTextOpts{ParseMode: formatting.HTML})
	}

	return ext.EndGroups
}

// unsetFedLog handles /unsetfedlog.
func (m moduleStruct) unsetFedLog(c *helpers.CommandContext) error {
	if c.Chat == nil || c.Chat.Type != "private" {
		_ = sendSimpleReply(c, "federations_pm_only")
		return ext.EndGroups
	}

	fed, err := federations.GetFederationByOwnerCached(c.User.Id)
	if err != nil {
		if err == federations.ErrFederationNotFound {
			_ = sendSimpleReply(c, "federations_fed_not_found")
		} else {
			log.Errorf("[Federations] /unsetfedlog: %v", err)
			_ = sendSimpleReply(c, "common_db_error")
		}
		return ext.EndGroups
	}

	if err := federations.ClearLogChat(fed.FedID); err != nil {
		log.Errorf("[Federations] /unsetfedlog ClearLogChat: %v", err)
		_ = sendSimpleReply(c, "common_db_error")
		return ext.EndGroups
	}

	_ = sendSimpleReply(c, "federations_unsetfedlog_done")
	notifyFederationAction(c, fed, actionSettingsChanged, 0, "log_chat_id=0")
	return ext.EndGroups
}

// fbanList handles /fbanlist to export a federation's ban list.
func (m moduleStruct) fbanList(c *helpers.CommandContext) error {
	if c.Chat == nil || c.Chat.Type != "private" {
		_ = sendSimpleReply(c, "federations_pm_only")
		return ext.EndGroups
	}

	fed, err := federations.GetFederationByOwnerCached(c.User.Id)
	if err != nil {
		if err == federations.ErrFederationNotFound {
			_ = sendSimpleReply(c, "federations_fed_not_found")
		} else {
			_ = sendSimpleReply(c, "common_db_error")
		}
		return ext.EndGroups
	}

	limiter := ratelimit.GetFederationRateLimiter()
	if ok, remaining := limiter.CanExportFbanList(c.User.Id); !ok {
		_ = sendSimpleReply(c, "federations_fbanlist_rate_limited", remaining.String())
		return ext.EndGroups
	}

	format := federations.FormatCSV
	args := c.Ctx.Args()[1:]
	if len(args) > 0 {
		format = strings.ToLower(strings.TrimSpace(args[0]))
	}

	data, fileExt, err := federations.ExportBans(c.Bot, fed.FedID, format)
	if err != nil {
		log.Errorf("[Federations] ExportBans failed: %v", err)
		_ = sendSimpleReply(c, "common_db_error")
		return ext.EndGroups
	}

	fileName := fmt.Sprintf("%s_bans.%s", fed.FedID, fileExt)
	caption, _ := c.Tr.GetString("federations_fbanlist_caption", i18n.TranslationParams{"name": fed.Name})

	log.Debugf("[Federations] /fbanlist: sending file %s (%d bytes)", fileName, len(data))

	_, sendErr := c.Bot.SendDocument(
		c.Chat.Id,
		gotgbot.InputFileByReader(fileName, bytes.NewReader(data)),
		&gotgbot.SendDocumentOpts{
			Caption:         caption,
			ParseMode:       "HTML",
			ReplyParameters: &gotgbot.ReplyParameters{MessageId: c.Msg.MessageId},
		},
	)
	if sendErr != nil {
		log.Errorf("[Federations] SendDocument failed: %v", sendErr)
		_ = sendSimpleReply(c, "federations_fbanlist_send_failed")
		return ext.EndGroups
	}

	limiter.RecordFbanListExport(c.User.Id)
	notifyFederationAction(c, fed, actionSettingsChanged, 0, fmt.Sprintf("exported %s banlist", format))
	return ext.EndGroups
}

// importFbans handles /importfbans to import bans from a replied document.
func (m moduleStruct) importFbans(c *helpers.CommandContext) error {
	if c.Chat == nil || c.Chat.Type != "private" {
		_ = sendSimpleReply(c, "federations_pm_only")
		return ext.EndGroups
	}

	fed, err := federations.GetFederationByOwnerCached(c.User.Id)
	if err != nil {
		if err == federations.ErrFederationNotFound {
			_ = sendSimpleReply(c, "federations_fed_not_found")
		} else {
			_ = sendSimpleReply(c, "common_db_error")
		}
		return ext.EndGroups
	}

	format := federations.FormatCSV
	args := c.Ctx.Args()[1:]
	if len(args) > 0 {
		format = strings.ToLower(strings.TrimSpace(args[0]))
	}

	data, key, err := downloadRepliedDocument(c, []string{".csv", ".json"}, 10*1024*1024)
	if err != nil {
		_ = sendSimpleReply(c, key)
		return ext.EndGroups
	}

	imported, skipped, err := federations.ImportBans(fed.FedID, data, format)
	if err != nil {
		log.Errorf("[Federations] ImportBans failed: %v", err)
		_ = sendSimpleReply(c, "federations_import_parse_failed")
		return ext.EndGroups
	}

	_ = sendSimpleReply(c, "federations_import_done", imported, skipped)
	notifyFederationAction(c, fed, actionSettingsChanged, 0, fmt.Sprintf("imported %d bans (skipped %d)", imported, skipped))
	return ext.EndGroups
}

// subFed handles /subfed to subscribe one federation to another.
func (m moduleStruct) subFed(c *helpers.CommandContext) error {
	if c.Chat == nil || c.Chat.Type != "private" {
		_ = sendSimpleReply(c, "federations_pm_only")
		return ext.EndGroups
	}

	fed, err := federations.GetFederationByOwnerCached(c.User.Id)
	if err != nil {
		if err == federations.ErrFederationNotFound {
			_ = sendSimpleReply(c, "federations_fed_not_found")
		} else {
			_ = sendSimpleReply(c, "common_db_error")
		}
		return ext.EndGroups
	}

	args := c.Ctx.Args()[1:]
	if len(args) == 0 {
		_ = sendSimpleReply(c, "federations_need_fed_id")
		return ext.EndGroups
	}

	targetFedID := strings.TrimSpace(args[0])
	switch err := federations.SubscribeFederation(fed.FedID, targetFedID); err {
	case nil:
		_ = sendSimpleReply(c, "federations_subfed_success", targetFedID)
		notifyFederationAction(c, fed, actionSettingsChanged, 0, fmt.Sprintf("subscribed to %s", targetFedID))
	case federations.ErrSelfSubscription:
		_ = sendSimpleReply(c, "federations_subfed_self")
	case federations.ErrMaxSubscriptionsReached:
		_ = sendSimpleReply(c, "federations_subfed_max")
	case federations.ErrFederationNotFound:
		_ = sendSimpleReply(c, "federations_fed_not_found")
	default:
		log.Errorf("[Federations] SubscribeFederation failed: %v", err)
		_ = sendSimpleReply(c, "common_db_error")
	}
	return ext.EndGroups
}

// unSubFed handles /unsubfed to remove a federation subscription.
func (m moduleStruct) unSubFed(c *helpers.CommandContext) error {
	if c.Chat == nil || c.Chat.Type != "private" {
		_ = sendSimpleReply(c, "federations_pm_only")
		return ext.EndGroups
	}

	fed, err := federations.GetFederationByOwnerCached(c.User.Id)
	if err != nil {
		if err == federations.ErrFederationNotFound {
			_ = sendSimpleReply(c, "federations_fed_not_found")
		} else {
			_ = sendSimpleReply(c, "common_db_error")
		}
		return ext.EndGroups
	}

	args := c.Ctx.Args()[1:]
	if len(args) == 0 {
		_ = sendSimpleReply(c, "federations_need_fed_id")
		return ext.EndGroups
	}

	targetFedID := strings.TrimSpace(args[0])
	if err := federations.UnsubscribeFederation(fed.FedID, targetFedID); err != nil {
		if err == federations.ErrFederationNotFound {
			_ = sendSimpleReply(c, "federations_fed_not_found")
		} else {
			log.Errorf("[Federations] UnsubscribeFederation failed: %v", err)
			_ = sendSimpleReply(c, "common_db_error")
		}
		return ext.EndGroups
	}

	_ = sendSimpleReply(c, "federations_unsubfed_success", targetFedID)
	notifyFederationAction(c, fed, actionSettingsChanged, 0, fmt.Sprintf("unsubscribed from %s", targetFedID))
	return ext.EndGroups
}

// fedSubs handles /fedsubs to list subscriptions.
func (m moduleStruct) fedSubs(c *helpers.CommandContext) error {
	fed, err := resolveFederation(c, c.Ctx.Args()[1:])
	if err != nil {
		return ext.EndGroups
	}

	isAdmin, err := federations.IsAdminCached(fed.FedID, c.User.Id)
	if err != nil || (!isAdmin && fed.OwnerID != c.User.Id) {
		_ = sendSimpleReply(c, "federations_admin_only")
		return ext.EndGroups
	}

	subs, err := federations.ListSubscriptionsCached(fed.FedID)
	if err != nil {
		_ = sendSimpleReply(c, "common_db_error")
		return ext.EndGroups
	}

	if len(subs) == 0 {
		_ = sendSimpleReply(c, "federations_fedsubs_none")
		return ext.EndGroups
	}

	var sb strings.Builder
	header, _ := c.Tr.GetString("federations_fedsubs_header", i18n.TranslationParams{"name": fed.Name})
	sb.WriteString(header)
	for _, sub := range subs {
		target, err := federations.GetFederationByIDFromDB(sub.SubscribedToFederationID)
		if err != nil {
			continue
		}
		fmt.Fprintf(&sb, "\n - <b>%s</b> (<code>%s</code>)", target.Name, target.FedID)
	}

	_, err = c.Msg.Reply(c.Bot, sb.String(), formatting.Shtml())
	if err != nil {
		log.Error(err)
	}
	return ext.EndGroups
}

// LoadFederations registers all federation module handlers.
func LoadFederations(dispatcher *ext.Dispatcher) {
	DefaultHelpRegistry().AbleMap.Store(federationsModule.moduleName, true)

	dispatcher.AddHandler(handlers.NewCommand("newfed", federationsModule.newFed))
	helpers.AddCmdToDisableable("newfed")

	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:           "renamefed",
		RequiredChecks: []helpers.CheckFunc{requirePrivate()},
		Disableable:    true,
	}, federationsModule.renameFed)

	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:           "delfed",
		RequiredChecks: []helpers.CheckFunc{requirePrivate()},
		Disableable:    true,
	}, federationsModule.delFed)

	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:        "fedinfo",
		Disableable: true,
	}, federationsModule.fedInfo)

	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:        "fedadmins",
		Disableable: true,
	}, federationsModule.fedAdmins)

	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:           "chatfed",
		RequiredChecks: []helpers.CheckFunc{requireGroup()},
		Disableable:    true,
	}, federationsModule.chatFed)

	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:           "myfeds",
		RequiredChecks: []helpers.CheckFunc{requirePrivate()},
		Disableable:    true,
	}, federationsModule.myFeds)

	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:           "joinfed",
		RequiredChecks: []helpers.CheckFunc{requireGroup(), helpers.RequireUserOwner()},
		Disableable:    true,
	}, federationsModule.joinFed)

	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:           "leavefed",
		RequiredChecks: []helpers.CheckFunc{requireGroup(), helpers.RequireUserOwner()},
		Disableable:    true,
	}, federationsModule.leaveFed)

	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:           "quietfed",
		RequiredChecks: []helpers.CheckFunc{requireGroup()},
		Disableable:    true,
	}, federationsModule.quietFed)

	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:        "fban",
		Disableable: true,
	}, federationsModule.fban)

	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:        "unfban",
		Disableable: true,
	}, federationsModule.unfban)

	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:        "fedstat",
		Disableable: true,
	}, federationsModule.fedStat)

	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:        "fbanstat",
		Disableable: true,
	}, federationsModule.fbanStat)

	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:        "fedpromote",
		Disableable: true,
	}, federationsModule.fedPromote)

	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:        "feddemote",
		Disableable: true,
	}, federationsModule.fedDemote)

	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:        "feddemoteme",
		Disableable: true,
	}, federationsModule.fedDemoteMe)

	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:        "fedreason",
		Disableable: true,
	}, federationsModule.fedReason)

	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:        "fednotif",
		Disableable: true,
	}, federationsModule.fedNotif)

	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:        "setfedlog",
		Disableable: true,
	}, federationsModule.setFedLog)

	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:           "unsetfedlog",
		RequiredChecks: []helpers.CheckFunc{requirePrivate()},
		Disableable:    true,
	}, federationsModule.unsetFedLog)

	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:           "fbanlist",
		RequiredChecks: []helpers.CheckFunc{requirePrivate()},
		Disableable:    true,
	}, federationsModule.fbanList)

	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:           "importfbans",
		RequiredChecks: []helpers.CheckFunc{requirePrivate()},
		Disableable:    true,
	}, federationsModule.importFbans)

	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:           "subfed",
		RequiredChecks: []helpers.CheckFunc{requirePrivate()},
		Disableable:    true,
	}, federationsModule.subFed)

	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:           "unsubfed",
		RequiredChecks: []helpers.CheckFunc{requirePrivate()},
		Disableable:    true,
	}, federationsModule.unSubFed)

	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:        "fedsubs",
		Disableable: true,
	}, federationsModule.fedSubs)

	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("setfedlog"), federationsModule.setFedLogCallback))

	// Passive enforcement watchers.
	dispatcher.AddHandlerToGroup(
		handlers.NewMessage(message.All, federations.EnforcementMessageHandler),
		federationEnforceGroup,
	)
	dispatcher.AddHandler(
		handlers.NewChatMember(
			func(u *gotgbot.ChatMemberUpdated) bool {
				newMem := u.NewChatMember.MergeChatMember()
				oldMem := u.OldChatMember.MergeChatMember()
				return newMem.Status == "member" && oldMem.Status == "left"
			},
			federations.EnforcementChatMemberHandler,
		),
	)
}

func init() {
	RegisterLegacyModule("Federations", 140, LoadFederations)
}
