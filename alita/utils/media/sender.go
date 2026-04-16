// Package media provides a unified interface for sending different types of Telegram media.
// It consolidates the duplicate logic from NotesEnumFuncMap, GreetingsEnumFuncMap, and FiltersEnumFuncMap.
//
//nolint:dupl // Media sender functions follow same pattern by design
package media

import (
	"fmt"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/errors"
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
	TypeVideoNote = db.VideoNote
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
	case db.VideoNote:
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
	if err != nil {
		errStr := err.Error()
		// Check for expected permission-related errors
		if isPermissionError(errStr) {
			cache.MarkChatRestricted(opts.ChatID)
			log.WithFields(log.Fields{
				"chat_id": opts.ChatID,
				"error":   errStr,
			}).Warning("Bot lacks permission to send messages in this chat")
			return nil, ErrNoPermission
		}
		return nil, errors.Wrapf(err, "failed to send message to chat %d", opts.ChatID)
	}
	cache.MarkChatNotRestricted(opts.ChatID)
	return msg, nil
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
	if err != nil {
		if isPermissionError(err.Error()) {
			cache.MarkChatRestricted(opts.ChatID)
			log.WithFields(log.Fields{
				"chat_id": opts.ChatID,
				"error":   err.Error(),
			}).Warning("Bot lacks permission to send sticker in this chat")
			return nil, ErrNoPermission
		}
		return nil, errors.Wrapf(err, "failed to send sticker to chat %d", opts.ChatID)
	}
	cache.MarkChatNotRestricted(opts.ChatID)
	return msg, nil
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
	if err != nil {
		if isPermissionError(err.Error()) {
			cache.MarkChatRestricted(opts.ChatID)
			log.WithFields(log.Fields{
				"chat_id": opts.ChatID,
				"error":   err.Error(),
			}).Warning("Bot lacks permission to send document in this chat")
			return nil, ErrNoPermission
		}
		return nil, errors.Wrapf(err, "failed to send document to chat %d", opts.ChatID)
	}
	cache.MarkChatNotRestricted(opts.ChatID)
	return msg, nil
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
	if err != nil {
		if isPermissionError(err.Error()) {
			cache.MarkChatRestricted(opts.ChatID)
			log.WithFields(log.Fields{
				"chat_id": opts.ChatID,
				"error":   err.Error(),
			}).Warning("Bot lacks permission to send photo in this chat")
			return nil, ErrNoPermission
		}
		return nil, errors.Wrapf(err, "failed to send photo to chat %d", opts.ChatID)
	}
	cache.MarkChatNotRestricted(opts.ChatID)
	return msg, nil
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
	if err != nil {
		if isPermissionError(err.Error()) {
			cache.MarkChatRestricted(opts.ChatID)
			log.WithFields(log.Fields{
				"chat_id": opts.ChatID,
				"error":   err.Error(),
			}).Warning("Bot lacks permission to send audio in this chat")
			return nil, ErrNoPermission
		}
		return nil, errors.Wrapf(err, "failed to send audio to chat %d", opts.ChatID)
	}
	cache.MarkChatNotRestricted(opts.ChatID)
	return msg, nil
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
	if err != nil {
		if isPermissionError(err.Error()) {
			cache.MarkChatRestricted(opts.ChatID)
			log.WithFields(log.Fields{
				"chat_id": opts.ChatID,
				"error":   err.Error(),
			}).Warning("Bot lacks permission to send voice in this chat")
			return nil, ErrNoPermission
		}
		return nil, errors.Wrapf(err, "failed to send voice to chat %d", opts.ChatID)
	}
	cache.MarkChatNotRestricted(opts.ChatID)
	return msg, nil
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
	if err != nil {
		if isPermissionError(err.Error()) {
			cache.MarkChatRestricted(opts.ChatID)
			log.WithFields(log.Fields{
				"chat_id": opts.ChatID,
				"error":   err.Error(),
			}).Warning("Bot lacks permission to send video in this chat")
			return nil, ErrNoPermission
		}
		return nil, errors.Wrapf(err, "failed to send video to chat %d", opts.ChatID)
	}
	cache.MarkChatNotRestricted(opts.ChatID)
	return msg, nil
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
	if err != nil {
		if isPermissionError(err.Error()) {
			cache.MarkChatRestricted(opts.ChatID)
			log.WithFields(log.Fields{
				"chat_id": opts.ChatID,
				"error":   err.Error(),
			}).Warning("Bot lacks permission to send video note in this chat")
			return nil, ErrNoPermission
		}
		return nil, errors.Wrapf(err, "failed to send video note to chat %d", opts.ChatID)
	}
	cache.MarkChatNotRestricted(opts.ChatID)
	return msg, nil
}

// SendNote is a convenience function for sending notes.
func SendNote(b *gotgbot.Bot, chatID int64, note *db.Notes, keyboard *gotgbot.InlineKeyboardMarkup, replyMsgID, threadID int64) (*gotgbot.Message, error) {
	return Send(b, Content{
		Text:    note.NoteContent,
		FileID:  note.FileID,
		MsgType: note.MsgType,
		Name:    note.NoteName,
	}, Options{
		ChatID:            chatID,
		ReplyMsgID:        replyMsgID,
		ThreadID:          threadID,
		Keyboard:          keyboard,
		NoFormat:          false, // Notes use formatting by default
		NoNotif:           note.NoNotif,
		WebPreview:        note.WebPreview,
		IsProtected:       note.IsProtected,
		AllowWithoutReply: true,
	})
}

// SendFilter is a convenience function for sending filters.
func SendFilter(b *gotgbot.Bot, chatID int64, filter *db.ChatFilters, keyboard *gotgbot.InlineKeyboardMarkup, replyMsgID, threadID int64) (*gotgbot.Message, error) {
	return Send(b, Content{
		Text:    filter.FilterReply,
		FileID:  filter.FileID,
		MsgType: filter.MsgType,
		Name:    filter.KeyWord,
	}, Options{
		ChatID:            chatID,
		ReplyMsgID:        replyMsgID,
		ThreadID:          threadID,
		Keyboard:          keyboard,
		NoFormat:          false, // Filters use formatting by default
		NoNotif:           filter.NoNotif,
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
