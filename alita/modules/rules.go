package modules

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"

	tgmd2html "github.com/PaulSonOfLars/gotg_md2html"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"

	"github.com/divkix/Alita_Robot/alita/utils/decorators/cmdDecorator"
	"github.com/divkix/Alita_Robot/alita/utils/decorators/misc"
)

var rulesModule = moduleStruct{
	moduleName:      "Rules",
	defaultRulesBtn: "Rules",
}

// clearRules handles commands to completely remove all rules
// from the chat, requiring admin permissions.
func (moduleStruct) clearRules(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	// connection status
	connectedChat := helpers.IsUserConnected(bot, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	chat := connectedChat

	db.SetChatRules(chat.Id, "")
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	text, _ := tr.GetString("rules_cleared_successfully")
	_, err := msg.Reply(bot, text, helpers.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// privaterules handles the /privaterules command to toggle whether
// rules are sent privately or in the group chat.
func (moduleStruct) privaterules(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	// connection status
	connectedChat := helpers.IsUserConnected(bot, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	chat := connectedChat
	args := ctx.Args()
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	var text string

	if len(args) >= 2 {
		switch strings.ToLower(args[1]) {
		case "on", "yes", "true":
			db.SetPrivateRules(chat.Id, true)
			text, _ = tr.GetString("rules_private_pm_usage")
		case "off", "no", "false":
			db.SetPrivateRules(chat.Id, false)
			temp, _ := tr.GetString("rules_private_group_usage")
			text = fmt.Sprintf(temp, chat.Title)
		default:
			text, _ = tr.GetString("pins_input_not_recognized")
		}
	} else {
		rulesprefs := db.GetChatRulesInfo(chat.Id)
		if rulesprefs.Private {
			text, _ = tr.GetString("rules_private_current_pm")
		} else {
			temp2, _ := tr.GetString("rules_private_current_group")
			text = fmt.Sprintf(temp2, chat.Title)
		}
	}

	_, err := msg.Reply(bot, text, helpers.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// sendRules handles the /rules command to display chat rules
// either in the group or privately based on settings.
func (m moduleStruct) sendRules(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	// if command is disabled, return
	if chat_status.CheckDisabledCmd(bot, msg, "rules") {
		return ext.EndGroups
	}
	// connection status
	connectedChat := helpers.IsUserConnected(bot, ctx, false, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	chat := connectedChat
	var (
		replyMsgId int64
		Text       = ""
		err        error
		rulesKb    gotgbot.InlineKeyboardMarkup
		rulesBtn   string
	)

	if reply := msg.ReplyToMessage; reply != nil {
		replyMsgId = reply.MessageId
	} else {
		replyMsgId = msg.MessageId
	}

	rules := db.GetChatRulesInfo(chat.Id)
	rulesBtn = rules.RulesBtn
	if rulesBtn == "" {
		rulesBtn = m.defaultRulesBtn
	}
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	if rules.Rules != "" {
		temp, _ := tr.GetString("rules_for_chat_header")
		Text += fmt.Sprintf(temp, chat.Title) + "\n\n"
		Text += rules.Rules
	} else {
		Text, _ = tr.GetString("rules_no_rules_set")
	}

	if chat_status.RequireGroup(bot, ctx, nil, true) && rules.Private {
		Text, _ = tr.GetString("rules_click_for_rules")
		rulesKb = gotgbot.InlineKeyboardMarkup{
			InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
				{
					{
						Text: rulesBtn,
						Url:  fmt.Sprintf("t.me/%s?start=rules_%d", bot.Username, chat.Id),
					},
				},
			},
		}
	}

	_, err = msg.Reply(bot, Text,
		&gotgbot.SendMessageOpts{
			ReplyMarkup: rulesKb,
			ReplyParameters: &gotgbot.ReplyParameters{
				MessageId:                replyMsgId,
				AllowSendingWithoutReply: true,
			},
			ParseMode: helpers.HTML,
		},
	)
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// setRules handles the /setrules command to create or update
// chat rules with markdown formatting support.
func (moduleStruct) setRules(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	// connection status
	connectedChat := helpers.IsUserConnected(bot, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	chat := connectedChat
	args := ctx.Args()
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	var text string

	if len(args) == 1 && msg.ReplyToMessage == nil {
		text, _ = tr.GetString("rules_need_text")
	} else {
		if msg.ReplyToMessage != nil {
			text = msg.ReplyToMessage.OriginalMDV2()
		} else {
			// Extract text safely to prevent panic and avoid setting empty rules
			parts := strings.SplitN(msg.OriginalMDV2(), " ", 2)
			if len(parts) >= 2 {
				text = parts[1]
			} else {
				// No text provided after command - show error
				text, _ = tr.GetString("rules_need_text")
				_, err := msg.Reply(bot, text, helpers.Shtml())
				if err != nil {
					log.Error(err)
					return err
				}
				return ext.EndGroups
			}
		}
		db.SetChatRules(chat.Id, tgmd2html.MD2HTMLV2(text))
		text, _ = tr.GetString("rules_set_successfully")
	}

	_, err := msg.Reply(bot, text, helpers.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// rulesBtn handles the /rulesbutton command to set or view
// the custom button text for private rules links.
func (m moduleStruct) rulesBtn(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	// connection status
	connectedChat := helpers.IsUserConnected(bot, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	chat := connectedChat
	user := chat_status.RequireUser(bot, ctx, false)
	if user == nil {
		return ext.EndGroups
	}
	args := ctx.Args()
	var err error
	var text string

	if !chat_status.IsUserAdmin(bot, chat.Id, user.Id) {
		return ext.EndGroups
	}

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	if len(args) >= 2 {
		rulesBtnCustomText := strings.Join(args[1:], " ")
		if len(rulesBtnCustomText) > 30 {
			text, _ = tr.GetString("rules_button_too_long")
		} else {
			db.SetChatRulesButton(chat.Id, rulesBtnCustomText)
			temp3, _ := tr.GetString("rules_button_set_successfully")
			text = fmt.Sprintf(temp3, rulesBtnCustomText)
		}
	} else {
		customRulesBtn := db.GetChatRulesInfo(chat.Id).RulesBtn
		if customRulesBtn == "" {
			temp4, _ := tr.GetString("rules_button_not_set")
			text = fmt.Sprintf(temp4, m.defaultRulesBtn)
		} else {
			temp5, _ := tr.GetString("rules_button_current")
			text = fmt.Sprintf(temp5, customRulesBtn)
		}
	}

	_, err = msg.Reply(bot, text, helpers.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// resetRulesBtn handles commands to reset the custom rules button
// text back to the default value.
func (moduleStruct) resetRulesBtn(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	// connection status
	connectedChat := helpers.IsUserConnected(bot, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	chat := connectedChat

	db.SetChatRulesButton(chat.Id, "")
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	text, _ := tr.GetString("rules_button_cleared")
	_, err := msg.Reply(bot, text, helpers.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// LoadRules registers all rules module handlers with the dispatcher,
// including rules management and display commands.
func LoadRules(dispatcher *ext.Dispatcher) {
	HelpModule.AbleMap.Store(rulesModule.moduleName, true)

	dispatcher.AddHandler(handlers.NewCommand("rules", rulesModule.sendRules))
	misc.AddCmdToDisableable("rules")
	dispatcher.AddHandler(handlers.NewCommand("setrules", rulesModule.setRules))
	cmdDecorator.MultiCommand(dispatcher, []string{"resetrules", "clearrules"}, rulesModule.clearRules)
	dispatcher.AddHandler(handlers.NewCommand("privaterules", rulesModule.privaterules))
	dispatcher.AddHandler(handlers.NewCommand("rulesbutton", rulesModule.rulesBtn))
	dispatcher.AddHandler(handlers.NewCommand("rulesbtn", rulesModule.rulesBtn))
	dispatcher.AddHandler(handlers.NewCommand("clearrulesbutton", rulesModule.resetRulesBtn))
	dispatcher.AddHandler(handlers.NewCommand("clearrulesbtn", rulesModule.resetRulesBtn))
	dispatcher.AddHandler(handlers.NewCommand("resetrulesbutton", rulesModule.resetRulesBtn))
	dispatcher.AddHandler(handlers.NewCommand("resetrulesbtn", rulesModule.resetRulesBtn))
}
