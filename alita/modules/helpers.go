package modules

import (
	"fmt"
	"html"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/divideprojects/Alita_Robot/alita/db"
	"github.com/divideprojects/Alita_Robot/alita/i18n"
	"github.com/divideprojects/Alita_Robot/alita/utils/chat_status"
	"github.com/divideprojects/Alita_Robot/alita/utils/helpers"

	"github.com/divideprojects/Alita_Robot/alita/utils/string_handling"
	log "github.com/sirupsen/logrus"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// module struct for all modules
type moduleStruct struct {
	moduleName          string
	handlerGroup        int
	permHandlerGroup    int
	restrHandlerGroup   int
	defaultRulesBtn     string
	overwriteFiltersMap map[string]overwriteFilter
	overwriteNotesMap   map[string]overwriteNote
	antiSpam            map[int64]*antiSpamInfo
	AbleMap             moduleEnabled
	AltHelpOptions      map[string][]string
	helpableKb          map[string][][]gotgbot.InlineKeyboardButton
}

// struct for filters module
type overwriteFilter struct {
	filterWord string
	text       string
	fileid     string
	buttons    []db.Button
	dataType   int
}

// struct for notes module
type overwriteNote struct {
	noteWord    string
	text        string
	fileId      string
	buttons     []db.Button
	dataType    int
	pvtOnly     bool
	grpOnly     bool
	adminOnly   bool
	webPrev     bool
	isProtected bool
	noNotif     bool
}

// struct for antiSpam module - antiSpamInfo
type antiSpamInfo struct {
	Levels []antiSpamLevel
}

// struct for antiSpam module - antiSpamLevel
type antiSpamLevel struct {
	Count    int
	Limit    int
	CurrTime time.Duration
	Expiry   time.Duration
	Spammed  bool
}

// helper functions for help module

// This var is used to add the back button to the help menu
// i.e. where modules are shown
var markup gotgbot.InlineKeyboardMarkup

// listModules returns a sorted slice of all currently enabled bot modules.
// Provides an alphabetically ordered list of active modules for help menu generation.
func listModules() []string {
	// sort the modules alphabetically
	modules := HelpModule.AbleMap.LoadModules()
	sort.Strings(modules) // Sort the modules
	return modules
}

// New menu, used for building help menu in bot!
// initHelpButtons initializes the help menu keyboard with all enabled modules.
// Creates a chunked inline keyboard layout for easy module navigation in help system.
func initHelpButtons() {
	var kb []gotgbot.InlineKeyboardButton

	for _, i := range listModules() {
		kb = append(kb, gotgbot.InlineKeyboardButton{Text: i, CallbackData: fmt.Sprintf("helpq.%s", i)})
	}
	zb := helpers.ChunkKeyboardSlices(kb, 3)
	tr := i18n.MustNewTranslator("en") // Default to English for help system
	backText, _ := tr.GetString("helpers_back_button")
	zb = append(zb, []gotgbot.InlineKeyboardButton{{Text: backText, CallbackData: "helpq.BackStart"}})
	markup = gotgbot.InlineKeyboardMarkup{InlineKeyboard: zb}
}

// getModuleHelpAndKb retrieves help text and keyboard for a specific module.
// Returns localized help content and navigation buttons for the requested module.
func getModuleHelpAndKb(module, lang string) (helpText string, replyMarkup gotgbot.InlineKeyboardMarkup) {
	ModName := cases.Title(language.English).String(module)
	tr := i18n.MustNewTranslator(lang)
	helpMsg, _ := tr.GetString(fmt.Sprintf("%s_help_msg", strings.ToLower(ModName)))
	helpText = fmt.Sprintf("Here is the help for the *%s* module:\n\n", ModName) + helpMsg

	// Create back button suffix dynamically
	backText, _ := tr.GetString("common_back_arrow")
	homeText, _ := tr.GetString("common_home")
	backBtnSuffix := []gotgbot.InlineKeyboardButton{
		{
			Text:         backText,
			CallbackData: "helpq.Help",
		},
		{
			Text:         homeText,
			CallbackData: "helpq.BackStart",
		},
	}

	replyMarkup = gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: append(
			HelpModule.helpableKb[ModName],
			backBtnSuffix,
		),
	}
	return
}

// sendHelpkb sends help information for a specific module with navigation keyboard.
// Displays module-specific help content or main help menu based on the requested module.
func sendHelpkb(b *gotgbot.Bot, ctx *ext.Context, module string) (msg *gotgbot.Message, err error) {
	module = strings.ToLower(module)
	if module == "help" {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		helpText := getMainHelp(tr, html.EscapeString(ctx.EffectiveMessage.From.FirstName))
		_, err = b.SendMessage(
			ctx.EffectiveMessage.Chat.Id,
			helpText,
			&gotgbot.SendMessageOpts{
				ParseMode:   helpers.HTML,
				ReplyMarkup: &markup,
			},
		)
		return
	}
	helpText, replyMarkup, _parsemode := getHelpTextAndMarkup(ctx, module)

	msg, err = b.SendMessage(
		ctx.EffectiveChat.Id,
		helpText,
		&gotgbot.SendMessageOpts{
			ParseMode:   _parsemode,
			ReplyMarkup: replyMarkup,
		},
	)
	return
}

// getModuleNameFromAltName resolves alternative module names to their canonical form.
// Searches through module aliases to find the actual module name for help lookups.
func getModuleNameFromAltName(altName string) string {
	for _, modName := range listModules() {
		tr := i18n.MustNewTranslator("config")
		altNamesFromConfig, _ := tr.GetStringSlice(fmt.Sprintf("alt_names.%s", modName))
		altNames := append(altNamesFromConfig, strings.ToLower(modName))
		for _, altNameInSlice := range altNames {
			if altNameInSlice == altName {
				return modName
			}
		}
	}
	return ""
}

// startHelpPrefixHandler processes /start command arguments for specific help topics.
// Handles deep links for help, connections, rules, notes, and about pages.
func startHelpPrefixHandler(b *gotgbot.Bot, ctx *ext.Context, user *gotgbot.User, arg string) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat

	if strings.HasPrefix(arg, "help_") {
		helpModule := strings.Split(arg, "_")[1]
		_, err := sendHelpkb(b, ctx, helpModule)
		if err != nil {
			log.Errorf("[Start]: %v", err)
			return err
		}
	} else if strings.HasPrefix(arg, "connect_") {
		chatID, _ := strconv.Atoi(strings.Split(arg, "_")[1])
		cochat, _ := b.GetChat(int64(chatID), nil)
		go db.ConnectId(user.Id, cochat.Id)

		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		Text, _ := tr.GetString("helpers_connected_to_chat", i18n.TranslationParams{"s": cochat.Title})
		connKeyboard := helpers.InitButtons(b, cochat.Id, user.Id)

		_, err := ctx.EffectiveMessage.Reply(b, Text,
			&gotgbot.SendMessageOpts{
				ReplyMarkup: connKeyboard,
			},
		)
		if err != nil {
			log.Error(err)
			return err
		}
	} else if strings.HasPrefix(arg, "rules_") {
		chatID, _ := strconv.Atoi(strings.Split(arg, "_")[1])
		chatinfo, _ := b.GetChat(int64(chatID), nil)
		rulesrc := db.GetChatRulesInfo(int64(chatID))

		if rulesrc.Rules == "" {
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			text, _ := tr.GetString("rules_not_set")
			_, err := msg.Reply(b, text, helpers.Shtml())
			if err != nil {
				log.Error(err)
				return err
			}
			return ext.EndGroups
		}

		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("rules_for_chat", i18n.TranslationParams{
			"first":  chatinfo.Title,
			"second": rulesrc.Rules,
		})
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
	} else if strings.HasPrefix(arg, "note") {
		nArgs := strings.SplitN(arg, "_", 3)
		chatID, _ := strconv.Atoi(nArgs[1])
		chatinfo, _ := b.GetChat(int64(chatID), nil)

		if strings.HasPrefix(arg, "notes_") {
			// check if feth admin notes or not
			admin := chat_status.IsUserAdmin(b, int64(chatID), user.Id)
			noteKeys := db.GetNotesList(chatinfo.Id, admin)
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			info, _ := tr.GetString("notes_none_in_chat")
			if len(noteKeys) > 0 {
				info, _ = tr.GetString("helpers_notes_current_header")
				for _, note := range noteKeys {
					info += fmt.Sprintf(" - <a href='https://t.me/%s?start=note_%d_%s'>%s</a>\n", b.Username, int64(chatID), note, note)
				}
			}

			_, err := msg.Reply(b, info, helpers.Shtml())
			if err != nil {
				log.Error(err)
				return err
			}
		} else if strings.HasPrefix(arg, "note_") {
			noteName := strings.ToLower(nArgs[2])
			noteData := db.GetNote(chatinfo.Id, noteName)
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			if noteData == nil {
				text, _ := tr.GetString("helpers_note_not_exist")
				_, err := msg.Reply(b, text, helpers.Shtml())
				if err != nil {
					log.Error(err)
					return err
				}
				return ext.EndGroups
			}
			if noteData.AdminOnly {
				if !chat_status.IsUserAdmin(b, int64(chatID), user.Id) {
					text, _ := tr.GetString("helpers_note_admin_only")
					_, err := msg.Reply(b, text, helpers.Shtml())
					if err != nil {
						log.Error(err)
						return err
					}
					return ext.ContinueGroups
				}
			}
			_chat := chatinfo.ToChat() // need to convert to chat
			_, err := helpers.SendNote(b, &_chat, ctx, noteData, msg.MessageId)
			if err != nil {
				log.Error(err)
				return err
			}
		}
	} else if arg == "about" {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		aboutText := getAboutText(tr)
		aboutKb := getAboutKb(tr)
		_, err := b.SendMessage(chat.Id,
			aboutText,
			&gotgbot.SendMessageOpts{
				ParseMode: "Markdown",
				LinkPreviewOptions: &gotgbot.LinkPreviewOptions{
					IsDisabled: true,
				},
				ReplyParameters: &gotgbot.ReplyParameters{
					MessageId:                msg.MessageId,
					AllowSendingWithoutReply: true,
				},
				ReplyMarkup: &aboutKb,
			},
		)
		if err != nil {
			log.Error(err)
			return err
		}
	} else {
		// This sends the normal help block
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		startHelpText := getStartHelp(tr)
		startMarkupKb := getStartMarkup(tr)
		_, err := b.SendMessage(chat.Id,
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
	}

	return ext.EndGroups
}

// getAltNamesOfModule returns all alternative names for a given module.
// Provides a list of aliases that can be used to reference the module in commands.
func getAltNamesOfModule(moduleName string) []string {
	tr := i18n.MustNewTranslator("config")
	altNamesFromConfig, _ := tr.GetStringSlice(fmt.Sprintf("alt_names.%s", moduleName))
	return append(altNamesFromConfig, strings.ToLower(moduleName))
}

// getHelpTextAndMarkup generates help content and keyboard for a module or main help.
// Returns appropriate help text, navigation markup, and parse mode based on module request.
func getHelpTextAndMarkup(ctx *ext.Context, module string) (helpText string, kbmarkup gotgbot.InlineKeyboardMarkup, _parsemode string) {
	var moduleName string
	userOrGroupLanguage := db.GetLanguage(ctx)

	for _, ModName := range listModules() {
		// add key as well to this array
		altnames := getAltNamesOfModule(ModName)

		if string_handling.FindInStringSlice(altnames, module) {
			moduleName = ModName
			break
		}
	}

	// compare and check if module name is not empty
	if moduleName != "" {
		_parsemode = helpers.Markdown
		helpText, kbmarkup = getModuleHelpAndKb(moduleName, userOrGroupLanguage)
	} else {
		_parsemode = helpers.HTML
		tr := i18n.MustNewTranslator(userOrGroupLanguage)
		helpText = getMainHelp(tr, html.EscapeString(ctx.EffectiveUser.FirstName))
		kbmarkup = markup
	}

	return
}
