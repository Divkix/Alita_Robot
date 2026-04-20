package modules

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	log "github.com/sirupsen/logrus"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/extraction"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

var devsModule = moduleStruct{moduleName: "Dev"}

// chatInfo retrieves and displays detailed information about a specific chat.
// Only accessible by bot owner and dev users. Returns chat name, ID, member count, and invite link.
func (moduleStruct) chatInfo(b *gotgbot.Bot, ctx *ext.Context) error {
	user := chat_status.RequireUser(b, ctx, false)
	if user == nil {
		return ext.EndGroups
	}
	memStatus := db.GetTeamMemInfo(user.Id)

	// only devs and owner can access this
	if user.Id != config.AppConfig.OwnerId && !memStatus.IsDev {
		return ext.ContinueGroups
	}

	msg := ctx.EffectiveMessage
	var replyText string

	args := ctx.Args()

	if len(args) < 2 {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		replyText, _ = tr.GetString("devs_specify_user")
	} else {
		_chatId := args[1]
		chatId, _ := strconv.Atoi(_chatId)
		chat, err := b.GetChat(int64(chatId), nil)
		if err != nil {
			_, _ = msg.Reply(b, err.Error(), nil)
			return ext.EndGroups
		}
		// need to convert chat to group chat to use GetMemberCount
		_chat := chat.ToChat()
		gChat := &_chat
		con, _ := gChat.GetMemberCount(b, nil)
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		textTemplate, _ := tr.GetString("devs_chat_info")
		replyText = fmt.Sprintf(textTemplate, chat.Title, chat.Id, con, chat.InviteLink)
	}

	_, err := msg.Reply(b, replyText, helpers.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.ContinueGroups
}

// chatList generates and sends a document containing all active chats the bot is in.
// Only accessible by bot owner and dev users. Creates a temporary file with chat IDs and names.
func (moduleStruct) chatList(b *gotgbot.Bot, ctx *ext.Context) error {
	user := chat_status.RequireUser(b, ctx, false)
	if user == nil {
		return ext.EndGroups
	}
	memStatus := db.GetTeamMemInfo(user.Id)

	// only devs and owner can access this
	if user.Id != config.AppConfig.OwnerId && !memStatus.IsDev {
		return ext.ContinueGroups
	}

	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	text, _ := tr.GetString("devs_getting_chat_list")
	rMsg, err := msg.Reply(
		b,
		text,
		nil,
	)
	if err != nil {
		log.Error(err)
		return err
	}

	tmpFile, err := os.CreateTemp("", "chatlist-*.txt")
	if err != nil {
		log.Error(err)
		return err
	}
	defer func() { _ = tmpFile.Close() }()
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	allChats := db.GetAllChats()

	var sb strings.Builder
	for chatId, v := range allChats {
		if !v.IsInactive {
			fmt.Fprintf(&sb, "%d: %s\n", chatId, v.ChatName)
		}
	}

	_, err = tmpFile.WriteString(sb.String())
	if err != nil {
		log.Error(err)
		return err
	}

	openedFile, err := os.Open(tmpFile.Name())
	if err != nil {
		log.Error(err)
		return err
	}
	defer func() { _ = openedFile.Close() }()

	_, err = rMsg.Delete(b, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	_, err = b.SendDocument(
		chat.Id,
		gotgbot.InputFileByReader("chatlist.txt", openedFile),
		&gotgbot.SendDocumentOpts{
			Caption: func() string { caption, _ := tr.GetString("devs_chat_list_caption"); return caption }(),
			ReplyParameters: &gotgbot.ReplyParameters{
				MessageId:                msg.MessageId,
				AllowSendingWithoutReply: true,
			},
		},
	)
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// leaveChat makes the bot leave a specified chat.
// Only accessible by bot owner and dev users. Requires chat ID as argument.
func (moduleStruct) leaveChat(b *gotgbot.Bot, ctx *ext.Context) error {
	user := chat_status.RequireUser(b, ctx, false)
	if user == nil {
		return ext.EndGroups
	}
	memStatus := db.GetTeamMemInfo(user.Id)

	// only devs and owner can access this
	if user.Id != config.AppConfig.OwnerId && !memStatus.IsDev {
		return ext.ContinueGroups
	}

	msg := ctx.EffectiveMessage
	args := ctx.Args()

	if len(args) < 2 {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		replyText, _ := tr.GetString("devs_specify_user")
		_, err := msg.Reply(b, replyText, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.ContinueGroups
	}

	chatId, _ := strconv.ParseInt(args[1], 10, 64)

	_, err := b.LeaveChat(chatId, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	text, _ := tr.GetString("devs_left_chat")
	_, err = msg.Reply(b, text, helpers.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.ContinueGroups
}

/*
	Functions used to manage sudo/dev users in database of bot

Can only be used by OWNER
*/

// teamRoleConfig holds configuration for team role management operations.
type teamRoleConfig struct {
	roleName      string                     // "sudo" or "dev"
	add           bool                       // true for add, false for remove
	checkRole     func(*db.DevSettings) bool // checks if user has role
	alreadyMsgKey string                     // i18n key for "already has role"
	notRoleMsgKey string                     // i18n key for "doesn't have role"
	failMsgKey    string                     // i18n key for operation failure
	successMsgKey string                     // i18n key for operation success
	dbOp          func(int64) error          // database operation (AddSudo, RemDev, etc.)
}

// manageTeamRole handles adding/removing team roles (sudo/dev).
// Only accessible by bot owner.
func (m moduleStruct) manageTeamRole(b *gotgbot.Bot, ctx *ext.Context, cfg teamRoleConfig) error {
	user := chat_status.RequireUser(b, ctx, false)
	if user == nil {
		return ext.EndGroups
	}
	if user.Id != config.AppConfig.OwnerId {
		return ext.ContinueGroups
	}

	msg := ctx.EffectiveMessage
	userId := extraction.ExtractUser(b, ctx)
	if userId == -1 {
		return ext.ContinueGroups
	} else if chat_status.IsChannelId(userId) {
		return ext.ContinueGroups
	}

	reqUser, err := b.GetChat(userId, nil)
	if err != nil {
		log.Error(err)
		return err
	}
	memStatus := db.GetTeamMemInfo(userId)

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	var txt string

	// Check if operation is valid based on current role status
	hasRole := cfg.checkRole(memStatus)
	if cfg.add && hasRole {
		txt, _ = tr.GetString(cfg.alreadyMsgKey)
	} else if !cfg.add && !hasRole {
		txt, _ = tr.GetString(cfg.notRoleMsgKey)
	} else {
		if err := cfg.dbOp(userId); err != nil {
			log.Errorf("[Devs] Failed to %s %s for user %d: %v",
				map[bool]string{true: "add", false: "remove"}[cfg.add], cfg.roleName, userId, err)
			txt, _ = tr.GetString(cfg.failMsgKey)
		} else {
			textTemplate, _ := tr.GetString(cfg.successMsgKey)
			txt = fmt.Sprintf(textTemplate, helpers.MentionHtml(reqUser.Id, reqUser.FirstName))
		}
	}

	_, err = msg.Reply(b, txt, &gotgbot.SendMessageOpts{ParseMode: helpers.HTML})
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.ContinueGroups
}

// addSudo adds a user to the sudo users list in the bot's database.
// Only accessible by bot owner. Grants elevated permissions to the specified user.
func (m moduleStruct) addSudo(b *gotgbot.Bot, ctx *ext.Context) error {
	return m.manageTeamRole(b, ctx, teamRoleConfig{
		roleName:      "sudo",
		add:           true,
		checkRole:     func(tm *db.DevSettings) bool { return tm.Sudo },
		alreadyMsgKey: "devs_user_already_sudo",
		failMsgKey:    "devs_failed_to_add_sudo",
		successMsgKey: "devs_added_to_sudo",
		dbOp:          db.AddSudo,
	})
}

// addDev adds a user to the developer users list in the bot's database.
// Only accessible by bot owner. Grants developer-level permissions to the specified user.
func (m moduleStruct) addDev(b *gotgbot.Bot, ctx *ext.Context) error {
	return m.manageTeamRole(b, ctx, teamRoleConfig{
		roleName:      "dev",
		add:           true,
		checkRole:     func(tm *db.DevSettings) bool { return tm.IsDev },
		alreadyMsgKey: "devs_user_already_dev",
		failMsgKey:    "devs_failed_to_add_dev",
		successMsgKey: "devs_added_to_dev",
		dbOp:          db.AddDev,
	})
}

// remSudo removes a user from the sudo users list in the bot's database.
// Only accessible by bot owner. Revokes elevated permissions from the specified user.
func (m moduleStruct) remSudo(b *gotgbot.Bot, ctx *ext.Context) error {
	return m.manageTeamRole(b, ctx, teamRoleConfig{
		roleName:      "sudo",
		add:           false,
		checkRole:     func(tm *db.DevSettings) bool { return tm.Sudo },
		notRoleMsgKey: "devs_user_not_sudo",
		failMsgKey:    "devs_failed_to_remove_sudo",
		successMsgKey: "devs_removed_from_sudo",
		dbOp:          db.RemSudo,
	})
}

// remDev removes a user from the developer users list in the bot's database.
// Only accessible by bot owner. Revokes developer-level permissions from the specified user.
func (m moduleStruct) remDev(b *gotgbot.Bot, ctx *ext.Context) error {
	return m.manageTeamRole(b, ctx, teamRoleConfig{
		roleName:      "dev",
		add:           false,
		checkRole:     func(tm *db.DevSettings) bool { return tm.IsDev },
		notRoleMsgKey: "devs_user_not_dev",
		failMsgKey:    "devs_failed_to_remove_dev",
		successMsgKey: "devs_removed_from_dev",
		dbOp:          db.RemDev,
	})
}

/*
	Function used to list all members of bot's development team

Can only be used by existing team members
*/
// listTeam displays all current team members including developers and sudo users.
// Only accessible by existing team members. Shows user mentions organized by permission level.
func (moduleStruct) listTeam(b *gotgbot.Bot, ctx *ext.Context) error {
	user := chat_status.RequireUser(b, ctx, false)
	if user == nil {
		return ext.EndGroups
	}

	teamUsers := db.GetTeamMembers()
	var teamint64Slice []int64
	for k := range teamUsers {
		teamint64Slice = append(teamint64Slice, k)
	}
	teamint64Slice = append(teamint64Slice, config.AppConfig.OwnerId)

	if !slices.Contains(teamint64Slice, user.Id) {
		return ext.EndGroups
	}

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	devHeader, _ := tr.GetString("devs_dev_users_header")
	sudoHeader, _ := tr.GetString("devs_sudo_users_header")
	var (
		txt       string
		dev       = devHeader + "\n"
		sudo      = sudoHeader + "\n"
		sudoUsers = make([]string, 0, len(teamUsers))
		devUsers  = make([]string, 0, len(teamUsers))
	)
	msg := ctx.EffectiveMessage

	if len(teamUsers) == 0 {
		txt, _ = tr.GetString("devs_no_team_users")
	} else {
		for userId, uPerm := range teamUsers {
			reqUser, err := b.GetChat(userId, nil)
			if err != nil {
				log.Error(err)
				return err
			}

			userMentioned := helpers.MentionHtml(reqUser.Id, helpers.GetFullName(reqUser.FirstName, reqUser.LastName))
			switch uPerm {
			case "dev":
				devUsers = append(devUsers, fmt.Sprintf("• %s", userMentioned))
			case "sudo":
				sudoUsers = append(sudoUsers, fmt.Sprintf("• %s", userMentioned))
			}
		}
		noUsersText, _ := tr.GetString("devs_no_users")
		if len(sudoUsers) == 0 {
			sudo += "\n" + noUsersText
		} else {
			sudo += strings.Join(sudoUsers, "\n")
		}
		if len(devUsers) == 0 {
			dev += "\n" + noUsersText
		} else {
			dev += strings.Join(devUsers, "\n")
		}
		txt = dev + "\n\n" + sudo
	}

	_, err := msg.Reply(b, txt, &gotgbot.SendMessageOpts{ParseMode: helpers.HTML})
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

/*
	Function used to fetch stats of bot

Can only be used by OWNER
*/
// getStats retrieves and displays bot statistics including user counts, chat counts, and other metrics.
// Only accessible by bot owner and dev users. Shows comprehensive bot usage statistics.
func (moduleStruct) getStats(b *gotgbot.Bot, ctx *ext.Context) error {
	user := chat_status.RequireUser(b, ctx, false)
	if user == nil {
		return ext.EndGroups
	}
	memStatus := db.GetTeamMemInfo(user.Id)

	// only devs and owner can access this
	if user.Id != config.AppConfig.OwnerId && !memStatus.IsDev {
		return ext.ContinueGroups
	}

	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	text, _ := tr.GetString("devs_fetching_stats")
	edits, err := msg.Reply(
		b,
		text,
		&gotgbot.SendMessageOpts{
			ParseMode: helpers.HTML,
		},
	)
	if err != nil {
		log.Error(err)
		return err
	}

	stats := db.LoadAllStats()
	_, _, err = edits.EditText(
		b,
		stats,
		&gotgbot.EditMessageTextOpts{
			ParseMode: helpers.HTML,
		},
	)
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.ContinueGroups
}

// LoadDev registers all development-related command handlers with the dispatcher.
// Sets up admin commands for bot management, user management, and statistics.
func LoadDev(dispatcher *ext.Dispatcher) {
	dispatcher.AddHandler(handlers.NewCommand("stats", devsModule.getStats))
	dispatcher.AddHandler(handlers.NewCommand("addsudo", devsModule.addSudo))
	dispatcher.AddHandler(handlers.NewCommand("adddev", devsModule.addDev))
	dispatcher.AddHandler(handlers.NewCommand("remsudo", devsModule.remSudo))
	dispatcher.AddHandler(handlers.NewCommand("remdev", devsModule.remDev))
	dispatcher.AddHandler(handlers.NewCommand("teamusers", devsModule.listTeam))
	dispatcher.AddHandler(handlers.NewCommand("chatinfo", devsModule.chatInfo))
	dispatcher.AddHandler(handlers.NewCommand("chatlist", devsModule.chatList))
	dispatcher.AddHandler(handlers.NewCommand("leavechat", devsModule.leaveChat))
}
