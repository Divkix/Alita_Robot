package modules

import (
	"fmt"
	"html"
	"slices"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	log "github.com/sirupsen/logrus"

	db_filters "github.com/divkix/Alita_Robot/alita/db/filters"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/i18n"

	"github.com/divkix/Alita_Robot/alita/utils/content"
	"github.com/divkix/Alita_Robot/alita/utils/extraction"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	"github.com/divkix/Alita_Robot/alita/utils/media"

	"github.com/divkix/Alita_Robot/alita/utils/keyword_matcher"
)

var filtersModule = moduleStruct{
	moduleName:   "Filters",
	handlerGroup: 9,
}

func filterOverwriteCacheKey(token string) string {
	return overwriteCacheKey("filter", token)
}

func setFilterOverwriteCache(token string, data overwriteFilter) error {
	return setOverwriteCache(filterOverwriteCacheKey(token), data)
}

func consumeFilterOverwriteCache(token string) (*overwriteFilter, error) {
	return consumeOverwriteCache[overwriteFilter](filterOverwriteCacheKey(token))
}

func deleteFilterOverwriteCache(token string) {
	deleteOverwriteCache(filterOverwriteCacheKey(token))
}

//nolint:dupl // addFilter shares validation logic with notes module by design
func (m moduleStruct) addFilter(b *gotgbot.Bot, ctx *ext.Context) error {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("[Filters][addFilter] Recovered from panic: %v", r)
		}
	}()
	msg := ctx.EffectiveMessage
	connectedChat := chat_status.IsUserConnected(b, ctx, true, false)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	args := ctx.Args()

	if !chat_status.CanUserChangeInfo(b, ctx, chat, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_change_info_cmd_error", "chat_status_change_info_button_error")
		return ext.EndGroups
	}

	filtersNum := db_filters.CountFilters(chat.Id)
	if filtersNum >= 150 {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("filters_limit_exceeded")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}

		return ext.EndGroups
	}

	if msg.ReplyToMessage != nil && len(args) <= 1 {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("filters_keyword_required")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	} else if len(args) <= 2 && msg.ReplyToMessage == nil {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("filters_invalid")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	result := content.ExtractNoteAndFilter(msg, true, lang.GetLanguage(ctx))
	filterWord, fileid, text, dataType, buttons, errorMsg := result.KeyWord, result.FileID, result.Text, result.DataType, result.Buttons, result.ErrorMsg
	if dataType == -1 {
		_, err := msg.Reply(b, errorMsg, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	filterWord = strings.ToLower(filterWord)

	if len([]rune(filterWord)) > 100 {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("filters_keyword_too_long")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if db_filters.DoesFilterExists(chat.Id, filterWord) {
		token, tokenErr := newOverwriteToken()
		if tokenErr != nil {
			log.Errorf("[Filters] Failed to generate overwrite token: %v", tokenErr)
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			errorText, _ := tr.GetString("filters_overwrite_token_failed")
			_, _ = msg.Reply(b, errorText, formatting.Shtml())
			return ext.EndGroups
		}

		err := setFilterOverwriteCache(token, overwriteFilter{
			overwriteBase: overwriteBase{
				ChatID:   chat.Id,
				UserID:   user.Id,
				ItemName: filterWord,
				Text:     text,
				FileID:   fileid,
				Buttons:  buttons,
				DataType: dataType,
			},
		})
		if err != nil {
			log.Errorf("[Filters] Failed to cache overwrite data: %v", err)
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			errorText, _ := tr.GetString("filters_overwrite_token_failed")
			_, _ = msg.Reply(b, errorText, formatting.Shtml())
			return ext.EndGroups
		}

		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		confirmText, _ := tr.GetString("filters_overwrite_confirm")
		yesText, _ := tr.GetString("common_yes")
		noText, _ := tr.GetString("common_no")
		_, err = msg.Reply(b,
			confirmText,
			&gotgbot.SendMessageOpts{
				ParseMode: formatting.HTML,
				ReplyMarkup: gotgbot.InlineKeyboardMarkup{
					InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
						{
							{
								Text: yesText,
								CallbackData: encodeCallbackData("filters_overwrite", map[string]string{
									"a": "yes",
									"t": token,
								}),
							},
							{
								Text: noText,
								CallbackData: encodeCallbackData("filters_overwrite", map[string]string{
									"a": "cancel",
									"t": token,
								}),
							},
						},
					},
				},
			},
		)
		if err != nil {
			deleteFilterOverwriteCache(token)
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if err := db_filters.AddFilter(chat.Id, filterWord, text, fileid, buttons, dataType); err != nil {
		log.Errorf("[Filters] AddFilter failed for chat %d: %v", chat.Id, err)
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		errText, _ := tr.GetString("common_settings_save_failed")
		_, _ = msg.Reply(b, errText, formatting.Shtml())
		return ext.EndGroups
	}

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	successText, _ := tr.GetString("filters_added_success")
	_, err := msg.Reply(b, fmt.Sprintf(successText, filterWord), formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func (moduleStruct) rmFilter(b *gotgbot.Bot, ctx *ext.Context) error {
	connectedChat := chat_status.IsUserConnected(b, ctx, true, false)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	args := ctx.Args()[1:]

	if !chat_status.CanUserChangeInfo(b, ctx, chat, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_change_info_cmd_error", "chat_status_change_info_button_error")
		return ext.EndGroups
	}

	if len(args) == 0 {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("filters_remove_keyword_required")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
	} else {

		filterWord, _ := extraction.ExtractQuotes(strings.Join(args, " "), true, true)

		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		if !slices.Contains(db_filters.GetFiltersList(chat.Id), strings.ToLower(filterWord)) {
			text, _ := tr.GetString("filters_not_exists")
			_, err := msg.Reply(b, text, formatting.Shtml())
			if err != nil {
				log.Error(err)
				return err
			}
		} else {
			if err := db_filters.RemoveFilter(chat.Id, strings.ToLower(filterWord)); err != nil {
				log.Errorf("[Filters] RemoveFilter failed for chat %d: %v", chat.Id, err)
				errText, _ := tr.GetString("common_settings_save_failed")
				_, _ = msg.Reply(b, errText, formatting.Shtml())
				return ext.EndGroups
			}
			successText, _ := tr.GetString("filters_removed_success")
			_, err := msg.Reply(b, fmt.Sprintf(successText, filterWord), formatting.Shtml())
			if err != nil {
				log.Error(err)
				return err
			}
		}
	}
	return ext.EndGroups
}

func (moduleStruct) filtersList(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	if chat_status.CheckDisabledCmd(b, msg, "filters") {
		return ext.EndGroups
	}
	connectedChat := chat_status.IsUserConnected(b, ctx, false, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat

	var replyMsgId int64

	if reply := msg.ReplyToMessage; reply != nil {
		replyMsgId = reply.MessageId
	} else {
		replyMsgId = msg.MessageId
	}

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	filterKeys := db_filters.GetFiltersList(chat.Id)
	info, _ := tr.GetString("filters_none_in_chat")
	newFilterKeys := make([]string, 0, len(filterKeys))

	for _, fkey := range filterKeys {
		newFilterKeys = append(newFilterKeys, fmt.Sprintf("<code>%s</code>", html.EscapeString(fkey)))
	}

	if len(newFilterKeys) > 0 {
		info, _ = tr.GetString("filters_current_in_chat")
		info += "\n - " + strings.Join(newFilterKeys, "\n - ")
	}

	_, err := msg.Reply(b,
		info,
		&gotgbot.SendMessageOpts{
			ParseMode: formatting.HTML,
			ReplyParameters: &gotgbot.ReplyParameters{
				MessageId:                replyMsgId,
				AllowSendingWithoutReply: true,
			},
		},
	)
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

//nolint:dupl // rmAllFilters shares confirmation pattern with notes module by design
func (moduleStruct) rmAllFilters(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}
	msg := ctx.EffectiveMessage
	filterKeys := db_filters.GetFiltersList(chat.Id)

	if len(filterKeys) == 0 {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("filters_none_in_chat")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}

		return ext.EndGroups
	}

	if chat_status.RequireUserOwner(b, ctx, chat, user.Id) {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		confirmText, _ := tr.GetString("filters_clear_all_confirm")
		yesText, _ := tr.GetString("common_yes")
		noText, _ := tr.GetString("common_no")
		_, err := msg.Reply(b, confirmText,
			&gotgbot.SendMessageOpts{
				ReplyMarkup: gotgbot.InlineKeyboardMarkup{
					InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
						{
							{
								Text:         yesText,
								CallbackData: encodeCallbackData("rmAllFilters", map[string]string{"a": "yes"}),
							},
							{
								Text:         noText,
								CallbackData: encodeCallbackData("rmAllFilters", map[string]string{"a": "no"}),
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
	}

	return ext.EndGroups
}

func (moduleStruct) filtersButtonHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	query, ok := callbackQueryFromContext(ctx)
	if !ok {
		return ext.EndGroups
	}
	user := query.From
	chat := ctx.EffectiveChat
	if chat == nil {
		return ext.EndGroups
	}

	if !chat_status.RequireUserOwner(b, ctx, nil, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_owner_cmd_error", "chat_status_owner_button_error", chat_status.WithReply())
		return ext.EndGroups
	}

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	response := ""
	if decoded, ok := decodeCallbackData(query.Data, "rmAllFilters"); ok {
		response, _ = decoded.Field("a")
	}
	if response == "" {
		log.Warnf("[Filters] Invalid callback data format: %s", query.Data)
		return answerInvalidCallback(b, ctx, query)
	}
	var helpText string

	switch response {
	case "yes":
		if err := db_filters.RemoveAllFilters(chat.Id); err != nil {
			helpText, _ = tr.GetString("filters_clear_all_failed")
			if helpText == "" {
				helpText = "Failed to remove all Filters from this Chat ❌"
			}
		} else {
			helpText, _ = tr.GetString("filters_clear_all_success")
		}
	case "no":
		helpText, _ = tr.GetString("filters_clear_all_cancelled")
	default:
		return answerInvalidCallback(b, ctx, query)
	}

	if query.Message == nil {
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: helpText})
		return ext.EndGroups
	}

	_, _, err := query.Message.EditText(b, &gotgbot.EditMessageTextOpts{Text: helpText})
	if err != nil {
		log.Error(err)
		return err
	}

	_, err = query.Answer(b,
		&gotgbot.AnswerCallbackQueryOpts{
			Text: helpText,
		},
	)
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func (m moduleStruct) filterOverWriteHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	query, ok := callbackQueryFromContext(ctx)
	if !ok {
		return ext.EndGroups
	}
	user := query.From
	chat := ctx.EffectiveChat
	if chat == nil {
		return ext.EndGroups
	}

	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
		return ext.EndGroups
	}

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	action, token, ok := parseOverwriteCallbackData(query.Data, "filters_overwrite")
	if !ok {
		log.Error("[Filters] Invalid callback data format")
		return ext.EndGroups
	}
	if action != "yes" && action != "cancel" {
		log.WithField("action", action).Warn("[Filters] Invalid overwrite action")
		return answerInvalidCallback(b, ctx, query)
	}
	var helpText string

	if action == "cancel" {
		if token != "" {
			if data, err := consumeFilterOverwriteCache(token); err == nil {
				if data.UserID != 0 && data.UserID != user.Id {
					helpText, _ = tr.GetString("filters_overwrite_expired")
					if query.Message != nil {
						_, _, _ = query.Message.EditText(b, &gotgbot.EditMessageTextOpts{Text: helpText})
					}
					_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: helpText})
					return ext.EndGroups
				}
				if data.ChatID != 0 && data.ChatID != chat.Id {
					helpText, _ = tr.GetString("filters_overwrite_expired")
					if query.Message != nil {
						_, _, _ = query.Message.EditText(b, &gotgbot.EditMessageTextOpts{Text: helpText})
					}
					_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: helpText})
					return ext.EndGroups
				}
			}
		}
		helpText, _ = tr.GetString("filters_overwrite_cancelled")
		if query.Message != nil {
			_, _, editErr := query.Message.EditText(b, &gotgbot.EditMessageTextOpts{Text: helpText})
			if editErr != nil {
				log.Error(editErr)
			}
		}
		_, answerErr := query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: helpText})
		if answerErr != nil {
			log.Error(answerErr)
		}
		return ext.EndGroups
	}

	filterData, err := consumeFilterOverwriteCache(token)
	if err != nil || filterData == nil {
		log.Debugf("[Filters] Failed to retrieve overwrite data from cache: %v", err)
		helpText, _ = tr.GetString("filters_overwrite_expired")
		if query.Message != nil {
			_, _, editErr := query.Message.EditText(b, &gotgbot.EditMessageTextOpts{Text: helpText})
			if editErr != nil {
				log.Error(editErr)
			}
		}
		_, answerErr := query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: helpText})
		if answerErr != nil {
			log.Error(answerErr)
		}
		return ext.EndGroups
	}
	if (filterData.UserID != 0 && filterData.UserID != user.Id) ||
		(filterData.ChatID != 0 && filterData.ChatID != chat.Id) {
		helpText, _ = tr.GetString("filters_overwrite_expired")
		if query.Message != nil {
			_, _, _ = query.Message.EditText(b, &gotgbot.EditMessageTextOpts{Text: helpText})
		}
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: helpText})
		return ext.EndGroups
	}

	updated, updateErr := db_filters.UpdateFilter(
		chat.Id,
		filterData.ItemName,
		filterData.Text,
		filterData.FileID,
		filterData.Buttons,
		filterData.DataType,
	)
	if updateErr != nil {
		log.Errorf("[Filters] UpdateFilter failed for chat %d: %v", chat.Id, updateErr)
		helpText, _ = tr.GetString("common_settings_save_failed")
	} else if updated {
		helpText, _ = tr.GetString("filters_overwrite_success")
	} else {
		helpText, _ = tr.GetString("filters_overwrite_cancelled")
	}

	if query.Message == nil {
		_, err = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: helpText})
		return err
	}
	_, _, err = query.Message.EditText(b, &gotgbot.EditMessageTextOpts{Text: helpText})
	if err != nil {
		log.Error(err)
		return err
	}

	_, err = query.Answer(b,
		&gotgbot.AnswerCallbackQueryOpts{
			Text: helpText,
		},
	)
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func (moduleStruct) filtersWatcher(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx == nil || ctx.EffectiveSender == nil {
		return ext.ContinueGroups
	}

	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	matchText := buildModerationMatchText(msg)
	if matchText == "" {
		return ext.ContinueGroups
	}
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.ContinueGroups
	}

	allFilters, err := db_filters.GetChatFiltersCached(chat.Id)
	if err != nil {
		log.WithField("chatId", chat.Id).WithError(err).Error("Failed to get chat filters")
		return ext.ContinueGroups
	}

	if len(allFilters) == 0 {
		return ext.ContinueGroups
	}

	filterKeys := make([]string, len(allFilters))
	filterMap := make(map[string]*db.ChatFilters, len(allFilters))
	for i, filter := range allFilters {
		filterKeys[i] = filter.KeyWord
		filterMap[filter.KeyWord] = filter
	}

	cache := keyword_matcher.GetNamedCache("filters")
	matcher := cache.GetOrCreateMatcher(chat.Id, filterKeys)

	firstPattern, found := matcher.FirstMatch(matchText)
	if !found {
		return ext.ContinueGroups
	}
	i := firstPattern

	noformatPattern := i + " noformat"
	noformatMatch := strings.Contains(strings.ToLower(matchText), strings.ToLower(noformatPattern))

	filtData, exists := filterMap[i]
	if !exists {
		return ext.ContinueGroups
	}

	if noformatMatch {
		if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id) {
			chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
			return ext.EndGroups
		}

		filtData.FilterReply = formatting.ReverseHTML2MD(filtData.FilterReply)

		filtData.FilterReply += content.RevertButtons(filtData.Buttons)

		var err error
		_, err = media.Send(b, media.Content{
			Text:    filtData.FilterReply,
			FileID:  filtData.FileID,
			MsgType: filtData.MsgType,
			Name:    filtData.KeyWord,
		}, media.Options{
			ChatID:            ctx.Message.Chat.Id,
			ReplyMsgID:        msg.MessageId,
			ThreadID:          ctx.Message.MessageThreadId,
			Keyboard:          &gotgbot.InlineKeyboardMarkup{InlineKeyboard: nil},
			NoFormat:          true,
			NoNotif:           filtData.NoNotif,
			AllowWithoutReply: true,
		})
		if err != nil {
			log.Error(err)
			return err
		}

	} else {
		var err error
		_, err = media.SendFilter(b, ctx, filtData, msg.MessageId)
		if err != nil {
			log.Error(err)
			return err
		}
	}

	return ext.ContinueGroups
}

func LoadFilters(dispatcher *ext.Dispatcher) {
	DefaultHelpRegistry().AbleMap[filtersModule.moduleName] = true

	DefaultHelpRegistry().helpableKb[filtersModule.moduleName] = [][]gotgbot.InlineKeyboardButton{
		{
			{
				Text: func() string {
					tr := i18n.MustNewTranslator("en")
					t, _ := tr.GetString("common_formatting_button")
					return t
				}(),
				CallbackData: encodeCallbackData("helpq", map[string]string{"m": "Formatting"}),
			},
		},
	}
	dispatcher.AddHandler(handlers.NewCommand("filter", filtersModule.addFilter))
	dispatcher.AddHandler(handlers.NewCommand("addfilter", filtersModule.addFilter))
	dispatcher.AddHandler(handlers.NewCommand("stop", filtersModule.rmFilter))
	dispatcher.AddHandler(handlers.NewCommand("rmfilter", filtersModule.rmFilter))
	dispatcher.AddHandler(handlers.NewCommand("removefilter", filtersModule.rmFilter))
	dispatcher.AddHandler(handlers.NewCommand("filters", filtersModule.filtersList))
	helpers.AddCmdToDisableable("filters")
	dispatcher.AddHandler(handlers.NewCommand("stopall", filtersModule.rmAllFilters))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("rmAllFilters"), filtersModule.filtersButtonHandler))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("filters_overwrite"), filtersModule.filterOverWriteHandler))
	dispatcher.AddHandlerToGroup(handlers.NewMessage(func(msg *gotgbot.Message) bool {
		return msg.Text != "" || msg.Caption != ""
	}, filtersModule.filtersWatcher), filtersModule.handlerGroup)
}

func init() {
	RegisterLegacyModule("Filters", 140, LoadFilters)
}
