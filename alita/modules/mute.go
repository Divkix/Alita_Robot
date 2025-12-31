package modules

import (
	"fmt"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/extraction"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

var mutesModule = moduleStruct{moduleName: "Mutes"}

// tMute handles the /tmute command to temporarily mute a user
// with a specified time duration, requiring admin permissions.
func (moduleStruct) tMute(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx, false)
	if user == nil {
		return ext.EndGroups
	}
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
		text, _ := tr.GetString("common_anonymous_user_error")
		_, err := msg.Reply(b, text, nil)
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	} else if userId == 0 {
		text, _ := tr.GetString("common_no_user_specified")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	// User should be in chat for getting restricted
	if !chat_status.IsUserInChat(b, chat, userId) {
		text, _ := tr.GetString("common_user_not_in_chat")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}
	if chat_status.IsUserBanProtected(b, ctx, nil, userId) {
		text, _ := tr.GetString("mutes_mute_admin_error")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if userId == b.Id {
		text, _ := tr.GetString("mutes_restrict_self_error")
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

	_, err := chat.RestrictMember(b, userId, gotgbot.ChatPermissions{
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
		&gotgbot.RestrictChatMemberOpts{
			UntilDate: _time,
		},
	)
	if err != nil {
		log.Error(err)
		return err
	}

	muteUser, err := b.GetChat(userId, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	temp, _ := tr.GetString("mutes_tmute_message")
	baseStr := fmt.Sprintf(temp, helpers.MentionHtml(muteUser.Id, muteUser.FirstName), timeVal)
	if reason != "" {
		temp, _ := tr.GetString("mutes_reason_suffix")
		baseStr += fmt.Sprintf(temp, reason)
	}

	_, err = msg.Reply(b,
		baseStr,
		helpers.Shtml(),
	)
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// mute handles the /mute command to permanently mute a user
// from the group, requiring admin permissions.
func (moduleStruct) mute(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx, false)
	if user == nil {
		return ext.EndGroups
	}
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
		text, _ := tr.GetString("common_anonymous_user_error")
		_, err := msg.Reply(b, text, nil)
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	} else if userId == 0 {
		text, _ := tr.GetString("common_no_user_specified")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	// User should be in chat for getting restricted
	if !chat_status.IsUserInChat(b, chat, userId) {
		text, _ := tr.GetString("common_user_not_in_chat")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}
	if chat_status.IsUserBanProtected(b, ctx, nil, userId) {
		text, _ := tr.GetString("mutes_mute_admin_polite_error")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if userId == b.Id {
		text, _ := tr.GetString("mutes_restrict_self_error")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	_, err := chat.RestrictMember(b, userId, gotgbot.ChatPermissions{
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
	}, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	muteUser, err := b.GetChat(userId, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	temp, _ := tr.GetString("mutes_mute_message")
	baseStr := temp
	if reason != "" {
		temp, _ := tr.GetString("mutes_reason_suffix")
		baseStr += fmt.Sprintf(temp, reason)
	}

	_, err = msg.Reply(b,
		fmt.Sprintf(baseStr, helpers.MentionHtml(muteUser.Id, muteUser.FirstName)),
		&gotgbot.SendMessageOpts{
			ParseMode: helpers.HTML,
			ReplyMarkup: gotgbot.InlineKeyboardMarkup{
				InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
					{
						{
							Text:         func() string { t, _ := tr.GetString("mutes_unmute_button"); return t }(),
							CallbackData: fmt.Sprintf("unrestrict.unmute.%d", userId),
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

// sMute handles the /smute command to silently mute a user
// and delete the command message, requiring admin permissions.
func (moduleStruct) sMute(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx, false)
	if user == nil {
		return ext.EndGroups
	}
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
		text, _ := tr.GetString("common_anonymous_user_error")
		_, err := msg.Reply(b, text, nil)
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	} else if userId == 0 {
		text, _ := tr.GetString("common_no_user_specified")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	// User should be in chat for getting restricted
	if !chat_status.IsUserInChat(b, chat, userId) {
		text, _ := tr.GetString("common_user_not_in_chat")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if chat_status.IsUserBanProtected(b, ctx, nil, userId) {
		text, _ := tr.GetString("mutes_mute_admin_error")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	// Check if trying to mute the bot itself (silent return since this is smute)
	if userId == b.Id {
		return ext.EndGroups
	}

	_, err := chat.RestrictMember(b, userId, gotgbot.ChatPermissions{
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
	}, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	_ = helpers.DeleteMessageWithErrorHandling(b, chat.Id, msg.MessageId)

	return ext.EndGroups
}

// dMute handles the /dmute command to mute a user and delete
// the replied message, requiring admin permissions.
func (moduleStruct) dMute(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx, false)
	if user == nil {
		return ext.EndGroups
	}
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

	userId, reason := extraction.ExtractUserAndText(b, ctx)
	if userId == -1 {
		return ext.EndGroups
	} else if helpers.IsChannelID(userId) {
		text, _ := tr.GetString("common_anonymous_user_error")
		_, err := msg.Reply(b, text, nil)
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	} else if userId == 0 {
		text, _ := tr.GetString("common_no_user_specified")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	// User should be in chat for getting restricted
	if !chat_status.IsUserInChat(b, chat, userId) {
		text, _ := tr.GetString("common_user_not_in_chat")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if chat_status.IsUserBanProtected(b, ctx, nil, userId) {
		text, _ := tr.GetString("mutes_mute_admin_polite_error")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	// Check if trying to mute the bot itself
	if userId == b.Id {
		text, _ := tr.GetString("mutes_restrict_self_error")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if msg.ReplyToMessage == nil {
		text, _ := tr.GetString("mute_reply_to_dmute")
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

	_, err = chat.RestrictMember(b, userId, gotgbot.ChatPermissions{
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
	}, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	muteUser, err := b.GetChat(userId, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	temp, _ := tr.GetString("mutes_mute_message")
	baseStr := temp
	if reason != "" {
		temp, _ := tr.GetString("mutes_reason_suffix")
		baseStr += fmt.Sprintf(temp, reason)
	}

	_, err = msg.Reply(b,
		fmt.Sprintf(baseStr, helpers.MentionHtml(muteUser.Id, muteUser.FirstName)),
		&gotgbot.SendMessageOpts{
			ParseMode: helpers.HTML,
			ReplyMarkup: gotgbot.InlineKeyboardMarkup{
				InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
					{
						{
							Text:         func() string { t, _ := tr.GetString("mutes_unmute_button"); return t }(),
							CallbackData: fmt.Sprintf("unrestrict.unmute.%d", userId),
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

// unmute handles the /unmute command to restore chat permissions
// to a previously muted user, requiring admin permissions.
func (moduleStruct) unmute(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx, false)
	if user == nil {
		return ext.EndGroups
	}
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

	userId := extraction.ExtractUser(b, ctx)
	if userId == -1 {
		return ext.EndGroups
	} else if helpers.IsChannelID(userId) {
		text, _ := tr.GetString("common_anonymous_user_error")
		_, err := msg.Reply(b, text, nil)
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	} else if userId == 0 {
		text, _ := tr.GetString("common_no_user_specified")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	// User should be in chat for getting restricted
	if !chat_status.IsUserInChat(b, chat, userId) {
		text, _ := tr.GetString("common_user_not_in_chat")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if userId == b.Id {
		text, _ := tr.GetString("mutes_restrict_self_error")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	c, err := b.GetChat(chat.Id, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	// should give the current chat permissions to the users who is unmuted
	_, err = chat.RestrictMember(
		b,
		userId,
		*c.Permissions,
		nil,
	)
	if err != nil {
		log.Error(err)
		return err
	}

	muteUser, err := b.GetChat(userId, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	temp, _ := tr.GetString("mutes_unmute_message")
	_, err = msg.Reply(b,
		fmt.Sprintf(temp, helpers.MentionHtml(muteUser.Id, muteUser.FirstName)),
		helpers.Shtml(),
	)
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// LoadMutes registers all mute module handlers with the dispatcher,
// including various mute commands and their variants.
func LoadMutes(dispatcher *ext.Dispatcher) {
	HelpModule.AbleMap.Store(mutesModule.moduleName, true)

	dispatcher.AddHandler(handlers.NewCommand("mute", mutesModule.mute))
	dispatcher.AddHandler(handlers.NewCommand("smute", mutesModule.sMute))
	dispatcher.AddHandler(handlers.NewCommand("tmute", mutesModule.tMute))
	dispatcher.AddHandler(handlers.NewCommand("dmute", mutesModule.dMute))
	dispatcher.AddHandler(handlers.NewCommand("unmute", mutesModule.unmute))
}
