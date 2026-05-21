package helpers

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
)

// A module extension of this struct is just going to be set in the struct, since it may have things like
// `moduleCmds` or `defaultRulesBtn`.
// -- We don't want to `SetType` to the type, but allow by closures / methods.

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
	user := chat_status.RequireUser(b, ctx, false)
	if user == nil {
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
// All wrappers use justCheck=false to enable automatic error messaging.

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
		return chat_status.RequireGroup(c.Bot, c.Ctx, nil, false)
	}
}

// RequireBotAdmin returns a CheckFunc that ensures the bot has admin
// privileges in the chat.
func RequireBotAdmin() CheckFunc {
	return func(c *CommandContext) bool {
		return chat_status.RequireBotAdmin(c.Bot, c.Ctx, nil, false)
	}
}

// RequireUserAdmin returns a CheckFunc that ensures the invoking user
// has admin privileges in the chat.
func RequireUserAdmin() CheckFunc {
	return func(c *CommandContext) bool {
		if c.User == nil {
			return false
		}
		return chat_status.RequireUserAdmin(c.Bot, c.Ctx, nil, c.User.Id, false)
	}
}

// RequireUserOwner returns a CheckFunc that ensures the invoking user
// is the chat creator/owner.
func RequireUserOwner() CheckFunc {
	return func(c *CommandContext) bool {
		if c.User == nil {
			return false
		}
		return chat_status.RequireUserOwner(c.Bot, c.Ctx, nil, c.User.Id, false)
	}
}

// CanUserPromote returns a CheckFunc that ensures the invoking user
// can promote/demote other members.
func CanUserPromote() CheckFunc {
	return func(c *CommandContext) bool {
		if c.User == nil {
			return false
		}
		return chat_status.CanUserPromote(c.Bot, c.Ctx, nil, c.User.Id, false)
	}
}

// CanBotPromote returns a CheckFunc that ensures the bot can promote/demote
// members.
func CanBotPromote() CheckFunc {
	return func(c *CommandContext) bool {
		return chat_status.CanBotPromote(c.Bot, c.Ctx, nil, false)
	}
}

// CanUserRestrict returns a CheckFunc that ensures the invoking user
// can restrict other members.
func CanUserRestrict() CheckFunc {
	return func(c *CommandContext) bool {
		if c.User == nil {
			return false
		}
		return chat_status.CanUserRestrict(c.Bot, c.Ctx, nil, c.User.Id, false)
	}
}

// CanBotRestrict returns a CheckFunc that ensures the bot can restrict
// members.
func CanBotRestrict() CheckFunc {
	return func(c *CommandContext) bool {
		return chat_status.CanBotRestrict(c.Bot, c.Ctx, nil, false)
	}
}

// CanUserPin returns a CheckFunc that ensures the invoking user
// can pin messages.
func CanUserPin() CheckFunc {
	return func(c *CommandContext) bool {
		if c.User == nil {
			return false
		}
		return chat_status.CanUserPin(c.Bot, c.Ctx, nil, c.User.Id, false)
	}
}

// CanBotPin returns a CheckFunc that ensures the bot can pin messages.
func CanBotPin() CheckFunc {
	return func(c *CommandContext) bool {
		return chat_status.CanBotPin(c.Bot, c.Ctx, nil, false)
	}
}

// CanUserChangeInfo returns a CheckFunc that ensures the invoking user
// can change chat information.
func CanUserChangeInfo() CheckFunc {
	return func(c *CommandContext) bool {
		if c.User == nil {
			return false
		}
		return chat_status.CanUserChangeInfo(c.Bot, c.Ctx, nil, c.User.Id, false)
	}
}

// CanBotDelete returns a CheckFunc that ensures the bot can delete messages.
func CanBotDelete() CheckFunc {
	return func(c *CommandContext) bool {
		return chat_status.CanBotDelete(c.Bot, c.Ctx, nil, false)
	}
}

// CanUserDelete returns a CheckFunc that ensures the invoking user
// can delete messages.
func CanUserDelete() CheckFunc {
	return func(c *CommandContext) bool {
		if c.User == nil {
			return false
		}
		return chat_status.CanUserDelete(c.Bot, c.Ctx, nil, c.User.Id, false)
	}
}

// CanInvite returns a CheckFunc that ensures the bot and user have
// permissions to generate invite links.
func CanInvite() CheckFunc {
	return func(c *CommandContext) bool {
		if c.Msg == nil {
			return false
		}
		return chat_status.Caninvite(c.Bot, c.Ctx, nil, c.Msg, false)
	}
}


