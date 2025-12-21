package modules

import (
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
	"github.com/eko/gocache/lib/v4/store"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/decorators/misc"
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/i18n"

	"github.com/divkix/Alita_Robot/alita/utils/extraction"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	"github.com/divkix/Alita_Robot/alita/utils/media"

	"github.com/divkix/Alita_Robot/alita/utils/keyword_matcher"
	"github.com/divkix/Alita_Robot/alita/utils/string_handling"
)

var filtersModule = moduleStruct{
	moduleName:   "Filters",
	handlerGroup: 9,
}

const (
	// Cache TTL for filter overwrite confirmations (5 minutes)
	filterOverwriteCacheTTL = 5 * time.Minute
)

// filterOverwriteCacheKey generates a cache key for filter overwrite confirmations.
func filterOverwriteCacheKey(filterWord string, chatID int64) string {
	return fmt.Sprintf("alita:filter_overwrite:%s:%d", filterWord, chatID)
}

// setFilterOverwriteCache stores filter overwrite data in cache with TTL.
func setFilterOverwriteCache(filterWord string, chatID int64, data overwriteFilter) error {
	if cache.Marshal == nil {
		return fmt.Errorf("cache not initialized")
	}

	key := filterOverwriteCacheKey(filterWord, chatID)
	return cache.Marshal.Set(cache.Context, key, data, store.WithExpiration(filterOverwriteCacheTTL))
}

// getFilterOverwriteCache retrieves filter overwrite data from cache.
func getFilterOverwriteCache(filterWord string, chatID int64) (*overwriteFilter, error) {
	if cache.Marshal == nil {
		return nil, fmt.Errorf("cache not initialized")
	}

	key := filterOverwriteCacheKey(filterWord, chatID)
	var data overwriteFilter
	_, err := cache.Marshal.Get(cache.Context, key, &data)
	if err != nil {
		return nil, err
	}

	return &data, nil
}

// deleteFilterOverwriteCache removes filter overwrite data from cache.
func deleteFilterOverwriteCache(filterWord string, chatID int64) {
	if cache.Marshal == nil {
		return
	}

	key := filterOverwriteCacheKey(filterWord, chatID)
	err := cache.Marshal.Delete(cache.Context, key)
	if err != nil {
		log.Debugf("[Filters] Failed to delete cache for key %s: %v", key, err)
	}
}

/*
	Used to add a filter to a specific keyword in chat!

# Connection - true, true

Only admin can add new filters in the chat
*/
// addFilter creates a new filter with a keyword trigger and response content.
// Only admins can add filters. Supports text, media, and buttons with a limit of 150 filters per chat.
func (m moduleStruct) addFilter(b *gotgbot.Bot, ctx *ext.Context) error {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("[Filters][addFilter] Recovered from panic: %v", r)
		}
	}()
	msg := ctx.EffectiveMessage
	// connection status
	connectedChat := helpers.IsUserConnected(b, ctx, true, false)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat
	user := ctx.EffectiveSender.User
	args := ctx.Args()

	// check permission
	if !chat_status.CanUserChangeInfo(b, ctx, chat, user.Id, false) {
		return ext.EndGroups
	}

	filtersNum := db.CountFilters(chat.Id)
	if filtersNum >= 150 {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("filters_limit_exceeded")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}

		return ext.EndGroups
	}

	if msg.ReplyToMessage != nil && len(args) <= 1 {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("filters_keyword_required")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	} else if len(args) <= 2 && msg.ReplyToMessage == nil {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("filters_invalid")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	filterWord, fileid, text, dataType, buttons, _, _, _, _, _, _, errorMsg := helpers.GetNoteAndFilterType(msg, true, db.GetLanguage(ctx))
	if dataType == -1 {
		_, err := msg.Reply(b, errorMsg, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	filterWord = strings.ToLower(filterWord) // convert string to it's lower form

	// Validate keyword length - max 100 characters
	if len(filterWord) > 100 {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("filters_keyword_too_long")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if db.DoesFilterExists(chat.Id, filterWord) {
		// Store in cache instead of in-memory map
		err := setFilterOverwriteCache(filterWord, chat.Id, overwriteFilter{
			filterWord: filterWord,
			text:       text,
			fileid:     fileid,
			buttons:    buttons,
			dataType:   dataType,
		})
		if err != nil {
			log.Errorf("[Filters] Failed to cache overwrite data: %v", err)
			// Fallback: allow the operation to continue
		}

		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		confirmText, _ := tr.GetString("filters_overwrite_confirm")
		yesText, _ := tr.GetString("common_yes")
		noText, _ := tr.GetString("common_no")
		_, err = msg.Reply(b,
			confirmText,
			&gotgbot.SendMessageOpts{
				ParseMode: helpers.HTML,
				ReplyMarkup: gotgbot.InlineKeyboardMarkup{
					InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
						{
							{
								Text:         yesText,
								CallbackData: "filters_overwrite." + filterWord,
							},
							{
								Text:         noText,
								CallbackData: "filters_overwrite.cancel",
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
		return ext.EndGroups
	}

	// Capture variables for goroutine closure
	chatId := chat.Id
	go func() {
		defer error_handling.RecoverFromPanic("AddFilter", "filters")
		db.AddFilter(chatId, filterWord, text, fileid, buttons, dataType)
	}()

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	successText, _ := tr.GetString("filters_added_success")
	_, err := msg.Reply(b, fmt.Sprintf(successText, filterWord), helpers.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

/*
	Used to remove a filter to a specific keyword in chat!

# Connection - true, true

Only admin can remove filters in the chat
*/
// rmFilter removes an existing filter by its keyword trigger.
// Only admins can remove filters. Requires the exact filter keyword as argument.
func (moduleStruct) rmFilter(b *gotgbot.Bot, ctx *ext.Context) error {
	// connection status
	connectedChat := helpers.IsUserConnected(b, ctx, true, false)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	user := ctx.EffectiveSender.User
	args := ctx.Args()[1:]

	// check permission
	if !chat_status.CanUserChangeInfo(b, ctx, chat, user.Id, false) {
		return ext.EndGroups
	}

	if len(args) == 0 {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("filters_remove_keyword_required")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
	} else {

		filterWord, _ := extraction.ExtractQuotes(strings.Join(args, " "), true, true)

		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		if !string_handling.FindInStringSlice(db.GetFiltersList(chat.Id), strings.ToLower(filterWord)) {
			text, _ := tr.GetString("filters_not_exists")
			_, err := msg.Reply(b, text, helpers.Shtml())
			if err != nil {
				log.Error(err)
				return err
			}
		} else {
			// Capture variables for goroutine closure
			chatId := chat.Id
			lowerFilterWord := strings.ToLower(filterWord)
			go func() {
				defer error_handling.RecoverFromPanic("RemoveFilter", "filters")
				db.RemoveFilter(chatId, lowerFilterWord)
			}()
			successText, _ := tr.GetString("filters_removed_success")
			_, err := msg.Reply(b, fmt.Sprintf(successText, filterWord), helpers.Shtml())
			if err != nil {
				log.Error(err)
				return err
			}
		}
	}
	return ext.EndGroups
}

/*
	Used to view all filters in the chat!

# Connection - false, true

Any user can view users in a chat
*/
// filtersList displays all active filter keywords in the current chat.
// Any user can view the list of available filters with their trigger keywords.
func (moduleStruct) filtersList(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	// if command is disabled, return
	if chat_status.CheckDisabledCmd(b, msg, "filters") {
		return ext.EndGroups
	}
	// connection status
	connectedChat := helpers.IsUserConnected(b, ctx, false, true)
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

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	filterKeys := db.GetFiltersList(chat.Id)
	info, _ := tr.GetString("filters_none_in_chat")
	newFilterKeys := make([]string, 0)

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
			ParseMode: helpers.HTML,
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

/*
	Used to remove all filters from the current chat

Only owner can remove all filters from the chat
*/
// rmAllFilters removes all filters from the current chat with confirmation.
// Only chat owners can use this command. Shows confirmation buttons before deletion.
func (moduleStruct) rmAllFilters(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	user := ctx.EffectiveSender.User
	msg := ctx.EffectiveMessage
	filterKeys := db.GetFiltersList(chat.Id)

	if len(filterKeys) == 0 {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("filters_none_in_chat")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}

		return ext.EndGroups
	}

	if chat_status.RequireUserOwner(b, ctx, chat, user.Id, false) {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		confirmText, _ := tr.GetString("filters_clear_all_confirm")
		yesText, _ := tr.GetString("common_yes")
		noText, _ := tr.GetString("common_no")
		_, err := msg.Reply(b, confirmText,
			&gotgbot.SendMessageOpts{
				ReplyMarkup: gotgbot.InlineKeyboardMarkup{
					InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
						{
							{Text: yesText, CallbackData: "rmAllFilters.yes"},
							{Text: noText, CallbackData: "rmAllFilters.no"},
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

// CallbackQuery handler for rmAllFilters
// filtersButtonHandler handles callback queries for filter-related button interactions.
// Processes confirmation dialogs for removing all filters from a chat.
func (moduleStruct) filtersButtonHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	query := ctx.CallbackQuery
	user := query.From
	chat := ctx.EffectiveChat

	// permission checks
	if !chat_status.RequireUserOwner(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	args := strings.Split(query.Data, ".")
	response := args[1]
	var helpText string

	switch response {
	case "yes":
		db.RemoveAllFilters(chat.Id)
		helpText, _ = tr.GetString("filters_clear_all_success")
	case "no":
		helpText, _ = tr.GetString("filters_clear_all_cancelled")
	}

	_, _, err := query.Message.EditText(b,
		helpText,
		nil,
	)
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

// CallbackQuery handler for filters_overwite. query
// filterOverWriteHandler handles callback queries for filter overwrite confirmations.
// Processes admin decisions when attempting to overwrite existing filter keywords.
func (m moduleStruct) filterOverWriteHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	query := ctx.CallbackQuery
	user := query.From
	chat := ctx.EffectiveChat

	// permission checks
	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	args := strings.Split(query.Data, ".")
	filterWord := args[1]
	var helpText string

	// Retrieve from cache instead of in-memory map
	filterData, err := getFilterOverwriteCache(filterWord, chat.Id)
	if err != nil {
		log.Debugf("[Filters] Failed to retrieve overwrite data from cache: %v", err)
		helpText, _ = tr.GetString("filters_overwrite_expired")
		_, _, editErr := query.Message.EditText(b, helpText, nil)
		if editErr != nil {
			log.Error(editErr)
		}
		_, answerErr := query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: helpText})
		if answerErr != nil {
			log.Error(answerErr)
		}
		return ext.EndGroups
	}

	if db.DoesFilterExists(chat.Id, filterWord) {
		db.RemoveFilter(chat.Id, filterWord)
		db.AddFilter(chat.Id, filterData.filterWord, filterData.text, filterData.fileid, filterData.buttons, filterData.dataType)
		deleteFilterOverwriteCache(filterWord, chat.Id) // Clean up cache
		helpText, _ = tr.GetString("filters_overwrite_success")
	} else {
		helpText, _ = tr.GetString("filters_overwrite_cancelled")
		deleteFilterOverwriteCache(filterWord, chat.Id) // Clean up cache
	}

	_, _, err = query.Message.EditText(b,
		helpText,
		nil,
	)
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

/*
	Watchers for filter

Replies with appropriate data to the filter.
*/
// filtersWatcher monitors incoming messages for filter keyword matches.
// Automatically responds with filter content when keywords are detected in messages.
func (moduleStruct) filtersWatcher(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	user := ctx.EffectiveSender.User

	// Use optimized cached query to fetch all filters at once (no N+1 query)
	optQueries := db.GetOptimizedQueries()
	allFilters, err := optQueries.GetChatFiltersCached(chat.Id)
	if err != nil {
		log.WithField("chatId", chat.Id).WithError(err).Error("Failed to get chat filters")
		return ext.ContinueGroups
	}

	if len(allFilters) == 0 {
		return ext.ContinueGroups
	}

	// Build keyword list for Aho-Corasick matching
	filterKeys := make([]string, len(allFilters))
	filterMap := make(map[string]*db.ChatFilters, len(allFilters))
	for i, filter := range allFilters {
		filterKeys[i] = filter.KeyWord
		filterMap[filter.KeyWord] = filter
	}

	// Use Aho-Corasick for efficient multi-pattern matching
	cache := keyword_matcher.GetGlobalCache()
	matcher := cache.GetOrCreateMatcher(chat.Id, filterKeys)

	// Check for any filter match first
	if !matcher.HasMatch(msg.Text) {
		return ext.ContinueGroups
	}

	// Get all matches to handle them individually
	matches := matcher.FindMatches(msg.Text)
	if len(matches) == 0 {
		return ext.ContinueGroups
	}

	// Process first match (same behavior as before)
	firstMatch := matches[0]
	i := firstMatch.Pattern

	// Check for noformat pattern using simpler string matching
	noformatPattern := i + " noformat"
	noformatMatch := strings.Contains(strings.ToLower(msg.Text), strings.ToLower(noformatPattern))

	// Get filter data from pre-loaded map (no additional DB query)
	filtData, exists := filterMap[i]
	if !exists {
		return ext.ContinueGroups
	}

	if noformatMatch {
		// check if user is admin or not
		if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id, false) {
			return ext.EndGroups
		}

		// Reverse notedata
		filtData.FilterReply = helpers.ReverseHTML2MD(filtData.FilterReply)

		// show the buttons back as text
		filtData.FilterReply += helpers.RevertButtons(filtData.Buttons)

		// using true as last argument to prevent the message from being formatted
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
		_, err = helpers.SendFilter(b, ctx, filtData, msg.MessageId)
		if err != nil {
			log.Error(err)
			return err
		}
	}

	return ext.ContinueGroups
}

// LoadFilters registers all filter-related handlers with the dispatcher.
// Sets up commands for managing filters and the message watcher for automatic responses.
func LoadFilters(dispatcher *ext.Dispatcher) {
	HelpModule.AbleMap.Store(filtersModule.moduleName, true)

	HelpModule.helpableKb[filtersModule.moduleName] = [][]gotgbot.InlineKeyboardButton{
		{
			{
				Text:         "Formatting", // This will be dynamically translated in the help system
				CallbackData: fmt.Sprintf("helpq.%s", "Formatting"),
			},
		},
	} // Adds Formatting kb button to Filters Menu
	dispatcher.AddHandler(handlers.NewCommand("filter", filtersModule.addFilter))
	dispatcher.AddHandler(handlers.NewCommand("addfilter", filtersModule.addFilter))
	dispatcher.AddHandler(handlers.NewCommand("stop", filtersModule.rmFilter))
	dispatcher.AddHandler(handlers.NewCommand("rmfilter", filtersModule.rmFilter))
	dispatcher.AddHandler(handlers.NewCommand("removefilter", filtersModule.rmFilter))
	dispatcher.AddHandler(handlers.NewCommand("filters", filtersModule.filtersList))
	misc.AddCmdToDisableable("filters")
	dispatcher.AddHandler(handlers.NewCommand("stopall", filtersModule.rmAllFilters))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("rmAllFilters"), filtersModule.filtersButtonHandler))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("filters_overwrite."), filtersModule.filterOverWriteHandler))
	dispatcher.AddHandlerToGroup(handlers.NewMessage(message.Text, filtersModule.filtersWatcher), filtersModule.handlerGroup)
}
