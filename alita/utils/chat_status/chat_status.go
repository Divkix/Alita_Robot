package chat_status

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/eko/gocache/lib/v4/store"
	log "github.com/sirupsen/logrus"

	"github.com/divideprojects/Alita_Robot/alita/db"
	"github.com/divideprojects/Alita_Robot/alita/i18n"
	"github.com/divideprojects/Alita_Robot/alita/utils/cache"
	"github.com/divideprojects/Alita_Robot/alita/utils/error_handling"
	"github.com/divideprojects/Alita_Robot/alita/utils/string_handling"
)

// 1087968824 - Group Anonymous Bot (For anonymous users)
// 777000 - Telegram
// 136817688 - SendAsChannel Bot (For users that send messages as channel)
const (
	groupAnonymousBot = 1087968824
	tgUserId          = 777000
)

var (
	tgAdminList            = []int64{groupAnonymousBot, tgUserId}
	anonChatMapExpirartion = 20 * time.Second
)

// GetChat retrieves chat information by chat ID or username.
// Makes a direct API request to support username-based chat retrieval.
func GetChat(bot *gotgbot.Bot, chatId string) (*gotgbot.Chat, error) {
	r, err := bot.Request("getChat", map[string]string{"chat_id": chatId}, nil, nil)
	if err != nil {
		return nil, err
	}

	var c gotgbot.Chat
	return &c, json.Unmarshal(r, &c)
}

// CheckDisabledCmd checks if a command is disabled in the chat and handles deletion if configured.
// Returns true if the command should be blocked, false if it should proceed.
// Skips checks for private chats and admin users.
func CheckDisabledCmd(bot *gotgbot.Bot, msg *gotgbot.Message, cmd string) bool {
	// Placing this first would not make additional queries if check is success!
	// if chat is private, return false
	if msg.Chat.Type == "private" {
		return false
	}

	// check if command is disabled
	if !db.IsCommandDisabled(msg.Chat.Id, cmd) {
		return false
	}

	// check if user is admin or creator, can bypass disabled commands
	if IsUserAdmin(bot, msg.Chat.Id, msg.From.Id) {
		return false
	}

	// check if ShouldDel is enabled, if not, just return true
	if !db.ShouldDel(msg.Chat.Id) {
		return false
	}

	// delete the message and return false
	_, err := msg.Delete(bot, nil)
	if err != nil {
		log.Error(err)
	}

	return true
}

// IsUserAdmin checks if a user has administrator privileges in a chat.
// Uses caching system to avoid repeated API calls and handles special Telegram admin accounts.
// Returns true if the user is an admin, creator, or special Telegram account.
func IsUserAdmin(b *gotgbot.Bot, chatID, userId int64) bool {
	// Validate user ID - channel IDs and other invalid IDs should not be checked
	// User IDs in Telegram are always positive, negative IDs are chat/channel IDs
	if userId <= 0 {
		log.WithFields(log.Fields{
			"chatID": chatID,
			"userID": userId,
		}).Warning("IsUserAdmin: Invalid user ID (negative/zero) - likely a channel/chat ID, not a user ID")
		return false
	}

	// Placing this first would not make additional queries if check is success!
	if string_handling.FindInInt64Slice(tgAdminList, userId) {
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
	// try a direct GetChatMember call as backup
	if len(adminList.UserInfo) == 0 {
		log.WithFields(log.Fields{
			"chatID": chatID,
			"userID": userId,
		}).Debug("IsUserAdmin: Admin cache empty, trying direct GetChatMember fallback")

		member, err := b.GetChatMember(chatID, userId, nil)
		if err != nil {
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
	if chat == nil {
		if ctx.CallbackQuery != nil {
			_chatValue := ctx.CallbackQuery.Message.GetChat()
			chat = &_chatValue
		} else if ctx.Message != nil {
			chat = &ctx.Message.Chat
		} else if ctx.MyChatMember != nil {
			chat = &ctx.MyChatMember.Chat
		} else {
			log.Error("IsBotAdmin: No chat information available in context")
			return false
		}
	}

	if chat.Type == "private" {
		return true
	}

	mem, err := chat.GetMember(b, b.Id, nil)
	error_handling.HandleErr(err)

	return mem.MergeChatMember().Status == "administrator"
}

// CanUserChangeInfo checks if a user has permission to change chat information.
// Handles anonymous admins and validates the CanChangeInfo permission.
// If justCheck is false, sends error messages to user.
func CanUserChangeInfo(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64, justCheck bool) bool {
	if chat == nil {
		if ctx.CallbackQuery != nil {
			_chatValue := ctx.CallbackQuery.Message.GetChat()
			chat = &_chatValue
		} else {
			chat = &ctx.Message.Chat
		}
	}

	msg := ctx.EffectiveMessage
	sender := ctx.EffectiveSender
	var userMember gotgbot.MergedChatMember

	if db.GetAdminSettings(chat.Id).AnonAdmin && sender.IsAnonymousAdmin() {
		return true
	}

	// group anonymous bot
	if sender.IsAnonymousAdmin() {
		setAnonAdminCache(chat.Id, msg)
		_, err := sendAnonAdminKeyboard(b, msg, chat)
		if err != nil {
			log.Error(err)
		}
		return false
	}

	found, userMember := cache.GetAdminCacheUser(chat.Id, userId)
	if !found {
		tmpUserMember, err := chat.GetMember(b, userId, nil)
		userMember = tmpUserMember.MergeChatMember()
		error_handling.HandleErr(err)
	}

	if !userMember.CanChangeInfo && userMember.Status != "creator" {
		query := ctx.CallbackQuery
		if query != nil {
			if !justCheck {
				tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
				text, _ := tr.GetString("chat_status_change_info_button_error")
				_, err := query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: text})
				if err != nil {
					log.Error(err)
					return false
				}
			}
			return false
		}
		if !justCheck {
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			text, _ := tr.GetString("chat_status_change_info_cmd_error")
			_, err := b.SendMessage(chat.Id, text,
				&gotgbot.SendMessageOpts{
					ReplyParameters: &gotgbot.ReplyParameters{
						MessageId:                ctx.EffectiveMessage.MessageId,
						AllowSendingWithoutReply: true,
					},
				},
			)
			error_handling.HandleErr(err)
		}
		return false
	}
	return true
}

// CanUserRestrict checks if a user has permission to restrict other members.
// Handles anonymous admins and validates the CanRestrictMembers permission.
// If justCheck is false, sends error messages to user.
func CanUserRestrict(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64, justCheck bool) bool {
	if chat == nil {
		if ctx.CallbackQuery != nil {
			_chatValue := ctx.CallbackQuery.Message.GetChat()
			chat = &_chatValue
		} else {
			chat = &ctx.Message.Chat
		}
	}

	msg := ctx.EffectiveMessage
	sender := ctx.EffectiveSender
	var userMember gotgbot.MergedChatMember

	if db.GetAdminSettings(chat.Id).AnonAdmin && sender.IsAnonymousAdmin() {
		return true
	}

	// group anonymous bot
	if sender.IsAnonymousAdmin() {
		setAnonAdminCache(chat.Id, msg)
		_, err := sendAnonAdminKeyboard(b, msg, chat)
		if err != nil {
			log.Error(err)
		}
		return false
	}

	found, userMember := cache.GetAdminCacheUser(chat.Id, userId)
	if !found {
		tmpUserMember, err := chat.GetMember(b, userId, nil)
		userMember = tmpUserMember.MergeChatMember()
		error_handling.HandleErr(err)
	}
	if !userMember.CanRestrictMembers && userMember.Status != "creator" {
		query := ctx.CallbackQuery
		if query != nil {
			if !justCheck {
				tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
				text, _ := tr.GetString("chat_status_restrict_button_error")
				_, err := query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: text})
				if err != nil {
					log.Error(err)
					return false
				}
			}
			return false
		}
		if !justCheck {
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			text, _ := tr.GetString("chat_status_restrict_cmd_error")
			_, err := b.SendMessage(chat.Id, text,
				&gotgbot.SendMessageOpts{
					ReplyParameters: &gotgbot.ReplyParameters{
						MessageId:                ctx.EffectiveMessage.MessageId,
						AllowSendingWithoutReply: true,
					},
				},
			)
			error_handling.HandleErr(err)
		}
		return false
	}
	return true
}

// CanBotRestrict checks if the bot has permission to restrict members in the chat.
// Validates the bot's CanRestrictMembers permission.
// If justCheck is false, sends error messages explaining the missing permission.
func CanBotRestrict(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, justCheck bool) bool {
	if chat == nil {
		if ctx.CallbackQuery != nil {
			_chatValue := ctx.CallbackQuery.Message.GetChat()
			chat = &_chatValue
		} else {
			chat = &ctx.Message.Chat
		}
	}

	botMember, err := chat.GetMember(b, b.Id, nil)
	error_handling.HandleErr(err)
	if !botMember.MergeChatMember().CanRestrictMembers {
		query := ctx.CallbackQuery
		if query != nil {
			if !justCheck {
				tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
				text, _ := tr.GetString("chat_status_bot_restrict_error")
				_, err := query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: text})
				if err != nil {
					log.Error(err)
					return false
				}
			}
			return false
		}
		if !justCheck {
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			text, _ := tr.GetString("chat_status_bot_restrict_group_error")
			_, err := b.SendMessage(chat.Id, text,
				&gotgbot.SendMessageOpts{
					ReplyParameters: &gotgbot.ReplyParameters{
						MessageId:                ctx.EffectiveMessage.MessageId,
						AllowSendingWithoutReply: true,
					},
				},
			)
			error_handling.HandleErr(err)
		}
		return false
	}
	return true
}

// CanUserPromote checks if a user has permission to promote/demote other members.
// Handles anonymous admins and validates the CanPromoteMembers permission.
// If justCheck is false, sends error messages to user.
func CanUserPromote(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64, justCheck bool) bool {
	if chat == nil {
		if ctx.CallbackQuery != nil {
			_chatValue := ctx.CallbackQuery.Message.GetChat()
			chat = &_chatValue
		} else {
			chat = &ctx.Message.Chat
		}
	}

	msg := ctx.EffectiveMessage
	sender := ctx.EffectiveSender
	var userMember gotgbot.MergedChatMember

	if db.GetAdminSettings(chat.Id).AnonAdmin && sender.IsAnonymousAdmin() {
		return true
	}

	// group anonymous bot
	if sender.IsAnonymousAdmin() {
		setAnonAdminCache(chat.Id, msg)
		_, err := sendAnonAdminKeyboard(b, msg, chat)
		if err != nil {
			log.Error(err)
		}
		return false
	}

	found, userMember := cache.GetAdminCacheUser(chat.Id, userId)
	if !found {
		tmpUserMember, err := chat.GetMember(b, userId, nil)
		userMember = tmpUserMember.MergeChatMember()
		error_handling.HandleErr(err)
	}
	if !userMember.CanPromoteMembers && userMember.Status != "creator" {
		query := ctx.CallbackQuery
		if query != nil {
			if !justCheck {
				tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
				text, _ := tr.GetString("chat_status_promote_button_error")
				_, err := query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: text})
				if err != nil {
					log.Error(err)
					return false
				}
			}
			return false
		}
		if !justCheck {
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			text, _ := tr.GetString("chat_status_promote_cmd_error")
			_, err := b.SendMessage(chat.Id, text,
				&gotgbot.SendMessageOpts{
					ReplyParameters: &gotgbot.ReplyParameters{
						MessageId:                ctx.EffectiveMessage.MessageId,
						AllowSendingWithoutReply: true,
					},
				},
			)
			error_handling.HandleErr(err)
		}
		return false
	}
	return true
}

// CanBotPromote checks if the bot has permission to promote/demote members in the chat.
// Validates the bot's CanPromoteMembers permission.
// If justCheck is false, sends error messages explaining the missing permission.
func CanBotPromote(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, justCheck bool) bool {
	if chat == nil {
		if ctx.CallbackQuery != nil {
			_chatValue := ctx.CallbackQuery.Message.GetChat()
			chat = &_chatValue
		} else {
			chat = &ctx.Message.Chat
		}
	}

	botChatMember, err := chat.GetMember(b, b.Id, nil)
	error_handling.HandleErr(err)
	if !botChatMember.MergeChatMember().CanPromoteMembers {
		if !justCheck {
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			text, _ := tr.GetString("chat_status_bot_promote_error")
			_, err := b.SendMessage(chat.Id, text,
				&gotgbot.SendMessageOpts{
					ReplyParameters: &gotgbot.ReplyParameters{
						MessageId:                ctx.EffectiveMessage.MessageId,
						AllowSendingWithoutReply: true,
					},
				},
			)
			error_handling.HandleErr(err)
		}
		return false
	}
	return true
}

// CanUserPin checks if a user has permission to pin messages in the chat.
// Handles anonymous admins and validates the CanPinMessages permission.
// If justCheck is false, sends error messages to user.
func CanUserPin(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64, justCheck bool) bool {
	if chat == nil {
		if ctx.CallbackQuery != nil {
			_chatValue := ctx.CallbackQuery.Message.GetChat()
			chat = &_chatValue
		} else {
			chat = &ctx.Message.Chat
		}
	}

	msg := ctx.EffectiveMessage
	sender := ctx.EffectiveSender
	var userMember gotgbot.MergedChatMember

	if db.GetAdminSettings(chat.Id).AnonAdmin && sender.IsAnonymousAdmin() {
		return true
	}

	// group anonymous bot
	if sender.IsAnonymousAdmin() {
		setAnonAdminCache(chat.Id, msg)
		_, err := sendAnonAdminKeyboard(b, msg, chat)
		if err != nil {
			log.Error(err)
		}
		return false
	}

	found, userMember := cache.GetAdminCacheUser(chat.Id, userId)
	if !found {
		tmpUserMember, err := chat.GetMember(b, userId, nil)
		userMember = tmpUserMember.MergeChatMember()
		error_handling.HandleErr(err)
	}
	if !userMember.CanPinMessages && userMember.Status != "creator" {
		if !justCheck {
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			text, _ := tr.GetString("chat_status_pin_user_error")
			_, err := b.SendMessage(chat.Id, text, &gotgbot.SendMessageOpts{
				ReplyParameters: &gotgbot.ReplyParameters{
					MessageId:                ctx.EffectiveMessage.MessageId,
					AllowSendingWithoutReply: true,
				},
			},
			)
			error_handling.HandleErr(err)
		}
		return false
	}
	return true
}

// CanBotPin checks if the bot has permission to pin messages in the chat.
// Validates the bot's CanPinMessages permission.
// If justCheck is false, sends error messages explaining the missing permission.
func CanBotPin(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, justCheck bool) bool {
	if chat == nil {
		if ctx.CallbackQuery != nil {
			_chatValue := ctx.CallbackQuery.Message.GetChat()
			chat = &_chatValue
		} else {
			chat = &ctx.Message.Chat
		}
	}

	botChatMember, err := chat.GetMember(b, b.Id, nil)
	error_handling.HandleErr(err)
	if !botChatMember.MergeChatMember().CanPinMessages {
		if !justCheck {
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			text, _ := tr.GetString("chat_status_pin_bot_error")
			_, err := b.SendMessage(chat.Id, text,
				&gotgbot.SendMessageOpts{
					ReplyParameters: &gotgbot.ReplyParameters{
						MessageId:                ctx.EffectiveMessage.MessageId,
						AllowSendingWithoutReply: true,
					},
				},
			)
			error_handling.HandleErr(err)
		}
		return false
	}
	return true
}

// Caninvite checks if the bot and user have permissions to generate invite links.
// Returns true immediately if the chat has a public username.
// Validates both bot and user permissions for invite link generation.
func Caninvite(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, msg *gotgbot.Message, justCheck bool) bool {
	if chat == nil {
		if ctx.CallbackQuery != nil {
			_chatValue := ctx.CallbackQuery.Message.GetChat()
			chat = &_chatValue
		} else {
			chat = &ctx.Message.Chat
		}
	}
	if chat.Username != "" {
		return true
	}
	botChatMember, err := chat.GetMember(b, b.Id, nil)
	error_handling.HandleErr(err)
	if !botChatMember.MergeChatMember().CanInviteUsers {
		if !justCheck {
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			text, _ := tr.GetString("chat_status_invite_link_bot_error")
			_, err := b.SendMessage(chat.Id, text,
				&gotgbot.SendMessageOpts{
					ReplyParameters: &gotgbot.ReplyParameters{
						MessageId:                ctx.EffectiveMessage.MessageId,
						AllowSendingWithoutReply: true,
					},
				},
			)
			error_handling.HandleErr(err)
		}
		return false
	}
	sender := ctx.EffectiveSender
	if db.GetAdminSettings(chat.Id).AnonAdmin && sender.IsAnonymousAdmin() {
		return true
	}
	var userMember gotgbot.MergedChatMember

	// group anonymous bot
	if sender.IsAnonymousAdmin() {
		setAnonAdminCache(chat.Id, msg)
		_, err := sendAnonAdminKeyboard(b, msg, chat)
		if err != nil {
			log.Error(err)
		}
		return false
	}
	userid := msg.From.Id
	found, userMember := cache.GetAdminCacheUser(chat.Id, userid)
	if !found {
		tmpUserMember, err := chat.GetMember(b, userid, nil)
		userMember = tmpUserMember.MergeChatMember()
		error_handling.HandleErr(err)
	}
	if !userMember.CanInviteUsers && userMember.Status != "creator" {
		if !justCheck {
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			text, _ := tr.GetString("chat_status_invite_link_user_error")
			_, err := b.SendMessage(chat.Id, text,
				&gotgbot.SendMessageOpts{
					ReplyParameters: &gotgbot.ReplyParameters{
						MessageId:                ctx.EffectiveMessage.MessageId,
						AllowSendingWithoutReply: true,
					},
				},
			)
			error_handling.HandleErr(err)
		}
		return false
	}
	return true
}

// CanUserDelete checks if a user has permission to delete messages in the chat.
// Handles anonymous admins and validates the CanDeleteMessages permission.
// If justCheck is false, sends error messages to user.
func CanUserDelete(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64, justCheck bool) bool {
	if chat == nil {
		if ctx.CallbackQuery != nil {
			_chatValue := ctx.CallbackQuery.Message.GetChat()
			chat = &_chatValue
		} else {
			chat = &ctx.Message.Chat
		}
	}

	msg := ctx.EffectiveMessage
	sender := ctx.EffectiveSender
	var userMember gotgbot.MergedChatMember

	if db.GetAdminSettings(chat.Id).AnonAdmin && sender.IsAnonymousAdmin() {
		return true
	}

	// group anonymous bot
	if sender.IsAnonymousAdmin() {
		setAnonAdminCache(chat.Id, msg)
		_, err := sendAnonAdminKeyboard(b, msg, chat)
		if err != nil {
			log.Error(err)
		}
		return false
	}

	found, userMember := cache.GetAdminCacheUser(chat.Id, userId)
	if !found {
		tmpUserMember, err := chat.GetMember(b, userId, nil)
		userMember = tmpUserMember.MergeChatMember()
		error_handling.HandleErr(err)
	}

	if !userMember.CanDeleteMessages && userMember.Status != "creator" {
		query := ctx.CallbackQuery
		if query != nil {
			if !justCheck {
				tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
				text, _ := tr.GetString("chat_status_delete_button_error")
				_, err := query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: text})
				if err != nil {
					log.Error(err)
					return false
				}
			}
			return false
		}
		if !justCheck {
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			text, _ := tr.GetString("chat_status_delete_cmd_error")
			_, err := msg.Reply(b, text, nil)
			if err != nil {
				log.Error(err)
				return false
			}
		}
		return false
	}
	return true
}

// CanBotDelete checks if the bot has permission to delete messages in the chat.
// Validates the bot's CanDeleteMessages permission.
// If justCheck is false, sends error messages explaining the missing permission.
func CanBotDelete(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, justCheck bool) bool {
	if chat == nil {
		if ctx.CallbackQuery != nil {
			_chatValue := ctx.CallbackQuery.Message.GetChat()
			chat = &_chatValue
		} else {
			chat = &ctx.Message.Chat
		}
	}

	msg := ctx.EffectiveMessage
	botChatMember, err := chat.GetMember(b, b.Id, nil)
	if err != nil {
		log.Error(err)
		return false
	}

	if !botChatMember.MergeChatMember().CanDeleteMessages {
		if !justCheck {
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			text, _ := tr.GetString("chat_status_bot_delete_error")
			_, err := msg.Reply(b, text, nil)
			if err != nil {
				log.Error(err)
				return false
			}
		}
		return false
	}
	return true
}

// RequireBotAdmin ensures the bot has administrator privileges in the chat.
// Uses IsBotAdmin internally to perform the check.
// If justCheck is false, sends error messages when bot is not admin.
func RequireBotAdmin(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, justCheck bool) bool {
	if chat == nil {
		if ctx.CallbackQuery != nil {
			_chatValue := ctx.CallbackQuery.Message.GetChat()
			chat = &_chatValue
		} else {
			chat = &ctx.Message.Chat
		}
	}

	msg := ctx.EffectiveMessage
	if !IsBotAdmin(b, ctx, chat) {
		if !justCheck {
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			text, _ := tr.GetString("chat_status_bot_not_admin")
			_, err := msg.Reply(b, text, nil)
			error_handling.HandleErr(err)
		}
		return false
	}
	return true
}

// IsUserInChat checks if a user is currently a member of the specified chat.
// Returns false for special Telegram accounts and users with "left" or "kicked" status.
func IsUserInChat(b *gotgbot.Bot, chat *gotgbot.Chat, userId int64) bool {
	// telegram cannot be in chat, will need to fix this later
	if userId == tgUserId {
		return false
	}
	member, err := chat.GetMember(b, userId, nil)
	error_handling.HandleErr(err)
	userStatus := member.MergeChatMember().Status
	return !string_handling.FindInStringSlice([]string{"left", "kicked"}, userStatus)
}

// IsUserBanProtected checks if a user is protected from being banned.
// Returns true for private chats, admins, and special Telegram accounts.
// Used to prevent banning of administrators and system accounts.
func IsUserBanProtected(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64) bool {
	if chat == nil {
		if ctx.CallbackQuery != nil {
			_chatValue := ctx.CallbackQuery.Message.GetChat()
			chat = &_chatValue
		} else {
			chat = &ctx.Message.Chat
		}
	}

	if chat.Type == "private" {
		return true
	}

	return IsUserAdmin(b, ctx.EffectiveChat.Id, userId) || string_handling.FindInInt64Slice(tgAdminList, userId)
}

// RequireUserAdmin ensures a user has administrator privileges in the chat.
// Uses IsUserAdmin internally to perform the check.
// If justCheck is false, sends error messages when user is not admin.
func RequireUserAdmin(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64, justCheck bool) bool {
	if chat == nil {
		if ctx.CallbackQuery != nil {
			_chatValue := ctx.CallbackQuery.Message.GetChat()
			chat = &_chatValue
		} else {
			chat = &ctx.Message.Chat
		}
	}

	msg := ctx.EffectiveMessage
	if !IsUserAdmin(b, chat.Id, userId) {
		query := ctx.CallbackQuery
		if query != nil {
			if !justCheck {
				tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
				text, _ := tr.GetString("chat_status_user_admin_button_error")
				_, err := query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: text})
				if err != nil {
					log.Error(err)
					return false
				}
			}
			return false
		}
		if !justCheck {
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			text, _ := tr.GetString("chat_status_user_admin_cmd_error")

			// Try to send error message with retry and fallback
			_, err := msg.Reply(b, text, nil)
			if err != nil {
				log.WithFields(log.Fields{
					"chatId": chat.Id,
					"userId": userId,
					"error":  err,
				}).Warning("RequireUserAdmin: Reply failed, trying SendMessage fallback")

				// Fallback to SendMessage if Reply fails
				_, fallbackErr := b.SendMessage(chat.Id, text, &gotgbot.SendMessageOpts{
					ReplyParameters: &gotgbot.ReplyParameters{
						MessageId:                msg.MessageId,
						AllowSendingWithoutReply: true,
					},
				})
				if fallbackErr != nil {
					log.WithFields(log.Fields{
						"chatId":        chat.Id,
						"userId":        userId,
						"replyError":    err,
						"fallbackError": fallbackErr,
					}).Error("RequireUserAdmin: Both Reply and SendMessage failed")
				}
			}
		}
		return false
	}
	return true
}

// RequireUserOwner ensures a user is the chat creator/owner.
// Checks for "creator" status specifically, not just administrator.
// If justCheck is false, sends error messages when user is not the creator.
func RequireUserOwner(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64, justCheck bool) bool {
	if chat == nil {
		if ctx.CallbackQuery != nil {
			_chatValue := ctx.CallbackQuery.Message.GetChat()
			chat = &_chatValue
		} else {
			chat = &ctx.Message.Chat
		}
	}

	msg := ctx.EffectiveMessage
	mem, err := chat.GetMember(b, userId, nil)
	error_handling.HandleErr(err)
	if mem == nil {
		return false
	}

	if mem.GetStatus() != "creator" {
		query := ctx.CallbackQuery
		if query != nil {
			if !justCheck {
				tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
				text, _ := tr.GetString("chat_status_owner_button_error")
				_, err := query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: text})
				if err != nil {
					log.Error(err)
					return false
				}
			}
			return false
		}
		if !justCheck {
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			text, _ := tr.GetString("chat_status_owner_cmd_error")
			_, err := msg.Reply(b, text, nil)
			error_handling.HandleErr(err)
		}
		return false
	}

	return true
}

// RequirePrivate ensures the command is being used in a private chat.
// Returns false for group chats and supergroups.
// If justCheck is false, sends error messages explaining the command is for private use only.
func RequirePrivate(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, justCheck bool) bool {
	if chat == nil {
		if ctx.CallbackQuery != nil {
			_chatValue := ctx.CallbackQuery.Message.GetChat()
			chat = &_chatValue
		} else if ctx.Message != nil {
			chat = &ctx.Message.Chat
		} else if ctx.MyChatMember != nil {
			chat = &ctx.MyChatMember.Chat
		} else {
			log.Error("RequirePrivate: No chat information available in context")
			return false
		}
	}
	msg := ctx.EffectiveMessage
	if chat.Type != "private" {
		if !justCheck {
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			text, _ := tr.GetString("chat_status_pm_only_error")
			_, err := msg.Reply(b, text,
				&gotgbot.SendMessageOpts{
					ReplyParameters: &gotgbot.ReplyParameters{
						MessageId: msg.MessageId,
					},
				},
			)
			error_handling.HandleErr(err)
		}
		return false
	}
	return true
}

// RequireGroup ensures the command is being used in a group chat.
// Returns false for private chats.
// If justCheck is false, sends error messages explaining the command is for group use only.
func RequireGroup(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, justCheck bool) bool {
	if chat == nil {
		if ctx.CallbackQuery != nil {
			_chatValue := ctx.CallbackQuery.Message.GetChat()
			chat = &_chatValue
		} else if ctx.Message != nil {
			chat = &ctx.Message.Chat
		} else if ctx.MyChatMember != nil {
			chat = &ctx.MyChatMember.Chat
		} else {
			log.Error("RequireGroup: No chat information available in context")
			return false
		}
	}
	msg := ctx.EffectiveMessage
	if chat.Type == "private" {
		if !justCheck {
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			text, _ := tr.GetString("chat_status_group_only_error")
			_, err := msg.Reply(b, text,
				&gotgbot.SendMessageOpts{
					ReplyParameters: &gotgbot.ReplyParameters{
						MessageId: msg.MessageId,
					},
				},
			)
			error_handling.HandleErr(err)
		}
		return false
	}
	return true
}

// setAnonAdminCache stores anonymous admin message information in cache.
// Used to track anonymous admin verification requests with expiration.
// Logs errors but doesn't fail since cache is non-critical.
func setAnonAdminCache(chatId int64, msg *gotgbot.Message) {
	err := cache.Marshal.Set(cache.Context, fmt.Sprintf("anonAdmin.%d.%d", chatId, msg.MessageId), msg, store.WithExpiration(anonChatMapExpirartion))
	if err != nil {
		// Log error but don't fail the operation since cache is not critical
		log.Errorf("Failed to set anonymous admin cache: %v", err)
	}
}
