package helpers

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"

	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
)

type CommandContext struct {
	Bot  *gotgbot.Bot
	Ctx  *ext.Context
	Chat *gotgbot.Chat
	Msg  *gotgbot.Message
	User *gotgbot.User
	Tr   *i18n.Translator
}

func BuildCommandContext(b *gotgbot.Bot, ctx *ext.Context) (*CommandContext, error) {
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		if ctx != nil && ctx.EffectiveMessage != nil {
			chat_status.NewPermissionResponder(b).Respond(ctx, "common_cannot_identify_user", "", chat_status.WithReply())
		}
		return nil, ext.EndGroups
	}
	return &CommandContext{
		Bot:  b,
		Ctx:  ctx,
		Chat: ctx.EffectiveChat,
		Msg:  ctx.EffectiveMessage,
		User: user,
		Tr:   i18n.MustNewTranslator(lang.GetLanguage(ctx)),
	}, nil
}

type CheckFunc func(c *CommandContext) bool

type CommandDescriptor struct {
	Name           string
	Aliases        []string
	Group          int
	RequiredChecks []CheckFunc
	Disableable    bool
}

func WrapCommand(
	dispatcher *ext.Dispatcher,
	desc CommandDescriptor,
	handler func(c *CommandContext) error,
) {
	h := func(b *gotgbot.Bot, ctx *ext.Context) error {
		defer error_handling.RecoverFromPanic("command_pipeline", "WrapCommand")
		c, err := BuildCommandContext(b, ctx)
		if err != nil {
			return ext.EndGroups
		}
		for _, check := range desc.RequiredChecks {
			if !check(c) {
				return ext.EndGroups
			}
		}
		return handler(c)
	}
	register(dispatcher, desc, h)
}

func register(dispatcher *ext.Dispatcher, desc CommandDescriptor, h handlers.Response) {
	cmds := append([]string{desc.Name}, desc.Aliases...)
	for _, c := range cmds {
		if desc.Group != 0 {
			dispatcher.AddHandlerToGroup(handlers.NewCommand(c, h), desc.Group)
		} else {
			dispatcher.AddHandler(handlers.NewCommand(c, h))
		}
		if desc.Disableable {
			AddCmdToDisableable(c)
		}
	}
}

func CheckDisabled(cmdName string) CheckFunc {
	return func(c *CommandContext) bool {
		if c.Msg == nil || c.Bot == nil {
			return false
		}
		return !chat_status.CheckDisabledCmd(c.Bot, c.Msg, cmdName)
	}
}

func RequireGroup() CheckFunc {
	return func(c *CommandContext) bool {
		result := chat_status.RequireGroup(c.Bot, c.Ctx, nil)
		if !result {
			chat_status.NewPermissionResponder(c.Bot).Respond(c.Ctx, "chat_status_group_only_error", "", chat_status.WithReply())
		}
		return result
	}
}

func RequireBotAdmin() CheckFunc {
	return func(c *CommandContext) bool {
		result := chat_status.RequireBotAdmin(c.Bot, c.Ctx, nil)
		if !result {
			chat_status.NewPermissionResponder(c.Bot).Respond(c.Ctx, "chat_status_bot_not_admin", "", chat_status.WithReply())
		}
		return result
	}
}

func RequireUserAdmin() CheckFunc {
	return func(c *CommandContext) bool {
		if c.User == nil {
			return false
		}
		result := chat_status.RequireUserAdmin(c.Bot, c.Ctx, nil, c.User.Id)
		if !result {
			chat_status.NewPermissionResponder(c.Bot).Respond(c.Ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
		}
		return result
	}
}

func CanUserPromote() CheckFunc {
	return func(c *CommandContext) bool {
		if c.User == nil {
			return false
		}
		result := chat_status.CanUserPromote(c.Bot, c.Ctx, nil, c.User.Id)
		if !result {
			chat_status.NewPermissionResponder(c.Bot).Respond(c.Ctx, "chat_status_promote_cmd_error", "chat_status_promote_button_error")
		}
		return result
	}
}

func CanBotPromote() CheckFunc {
	return func(c *CommandContext) bool {
		result := chat_status.CanBotPromote(c.Bot, c.Ctx, nil)
		if !result {
			chat_status.NewPermissionResponder(c.Bot).Respond(c.Ctx, "chat_status_bot_promote_error", "")
		}
		return result
	}
}

func CanUserPin() CheckFunc {
	return func(c *CommandContext) bool {
		if c.User == nil {
			return false
		}
		result := chat_status.CanUserPin(c.Bot, c.Ctx, nil, c.User.Id)
		if !result {
			chat_status.NewPermissionResponder(c.Bot).Respond(c.Ctx, "chat_status_pin_user_error", "")
		}
		return result
	}
}

func CanBotPin() CheckFunc {
	return func(c *CommandContext) bool {
		result := chat_status.CanBotPin(c.Bot, c.Ctx, nil)
		if !result {
			chat_status.NewPermissionResponder(c.Bot).Respond(c.Ctx, "chat_status_pin_bot_error", "")
		}
		return result
	}
}

func CanInvite() CheckFunc {
	return func(c *CommandContext) bool {
		if c.Msg == nil {
			return false
		}
		result := chat_status.CanInvite(c.Bot, c.Ctx, nil, c.Msg)
		if !result {
			chat_status.NewPermissionResponder(c.Bot).Respond(c.Ctx, "chat_status_invite_link_bot_error", "")
		}
		return result
	}
}

func CanUserChangeInfo() CheckFunc {
	return func(c *CommandContext) bool {
		if c.User == nil {
			return false
		}
		result := chat_status.CanUserChangeInfo(c.Bot, c.Ctx, nil, c.User.Id)
		if !result {
			chat_status.NewPermissionResponder(c.Bot).Respond(
				c.Ctx,
				"chat_status_change_info_cmd_error",
				"chat_status_change_info_button_error",
			)
		}
		return result
	}
}

func CanBotChangeInfo() CheckFunc {
	return func(c *CommandContext) bool {
		result := chat_status.CanBotChangeInfo(c.Bot, c.Ctx, nil)
		if !result {
			chat_status.NewPermissionResponder(c.Bot).Respond(c.Ctx, "chat_status_bot_change_info_error", "")
		}
		return result
	}
}

func CanUserRestrict() CheckFunc {
	return func(c *CommandContext) bool {
		if c.User == nil {
			return false
		}
		result := chat_status.CanUserRestrict(c.Bot, c.Ctx, nil, c.User.Id)
		if !result {
			chat_status.NewPermissionResponder(c.Bot).Respond(c.Ctx, "chat_status_restrict_cmd_error", "chat_status_restrict_button_error")
		}
		return result
	}
}

func CanBotRestrict() CheckFunc {
	return func(c *CommandContext) bool {
		result := chat_status.CanBotRestrict(c.Bot, c.Ctx, nil)
		if !result {
			chat_status.NewPermissionResponder(c.Bot).Respond(c.Ctx, "chat_status_bot_restrict_group_error", "chat_status_bot_restrict_error")
		}
		return result
	}
}

func RequireUserOwner() CheckFunc {
	return func(c *CommandContext) bool {
		if c.User == nil {
			return false
		}
		result := chat_status.RequireUserOwner(c.Bot, c.Ctx, nil, c.User.Id)
		if !result {
			chat_status.NewPermissionResponder(c.Bot).Respond(c.Ctx, "chat_status_owner_cmd_error", "chat_status_owner_button_error", chat_status.WithReply())
		}
		return result
	}
}

func CanUserDelete() CheckFunc {
	return func(c *CommandContext) bool {
		if c.User == nil {
			return false
		}
		result := chat_status.CanUserDelete(c.Bot, c.Ctx, nil, c.User.Id)
		if !result {
			chat_status.NewPermissionResponder(c.Bot).Respond(c.Ctx, "chat_status_delete_cmd_error", "chat_status_delete_button_error", chat_status.WithReply())
		}
		return result
	}
}

func CanBotDelete() CheckFunc {
	return func(c *CommandContext) bool {
		result := chat_status.CanBotDelete(c.Bot, c.Ctx, nil)
		if !result {
			chat_status.NewPermissionResponder(c.Bot).Respond(c.Ctx, "chat_status_bot_delete_error", "", chat_status.WithReply())
		}
		return result
	}
}

// RunChecks re-runs pipeline checks, for anonymous-admin post-proof handlers
// which bypass WrapCommand's RequiredChecks. Returns false on first failure.
func RunChecks(c *CommandContext, checks []CheckFunc) bool {
	for _, check := range checks {
		if !check(c) {
			return false
		}
	}
	return true
}
