package chat_status

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

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

func CanUserChangeInfo(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64) bool {
	return hasUserPermission(b, ctx, chat, userId, func(m *gotgbot.MergedChatMember) bool {
		return m.CanChangeInfo
	})
}

func CanUserRestrict(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64) bool {
	return hasUserPermission(b, ctx, chat, userId, func(m *gotgbot.MergedChatMember) bool {
		return m.CanRestrictMembers
	})
}

func CanUserPromote(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64) bool {
	return hasUserPermission(b, ctx, chat, userId, func(m *gotgbot.MergedChatMember) bool {
		return m.CanPromoteMembers
	})
}

func CanUserPin(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64) bool {
	return hasUserPermission(b, ctx, chat, userId, func(m *gotgbot.MergedChatMember) bool {
		return m.CanPinMessages
	})
}

func CanUserDelete(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64) bool {
	return hasUserPermission(b, ctx, chat, userId, func(m *gotgbot.MergedChatMember) bool {
		return m.CanDeleteMessages
	})
}

func CanUserInvite(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64) bool {
	return hasUserPermission(b, ctx, chat, userId, func(m *gotgbot.MergedChatMember) bool {
		return m.CanInviteUsers
	})
}

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

func CanBotChangeInfo(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool {
	if b == nil {
		return false
	}
	chat = extractChatFromContext(ctx, chat)
	if chat == nil {
		return false
	}

	botMember, ok := getUserMemberWithCache(b, chat, b.Id, "canBotChangeInfo")
	if !ok {
		return false
	}
	return botMember.CanChangeInfo
}

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

func RequireBotAdmin(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool {
	return IsBotAdmin(b, ctx, chat)
}

func RequireUserAdmin(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64) bool {
	chat = extractChatFromContext(ctx, chat)
	if chat == nil {
		return false
	}
	return IsUserAdmin(b, chat.Id, userId)
}

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

//nolint:dupl // RequirePrivate/RequireGroup have symmetric logic
func RequireGroup(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool {
	chat = extractChatFromContext(ctx, chat)
	if chat == nil {
		return false
	}
	return chat.Type != "private"
}

//nolint:dupl // RequirePrivate/RequireGroup have symmetric logic
func RequirePrivate(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool {
	chat = extractChatFromContext(ctx, chat)
	if chat == nil {
		return false
	}
	return chat.Type == "private"
}
