package chat_status

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/utils/tracing"
	"github.com/divkix/Alita_Robot/alita/utils/updatememo"
)

type (
	adminMemoKey     struct{ chatID, userID int64 }
	approvedMemoKey  struct{ chatID, userID int64 }
	memberMemoKey    struct{ chatID, userID int64 }
	protectedMemoKey struct{ chatID, userID, senderID int64 }
)

type memberLookup struct {
	member gotgbot.MergedChatMember
	ok     bool
}

func getUserMemberWithCacheForUpdate(
	b *gotgbot.Bot,
	ctx *ext.Context,
	chat *gotgbot.Chat,
	userId int64,
	funcName string,
) (gotgbot.MergedChatMember, bool) {
	lookup := updatememo.Get(updatememo.From(ctx), memberMemoKey{chat.Id, userId}, func() memberLookup {
		member, ok := getUserMemberWithCache(tracing.UpdateContext(ctx), b, chat, userId, funcName)
		return memberLookup{member: member, ok: ok}
	})
	return lookup.member, lookup.ok
}

// IsBotAdminForUpdate is IsBotAdmin memoised for the current update.
func IsBotAdminForUpdate(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool {
	chat = extractChatFromContext(ctx, chat)
	if chat == nil {
		return false
	}
	if chat.Type == "private" {
		return true
	}
	if b == nil {
		return false
	}
	mem, ok := getUserMemberWithCacheForUpdate(b, ctx, chat, b.Id, "IsBotAdmin")
	if !ok {
		return false
	}
	return mem.Status == "administrator"
}

// UserHasPermissionForUpdate is hasUserPermission memoised for the current update.
func UserHasPermissionForUpdate(
	b *gotgbot.Bot,
	ctx *ext.Context,
	chat *gotgbot.Chat,
	userId int64,
	requiredField func(*gotgbot.MergedChatMember) bool,
) bool {
	if ctx == nil {
		return false
	}
	chat = extractChatFromContext(ctx, chat)
	if chat == nil {
		return false
	}

	if isAdmin, shouldReturn := checkAnonAdmin(b, chat, ctx.EffectiveMessage, ctx.EffectiveSender); shouldReturn {
		return isAdmin
	}
	if isConfirmedAnonymousAdmin(ctx, chat.Id, userId) {
		return true
	}

	userMember, ok := getUserMemberWithCacheForUpdate(b, ctx, chat, userId, "hasUserPermission")
	if !ok {
		return false
	}
	return requiredField(&userMember) || userMember.Status == "creator"
}

// BotHasPermissionForUpdate is hasBotPermission memoised for the current update.
func BotHasPermissionForUpdate(
	b *gotgbot.Bot,
	ctx *ext.Context,
	chat *gotgbot.Chat,
	requiredField func(*gotgbot.MergedChatMember) bool,
) bool {
	if b == nil {
		return false
	}
	chat = extractChatFromContext(ctx, chat)
	if chat == nil {
		return false
	}
	botMember, ok := getUserMemberWithCacheForUpdate(b, ctx, chat, b.Id, "hasBotPermission")
	if !ok {
		return false
	}
	return requiredField(&botMember)
}

// CanBotDeleteForUpdate is CanBotDelete memoised for the current update.
func CanBotDeleteForUpdate(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool {
	return BotHasPermissionForUpdate(b, ctx, chat, func(m *gotgbot.MergedChatMember) bool {
		return m.CanDeleteMessages
	})
}

// IsUserAdminForUpdate is IsUserAdmin memoised for the current update. Use it
// only in watchers and command-pipeline checks, which run before the update
// changes any state: the result is not refreshed afterwards.
func IsUserAdminForUpdate(b *gotgbot.Bot, ctx *ext.Context, chatID, userID int64) bool {
	return updatememo.Get(updatememo.From(ctx), adminMemoKey{chatID, userID}, func() bool {
		return isUserAdminForContext(b, ctx, chatID, userID)
	})
}

// IsApprovedForUpdate is IsApproved memoised for the current update. Use it
// only in watchers: the result is not refreshed if the update itself changes approvals.
func IsApprovedForUpdate(b *gotgbot.Bot, ctx *ext.Context, chatID, userID int64) bool {
	return updatememo.Get(updatememo.From(ctx), approvedMemoKey{chatID, userID}, func() bool {
		return IsApproved(b, chatID, userID)
	})
}

// IsProtectedForUpdate reports whether userID is exempt from the automatic
// checks a group runs on messages and joins, memoised for the current update.
// It owns the canonical exemption order: system accounts and non-positive
// IDs, then the update's own anonymous or channel sender, then admins, then
// approved users. The sender steps apply only when userID is the update's
// sender; checks on another user (a join member, a reported author) skip
// them. Like the other *ForUpdate predicates it sends nothing: call it in
// watchers and pipeline checks, before any state change in the same update.
func IsProtectedForUpdate(b *gotgbot.Bot, ctx *ext.Context, chatID, userID int64) bool {
	var sender *gotgbot.Sender
	if ctx != nil {
		sender = ctx.EffectiveSender
	}
	var senderID int64
	if sender != nil {
		senderID = sender.Id()
	}
	return updatememo.Get(updatememo.From(ctx), protectedMemoKey{chatID, userID, senderID}, func() bool {
		if userID <= 0 || IsSystemUser(userID) {
			return true
		}
		if sender != nil && senderID == userID &&
			(sender.IsBot() || sender.IsAnonymousAdmin() || sender.IsAnonymousChannel() || sender.IsLinkedChannel()) {
			return true
		}
		return IsUserAdminForUpdate(b, ctx, chatID, userID) || IsApprovedForUpdate(b, ctx, chatID, userID)
	})
}
