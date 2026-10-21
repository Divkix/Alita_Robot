package keyboard

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/callbackcodec"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
)

func GetLangFormat(langCode string) string {
	tr := i18n.MustNewTranslator(langCode)
	langName, _ := tr.GetString("language_name")
	langFlag, _ := tr.GetString("language_flag")
	return langName + " " + langFlag
}

// UnrestrictButton builds the callback button that lifts a mute or ban.
func UnrestrictButton(tr *i18n.Translator, labelKey, action string, userID int64) gotgbot.InlineKeyboardButton {
	label, _ := tr.GetString(labelKey)
	data, _ := callbackcodec.Encode("unrestrict", map[string]string{
		"a": action,
		"u": strconv.FormatInt(userID, 10),
	})
	return gotgbot.InlineKeyboardButton{Text: label, CallbackData: data}
}

func BuildKeyboard(buttons []db.Button) [][]gotgbot.InlineKeyboardButton {
	keyb := make([][]gotgbot.InlineKeyboardButton, 0)
	for _, btn := range buttons {
		if btn.SameLine && len(keyb) > 0 {
			keyb[len(keyb)-1] = append(keyb[len(keyb)-1], gotgbot.InlineKeyboardButton{Text: btn.Name, Url: btn.Url})
		} else {
			k := make([]gotgbot.InlineKeyboardButton, 1)
			k[0] = gotgbot.InlineKeyboardButton{Text: btn.Name, Url: btn.Url}
			keyb = append(keyb, k)
		}
	}
	return keyb
}

func MakeLanguageKeyboard() [][]gotgbot.InlineKeyboardButton {
	return makeLanguageKeyboard(0)
}

// MakeGroupLanguageKeyboard builds the language picker used by /lang group in
// PM: its buttons carry g=1 and the group id c so the callback changes that
// group's language instead of the user's.
func MakeGroupLanguageKeyboard(chatID int64) [][]gotgbot.InlineKeyboardButton {
	return makeLanguageKeyboard(chatID)
}

func makeLanguageKeyboard(groupID int64) [][]gotgbot.InlineKeyboardButton {
	var kb []gotgbot.InlineKeyboardButton

	for _, langCode := range config.AppConfig.ValidLangCodes {
		properLang := GetLangFormat(langCode)
		if properLang == "" || properLang == " " {
			continue
		}

		fields := map[string]string{"l": langCode}
		if groupID != 0 {
			fields["g"] = "1"
			fields["c"] = strconv.FormatInt(groupID, 10)
		}
		kb = append(
			kb,
			gotgbot.InlineKeyboardButton{
				Text:         properLang,
				CallbackData: callbackcodec.EncodeOrFallback("change_language", fields, fmt.Sprintf("change_language.%s", langCode)),
			},
		)
	}

	return slices.Collect(slices.Chunk(kb, 2))
}

// InitButtons builds the connection keyboard with labels from tr, the
// translator of the chat the reply goes to; a nil tr falls back to English.
func InitButtons(b *gotgbot.Bot, tr *i18n.Translator, chatId, userId int64) gotgbot.InlineKeyboardMarkup {
	if tr == nil {
		tr = i18n.MustNewTranslator("en")
	}
	adminText, _ := tr.GetString("helpers_admin_commands")
	if adminText == "" {
		adminText = "Admin commands"
	}
	userText, _ := tr.GetString("helpers_user_commands")
	if userText == "" {
		userText = "User commands"
	}

	var connButtons [][]gotgbot.InlineKeyboardButton
	if chat_status.IsUserAdmin(b, chatId, userId) {
		connButtons = [][]gotgbot.InlineKeyboardButton{
			{
				{
					Text:         adminText,
					CallbackData: callbackcodec.EncodeOrFallback("connbtns", map[string]string{"t": "Admin"}, "connbtns.Admin"),
				},
			},
			{
				{
					Text:         userText,
					CallbackData: callbackcodec.EncodeOrFallback("connbtns", map[string]string{"t": "User"}, "connbtns.User"),
				},
			},
		}
	} else {
		connButtons = [][]gotgbot.InlineKeyboardButton{
			{
				{
					Text:         userText,
					CallbackData: callbackcodec.EncodeOrFallback("connbtns", map[string]string{"t": "User"}, "connbtns.User"),
				},
			},
		}
	}
	connKeyboard := gotgbot.InlineKeyboardMarkup{InlineKeyboard: connButtons}
	return connKeyboard
}
