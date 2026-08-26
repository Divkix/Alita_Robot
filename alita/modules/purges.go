package modules

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"

	log "github.com/sirupsen/logrus"

	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"

	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
)

type delMsgEntry struct {
	userID    int64
	timestamp time.Time
	msgID     int64
}

var (
	purgesModule = moduleStruct{moduleName: "Purges"}
	delMsgs      = sync.Map{} // Concurrent-safe map for tracking messages to delete (value is delMsgEntry)
)

// checkPurgePermissions verifies all permissions required for purge/delete commands.
// Returns the user and true if all checks pass, nil and false otherwise (response already sent).
func checkPurgePermissions(bot *gotgbot.Bot, ctx *ext.Context) (*gotgbot.User, bool) {
	user := chat_status.RequireUser(bot, ctx)
	if user == nil {
		chat_status.NewPermissionResponder(bot).Respond(ctx, "common_cannot_identify_user", "", chat_status.WithReply())
		return nil, false
	}
	if !chat_status.RequireGroup(bot, ctx, nil) {
		chat_status.NewPermissionResponder(bot).Respond(ctx, "chat_status_group_only_error", "", chat_status.WithReply())
		return nil, false
	}
	if !chat_status.RequireBotAdmin(bot, ctx, nil) {
		chat_status.NewPermissionResponder(bot).Respond(ctx, "chat_status_bot_not_admin", "", chat_status.WithReply())
		return nil, false
	}
	if !chat_status.CanBotDelete(bot, ctx, nil) {
		chat_status.NewPermissionResponder(bot).Respond(ctx, "chat_status_bot_delete_error", "", chat_status.WithReply())
		return nil, false
	}
	if !chat_status.RequireUserAdmin(bot, ctx, nil, user.Id) {
		chat_status.NewPermissionResponder(bot).Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
		return nil, false
	}
	if !chat_status.CanUserDelete(bot, ctx, nil, user.Id) {
		chat_status.NewPermissionResponder(bot).Respond(ctx, "chat_status_delete_cmd_error", "chat_status_delete_button_error", chat_status.WithReply())
		return nil, false
	}
	return user, true
}

// PurgeWorker collects errors from concurrent message deletion.
type PurgeWorker struct {
	errors     []error // Collect errors
	errorCount int     // Count of errors
	mu         sync.Mutex
}

// purgeMsgsConcurrent performs concurrent message deletion with rate limiting.
// Uses a fixed worker pool to avoid spawning up to 1000 goroutines.
func (moduleStruct) purgeMsgsConcurrent(bot *gotgbot.Bot, chat *gotgbot.Chat, pFrom bool, msgId, deleteTo int64) bool {
	// Handle the starting message if not pFrom
	if !pFrom {
		_, err := bot.DeleteMessage(chat.Id, msgId, nil)
		if err != nil {
			if strings.Contains(err.Error(), "message can't be deleted") {
				tr := i18n.MustNewTranslator(lang.GetLanguage(&ext.Context{EffectiveChat: chat}))
				text, _ := tr.GetString("purges_cannot_delete_old")
				_, err = bot.SendMessage(chat.Id, text,
					&gotgbot.SendMessageOpts{
						ReplyParameters: &gotgbot.ReplyParameters{
							MessageId:                deleteTo + 1,
							AllowSendingWithoutReply: true,
						},
					},
				)
				if err != nil {
					log.Error(err)
					return false
				}
			} else if !strings.Contains(err.Error(), "message to delete not found") {
				log.Error(err)
				return false
			}
		}
	}

	// When !pFrom, msgId was already deleted above; skip it in the loop.
	loopFrom := msgId
	if !pFrom {
		loopFrom = msgId + 1
	}

	// Calculate total messages to delete
	totalMessages := deleteTo - loopFrom + 1
	if totalMessages <= 0 {
		return true
	}

	// For small ranges, use sequential deletion
	if totalMessages <= 10 {
		for mId := deleteTo; mId >= loopFrom; mId-- {
			_ = helpers.DeleteMessageWithErrorHandling(bot, chat.Id, mId)
		}
		return true
	}

	// For larger ranges, use fixed worker pool
	const maxConcurrentMsgDeletions = 10
	worker := &PurgeWorker{
		errors: make([]error, 0),
	}

	jobs := make(chan int64, totalMessages)
	for mId := deleteTo; mId >= loopFrom; mId-- {
		jobs <- mId
	}
	close(jobs)

	var wg sync.WaitGroup
	for i := 0; i < maxConcurrentMsgDeletions; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for messageID := range jobs {
				if err := helpers.DeleteMessageWithErrorHandling(bot, chat.Id, messageID); err != nil {
					worker.mu.Lock()
					worker.errorCount++
					if worker.errorCount <= 5 {
						worker.errors = append(worker.errors, err)
					}
					worker.mu.Unlock()
				}
			}
		}()
	}
	wg.Wait()

	// Log errors if any (excluding "not found" errors)
	if len(worker.errors) > 0 {
		log.WithFields(log.Fields{
			"chat_id":       chat.Id,
			"error_count":   worker.errorCount,
			"sample_errors": worker.errors,
		}).Warn("Some messages could not be deleted during purge")
	}

	return true
}

// purgeMsgs performs the actual message deletion operation for purge commands,
// deleting messages in the specified range with error handling for old messages.
// This is a wrapper that calls the concurrent version for better performance.
func (moduleStruct) purgeMsgs(bot *gotgbot.Bot, chat *gotgbot.Chat, pFrom bool, msgId, deleteTo int64) bool {
	return purgesModule.purgeMsgsConcurrent(bot, chat, pFrom, msgId, deleteTo)
}

// purge handles the /purge command to delete all messages from a replied
// message up to the command message, requiring admin permissions.
func (m moduleStruct) purge(bot *gotgbot.Bot, ctx *ext.Context) error {
	if _, ok := checkPurgePermissions(bot, ctx); !ok {
		return ext.EndGroups
	}

	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	args := ctx.Args()[1:]

	if msg.ReplyToMessage != nil {
		msgId := msg.ReplyToMessage.MessageId
		deleteTo := msg.MessageId - 1
		totalMsgs := deleteTo - msgId + 1 // adding 1 because we want to delete the message we are replying to

		// Limit purge range to prevent abuse and API overload
		const maxPurgeMessages = 1000
		if totalMsgs > maxPurgeMessages {
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			text, _ := tr.GetString("purges_limit_exceeded")
			_, err := msg.Reply(bot, fmt.Sprintf(text, maxPurgeMessages), formatting.Shtml())
			if err != nil {
				log.Error(err)
			}
			return ext.EndGroups
		}

		purge := m.purgeMsgs(bot, chat, false, msgId, deleteTo)
		_ = helpers.DeleteMessageWithErrorHandling(bot, chat.Id, msg.MessageId)

		if purge {
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			var Text string
			if len(args) >= 1 {
				temp, _ := tr.GetString("purges_purged_with_reason")
				Text = fmt.Sprintf(temp, totalMsgs, strings.Join(args, " "))
			} else {
				temp, _ := tr.GetString("purges_purged_messages")
				Text = fmt.Sprintf(temp, totalMsgs)
			}
			pMsg, err := bot.SendMessage(chat.Id, Text, formatting.Smarkdown())
			if err != nil {
				log.Error(err)
			} else {
				// Delete notification message after 3 seconds in background
				go func(msgToDelete *gotgbot.Message) {
					defer error_handling.RecoverFromPanic("purgeNotifyDelete", "purges")
					time.Sleep(3 * time.Second)
					_, _ = msgToDelete.Delete(bot, nil)
				}(pMsg)
			}
		}
	} else {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("purges_reply_to_purge")
		_, err := msg.Reply(bot, text, nil)
		if err != nil {
			log.Error(err)
			return err
		}
	}

	return ext.EndGroups
}

// delCmd handles the /del command to delete a specific replied message
// along with the command message, requiring admin permissions.
func (moduleStruct) delCmd(bot *gotgbot.Bot, ctx *ext.Context) error {
	if _, ok := checkPurgePermissions(bot, ctx); !ok {
		return ext.EndGroups
	}

	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat

	if msg.ReplyToMessage == nil {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("purges_reply_to_delete")
		_, err := msg.Reply(bot, text, nil)
		if err != nil {
			log.Error(err)
			return err
		}

	} else {
		msgId := msg.ReplyToMessage.MessageId
		_ = helpers.DeleteMessageWithErrorHandling(bot, chat.Id, msgId)
		_, _ = msg.Delete(bot, nil)
	}

	return ext.EndGroups
}

// deleteButtonHandler processes callback queries from delete buttons
// to remove specific messages, requiring admin permissions.
func (moduleStruct) deleteButtonHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	query, ok := callbackQueryFromContext(ctx)
	if !ok {
		return ext.EndGroups
	}
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		chat_status.NewPermissionResponder(b).Respond(ctx, "", "common_cannot_identify_user")
		return ext.EndGroups
	}

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))

	msgIDRaw := ""
	decoded, ok := decodeCallbackData(query.Data, "deleteMsg")
	if !ok {
		log.Warnf("[Purges] Invalid callback data format: %s", query.Data)
		errText, _ := tr.GetString("purges_invalid_button_data")
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: errText})
		return ext.EndGroups
	}
	msgIDRaw, _ = decoded.Field("m")
	if msgIDRaw == "" {
		log.Warnf("[Purges] Invalid callback data format: %s", query.Data)
		errText, _ := tr.GetString("purges_invalid_button_data")
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: errText})
		return ext.EndGroups
	}

	// Parse message ID from callback data
	msgId, err := strconv.ParseInt(msgIDRaw, 10, 64)
	if err != nil {
		log.Warnf("[Purges] Invalid message ID in callback: %s", msgIDRaw)
		errText, _ := tr.GetString("purges_invalid_message_id")
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: errText})
		return ext.EndGroups
	}

	// permissions check
	if !chat_status.CanUserDelete(b, ctx, nil, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_delete_cmd_error", "chat_status_delete_button_error", chat_status.WithReply())
		return ext.EndGroups
	}
	if !chat_status.CanBotDelete(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_bot_delete_error", "", chat_status.WithReply())
		return ext.EndGroups
	}

	if err := helpers.DeleteMessageWithErrorHandling(b, chat.Id, msgId); err != nil {
		return err
	}

	_, err = query.Answer(b, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// purgeFrom handles the /purgefrom command to mark a starting message
// for range deletion, requiring admin permissions.
func (moduleStruct) purgeFrom(bot *gotgbot.Bot, ctx *ext.Context) error {
	user, ok := checkPurgePermissions(bot, ctx)
	if !ok {
		return ext.EndGroups
	}

	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat

	if msg.ReplyToMessage != nil {
		TodelId := msg.ReplyToMessage.MessageId
		if existing, ok := delMsgs.Load(chat.Id); ok {
			if entry, ok := existing.(delMsgEntry); ok && entry.msgID == TodelId {
				tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
				text, _ := tr.GetString("purges_message_marked")
				_, _ = msg.Reply(bot, text, nil)
				return ext.EndGroups
			}
			// Reject overwrite — inform user to use purgefrom again or clear
			if existing != nil {
				tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
				text, _ := tr.GetString("purges_message_marked")
				_, _ = msg.Reply(bot, text, nil)
				return ext.EndGroups
			}
		}
		if err := helpers.DeleteMessageWithErrorHandling(bot, chat.Id, msg.MessageId); err != nil {
			_, _ = msg.Reply(bot, err.Error(), nil)
			return ext.EndGroups
		}
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("purges_marked_for_deletion")
		pMsg, err := bot.SendMessage(chat.Id, text,
			&gotgbot.SendMessageOpts{
				ReplyParameters: &gotgbot.ReplyParameters{
					MessageId:                TodelId,
					AllowSendingWithoutReply: true,
				},
			},
		)
		if err != nil {
			log.Error(err)
			return err
		}
		delMsgs.Store(chat.Id, delMsgEntry{userID: user.Id, timestamp: time.Now(), msgID: TodelId})

		// Run cleanup in background goroutine to avoid blocking the handler
		go func(chatId, toDelId int64, msgToDelete *gotgbot.Message) {
			defer error_handling.RecoverFromPanic("purgeFromCleanup", "purges")
			time.Sleep(30 * time.Second)
			// Only clean up if the stored ID is still the same (not overwritten by another purgefrom)
			if existingId, ok := delMsgs.Load(chatId); ok {
				if entry, ok := existingId.(delMsgEntry); ok && entry.msgID == toDelId {
					delMsgs.Delete(chatId)
				}
			}
			_, err := msgToDelete.Delete(bot, nil)
			if err != nil {
				log.WithFields(log.Fields{
					"chat_id":    chatId,
					"message_id": msgToDelete.MessageId,
				}).Debug("Failed to delete purgefrom notification message")
			}
		}(chat.Id, TodelId, pMsg)
	} else {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("purges_reply_to_purgefrom")
		_, err := msg.Reply(bot, text, nil)
		if err != nil {
			log.Error(err)
			return err
		}
	}
	return ext.EndGroups
}

// purgeTo handles the /purgeto command to complete range deletion
// from a previously marked message, requiring admin permissions.
func (m moduleStruct) purgeTo(bot *gotgbot.Bot, ctx *ext.Context) error {
	if _, ok := checkPurgePermissions(bot, ctx); !ok {
		return ext.EndGroups
	}

	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	args := ctx.Args()[1:]

	if msg.ReplyToMessage != nil {
		msgIdInterface, ok := delMsgs.Load(chat.Id)
		msgId := int64(0)
		if ok {
			if entry, ok := msgIdInterface.(delMsgEntry); ok {
				msgId = entry.msgID
			} else if legacy, ok := msgIdInterface.(int64); ok {
				msgId = legacy
			}
		}
		if msgId == 0 {
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			text, _ := tr.GetString("purges_need_purgefrom")
			_, err := msg.Reply(bot, text, nil)
			if err != nil {
				log.Error(err)
				return err
			}
			return ext.EndGroups
		}
		deleteTo := msg.ReplyToMessage.MessageId
		if msgId == deleteTo {
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			text, _ := tr.GetString("purges_use_del_single")
			_, err := msg.Reply(bot, text, nil)
			if err != nil {
				log.Error(err)
				return err
			}
			return ext.EndGroups
		}
		// Ensure msgId is the lower bound and deleteTo is the upper bound
		// This normalizes the range regardless of which message was marked first
		startId, endId := msgId, deleteTo
		if deleteTo < msgId {
			startId, endId = deleteTo, msgId
		}
		totalMsgs := endId - startId + 1

		// Enforce same limit as /purge command to prevent abuse
		const maxPurgeMessages = 1000
		if totalMsgs > maxPurgeMessages {
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			text, _ := tr.GetString("purges_limit_exceeded")
			_, err := msg.Reply(bot, fmt.Sprintf(text, maxPurgeMessages), formatting.Shtml())
			if err != nil {
				log.Error(err)
			}
			return ext.EndGroups
		}

		// Clear the stored purgefrom marker since we're using it now
		delMsgs.Delete(chat.Id)

		purge := m.purgeMsgs(bot, chat, true, startId, endId)
		if err := helpers.DeleteMessageWithErrorHandling(bot, chat.Id, msg.MessageId); err != nil {
			log.Error(err)
		}
		if purge {
			var Text string
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			if len(args) >= 1 {
				temp, _ := tr.GetString("purges_purged_with_reason")
				Text = fmt.Sprintf(temp, totalMsgs, strings.Join(args, " "))
			} else {
				temp, _ := tr.GetString("purges_purged_messages")
				Text = fmt.Sprintf(temp, totalMsgs)
			}
			pMsg, err := bot.SendMessage(chat.Id, Text, formatting.Smarkdown())
			if err != nil {
				log.Error(err)
			} else {
				// Delete notification message after 3 seconds in background
				go func(msgToDelete *gotgbot.Message) {
					defer error_handling.RecoverFromPanic("purgeNotifyDelete", "purges")
					time.Sleep(3 * time.Second)
					_, _ = msgToDelete.Delete(bot, nil)
				}(pMsg)
			}
		}
	} else {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("purges_reply_to_purgeto")
		_, err := msg.Reply(bot, text, nil)
		if err != nil {
			log.Error(err)
			return err
		}
	}
	return ext.EndGroups
}

// LoadPurges registers all purges module handlers with the dispatcher,
// including message deletion commands and callback handlers.
func LoadPurges(dispatcher *ext.Dispatcher) {
	DefaultHelpRegistry().AbleMap[purgesModule.moduleName] = true

	dispatcher.AddHandler(handlers.NewCommand("del", purgesModule.delCmd))
	dispatcher.AddHandler(handlers.NewCommand("purge", purgesModule.purge))
	dispatcher.AddHandler(handlers.NewCommand("purgefrom", purgesModule.purgeFrom))
	dispatcher.AddHandler(handlers.NewCommand("purgeto", purgesModule.purgeTo))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("deleteMsg"), purgesModule.deleteButtonHandler))
}

func init() {
	RegisterLegacyModule("Purges", 90, LoadPurges)
	RegisterAnonymousAdminHandler("purge", purgesModule.purge)
	RegisterAnonymousAdminHandler("del", purgesModule.delCmd)
}
