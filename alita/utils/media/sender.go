// Package media sends the messages admins save: notes, filters, greetings,
// recurring messages and permapins.
//
// Features send a saved message only through SendSaved. The kinds table is the
// one place to change what a kind supports (a random part, fillings,
// directives, waiting for the send limiter); the transport below it is
// unexported so no feature fills send options by hand.
package media

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	tgmd2html "github.com/PaulSonOfLars/gotg_md2html"
	"github.com/PaulSonOfLars/gotgbot/v2"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/content"
	"github.com/divkix/Alita_Robot/alita/utils/errors"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	"github.com/divkix/Alita_Robot/alita/utils/keyboard"
	"github.com/divkix/Alita_Robot/alita/utils/ratelimit"
)

func resolveSendResult[T any](result T, err error, chatID int64, mediaType string) (T, error) {
	if err == nil {
		return result, nil
	}

	errStr := err.Error()
	if helpers.IsChatPermissionError(err, chatID) {
		log.WithFields(log.Fields{
			"chat_id":    chatID,
			"media_type": mediaType,
			"error":      errStr,
		}).Debugf("Bot lacks permission to send %s in this chat", mediaType)
		var zero T
		return zero, ErrNoPermission
	}
	return result, errors.Wrapf(err, "failed to send %s to chat %d", mediaType, chatID)
}

var ErrNoPermission = fmt.Errorf("bot lacks permission to send messages")

type outgoing struct {
	Text    string
	FileID  string
	MsgType int
	Name    string
}

type sendOptions struct {
	ChatID            int64
	ReplyMsgID        int64
	ThreadID          int64
	Keyboard          *gotgbot.InlineKeyboardMarkup
	NoFormat          bool
	NoNotif           bool
	WebPreview        bool
	IsProtected       bool
	AllowWithoutReply bool
	Ephemeral         *gotgbot.EphemeralMessageParameters
}

// deliver sends content with opts, skipping a chat known to be restricted.
func deliver(b *gotgbot.Bot, content outgoing, opts sendOptions) (*gotgbot.Message, error) {
	restricted, restrictedFlagPresent := cache.IsChatRestrictedState(opts.ChatID)
	if restricted {
		log.WithField("chat_id", opts.ChatID).Debug("[Media] Skipping send to restricted chat")
		return nil, ErrNoPermission
	}
	sentAt := time.Now()

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

	var (
		msg *gotgbot.Message
		err error
	)
	switch content.MsgType {
	case db.TEXT, 0:
		msg, err = sendText(b, content, opts, parseMode, replyParams)
	case db.STICKER, db.DOCUMENT, db.PHOTO, db.AUDIO, db.VOICE, db.VIDEO, db.VIDEO_NOTE:
		if content.FileID == "" {
			log.Warnf("[Media] Empty FileID for type %d '%s' in chat %d, falling back to text", content.MsgType, content.Name, opts.ChatID)
			msg, err = sendText(b, content, opts, parseMode, replyParams)
			break
		}
		msg, err = sendMedia(b, content, opts, parseMode, replyParams)
	default:
		log.Warnf("[Media] Unknown message type %d, falling back to text", content.MsgType)
		msg, err = sendText(b, content, opts, parseMode, replyParams)
	}
	if err == nil && restrictedFlagPresent {
		cache.MarkChatNotRestrictedIfOlder(opts.ChatID, sentAt)
	}
	return msg, err
}

func sendMedia(b *gotgbot.Bot, content outgoing, opts sendOptions, parseMode string, replyParams *gotgbot.ReplyParameters) (*gotgbot.Message, error) {
	switch content.MsgType {
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
	default:
		return sendVideoNote(b, content, opts, replyParams)
	}
}

func sendText(b *gotgbot.Bot, content outgoing, opts sendOptions, parseMode string, replyParams *gotgbot.ReplyParameters) (*gotgbot.Message, error) {
	msg, err := b.SendMessage(opts.ChatID, content.Text, &gotgbot.SendMessageOpts{
		ParseMode: parseMode,
		LinkPreviewOptions: &gotgbot.LinkPreviewOptions{
			IsDisabled: !opts.WebPreview,
		},
		ReplyMarkup:                opts.Keyboard,
		ReplyParameters:            replyParams,
		ProtectContent:             opts.IsProtected,
		DisableNotification:        opts.NoNotif,
		MessageThreadId:            opts.ThreadID,
		EphemeralMessageParameters: opts.Ephemeral,
	})
	return resolveSendResult(msg, err, opts.ChatID, "text")
}

func sendSticker(b *gotgbot.Bot, content outgoing, opts sendOptions, replyParams *gotgbot.ReplyParameters) (*gotgbot.Message, error) {
	msg, err := b.SendSticker(opts.ChatID, gotgbot.InputFileByID(content.FileID), &gotgbot.SendStickerOpts{
		ReplyParameters:            replyParams,
		ReplyMarkup:                opts.Keyboard,
		ProtectContent:             opts.IsProtected,
		DisableNotification:        opts.NoNotif,
		MessageThreadId:            opts.ThreadID,
		EphemeralMessageParameters: opts.Ephemeral,
	})
	return resolveSendResult(msg, err, opts.ChatID, "sticker")
}

func sendDocument(b *gotgbot.Bot, content outgoing, opts sendOptions, parseMode string, replyParams *gotgbot.ReplyParameters) (*gotgbot.Message, error) {
	msg, err := b.SendDocument(opts.ChatID, gotgbot.InputFileByID(content.FileID), &gotgbot.SendDocumentOpts{
		ReplyParameters:            replyParams,
		ParseMode:                  parseMode,
		ReplyMarkup:                opts.Keyboard,
		Caption:                    content.Text,
		ProtectContent:             opts.IsProtected,
		DisableNotification:        opts.NoNotif,
		MessageThreadId:            opts.ThreadID,
		EphemeralMessageParameters: opts.Ephemeral,
	})
	return resolveSendResult(msg, err, opts.ChatID, "document")
}

func sendPhoto(b *gotgbot.Bot, content outgoing, opts sendOptions, parseMode string, replyParams *gotgbot.ReplyParameters) (*gotgbot.Message, error) {
	msg, err := b.SendPhoto(opts.ChatID, gotgbot.InputFileByID(content.FileID), &gotgbot.SendPhotoOpts{
		ReplyParameters:            replyParams,
		ParseMode:                  parseMode,
		ReplyMarkup:                opts.Keyboard,
		Caption:                    content.Text,
		ProtectContent:             opts.IsProtected,
		DisableNotification:        opts.NoNotif,
		MessageThreadId:            opts.ThreadID,
		EphemeralMessageParameters: opts.Ephemeral,
	})
	return resolveSendResult(msg, err, opts.ChatID, "photo")
}

func sendAudio(b *gotgbot.Bot, content outgoing, opts sendOptions, parseMode string, replyParams *gotgbot.ReplyParameters) (*gotgbot.Message, error) {
	msg, err := b.SendAudio(opts.ChatID, gotgbot.InputFileByID(content.FileID), &gotgbot.SendAudioOpts{
		ReplyParameters:            replyParams,
		ParseMode:                  parseMode,
		ReplyMarkup:                opts.Keyboard,
		Caption:                    content.Text,
		ProtectContent:             opts.IsProtected,
		DisableNotification:        opts.NoNotif,
		MessageThreadId:            opts.ThreadID,
		EphemeralMessageParameters: opts.Ephemeral,
	})
	return resolveSendResult(msg, err, opts.ChatID, "audio")
}

func sendVoice(b *gotgbot.Bot, content outgoing, opts sendOptions, parseMode string, replyParams *gotgbot.ReplyParameters) (*gotgbot.Message, error) {
	msg, err := b.SendVoice(opts.ChatID, gotgbot.InputFileByID(content.FileID), &gotgbot.SendVoiceOpts{
		ReplyParameters:            replyParams,
		ParseMode:                  parseMode,
		ReplyMarkup:                opts.Keyboard,
		Caption:                    content.Text,
		ProtectContent:             opts.IsProtected,
		DisableNotification:        opts.NoNotif,
		MessageThreadId:            opts.ThreadID,
		EphemeralMessageParameters: opts.Ephemeral,
	})
	return resolveSendResult(msg, err, opts.ChatID, "voice")
}

func sendVideo(b *gotgbot.Bot, content outgoing, opts sendOptions, parseMode string, replyParams *gotgbot.ReplyParameters) (*gotgbot.Message, error) {
	msg, err := b.SendVideo(opts.ChatID, gotgbot.InputFileByID(content.FileID), &gotgbot.SendVideoOpts{
		ReplyParameters:            replyParams,
		ParseMode:                  parseMode,
		ReplyMarkup:                opts.Keyboard,
		Caption:                    content.Text,
		ProtectContent:             opts.IsProtected,
		DisableNotification:        opts.NoNotif,
		MessageThreadId:            opts.ThreadID,
		EphemeralMessageParameters: opts.Ephemeral,
	})
	return resolveSendResult(msg, err, opts.ChatID, "video")
}

func sendVideoNote(b *gotgbot.Bot, content outgoing, opts sendOptions, replyParams *gotgbot.ReplyParameters) (*gotgbot.Message, error) {
	msg, err := b.SendVideoNote(opts.ChatID, gotgbot.InputFileByID(content.FileID), &gotgbot.SendVideoNoteOpts{
		ReplyParameters:            replyParams,
		ReplyMarkup:                opts.Keyboard,
		ProtectContent:             opts.IsProtected,
		DisableNotification:        opts.NoNotif,
		MessageThreadId:            opts.ThreadID,
		EphemeralMessageParameters: opts.Ephemeral,
	})
	return resolveSendResult(msg, err, opts.ChatID, "video note")
}

// Kind is the feature a saved message belongs to. What each kind supports is
// the kinds table below.
type Kind int

const (
	KindNote Kind = iota
	KindFilter
	KindGreeting // welcome and goodbye
	KindRecurring
	KindPermapin
)

// View is how a saved message is shown: Rendered for members, Raw for the
// admin who copies it to edit it.
type View int

const (
	Rendered View = iota
	Raw
)

type directives int

const (
	// directivesNone sends the text as stored, with the SavedMessage flags.
	directivesNone directives = iota
	// directivesStored takes the flags stored when the message was saved and
	// strips the directives the stored text still holds.
	directivesStored
	// directivesFromText reads {preview}/{protect}/{nonotif} from the text
	// after the fillings, then strips them.
	directivesFromText
)

// kindRules is what a kind supports in the rendered view. scheduled marks a
// send no update caused: it waits for the send limiter, while a send that
// answers an update (a command, a trigger, a join or a leave) goes out at once.
type kindRules struct {
	random     bool // one of the %%%-separated parts, picked at random
	fillings   bool // {first}, {chatname}, {rules}, ...
	directives directives
	scheduled  bool
}

var kinds = [...]kindRules{
	KindNote:      {random: true, fillings: true, directives: directivesStored},
	KindFilter:    {random: true, fillings: true},
	KindGreeting:  {fillings: true},
	KindRecurring: {random: true, fillings: true, directives: directivesFromText, scheduled: true},
	KindPermapin:  {},
}

// SavedMessage is a message an admin saved: its stored HTML text, media,
// buttons and stored flags.
type SavedMessage struct {
	Kind        Kind
	Name        string // log label only
	Text        string
	FileID      string
	MsgType     int
	Buttons     []db.Button
	WebPreview  bool
	IsProtected bool
	NoNotif     bool
}

// Target is where a saved message goes and whom its fillings describe.
type Target struct {
	// Chat is the chat whose fillings and fallback language apply; nil means
	// the chat sent to. A note fetched in PM for a group fills from the group.
	Chat       *gotgbot.Chat
	ChatID     int64
	ThreadID   int64
	ReplyMsgID int64 // 0 sends without a reply; a deleted target still sends
	// Member is the user the fillings describe; nil fills them as a person
	// with no name.
	Member *gotgbot.User
	// Limiter paces a scheduled kind; nil means ratelimit.GlobalSendLimiter.
	// Kinds that answer an update never wait for it.
	Limiter *ratelimit.SendLimiter
	// EphemeralTo, when set, shows the message in ChatID only to that user
	// (a Bot API ephemeral message), without a reply. Telegram refuses it
	// unless the bot is an admin, and for web_app or login_url buttons.
	EphemeralTo int64
}

// NoteMessage is the saved message of a stored note.
func NoteMessage(n *db.Notes) SavedMessage {
	return SavedMessage{
		Kind: KindNote, Name: n.NoteName, Text: n.NoteContent, FileID: n.FileID, MsgType: n.MsgType, Buttons: n.Buttons,
		WebPreview: n.WebPreview, IsProtected: n.IsProtected, NoNotif: n.NoNotif,
	}
}

// FilterMessage is the saved message of a stored filter. Filters store no
// preview or protect flag, so both stay off.
func FilterMessage(f *db.ChatFilters) SavedMessage {
	return SavedMessage{
		Kind: KindFilter, Name: f.KeyWord, Text: f.FilterReply, FileID: f.FileID, MsgType: f.MsgType, Buttons: f.Buttons,
		NoNotif: f.NoNotif,
	}
}

// GreetingMessage is a stored welcome or goodbye as a saved message.
// Greetings store no preview, protect or silent flag, so all stay off.
func GreetingMessage(text, fileID string, msgType int, buttons []db.Button) SavedMessage {
	return SavedMessage{Kind: KindGreeting, Name: "greeting", Text: text, FileID: fileID, MsgType: msgType, Buttons: buttons}
}

// SendSaved sends msg to target in view and returns the message Telegram
// accepted.
//
// Rendered applies msg.Kind's row of the kinds table, builds the keyboard and
// sends HTML. Raw is one recipe for every kind: the stored text turned back
// into the markup an admin types plus the button codes, fillings as typed, no
// parse mode, no keyboard and no link preview; media and the protect and
// silent flags are kept. A scheduled kind waits for the limiter; ctx bounds
// that wait.
func SendSaved(ctx context.Context, b *gotgbot.Bot, msg SavedMessage, to Target, view View) (*gotgbot.Message, error) {
	rules := kinds[msg.Kind]
	var (
		text string
		opts sendOptions
	)
	if view == Raw {
		text, opts = raw(msg, rules)
	} else {
		text, opts = render(b, msg, to, rules)
	}
	opts.ChatID, opts.ThreadID, opts.ReplyMsgID, opts.AllowWithoutReply = to.ChatID, to.ThreadID, to.ReplyMsgID, true
	if to.EphemeralTo != 0 {
		opts.ReplyMsgID = 0
		opts.Ephemeral = &gotgbot.EphemeralMessageParameters{ReceiverUserId: to.EphemeralTo}
	}
	payload := outgoing{Text: text, FileID: msg.FileID, MsgType: msg.MsgType, Name: msg.Name}

	if !rules.scheduled {
		return deliver(b, payload, opts)
	}
	limiter := to.Limiter
	if limiter == nil {
		limiter = ratelimit.GlobalSendLimiter()
	}
	var sent *gotgbot.Message
	err := limiter.Do(ctx, func() error {
		var sendErr error
		sent, sendErr = deliver(b, payload, opts)
		return sendErr
	})
	return sent, err
}

func render(b *gotgbot.Bot, msg SavedMessage, to Target, rules kindRules) (string, sendOptions) {
	text, buttons := msg.Text, msg.Buttons
	if rules.random {
		if parts := strings.Split(text, "%%%"); len(parts) > 1 {
			text = parts[rand.Intn(len(parts))] // #nosec G404 - Non-cryptographic random is sufficient for selecting messages
		}
	}
	if rules.fillings {
		chat := to.Chat
		if chat == nil {
			chat = &gotgbot.Chat{Id: to.ChatID}
		}
		text, buttons = formatting.FormattingReplacer(b, chat, to.Member, text, buttons)
	}
	opts := sendOptions{WebPreview: msg.WebPreview, IsProtected: msg.IsProtected, NoNotif: msg.NoNotif}
	switch rules.directives {
	case directivesNone:
	case directivesStored:
		_, _, _, _, _, _, text = content.NotesParser(text)
	case directivesFromText:
		_, _, _, opts.WebPreview, opts.IsProtected, opts.NoNotif, text = content.NotesParser(text)
	}
	opts.Keyboard = &gotgbot.InlineKeyboardMarkup{InlineKeyboard: keyboard.BuildKeyboard(buttons)}
	return text, opts
}

func raw(msg SavedMessage, rules kindRules) (string, sendOptions) {
	opts := sendOptions{NoFormat: true, IsProtected: msg.IsProtected, NoNotif: msg.NoNotif, Keyboard: &gotgbot.InlineKeyboardMarkup{}}
	if rules.directives == directivesFromText {
		_, _, _, _, opts.IsProtected, opts.NoNotif, _ = content.NotesParser(msg.Text)
	}
	return typedSource(msg.Text) + content.RevertButtons(msg.Buttons), opts
}

// typedSource turns stored HTML back into the MarkdownV2 an admin types, with
// the converter /save uses, so the raw view can be sent back unchanged. A
// stored text the converter rejects is shown as stored.
func typedSource(stored string) string {
	source, err := tgmd2html.ReverseV2(stored, nil)
	if err != nil {
		log.WithError(err).Debug("[Media] Could not reverse a stored message for the raw view")
		return stored
	}
	return source
}
