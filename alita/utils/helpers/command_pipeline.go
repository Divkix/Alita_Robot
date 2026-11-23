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

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db/devs"
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
	return buildCommandContext(b, ctx, true)
}

// Public descriptors need no actor; checked commands and direct callers do.
func buildCommandContext(b *gotgbot.Bot, ctx *ext.Context, requireUser bool) (*CommandContext, error) {
	user := chat_status.RequireUser(b, ctx)
	if user == nil && requireUser {
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

// CheckFunc is the runnable half of a Check: it replies on refusal and
// returns whether the command may run.
type CheckFunc func(c *CommandContext) bool

// CheckKind classifies what a Check gates on, so the command manifest can
// derive whether any member-usable form of a command exists without running
// the opaque CheckFunc.
type CheckKind int

const (
	// CheckKindAny gates on neither role nor identity: chat shape, connection
	// state, federation membership, argument shape. A member can pass it.
	CheckKindAny CheckKind = iota
	// CheckKindMember additionally requires an identifiable user or a group,
	// never an admin role.
	CheckKindMember
	// CheckKindBotTeam restricts to the bot owner, developers, sudo users or
	// the bot's own admin status: invisible to group members either way.
	CheckKindBotTeam
	// CheckKindAdmin requires a group admin role (user admin, owner, a
	// connected/named group's admin, a CanUser* right, or a setting declared
	// with an admin Right).
	CheckKindAdmin
	// CheckKindUnknown is for checks whose kind was not declared. It counts
	// as admin-gated so an unlabeled check can never leak a command into the
	// member menu; declare the kind instead of leaving this.
	CheckKindUnknown
)

// checkKindAdminOnly reports whether a check of this kind can only pass for
// an admin (or the bot team), never for a plain member.
func checkKindAdminOnly(kind CheckKind) bool {
	switch kind {
	case CheckKindAny, CheckKindMember:
		return false
	default:
		return true
	}
}

// Check is one RequiredChecks entry: the runnable check plus the CheckKind
// the command manifest derives admin-only visibility from. Declare checks
// through the helpers.Check* constructors or helpers.NewCheck; a bare
// CheckFunc literal has no kind and must not appear in RequiredChecks.
type Check struct {
	Kind CheckKind
	Run  CheckFunc
}

// NewCheck pairs run with kind. Use the CheckKind constants; CheckKindUnknown
// keeps the command out of the member menu.
func NewCheck(kind CheckKind, run CheckFunc) Check {
	return Check{Kind: kind, Run: run}
}

// adminOnly reports whether a plain member can never run the command: one
// admin-, bot-team- or unknown-gated check is enough, since every check in
// RequiredChecks must pass.
func adminOnly(checks []Check) bool {
	for _, check := range checks {
		if checkKindAdminOnly(check.Kind) {
			return true
		}
	}
	return false
}

// CommandEntry is the manifest record WrapCommand writes once per command
// name (aliases included): the module that registered it and whether the
// /commands menu shows it to members only.
type CommandEntry struct {
	// Module is the registered module name ("Bans", "AntiRaid"), not the
	// lowercase locale key; callers lowercase it.
	Module string
	// AdminOnly is the menu's member-visibility flag: derived from
	// RequiredChecks unless the descriptor set AdminOnlySet.
	AdminOnly bool
}

var (
	commandManifestMu sync.Mutex
	commandManifest   = map[string]CommandEntry{}
	// currentCommandModule is the module whose loader is running; set by
	// loadModule in alita/modules, read by WrapCommand.
	currentCommandModule string
)

// SetCommandModule records the module a loader is registering commands for.
// It is set by the module loader wrapper, not by callers.
func SetCommandModule(name string) { currentCommandModule = name }

// CurrentCommandModule returns the module whose loader is running, or ""
// outside one (direct LoadXxx calls in tests).
func CurrentCommandModule() string { return currentCommandModule }

// RecordCommand stores name's manifest entry. WrapCommand calls it for the
// command name and every alias. A registration outside a module loader
// (Module "") never erases a module an earlier loader recorded.
func RecordCommand(name string, entry CommandEntry) {
	commandManifestMu.Lock()
	defer commandManifestMu.Unlock()
	key := strings.ToLower(name)
	if old, ok := commandManifest[key]; ok && old.Module != "" && entry.Module == "" {
		entry.Module = old.Module
	}
	commandManifest[key] = entry
}

// CommandManifest returns a copy of the recorded command→module manifest.
func CommandManifest() map[string]CommandEntry {
	commandManifestMu.Lock()
	defer commandManifestMu.Unlock()
	out := make(map[string]CommandEntry, len(commandManifest))
	for k, v := range commandManifest {
		out[k] = v
	}
	return out
}

type CommandDescriptor struct {
	Name    string
	Aliases []string
	Group   int
	// Checks require an identifiable user. With no checks, User may be nil.
	RequiredChecks []Check
	Disableable    bool
	// PMGroupArg lets the command name its group as the first argument in
	// PM. The group is resolved before RequiredChecks.
	PMGroupArg PMGroupArg
	// AdminOnlySet marks AdminOnly as an explicit declaration instead of the
	// value derived from RequiredChecks. Set it only when the checks cannot
	// express the menu's member visibility (a member-usable command the menu
	// hides, or an admin-gated command a member path makes look open); record
	// why next to the descriptor.
	AdminOnlySet bool
	AdminOnly    bool
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

// RunCommand runs a registered command pipeline as the effective actor, for
// proof callbacks and watcher entry points. It does not re-enter watcher groups
// or rewrite the message.
func RunCommand(b *gotgbot.Bot, ctx *ext.Context, command string) error {
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
	entryAdminOnly := desc.AdminOnly
	if !desc.AdminOnlySet {
		entryAdminOnly = adminOnly(desc.RequiredChecks)
	}
	entry := CommandEntry{Module: currentCommandModule, AdminOnly: entryAdminOnly}
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
		c, err := buildCommandContext(b, ctx, len(desc.RequiredChecks) != 0)
		if err != nil {
			return ext.EndGroups
		}
		if desc.Disableable && commandDisabled(c, names) {
			return ext.EndGroups
		}
		checkedChat := c.Chat
		for _, check := range desc.RequiredChecks {
			if !check.Run(c) {
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
		RecordCommand(name, entry)
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
	if c.Chat == nil || c.Chat.Type == "private" {
		return false
	}
	disabled := disabling.GetChatDisabledCMDsCached(c.Chat.Id)
	for _, name := range names {
		if !slices.Contains(disabled, name) {
			continue
		}
		if c.User != nil && chat_status.IsUserAdminForUpdate(c.Bot, c.Ctx, c.Chat.Id, c.User.Id) {
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

// BotAdminRequirement names whether a connected check also requires bot admin.
type BotAdminRequirement bool

const (
	BotAdminNotRequired BotAdminRequirement = false
	BotAdminRequired    BotAdminRequirement = true
)

// RequireConnectedAdmin resolves the current group, or the named/connected group
// in PM, requiring its admin (and the bot when requested). Connection refusals
// keep their bot-before-user ordering. Both context chat fields follow the target.
func RequireConnectedAdmin(botAdmin BotAdminRequirement) Check {
	return requireConnectedChat(true, bool(botAdmin))
}

// RequireConnectedMember resolves the same target without requiring user admin.
// PM membership and stale connections are checked by IsUserConnected.
func RequireConnectedMember(botAdmin BotAdminRequirement) Check {
	return requireConnectedChat(false, bool(botAdmin))
}

func requireConnectedChat(requireUserAdmin, requireBotAdmin bool) Check {
	kind := CheckKindMember
	if requireUserAdmin {
		kind = CheckKindAdmin
	}
	return NewCheck(kind, func(c *CommandContext) bool {
		chat := chat_status.IsUserConnected(c.Bot, c.Ctx, requireUserAdmin, requireBotAdmin)
		if chat == nil {
			return false
		}
		c.Chat = chat
		c.Ctx.EffectiveChat = chat
		return true
	})
}

// RequireBotOwner silently refuses everyone except the configured bot owner.
func RequireBotOwner() Check {
	return NewCheck(CheckKindBotTeam, func(c *CommandContext) bool {
		return c.User != nil && c.User.Id == config.AppConfig.OwnerId
	})
}

// RequireBotDeveloper accepts the bot owner and developers, but not sudo users.
// Bot-team checks deliberately send no permission refusal.
func RequireBotDeveloper() Check {
	return NewCheck(CheckKindBotTeam, func(c *CommandContext) bool {
		return c.User != nil && (c.User.Id == config.AppConfig.OwnerId || devs.GetTeamMemInfo(c.User.Id).IsDev)
	})
}

// RequireBotTeamMember silently accepts the bot owner, developers and sudo users.
func RequireBotTeamMember() Check {
	return NewCheck(CheckKindBotTeam, func(c *CommandContext) bool {
		if c.User == nil {
			return false
		}
		if c.User.Id == config.AppConfig.OwnerId {
			return true
		}
		member := devs.GetTeamMemInfo(c.User.Id)
		return member.IsDev || member.Sudo
	})
}

// RequireIdentifiedUser preserves identity-only gates on commands with no role check.
func RequireIdentifiedUser() Check {
	return NewCheck(CheckKindMember, func(c *CommandContext) bool {
		if c.User != nil {
			return true
		}
		return chat_status.NewPermissionResponder(c.Bot).Respond(c.Ctx, "common_cannot_identify_user", "", chat_status.WithReply())
	})
}

func RequireGroup() Check {
	return NewCheck(CheckKindMember, func(c *CommandContext) bool {
		result := chat_status.RequireGroup(c.Bot, c.Ctx, nil)
		if !result {
			chat_status.NewPermissionResponder(c.Bot).Respond(c.Ctx, "chat_status_group_only_error", "", chat_status.WithReply())
		}
		return result
	})
}

// RequireBotAdmin gates on the bot's own admin status, not the caller's role:
// a member can run a command whose only checks are this and member-safe ones.
func RequireBotAdmin() Check {
	return NewCheck(CheckKindAny, func(c *CommandContext) bool {
		result := chat_status.IsBotAdminForUpdate(c.Bot, c.Ctx, nil)
		if !result {
			chat_status.NewPermissionResponder(c.Bot).Respond(c.Ctx, "chat_status_bot_not_admin", "", chat_status.WithReply())
		}
		return result
	})
}

func RequireUserAdmin() Check {
	return NewCheck(CheckKindAdmin, func(c *CommandContext) bool {
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
	})
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

func CanUserPromote() Check {
	return NewCheck(CheckKindAdmin, func(c *CommandContext) bool {
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
	})
}

func CanBotPromote() Check {
	return NewCheck(CheckKindAny, func(c *CommandContext) bool {
		result := chat_status.BotHasPermissionForUpdate(c.Bot, c.Ctx, nil, func(m *gotgbot.MergedChatMember) bool {
			return m.CanPromoteMembers
		})
		if !result {
			chat_status.NewPermissionResponder(c.Bot).Respond(c.Ctx, "chat_status_bot_promote_error", "")
		}
		return result
	})
}

func CanUserPin() Check {
	return NewCheck(CheckKindAdmin, func(c *CommandContext) bool {
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
	})
}

func CanBotPin() Check {
	return NewCheck(CheckKindAny, func(c *CommandContext) bool {
		result := chat_status.BotHasPermissionForUpdate(c.Bot, c.Ctx, nil, func(m *gotgbot.MergedChatMember) bool {
			return m.CanPinMessages
		})
		if !result {
			chat_status.NewPermissionResponder(c.Bot).Respond(c.Ctx, "chat_status_pin_bot_error", "")
		}
		return result
	})
}

func CanInvite() Check {
	return NewCheck(CheckKindAny, func(c *CommandContext) bool {
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
	})
}

func CanUserChangeInfo() Check {
	return NewCheck(CheckKindAdmin, func(c *CommandContext) bool {
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
	})
}

func CanBotChangeInfo() Check {
	return NewCheck(CheckKindAny, func(c *CommandContext) bool {
		result := chat_status.BotHasPermissionForUpdate(c.Bot, c.Ctx, nil, func(m *gotgbot.MergedChatMember) bool {
			return m.CanChangeInfo
		})
		if !result {
			chat_status.NewPermissionResponder(c.Bot).Respond(c.Ctx, "chat_status_bot_change_info_error", "")
		}
		return result
	})
}

func CanUserRestrict() Check {
	return NewCheck(CheckKindAdmin, func(c *CommandContext) bool {
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
	})
}

func CanBotRestrict() Check {
	return NewCheck(CheckKindAny, func(c *CommandContext) bool {
		result := chat_status.BotHasPermissionForUpdate(c.Bot, c.Ctx, nil, func(m *gotgbot.MergedChatMember) bool {
			return m.CanRestrictMembers
		})
		if !result {
			chat_status.NewPermissionResponder(c.Bot).Respond(c.Ctx, "chat_status_bot_restrict_group_error", "chat_status_bot_restrict_error")
		}
		return result
	})
}

func RequireUserOwner() Check {
	return NewCheck(CheckKindAdmin, func(c *CommandContext) bool {
		if c.User == nil {
			return false
		}
		result := chat_status.RequireUserOwner(c.Bot, c.Ctx, nil, c.User.Id)
		if !result {
			chat_status.NewPermissionResponder(c.Bot).Respond(c.Ctx, "chat_status_owner_cmd_error", "chat_status_owner_button_error", chat_status.WithReply())
		}
		return result
	})
}

func CanUserDelete() Check {
	return NewCheck(CheckKindAdmin, func(c *CommandContext) bool {
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
	})
}

func CanBotDelete() Check {
	return NewCheck(CheckKindAny, func(c *CommandContext) bool {
		result := chat_status.BotHasPermissionForUpdate(c.Bot, c.Ctx, nil, func(m *gotgbot.MergedChatMember) bool {
			return m.CanDeleteMessages
		})
		if !result {
			chat_status.NewPermissionResponder(c.Bot).Respond(c.Ctx, "chat_status_bot_delete_error", "", chat_status.WithReply())
		}
		return result
	})
}

// RunChecks runs ordered checks until the first refusal.
func RunChecks(c *CommandContext, checks []Check) bool {
	for _, check := range checks {
		if !check.Run(c) {
			return false
		}
	}
	return true
}
