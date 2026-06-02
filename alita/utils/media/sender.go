// Package media provides a unified interface for sending different types of Telegram media.
// It consolidates the duplicate logic from NotesEnumFuncMap, GreetingsEnumFuncMap, and FiltersEnumFuncMap.
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
	"github.com/divkix/Alita_Robot/alita/utils/keyboard"
)

// isPermissionError reports whether the Telegram error string indicates that the
// bot lacks permission to send messages in a chat.
func isPermissionError(errStr string) bool {
	return strings.Contains(errStr, "not enough rights to send text messages") ||
		strings.Contains(errStr, "have no rights to send a message") ||
		strings.Contains(errStr, "CHAT_WRITE_FORBIDDEN") ||
		strings.Contains(errStr, "CHAT_RESTRICTED") ||
		strings.Contains(errStr, "need administrator rights in the channel chat")
}

// resolveSendResult handles the shared error-handling path for all send methods.
// It marks the chat as restricted on permission errors, clears it on success,
// and wraps unhandled errors with context.
func resolveSendResult[T any](result T, err error, chatID int64, mediaType string) (T, error) {
	if err == nil {
		cache.MarkChatNotRestricted(chatID)
		return result, nil
	}

	errStr := err.Error()
	if isPermissionError(errStr) {
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

// Type constants matching db package for convenience
// Sentinel errors
var ErrNoPermission = fmt.Errorf("bot lacks permission to send messages")

const (
	TypeText      = db.TEXT
	TypeSticker   = db.STICKER
	TypeDocument  = db.DOCUMENT
	TypePhoto     = db.PHOTO
	TypeAudio     = db.AUDIO
	TypeVoice     = db.VOICE
	TypeVideo     = db.VIDEO
	TypeVideoNote = db.VIDEO_NOTE
)

// ParseMode constants
const (
	HTML = "HTML"
	None = ""
)

// Content represents the media content to be sent.
type Content struct {
	Text    string // Text content or caption
	FileID  string // File ID for media types
	MsgType int    // One of db.TEXT, db.STICKER, etc.
	Name    string // Optional name for logging (note name, filter keyword, etc.)
}

// Options configures how the media is sent.
type Options struct {
	ChatID            int64
	ReplyMsgID        int64
	ThreadID          int64
	Keyboard          *gotgbot.InlineKeyboardMarkup
	NoFormat          bool // If true, don't parse HTML
	NoNotif           bool // Disable notification
	WebPreview        bool // Enable link preview (TEXT only)
	IsProtected       bool // Protect content from forwarding
	AllowWithoutReply bool // Allow sending if reply message is deleted
}

// Send sends media content using the appropriate Telegram API method.
// Returns the sent message and any error encountered.
func Send(b *gotgbot.Bot, content Content, opts Options) (*gotgbot.Message, error) {
	// Short-circuit if bot is known to be restricted in this chat.
	if cache.IsChatRestricted(opts.ChatID) {
		log.WithField("chat_id", opts.ChatID).Debug("[Media] Skipping send to restricted chat")
		return nil, ErrNoPermission
	}

	// Determine parse mode
	parseMode := HTML
	if opts.NoFormat {
		parseMode = None
	}

	// Build reply parameters if reply message ID is set
	var replyParams *gotgbot.ReplyParameters
	if opts.ReplyMsgID > 0 {
		replyParams = &gotgbot.ReplyParameters{
			MessageId:                opts.ReplyMsgID,
			AllowSendingWithoutReply: opts.AllowWithoutReply,
		}
	}

	switch content.MsgType {
	case db.TEXT, 0: // 0 is fallback for uninitialized/legacy records
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

// sendText sends a text message with error handling for expected permission errors.
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

// sendSticker sends a sticker or falls back to text if FileID is empty.
func sendSticker(b *gotgbot.Bot, content Content, opts Options, replyParams *gotgbot.ReplyParameters) (*gotgbot.Message, error) {
	if content.FileID == "" {
		log.Warnf("[Media] Empty FileID for STICKER '%s' in chat %d, falling back to text", content.Name, opts.ChatID)
		return sendText(b, content, opts, HTML, replyParams)
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

// sendDocument sends a document or falls back to text if FileID is empty.
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

// sendPhoto sends a photo or falls back to text if FileID is empty.
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

// sendAudio sends an audio file or falls back to text if FileID is empty.
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

// sendVoice sends a voice message or falls back to text if FileID is empty.
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

// sendVideo sends a video or falls back to text if FileID is empty.
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

// sendVideoNote sends a video note or falls back to text if FileID is empty.
func sendVideoNote(b *gotgbot.Bot, content Content, opts Options, replyParams *gotgbot.ReplyParameters) (*gotgbot.Message, error) {
	if content.FileID == "" {
		log.Warnf("[Media] Empty FileID for VideoNote '%s' in chat %d, falling back to text", content.Name, opts.ChatID)
		return sendText(b, content, opts, HTML, replyParams)
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

// SendNote sends a note with full preprocessing (randomization, formatting, keyboard, options).
// The chat parameter provides the group context for formatting placeholders like {chatname};
// ctx.EffectiveChat is used as the actual send target.
func SendNote(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, note *db.Notes, replyMsgID, threadID int64) (*gotgbot.Message, error) {
	var (
		buttons []db.Button
		sent    string
	)

	// copy just in case
	buttons = note.Buttons

	// Random data selection
	rstrings := strings.Split(note.NoteContent, "%%%")
	if len(rstrings) == 1 {
		sent = rstrings[0]
	} else {
		n := rand.Intn(len(rstrings)) // #nosec G404 - Non-cryptographic random is sufficient for selecting messages
		sent = rstrings[n]
	}

	// Avoid mutating the original note
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
		NoFormat:          false, // Notes use formatting by default
		NoNotif:           noteCopy.NoNotif,
		WebPreview:        noteCopy.WebPreview,
		IsProtected:       noteCopy.IsProtected,
		AllowWithoutReply: true,
	})
}

// SendFilter sends a filter with full preprocessing (randomization, formatting, keyboard).
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

	// Random data selection
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
		NoFormat:          false, // Filters use formatting by default
		NoNotif:           tmpfilterData.NoNotif,
		WebPreview:        false, // Filters disable web preview by default
		IsProtected:       false, // Filters don't support protection
		AllowWithoutReply: true,
	})
}

// SendGreeting is a convenience function for sending welcome/goodbye messages.
func SendGreeting(b *gotgbot.Bot, chatID int64, text, fileID string, msgType int, keyboard *gotgbot.InlineKeyboardMarkup, threadID int64) (*gotgbot.Message, error) {
	return Send(b, Content{
		Text:    text,
		FileID:  fileID,
		MsgType: msgType,
		Name:    "greeting",
	}, Options{
		ChatID:            chatID,
		ReplyMsgID:        0, // Greetings don't reply to messages
		ThreadID:          threadID,
		Keyboard:          keyboard,
		NoFormat:          false, // Greetings use formatting
		NoNotif:           false, // Greetings notify by default
		WebPreview:        false, // Greetings disable web preview
		IsProtected:       false, // Greetings don't support protection
		AllowWithoutReply: true,
	})
}
