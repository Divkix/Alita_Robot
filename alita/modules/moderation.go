package modules

import (
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/moderation"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/actionlog"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/extraction"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

// moderationCtx holds the decomposed context for a moderation command.
// It provides a minimal surface that every moderation handler needs.
type moderationCtx struct {
	Bot    *gotgbot.Bot
	Chat   *gotgbot.Chat
	Msg    *gotgbot.Message
	User   *gotgbot.User
	Ctx    *ext.Context
	Tr     *i18n.Translator
	Module *moduleStruct
	// Command is the command name whose help answers a wrong call.
	Command string
}

// replyCommandHelp answers a call the command cannot run as given with
// errText (HTML) and the command's help.
func (c *moderationCtx) replyCommandHelp(errText string) error {
	return replyCommandHelp(c.Bot, c.Msg, c.Tr, c.Command, errText)
}

// inPM reports whether msg was sent in a private chat with the bot.
func inPM(msg *gotgbot.Message) bool {
	return msg != nil && msg.Chat.Type == "private"
}

// sentInPM reports whether the command came from PM. A silent command sent
// there has no message in the group to delete, so it answers like its plain
// form.
func (c *moderationCtx) sentInPM() bool {
	return inPM(c.Msg)
}

// target holds the resolved target user and optional reason text.
type target struct {
	userID    int64
	reason    string
	timeVal   string // used by time-based commands (e.g., tban)
	isChannel bool   // true if original target was a channel ID
	// untilUnix is the until_date a temporary ban was made with, 0 for a
	// permanent one.
	untilUnix int64
}

// extractFromArgs resolves the target from command arguments using ExtractUserAndText.
// Returns a sentinel userID of -1 when extraction itself signals an error reply
// has already been sent by the extractor (e.g. bad mention).
func extractFromArgs(c *moderationCtx) (target, error) {
	uid, reason := extraction.ExtractUserAndText(c.Bot, c.Ctx)
	if uid == -1 {
		// extraction already sent an error message
		return target{}, fmt.Errorf("extraction failed")
	}
	if chat_status.IsChannelId(uid) {
		return target{}, replyAnonymousTarget(c)
	}
	if uid == 0 {
		text, _ := c.Tr.GetString("common_no_user_specified")
		if err := c.replyCommandHelp(text); err != nil {
			log.Error(err)
			return target{}, err
		}
		return target{}, fmt.Errorf("no user")
	}
	return target{userID: uid, reason: reason}, nil
}

// replyAnonymousTarget refuses a channel or anonymous admin as the target.
func replyAnonymousTarget(c *moderationCtx) error {
	text, _ := c.Tr.GetString("bans_anonymous_ban_only_error")
	if _, err := c.Msg.Reply(c.Bot, text, formatting.Shtml()); err != nil {
		log.Error(err)
		return err
	}
	return fmt.Errorf("anonymous user")
}

// extractFromReply resolves the target from the replied-to message.
func extractFromReply(c *moderationCtx) (target, error) {
	if c.Msg.ReplyToMessage == nil {
		text, _ := c.Tr.GetString("bans_dkick_no_reply")
		if err := c.replyCommandHelp(text); err != nil {
			log.Error(err)
			return target{}, err
		}
		return target{}, fmt.Errorf("no reply")
	}
	if c.Msg.ReplyToMessage.From == nil {
		text, _ := c.Tr.GetString("bans_cannot_identify_user")
		_, err := c.Msg.Reply(c.Bot, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return target{}, err
		}
		return target{}, fmt.Errorf("nil from")
	}
	uid := c.Msg.ReplyToMessage.From.Id
	if chat_status.IsChannelId(uid) {
		return target{}, replyAnonymousTarget(c)
	}
	if uid == 0 {
		noUserKey := "common_no_user_specified"
		text, _ := c.Tr.GetString(noUserKey)
		_, err := c.Msg.Reply(c.Bot, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return target{}, err
		}
		return target{}, fmt.Errorf("no user")
	}
	return target{userID: uid}, nil
}

// validateTargetFn checks the resolved target before executing the action.
// The function receives the moderationCtx and resolved target; on failure
// it sends the i18n error message and returns a non-nil error.
type validateTargetFn func(c *moderationCtx, t *target) error

// actionFn performs the moderation API call.
// It receives the moderationCtx and the validated target.
type actionFn func(c *moderationCtx, t *target) error

// replyFn builds and sends the success reply for a moderation command.
// It receives the moderationCtx, the validated target, and the API action result.
type replyFn func(c *moderationCtx, t *target) error

// moderationCommand wires the fixed-order pipeline for a moderation handler:
//
//	RequireUser -> extractTarget -> validate -> execute -> reply.
//
// Permission checks are not part of it: the command's descriptor runs them
// through WrapCommand, or anonPipelineHandler after an anonymous admin's proof.
//
// It eliminates the 30+ lines of boilerplate that every moderation command repeats.
type moderationCommand struct {
	// command names the help entry a wrong call answers with (commandHelpModules).
	command  string
	extract  func(*moderationCtx) (target, error)
	validate validateTargetFn
	execute  actionFn
	// cleanup runs after the event is recorded, for a step that must not
	// decide whether the action counts: /sban and /skick delete the command
	// message after banning or kicking, and a failed delete loses nothing.
	cleanup   actionFn
	reply     replyFn
	module    *moduleStruct
	logAction string
	logUser   bool
	// eventAction is the moderation_events action recorded after execute
	// succeeds. Variants set their base action; empty (unmute, kickme, banme,
	// restrict) records nothing.
	eventAction string
	// selfBan records the ban execute made in ban_records although it records
	// no event (banme). An eventAction of ban records the ban too, and one of
	// unban closes the record of the ban it lifted.
	selfBan bool
}

// buildModerationCtx creates the common decomposed context from a gotgbot update.
func buildModerationCtx(m *moduleStruct, b *gotgbot.Bot, ctx *ext.Context) (*moderationCtx, error) {
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		chat_status.NewPermissionResponder(b).Respond(ctx, "common_cannot_identify_user", "", chat_status.WithReply())
		return nil, ext.EndGroups //nolint:goerr113 // EndGroups is not a real error
	}
	return &moderationCtx{
		Bot:    b,
		Chat:   ctx.EffectiveChat,
		Msg:    ctx.EffectiveMessage,
		User:   user,
		Ctx:    ctx,
		Tr:     i18n.MustNewTranslator(lang.GetLanguage(ctx)),
		Module: m,
	}, nil
}

// run executes the moderation pipeline.
func (cmd *moderationCommand) run(b *gotgbot.Bot, ctx *ext.Context) error {
	mc, err := buildModerationCtx(cmd.module, b, ctx)
	if err != nil {
		return ext.EndGroups
	}
	mc.Command = cmd.command

	// Extract the target user (+ optional reason).
	tgt, err := cmd.extract(mc)
	if err != nil {
		return ext.EndGroups
	}

	// Validate the resolved target.
	if cmd.validate != nil {
		if err := cmd.validate(mc, &tgt); err != nil {
			return ext.EndGroups
		}
	}

	// Execute the moderation action.
	if err := cmd.execute(mc, &tgt); err != nil {
		log.Error(err)
		return err
	}

	// The action happened: record its evidence row before any cleanup or
	// reply, so a failure there cannot lose it. The write never affects the
	// command.
	if cmd.eventAction != "" {
		recordModerationEvent(mc.Chat.Id, mc.User.Id, tgt.userID, cmd.eventAction, tgt.reason)
	}
	var banRecordID int64
	if cmd.selfBan || cmd.eventAction == models.ModerationActionBan {
		banRecordID = recordBan(banRecord{
			chatID:    mc.Chat.Id,
			userID:    tgt.userID,
			bannedBy:  mc.User.Id,
			source:    models.BanSourceManual,
			reason:    tgt.reason,
			untilUnix: tgt.untilUnix,
		})
	}
	if cmd.eventAction == models.ModerationActionUnban {
		liftBanRecord(mc.Chat.Id, tgt.userID, mc.User.Id)
	}

	// Cleanup runs after the record: the ban or kick stands even when the
	// command message's delete fails.
	if cmd.cleanup != nil {
		if err := cmd.cleanup(mc, &tgt); err != nil {
			log.Error(err)
			return err
		}
	}

	// Build and send the success reply.
	if cmd.reply != nil {
		if err := cmd.reply(mc, &tgt); err != nil {
			log.Error(err)
			return err
		}
	}

	if cmd.logAction != "" {
		if cmd.logUser {
			actionlog.User(mc.Bot, mc.Chat, mc.User, cmd.logAction)
		} else {
			actionlog.AdminWithMarkup(mc.Bot, mc.Chat, mc.User, cmd.logAction, tgt.userID, tgt.reason, unbanLogMarkup(mc.Chat.Id, banRecordID))
		}
	}
	return ext.EndGroups
}

// recordModerationEvent appends the moderation_events row for an action that
// already succeeded. It returns nothing on purpose: a database error is
// logged and never changes the reply or the action's success.
// Commands and inline moderation buttons record each successful human action only here.
// Automated spamdb, captcha, antiraid, locks, blacklist, warn-limit and report auto-mute
// actions never enter this table: the shared-reputation join check reads it, so a blocked
// join must not feed its own signal back. A human unban reverses the reputation signal only
// when the same chat recorded a ban or kick of that user earlier in the reputation window.
func recordModerationEvent(chatID, actorID, targetID int64, action, reason string) {
	err := moderation.Record(models.ModerationEvent{
		ChatId:       chatID,
		ActorId:      actorID,
		TargetUserId: targetID,
		Action:       action,
		Reason:       reason,
		CreatedAt:    time.Now().UTC(),
	})
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"chat_id": chatID,
			"action":  action,
		}).Error("[Moderation] Failed to record the event")
	}
}

// Sentinel errors used by moderation command templates.
var (
	errUserNotInChat = fmt.Errorf("target user is not in chat")
	errAdminTarget   = fmt.Errorf("target is a protected admin")
	errTargetIsBot   = fmt.Errorf("target is the bot itself")
)

// replyValidationError sends a validation failure reply, logging send errors.
func replyValidationError(c *moderationCtx, text string) error {
	_, err := c.Msg.Reply(c.Bot, text, formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// validateTarget runs the shared target checks: optional in-chat membership,
// admin-protection, and bot-self. Each key falls back to the shared common_*
// string when the module-specific key is empty. Returns the sentinel for the
// first failing check, nil when the target is actionable.
func validateTarget(c *moderationCtx, userID int64, checkInChat bool, notInChatKey, adminKey, selfKey string) error {
	if checkInChat && !chat_status.IsUserInChat(c.Bot, c.Chat, userID) {
		text, _ := c.Tr.GetString(notInChatKey)
		if text == "" {
			text, _ = c.Tr.GetString("common_user_not_in_chat")
		}
		if err := replyValidationError(c, text); err != nil {
			return err
		}
		return errUserNotInChat
	}
	if chat_status.IsUserBanProtected(c.Bot, c.Ctx, nil, userID) {
		text, _ := c.Tr.GetString(adminKey)
		if text == "" {
			text, _ = c.Tr.GetString("common_cannot_target_admin")
		}
		if err := replyValidationError(c, text); err != nil {
			return err
		}
		return errAdminTarget
	}
	if userID == c.Bot.Id {
		text, _ := c.Tr.GetString(selfKey)
		if text == "" {
			text, _ = c.Tr.GetString("common_cannot_target_self")
		}
		if err := replyValidationError(c, text); err != nil {
			return err
		}
		return errTargetIsBot
	}
	return nil
}

// extractTemporalTarget parses the duration from the target reason via
// ExtractTime, storing the normalized timeVal/reason back on the target.
// Returns the Telegram until_date, or -1 when it already replied. A call
// without any time gets the command's help; a malformed time keeps the
// extractor's reply, which names the accepted units.
func extractTemporalTarget(c *moderationCtx, t *target) int64 {
	if len(strings.Fields(t.reason)) == 0 {
		text, _ := c.Tr.GetString("extraction_no_time_specified")
		if err := c.replyCommandHelp(html.EscapeString(text)); err != nil {
			log.Error(err)
		}
		return -1
	}
	_time, timeVal, reason := extraction.ExtractTime(c.Bot, c.Ctx, t.reason)
	if _time == -1 {
		return -1
	}
	t.timeVal = timeVal
	t.reason = reason
	return _time
}

// pipelineHandler adapts a legacy (bot,ctx) moderation handler to the command
// pipeline. WrapCommand has already run desc.RequiredChecks, so the domain
// handler need not repeat them.
func pipelineHandler(domain func(*gotgbot.Bot, *ext.Context) error) func(*helpers.CommandContext) error {
	return func(c *helpers.CommandContext) error {
		return domain(c.Bot, c.Ctx)
	}
}

// anonPipelineHandler is the post-proof anonymous-admin entry point for migrated
// commands. It re-runs the same pipeline checks via helpers.RunChecks, making
// migrated commands anon-safe by default without per-module re-enforcement.
func anonPipelineHandler(desc helpers.CommandDescriptor, domain func(*gotgbot.Bot, *ext.Context) error) AnonymousAdminHandler {
	return func(b *gotgbot.Bot, ctx *ext.Context) error {
		c, err := helpers.BuildCommandContext(b, ctx)
		if err != nil {
			return ext.EndGroups
		}
		if !helpers.RunChecks(c, desc.RequiredChecks) {
			return ext.EndGroups
		}
		return domain(b, ctx)
	}
}
