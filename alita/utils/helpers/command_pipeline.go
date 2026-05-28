package helpers

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
)

// CommandPipeline provides declarative command registration with pre-flight permission checks.

// CommandContext holds the decomposed context for a command handler.
// It provides pre-extracted common objects so that permission checks and
// business logic receive a clean, uniform input.
type CommandContext struct {
	Bot  *gotgbot.Bot
	Ctx  *ext.Context
	Chat *gotgbot.Chat
	Msg  *gotgbot.Message
	User *gotgbot.User
	Tr   *i18n.Translator
}

// BuildCommandContext populates a CommandContext from a raw gotgbot update.
// It extracts the user via chat_status.RequireUser, constructs the translator,
// and returns the populated struct. If user extraction fails, an error sentinel
// is returned so the wrapper can return ext.EndGroups without invoking checks.
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
		Tr:   i18n.MustNewTranslator(db.GetLanguage(ctx)),
	}, nil
}

// CheckFunc is a permission gate. Returns true if the gate passes.
// On failure it must already have sent any required error messages.
type CheckFunc func(c *CommandContext) bool

// CommandDescriptor declares what a command needs before business logic runs.
// It is used by WrapCommand to wire checks, alias registration, and group
// assignment declaratively.
type CommandDescriptor struct {
	Name           string
	Aliases        []string
	Group          int
	RequiredChecks []CheckFunc
	Disableable    bool
}

// WrapCommand registers a command with the dispatcher, running all declared
// checks before invoking the business logic handler. If a check returns false,
// the pipeline short-circuits with ext.EndGroups.
//
// The handler receives a pre-built *CommandContext containing Bot, Ctx, Chat,
// Msg, User, and Tr. Use this for new handlers; for gradual migration of
// legacy handlers that still expect raw (b, ctx), see WrapCommandRaw.
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

// WrapCommandRaw registers a command with the dispatcher, running all declared
// checks before invoking the business logic handler. If a check returns false,
// the pipeline short-circuits with ext.EndGroups.
//
// The handler receives the original (b, ctx) pair and must construct its own
// CommandContext or translator. This supports gradual migration; prefer
// WrapCommand for new handlers.
func WrapCommandRaw(
	dispatcher *ext.Dispatcher,
	desc CommandDescriptor,
	handler func(b *gotgbot.Bot, ctx *ext.Context) error,
) {
	h := func(b *gotgbot.Bot, ctx *ext.Context) error {
		defer error_handling.RecoverFromPanic("command_pipeline", "WrapCommandRaw")
		c, err := BuildCommandContext(b, ctx)
		if err != nil {
			return ext.EndGroups
		}
		for _, check := range desc.RequiredChecks {
			if !check(c) {
				return ext.EndGroups
			}
		}
		return handler(b, ctx)
	}
	register(dispatcher, desc, h)
}

// register wires a handler (already wrapped) into the dispatcher.
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

// --- pre-built check function builders ---
// All wrappers call pure permission checks and explicitly invoke
// PermissionResponder when checks fail. This absorbs the messaging
// responsibility so module handlers using the pipeline need no changes.

// CheckDisabled returns a CheckFunc that blocks the command when
// chat_status.CheckDisabledCmd reports it disabled in the current chat.
func CheckDisabled(cmdName string) CheckFunc {
	return func(c *CommandContext) bool {
		if c.Msg == nil || c.Bot == nil {
			return false
		}
		return !chat_status.CheckDisabledCmd(c.Bot, c.Msg, cmdName)
	}
}

// RequireGroup returns a CheckFunc that ensures the chat is a group
// (not private). If the check fails, an error message is sent automatically.
func RequireGroup() CheckFunc {
	return func(c *CommandContext) bool {
		result := chat_status.RequireGroup(c.Bot, c.Ctx, nil)
		if !result {
			chat_status.NewPermissionResponder(c.Bot).Respond(c.Ctx, "chat_status_group_only_error", "", chat_status.WithReply())
		}
		return result
	}
}

// RequireBotAdmin returns a CheckFunc that ensures the bot has admin
// privileges in the chat.
func RequireBotAdmin() CheckFunc {
	return func(c *CommandContext) bool {
		result := chat_status.RequireBotAdmin(c.Bot, c.Ctx, nil)
		if !result {
			chat_status.NewPermissionResponder(c.Bot).Respond(c.Ctx, "chat_status_bot_not_admin", "", chat_status.WithReply())
		}
		return result
	}
}

// RequireUserAdmin returns a CheckFunc that ensures the invoking user
// has admin privileges in the chat.
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

// RequireUserOwner returns a CheckFunc that ensures the invoking user
// is the chat creator/owner.
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

// CanUserPromote returns a CheckFunc that ensures the invoking user
// can promote/demote other members.
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

// CanBotPromote returns a CheckFunc that ensures the bot can promote/demote
// members.
func CanBotPromote() CheckFunc {
	return func(c *CommandContext) bool {
		result := chat_status.CanBotPromote(c.Bot, c.Ctx, nil)
		if !result {
			chat_status.NewPermissionResponder(c.Bot).Respond(c.Ctx, "chat_status_bot_promote_error", "")
		}
		return result
	}
}

// CanUserRestrict returns a CheckFunc that ensures the invoking user
// can restrict other members.
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

// CanBotRestrict returns a CheckFunc that ensures the bot can restrict
// members.
func CanBotRestrict() CheckFunc {
	return func(c *CommandContext) bool {
		result := chat_status.CanBotRestrict(c.Bot, c.Ctx, nil)
		if !result {
			chat_status.NewPermissionResponder(c.Bot).Respond(c.Ctx, "chat_status_bot_restrict_group_error", "chat_status_bot_restrict_error")
		}
		return result
	}
}

// CanUserPin returns a CheckFunc that ensures the invoking user
// can pin messages.
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

// CanBotPin returns a CheckFunc that ensures the bot can pin messages.
func CanBotPin() CheckFunc {
	return func(c *CommandContext) bool {
		result := chat_status.CanBotPin(c.Bot, c.Ctx, nil)
		if !result {
			chat_status.NewPermissionResponder(c.Bot).Respond(c.Ctx, "chat_status_pin_bot_error", "")
		}
		return result
	}
}

// CanUserChangeInfo returns a CheckFunc that ensures the invoking user
// can change chat information.
func CanUserChangeInfo() CheckFunc {
	return func(c *CommandContext) bool {
		if c.User == nil {
			return false
		}
		result := chat_status.CanUserChangeInfo(c.Bot, c.Ctx, nil, c.User.Id)
		if !result {
			chat_status.NewPermissionResponder(c.Bot).Respond(c.Ctx, "chat_status_change_info_cmd_error", "chat_status_change_info_button_error")
		}
		return result
	}
}

// CanBotDelete returns a CheckFunc that ensures the bot can delete messages.
func CanBotDelete() CheckFunc {
	return func(c *CommandContext) bool {
		result := chat_status.CanBotDelete(c.Bot, c.Ctx, nil)
		if !result {
			chat_status.NewPermissionResponder(c.Bot).Respond(c.Ctx, "chat_status_bot_delete_error", "", chat_status.WithReply())
		}
		return result
	}
}

// CanUserDelete returns a CheckFunc that ensures the invoking user
// can delete messages.
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

// CanInvite returns a CheckFunc that ensures the bot and user have
// permissions to generate invite links.
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


