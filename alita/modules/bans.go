package modules

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/extraction"
)

var bansModule = moduleStruct{moduleName: "Bans"}

// delayedUnban performs a delayed unban after kick with timeout protection.
// Runs in a goroutine to avoid blocking the main execution.
func delayedUnban(chat *gotgbot.Chat, b *gotgbot.Bot, userId int64, operation string) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.WithField("panic", r).Error("Panic in delayed unban goroutine")
			}
		}()

		// Create context with timeout to prevent goroutine from hanging indefinitely
		timeoutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		timer := time.NewTimer(2 * time.Second)
		defer timer.Stop()

		select {
		case <-timer.C:
			_, unbanErr := chat.UnbanMember(b, userId, nil)
			if unbanErr != nil {
				log.WithFields(log.Fields{
					"chatId": chat.Id,
					"userId": userId,
					"error":  unbanErr,
				}).Errorf("Failed to unban user after %s", operation)
			}
		case <-timeoutCtx.Done():
			log.WithFields(log.Fields{
				"chatId": chat.Id,
				"userId": userId,
			}).Warnf("%s unban operation timed out", cases.Title(language.English).String(operation))
		}
	}()
}

/* Used to Kick a user from group

The Bot, Kicker should be admin with ban permissions in order to use this */

// dkick handles the /dkick command to delete a message and kick the sender.
// Removes the replied-to message and kicks the user from the group.
func (m moduleStruct) dkick(b *gotgbot.Bot, ctx *ext.Context) error {
	return moderationDkick(&m).run(b, ctx)
}

// kickTargetValidation validates the target for kick commands.
// Checks: user in chat, not ban-protected, not the bot itself.
func kickTargetValidation(c *moderationCtx, t *target) error {
	if !chat_status.IsUserInChat(c.Bot, c.Chat, t.userID) {
		text, _ := c.Tr.GetString(strings.ToLower(c.Module.moduleName) + "_kick_user_not_in_chat")
		if text == "" {
			text, _ = c.Tr.GetString("common_user_not_in_chat")
		}
		_, err := c.Msg.Reply(c.Bot, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return errUserNotInChat
	}
	if chat_status.IsUserBanProtected(c.Bot, c.Ctx, nil, t.userID) {
		text, _ := c.Tr.GetString(strings.ToLower(c.Module.moduleName) + "_kick_cannot_kick_admin")
		if text == "" {
			text, _ = c.Tr.GetString("common_cannot_target_admin")
		}
		_, err := c.Msg.Reply(c.Bot, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return errAdminTarget
	}
	if t.userID == c.Bot.Id {
		text, _ := c.Tr.GetString(strings.ToLower(c.Module.moduleName) + "_kick_is_bot_itself")
		if text == "" {
			text, _ = c.Tr.GetString("common_cannot_target_self")
		}
		_, err := c.Msg.Reply(c.Bot, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return errTargetIsBot
	}
	return nil
}

// kickReply builds and sends the success reply for kick commands.
func kickReply(c *moderationCtx, t *target) error {
	kickuser, err := c.Bot.GetChat(t.userID, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	baseStr, _ := c.Tr.GetString(strings.ToLower(c.Module.moduleName) + "_kick_kicked_user")
	if t.reason != "" {
		temp, _ := c.Tr.GetString(strings.ToLower(c.Module.moduleName) + "_kick_kicked_reason")
		if temp != "" {
			baseStr += fmt.Sprintf(temp, t.reason)
		}
	}

	_, err = c.Msg.Reply(c.Bot,
		fmt.Sprintf(baseStr, formatting.MentionHtml(kickuser.Id, kickuser.FirstName)),
		formatting.Shtml(),
	)
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// banTargetValidation validates the target for ban commands.
// Checks: not ban-protected, not the bot itself.
func banTargetValidation(c *moderationCtx, t *target) error {
	if chat_status.IsUserBanProtected(c.Bot, c.Ctx, nil, t.userID) {
		text, _ := c.Tr.GetString(strings.ToLower(c.Module.moduleName) + "_ban_is_admin")
		if text == "" {
			text, _ = c.Tr.GetString("common_cannot_target_admin")
		}
		_, err := c.Msg.Reply(c.Bot, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return errAdminTarget
	}
	if t.userID == c.Bot.Id {
		text, _ := c.Tr.GetString(strings.ToLower(c.Module.moduleName) + "_ban_is_bot_itself")
		if text == "" {
			text, _ = c.Tr.GetString("common_cannot_target_self")
		}
		_, err := c.Msg.Reply(c.Bot, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return errTargetIsBot
	}
	return nil
}

// moderationDkick is the shared moderationCommand definition for /dkick.
// It deletes the replied message and kicks the user.
func moderationDkick(m *moduleStruct) *moderationCommand {
	return &moderationCommand{
		module:   m,
		gates:    []gateFn{deleteModGates},
		extract:  extractFromReply,
		validate: kickTargetValidation,
		execute: func(c *moderationCtx, t *target) error {
			_, err := c.Msg.ReplyToMessage.Delete(c.Bot, nil)
			if err != nil {
				log.Error(err)
				return err
			}
			_, err = c.Chat.BanMember(c.Bot, t.userID, nil)
			if err == nil {
				delayedUnban(c.Chat, c.Bot, t.userID, "dkick")
			}
			return err
		},
		reply: kickReply,
	}
}

// moderationTban is the shared moderationCommand definition for /tban.
func moderationTban(m *moduleStruct) *moderationCommand {
	return &moderationCommand{
		module:   m,
		gates:    []gateFn{standardModGates},
		extract:  extractFromArgs,
		validate: banTargetValidation,
		execute: func(c *moderationCtx, t *target) error {
			_time, timeVal, reason := extraction.ExtractTime(c.Bot, c.Ctx, t.reason)
			if _time == -1 {
				return ext.EndGroups
			}
			t.timeVal = timeVal
			t.reason = reason
			_, err := c.Chat.BanMember(c.Bot, t.userID, &gotgbot.BanChatMemberOpts{UntilDate: _time})
			return err
		},
		reply: func(c *moderationCtx, t *target) error {
			banUser, err := c.Bot.GetChat(t.userID, nil)
			if err != nil {
				log.Error(err)
				return err
			}

			temp, _ := c.Tr.GetString(strings.ToLower(c.Module.moduleName) + "_ban_tban")
			baseStr := fmt.Sprintf(temp, formatting.MentionHtml(banUser.Id, banUser.FirstName), t.timeVal)
			if t.reason != "" {
				temp, _ := c.Tr.GetString(strings.ToLower(c.Module.moduleName) + "_ban_ban_reason")
				if temp != "" {
					baseStr += fmt.Sprintf(temp, t.reason)
				}
			}

			_, err = c.Msg.Reply(c.Bot, baseStr, formatting.Shtml())
			if err != nil {
				log.Error(err)
				return err
			}
			return nil
		},
	}
}

// banReplyWithButton builds and sends the success reply for ban commands
// with an inline unban button.
func banReplyWithButton(c *moderationCtx, t *target) error {
	banUser, err := c.Bot.GetChat(t.userID, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	baseStr, _ := c.Tr.GetString(strings.ToLower(c.Module.moduleName) + "_ban_normal_ban")
	if t.reason != "" {
		temp, _ := c.Tr.GetString(strings.ToLower(c.Module.moduleName) + "_ban_ban_reason")
		baseStr += fmt.Sprintf(temp, t.reason)
	}

	text := fmt.Sprintf(baseStr, formatting.MentionHtml(banUser.Id, banUser.FirstName))

	_, err = c.Msg.Reply(c.Bot, text,
		&gotgbot.SendMessageOpts{
			ParseMode: formatting.HTML,
			ReplyMarkup: gotgbot.InlineKeyboardMarkup{
				InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
					{
						{
							Text:         func() string { t, _ := c.Tr.GetString("bans_unban_button"); return t }(),
							CallbackData: encodeCallbackData("unrestrict", map[string]string{"a": "unban", "u": fmt.Sprint(t.userID)}, fmt.Sprintf("unrestrict.unban.%d", t.userID)),
						},
					},
				},
			},
		},
	)
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// moderationBan is the shared moderationCommand definition for /ban.
// Handles both regular users and anonymous channels with inline unban button.
func moderationBan(m *moduleStruct) *moderationCommand {
	return &moderationCommand{
		module: m,
		gates:  []gateFn{standardModGates},
		extract: func(c *moderationCtx) (target, error) {
			uid, reason := extraction.ExtractUserAndText(c.Bot, c.Ctx)
			if uid == -1 {
				return target{}, fmt.Errorf("extraction failed")
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
			return target{userID: uid, reason: reason, isChannel: chat_status.IsChannelId(uid)}, nil
		},
		validate: func(c *moderationCtx, t *target) error {
			if t.isChannel {
				return nil
			}
			return banTargetValidation(c, t)
		},
		execute: func(c *moderationCtx, t *target) error {
			if t.isChannel {
				if c.Msg.ReplyToMessage != nil {
					t.userID = c.Msg.ReplyToMessage.GetSender().Id()
					_, err := c.Bot.BanChatSenderChat(c.Chat.Id, t.userID, nil)
					return err
				}
				return nil
			}
			_, err := c.Chat.BanMember(c.Bot, t.userID, nil)
			return err
		},
		reply: func(c *moderationCtx, t *target) error {
			var text string
			var sendMsgOptns *gotgbot.SendMessageOpts

			if t.isChannel {
				if c.Msg.ReplyToMessage != nil {
					temp, _ := c.Tr.GetString("bans_anonymous_ban_user")
					text = fmt.Sprintf(temp, formatting.MentionHtml(t.userID, c.Msg.ReplyToMessage.GetSender().Name()))
				} else {
					text, _ = c.Tr.GetString("bans_anonymous_ban_reply_only")
				}
				sendMsgOptns = formatting.Shtml()
			} else {
				return banReplyWithButton(c, t)
			}
			_, err := c.Msg.Reply(c.Bot, text, sendMsgOptns)
			if err != nil {
				log.Error(err)
				return err
			}
			return nil
		},
	}
}

// moderationKick is the shared moderationCommand definition for /kick.
// It is reused by both the direct handler and any aliases.
func moderationKick(m *moduleStruct) *moderationCommand {
	return &moderationCommand{
		module:   m,
		gates:    []gateFn{standardModGates},
		extract:  extractFromArgs,
		validate: kickTargetValidation,
		execute: func(c *moderationCtx, t *target) error {
			_, err := c.Chat.BanMember(c.Bot, t.userID, nil)
			if err == nil {
				delayedUnban(c.Chat, c.Bot, t.userID, "kick")
			}
			return err
		},
		reply: kickReply,
	}
}

// moderationKickme is the shared moderationCommand definition for /kickme.
func moderationKickme(m *moduleStruct) *moderationCommand {
	return &moderationCommand{
		module: m,
		gates: []gateFn{
			func(c *moderationCtx) bool {
				if !chat_status.RequireGroup(c.Bot, c.Ctx, nil) {
					chat_status.NewPermissionResponder(c.Bot).Respond(c.Ctx, "chat_status_group_only_error", "", chat_status.WithReply())
					return false
				}
				if !chat_status.CanBotRestrict(c.Bot, c.Ctx, nil) {
					chat_status.NewPermissionResponder(c.Bot).Respond(c.Ctx, "chat_status_bot_restrict_group_error", "chat_status_bot_restrict_error")
					return false
				}
				return true
			},
		},
		extract: func(c *moderationCtx) (target, error) {
			// Don't allow admins to use the command
			if chat_status.IsUserAdmin(c.Bot, c.Chat.Id, c.User.Id) {
				text, _ := c.Tr.GetString(strings.ToLower(c.Module.moduleName) + "_kickme_is_admin")
				_, err := c.Msg.Reply(c.Bot, text, formatting.Shtml())
				if err != nil {
					log.Error(err)
					return target{}, err
				}
				return target{}, fmt.Errorf("user is admin")
			}
			return target{userID: c.User.Id}, nil
		},
		execute: func(c *moderationCtx, t *target) error {
			_, err := c.Chat.BanMember(c.Bot, t.userID, nil)
			if err != nil {
				return err
			}
			delayedUnban(c.Chat, c.Bot, t.userID, "kickme")
			return nil
		},
		reply: func(c *moderationCtx, t *target) error {
			text, _ := c.Tr.GetString(strings.ToLower(c.Module.moduleName) + "_kickme_ok_out")
			_, err := c.Msg.Reply(c.Bot, text, formatting.Shtml())
			if err != nil {
				log.Error(err)
				return err
			}
			return nil
		},
	}
}

// moderationSban is the shared moderationCommand definition for /sban.
func moderationSban(m *moduleStruct) *moderationCommand {
	return &moderationCommand{
		module:   m,
		gates:    []gateFn{deleteModGates},
		extract:  extractUserOnly,
		validate: banTargetValidation,
		execute: func(c *moderationCtx, t *target) error {
			_, err := c.Chat.BanMember(c.Bot, t.userID, nil)
			if err != nil {
				return err
			}
			_, err = c.Msg.Delete(c.Bot, nil)
			return err
		},
		reply: nil,
	}
}

// moderationDban is the shared moderationCommand definition for /dban.
func moderationDban(m *moduleStruct) *moderationCommand {
	return &moderationCommand{
		module:  m,
		gates:   []gateFn{deleteModGates},
		extract: extractFromArgs,
		validate: func(c *moderationCtx, t *target) error {
			if c.Msg.ReplyToMessage == nil {
				text, _ := c.Tr.GetString(strings.ToLower(c.Module.moduleName) + "_ban_dban_no_reply")
				if text == "" {
					text, _ = c.Tr.GetString("common_no_reply_to_message")
				}
				_, err := c.Msg.Reply(c.Bot, text, formatting.Shtml())
				if err != nil {
					log.Error(err)
					return err
				}
				return fmt.Errorf("no reply")
			}
			return banTargetValidation(c, t)
		},
		execute: func(c *moderationCtx, t *target) error {
			_, err := c.Msg.ReplyToMessage.Delete(c.Bot, nil)
			if err != nil {
				log.Error(err)
				return err
			}
			_, err = c.Chat.BanMember(c.Bot, t.userID, nil)
			return err
		},
		reply: banReplyWithButton,
	}
}

// moderationUnban is the shared moderationCommand definition for /unban.
// Supports both regular users and anonymous channels.
func moderationUnban(m *moduleStruct) *moderationCommand {
	return &moderationCommand{
		module: m,
		gates:  []gateFn{standardModGates},
		extract: func(c *moderationCtx) (target, error) {
			uid := extraction.ExtractUser(c.Bot, c.Ctx)
			if uid == -1 {
				return target{}, fmt.Errorf("extraction failed")
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
			return target{userID: uid, isChannel: chat_status.IsChannelId(uid)}, nil
		},
		validate: func(c *moderationCtx, t *target) error {
			if t.userID == c.Bot.Id {
				text, _ := c.Tr.GetString(strings.ToLower(c.Module.moduleName) + "_unban_is_bot_itself")
				if text == "" {
					text, _ = c.Tr.GetString("common_cannot_target_self")
				}
				_, err := c.Msg.Reply(c.Bot, text, formatting.Shtml())
				if err != nil {
					log.Error(err)
					return err
				}
				return errTargetIsBot
			}
			return nil
		},
		execute: func(c *moderationCtx, t *target) error {
			if t.isChannel {
				if c.Msg.ReplyToMessage != nil {
					t.userID = c.Msg.ReplyToMessage.GetSender().Id()
					_, err := c.Bot.UnbanChatSenderChat(c.Chat.Id, t.userID, nil)
					return err
				}
				return nil
			}
			_, err := c.Chat.UnbanMember(c.Bot, t.userID, nil)
			return err
		},
		reply: func(c *moderationCtx, t *target) error {
			var text string
			if t.isChannel {
				if c.Msg.ReplyToMessage != nil {
					temp, _ := c.Tr.GetString("bans_anonymous_unban_user")
					text = fmt.Sprintf(temp, formatting.MentionHtml(t.userID, c.Msg.ReplyToMessage.GetSender().Name()))
				} else {
					text, _ = c.Tr.GetString("bans_anonymous_unban_reply_only")
				}
			} else {
				banUser, err := c.Bot.GetChat(t.userID, nil)
				if err != nil {
					log.Error(err)
					return err
				}
				temp, _ := c.Tr.GetString(strings.ToLower(c.Module.moduleName) + "_unban_unbanned_user")
				text = fmt.Sprintf(temp, formatting.MentionHtml(banUser.Id, banUser.FirstName))
			}
			_, err := c.Msg.Reply(c.Bot, text, formatting.Shtml())
			if err != nil {
				log.Error(err)
				return err
			}
			return nil
		},
	}
}

// kick delegates to the shared moderationKick command template.
func (m moduleStruct) kick(b *gotgbot.Bot, ctx *ext.Context) error {
	return moderationKick(&m).run(b, ctx)
}

// kickme handles the /kickme command allowing users to remove themselves.
// Only works for non-admin users who want to leave the group.
func (m moduleStruct) kickme(b *gotgbot.Bot, ctx *ext.Context) error {
	return moderationKickme(&m).run(b, ctx)
}

/* Used to temporarily ban a user from chat

The Bot, Kick should be admin with ban permissions in order to use this */

// tBan handles the /tban command to temporarily ban a user.
// Bans a user for a specified time period with optional reason.
func (m moduleStruct) tBan(b *gotgbot.Bot, ctx *ext.Context) error {
	return moderationTban(&m).run(b, ctx)
}

/* Used to indefinitely ban a user from group

The Bot, Banner should be admin with ban permissions in order to use this */

// ban handles the /ban command to permanently ban a user from the group.
// Supports both regular users and anonymous channels with inline unban button.
func (m moduleStruct) ban(b *gotgbot.Bot, ctx *ext.Context) error {
	return moderationBan(&m).run(b, ctx)
}

/* Used to Silently Ban a user from group

This deletes the command of Banner and also does not reply.

The Bot, Banner should be admin with ban permissions in order to use this */

// sBan handles the /sban command to silently ban a user.
// Bans the user and deletes the command message without notification.
func (m moduleStruct) sBan(b *gotgbot.Bot, ctx *ext.Context) error {
	return moderationSban(&m).run(b, ctx)
}

// dBan handles the /dban command to delete a message and ban the sender.
// Removes the replied-to message and permanently bans the user.
func (m moduleStruct) dBan(b *gotgbot.Bot, ctx *ext.Context) error {
	return moderationDban(&m).run(b, ctx)
}

// unban handles the /unban command to remove a ban from a user.
// Supports both regular users and anonymous channels.
func (m moduleStruct) unban(b *gotgbot.Bot, ctx *ext.Context) error {
	return moderationUnban(&m).run(b, ctx)
}

/* Used to Restrict members from a chat
Shows an inline keyboard menu which shows options to kick, ban and mute */

// restrict handles the /restrict command to show restriction options.
// Displays an inline keyboard with ban, kick, and mute options for a user.
func (moduleStruct) restrict(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	msg := ctx.EffectiveMessage

	// Permission checks
	if !chat_status.RequireGroup(b, ctx, chat) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_group_only_error", "", chat_status.WithReply())
		return ext.EndGroups
	}
	if !chat_status.CanUserRestrict(b, ctx, chat, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_restrict_cmd_error", "chat_status_restrict_button_error")
		return ext.EndGroups
	}
	if !chat_status.CanBotRestrict(b, ctx, chat) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_bot_restrict_group_error", "chat_status_bot_restrict_error")
		return ext.EndGroups
	}

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	userId := extraction.ExtractUser(b, ctx)
	switch userId {
	case -1:
		return ext.EndGroups
	case 0:
		text, _ := tr.GetString("common_no_user_specified")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	// User should be in chat for getting restricted
	if !chat_status.IsUserInChat(b, chat, userId) {
		text, _ := tr.GetString("common_user_not_in_chat")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if chat_status.IsUserBanProtected(b, ctx, nil, userId) {
		text, _ := tr.GetString("bans_restrict_admin_error")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if userId == b.Id {
		text, _ := tr.GetString("bans_restrict_self_error")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	text, _ := tr.GetString("bans_restrict_question")
	_, err := msg.Reply(b, text,
		&gotgbot.SendMessageOpts{
			ReplyMarkup: gotgbot.InlineKeyboardMarkup{
				InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
					{
						{
							Text:         func() string { t, _ := tr.GetString("button_ban"); return t }(),
							CallbackData: encodeCallbackData("restrict", map[string]string{"a": "ban", "u": fmt.Sprint(userId)}, fmt.Sprintf("restrict.ban.%d", userId)),
						},
						{
							Text:         func() string { t, _ := tr.GetString("button_kick"); return t }(),
							CallbackData: encodeCallbackData("restrict", map[string]string{"a": "kick", "u": fmt.Sprint(userId)}, fmt.Sprintf("restrict.kick.%d", userId)),
						},
					},
					{{
						Text:         func() string { t, _ := tr.GetString("button_mute"); return t }(),
						CallbackData: encodeCallbackData("restrict", map[string]string{"a": "mute", "u": fmt.Sprint(userId)}, fmt.Sprintf("restrict.mute.%d", userId)),
					}},
				},
			},
		},
	)
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// Handles the queries fore restrict command
// restrictButtonHandler processes inline keyboard callbacks for restriction actions.
// Handles ban, kick, and mute actions triggered from the restrict command keyboard.
func (moduleStruct) restrictButtonHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	query, ok := callbackQueryFromContext(ctx)
	if !ok {
		return ext.EndGroups
	}
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	// permissions check
	if !chat_status.CanUserRestrict(b, ctx, chat, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_restrict_cmd_error", "chat_status_restrict_button_error")
		return ext.EndGroups
	}

	action := ""
	userIDRaw := ""
	if decoded, ok := decodeCallbackData(query.Data, "restrict"); ok {
		action, _ = decoded.Field("a")
		userIDRaw, _ = decoded.Field("u")
	} else {
		args := strings.Split(query.Data, ".")
		if len(args) >= 3 {
			action = args[1]
			userIDRaw = args[2]
		}
	}
	if action == "" || userIDRaw == "" {
		log.WithField("callbackData", query.Data).Error("Malformed restrict callback data")
		errText, _ := tr.GetString("bans_invalid_callback_data")
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
			Text:      errText,
			ShowAlert: true,
		})
		return ext.EndGroups
	}
	userId, err := strconv.Atoi(userIDRaw)
	if err != nil {
		log.WithFields(log.Fields{
			"callbackData": query.Data,
			"error":        err,
		}).Error("Failed to parse userId from restrict callback")
		errText, _ := tr.GetString("bans_invalid_user_id")
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
			Text:      errText,
			ShowAlert: true,
		})
		return ext.EndGroups
	}

	var helpText string

	actionUser, err := b.GetChat(int64(userId), nil)
	if err != nil {
		log.Error(err)
		return err
	}

	switch action {
	case "kick":
		_, err := chat.BanMember(b, int64(userId), nil)
		if err != nil {
			log.Error(err)
			return err
		}
		temp, _ := tr.GetString("bans_restrict_kicked")
		helpText = fmt.Sprintf(temp,
			formatting.MentionHtml(user.Id, user.FirstName),
			formatting.MentionHtml(int64(userId), actionUser.FirstName),
		)
		// Use non-blocking delayed unban for restrict kick action with timeout
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.WithField("panic", r).Error("Panic in restrict delayed unban goroutine")
				}
			}()

			// Create context with timeout to prevent goroutine from hanging indefinitely
			timeoutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			timer := time.NewTimer(3 * time.Second)
			defer timer.Stop()

			select {
			case <-timer.C:
				_, unbanErr := chat.UnbanMember(b, int64(userId), nil)
				if unbanErr != nil {
					log.WithFields(log.Fields{
						"chatId": chat.Id,
						"userId": userId,
						"error":  unbanErr,
					}).Error("Failed to unban user after restrict kick")
				}
			case <-timeoutCtx.Done():
				log.WithFields(log.Fields{
					"chatId": chat.Id,
					"userId": userId,
				}).Warn("Restrict kick unban operation timed out")
			}
		}()
	case "mute":
		_, err := chat.RestrictMember(b, int64(userId),
			MutedPermissions,
			nil,
		)
		if err != nil {
			log.Error(err)
			return err
		}
		temp, _ := tr.GetString("bans_restrict_muted")
		helpText = fmt.Sprintf(temp,
			formatting.MentionHtml(user.Id, user.FirstName),
			formatting.MentionHtml(int64(userId), actionUser.FirstName),
		)
	case "ban":
		_, err := chat.BanMember(b, int64(userId), &gotgbot.BanChatMemberOpts{})
		if err != nil {
			log.Error(err)
			return err
		}
		temp, _ := tr.GetString("bans_restrict_banned")
		helpText = fmt.Sprintf(temp,
			formatting.MentionHtml(user.Id, user.FirstName),
			formatting.MentionHtml(int64(userId), actionUser.FirstName),
		)
	}

	_, _, err = query.Message.EditText(b,
		helpText,
		&gotgbot.EditMessageTextOpts{
			ParseMode: formatting.HTML,
		},
	)
	if err != nil {
		log.Error(err)
		return err
	}
	_, err = query.Answer(b, nil)
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}

/* Used to Unrestrict members from a chat
Shows an inline keyboard menu which shows options to unban and unmute */

// unrestrict handles the /unrestrict command to show unrestriction options.
// Displays an inline keyboard with unban and unmute options for a user.
func (moduleStruct) unrestrict(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	// Permission checks
	if !chat_status.RequireGroup(b, ctx, chat) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_group_only_error", "", chat_status.WithReply())
		return ext.EndGroups
	}
	if !chat_status.CanUserRestrict(b, ctx, chat, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_restrict_cmd_error", "chat_status_restrict_button_error")
		return ext.EndGroups
	}
	if !chat_status.CanBotRestrict(b, ctx, chat) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_bot_restrict_group_error", "chat_status_bot_restrict_error")
		return ext.EndGroups
	}

	userId := extraction.ExtractUser(b, ctx)
	switch userId {
	case -1:
		return ext.EndGroups
	case 0:
		text, _ := tr.GetString("common_no_user_specified")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	// User should be in chat for getting restricted
	if !chat_status.IsUserInChat(b, chat, userId) {
		text, _ := tr.GetString("common_user_not_in_chat")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if chat_status.IsUserBanProtected(b, ctx, nil, userId) {
		text, _ := tr.GetString("bans_unrestrict_admin_error")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if userId == b.Id {
		text, _ := tr.GetString("bans_unrestrict_self_error")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	text, _ := tr.GetString("bans_unrestrict_question")
	unbanText, _ := tr.GetString("button_unban")
	unmuteText, _ := tr.GetString("button_unmute")
	_, err := msg.Reply(b, text,
		&gotgbot.SendMessageOpts{
			ReplyMarkup: gotgbot.InlineKeyboardMarkup{
				InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
					{
						{
							Text:         unbanText,
							CallbackData: encodeCallbackData("unrestrict", map[string]string{"a": "unban", "u": fmt.Sprint(userId)}, fmt.Sprintf("unrestrict.unban.%d", userId)),
						},
						{
							Text:         unmuteText,
							CallbackData: encodeCallbackData("unrestrict", map[string]string{"a": "unmute", "u": fmt.Sprint(userId)}, fmt.Sprintf("unrestrict.unmute.%d", userId)),
						},
					},
				},
			},
		},
	)
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// Handles queries for unrestrict command
// unrestrictButtonHandler processes inline keyboard callbacks for unrestriction actions.
// Handles unban and unmute actions triggered from the unrestrict command keyboard.
func (moduleStruct) unrestrictButtonHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	query, ok := callbackQueryFromContext(ctx)
	if !ok {
		return ext.EndGroups
	}
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	msg := query.Message
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	// permissions check
	if !chat_status.CanUserRestrict(b, ctx, chat, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_restrict_cmd_error", "chat_status_restrict_button_error")
		return ext.EndGroups
	}

	action := ""
	userIDRaw := ""
	if decoded, ok := decodeCallbackData(query.Data, "unrestrict"); ok {
		action, _ = decoded.Field("a")
		userIDRaw, _ = decoded.Field("u")
	} else {
		args := strings.Split(query.Data, ".")
		if len(args) >= 3 {
			action = args[1]
			userIDRaw = args[2]
		}
	}
	if action == "" || userIDRaw == "" {
		log.WithField("callbackData", query.Data).Error("Malformed unrestrict callback data")
		errText, _ := tr.GetString("bans_invalid_callback_data")
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
			Text:      errText,
			ShowAlert: true,
		})
		return ext.EndGroups
	}
	userId, err := strconv.Atoi(userIDRaw)
	if err != nil {
		log.WithFields(log.Fields{
			"callbackData": query.Data,
			"error":        err,
		}).Error("Failed to parse userId from unrestrict callback")
		errText, _ := tr.GetString("bans_invalid_user_id")
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
			Text:      errText,
			ShowAlert: true,
		})
		return ext.EndGroups
	}

	var helpText string

	switch action {
	case "unmute":

		c, err := b.GetChat(chat.Id, nil)
		if err != nil {
			log.Error(err)
			return err
		}
		unmutePermissions := resolveUnmutePermissions(c)

		_, err = chat.RestrictMember(b, int64(userId),
			unmutePermissions,
			nil,
		)
		if err != nil {
			log.Error(err)
			return err
		}

		temp, _ := tr.GetString("bans_unrestrict_unmuted")
		helpText = fmt.Sprintf(temp, formatting.MentionHtml(user.Id, user.FirstName))
	case "unban":
		_, err := chat.Unban(b,
			int64(userId),
			&gotgbot.UnbanChatMemberOpts{
				OnlyIfBanned: true,
			},
		)
		if err != nil {
			log.Error(err)
			return err
		}

		temp, _ := tr.GetString("bans_unrestrict_unbanned")
		helpText = fmt.Sprintf(temp, formatting.MentionHtml(user.Id, user.FirstName))
	}

	// type assertion to get the message
	_updatedMsg, ok := msg.(*gotgbot.Message)
	if !ok || _updatedMsg == nil {
		log.Warn("[Bans] Could not cast message for strikethrough formatting")
		return ext.EndGroups
	}

	// only strikethrough if msg.Text is non-empty
	if _updatedMsg.Text != "" {
		_updatedMsg.Text = fmt.Sprint("<s>", _updatedMsg.Text, "</s>", "\n\n")
	}

	_, _, err = msg.EditText(
		b,
		fmt.Sprint(_updatedMsg.Text, helpText),
		&gotgbot.EditMessageTextOpts{
			ParseMode: formatting.HTML,
		},
	)
	if err != nil {
		log.Error(err)
		return err
	}

	_, err = query.Answer(b, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// LoadBans registers all ban-related command handlers with the dispatcher.
// Sets up ban, kick, restrict commands and their associated callback handlers.
func LoadBans(dispatcher *ext.Dispatcher) {
	DefaultHelpRegistry().AbleMap.Store(bansModule.moduleName, true)

	// ban cmds
	dispatcher.AddHandler(handlers.NewCommand("ban", bansModule.ban))
	dispatcher.AddHandler(handlers.NewCommand("sban", bansModule.sBan))
	dispatcher.AddHandler(handlers.NewCommand("tban", bansModule.tBan))
	dispatcher.AddHandler(handlers.NewCommand("dban", bansModule.dBan))
	dispatcher.AddHandler(handlers.NewCommand("unban", bansModule.unban))

	// kick cmds
	dispatcher.AddHandler(handlers.NewCommand("kick", bansModule.kick))
	dispatcher.AddHandler(handlers.NewCommand("dkick", bansModule.dkick))
	dispatcher.AddHandler(handlers.NewCommand("kickme", bansModule.kickme))

	// special commands
	dispatcher.AddHandler(handlers.NewCommand("restrict", bansModule.restrict))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("restrict"), bansModule.restrictButtonHandler))
	dispatcher.AddHandler(handlers.NewCommand("unrestrict", bansModule.unrestrict))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("unrestrict"), bansModule.unrestrictButtonHandler))
}

func init() {
	RegisterLegacyModule("Bans", 70, LoadBans)
}
