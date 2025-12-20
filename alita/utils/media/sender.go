// Package media provides a unified interface for sending different types of Telegram media.
// It consolidates the duplicate logic from NotesEnumFuncMap, GreetingsEnumFuncMap, and FiltersEnumFuncMap.
package media

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db"
)

// Type constants matching db package for convenience
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

// sendText sends a text message.
func sendText(b *gotgbot.Bot, content Content, opts Options, parseMode string, replyParams *gotgbot.ReplyParameters) (*gotgbot.Message, error) {
	return b.SendMessage(opts.ChatID, content.Text, &gotgbot.SendMessageOpts{
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
}

// sendSticker sends a sticker or falls back to text if FileID is empty.
func sendSticker(b *gotgbot.Bot, content Content, opts Options, replyParams *gotgbot.ReplyParameters) (*gotgbot.Message, error) {
	if content.FileID == "" {
		log.Warnf("[Media] Empty FileID for STICKER '%s' in chat %d, falling back to text", content.Name, opts.ChatID)
		return sendText(b, content, opts, HTML, replyParams)
	}
	return b.SendSticker(opts.ChatID, gotgbot.InputFileByID(content.FileID), &gotgbot.SendStickerOpts{
		ReplyParameters:     replyParams,
		ReplyMarkup:         opts.Keyboard,
		ProtectContent:      opts.IsProtected,
		DisableNotification: opts.NoNotif,
		MessageThreadId:     opts.ThreadID,
	})
}

// sendDocument sends a document or falls back to text if FileID is empty.
func sendDocument(b *gotgbot.Bot, content Content, opts Options, parseMode string, replyParams *gotgbot.ReplyParameters) (*gotgbot.Message, error) {
	if content.FileID == "" {
		log.Warnf("[Media] Empty FileID for DOCUMENT '%s' in chat %d, falling back to text", content.Name, opts.ChatID)
		return sendText(b, content, opts, parseMode, replyParams)
	}
	return b.SendDocument(opts.ChatID, gotgbot.InputFileByID(content.FileID), &gotgbot.SendDocumentOpts{
		ReplyParameters:     replyParams,
		ParseMode:           parseMode,
		ReplyMarkup:         opts.Keyboard,
		Caption:             content.Text,
		ProtectContent:      opts.IsProtected,
		DisableNotification: opts.NoNotif,
		MessageThreadId:     opts.ThreadID,
	})
}

// sendPhoto sends a photo or falls back to text if FileID is empty.
func sendPhoto(b *gotgbot.Bot, content Content, opts Options, parseMode string, replyParams *gotgbot.ReplyParameters) (*gotgbot.Message, error) {
	if content.FileID == "" {
		log.Warnf("[Media] Empty FileID for PHOTO '%s' in chat %d, falling back to text", content.Name, opts.ChatID)
		return sendText(b, content, opts, parseMode, replyParams)
	}
	return b.SendPhoto(opts.ChatID, gotgbot.InputFileByID(content.FileID), &gotgbot.SendPhotoOpts{
		ReplyParameters:     replyParams,
		ParseMode:           parseMode,
		ReplyMarkup:         opts.Keyboard,
		Caption:             content.Text,
		ProtectContent:      opts.IsProtected,
		DisableNotification: opts.NoNotif,
		MessageThreadId:     opts.ThreadID,
	})
}

// sendAudio sends an audio file or falls back to text if FileID is empty.
func sendAudio(b *gotgbot.Bot, content Content, opts Options, parseMode string, replyParams *gotgbot.ReplyParameters) (*gotgbot.Message, error) {
	if content.FileID == "" {
		log.Warnf("[Media] Empty FileID for AUDIO '%s' in chat %d, falling back to text", content.Name, opts.ChatID)
		return sendText(b, content, opts, parseMode, replyParams)
	}
	return b.SendAudio(opts.ChatID, gotgbot.InputFileByID(content.FileID), &gotgbot.SendAudioOpts{
		ReplyParameters:     replyParams,
		ParseMode:           parseMode,
		ReplyMarkup:         opts.Keyboard,
		Caption:             content.Text,
		ProtectContent:      opts.IsProtected,
		DisableNotification: opts.NoNotif,
		MessageThreadId:     opts.ThreadID,
	})
}

// sendVoice sends a voice message or falls back to text if FileID is empty.
func sendVoice(b *gotgbot.Bot, content Content, opts Options, parseMode string, replyParams *gotgbot.ReplyParameters) (*gotgbot.Message, error) {
	if content.FileID == "" {
		log.Warnf("[Media] Empty FileID for VOICE '%s' in chat %d, falling back to text", content.Name, opts.ChatID)
		return sendText(b, content, opts, parseMode, replyParams)
	}
	return b.SendVoice(opts.ChatID, gotgbot.InputFileByID(content.FileID), &gotgbot.SendVoiceOpts{
		ReplyParameters:     replyParams,
		ParseMode:           parseMode,
		ReplyMarkup:         opts.Keyboard,
		Caption:             content.Text,
		ProtectContent:      opts.IsProtected,
		DisableNotification: opts.NoNotif,
		MessageThreadId:     opts.ThreadID,
	})
}

// sendVideo sends a video or falls back to text if FileID is empty.
func sendVideo(b *gotgbot.Bot, content Content, opts Options, parseMode string, replyParams *gotgbot.ReplyParameters) (*gotgbot.Message, error) {
	if content.FileID == "" {
		log.Warnf("[Media] Empty FileID for VIDEO '%s' in chat %d, falling back to text", content.Name, opts.ChatID)
		return sendText(b, content, opts, parseMode, replyParams)
	}
	return b.SendVideo(opts.ChatID, gotgbot.InputFileByID(content.FileID), &gotgbot.SendVideoOpts{
		ReplyParameters:     replyParams,
		ParseMode:           parseMode,
		ReplyMarkup:         opts.Keyboard,
		Caption:             content.Text,
		ProtectContent:      opts.IsProtected,
		DisableNotification: opts.NoNotif,
		MessageThreadId:     opts.ThreadID,
	})
}

// sendVideoNote sends a video note or falls back to text if FileID is empty.
func sendVideoNote(b *gotgbot.Bot, content Content, opts Options, replyParams *gotgbot.ReplyParameters) (*gotgbot.Message, error) {
	if content.FileID == "" {
		log.Warnf("[Media] Empty FileID for VideoNote '%s' in chat %d, falling back to text", content.Name, opts.ChatID)
		return sendText(b, content, opts, HTML, replyParams)
	}
	return b.SendVideoNote(opts.ChatID, gotgbot.InputFileByID(content.FileID), &gotgbot.SendVideoNoteOpts{
		ReplyParameters:     replyParams,
		ReplyMarkup:         opts.Keyboard,
		ProtectContent:      opts.IsProtected,
		DisableNotification: opts.NoNotif,
		MessageThreadId:     opts.ThreadID,
	})
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
