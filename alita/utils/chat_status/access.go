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
	if ctx == nil {
		return false
	}
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

// CanUserChangeInfo reports whether the user can change chat information.
func CanUserChangeInfo(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64) bool {
	return hasUserPermission(b, ctx, chat, userId, func(m *gotgbot.MergedChatMember) bool {
		return m.CanChangeInfo
	})
}

// CanUserRestrict reports whether the user can restrict other members.
func CanUserRestrict(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64) bool {
	return hasUserPermission(b, ctx, chat, userId, func(m *gotgbot.MergedChatMember) bool {
		return m.CanRestrictMembers
	})
}

// CanUserPromote reports whether the user can promote/demote other members.
func CanUserPromote(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64) bool {
	return hasUserPermission(b, ctx, chat, userId, func(m *gotgbot.MergedChatMember) bool {
		return m.CanPromoteMembers
	})
}

// CanUserPin reports whether the user can pin messages.
func CanUserPin(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64) bool {
	return hasUserPermission(b, ctx, chat, userId, func(m *gotgbot.MergedChatMember) bool {
		return m.CanPinMessages
	})
}

// CanUserDelete reports whether the user can delete messages.
func CanUserDelete(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64) bool {
	return hasUserPermission(b, ctx, chat, userId, func(m *gotgbot.MergedChatMember) bool {
		return m.CanDeleteMessages
	})
}

// CanUserInvite reports whether the user can invite members and manage join requests.
func CanUserInvite(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64) bool {
	return hasUserPermission(b, ctx, chat, userId, func(m *gotgbot.MergedChatMember) bool {
		return m.CanInviteUsers
	})
}

// CanBotRestrict reports whether the bot can restrict members.
func CanBotRestrict(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool {
	if b == nil {
		return false
	}
	chat = extractChatFromContext(ctx, chat)
	if chat == nil {
		return false
	}

	botMember, ok := getUserMemberWithCache(b, chat, b.Id, "canBotRestrict")
	if !ok {
		return false
	}
	return botMember.CanRestrictMembers
}

// CanBotPromote reports whether the bot can promote/demote members.
func CanBotPromote(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool {
	if b == nil {
		return false
	}
	chat = extractChatFromContext(ctx, chat)
	if chat == nil {
		return false
	}

	botMember, ok := getUserMemberWithCache(b, chat, b.Id, "canBotPromote")
	if !ok {
		return false
	}
	return botMember.CanPromoteMembers
}

// CanBotPin reports whether the bot can pin messages.
func CanBotPin(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool {
	if b == nil {
		return false
	}
	chat = extractChatFromContext(ctx, chat)
	if chat == nil {
		return false
	}

	botMember, ok := getUserMemberWithCache(b, chat, b.Id, "canBotPin")
	if !ok {
		return false
	}
	return botMember.CanPinMessages
}

// CanBotDelete reports whether the bot can delete messages.
func CanBotDelete(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool {
	if b == nil {
		return false
	}
	chat = extractChatFromContext(ctx, chat)
	if chat == nil {
		return false
	}

	botMember, ok := getUserMemberWithCache(b, chat, b.Id, "canBotDelete")
	if !ok {
		return false
	}
	return botMember.CanDeleteMessages
}

// CanBotInvite reports whether the bot can invite members and manage join requests.
func CanBotInvite(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool {
	if b == nil {
		return false
	}
	chat = extractChatFromContext(ctx, chat)
	if chat == nil {
		return false
	}

	botMember, ok := getUserMemberWithCache(b, chat, b.Id, "canBotInvite")
	if !ok {
		return false
	}
	return botMember.CanInviteUsers
}

// RequireBotAdmin reports whether the bot is an admin.
func RequireBotAdmin(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool {
	return IsBotAdmin(b, ctx, chat)
}

// RequireUserAdmin reports whether the user is an admin.
func RequireUserAdmin(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64) bool {
	chat = extractChatFromContext(ctx, chat)
	if chat == nil {
		return false
	}
	return IsUserAdmin(b, chat.Id, userId)
}

// RequireUserOwner reports whether the user is the chat owner.
func RequireUserOwner(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64) bool {
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

// RequireGroup reports whether the chat is a group (not private).
//
//nolint:dupl // RequirePrivate/RequireGroup have symmetric logic
func RequireGroup(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool {
	chat = extractChatFromContext(ctx, chat)
	if chat == nil {
		return false
	}
	return chat.Type != "private"
}

// RequirePrivate reports whether the chat is a private chat.
//
//nolint:dupl // RequirePrivate/RequireGroup have symmetric logic
func RequirePrivate(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool {
	chat = extractChatFromContext(ctx, chat)
	if chat == nil {
		return false
	}
	return chat.Type == "private"
}
