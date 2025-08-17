package modules

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/debug_bot"
	"github.com/divkix/Alita_Robot/alita/utils/decorators/misc"
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
func (m moduleStruct) adminlist(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	cached := true

	// if command is disabled, return
	if chat_status.CheckDisabledCmd(b, msg, "adminlist") {
		return ext.EndGroups
	}

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	// permission checks
	if !chat_status.RequireBotAdmin(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireGroup(b, ctx, nil, false) {
		return ext.EndGroups
	}

	temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_adminlist")
	text := fmt.Sprintf(temp, chat.Title)

	adminsAvail, admins := cache.GetAdminCacheList(chat.Id)
	if !adminsAvail {
		admins = cache.LoadAdminCache(b, chat.Id)
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
			sb.WriteString(fmt.Sprintf("\n- @%s", user.Username))
		} else {
			sb.WriteString(fmt.Sprintf("\n- %s", helpers.MentionHtml(user.Id, user.FirstName)))
		}
	}
	text += sb.String()
	if !cached {
		noteText, _ := tr.GetString("admin_adminlist_note_fresh")
		text += noteText
	} else {
		noteText, _ := tr.GetString("admin_adminlist_note_cached")
		text += noteText
	}
	_, err := msg.Reply(b, text, helpers.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

/* Used to Demote a member in chat

connection = true, true

Bot can only Demote people it promoted! */

// demote handles the /demote command to remove admin privileges from a user.
// The bot can only demote users it has previously promoted.
func (m moduleStruct) demote(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	user := ctx.EffectiveSender.User
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	// permission checks
	if !chat_status.RequireGroup(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireBotAdmin(b, ctx, nil, false) {
		return ext.EndGroups
	}

	// Validate admin cache before proceeding
	adminsAvail, admins := cache.GetAdminCacheList(chat.Id)
	if !adminsAvail {
		admins = cache.LoadAdminCache(b, chat.Id)
	}

	// If we still can't get admin list, inform user and abort
	if len(admins.UserInfo) == 0 {
		text, translationErr := tr.GetString(strings.ToLower(m.moduleName) + "_errors_admin_cache_failed")
		if translationErr != nil || text == "" {
			text = "I'm having trouble accessing the admin list. Please try again later."
		}
		_, err := msg.Reply(b, text, nil)
		if err != nil {
			log.Error(err)
		}
		return ext.EndGroups
	}

	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}
	if !chat_status.CanUserPromote(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}
	if !chat_status.CanBotPromote(b, ctx, nil, false) {
		return ext.EndGroups
	}

	userId := extraction.ExtractUser(b, ctx)
	if userId == -1 {
		return ext.EndGroups
	} else if helpers.IsChannelID(userId) {
		text, _ := tr.GetString("admin_anonymous_user_error")
		_, err := msg.Reply(b, text, nil)
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	} else if userId == 0 {
		text, _ := tr.GetString("admin_no_user_specified")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if chat_status.RequireUserOwner(b, ctx, nil, userId, true) {
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_demote_is_owner")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}

		return ext.EndGroups
	}
	if userId == b.Id {
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_demote_is_bot_itself")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}

		return ext.EndGroups
	}
	if !chat_status.IsUserAdmin(b, chat.Id, userId) {
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_demote_is_admin")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}

		return ext.EndGroups
	}

	bb, err := chat.PromoteMember(b,
		userId,
		&gotgbot.PromoteChatMemberOpts{
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
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_errors_err_cannot_demote")
		_, err = msg.Reply(b, text, nil)
		if err != nil {
			log.Error(err)
			return err
		}

		return ext.EndGroups
	}

	userMember, err := chat.GetMember(b, userId, nil)
	if err != nil {
		log.Error(err)
		return err
	}
	mem := userMember.MergeChatMember().User
	_, err = msg.Reply(b,
		func() string {
			temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_demote_success_demote")
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

/* Used to Promote a member in chat

connection = true, true

Bot will give promoted user permissions of bot*/

// promote handles the /promote command to grant admin privileges to a user.
// The bot grants permissions based on its own capabilities and the promoter's status.
func (m moduleStruct) promote(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	user := ctx.EffectiveSender.User
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	extraText := ""

	// permission checks
	if !chat_status.RequireGroup(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireBotAdmin(b, ctx, nil, false) {
		return ext.EndGroups
	}

	// Validate admin cache before proceeding
	adminsAvail, admins := cache.GetAdminCacheList(chat.Id)
	if !adminsAvail {
		admins = cache.LoadAdminCache(b, chat.Id)
	}

	// If we still can't get admin list, inform user and abort
	if len(admins.UserInfo) == 0 {
		text, translationErr := tr.GetString(strings.ToLower(m.moduleName) + "_errors_admin_cache_failed")
		if translationErr != nil || text == "" {
			text = "I'm having trouble accessing the admin list. Please try again later."
		}
		_, err := msg.Reply(b, text, nil)
		if err != nil {
			log.Error(err)
		}
		return ext.EndGroups
	}

	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}
	if !chat_status.CanUserPromote(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}
	if !chat_status.CanBotPromote(b, ctx, nil, false) {
		return ext.EndGroups
	}

	userId, customTitle := extraction.ExtractUserAndText(b, ctx)
	if userId == -1 {
		return ext.EndGroups
	} else if helpers.IsChannelID(userId) {
		text, _ := tr.GetString("admin_anonymous_user_error")
		_, err := msg.Reply(b, text, nil)
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	} else if userId == 0 {
		text, _ := tr.GetString("admin_no_user_specified")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if userId == b.Id {
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_promote_is_bot_itself")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}

		return ext.EndGroups
	}

	// checks if user being promoted is already admin or owner
	if chat_status.RequireUserOwner(b, ctx, nil, userId, true) {
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_promote_is_owner")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}

		return ext.EndGroups
	}
	if chat_status.IsUserAdmin(b, chat.Id, userId) {
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_promote_is_admin")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}

		return ext.EndGroups
	}

	userMember, err := chat.GetMember(b, userId, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	botMember, err := chat.GetMember(b, b.Id, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	// makes code short
	bMem := botMember.MergeChatMember()
	pMem := userMember.MergeChatMember()

	teamMem := db.GetTeamMemInfo(user.Id)
	teamMemInfo := teamMem.Sudo || teamMem.Dev
	isPromoterOwner := chat_status.RequireUserOwner(b, ctx, nil, user.Id, true)

	checkCommonPerms := isPromoterOwner || teamMemInfo

	_, err = chat.PromoteMember(b,
		userId,
		&gotgbot.PromoteChatMemberOpts{
			CanPostMessages:     bMem.CanPostMessages && (pMem.CanPostMessages || checkCommonPerms),
			CanDeleteMessages:   bMem.CanDeleteMessages && (pMem.CanDeleteMessages || checkCommonPerms),
			CanRestrictMembers:  bMem.CanRestrictMembers && (pMem.CanRestrictMembers || checkCommonPerms),
			CanChangeInfo:       bMem.CanChangeInfo && (pMem.CanChangeInfo || checkCommonPerms),
			CanInviteUsers:      bMem.CanInviteUsers && (pMem.CanInviteUsers || checkCommonPerms),
			CanPinMessages:      bMem.CanPinMessages && (pMem.CanPinMessages || checkCommonPerms),
			CanManageVideoChats: bMem.CanManageVideoChats && (pMem.CanManageVideoChats || checkCommonPerms),
			CanManageChat:       bMem.CanManageChat && (pMem.CanManageChat || checkCommonPerms),
			CanManageTopics:     bMem.CanManageTopics && (pMem.CanManageTopics || checkCommonPerms),
		},
	)
	if err != nil {
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_errors_err_cannot_promote")
		_, _ = msg.Reply(b, text, helpers.Shtml())
		return err
	}

	if len(customTitle) > 16 {
		// trim title to 16 characters (telegram restriction)
		temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_promote_admin_title_truncated")
		extraText += fmt.Sprintf(temp, len(customTitle))
		customTitle = customTitle[0:16]
	}

	// set the custom title
	if customTitle != "" {
		_, err = chat.SetAdministratorCustomTitle(
			b,
			userId,
			customTitle,
			nil,
		)
		if err != nil {
			text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_errors_err_set_title")
			_, err = msg.Reply(b, text, nil)
			if err != nil {
				log.Error(err)
			}
			return ext.EndGroups
		}
	}
	mem := userMember.MergeChatMember().User
	_, err = msg.Reply(b,
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

// getinvitelink handles the /invitelink command to retrieve the chat's invite link.
// Returns either the public username or generates an invite link for private groups.
func (moduleStruct) getinvitelink(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	// permission checks
	if !chat_status.RequireGroup(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireBotAdmin(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.Caninvite(b, ctx, nil, msg, false) {
		return ext.EndGroups
	}
	if chat.Username != "" {
		linkText, _ := tr.GetString("admin_invitelink_public")
		_, _ = msg.Reply(b, fmt.Sprintf(linkText, chat.Username), nil)
	} else {
		nchat, err := b.GetChat(chat.Id, nil)
		if err != nil {
			_, _ = msg.Reply(b, err.Error(), nil)
			return ext.EndGroups
		}
		linkText, _ := tr.GetString("admin_invitelink_private")
		_, _ = msg.Reply(b, fmt.Sprintf(linkText, nchat.InviteLink), nil)
	}
	return ext.EndGroups
}

/*
Sets a custom title for an admin.
Only works with admins whom bot has promoted.*/

// setTitle handles the /title command to set a custom administrator title.
// Only works with admins that the bot has promoted and titles are limited to 16 characters.
func (m moduleStruct) setTitle(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	user := ctx.EffectiveSender.User
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	// permission checks
	if !chat_status.RequireGroup(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireBotAdmin(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.CanUserPromote(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}
	if !chat_status.CanBotPromote(b, ctx, nil, false) {
		return ext.EndGroups
	}

	userId, customTitle := extraction.ExtractUserAndText(b, ctx)
	if userId == -1 {
		return ext.EndGroups
	} else if helpers.IsChannelID(userId) {
		text, _ := tr.GetString("admin_anonymous_user_error")
		_, err := msg.Reply(b, text, nil)
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	} else if userId == 0 {
		text, _ := tr.GetString("admin_no_user_specified")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if chat_status.RequireUserOwner(b, ctx, nil, userId, true) {
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_title_is_owner")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}
	if !chat_status.IsUserAdmin(b, chat.Id, userId) {
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_title_is_admin")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if userId == b.Id {
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_title_is_bot_itself")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}

		return ext.EndGroups
	}

	// for managing custom title
	if customTitle == "" {
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_errors_title_empty")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	} else if len(customTitle) > 16 {
		// trim title to 16 characters (telegram restriction)
		customTitle = customTitle[0:16]
	}

	_, err := chat.SetAdministratorCustomTitle(b,
		userId,
		customTitle,
		nil,
	)
	if err != nil {
		log.Error(err)
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_errors_err_set_title")
		_, _ = msg.Reply(b, text, helpers.Shtml())
		return err
	}

	userMember, err := chat.GetMember(b, userId, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	mem := userMember.MergeChatMember()

	_, err = msg.Reply(b,
		func() string {
			temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_title_success_set")
			return fmt.Sprintf(temp, mem.User.FirstName, mem.CustomTitle)
		}(),
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
func (m moduleStruct) anonAdmin(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	user := ctx.EffectiveSender.User
	args := ctx.Args()

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	var text string

	// permission checks
	if !chat_status.RequireGroup(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireBotAdmin(b, ctx, nil, false) {
		return ext.EndGroups
	}

	adminSettings := db.GetAdminSettings(chat.Id)

	if len(args) == 1 {
		if adminSettings.AnonAdmin {
			temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_anon_admin_enabled")
			text = fmt.Sprintf(temp, chat.Title)
		} else {
			temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_anon_admin_disabled")
			text = fmt.Sprintf(temp, chat.Title)
		}
	} else {
		// only need owner if you want to change value
		if !chat_status.RequireUserOwner(b, ctx, nil, user.Id, false) {
			return ext.EndGroups
		}
		switch args[1] {
		case "on", "true", "yes":
			if adminSettings.AnonAdmin {
				temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_anon_admin_already_enabled")
				text = fmt.Sprintf(temp, chat.Title)
			} else {
				go db.SetAnonAdminMode(chat.Id, true)
				temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_anon_admin_enabled_now")
				text = fmt.Sprintf(temp, chat.Title)
			}
		case "off", "no", "false":
			if !adminSettings.AnonAdmin {
				temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_anon_admin_already_disabled")
				text = fmt.Sprintf(temp, chat.Title)
			} else {
				go db.SetAnonAdminMode(chat.Id, false)
				temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_anon_admin_disabled_now")
				text = fmt.Sprintf(temp, chat.Title)
			}
		default:
			text, _ = tr.GetString(strings.ToLower(m.moduleName) + "_anon_admin_invalid_arg")
		}
	}

	_, err := msg.Reply(b, text, helpers.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// adminCache handles the /admincache command to refresh the admin cache for a chat.
// Forces a reload of admin permissions from Telegram's API.
func (moduleStruct) adminCache(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	user := ctx.EffectiveSender.User

	var err error

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	debug_bot.PrettyPrintStruct(tr)

	// permission checks
	userMember, _ := chat.GetMember(b, user.Id, nil)
	mem := userMember.MergeChatMember()
	if mem.Status == "member" {
		errorText, _ := tr.GetString("admin_need_admin")
		_, err = msg.Reply(b, errorText, nil)
		if err != nil {
			log.Error(err)
		}
		return ext.EndGroups
	}
	if !chat_status.RequireBotAdmin(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireGroup(b, ctx, nil, false) {
		return ext.EndGroups
	}

	cache.LoadAdminCache(b, chat.Id)

	k, _ := tr.GetString("commonstrings_admin_cache_cache_reloaded")
	debug_bot.PrettyPrintStruct(k)
	_, err = msg.Reply(b, k, helpers.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// LoadAdmin registers all admin module command handlers with the dispatcher.
// Sets up commands for promotion, demotion, title setting, and admin management.
func LoadAdmin(dispatcher *ext.Dispatcher) {
	HelpModule.AbleMap.Store("Admin", true)

	dispatcher.AddHandler(handlers.NewCommand("promote", adminModule.promote))
	dispatcher.AddHandler(handlers.NewCommand("demote", adminModule.demote))
	dispatcher.AddHandler(handlers.NewCommand("invitelink", adminModule.getinvitelink))
	dispatcher.AddHandler(handlers.NewCommand("title", adminModule.setTitle))
	dispatcher.AddHandler(handlers.NewCommand("adminlist", adminModule.adminlist))
	misc.AddCmdToDisableable("adminlist")
	dispatcher.AddHandler(handlers.NewCommand("anonadmin", adminModule.anonAdmin))
	dispatcher.AddHandler(handlers.NewCommand("admincache", adminModule.adminCache))
	dispatcher.AddHandler(
		handlers.NewCommand(
			"clearadmincache",
			func(b *gotgbot.Bot, ctx *ext.Context) error {
				chat := ctx.EffectiveChat
				err := cache.Marshal.Delete(cache.Context, fmt.Sprintf("alita:adminCache:%d", chat.Id))
				if err != nil {
					log.Error(err)
					return err
				}
				log.Info(fmt.Sprintf("Cleared admin cache for %d (%s)", chat.Id, chat.Title))
				return ext.EndGroups
			},
		),
	)
}
