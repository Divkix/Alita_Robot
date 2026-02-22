package modules

import (
	"fmt"
	"strings"

	tgmd2html "github.com/PaulSonOfLars/gotg_md2html"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/utils/helpers"

	"github.com/divkix/Alita_Robot/alita/utils/string_handling"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
)

var pinsModule = moduleStruct{
	moduleName:   "Pins",
	handlerGroup: 10,
}

type pinType struct {
	MsgText  string
	FileID   string
	DataType int
}

// checkPinned monitors channel messages and handles them according to
// AntiChannelPin and CleanLinked settings - either unpinning or deleting.
func (moduleStruct) checkPinned(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	pinprefs := db.GetPinData(chat.Id)

	if pinprefs.CleanLinked {
		if err := helpers.DeleteMessageWithErrorHandling(b, chat.Id, msg.MessageId); err != nil {
			log.Error(err)
			return err
		}
	} else if pinprefs.AntiChannelPin {
		_, err := b.UnpinChatMessage(chat.Id,
			&gotgbot.UnpinChatMessageOpts{
				MessageId: &msg.MessageId,
			},
		)
		if err != nil {
			log.Error(err)
			return err
		}
	}

	return ext.ContinueGroups
}

// unpin handles the /unpin command to unpin messages, either the latest
// pinned message or a specific replied message, requiring admin permissions.
func (moduleStruct) unpin(b *gotgbot.Bot, ctx *ext.Context) error {
	user := chat_status.RequireUser(b, ctx, false)
	if user == nil {
		return ext.EndGroups
	}
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage

	// Check permissions
	if !chat_status.RequireGroup(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireBotAdmin(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.CanBotPin(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.CanUserPin(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}

	var (
		replyText  string
		replyMsgId int64
	)

	if replyMsg := msg.ReplyToMessage; replyMsg != nil {
		replyMsgId = replyMsg.MessageId
	} else {
		replyMsgId = msg.MessageId
	}

	if rMsg := msg.ReplyToMessage; rMsg != nil {
		if rMsg.PinnedMessage == nil {
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			replyText, _ = tr.GetString("pins_unpin_not_pinned")
		} else {
			_, err := b.UnpinChatMessage(chat.Id, &gotgbot.UnpinChatMessageOpts{MessageId: &rMsg.MessageId})
			if err != nil {
				log.Error(err)
				return err
			}
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			replyText, _ = tr.GetString("pins_unpinned_message")
			replyMsgId = rMsg.MessageId
		}
	} else {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		replyText, _ = tr.GetString("pins_unpinned_last")
		_, err := b.UnpinChatMessage(chat.Id, nil)
		if err != nil {
			log.Error(err)
			return err
		}
	}

	_, err := msg.Reply(b, replyText,
		&gotgbot.SendMessageOpts{
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

// unpinallCallback processes callback queries for the unpin all confirmation
// dialog, handling the user's yes/no response.
func (moduleStruct) unpinallCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	query := ctx.CallbackQuery
	chat := ctx.EffectiveChat

	action := ""
	if decoded, ok := decodeCallbackData(query.Data, "unpinallbtn"); ok {
		action, _ = decoded.Field("a")
	} else {
		switch query.Data {
		case "unpinallbtn(yes)":
			action = "yes"
		case "unpinallbtn(no)":
			action = "no"
		}
	}

	switch action {
	case "yes":
		status, err := b.UnpinAllChatMessages(chat.Id, nil)
		if !status && err != nil {
			log.Errorf("[Pin] UnpinAllChatMessages for chat %d: %v", chat.Id, err)
			return err
		}
		tr := i18n.MustNewTranslator(db.GetLanguage(&ext.Context{EffectiveChat: chat}))
		text, _ := tr.GetString("pins_unpin_all_success")
		_, _, erredit := query.Message.EditText(b, text, nil)
		if erredit != nil {
			log.Errorf("[Pin] EditText failed for chat %d: %v", chat.Id, erredit)
			return erredit
		}
	case "no":
		tr := i18n.MustNewTranslator(db.GetLanguage(&ext.Context{EffectiveChat: chat}))
		text, _ := tr.GetString("pins_unpin_all_cancelled")
		_, _, err := query.Message.EditText(b, text, nil)
		if err != nil {
			log.Errorf("[Pin] EditText failed for chat %d: %v", chat.Id, err)
			return err
		}
	default:
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "Invalid request."})
		return ext.EndGroups
	}
	return ext.EndGroups
}

// unpinAll handles the /unpinall command to unpin all messages in the chat
// with a confirmation dialog, requiring admin permissions.
func (moduleStruct) unpinAll(b *gotgbot.Bot, ctx *ext.Context) error {
	user := chat_status.RequireUser(b, ctx, false)
	if user == nil {
		return ext.EndGroups
	}

	if !chat_status.RequireGroup(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireBotAdmin(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.CanBotPin(b, ctx, nil, false) {
		return ext.EndGroups
	}

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	text, _ := tr.GetString("pins_unpin_all_confirm")
	yesText, _ := tr.GetString("button_yes")
	noText, _ := tr.GetString("button_no")
	_, err := b.SendMessage(ctx.EffectiveChat.Id, text,
		&gotgbot.SendMessageOpts{
			ReplyMarkup: gotgbot.InlineKeyboardMarkup{
				InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
					{
						{Text: yesText, CallbackData: encodeCallbackData("unpinallbtn", map[string]string{"a": "yes"}, "unpinallbtn(yes)")},
						{Text: noText, CallbackData: encodeCallbackData("unpinallbtn", map[string]string{"a": "no"}, "unpinallbtn(no)")},
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

// permaPin handles the /permapin command to create and pin a new message
// with custom content and buttons, requiring admin permissions.
func (m moduleStruct) permaPin(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	user := chat_status.RequireUser(b, ctx, false)
	if user == nil {
		return ext.EndGroups
	}
	args := ctx.Args()

	// permission checks
	if !chat_status.RequireGroup(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireBotAdmin(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.CanUserPin(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}
	if !chat_status.CanBotPin(b, ctx, nil, false) {
		return ext.EndGroups
	}

	// if command is empty (i.e. Without Arguments) not replied to a message, return and end group
	if len(args) == 1 && msg.ReplyToMessage == nil {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("pins_permapin_reply_or_text")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	var (
		buttons []tgmd2html.ButtonV2
		pinT    = pinType{}
	)

	pinT.FileID, pinT.MsgText, pinT.DataType, buttons = m.GetPinType(msg)
	if pinT.DataType == -1 {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("pins_permapin_unsupported")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	keyb := helpers.BuildKeyboard(helpers.ConvertButtonV2ToDbButton(buttons))

	// Validate that enum function exists before calling to prevent panic from invalid dataType
	// This protects against database corruption or invalid message types
	pinFunc, exists := PinsEnumFuncMap[pinT.DataType]
	if !exists || pinFunc == nil {
		log.Errorf("Invalid or missing pin type: %d, cannot send permapin", pinT.DataType)
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("pins_permapin_unsupported")
		_, err := msg.Reply(b, text, helpers.Shtml())
		return err
	}

	ppmsg, err := pinFunc(b, ctx, pinT, &gotgbot.InlineKeyboardMarkup{InlineKeyboard: keyb}, 0)
	if err != nil {
		log.Error(err)
		return err
	}

	msgToPin := ppmsg.MessageId
	pin, err := b.PinChatMessage(chat.Id, msgToPin, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	if pin {
		pinLink := helpers.GetMessageLinkFromMessageId(chat, msgToPin)
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		temp, _ := tr.GetString("pins_pinned_message")
		text := fmt.Sprintf(temp, pinLink)
		_, err = msg.Reply(b, text,
			&gotgbot.SendMessageOpts{
				ParseMode: helpers.HTML,
				ReplyParameters: &gotgbot.ReplyParameters{
					MessageId:                msgToPin,
					AllowSendingWithoutReply: true,
				},
				LinkPreviewOptions: &gotgbot.LinkPreviewOptions{
					IsDisabled: true,
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

// pin handles the /pin command to pin a replied message with options
// for silent or loud pinning, requiring admin permissions.
func (moduleStruct) pin(b *gotgbot.Bot, ctx *ext.Context) error {
	user := chat_status.RequireUser(b, ctx, false)
	if user == nil {
		return ext.EndGroups
	}
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	isSilent := true
	args := ctx.Args()

	// permission checks
	if !chat_status.RequireGroup(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireBotAdmin(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.CanUserPin(b, ctx, nil, user.Id, false) {
		return ext.EndGroups
	}
	if !chat_status.CanBotPin(b, ctx, nil, false) {
		return ext.EndGroups
	}

	if msg.ReplyToMessage == nil {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("pins_reply_to_pin")
		_, err := msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	prevMessage := msg.ReplyToMessage
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	pinMsg, _ := tr.GetString("pins_pinned_message")

	if len(args) > 1 {
		isSilent = !string_handling.FindInStringSlice([]string{"notify", "violent", "loud"}, args[1])
		if !isSilent {
			pinMsg, _ = tr.GetString("pins_pinned_message_loud")
		}
	}

	pin, err := b.PinChatMessage(chat.Id,
		prevMessage.MessageId,
		&gotgbot.PinChatMessageOpts{
			DisableNotification: isSilent,
		},
	)
	if err != nil {
		log.Error(err)
		return err
	}

	if pin {
		pinLink := helpers.GetMessageLinkFromMessageId(chat, prevMessage.MessageId)
		text := fmt.Sprintf(pinMsg, pinLink)
		_, err = prevMessage.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
	}

	return ext.EndGroups
}

// antichannelpin handles the /antichannelpin command to toggle automatic
// unpinning of channel-pinned messages, requiring admin permissions.
func (moduleStruct) antichannelpin(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	// connection status
	connectedChat := helpers.IsUserConnected(b, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat
	args := ctx.Args()

	if len(args) >= 2 {
		switch strings.ToLower(args[1]) {
		case "on", "yes", "true":
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			if err := db.SetAntiChannelPin(chat.Id, true); err != nil {
				log.Errorf("[Pins] SetAntiChannelPin failed for chat %d: %v", chat.Id, err)
				errText, _ := tr.GetString("common_settings_save_failed")
				_, _ = msg.Reply(b, errText, helpers.Shtml())
				return ext.EndGroups
			}
			text, _ := tr.GetString("pins_antichannelpin_enabled")
			_, err := msg.Reply(b, text, helpers.Shtml())
			if err != nil {
				log.Error(err)
				return err
			}
		case "off", "no", "false":
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			if err := db.SetAntiChannelPin(chat.Id, false); err != nil {
				log.Errorf("[Pins] SetAntiChannelPin failed for chat %d: %v", chat.Id, err)
				errText, _ := tr.GetString("common_settings_save_failed")
				_, _ = msg.Reply(b, errText, helpers.Shtml())
				return ext.EndGroups
			}
			text, _ := tr.GetString("pins_antichannelpin_disabled")
			_, err := msg.Reply(b, text, helpers.Shtml())
			if err != nil {
				log.Error(err)
				return err
			}
		default:
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			text, _ := tr.GetString("pins_input_not_recognized")
			_, err := msg.Reply(b, text, helpers.Shtml())
			if err != nil {
				log.Error(err)
				return err
			}
		}
	} else {
		pinprefs := db.GetPinData(chat.Id)
		if pinprefs.AntiChannelPin {
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			temp, _ := tr.GetString("pins_antichannelpin_current_enabled")
			text := fmt.Sprintf(temp, chat.Title)
			_, err := msg.Reply(b, text, helpers.Shtml())
			if err != nil {
				log.Error(err)
				return err
			}
		} else {
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			temp, _ := tr.GetString("pins_antichannelpin_current_disabled")
			text := fmt.Sprintf(temp, chat.Title)
			_, err := msg.Reply(b, text, helpers.Shtml())
			if err != nil {
				log.Error(err)
				return err
			}
		}
	}

	return ext.EndGroups
}

// cleanlinked handles the /cleanlinked command to toggle automatic
// deletion of linked channel messages, requiring admin permissions.
func (moduleStruct) cleanlinked(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	// connection status
	connectedChat := helpers.IsUserConnected(b, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
	chat := ctx.EffectiveChat
	args := ctx.Args()

	if len(args) >= 2 {
		switch strings.ToLower(args[1]) {
		case "on", "yes", "true":
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			if err := db.SetCleanLinked(chat.Id, true); err != nil {
				log.Errorf("[Pins] SetCleanLinked failed for chat %d: %v", chat.Id, err)
				errText, _ := tr.GetString("common_settings_save_failed")
				_, _ = msg.Reply(b, errText, helpers.Shtml())
				return ext.EndGroups
			}
			text, _ := tr.GetString("pins_cleanlinked_enabled")
			_, err := msg.Reply(b, text, helpers.Shtml())
			if err != nil {
				log.Error(err)
				return err
			}
		case "off", "no", "false":
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			if err := db.SetCleanLinked(chat.Id, false); err != nil {
				log.Errorf("[Pins] SetCleanLinked failed for chat %d: %v", chat.Id, err)
				errText, _ := tr.GetString("common_settings_save_failed")
				_, _ = msg.Reply(b, errText, helpers.Shtml())
				return ext.EndGroups
			}
			text, _ := tr.GetString("pins_cleanlinked_disabled")
			_, err := msg.Reply(b, text, helpers.Shtml())
			if err != nil {
				log.Error(err)
				return err
			}
		default:
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			text, _ := tr.GetString("pins_input_not_recognized")
			_, err := msg.Reply(b, text, helpers.Shtml())
			if err != nil {
				log.Error(err)
				return err
			}
		}
	} else {
		pinprefs := db.GetPinData(chat.Id)
		if pinprefs.CleanLinked {
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			temp, _ := tr.GetString("pins_cleanlinked_current_enabled")
			text := fmt.Sprintf(temp, chat.Title)
			_, err := msg.Reply(b, text, helpers.Shtml())
			if err != nil {
				log.Error(err)
				return err
			}
		} else {
			tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
			temp, _ := tr.GetString("pins_cleanlinked_current_disabled")
			text := fmt.Sprintf(temp, chat.Title)
			_, err := msg.Reply(b, text, helpers.Shtml())
			if err != nil {
				log.Error(err)
				return err
			}
		}
	}

	return ext.EndGroups
}

// pinned handles the /pinned command to display a link to the latest
// pinned message in the chat with a convenient button.
func (moduleStruct) pinned(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage

	// permission checks
	if !chat_status.RequireGroup(b, ctx, nil, false) {
		return ext.EndGroups
	}
	if !chat_status.RequireBotAdmin(b, ctx, nil, false) {
		return ext.EndGroups
	}

	var (
		pinLink    string
		replyMsgId int64
	)

	if reply := msg.ReplyToMessage; reply != nil {
		replyMsgId = reply.MessageId
	} else {
		replyMsgId = msg.MessageId
	}

	chatInfo, err := b.GetChat(chat.Id, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	pinnedMsg := chatInfo.PinnedMessage

	if pinnedMsg == nil {
		tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
		text, _ := tr.GetString("pins_no_pinned_message")
		_, err = msg.Reply(b, text, helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return err
	}

	pinLink = helpers.GetMessageLinkFromMessageId(chat, pinnedMsg.MessageId)

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	temp, _ := tr.GetString("pins_here_is_pinned")
	text := fmt.Sprintf(temp, pinLink)
	buttonText, _ := tr.GetString("pins_pinned_message_button")
	_, err = msg.Reply(b, text,
		&gotgbot.SendMessageOpts{
			ParseMode: helpers.HTML,
			LinkPreviewOptions: &gotgbot.LinkPreviewOptions{
				IsDisabled: true,
			},
			ReplyParameters: &gotgbot.ReplyParameters{
				MessageId:                replyMsgId,
				AllowSendingWithoutReply: true,
			},
			ReplyMarkup: gotgbot.InlineKeyboardMarkup{
				InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
					{
						{Text: buttonText, Url: pinLink},
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

// PinsEnumFuncMap
// A rather very complicated PinsEnumFuncMap Variable made by me to send filters in an appropriate way
var PinsEnumFuncMap = map[int]func(b *gotgbot.Bot, ctx *ext.Context, pinT pinType, keyb *gotgbot.InlineKeyboardMarkup, replyMsgId int64) (*gotgbot.Message, error){
	db.TEXT: func(b *gotgbot.Bot, ctx *ext.Context, pinT pinType, keyb *gotgbot.InlineKeyboardMarkup, replyMsgId int64) (*gotgbot.Message, error) {
		return b.SendMessage(
			ctx.EffectiveChat.Id,
			pinT.MsgText,
			&gotgbot.SendMessageOpts{
				ParseMode: helpers.HTML,
				LinkPreviewOptions: &gotgbot.LinkPreviewOptions{
					IsDisabled: true,
				},
				ReplyMarkup: keyb,
				ReplyParameters: &gotgbot.ReplyParameters{
					MessageId:                replyMsgId,
					AllowSendingWithoutReply: true,
				},
				MessageThreadId: ctx.EffectiveMessage.MessageThreadId,
			},
		)
	},
	db.STICKER: func(b *gotgbot.Bot, ctx *ext.Context, pinT pinType, keyb *gotgbot.InlineKeyboardMarkup, replyMsgId int64) (*gotgbot.Message, error) {
		// Validate FileID is not empty to prevent API errors
		if pinT.FileID == "" {
			log.Warnf("Empty FileID for STICKER type in chat %d, falling back to text message", ctx.EffectiveChat.Id)
			return b.SendMessage(
				ctx.EffectiveChat.Id,
				pinT.MsgText,
				&gotgbot.SendMessageOpts{
					ReplyParameters: &gotgbot.ReplyParameters{
						MessageId:                replyMsgId,
						AllowSendingWithoutReply: true,
					},
					ParseMode:       helpers.HTML,
					ReplyMarkup:     keyb,
					MessageThreadId: ctx.EffectiveMessage.MessageThreadId,
				},
			)
		}
		return b.SendSticker(
			ctx.EffectiveChat.Id,
			gotgbot.InputFileByID(pinT.FileID),
			&gotgbot.SendStickerOpts{
				ReplyParameters: &gotgbot.ReplyParameters{
					MessageId:                replyMsgId,
					AllowSendingWithoutReply: true,
				},
				ReplyMarkup:     keyb,
				MessageThreadId: ctx.EffectiveMessage.MessageThreadId,
			},
		)
	},
	db.DOCUMENT: func(b *gotgbot.Bot, ctx *ext.Context, pinT pinType, keyb *gotgbot.InlineKeyboardMarkup, replyMsgId int64) (*gotgbot.Message, error) {
		// Validate FileID is not empty to prevent API errors
		if pinT.FileID == "" {
			log.Warnf("Empty FileID for DOCUMENT type in chat %d, falling back to text message", ctx.EffectiveChat.Id)
			return b.SendMessage(
				ctx.EffectiveChat.Id,
				pinT.MsgText,
				&gotgbot.SendMessageOpts{
					ReplyParameters: &gotgbot.ReplyParameters{
						MessageId:                replyMsgId,
						AllowSendingWithoutReply: true,
					},
					ParseMode:       helpers.HTML,
					ReplyMarkup:     keyb,
					MessageThreadId: ctx.EffectiveMessage.MessageThreadId,
				},
			)
		}
		return b.SendDocument(
			ctx.EffectiveChat.Id,
			gotgbot.InputFileByID(pinT.FileID),
			&gotgbot.SendDocumentOpts{
				ReplyParameters: &gotgbot.ReplyParameters{
					MessageId:                replyMsgId,
					AllowSendingWithoutReply: true,
				},
				ParseMode:       helpers.HTML,
				ReplyMarkup:     keyb,
				Caption:         pinT.MsgText,
				MessageThreadId: ctx.EffectiveMessage.MessageThreadId,
			},
		)
	},
	db.PHOTO: func(b *gotgbot.Bot, ctx *ext.Context, pinT pinType, keyb *gotgbot.InlineKeyboardMarkup, replyMsgId int64) (*gotgbot.Message, error) {
		// Validate FileID is not empty to prevent "there is no photo in the request" errors
		if pinT.FileID == "" {
			log.Warnf("Empty FileID for PHOTO type in chat %d, falling back to text message", ctx.EffectiveChat.Id)
			return b.SendMessage(
				ctx.EffectiveChat.Id,
				pinT.MsgText,
				&gotgbot.SendMessageOpts{
					ReplyParameters: &gotgbot.ReplyParameters{
						MessageId:                replyMsgId,
						AllowSendingWithoutReply: true,
					},
					ParseMode:       helpers.HTML,
					ReplyMarkup:     keyb,
					MessageThreadId: ctx.EffectiveMessage.MessageThreadId,
				},
			)
		}
		return b.SendPhoto(
			ctx.EffectiveChat.Id,
			gotgbot.InputFileByID(pinT.FileID),
			&gotgbot.SendPhotoOpts{
				ReplyParameters: &gotgbot.ReplyParameters{
					MessageId:                replyMsgId,
					AllowSendingWithoutReply: true,
				},
				ParseMode:       helpers.HTML,
				ReplyMarkup:     keyb,
				Caption:         pinT.MsgText,
				MessageThreadId: ctx.EffectiveMessage.MessageThreadId,
			},
		)
	},
	db.AUDIO: func(b *gotgbot.Bot, ctx *ext.Context, pinT pinType, keyb *gotgbot.InlineKeyboardMarkup, replyMsgId int64) (*gotgbot.Message, error) {
		// Validate FileID is not empty to prevent API errors
		if pinT.FileID == "" {
			log.Warnf("Empty FileID for AUDIO type in chat %d, falling back to text message", ctx.EffectiveChat.Id)
			return b.SendMessage(
				ctx.EffectiveChat.Id,
				pinT.MsgText,
				&gotgbot.SendMessageOpts{
					ReplyParameters: &gotgbot.ReplyParameters{
						MessageId:                replyMsgId,
						AllowSendingWithoutReply: true,
					},
					ParseMode:       helpers.HTML,
					ReplyMarkup:     keyb,
					MessageThreadId: ctx.EffectiveMessage.MessageThreadId,
				},
			)
		}
		return b.SendAudio(
			ctx.EffectiveChat.Id,
			gotgbot.InputFileByID(pinT.FileID),
			&gotgbot.SendAudioOpts{
				ReplyParameters: &gotgbot.ReplyParameters{
					MessageId:                replyMsgId,
					AllowSendingWithoutReply: true,
				},
				ParseMode:       helpers.HTML,
				ReplyMarkup:     keyb,
				Caption:         pinT.MsgText,
				MessageThreadId: ctx.EffectiveMessage.MessageThreadId,
			},
		)
	},
	db.VOICE: func(b *gotgbot.Bot, ctx *ext.Context, pinT pinType, keyb *gotgbot.InlineKeyboardMarkup, replyMsgId int64) (*gotgbot.Message, error) {
		// Validate FileID is not empty to prevent API errors
		if pinT.FileID == "" {
			log.Warnf("Empty FileID for VOICE type in chat %d, falling back to text message", ctx.EffectiveChat.Id)
			return b.SendMessage(
				ctx.EffectiveChat.Id,
				pinT.MsgText,
				&gotgbot.SendMessageOpts{
					ReplyParameters: &gotgbot.ReplyParameters{
						MessageId:                replyMsgId,
						AllowSendingWithoutReply: true,
					},
					ParseMode:       helpers.HTML,
					ReplyMarkup:     keyb,
					MessageThreadId: ctx.EffectiveMessage.MessageThreadId,
				},
			)
		}
		return b.SendVoice(
			ctx.EffectiveChat.Id,
			gotgbot.InputFileByID(pinT.FileID),
			&gotgbot.SendVoiceOpts{
				ReplyParameters: &gotgbot.ReplyParameters{
					MessageId:                replyMsgId,
					AllowSendingWithoutReply: true,
				},
				ParseMode:       helpers.HTML,
				ReplyMarkup:     keyb,
				Caption:         pinT.MsgText,
				MessageThreadId: ctx.EffectiveMessage.MessageThreadId,
			},
		)
	},
	db.VIDEO: func(b *gotgbot.Bot, ctx *ext.Context, pinT pinType, keyb *gotgbot.InlineKeyboardMarkup, replyMsgId int64) (*gotgbot.Message, error) {
		// Validate FileID is not empty to prevent API errors
		if pinT.FileID == "" {
			log.Warnf("Empty FileID for VIDEO type in chat %d, falling back to text message", ctx.EffectiveChat.Id)
			return b.SendMessage(
				ctx.EffectiveChat.Id,
				pinT.MsgText,
				&gotgbot.SendMessageOpts{
					ReplyParameters: &gotgbot.ReplyParameters{
						MessageId:                replyMsgId,
						AllowSendingWithoutReply: true,
					},
					ParseMode:       helpers.HTML,
					ReplyMarkup:     keyb,
					MessageThreadId: ctx.EffectiveMessage.MessageThreadId,
				},
			)
		}
		return b.SendVideo(
			ctx.EffectiveChat.Id,
			gotgbot.InputFileByID(pinT.FileID),
			&gotgbot.SendVideoOpts{
				ReplyParameters: &gotgbot.ReplyParameters{
					MessageId:                replyMsgId,
					AllowSendingWithoutReply: true,
				},
				ParseMode:       helpers.HTML,
				ReplyMarkup:     keyb,
				Caption:         pinT.MsgText,
				MessageThreadId: ctx.EffectiveMessage.MessageThreadId,
			},
		)
	},
	db.VideoNote: func(b *gotgbot.Bot, ctx *ext.Context, pinT pinType, keyb *gotgbot.InlineKeyboardMarkup, replyMsgId int64) (*gotgbot.Message, error) {
		// Validate FileID is not empty to prevent API errors
		if pinT.FileID == "" {
			log.Warnf("Empty FileID for VideoNote type in chat %d, falling back to text message", ctx.EffectiveChat.Id)
			return b.SendMessage(
				ctx.EffectiveChat.Id,
				pinT.MsgText,
				&gotgbot.SendMessageOpts{
					ReplyParameters: &gotgbot.ReplyParameters{
						MessageId:                replyMsgId,
						AllowSendingWithoutReply: true,
					},
					ParseMode:       helpers.HTML,
					ReplyMarkup:     keyb,
					MessageThreadId: ctx.EffectiveMessage.MessageThreadId,
				},
			)
		}
		return b.SendVideoNote(
			ctx.EffectiveChat.Id,
			gotgbot.InputFileByID(pinT.FileID),
			&gotgbot.SendVideoNoteOpts{
				ReplyParameters: &gotgbot.ReplyParameters{
					MessageId:                replyMsgId,
					AllowSendingWithoutReply: true,
				},
				ReplyMarkup:     keyb,
				MessageThreadId: ctx.EffectiveMessage.MessageThreadId,
			},
		)
	},
}

// GetPinType analyzes a message to determine its content type and extract
// relevant data for pinning, including file IDs, text, and buttons.
func (moduleStruct) GetPinType(msg *gotgbot.Message) (fileid, text string, dataType int, buttons []tgmd2html.ButtonV2) {
	dataType = -1 // not defined datatype; invalid filter
	var (
		rawText string
		args    = strings.Split(msg.Text, " ")[1:]
	)

	if reply := msg.ReplyToMessage; reply != nil {
		if reply.Text == "" {
			rawText = reply.OriginalCaptionMDV2()
		} else {
			rawText = reply.OriginalMDV2()
		}
	} else {
		// Extract text safely to prevent panic on malformed input
		var parts []string
		if msg.Text == "" {
			parts = strings.SplitN(msg.OriginalCaptionMDV2(), " ", 2)
		} else {
			parts = strings.SplitN(msg.OriginalMDV2(), " ", 2)
		}
		if len(parts) >= 2 {
			rawText = parts[1]
		}
		// If len(parts) < 2, rawText stays empty - handled by later validation
	}

	// get text and buttons
	text, buttons = tgmd2html.MD2HTMLButtonsV2(rawText)

	if len(args) >= 1 && msg.ReplyToMessage == nil {
		dataType = db.TEXT
	} else if msg.ReplyToMessage != nil && len(args) >= 0 {
		if len(args) >= 0 && msg.ReplyToMessage.Text != "" {
			dataType = db.TEXT
		} else if msg.ReplyToMessage.Sticker != nil {
			fileid = msg.ReplyToMessage.Sticker.FileId
			dataType = db.STICKER
		} else if msg.ReplyToMessage.Document != nil {
			fileid = msg.ReplyToMessage.Document.FileId
			dataType = db.DOCUMENT
		} else if len(msg.ReplyToMessage.Photo) > 0 {
			fileid = msg.ReplyToMessage.Photo[len(msg.ReplyToMessage.Photo)-1].FileId // using -1 index to get best photo quality
			dataType = db.PHOTO
		} else if msg.ReplyToMessage.Audio != nil {
			fileid = msg.ReplyToMessage.Audio.FileId
			dataType = db.AUDIO
		} else if msg.ReplyToMessage.Voice != nil {
			fileid = msg.ReplyToMessage.Voice.FileId
			dataType = db.VOICE
		} else if msg.ReplyToMessage.Video != nil {
			fileid = msg.ReplyToMessage.Video.FileId
			dataType = db.VIDEO
		} else if msg.ReplyToMessage.VideoNote != nil {
			fileid = msg.ReplyToMessage.VideoNote.FileId
			dataType = db.VideoNote
		}
	}

	return
}

// LoadPin registers all pins module handlers with the dispatcher,
// including pin management commands and channel message monitoring.
func LoadPin(dispatcher *ext.Dispatcher) {
	HelpModule.AbleMap.Store(pinsModule.moduleName, true)

	dispatcher.AddHandler(handlers.NewCommand("unpin", pinsModule.unpin))
	dispatcher.AddHandler(handlers.NewCommand("unpinall", pinsModule.unpinAll))
	dispatcher.AddHandler(handlers.NewCommand("pin", pinsModule.pin))
	dispatcher.AddHandler(handlers.NewCommand("pinned", pinsModule.pinned))
	dispatcher.AddHandler(handlers.NewCommand("antichannelpin", pinsModule.antichannelpin))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("unpinallbtn"), pinsModule.unpinallCallback))
	dispatcher.AddHandlerToGroup(
		handlers.NewMessage(
			func(msg *gotgbot.Message) bool {
				return msg.GetSender().IsLinkedChannel()
			},
			pinsModule.checkPinned,
		),
		pinsModule.handlerGroup,
	)
	dispatcher.AddHandler(handlers.NewCommand("permapin", pinsModule.permaPin))
	dispatcher.AddHandler(handlers.NewCommand("cleanlinked", pinsModule.cleanlinked))
}
