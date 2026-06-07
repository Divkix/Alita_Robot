package modules

import (
	"fmt"
	"html"
	"slices"
	"strings"

	tgmd2html "github.com/PaulSonOfLars/gotg_md2html"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/keyboard"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// markup is the global help menu keyboard.
var markup gotgbot.InlineKeyboardMarkup

// listModules returns a sorted slice of all currently enabled bot modules.
func listModules() []string {
	return listModulesFrom(DefaultHelpRegistry())
}

func listModulesFrom(registry *moduleStruct) []string {
	modules := registry.AbleMap.LoadModules()
	slices.Sort(modules)
	return modules
}

// initHelpButtons initializes the help menu keyboard with all enabled modules.
func initHelpButtons() {
	markup = initHelpButtonsFrom(DefaultHelpRegistry())
}

// initHelpButtonsFrom builds a help menu keyboard from the given registry.
func initHelpButtonsFrom(registry *moduleStruct) gotgbot.InlineKeyboardMarkup {
	var kb []gotgbot.InlineKeyboardButton

	for _, i := range listModulesFrom(registry) {
		kb = append(kb, gotgbot.InlineKeyboardButton{
			Text: i,
			CallbackData: encodeCallbackData("helpq", map[string]string{"m": i},
				fmt.Sprintf("helpq.%s", i),
			),
		})
	}
	zb := keyboard.ChunkKeyboardSlices(kb, 3)
	tr := i18n.MustNewTranslator("en")
	backText, _ := tr.GetString("helpers_back_button")
	zb = append(zb, []gotgbot.InlineKeyboardButton{{
		Text:         backText,
		CallbackData: encodeCallbackData("helpq", map[string]string{"m": "BackStart"}, "helpq.BackStart"),
	}})
	return gotgbot.InlineKeyboardMarkup{InlineKeyboard: zb}
}

// getModuleHelpAndKb retrieves help text and keyboard for a specific module.
func getModuleHelpAndKb(module, lang string, registry *moduleStruct) (helpText string, replyMarkup gotgbot.InlineKeyboardMarkup) {
	ModName := cases.Title(language.English).String(module)
	tr := i18n.MustNewTranslator(lang)
	helpMsg, _ := tr.GetString(fmt.Sprintf("%s_help_msg", strings.ToLower(ModName)))
	headerTemplate, _ := tr.GetString("helpers_module_help_header")
	helpText = tgmd2html.MD2HTMLV2(fmt.Sprintf(headerTemplate, ModName) + helpMsg)

	backText, _ := tr.GetString("common_back_arrow")
	homeText, _ := tr.GetString("common_home")
	backBtnSuffix := []gotgbot.InlineKeyboardButton{
		{
			Text:         backText,
			CallbackData: encodeCallbackData("helpq", map[string]string{"m": "Help"}, "helpq.Help"),
		},
		{
			Text:         homeText,
			CallbackData: encodeCallbackData("helpq", map[string]string{"m": "BackStart"}, "helpq.BackStart"),
		},
	}

	kb := slices.Clone(registry.helpableKb[ModName])
	replyMarkup = gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: append(kb, backBtnSuffix),
	}
	return
}

// sendHelpkb sends help information for a specific module with navigation keyboard.
func sendHelpkb(b *gotgbot.Bot, ctx *ext.Context, module string, registry *moduleStruct) (msg *gotgbot.Message, err error) {
	module = strings.ToLower(module)
	if module == "help" {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		helpText := getMainHelp(tr, html.EscapeString(ctx.EffectiveMessage.From.FirstName))
		_, err = b.SendMessage(
			ctx.EffectiveMessage.Chat.Id,
			helpText,
			&gotgbot.SendMessageOpts{
				ParseMode:   formatting.HTML,
				ReplyMarkup: &markup,
			},
		)
		return
	}
	helpText, replyMarkup, _parsemode := getHelpTextAndMarkup(ctx, module, registry)

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
func getModuleNameFromAltName(altName string, registry *moduleStruct) string {
	for _, modName := range listModulesFrom(registry) {
		altNames := getAltNamesOfModule(modName)
		for _, altNameInSlice := range altNames {
			if altNameInSlice == altName {
				return modName
			}
		}
	}
	return ""
}

// getAltNamesOfModule returns all alternative names for a given module.
func getAltNamesOfModule(moduleName string) []string {
	tr := i18n.MustNewTranslator("config")
	altNamesFromConfig, _ := tr.GetStringSlice(fmt.Sprintf("alt_names.%s", moduleName))
	return append(altNamesFromConfig, strings.ToLower(moduleName))
}

// getHelpTextAndMarkup generates help content and keyboard for a module or main help.
func getHelpTextAndMarkup(ctx *ext.Context, module string, registry *moduleStruct) (helpText string, kbmarkup gotgbot.InlineKeyboardMarkup, _parsemode string) {
	var moduleName string
	userOrGroupLanguage := lang.GetLanguage(ctx)

	for _, ModName := range listModulesFrom(registry) {
		altnames := getAltNamesOfModule(ModName)
		if slices.Contains(altnames, module) {
			moduleName = ModName
			break
		}
	}

	if moduleName != "" {
		_parsemode = formatting.HTML
		helpText, kbmarkup = getModuleHelpAndKb(moduleName, userOrGroupLanguage, registry)
	} else {
		_parsemode = formatting.HTML
		tr := i18n.MustNewTranslator(userOrGroupLanguage)
		helpText = getMainHelp(tr, html.EscapeString(ctx.EffectiveUser.FirstName))
		kbmarkup = initHelpButtonsFrom(registry)
	}

	return
}
