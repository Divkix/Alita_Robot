package media

import (
	"fmt"
	"math/rand"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/content"
	"github.com/divkix/Alita_Robot/alita/utils/errors"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	"github.com/divkix/Alita_Robot/alita/utils/keyboard"
)

func resolveSendResult[T any](result T, err error, chatID int64, mediaType string) (T, error) {
	if err == nil {
		cache.MarkChatNotRestricted(chatID)
		return result, nil
	}

	errStr := err.Error()
	if helpers.IsPermissionError(errStr) {
		cache.MarkChatRestricted(chatID)
		log.WithFields(log.Fields{
			"chat_id":    chatID,
			"media_type": mediaType,
			"error":      errStr,
		}).Warningf("Bot lacks permission to send %s in this chat", mediaType)
		var zero T
		return zero, ErrNoPermission
	}
	return result, errors.Wrapf(err, "failed to send %s to chat %d", mediaType, chatID)
}

var ErrNoPermission = fmt.Errorf("bot lacks permission to send messages")

type Content struct {
	Text    string
	FileID  string
	MsgType int
	Name    string
}

type Options struct {
	ChatID            int64
	ReplyMsgID        int64
	ThreadID          int64
	Keyboard          *gotgbot.InlineKeyboardMarkup
	NoFormat          bool
	NoNotif           bool
	WebPreview        bool
	IsProtected       bool
	AllowWithoutReply bool
}

func Send(b *gotgbot.Bot, content Content, opts Options) (*gotgbot.Message, error) {
	if cache.IsChatRestricted(opts.ChatID) {
		log.WithField("chat_id", opts.ChatID).Debug("[Media] Skipping send to restricted chat")
		return nil, ErrNoPermission
	}

	parseMode := formatting.HTML
	if opts.NoFormat {
		parseMode = ""
	}

	var replyParams *gotgbot.ReplyParameters
	if opts.ReplyMsgID > 0 {
		replyParams = &gotgbot.ReplyParameters{
			MessageId:                opts.ReplyMsgID,
			AllowSendingWithoutReply: opts.AllowWithoutReply,
		}
	}

	switch content.MsgType {
	case db.TEXT, 0:
		return sendText(b, content, opts, parseMode, replyParams)
	case db.STICKER:
		return sendSticker(b, content, opts, replyParams)
	case db.DOCUMENT:
		return sendDocument(b, content, opts, parseMode, replyParams)
	case db.PHOTO:
		return sendPhoto(b, content, opts, parseMode, replyParams)
	case db.AUDIO:
		return sendAudio(b, content, opts, parseMode, replyParams)
	case db.VOICE:
		return sendVoice(b, content, opts, parseMode, replyParams)
	case db.VIDEO:
		return sendVideo(b, content, opts, parseMode, replyParams)
	case db.VIDEO_NOTE:
		return sendVideoNote(b, content, opts, replyParams)
	default:
		log.Warnf("[Media] Unknown message type %d, falling back to text", content.MsgType)
		return sendText(b, content, opts, parseMode, replyParams)
	}
}

func sendText(b *gotgbot.Bot, content Content, opts Options, parseMode string, replyParams *gotgbot.ReplyParameters) (*gotgbot.Message, error) {
	msg, err := b.SendMessage(opts.ChatID, content.Text, &gotgbot.SendMessageOpts{
		ParseMode: parseMode,
		LinkPreviewOptions: &gotgbot.LinkPreviewOptions{
			IsDisabled: !opts.WebPreview,
		},
		ReplyMarkup:         opts.Keyboard,
		ReplyParameters:     replyParams,
		ProtectContent:      opts.IsProtected,
		DisableNotification: opts.NoNotif,
		MessageThreadId:     opts.ThreadID,
	})
	return resolveSendResult(msg, err, opts.ChatID, "text")
}

func sendSticker(b *gotgbot.Bot, content Content, opts Options, replyParams *gotgbot.ReplyParameters) (*gotgbot.Message, error) {
	if content.FileID == "" {
		log.Warnf("[Media] Empty FileID for STICKER '%s' in chat %d, falling back to text", content.Name, opts.ChatID)
		return sendText(b, content, opts, formatting.HTML, replyParams)
	}
	msg, err := b.SendSticker(opts.ChatID, gotgbot.InputFileByID(content.FileID), &gotgbot.SendStickerOpts{
		ReplyParameters:     replyParams,
		ReplyMarkup:         opts.Keyboard,
		ProtectContent:      opts.IsProtected,
		DisableNotification: opts.NoNotif,
		MessageThreadId:     opts.ThreadID,
	})
	return resolveSendResult(msg, err, opts.ChatID, "sticker")
}

func sendDocument(b *gotgbot.Bot, content Content, opts Options, parseMode string, replyParams *gotgbot.ReplyParameters) (*gotgbot.Message, error) {
	if content.FileID == "" {
		log.Warnf("[Media] Empty FileID for DOCUMENT '%s' in chat %d, falling back to text", content.Name, opts.ChatID)
		return sendText(b, content, opts, parseMode, replyParams)
	}
	msg, err := b.SendDocument(opts.ChatID, gotgbot.InputFileByID(content.FileID), &gotgbot.SendDocumentOpts{
		ReplyParameters:     replyParams,
		ParseMode:           parseMode,
		ReplyMarkup:         opts.Keyboard,
		Caption:             content.Text,
		ProtectContent:      opts.IsProtected,
		DisableNotification: opts.NoNotif,
		MessageThreadId:     opts.ThreadID,
	})
	return resolveSendResult(msg, err, opts.ChatID, "document")
}

func sendPhoto(b *gotgbot.Bot, content Content, opts Options, parseMode string, replyParams *gotgbot.ReplyParameters) (*gotgbot.Message, error) {
	if content.FileID == "" {
		log.Warnf("[Media] Empty FileID for PHOTO '%s' in chat %d, falling back to text", content.Name, opts.ChatID)
		return sendText(b, content, opts, parseMode, replyParams)
	}
	msg, err := b.SendPhoto(opts.ChatID, gotgbot.InputFileByID(content.FileID), &gotgbot.SendPhotoOpts{
		ReplyParameters:     replyParams,
		ParseMode:           parseMode,
		ReplyMarkup:         opts.Keyboard,
		Caption:             content.Text,
		ProtectContent:      opts.IsProtected,
		DisableNotification: opts.NoNotif,
		MessageThreadId:     opts.ThreadID,
	})
	return resolveSendResult(msg, err, opts.ChatID, "photo")
}

func sendAudio(b *gotgbot.Bot, content Content, opts Options, parseMode string, replyParams *gotgbot.ReplyParameters) (*gotgbot.Message, error) {
	if content.FileID == "" {
		log.Warnf("[Media] Empty FileID for AUDIO '%s' in chat %d, falling back to text", content.Name, opts.ChatID)
		return sendText(b, content, opts, parseMode, replyParams)
	}
	msg, err := b.SendAudio(opts.ChatID, gotgbot.InputFileByID(content.FileID), &gotgbot.SendAudioOpts{
		ReplyParameters:     replyParams,
		ParseMode:           parseMode,
		ReplyMarkup:         opts.Keyboard,
		Caption:             content.Text,
		ProtectContent:      opts.IsProtected,
		DisableNotification: opts.NoNotif,
		MessageThreadId:     opts.ThreadID,
	})
	return resolveSendResult(msg, err, opts.ChatID, "audio")
}

func sendVoice(b *gotgbot.Bot, content Content, opts Options, parseMode string, replyParams *gotgbot.ReplyParameters) (*gotgbot.Message, error) {
	if content.FileID == "" {
		log.Warnf("[Media] Empty FileID for VOICE '%s' in chat %d, falling back to text", content.Name, opts.ChatID)
		return sendText(b, content, opts, parseMode, replyParams)
	}
	msg, err := b.SendVoice(opts.ChatID, gotgbot.InputFileByID(content.FileID), &gotgbot.SendVoiceOpts{
		ReplyParameters:     replyParams,
		ParseMode:           parseMode,
		ReplyMarkup:         opts.Keyboard,
		Caption:             content.Text,
		ProtectContent:      opts.IsProtected,
		DisableNotification: opts.NoNotif,
		MessageThreadId:     opts.ThreadID,
	})
	return resolveSendResult(msg, err, opts.ChatID, "voice")
}

func sendVideo(b *gotgbot.Bot, content Content, opts Options, parseMode string, replyParams *gotgbot.ReplyParameters) (*gotgbot.Message, error) {
	if content.FileID == "" {
		log.Warnf("[Media] Empty FileID for VIDEO '%s' in chat %d, falling back to text", content.Name, opts.ChatID)
		return sendText(b, content, opts, parseMode, replyParams)
	}
	msg, err := b.SendVideo(opts.ChatID, gotgbot.InputFileByID(content.FileID), &gotgbot.SendVideoOpts{
		ReplyParameters:     replyParams,
		ParseMode:           parseMode,
		ReplyMarkup:         opts.Keyboard,
		Caption:             content.Text,
		ProtectContent:      opts.IsProtected,
		DisableNotification: opts.NoNotif,
		MessageThreadId:     opts.ThreadID,
	})
	return resolveSendResult(msg, err, opts.ChatID, "video")
}

func sendVideoNote(b *gotgbot.Bot, content Content, opts Options, replyParams *gotgbot.ReplyParameters) (*gotgbot.Message, error) {
	if content.FileID == "" {
		log.Warnf("[Media] Empty FileID for VideoNote '%s' in chat %d, falling back to text", content.Name, opts.ChatID)
		return sendText(b, content, opts, formatting.HTML, replyParams)
	}
	msg, err := b.SendVideoNote(opts.ChatID, gotgbot.InputFileByID(content.FileID), &gotgbot.SendVideoNoteOpts{
		ReplyParameters:     replyParams,
		ReplyMarkup:         opts.Keyboard,
		ProtectContent:      opts.IsProtected,
		DisableNotification: opts.NoNotif,
		MessageThreadId:     opts.ThreadID,
	})
	return resolveSendResult(msg, err, opts.ChatID, "video note")
}

func SendNote(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, note *db.Notes, replyMsgID, threadID int64) (*gotgbot.Message, error) {
	var (
		buttons []db.Button
		sent    string
	)

	buttons = note.Buttons

	rstrings := strings.Split(note.NoteContent, "%%%")
	if len(rstrings) == 1 {
		sent = rstrings[0]
	} else {
		n := rand.Intn(len(rstrings)) // #nosec G404 - Non-cryptographic random is sufficient for selecting messages
		sent = rstrings[n]
	}

	noteCopy := *note
	noteCopy.NoteContent, buttons = formatting.FormattingReplacer(b, chat, ctx.EffectiveUser, sent, buttons)
	_, _, _, _, _, _, noteCopy.NoteContent = content.NotesParser(noteCopy.NoteContent)

	keyb := keyboard.BuildKeyboard(buttons)
	keyboardMarkup := gotgbot.InlineKeyboardMarkup{InlineKeyboard: keyb}

	return Send(b, Content{
		Text:    noteCopy.NoteContent,
		FileID:  noteCopy.FileID,
		MsgType: noteCopy.MsgType,
		Name:    noteCopy.NoteName,
	}, Options{
		ChatID:            ctx.EffectiveChat.Id,
		ReplyMsgID:        replyMsgID,
		ThreadID:          threadID,
		Keyboard:          &keyboardMarkup,
		NoFormat:          false,
		NoNotif:           noteCopy.NoNotif,
		WebPreview:        noteCopy.WebPreview,
		IsProtected:       noteCopy.IsProtected,
		AllowWithoutReply: true,
	})
}

func SendFilter(b *gotgbot.Bot, ctx *ext.Context, filter *db.ChatFilters, replyMsgID int64) (*gotgbot.Message, error) {
	if filter == nil {
		return nil, fmt.Errorf("filter data is nil")
	}

	chat := ctx.EffectiveChat

	var (
		buttons       []db.Button
		sent          string
		tmpfilterData db.ChatFilters
	)
	tmpfilterData = *filter
	buttons = tmpfilterData.Buttons

	rstrings := strings.Split(tmpfilterData.FilterReply, "%%%")
	if len(rstrings) == 1 {
		sent = rstrings[0]
	} else {
		n := rand.Intn(len(rstrings)) // #nosec G404 - Non-cryptographic random is sufficient for selecting messages
		sent = rstrings[n]
	}

	tmpfilterData.FilterReply, buttons = formatting.FormattingReplacer(b, chat, ctx.EffectiveUser, sent, buttons)
	keyb := keyboard.BuildKeyboard(buttons)
	keyboardMarkup := gotgbot.InlineKeyboardMarkup{InlineKeyboard: keyb}

	return Send(b, Content{
		Text:    tmpfilterData.FilterReply,
		FileID:  tmpfilterData.FileID,
		MsgType: tmpfilterData.MsgType,
		Name:    tmpfilterData.KeyWord,
	}, Options{
		ChatID:            chat.Id,
		ReplyMsgID:        replyMsgID,
		ThreadID:          ctx.Message.MessageThreadId,
		Keyboard:          &keyboardMarkup,
		NoFormat:          false,
		NoNotif:           tmpfilterData.NoNotif,
		WebPreview:        false,
		IsProtected:       false,
		AllowWithoutReply: true,
	})
}

func SendGreeting(b *gotgbot.Bot, chatID int64, text, fileID string, msgType int, keyboard *gotgbot.InlineKeyboardMarkup, threadID int64) (*gotgbot.Message, error) {
	return Send(b, Content{
		Text:    text,
		FileID:  fileID,
		MsgType: msgType,
		Name:    "greeting",
	}, Options{
		ChatID:            chatID,
		ReplyMsgID:        0,
		ThreadID:          threadID,
		Keyboard:          keyboard,
		NoFormat:          false,
		NoNotif:           false,
		WebPreview:        false,
		IsProtected:       false,
		AllowWithoutReply: true,
	})
}
