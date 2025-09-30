package modules

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"

	log "github.com/sirupsen/logrus"

	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
)

var (
	purgesModule = moduleStruct{moduleName: "Purges"}
	delMsgs      = sync.Map{} // Changed to sync.Map for concurrent access
)

// PurgeWorker manages concurrent message deletion with rate limiting
type PurgeWorker struct {
	sem        chan struct{} // Semaphore for rate limiting
	errors     []error       // Collect errors
	errorCount int           // Count of errors
	mu         sync.Mutex    // Protect error slice
}

// purgeMsgsConcurrent performs concurrent message deletion with rate limiting.
// Uses goroutines to delete messages in parallel for better performance.
func (moduleStruct) purgeMsgsConcurrent(bot *gotgbot.Bot, chat *gotgbot.Chat, pFrom bool, msgId, deleteTo int64) bool {
	// Handle the starting message if not pFrom
	if !pFrom {
		_, err := bot.DeleteMessage(chat.Id, msgId, nil)
		if err != nil {
			if strings.Contains(err.Error(), "message can't be deleted") {
				tr := i18n.MustNewTranslator(db.GetLanguage(&ext.Context{EffectiveChat: chat}))
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

	// Calculate total messages to delete
	totalMessages := deleteTo - msgId + 1
	if totalMessages <= 0 {
		return true
	}

	// For small ranges, use sequential deletion
	if totalMessages <= 10 {
		for mId := deleteTo + 1; mId > msgId-1; mId-- {
			_, _ = bot.DeleteMessage(chat.Id, mId, nil)
		}
		return true
	}

	// For larger ranges, use concurrent deletion
	worker := &PurgeWorker{
		sem:    make(chan struct{}, 10), // Max 10 concurrent deletions
		errors: make([]error, 0),
	}

	var wg sync.WaitGroup

	// Delete messages concurrently
	for mId := deleteTo + 1; mId > msgId-1; mId-- {
		wg.Add(1)
		worker.sem <- struct{}{} // Acquire semaphore

		go func(messageId int64) {
			defer wg.Done()
			defer func() { <-worker.sem }() // Release semaphore

			_, err := bot.DeleteMessage(chat.Id, messageId, nil)
			if err != nil && !strings.Contains(err.Error(), "message to delete not found") {
				worker.mu.Lock()
				worker.errorCount++
				// Only log first 5 errors to avoid spam
				if worker.errorCount <= 5 {
					worker.errors = append(worker.errors, err)
				}
				worker.mu.Unlock()
			}
		}(mId)
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
	user := ctx.EffectiveSender.User

	// Permission checks
	if !chat_status.RequireGroup(bot, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireBotAdmin(bot, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.CanBotDelete(bot, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(bot, ctx, nil, ctx.EffectiveUser.Id, false) {
		return ext.EndGroups
	}
	if !chat_status.CanUserDelete(bot, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}

	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	args := ctx.Args()[1:]

	if msg.ReplyToMessage != nil {
		msgId := msg.ReplyToMessage.MessageId
		deleteTo := msg.MessageId - 1
		totalMsgs := deleteTo - msgId + 1 // adding 1 because we want to delete the message we are replying to
		purge := m.purgeMsgs(bot, chat, false, msgId, deleteTo)
		_, err := bot.DeleteMessage(chat.Id, msg.MessageId, nil)
		// if err.Error() == "unable to deleteMessage: Bad Request: message to delete not found" {
		// 	log.WithFields(
		// 		log.Fields{
		// 			"chat": chat.Id,
		// 		},
		// 	).Error("error deleting message")
		// } else
		if err != nil {
			log.Error(err)
		}

		if purge {
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			var Text string
			if len(args) >= 1 {
				temp, _ := tr.GetString("purges_purged_with_reason")
				Text = fmt.Sprintf(temp, totalMsgs, strings.Join(args, " "))
			} else {
				temp, _ := tr.GetString("purges_purged_messages")
				Text = fmt.Sprintf(temp, totalMsgs)
			}
			pMsg, err := bot.SendMessage(chat.Id, Text, helpers.Smarkdown())
			if err != nil {
				log.Error(err)
			}

			// Use timer instead of sleep for better resource management
			timer := time.NewTimer(3 * time.Second)
			<-timer.C
			_, err = pMsg.Delete(bot, nil)
			if err != nil {
				log.Error(err)
				return err
			}
		}
	} else {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
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
	user := ctx.EffectiveSender.User

	// Permission checks
	if !chat_status.RequireGroup(bot, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireBotAdmin(bot, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.CanBotDelete(bot, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(bot, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}
	if !chat_status.CanUserDelete(bot, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}

	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat

	if msg.ReplyToMessage == nil {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("purges_reply_to_delete")
		_, err := msg.Reply(bot, text, nil)
		if err != nil {
			log.Error(err)
			return err
		}

	} else {
		msgId := msg.ReplyToMessage.MessageId
		_, _ = bot.DeleteMessage(chat.Id, msgId, nil)
		_, _ = msg.Delete(bot, nil)
	}

	return ext.EndGroups
}

// deleteButtonHandler processes callback queries from delete buttons
// to remove specific messages, requiring admin permissions.
func (moduleStruct) deleteButtonHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	query := ctx.CallbackQuery
	chat := ctx.EffectiveChat
	user := ctx.EffectiveSender.User

	// permissions check
	if !chat_status.CanUserDelete(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}
	if !chat_status.CanBotDelete(b, ctx, nil, false) {
		return ext.EndGroups
	}

	args := strings.Split(query.Data, ".")
	msgId, _ := strconv.Atoi(args[1])

	_, err := b.DeleteMessage(chat.Id, int64(msgId), nil)
	// if err.Error() == "unable to deleteMessage: Bad Request: message to delete not found" {
	// 	log.WithFields(
	// 		log.Fields{
	// 			"chat": chat.Id,
	// 		},
	// 	).Error("error deleting message")
	// 	return ext.EndGroups
	// } else
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

// purgeFrom handles the /purgefrom command to mark a starting message
// for range deletion, requiring admin permissions.
func (moduleStruct) purgeFrom(bot *gotgbot.Bot, ctx *ext.Context) error {
	user := ctx.EffectiveSender.User

	// Permission checks
	if !chat_status.RequireGroup(bot, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireBotAdmin(bot, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.CanBotDelete(bot, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(bot, ctx, nil, ctx.EffectiveUser.Id, false) {
		return ext.EndGroups
	}
	if !chat_status.CanUserDelete(bot, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}

	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat

	if msg.ReplyToMessage != nil {
		TodelId := msg.ReplyToMessage.MessageId
		if existingId, ok := delMsgs.Load(chat.Id); ok && existingId == TodelId {
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			text, _ := tr.GetString("purges_message_marked")
			_, _ = msg.Reply(bot, text, nil)
			return ext.EndGroups
		}
		_, err := bot.DeleteMessage(chat.Id, msg.MessageId, nil)
		if err != nil {
			_, _ = msg.Reply(bot, err.Error(), nil)
			return ext.EndGroups
		}
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
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
		delMsgs.Store(chat.Id, TodelId)
		// Use timer with timeout for cleanup
		timer := time.NewTimer(30 * time.Second)
		<-timer.C
		if existingId, ok := delMsgs.Load(chat.Id); ok && existingId == TodelId {
			delMsgs.Delete(chat.Id)
		}
		_, err = pMsg.Delete(bot, nil)
		if err != nil {
			log.Error(err)
			return err
		}
	} else {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
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
	user := ctx.EffectiveSender.User

	// Permission checks
	if !chat_status.RequireGroup(bot, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireBotAdmin(bot, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.CanBotDelete(bot, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(bot, ctx, nil, ctx.EffectiveUser.Id, false) {
		return ext.EndGroups
	}
	if !chat_status.CanUserDelete(bot, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}

	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	args := ctx.Args()[1:]

	if msg.ReplyToMessage != nil {
		msgIdInterface, ok := delMsgs.Load(chat.Id)
		msgId := int64(0)
		if ok {
			msgId = msgIdInterface.(int64)
		}
		if msgId == 0 {
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
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
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			text, _ := tr.GetString("purges_use_del_single")
			_, err := msg.Reply(bot, text, nil)
			if err != nil {
				log.Error(err)
				return err
			}
			return ext.EndGroups
		}
		totalMsgs := int64(0)
		if deleteTo > msgId {
			totalMsgs = deleteTo - msgId + 1
		} else {
			totalMsgs = msgId - deleteTo + 1
		}
		purge := m.purgeMsgs(bot, chat, true, msgId, deleteTo)
		_, err := bot.DeleteMessage(chat.Id, msg.MessageId, nil)
		if err != nil {
			log.Error(err)
		}
		if purge {
			var Text string
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			if len(args) >= 1 {
				temp, _ := tr.GetString("purges_purged_with_reason")
				Text = fmt.Sprintf(temp, totalMsgs, strings.Join(args, " "))
			} else {
				temp, _ := tr.GetString("purges_purged_messages")
				Text = fmt.Sprintf(temp, totalMsgs)
			}
			pMsg, err := bot.SendMessage(chat.Id, Text, helpers.Smarkdown())
			if err != nil {
				log.Error(err)
			}
			// Use timer instead of sleep for better resource management
			timer := time.NewTimer(3 * time.Second)
			<-timer.C
			_, err = pMsg.Delete(bot, nil)
			if err != nil {
				log.Error(err)
				return err
			}
		}
	} else {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
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
	HelpModule.AbleMap.Store(purgesModule.moduleName, true)

	dispatcher.AddHandler(handlers.NewCommand("del", purgesModule.delCmd))
	dispatcher.AddHandler(handlers.NewCommand("purge", purgesModule.purge))
	dispatcher.AddHandler(handlers.NewCommand("purgefrom", purgesModule.purgeFrom))
	dispatcher.AddHandler(handlers.NewCommand("purgeto", purgesModule.purgeTo))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("deleteMsg."), purgesModule.deleteButtonHandler))
}
