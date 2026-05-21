package modules

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"

	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/extraction"
)

var adminModule = moduleStruct{moduleName: "Admin"}

/*
	Used to list all the admin in a group

Connection - false, false
*/
// adminlist handles the /adminlist command to display all admins in a group.
// It returns a cached or fresh list of group administrators excluding bots and anonymous admins.
func (m moduleStruct) adminlist(c *helpers.CommandContext) error {
	chat := c.Chat
	msg := c.Msg
	cached := true

	tr := c.Tr

	temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_adminlist")
	text := fmt.Sprintf(temp, helpers.HtmlEscape(chat.Title))

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
			// don't list bots and anonymous admins
			continue
		}
		if user.Username != "" {
			fmt.Fprintf(&sb, "\n- @%s", helpers.HtmlEscape(user.Username))
		} else {
			fmt.Fprintf(&sb, "\n- %s", helpers.MentionHtml(user.Id, user.FirstName))
		}
	}
	if sb.Len() == 0 {
		// All admins are bots or anonymous
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
	_, err := msg.Reply(c.Bot, text, helpers.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

/* Used to Demote a member in chat

connection = true, true

Bot can only Demote people it promoted! */

// loadAdminCacheOrFail returns the cached admin list, or loads it from
// Telegram. If the cache is empty after loading, it replies with an error
// and returns a nil pointer so the caller can early-return.
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

// validateDemotionTarget checks the extracted user ID for demotion.
// It returns the validated user ID and an error sentinel if the target is
// invalid (the caller should return ext.EndGroups).
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
		_, err := c.Msg.Reply(c.Bot, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return 0, err
		}
		return 0, ext.EndGroups
	}

	if chat_status.RequireUserOwner(c.Bot, c.Ctx, nil, userId, true) {
		text, _ := c.Tr.GetString(strings.ToLower(m.moduleName) + "_demote_is_owner")
		_, err := c.Msg.Reply(c.Bot, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return 0, err
		}
		return 0, ext.EndGroups
	}
	if userId == c.Bot.Id {
		text, _ := c.Tr.GetString(strings.ToLower(m.moduleName) + "_demote_is_bot_itself")
		_, err := c.Msg.Reply(c.Bot, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return 0, err
		}
		return 0, ext.EndGroups
	}
	// Using IsUserAdmin (not RequireUserAdmin) because we need a custom error message
	// specific to the demote context rather than the generic permission error.
	if !chat_status.IsUserAdmin(c.Bot, c.Chat.Id, userId) {
		text, _ := c.Tr.GetString(strings.ToLower(m.moduleName) + "_demote_not_admin")
		_, err := c.Msg.Reply(c.Bot, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return 0, err
		}
		return 0, ext.EndGroups
	}

	return userId, nil
}

// performDemotion removes admin privileges from the target user and sends
// a success confirmation.
func (m moduleStruct) performDemotion(c *helpers.CommandContext, userId int64) error {
	bb, err := c.Chat.PromoteMember(c.Bot,
		userId,
		&gotgbot.PromoteChatMemberOpts{
			CanPostMessages:     false,
			CanDeleteMessages:   false,
			CanRestrictMembers:  false,
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
			return fmt.Sprintf(temp, helpers.MentionHtml(mem.Id, mem.FirstName))
		}(),
		helpers.Shtml(),
	)
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}

// demote handles the /demote command to remove admin privileges from a user.
// The bot can only demote users it has previously promoted.
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

/* Used to Promote a member in chat

connection = true, true

Bot will give promoted user permissions of bot*/

// validatePromotionTarget extracts and validates the target user for promotion.
// It returns the user ID, custom title, and an error sentinel (ext.EndGroups) on failure.
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
		_, err := c.Msg.Reply(c.Bot, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return 0, "", err
		}
		return 0, "", ext.EndGroups
	}
	if userId == c.Bot.Id {
		text, _ := c.Tr.GetString(strings.ToLower(m.moduleName) + "_promote_is_bot_itself")
		_, err := c.Msg.Reply(c.Bot, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return 0, "", err
		}
		return 0, "", ext.EndGroups
	}
	if chat_status.RequireUserOwner(c.Bot, c.Ctx, nil, userId, true) {
		text, _ := c.Tr.GetString(strings.ToLower(m.moduleName) + "_promote_is_owner")
		_, err := c.Msg.Reply(c.Bot, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return 0, "", err
		}
		return 0, "", ext.EndGroups
	}
	if chat_status.IsUserAdmin(c.Bot, c.Chat.Id, userId) {
		text, _ := c.Tr.GetString(strings.ToLower(m.moduleName) + "_promote_is_admin")
		_, err := c.Msg.Reply(c.Bot, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return 0, "", err
		}
		return 0, "", ext.EndGroups
	}
	return userId, customTitle, nil
}

// canGrantPerm returns whether the bot can grant a specific permission,
// considering both the bot's own capability and the promoter privileges.
func canGrantPerm(botHas, promoterHas, bypass bool) bool {
	return botHas && (promoterHas || bypass)
}

// buildPromoteOpts constructs the permission options for PromoteMember
// based on bot and promoter capabilities.
func buildPromoteOpts(botMember, promoterMember gotgbot.ChatMember, user *gotgbot.User, c *helpers.CommandContext) *gotgbot.PromoteChatMemberOpts {
	bMem := botMember.MergeChatMember()
	pMem := promoterMember.MergeChatMember()

	teamMem := db.GetTeamMemInfo(user.Id)
	teamMemInfo := teamMem.Sudo || teamMem.IsDev
	isPromoterOwner := chat_status.RequireUserOwner(c.Bot, c.Ctx, nil, user.Id, true)
	checkCommonPerms := isPromoterOwner || teamMemInfo

	return &gotgbot.PromoteChatMemberOpts{
		CanPostMessages:     canGrantPerm(bMem.CanPostMessages, pMem.CanPostMessages, checkCommonPerms),
		CanDeleteMessages:   canGrantPerm(bMem.CanDeleteMessages, pMem.CanDeleteMessages, checkCommonPerms),
		CanRestrictMembers:  canGrantPerm(bMem.CanRestrictMembers, pMem.CanRestrictMembers, checkCommonPerms),
		CanChangeInfo:       canGrantPerm(bMem.CanChangeInfo, pMem.CanChangeInfo, checkCommonPerms),
		CanInviteUsers:      canGrantPerm(bMem.CanInviteUsers, pMem.CanInviteUsers, checkCommonPerms),
		CanPinMessages:      canGrantPerm(bMem.CanPinMessages, pMem.CanPinMessages, checkCommonPerms),
		CanManageVideoChats: canGrantPerm(bMem.CanManageVideoChats, pMem.CanManageVideoChats, checkCommonPerms),
		CanManageChat:       canGrantPerm(bMem.CanManageChat, pMem.CanManageChat, checkCommonPerms),
		CanManageTopics:     canGrantPerm(bMem.CanManageTopics, pMem.CanManageTopics, checkCommonPerms),
	}
}

// handlePromotionSuccess sends the success reply after a promotion,
// optionally setting a custom title and truncating it if needed.
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
			return fmt.Sprintf(temp, helpers.MentionHtml(mem.Id, mem.FirstName))
		}()+extraText,
		helpers.Shtml(),
	)
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}

// promote handles the /promote command to grant admin privileges to a user.
// The bot grants permissions based on its own capabilities and the promoter's status.
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

	promoterMember, err := c.Chat.GetMember(c.Bot, c.User.Id, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	botMember, err := c.Chat.GetMember(c.Bot, c.Bot.Id, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	opts := buildPromoteOpts(botMember, promoterMember, c.User, c)

	status, err := c.Chat.PromoteMember(c.Bot, userId, opts)
	if err != nil || !status {
		text, _ := c.Tr.GetString(strings.ToLower(m.moduleName) + "_errors_err_cannot_promote")
		_, _ = c.Msg.Reply(c.Bot, text, helpers.Shtml())
		if err == nil {
			err = fmt.Errorf("promote member returned false status")
		}
		return err
	}

	return m.handlePromotionSuccess(c, userId, customTitle, userMember)
}

// getinvitelink handles the /invitelink command to retrieve the chat's invite link.
// Returns either the public username or generates an invite link for private groups.
func (moduleStruct) getinvitelink(c *helpers.CommandContext) error {
	chat := c.Chat
	msg := c.Msg
	tr := c.Tr

	if chat.Username != "" {
		linkText, _ := tr.GetString("admin_invitelink_public")
		_, _ = msg.Reply(c.Bot, fmt.Sprintf(linkText, helpers.HtmlEscape(chat.Username)), nil)
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

/*
Sets a custom title for an admin.
Only works with admins whom bot has promoted.*/

// setTitle handles the /title command to set a custom administrator title.
// Only works with admins that the bot has promoted and titles are limited to 16 characters.
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
		_, err := msg.Reply(c.Bot, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if chat_status.RequireUserOwner(c.Bot, c.Ctx, nil, userId, true) {
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_title_is_owner")
		_, err := msg.Reply(c.Bot, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}
	if !chat_status.IsUserAdmin(c.Bot, chat.Id, userId) {
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_title_is_admin")
		_, err := msg.Reply(c.Bot, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if userId == c.Bot.Id {
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_title_is_bot_itself")
		_, err := msg.Reply(c.Bot, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}

		return ext.EndGroups
	}

	// for managing custom title
	var extraText string
	if customTitle == "" {
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_errors_title_empty")
		_, err := msg.Reply(c.Bot, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	} else if len(customTitle) > 16 {
		// trim title to 16 characters (telegram restriction) and notify user
		temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_title_truncated")
		extraText = fmt.Sprintf(temp, len(customTitle))
		customTitle = customTitle[0:16]
	}

	_, err := chat.SetAdministratorCustomTitle(c.Bot,
		userId,
		customTitle,
		nil,
	)
	if err != nil {
		log.Error(err)
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_errors_err_set_title")
		_, _ = msg.Reply(c.Bot, text, helpers.Shtml())
		return err
	}

	userMember, err := chat.GetMember(c.Bot, userId, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	mem := userMember.MergeChatMember()

	_, err = msg.Reply(c.Bot,
		func() string {
			temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_title_success_set")
			return fmt.Sprintf(temp, mem.User.FirstName, mem.CustomTitle)
		}()+extraText,
		helpers.Shtml(),
	)
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// anonAdmin handles the /anonadmin command to toggle anonymous admin mode in groups.
// Only chat owners can modify this setting which affects how anonymous admins are handled.
func (m moduleStruct) anonAdmin(c *helpers.CommandContext) error {
	chat := c.Chat
	msg := c.Msg
	user := c.User
	args := c.Ctx.Args()

	tr := c.Tr
	var text string

	adminSettings := db.GetAdminSettings(chat.Id)

	if len(args) == 1 {
		if adminSettings.AnonAdmin {
			temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_anon_admin_enabled")
			text = fmt.Sprintf(temp, helpers.HtmlEscape(chat.Title))
		} else {
			temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_anon_admin_disabled")
			text = fmt.Sprintf(temp, helpers.HtmlEscape(chat.Title))
		}
	} else {
		// only need owner if you want to change value
		if !chat_status.RequireUserOwner(c.Bot, c.Ctx, nil, user.Id, false) {
			return ext.EndGroups
		}
		switch args[1] {
		case "on", "true", "yes":
			if adminSettings.AnonAdmin {
				temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_anon_admin_already_enabled")
				text = fmt.Sprintf(temp, helpers.HtmlEscape(chat.Title))
			} else {
				// Synchronous DB write - confirm success before sending message
				if err := db.SetAnonAdminMode(chat.Id, true); err != nil {
					log.Errorf("[Admin] Failed to set anon admin mode for chat %d: %v", chat.Id, err)
					errorText, _ := tr.GetString(strings.ToLower(m.moduleName) + "_anon_admin_db_error")
					_, _ = msg.Reply(c.Bot, errorText, helpers.Shtml())
					return ext.EndGroups
				}
				temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_anon_admin_enabled_now")
				text = fmt.Sprintf(temp, helpers.HtmlEscape(chat.Title))
			}
		case "off", "no", "false":
			if !adminSettings.AnonAdmin {
				temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_anon_admin_already_disabled")
				text = fmt.Sprintf(temp, helpers.HtmlEscape(chat.Title))
			} else {
				// Synchronous DB write - confirm success before sending message
				if err := db.SetAnonAdminMode(chat.Id, false); err != nil {
					log.Errorf("[Admin] Failed to set anon admin mode for chat %d: %v", chat.Id, err)
					errorText, _ := tr.GetString(strings.ToLower(m.moduleName) + "_anon_admin_db_error")
					_, _ = msg.Reply(c.Bot, errorText, helpers.Shtml())
					return ext.EndGroups
				}
				temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_anon_admin_disabled_now")
				text = fmt.Sprintf(temp, helpers.HtmlEscape(chat.Title))
			}
		default:
			text, _ = tr.GetString(strings.ToLower(m.moduleName) + "_anon_admin_invalid_arg")
		}
	}

	_, err := msg.Reply(c.Bot, text, helpers.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// adminCache handles the /admincache command to refresh the admin cache for a chat.
// Forces a reload of admin permissions from Telegram's API.
func (moduleStruct) adminCache(c *helpers.CommandContext) error {
	b := c.Bot
	chat := c.Chat
	msg := c.Msg
	user := c.User
	if user == nil {
		return ext.EndGroups
	}

	var err error

	// permission checks
	userMember, err := chat.GetMember(b, user.Id, nil)
	if err != nil {
		log.Errorf("[Admin] Failed to get member %d: %v", user.Id, err)
		errorText, _ := c.Tr.GetString("admin_check_status_failed")
		_, _ = msg.Reply(b, errorText, helpers.Shtml())
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
	if !chat_status.RequireBotAdmin(b, c.Ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireGroup(b, c.Ctx, nil, false) {
		return ext.EndGroups
	}

	cache.LoadAdminCache(b, chat.Id)

	k, _ := c.Tr.GetString("commonstrings_admin_cache_cache_reloaded")
	_, err = msg.Reply(b, k, helpers.Shtml())
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
)

// LoadAdmin registers all admin module command handlers with the dispatcher.
// Sets up commands for promotion, demotion, title setting, and admin management.
func LoadAdmin(dispatcher *ext.Dispatcher) {
	DefaultHelpRegistry().AbleMap.Store("Admin", true)

	helpers.WrapCommand(dispatcher, adminlistDesc, adminModule.adminlist)
	helpers.WrapCommand(dispatcher, promoteDesc, adminModule.promote)
	helpers.WrapCommand(dispatcher, demoteDesc, adminModule.demote)
	helpers.WrapCommand(dispatcher, setTitleDesc, adminModule.setTitle)
	helpers.WrapCommand(dispatcher, getinvitelinkDesc, adminModule.getinvitelink)
	helpers.WrapCommand(dispatcher, clearAdminCacheDesc, adminModule.clearAdminCache)
	helpers.WrapCommand(dispatcher, anonAdminDesc, adminModule.anonAdmin)

	// adminCache uses custom permission checking (direct member status lookup),
	// so it remains a raw handler.
	dispatcher.AddHandler(handlers.NewCommand("admincache", func(b *gotgbot.Bot, ctx *ext.Context) error {
		c, err := helpers.BuildCommandContext(b, ctx)
		if err != nil {
			return ext.EndGroups
		}
		return adminModule.adminCache(c)
	}))
}

// clearAdminCache handles the /clearadmincache command to delete the cached admin list.
// Requires admin permissions and provides user feedback on success.
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
	_, err = msg.Reply(c.Bot, text, helpers.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func init() {
	RegisterLegacyModule("Admin", 30, LoadAdmin)
}
