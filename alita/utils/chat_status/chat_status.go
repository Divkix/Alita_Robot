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
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/tracing"
)

// 1087968824 - Group Anonymous Bot (For anonymous users)
// 777000 - Telegram
// 136817688 - SendAsChannel Bot (For users that send messages as channel)
const (
	GroupAnonymousBotID int64 = 1087968824
	TelegramUserID      int64 = 777000
	SendAsChannelBotID  int64 = 136817688
)

// IsSystemUser reports whether id belongs to one of Telegram's service
// accounts: GroupAnonymousBot, Telegram itself, or SendAsChannelBot.
func IsSystemUser(id int64) bool {
	switch id {
	case GroupAnonymousBotID, TelegramUserID, SendAsChannelBotID:
		return true
	default:
		return false
	}
}

var (
	tgAdminList           = []int64{GroupAnonymousBotID, TelegramUserID}
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

func CallbackQueryFromContext(ctx *ext.Context) (*gotgbot.CallbackQuery, bool) {
	if ctx == nil {
		return nil, false
	}
	update := ctx.Update
	if update == nil || update.CallbackQuery == nil {
		return nil, false
	}
	return update.CallbackQuery, true
}

const confirmedAnonymousAdminDataKey = "confirmedAnonymousAdmin"

type confirmedAnonymousAdmin struct {
	chatID, userID int64
}

// SetConfirmedAnonymousAdmin records the actor for command replay without
// replacing the original anonymous message sender. Call only after verifying
// the confirmation callback and retrieving its original command. Real admins
// still need their own rights; only the existing service-admin shortcut skips
// a member lookup, and confirmation never grants ownership.
func SetConfirmedAnonymousAdmin(ctx *ext.Context, chatID, userID int64) {
	if ctx == nil || !IsValidUserId(userID) || (IsSystemUser(userID) && !slices.Contains(tgAdminList, userID)) {
		return
	}
	if ctx.Data == nil {
		ctx.Data = make(map[string]any)
	}
	ctx.Data[confirmedAnonymousAdminDataKey] = confirmedAnonymousAdmin{chatID, userID}
}

func confirmedAnonymousAdminUser(ctx *ext.Context, chatID int64) *gotgbot.User {
	if ctx == nil || ctx.EffectiveUser == nil {
		return nil
	}
	proof, ok := ctx.Data[confirmedAnonymousAdminDataKey].(confirmedAnonymousAdmin)
	if !ok || proof.chatID != chatID || proof.userID != ctx.EffectiveUser.Id {
		return nil
	}
	return ctx.EffectiveUser
}

func isConfirmedAnonymousAdmin(ctx *ext.Context, chatID, userID int64) bool {
	user := confirmedAnonymousAdminUser(ctx, chatID)
	return user != nil && user.Id == userID && slices.Contains(tgAdminList, userID)
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

// extractChatFromContext returns chat if set, else the chat the update came
// from. A PM command acting on a connected group points ctx.EffectiveChat at
// that group; when EffectiveChat differs from the update's chat (gotgbot sets
// them equal otherwise), the group is returned so checks run there.
func extractChatFromContext(ctx *ext.Context, chat *gotgbot.Chat) *gotgbot.Chat {
	if chat != nil {
		return chat
	}
	if ctx == nil {
		return nil
	}
	updateChat := chatFromUpdate(ctx.Update)
	if updateChat != nil && ctx.EffectiveChat != nil && ctx.EffectiveChat.Id != updateChat.Id {
		return ctx.EffectiveChat
	}
	return updateChat
}

func chatFromUpdate(update *gotgbot.Update) *gotgbot.Chat {
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

// Synthetic sender users identify messages sent on behalf of chats, not
// queryable members. Anonymous-admin authorization remains in checkAnonAdmin
// and the existing tgAdminList shortcut, never in a membership lookup.
func getChatMember(ctx context.Context, b *gotgbot.Bot, chatID, userID int64) (gotgbot.ChatMember, error) {
	if b == nil || !IsValidUserId(userID) || IsSystemUser(userID) {
		return nil, nil
	}
	member, err := b.GetChatMemberWithContext(ctx, chatID, userID, nil)
	if error_handling.IsMissingChatMemberError(err) {
		return nil, nil
	}
	return member, err
}

func getUserMemberWithCache(ctx context.Context, b *gotgbot.Bot, chat *gotgbot.Chat, userId int64, funcName string) (gotgbot.MergedChatMember, bool) {
	if b == nil || chat == nil || !IsValidUserId(userId) || IsSystemUser(userId) {
		return gotgbot.MergedChatMember{}, false
	}
	found, userMember := cache.GetAdminCacheUser(chat.Id, userId)
	if found {
		return userMember, true
	}
	tmpUserMember, err := getChatMember(ctx, b, chat.Id, userId)
	if err != nil {
		log.WithContext(ctx).Errorf("[%s] GetMember failed for user %d in chat %d: %v", funcName, userId, chat.Id, err)
		return gotgbot.MergedChatMember{}, false
	}
	if tmpUserMember == nil {
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

// IsUserAdminCached answers IsUserAdmin from the stored admin list without
// calling Telegram. known is false when the list is missing, negative or empty;
// callers that need an answer then must fall back to IsUserAdmin.
func IsUserAdminCached(chatID, userId int64) (isAdmin, known bool) {
	isAdmin, known, _ = userAdminFromCache(chatID, userId)
	return isAdmin, known
}

// userAdminFromCache is IsUserAdminCached that also reports whether the stored
// list holds no admins, so IsUserAdminWithContext reads the list only once.
func userAdminFromCache(chatID, userId int64) (isAdmin, known, noAdmins bool) {
	if !IsValidUserId(userId) {
		return false, true, false
	}
	if slices.Contains(tgAdminList, userId) {
		return true, true, false
	}
	if IsSystemUser(userId) {
		return false, true, false
	}
	adminsAvail, admins := cache.GetAdminCacheList(chatID)
	if !adminsAvail || !admins.Cached || admins.Negative || len(admins.UserInfo) == 0 {
		return false, false, adminsAvail && admins.Cached && len(admins.UserInfo) == 0
	}
	for i := range admins.UserInfo {
		if admins.UserInfo[i].User.Id == userId {
			return true, true, false
		}
	}
	return false, true, false
}

func IsUserAdmin(b *gotgbot.Bot, chatID, userId int64) bool {
	isAdmin, err := IsUserAdminWithContext(context.Background(), b, chatID, userId)
	cache.RethrowAdminLoadPanic(err)
	return isAdmin
}

func isUserAdminForContext(b *gotgbot.Bot, ctx *ext.Context, chatID, userID int64) bool {
	isAdmin, err := IsUserAdminWithContext(tracing.UpdateContext(ctx), b, chatID, userID)
	cache.RethrowAdminLoadPanic(err)
	return isAdmin
}

// IsUserAdminWithContext is IsUserAdmin bounded by ctx: its Telegram calls and
// the wait for the shared admin-list load end with ctx. It then returns false
// with ctx.Err(), so callers can tell an unfinished check from a non-admin;
// nothing is cached for the unfinished check. A panic in the admin-list load
// comes back as a *cache.AdminLoadPanicError.
func IsUserAdminWithContext(ctx context.Context, b *gotgbot.Bot, chatID, userId int64) (bool, error) {
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
		return false, nil
	}

	isAdmin, known, noAdmins := userAdminFromCache(chatID, userId)
	if known {
		return isAdmin, nil
	}
	// A stored list without admins (the bot is not an admin there, or could
	// not check its own status) cannot answer, and reloading it before it
	// expires would only store it again: ask for the user's membership instead.
	// Only groups (negative IDs) qualify; a private chat still goes through
	// getChat below, which answers false for it.
	if chatID < 0 && noAdmins {
		return isUserAdminByMember(ctx, b, chatID, userId)
	}

	ctxGetChat, cancelGetChat := context.WithTimeout(ctx, 5*time.Second)
	defer cancelGetChat()
	chat, err := b.GetChatWithContext(ctxGetChat, chatID, nil)
	if err != nil {
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		log.WithFields(log.Fields{
			"chatID": chatID,
			"userID": userId,
			"error":  err,
		}).Warning("IsUserAdmin: Failed to get chat, treating as non-admin")
		return false, nil
	}

	if chat.Type != "group" && chat.Type != "supergroup" {
		return false, nil
	}

	adminList, err := cache.LoadAdminCacheWithContext(ctx, b, chatID)
	if err != nil {
		return false, err
	}

	for i := range adminList.UserInfo {
		if adminList.UserInfo[i].User.Id == userId {
			return true, nil
		}
	}

	if len(adminList.UserInfo) == 0 {
		return isUserAdminByMember(ctx, b, chatID, userId)
	}

	return false, nil
}

// isUserAdminByMember asks Telegram for the user's own membership when the
// admin list is empty (for example when the bot cannot read it).
func isUserAdminByMember(ctx context.Context, b *gotgbot.Bot, chatID, userId int64) (bool, error) {
	log.WithFields(log.Fields{
		"chatID": chatID,
		"userID": userId,
	}).Debug("IsUserAdmin: Admin cache empty, trying direct GetChatMember fallback")

	ctxMember, cancelMember := context.WithTimeout(ctx, 5*time.Second)
	defer cancelMember()
	member, err := getChatMember(ctxMember, b, chatID, userId)
	if err != nil {
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		if ctxMember.Err() != nil {
			log.WithFields(log.Fields{
				"chatID": chatID,
				"userID": userId,
			}).Warn("IsUserAdmin: GetChatMember fallback timed out, assuming non-admin")
			return false, nil
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
		return false, nil
	}
	if member == nil {
		return false, nil
	}

	status := member.GetStatus()
	isAdmin := status == "administrator" || status == "creator"

	log.WithFields(log.Fields{
		"chatID":  chatID,
		"userID":  userId,
		"status":  status,
		"isAdmin": isAdmin,
	}).Debug("IsUserAdmin: Used fallback GetChatMember")

	return isAdmin, nil
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
	if b == nil {
		return false
	}

	mem, ok := getUserMemberWithCache(tracing.UpdateContext(ctx), b, chat, b.Id, "IsBotAdmin")
	if !ok {
		return false
	}

	return mem.Status == "administrator"
}

// CanInvite reports whether the invite link may be shared: the chat is public,
// or the bot and the sender both hold the invite right.
func CanInvite(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, msg *gotgbot.Message) bool {
	allowed, _ := CanInviteAnswered(b, ctx, chat, msg)
	return allowed
}

// CanInviteAnswered is CanInvite that also reports whether the check already
// answered the sender: answered is true only when an anonymous admin was sent
// the proof keyboard, so the caller must not send a refusal on top of it.
func CanInviteAnswered(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, msg *gotgbot.Message) (allowed, answered bool) {
	chat = extractChatFromContext(ctx, chat)
	if chat == nil {
		log.Error("CanInvite: No chat information available in context")
		return false, false
	}
	if chat.Username != "" {
		return true, false
	}
	if b == nil || ctx == nil {
		return false, false
	}
	var user *gotgbot.User
	if msg != nil {
		user = msg.From
	}
	if confirmed := confirmedAnonymousAdminUser(ctx, chat.Id); confirmed != nil {
		user = confirmed
	}
	sender := ctx.EffectiveSender
	if sender == nil || !sender.IsAnonymousAdmin() {
		if user == nil || !IsValidUserId(user.Id) ||
			(IsSystemUser(user.Id) && !isConfirmedAnonymousAdmin(ctx, chat.Id, user.Id)) {
			return false, false
		}
	}
	botChatMember, err := getChatMember(tracing.UpdateContext(ctx), b, chat.Id, b.Id)
	if err != nil {
		log.WithContext(tracing.UpdateContext(ctx)).Errorf("[CanInvite] GetMember failed for bot in chat %d: %v", chat.Id, err)
		return false, false
	}
	if botChatMember == nil {
		return false, false
	}
	if !botChatMember.MergeChatMember().CanInviteUsers {
		return false, false
	}

	if isAdmin, shouldReturn := checkAnonAdmin(b, chat, msg, sender); shouldReturn {
		return isAdmin, !isAdmin && msg != nil
	}
	if ctx.EffectiveUser != nil && isConfirmedAnonymousAdmin(ctx, chat.Id, ctx.EffectiveUser.Id) {
		return true, false
	}

	// The original message sender stays anonymous during confirmed replay.
	if user == nil {
		return false, false
	}

	userMember, ok := getUserMemberWithCache(tracing.UpdateContext(ctx), b, chat, user.Id, "CanInvite")
	if !ok {
		return false, false
	}

	if !userMember.CanInviteUsers && userMember.Status != "creator" {
		return false, false
	}
	return true, false
}

// IsUserInChatWithError reports definitive membership separately from Telegram
// lookup failures so callers do not destroy persisted state on transient errors.
func IsUserInChatWithError(b *gotgbot.Bot, chat *gotgbot.Chat, userId int64) (bool, error) {
	if chat == nil {
		return false, nil
	}
	member, err := getChatMember(context.Background(), b, chat.Id, userId)
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
		if query, ok := CallbackQueryFromContext(ctx); ok {
			_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: text})
			return
		}
		_, _ = msg.Reply(b, text, nil)
	}

	if oneOff := OneOffGroup(ctx); msg.Chat.Type == "private" && oneOff != nil {
		// The group named in the command passed the /connect checks and one
		// live membership lookup before it was set; only the chatAdmin and
		// botAdmin checks below remain, and a failed one leaves the saved
		// connection alone.
		chat = oneOff
	} else if msg.Chat.Type == "private" {
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
			if chatAdmin && isUserAdminForContext(b, ctx, _chat.Id, user.Id) {
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
			if query, ok := CallbackQueryFromContext(ctx); ok {
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
		if !isUserAdminForContext(b, ctx, chat.Id, b.Id) {
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
		if !userAdminVerified && !isUserAdminForContext(b, ctx, chat.Id, user.Id) {
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

	return isUserAdminForContext(b, ctx, chat.Id, userId) || slices.Contains(tgAdminList, userId)
}

// AnonAdminCacheKey is the operational Redis key holding the message behind an
// anonymous admin's confirmation button.
func AnonAdminCacheKey(chatId, msgId int64) string {
	return fmt.Sprintf("alita:anonAdmin:%d:%d", chatId, msgId)
}

func setAnonAdminCache(chatId int64, msg *gotgbot.Message) {
	m := cache.GetMarshal()
	if m == nil || msg == nil {
		log.Debug("Skipping anonymous admin cache set: cache unavailable or message nil")
		return
	}
	err := m.Set(cache.Context, AnonAdminCacheKey(chatId, msg.MessageId), msg, store.WithExpiration(anonChatMapExpiration))
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
	if u == nil || u.Chat.Type == "channel" || u.OldChatMember == nil || u.NewChatMember == nil {
		return false
	}

	oldMemberStatus := u.OldChatMember.MergeChatMember().Status
	newMemberStatus := u.NewChatMember.MergeChatMember().Status

	return oldMemberStatus == "administrator" || oldMemberStatus == "creator" ||
		newMemberStatus == "administrator" || newMemberStatus == "creator"
}

func sendAnonAdminKeyboard(b *gotgbot.Bot, msg *gotgbot.Message, chat *gotgbot.Chat) (*gotgbot.Message, error) {
	// lang.GetLanguage answers "en" without a chat, so pass the chat the
	// message was sent in.
	ctx := &ext.Context{
		EffectiveMessage: msg,
		EffectiveChat:    &msg.Chat,
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
