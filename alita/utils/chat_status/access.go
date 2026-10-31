package chat_status

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/utils/tracing"
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

	userMember, ok := getUserMemberWithCache(tracing.UpdateContext(ctx), b, chat, userId, "hasUserPermission")
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

func hasBotPermission(
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

	botMember, ok := getUserMemberWithCache(tracing.UpdateContext(ctx), b, chat, b.Id, "hasBotPermission")
	if !ok {
		return false
	}
	return requiredField(&botMember)
}

func CanBotRestrict(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool {
	return hasBotPermission(b, ctx, chat, func(m *gotgbot.MergedChatMember) bool {
		return m.CanRestrictMembers
	})
}

func CanBotPromote(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool {
	return hasBotPermission(b, ctx, chat, func(m *gotgbot.MergedChatMember) bool {
		return m.CanPromoteMembers
	})
}

func CanBotPin(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool {
	return hasBotPermission(b, ctx, chat, func(m *gotgbot.MergedChatMember) bool {
		return m.CanPinMessages
	})
}

func CanBotChangeInfo(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool {
	return hasBotPermission(b, ctx, chat, func(m *gotgbot.MergedChatMember) bool {
		return m.CanChangeInfo
	})
}

func CanBotDelete(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool {
	return hasBotPermission(b, ctx, chat, func(m *gotgbot.MergedChatMember) bool {
		return m.CanDeleteMessages
	})
}

func CanBotInvite(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool {
	return hasBotPermission(b, ctx, chat, func(m *gotgbot.MergedChatMember) bool {
		return m.CanInviteUsers
	})
}

func RequireBotAdmin(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool {
	return IsBotAdmin(b, ctx, chat)
}

func RequireUserAdmin(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64) bool {
	chat = extractChatFromContext(ctx, chat)
	if chat == nil {
		return false
	}
	return isUserAdminForContext(b, ctx, chat.Id, userId)
}

func RequireUserOwner(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, userId int64) bool {
	chat = extractChatFromContext(ctx, chat)
	if chat == nil {
		return false
	}

	mem, err := getChatMember(tracing.UpdateContext(ctx), b, chat.Id, userId)
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
