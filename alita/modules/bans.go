package modules

import (
	"fmt"
	"html"
	"strconv"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/bans"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
	"github.com/divkix/Alita_Robot/alita/utils/extraction"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	"github.com/divkix/Alita_Robot/alita/utils/keyboard"
	"github.com/divkix/Alita_Robot/alita/utils/tracing"
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
	text := fmt.Sprintf(baseStr, formatting.MentionHtml(kickuser.Id, kickuser.FirstName))
	if t.reason != "" {
		temp, _ := c.Tr.GetString(strings.ToLower(c.Module.moduleName) + "_kick_kicked_reason")
		if temp != "" {
			text += fmt.Sprintf(temp, html.EscapeString(t.reason))
		}
	}

	_, err = c.Msg.Reply(c.Bot, withGroupLine(c.Tr, c.Msg, c.Chat, text), formatting.Shtml())
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
		command:  "dkick",
		module:   m,
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
		reply:       kickReply,
		logAction:   "dkick",
		eventAction: models.ModerationActionKick,
	}
}

func moderationTban(m *moduleStruct) *moderationCommand {
	return &moderationCommand{
		command:  "tban",
		module:   m,
		extract:  extractFromArgs,
		validate: banTargetValidation,
		execute: func(c *moderationCtx, t *target) error {
			_time := extractTemporalTarget(c, t)
			if _time == -1 {
				return ext.EndGroups
			}
			_, err := c.Chat.BanMember(c.Bot, t.userID, &gotgbot.BanChatMemberOpts{UntilDate: _time})
			t.untilUnix = _time
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
					baseStr += fmt.Sprintf(temp, html.EscapeString(t.reason))
				}
			}

			_, err = c.Msg.Reply(c.Bot, withGroupLine(c.Tr, c.Msg, c.Chat, baseStr), formatting.Shtml())
			if err != nil {
				log.Error(err)
				return err
			}
			return nil
		},
		logAction:   "tban",
		eventAction: models.ModerationActionBan,
	}
}

func banReplyWithButton(c *moderationCtx, t *target) error {
	banUser, err := c.Bot.GetChat(t.userID, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	baseStr, _ := c.Tr.GetString(strings.ToLower(c.Module.moduleName) + "_ban_normal_ban")
	text := fmt.Sprintf(baseStr, formatting.MentionHtml(banUser.Id, banUser.FirstName))
	if t.reason != "" {
		temp, _ := c.Tr.GetString(strings.ToLower(c.Module.moduleName) + "_ban_ban_reason")
		text += fmt.Sprintf(temp, html.EscapeString(t.reason))
	}

	_, err = c.Msg.Reply(c.Bot, withGroupLine(c.Tr, c.Msg, c.Chat, text),
		&gotgbot.SendMessageOpts{
			ParseMode: formatting.HTML,
			ReplyMarkup: gotgbot.InlineKeyboardMarkup{
				InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
					{keyboard.UnrestrictButton(c.Tr, "bans_unban_button", "unban", t.userID, pmButtonChatID(c.Msg, c.Chat))},
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
		command: "ban",
		module:  m,
		extract: func(c *moderationCtx) (target, error) {
			uid, reason := extraction.ExtractUserAndText(c.Bot, c.Ctx)
			if uid == -1 {
				return target{}, fmt.Errorf("extraction failed")
			}
			if uid == 0 {
				text, _ := c.Tr.GetString("common_no_user_specified")
				if err := c.replyCommandHelp(text); err != nil {
					log.Error(err)
					return target{}, err
				}
				return target{}, fmt.Errorf("no user")
			}
			return target{userID: uid, reason: reason, isChannel: chat_status.IsChannelId(uid)}, nil
		},
		validate: func(c *moderationCtx, t *target) error {
			if t.isChannel {
				if c.Msg.ReplyToMessage == nil {
					text, _ := c.Tr.GetString("bans_anonymous_ban_reply_only")
					if err := replyValidationError(c, text); err != nil {
						return err
					}
					return fmt.Errorf("channel ban needs reply")
				}
				return nil
			}
			return banTargetValidation(c, t)
		},
		execute: func(c *moderationCtx, t *target) error {
			if t.isChannel {
				t.userID = c.Msg.ReplyToMessage.GetSender().Id()
				_, err := c.Bot.BanChatSenderChat(c.Chat.Id, t.userID, nil)
				return err
			}
			_, err := c.Chat.BanMember(c.Bot, t.userID, nil)
			return err
		},
		reply: func(c *moderationCtx, t *target) error {
			var text string
			var sendMsgOptns *gotgbot.SendMessageOpts

			if t.isChannel {
				temp, _ := c.Tr.GetString("bans_anonymous_ban_user")
				text = fmt.Sprintf(temp, formatting.MentionHtml(t.userID, c.Msg.ReplyToMessage.GetSender().Name()))
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
		logAction:   "ban",
		eventAction: models.ModerationActionBan,
	}
}

func moderationKick(m *moduleStruct) *moderationCommand {
	return &moderationCommand{
		command:  "kick",
		module:   m,
		extract:  extractFromArgs,
		validate: kickTargetValidation,
		execute: func(c *moderationCtx, t *target) error {
			return kickMember(c.Bot, c.Chat.Id, t.userID)
		},
		reply:       kickReply,
		logAction:   "kick",
		eventAction: models.ModerationActionKick,
	}
}

func moderationKickme(m *moduleStruct) *moderationCommand {
	return &moderationCommand{
		module: m,
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
		command:  "sban",
		module:   m,
		extract:  extractUserOnly,
		validate: banTargetValidation,
		execute: func(c *moderationCtx, t *target) error {
			_, err := c.Chat.BanMember(c.Bot, t.userID, nil)
			return err
		},
		cleanup: func(c *moderationCtx, _ *target) error {
			if c.sentInPM() {
				return nil
			}
			_, err := c.Msg.Delete(c.Bot, nil)
			return err
		},
		reply: func(c *moderationCtx, t *target) error {
			if c.sentInPM() {
				return banReplyWithButton(c, t)
			}
			return nil
		},
		logAction:   "sban",
		eventAction: models.ModerationActionBan,
	}
}

func moderationDban(m *moduleStruct) *moderationCommand {
	return &moderationCommand{
		command: "dban",
		module:  m,
		extract: extractFromArgs,
		validate: func(c *moderationCtx, t *target) error {
			if c.Msg.ReplyToMessage == nil {
				text, _ := c.Tr.GetString(strings.ToLower(c.Module.moduleName) + "_ban_dban_no_reply")
				if text == "" {
					text, _ = c.Tr.GetString("common_no_reply_to_message")
				}
				if err := c.replyCommandHelp(text); err != nil {
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
		reply:       banReplyWithButton,
		logAction:   "dban",
		eventAction: models.ModerationActionBan,
	}
}

func moderationUnban(m *moduleStruct) *moderationCommand {
	return &moderationCommand{
		command: "unban",
		module:  m,
		extract: func(c *moderationCtx) (target, error) {
			uid := extraction.ExtractUser(c.Bot, c.Ctx)
			if uid == -1 {
				return target{}, fmt.Errorf("extraction failed")
			}
			if uid == 0 {
				text, _ := c.Tr.GetString("common_no_user_specified")
				if err := c.replyCommandHelp(text); err != nil {
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
			if t.isChannel && c.Msg.ReplyToMessage == nil {
				text, _ := c.Tr.GetString("bans_anonymous_unban_reply_only")
				if err := replyValidationError(c, text); err != nil {
					return err
				}
				return fmt.Errorf("channel unban needs reply")
			}
			return nil
		},
		execute: func(c *moderationCtx, t *target) error {
			if t.isChannel {
				t.userID = c.Msg.ReplyToMessage.GetSender().Id()
				_, err := c.Bot.UnbanChatSenderChat(c.Chat.Id, t.userID, nil)
				return err
			}
			_, err := c.Chat.UnbanMember(c.Bot, t.userID, nil)
			return err
		},
		cleanup: func(c *moderationCtx, t *target) error {
			spamFilterUnbanHam(c.Chat.Id, t.userID, c.User.Id)
			return nil
		},
		reply: func(c *moderationCtx, t *target) error {
			var text string
			if t.isChannel {
				temp, _ := c.Tr.GetString("bans_anonymous_unban_user")
				text = fmt.Sprintf(temp, formatting.MentionHtml(t.userID, c.Msg.ReplyToMessage.GetSender().Name()))
			} else {
				banUser, err := c.Bot.GetChat(t.userID, nil)
				if err != nil {
					log.Error(err)
					return err
				}
				temp, _ := c.Tr.GetString(strings.ToLower(c.Module.moduleName) + "_unban_unbanned_user")
				text = fmt.Sprintf(temp, formatting.MentionHtml(banUser.Id, banUser.FirstName))
			}
			_, err := c.Msg.Reply(c.Bot, withGroupLine(c.Tr, c.Msg, c.Chat, text), formatting.Shtml())
			if err != nil {
				log.Error(err)
				return err
			}
			return nil
		},
		logAction:   "unban",
		eventAction: models.ModerationActionUnban,
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
	if chat_status.RequireUser(b, ctx) == nil {
		return ext.EndGroups
	}
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	userId := extraction.ExtractUser(b, ctx)
	switch userId {
	case -1:
		return ext.EndGroups
	case 0:
		text, _ := tr.GetString("common_no_user_specified")
		if err := replyCommandHelp(b, msg, tr, "restrict", text); err != nil {
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
	_, err := msg.Reply(b, withGroupLine(tr, msg, chat, text),
		&gotgbot.SendMessageOpts{
			ParseMode: formatting.HTML,
			ReplyMarkup: gotgbot.InlineKeyboardMarkup{
				InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
					{
						{
							Text:         trS(tr, "button_ban"),
							CallbackData: moderationButtonData("restrict", msg, chat, map[string]string{"a": "ban", "u": fmt.Sprint(userId)}),
						},
						{
							Text:         trS(tr, "button_kick"),
							CallbackData: moderationButtonData("restrict", msg, chat, map[string]string{"a": "kick", "u": fmt.Sprint(userId)}),
						},
					},
					{{
						Text:         trS(tr, "button_mute"),
						CallbackData: moderationButtonData("restrict", msg, chat, map[string]string{"a": "mute", "u": fmt.Sprint(userId)}),
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
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	decoded, decodedOK := decodeCallbackData(query.Data, "restrict")
	chat, chatOK := buttonChat(ctx, decoded)
	if chatOK && chat != nil && !chat_status.CanUserRestrict(b, ctx, chat, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_restrict_cmd_error", "chat_status_restrict_button_error")
		return ext.EndGroups
	}

	action := ""
	userIDRaw := ""
	if decodedOK {
		action, _ = decoded.Field("a")
		userIDRaw, _ = decoded.Field("u")
	}
	if !chatOK || chat == nil || action == "" || userIDRaw == "" {
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
		recordModerationEvent(chat.Id, user.Id, userId, models.ModerationActionKick, "")
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
		recordModerationEvent(chat.Id, user.Id, userId, models.ModerationActionMute, "")
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
		recordModerationEvent(chat.Id, user.Id, userId, models.ModerationActionBan, "")
		recordBan(banRecord{chatID: chat.Id, userID: userId, bannedBy: user.Id, source: models.BanSourceManual})
		temp, _ := tr.GetString("bans_restrict_banned")
		helpText = fmt.Sprintf(temp,
			formatting.MentionHtml(user.Id, user.FirstName),
			formatting.MentionHtml(int64(userId), actionUser.FirstName),
		)
	}

	// Sent in PM, the menu named the group, and the text that replaces it
	// names it too; the button carries only the group's ID. The line is only
	// text, so a failed lookup leaves it out.
	if inPM(ctx.EffectiveMessage) {
		if full, err := b.GetChat(chat.Id, nil); err != nil {
			log.Error(err)
		} else {
			group := full.ToChat()
			helpText = withGroupLine(tr, ctx.EffectiveMessage, &group, helpText)
		}
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
	if chat_status.RequireUser(b, ctx) == nil {
		return ext.EndGroups
	}
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	userId := extraction.ExtractUser(b, ctx)
	switch userId {
	case -1:
		return ext.EndGroups
	case 0:
		text, _ := tr.GetString("common_no_user_specified")
		if err := replyCommandHelp(b, msg, tr, "unrestrict", text); err != nil {
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
	_, err := msg.Reply(b, withGroupLine(tr, msg, ctx.EffectiveChat, text),
		&gotgbot.SendMessageOpts{
			ParseMode: formatting.HTML,
			ReplyMarkup: gotgbot.InlineKeyboardMarkup{
				InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
					{
						keyboard.UnrestrictButton(tr, "button_unban", "unban", userId, pmButtonChatID(msg, ctx.EffectiveChat)),
						keyboard.UnrestrictButton(tr, "button_unmute", "unmute", userId, pmButtonChatID(msg, ctx.EffectiveChat)),
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
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	msg := query.Message
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	if msg == nil {
		return answerInvalidCallback(b, ctx, query)
	}

	decoded, decodedOK := decodeCallbackData(query.Data, "unrestrict")
	chat, chatOK := buttonChat(ctx, decoded)
	if chatOK && chat != nil && !chat_status.CanUserRestrict(b, ctx, chat, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_restrict_cmd_error", "chat_status_restrict_button_error")
		return ext.EndGroups
	}

	action := ""
	userIDRaw := ""
	if decodedOK {
		action, _ = decoded.Field("a")
		userIDRaw, _ = decoded.Field("u")
	}
	if !chatOK || chat == nil || action == "" || userIDRaw == "" {
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

	// A failed unrestrict still answers the query, or the button keeps spinning.
	failAction := func(err error) error {
		log.Error(err)
		_, _ = query.Answer(b, nil)
		return err
	}

	var helpText string

	switch action {
	case "unmute":

		c, err := b.GetChat(chat.Id, nil)
		if err != nil {
			return failAction(err)
		}
		unmutePermissions := resolveUnmutePermissions(c)

		_, err = chat.RestrictMember(b, userId,
			unmutePermissions,
			nil,
		)
		if err != nil {
			return failAction(err)
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
				return failAction(err)
			}
		} else {
			_, err := chat.Unban(b,
				userId,
				&gotgbot.UnbanChatMemberOpts{
					OnlyIfBanned: true,
				},
			)
			if err != nil {
				return failAction(err)
			}
		}
		recordModerationEvent(chat.Id, user.Id, userId, models.ModerationActionUnban, "")
		liftBanRecord(chat.Id, userId, user.Id)
		spamFilterUnbanHam(chat.Id, userId, user.Id)

		temp, _ := tr.GetString("bans_unrestrict_unbanned")
		helpText = fmt.Sprintf(temp, formatting.MentionHtml(user.Id, user.FirstName))
	}

	updatedText := ""
	if ctx.EffectiveMessage != nil {
		updatedText = ctx.EffectiveMessage.Text
	}
	if updatedText != "" {
		updatedText = "<s>" + html.EscapeString(updatedText) + "</s>\n\n"
	}

	// Answer even when the edit fails, or the button keeps spinning.
	_, _, editErr := msg.EditText(b, &gotgbot.EditMessageTextOpts{Text: updatedText + helpText, ParseMode: formatting.HTML})
	if editErr != nil {
		log.Error(editErr)
	}

	if _, err = query.Answer(b, nil); err != nil {
		log.Error(err)
		return err
	}
	if editErr != nil {
		return editErr
	}

	return ext.EndGroups
}

func moderationSkick(m *moduleStruct) *moderationCommand {
	return &moderationCommand{
		command:  "skick",
		module:   m,
		extract:  extractUserOnly,
		validate: kickTargetValidation,
		execute: func(c *moderationCtx, t *target) error {
			return kickMember(c.Bot, c.Chat.Id, t.userID)
		},
		cleanup: func(c *moderationCtx, _ *target) error {
			if c.sentInPM() {
				return nil
			}
			_, err := c.Msg.Delete(c.Bot, nil)
			return err
		},
		reply: func(c *moderationCtx, t *target) error {
			if c.sentInPM() {
				return kickReply(c, t)
			}
			return nil
		},
		logAction:   "skick",
		eventAction: models.ModerationActionKick,
	}
}

func (m moduleStruct) sKick(b *gotgbot.Bot, ctx *ext.Context) error {
	return moderationSkick(&m).run(b, ctx)
}

func moderationBanme(m *moduleStruct) *moderationCommand {
	return &moderationCommand{
		module: m,
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
		selfBan:   true,
	}
}

func (m moduleStruct) banme(b *gotgbot.Bot, ctx *ext.Context) error {
	return moderationBanme(&m).run(b, ctx)
}

func (m moduleStruct) unbanAllHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if chat_status.RequireUser(b, ctx) == nil {
		return ext.EndGroups
	}
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	text, _ := tr.GetString("bans_unbanall_ask")
	text = confirmText(ctx, tr, text, ctx.EffectiveChat)
	_, err := msg.Reply(b, text, &gotgbot.SendMessageOpts{
		ReplyMarkup: confirmKeyboard(tr, "unbanall", ctx.EffectiveChat),
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
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	decoded, _ := decodeCallbackData(query.Data, "unbanall")
	chat := ownerConfirmChat(b, ctx, query, decoded)
	if chat == nil {
		return ext.EndGroups
	}
	action, _ := decoded.Field("a")
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
		chatID := chat.Id
		users, err := chats.GetChatUsersCachedContext(tracing.UpdateContext(ctx), chatID)
		if err != nil {
			log.Debugf("[Bans] unbanall GetChatUsersCached %d: %v", chatID, err)
			users = nil
		}
		unbanned := make([]int64, 0, len(users))
		for _, userID := range users {
			if !chat_status.IsValidUserId(userID) || userID == b.Id {
				continue
			}
			if _, err := b.UnbanChatMember(chatID, userID, &gotgbot.UnbanChatMemberOpts{OnlyIfBanned: true}); err != nil {
				log.Debugf("[Bans] unbanall user %d: %v", userID, err)
				continue
			}
			unbanned = append(unbanned, userID)
		}
		// Close the records of the bans it lifted; a failed write changes
		// nothing else, like every ban record write.
		if _, err := bans.LiftUsers(chatID, unbanned, query.From.Id); err != nil {
			log.WithError(err).WithField("chat_id", chatID).Error("[Bans] Failed to close the ban records of /unbanall")
		}
		helpText, _ = tr.GetString("bans_unbanall_done", i18n.TranslationParams{"count": len(unbanned)})
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

// restrictChecks are the checks of a command that restricts a user; chat
// checks (pmTargetChatCheck) run before the group's.
func restrictChecks(cmd string, chat ...helpers.CheckFunc) []helpers.CheckFunc {
	return append(append([]helpers.CheckFunc{helpers.CheckDisabled(cmd)}, chat...),
		helpers.RequireGroup(),
		helpers.RequireUserAdmin(),
		helpers.RequireBotAdmin(),
		helpers.CanUserRestrict(),
		helpers.CanBotRestrict(),
	)
}

func deleteRestrictChecks(cmd string, chat ...helpers.CheckFunc) []helpers.CheckFunc {
	return append(restrictChecks(cmd, chat...), helpers.CanBotDelete(), helpers.CanUserDelete())
}

// pmTargetChatCheck is groupOnlyChatCheck for a command that acts on a user.
// From PM a reply is to a PM message, the bot's or the caller's own, so it
// names no target: the command reads its user from the arguments after the
// group only.
func pmTargetChatCheck(c *helpers.CommandContext) bool {
	if !groupOnlyChatCheck(c) {
		return false
	}
	dropPMReply(c)
	return true
}

// dropPMReply drops the reply from a command sent in PM, where the message
// replied to is no group member's.
func dropPMReply(c *helpers.CommandContext) {
	if msg := c.Ctx.EffectiveMessage; inPM(msg) && msg.ReplyToMessage != nil {
		noReply := *msg
		noReply.ReplyToMessage = nil
		c.Ctx.EffectiveMessage = &noReply
		c.Msg = &noReply
	}
}

// withGroupLine adds the group chat to the HTML reply text of a moderation
// command sent in reply to msg (pmGroupLine).
func withGroupLine(tr *i18n.Translator, msg *gotgbot.Message, chat *gotgbot.Chat, text string) string {
	return text + pmGroupLine(tr, msg, chat, true)
}

// pmButtonChatID is the group a button a moderation command sends in reply to
// msg names: chat, the group it acts on, when msg came from PM, whose chat
// names no group; else 0, so the button acts on the chat it is pressed in.
// Pass this value to keyboard.UnrestrictButton for unban and unmute buttons.
func pmButtonChatID(msg *gotgbot.Message, chat *gotgbot.Chat) int64 {
	if inPM(msg) {
		return chat.Id
	}
	return 0
}

// moderationButtonData is the callback data of a button a bans command sends
// in reply to msg, naming the group in its "c" field when pmButtonChatID does.
// In groups it adds no "c"; buttonChat then falls back to the button's chat.
func moderationButtonData(namespace string, msg *gotgbot.Message, chat *gotgbot.Chat, fields map[string]string) string {
	if chatID := pmButtonChatID(msg, chat); chatID != 0 {
		fields["c"] = strconv.FormatInt(chatID, 10)
	}
	return encodeCallbackData(namespace, fields)
}

// From PM, the commands that act on a user named after the group take the
// group as their first argument (or from the connection), and their checks
// see that group. /dban and /dkick need a reply to a group message, and
// /banme and /kickme act on the caller in the group, so they stay group-only.
var (
	banDesc        = helpers.CommandDescriptor{Name: "ban", PMGroupArg: helpers.PMGroupArgOwn}
	sbanDesc       = helpers.CommandDescriptor{Name: "sban", PMGroupArg: helpers.PMGroupArgOwn}
	tbanDesc       = helpers.CommandDescriptor{Name: "tban", PMGroupArg: helpers.PMGroupArgOwn}
	dbanDesc       = helpers.CommandDescriptor{Name: "dban"}
	unbanDesc      = helpers.CommandDescriptor{Name: "unban", PMGroupArg: helpers.PMGroupArgOwn}
	banmeDesc      = helpers.CommandDescriptor{Name: "banme", Disableable: true}
	unbanAllDesc   = helpers.CommandDescriptor{Name: "unbanall", PMGroupArg: helpers.PMGroupArgNone}
	kickDesc       = helpers.CommandDescriptor{Name: "kick", PMGroupArg: helpers.PMGroupArgOwn}
	dkickDesc      = helpers.CommandDescriptor{Name: "dkick"}
	skickDesc      = helpers.CommandDescriptor{Name: "skick", PMGroupArg: helpers.PMGroupArgOwn}
	kickmeDesc     = helpers.CommandDescriptor{Name: "kickme", Disableable: true}
	restrictDesc   = helpers.CommandDescriptor{Name: "restrict", PMGroupArg: helpers.PMGroupArgOwn}
	unrestrictDesc = helpers.CommandDescriptor{Name: "unrestrict", PMGroupArg: helpers.PMGroupArgOwn}
)

func initBanDescs() {
	banDesc.RequiredChecks = restrictChecks("ban", pmTargetChatCheck)
	sbanDesc.RequiredChecks = deleteRestrictChecks("sban", pmTargetChatCheck)
	tbanDesc.RequiredChecks = restrictChecks("tban", pmTargetChatCheck)
	dbanDesc.RequiredChecks = deleteRestrictChecks("dban")
	unbanDesc.RequiredChecks = restrictChecks("unban", pmTargetChatCheck)
	banmeDesc.RequiredChecks = []helpers.CheckFunc{helpers.CheckDisabled("banme"), helpers.RequireGroup(), helpers.CanBotRestrict()}
	unbanAllDesc.RequiredChecks = []helpers.CheckFunc{helpers.CheckDisabled("unbanall"), groupOnlyChatCheck, helpers.RequireGroup(), helpers.RequireUserOwner(), helpers.CanBotRestrict()}
	kickDesc.RequiredChecks = restrictChecks("kick", pmTargetChatCheck)
	dkickDesc.RequiredChecks = deleteRestrictChecks("dkick")
	skickDesc.RequiredChecks = deleteRestrictChecks("skick", pmTargetChatCheck)
	kickmeDesc.RequiredChecks = []helpers.CheckFunc{helpers.CheckDisabled("kickme"), helpers.RequireGroup(), helpers.CanBotRestrict()}
	restrictDesc.RequiredChecks = []helpers.CheckFunc{helpers.CheckDisabled("restrict"), pmTargetChatCheck, helpers.RequireGroup(), helpers.CanUserRestrict(), helpers.CanBotRestrict()}
	unrestrictDesc.RequiredChecks = []helpers.CheckFunc{helpers.CheckDisabled("unrestrict"), pmTargetChatCheck, helpers.RequireGroup(), helpers.CanUserRestrict(), helpers.CanBotRestrict()}
	initBanRecordDescs()
}

func LoadBans(dispatcher *ext.Dispatcher) {
	SetModuleEnabled(bansModule.moduleName, true)
	initBanDescs()
	startModerationEventRetentionSweeper()
	startBanRecordRetentionSweeper()

	helpers.WrapCommand(dispatcher, banDesc, pipelineHandler(bansModule.ban))
	helpers.WrapCommand(dispatcher, sbanDesc, pipelineHandler(bansModule.sBan))
	helpers.WrapCommand(dispatcher, tbanDesc, pipelineHandler(bansModule.tBan))
	helpers.WrapCommand(dispatcher, dbanDesc, pipelineHandler(bansModule.dBan))
	helpers.WrapCommand(dispatcher, unbanDesc, pipelineHandler(bansModule.unban))
	helpers.WrapCommand(dispatcher, banmeDesc, pipelineHandler(bansModule.banme))
	helpers.WrapCommand(dispatcher, unbanAllDesc, pipelineHandler(bansModule.unbanAllHandler))
	helpers.WrapCommand(dispatcher, banInfoDesc, pipelineHandler(bansModule.banInfo))
	helpers.WrapCommand(dispatcher, banListDesc, pipelineHandler(bansModule.banList))

	helpers.WrapCommand(dispatcher, kickDesc, pipelineHandler(bansModule.kick))
	helpers.WrapCommand(dispatcher, dkickDesc, pipelineHandler(bansModule.dkick))
	helpers.WrapCommand(dispatcher, skickDesc, pipelineHandler(bansModule.sKick))
	helpers.WrapCommand(dispatcher, kickmeDesc, pipelineHandler(bansModule.kickme))

	helpers.WrapCommand(dispatcher, restrictDesc, pipelineHandler(bansModule.restrict))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("restrict"), bansModule.restrictButtonHandler))
	helpers.WrapCommand(dispatcher, unrestrictDesc, pipelineHandler(bansModule.unrestrict))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("unrestrict"), bansModule.unrestrictButtonHandler))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("unbanall|"), bansModule.unbanAllCallback))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix(banListNamespace+"|"), bansModule.banListCallback))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix(banLogUnbanNamespace+"|"), bansModule.banLogUnbanCallback))
}

func init() {
	RegisterLegacyModule("Bans", 70, LoadBans)
	initBanDescs()
	RegisterAnonymousAdminHandler("ban", anonPipelineHandler(banDesc, bansModule.ban))
	RegisterAnonymousAdminHandler("dban", anonPipelineHandler(dbanDesc, bansModule.dBan))
	RegisterAnonymousAdminHandler("sban", anonPipelineHandler(sbanDesc, bansModule.sBan))
	RegisterAnonymousAdminHandler("tban", anonPipelineHandler(tbanDesc, bansModule.tBan))
	RegisterAnonymousAdminHandler("unban", anonPipelineHandler(unbanDesc, bansModule.unban))
	RegisterAnonymousAdminHandler("baninfo", anonPipelineHandler(banInfoDesc, bansModule.banInfo))
	RegisterAnonymousAdminHandler("banlist", anonPipelineHandler(banListDesc, bansModule.banList))
	RegisterAnonymousAdminHandler("kick", anonPipelineHandler(kickDesc, bansModule.kick))
	RegisterAnonymousAdminHandler("dkick", anonPipelineHandler(dkickDesc, bansModule.dkick))
	RegisterAnonymousAdminHandler("skick", anonPipelineHandler(skickDesc, bansModule.sKick))
	RegisterAnonymousAdminHandler("restrict", anonPipelineHandler(restrictDesc, bansModule.restrict))
	RegisterAnonymousAdminHandler("unrestrict", anonPipelineHandler(unrestrictDesc, bansModule.unrestrict))
}
