package modules

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

type delMsgEntry struct {
	userID int64
	msgID  int64
}

var (
	purgesModule = moduleStruct{moduleName: "Purges"}
	delMsgs      = sync.Map{}
)

type PurgeWorker struct {
	errors     []error
	errorCount int
	mu         sync.Mutex
}

var errPurgeDeletePanicked = errors.New("message deletion panicked")

// purgeDeleteMessage deletes one message for a purge worker. A panic in the
// delete is logged and returned as errPurgeDeletePanicked, so it counts as a
// failed deletion and the worker moves on to the next message.
func purgeDeleteMessage(bot *gotgbot.Bot, chatID, messageID int64) (err error) {
	err = errPurgeDeletePanicked
	defer error_handling.RecoverFromPanic("purgeDeleteMessage", "purges")
	return helpers.DeleteMessageWithErrorHandling(bot, chatID, messageID)
}

func (moduleStruct) purgeMsgsConcurrent(bot *gotgbot.Bot, chat *gotgbot.Chat, pFrom bool, msgId, deleteTo int64) bool {
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

	loopFrom := msgId
	if !pFrom {
		loopFrom = msgId + 1
	}

	totalMessages := deleteTo - loopFrom + 1
	if totalMessages <= 0 {
		return true
	}

	if totalMessages <= 10 {
		for mId := deleteTo; mId >= loopFrom; mId-- {
			_ = helpers.DeleteMessageWithErrorHandling(bot, chat.Id, mId)
		}
		return true
	}

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
			defer error_handling.RecoverFromPanic("purgeMsgsConcurrent", "purges")
			for messageID := range jobs {
				if err := purgeDeleteMessage(bot, chat.Id, messageID); err != nil {
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

	if len(worker.errors) > 0 {
		log.WithFields(log.Fields{
			"chat_id":       chat.Id,
			"error_count":   worker.errorCount,
			"sample_errors": worker.errors,
		}).Warn("Some messages could not be deleted during purge")
	}

	return true
}

func (moduleStruct) purgeMsgs(bot *gotgbot.Bot, chat *gotgbot.Chat, pFrom bool, msgId, deleteTo int64) bool {
	return purgesModule.purgeMsgsConcurrent(bot, chat, pFrom, msgId, deleteTo)
}

func (m moduleStruct) purge(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	args := ctx.Args()[1:]

	if msg.ReplyToMessage != nil {
		msgId := msg.ReplyToMessage.MessageId
		deleteTo := msg.MessageId - 1
		totalMsgs := deleteTo - msgId + 1

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
			announcePurge(bot, ctx, chat, args, totalMsgs)
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

func (moduleStruct) delCmd(bot *gotgbot.Bot, ctx *ext.Context) error {
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

func (moduleStruct) purgeFrom(bot *gotgbot.Bot, ctx *ext.Context) error {
	user := chat_status.RequireUser(bot, ctx)
	if user == nil {
		chat_status.NewPermissionResponder(bot).Respond(ctx, "common_cannot_identify_user", "", chat_status.WithReply())
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
			// The chat holds one mark, and another message has it.
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			text, _ := tr.GetString("purges_mark_already_active")
			_, _ = msg.Reply(bot, text, nil)
			return ext.EndGroups
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
		delMsgs.Store(chat.Id, delMsgEntry{userID: user.Id, msgID: TodelId})

		go func(chatId, toDelId int64, msgToDelete *gotgbot.Message) {
			defer error_handling.RecoverFromPanic("purgeFromCleanup", "purges")
			time.Sleep(30 * time.Second)
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

func (m moduleStruct) purgeTo(bot *gotgbot.Bot, ctx *ext.Context) error {
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
		startId, endId := msgId, deleteTo
		if deleteTo < msgId {
			startId, endId = deleteTo, msgId
		}
		totalMsgs := endId - startId + 1

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

		delMsgs.Delete(chat.Id)

		purge := m.purgeMsgs(bot, chat, true, startId, endId)
		if err := helpers.DeleteMessageWithErrorHandling(bot, chat.Id, msg.MessageId); err != nil {
			log.Error(err)
		}
		if purge {
			announcePurge(bot, ctx, chat, args, totalMsgs)
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

// announcePurge posts the purge summary and deletes it after a short delay.
func announcePurge(bot *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, args []string, totalMsgs int64) {
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	var text string
	if len(args) >= 1 {
		temp, _ := tr.GetString("purges_purged_with_reason")
		text = fmt.Sprintf(temp, totalMsgs, escapeLegacyMarkdown(strings.Join(args, " ")))
	} else {
		temp, _ := tr.GetString("purges_purged_messages")
		text = fmt.Sprintf(temp, totalMsgs)
	}
	pMsg, err := bot.SendMessage(chat.Id, text, formatting.Smarkdown())
	if err != nil {
		log.Error(err)
		return
	}
	go func(msgToDelete *gotgbot.Message) {
		defer error_handling.RecoverFromPanic("purgeNotifyDelete", "purges")
		time.Sleep(3 * time.Second)
		_, _ = msgToDelete.Delete(bot, nil)
	}(pMsg)
}

// legacyMarkdownEscaper escapes the characters legacy Markdown treats as
// markup outside an entity, so user text sent with Smarkdown shows literally.
var legacyMarkdownEscaper = strings.NewReplacer("_", `\_`, "*", `\*`, "`", "\\`", "[", `\[`)

func escapeLegacyMarkdown(s string) string {
	return legacyMarkdownEscaper.Replace(s)
}

func purgeChecks(cmd string) []helpers.CheckFunc {
	return []helpers.CheckFunc{
		helpers.CheckDisabled(cmd),
		helpers.RequireGroup(),
		helpers.RequireBotAdmin(),
		helpers.CanBotDelete(),
		helpers.RequireUserAdmin(),
		helpers.CanUserDelete(),
	}
}

var (
	delDesc       = helpers.CommandDescriptor{Name: "del"}
	purgeDesc     = helpers.CommandDescriptor{Name: "purge"}
	purgeFromDesc = helpers.CommandDescriptor{Name: "purgefrom"}
	purgeToDesc   = helpers.CommandDescriptor{Name: "purgeto"}
)

func initPurgeDescs() {
	delDesc.RequiredChecks = purgeChecks("del")
	purgeDesc.RequiredChecks = purgeChecks("purge")
	purgeFromDesc.RequiredChecks = purgeChecks("purgefrom")
	purgeToDesc.RequiredChecks = purgeChecks("purgeto")
}

func LoadPurges(dispatcher *ext.Dispatcher) {
	SetModuleEnabled(purgesModule.moduleName, true)
	initPurgeDescs()

	helpers.WrapCommand(dispatcher, delDesc, pipelineHandler(purgesModule.delCmd))
	helpers.WrapCommand(dispatcher, purgeDesc, pipelineHandler(purgesModule.purge))
	helpers.WrapCommand(dispatcher, purgeFromDesc, pipelineHandler(purgesModule.purgeFrom))
	helpers.WrapCommand(dispatcher, purgeToDesc, pipelineHandler(purgesModule.purgeTo))
}

func init() {
	RegisterLegacyModule("Purges", 90, LoadPurges)
	initPurgeDescs()
	RegisterAnonymousAdminHandler("purge", anonPipelineHandler(purgeDesc, purgesModule.purge))
	RegisterAnonymousAdminHandler("del", anonPipelineHandler(delDesc, purgesModule.delCmd))
	RegisterAnonymousAdminHandler("purgefrom", anonPipelineHandler(purgeFromDesc, purgesModule.purgeFrom))
	RegisterAnonymousAdminHandler("purgeto", anonPipelineHandler(purgeToDesc, purgesModule.purgeTo))
}
