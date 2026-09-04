package modules

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	log "github.com/sirupsen/logrus"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/devs"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/extraction"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
)

var devsModule = moduleStruct{moduleName: "Dev"}

func (moduleStruct) chatInfo(b *gotgbot.Bot, ctx *ext.Context) error {
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	memStatus := devs.GetTeamMemInfo(user.Id)

	if user.Id != config.AppConfig.OwnerId && !memStatus.IsDev {
		return ext.ContinueGroups
	}

	msg := ctx.EffectiveMessage
	var replyText string

	args := ctx.Args()

	if len(args) < 2 {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		replyText, _ = tr.GetString("devs_specify_user")
	} else {
		_chatId := args[1]
		chatId, _ := strconv.ParseInt(_chatId, 10, 64)
		chat, err := b.GetChat(chatId, nil)
		if err != nil {
			_, _ = msg.Reply(b, err.Error(), nil)
			return ext.EndGroups
		}
		_chat := chat.ToChat()
		gChat := &_chat
		con, _ := gChat.GetMemberCount(b, nil)
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		textTemplate, _ := tr.GetString("devs_chat_info")
		replyText = fmt.Sprintf(textTemplate, chat.Title, chat.Id, con, chat.InviteLink)
	}

	_, err := msg.Reply(b, replyText, formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.ContinueGroups
}

func (moduleStruct) chatList(b *gotgbot.Bot, ctx *ext.Context) error {
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	memStatus := devs.GetTeamMemInfo(user.Id)

	if user.Id != config.AppConfig.OwnerId && !memStatus.IsDev {
		return ext.ContinueGroups
	}

	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
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

	allChats := chats.GetAllChats()

	var sb strings.Builder
	for chatId, v := range allChats {
		if !v.IsInactive {
			fmt.Fprintf(&sb, "%d: %s\n", chatId, v.ChatName)
		}
	}

	_, err = rMsg.Delete(b, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	_, err = b.SendDocument(
		chat.Id,
		gotgbot.InputFileByReader("chatlist.txt", strings.NewReader(sb.String())),
		&gotgbot.SendDocumentOpts{
			Caption: trS(tr, "devs_chat_list_caption"),
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

func (moduleStruct) leaveChat(b *gotgbot.Bot, ctx *ext.Context) error {
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	memStatus := devs.GetTeamMemInfo(user.Id)

	if user.Id != config.AppConfig.OwnerId && !memStatus.IsDev {
		return ext.ContinueGroups
	}

	msg := ctx.EffectiveMessage
	args := ctx.Args()

	if len(args) < 2 {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		replyText, _ := tr.GetString("devs_specify_user")
		_, err := msg.Reply(b, replyText, formatting.Shtml())
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

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	text, _ := tr.GetString("devs_left_chat")
	_, err = msg.Reply(b, text, formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.ContinueGroups
}

type teamRoleConfig struct {
	roleName      string
	add           bool
	checkRole     func(*db.DevSettings) bool
	alreadyMsgKey string
	notRoleMsgKey string
	failMsgKey    string
	successMsgKey string
	dbOp          func(int64) error
}

func (m moduleStruct) manageTeamRole(b *gotgbot.Bot, ctx *ext.Context, cfg teamRoleConfig) error {
	user := chat_status.RequireUser(b, ctx)
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
	memStatus := devs.GetTeamMemInfo(userId)

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	var txt string

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
			txt = fmt.Sprintf(textTemplate, formatting.MentionHtml(reqUser.Id, reqUser.FirstName))
		}
	}

	_, err = msg.Reply(b, txt, &gotgbot.SendMessageOpts{ParseMode: formatting.HTML})
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.ContinueGroups
}

func (m moduleStruct) addSudo(b *gotgbot.Bot, ctx *ext.Context) error {
	return m.manageTeamRole(b, ctx, teamRoleConfig{
		roleName:      "sudo",
		add:           true,
		checkRole:     func(tm *db.DevSettings) bool { return tm.Sudo },
		alreadyMsgKey: "devs_user_already_sudo",
		failMsgKey:    "devs_failed_to_add_sudo",
		successMsgKey: "devs_added_to_sudo",
		dbOp:          devs.AddSudo,
	})
}

func (m moduleStruct) addDev(b *gotgbot.Bot, ctx *ext.Context) error {
	return m.manageTeamRole(b, ctx, teamRoleConfig{
		roleName:      "dev",
		add:           true,
		checkRole:     func(tm *db.DevSettings) bool { return tm.IsDev },
		alreadyMsgKey: "devs_user_already_dev",
		failMsgKey:    "devs_failed_to_add_dev",
		successMsgKey: "devs_added_to_dev",
		dbOp:          devs.AddDev,
	})
}

func (m moduleStruct) remSudo(b *gotgbot.Bot, ctx *ext.Context) error {
	return m.manageTeamRole(b, ctx, teamRoleConfig{
		roleName:      "sudo",
		add:           false,
		checkRole:     func(tm *db.DevSettings) bool { return tm.Sudo },
		notRoleMsgKey: "devs_user_not_sudo",
		failMsgKey:    "devs_failed_to_remove_sudo",
		successMsgKey: "devs_removed_from_sudo",
		dbOp:          devs.RemSudo,
	})
}

func (m moduleStruct) remDev(b *gotgbot.Bot, ctx *ext.Context) error {
	return m.manageTeamRole(b, ctx, teamRoleConfig{
		roleName:      "dev",
		add:           false,
		checkRole:     func(tm *db.DevSettings) bool { return tm.IsDev },
		notRoleMsgKey: "devs_user_not_dev",
		failMsgKey:    "devs_failed_to_remove_dev",
		successMsgKey: "devs_removed_from_dev",
		dbOp:          devs.RemDev,
	})
}

func (moduleStruct) listTeam(b *gotgbot.Bot, ctx *ext.Context) error {
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}

	teamUsers := devs.GetTeamMembers()
	var teamint64Slice []int64
	for k := range teamUsers {
		teamint64Slice = append(teamint64Slice, k)
	}
	teamint64Slice = append(teamint64Slice, config.AppConfig.OwnerId)

	if !slices.Contains(teamint64Slice, user.Id) {
		return ext.EndGroups
	}

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
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
				log.Errorf("[Devs] GetChat failed for user %d: %v", userId, err)
				userMentioned := formatting.MentionHtml(userId, fmt.Sprintf("%d", userId))
				switch uPerm {
				case "dev":
					devUsers = append(devUsers, fmt.Sprintf("• %s", userMentioned))
				case "sudo":
					sudoUsers = append(sudoUsers, fmt.Sprintf("• %s", userMentioned))
				}
				continue
			}

			userMentioned := formatting.MentionHtml(reqUser.Id, formatting.GetFullName(reqUser.FirstName, reqUser.LastName))
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

	_, err := msg.Reply(b, txt, &gotgbot.SendMessageOpts{ParseMode: formatting.HTML})
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func (moduleStruct) getStats(b *gotgbot.Bot, ctx *ext.Context) error {
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	memStatus := devs.GetTeamMemInfo(user.Id)

	if user.Id != config.AppConfig.OwnerId && !memStatus.IsDev {
		return ext.ContinueGroups
	}

	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	text, _ := tr.GetString("devs_fetching_stats")
	edits, err := msg.Reply(
		b,
		text,
		&gotgbot.SendMessageOpts{
			ParseMode: formatting.HTML,
		},
	)
	if err != nil {
		log.Error(err)
		return err
	}

	stats := devs.LoadAllStats()
	_, _, err = edits.EditText(b, &gotgbot.EditMessageTextOpts{Text: stats, ParseMode: formatting.HTML})
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.ContinueGroups
}

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

func init() {
	RegisterLegacyModule("Dev", 120, LoadDev)
}
