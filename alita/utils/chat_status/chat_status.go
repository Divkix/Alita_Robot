package chat_status

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/eko/gocache/lib/v4/store"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/callbackcodec"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
)

// 1087968824 - Group Anonymous Bot (For anonymous users)
// 777000 - Telegram
// 136817688 - SendAsChannel Bot (For users that send messages as channel)
const (
	groupAnonymousBot = 1087968824
	tgUserId          = 777000
)

var (
	tgAdminList           = []int64{groupAnonymousBot, tgUserId}
	anonChatMapExpiration = 20 * time.Second
)

// IsValidUserId checks if an ID represents a valid Telegram user.
// User IDs are always positive (> 0).
// Channel IDs are negative with format -100XXXXXXXXXX (< -1000000000000).
// Regular chat/group IDs are negative but in a different range.
func IsValidUserId(id int64) bool {
	// Valid user IDs are always positive
	return id > 0
}

// IsChannelId checks if an ID represents a Telegram channel.
// Channel IDs have the format -100XXXXXXXXXX (-100 prefix followed by 10+ digits).
func IsChannelId(id int64) bool {
	// Channel IDs are < -1000000000000 (-100 followed by 10+ digits)
	return id < -1000000000000
}

func callbackQueryFromContext(ctx *ext.Context) (*gotgbot.CallbackQuery, bool) {
	if ctx == nil {
		return nil, false
	}
	update := ctx.Update
	if update == nil || update.CallbackQuery == nil {
		return nil, false
	}
	return update.CallbackQuery, true
}

// checkAnonAdmin handles anonymous admin checks.
// Returns true if user should be treated as admin (anon bypass enabled),
// false if anon keyboard was sent, and a bool indicating if caller should return immediately.
func checkAnonAdmin(b *gotgbot.Bot, chat *gotgbot.Chat, msg *gotgbot.Message, sender *gotgbot.Sender) (isAdmin bool, shouldReturn bool) {
	if sender == nil || !sender.IsAnonymousAdmin() {
		return false, false
	}
	if db.GetAdminSettings(chat.Id).AnonAdmin {
		return true, true
	}
	setAnonAdminCache(chat.Id, msg)
	_, err := sendAnonAdminKeyboard(b, msg, chat)
	if err != nil {
		log.Error(err)
	}
	return false, true
}

// extractChatFromContext extracts the chat from the context.
// It handles callback queries, regular messages, and MyChatMember updates.
// If chat parameter is already provided (non-nil), it returns it directly.
//
// SAFETY NOTE: This function returns pointers to values within the context struct
// or local variables. Go's escape analysis ensures these are heap-allocated when
// their addresses escape, making the returned pointers valid for the lifetime of
// the context. The caller must ensure the context remains valid while using the
// returned pointer. This pattern is safe because:
//  1. Go's compiler escape analysis moves address-taken variables to the heap
//  2. The gotgbot.Chat struct is a value type that gets copied when assigned
//  3. All returned pointers point to stable memory locations
func extractChatFromContext(ctx *ext.Context, chat *gotgbot.Chat) *gotgbot.Chat {
	if chat != nil {
		return chat
	}
	if ctx == nil {
		return nil
	}
	update := ctx.Update
	if update == nil {
		return nil
	}
	if query := update.CallbackQuery; query != nil && query.Message != nil {
		chatValue := query.Message.GetChat()
		return &chatValue
	}
	if update.Message != nil {
		return &update.Message.Chat
	}
	if update.MyChatMember != nil {
		return &update.MyChatMember.Chat
	}
	if update.ChatMember != nil {
		return &update.ChatMember.Chat
	}
	if update.ChatJoinRequest != nil {
		return &update.ChatJoinRequest.Chat
	}
	return nil
}

// getUserMemberWithCache retrieves a chat member, using cache if available.
// Returns the merged chat member and a boolean indicating if the lookup was successful.
func getUserMemberWithCache(b *gotgbot.Bot, chat *gotgbot.Chat, userId int64, funcName string) (gotgbot.MergedChatMember, bool) {
	found, userMember := cache.GetAdminCacheUser(chat.Id, userId)
	if found {
		return userMember, true
	}
	tmpUserMember, err := chat.GetMember(b, userId, nil)
	if err != nil {
		log.Errorf("[%s] GetMember failed for user %d in chat %d: %v", funcName, userId, chat.Id, err)
		return gotgbot.MergedChatMember{}, false
	}
	return tmpUserMember.MergeChatMember(), true
}

// GetChat retrieves chat information by chat ID or username.
// Makes a direct API request to support username-based chat retrieval.
func GetChat(bot *gotgbot.Bot, chatId string) (*gotgbot.Chat, error) {
	r, err := bot.Request("getChat", map[string]any{"chat_id": chatId}, nil)
	if err != nil {
		return nil, err
	}

	var c gotgbot.Chat
	return &c, json.Unmarshal(r, &c)
}

// CheckDisabledCmd checks if a command is disabled in the chat and handles deletion if configured.
// Returns true if the command should be blocked, false if it should proceed.
// Skips checks for private chats and admin users.
// If command is disabled for non-admin users, optionally deletes the message based on chat settings.
func CheckDisabledCmd(bot *gotgbot.Bot, msg *gotgbot.Message, cmd string) bool {
	// Private chats don't have disabled commands
	if msg.Chat.Type == "private" {
		return false
	}

	// Check if command is disabled in this chat
	if !db.IsCommandDisabled(msg.Chat.Id, cmd) {
		return false
	}

	// msg.From can be nil for channel posts
	if msg.From == nil {
		return false
	}

	// Admins and creators can bypass disabled commands
	if IsUserAdmin(bot, msg.Chat.Id, msg.From.Id) {
		return false
	}

	// Command is disabled and user is not admin - block the command
	// Optionally delete the message if chat has deletion enabled
	if db.ShouldDel(msg.Chat.Id) {
		_, err := msg.Delete(bot, nil)
		if err != nil {
			log.Errorf("[CheckDisabledCmd] Failed to delete message for disabled command '%s' in chat %d: %v", cmd, msg.Chat.Id, err)
		}
	}

	// Return true to indicate command is blocked (regardless of whether deletion succeeded)
	return true
}

// IsApproved checks if a user is in the approved whitelist for a chat.
// Approved users are immune to anti-spam measures (antiflood, blacklists, locks, captcha, antispam).
// This is a simple delegation to the DB layer for consistent usage in watcher handlers.
func IsApproved(b *gotgbot.Bot, chatID, userID int64) bool {
	return db.IsUserApproved(chatID, userID)
}

// IsUserAdmin checks if a user has administrator privileges in a chat.
// Uses caching system to avoid repeated API calls and handles special Telegram admin accounts.
// Returns true if the user is an admin, creator, or special Telegram account.
func IsUserAdmin(b *gotgbot.Bot, chatID, userId int64) bool {
	// Validate user ID - channel IDs and other invalid IDs should not be checked
	// User IDs in Telegram are always positive, negative IDs are chat/channel IDs
	if !IsValidUserId(userId) {
		// Provide more specific error messages based on ID type
		if IsChannelId(userId) {
			log.WithFields(log.Fields{
				"chatID": chatID,
				"userID": userId,
			}).Debug("IsUserAdmin: Channel ID provided instead of user ID - channels cannot be admins")
		} else if userId <= 0 {
			log.WithFields(log.Fields{
				"chatID": chatID,
				"userID": userId,
			}).Warning("IsUserAdmin: Invalid user ID (negative/zero) - likely a chat/group ID, not a user ID")
		}
		return false
	}

	// Placing this first would not make additional queries if check is success!
	if slices.Contains(tgAdminList, userId) {
		return true
	}

	// Create context with timeout to prevent blocking indefinitely
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check cache first - avoid GetChat call if possible
	adminsAvail, admins := cache.GetAdminCacheList(chatID)
	if adminsAvail && admins.Cached {
		// Use cached data without making API calls
		for i := range admins.UserInfo {
			admin := &admins.UserInfo[i]
			if admin.User.Id == userId {
				return true
			}
		}
		return false
	}

	// Only make GetChat call if cache miss - use context with timeout
	chat, err := b.GetChatWithContext(ctx, chatID, nil)
	if err != nil {
		log.WithFields(log.Fields{
			"chatID": chatID,
			"userID": userId,
			"error":  err,
		}).Warning("IsUserAdmin: Failed to get chat, treating as non-admin")
		return false
	}

	// Don't allow check if not a group/supergroup
	if chat.Type != "group" && chat.Type != "supergroup" {
		return false
	}

	// Load admin cache with timeout protection
	adminList := cache.LoadAdminCache(b, chatID)

	// Check if user is in admin list
	for i := range adminList.UserInfo {
		admin := &adminList.UserInfo[i]
		if admin.User.Id == userId {
			return true
		}
	}

	// Fallback: If admin cache is empty but we know this is a group/supergroup,
	// try a direct GetChatMember call as backup using the existing context timeout
	if len(adminList.UserInfo) == 0 {
		log.WithFields(log.Fields{
			"chatID": chatID,
			"userID": userId,
		}).Debug("IsUserAdmin: Admin cache empty, trying direct GetChatMember fallback")

		// Use context-aware API call to ensure proper cancellation on timeout
		member, err := b.GetChatMemberWithContext(ctx, chatID, userId, nil)
		if err != nil {
			// Check for context timeout
			if ctx.Err() != nil {
				log.WithFields(log.Fields{
					"chatID": chatID,
					"userID": userId,
				}).Warn("IsUserAdmin: GetChatMember fallback timed out, assuming non-admin")
				return false
			}
			// Check for specific permission errors to avoid spam
			errStr := err.Error()
			if strings.Contains(errStr, "CHAT_ADMIN_REQUIRED") {
				log.WithFields(log.Fields{
					"chatID": chatID,
					"userID": userId,
				}).Debug("IsUserAdmin: Bot lacks admin rights for GetChatMember fallback")
			} else if strings.Contains(errStr, "invalid user_id specified") {
				log.WithFields(log.Fields{
					"chatID": chatID,
					"userID": userId,
				}).Warning("IsUserAdmin: Invalid user ID provided to GetChatMember")
			} else {
				log.WithFields(log.Fields{
					"chatID":    chatID,
					"userID":    userId,
					"error":     err,
					"errorType": fmt.Sprintf("%T", err),
				}).Warning("IsUserAdmin: Direct GetChatMember failed with unexpected error")
			}
			return false
		}

		status := member.GetStatus()
		isAdmin := status == "administrator" || status == "creator"

		log.WithFields(log.Fields{
			"chatID":  chatID,
			"userID":  userId,
			"status":  status,
			"isAdmin": isAdmin,
		}).Debug("IsUserAdmin: Used fallback GetChatMember")

		return isAdmin
	}

	return false
}

// IsBotAdmin checks if the bot has administrator privileges in the specified chat.
// Returns true for private chats (bot is always "admin" in private).
// For groups, verifies the bot's actual admin status.
func IsBotAdmin(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool {
	chat = extractChatFromContext(ctx, chat)
	if chat == nil {
		log.Error("IsBotAdmin: No chat information available in context")
		return false
	}

	if chat.Type == "private" {
		return true
	}

	mem, err := chat.GetMember(b, b.Id, nil)
	if err != nil {
		log.Errorf("[IsBotAdmin] GetMember failed for chat %d: %v", chat.Id, err)
		return false
	}

	return mem.MergeChatMember().Status == "administrator"
}

// CanUserChangeInfo checks if a user has permission to change chat information.
// Handles anonymous admins and validates the CanChangeInfo permission.
func CanUserChangeInfo(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64) bool {
	return canUserChangeInfo(b, ctx, chat, userId)
}

// CanUserRestrict checks if a user has permission to restrict other members.
// Handles anonymous admins and validates the CanRestrictMembers permission.
func CanUserRestrict(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64) bool {
	return canUserRestrict(b, ctx, chat, userId)
}

// CanBotRestrict checks if the bot has permission to restrict members in the chat.
// Validates the bot's CanRestrictMembers permission.
func CanBotRestrict(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool {
	return canBotRestrict(b, ctx, chat)
}

// CanUserPromote checks if a user has permission to promote/demote other members.
// Handles anonymous admins and validates the CanPromoteMembers permission.
func CanUserPromote(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64) bool {
	return canUserPromote(b, ctx, chat, userId)
}

// CanBotPromote checks if the bot has permission to promote/demote members in the chat.
// Validates the bot's CanPromoteMembers permission.
func CanBotPromote(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool {
	return canBotPromote(b, ctx, chat)
}

// CanUserPin checks if a user has permission to pin messages in the chat.
// Handles anonymous admins and validates the CanPinMessages permission.
func CanUserPin(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64) bool {
	return canUserPin(b, ctx, chat, userId)
}

// CanBotPin checks if the bot has permission to pin messages in the chat.
// Validates the bot's CanPinMessages permission.
func CanBotPin(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool {
	return canBotPin(b, ctx, chat)
}

// CanInvite checks if the bot and user have permissions to generate invite links.
// Returns true immediately if the chat has a public username.
// Validates both bot and user permissions for invite link generation.
func CanInvite(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, msg *gotgbot.Message) bool {
	chat = extractChatFromContext(ctx, chat)
	if chat == nil {
		log.Error("CanInvite: No chat information available in context")
		return false
	}
	if chat.Username != "" {
		return true
	}
	botChatMember, err := chat.GetMember(b, b.Id, nil)
	if err != nil {
		log.Errorf("[CanInvite] GetMember failed for bot in chat %d: %v", chat.Id, err)
		return false
	}
	if !botChatMember.MergeChatMember().CanInviteUsers {
		return false
	}
	sender := ctx.EffectiveSender

	if isAdmin, shouldReturn := checkAnonAdmin(b, chat, msg, sender); shouldReturn {
		return isAdmin
	}

	// msg.From can be nil for channel posts
	if msg.From == nil {
		return false
	}

	userid := msg.From.Id
	userMember, ok := getUserMemberWithCache(b, chat, userid, "CanInvite")
	if !ok {
		return false
	}

	if !userMember.CanInviteUsers && userMember.Status != "creator" {
		return false
	}
	return true
}

// CanUserDelete checks if a user has permission to delete messages in the chat.
// Handles anonymous admins and validates the CanDeleteMessages permission.
func CanUserDelete(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64) bool {
	return canUserDelete(b, ctx, chat, userId)
}

// CanBotDelete checks if the bot has permission to delete messages in the chat.
// Validates the bot's CanDeleteMessages permission.
func CanBotDelete(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool {
	return canBotDelete(b, ctx, chat)
}

// RequireBotAdmin ensures the bot has administrator privileges in the chat.
// Uses IsBotAdmin internally to perform the check.
func RequireBotAdmin(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool {
	return requireBotAdminPure(b, ctx, chat)
}

// IsUserInChat checks if a user is currently a member of the specified chat.
// Returns false for special Telegram accounts and users with "left" or "kicked" status.
func IsUserInChat(b *gotgbot.Bot, chat *gotgbot.Chat, userId int64) bool {
	// telegram cannot be in chat, will need to fix this later
	if userId == tgUserId {
		return false
	}
	member, err := chat.GetMember(b, userId, nil)
	if err != nil {
		log.Errorf("[IsUserInChat] GetMember failed for user %d in chat %d: %v", userId, chat.Id, err)
		return false
	}
	userStatus := member.MergeChatMember().Status
	return !slices.Contains([]string{"left", "kicked"}, userStatus)
}

// IsUserConnected checks if a user is connected to a chat and validates permissions.
// Handles both private messages (with connection system) and group messages.
// Returns the effective chat if all checks pass, nil otherwise.
func IsUserConnected(b *gotgbot.Bot, ctx *ext.Context, chatAdmin, botAdmin bool) (chat *gotgbot.Chat) {
	msg := ctx.EffectiveMessage
	user := ctx.EffectiveUser
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	if msg == nil || user == nil {
		return nil
	}

	if msg.Chat.Type == "private" {
		conn := db.Connection(user.Id)
		if conn.Connected && conn.ChatId != 0 {
			chatFullInfo, err := b.GetChat(conn.ChatId, nil)
			if err != nil {
				log.WithFields(log.Fields{
					"userId": user.Id,
					"chatId": conn.ChatId,
					"error":  err,
				}).Warn("Stale connection detected - chat no longer accessible")
				// Provide user feedback about stale connection
				text, _ := tr.GetString("connections_stale_connection")
				_, _ = msg.Reply(b, text, nil)
				return nil
			}
			_chat := chatFullInfo.ToChat() // need to convert to Chat type
			chat = &_chat
		} else {
			text, _ := tr.GetString("connections_is_user_connected_need_group")
			_, err := msg.Reply(b,
				text,
				&gotgbot.SendMessageOpts{
					ReplyParameters: &gotgbot.ReplyParameters{
						MessageId:                msg.MessageId,
						AllowSendingWithoutReply: true,
					},
				},
			)
			if err != nil {
				log.Error(err)
				return nil
			}

			return nil
		}
	} else {
		chat = ctx.EffectiveChat
	}
	if botAdmin {
		if !IsUserAdmin(b, chat.Id, b.Id) {
			text, _ := tr.GetString("connections_is_user_connected_bot_not_admin")
			_, err := msg.Reply(b, text, formatting.Shtml())
			if err != nil {
				log.Error(err)
				return nil
			}

			return nil
		}
	}
	if chatAdmin {
		if !IsUserAdmin(b, chat.Id, user.Id) {
			text, _ := tr.GetString("connections_is_user_connected_user_not_admin")
			_, err := msg.Reply(b, text, formatting.Shtml())
			if err != nil {
				log.Error(err)
				return nil
			}

			return nil
		}
	}
	return chat
}

// IsUserBanProtected checks if a user is protected from being banned.
// Returns true for private chats, admins, and special Telegram accounts.
// Used to prevent banning of administrators and system accounts.
func IsUserBanProtected(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64) bool {
	chat = extractChatFromContext(ctx, chat)
	if chat == nil {
		log.Error("IsUserBanProtected: No chat information available in context")
		return false
	}

	if chat.Type == "private" {
		return true
	}

	return IsUserAdmin(b, ctx.EffectiveChat.Id, userId) || slices.Contains(tgAdminList, userId)
}

// RequireUserAdmin ensures a user has administrator privileges in the chat.
// Uses IsUserAdmin internally to perform the check.
func RequireUserAdmin(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64) bool {
	return requireUserAdminPure(b, ctx, chat, userId)
}

// RequireUserOwner ensures a user is the chat creator/owner.
// Checks for "creator" status specifically, not just administrator.
func RequireUserOwner(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64) bool {
	return requireUserOwnerPure(b, ctx, chat, userId)
}

// RequirePrivate ensures the command is being used in a private chat.
// Returns false for group chats and supergroups.
//
//nolint:dupl // RequirePrivate/RequireGroup have symmetric logic
func RequirePrivate(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool {
	return requirePrivatePure(b, ctx, chat)
}

// RequireGroup ensures the command is being used in a group chat.
// Returns false for private chats.
//
//nolint:dupl // RequirePrivate/RequireGroup have symmetric logic
func RequireGroup(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool {
	return requireGroupPure(b, ctx, chat)
}

// setAnonAdminCache stores anonymous admin message information in cache.
// Used to track anonymous admin verification requests with expiration.
// Logs errors but doesn't fail since cache is non-critical.
func setAnonAdminCache(chatId int64, msg *gotgbot.Message) {
	m := cache.GetMarshal()
	if m == nil || msg == nil {
		log.Debug("Skipping anonymous admin cache set: cache unavailable or message nil")
		return
	}
	err := m.Set(cache.Context, fmt.Sprintf("alita:anonAdmin:%d:%d", chatId, msg.MessageId), msg, store.WithExpiration(anonChatMapExpiration))
	if err != nil {
		// Log error but don't fail the operation since cache is not critical
		log.Errorf("Failed to set anonymous admin cache: %v", err)
	}
}

// GetEffectiveUser safely extracts the user from context.
// Returns nil for channel posts and cases where user is unavailable.
func GetEffectiveUser(ctx *ext.Context) *gotgbot.User {
	if ctx == nil || ctx.EffectiveSender == nil {
		return nil
	}
	return ctx.EffectiveSender.User
}

// RequireUser ensures a valid user exists in context.
// Returns the user or nil.
func RequireUser(b *gotgbot.Bot, ctx *ext.Context) *gotgbot.User {
	return GetEffectiveUser(ctx)
}

// GetMessageLinkFromMessageId generates a Telegram message link from chat and message ID.
// Handles both public groups (with username) and private groups (without username).
// NOTE: msg.GetLink() only works for supergroups/channels. This custom implementation
// also handles private groups and non-supergroups by constructing the link manually.
func GetMessageLinkFromMessageId(chat *gotgbot.Chat, messageId int64) (messageLink string) {
	// This function expects group/supergroup/channel chats (negative IDs).
	// For user chats or invalid contexts, return empty string.
	if chat == nil || chat.Id >= 0 {
		return ""
	}

	messageLink = "https://t.me/"
	chatIdStr := fmt.Sprint(chat.Id)
	if chat.Username == "" {
		var linkId string
		if IsChannelId(chat.Id) {
			linkId = strings.ReplaceAll(chatIdStr, "-100", "")
		} else if strings.HasPrefix(chatIdStr, "-") && !IsChannelId(chat.Id) {
			// this is for non-supergroups
			linkId = strings.ReplaceAll(chatIdStr, "-", "")
		}
		messageLink += fmt.Sprintf("c/%s/%d", linkId, messageId)
	} else {
		messageLink += fmt.Sprintf("%s/%d", chat.Username, messageId)
	}
	return
}

// ExtractJoinLeftStatusChange analyzes ChatMemberUpdated events to detect join/leave status changes.
// Returns (was_member, is_member) booleans indicating membership status transition.
// Returns (false, false) for channels or if no status change occurred.
func ExtractJoinLeftStatusChange(u *gotgbot.ChatMemberUpdated) (bool, bool) {
	// return false for channels
	if u.Chat.Type == "channel" {
		return false, false
	}

	oldMemberStatus := u.OldChatMember.MergeChatMember().Status
	newMemberStatus := u.NewChatMember.MergeChatMember().Status
	oldIsMember := u.OldChatMember.MergeChatMember().IsMember
	newIsMember := u.NewChatMember.MergeChatMember().IsMember

	if oldMemberStatus == newMemberStatus {
		return false, false
	}

	wasMember := slices.Contains(
		[]string{"member", "administrator", "creator"},
		oldMemberStatus,
	) || (oldMemberStatus == "restricted" && oldIsMember)

	isMember := slices.Contains(
		[]string{"member", "administrator", "creator"},
		newMemberStatus,
	) || (newMemberStatus == "restricted" && newIsMember)

	return wasMember, isMember
}

// ExtractAdminUpdateStatusChange detects admin status changes from ChatMemberUpdated events.
// Returns true if there was a transition to/from administrator or creator status.
// Returns false for channels or if no admin status change occurred.
func ExtractAdminUpdateStatusChange(u *gotgbot.ChatMemberUpdated) bool {
	// return false for channels
	if u.Chat.Type == "channel" {
		return false
	}

	oldMemberStatus := u.OldChatMember.MergeChatMember().Status
	newMemberStatus := u.NewChatMember.MergeChatMember().Status

	// status remains same
	if oldMemberStatus == newMemberStatus {
		return false
	}

	adminStatusChanged := (slices.Contains(
		[]string{"administrator", "creator"},
		oldMemberStatus,
	) && !slices.Contains(
		[]string{"administrator", "creator"},
		newMemberStatus,
	)) ||
		(slices.Contains(
			[]string{"administrator", "creator"},
			newMemberStatus,
		) && !slices.Contains(
			[]string{"administrator", "creator"},
			oldMemberStatus,
		))

	return adminStatusChanged
}

// sendAnonAdminKeyboard sends an inline keyboard to verify anonymous admin identity.
// Creates a callback button that anonymous admins can click to prove their admin status.
func sendAnonAdminKeyboard(b *gotgbot.Bot, msg *gotgbot.Message, chat *gotgbot.Chat) (*gotgbot.Message, error) {
	// Create a minimal context to get the language
	ctx := &ext.Context{
		EffectiveMessage: msg,
	}

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	mainText, _ := tr.GetString("chat_status_anon_confirm")
	buttonText, _ := tr.GetString("chat_status_anon_prove_admin")

	return msg.Reply(b,
		mainText,
		&gotgbot.SendMessageOpts{
			ReplyMarkup: gotgbot.InlineKeyboardMarkup{
				InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
					{{
						Text: buttonText,
						CallbackData: callbackcodec.EncodeOrFallback(
							"anon_admin",
							map[string]string{
								"c": fmt.Sprint(chat.Id),
								"m": fmt.Sprint(msg.MessageId),
							},
							fmt.Sprintf("alita:anonAdmin:%d:%d", chat.Id, msg.MessageId),
						),
					}},
				},
			},
		},
	)
}
