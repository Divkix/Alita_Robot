package modules

import (
	"fmt"
	"html"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/i18n"

	"github.com/divkix/Alita_Robot/alita/utils/string_handling"
)

// cache the bot username after first successful fetch
var cachedBotUsername string

func getBotUsername(b *gotgbot.Bot) string {
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
					CallbackData: "about.me",
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
					CallbackData: "configuration.step1",
				},
			},
			{
				{
					Text:         backText,
					CallbackData: "helpq.BackStart",
				},
			},
		},
	}
}

func getStartMarkup(tr *i18n.Translator) gotgbot.InlineKeyboardMarkup {
	aboutText, _ := tr.GetString("help_button_about")
	addToChatText, _ := tr.GetString("help_button_add_to_chat")
	supportGroupText, _ := tr.GetString("help_button_support_group")
	commandsHelpText, _ := tr.GetString("help_button_commands_help")
	languageText, _ := tr.GetString("help_button_language")

	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{
					Text:         aboutText,
					CallbackData: "about.main",
				},
			},
			{
				{
					Text: addToChatText,
					Url:  "https://t.me/Alita_Robot?startgroup=botstart",
				},
				{
					Text: supportGroupText,
					Url:  "https://t.me/DivideSupport",
				},
			},
			{
				{
					Text:         commandsHelpText,
					CallbackData: "helpq.Help",
				},
			},
			{
				{
					Text:         languageText,
					CallbackData: "helpq.Languages",
				},
			},
		},
	}
}

var HelpModule = moduleStruct{
	moduleName:     "Help",
	AbleMap:        moduleEnabled{},
	AltHelpOptions: make(map[string][]string),
	helpableKb:     make(map[string][][]gotgbot.InlineKeyboardButton),
}

type moduleEnabled struct {
	modules map[string]bool
}

// Init initializes the module enabled map for tracking enabled bot modules.
// Sets up the internal map structure for storing module activation states.
func (m *moduleEnabled) Init() {
	m.modules = make(map[string]bool)
}

// Store sets the enabled status for a specific module in the bot.
// Records whether a module is active and available for use.
func (m *moduleEnabled) Store(module string, enabled bool) {
	m.modules[module] = enabled
}

// Load retrieves the enabled status for a specific module.
// Returns the module name and whether it's currently enabled and accessible.
func (m *moduleEnabled) Load(module string) (string, bool) {
	log.Info(fmt.Sprintf("[Module] Loading %s module", module))
	return module, m.modules[module]
}

// LoadModules returns a slice of all currently enabled module names.
// Provides a list of active modules that users can access and get help for.
func (m *moduleEnabled) LoadModules() []string {
	modules := make([]string, 0)
	for module := range m.modules {
		moduleName, enabled := m.Load(module)
		if enabled {
			modules = append(modules, moduleName)
		}
	}
	return modules
}

// about displays information about the bot including version and features.
// Shows bot details, links to support channels, and configuration options.
func (moduleStruct) about(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	var (
		currText string
		currKb   gotgbot.InlineKeyboardMarkup
	)

	if query := ctx.CallbackQuery; query != nil {
		args := strings.Split(query.Data, ".")
		response := args[1]

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
							CallbackData: "about.main",
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
				ParseMode: helpers.HTML,
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
								CallbackData: "about.main",
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
				ParseMode: helpers.HTML,
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
	query := ctx.CallbackQuery
	args := strings.Split(query.Data, ".")
	module := args[1]

	var (
		parsemode, helpText string
		replyKb             gotgbot.InlineKeyboardMarkup
	)

	// Sort the module names
	if string_handling.FindInStringSlice([]string{"BackStart", "Help"}, module) {
		parsemode = helpers.HTML
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		switch module {
		case "Help":
			// This shows the main start menu
			helpText = getMainHelp(tr, html.EscapeString(query.From.FirstName))
			replyKb = markup
		case "BackStart":
			// This shows the modules menu
			helpText = getStartHelp(tr)
			replyKb = getStartMarkup(tr)
		}
	} else {
		// For all remaining modules
		// FIXME: error for pins, purges, reports, rules, warns
		helpText, replyKb, parsemode = getHelpTextAndMarkup(ctx, strings.ToLower(module))
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
	user := ctx.EffectiveSender.User
	msg := ctx.EffectiveMessage
	args := ctx.Args()

	if ctx.Message.Chat.Type == "private" {
		if len(args) == 1 {
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			startHelpText := getStartHelp(tr)
			startMarkupKb := getStartMarkup(tr)
			_, err := msg.Reply(b,
				startHelpText,
				&gotgbot.SendMessageOpts{
					ParseMode: helpers.HTML,
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
			err := startHelpPrefixHandler(b, ctx, user, args[1])
			if err != nil {
				log.Error(err)
				return err
			}
		} else {
			log.WithField("args", args).Debug("Unexpected number of args in /start deep link")
		}
	} else {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("help_pm_questions")
		_, err := msg.Reply(b, text, helpers.Shtml())
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
			ParseMode: helpers.HTML,
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
	query := ctx.CallbackQuery
	msg := query.Message

	// just in case
	if msg.GetChat().Type != "private" {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("help_config_private_only")
		_, _, err := msg.EditText(b, text, nil)
		if err != nil {
			log.Error(err)
			return err
		}
	}

	args := strings.Split(query.Data, ".")
	response := args[1]

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
					CallbackData: "configuration.step2",
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
					CallbackData: "configuration.step3",
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
					CallbackData: "helpq.Help",
				},
			},
		}
		text, _ = tr.GetString("help_configuration_step-3")
	}
	_, _, err := msg.EditText(
		b,
		text,
		&gotgbot.EditMessageTextOpts{
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
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			mainHelpText := getMainHelp(tr, html.EscapeString(msg.From.FirstName))
			_, err := b.SendMessage(chat.Id,
				mainHelpText,
				&gotgbot.SendMessageOpts{
					ParseMode:   helpers.HTML,
					ReplyMarkup: &markup,
				},
			)
			if err != nil {
				log.Error(err)
				return err
			}
		} else if len(args) == 2 {
			module := strings.ToLower(args[1])
			helpText, replyMarkup, _parsemode := getHelpTextAndMarkup(ctx, module)
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
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		pmMeKbText, _ := tr.GetString("help_click_here")
		pmMeKbUri := fmt.Sprintf("https://t.me/%s?start=help_help", b.Username)
		moduleHelpString, _ := tr.GetString("help_contact_pm")
		replyMsgId := msg.MessageId
		var lowerModName string

		if len(args) == 2 {
			helpModName := args[1]
			lowerModName = strings.ToLower(helpModName)
			originalModuleName := getModuleNameFromAltName(lowerModName)
			if originalModuleName != "" && string_handling.FindInStringSlice(getAltNamesOfModule(originalModuleName), lowerModName) {
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
				ParseMode: helpers.HTML,
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
	dispatcher.AddHandler(handlers.NewCommand("start", HelpModule.start))
	dispatcher.AddHandler(handlers.NewCommand("help", HelpModule.help))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("helpq"), HelpModule.helpButtonHandler))
	dispatcher.AddHandler(handlers.NewCommand("donate", HelpModule.donate))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("configuration"), HelpModule.botConfig))
	dispatcher.AddHandler(handlers.NewCommand("about", HelpModule.about))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("about"), HelpModule.about))
	initHelpButtons()
}
