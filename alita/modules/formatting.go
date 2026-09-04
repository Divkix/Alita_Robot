package modules

import (
	"fmt"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
)

var formattingModule = moduleStruct{moduleName: "Formatting"}

func (m moduleStruct) markdownHelp(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	if !chat_status.RequirePrivate(b, ctx, nil) {
		reply := msg.Reply
		if msg.ReplyToMessage != nil {
			reply = msg.ReplyToMessage.Reply
		}

		markdownHelpText, _ := tr.GetString("help_markdown_help_button")
		pressButtonText, _ := tr.GetString("formatting_press_button")

		_, err := reply(b,
			pressButtonText,
			&gotgbot.SendMessageOpts{
				ParseMode: formatting.HTML,
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

		Mkdkb := append(m.genFormattingKb(lang.GetLanguage(ctx)),
			[]gotgbot.InlineKeyboardButton{
				{
					Text:         backText,
					CallbackData: encodeCallbackData("helpq", map[string]string{"m": "Help"}),
				},
			},
		)

		largeOptionsText, _ := tr.GetString("formatting_large_options")
		_, err := msg.Reply(b,
			largeOptionsText,
			&gotgbot.SendMessageOpts{
				ParseMode: formatting.HTML,
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

	keyboard[0][0] = gotgbot.InlineKeyboardButton{
		Text:         markdownFormattingText,
		CallbackData: encodeCallbackData("formatting", map[string]string{"m": "md_formatting"}),
	}
	keyboard[0][1] = gotgbot.InlineKeyboardButton{
		Text:         fillingsText,
		CallbackData: encodeCallbackData("formatting", map[string]string{"m": "fillings"}),
	}

	keyboard[1][0] = gotgbot.InlineKeyboardButton{
		Text:         randomContentText,
		CallbackData: encodeCallbackData("formatting", map[string]string{"m": "random"}),
	}

	return keyboard
}

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

func (m moduleStruct) formattingHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	query, ok := callbackQueryFromContext(ctx)
	if !ok {
		return ext.EndGroups
	}
	msg := query.Message
	if msg == nil {
		return answerInvalidCallback(b, ctx, query)
	}
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	module := ""
	if decoded, ok := decodeCallbackData(query.Data, "formatting"); ok {
		module, _ = decoded.Field("m")
	}
	if module == "" {
		log.Warnf("[Formatting] Invalid callback data format: %s", query.Data)
		return answerInvalidCallback(b, ctx, query)
	}
	switch module {
	case "md_formatting", "fillings", "random":
	default:
		return answerInvalidCallback(b, ctx, query)
	}

	backText, _ := tr.GetString("common_back")

	opts := &gotgbot.EditMessageTextOpts{
		MessageId: msg.GetMessageId(),
		ReplyMarkup: gotgbot.InlineKeyboardMarkup{
			InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
				{
					{
						Text:         backText,
						CallbackData: encodeCallbackData("helpq", map[string]string{"m": "Formatting"}),
					},
				},
			},
		},
		ParseMode: formatting.HTML,
	}
	opts.Text = m.getMarkdownHelp(tr, module)
	_, _, err := msg.EditText(b, opts)
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

func LoadMkdCmd(dispatcher *ext.Dispatcher) {
	DefaultHelpRegistry().AbleMap[formattingModule.moduleName] = true
	DefaultHelpRegistry().helpableKb[formattingModule.moduleName] = formattingModule.genFormattingKb("en")
	helpers.MultiCommand(dispatcher, []string{"markdownhelp", "formatting"}, formattingModule.markdownHelp)
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("formatting"), formattingModule.formattingHandler))
}

func init() {
	RegisterLegacyModule("Formatting", 260, LoadMkdCmd)
}
