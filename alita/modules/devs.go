package modules

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	log "github.com/sirupsen/logrus"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/extraction"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"

	"github.com/divkix/Alita_Robot/alita/utils/string_handling"
)

var devsModule = moduleStruct{moduleName: "Dev"}

// for general purposes for strings in functions below
var txt string

// chatInfo retrieves and displays detailed information about a specific chat.
// Only accessible by bot owner and dev users. Returns chat name, ID, member count, and invite link.
func (moduleStruct) chatInfo(b *gotgbot.Bot, ctx *ext.Context) error {
	user := ctx.EffectiveSender.User
	memStatus := db.GetTeamMemInfo(user.Id)

	// only devs and owner can access this
	if user.Id != config.AppConfig.OwnerId && !memStatus.Dev {
		return ext.ContinueGroups
	}

	msg := ctx.EffectiveMessage
	var replyText string

	args := ctx.Args()

	if len(args) == 0 {
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
	user := ctx.EffectiveSender.User
	memStatus := db.GetTeamMemInfo(user.Id)

	// only devs and owner can access this
	if user.Id != config.AppConfig.OwnerId && !memStatus.Dev {
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

	var writeString string
	fileName := "chatlist.txt"

	allChats := db.GetAllChats()

	var sb strings.Builder
	for chatId, v := range allChats {
		if !v.IsInactive {
			sb.WriteString(fmt.Sprintf("%d: %s\n", chatId, v.ChatName))
		}
	}
	writeString += sb.String()

	// If the file doesn't exist, create it or re-write it
	err = os.WriteFile(fileName, []byte(writeString), 0o600)
	if err != nil {
		log.Error(err)
		return err
	}

	openedFile, _ := os.Open(fileName)

	_, err = rMsg.Delete(b, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	_, err = b.SendDocument(
		chat.Id,
		gotgbot.InputFileByReader(fileName, openedFile),
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

	err = openedFile.Close()
	if err != nil {
		log.Error(err)
	}
	err = os.Remove(fileName)
	if err != nil {
		log.Error(err)
	}

	return ext.EndGroups
}

// leaveChat makes the bot leave a specified chat.
// Only accessible by bot owner and dev users. Requires chat ID as argument.
func (moduleStruct) leaveChat(b *gotgbot.Bot, ctx *ext.Context) error {
	user := ctx.EffectiveSender.User
	memStatus := db.GetTeamMemInfo(user.Id)

	// only devs and owner can access this
	if user.Id != config.AppConfig.OwnerId && !memStatus.Dev {
		return ext.ContinueGroups
	}

	msg := ctx.EffectiveMessage
	args := ctx.Args()
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
	Function used to add sudo users in database of bot

Can only be used by OWNER
*/
// addSudo adds a user to the sudo users list in the bot's database.
// Only accessible by bot owner. Grants elevated permissions to the specified user.
func (moduleStruct) addSudo(b *gotgbot.Bot, ctx *ext.Context) error {
	user := ctx.EffectiveSender.User
	if user.Id != config.AppConfig.OwnerId {
		return ext.ContinueGroups
	}

	msg := ctx.EffectiveMessage
	userId := extraction.ExtractUser(b, ctx)
	if userId == -1 {
		return ext.ContinueGroups
	} else if helpers.IsChannelID(userId) {
		return ext.ContinueGroups
	}

	reqUser, err := b.GetChat(userId, nil)
	if err != nil {
		log.Error(err)
		return err
	}
	memStatus := db.GetTeamMemInfo(userId)

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	if memStatus.Sudo {
		txt, _ = tr.GetString("devs_user_already_sudo")
	} else {
		textTemplate, _ := tr.GetString("devs_added_to_sudo")
		txt = fmt.Sprintf(textTemplate, helpers.MentionHtml(reqUser.Id, reqUser.FirstName))
		go db.AddDev(userId)
	}
	_, err = msg.Reply(b, txt, &gotgbot.SendMessageOpts{ParseMode: helpers.HTML})
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.ContinueGroups
}

/*
	Function used to add dev users in database of bot

Can only be used by OWNER
*/
// addDev adds a user to the developer users list in the bot's database.
// Only accessible by bot owner. Grants developer-level permissions to the specified user.
func (moduleStruct) addDev(b *gotgbot.Bot, ctx *ext.Context) error {
	user := ctx.EffectiveSender.User
	if user.Id != config.AppConfig.OwnerId {
		return ext.ContinueGroups
	}

	msg := ctx.EffectiveMessage
	userId := extraction.ExtractUser(b, ctx)
	if userId == -1 {
		return ext.ContinueGroups
	} else if helpers.IsChannelID(userId) {
		return ext.ContinueGroups
	}

	reqUser, err := b.GetChat(userId, nil)
	if err != nil {
		log.Error(err)
		return err
	}
	memStatus := db.GetTeamMemInfo(userId)

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	if memStatus.Dev {
		txt, _ = tr.GetString("devs_user_already_dev")
	} else {
		textTemplate, _ := tr.GetString("devs_added_to_dev")
		txt = fmt.Sprintf(textTemplate, helpers.MentionHtml(reqUser.Id, reqUser.FirstName))
		go db.AddDev(userId)
	}
	_, err = msg.Reply(b, txt, &gotgbot.SendMessageOpts{ParseMode: helpers.HTML})
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.ContinueGroups
}

/*
	Function used to remove sudo users from database of bot

Can only be used by OWNER
*/
// remSudo removes a user from the sudo users list in the bot's database.
// Only accessible by bot owner. Revokes elevated permissions from the specified user.
func (moduleStruct) remSudo(b *gotgbot.Bot, ctx *ext.Context) error {
	user := ctx.EffectiveSender.User
	if user.Id != config.AppConfig.OwnerId {
		return ext.ContinueGroups
	}

	msg := ctx.EffectiveMessage
	userId := extraction.ExtractUser(b, ctx)
	if userId == -1 {
		return ext.ContinueGroups
	} else if helpers.IsChannelID(userId) {
		return ext.ContinueGroups
	}

	reqUser, err := b.GetChat(userId, nil)
	if err != nil {
		log.Error(err)
		return err
	}
	memStatus := db.GetTeamMemInfo(userId)

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	if !memStatus.Sudo {
		txt, _ = tr.GetString("devs_user_not_sudo")
	} else {
		textTemplate, _ := tr.GetString("devs_removed_from_sudo")
		txt = fmt.Sprintf(textTemplate, helpers.MentionHtml(reqUser.Id, reqUser.FirstName))
		go db.RemDev(userId)
	}
	_, err = msg.Reply(b, txt, &gotgbot.SendMessageOpts{ParseMode: helpers.HTML})
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.ContinueGroups
}

/*
	Function used to remove dev users from database of bot

Can only be used by OWNER
*/
// remDev removes a user from the developer users list in the bot's database.
// Only accessible by bot owner. Revokes developer-level permissions from the specified user.
func (moduleStruct) remDev(b *gotgbot.Bot, ctx *ext.Context) error {
	user := ctx.EffectiveSender.User
	if user.Id != config.AppConfig.OwnerId {
		return ext.ContinueGroups
	}

	msg := ctx.EffectiveMessage
	userId := extraction.ExtractUser(b, ctx)
	if userId == -1 {
		return ext.ContinueGroups
	} else if helpers.IsChannelID(userId) {
		return ext.ContinueGroups
	}

	reqUser, err := b.GetChat(userId, nil)
	if err != nil {
		log.Error(err)
		return err
	}
	memStatus := db.GetTeamMemInfo(userId)

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	if !memStatus.Dev {
		txt, _ = tr.GetString("devs_user_not_dev")
	} else {
		textTemplate, _ := tr.GetString("devs_removed_from_dev")
		txt = fmt.Sprintf(textTemplate, helpers.MentionHtml(reqUser.Id, reqUser.FirstName))
		go db.RemDev(userId)
	}
	_, err = msg.Reply(b, txt, &gotgbot.SendMessageOpts{ParseMode: helpers.HTML})
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.ContinueGroups
}

/*
	Function used to list all members of bot's development team

Can only be used by existing team members
*/
// listTeam displays all current team members including developers and sudo users.
// Only accessible by existing team members. Shows user mentions organized by permission level.
func (moduleStruct) listTeam(b *gotgbot.Bot, ctx *ext.Context) error {
	user := ctx.EffectiveSender.User

	teamUsers := db.GetTeamMembers()
	var teamint64Slice []int64
	for k := range teamUsers {
		teamint64Slice = append(teamint64Slice, k)
	}
	teamint64Slice = append(teamint64Slice, config.AppConfig.OwnerId)

	if !string_handling.FindInInt64Slice(teamint64Slice, user.Id) {
		return ext.EndGroups
	}

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	devHeader, _ := tr.GetString("devs_dev_users_header")
	sudoHeader, _ := tr.GetString("devs_sudo_users_header")
	var (
		txt       string
		dev       = devHeader + "\n"
		sudo      = sudoHeader + "\n"
		sudoUsers = make([]string, 0)
		devUsers  = make([]string, 0)
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
	user := ctx.EffectiveSender.User
	memStatus := db.GetTeamMemInfo(user.Id)

	// only devs and owner can access this
	if user.Id != config.AppConfig.OwnerId && !memStatus.Dev {
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
