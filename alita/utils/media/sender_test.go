package media

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/alicebob/miniredis/v2"
	gocache "github.com/eko/gocache/lib/v4/cache"
	"github.com/eko/gocache/lib/v4/marshaler"
	gocache_store "github.com/eko/gocache/store/redis/v4"
	"github.com/redis/go-redis/v9"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/ratelimit"
)

type recordingBotClient struct {
	method string
	params map[string]any
	err    error
	calls  int // requests made, for asserting that nothing was sent
}

func (c *recordingBotClient) RequestWithContext(_ context.Context, _ string, method string, params map[string]any, _ *gotgbot.RequestOpts) (json.RawMessage, error) {
	c.calls++
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

func TestErrNoPermission(t *testing.T) {
	t.Parallel()

	if ErrNoPermission == nil {
		t.Fatal("ErrNoPermission should not be nil")
	}
	if !strings.Contains(ErrNoPermission.Error(), "permission") {
		t.Fatalf("ErrNoPermission message = %q, want permission context", ErrNoPermission.Error())
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

	msg, err := deliver(bot, outgoing{Text: "hello", MsgType: db.TEXT}, sendOptions{
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
		{name: "sticker", msgType: db.STICKER},
		{name: "document", msgType: db.DOCUMENT},
		{name: "photo", msgType: db.PHOTO},
		{name: "audio", msgType: db.AUDIO},
		{name: "voice", msgType: db.VOICE},
		{name: "video", msgType: db.VIDEO},
		{name: "video note", msgType: db.VIDEO_NOTE},
		{name: "unknown", msgType: 999},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client := &recordingBotClient{}
			bot := newRecordingBot(client)
			_, err := deliver(bot, outgoing{Text: "fallback", MsgType: tc.msgType, Name: tc.name}, sendOptions{ChatID: -100})
			if err != nil {
				t.Fatalf("Send fallback error = %v", err)
			}
			if client.method != "sendMessage" {
				t.Fatalf("fallback method = %q, want sendMessage", client.method)
			}
			if got := client.params["text"]; got != "fallback" {
				t.Fatalf("fallback text = %#v, want fallback", got)
			}

			plain := &recordingBotClient{}
			if _, err := deliver(newRecordingBot(plain), outgoing{Text: "<b>", MsgType: tc.msgType, Name: tc.name}, sendOptions{ChatID: -100, NoFormat: true}); err != nil {
				t.Fatalf("Send NoFormat fallback error = %v", err)
			}
			if _, ok := plain.params["parse_mode"]; ok {
				t.Fatal("NoFormat fallback must send without parse_mode")
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
		{name: "sticker", msgType: db.STICKER, wantKey: "sticker", wantMedia: "sendSticker"},
		{name: "document", msgType: db.DOCUMENT, wantKey: "document", wantMedia: "sendDocument"},
		{name: "photo", msgType: db.PHOTO, wantKey: "photo", wantMedia: "sendPhoto"},
		{name: "audio", msgType: db.AUDIO, wantKey: "audio", wantMedia: "sendAudio"},
		{name: "voice", msgType: db.VOICE, wantKey: "voice", wantMedia: "sendVoice"},
		{name: "video", msgType: db.VIDEO, wantKey: "video", wantMedia: "sendVideo"},
		{name: "video note", msgType: db.VIDEO_NOTE, wantKey: "video_note", wantMedia: "sendVideoNote"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client := &recordingBotClient{}
			bot := newRecordingBot(client)
			_, err := deliver(bot, outgoing{
				Text:    "caption",
				FileID:  "file-123",
				MsgType: tc.msgType,
				Name:    tc.name,
			}, sendOptions{
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

var allKinds = []struct {
	name string
	kind Kind
}{
	{"note", KindNote},
	{"filter", KindFilter},
	{"greeting", KindGreeting},
	{"recurring", KindRecurring},
	{"permapin", KindPermapin},
}

var testButtons = []db.Button{
	{Name: "A", Url: "https://example.com/a"},
	{Name: "B", Url: "https://example.com/b", SameLine: true},
}

func sendSaved(t *testing.T, msg SavedMessage, to Target, view View) *recordingBotClient {
	t.Helper()
	client := &recordingBotClient{}
	if _, err := SendSaved(context.Background(), newRecordingBot(client), msg, to, view); err != nil {
		t.Fatalf("SendSaved error = %v", err)
	}
	return client
}

// groupTarget answers message 9 in topic 7 of a group, for member Ann. Its own
// fast limiter keeps scheduled kinds off the global one.
func groupTarget() Target {
	return Target{
		Chat:       &gotgbot.Chat{Id: -100, Type: "supergroup", Title: "Group"},
		ChatID:     -100,
		ThreadID:   7,
		ReplyMsgID: 9,
		Member:     &gotgbot.User{Id: 1, FirstName: "Ann"},
		Limiter:    ratelimit.NewSendLimiter(10000, 1000),
	}
}

func TestSendSavedRenderedFollowsTheKindTable(t *testing.T) {
	t.Parallel()

	// Stored flags are off; the text asks for a silent send.
	text := "Hi {first} in {chatname}{nonotif}"
	want := map[Kind]struct {
		text   string
		silent bool
	}{
		KindNote:      {"Hi Ann in Group", false}, // directives were stored as flags at save time; text ones are stripped
		KindFilter:    {"Hi Ann in Group{nonotif}", false},
		KindGreeting:  {"Hi Ann in Group{nonotif}", false},
		KindRecurring: {"Hi Ann in Group", true}, // directives read from the text
		KindPermapin:  {"Hi {first} in {chatname}{nonotif}", false},
	}
	for _, tc := range allKinds {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client := sendSaved(t, SavedMessage{Kind: tc.kind, Text: text, MsgType: db.TEXT, Buttons: testButtons}, groupTarget(), Rendered)
			assertTextSendParams(t, client, textSendWant{text: want[tc.kind].text, threadID: 7, replyID: 9, noNotif: want[tc.kind].silent})
			if got := client.params["chat_id"]; got != int64(-100) {
				t.Fatalf("chat_id = %#v, want -100", got)
			}
			if got := client.params["parse_mode"]; got != "HTML" {
				t.Fatalf("parse_mode = %#v, want HTML", got)
			}
			markup, ok := client.params["reply_markup"].(*gotgbot.InlineKeyboardMarkup)
			if !ok || len(markup.InlineKeyboard) != 1 || len(markup.InlineKeyboard[0]) != 2 {
				t.Fatalf("reply_markup = %#v, want one row of A and B", client.params["reply_markup"])
			}
		})
	}
}

func TestSendSavedRandomParts(t *testing.T) {
	t.Parallel()

	random := map[Kind]bool{KindNote: true, KindFilter: true, KindRecurring: true}
	for _, tc := range allKinds {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			one := sendSaved(t, SavedMessage{Kind: tc.kind, Text: "only", MsgType: db.TEXT}, groupTarget(), Rendered)
			if got := one.params["text"]; got != "only" {
				t.Fatalf("one part text = %#v, want only", got)
			}
			seen := map[any]bool{}
			for range 60 {
				seen[sendSaved(t, SavedMessage{Kind: tc.kind, Text: "a%%%b%%%c", MsgType: db.TEXT}, groupTarget(), Rendered).params["text"]] = true
			}
			want := map[any]bool{"a%%%b%%%c": true}
			if random[tc.kind] {
				want = map[any]bool{"a": true, "b": true, "c": true}
			}
			if !reflect.DeepEqual(seen, want) {
				t.Fatalf("texts sent = %v, want %v", seen, want)
			}
		})
	}
}

func TestSendSavedFillings(t *testing.T) {
	t.Parallel()

	member := groupTarget()
	member.Member = &gotgbot.User{Id: 42, FirstName: "Ann", LastName: "Lee", Username: "ann"}
	if got := sendSaved(t, SavedMessage{Kind: KindNote, Text: "{fullname} {username} {id}", MsgType: db.TEXT}, member, Rendered).params["text"]; got != "Ann Lee @ann 42" {
		t.Fatalf("text with member = %#v", got)
	}

	nobody := groupTarget()
	nobody.Member = nil
	if got := sendSaved(t, SavedMessage{Kind: KindRecurring, Text: "Hi {first}, id {id}", MsgType: db.TEXT}, nobody, Rendered).params["text"]; got != "Hi PersonWithNoName, id 0" {
		t.Fatalf("text without member = %#v", got)
	}
}

func TestSendSavedFillsFromTheTargetChatButSendsToTheTargetChatID(t *testing.T) {
	t.Parallel()

	// A note fetched in PM for a group: fillings describe the group, the send goes to the PM.
	to := Target{Chat: &gotgbot.Chat{Id: -100, Type: "supergroup", Title: "Group"}, ChatID: 555, Member: &gotgbot.User{Id: 555, FirstName: "Ann"}}
	client := sendSaved(t, SavedMessage{Kind: KindNote, Text: "Welcome to {chatname}!", MsgType: db.TEXT}, to, Rendered)
	if got := client.params["text"]; got != "Welcome to Group!" {
		t.Fatalf("text = %#v", got)
	}
	if got := client.params["chat_id"]; got != int64(555) {
		t.Fatalf("chat_id = %#v, want 555", got)
	}
	if _, ok := client.params["reply_parameters"]; ok {
		t.Fatal("reply_parameters must be omitted without a message to reply to")
	}
}

func TestSendSavedDirectives(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		msg  SavedMessage
		want textSendWant
	}{
		{"stored preview", SavedMessage{Kind: KindNote, Text: "t", WebPreview: true}, textSendWant{text: "t", webPreview: true}},
		{"stored protect", SavedMessage{Kind: KindNote, Text: "t", IsProtected: true}, textSendWant{text: "t", protected: true}},
		{"stored nonotif", SavedMessage{Kind: KindNote, Text: "t", NoNotif: true}, textSendWant{text: "t", noNotif: true}},
		{"stored filter nonotif", SavedMessage{Kind: KindFilter, Text: "t", NoNotif: true}, textSendWant{text: "t", noNotif: true}},
		{"text preview", SavedMessage{Kind: KindRecurring, Text: "{preview}t"}, textSendWant{text: "t", webPreview: true}},
		{"text protect", SavedMessage{Kind: KindRecurring, Text: "t{protect}"}, textSendWant{text: "t", protected: true}},
		{"text nonotif", SavedMessage{Kind: KindRecurring, Text: "t{nonotif}"}, textSendWant{text: "t", noNotif: true}},
		{"recurring ignores stored flags", SavedMessage{Kind: KindRecurring, Text: "t", WebPreview: true, IsProtected: true, NoNotif: true}, textSendWant{text: "t"}},
		{"note strips every directive", SavedMessage{Kind: KindNote, Text: "{private}{noprivate}{admin}{preview}{protect}{nonotif}t"}, textSendWant{text: "t"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.msg.MsgType = db.TEXT
			tc.want.threadID, tc.want.replyID = 7, 9
			assertTextSendParams(t, sendSaved(t, tc.msg, groupTarget(), Rendered), tc.want)
		})
	}
}

func TestSendSavedMediaTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, method, fileKey string
		msgType               int
		caption               bool
	}{
		{"text", "sendMessage", "", db.TEXT, false},
		{"sticker", "sendSticker", "sticker", db.STICKER, false},
		{"document", "sendDocument", "document", db.DOCUMENT, true},
		{"photo", "sendPhoto", "photo", db.PHOTO, true},
		{"audio", "sendAudio", "audio", db.AUDIO, true},
		{"voice", "sendVoice", "voice", db.VOICE, true},
		{"video", "sendVideo", "video", db.VIDEO, true},
		{"video note", "sendVideoNote", "video_note", db.VIDEO_NOTE, false},
	}
	for _, tc := range tests {
		for _, view := range []View{Rendered, Raw} {
			t.Run(fmt.Sprintf("%s/%d", tc.name, view), func(t *testing.T) {
				t.Parallel()
				msg := SavedMessage{Kind: KindNote, Text: "<b>Hi</b> {first}", MsgType: tc.msgType, IsProtected: true, NoNotif: true}
				if tc.fileKey != "" {
					msg.FileID = "file-1"
				}
				client := sendSaved(t, msg, groupTarget(), view)
				if client.method != tc.method {
					t.Fatalf("method = %q, want %q", client.method, tc.method)
				}
				if tc.fileKey != "" && client.params[tc.fileKey] == nil {
					t.Fatalf("%s missing from %#v", tc.fileKey, client.params)
				}
				wantText := map[View]string{Rendered: "<b>Hi</b> Ann", Raw: "*Hi* {first}"}[view]
				textKey := "caption"
				if tc.fileKey == "" {
					textKey = "text"
				}
				if tc.caption || tc.fileKey == "" {
					if got := client.params[textKey]; got != wantText {
						t.Fatalf("%s = %#v, want %q", textKey, got, wantText)
					}
					if _, ok := client.params["parse_mode"]; ok == (view == Raw) {
						t.Fatalf("parse_mode = %#v in view %d", client.params["parse_mode"], view)
					}
				}
				for _, key := range []string{"protect_content", "disable_notification"} {
					if client.params[key] != true {
						t.Fatalf("%s = %#v, want true", key, client.params[key])
					}
				}
				if client.params["chat_id"] != int64(-100) || client.params["message_thread_id"] != int64(7) {
					t.Fatalf("chat/topic = %#v/%#v", client.params["chat_id"], client.params["message_thread_id"])
				}
			})
		}
	}
}

func TestSendSavedRawViewIsOneRecipeForEveryKind(t *testing.T) {
	t.Parallel()

	text := `<b>Hi</b> <i>{first}</i> <a href="https://example.com">site</a>{protect}{nonotif}%%%two`
	wantText := "*Hi* _{first}_ [site](https://example.com){protect}{nonotif}%%%two" +
		"\n[A](buttonurl://https://example.com/a)\n[B](buttonurl://https://example.com/b:same)"
	for _, tc := range allKinds {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			msg := SavedMessage{Kind: tc.kind, Text: text, MsgType: db.TEXT, Buttons: testButtons, WebPreview: true, IsProtected: true, NoNotif: true}
			client := sendSaved(t, msg, groupTarget(), Raw)
			assertTextSendParams(t, client, textSendWant{text: wantText, threadID: 7, replyID: 9, noNotif: true, protected: true})
			if _, ok := client.params["parse_mode"]; ok {
				t.Fatalf("parse_mode = %#v, want omitted", client.params["parse_mode"])
			}
			markup, ok := client.params["reply_markup"].(*gotgbot.InlineKeyboardMarkup)
			if !ok || len(markup.InlineKeyboard) != 0 {
				t.Fatalf("reply_markup = %#v, want an empty keyboard", client.params["reply_markup"])
			}
		})
	}
}

func TestSendSavedLimiterChoice(t *testing.T) {
	t.Parallel()

	paused := ratelimit.NewSendLimiter(ratelimit.DefaultSendRate, ratelimit.DefaultSendBurst)
	paused.Pause(2 * ratelimit.MaxSendRetryAfter)
	for _, tc := range allKinds {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client := &recordingBotClient{}
			_, err := SendSaved(context.Background(), newRecordingBot(client), SavedMessage{Kind: tc.kind, Text: "t", MsgType: db.TEXT}, Target{ChatID: -100, Limiter: paused}, Rendered)
			if tc.kind == KindRecurring {
				if !errors.Is(err, ratelimit.ErrSendPaused) || client.calls != 0 {
					t.Fatalf("scheduled send err = %v calls = %d, want ErrSendPaused and no request", err, client.calls)
				}
				return
			}
			if err != nil || client.calls != 1 {
				t.Fatalf("update send err = %v calls = %d, want sent at once", err, client.calls)
			}
		})
	}
}

func TestSendSavedScheduledSendWaitsForTheLimiter(t *testing.T) {
	t.Parallel()

	const pause = 150 * time.Millisecond
	limiter := ratelimit.NewSendLimiter(ratelimit.DefaultSendRate, ratelimit.DefaultSendBurst)
	limiter.Pause(pause)
	start := time.Now()
	client := sendSaved(t, SavedMessage{Kind: KindRecurring, Text: "t", MsgType: db.TEXT}, Target{ChatID: -100, Limiter: limiter}, Rendered)
	if elapsed := time.Since(start); elapsed < pause*2/3 || client.calls != 1 {
		t.Fatalf("scheduled send took %v with %d requests, want one request after the %v pause", elapsed, client.calls, pause)
	}
}

func TestSendSavedSkipsARestrictedChat(t *testing.T) {
	mr := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	manager := gocache.New[any](gocache_store.NewRedis(redisClient))
	prevMarshal, prevManager, prevClient := cache.GetCacheState()
	cache.SetCacheState(marshaler.New(manager), manager, redisClient)
	t.Cleanup(func() {
		cache.SetCacheState(prevMarshal, prevManager, prevClient)
		_ = redisClient.Close()
	})

	const chatID = -777
	cache.MarkChatRestricted(chatID)
	client := &recordingBotClient{}
	_, err := SendSaved(context.Background(), newRecordingBot(client), SavedMessage{Kind: KindNote, Text: "t", MsgType: db.TEXT}, Target{ChatID: chatID}, Rendered)
	if !errors.Is(err, ErrNoPermission) || client.calls != 0 {
		t.Fatalf("err = %v calls = %d, want ErrNoPermission and no request", err, client.calls)
	}
}
