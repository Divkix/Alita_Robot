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

	cmdsMu.Lock()
	DisableCmds = orig
	cmdsMu.Unlock()
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
	_ = d.ProcessUpdate(b, u1, nil)
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
	_ = d.ProcessUpdate(b, u2, nil)
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
	_ = d.ProcessUpdate(b, u3, nil)
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

func TestSetRawTextDirectText(t *testing.T) {
	t.Parallel()

	msg := &gotgbot.Message{Text: "/note hello world"}
	args := strings.Fields(msg.Text)[1:]
	var rawText string
	setRawText(msg, args, &rawText)

	if rawText != "hello world" {
		t.Fatalf("expected %q, got %q", "hello world", rawText)
	}
}

func TestSetRawTextDirectCaption(t *testing.T) {
	t.Parallel()

	msg := &gotgbot.Message{Caption: "/note hello cap"}
	args := strings.Fields(msg.Caption)[1:]
	var rawText string
	setRawText(msg, args, &rawText)

	if rawText != "hello cap" {
		t.Fatalf("expected %q, got %q", "hello cap", rawText)
	}
}

func TestSetRawTextReplyText(t *testing.T) {
	t.Parallel()

	msg := &gotgbot.Message{
		Text: "/note mykeyword",
		ReplyToMessage: &gotgbot.Message{
			Text: "reply text content",
		},
	}
	args := strings.Fields(msg.Text)[1:]
	var rawText string
	setRawText(msg, args, &rawText)

	if rawText != "reply text content" {
		t.Fatalf("expected %q, got %q", "reply text content", rawText)
	}
}

func TestSetRawTextReplyCaption(t *testing.T) {
	t.Parallel()

	msg := &gotgbot.Message{
		Text: "/note mykeyword",
		ReplyToMessage: &gotgbot.Message{
			Caption: "reply caption content",
		},
	}
	args := strings.Fields(msg.Text)[1:]
	var rawText string
	setRawText(msg, args, &rawText)

	if rawText != "reply caption content" {
		t.Fatalf("expected %q, got %q", "reply caption content", rawText)
	}
}

func TestSetRawTextReplyWithArgs(t *testing.T) {
	t.Parallel()

	msg := &gotgbot.Message{
		Text: "/note arg1 extra content here",
		ReplyToMessage: &gotgbot.Message{
			Text: "reply text",
		},
	}
	args := strings.Fields(msg.Text)[1:]
	var rawText string
	setRawText(msg, args, &rawText)

	if rawText != "extra content here" {
		t.Fatalf("expected %q, got %q", "extra content here", rawText)
	}
}

func TestSetRawTextOnlyCommand(t *testing.T) {
	t.Parallel()

	msg := &gotgbot.Message{Text: "/note"}
	args := strings.Fields(msg.Text)[1:]
	var rawText string
	setRawText(msg, args, &rawText)

	if rawText != "" {
		t.Fatalf("expected empty rawText for command-only message, got %q", rawText)
	}
}

// ---------------------------------------------------------------------------
// extractMediaFromReply
// ---------------------------------------------------------------------------

func TestExtractMediaFromReplyPhoto(t *testing.T) {
	t.Parallel()

	reply := &gotgbot.Message{
		Photo: []gotgbot.PhotoSize{
			{FileId: "small", Width: 100, Height: 100},
			{FileId: "large_photo", Width: 800, Height: 600},
		},
	}
	fileID, dt := extractMediaFromReply(reply)
	if fileID != "large_photo" {
		t.Fatalf("expected last photo file_id %q, got %q", "large_photo", fileID)
	}
	if dt != db.PHOTO {
		t.Fatalf("expected dataType=%d (PHOTO), got %d", db.PHOTO, dt)
	}
}

func TestExtractMediaFromReplyVideo(t *testing.T) {
	t.Parallel()

	reply := &gotgbot.Message{
		Video: &gotgbot.Video{FileId: "video_123"},
	}
	fileID, dt := extractMediaFromReply(reply)
	if fileID != "video_123" {
		t.Fatalf("expected file_id %q, got %q", "video_123", fileID)
	}
	if dt != db.VIDEO {
		t.Fatalf("expected dataType=%d (VIDEO), got %d", db.VIDEO, dt)
	}
}

func TestExtractMediaFromReplyAudio(t *testing.T) {
	t.Parallel()

	reply := &gotgbot.Message{
		Audio: &gotgbot.Audio{FileId: "audio_456"},
	}
	fileID, dt := extractMediaFromReply(reply)
	if fileID != "audio_456" {
		t.Fatalf("expected file_id %q, got %q", "audio_456", fileID)
	}
	if dt != db.AUDIO {
		t.Fatalf("expected dataType=%d (AUDIO), got %d", db.AUDIO, dt)
	}
}

func TestExtractMediaFromReplyVoice(t *testing.T) {
	t.Parallel()

	reply := &gotgbot.Message{
		Voice: &gotgbot.Voice{FileId: "voice_789"},
	}
	fileID, dt := extractMediaFromReply(reply)
	if fileID != "voice_789" {
		t.Fatalf("expected file_id %q, got %q", "voice_789", fileID)
	}
	if dt != db.VOICE {
		t.Fatalf("expected dataType=%d (VOICE), got %d", db.VOICE, dt)
	}
}

func TestExtractMediaFromReplyVideoNote(t *testing.T) {
	t.Parallel()

	reply := &gotgbot.Message{
		VideoNote: &gotgbot.VideoNote{FileId: "vn_abc"},
	}
	fileID, dt := extractMediaFromReply(reply)
	if fileID != "vn_abc" {
		t.Fatalf("expected file_id %q, got %q", "vn_abc", fileID)
	}
	if dt != db.VideoNote {
		t.Fatalf("expected dataType=%d (VideoNote), got %d", db.VideoNote, dt)
	}
}

func TestExtractMediaFromReplyDocument(t *testing.T) {
	t.Parallel()

	reply := &gotgbot.Message{
		Document: &gotgbot.Document{FileId: "doc_def"},
	}
	fileID, dt := extractMediaFromReply(reply)
	if fileID != "doc_def" {
		t.Fatalf("expected file_id %q, got %q", "doc_def", fileID)
	}
	if dt != db.DOCUMENT {
		t.Fatalf("expected dataType=%d (DOCUMENT), got %d", db.DOCUMENT, dt)
	}
}

func TestExtractMediaFromReplySticker(t *testing.T) {
	t.Parallel()

	reply := &gotgbot.Message{
		Sticker: &gotgbot.Sticker{FileId: "sticker_ghi"},
	}
	fileID, dt := extractMediaFromReply(reply)
	if fileID != "sticker_ghi" {
		t.Fatalf("expected file_id %q, got %q", "sticker_ghi", fileID)
	}
	if dt != db.STICKER {
		t.Fatalf("expected dataType=%d (STICKER), got %d", db.STICKER, dt)
	}
}

func TestExtractMediaFromReplyAnimation(t *testing.T) {
	t.Parallel()

	reply := &gotgbot.Message{
		Animation: &gotgbot.Animation{FileId: "anim_xyz"},
	}
	fileID, dt := extractMediaFromReply(reply)
	if fileID != "anim_xyz" {
		t.Fatalf("expected file_id %q, got %q", "anim_xyz", fileID)
	}
	if dt != db.DOCUMENT {
		t.Fatalf("expected dataType=%d (DOCUMENT), got %d", db.DOCUMENT, dt)
	}
}

func TestExtractMediaFromReplyTextOnly(t *testing.T) {
	t.Parallel()

	reply := &gotgbot.Message{
		Text: "Hello world",
	}
	fileID, dt := extractMediaFromReply(reply)
	if fileID != "" {
		t.Fatalf("expected empty file_id for text-only message, got %q", fileID)
	}
	if dt != -1 {
		t.Fatalf("expected dataType=-1 for text-only message, got %d", dt)
	}
}

func TestExtractMediaFromReplyNil(t *testing.T) {
	t.Parallel()

	fileID, dt := extractMediaFromReply(nil)
	if fileID != "" {
		t.Fatalf("expected empty file_id for nil message, got %q", fileID)
	}
	if dt != -1 {
		t.Fatalf("expected dataType=-1 for nil message, got %d", dt)
	}
}

func TestExtractMediaFromReplyPriorityStickerOverDocument(t *testing.T) {
	t.Parallel()

	reply := &gotgbot.Message{
		Sticker:  &gotgbot.Sticker{FileId: "sticker_prio"},
		Document: &gotgbot.Document{FileId: "doc_other"},
	}
	fileID, dt := extractMediaFromReply(reply)
	if fileID != "sticker_prio" {
		t.Fatalf("expected sticker to win over document, got %q", fileID)
	}
	if dt != db.STICKER {
		t.Fatalf("expected dataType=%d (STICKER), got %d", db.STICKER, dt)
	}
}
