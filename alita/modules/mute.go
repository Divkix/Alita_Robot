package modules

import (
	"fmt"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/extraction"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

var mutesModule = moduleStruct{moduleName: "Mutes"}

func muteTargetValidation(c *moderationCtx, t *target) error {
	return validateTarget(c, t.userID, true, "common_user_not_in_chat", "mutes_mute_admin_error", "mutes_restrict_self_error")
}

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
							Text:         trS(c.Tr, "mutes_unmute_button"),
							CallbackData: encodeCallbackData("unrestrict", map[string]string{"a": "unmute", "u": fmt.Sprint(t.userID)}),
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

func moderationTmute(m *moduleStruct) *moderationCommand {
	return &moderationCommand{
		module:   m,
		gates:    []gateFn{standardModGates},
		extract:  extractFromArgs,
		validate: muteTargetValidation,
		execute: func(c *moderationCtx, t *target) error {
			_time := extractTemporalTarget(c, t)
			if _time == -1 {
				return ext.EndGroups
			}
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
		logAction: "tmute",
	}
}

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
		reply:     muteReplyWithButton,
		logAction: "mute",
	}
}

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
		logAction: "smute",
	}
}

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
		reply:     muteReplyWithButton,
		logAction: "dmute",
	}
}

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
		logAction: "unmute",
	}
}

func (m moduleStruct) tMute(b *gotgbot.Bot, ctx *ext.Context) error {
	return moderationTmute(&m).run(b, ctx)
}

func (m moduleStruct) mute(b *gotgbot.Bot, ctx *ext.Context) error {
	return moderationMute(&m).run(b, ctx)
}

func (m moduleStruct) sMute(b *gotgbot.Bot, ctx *ext.Context) error {
	return moderationSmute(&m).run(b, ctx)
}

func (m moduleStruct) dMute(b *gotgbot.Bot, ctx *ext.Context) error {
	return moderationDmute(&m).run(b, ctx)
}

func (m moduleStruct) unmute(b *gotgbot.Bot, ctx *ext.Context) error {
	return moderationUnmute(&m).run(b, ctx)
}

var (
	muteDesc   = helpers.CommandDescriptor{Name: "mute"}
	smuteDesc  = helpers.CommandDescriptor{Name: "smute"}
	tmuteDesc  = helpers.CommandDescriptor{Name: "tmute"}
	dmuteDesc  = helpers.CommandDescriptor{Name: "dmute"}
	unmuteDesc = helpers.CommandDescriptor{Name: "unmute"}
)

func initMuteDescs() {
	muteDesc.RequiredChecks = restrictChecks("mute")
	smuteDesc.RequiredChecks = deleteRestrictChecks("smute")
	tmuteDesc.RequiredChecks = restrictChecks("tmute")
	dmuteDesc.RequiredChecks = deleteRestrictChecks("dmute")
	unmuteDesc.RequiredChecks = restrictChecks("unmute")
}

func LoadMutes(dispatcher *ext.Dispatcher) {
	DefaultHelpRegistry().AbleMap[mutesModule.moduleName] = true
	initMuteDescs()

	helpers.WrapCommand(dispatcher, muteDesc, pipelineHandler(mutesModule.mute))
	helpers.WrapCommand(dispatcher, smuteDesc, pipelineHandler(mutesModule.sMute))
	helpers.WrapCommand(dispatcher, tmuteDesc, pipelineHandler(mutesModule.tMute))
	helpers.WrapCommand(dispatcher, dmuteDesc, pipelineHandler(mutesModule.dMute))
	helpers.WrapCommand(dispatcher, unmuteDesc, pipelineHandler(mutesModule.unmute))
}

func init() {
	RegisterLegacyModule("Mutes", 80, LoadMutes)
	initMuteDescs()
	RegisterAnonymousAdminHandler("mute", anonPipelineHandler(muteDesc, mutesModule.mute))
	RegisterAnonymousAdminHandler("smute", anonPipelineHandler(smuteDesc, mutesModule.sMute))
	RegisterAnonymousAdminHandler("dmute", anonPipelineHandler(dmuteDesc, mutesModule.dMute))
	RegisterAnonymousAdminHandler("tmute", anonPipelineHandler(tmuteDesc, mutesModule.tMute))
	RegisterAnonymousAdminHandler("unmute", anonPipelineHandler(unmuteDesc, mutesModule.unmute))
}
