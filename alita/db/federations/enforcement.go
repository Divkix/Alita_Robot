package federations

import (
	"fmt"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	log "github.com/sirupsen/logrus"
)

// CheckAndBan checks if a user is banned in the chat's federation (or subscribed feds)
// and bans them if so. Returns true if enforcement action was taken.
func CheckAndBan(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userID int64) bool {
	if b == nil || chat == nil || userID <= 0 {
		return false
	}

	fed, err := GetChatFederationCached(chat.Id)
	if err != nil {
		log.Debugf("[Federations] Enforcement skip chat %d: %v", chat.Id, err)
		return false
	}

	banned, banInfo, err := IsBannedInFederationOrSubs(fed.FedID, userID)
	if err != nil {
		log.Debugf("[Federations] Enforcement ban check error for user %d in fed %s: %v", userID, fed.FedID, err)
		return false
	}
	if !banned {
		return false
	}

	if !chat_status.CanBotRestrict(b, ctx, chat) {
		log.Debugf("[Federations] Bot cannot restrict in chat %d", chat.Id)
		return false
	}

	_, err = b.BanChatMember(chat.Id, userID, nil)
	if err != nil {
		errDesc := err.Error()
		// USER_NOT_PARTICIPANT is expected in supergroups — the ban still
		// applies to block re-entry via invite links.
		if chat.Type == "supergroup" && strings.Contains(errDesc, "USER_NOT_PARTICIPANT") {
			log.Infof("[Federations] Enforced ban: user %d not participant in supergroup %d, ban applied (blocks re-entry)", userID, chat.Id)
			quiet, _ := IsQuietCached(chat.Id)
			if !quiet {
				sendEnforcementNotice(b, ctx, chat.Id, userID, banInfo)
			}
			return true
		}
		log.Errorf("[Federations] Failed to enforce ban user %d in chat %d: %v", userID, chat.Id, err)
		return false
	}

	log.Infof("[Federations] Enforced ban: user %d banned in chat %d (fed %s)", userID, chat.Id, fed.FedID)

	quiet, _ := IsQuietCached(chat.Id)
	if !quiet {
		sendEnforcementNotice(b, ctx, chat.Id, userID, banInfo)
	}

	return true
}

// sendEnforcementNotice sends a localized ban notice to the chat.
func sendEnforcementNotice(b *gotgbot.Bot, ctx *ext.Context, chatID int64, userID int64, banInfo *models.FederationBanInfo) {
	userName := fmt.Sprintf("%d", userID)
	userChat, err := b.GetChat(userID, nil)
	if err == nil {
		userName = formatting.MentionHtml(userChat.Id, userChat.FirstName)
	}

	fedName := "a federation"
	reason := ""
	if banInfo != nil {
		fedName = banInfo.FedName
		reason = strings.TrimSpace(banInfo.Reason)
	}

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	if reason == "" {
		reason, _ = tr.GetString("federations_no_reason")
		if reason == "" {
			reason = "No reason provided"
		}
	}

	base, _ := tr.GetString("federations_enforcement_banned_message")
	if base == "" {
		base = "%s has been banned from %s.\nReason: %s"
	}

	text := fmt.Sprintf(base, userName, formatting.HtmlEscape(fedName), formatting.HtmlEscape(reason))

	_, err = b.SendMessage(chatID, text, formatting.Shtml())
	if err != nil {
		log.Debugf("[Federations] Failed to send enforcement notice: %v", err)
	}
}

// ActiveBanInChats bans a user in all federation chats where they appear.
// Intended to be run in a goroutine after /fban.
func ActiveBanInChats(b *gotgbot.Bot, fedID string, userID int64) {
	defer error_handling.RecoverFromPanic("federations", "ActiveBanInChats")

	if b == nil || userID <= 0 {
		return
	}

	chatIDs, err := ListFederationChatsCached(fedID)
	if err != nil {
		log.Errorf("[Federations] ActiveBanInChats list chats for fed %s: %v", fedID, err)
		return
	}

	// Also propagate to federations that subscribed to this federation.
	subFeds, subErr := ListFederationsSubscribedTo(fedID)
	if subErr == nil {
		for _, subFed := range subFeds {
			subChatIDs, err := ListFederationChatsCached(subFed.FedID)
			if err != nil {
				log.Errorf("[Federations] ActiveBanInChats list chats for sub-fed %s: %v", subFed.FedID, err)
				continue
			}
			chatIDs = append(chatIDs, subChatIDs...)
		}
	}

	// Deduplicate chat IDs.
	seen := make(map[int64]struct{}, len(chatIDs))
	var banSuccess, banFail int
	for _, chatID := range chatIDs {
		if _, ok := seen[chatID]; ok {
			continue
		}
		seen[chatID] = struct{}{}

		chatFull, err := b.GetChat(chatID, nil)
		if err != nil {
			log.Errorf("[Federations] ActiveBanInChats: GetChat(%d) failed: %v", chatID, err)
			banFail++
			continue
		}
		chat := chatFull.ToChat()

		if !chat_status.CanBotRestrict(b, nil, &chat) {
			log.Warnf("[Federations] ActiveBanInChats: bot cannot restrict in chat %d (missing admin rights)", chatID)
			banFail++
			continue
		}

		_, err = b.BanChatMember(chatID, userID, nil)
		if err != nil {
			errDesc := err.Error()
			// USER_NOT_PARTICIPANT is expected in supergroups — the ban still
			// applies to block re-entry via invite links.
			if chat.Type == "supergroup" && strings.Contains(errDesc, "USER_NOT_PARTICIPANT") {
				log.Infof("[Federations] ActiveBanInChats: user %d not participant in supergroup %d, ban applied (blocks re-entry)", userID, chatID)
				banSuccess++
				continue
			}
			log.Errorf("[Federations] ActiveBanInChats: BanChatMember(%d, %d) failed: %v", chatID, userID, err)
			banFail++
			continue
		}

		log.Infof("[Federations] ActiveBanInChats: banned user %d in chat %d", userID, chatID)
		banSuccess++
	}

	log.Infof("[Federations] ActiveBanInChats: fed=%s user=%d results: %d success, %d failed (out of %d chats)",
		fedID, userID, banSuccess, banFail, len(seen))
}

// EnforcementMessageHandler is a watcher that enforces federation bans on incoming messages.
func EnforcementMessageHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	sender := ctx.EffectiveSender

	if chat == nil || msg == nil || sender == nil {
		return ext.ContinueGroups
	}

	// Only enforce in groups.
	if chat.Type != "group" && chat.Type != "supergroup" {
		return ext.ContinueGroups
	}

	// Skip channel messages and the bot itself.
	senderID := sender.Id()
	if senderID <= 0 || senderID == b.Id {
		return ext.ContinueGroups
	}

	// Skip chat admins.
	if chat_status.IsUserAdmin(b, chat.Id, senderID) {
		return ext.ContinueGroups
	}

	CheckAndBan(b, ctx, chat, senderID)
	return ext.ContinueGroups
}

// EnforcementChatMemberHandler enforces federation bans when a new member joins.
func EnforcementChatMemberHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	update := ctx.Update
	if chat == nil || update == nil || update.ChatMember == nil {
		return ext.ContinueGroups
	}

	// Only act on new members.
	newMember := update.ChatMember.NewChatMember.MergeChatMember()
	oldMember := update.ChatMember.OldChatMember.MergeChatMember()
	if newMember.Status != "member" || oldMember.Status != "left" {
		return ext.ContinueGroups
	}

	userID := newMember.User.Id
	if userID <= 0 || userID == b.Id {
		return ext.ContinueGroups
	}

	CheckAndBan(b, ctx, chat, userID)
	return ext.ContinueGroups
}
