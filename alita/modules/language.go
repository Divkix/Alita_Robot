package modules

import (
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

var languagesModule = moduleStruct{moduleName: "Languages"}

// genFullLanguageKb generates the complete language selection keyboard.
// Creates inline buttons for all available languages plus a translation contribution link.
func (moduleStruct) genFullLanguageKb() [][]gotgbot.InlineKeyboardButton {
	keyboard := helpers.MakeLanguageKeyboard()
	keyboard = append(
		keyboard,
		[]gotgbot.InlineKeyboardButton{
			{
				Text: "Help Us Translate 🌎", // This can stay hardcoded as it's a universal call to action
				Url:  "https://crowdin.com/project/alita_robot",
			},
		},
	)
	return keyboard
}

// changeLanguage displays the language selection menu for users or groups.
// Shows current language and allows users/admins to select a different interface language.
func (m moduleStruct) changeLanguage(b *gotgbot.Bot, ctx *ext.Context) error {
	user := ctx.EffectiveSender.User
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage

	var replyString string

	cLang := db.GetLanguage(ctx)
	tr := i18n.MustNewTranslator(cLang)

	if ctx.Message.Chat.Type == "private" {
		replyString, _ = tr.GetString("language_current_user", i18n.TranslationParams{"s": helpers.GetLangFormat(cLang)})
	} else {

		// language won't be changed if user is not admin
		if !chat_status.RequireUserAdmin(b, ctx, chat, user.Id, false) {
			return ext.EndGroups
		}

		replyString, _ = tr.GetString("language_current_group", i18n.TranslationParams{"s": helpers.GetLangFormat(cLang)})
	}

	_, err := msg.Reply(
		b,
		replyString,
		&gotgbot.SendMessageOpts{
			ReplyMarkup: gotgbot.InlineKeyboardMarkup{
				InlineKeyboard: m.genFullLanguageKb(),
			},
		},
	)
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// langBtnHandler processes language selection callback queries from the language menu.
// Updates user or group language preferences based on admin permissions and context.
func (moduleStruct) langBtnHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	query := ctx.CallbackQuery
	chat := ctx.EffectiveChat
	user := query.From

	var replyString string
	language := strings.Split(query.Data, ".")[1]

	tr := i18n.MustNewTranslator(language)
	if chat.Type == "private" {
		db.ChangeUserLanguage(user.Id, language)
		replyString, _ = tr.GetString("language_changed_user", i18n.TranslationParams{"s": helpers.GetLangFormat(language)})
	} else {
		// Check if user is admin before changing group language
		if !chat_status.RequireUserAdmin(b, ctx, chat, user.Id, false) {
			// Use current language for error message since we can't change it
			currentLang := db.GetLanguage(ctx)
			tr := i18n.MustNewTranslator(currentLang)
			replyString, _ = tr.GetString("language_admin_required", i18n.TranslationParams{})
			if replyString == "" {
				replyString = "You need to be an admin to change the group language."
			}
		} else {
			db.ChangeGroupLanguage(chat.Id, language)
			replyString, _ = tr.GetString("language_changed_group", i18n.TranslationParams{"s": helpers.GetLangFormat(language)})
		}
	}

	// Answer the callback query to stop the loading spinner
	_, err := query.Answer(b, nil)
	if err != nil {
		log.Error(err)
	}

	_, _, err = query.Message.EditText(
		b,
		replyString,
		&gotgbot.EditMessageTextOpts{
			ParseMode: helpers.HTML,
			LinkPreviewOptions: &gotgbot.LinkPreviewOptions{
				IsDisabled: true,
			},
		},
	)
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// LoadLanguage registers language-related command and callback handlers.
// Sets up language selection commands and keyboard navigation for internationalization.
func LoadLanguage(dispatcher *ext.Dispatcher) {
	HelpModule.AbleMap.Store(languagesModule.moduleName, true)
	HelpModule.helpableKb[languagesModule.moduleName] = languagesModule.genFullLanguageKb()

	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("change_language."), languagesModule.langBtnHandler))
	dispatcher.AddHandler(handlers.NewCommand("lang", languagesModule.changeLanguage))
}
