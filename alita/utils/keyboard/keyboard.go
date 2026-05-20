// Package keyboard provides utilities for building Telegram inline keyboards.
package keyboard

import (
	"fmt"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/callbackcodec"
)

// GetLangFormat returns a formatted language display string with name and flag emoji.
// Uses i18n system to get localized language name and flag for the given language code.
func GetLangFormat(langCode string) string {
	tr := i18n.MustNewTranslator(langCode)
	langName, _ := tr.GetString("language_name")
	langFlag, _ := tr.GetString("language_flag")
	return langName + " " + langFlag
}

// BuildKeyboard constructs an inline keyboard from a slice of database button objects.
// Handles button grouping based on the SameLine property for proper layout.
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

// ChunkKeyboardSlices splits a slice of inline keyboard buttons into chunks of specified size.
// Used for creating organized help menu keyboards with consistent row layouts.
func ChunkKeyboardSlices(slice []gotgbot.InlineKeyboardButton, chunkSize int) (chunks [][]gotgbot.InlineKeyboardButton) {
	if chunkSize <= 0 {
		return nil
	}
	for len(slice) > 0 {
		if len(slice) < chunkSize {
			chunkSize = len(slice)
		}

		chunks = append(chunks, slice[0:chunkSize])
		slice = slice[chunkSize:]
	}
	return
}

// MakeLanguageKeyboard creates an inline keyboard with all available language options.
// Uses valid language codes from config and chunks them into 2-column layout.
func MakeLanguageKeyboard() [][]gotgbot.InlineKeyboardButton {
	var kb []gotgbot.InlineKeyboardButton

	for _, langCode := range config.AppConfig.ValidLangCodes {
		properLang := GetLangFormat(langCode)
		if properLang == "" || properLang == " " {
			continue
		}

		kb = append(
			kb,
			gotgbot.InlineKeyboardButton{
				Text:         properLang,
				CallbackData: callbackcodec.EncodeOrFallback("change_language", map[string]string{"l": langCode}, fmt.Sprintf("change_language.%s", langCode)),
			},
		)
	}

	return ChunkKeyboardSlices(kb, 2)
}
