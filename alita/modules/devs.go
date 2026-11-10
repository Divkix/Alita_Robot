package modules

import (
	"fmt"
	"html"
	"strconv"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/devs"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/extraction"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

var devsModule = moduleStruct{moduleName: "Dev"}

func (moduleStruct) chatInfo(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	var replyText string

	args := ctx.Args()

	if len(args) < 2 {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		replyText, _ = tr.GetString("devs_specify_chat")
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
		replyText = fmt.Sprintf(textTemplate, html.EscapeString(chat.Title), chat.Id, con, html.EscapeString(chat.InviteLink))
	}

	_, err := msg.Reply(b, replyText, formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func (moduleStruct) chatList(b *gotgbot.Bot, ctx *ext.Context) error {
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

	if _, err = rMsg.Delete(b, nil); err != nil {
		log.Error(err)
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
		if _, rErr := msg.Reply(b, trS(tr, "devs_chatlist_error"), nil); rErr != nil {
			log.Error(rErr)
		}
		return ext.EndGroups
	}

	return ext.EndGroups
}

func (moduleStruct) leaveChat(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	args := ctx.Args()

	if len(args) < 2 {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		replyText, _ := tr.GetString("devs_specify_chat")
		_, err := msg.Reply(b, replyText, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
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

	return ext.EndGroups
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
	msg := ctx.EffectiveMessage
	userId := extraction.ExtractUser(b, ctx)
	if userId == -1 {
		return ext.ContinueGroups
	} else if userId == 0 {
		if _, err := msg.Reply(b, trS(ctxTr(ctx), "common_no_user_specified"), nil); err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
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
	return ext.EndGroups
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
	teamUsers := devs.GetTeamMembers()
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
	return ext.EndGroups
}

func LoadDev(dispatcher *ext.Dispatcher) {
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:           "stats",
		RequiredChecks: []helpers.CheckFunc{helpers.RequireBotDeveloper()},
	}, pipelineHandler(devsModule.getStats))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:           "addsudo",
		RequiredChecks: []helpers.CheckFunc{helpers.RequireBotOwner()},
	}, pipelineHandler(devsModule.addSudo))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:           "adddev",
		RequiredChecks: []helpers.CheckFunc{helpers.RequireBotOwner()},
	}, pipelineHandler(devsModule.addDev))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:           "remsudo",
		RequiredChecks: []helpers.CheckFunc{helpers.RequireBotOwner()},
	}, pipelineHandler(devsModule.remSudo))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:           "remdev",
		RequiredChecks: []helpers.CheckFunc{helpers.RequireBotOwner()},
	}, pipelineHandler(devsModule.remDev))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:           "teamusers",
		RequiredChecks: []helpers.CheckFunc{helpers.RequireBotTeamMember()},
	}, pipelineHandler(devsModule.listTeam))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:           "chatinfo",
		RequiredChecks: []helpers.CheckFunc{helpers.RequireBotDeveloper()},
	}, pipelineHandler(devsModule.chatInfo))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:           "chatlist",
		RequiredChecks: []helpers.CheckFunc{helpers.RequireBotDeveloper()},
	}, pipelineHandler(devsModule.chatList))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name:           "leavechat",
		RequiredChecks: []helpers.CheckFunc{helpers.RequireBotDeveloper()},
	}, pipelineHandler(devsModule.leaveChat))
}

func init() {
	RegisterLegacyModule("Dev", 120, LoadDev)
}
