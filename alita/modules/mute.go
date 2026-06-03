package modules

import (
	"fmt"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/extraction"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

var mutesModule = moduleStruct{moduleName: "Mutes"}

// muteTargetValidation validates the target for mute commands.
// Checks: user is in chat, not ban-protected, not the bot itself.
func muteTargetValidation(c *moderationCtx, t *target) error {
	if !chat_status.IsUserInChat(c.Bot, c.Chat, t.userID) {
		text, _ := c.Tr.GetString("common_user_not_in_chat")
		_, err := c.Msg.Reply(c.Bot, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return errUserNotInChat
	}
	if chat_status.IsUserBanProtected(c.Bot, c.Ctx, nil, t.userID) {
		text, _ := c.Tr.GetString("mutes_mute_admin_error")
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
		text, _ := c.Tr.GetString("mutes_restrict_self_error")
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

// muteReplyWithButton builds and sends the success reply for mute commands
// with an inline unmute button.
func muteReplyWithButton(c *moderationCtx, t *target) error {
	muteUser, err := c.Bot.GetChat(t.userID, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	baseStr, _ := c.Tr.GetString("mutes_mute_message")
	if t.reason != "" {
		temp, _ := c.Tr.GetString("mutes_reason_suffix")
		baseStr += fmt.Sprintf(temp, t.reason)
	}

	_, err = c.Msg.Reply(c.Bot,
		fmt.Sprintf(baseStr, formatting.MentionHtml(muteUser.Id, muteUser.FirstName)),
		&gotgbot.SendMessageOpts{
			ParseMode: formatting.HTML,
			ReplyMarkup: gotgbot.InlineKeyboardMarkup{
				InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
					{
						{
							Text:         func() string { t, _ := c.Tr.GetString("mutes_unmute_button"); return t }(),
							CallbackData: encodeCallbackData("unrestrict", map[string]string{"a": "unmute", "u": fmt.Sprint(t.userID)}, fmt.Sprintf("unrestrict.unmute.%d", t.userID)),
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

// extractUserOnly resolves the target from command arguments using ExtractUser.
// It rejects channel IDs and validates the user ID.
func extractUserOnly(c *moderationCtx) (target, error) {
	uid := extraction.ExtractUser(c.Bot, c.Ctx)
	if uid == -1 {
		return target{}, fmt.Errorf("extraction failed")
	}
	if chat_status.IsChannelId(uid) {
		text, _ := c.Tr.GetString("common_anonymous_user_error")
		_, err := c.Msg.Reply(c.Bot, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return target{}, err
		}
		return target{}, fmt.Errorf("anonymous user")
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

// moderationTmute is the shared moderationCommand definition for /tmute.
func moderationTmute(m *moduleStruct) *moderationCommand {
	return &moderationCommand{
		module:   m,
		gates:    []gateFn{standardModGates},
		extract:  extractFromArgs,
		validate: muteTargetValidation,
		execute: func(c *moderationCtx, t *target) error {
			_time, timeVal, reason := extraction.ExtractTime(c.Bot, c.Ctx, t.reason)
			if _time == -1 {
				return ext.EndGroups
			}
			t.timeVal = timeVal
			t.reason = reason
			_, err := c.Chat.RestrictMember(c.Bot, t.userID, MutedPermissions,
				&gotgbot.RestrictChatMemberOpts{UntilDate: _time},
			)
			return err
		},
		reply: func(c *moderationCtx, t *target) error {
			muteUser, err := c.Bot.GetChat(t.userID, nil)
			if err != nil {
				log.Error(err)
				return err
			}

			temp, _ := c.Tr.GetString("mutes_tmute_message")
			baseStr := fmt.Sprintf(temp, formatting.MentionHtml(muteUser.Id, muteUser.FirstName), t.timeVal)
			if t.reason != "" {
				temp, _ := c.Tr.GetString("mutes_reason_suffix")
				baseStr += fmt.Sprintf(temp, t.reason)
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

// moderationMute is the shared moderationCommand definition for /mute.
func moderationMute(m *moduleStruct) *moderationCommand {
	return &moderationCommand{
		module:   m,
		gates:    []gateFn{standardModGates},
		extract:  extractFromArgs,
		validate: muteTargetValidation,
		execute: func(c *moderationCtx, t *target) error {
			_, err := c.Chat.RestrictMember(c.Bot, t.userID, MutedPermissions, nil)
			return err
		},
		reply: muteReplyWithButton,
	}
}

// moderationSmute is the shared moderationCommand definition for /smute.
func moderationSmute(m *moduleStruct) *moderationCommand {
	return &moderationCommand{
		module:   m,
		gates:    []gateFn{deleteModGates},
		extract:  extractUserOnly,
		validate: muteTargetValidation,
		execute: func(c *moderationCtx, t *target) error {
			_, err := c.Chat.RestrictMember(c.Bot, t.userID, MutedPermissions, nil)
			return err
		},
		reply: func(c *moderationCtx, t *target) error {
			_ = helpers.DeleteMessageWithErrorHandling(c.Bot, c.Chat.Id, c.Msg.MessageId)
			return nil
		},
	}
}

// moderationDmute is the shared moderationCommand definition for /dmute.
func moderationDmute(m *moduleStruct) *moderationCommand {
	return &moderationCommand{
		module:  m,
		gates:   []gateFn{deleteModGates},
		extract: extractFromArgs,
		validate: func(c *moderationCtx, t *target) error {
			if c.Msg.ReplyToMessage == nil {
				text, _ := c.Tr.GetString("mute_reply_to_dmute")
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
			return muteTargetValidation(c, t)
		},
		execute: func(c *moderationCtx, t *target) error {
			_, err := c.Msg.ReplyToMessage.Delete(c.Bot, nil)
			if err != nil {
				log.Error(err)
				return err
			}
			_, err = c.Chat.RestrictMember(c.Bot, t.userID, MutedPermissions, nil)
			return err
		},
		reply: muteReplyWithButton,
	}
}

// moderationUnmute is the shared moderationCommand definition for /unmute.
func moderationUnmute(m *moduleStruct) *moderationCommand {
	return &moderationCommand{
		module:  m,
		gates:   []gateFn{standardModGates},
		extract: extractUserOnly,
		validate: func(c *moderationCtx, t *target) error {
			if !chat_status.IsUserInChat(c.Bot, c.Chat, t.userID) {
				text, _ := c.Tr.GetString("common_user_not_in_chat")
				_, err := c.Msg.Reply(c.Bot, text, formatting.Shtml())
				if err != nil {
					log.Error(err)
					return err
				}
				return errUserNotInChat
			}
			if t.userID == c.Bot.Id {
				text, _ := c.Tr.GetString("mutes_restrict_self_error")
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
			chat, err := c.Bot.GetChat(c.Chat.Id, nil)
			if err != nil {
				log.Error(err)
				return err
			}
			unmutePermissions := resolveUnmutePermissions(chat)
			_, err = c.Chat.RestrictMember(c.Bot, t.userID, unmutePermissions, nil)
			return err
		},
		reply: func(c *moderationCtx, t *target) error {
			muteUser, err := c.Bot.GetChat(t.userID, nil)
			if err != nil {
				log.Error(err)
				return err
			}

			temp, _ := c.Tr.GetString("mutes_unmute_message")
			_, err = c.Msg.Reply(c.Bot,
				fmt.Sprintf(temp, formatting.MentionHtml(muteUser.Id, muteUser.FirstName)),
				formatting.Shtml(),
			)
			if err != nil {
				log.Error(err)
				return err
			}
			return nil
		},
	}
}

// tMute handles the /tmute command to temporarily mute a user
// with a specified time duration, requiring admin permissions.
func (m moduleStruct) tMute(b *gotgbot.Bot, ctx *ext.Context) error {
	return moderationTmute(&m).run(b, ctx)
}

// mute handles the /mute command to permanently mute a user
// from the group, requiring admin permissions.
func (m moduleStruct) mute(b *gotgbot.Bot, ctx *ext.Context) error {
	return moderationMute(&m).run(b, ctx)
}

// sMute handles the /smute command to silently mute a user
// and delete the command message, requiring admin permissions.
func (m moduleStruct) sMute(b *gotgbot.Bot, ctx *ext.Context) error {
	return moderationSmute(&m).run(b, ctx)
}

// dMute handles the /dmute command to mute a user and delete
// the replied message, requiring admin permissions.
func (m moduleStruct) dMute(b *gotgbot.Bot, ctx *ext.Context) error {
	return moderationDmute(&m).run(b, ctx)
}

// unmute handles the /unmute command to restore chat permissions
// to a previously muted user, requiring admin permissions.
func (m moduleStruct) unmute(b *gotgbot.Bot, ctx *ext.Context) error {
	return moderationUnmute(&m).run(b, ctx)
}

// LoadMutes registers all mute module handlers with the dispatcher,
// including various mute commands and their variants.
func LoadMutes(dispatcher *ext.Dispatcher) {
	DefaultHelpRegistry().AbleMap.Store(mutesModule.moduleName, true)

	dispatcher.AddHandler(handlers.NewCommand("mute", mutesModule.mute))
	dispatcher.AddHandler(handlers.NewCommand("smute", mutesModule.sMute))
	dispatcher.AddHandler(handlers.NewCommand("tmute", mutesModule.tMute))
	dispatcher.AddHandler(handlers.NewCommand("dmute", mutesModule.dMute))
	dispatcher.AddHandler(handlers.NewCommand("unmute", mutesModule.unmute))
}

func init() {
	RegisterLegacyModule("Mutes", 80, LoadMutes)
	RegisterAnonymousAdminHandler("mute", mutesModule.mute)
	RegisterAnonymousAdminHandler("smute", mutesModule.sMute)
	RegisterAnonymousAdminHandler("dmute", mutesModule.dMute)
	RegisterAnonymousAdminHandler("tmute", mutesModule.tMute)
	RegisterAnonymousAdminHandler("unmute", mutesModule.unmute)
}
