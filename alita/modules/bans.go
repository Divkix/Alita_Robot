package modules

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/extraction"
)

var bansModule = moduleStruct{moduleName: "Bans"}

func kickMember(b *gotgbot.Bot, chatID, userID int64) error {
	_, err := b.UnbanChatMember(chatID, userID, &gotgbot.UnbanChatMemberOpts{OnlyIfBanned: false})
	return err
}

func (m moduleStruct) dkick(b *gotgbot.Bot, ctx *ext.Context) error {
	return moderationDkick(&m).run(b, ctx)
}

func kickTargetValidation(c *moderationCtx, t *target) error {
	prefix := strings.ToLower(c.Module.moduleName)
	return validateTarget(c, t.userID, true, prefix+"_kick_user_not_in_chat", prefix+"_kick_cannot_kick_admin", prefix+"_kick_is_bot_itself")
}

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

func banTargetValidation(c *moderationCtx, t *target) error {
	prefix := strings.ToLower(c.Module.moduleName)
	return validateTarget(c, t.userID, false, "", prefix+"_ban_is_admin", prefix+"_ban_is_bot_itself")
}

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
			return kickMember(c.Bot, c.Chat.Id, t.userID)
		},
		reply:     kickReply,
		logAction: "dkick",
	}
}

func moderationTban(m *moduleStruct) *moderationCommand {
	return &moderationCommand{
		module:   m,
		gates:    []gateFn{standardModGates},
		extract:  extractFromArgs,
		validate: banTargetValidation,
		execute: func(c *moderationCtx, t *target) error {
			_time := extractTemporalTarget(c, t)
			if _time == -1 {
				return ext.EndGroups
			}
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
		logAction: "tban",
	}
}

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
							Text:         trS(c.Tr, "bans_unban_button"),
							CallbackData: encodeCallbackData("unrestrict", map[string]string{"a": "unban", "u": fmt.Sprint(t.userID)}),
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
		logAction: "ban",
	}
}

func moderationKick(m *moduleStruct) *moderationCommand {
	return &moderationCommand{
		module:   m,
		gates:    []gateFn{standardModGates},
		extract:  extractFromArgs,
		validate: kickTargetValidation,
		execute: func(c *moderationCtx, t *target) error {
			return kickMember(c.Bot, c.Chat.Id, t.userID)
		},
		reply:     kickReply,
		logAction: "kick",
	}
}

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
			return kickMember(c.Bot, c.Chat.Id, t.userID)
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
		logAction: "kickme",
		logUser:   true,
	}
}

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
		reply:     nil,
		logAction: "sban",
	}
}

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
		reply:     banReplyWithButton,
		logAction: "dban",
	}
}

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
		logAction: "unban",
	}
}

func (m moduleStruct) kick(b *gotgbot.Bot, ctx *ext.Context) error {
	return moderationKick(&m).run(b, ctx)
}

func (m moduleStruct) kickme(b *gotgbot.Bot, ctx *ext.Context) error {
	return moderationKickme(&m).run(b, ctx)
}

func (m moduleStruct) tBan(b *gotgbot.Bot, ctx *ext.Context) error {
	return moderationTban(&m).run(b, ctx)
}

func (m moduleStruct) ban(b *gotgbot.Bot, ctx *ext.Context) error {
	return moderationBan(&m).run(b, ctx)
}

func (m moduleStruct) sBan(b *gotgbot.Bot, ctx *ext.Context) error {
	return moderationSban(&m).run(b, ctx)
}

func (m moduleStruct) dBan(b *gotgbot.Bot, ctx *ext.Context) error {
	return moderationDban(&m).run(b, ctx)
}

func (m moduleStruct) unban(b *gotgbot.Bot, ctx *ext.Context) error {
	return moderationUnban(&m).run(b, ctx)
}

func (moduleStruct) restrict(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	msg := ctx.EffectiveMessage

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

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
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
							Text:         trS(tr, "button_ban"),
							CallbackData: encodeCallbackData("restrict", map[string]string{"a": "ban", "u": fmt.Sprint(userId)}),
						},
						{
							Text:         trS(tr, "button_kick"),
							CallbackData: encodeCallbackData("restrict", map[string]string{"a": "kick", "u": fmt.Sprint(userId)}),
						},
					},
					{{
						Text:         trS(tr, "button_mute"),
						CallbackData: encodeCallbackData("restrict", map[string]string{"a": "mute", "u": fmt.Sprint(userId)}),
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

func (moduleStruct) restrictButtonHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	query, ok := callbackQueryFromContext(ctx)
	if !ok {
		return ext.EndGroups
	}
	if query.Message == nil {
		return answerInvalidCallback(b, ctx, query)
	}
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	if !chat_status.CanUserRestrict(b, ctx, chat, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_restrict_cmd_error", "chat_status_restrict_button_error")
		return ext.EndGroups
	}

	action := ""
	userIDRaw := ""
	if decoded, ok := decodeCallbackData(query.Data, "restrict"); ok {
		action, _ = decoded.Field("a")
		userIDRaw, _ = decoded.Field("u")
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
	switch action {
	case "kick", "mute", "ban":
	default:
		log.WithField("callbackData", query.Data).Error("Unknown restrict callback action")
		errText, _ := tr.GetString("bans_invalid_callback_data")
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: errText, ShowAlert: true})
		return ext.EndGroups
	}
	userId, err := strconv.ParseInt(userIDRaw, 10, 64)
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

	actionUser, err := b.GetChat(userId, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	switch action {
	case "kick":
		if err := kickMember(b, chat.Id, userId); err != nil {
			log.Error(err)
			return err
		}
		temp, _ := tr.GetString("bans_restrict_kicked")
		helpText = fmt.Sprintf(temp,
			formatting.MentionHtml(user.Id, user.FirstName),
			formatting.MentionHtml(userId, actionUser.FirstName),
		)
	case "mute":
		_, err := chat.RestrictMember(b, userId,
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

	_, _, err = query.Message.EditText(b, &gotgbot.EditMessageTextOpts{Text: helpText, ParseMode: formatting.HTML})
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

func (moduleStruct) unrestrict(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

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
							CallbackData: encodeCallbackData("unrestrict", map[string]string{"a": "unban", "u": fmt.Sprint(userId)}),
						},
						{
							Text:         unmuteText,
							CallbackData: encodeCallbackData("unrestrict", map[string]string{"a": "unmute", "u": fmt.Sprint(userId)}),
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
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	if msg == nil {
		return answerInvalidCallback(b, ctx, query)
	}

	if !chat_status.CanUserRestrict(b, ctx, chat, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_restrict_cmd_error", "chat_status_restrict_button_error")
		return ext.EndGroups
	}

	action := ""
	userIDRaw := ""
	if decoded, ok := decodeCallbackData(query.Data, "unrestrict"); ok {
		action, _ = decoded.Field("a")
		userIDRaw, _ = decoded.Field("u")
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
	switch action {
	case "unmute", "unban":
	default:
		log.WithField("callbackData", query.Data).Error("Unknown unrestrict callback action")
		errText, _ := tr.GetString("bans_invalid_callback_data")
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: errText, ShowAlert: true})
		return ext.EndGroups
	}
	userId, err := strconv.ParseInt(userIDRaw, 10, 64)
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

		_, err = chat.RestrictMember(b, userId,
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
		if chat_status.IsChannelId(userId) {
			// Anonymous channel bans use BanChatSenderChat; unban must use the
			// matching sender-chat endpoint, not UnbanChatMember (which rejects
			// channel IDs and leaves the channel permanently banned).
			_, err := chat.UnbanSenderChat(b, userId, nil)
			if err != nil {
				log.Error(err)
				return err
			}
		} else {
			_, err := chat.Unban(b,
				userId,
				&gotgbot.UnbanChatMemberOpts{
					OnlyIfBanned: true,
				},
			)
			if err != nil {
				log.Error(err)
				return err
			}
		}

		temp, _ := tr.GetString("bans_unrestrict_unbanned")
		helpText = fmt.Sprintf(temp, formatting.MentionHtml(user.Id, user.FirstName))
	}

	updatedText := ""
	if ctx.EffectiveMessage != nil {
		updatedText = ctx.EffectiveMessage.Text
	}
	if updatedText != "" {
		updatedText = "<s>" + updatedText + "</s>\n\n"
	}

	_, _, err = msg.EditText(b, &gotgbot.EditMessageTextOpts{Text: updatedText + helpText, ParseMode: formatting.HTML})
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

func moderationSkick(m *moduleStruct) *moderationCommand {
	return &moderationCommand{
		module:   m,
		gates:    []gateFn{deleteModGates},
		extract:  extractUserOnly,
		validate: kickTargetValidation,
		execute: func(c *moderationCtx, t *target) error {
			if err := kickMember(c.Bot, c.Chat.Id, t.userID); err != nil {
				return err
			}
			_, err := c.Msg.Delete(c.Bot, nil)
			return err
		},
		reply:     nil,
		logAction: "skick",
	}
}

func (m moduleStruct) sKick(b *gotgbot.Bot, ctx *ext.Context) error {
	return moderationSkick(&m).run(b, ctx)
}

func moderationBanme(m *moduleStruct) *moderationCommand {
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
			if chat_status.IsUserAdmin(c.Bot, c.Chat.Id, c.User.Id) {
				text, _ := c.Tr.GetString("bans_banme_is_admin")
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
			return err
		},
		reply: func(c *moderationCtx, t *target) error {
			text, _ := c.Tr.GetString("bans_banme_ok")
			_, err := c.Msg.Reply(c.Bot, text, formatting.Shtml())
			if err != nil {
				log.Error(err)
				return err
			}
			return nil
		},
		logAction: "banme",
		logUser:   true,
	}
}

func (m moduleStruct) banme(b *gotgbot.Bot, ctx *ext.Context) error {
	return moderationBanme(&m).run(b, ctx)
}

func (m moduleStruct) unbanAllHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	if !chat_status.RequireGroup(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_group_only_error", "", chat_status.WithReply())
		return ext.EndGroups
	}
	if !chat_status.RequireUserOwner(b, ctx, chat, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_owner_cmd_error", "chat_status_owner_button_error", chat_status.WithReply())
		return ext.EndGroups
	}
	if !chat_status.CanBotRestrict(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_bot_restrict_group_error", "chat_status_bot_restrict_error")
		return ext.EndGroups
	}
	text, _ := tr.GetString("bans_unbanall_ask")
	yesText, _ := tr.GetString("button_yes")
	noText, _ := tr.GetString("button_no")
	_, err := msg.Reply(b, text, &gotgbot.SendMessageOpts{
		ReplyMarkup: gotgbot.InlineKeyboardMarkup{
			InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
				{
					{Text: yesText, CallbackData: encodeCallbackData("unbanall", map[string]string{"a": "yes"})},
					{Text: noText, CallbackData: encodeCallbackData("unbanall", map[string]string{"a": "no"})},
				},
			},
		},
	})
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}

func (m moduleStruct) unbanAllCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	query, ok := callbackQueryFromContext(ctx)
	if !ok {
		return ext.EndGroups
	}
	user := query.From
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	if !chat_status.RequireUserOwner(b, ctx, nil, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_owner_cmd_error", "chat_status_owner_button_error", chat_status.WithReply())
		return ext.EndGroups
	}
	action := ""
	if decoded, ok := decodeCallbackData(query.Data, "unbanall"); ok {
		action, _ = decoded.Field("a")
	}
	if action == "" {
		return answerInvalidCallback(b, ctx, query)
	}
	if (action == "yes" || action == "no") && query.Message == nil {
		text, _ := tr.GetString("common_callback_message_unavailable")
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return ext.EndGroups
	}
	defer error_handling.RecoverFromPanic("unbanall", "bans")
	var helpText string
	switch action {
	case "yes":
		chatID := query.Message.GetChat().Id
		users, err := chats.GetChatUsersCached(chatID)
		if err != nil {
			log.Debugf("[Bans] unbanall GetChatUsersCached %d: %v", chatID, err)
			users = nil
		}
		unbanned := 0
		for _, userID := range users {
			if !chat_status.IsValidUserId(userID) || userID == b.Id {
				continue
			}
			if _, err := b.UnbanChatMember(chatID, userID, &gotgbot.UnbanChatMemberOpts{OnlyIfBanned: true}); err != nil {
				log.Debugf("[Bans] unbanall user %d: %v", userID, err)
				continue
			}
			unbanned++
		}
		helpText, _ = tr.GetString("bans_unbanall_done", i18n.TranslationParams{"count": unbanned})
	case "no":
		helpText, _ = tr.GetString("bans_unbanall_cancel")
	default:
		helpText, _ = tr.GetString("bans_unbanall_cancel")
	}
	if query.Message != nil {
		_, _, err := query.Message.EditText(b, &gotgbot.EditMessageTextOpts{Text: helpText, ParseMode: formatting.HTML})
		if err != nil {
			log.Error(err)
			return err
		}
	}
	_, err := query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: helpText})
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}

func restrictChecks(cmd string) []helpers.CheckFunc {
	return []helpers.CheckFunc{
		helpers.CheckDisabled(cmd),
		helpers.RequireGroup(),
		helpers.RequireUserAdmin(),
		helpers.RequireBotAdmin(),
		helpers.CanUserRestrict(),
		helpers.CanBotRestrict(),
	}
}

func deleteRestrictChecks(cmd string) []helpers.CheckFunc {
	return append(restrictChecks(cmd), helpers.CanBotDelete(), helpers.CanUserDelete())
}

var (
	banDesc        = helpers.CommandDescriptor{Name: "ban", RequiredChecks: []helpers.CheckFunc(nil)}
	sbanDesc       = helpers.CommandDescriptor{Name: "sban"}
	tbanDesc       = helpers.CommandDescriptor{Name: "tban"}
	dbanDesc       = helpers.CommandDescriptor{Name: "dban"}
	unbanDesc      = helpers.CommandDescriptor{Name: "unban"}
	banmeDesc      = helpers.CommandDescriptor{Name: "banme", Disableable: true}
	unbanAllDesc   = helpers.CommandDescriptor{Name: "unbanall"}
	kickDesc       = helpers.CommandDescriptor{Name: "kick"}
	dkickDesc      = helpers.CommandDescriptor{Name: "dkick"}
	skickDesc      = helpers.CommandDescriptor{Name: "skick"}
	kickmeDesc     = helpers.CommandDescriptor{Name: "kickme", Disableable: true}
	restrictDesc   = helpers.CommandDescriptor{Name: "restrict"}
	unrestrictDesc = helpers.CommandDescriptor{Name: "unrestrict"}
)

func initBanDescs() {
	banDesc.RequiredChecks = restrictChecks("ban")
	sbanDesc.RequiredChecks = deleteRestrictChecks("sban")
	tbanDesc.RequiredChecks = restrictChecks("tban")
	dbanDesc.RequiredChecks = deleteRestrictChecks("dban")
	unbanDesc.RequiredChecks = restrictChecks("unban")
	banmeDesc.RequiredChecks = []helpers.CheckFunc{helpers.CheckDisabled("banme"), helpers.RequireGroup(), helpers.CanBotRestrict()}
	unbanAllDesc.RequiredChecks = []helpers.CheckFunc{helpers.CheckDisabled("unbanall"), helpers.RequireGroup(), helpers.RequireUserOwner(), helpers.CanBotRestrict()}
	kickDesc.RequiredChecks = restrictChecks("kick")
	dkickDesc.RequiredChecks = deleteRestrictChecks("dkick")
	skickDesc.RequiredChecks = deleteRestrictChecks("skick")
	kickmeDesc.RequiredChecks = []helpers.CheckFunc{helpers.CheckDisabled("kickme"), helpers.RequireGroup(), helpers.CanBotRestrict()}
	restrictDesc.RequiredChecks = []helpers.CheckFunc{helpers.CheckDisabled("restrict"), helpers.RequireGroup(), helpers.CanUserRestrict(), helpers.CanBotRestrict()}
	unrestrictDesc.RequiredChecks = []helpers.CheckFunc{helpers.CheckDisabled("unrestrict"), helpers.RequireGroup(), helpers.CanUserRestrict(), helpers.CanBotRestrict()}
}

func LoadBans(dispatcher *ext.Dispatcher) {
	DefaultHelpRegistry().AbleMap[bansModule.moduleName] = true
	initBanDescs()

	helpers.WrapCommand(dispatcher, banDesc, pipelineHandler(bansModule.ban))
	helpers.WrapCommand(dispatcher, sbanDesc, pipelineHandler(bansModule.sBan))
	helpers.WrapCommand(dispatcher, tbanDesc, pipelineHandler(bansModule.tBan))
	helpers.WrapCommand(dispatcher, dbanDesc, pipelineHandler(bansModule.dBan))
	helpers.WrapCommand(dispatcher, unbanDesc, pipelineHandler(bansModule.unban))
	helpers.WrapCommand(dispatcher, banmeDesc, pipelineHandler(bansModule.banme))
	helpers.WrapCommand(dispatcher, unbanAllDesc, pipelineHandler(bansModule.unbanAllHandler))

	helpers.WrapCommand(dispatcher, kickDesc, pipelineHandler(bansModule.kick))
	helpers.WrapCommand(dispatcher, dkickDesc, pipelineHandler(bansModule.dkick))
	helpers.WrapCommand(dispatcher, skickDesc, pipelineHandler(bansModule.sKick))
	helpers.WrapCommand(dispatcher, kickmeDesc, pipelineHandler(bansModule.kickme))

	helpers.WrapCommand(dispatcher, restrictDesc, pipelineHandler(bansModule.restrict))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("restrict"), bansModule.restrictButtonHandler))
	helpers.WrapCommand(dispatcher, unrestrictDesc, pipelineHandler(bansModule.unrestrict))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("unrestrict"), bansModule.unrestrictButtonHandler))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("unbanall|"), bansModule.unbanAllCallback))
}

func init() {
	RegisterLegacyModule("Bans", 70, LoadBans)
	initBanDescs()
	RegisterAnonymousAdminHandler("ban", anonPipelineHandler(banDesc, bansModule.ban))
	RegisterAnonymousAdminHandler("dban", anonPipelineHandler(dbanDesc, bansModule.dBan))
	RegisterAnonymousAdminHandler("sban", anonPipelineHandler(sbanDesc, bansModule.sBan))
	RegisterAnonymousAdminHandler("tban", anonPipelineHandler(tbanDesc, bansModule.tBan))
	RegisterAnonymousAdminHandler("unban", anonPipelineHandler(unbanDesc, bansModule.unban))
	RegisterAnonymousAdminHandler("skick", anonPipelineHandler(skickDesc, bansModule.sKick))
	RegisterAnonymousAdminHandler("restrict", anonPipelineHandler(restrictDesc, bansModule.restrict))
	RegisterAnonymousAdminHandler("unrestrict", anonPipelineHandler(unrestrictDesc, bansModule.unrestrict))
}
