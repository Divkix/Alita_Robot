package modules

import (
	"fmt"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/decorators/cmdDecorator"
)

var formattingModule = moduleStruct{moduleName: "Formatting"}

// markdownHelp provides markdown formatting help and examples to users.
// Shows formatting options in private messages or sends a button to open help in PM.
func (m moduleStruct) markdownHelp(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	// Check of group or pm
	if !chat_status.RequirePrivate(b, ctx, nil, true) {
		reply := msg.Reply
		if msg.ReplyToMessage != nil {
			reply = msg.ReplyToMessage.Reply
		}

		markdownHelpText, _ := tr.GetString("help_markdown_help_button")
		pressButtonText, _ := tr.GetString("formatting_press_button")

		_, err := reply(b,
			pressButtonText,
			&gotgbot.SendMessageOpts{
				ParseMode: helpers.HTML,
				ReplyMarkup: gotgbot.InlineKeyboardMarkup{
					InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
						{
							{
								Text: markdownHelpText,
								Url:  fmt.Sprintf("https://t.me/%s?start=help_formatting", b.Username),
							},
						},
					},
				},
			},
		)
		if err != nil {
			log.Error(err)
			return err
		}
	} else {
		backText, _ := tr.GetString("common_back")

		// Keyboard for markdown help menu
		Mkdkb := append(m.genFormattingKb(db.GetLanguage(ctx)),
			[]gotgbot.InlineKeyboardButton{
				{
					Text:         backText,
					CallbackData: encodeCallbackData("helpq", map[string]string{"m": "Help"}, "helpq.Help"),
				},
			},
		)

		largeOptionsText, _ := tr.GetString("formatting_large_options")
		_, err := msg.Reply(b,
			// Uses localized largeOptionsText for the formatting help message.
			largeOptionsText,
			&gotgbot.SendMessageOpts{
				ParseMode: helpers.HTML,
				ReplyMarkup: gotgbot.InlineKeyboardMarkup{
					InlineKeyboard: Mkdkb,
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

// genFormattingKb generates the inline keyboard layout for formatting help options.
// Creates buttons for different formatting categories like markdown, fillings, and random content.
// If lang is empty, defaults to "en".
func (moduleStruct) genFormattingKb(lang string) [][]gotgbot.InlineKeyboardButton {
	if lang == "" {
		lang = "en"
	}

	tr := i18n.MustNewTranslator(lang)

	keyboard := [][]gotgbot.InlineKeyboardButton{
		make([]gotgbot.InlineKeyboardButton, 2),
		make([]gotgbot.InlineKeyboardButton, 1),
	}

	markdownFormattingText, _ := tr.GetString("help_markdown_formatting_button")
	fillingsText, _ := tr.GetString("help_fillings_button")
	randomContentText, _ := tr.GetString("help_random_content_button")

	// First row
	keyboard[0][0] = gotgbot.InlineKeyboardButton{
		Text:         markdownFormattingText,
		CallbackData: encodeCallbackData("formatting", map[string]string{"m": "md_formatting"}, "formatting.md_formatting"),
	}
	keyboard[0][1] = gotgbot.InlineKeyboardButton{
		Text:         fillingsText,
		CallbackData: encodeCallbackData("formatting", map[string]string{"m": "fillings"}, "formatting.fillings"),
	}

	// Second Row
	keyboard[1][0] = gotgbot.InlineKeyboardButton{
		Text:         randomContentText,
		CallbackData: encodeCallbackData("formatting", map[string]string{"m": "random"}, "formatting.random"),
	}

	return keyboard
}

// getMarkdownHelp retrieves the appropriate help text for a specific formatting module.
// Returns localized help content based on the requested formatting category.
func (moduleStruct) getMarkdownHelp(tr *i18n.Translator, module string) string {
	var helpTxt string
	switch module {
	case "md_formatting":
		helpTxt, _ = tr.GetString("formatting_markdown")
	case "fillings":
		helpTxt, _ = tr.GetString("formatting_fillings")
	case "random":
		helpTxt, _ = tr.GetString("formatting_random")
	}
	return helpTxt
}

// formattingHandler processes callback queries for formatting help navigation.
// Updates help messages based on user selections from the formatting keyboard.
func (m moduleStruct) formattingHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	query := ctx.CallbackQuery
	msg := query.Message
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	module := ""
	if decoded, ok := decodeCallbackData(query.Data, "formatting"); ok {
		module, _ = decoded.Field("m")
	} else {
		parts := strings.Split(query.Data, ".")
		if len(parts) >= 2 {
			module = parts[1]
		}
	}
	if module == "" {
		log.Warnf("[Formatting] Invalid callback data format: %s", query.Data)
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "Invalid request."})
		return ext.EndGroups
	}

	backText, _ := tr.GetString("common_back")

	// Edit the help as per sub-module selected in markdownhelp
	opts := &gotgbot.EditMessageTextOpts{
		MessageId: msg.GetMessageId(),
		ReplyMarkup: gotgbot.InlineKeyboardMarkup{
			InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
				{
					{
						Text:         backText,
						CallbackData: encodeCallbackData("helpq", map[string]string{"m": "Formatting"}, "helpq.Formatting"),
					},
				},
			},
		},
		ParseMode: helpers.HTML,
	}
	_, _, err := msg.EditText(b, m.getMarkdownHelp(tr, module), opts)
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

// LoadMkdCmd registers markdown and formatting command handlers with the dispatcher.
// Sets up help commands and callback handlers for formatting assistance.
func LoadMkdCmd(dispatcher *ext.Dispatcher) {
	HelpModule.AbleMap.Store(formattingModule.moduleName, true)
	HelpModule.helpableKb[formattingModule.moduleName] = formattingModule.genFormattingKb("en")
	cmdDecorator.MultiCommand(dispatcher, []string{"markdownhelp", "formatting"}, formattingModule.markdownHelp)
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("formatting"), formattingModule.formattingHandler))
}
