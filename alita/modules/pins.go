package modules

import (
	"strings"

	tgmd2html "github.com/PaulSonOfLars/gotg_md2html"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	log "github.com/sirupsen/logrus"

	"github.com/divideprojects/Alita_Robot/alita/utils/helpers"

	"github.com/divideprojects/Alita_Robot/alita/utils/string_handling"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divideprojects/Alita_Robot/alita/db"
	"github.com/divideprojects/Alita_Robot/alita/i18n"
	"github.com/divideprojects/Alita_Robot/alita/utils/chat_status"
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
		_, err := b.DeleteMessage(chat.Id, msg.MessageId, nil)
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
	user := ctx.EffectiveSender.User
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage

	// Get translator for the chat
	translator := i18n.MustNewTranslator(db.GetLanguage(ctx))

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
			replyText = translator.Message("pins_replied_not_pinned", nil)
		} else {
			_, err := b.UnpinChatMessage(chat.Id, &gotgbot.UnpinChatMessageOpts{MessageId: &rMsg.MessageId})
			if err != nil {
				log.Error(err)
				return err
			}
			replyText = translator.Message("pins_unpinned_this", nil)
			replyMsgId = rMsg.MessageId
		}
	} else {
		replyText = translator.Message("pins_unpinned_last", nil)
		_, err := b.UnpinChatMessage(chat.Id, nil)
		if err != nil {
			// if err.Error() == "unable to unpinChatMessage: Bad Request: message to unpin not found" {
			// 	replyText = "No pinned message found."
			// } else
			// if err != nil {
			log.Error(err)
			return err
			// }
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

	// Get translator for the chat
	translator := i18n.MustNewTranslator("en") // fallback for callback queries

	switch query.Data {
	case "unpinallbtn(yes)":
		status, err := b.UnpinAllChatMessages(chat.Id, nil)
		if !status && err != nil {
			log.Errorf("[Pin] UnpinAllChatMessages: %d", chat.Id)
			return err
		}
		_, _, erredit := query.Message.EditText(b, translator.Message("pins_unpinned_all_success", nil), nil)
		if erredit != nil {
			log.Errorf("[Pin] UnpinAllChatMessages: %d", chat.Id)
			return err
		}
	case "unpinallbtn(no)":
		_, _, err := query.Message.EditText(b, translator.Message("pins_unpin_cancelled", nil), nil)
		if err != nil {
			log.Errorf("[Pin] UnpinAllChatMessages: %d", chat.Id)
			return err
		}
	}
	return ext.EndGroups
}

// unpinAll handles the /unpinall command to unpin all messages in the chat
// with a confirmation dialog, requiring admin permissions.
func (moduleStruct) unpinAll(b *gotgbot.Bot, ctx *ext.Context) error {
	user := ctx.EffectiveSender.User

	// Get translator for the chat
	translator := i18n.MustNewTranslator(db.GetLanguage(ctx))

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

	_, err := b.SendMessage(ctx.EffectiveChat.Id, translator.Message("pins_confirm_unpin_all", nil),
		&gotgbot.SendMessageOpts{
			ReplyMarkup: gotgbot.InlineKeyboardMarkup{
				InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
					{
						{Text: "Yes", CallbackData: "unpinallbtn(yes)"},
						{Text: "No", CallbackData: "unpinallbtn(no)"},
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
	user := ctx.EffectiveSender.User
	args := ctx.Args()

	// Get translator for the chat
	translator := i18n.MustNewTranslator(db.GetLanguage(ctx))

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
		_, err := msg.Reply(b, translator.Message("pins_reply_or_text_needed", nil), helpers.Shtml())
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
		_, err := msg.Reply(b, translator.Message("pins_permapin_not_supported", nil), helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	keyb := helpers.BuildKeyboard(helpers.ConvertButtonV2ToDbButton(buttons))

	// enum func works here
	ppmsg, err := PinsEnumFuncMap[pinT.DataType](b, ctx, pinT, &gotgbot.InlineKeyboardMarkup{InlineKeyboard: keyb}, 0)
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
		_, err = msg.Reply(b,
			translator.Message("pins_pinned_message", i18n.Params{
				"link": pinLink,
			}),
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
	user := ctx.EffectiveSender.User
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	isSilent := true
	args := ctx.Args

	// Get translator for the chat
	translator := i18n.MustNewTranslator(db.GetLanguage(ctx))

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
		_, err := msg.Reply(b, translator.Message("pins_reply_to_pin", nil), helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	prevMessage := msg.ReplyToMessage
	var pinMsg string

	if len(args()) > 0 {
		isSilent = !string_handling.FindInStringSlice([]string{"notify", "violent", "loud"}, args()[0])
		if !isSilent {
			pinMsg = "pins_pinned_message_loudly"
		} else {
			pinMsg = "pins_pinned_message"
		}
	} else {
		pinMsg = "pins_pinned_message"
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
		_, err = prevMessage.Reply(b,
			translator.Message(pinMsg, i18n.Params{
				"link": pinLink,
			}),
			helpers.Shtml(),
		)
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

	// Get translator for the chat
	translator := i18n.MustNewTranslator(db.GetLanguage(ctx))

	if len(args) >= 2 {
		switch strings.ToLower(args[1]) {
		case "on", "yes", "true":
			go db.SetAntiChannelPin(chat.Id, true)
			_, err := msg.Reply(b,
				translator.Message("pins_antichannelpin_enabled", nil),
				helpers.Shtml(),
			)
			if err != nil {
				log.Error(err)
				return err
			}
		case "off", "no", "false":
			go db.SetAntiChannelPin(chat.Id, false)
			_, err := msg.Reply(b,
				translator.Message("pins_antichannelpin_disabled", nil),
				helpers.Shtml(),
			)
			if err != nil {
				log.Error(err)
				return err
			}
		default:
			_, err := msg.Reply(b, translator.Message("pins_invalid_option", nil), helpers.Shtml())
			if err != nil {
				log.Error(err)
				return err
			}
		}
	} else {
		pinprefs := db.GetPinData(chat.Id)
		if pinprefs.AntiChannelPin {
			_, err := msg.Reply(b,
				translator.Message("pins_antichannelpin_status_enabled", i18n.Params{
					"chat_title": chat.Title,
				}),
				helpers.Shtml(),
			)
			if err != nil {
				log.Error(err)
				return err
			}
		} else {
			_, err := msg.Reply(b,
				translator.Message("pins_antichannelpin_status_disabled", i18n.Params{
					"chat_title": chat.Title,
				}),
				helpers.Shtml(),
			)
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

	// Get translator for the chat
	translator := i18n.MustNewTranslator(db.GetLanguage(ctx))

	if len(args) >= 2 {
		switch strings.ToLower(args[1]) {
		case "on", "yes", "true":
			go db.SetCleanLinked(chat.Id, true)
			_, err := msg.Reply(b,
				translator.Message("pins_cleanlinked_enabled", nil),
				helpers.Shtml(),
			)
			if err != nil {
				log.Error(err)
				return err
			}
		case "off", "no", "false":
			go db.SetCleanLinked(chat.Id, false)
			_, err := msg.Reply(b,
				translator.Message("pins_cleanlinked_disabled", nil),
				helpers.Shtml(),
			)
			if err != nil {
				log.Error(err)
				return err
			}
		default:
			_, err := msg.Reply(b, translator.Message("pins_invalid_option", nil), helpers.Shtml())
			if err != nil {
				log.Error(err)
				return err
			}
		}
	} else {
		pinprefs := db.GetPinData(chat.Id)
		if pinprefs.CleanLinked {
			_, err := msg.Reply(b,
				translator.Message("pins_cleanlinked_status_enabled", i18n.Params{
					"chat_title": chat.Title,
				}),
				helpers.Shtml(),
			)
			if err != nil {
				log.Error(err)
				return err
			}
		} else {
			_, err := msg.Reply(b,
				translator.Message("pins_cleanlinked_status_disabled", i18n.Params{
					"chat_title": chat.Title,
				}),
				helpers.Shtml(),
			)
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

	// Get translator for the chat
	translator := i18n.MustNewTranslator(db.GetLanguage(ctx))

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
		_, err = msg.Reply(b, translator.Message("pins_no_pinned_message", nil), helpers.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return err
	}

	pinLink = helpers.GetMessageLinkFromMessageId(chat, pinnedMsg.MessageId)

	_, err = msg.Reply(b,
		translator.Message("pins_here_pinned_message", i18n.Params{
			"link": pinLink,
		}),
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
						{Text: translator.Message("pins_button_pinned_message", nil), Url: pinLink},
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
		if msg.Text == "" {
			rawText = strings.SplitN(msg.OriginalCaptionMDV2(), " ", 2)[1]
		} else {
			rawText = strings.SplitN(msg.OriginalMDV2(), " ", 2)[1]
		}
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
