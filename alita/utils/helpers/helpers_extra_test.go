package helpers

import (
	"fmt"
	"strings"
	"testing"

	tgmd2html "github.com/PaulSonOfLars/gotg_md2html"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"

	"github.com/divkix/Alita_Robot/alita/db"
)

// ---------------------------------------------------------------------------
// AddCmdToDisableable
// ---------------------------------------------------------------------------

func TestAddCmdToDisableable(t *testing.T) {
	const testCmd = "test_disableable_cmd_42"

	cmdsMu.Lock()
	orig := make([]string, len(DisableCmds))
	copy(orig, DisableCmds)
	cmdsMu.Unlock()

	defer func() {
		cmdsMu.Lock()
		DisableCmds = orig
		cmdsMu.Unlock()
	}()

	AddCmdToDisableable(testCmd)

	cmdsMu.Lock()
	found := false
	for _, c := range DisableCmds {
		if c == testCmd {
			found = true
			break
		}
	}
	cmdsMu.Unlock()

	if !found {
		t.Fatalf("expected %q in DisableCmds", testCmd)
	}
}

func TestAddCmdToDisableableThreadSafe(t *testing.T) {
	cmdsMu.Lock()
	orig := make([]string, len(DisableCmds))
	copy(orig, DisableCmds)
	cmdsMu.Unlock()

	defer func() {
		cmdsMu.Lock()
		DisableCmds = orig
		cmdsMu.Unlock()
	}()

	const n = 100
	done := make(chan bool, n)
	for i := 0; i < n; i++ {
		go func(idx int) {
			AddCmdToDisableable(fmt.Sprintf("concurrent_cmd_%d", idx))
			done <- true
		}(i)
	}
	for i := 0; i < n; i++ {
		<-done
	}

	cmdsMu.Lock()
	count := len(DisableCmds)
	cmdsMu.Unlock()

	if count != len(orig)+n {
		t.Fatalf("expected %d commands in DisableCmds, got %d", len(orig)+n, count)
	}
}

// ---------------------------------------------------------------------------
// MultiCommand
// ---------------------------------------------------------------------------

func TestMultiCommand(t *testing.T) {
	t.Parallel()

	b := &gotgbot.Bot{Token: "123:abc"}
	d := ext.NewDispatcher(&ext.DispatcherOpts{})

	called := make(map[string]bool)
	handler := func(cmd string) handlers.Response {
		return func(_ *gotgbot.Bot, _ *ext.Context) error {
			called[cmd] = true
			return nil
		}
	}

	MultiCommand(d, []string{"cmda", "cmdb"}, handler("multi"))

	u1 := &gotgbot.Update{
		Message: &gotgbot.Message{
			Chat:     gotgbot.Chat{Id: 1, Type: "private"},
			From:     &gotgbot.User{Id: 1, IsBot: false, FirstName: "T"},
			Text:     "/cmda",
			Entities: []gotgbot.MessageEntity{{Type: "bot_command", Offset: 0, Length: 5}},
		},
	}
	if err := d.ProcessUpdate(b, u1, nil); err != nil {
		t.Fatalf("ProcessUpdate(/cmda) unexpected error: %v", err)
	}
	if !called["multi"] {
		t.Fatal("expected handler to be called for /cmda")
	}

	called = make(map[string]bool)
	u2 := &gotgbot.Update{
		Message: &gotgbot.Message{
			Chat:     gotgbot.Chat{Id: 1, Type: "private"},
			From:     &gotgbot.User{Id: 1, IsBot: false, FirstName: "T"},
			Text:     "/cmdb",
			Entities: []gotgbot.MessageEntity{{Type: "bot_command", Offset: 0, Length: 5}},
		},
	}
	if err := d.ProcessUpdate(b, u2, nil); err != nil {
		t.Fatalf("ProcessUpdate(/cmdb) unexpected error: %v", err)
	}
	if !called["multi"] {
		t.Fatal("expected handler to be called for /cmdb")
	}

	called = make(map[string]bool)
	u3 := &gotgbot.Update{
		Message: &gotgbot.Message{
			Chat:     gotgbot.Chat{Id: 1, Type: "private"},
			From:     &gotgbot.User{Id: 1, IsBot: false, FirstName: "T"},
			Text:     "/cmdc",
			Entities: []gotgbot.MessageEntity{{Type: "bot_command", Offset: 0, Length: 5}},
		},
	}
	if err := d.ProcessUpdate(b, u3, nil); err != nil {
		t.Fatalf("ProcessUpdate(/cmdc) unexpected error: %v", err)
	}
	if called["multi"] {
		t.Fatal("expected handler NOT to be called for /cmdc")
	}
}

// ---------------------------------------------------------------------------
// preFixes
// ---------------------------------------------------------------------------

func TestPreFixesTextTooLong(t *testing.T) {
	t.Parallel()

	text := strings.Repeat("a", 4097)
	dataType := db.TEXT
	var buttons []tgmd2html.ButtonV2
	var dbButtons []db.Button
	errorMsg := "initial"

	preFixes(buttons, "btn", &text, &dataType, "", &dbButtons, &errorMsg, "en")

	if dataType != -1 {
		t.Fatalf("expected dataType=-1 for long text, got %d", dataType)
	}
	if errorMsg == "initial" {
		t.Fatalf("expected errorMsg to be set")
	}
}

func TestPreFixesCaptionTooLong(t *testing.T) {
	t.Parallel()

	text := strings.Repeat("a", 1025)
	dataType := db.PHOTO
	var buttons []tgmd2html.ButtonV2
	var dbButtons []db.Button
	errorMsg := "initial"

	preFixes(buttons, "btn", &text, &dataType, "file123", &dbButtons, &errorMsg, "en")

	if dataType != -1 {
		t.Fatalf("expected dataType=-1 for long caption, got %d", dataType)
	}
}

func TestPreFixesValid(t *testing.T) {
	t.Parallel()

	text := "hello world"
	dataType := db.TEXT
	buttons := []tgmd2html.ButtonV2{
		{Name: "", Content: "https://example.com", SameLine: false},
		{Name: "Named", Content: "not-a-url", SameLine: false},
	}
	var dbButtons []db.Button
	errorMsg := "initial"

	preFixes(buttons, "Default", &text, &dataType, "", &dbButtons, &errorMsg, "en")

	if dataType != db.TEXT {
		t.Fatalf("expected dataType=%d, got %d", db.TEXT, dataType)
	}
	if len(dbButtons) != 1 {
		t.Fatalf("expected 1 valid button, got %d", len(dbButtons))
	}
	if dbButtons[0].Name != "Default" {
		t.Fatalf("expected default name %q, got %q", "Default", dbButtons[0].Name)
	}
	if dbButtons[0].Url != "https://example.com" {
		t.Fatalf("expected URL %q, got %q", "https://example.com", dbButtons[0].Url)
	}
	if text != "hello world" {
		t.Fatalf("expected text unchanged, got %q", text)
	}
}

func TestPreFixesEmptyTextNoFile(t *testing.T) {
	t.Parallel()

	text := "   \n\t\r "
	dataType := db.TEXT
	var buttons []tgmd2html.ButtonV2
	var dbButtons []db.Button
	errorMsg := "initial"

	preFixes(buttons, "btn", &text, &dataType, "", &dbButtons, &errorMsg, "en")

	if dataType != -1 {
		t.Fatalf("expected dataType=-1 for empty text and no file, got %d", dataType)
	}
}

func TestPreFixesEmptyTextWithFile(t *testing.T) {
	t.Parallel()

	text := "   \n\t\r "
	dataType := db.PHOTO
	var buttons []tgmd2html.ButtonV2
	var dbButtons []db.Button
	errorMsg := "initial"

	preFixes(buttons, "btn", &text, &dataType, "file123", &dbButtons, &errorMsg, "en")

	if dataType != db.PHOTO {
		t.Fatalf("expected dataType=%d, got %d", db.PHOTO, dataType)
	}
}

// ---------------------------------------------------------------------------
// setRawText
// ---------------------------------------------------------------------------

func TestSetRawText(t *testing.T) {
	tests := []struct {
		name string
		msg  *gotgbot.Message
		want string
	}{
		{
			name: "direct text",
			msg:  &gotgbot.Message{Text: "/note hello world"},
			want: "hello world",
		},
		{
			name: "direct caption",
			msg:  &gotgbot.Message{Caption: "/note hello cap"},
			want: "hello cap",
		},
		{
			name: "reply text",
			msg: &gotgbot.Message{
				Text: "/note mykeyword",
				ReplyToMessage: &gotgbot.Message{
					Text: "reply text content",
				},
			},
			want: "reply text content",
		},
		{
			name: "reply caption",
			msg: &gotgbot.Message{
				Text: "/note mykeyword",
				ReplyToMessage: &gotgbot.Message{
					Caption: "reply caption content",
				},
			},
			want: "reply caption content",
		},
		{
			name: "reply with args",
			msg: &gotgbot.Message{
				Text: "/note arg1 extra content here",
				ReplyToMessage: &gotgbot.Message{
					Text: "reply text",
				},
			},
			want: "extra content here",
		},
		{
			name: "command only",
			msg:  &gotgbot.Message{Text: "/note"},
			want: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var rawText string
			var args []string
			if tc.msg.Text != "" {
				args = strings.Fields(tc.msg.Text)[1:]
			} else if tc.msg.Caption != "" {
				args = strings.Fields(tc.msg.Caption)[1:]
			}
			setRawText(tc.msg, args, &rawText)
			if rawText != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, rawText)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// extractMediaFromReply
// ---------------------------------------------------------------------------

func TestExtractMediaFromReply(t *testing.T) {
	tests := []struct {
		name     string
		msg      *gotgbot.Message
		wantFile string
		wantType int
	}{
		{
			name: "photo",
			msg: &gotgbot.Message{
				Photo: []gotgbot.PhotoSize{
					{FileId: "small", Width: 100, Height: 100},
					{FileId: "large_photo", Width: 800, Height: 600},
				},
			},
			wantFile: "large_photo",
			wantType: db.PHOTO,
		},
		{
			name:     "video",
			msg:      &gotgbot.Message{Video: &gotgbot.Video{FileId: "video_123"}},
			wantFile: "video_123",
			wantType: db.VIDEO,
		},
		{
			name:     "audio",
			msg:      &gotgbot.Message{Audio: &gotgbot.Audio{FileId: "audio_456"}},
			wantFile: "audio_456",
			wantType: db.AUDIO,
		},
		{
			name:     "voice",
			msg:      &gotgbot.Message{Voice: &gotgbot.Voice{FileId: "voice_789"}},
			wantFile: "voice_789",
			wantType: db.VOICE,
		},
		{
			name:     "video note",
			msg:      &gotgbot.Message{VideoNote: &gotgbot.VideoNote{FileId: "vn_abc"}},
			wantFile: "vn_abc",
			wantType: db.VideoNote,
		},
		{
			name:     "document",
			msg:      &gotgbot.Message{Document: &gotgbot.Document{FileId: "doc_def"}},
			wantFile: "doc_def",
			wantType: db.DOCUMENT,
		},
		{
			name:     "sticker",
			msg:      &gotgbot.Message{Sticker: &gotgbot.Sticker{FileId: "sticker_ghi"}},
			wantFile: "sticker_ghi",
			wantType: db.STICKER,
		},
		{
			name:     "animation",
			msg:      &gotgbot.Message{Animation: &gotgbot.Animation{FileId: "anim_xyz"}},
			wantFile: "anim_xyz",
			wantType: db.DOCUMENT,
		},
		{
			name:     "text only",
			msg:      &gotgbot.Message{Text: "Hello world"},
			wantFile: "",
			wantType: -1,
		},
		{
			name:     "nil message",
			msg:      nil,
			wantFile: "",
			wantType: -1,
		},
		{
			name: "sticker priority over document",
			msg: &gotgbot.Message{
				Sticker:  &gotgbot.Sticker{FileId: "sticker_prio"},
				Document: &gotgbot.Document{FileId: "doc_other"},
			},
			wantFile: "sticker_prio",
			wantType: db.STICKER,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fileID, dt := extractMediaFromReply(tc.msg)
			if fileID != tc.wantFile {
				t.Fatalf("expected file_id %q, got %q", tc.wantFile, fileID)
			}
			if dt != tc.wantType {
				t.Fatalf("expected dataType=%d, got %d", tc.wantType, dt)
			}
		})
	}
}

func TestFormattingReplacerWrapperWithoutRules(t *testing.T) {
	t.Parallel()

	chat := &gotgbot.Chat{Id: -1001234567890, Title: "Test Chat"}
	user := &gotgbot.User{Id: 42, FirstName: "Ada", LastName: "Lovelace"}

	got, buttons := FormattingReplacer(nil, chat, user, "Hi {fullname} in {chatname}", nil)
	if got != "Hi Ada Lovelace in Test Chat" {
		t.Fatalf("FormattingReplacer() = %q", got)
	}
	if len(buttons) != 0 {
		t.Fatalf("FormattingReplacer() buttons = %#v, want none", buttons)
	}
}
