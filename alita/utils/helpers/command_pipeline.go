package helpers

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/disabling"
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
	// PMGroupArg lets the command name its group as the first argument in
	// PM. The group is resolved before RequiredChecks.
	PMGroupArg PMGroupArg
}

// PMGroupArg says whether a command takes its group as the first argument in
// PM, and whether it has arguments of its own after the group.
type PMGroupArg int

const (
	// PMGroupArgOff: the command does not take a group from PM.
	PMGroupArgOff PMGroupArg = iota
	// PMGroupArgNone: the command takes no arguments, so with a connection
	// any argument that names no group is a mistake.
	PMGroupArgNone
	// PMGroupArgOwn: the command has arguments of its own, so with a
	// connection an argument that names no group belongs to the command.
	PMGroupArgOwn
)

// PMGroupResolver finds the group a PM command names as its first argument,
// stores it with chat_status.SetOneOffGroup and cuts it from
// ctx.EffectiveMessage. It returns false when it has answered the call
// itself; the command must not run then.
type PMGroupResolver func(b *gotgbot.Bot, ctx *ext.Context, cmd string, arg PMGroupArg) bool

var pmGroupResolver PMGroupResolver

// SetPMGroupResolver sets the resolver WrapCommand runs for commands with a
// PMGroupArg. Without one they run on the saved connection only. Call it from
// init() only: the resolver is read without a lock.
func SetPMGroupResolver(r PMGroupResolver) {
	pmGroupResolver = r
}

// commandReplays is populated only by WrapCommand, alongside dispatcher registration.
var commandReplays sync.Map

// ReplayCommand runs the registered pipeline as the effective actor restored by
// the proof callback. It does not re-enter watcher groups or rewrite the message.
func ReplayCommand(b *gotgbot.Bot, ctx *ext.Context, command string) error {
	h, ok := commandReplays.Load(strings.ToLower(command))
	if !ok {
		return fmt.Errorf("unknown pipeline command: %s", command)
	}
	return h.(handlers.Response)(b, ctx)
}

func WrapCommand(
	dispatcher *ext.Dispatcher,
	desc CommandDescriptor,
	handler func(c *CommandContext) error,
) {
	names := append([]string{desc.Name}, desc.Aliases...)
	h := func(b *gotgbot.Bot, ctx *ext.Context) (result error) {
		defer error_handling.RecoverFromUpdatePanic(ctx, "command_pipeline", "WrapCommand")
		// Runs before the recover above, so a panic after a swap ends groups too.
		defer func() { result = endGroupsAfterChatSwap(ctx, result) }()
		if desc.PMGroupArg != PMGroupArgOff {
			if pmGroupResolver != nil {
				if !pmGroupResolver(b, ctx, desc.Name, desc.PMGroupArg) {
					return ext.EndGroups
				}
				if chat_status.OneOffGroup(ctx) != nil {
					// Later handler groups must not see the cut message.
					defer func() { result = EndGroupsLogging(ctx, result) }()
				}
			}
		}
		c, err := BuildCommandContext(b, ctx)
		if err != nil {
			return ext.EndGroups
		}
		if desc.Disableable && commandDisabled(c, names) {
			return ext.EndGroups
		}
		checkedChat := c.Chat
		for _, check := range desc.RequiredChecks {
			if !check(c) {
				return ext.EndGroups
			}
			// A PM connection check may resolve a different target. Honour that
			// group's setting without ever deleting the private message.
			if desc.Disableable && c.Chat != checkedChat {
				if commandDisabled(c, names) {
					return ext.EndGroups
				}
				checkedChat = c.Chat
			}
		}
		return handler(c)
	}
	for _, name := range names {
		commandReplays.Store(strings.ToLower(name), handlers.Response(h))
		dispatcher.AddHandlerToGroup(handlers.NewCommand(name, h), desc.Group)
		if desc.Disableable {
			AddCmdToDisableable(name)
		}
	}
}

// endGroupsAfterChatSwap ends handler-group iteration once the command has
// pointed ctx.EffectiveChat away from the chat the message was sent in, as a
// PM command acting on the connected group does. gotgbot hands the same ctx to
// every later handler group, so their watchers (reactions, filters, locks, ...)
// would otherwise act on that chat with the PM message's id. gotgbot sets
// EffectiveChat to the message's chat, so the ids differ only after such a
// swap. A real error is logged here, as the dispatcher's error handler would:
// returning EndGroups keeps it from that handler.
func endGroupsAfterChatSwap(ctx *ext.Context, err error) error {
	if ctx == nil || ctx.EffectiveMessage == nil || ctx.EffectiveChat == nil ||
		ctx.EffectiveChat.Id == ctx.EffectiveMessage.Chat.Id {
		return err
	}
	return EndGroupsLogging(ctx, err)
}

// EndGroupsLogging returns ext.EndGroups in place of err, logging a real
// error as the dispatcher's error handler would.
func EndGroupsLogging(ctx *ext.Context, err error) error {
	if err != nil && !errors.Is(err, ext.EndGroups) && !errors.Is(err, ext.ContinueGroups) {
		LogHandlerError(ctx, err)
	}
	return ext.EndGroups
}

// commandDisabled applies one setting to all spellings, using the effective
// actor rather than the anonymous sender retained on a replayed message.
func commandDisabled(c *CommandContext, names []string) bool {
	if c.Chat == nil || c.Chat.Type == "private" || c.User == nil {
		return false
	}
	disabled := disabling.GetChatDisabledCMDsCached(c.Chat.Id)
	for _, name := range names {
		if !slices.Contains(disabled, name) {
			continue
		}
		if chat_status.IsUserAdminForUpdate(c.Bot, c.Ctx, c.Chat.Id, c.User.Id) {
			return false
		}
		if c.Msg != nil && c.Msg.Chat.Type != "private" && disabling.ShouldDel(c.Chat.Id) {
			if _, err := c.Msg.Delete(c.Bot, nil); err != nil {
				log.Errorf("[command_pipeline] Failed to delete disabled command %q in chat %d: %v", name, c.Chat.Id, err)
			}
		}
		return true
	}
	return false
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
		result := chat_status.IsBotAdminForUpdate(c.Bot, c.Ctx, nil)
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
		chat := c.Chat
		if chat == nil && c.Ctx != nil {
			chat = c.Ctx.EffectiveChat
		}
		result := chat != nil && chat_status.IsUserAdminForUpdate(c.Bot, c.Ctx, chat.Id, c.User.Id)
		if !result {
			chat_status.NewPermissionResponder(c.Bot).Respond(c.Ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
		}
		return result
	}
}

// anonProofSent reports whether a chat_status check has already answered an
// anonymous admin with the proof keyboard (checkAnonAdmin), so the CheckFunc
// must not send a refusal on top of it.
func anonProofSent(c *CommandContext) bool {
	if c.Ctx == nil || c.Ctx.EffectiveMessage == nil {
		return false
	}
	s := c.Ctx.EffectiveSender
	return s != nil && s.IsAnonymousAdmin()
}

func CanUserPromote() CheckFunc {
	return func(c *CommandContext) bool {
		if c.User == nil {
			return false
		}
		result := chat_status.UserHasPermissionForUpdate(c.Bot, c.Ctx, nil, c.User.Id, func(m *gotgbot.MergedChatMember) bool {
			return m.CanPromoteMembers
		})
		if !result && !anonProofSent(c) {
			chat_status.NewPermissionResponder(c.Bot).Respond(c.Ctx, "chat_status_promote_cmd_error", "chat_status_promote_button_error")
		}
		return result
	}
}

func CanBotPromote() CheckFunc {
	return func(c *CommandContext) bool {
		result := chat_status.BotHasPermissionForUpdate(c.Bot, c.Ctx, nil, func(m *gotgbot.MergedChatMember) bool {
			return m.CanPromoteMembers
		})
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
		result := chat_status.UserHasPermissionForUpdate(c.Bot, c.Ctx, nil, c.User.Id, func(m *gotgbot.MergedChatMember) bool {
			return m.CanPinMessages
		})
		if !result && !anonProofSent(c) {
			chat_status.NewPermissionResponder(c.Bot).Respond(c.Ctx, "chat_status_pin_user_error", "")
		}
		return result
	}
}

func CanBotPin() CheckFunc {
	return func(c *CommandContext) bool {
		result := chat_status.BotHasPermissionForUpdate(c.Bot, c.Ctx, nil, func(m *gotgbot.MergedChatMember) bool {
			return m.CanPinMessages
		})
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
		result, answered := chat_status.CanInviteAnswered(c.Bot, c.Ctx, nil, c.Msg)
		if !result && !answered {
			// CanInvite fails for the bot's rights and the user's alike; blame the right one.
			key := "chat_status_invite_link_user_error"
			if !chat_status.CanBotInvite(c.Bot, c.Ctx, nil) {
				key = "chat_status_invite_link_bot_error"
			}
			chat_status.NewPermissionResponder(c.Bot).Respond(c.Ctx, key, "")
		}
		return result
	}
}

func CanUserChangeInfo() CheckFunc {
	return func(c *CommandContext) bool {
		if c.User == nil {
			return false
		}
		result := chat_status.UserHasPermissionForUpdate(c.Bot, c.Ctx, nil, c.User.Id, func(m *gotgbot.MergedChatMember) bool {
			return m.CanChangeInfo
		})
		if !result && !anonProofSent(c) {
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
		result := chat_status.BotHasPermissionForUpdate(c.Bot, c.Ctx, nil, func(m *gotgbot.MergedChatMember) bool {
			return m.CanChangeInfo
		})
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
		result := chat_status.UserHasPermissionForUpdate(c.Bot, c.Ctx, nil, c.User.Id, func(m *gotgbot.MergedChatMember) bool {
			return m.CanRestrictMembers
		})
		if !result && !anonProofSent(c) {
			chat_status.NewPermissionResponder(c.Bot).Respond(c.Ctx, "chat_status_restrict_cmd_error", "chat_status_restrict_button_error")
		}
		return result
	}
}

func CanBotRestrict() CheckFunc {
	return func(c *CommandContext) bool {
		result := chat_status.BotHasPermissionForUpdate(c.Bot, c.Ctx, nil, func(m *gotgbot.MergedChatMember) bool {
			return m.CanRestrictMembers
		})
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
		result := chat_status.UserHasPermissionForUpdate(c.Bot, c.Ctx, nil, c.User.Id, func(m *gotgbot.MergedChatMember) bool {
			return m.CanDeleteMessages
		})
		if !result && !anonProofSent(c) {
			chat_status.NewPermissionResponder(c.Bot).Respond(c.Ctx, "chat_status_delete_cmd_error", "chat_status_delete_button_error", chat_status.WithReply())
		}
		return result
	}
}

func CanBotDelete() CheckFunc {
	return func(c *CommandContext) bool {
		result := chat_status.BotHasPermissionForUpdate(c.Bot, c.Ctx, nil, func(m *gotgbot.MergedChatMember) bool {
			return m.CanDeleteMessages
		})
		if !result {
			chat_status.NewPermissionResponder(c.Bot).Respond(c.Ctx, "chat_status_bot_delete_error", "", chat_status.WithReply())
		}
		return result
	}
}

// RunChecks runs ordered checks until the first refusal.
func RunChecks(c *CommandContext, checks []CheckFunc) bool {
	for _, check := range checks {
		if !check(c) {
			return false
		}
	}
	return true
}
