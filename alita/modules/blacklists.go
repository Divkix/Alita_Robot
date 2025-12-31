package modules

import (
	"context"
	"fmt"
	"html"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/decorators/cmdDecorator"
	"github.com/divkix/Alita_Robot/alita/utils/decorators/misc"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	"github.com/divkix/Alita_Robot/alita/utils/keyword_matcher"
	"github.com/divkix/Alita_Robot/alita/utils/string_handling"
)

var blacklistsModule = moduleStruct{
	moduleName:   "Blacklists",
	handlerGroup: 7,
}

// Use the shared global regex cache from filters module

/*
	Used to add a blacklist to group!

Connection - true, true
Admin can add a blacklist to the chat
*/
// addBlacklist handles the /addblacklist command to add blacklisted words to a group.
// Admins can add words that will trigger automatic moderation actions.
func (m moduleStruct) addBlacklist(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	// connection status
	connectedChat := helpers.IsUserConnected(b, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	chat := connectedChat
	user := ctx.EffectiveSender.User
	args := ctx.Args()[1:]
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	var (
		alreadyBlacklisted, newBlacklist []string
		text                             string
	)

	// Permission Checks
	if !chat_status.IsUserAdmin(b, chat.Id, user.Id) {
		return ext.EndGroups
	}
	if !chat_status.IsBotAdmin(b, ctx, chat) {
		return ext.EndGroups
	}
	if !chat_status.CanUserRestrict(b, ctx, chat, user.Id, false) {
		return ext.EndGroups
	}
	if !chat_status.CanBotRestrict(b, ctx, chat, false) {
		return ext.EndGroups
	}

	if len(args) == 0 {
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_blacklist_give_bl_word")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	} else if len(args) >= 1 {
		allBlWords := db.GetBlacklistSettings(chat.Id).Triggers()

		// Validate word lengths - reject words over 100 characters
		var tooLong []string
		validArgs := make([]string, 0, len(args))
		for _, word := range args {
			if len(word) > 100 {
				// Use rune-based truncation to avoid splitting multi-byte UTF-8 characters
				runes := []rune(word)
				preview := word
				if len(runes) > 20 {
					preview = string(runes[:20]) + "..."
				}
				tooLong = append(tooLong, preview)
			} else {
				validArgs = append(validArgs, word)
			}
		}
		if len(tooLong) > 0 {
			text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_blacklist_word_too_long")
			_, err := msg.Reply(b, fmt.Sprintf(text, strings.Join(tooLong, ", ")), helpers.Shtml())
			if err != nil {
				log.Error(err)
			}
		}
		if len(validArgs) == 0 {
			return ext.EndGroups
		}
		args = validArgs

		// For small lists, process sequentially with WaitGroup
		if len(args) <= 3 {
			var wg sync.WaitGroup
			for _, blWord := range args {
				if string_handling.FindInStringSlice(allBlWords, blWord) {
					alreadyBlacklisted = append(alreadyBlacklisted, blWord)
				} else {
					wg.Add(1)
					go func(chatId int64, word string) {
						defer func() {
							if r := recover(); r != nil {
								log.WithFields(log.Fields{
									"panic":  r,
									"chatId": chatId,
									"word":   word,
								}).Error("Panic in AddBlacklist goroutine")
							}
							wg.Done()
						}()
						db.AddBlacklist(chatId, word)
					}(chat.Id, blWord)
					newBlacklist = append(newBlacklist, fmt.Sprintf("<code>%s</code>", html.EscapeString(blWord)))
				}
			}
			// Wait for all goroutines to complete
			wg.Wait()
		} else {
			// For larger lists, process concurrently
			type result struct {
				word            string
				isAlreadyListed bool
			}

			resultChan := make(chan result, len(args))
			var wg sync.WaitGroup

			for _, blWord := range args {
				wg.Add(1)
				go func(word string) {
					defer func() {
						if r := recover(); r != nil {
							log.WithFields(log.Fields{
								"panic":  r,
								"chatId": chat.Id,
								"word":   word,
							}).Error("Panic in AddBlacklist concurrent goroutine")
						}
						wg.Done()
					}()

					isListed := string_handling.FindInStringSlice(allBlWords, word)
					resultChan <- result{word: word, isAlreadyListed: isListed}

					if !isListed {
						db.AddBlacklist(chat.Id, word)
					}
				}(blWord)
			}

			// Close channel after all goroutines complete
			go func() {
				wg.Wait()
				close(resultChan)
			}()

			// Collect results
			for res := range resultChan {
				if res.isAlreadyListed {
					alreadyBlacklisted = append(alreadyBlacklisted, res.word)
				} else {
					newBlacklist = append(newBlacklist, fmt.Sprintf("<code>%s</code>", html.EscapeString(res.word)))
				}
			}
		}

		if len(alreadyBlacklisted) >= 1 {
			temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_blacklist_already_blacklisted")
			text += temp + fmt.Sprintf("\n - %s\n\n", strings.Join(alreadyBlacklisted, "\n - "))
		}
		if len(newBlacklist) >= 1 {
			temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_blacklist_added_bl")
			text += temp + fmt.Sprintf("\n - %s\n\n", strings.Join(newBlacklist, "\n - "))
		}

		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
	}

	return ext.EndGroups
}

/*
	Used to remove a blacklist from group!

Connection - true, true
Admin can add a blacklist to the chat
*/
// removeBlacklist handles the /rmblacklist command to remove blacklisted words.
// Allows admins to remove previously blacklisted words from the group.
func (m moduleStruct) removeBlacklist(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	// connection status
	connectedChat := helpers.IsUserConnected(b, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	chat := connectedChat
	user := ctx.EffectiveSender.User
	args := ctx.Args()[1:]
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	var removedBlacklists []string

	// Permission Checks
	if !chat_status.IsUserAdmin(b, chat.Id, user.Id) {
		return ext.EndGroups
	}
	if !chat_status.IsBotAdmin(b, ctx, chat) {
		return ext.EndGroups
	}
	if !chat_status.CanUserRestrict(b, ctx, chat, user.Id, false) {
		return ext.EndGroups
	}
	if !chat_status.CanBotRestrict(b, ctx, chat, false) {
		return ext.EndGroups
	}

	if len(args) == 0 {
		text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_unblacklist_give_bl_word")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	} else {
		allBlWords := db.GetBlacklistSettings(chat.Id).Triggers()
		for _, blWord := range args {
			if string_handling.FindInStringSlice(allBlWords, blWord) {
				removedBlacklists = append(removedBlacklists, blWord)
				go func(chatId int64, word string) {
					defer func() {
						if r := recover(); r != nil {
							log.WithFields(log.Fields{
								"panic":  r,
								"chatId": chatId,
								"word":   word,
							}).Error("Panic in RemoveBlacklist goroutine")
						}
					}()
					db.RemoveBlacklist(chatId, word)
				}(chat.Id, blWord)
			}
		}
		if len(removedBlacklists) <= 0 {
			text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_unblacklist_no_removed_bl")
			_, err := msg.Reply(b, text, nil)
			if err != nil {
				log.Error(err)
				return err
			}
		} else {
			temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_unblacklist_removed_bl")
			_, err := msg.Reply(b, fmt.Sprintf(temp, strings.Join(removedBlacklists, ", ")), nil)
			if err != nil {
				log.Error(err)
				return err
			}
		}
	}
	return ext.EndGroups
}

/*
	Used to list all blacklists of a group!

Connection - false, true
Anyone can view blacklists in group
*/
// listBlacklists handles the /blacklists command to display all blacklisted words.
// Shows a sorted list of all currently blacklisted words in the group.
func (m moduleStruct) listBlacklists(b *gotgbot.Bot, ctx *ext.Context) error {
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	msg := ctx.EffectiveMessage
	// if command is disabled, return
	if chat_status.CheckDisabledCmd(b, msg, "blacklists") {
		return ext.EndGroups
	}
	// connection status
	connectedChat := helpers.IsUserConnected(b, ctx, false, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	chat := connectedChat

	var (
		replyMsgId     int64
		blacklistsText string
	)

	if reply := msg.ReplyToMessage; reply != nil {
		replyMsgId = reply.MessageId
	} else {
		replyMsgId = msg.MessageId
	}

	blSrc := db.GetBlacklistSettings(chat.Id)
	slices.Sort(blSrc.Triggers())
	var sb strings.Builder
	for _, i := range blSrc.Triggers() {
		sb.WriteString(fmt.Sprintf("\n - <code>%s</code>", html.EscapeString(i)))
	}
	blacklistsText += sb.String()

	if blacklistsText != "" {
		temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_ls_bl_list_bl")
		blacklistsText = temp + blacklistsText
	} else {
		blacklistsText, _ = tr.GetString(strings.ToLower(m.moduleName) + "_ls_bl_no_blacklisted")
	}

	_, err := msg.Reply(b,
		blacklistsText,
		&gotgbot.SendMessageOpts{
			ReplyParameters: &gotgbot.ReplyParameters{
				MessageId:                replyMsgId,
				AllowSendingWithoutReply: true,
			},
			ParseMode: helpers.HTML,
		},
	)
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

/*
	Used to set mode for blacklists in chat

# Connection - true, true

Admin with restriction permission can set blacklist action in group out of - ick, ban, mute
*/
// setBlacklistAction handles the /blaction command to configure blacklist punishment.
// Sets the action (mute/kick/warn/ban/none) taken when blacklisted words are detected.
func (m moduleStruct) setBlacklistAction(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	// connection status
	connectedChat := helpers.IsUserConnected(b, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	chat := connectedChat
	user := ctx.EffectiveSender.User
	args := ctx.Args()[1:]
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	var rMsg string

	// Permission Checks
	if !chat_status.CanUserRestrict(b, ctx, chat, user.Id, false) {
		return ext.EndGroups
	}
	if !chat_status.CanBotRestrict(b, ctx, chat, false) {
		return ext.EndGroups
	}

	if len(args) == 0 {
		currAction := db.GetBlacklistSettings(chat.Id).Action()
		temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_set_bl_action_current_mode")
		rMsg = fmt.Sprintf(temp, currAction)
	} else if len(args) == 1 {
		action := strings.ToLower(args[0])
		if string_handling.FindInStringSlice([]string{"mute", "kick", "warn", "ban", "none"}, action) {
			temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_set_bl_action_changed_mode")
			rMsg = fmt.Sprintf(temp, action)
			go func(chatId int64, actionType string) {
				defer func() {
					if r := recover(); r != nil {
						log.WithFields(log.Fields{
							"panic":  r,
							"chatId": chatId,
							"action": actionType,
						}).Error("Panic in SetBlacklistAction goroutine")
					}
				}()
				db.SetBlacklistAction(chatId, actionType)
			}(chat.Id, action)
		} else {
			rMsg, _ = tr.GetString(strings.ToLower(m.moduleName) + "_set_bl_action_choose_correct_option")
		}
	} else {
		rMsg, _ = tr.GetString(strings.ToLower(m.moduleName) + "_set_bl_action_choose_correct_option")
	}
	_, err := msg.Reply(b, rMsg, helpers.Smarkdown())
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}

/*
	Used to remove all blacklists from a group

Only chat creator can use this command to remove all blacklists aat once from the current chat
*/
// rmAllBlacklists handles the /rmallbl command to remove all blacklisted words.
// Only chat owners can use this command with confirmation via inline keyboard.
func (m moduleStruct) rmAllBlacklists(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	user := ctx.EffectiveSender.User
	msg := ctx.EffectiveMessage
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	// permission checks
	if !chat_status.RequireGroup(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireUserOwner(b, ctx, chat, user.Id, false) {
		return ext.EndGroups
	}

	text, _ := tr.GetString(strings.ToLower(m.moduleName) + "_rm_all_bl_ask")
	yesText, _ := tr.GetString("button_yes")
	noText, _ := tr.GetString("button_no")
	_, err := msg.Reply(b, text,
		&gotgbot.SendMessageOpts{
			ReplyMarkup: gotgbot.InlineKeyboardMarkup{
				InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
					{
						{Text: yesText, CallbackData: "rmAllBlacklist.yes"},
						{Text: noText, CallbackData: "rmAllBlacklist.no"},
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

// Callback Handler for rmallblacklist
// buttonHandler processes confirmation callbacks for removing all blacklists.
// Handles the yes/no confirmation when owners attempt to clear all blacklisted words.
func (m moduleStruct) buttonHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	query := ctx.CallbackQuery
	user := query.From
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	// permission checks
	if !chat_status.RequireUserOwner(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}

	args := strings.Split(query.Data, ".")
	creatorAction := args[1]
	var helpText string

	switch creatorAction {
	case "yes":
		go func(chatId int64) {
			defer func() {
				if r := recover(); r != nil {
					log.WithFields(log.Fields{
						"panic":  r,
						"chatId": chatId,
					}).Error("Panic in RemoveAllBlacklist goroutine")
				}
			}()
			db.RemoveAllBlacklist(chatId)
		}(query.Message.GetChat().Id)
		helpText, _ = tr.GetString(strings.ToLower(m.moduleName) + "_rm_all_bl_button_handler_yes")
	case "no":
		helpText, _ = tr.GetString(strings.ToLower(m.moduleName) + "_rm_all_bl_button_handler_no")
	}

	_, _, err := query.Message.EditText(b,
		helpText,
		&gotgbot.EditMessageTextOpts{
			ParseMode: helpers.HTML,
		},
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
	Blacklist watcher

Watcher for blacklisted words, if any of the sentence contains the word, it will remove and use the appropriate action
*/
// blacklistWatcher monitors all messages for blacklisted words.
// Automatically applies configured punishment when blacklisted content is detected.
func (m moduleStruct) blacklistWatcher(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	user := ctx.EffectiveSender
	if user.IsAnonymousAdmin() {
		return ext.ContinueGroups
	}

	// skip admins and creator + approved users and anonymous channel
	// Only check admin status for actual users, not anonymous channels
	if !user.IsAnonymousChannel() && user.IsUser() && user.Id() > 0 && chat_status.IsUserAdmin(b, chat.Id, user.Id()) {
		return ext.ContinueGroups
	}

	// Check if bot has admin permissions to take action
	// This prevents wasted API calls when bot can't delete/restrict
	if !chat_status.IsBotAdmin(b, ctx, chat) {
		return ext.ContinueGroups
	}

	msg := ctx.EffectiveMessage
	blSettings := db.GetBlacklistSettings(chat.Id)
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	triggers := blSettings.Triggers()
	if len(triggers) == 0 {
		return ext.ContinueGroups
	}

	// Use Aho-Corasick for efficient multi-pattern matching
	cache := keyword_matcher.GetGlobalCache()
	matcher := cache.GetOrCreateMatcher(chat.Id, triggers)

	// Check for any blacklist match first
	if !matcher.HasMatch(msg.Text) {
		return ext.ContinueGroups
	}

	// Get first match to process
	matches := matcher.FindMatches(msg.Text)
	if len(matches) == 0 {
		return ext.ContinueGroups
	}

	// Process first matched trigger
	i := matches[0].Pattern

	_, err := msg.Delete(b, nil)
	if err != nil {
		log.Error(err)
		return err
	}
	switch blSettings.Action() {
	case "mute":
		// don't work on anonymous channels
		if user.IsAnonymousChannel() {
			return ext.ContinueGroups
		}

		_, err = b.RestrictChatMember(chat.Id, user.Id(), gotgbot.ChatPermissions{CanSendMessages: false}, nil)
		if err != nil {
			log.Error(err)
			return err
		}

		_, err = b.SendMessage(chat.Id,
			func() string {
				temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_bl_watcher_muted_user")
				return fmt.Sprintf(temp, helpers.MentionHtml(user.Id(), user.Name()), fmt.Sprintf(blSettings.Reason(), i))
			}(),
			helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
	case "ban":
		// ban anonymous channels as well
		if user.IsAnonymousChannel() {
			_, err = b.BanChatSenderChat(chat.Id, user.Id(), nil)
		} else {
			_, err = b.BanChatMember(chat.Id, user.Id(), nil)
		}
		if err != nil {
			log.Error(err)
			return err
		}

		_, err = b.SendMessage(chat.Id,
			func() string {
				temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_bl_watcher_banned_user")
				return fmt.Sprintf(temp, helpers.MentionHtml(user.Id(), user.Name()), fmt.Sprintf(blSettings.Reason(), i))
			}(),
			helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
	case "kick":
		// don't work on anonymous channels
		if user.IsAnonymousChannel() {
			return ext.ContinueGroups
		}

		_, err = b.BanChatMember(chat.Id, user.Id(), nil)
		if err != nil {
			log.Error(err)
			return err
		}

		_, err = b.SendMessage(chat.Id,
			func() string {
				temp, _ := tr.GetString(strings.ToLower(m.moduleName) + "_bl_watcher_kicked_user")
				return fmt.Sprintf(temp, helpers.MentionHtml(user.Id(), user.Name()), fmt.Sprintf(blSettings.Reason(), i))
			}(),
			helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}

		// Use non-blocking delayed unban for blacklist kick action
		go func(userId int64, chatId int64) {
			defer func() {
				if r := recover(); r != nil {
					log.WithField("panic", r).Error("Panic in blacklist delayed unban goroutine")
				}
			}()

			// Create context with timeout
			timeoutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			timer := time.NewTimer(3 * time.Second)
			defer timer.Stop()

			select {
			case <-timer.C:
				_, unbanErr := chat.UnbanMember(b, userId, nil)
				if unbanErr != nil {
					log.WithFields(log.Fields{
						"chatId": chatId,
						"userId": userId,
						"error":  unbanErr,
					}).Error("Failed to unban user after blacklist kick")
				}
			case <-timeoutCtx.Done():
				log.WithFields(log.Fields{
					"chatId": chatId,
					"userId": userId,
				}).Warn("Blacklist unban operation timed out")
			}
		}(user.Id(), chat.Id)
	case "warn":
		// don't work on anonymous channels
		if user.IsAnonymousChannel() {
			return ext.ContinueGroups
		}

		err = warnsModule.warnThisUser(b, ctx, user.Id(), fmt.Sprintf(blSettings.Reason(), i), "warn")
		if err != nil {
			log.Error(err)
			return err
		}
	case "none":
		// Message already deleted, no further action needed
		return ext.ContinueGroups
	}

	return ext.ContinueGroups
}

// LoadBlacklists registers all blacklist module handlers with the dispatcher.
// Sets up commands for managing blacklists and the message watcher for enforcement.
func LoadBlacklists(dispatcher *ext.Dispatcher) {
	HelpModule.AbleMap.Store(blacklistsModule.moduleName, true)

	dispatcher.AddHandler(handlers.NewCommand("blacklists", blacklistsModule.listBlacklists))
	misc.AddCmdToDisableable("blacklists")
	dispatcher.AddHandler(handlers.NewCommand("addblacklist", blacklistsModule.addBlacklist))
	dispatcher.AddHandler(handlers.NewCommand("blacklist", blacklistsModule.addBlacklist))
	dispatcher.AddHandler(handlers.NewCommand("rmblacklist", blacklistsModule.removeBlacklist))
	dispatcher.AddHandler(handlers.NewCommand("blaction", blacklistsModule.setBlacklistAction))
	dispatcher.AddHandler(handlers.NewCommand("blacklistaction", blacklistsModule.setBlacklistAction))
	cmdDecorator.MultiCommand(dispatcher, []string{"remallbl", "rmallbl"}, blacklistsModule.rmAllBlacklists)
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("rmAllBlacklist"), blacklistsModule.buttonHandler))
	dispatcher.AddHandlerToGroup(handlers.NewMessage(message.Text, blacklistsModule.blacklistWatcher), blacklistsModule.handlerGroup)
}
