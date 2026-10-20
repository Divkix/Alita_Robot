package chat_status

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/utils/updatememo"
)

type (
	adminMemoKey    struct{ chatID, userID int64 }
	approvedMemoKey struct{ chatID, userID int64 }
	memberMemoKey   struct{ chatID, userID int64 }
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
		member, ok := getUserMemberWithCache(b, chat, userId, funcName)
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

// IsUserAdminForUpdate is IsUserAdmin memoised for the current update. Use it
// only in watchers and command-pipeline checks, which run before the update
// changes any state: the result is not refreshed afterwards.
func IsUserAdminForUpdate(b *gotgbot.Bot, ctx *ext.Context, chatID, userID int64) bool {
	return updatememo.Get(updatememo.From(ctx), adminMemoKey{chatID, userID}, func() bool {
		return IsUserAdmin(b, chatID, userID)
	})
}

// IsApprovedForUpdate is IsApproved memoised for the current update. Use it
// only in watchers: the result is not refreshed if the update itself changes approvals.
func IsApprovedForUpdate(b *gotgbot.Bot, ctx *ext.Context, chatID, userID int64) bool {
	return updatememo.Get(updatememo.From(ctx), approvedMemoKey{chatID, userID}, func() bool {
		return IsApproved(b, chatID, userID)
	})
}
