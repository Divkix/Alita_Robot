package modules

import (
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/settings"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	"github.com/divkix/Alita_Robot/alita/utils/keyboard"
	"github.com/divkix/Alita_Robot/alita/utils/tracing"
)

var languagesModule = moduleStruct{moduleName: "Languages"}
var languageGroupSettingChecks = declaredGroupSettingChecks(settings.Language)

func (moduleStruct) genFullLanguageKb(langCode string) [][]gotgbot.InlineKeyboardButton {
	return withHelpTranslateRow(keyboard.MakeLanguageKeyboard(), langCode)
}

// withHelpTranslateRow appends the Crowdin "help translate" link row, labelled in langCode.
func withHelpTranslateRow(kb [][]gotgbot.InlineKeyboardButton, langCode string) [][]gotgbot.InlineKeyboardButton {
	tr := i18n.MustNewTranslator(langCode)
	helpTranslateText, _ := tr.GetString("language_help_translate")
	return append(
		kb,
		[]gotgbot.InlineKeyboardButton{
			{
				Text: helpTranslateText,
				Url:  "https://crowdin.com/project/alita_robot",
			},
		},
	)
}

func (m moduleStruct) changeLanguage(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	var replyString string

	cLang := lang.GetLanguage(ctx)
	if msg.Chat.Type != "private" {
		cLang = settings.Language.ReadContext(tracing.UpdateContext(ctx), ctx.EffectiveChat.Id).Value.(string)
	}
	tr := i18n.MustNewTranslator(cLang)

	langKb := m.genFullLanguageKb(cLang)
	if args := ctx.Args(); msg.Chat.Type == "private" && len(args) > 1 && strings.EqualFold(args[1], "group") {
		group := ctx.EffectiveChat
		// lang.GetLanguage(ctx) answers in the PM's language; name the group's.
		groupLang := settings.Language.ReadContext(tracing.UpdateContext(ctx), group.Id).Value.(string)
		replyString, _ = tr.GetString("language_current_group", i18n.TranslationParams{"s": keyboard.GetLangFormat(groupLang)})
		langKb = withHelpTranslateRow(keyboard.MakeGroupLanguageKeyboard(group.Id), cLang)
	} else if msg.Chat.Type == "private" {
		replyString, _ = tr.GetString("language_current_user", i18n.TranslationParams{"s": keyboard.GetLangFormat(cLang)})
	} else {
		replyString, _ = tr.GetString("language_current_group", i18n.TranslationParams{"s": keyboard.GetLangFormat(cLang)})
	}

	_, err := msg.Reply(
		b,
		replyString,
		&gotgbot.SendMessageOpts{
			ReplyMarkup: gotgbot.InlineKeyboardMarkup{
				InlineKeyboard: langKb,
			},
		},
	)
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func (moduleStruct) langBtnHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	query, ok := callbackQueryFromContext(ctx)
	if !ok {
		return ext.EndGroups
	}
	if query == nil {
		return ext.EndGroups
	}
	chat := ctx.EffectiveChat
	user := query.From
	if chat == nil || user.Id == 0 {
		return answerInvalidCallback(b, ctx, query)
	}

	language, forGroup := "", false
	decoded, ok := decodeCallbackData(query.Data, "change_language")
	if ok {
		language, _ = decoded.Field("l")
		g, _ := decoded.Field("g")
		forGroup = g == "1"
	}
	if language == "" {
		log.Warnf("[Language] Invalid callback data format: %s", query.Data)
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		errText, _ := tr.GetString("language_invalid_selection")
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
			Text: errText,
		})
		return ext.EndGroups
	}
	if settings.Language.Validate(language) != nil {
		log.Warnf("[Language] Unsupported callback language: %s", language)
		currentTr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		errText, _ := currentTr.GetString("language_invalid_selection")
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: errText})
		return ext.EndGroups
	}
	tr := i18n.MustNewTranslator(language)

	if forGroup && chat.Type == "private" {
		// Button from /lang group in PM: change the group it was sent for.
		group := callbackAdminChat(b, ctx, query, decoded, false)
		if group == nil {
			return ext.EndGroups
		}
		chat = group
		// The confirmation edits the admin's PM message: keep their language.
		tr = i18n.MustNewTranslator(lang.GetLanguage(ctx))
	}

	if chat.Type != "private" {
		policyContext := *ctx
		policyContext.EffectiveChat = chat
		command := helpers.CommandContext{Bot: b, Ctx: &policyContext, Chat: chat, User: &user, Tr: tr}
		if !helpers.RunChecks(&command, languageGroupSettingChecks) {
			return ext.EndGroups
		}
	}

	var replyString string

	if chat.Type == "private" {
		if err := lang.ChangeUserLanguage(user.Id, language); err != nil {
			log.Errorf("[Language] ChangeUserLanguage failed for user %d: %v", user.Id, err)
			errText, _ := tr.GetString("common_settings_save_failed")
			_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: errText})
			return ext.EndGroups
		}
		replyString, _ = tr.GetString("language_changed_user", i18n.TranslationParams{"s": keyboard.GetLangFormat(language)})
	} else {
		if err := settings.Language.Write(settings.WriteContext{Context: tracing.UpdateContext(ctx), ChatID: chat.Id, Title: chat.Title, Bot: b}, language); err != nil {
			log.Errorf("[Language] ChangeGroupLanguage failed for chat %d: %v", chat.Id, err)
			errText, _ := tr.GetString("common_settings_save_failed")
			_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: errText})
			return ext.EndGroups
		}
		replyString, _ = tr.GetString("language_changed_group", i18n.TranslationParams{"s": keyboard.GetLangFormat(language)})
	}

	if query.Message == nil {
		_, err := query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: replyString})
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	_, err := query.Answer(b, nil)
	if err != nil {
		log.Error(err)
	}

	_, _, err = query.Message.EditText(b, &gotgbot.EditMessageTextOpts{Text: replyString, ParseMode: formatting.HTML,
		LinkPreviewOptions: &gotgbot.LinkPreviewOptions{
			IsDisabled: true,
		}})
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func LoadLanguage(dispatcher *ext.Dispatcher) {
	SetModuleEnabled(languagesModule.moduleName, true)
	SetModuleHelpBuilder(languagesModule.moduleName, languagesModule.genFullLanguageKb)
	connectedChecks := declaredSettingChecks(settings.Language)
	groupChecks := languageGroupSettingChecks

	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("change_language"), languagesModule.langBtnHandler))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "lang",
		RequiredChecks: []helpers.CheckFunc{checkInPMOrGroup(
			func(c *helpers.CommandContext) bool {
				args := c.Ctx.Args()
				return len(args) == 1 || !strings.EqualFold(args[1], "group") || helpers.RunChecks(c, connectedChecks)
			},
			func(c *helpers.CommandContext) bool {
				return helpers.RunChecks(c, groupChecks)
			},
		)},
	}, pipelineHandler(languagesModule.changeLanguage))
}

func init() {
	RegisterLegacyModule("Languages", 20, LoadLanguage)
}
