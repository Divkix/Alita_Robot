package media

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/divkix/Alita_Robot/alita/db"
)

type recordingBotClient struct {
	method string
	params map[string]any
	err    error
}

func (c *recordingBotClient) RequestWithContext(_ context.Context, _ string, method string, params map[string]any, _ *gotgbot.RequestOpts) (json.RawMessage, error) {
	c.method = method
	c.params = params
	if c.err != nil {
		return nil, c.err
	}
	return json.RawMessage(`{"message_id":42,"date":1,"chat":{"id":-100,"type":"supergroup"}}`), nil
}

func (c *recordingBotClient) GetAPIURL(_ *gotgbot.RequestOpts) string {
	return gotgbot.DefaultAPIURL
}

func (c *recordingBotClient) FileURL(_ string, tgFilePath string, _ *gotgbot.RequestOpts) string {
	return gotgbot.DefaultAPIURL + "/file/" + tgFilePath
}

func newRecordingBot(client *recordingBotClient) *gotgbot.Bot {
	return &gotgbot.Bot{
		Token:     "123:test",
		BotClient: client,
	}
}

func TestTypeConstantsMatchDB(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		got  int
		want int
	}{
		{name: "TypeText", got: TypeText, want: db.TEXT},
		{name: "TypeSticker", got: TypeSticker, want: db.STICKER},
		{name: "TypeDocument", got: TypeDocument, want: db.DOCUMENT},
		{name: "TypePhoto", got: TypePhoto, want: db.PHOTO},
		{name: "TypeAudio", got: TypeAudio, want: db.AUDIO},
		{name: "TypeVoice", got: TypeVoice, want: db.VOICE},
		{name: "TypeVideo", got: TypeVideo, want: db.VIDEO},
		{name: "TypeVideoNote", got: TypeVideoNote, want: db.VideoNote},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.got != tc.want {
				t.Fatalf("%s = %d, want %d", tc.name, tc.got, tc.want)
			}
		})
	}
}

func TestContentAndOptionsZeroValues(t *testing.T) {
	t.Parallel()

	var content Content
	if content.Text != "" || content.FileID != "" || content.MsgType != 0 || content.Name != "" {
		t.Fatalf("Content zero value = %#v, want empty fields", content)
	}

	var opts Options
	if opts.ChatID != 0 || opts.ReplyMsgID != 0 || opts.ThreadID != 0 {
		t.Fatalf("Options ID zero value = %#v, want zero IDs", opts)
	}
	if opts.Keyboard != nil {
		t.Fatalf("Options keyboard = %#v, want nil", opts.Keyboard)
	}
	if opts.NoFormat || opts.NoNotif || opts.WebPreview || opts.IsProtected || opts.AllowWithoutReply {
		t.Fatalf("Options bool zero value = %#v, want false flags", opts)
	}
}

func TestErrNoPermission(t *testing.T) {
	t.Parallel()

	if ErrNoPermission == nil {
		t.Fatal("ErrNoPermission should not be nil")
	}
	if !strings.Contains(ErrNoPermission.Error(), "permission") {
		t.Fatalf("ErrNoPermission message = %q, want permission context", ErrNoPermission.Error())
	}
}

func TestIsPermissionError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  string
		want bool
	}{
		{name: "not enough rights", err: "not enough rights to send text messages", want: true},
		{name: "no message rights", err: "have no rights to send a message", want: true},
		{name: "write forbidden", err: "CHAT_WRITE_FORBIDDEN", want: true},
		{name: "chat restricted", err: "CHAT_RESTRICTED", want: true},
		{name: "channel admin required", err: "need administrator rights in the channel chat", want: true},
		{name: "unrelated", err: "Bad Request: message text is empty", want: false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := isPermissionError(tc.err); got != tc.want {
				t.Fatalf("isPermissionError(%q) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestResolveSendResult(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		result        string
		err           error
		chatID        int64
		mediaType     string
		wantResult    string
		wantErr       error
		wantErrSubstr string
	}{
		{
			name:       "success returns result",
			result:     "sent",
			chatID:     -100,
			mediaType:  "text",
			wantResult: "sent",
		},
		{
			name:       "permission error returns sentinel",
			result:     "ignored",
			err:        errors.New("CHAT_WRITE_FORBIDDEN"),
			chatID:     -100,
			mediaType:  "text",
			wantResult: "",
			wantErr:    ErrNoPermission,
		},
		{
			name:          "unexpected error is wrapped",
			result:        "ignored",
			err:           errors.New("network failed"),
			chatID:        -100,
			mediaType:     "photo",
			wantResult:    "ignored",
			wantErrSubstr: "failed to send photo to chat -100",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := resolveSendResult(tc.result, tc.err, tc.chatID, tc.mediaType)
			if got != tc.wantResult {
				t.Fatalf("resolveSendResult result = %q, want %q", got, tc.wantResult)
			}
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("resolveSendResult error = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if tc.wantErrSubstr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErrSubstr) {
					t.Fatalf("resolveSendResult error = %v, want substring %q", err, tc.wantErrSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveSendResult error = %v, want nil", err)
			}
		})
	}
}

func TestSendTextBuildsTelegramParams(t *testing.T) {
	t.Parallel()

	client := &recordingBotClient{}
	bot := newRecordingBot(client)

	msg, err := Send(bot, Content{Text: "hello", MsgType: TypeText}, Options{
		ChatID:            -100,
		ReplyMsgID:        123,
		ThreadID:          456,
		NoFormat:          true,
		WebPreview:        true,
		NoNotif:           true,
		IsProtected:       true,
		AllowWithoutReply: true,
	})
	if err != nil {
		t.Fatalf("Send text error = %v", err)
	}
	if msg == nil || msg.MessageId != 42 {
		t.Fatalf("sent message = %#v, want message_id 42", msg)
	}
	if client.method != "sendMessage" {
		t.Fatalf("method = %q, want sendMessage", client.method)
	}

	wantParams := map[string]any{
		"chat_id":              int64(-100),
		"text":                 "hello",
		"message_thread_id":    int64(456),
		"disable_notification": true,
		"protect_content":      true,
	}
	for key, want := range wantParams {
		if got := client.params[key]; !reflect.DeepEqual(got, want) {
			t.Fatalf("param %s = %#v, want %#v", key, got, want)
		}
	}
	if _, ok := client.params["parse_mode"]; ok {
		t.Fatal("parse_mode must be omitted when NoFormat is true")
	}
	reply, ok := client.params["reply_parameters"].(*gotgbot.ReplyParameters)
	if !ok {
		t.Fatalf("reply_parameters = %#v, want *gotgbot.ReplyParameters", client.params["reply_parameters"])
	}
	if reply.MessageId != 123 || !reply.AllowSendingWithoutReply {
		t.Fatalf("reply_parameters = %#v, want message 123 and allow without reply", reply)
	}
	preview, ok := client.params["link_preview_options"].(*gotgbot.LinkPreviewOptions)
	if !ok {
		t.Fatalf("link_preview_options = %#v, want *gotgbot.LinkPreviewOptions", client.params["link_preview_options"])
	}
	if preview.IsDisabled {
		t.Fatal("link preview should be enabled when WebPreview is true")
	}
}

func TestSendMediaFallsBackToTextWhenFileIDMissing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		msgType int
	}{
		{name: "sticker", msgType: TypeSticker},
		{name: "document", msgType: TypeDocument},
		{name: "photo", msgType: TypePhoto},
		{name: "audio", msgType: TypeAudio},
		{name: "voice", msgType: TypeVoice},
		{name: "video", msgType: TypeVideo},
		{name: "video note", msgType: TypeVideoNote},
		{name: "unknown", msgType: 999},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client := &recordingBotClient{}
			bot := newRecordingBot(client)
			_, err := Send(bot, Content{Text: "fallback", MsgType: tc.msgType, Name: tc.name}, Options{ChatID: -100})
			if err != nil {
				t.Fatalf("Send fallback error = %v", err)
			}
			if client.method != "sendMessage" {
				t.Fatalf("fallback method = %q, want sendMessage", client.method)
			}
			if got := client.params["text"]; got != "fallback" {
				t.Fatalf("fallback text = %#v, want fallback", got)
			}
		})
	}
}

func TestSendMediaUsesTelegramMethodForFileID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		msgType   int
		wantKey   string
		wantMedia string
	}{
		{name: "sticker", msgType: TypeSticker, wantKey: "sticker", wantMedia: "sendSticker"},
		{name: "document", msgType: TypeDocument, wantKey: "document", wantMedia: "sendDocument"},
		{name: "photo", msgType: TypePhoto, wantKey: "photo", wantMedia: "sendPhoto"},
		{name: "audio", msgType: TypeAudio, wantKey: "audio", wantMedia: "sendAudio"},
		{name: "voice", msgType: TypeVoice, wantKey: "voice", wantMedia: "sendVoice"},
		{name: "video", msgType: TypeVideo, wantKey: "video", wantMedia: "sendVideo"},
		{name: "video note", msgType: TypeVideoNote, wantKey: "video_note", wantMedia: "sendVideoNote"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client := &recordingBotClient{}
			bot := newRecordingBot(client)
			_, err := Send(bot, Content{
				Text:    "caption",
				FileID:  "file-123",
				MsgType: tc.msgType,
				Name:    tc.name,
			}, Options{
				ChatID:      -100,
				ThreadID:    10,
				IsProtected: true,
				NoNotif:     true,
			})
			if err != nil {
				t.Fatalf("Send media error = %v", err)
			}
			if client.method != tc.wantMedia {
				t.Fatalf("method = %q, want %q", client.method, tc.wantMedia)
			}
			if got := client.params[tc.wantKey]; got == nil {
				t.Fatalf("param %s missing from %#v", tc.wantKey, client.params)
			}
			if got := client.params["message_thread_id"]; got != int64(10) {
				t.Fatalf("message_thread_id = %#v, want 10", got)
			}
			if got := client.params["protect_content"]; got != true {
				t.Fatalf("protect_content = %#v, want true", got)
			}
			if got := client.params["disable_notification"]; got != true {
				t.Fatalf("disable_notification = %#v, want true", got)
			}
		})
	}
}

func TestSendConvenienceWrappersBuildContentAndOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		send      func(*gotgbot.Bot, *gotgbot.InlineKeyboardMarkup) (*gotgbot.Message, error)
		want      textSendWant
		noReplyID bool
	}{
		{
			name: "note",
			send: func(bot *gotgbot.Bot, keyboard *gotgbot.InlineKeyboardMarkup) (*gotgbot.Message, error) {
				note := &db.Notes{
					NoteName:    "rules",
					NoteContent: "note text",
					MsgType:     TypeText,
					NoNotif:     true,
					WebPreview:  true,
					IsProtected: true,
				}
				return SendNote(bot, -100, note, keyboard, 111, 222)
			},
			want: textSendWant{
				text:           "note text",
				threadID:       222,
				replyID:        111,
				noNotif:        true,
				webPreview:     true,
				protected:      true,
				replyMarkupSet: true,
			},
		},
		{
			name: "filter",
			send: func(bot *gotgbot.Bot, keyboard *gotgbot.InlineKeyboardMarkup) (*gotgbot.Message, error) {
				filter := &db.ChatFilters{
					KeyWord:     "hello",
					FilterReply: "filter text",
					MsgType:     TypeText,
					NoNotif:     true,
				}
				return SendFilter(bot, -200, filter, keyboard, 333, 444)
			},
			want: textSendWant{
				text:           "filter text",
				threadID:       444,
				replyID:        333,
				noNotif:        true,
				replyMarkupSet: true,
			},
		},
		{
			name: "greeting",
			send: func(bot *gotgbot.Bot, keyboard *gotgbot.InlineKeyboardMarkup) (*gotgbot.Message, error) {
				return SendGreeting(bot, -300, "welcome", "", TypeText, keyboard, 555)
			},
			want: textSendWant{
				text:           "welcome",
				threadID:       555,
				replyMarkupSet: true,
			},
			noReplyID: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client := &recordingBotClient{}
			bot := newRecordingBot(client)
			keyboard := &gotgbot.InlineKeyboardMarkup{}
			if _, err := tc.send(bot, keyboard); err != nil {
				t.Fatalf("%s send error = %v", tc.name, err)
			}
			assertTextSendParams(t, client, tc.want)
			if tc.noReplyID {
				if _, ok := client.params["reply_parameters"]; ok {
					t.Fatalf("%s reply_parameters = %#v, want omitted", tc.name, client.params["reply_parameters"])
				}
			}
		})
	}
}

type textSendWant struct {
	text           string
	threadID       int64
	replyID        int64
	noNotif        bool
	webPreview     bool
	protected      bool
	replyMarkupSet bool
}

func assertTextSendParams(t *testing.T, client *recordingBotClient, want textSendWant) {
	t.Helper()

	if client.method != "sendMessage" {
		t.Fatalf("method = %q, want sendMessage", client.method)
	}
	if got := client.params["text"]; got != want.text {
		t.Fatalf("text = %#v, want %q", got, want.text)
	}
	if got := client.params["message_thread_id"]; got != want.threadID {
		t.Fatalf("message_thread_id = %#v, want %d", got, want.threadID)
	}
	if got := boolParam(client.params, "disable_notification"); got != want.noNotif {
		t.Fatalf("disable_notification = %#v, want %v", got, want.noNotif)
	}
	if got := boolParam(client.params, "protect_content"); got != want.protected {
		t.Fatalf("protect_content = %#v, want %v", got, want.protected)
	}
	preview, ok := client.params["link_preview_options"].(*gotgbot.LinkPreviewOptions)
	if !ok {
		t.Fatalf("link_preview_options = %#v, want *gotgbot.LinkPreviewOptions", client.params["link_preview_options"])
	}
	if preview.IsDisabled == want.webPreview {
		t.Fatalf("link preview disabled = %v, want webPreview %v", preview.IsDisabled, want.webPreview)
	}
	if want.replyID > 0 {
		reply, ok := client.params["reply_parameters"].(*gotgbot.ReplyParameters)
		if !ok {
			t.Fatalf("reply_parameters = %#v, want *gotgbot.ReplyParameters", client.params["reply_parameters"])
		}
		if reply.MessageId != want.replyID || !reply.AllowSendingWithoutReply {
			t.Fatalf("reply_parameters = %#v, want message %d with allow", reply, want.replyID)
		}
	}
	if want.replyMarkupSet && client.params["reply_markup"] == nil {
		t.Fatal("reply_markup missing")
	}
}

func boolParam(params map[string]any, key string) bool {
	value, _ := params[key].(bool)
	return value
}
