package modules

import (
	"fmt"
	"html"
	"slices"
	"strings"
	"sync"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/i18n"
)

// cachedBotUsernameMu protects cachedBotUsername against concurrent reads/writes.
var (
	cachedBotUsername   string
	cachedBotUsernameMu sync.RWMutex
)

func getBotUsername(b *gotgbot.Bot) string {
	cachedBotUsernameMu.RLock()
	cached := cachedBotUsername
	cachedBotUsernameMu.RUnlock()
	if cached != "" {
		return cached
	}
	cachedBotUsernameMu.Lock()
	defer cachedBotUsernameMu.Unlock()
	// double-check under write lock
	if cachedBotUsername != "" {
		return cachedBotUsername
	}
	// try bot struct first
	if b != nil && b.Username != "" {
		cachedBotUsername = b.Username
		return cachedBotUsername
	}
	// fallback to GetMe
	if b != nil {
		if me, err := b.GetMe(nil); err == nil && me != nil && me.Username != "" {
			cachedBotUsername = me.Username
			return cachedBotUsername
		}
	}
	return ""
}

// Dynamic strings that will be loaded using i18n
func getAboutText(tr *i18n.Translator) string {
	text, _ := tr.GetString("help_info_about_header")
	return text
}

func getStartHelp(tr *i18n.Translator) string {
	text1, _ := tr.GetString("help_bot_intro")
	text2, _ := tr.GetString("help_news_channel_text")
	return text1 + text2
}

func getMainHelp(tr *i18n.Translator, firstName string) string {
	text1, _ := tr.GetString("help_pm_intro", i18n.TranslationParams{"s": firstName})
	text2, _ := tr.GetString("help_all_commands_usage")
	return text1 + text2
}

// Dynamic keyboard generation functions
func getAboutKb(tr *i18n.Translator) gotgbot.InlineKeyboardMarkup {
	aboutMeText, _ := tr.GetString("help_button_about_me")
	newsChannelText, _ := tr.GetString("help_button_news_channel")
	supportGroupText, _ := tr.GetString("help_button_support_group")
	configurationText, _ := tr.GetString("help_button_configuration")
	backText, _ := tr.GetString("common_back_arrow_alt")

	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{
					Text:         aboutMeText,
					CallbackData: encodeCallbackData("about", map[string]string{"a": "me"}, "about.me"),
				},
			},
			{
				{
					Text: newsChannelText,
					Url:  "https://t.me/AlitaRobotUpdates",
				},
				{
					Text: supportGroupText,
					Url:  "https://t.me/DivideSupport",
				},
			},
			{
				{
					Text:         configurationText,
					CallbackData: encodeCallbackData("configuration", map[string]string{"s": "step1"}, "configuration.step1"),
				},
			},
			{
				{
					Text:         backText,
					CallbackData: encodeCallbackData("helpq", map[string]string{"m": "BackStart"}, "helpq.BackStart"),
				},
			},
		},
	}
}

func getStartMarkup(tr *i18n.Translator, botUsername string) gotgbot.InlineKeyboardMarkup {
	aboutText, _ := tr.GetString("help_button_about")
	addToChatText, _ := tr.GetString("help_button_add_to_chat")
	supportGroupText, _ := tr.GetString("help_button_support_group")
	commandsHelpText, _ := tr.GetString("help_button_commands_help")
	languageText, _ := tr.GetString("help_button_language")

	// Build the add to chat URL dynamically using the bot's username
	addToChatUrl := fmt.Sprintf("https://t.me/%s?startgroup=botstart", botUsername)

	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{
					Text:         aboutText,
					CallbackData: encodeCallbackData("about", map[string]string{"a": "main"}, "about.main"),
				},
			},
			{
				{
					Text: addToChatText,
					Url:  addToChatUrl,
				},
				{
					Text: supportGroupText,
					Url:  "https://t.me/DivideSupport",
				},
			},
			{
				{
					Text:         commandsHelpText,
					CallbackData: encodeCallbackData("helpq", map[string]string{"m": "Help"}, "helpq.Help"),
				},
			},
			{
				{
					Text:         languageText,
					CallbackData: encodeCallbackData("helpq", map[string]string{"m": "Languages"}, "helpq.Languages"),
				},
			},
		},
	}
}

// about displays information about the bot including version and features.
// Shows bot details, links to support channels, and configuration options.
func (moduleStruct) about(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	var (
		currText string
		currKb   gotgbot.InlineKeyboardMarkup
	)

	if query, ok := callbackQueryFromContext(ctx); ok {
		if query.Message == nil {
			text, _ := tr.GetString("common_callback_invalid_request")
			_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: text})
			return ext.EndGroups
		}
		response := ""
		if decoded, ok := decodeCallbackData(query.Data, "about"); ok {
			response, _ = decoded.Field("a")
		} else {
			args := strings.Split(query.Data, ".")
			if len(args) >= 2 {
				response = args[1]
			}
		}
		if response == "" {
			log.Warn("[About] Invalid callback data format - missing response part")
			_, _ = query.Answer(b, nil)
			return ext.EndGroups
		}

		switch response {
		case "main":
			currText = getAboutText(tr)
			currKb = getAboutKb(tr)
		case "me":
			temp, _ := tr.GetString("help_about")
			currText = fmt.Sprintf(temp, b.Username, config.AppConfig.BotVersion)
			backText, _ := tr.GetString("common_back")
			currKb = gotgbot.InlineKeyboardMarkup{
				InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
					{
						{
							Text:         backText,
							CallbackData: encodeCallbackData("about", map[string]string{"a": "main"}, "about.main"),
						},
					},
				},
			}
		}
		_, _, err := query.Message.EditText(b,
			currText,
			&gotgbot.EditMessageTextOpts{
				ReplyMarkup: currKb,
				LinkPreviewOptions: &gotgbot.LinkPreviewOptions{
					IsDisabled: true,
				},
				ParseMode: formatting.HTML,
			},
		)
		if err != nil {
			log.Error(err)
			return err
		}

		_, err = query.Answer(b, nil)
		if err != nil {
			log.Error(err)
			return err
		}
	} else {
		if ctx.Message.Chat.Type == "private" {
			currText = getAboutText(tr)
			currKb = getAboutKb(tr)
		} else {
			clickButtonText, _ := tr.GetString("help_click_button_info")
			aboutButtonText, _ := tr.GetString("help_button_about")
			currText = clickButtonText
			if uname := getBotUsername(b); uname != "" {
				currKb = gotgbot.InlineKeyboardMarkup{
					InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
						{
							{
								Text: aboutButtonText,
								Url:  fmt.Sprintf("https://t.me/%s?start=about", uname),
							},
						},
					},
				}
			} else {
				// If bot username unavailable, present a non-linking button to avoid broken deep links
				currKb = gotgbot.InlineKeyboardMarkup{
					InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
						{
							{
								Text:         aboutButtonText,
								CallbackData: encodeCallbackData("about", map[string]string{"a": "main"}, "about.main"),
							},
						},
					},
				}
			}
		}
		_, err := msg.Reply(
			b,
			currText,
			&gotgbot.SendMessageOpts{
				ParseMode: formatting.HTML,
				LinkPreviewOptions: &gotgbot.LinkPreviewOptions{
					IsDisabled: true,
				},
				ReplyMarkup: &currKb,
			},
		)
		if err != nil {
			log.Error(err)
			return err
		}
	}

	return ext.EndGroups
}

// helpButtonHandler processes callback queries from help menu button interactions.
// Navigates between help sections and displays appropriate help content for modules.
func (moduleStruct) helpButtonHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	query, ok := callbackQueryFromContext(ctx)
	if !ok {
		return ext.EndGroups
	}
	if query == nil {
		return ext.EndGroups
	}
	if query.Message == nil {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("common_callback_invalid_request")
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return ext.EndGroups
	}
	module := ""
	if decoded, ok := decodeCallbackData(query.Data, "helpq"); ok {
		module, _ = decoded.Field("m")
	} else {
		args := strings.Split(query.Data, ".")
		if len(args) >= 2 {
			module = args[1]
		}
	}
	if module == "" {
		log.Warn("[HelpButtonHandler] Invalid callback data format - missing module part")
		_, _ = query.Answer(b, nil)
		return ext.EndGroups
	}

	var (
		parsemode, helpText string
		replyKb             gotgbot.InlineKeyboardMarkup
	)

	// Sort the module names
	if slices.Contains([]string{"BackStart", "Help"}, module) {
		parsemode = formatting.HTML
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		switch module {
		case "Help":
			// This shows the main start menu
			helpText = getMainHelp(tr, html.EscapeString(query.From.FirstName))
			replyKb = markup
		case "BackStart":
			// This shows the modules menu
			helpText = getStartHelp(tr)
			replyKb = getStartMarkup(tr, getBotUsername(b))
		}
	} else {
		// For all remaining modules
		helpText, replyKb, parsemode = getHelpTextAndMarkup(ctx, strings.ToLower(module), DefaultHelpRegistry())
	}

	// Edit the main message, the main querymessage
	_, _, err := query.Message.EditText(
		b,
		helpText,
		&gotgbot.EditMessageTextOpts{
			ParseMode:   parsemode,
			ReplyMarkup: replyKb,
			LinkPreviewOptions: &gotgbot.LinkPreviewOptions{
				IsDisabled: true,
			},
		},
	)
	if err != nil {
		log.Error(err)
		return err
	}

	_, err = query.Answer(b, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// start introduces the bot
// start handles the /start command and displays welcome message with navigation options.
// Shows different content in private vs group chats and handles start parameters.
func (moduleStruct) start(b *gotgbot.Bot, ctx *ext.Context) error {
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	msg := ctx.EffectiveMessage
	args := ctx.Args()

	if ctx.Message.Chat.Type == "private" {
		if len(args) == 1 {
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			startHelpText := getStartHelp(tr)
			startMarkupKb := getStartMarkup(tr, getBotUsername(b))
			_, err := msg.Reply(b,
				startHelpText,
				&gotgbot.SendMessageOpts{
					ParseMode: formatting.HTML,
					LinkPreviewOptions: &gotgbot.LinkPreviewOptions{
						IsDisabled: true,
					},
					ReplyMarkup: &startMarkupKb,
				},
			)
			if err != nil {
				log.Error(err)
				return err
			}
		} else if len(args) == 2 {
			err := HandleDeepLink(b, ctx, user, args[1])
			if err != nil {
				log.Error(err)
				return err
			}
		} else {
			log.WithField("args", args).Debug("Unexpected number of args in /start deep link")
		}
	} else {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("help_pm_questions")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
	}
	return ext.EndGroups
}

// donate displays information about supporting the bot and its development.
// Shows donation links and ways users can contribute to bot maintenance.
func (moduleStruct) donate(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat

	_, err := b.SendMessage(chat.Id,
		func() string {
			tr := i18n.MustNewTranslator("en")
			text, _ := tr.GetString("help_donatetext")
			return text
		}(),
		&gotgbot.SendMessageOpts{
			ParseMode: formatting.HTML,
			LinkPreviewOptions: &gotgbot.LinkPreviewOptions{
				IsDisabled: true,
			},
			ReplyParameters: &gotgbot.ReplyParameters{
				MessageId:                msg.MessageId,
				AllowSendingWithoutReply: true,
			},
		},
	)
	if err != nil {
		log.Error(err)
	}

	return ext.EndGroups
}

// botConfig provides step-by-step configuration guidance for new users.
// Walks users through adding the bot to chats and basic setup procedures.
func (moduleStruct) botConfig(b *gotgbot.Bot, ctx *ext.Context) error {
	query, ok := callbackQueryFromContext(ctx)
	if !ok {
		return ext.EndGroups
	}
	if query == nil {
		return ext.EndGroups
	}
	msg := query.Message
	if msg == nil {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("common_callback_invalid_request")
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return ext.EndGroups
	}

	// just in case
	if msg.GetChat().Type != "private" {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("help_config_private_only")
		_, _, err := msg.EditText(b, text, nil)
		if err != nil {
			log.Error(err)
			return err
		}
		_, _ = query.Answer(b, nil)
		return ext.EndGroups
	}

	response := ""
	if decoded, ok := decodeCallbackData(query.Data, "configuration"); ok {
		response, _ = decoded.Field("s")
	} else {
		args := strings.Split(query.Data, ".")
		if len(args) >= 2 {
			response = args[1]
		}
	}
	if response == "" {
		log.Warn("[BotConfig] Invalid callback data format - missing response part")
		_, _ = query.Answer(b, nil)
		return ext.EndGroups
	}

	var (
		iKeyboard [][]gotgbot.InlineKeyboardButton
		text      string
	)

	tr := i18n.MustNewTranslator("en")

	switch response {
	case "step1":
		addAlitaText, _ := tr.GetString("help_button_add_alita")
		doneText, _ := tr.GetString("common_done")
		iKeyboard = [][]gotgbot.InlineKeyboardButton{
			{
				{
					Text: addAlitaText,
					Url:  fmt.Sprintf("https://t.me/%s?startgroup=botstart", b.Username),
				},
			},
			{
				{
					Text:         doneText,
					CallbackData: encodeCallbackData("configuration", map[string]string{"s": "step2"}, "configuration.step2"),
				},
			},
		}
		text, _ = tr.GetString("help_configuration_step-1")
	case "step2":
		doneText, _ := tr.GetString("common_done")
		iKeyboard = [][]gotgbot.InlineKeyboardButton{
			{
				{
					Text:         doneText,
					CallbackData: encodeCallbackData("configuration", map[string]string{"s": "step3"}, "configuration.step3"),
				},
			},
		}
		temp, _ := tr.GetString("help_configuration_step-2")
		text = fmt.Sprintf(temp, b.Username)
	case "step3":
		continueText, _ := tr.GetString("common_continue")
		iKeyboard = [][]gotgbot.InlineKeyboardButton{
			{
				{
					Text:         continueText,
					CallbackData: encodeCallbackData("helpq", map[string]string{"m": "Help"}, "helpq.Help"),
				},
			},
		}
		text, _ = tr.GetString("help_configuration_step-3")
	}
	_, _, err := msg.EditText(
		b,
		text,
		&gotgbot.EditMessageTextOpts{
			ParseMode: formatting.HTML,
			LinkPreviewOptions: &gotgbot.LinkPreviewOptions{
				IsDisabled: true,
			},
			ReplyMarkup: gotgbot.InlineKeyboardMarkup{
				InlineKeyboard: iKeyboard,
			},
		},
	)
	if err != nil {
		log.Error(err)
		return err
	}

	_, err = query.Answer(b, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// help displays the main help menu or specific module help information.
// Shows module list in private messages or provides links to PM help in groups.
func (moduleStruct) help(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	args := ctx.Args()

	if ctx.Message.Chat.Type == "private" {
		if len(args) == 1 {
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			name := "User"
			if msg.From != nil {
				name = html.EscapeString(msg.From.FirstName)
			}
			mainHelpText := getMainHelp(tr, name)
			_, err := b.SendMessage(chat.Id,
				mainHelpText,
				&gotgbot.SendMessageOpts{
					ParseMode:   formatting.HTML,
					ReplyMarkup: &markup,
				},
			)
			if err != nil {
				log.Error(err)
				return err
			}
		} else if len(args) == 2 {
			module := strings.ToLower(args[1])
			helpText, replyMarkup, _parsemode := getHelpTextAndMarkup(ctx, module, DefaultHelpRegistry())
			_, err := b.SendMessage(
				chat.Id,
				helpText,
				&gotgbot.SendMessageOpts{
					ParseMode:   _parsemode,
					ReplyMarkup: &replyMarkup,
				},
			)
			if err != nil {
				log.Error(err)
				return err
			}
		}
	} else {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		pmMeKbText, _ := tr.GetString("help_click_here")
		pmMeKbUri := fmt.Sprintf("https://t.me/%s?start=help_help", b.Username)
		moduleHelpString, _ := tr.GetString("help_contact_pm")
		replyMsgId := msg.MessageId
		var lowerModName string

		if len(args) == 2 {
			helpModName := args[1]
			lowerModName = strings.ToLower(helpModName)
			originalModuleName := getModuleNameFromAltName(lowerModName, DefaultHelpRegistry())
			if originalModuleName != "" && slices.Contains(getAltNamesOfModule(originalModuleName), lowerModName) {
				contactPmText, _ := tr.GetString("help_contact_pm")
				moduleHelpString = strings.Replace(contactPmText, "for help!", fmt.Sprintf("for help regarding <code>%s</code>!", originalModuleName), 1)
				pmMeKbUri = fmt.Sprintf("https://t.me/%s?start=help_%s", b.Username, lowerModName)
			}
		}

		if msg.ReplyToMessage != nil {
			replyMsgId = msg.ReplyToMessage.MessageId
		}

		_, err := msg.Reply(b,
			moduleHelpString,
			&gotgbot.SendMessageOpts{
				ParseMode: formatting.HTML,
				ReplyParameters: &gotgbot.ReplyParameters{
					MessageId:                replyMsgId,
					AllowSendingWithoutReply: true,
				},
				ReplyMarkup: gotgbot.InlineKeyboardMarkup{
					InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
						{
							{Text: pmMeKbText, Url: pmMeKbUri},
						},
					},
				},
			},
		)
		if err != nil {
			log.Error(err)
			return err
		}
	}
	return ext.EndGroups
}

// LoadHelp registers all help-related command and callback handlers.
// Sets up the help system including start, about, donate, and configuration commands.
func LoadHelp(dispatcher *ext.Dispatcher) {
	dispatcher.AddHandler(handlers.NewCommand("start", DefaultHelpRegistry().start))
	dispatcher.AddHandler(handlers.NewCommand("help", DefaultHelpRegistry().help))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("helpq"), DefaultHelpRegistry().helpButtonHandler))
	dispatcher.AddHandler(handlers.NewCommand("donate", DefaultHelpRegistry().donate))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("configuration"), DefaultHelpRegistry().botConfig))
	dispatcher.AddHandler(handlers.NewCommand("about", DefaultHelpRegistry().about))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("about"), DefaultHelpRegistry().about))
	initHelpButtons()
}

func init() {
	RegisterDeepLinkHandler("help_", helpDeepLinkHandler)
	RegisterExactDeepLinkHandler("about", aboutDeepLinkHandler)
}

func helpDeepLinkHandler(b *gotgbot.Bot, ctx *ext.Context, user *gotgbot.User, arg string) error {
	parts := strings.Split(arg, "_")
	if len(parts) < 2 {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("helpers_invalid_deep_link")
		_, _ = ctx.EffectiveMessage.Reply(b, text, formatting.Shtml())
		return ext.EndGroups
	}
	helpModule := parts[1]
	_, err := sendHelpkb(b, ctx, helpModule, DefaultHelpRegistry())
	if err != nil {
		log.Errorf("[Start]: %v", err)
		return err
	}
	return ext.EndGroups
}

func aboutDeepLinkHandler(b *gotgbot.Bot, ctx *ext.Context, user *gotgbot.User, arg string) error {
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	aboutText := getAboutText(tr)
	aboutKb := getAboutKb(tr)
	_, err := b.SendMessage(ctx.EffectiveChat.Id,
		aboutText,
		&gotgbot.SendMessageOpts{
			ParseMode: formatting.HTML,
			LinkPreviewOptions: &gotgbot.LinkPreviewOptions{
				IsDisabled: true,
			},
			ReplyParameters: &gotgbot.ReplyParameters{
				MessageId:                ctx.EffectiveMessage.MessageId,
				AllowSendingWithoutReply: true,
			},
			ReplyMarkup: &aboutKb,
		},
	)
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}
