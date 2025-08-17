package modules

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/extraction"
)

var bansModule = moduleStruct{moduleName: "Bans"}

/* Used to Kick a user from group

The Bot, Kicker should be admin with ban permissions in order to use this */

// dkick handles the /dkick command to delete a message and kick the sender.
// Removes the replied-to message and kicks the user from the group.
func (m moduleStruct) dkick(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	user := ctx.EffectiveSender.User
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	// Permission checks
	if !chat_status.RequireGroup(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireBotAdmin(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.CanUserRestrict(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}
	if !chat_status.CanBotRestrict(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.CanBotDelete(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.CanUserDelete(b, ctx, chat, user.Id, false) {
		return ext.EndGroups
	}
	if msg.ReplyToMessage == nil {
		text, _ := tr.GetString("bans_dkick_reply_error")
		_, _ = msg.Reply(b, text, nil)
		return ext.EndGroups
	}

	_, reason := extraction.ExtractUserAndText(b, ctx)
	userId := msg.ReplyToMessage.From.Id
	if userId == -1 {
		return ext.EndGroups
	} else if helpers.IsChannelID(userId) {
		text, _ := tr.GetString("bans_anonymous_ban_only_error")
		_, err := msg.Reply(b, text, nil)
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	} else if userId == 0 {
		text, _ := tr.GetString("bans_no_user_specified")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	_, _ = msg.ReplyToMessage.Delete(b, nil)

	// User should be in chat for getting restricted
	if !chat_status.IsUserInChat(b, chat, userId) {
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_kick_user_not_in_chat")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}
	if chat_status.IsUserBanProtected(b, ctx, nil, userId) {
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_kick_cannot_kick_admin")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if userId == b.Id {
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_kick_is_bot_itself")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	_, err := chat.BanMember(b, userId, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	// Use non-blocking approach with goroutine for delayed unban with timeout
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.WithField("panic", r).Error("Panic in delayed unban goroutine")
			}
		}()

		// Create context with timeout
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
				}).Error("Failed to unban user after dkick")
			}
		case <-timeoutCtx.Done():
			log.WithFields(log.Fields{
				"chatId": chat.Id,
				"userId": userId,
			}).Warn("Dkick unban operation timed out")
		}
	}()

	// Continue immediately without blocking

	kickuser, err := b.GetChat(userId, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	baseStr, _ := tr.GetString(strings.ToLower(m.moduleName) + "_kick_kicked_user")
	if reason != "" {
		temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_kick_kicked_reason")
		baseStr += fmt.Sprintf(temp, reason)
	}

	_, err = msg.Reply(b,
		fmt.Sprintf(baseStr, helpers.MentionHtml(kickuser.Id, kickuser.FirstName)),
		helpers.Shtml(),
	)
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// kick handles the /kick command to remove a user from the group.
// Temporarily bans then unbans the user to achieve a kick effect.
func (m moduleStruct) kick(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	user := ctx.EffectiveSender.User
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	// Permission checks
	if !chat_status.RequireGroup(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireBotAdmin(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.CanUserRestrict(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}
	if !chat_status.CanBotRestrict(b, ctx, nil, false) {
		return ext.EndGroups
	}

	userId, reason := extraction.ExtractUserAndText(b, ctx)
	if userId == -1 {
		return ext.EndGroups
	} else if helpers.IsChannelID(userId) {
		text, _ := tr.GetString("bans_anonymous_ban_only_error")
		_, err := msg.Reply(b, text, nil)
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	} else if userId == 0 {
		text, _ := tr.GetString("bans_no_user_specified")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	// User should be in chat for getting restricted
	if !chat_status.IsUserInChat(b, chat, userId) {
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_kick_user_not_in_chat")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}
	if chat_status.IsUserBanProtected(b, ctx, nil, userId) {
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_kick_cannot_kick_admin")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if userId == b.Id {
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_kick_is_bot_itself")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	_, err := chat.BanMember(b, userId, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	// Use non-blocking approach with goroutine for delayed unban with timeout
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.WithField("panic", r).Error("Panic in delayed unban goroutine")
			}
		}()

		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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
				}).Error("Failed to unban user after kick")
			}
		case <-ctx.Done():
			log.WithFields(log.Fields{
				"chatId": chat.Id,
				"userId": userId,
			}).Warn("Kick unban operation timed out")
		}
	}()

	// Continue immediately without blocking
	kickuser, err := b.GetChat(userId, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	baseStr, _ := tr.GetString(strings.ToLower(m.moduleName) + "_kick_kicked_user")
	if reason != "" {
		temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_kick_kicked_reason")
		baseStr += fmt.Sprintf(temp, reason)
	}

	_, err = msg.Reply(b,
		fmt.Sprintf(baseStr, helpers.MentionHtml(kickuser.Id, kickuser.FirstName)),
		helpers.Shtml(),
	)
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

/*
	Used to kick a user from group

The Bot should be admin with ban permissions in order to use this
*/
// kickme handles the /kickme command allowing users to remove themselves.
// Only works for non-admin users who want to leave the group.
func (m moduleStruct) kickme(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	user := ctx.EffectiveSender.User
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	// Permission checks
	if !chat_status.RequireGroup(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.CanBotRestrict(b, ctx, nil, false) {
		return ext.EndGroups
	}

	// Don't allow admins to use the command
	if chat_status.IsUserAdmin(b, chat.Id, user.Id) {
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_kickme_is_admin")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	// kick the member
	_, err := chat.UnbanMember(b, user.Id, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_kickme_ok_out")
	_, err = msg.Reply(b, text, helpers.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

/* Used to temporarily ban a user from chat

The Bot, Kick should be admin with ban permissions in order to use this */

// tBan handles the /tban command to temporarily ban a user.
// Bans a user for a specified time period with optional reason.
func (m moduleStruct) tBan(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	user := ctx.EffectiveSender.User
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	// Permission checks
	if !chat_status.RequireGroup(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireBotAdmin(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.CanUserRestrict(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}
	if !chat_status.CanBotRestrict(b, ctx, nil, false) {
		return ext.EndGroups
	}

	userId, reason := extraction.ExtractUserAndText(b, ctx)
	if userId == -1 {
		return ext.EndGroups
	} else if helpers.IsChannelID(userId) {
		text, _ := tr.GetString("bans_anonymous_ban_only_error")
		_, err := msg.Reply(b, text, nil)
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	} else if userId == 0 {
		text, _ := tr.GetString("bans_no_user_specified")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if chat_status.IsUserBanProtected(b, ctx, nil, userId) {
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_ban_is_admin")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if userId == b.Id {
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_ban_is_bot_itself")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	// Extract Time
	_time, timeVal, reason := extraction.ExtractTime(b, ctx, reason)
	if _time == -1 {
		return ext.EndGroups
	}

	_, err := chat.BanMember(b,
		userId,
		&gotgbot.BanChatMemberOpts{
			UntilDate: _time,
		},
	)
	if err != nil {
		log.Error(err)
		return err
	}

	banUser, err := b.GetChat(userId, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_ban_tban")
	baseStr := fmt.Sprintf(temp, helpers.MentionHtml(banUser.Id, banUser.FirstName), timeVal)
	if reason != "" {
		temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_ban_ban_reason")
		baseStr += fmt.Sprintf(temp, reason)
	}

	_, err = msg.Reply(
		b,
		baseStr,
		helpers.Shtml(),
	)
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

/* Used to indefinitely ban a user from group

The Bot, Banner should be admin with ban permissions in order to use this */

// ban handles the /ban command to permanently ban a user from the group.
// Supports both regular users and anonymous channels with inline unban button.
func (m moduleStruct) ban(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	user := ctx.EffectiveSender.User
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	var text string
	var sendMsgOptns *gotgbot.SendMessageOpts

	// Permission checks
	if !chat_status.RequireGroup(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireBotAdmin(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.CanUserRestrict(b, ctx, nil, user.Id, true) {
		return ext.EndGroups
	}
	if !chat_status.CanBotRestrict(b, ctx, nil, false) {
		return ext.EndGroups
	}

	userId, reason := extraction.ExtractUserAndText(b, ctx)
	switch userId {
	case -1:
		return ext.EndGroups
	case 0:
		text, _ := tr.GetString("bans_no_user_specified")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if chat_status.IsUserBanProtected(b, ctx, nil, userId) {
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_ban_is_admin")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if userId == b.Id {
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_ban_is_bot_itself")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if helpers.IsChannelID(userId) {
		if msg.ReplyToMessage != nil {
			userId := msg.ReplyToMessage.GetSender().Id()
			_, err := b.BanChatSenderChat(chat.Id, userId, nil)
			if err != nil {
				log.Error(err)
				return err
			}
			temp, _ := tr.GetString("bans_anonymous_ban_message")
			text = fmt.Sprintf(temp, helpers.MentionHtml(userId, msg.ReplyToMessage.GetSender().Name()))
		} else {
			text, _ = tr.GetString("bans_anonymous_ban_reply_required")
		}
		sendMsgOptns = helpers.Shtml()
	} else {
		_, err := chat.BanMember(b, userId, nil)
		if err != nil {
			log.Error(err)
			return err
		}

		_, name, _ := extraction.GetUserInfo(userId)

		baseStr, _ := tr.GetString(strings.ToLower(m.moduleName) + "_ban_normal_ban")
		if reason != "" {
			temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_ban_ban_reason")
			baseStr += fmt.Sprintf(temp, reason)
		}

		text = fmt.Sprintf(baseStr, helpers.MentionHtml(userId, name))

		sendMsgOptns = &gotgbot.SendMessageOpts{
			ParseMode: helpers.HTML,
			ReplyMarkup: gotgbot.InlineKeyboardMarkup{
				InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
					{
						{
							Text:         func() string { t, _ := tr.GetString("bans_unban_button"); return t }(),
							CallbackData: fmt.Sprintf("unrestrict.unban.%d", userId),
						},
					},
				},
			},
		}
	}

	_, err := msg.Reply(b, text, sendMsgOptns)
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

/* Used to Silently Ban a user from group

This deletes the command of Banner and also does not reply.

The Bot, Banner should be admin with ban permissions in order to use this */

// sBan handles the /sban command to silently ban a user.
// Bans the user and deletes the command message without notification.
func (m moduleStruct) sBan(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	user := ctx.EffectiveSender.User
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	// Permission checks
	if !chat_status.RequireGroup(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireBotAdmin(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.CanUserRestrict(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}
	if !chat_status.CanBotRestrict(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.CanBotDelete(b, ctx, nil, false) {
		return ext.EndGroups
	}

	userId := extraction.ExtractUser(b, ctx)
	if userId == -1 {
		return ext.EndGroups
	} else if helpers.IsChannelID(userId) {
		text, _ := tr.GetString("bans_anonymous_ban_only_error")
		_, err := msg.Reply(b, text, nil)
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	} else if userId == 0 {
		text, _ := tr.GetString("bans_no_user_specified")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if chat_status.IsUserBanProtected(b, ctx, nil, userId) {
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_ban_is_admin")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	_, err := chat.BanMember(b, userId, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	_, err = msg.Delete(b, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

/* Used to Ban a user from group and delete their message

This deletes the message of replied user

The Bot, Banner should be admin with ban permissions in order to use this */

// dBan handles the /dban command to delete a message and ban the sender.
// Removes the replied-to message and permanently bans the user.
func (m moduleStruct) dBan(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	user := ctx.EffectiveSender.User
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	// Permission checks
	if !chat_status.RequireGroup(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireBotAdmin(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.CanUserRestrict(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}
	if !chat_status.CanBotRestrict(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.CanUserDelete(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}
	if !chat_status.CanBotDelete(b, ctx, nil, false) {
		return ext.EndGroups
	}

	userId, reason := extraction.ExtractUserAndText(b, ctx)
	if userId == -1 {
		return ext.EndGroups
	} else if helpers.IsChannelID(userId) {
		text, _ := tr.GetString("bans_anonymous_ban_only_error")
		_, err := msg.Reply(b, text, nil)
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	} else if userId == 0 {
		text, _ := tr.GetString("bans_no_user_specified")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if chat_status.IsUserBanProtected(b, ctx, nil, userId) {
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_ban_is_admin")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if msg.ReplyToMessage == nil {
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_ban_dban_no_reply")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	_, err := msg.ReplyToMessage.Delete(b, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	_, err = chat.BanMember(b, userId, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	banUser, err := b.GetChat(userId, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	baseStr, _ := tr.GetString(strings.ToLower(m.moduleName) + "_ban_normal_ban")
	if reason != "" {
		temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_ban_ban_reason")
		baseStr += fmt.Sprintf(temp, reason)
	}

	_, err = msg.Reply(b,
		fmt.Sprintf(baseStr, helpers.MentionHtml(banUser.Id, banUser.FirstName)),
		&gotgbot.SendMessageOpts{
			ParseMode: helpers.HTML,
			ReplyMarkup: gotgbot.InlineKeyboardMarkup{
				InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
					{
						{
							Text:         func() string { t, _ := tr.GetString("bans_unban_button"); return t }(),
							CallbackData: fmt.Sprintf("unrestrict.unban.%d", userId),
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

/* Used to Unban a user from group

The Bot, Unbanner should be admin with ban permissions in order to use this */

// unban handles the /unban command to remove a ban from a user.
// Supports both regular users and anonymous channels.
func (m moduleStruct) unban(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	user := ctx.EffectiveSender.User
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	var text string

	// Permission checks
	if !chat_status.RequireGroup(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireBotAdmin(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.CanUserRestrict(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}
	if !chat_status.CanBotRestrict(b, ctx, nil, false) {
		return ext.EndGroups
	}

	userId := extraction.ExtractUser(b, ctx)
	switch userId {
	case -1:
		return ext.EndGroups
	case 0:
		text, _ := tr.GetString("bans_no_user_specified")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if userId == b.Id {
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_unban_is_bot_itself")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if helpers.IsChannelID(userId) {
		if msg.ReplyToMessage != nil {
			userId := msg.ReplyToMessage.GetSender().Id()
			_, err := b.UnbanChatSenderChat(chat.Id, userId, nil)
			if err != nil {
				log.Error(err)
				return err
			}
			temp, _ := tr.GetString("bans_anonymous_unban_message")
			text = fmt.Sprintf(temp, helpers.MentionHtml(userId, msg.ReplyToMessage.GetSender().Name()))
		} else {
			text, _ = tr.GetString("bans_anonymous_unban_reply_required")
		}
	} else {
		_, err := chat.UnbanMember(b, userId, nil)
		if err != nil {
			log.Error(err)
			return err
		}

		banUser, err := b.GetChat(userId, nil)
		if err != nil {
			log.Error(err)
			return err
		}

		temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_unban_unbanned_user")
		text = fmt.Sprintf(temp, helpers.MentionHtml(banUser.Id, banUser.FirstName))
	}

	_, err := msg.Reply(b, text, helpers.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

/* Used to Restrict members from a chat
Shows an inline keyboard menu which shows options to kick, ban and mute */

// restrict handles the /restrict command to show restriction options.
// Displays an inline keyboard with ban, kick, and mute options for a user.
func (moduleStruct) restrict(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	user := ctx.EffectiveSender.User
	msg := ctx.EffectiveMessage

	// Permission checks
	if !chat_status.RequireGroup(b, ctx, chat, false) {
		return ext.EndGroups
	}
	if !chat_status.CanUserRestrict(b, ctx, chat, user.Id, false) {
		return ext.EndGroups
	}
	if !chat_status.CanBotRestrict(b, ctx, chat, false) {
		return ext.EndGroups
	}

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	userId := extraction.ExtractUser(b, ctx)
	switch userId {
	case -1:
		return ext.EndGroups
	case 0:
		text, _ := tr.GetString("bans_no_user_specified")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	// User should be in chat for getting restricted
	if !chat_status.IsUserInChat(b, chat, userId) {
		text, _ := tr.GetString("bans_user_not_in_chat")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if chat_status.IsUserBanProtected(b, ctx, nil, userId) {
		text, _ := tr.GetString("bans_restrict_admin_error")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if userId == b.Id {
		text, _ := tr.GetString("bans_restrict_self_error")
		_, err := msg.Reply(b, text, helpers.Shtml())
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
						{Text: func() string { t, _ := tr.GetString("button_ban"); return t }(), CallbackData: fmt.Sprintf("restrict.ban.%d", userId)},
						{Text: func() string { t, _ := tr.GetString("button_kick"); return t }(), CallbackData: fmt.Sprintf("restrict.kick.%d", userId)},
					},
					{{Text: func() string { t, _ := tr.GetString("button_mute"); return t }(), CallbackData: fmt.Sprintf("restrict.mute.%d", userId)}},
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
	query := ctx.CallbackQuery
	chat := ctx.EffectiveChat
	user := ctx.EffectiveSender.User
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	// permissions check
	if !chat_status.CanUserRestrict(b, ctx, chat, user.Id, false) {
		return ext.EndGroups
	}

	args := strings.Split(query.Data, ".")
	var helpText string

	action := args[0]
	userId, _ := strconv.Atoi(args[1])

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
		temp, _ := tr.GetString("bans_restrict_action_kick")
		helpText = fmt.Sprintf(temp,
			helpers.MentionHtml(user.Id, user.FirstName),
			helpers.MentionHtml(int64(userId), actionUser.FirstName),
		)
		// Use non-blocking delayed unban for restrict kick action with timeout
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.WithField("panic", r).Error("Panic in restrict delayed unban goroutine")
				}
			}()

			// Create context with timeout
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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
			case <-ctx.Done():
				log.WithFields(log.Fields{
					"chatId": chat.Id,
					"userId": userId,
				}).Warn("Restrict kick unban operation timed out")
			}
		}()
	case "mute":
		_, err := chat.RestrictMember(b, int64(userId),
			gotgbot.ChatPermissions{
				CanSendMessages:       false,
				CanSendPhotos:         false,
				CanSendVideos:         false,
				CanSendAudios:         false,
				CanSendDocuments:      false,
				CanSendVideoNotes:     false,
				CanSendVoiceNotes:     false,
				CanAddWebPagePreviews: false,
				CanChangeInfo:         false,
				CanInviteUsers:        false,
				CanPinMessages:        false,
				CanManageTopics:       false,
				CanSendPolls:          false,
				CanSendOtherMessages:  false,
			},
			nil,
		)
		if err != nil {
			log.Error(err)
			return err
		}
		temp, _ := tr.GetString("bans_restrict_action_mute")
		helpText = fmt.Sprintf(temp,
			helpers.MentionHtml(user.Id, user.FirstName),
			helpers.MentionHtml(int64(userId), actionUser.FirstName),
		)
	case "ban":
		_, err := chat.BanMember(b, int64(userId), &gotgbot.BanChatMemberOpts{})
		if err != nil {
			log.Error(err)
			return err
		}
		temp, _ := tr.GetString("bans_restrict_action_ban")
		helpText = fmt.Sprintf(temp,
			helpers.MentionHtml(user.Id, user.FirstName),
			helpers.MentionHtml(int64(userId), actionUser.FirstName),
		)
	}

	_, _, err = query.Message.EditText(b,
		helpText,
		&gotgbot.EditMessageTextOpts{
			ParseMode: helpers.HTML,
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
	user := ctx.EffectiveSender.User
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	// Permission checks
	if !chat_status.RequireGroup(b, ctx, chat, false) {
		return ext.EndGroups
	}
	if !chat_status.CanUserRestrict(b, ctx, chat, user.Id, false) {
		return ext.EndGroups
	}
	if !chat_status.CanBotRestrict(b, ctx, chat, false) {
		return ext.EndGroups
	}

	userId := extraction.ExtractUser(b, ctx)
	switch userId {
	case -1:
		return ext.EndGroups
	case 0:
		text, _ := tr.GetString("bans_no_user_specified")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	// User should be in chat for getting restricted
	if !chat_status.IsUserInChat(b, chat, userId) {
		text, _ := tr.GetString("bans_user_not_in_chat")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if chat_status.IsUserBanProtected(b, ctx, nil, userId) {
		text, _ := tr.GetString("bans_unrestrict_admin_error")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if userId == b.Id {
		text, _ := tr.GetString("bans_unrestrict_self_error")
		_, err := msg.Reply(b, text, helpers.Shtml())
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
						{Text: unbanText, CallbackData: fmt.Sprintf("unrestrict.unban.%d", userId)},
						{Text: unmuteText, CallbackData: fmt.Sprintf("unrestrict.unmute.%d", userId)},
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
	query := ctx.CallbackQuery
	chat := ctx.EffectiveChat
	user := ctx.EffectiveSender.User
	msg := query.Message
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	// permissions check
	if !chat_status.CanUserRestrict(b, ctx, chat, user.Id, false) {
		return ext.EndGroups
	}

	args := strings.Split(query.Data, ".")
	var helpText string

	action := args[0]
	userId, _ := strconv.Atoi(args[1])

	switch action {
	case "unmute":

		c, err := b.GetChat(chat.Id, nil)
		if err != nil {
			log.Error(err)
			return err
		}

		_, err = chat.RestrictMember(b, int64(userId),
			*c.Permissions,
			nil,
		)
		if err != nil {
			log.Error(err)
			return err
		}

		temp, _ := tr.GetString("bans_unrestrict_action_unmute")
		helpText = fmt.Sprintf(temp, helpers.MentionHtml(user.Id, user.FirstName))
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

		temp, _ := tr.GetString("bans_unrestrict_action_unban")
		helpText = fmt.Sprintf(temp, helpers.MentionHtml(user.Id, user.FirstName))
	}

	// type assertion to get the message
	_updatedMsg := msg.(*gotgbot.Message)

	// only strikethrough if msg.Text is non-empty
	if _updatedMsg.Text != "" {
		_updatedMsg.Text = fmt.Sprint("<s>", _updatedMsg.Text, "</s>", "\n\n")
	}

	_, _, err := msg.EditText(
		b,
		fmt.Sprint(_updatedMsg.Text, helpText),
		&gotgbot.EditMessageTextOpts{
			ParseMode: helpers.HTML,
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
	HelpModule.AbleMap.Store(bansModule.moduleName, true)

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
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("restrict."), bansModule.restrictButtonHandler))
	dispatcher.AddHandler(handlers.NewCommand("unrestrict", bansModule.unrestrict))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("unrestrict."), bansModule.unrestrictButtonHandler))
}
