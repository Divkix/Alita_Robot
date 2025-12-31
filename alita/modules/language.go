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

	// Validate callback data format to prevent index out of bounds panic
	parts := strings.Split(query.Data, ".")
	if len(parts) < 2 {
		log.Warnf("[Language] Invalid callback data format: %s", query.Data)
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
			Text: "Invalid language selection.",
		})
		return ext.EndGroups
	}
	language := parts[1]

	// For group chats, check admin permissions first before any language operations
	if chat.Type != "private" {
		// RequireUserAdmin with justCheck=false already answers the callback with error
		if !chat_status.RequireUserAdmin(b, ctx, chat, user.Id, false) {
			// Permission denied - RequireUserAdmin already showed error via callback answer
			// No need to edit the message or answer again, just return early
			return ext.EndGroups
		}
	}

	// Now we can safely create translator for the target language
	tr := i18n.MustNewTranslator(language)
	var replyString string

	if chat.Type == "private" {
		db.ChangeUserLanguage(user.Id, language)
		replyString, _ = tr.GetString("language_changed_user", i18n.TranslationParams{"s": helpers.GetLangFormat(language)})
	} else {
		// User is admin (already verified above)
		db.ChangeGroupLanguage(chat.Id, language)
		replyString, _ = tr.GetString("language_changed_group", i18n.TranslationParams{"s": helpers.GetLangFormat(language)})
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
