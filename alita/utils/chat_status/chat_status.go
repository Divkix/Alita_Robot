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

	"github.com/divkix/Alita_Robot/alita/db/admin"
	"github.com/divkix/Alita_Robot/alita/db/approvals"
	"github.com/divkix/Alita_Robot/alita/db/connections"
	"github.com/divkix/Alita_Robot/alita/db/disabling"
	"github.com/divkix/Alita_Robot/alita/db/lang"
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

// User IDs are always positive (> 0).
// Channel IDs are negative with format -100XXXXXXXXXX (< -1000000000000).
// Regular chat/group IDs are negative but in a different range.
func IsValidUserId(id int64) bool {
	return id > 0
}

// Channel IDs have the format -100XXXXXXXXXX (-100 prefix followed by 10+ digits).
func IsChannelId(id int64) bool {
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

func checkAnonAdmin(b *gotgbot.Bot, chat *gotgbot.Chat, msg *gotgbot.Message, sender *gotgbot.Sender) (isAdmin bool, shouldReturn bool) {
	if sender == nil || !sender.IsAnonymousAdmin() {
		return false, false
	}
	if admin.GetAdminSettings(chat.Id).AnonAdmin {
		return true, true
	}
	if msg == nil {
		return false, true
	}
	setAnonAdminCache(chat.Id, msg)
	_, err := sendAnonAdminKeyboard(b, msg, chat)
	if err != nil {
		log.Error(err)
	}
	return false, true
}

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

func GetChat(bot *gotgbot.Bot, chatId string) (*gotgbot.Chat, error) {
	r, err := bot.Request("getChat", map[string]any{"chat_id": chatId}, nil)
	if err != nil {
		return nil, err
	}

	var c gotgbot.Chat
	return &c, json.Unmarshal(r, &c)
}

func CheckDisabledCmd(bot *gotgbot.Bot, msg *gotgbot.Message, cmd string) bool {
	if msg.Chat.Type == "private" {
		return false
	}

	if !disabling.IsCommandDisabled(msg.Chat.Id, cmd) {
		return false
	}

	// msg.From can be nil for channel posts
	if msg.From == nil {
		return false
	}

	if IsUserAdmin(bot, msg.Chat.Id, msg.From.Id) {
		return false
	}

	if disabling.ShouldDel(msg.Chat.Id) {
		_, err := msg.Delete(bot, nil)
		if err != nil {
			log.Errorf("[CheckDisabledCmd] Failed to delete message for disabled command '%s' in chat %d: %v", cmd, msg.Chat.Id, err)
		}
	}

	return true
}

func IsApproved(b *gotgbot.Bot, chatID, userID int64) bool {
	return approvals.IsUserApproved(chatID, userID)
}

func IsUserAdmin(b *gotgbot.Bot, chatID, userId int64) bool {
	if !IsValidUserId(userId) {
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

	if slices.Contains(tgAdminList, userId) {
		return true
	}

	adminsAvail, admins := cache.GetAdminCacheList(chatID)
	if adminsAvail && admins.Cached {
		if admins.UserMap != nil {
			if admin, found := admins.UserMap[userId]; found && admin.User.Id != 0 {
				return true
			}
			return false
		}
		for i := range admins.UserInfo {
			if admins.UserInfo[i].User.Id == userId {
				return true
			}
		}
		return false
	}

	ctxGetChat, cancelGetChat := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelGetChat()
	chat, err := b.GetChatWithContext(ctxGetChat, chatID, nil)
	if err != nil {
		log.WithFields(log.Fields{
			"chatID": chatID,
			"userID": userId,
			"error":  err,
		}).Warning("IsUserAdmin: Failed to get chat, treating as non-admin")
		return false
	}

	if chat.Type != "group" && chat.Type != "supergroup" {
		return false
	}

	adminList := cache.LoadAdminCache(b, chatID)

	if adminList.UserMap != nil {
		if admin, found := adminList.UserMap[userId]; found && admin.User.Id != 0 {
			return true
		}
	} else {
		for i := range adminList.UserInfo {
			if adminList.UserInfo[i].User.Id == userId {
				return true
			}
		}
	}

	if len(adminList.UserInfo) == 0 {
		log.WithFields(log.Fields{
			"chatID": chatID,
			"userID": userId,
		}).Debug("IsUserAdmin: Admin cache empty, trying direct GetChatMember fallback")

		ctxMember, cancelMember := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelMember()
		member, err := b.GetChatMemberWithContext(ctxMember, chatID, userId, nil)
		if err != nil {
			if ctxMember.Err() != nil {
				log.WithFields(log.Fields{
					"chatID": chatID,
					"userID": userId,
				}).Warn("IsUserAdmin: GetChatMember fallback timed out, assuming non-admin")
				return false
			}
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

func IsBotAdmin(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool {
	chat = extractChatFromContext(ctx, chat)
	if chat == nil {
		log.Error("IsBotAdmin: No chat information available in context")
		return false
	}

	if chat.Type == "private" {
		return true
	}

	mem, ok := getUserMemberWithCache(b, chat, b.Id, "IsBotAdmin")
	if !ok {
		return false
	}

	return mem.Status == "administrator"
}

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

// IsUserInChatWithError reports definitive membership separately from Telegram
// lookup failures so callers do not destroy persisted state on transient errors.
func IsUserInChatWithError(b *gotgbot.Bot, chat *gotgbot.Chat, userId int64) (bool, error) {
	if b == nil || chat == nil || userId == tgUserId {
		return false, nil
	}
	member, err := chat.GetMember(b, userId, nil)
	if err != nil {
		return false, err
	}
	if member == nil {
		return false, nil
	}

	merged := member.MergeChatMember()
	switch merged.Status {
	case "creator", "administrator", "member":
		return true, nil
	case "restricted":
		return merged.IsMember, nil
	default:
		return false, nil
	}
}

func IsUserInChat(b *gotgbot.Bot, chat *gotgbot.Chat, userId int64) bool {
	isMember, err := IsUserInChatWithError(b, chat, userId)
	if err != nil {
		log.Errorf("[IsUserInChat] GetMember failed for user %d in chat %d: %v", userId, chat.Id, err)
		return false
	}
	return isMember
}

func IsUserConnected(b *gotgbot.Bot, ctx *ext.Context, chatAdmin, botAdmin bool) (chat *gotgbot.Chat) {
	msg := ctx.EffectiveMessage
	user := ctx.EffectiveUser
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	if msg == nil || user == nil {
		return nil
	}

	userAdminVerified := false
	respond := func(text string) {
		if query, ok := callbackQueryFromContext(ctx); ok {
			_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: text})
			return
		}
		_, _ = msg.Reply(b, text, nil)
	}

	if msg.Chat.Type == "private" {
		conn := connections.Connection(user.Id)
		if conn.Connected && conn.ChatId != 0 {
			disconnectStale := func() {
				key := "connections_stale_connection"
				if err := connections.DisconnectId(user.Id); err != nil {
					key = "error_generic"
				}
				text, _ := tr.GetString(key)
				respond(text)
			}

			chatFullInfo, err := b.GetChat(conn.ChatId, nil)
			if err != nil || chatFullInfo == nil {
				log.WithFields(log.Fields{
					"userId": user.Id,
					"chatId": conn.ChatId,
					"error":  err,
				}).Warn("Connected chat lookup failed")
				text, _ := tr.GetString("error_generic")
				respond(text)
				return nil
			}

			_chat := chatFullInfo.ToChat()
			if chatAdmin && IsUserAdmin(b, _chat.Id, user.Id) {
				userAdminVerified = true
			} else {
				isMember, err := IsUserInChatWithError(b, &_chat, user.Id)
				if err != nil {
					log.WithFields(log.Fields{
						"userId": user.Id,
						"chatId": conn.ChatId,
						"error":  err,
					}).Warn("Connected chat membership check failed")
					text, _ := tr.GetString("error_generic")
					respond(text)
					return nil
				}
				if !isMember {
					log.WithFields(log.Fields{
						"userId": user.Id,
						"chatId": conn.ChatId,
					}).Info("Stale connection detected - user is no longer a member")
					disconnectStale()
					return nil
				}
			}
			chat = &_chat
		} else {
			text, _ := tr.GetString("connections_is_user_connected_need_group")
			if query, ok := callbackQueryFromContext(ctx); ok {
				_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: text})
			} else {
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
				}
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
		if !userAdminVerified && !IsUserAdmin(b, chat.Id, user.Id) {
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

func setAnonAdminCache(chatId int64, msg *gotgbot.Message) {
	m := cache.GetMarshal()
	if m == nil || msg == nil {
		log.Debug("Skipping anonymous admin cache set: cache unavailable or message nil")
		return
	}
	err := m.Set(cache.Context, fmt.Sprintf("alita:anonAdmin:%d:%d", chatId, msg.MessageId), msg, store.WithExpiration(anonChatMapExpiration))
	if err != nil {
		log.Errorf("Failed to set anonymous admin cache: %v", err)
	}
}

// Returns nil for channel posts and cases where user is unavailable.
func GetEffectiveUser(ctx *ext.Context) *gotgbot.User {
	if ctx == nil || ctx.EffectiveSender == nil {
		return nil
	}
	return ctx.EffectiveSender.User
}

func RequireUser(b *gotgbot.Bot, ctx *ext.Context) *gotgbot.User {
	return GetEffectiveUser(ctx)
}

// NOTE: msg.GetLink() only works for supergroups/channels. This custom implementation
// also handles private groups and non-supergroups by constructing the link manually.
func GetMessageLinkFromMessageId(chat *gotgbot.Chat, messageId int64) (messageLink string) {
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
			linkId = strings.ReplaceAll(chatIdStr, "-", "")
		}
		messageLink += fmt.Sprintf("c/%s/%d", linkId, messageId)
	} else {
		messageLink += fmt.Sprintf("%s/%d", chat.Username, messageId)
	}
	return
}

func ExtractJoinLeftStatusChange(u *gotgbot.ChatMemberUpdated) (bool, bool) {
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

func ExtractAdminUpdateStatusChange(u *gotgbot.ChatMemberUpdated) bool {
	if u.Chat.Type == "channel" {
		return false
	}

	oldMemberStatus := u.OldChatMember.MergeChatMember().Status
	newMemberStatus := u.NewChatMember.MergeChatMember().Status

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

func sendAnonAdminKeyboard(b *gotgbot.Bot, msg *gotgbot.Message, chat *gotgbot.Chat) (*gotgbot.Message, error) {
	ctx := &ext.Context{
		EffectiveMessage: msg,
	}

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	mainText, _ := tr.GetString("chat_status_anon_confirm")
	buttonText, _ := tr.GetString("chat_status_anon_prove_admin")
	callbackData, err := callbackcodec.Encode("anon_admin", map[string]string{
		"c": fmt.Sprint(chat.Id),
		"m": fmt.Sprint(msg.MessageId),
	})
	if err != nil {
		return nil, fmt.Errorf("encode anonymous-admin callback: %w", err)
	}

	return msg.Reply(b,
		mainText,
		&gotgbot.SendMessageOpts{
			ReplyMarkup: gotgbot.InlineKeyboardMarkup{
				InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
					{{
						Text:         buttonText,
						CallbackData: callbackData,
					}},
				},
			},
		},
	)
}
