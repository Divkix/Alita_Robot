package modules

import (
	"fmt"
	"html"
	"strings"
	"unicode"

	log "github.com/sirupsen/logrus"

	tgmd2html "github.com/PaulSonOfLars/gotg_md2html"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"

	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/db/rules"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

var rulesModule = moduleStruct{
	moduleName:      "Rules",
	defaultRulesBtn: "Rules",
}

// textAfterCommand returns the text after the command token, split at the first
// whitespace of any kind (as gotgbot does), with leading whitespace trimmed.
func textAfterCommand(s string) string {
	i := strings.IndexFunc(s, unicode.IsSpace)
	if i < 0 {
		return ""
	}
	return strings.TrimLeftFunc(s[i:], unicode.IsSpace)
}

// defaultRulesButtonText returns the localized default rules-button label.
func (m moduleStruct) defaultRulesButtonText(tr *i18n.Translator) string {
	if text, _ := tr.GetString("button_rules_default"); text != "" {
		return text
	}
	return m.defaultRulesBtn
}

//nolint:dupl // clearRules has similar structure to resetRulesBtn
func (moduleStruct) clearRules(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	connectedChat := chat_status.IsUserConnected(bot, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	chat := connectedChat

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	var text string
	if err := rules.SetChatRules(chat.Id, ""); err != nil {
		log.Errorf("[Rules] Failed to clear rules for chat %d: %v", chat.Id, err)
		text, _ = tr.GetString("common_settings_save_failed")
	} else {
		text, _ = tr.GetString("rules_cleared_successfully")
	}
	_, err := msg.Reply(bot, text, formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func (moduleStruct) privaterules(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	connectedChat := chat_status.IsUserConnected(bot, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	chat := connectedChat
	args := ctx.Args()
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	var text string

	if len(args) >= 2 {
		switch strings.ToLower(args[1]) {
		case "on", "yes", "true":
			if err := rules.SetPrivateRules(chat.Id, true); err != nil {
				log.Errorf("[Rules] Failed to enable private rules for chat %d: %v", chat.Id, err)
				text, _ = tr.GetString("common_settings_save_failed")
			} else {
				text, _ = tr.GetString("rules_private_pm_usage")
			}
		case "off", "no", "false":
			if err := rules.SetPrivateRules(chat.Id, false); err != nil {
				log.Errorf("[Rules] Failed to disable private rules for chat %d: %v", chat.Id, err)
				text, _ = tr.GetString("common_settings_save_failed")
			} else {
				temp, _ := tr.GetString("rules_private_group_usage")
				text = fmt.Sprintf(temp, html.EscapeString(chat.Title))
			}
		default:
			text, _ = tr.GetString("pins_input_not_recognized")
		}
	} else {
		rulesprefs := rules.GetChatRulesInfo(chat.Id)
		if rulesprefs.Private {
			text, _ = tr.GetString("rules_private_current_pm")
		} else {
			temp2, _ := tr.GetString("rules_private_current_group")
			text = fmt.Sprintf(temp2, html.EscapeString(chat.Title))
		}
	}

	_, err := msg.Reply(bot, text, formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func (m moduleStruct) sendRules(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	if chat_status.CheckDisabledCmd(bot, msg, "rules") {
		return ext.EndGroups
	}
	connectedChat := chat_status.IsUserConnected(bot, ctx, false, true)
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

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	rules := rules.GetChatRulesInfo(chat.Id)
	rulesBtn = rules.RulesBtn
	if rulesBtn == "" {
		rulesBtn = m.defaultRulesButtonText(tr)
	}
	normalizedRules := normalizeRulesForHTML(rules.Rules)
	if normalizedRules != "" {
		temp, _ := tr.GetString("rules_for_chat_header")
		Text += fmt.Sprintf(temp, html.EscapeString(chat.Title)) + "\n\n"
		Text += normalizedRules
	} else {
		Text, _ = tr.GetString("rules_no_rules_set")
	}

	if chat_status.RequireGroup(bot, ctx, nil) && rules.Private {
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
			ParseMode: formatting.HTML,
		},
	)
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func (moduleStruct) setRules(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	connectedChat := chat_status.IsUserConnected(bot, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	chat := connectedChat
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	var text, rulesText string
	// A reply to the topic-creation service message is Telegram's implicit
	// reply in forum topics, not a user's choice of rules source.
	if reply := msg.ReplyToMessage; reply != nil && reply.ForumTopicCreated == nil {
		rulesText = reply.OriginalTextMDV2() // text, or caption for media
	}
	if strings.TrimSpace(rulesText) == "" {
		rulesText = textAfterCommand(msg.OriginalMDV2())
	}
	if strings.TrimSpace(rulesText) == "" {
		text, _ = tr.GetString("rules_need_text")
	} else if err := rules.SetChatRules(chat.Id, tgmd2html.MD2HTMLV2(rulesText)); err != nil {
		log.Errorf("[Rules] Failed to set rules for chat %d: %v", chat.Id, err)
		text, _ = tr.GetString("common_settings_save_failed")
	} else {
		text, _ = tr.GetString("rules_set_successfully")
	}

	_, err := msg.Reply(bot, text, formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func (m moduleStruct) rulesBtn(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	connectedChat := chat_status.IsUserConnected(bot, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	chat := connectedChat
	user := chat_status.RequireUser(bot, ctx)
	if user == nil {
		return ext.EndGroups
	}
	args := ctx.Args()
	var err error
	var text string

	if !chat_status.IsUserAdmin(bot, chat.Id, user.Id) {
		return ext.EndGroups
	}

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	if len(args) >= 2 {
		rulesBtnCustomText := strings.Join(args[1:], " ")
		if len([]rune(rulesBtnCustomText)) > 30 {
			text, _ = tr.GetString("rules_button_too_long")
		} else {
			if err := rules.SetChatRulesButton(chat.Id, rulesBtnCustomText); err != nil {
				log.Errorf("[Rules] Failed to set rules button for chat %d: %v", chat.Id, err)
				text, _ = tr.GetString("common_settings_save_failed")
			} else {
				temp3, _ := tr.GetString("rules_button_set_successfully")
				text = fmt.Sprintf(temp3, html.EscapeString(rulesBtnCustomText))
			}
		}
	} else {
		customRulesBtn := rules.GetChatRulesInfo(chat.Id).RulesBtn
		if customRulesBtn == "" {
			temp4, _ := tr.GetString("rules_button_not_set")
			// The button is drawn in the group, so its label follows the
			// group's language even when /rulesbtn runs from a connected PM.
			groupTr := i18n.MustNewTranslator(lang.GetLanguage(&ext.Context{EffectiveChat: chat}))
			text = fmt.Sprintf(temp4, html.EscapeString(m.defaultRulesButtonText(groupTr)))
		} else {
			temp5, _ := tr.GetString("rules_button_current")
			text = fmt.Sprintf(temp5, html.EscapeString(customRulesBtn))
		}
	}

	_, err = msg.Reply(bot, text, formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

//nolint:dupl // resetRulesBtn has similar structure to clearRules
func (moduleStruct) resetRulesBtn(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	connectedChat := chat_status.IsUserConnected(bot, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	chat := connectedChat

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	var text string
	if err := rules.SetChatRulesButton(chat.Id, ""); err != nil {
		log.Errorf("[Rules] Failed to clear rules button for chat %d: %v", chat.Id, err)
		text, _ = tr.GetString("common_settings_save_failed")
	} else {
		text, _ = tr.GetString("rules_button_cleared")
	}
	_, err := msg.Reply(bot, text, formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func LoadRules(dispatcher *ext.Dispatcher) {
	SetModuleEnabled(rulesModule.moduleName, true)

	dispatcher.AddHandler(handlers.NewCommand("rules", withPMGroupArg("rules", pmArgsNone, rulesModule.sendRules)))
	helpers.AddCmdToDisableable("rules")
	dispatcher.AddHandler(handlers.NewCommand("setrules", withPMGroupArg("setrules", pmArgsOwn, rulesModule.setRules)))
	dispatcher.AddHandler(handlers.NewCommand("resetrules", withPMGroupArg("resetrules", pmArgsNone, rulesModule.clearRules)))
	dispatcher.AddHandler(handlers.NewCommand("clearrules", withPMGroupArg("resetrules", pmArgsNone, rulesModule.clearRules)))
	dispatcher.AddHandler(handlers.NewCommand("privaterules", withPMGroupArg("privaterules", pmArgsOwn, rulesModule.privaterules)))
	dispatcher.AddHandler(handlers.NewCommand("rulesbutton", withPMGroupArg("rulesbtn", pmArgsOwn, rulesModule.rulesBtn)))
	dispatcher.AddHandler(handlers.NewCommand("rulesbtn", withPMGroupArg("rulesbtn", pmArgsOwn, rulesModule.rulesBtn)))
	dispatcher.AddHandler(handlers.NewCommand("clearrulesbutton", withPMGroupArg("resetrulesbutton", pmArgsNone, rulesModule.resetRulesBtn)))
	dispatcher.AddHandler(handlers.NewCommand("clearrulesbtn", withPMGroupArg("resetrulesbutton", pmArgsNone, rulesModule.resetRulesBtn)))
	dispatcher.AddHandler(handlers.NewCommand("resetrulesbutton", withPMGroupArg("resetrulesbutton", pmArgsNone, rulesModule.resetRulesBtn)))
	dispatcher.AddHandler(handlers.NewCommand("resetrulesbtn", withPMGroupArg("resetrulesbutton", pmArgsNone, rulesModule.resetRulesBtn)))
}

func init() {
	RegisterLegacyModule("Rules", 190, LoadRules)
	RegisterDeepLinkHandler("rules_", rulesDeepLinkHandler)
}

func rulesDeepLinkHandler(b *gotgbot.Bot, ctx *ext.Context, user *gotgbot.User, arg string) error {
	msg := ctx.EffectiveMessage

	chatinfo, err := parseChatInfoFromDeepLink(b, ctx, arg)
	if err != nil {
		return err
	}
	chatID := chatinfo.Id

	_chat := chatinfo.ToChat()
	if !chat_status.IsUserInChat(b, &_chat, user.Id) {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("helpers_chat_not_found")
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return ext.EndGroups
	}

	rulesrc := rules.GetChatRulesInfo(chatID)
	normalizedRules := normalizeRulesForHTML(rulesrc.Rules)

	if normalizedRules == "" {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("rules_not_set")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	text, _ := tr.GetString("rules_for_chat", i18n.TranslationParams{
		"first":  html.EscapeString(chatinfo.Title),
		"second": normalizedRules,
	})
	_, err = msg.Reply(b, text, formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}
