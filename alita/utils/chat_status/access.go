package chat_status

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// hasUserPermission checks whether the specified user in a chat satisfies a
// permission predicate. It handles anonymous admin bypass and member lookup.
func hasUserPermission(
	b *gotgbot.Bot,
	ctx *ext.Context,
	chat *gotgbot.Chat,
	userId int64,
	requiredField func(*gotgbot.MergedChatMember) bool,
) bool {
	chat = extractChatFromContext(ctx, chat)
	if chat == nil {
		return false
	}

	msg := ctx.EffectiveMessage
	sender := ctx.EffectiveSender

	if isAdmin, shouldReturn := checkAnonAdmin(b, chat, msg, sender); shouldReturn {
		return isAdmin
	}

	userMember, ok := getUserMemberWithCache(b, chat, userId, "hasUserPermission")
	if !ok {
		return false
	}

	return requiredField(&userMember) || userMember.Status == "creator"
}

// canUserChangeInfo reports whether the user can change chat information.
func canUserChangeInfo(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64) bool {
	return hasUserPermission(b, ctx, chat, userId, func(m *gotgbot.MergedChatMember) bool {
		return m.CanChangeInfo
	})
}

// canUserRestrict reports whether the user can restrict other members.
func canUserRestrict(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64) bool {
	return hasUserPermission(b, ctx, chat, userId, func(m *gotgbot.MergedChatMember) bool {
		return m.CanRestrictMembers
	})
}

// canUserPromote reports whether the user can promote/demote other members.
func canUserPromote(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64) bool {
	return hasUserPermission(b, ctx, chat, userId, func(m *gotgbot.MergedChatMember) bool {
		return m.CanPromoteMembers
	})
}

// canUserPin reports whether the user can pin messages.
func canUserPin(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64) bool {
	return hasUserPermission(b, ctx, chat, userId, func(m *gotgbot.MergedChatMember) bool {
		return m.CanPinMessages
	})
}

// canUserDelete reports whether the user can delete messages.
func canUserDelete(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64) bool {
	return hasUserPermission(b, ctx, chat, userId, func(m *gotgbot.MergedChatMember) bool {
		return m.CanDeleteMessages
	})
}

// canBotRestrict reports whether the bot can restrict members.
func canBotRestrict(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool {
	chat = extractChatFromContext(ctx, chat)
	if chat == nil {
		return false
	}

	botMember, err := chat.GetMember(b, b.Id, nil)
	if err != nil {
		return false
	}
	return botMember.MergeChatMember().CanRestrictMembers
}

// canBotPromote reports whether the bot can promote/demote members.
func canBotPromote(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool {
	chat = extractChatFromContext(ctx, chat)
	if chat == nil {
		return false
	}

	botMember, err := chat.GetMember(b, b.Id, nil)
	if err != nil {
		return false
	}
	return botMember.MergeChatMember().CanPromoteMembers
}

// canBotPin reports whether the bot can pin messages.
func canBotPin(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool {
	chat = extractChatFromContext(ctx, chat)
	if chat == nil {
		return false
	}

	botMember, err := chat.GetMember(b, b.Id, nil)
	if err != nil {
		return false
	}
	return botMember.MergeChatMember().CanPinMessages
}

// canBotDelete reports whether the bot can delete messages.
func canBotDelete(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool {
	chat = extractChatFromContext(ctx, chat)
	if chat == nil {
		return false
	}

	botMember, err := chat.GetMember(b, b.Id, nil)
	if err != nil {
		return false
	}
	return botMember.MergeChatMember().CanDeleteMessages
}

// isBotAdminPure reports whether the bot is an admin, without sending messages.
func isBotAdminPure(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool {
	return IsBotAdmin(b, ctx, chat)
}

// isUserAdminPure reports whether the user is an admin, without sending messages.
func isUserAdminPure(b *gotgbot.Bot, chat *gotgbot.Chat, userId int64) bool {
	return IsUserAdmin(b, chat.Id, userId)
}

// requireBotAdminPure reports whether the bot is an admin.
func requireBotAdminPure(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool {
	return isBotAdminPure(b, ctx, chat)
}

// requireUserAdminPure reports whether the user is an admin.
func requireUserAdminPure(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64) bool {
	chat = extractChatFromContext(ctx, chat)
	if chat == nil {
		return false
	}
	return isUserAdminPure(b, chat, userId)
}

// requireUserOwnerPure reports whether the user is the chat owner.
func requireUserOwnerPure(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64) bool {
	chat = extractChatFromContext(ctx, chat)
	if chat == nil {
		return false
	}

	mem, err := chat.GetMember(b, userId, nil)
	if err != nil || mem == nil {
		return false
	}
	return mem.GetStatus() == "creator"
}

// requireGroupPure reports whether the chat is a group (not private).
func requireGroupPure(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool {
	chat = extractChatFromContext(ctx, chat)
	if chat == nil {
		return false
	}
	return chat.Type != "private"
}

// requirePrivatePure reports whether the chat is a private chat.
func requirePrivatePure(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool {
	chat = extractChatFromContext(ctx, chat)
	if chat == nil {
		return false
	}
	return chat.Type == "private"
}
