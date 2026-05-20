package modules

import (
	"fmt"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/extraction"
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
}

// gateFn is a permission gate. Returns true if the gate passes.
// On failure, it should already have sent any required error messages.
type gateFn func(c *moderationCtx) bool

// standardModGates are the most common permission checks for moderation commands.
// RequireGroup -> RequireUserAdmin -> RequireBotAdmin -> CanUserRestrict -> CanBotRestrict
func standardModGates(c *moderationCtx) bool {
	if !chat_status.RequireGroup(c.Bot, c.Ctx, nil, false) {
		return false
	}
	if !chat_status.RequireUserAdmin(c.Bot, c.Ctx, nil, c.User.Id, false) {
		return false
	}
	if !chat_status.RequireBotAdmin(c.Bot, c.Ctx, nil, false) {
		return false
	}
	if !chat_status.CanUserRestrict(c.Bot, c.Ctx, nil, c.User.Id, false) {
		return false
	}
	if !chat_status.CanBotRestrict(c.Bot, c.Ctx, nil, false) {
		return false
	}
	return true
}

// deleteModGates extends standardModGates with delete permissions.
// Used by purge-like commands.
//nolint:unused
func deleteModGates(c *moderationCtx) bool {
	if !standardModGates(c) {
		return false
	}
	if !chat_status.CanBotDelete(c.Bot, c.Ctx, nil, false) {
		return false
	}
	if !chat_status.CanUserDelete(c.Bot, c.Ctx, c.Chat, c.User.Id, false) {
		return false
	}
	return true
}

// target holds the resolved target user and optional reason text.
type target struct {
	userID int64
	reason string
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
		anonKey := "bans_anonymous_ban_only_error"
		text, _ := c.Tr.GetString(anonKey)
		if text == "" {
			text, _ = c.Tr.GetString("common_anonymous_user_error")
		}
		_, err := c.Msg.Reply(c.Bot, text, nil)
		if err != nil {
			log.Error(err)
			return target{}, err
		}
		return target{}, fmt.Errorf("anonymous user")
	}
	if uid == 0 {
		noUserKey := "common_no_user_specified"
		if c.Module != nil && strings.ToLower(c.Module.moduleName) == "mutes" {
			noUserKey = "common_no_user_specified"
		}
		text, _ := c.Tr.GetString(noUserKey)
		_, err := c.Msg.Reply(c.Bot, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return target{}, err
		}
		return target{}, fmt.Errorf("no user")
	}
	return target{userID: uid, reason: reason}, nil
}

// extractFromReply resolves the target from the replied-to message.
//nolint:unused
func extractFromReply(c *moderationCtx) (target, error) {
	if c.Msg.ReplyToMessage == nil {
		text, _ := c.Tr.GetString("bans_dkick_no_reply")
		if text == "" {
			text, _ = c.Tr.GetString("common_no_reply_to_message")
		}
		_, err := c.Msg.Reply(c.Bot, text, nil)
		if err != nil {
			log.Error(err)
			return target{}, err
		}
		return target{}, fmt.Errorf("no reply")
	}
	if c.Msg.ReplyToMessage.From == nil {
		text, _ := c.Tr.GetString("bans_cannot_identify_user")
		_, err := c.Msg.Reply(c.Bot, text, nil)
		if err != nil {
			log.Error(err)
			return target{}, err
		}
		return target{}, fmt.Errorf("nil from")
	}
	return target{userID: c.Msg.ReplyToMessage.From.Id}, nil
}

// validateTargetFn checks the resolved target before executing the action.
// The function receives the moderationCtx and resolved target; on failure
// it sends the i18n error message and returns a non-nil error.
type validateTargetFn func(c *moderationCtx, t *target) error

// defaultTargetValidation checks:
//  1. user is in chat
//  2. user is not ban-protected
//  3. user is not the bot itself
// defaultTargetValidation checks the resolved target before executing the action.
//nolint:unused
func defaultTargetValidation(c *moderationCtx, t *target) error {
	if !chat_status.IsUserInChat(c.Bot, c.Chat, t.userID) {
		text, _ := c.Tr.GetString(strings.ToLower(c.Module.moduleName) + "_user_not_in_chat")
		if text == "" {
			text, _ = c.Tr.GetString("common_user_not_in_chat")
		}
		_, err := c.Msg.Reply(c.Bot, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return fmt.Errorf("user not in chat")
	}
	if chat_status.IsUserBanProtected(c.Bot, c.Ctx, nil, t.userID) {
		text, _ := c.Tr.GetString(strings.ToLower(c.Module.moduleName) + "_cannot_target_admin")
		if text == "" {
			text, _ = c.Tr.GetString("common_cannot_target_admin")
		}
		_, err := c.Msg.Reply(c.Bot, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return fmt.Errorf("target is admin")
	}
	if t.userID == c.Bot.Id {
		text, _ := c.Tr.GetString(strings.ToLower(c.Module.moduleName) + "_self_error")
		if text == "" {
			text, _ = c.Tr.GetString("common_cannot_target_self")
		}
		_, err := c.Msg.Reply(c.Bot, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return fmt.Errorf("target is bot")
	}
	return nil
}

// actionFn performs the moderation API call.
// It receives the moderationCtx and the validated target.
type actionFn func(c *moderationCtx, t *target) error

// replyFn builds and sends the success reply for a moderation command.
// It receives the moderationCtx, the validated target, and the API action result.
type replyFn func(c *moderationCtx, t *target) error

// moderationCommand wires the fixed-order pipeline for a moderation handler:
//   RequireUser -> gates -> extractTarget -> validate -> execute -> reply.
//
// It eliminates the 30+ lines of boilerplate that every moderation command repeats.
type moderationCommand struct {
	gates     []gateFn
	extract   func(*moderationCtx) (target, error)
	validate  validateTargetFn
	execute   actionFn
	reply     replyFn
	module    *moduleStruct
}

// buildModerationCtx creates the common decomposed context from a gotgbot update.
func buildModerationCtx(m *moduleStruct, b *gotgbot.Bot, ctx *ext.Context) (*moderationCtx, error) {
	user := chat_status.RequireUser(b, ctx, false)
	if user == nil {
		return nil, ext.EndGroups //nolint:goerr113 // EndGroups is not a real error
	}
	return &moderationCtx{
		Bot:    b,
		Chat:   ctx.EffectiveChat,
		Msg:    ctx.EffectiveMessage,
		User:   user,
		Ctx:    ctx,
		Tr:     i18n.MustNewTranslator(db.GetLanguage(ctx)),
		Module: m,
	}, nil
}

// run executes the moderation pipeline.
func (cmd *moderationCommand) run(b *gotgbot.Bot, ctx *ext.Context) error {
	mc, err := buildModerationCtx(cmd.module, b, ctx)
	if err != nil {
		return ext.EndGroups
	}

	// Run all permission gates in order.
	for _, g := range cmd.gates {
		if !g(mc) {
			return ext.EndGroups
		}
	}

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

	// Build and send the success reply.
	if cmd.reply != nil {
		if err := cmd.reply(mc, &tgt); err != nil {
			return ext.EndGroups
		}
	}
	return ext.EndGroups
}

// MentionHtml is a local alias to avoid importing helpers in every moderation handler file.
// _mentionHtml is a local wrapper.
//nolint:unused
func _mentionHtml(userId int64, name string) string {
	return helpers.MentionHtml(userId, name)
}

// Sentinel errors used by moderation command templates.
var (
	errUserNotInChat = fmt.Errorf("target user is not in chat")
	errAdminTarget   = fmt.Errorf("target is a protected admin")
	errTargetIsBot   = fmt.Errorf("target is the bot itself")
)

