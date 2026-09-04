package modules

import (
	"bytes"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/admin"
	"github.com/divkix/Alita_Robot/alita/db/devs"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"

	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
	"github.com/divkix/Alita_Robot/alita/utils/extraction"
)

var adminModule = moduleStruct{moduleName: "Admin"}

func (m moduleStruct) adminlist(c *helpers.CommandContext) error {
	chat := c.Chat
	msg := c.Msg
	cached := true

	tr := c.Tr

	temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_adminlist")
	text := fmt.Sprintf(temp, formatting.HtmlEscape(chat.Title))

	adminsAvail, admins := cache.GetAdminCacheList(chat.Id)
	if !adminsAvail {
		admins = cache.LoadAdminCache(c.Bot, chat.Id)
		cached = false
	}

	var sb strings.Builder
	for i := range admins.UserInfo {
		admin := &admins.UserInfo[i]
		user := admin.User
		if user.IsBot || admin.IsAnonymous {
			continue
		}
		if user.Username != "" {
			fmt.Fprintf(&sb, "\n- @%s", formatting.HtmlEscape(user.Username))
		} else {
			fmt.Fprintf(&sb, "\n- %s", formatting.MentionHtml(user.Id, user.FirstName))
		}
	}
	if sb.Len() == 0 {
		noVisibleText, _ := tr.GetString("admin_no_visible_admins")
		text += noVisibleText
	} else {
		text += sb.String()
	}
	if !cached {
		noteText, _ := tr.GetString("admin_adminlist_note_fresh")
		text += noteText
	} else {
		noteText, _ := tr.GetString("admin_adminlist_note_cached")
		text += noteText
	}
	_, err := msg.Reply(c.Bot, text, formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func (m moduleStruct) loadAdminCacheOrFail(c *helpers.CommandContext) *cache.AdminCache {
	adminsAvail, admins := cache.GetAdminCacheList(c.Chat.Id)
	if !adminsAvail {
		admins = cache.LoadAdminCache(c.Bot, c.Chat.Id)
	}
	if len(admins.UserInfo) == 0 {
		text, _ := c.Tr.GetString(strings.ToLower(m.moduleName) + "_errors_admin_cache_failed")
		_, err := c.Msg.Reply(c.Bot, text, nil)
		if err != nil {
			log.Error(err)
		}
		return nil
	}
	return &admins
}

func (m moduleStruct) validateDemotionTarget(c *helpers.CommandContext) (int64, error) {
	userId := extraction.ExtractUser(c.Bot, c.Ctx)
	if userId == -1 {
		return 0, ext.EndGroups
	} else if chat_status.IsChannelId(userId) {
		text, _ := c.Tr.GetString("common_anonymous_user_error")
		_, err := c.Msg.Reply(c.Bot, text, nil)
		if err != nil {
			log.Error(err)
			return 0, err
		}
		return 0, ext.EndGroups
	} else if userId == 0 {
		text, _ := c.Tr.GetString("common_no_user_specified")
		_, err := c.Msg.Reply(c.Bot, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return 0, err
		}
		return 0, ext.EndGroups
	}

	if chat_status.RequireUserOwner(c.Bot, c.Ctx, nil, userId) {
		text, _ := c.Tr.GetString(strings.ToLower(m.moduleName) + "_demote_is_owner")
		_, err := c.Msg.Reply(c.Bot, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return 0, err
		}
		return 0, ext.EndGroups
	}
	if userId == c.Bot.Id {
		text, _ := c.Tr.GetString(strings.ToLower(m.moduleName) + "_demote_is_bot_itself")
		_, err := c.Msg.Reply(c.Bot, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return 0, err
		}
		return 0, ext.EndGroups
	}
	if !chat_status.IsUserAdmin(c.Bot, c.Chat.Id, userId) {
		text, _ := c.Tr.GetString(strings.ToLower(m.moduleName) + "_demote_not_admin")
		_, err := c.Msg.Reply(c.Bot, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return 0, err
		}
		return 0, ext.EndGroups
	}

	return userId, nil
}

func (m moduleStruct) performDemotion(c *helpers.CommandContext, userId int64) error {
	bb, err := c.Chat.PromoteMember(c.Bot,
		userId,
		&gotgbot.PromoteChatMemberOpts{
			CanPostMessages:     false,
			CanDeleteMessages:   false,
			CanRestrictMembers:  helpers.Ptr(false),
			CanChangeInfo:       false,
			CanInviteUsers:      false,
			CanPinMessages:      false,
			CanManageVideoChats: false,
			CanManageTopics:     false,
		},
	)
	if err != nil || !bb {
		log.Error(err)
		text, _ := c.Tr.GetString(strings.ToLower(m.moduleName) + "_errors_err_cannot_demote")
		_, err = c.Msg.Reply(c.Bot, text, nil)
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	cache.InvalidateAdminCache(c.Chat.Id)

	userMember, err := c.Chat.GetMember(c.Bot, userId, nil)
	if err != nil {
		log.Error(err)
		return err
	}
	if userMember == nil {
		err := fmt.Errorf("GetMember returned nil for userId %d", userId)
		log.Error(err)
		return err
	}

	mem := userMember.MergeChatMember().User
	_, err = c.Msg.Reply(c.Bot,
		func() string {
			temp, _ := c.Tr.GetString(strings.ToLower(m.moduleName) + "_demote_success_demote")
			return fmt.Sprintf(temp, formatting.MentionHtml(mem.Id, mem.FirstName))
		}(),
		formatting.Shtml(),
	)
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}

func (m moduleStruct) demote(c *helpers.CommandContext) error {
	admins := m.loadAdminCacheOrFail(c)
	if admins == nil {
		return ext.EndGroups
	}

	userId, err := m.validateDemotionTarget(c)
	if err != nil {
		return err
	}

	return m.performDemotion(c, userId)
}

func (m moduleStruct) validatePromotionTarget(c *helpers.CommandContext) (int64, string, error) {
	userId, customTitle := extraction.ExtractUserAndText(c.Bot, c.Ctx)
	if userId == -1 {
		return 0, "", ext.EndGroups
	} else if chat_status.IsChannelId(userId) {
		text, _ := c.Tr.GetString("common_anonymous_user_error")
		_, err := c.Msg.Reply(c.Bot, text, nil)
		if err != nil {
			log.Error(err)
			return 0, "", err
		}
		return 0, "", ext.EndGroups
	} else if userId == 0 {
		text, _ := c.Tr.GetString("common_no_user_specified")
		_, err := c.Msg.Reply(c.Bot, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return 0, "", err
		}
		return 0, "", ext.EndGroups
	}
	if userId == c.Bot.Id {
		text, _ := c.Tr.GetString(strings.ToLower(m.moduleName) + "_promote_is_bot_itself")
		_, err := c.Msg.Reply(c.Bot, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return 0, "", err
		}
		return 0, "", ext.EndGroups
	}
	if chat_status.RequireUserOwner(c.Bot, c.Ctx, nil, userId) {
		text, _ := c.Tr.GetString(strings.ToLower(m.moduleName) + "_promote_is_owner")
		_, err := c.Msg.Reply(c.Bot, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return 0, "", err
		}
		return 0, "", ext.EndGroups
	}
	if chat_status.IsUserAdmin(c.Bot, c.Chat.Id, userId) {
		text, _ := c.Tr.GetString(strings.ToLower(m.moduleName) + "_promote_is_admin")
		_, err := c.Msg.Reply(c.Bot, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return 0, "", err
		}
		return 0, "", ext.EndGroups
	}
	return userId, customTitle, nil
}

func canGrantPerm(botHas, promoterHas, bypass bool) bool {
	return botHas && (promoterHas || bypass)
}

func buildPromoteOpts(botMember, promoterMember gotgbot.ChatMember, user *gotgbot.User, c *helpers.CommandContext) *gotgbot.PromoteChatMemberOpts {
	bMem := botMember.MergeChatMember()
	pMem := promoterMember.MergeChatMember()

	teamMem := devs.GetTeamMemInfo(user.Id)
	teamMemInfo := teamMem.Sudo || teamMem.IsDev
	isPromoterOwner := chat_status.RequireUserOwner(c.Bot, c.Ctx, nil, user.Id)
	checkCommonPerms := isPromoterOwner || teamMemInfo

	return &gotgbot.PromoteChatMemberOpts{
		CanPostMessages:     canGrantPerm(bMem.CanPostMessages, pMem.CanPostMessages, checkCommonPerms),
		CanDeleteMessages:   canGrantPerm(bMem.CanDeleteMessages, pMem.CanDeleteMessages, checkCommonPerms),
		CanRestrictMembers:  helpers.Ptr(canGrantPerm(bMem.CanRestrictMembers, pMem.CanRestrictMembers, checkCommonPerms)),
		CanChangeInfo:       canGrantPerm(bMem.CanChangeInfo, pMem.CanChangeInfo, checkCommonPerms),
		CanInviteUsers:      canGrantPerm(bMem.CanInviteUsers, pMem.CanInviteUsers, checkCommonPerms),
		CanPinMessages:      canGrantPerm(bMem.CanPinMessages, pMem.CanPinMessages, checkCommonPerms),
		CanManageVideoChats: canGrantPerm(bMem.CanManageVideoChats, pMem.CanManageVideoChats, checkCommonPerms),
		CanManageChat:       canGrantPerm(bMem.CanManageChat, pMem.CanManageChat, checkCommonPerms),
		CanManageTopics:     canGrantPerm(bMem.CanManageTopics, pMem.CanManageTopics, checkCommonPerms),
	}
}

func (m moduleStruct) handlePromotionSuccess(c *helpers.CommandContext, userId int64, customTitle string, userMember gotgbot.ChatMember) error {
	tr := c.Tr
	msg := c.Msg

	cache.InvalidateAdminCache(c.Chat.Id)

	extraText := ""
	titleRunes := []rune(customTitle)
	if len(titleRunes) > 16 {
		temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_promote_admin_title_truncated")
		extraText += fmt.Sprintf(temp, len(titleRunes))
		customTitle = string(titleRunes[0:16])
	}

	if customTitle != "" {
		_, err := c.Chat.SetAdministratorCustomTitle(c.Bot, userId, customTitle, nil)
		if err != nil {
			text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_errors_err_set_title")
			_, err = msg.Reply(c.Bot, text, nil)
			if err != nil {
				log.Error(err)
			}
			return ext.EndGroups
		}
	}

	mem := userMember.MergeChatMember().User
	_, err := msg.Reply(c.Bot,
		func() string {
			temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_promote_success_promote")
			return fmt.Sprintf(temp, formatting.MentionHtml(mem.Id, mem.FirstName))
		}()+extraText,
		formatting.Shtml(),
	)
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}

func (m moduleStruct) promote(c *helpers.CommandContext) error {
	admins := m.loadAdminCacheOrFail(c)
	if admins == nil {
		return ext.EndGroups
	}

	userId, customTitle, err := m.validatePromotionTarget(c)
	if err != nil {
		return err
	}

	userMember, err := c.Chat.GetMember(c.Bot, userId, nil)
	if err != nil {
		log.Error(err)
		return err
	}
	if userMember == nil {
		err := fmt.Errorf("GetMember returned nil for userId %d", userId)
		log.Error(err)
		return err
	}

	promoterMember, err := c.Chat.GetMember(c.Bot, c.User.Id, nil)
	if err != nil {
		log.Error(err)
		return err
	}
	if promoterMember == nil {
		err := fmt.Errorf("GetMember returned nil for promoterId %d", c.User.Id)
		log.Error(err)
		return err
	}

	botMember, err := c.Chat.GetMember(c.Bot, c.Bot.Id, nil)
	if err != nil {
		log.Error(err)
		return err
	}
	if botMember == nil {
		err := fmt.Errorf("GetMember returned nil for botId %d", c.Bot.Id)
		log.Error(err)
		return err
	}

	opts := buildPromoteOpts(botMember, promoterMember, c.User, c)

	status, err := c.Chat.PromoteMember(c.Bot, userId, opts)
	if err != nil || !status {
		text, _ := c.Tr.GetString(strings.ToLower(m.moduleName) + "_errors_err_cannot_promote")
		_, _ = c.Msg.Reply(c.Bot, text, formatting.Shtml())
		if err == nil {
			err = fmt.Errorf("promote member returned false status")
		}
		return err
	}

	return m.handlePromotionSuccess(c, userId, customTitle, userMember)
}

func (moduleStruct) getinvitelink(c *helpers.CommandContext) error {
	chat := c.Chat
	msg := c.Msg
	tr := c.Tr

	if chat.Username != "" {
		linkText, _ := tr.GetString("admin_invitelink_public")
		_, _ = msg.Reply(c.Bot, fmt.Sprintf(linkText, formatting.HtmlEscape(chat.Username)), nil)
	} else {
		nchat, err := c.Bot.GetChat(chat.Id, nil)
		if err != nil {
			_, _ = msg.Reply(c.Bot, err.Error(), nil)
			return ext.EndGroups
		}
		linkText, _ := tr.GetString("admin_invitelink_private")
		_, _ = msg.Reply(c.Bot, fmt.Sprintf(linkText, nchat.InviteLink), nil)
	}
	return ext.EndGroups
}

func (moduleStruct) setGTitle(c *helpers.CommandContext) error {
	msg := c.Msg
	tr := c.Tr
	title := strings.TrimSpace(strings.Join(c.Ctx.Args()[1:], " "))
	if title == "" {
		text, _ := tr.GetString("admin_setgtitle_need_title")
		_, _ = msg.Reply(c.Bot, text, formatting.Shtml())
		return ext.EndGroups
	}
	if _, err := c.Bot.SetChatTitle(c.Chat.Id, title, nil); err != nil {
		log.Error(err)
		text, _ := tr.GetString("admin_setgtitle_failed")
		_, _ = msg.Reply(c.Bot, text, formatting.Shtml())
		return ext.EndGroups
	}
	text, _ := tr.GetString("admin_setgtitle_ok")
	_, _ = msg.Reply(c.Bot, text, formatting.Shtml())
	return ext.EndGroups
}

func (moduleStruct) setGPic(c *helpers.CommandContext) error {
	msg := c.Msg
	tr := c.Tr
	if msg.ReplyToMessage == nil || len(msg.ReplyToMessage.Photo) == 0 {
		text, _ := tr.GetString("admin_setgpic_need_photo")
		_, _ = msg.Reply(c.Bot, text, formatting.Shtml())
		return ext.EndGroups
	}
	photo := msg.ReplyToMessage.Photo
	fileID := photo[len(photo)-1].FileId
	data, err := downloadTelegramFile(c.Bot, fileID)
	if err != nil || len(data) == 0 {
		log.Error(err)
		text, _ := tr.GetString("admin_setgpic_failed")
		_, _ = msg.Reply(c.Bot, text, formatting.Shtml())
		return ext.EndGroups
	}
	if _, err := c.Bot.SetChatPhoto(c.Chat.Id, gotgbot.InputFileByReader("photo.jpg", bytes.NewReader(data)), nil); err != nil {
		log.Error(err)
		text, _ := tr.GetString("admin_setgpic_failed")
		_, _ = msg.Reply(c.Bot, text, formatting.Shtml())
		return ext.EndGroups
	}
	text, _ := tr.GetString("admin_setgpic_ok")
	_, _ = msg.Reply(c.Bot, text, formatting.Shtml())
	return ext.EndGroups
}

func (moduleStruct) setGDesc(c *helpers.CommandContext) error {
	msg := c.Msg
	tr := c.Tr
	desc := strings.TrimSpace(strings.Join(c.Ctx.Args()[1:], " "))
	if _, err := c.Bot.SetChatDescription(c.Chat.Id, &gotgbot.SetChatDescriptionOpts{Description: desc}); err != nil {
		log.Error(err)
		text, _ := tr.GetString("admin_setgdesc_failed")
		_, _ = msg.Reply(c.Bot, text, formatting.Shtml())
		return ext.EndGroups
	}
	text, _ := tr.GetString("admin_setgdesc_ok")
	_, _ = msg.Reply(c.Bot, text, formatting.Shtml())
	return ext.EndGroups
}

func (m moduleStruct) setTitle(c *helpers.CommandContext) error {
	chat := c.Chat
	msg := c.Msg
	tr := c.Tr

	userId, customTitle := extraction.ExtractUserAndText(c.Bot, c.Ctx)
	if userId == -1 {
		return ext.EndGroups
	} else if chat_status.IsChannelId(userId) {
		text, _ := tr.GetString("common_anonymous_user_error")
		_, err := msg.Reply(c.Bot, text, nil)
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	} else if userId == 0 {
		text, _ := tr.GetString("common_no_user_specified")
		_, err := msg.Reply(c.Bot, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if chat_status.RequireUserOwner(c.Bot, c.Ctx, nil, userId) {
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_title_is_owner")
		_, err := msg.Reply(c.Bot, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}
	if !chat_status.IsUserAdmin(c.Bot, chat.Id, userId) {
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_title_is_admin")
		_, err := msg.Reply(c.Bot, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if userId == c.Bot.Id {
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_title_is_bot_itself")
		_, err := msg.Reply(c.Bot, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}

		return ext.EndGroups
	}

	var extraText string
	if customTitle == "" {
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_errors_title_empty")
		_, err := msg.Reply(c.Bot, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	} else if len([]rune(customTitle)) > 16 {
		runes := []rune(customTitle)
		temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_title_truncated")
		extraText = fmt.Sprintf(temp, len(runes))
		customTitle = string(runes[:16])
	}

	_, err := chat.SetAdministratorCustomTitle(c.Bot,
		userId,
		customTitle,
		nil,
	)
	if err != nil {
		log.Error(err)
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_errors_err_set_title")
		_, _ = msg.Reply(c.Bot, text, formatting.Shtml())
		return err
	}

	userMember, err := chat.GetMember(c.Bot, userId, nil)
	if err != nil {
		log.Error(err)
		return err
	}
	if userMember == nil {
		err := fmt.Errorf("GetMember returned nil for userId %d", userId)
		log.Error(err)
		return err
	}

	mem := userMember.MergeChatMember()

	_, err = msg.Reply(c.Bot,
		func() string {
			temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_title_success_set")
			return fmt.Sprintf(temp, mem.User.FirstName, mem.CustomTitle)
		}()+extraText,
		formatting.Shtml(),
	)
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func (m moduleStruct) anonAdmin(c *helpers.CommandContext) error {
	chat := c.Chat
	msg := c.Msg
	user := c.User
	args := c.Ctx.Args()

	tr := c.Tr
	var text string

	adminSettings := admin.GetAdminSettings(chat.Id)

	if len(args) == 1 {
		if adminSettings.AnonAdmin {
			temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_anon_admin_enabled")
			text = fmt.Sprintf(temp, formatting.HtmlEscape(chat.Title))
		} else {
			temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_anon_admin_disabled")
			text = fmt.Sprintf(temp, formatting.HtmlEscape(chat.Title))
		}
	} else {
		if !chat_status.RequireUserOwner(c.Bot, c.Ctx, nil, user.Id) {
			chat_status.NewPermissionResponder(c.Bot).Respond(c.Ctx, "chat_status_owner_cmd_error", "chat_status_owner_button_error", chat_status.WithReply())
			return ext.EndGroups
		}
		switch args[1] {
		case "on", "true", "yes":
			if adminSettings.AnonAdmin {
				temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_anon_admin_already_enabled")
				text = fmt.Sprintf(temp, formatting.HtmlEscape(chat.Title))
			} else {
				if err := admin.SetAnonAdminMode(chat.Id, true); err != nil {
					log.Errorf("[Admin] Failed to set anon admin mode for chat %d: %v", chat.Id, err)
					errorText, _ := tr.GetString(strings.ToLower(m.moduleName) + "_anon_admin_db_error")
					_, _ = msg.Reply(c.Bot, errorText, formatting.Shtml())
					return ext.EndGroups
				}
				temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_anon_admin_enabled_now")
				text = fmt.Sprintf(temp, formatting.HtmlEscape(chat.Title))
			}
		case "off", "no", "false":
			if !adminSettings.AnonAdmin {
				temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_anon_admin_already_disabled")
				text = fmt.Sprintf(temp, formatting.HtmlEscape(chat.Title))
			} else {
				if err := admin.SetAnonAdminMode(chat.Id, false); err != nil {
					log.Errorf("[Admin] Failed to set anon admin mode for chat %d: %v", chat.Id, err)
					errorText, _ := tr.GetString(strings.ToLower(m.moduleName) + "_anon_admin_db_error")
					_, _ = msg.Reply(c.Bot, errorText, formatting.Shtml())
					return ext.EndGroups
				}
				temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_anon_admin_disabled_now")
				text = fmt.Sprintf(temp, formatting.HtmlEscape(chat.Title))
			}
		default:
			text, _ = tr.GetString(strings.ToLower(m.moduleName) + "_anon_admin_invalid_arg")
		}
	}

	_, err := msg.Reply(c.Bot, text, formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func (moduleStruct) adminCache(c *helpers.CommandContext) error {
	b := c.Bot
	chat := c.Chat
	msg := c.Msg
	user := c.User
	if user == nil {
		return ext.EndGroups
	}

	var err error

	userMember, err := chat.GetMember(b, user.Id, nil)
	if err != nil {
		log.Errorf("[Admin] Failed to get member %d: %v", user.Id, err)
		errorText, _ := c.Tr.GetString("admin_check_status_failed")
		_, _ = msg.Reply(b, errorText, formatting.Shtml())
		return ext.EndGroups
	}
	mem := userMember.MergeChatMember()
	if mem.Status == "member" {
		errorText, _ := c.Tr.GetString("admin_need_admin")
		_, err = msg.Reply(b, errorText, nil)
		if err != nil {
			log.Error(err)
		}
		return ext.EndGroups
	}
	if !chat_status.RequireBotAdmin(b, c.Ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(c.Ctx, "chat_status_bot_not_admin", "", chat_status.WithReply())
		return ext.EndGroups
	}
	if !chat_status.RequireGroup(b, c.Ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(c.Ctx, "chat_status_group_only_error", "", chat_status.WithReply())
		return ext.EndGroups
	}

	cache.LoadAdminCache(b, chat.Id)

	k, _ := c.Tr.GetString("commonstrings_admin_cache_cache_reloaded")
	_, err = msg.Reply(b, k, formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

var (
	adminlistDesc = helpers.CommandDescriptor{
		Name:        "adminlist",
		Disableable: true,
		RequiredChecks: []helpers.CheckFunc{
			helpers.CheckDisabled("adminlist"),
			helpers.RequireBotAdmin(),
			helpers.RequireGroup(),
		},
	}
	promoteDesc = helpers.CommandDescriptor{
		Name: "promote",
		RequiredChecks: []helpers.CheckFunc{
			helpers.RequireGroup(),
			helpers.RequireBotAdmin(),
			helpers.RequireUserAdmin(),
			helpers.CanUserPromote(),
			helpers.CanBotPromote(),
		},
	}
	demoteDesc = helpers.CommandDescriptor{
		Name: "demote",
		RequiredChecks: []helpers.CheckFunc{
			helpers.RequireGroup(),
			helpers.RequireBotAdmin(),
			helpers.RequireUserAdmin(),
			helpers.CanUserPromote(),
			helpers.CanBotPromote(),
		},
	}
	setTitleDesc = helpers.CommandDescriptor{
		Name: "title",
		RequiredChecks: []helpers.CheckFunc{
			helpers.RequireGroup(),
			helpers.RequireUserAdmin(),
			helpers.RequireBotAdmin(),
			helpers.CanUserPromote(),
			helpers.CanBotPromote(),
		},
	}
	getinvitelinkDesc = helpers.CommandDescriptor{
		Name: "invitelink",
		RequiredChecks: []helpers.CheckFunc{
			helpers.RequireGroup(),
			helpers.RequireBotAdmin(),
			helpers.CanInvite(),
		},
	}
	clearAdminCacheDesc = helpers.CommandDescriptor{
		Name: "clearadmincache",
		RequiredChecks: []helpers.CheckFunc{
			helpers.RequireGroup(),
			helpers.RequireBotAdmin(),
			helpers.RequireUserAdmin(),
		},
	}
	anonAdminDesc = helpers.CommandDescriptor{
		Name: "anonadmin",
		RequiredChecks: []helpers.CheckFunc{
			helpers.RequireGroup(),
			helpers.RequireBotAdmin(),
		},
	}
	setGTitleDesc = helpers.CommandDescriptor{
		Name:    "setgtitle",
		Aliases: []string{"setgrouptitle"},
		RequiredChecks: []helpers.CheckFunc{
			helpers.RequireGroup(),
			helpers.RequireBotAdmin(),
			helpers.RequireUserAdmin(),
			helpers.CanUserChangeInfo(),
			helpers.CanBotChangeInfo(),
		},
	}
	setGPicDesc = helpers.CommandDescriptor{
		Name:    "setgpic",
		Aliases: []string{"setgrouppic"},
		RequiredChecks: []helpers.CheckFunc{
			helpers.RequireGroup(),
			helpers.RequireBotAdmin(),
			helpers.RequireUserAdmin(),
			helpers.CanUserChangeInfo(),
			helpers.CanBotChangeInfo(),
		},
	}
	setGDescDesc = helpers.CommandDescriptor{
		Name:    "setgdesc",
		Aliases: []string{"setgroupdesc", "setdescription"},
		RequiredChecks: []helpers.CheckFunc{
			helpers.RequireGroup(),
			helpers.RequireBotAdmin(),
			helpers.RequireUserAdmin(),
			helpers.CanUserChangeInfo(),
			helpers.CanBotChangeInfo(),
		},
	}
)

func LoadAdmin(dispatcher *ext.Dispatcher) {
	DefaultHelpRegistry().AbleMap["Admin"] = true

	helpers.WrapCommand(dispatcher, adminlistDesc, adminModule.adminlist)
	helpers.WrapCommand(dispatcher, promoteDesc, adminModule.promote)
	helpers.WrapCommand(dispatcher, demoteDesc, adminModule.demote)
	helpers.WrapCommand(dispatcher, setTitleDesc, adminModule.setTitle)
	helpers.WrapCommand(dispatcher, getinvitelinkDesc, adminModule.getinvitelink)
	helpers.WrapCommand(dispatcher, clearAdminCacheDesc, adminModule.clearAdminCache)
	helpers.WrapCommand(dispatcher, anonAdminDesc, adminModule.anonAdmin)
	helpers.WrapCommand(dispatcher, setGTitleDesc, adminModule.setGTitle)
	helpers.WrapCommand(dispatcher, setGPicDesc, adminModule.setGPic)
	helpers.WrapCommand(dispatcher, setGDescDesc, adminModule.setGDesc)

	dispatcher.AddHandler(handlers.NewCommand("admincache", func(b *gotgbot.Bot, ctx *ext.Context) error {
		defer error_handling.RecoverFromPanic("admincache", "admin")
		c, err := helpers.BuildCommandContext(b, ctx)
		if err != nil {
			return ext.EndGroups
		}
		return adminModule.adminCache(c)
	}))
}

func (moduleStruct) clearAdminCache(c *helpers.CommandContext) error {
	chat := c.Chat
	msg := c.Msg

	m := cache.GetMarshal()
	if m == nil {
		return ext.EndGroups
	}
	err := m.Delete(cache.Context, fmt.Sprintf("alita:adminCache:%d", chat.Id))
	if err != nil {
		log.Error(err)
		return err
	}
	log.Infof("[Admin] Cleared admin cache for %d (%s)", chat.Id, chat.Title)

	text, _ := c.Tr.GetString("admin_cache_cleared")
	_, err = msg.Reply(c.Bot, text, formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func (m moduleStruct) promoteAnonAdmin(b *gotgbot.Bot, ctx *ext.Context) error {
	c, err := helpers.BuildCommandContext(b, ctx)
	if err != nil {
		return ext.EndGroups
	}
	if !chat_status.CanUserPromote(b, ctx, nil, ctx.EffectiveUser.Id) {
		return ext.EndGroups
	}
	if !chat_status.CanBotPromote(b, ctx, nil) {
		return ext.EndGroups
	}
	return m.promote(c)
}

func (m moduleStruct) demoteAnonAdmin(b *gotgbot.Bot, ctx *ext.Context) error {
	c, err := helpers.BuildCommandContext(b, ctx)
	if err != nil {
		return ext.EndGroups
	}
	if !chat_status.CanUserPromote(b, ctx, nil, ctx.EffectiveUser.Id) {
		return ext.EndGroups
	}
	if !chat_status.CanBotPromote(b, ctx, nil) {
		return ext.EndGroups
	}
	return m.demote(c)
}

func (m moduleStruct) setTitleAnonAdmin(b *gotgbot.Bot, ctx *ext.Context) error {
	c, err := helpers.BuildCommandContext(b, ctx)
	if err != nil {
		return ext.EndGroups
	}
	if !chat_status.CanUserPromote(b, ctx, nil, ctx.EffectiveUser.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_promote_cmd_error", "chat_status_promote_button_error")
		return ext.EndGroups
	}
	if !chat_status.CanBotPromote(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_bot_promote_error", "")
		return ext.EndGroups
	}
	return m.setTitle(c)
}

func init() {
	RegisterLegacyModule("Admin", 30, LoadAdmin)
	RegisterAnonymousAdminHandler("promote", adminModule.promoteAnonAdmin)
	RegisterAnonymousAdminHandler("demote", adminModule.demoteAnonAdmin)
	RegisterAnonymousAdminHandler("title", adminModule.setTitleAnonAdmin)
}
