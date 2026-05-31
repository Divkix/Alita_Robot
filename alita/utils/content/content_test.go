package content

import (
	"strings"
	"testing"

	tgmd2html "github.com/PaulSonOfLars/gotg_md2html"
	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/divkix/Alita_Robot/alita/db"
)

// ---------------------------------------------------------------------------
// ExtractNoteAndFilter
// ---------------------------------------------------------------------------

func TestExtractNoteAndFilterNilMessage(t *testing.T) {
	t.Parallel()

	result := ExtractNoteAndFilter(nil, false, "en")
	if result.DataType != -1 {
		t.Fatalf("expected dataType=-1 for nil message, got %d", result.DataType)
	}
	if result.ErrorMsg == "" {
		t.Fatalf("expected errorMsg for nil message")
	}
}

func TestExtractNoteAndFilterDirectText(t *testing.T) {
	t.Parallel()

	msg := &gotgbot.Message{
		Text: "/save keyword hello world",
	}
	result := ExtractNoteAndFilter(msg, false, "en")
	if result.DataType != db.TEXT {
		t.Fatalf("expected dataType=%d for text message, got %d", db.TEXT, result.DataType)
	}
	if result.KeyWord != "keyword" {
		t.Fatalf("expected keyword %q, got %q", "keyword", result.KeyWord)
	}
	if result.Text != "hello world" {
		t.Fatalf("expected text %q, got %q", "hello world", result.Text)
	}
}

func TestExtractNoteAndFilterReplyMedia(t *testing.T) {
	t.Parallel()

	msg := &gotgbot.Message{
		Text: "/save keyword",
		ReplyToMessage: &gotgbot.Message{
			Video: &gotgbot.Video{FileId: "video_123"},
		},
	}
	result := ExtractNoteAndFilter(msg, false, "en")
	if result.DataType != db.VIDEO {
		t.Fatalf("expected dataType=%d for video reply, got %d", db.VIDEO, result.DataType)
	}
	if result.FileID != "video_123" {
		t.Fatalf("expected fileID %q, got %q", "video_123", result.FileID)
	}
	if result.KeyWord != "keyword" {
		t.Fatalf("expected keyword %q, got %q", "keyword", result.KeyWord)
	}
}

func TestExtractNoteAndFilterOptions(t *testing.T) {
	t.Parallel()

	msg := &gotgbot.Message{
		Text: "/save keyword hello {private} {admin} {preview}",
	}
	result := ExtractNoteAndFilter(msg, false, "en")
	if result.DataType != db.TEXT {
		t.Fatalf("expected dataType=%d, got %d", db.TEXT, result.DataType)
	}
	if !result.PvtOnly {
		t.Fatal("expected PvtOnly=true")
	}
	if !result.AdminOnly {
		t.Fatal("expected AdminOnly=true")
	}
	if !result.WebPreview {
		t.Fatal("expected WebPreview=true")
	}
	if !strings.Contains(result.Text, "{private}") {
		t.Fatal("expected {private} to be preserved in text")
	}
}

func TestExtractNoteAndFilterFilterMode(t *testing.T) {
	t.Parallel()

	msg := &gotgbot.Message{
		Text: "/filter keyword hello world",
	}
	result := ExtractNoteAndFilter(msg, true, "en")
	if result.DataType != db.TEXT {
		t.Fatalf("expected dataType=%d for filter, got %d", db.TEXT, result.DataType)
	}
	// Filters should not parse note options
	if result.PvtOnly {
		t.Fatal("expected PvtOnly=false for filter")
	}
	if result.AdminOnly {
		t.Fatal("expected AdminOnly=false for filter")
	}
}

// ---------------------------------------------------------------------------
// ExtractWelcome
// ---------------------------------------------------------------------------

func TestExtractWelcomeDirectText(t *testing.T) {
	t.Parallel()

	msg := &gotgbot.Message{
		Text: "/setwelcome hello {first}",
	}
	result := ExtractWelcome(msg, "welcome", "en")
	if result.DataType != db.TEXT {
		t.Fatalf("expected dataType=%d, got %d", db.TEXT, result.DataType)
	}
	if result.Text != "hello {first}" {
		t.Fatalf("expected text %q, got %q", "hello {first}", result.Text)
	}
}

func TestExtractWelcomeReplyMedia(t *testing.T) {
	t.Parallel()

	msg := &gotgbot.Message{
		Text: "/setwelcome",
		ReplyToMessage: &gotgbot.Message{
			Photo: []gotgbot.PhotoSize{
				{FileId: "small", Width: 100, Height: 100},
				{FileId: "large", Width: 800, Height: 600},
			},
		},
	}
	result := ExtractWelcome(msg, "welcome", "en")
	if result.DataType != db.PHOTO {
		t.Fatalf("expected dataType=%d for photo reply, got %d", db.PHOTO, result.DataType)
	}
	if result.FileID != "large" {
		t.Fatalf("expected fileID %q, got %q", "large", result.FileID)
	}
}

func TestExtractWelcomeNoContent(t *testing.T) {
	t.Parallel()

	msg := &gotgbot.Message{
		Text: "/setwelcome",
	}
	result := ExtractWelcome(msg, "welcome", "en")
	if result.DataType != -1 {
		t.Fatalf("expected dataType=-1 for no content, got %d", result.DataType)
	}
	if result.ErrorMsg == "" {
		t.Fatal("expected errorMsg for no content")
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
			name: "command only",
			msg:  &gotgbot.Message{Text: "/note"},
			want: "",
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
			name: "reply with args",
			msg: &gotgbot.Message{
				Text: "/note arg1 extra content here",
				ReplyToMessage: &gotgbot.Message{
					Text: "reply text",
				},
			},
			want: "extra content here",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var rawText string
			var args []string
			if tc.msg.Text != "" {
				args = strings.Fields(tc.msg.Text)[1:]
			}
			setRawText(tc.msg, args, &rawText)
			if rawText != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, rawText)
			}
		})
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
// NotesParser
// ---------------------------------------------------------------------------

func TestNotesParser(t *testing.T) {
	t.Parallel()

	sent := "Hello {private} {admin} {preview} {noprivate} {protect} {nonotif}"
	pvt, grp, admin, preview, protect, noNotif, clean := NotesParser(sent)

	if !pvt {
		t.Fatal("expected pvt=true")
	}
	if !grp {
		t.Fatal("expected grp=true")
	}
	if !admin {
		t.Fatal("expected admin=true")
	}
	if !preview {
		t.Fatal("expected preview=true")
	}
	if !protect {
		t.Fatal("expected protect=true")
	}
	if !noNotif {
		t.Fatal("expected noNotif=true")
	}
	// Each tag is replaced by an empty string, leaving spaces between them.
	if clean != "Hello      " {
		t.Fatalf("expected clean text %q, got %q", "Hello      ", clean)
	}
}

func TestNotesParserNone(t *testing.T) {
	t.Parallel()

	sent := "Hello world"
	pvt, grp, admin, preview, protect, noNotif, clean := NotesParser(sent)

	if pvt || grp || admin || preview || protect || noNotif {
		t.Fatal("expected all flags false")
	}
	if clean != "Hello world" {
		t.Fatalf("expected clean text unchanged, got %q", clean)
	}
}

// ---------------------------------------------------------------------------
	// ConvertButtonV2ToDbButton
	// ---------------------------------------------------------------------------

func TestConvertButtonV2ToDbButton(t *testing.T) {
	t.Parallel()

	buttons := []tgmd2html.ButtonV2{
		{Name: "Btn1", Content: "https://example.com", SameLine: false},
		{Name: "Btn2", Content: "https://test.com", SameLine: true},
	}

	dbButtons := ConvertButtonV2ToDbButton(buttons)
	if len(dbButtons) != 2 {
		t.Fatalf("expected 2 buttons, got %d", len(dbButtons))
	}
	if dbButtons[0].Name != "Btn1" || dbButtons[0].Url != "https://example.com" || dbButtons[0].SameLine != false {
		t.Fatal("first button mismatch")
	}
	if dbButtons[1].Name != "Btn2" || dbButtons[1].Url != "https://test.com" || dbButtons[1].SameLine != true {
		t.Fatal("second button mismatch")
	}
}

// ---------------------------------------------------------------------------
// RevertButtons
// ---------------------------------------------------------------------------

func TestRevertButtons(t *testing.T) {
	t.Parallel()

	buttons := []db.Button{
		{Name: "Btn1", Url: "https://example.com", SameLine: false},
		{Name: "Btn2", Url: "https://test.com", SameLine: true},
	}

	result := RevertButtons(buttons)
	expected := "\n[Btn1](buttonurl://https://example.com)\n[Btn2](buttonurl://https://test.com:same)"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestRevertButtonsEmpty(t *testing.T) {
	t.Parallel()

	result := RevertButtons(nil)
	if result != "" {
		t.Fatalf("expected empty string, got %q", result)
	}
}

// ---------------------------------------------------------------------------
// InlineKeyboardToDbButtons
// ---------------------------------------------------------------------------

func TestInlineKeyboardToDbButtons(t *testing.T) {
	t.Parallel()

	replyMarkup := &gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{Text: "Google", Url: "https://google.com"},
				{Text: "Invalid", Url: ""},
				{Text: "Bad", Url: "not-a-url"},
			},
			{
				{Text: "Single", Url: "https://single.com"},
			},
		},
	}

	buttons := InlineKeyboardToDbButtons(replyMarkup)
	// Invalid URL is filtered by empty check; bad URL is filtered by URL validation.
	if len(buttons) != 2 {
		t.Fatalf("expected 2 valid buttons (filtered invalid), got %d", len(buttons))
	}
	if buttons[0].Name != "Google" || buttons[0].SameLine != false {
		t.Fatal("first button mismatch")
	}
	if buttons[1].Name != "Single" || buttons[1].SameLine != false {
		t.Fatal("second button mismatch")
	}
}

func TestInlineKeyboardToDbButtonsNil(t *testing.T) {
	t.Parallel()

	buttons := InlineKeyboardToDbButtons(nil)
	if buttons != nil {
		t.Fatalf("expected nil, got %v", buttons)
	}
}

func TestInlineKeyboardToDbButtonsSameLine(t *testing.T) {
	t.Parallel()

	replyMarkup := &gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{Text: "First", Url: "https://first.com"},
				{Text: "Second", Url: "https://second.com"},
			},
		},
	}

	buttons := InlineKeyboardToDbButtons(replyMarkup)
	if len(buttons) != 2 {
		t.Fatalf("expected 2 buttons, got %d", len(buttons))
	}
	if buttons[0].SameLine != false {
		t.Fatal("expected first button SameLine=false")
	}
	if buttons[1].SameLine != true {
		t.Fatal("expected second button SameLine=true")
	}
}

// ---------------------------------------------------------------------------
// inlineKeyboardToButtonV2
// ---------------------------------------------------------------------------

func TestInlineKeyboardToButtonV2(t *testing.T) {
	t.Parallel()

	replyMarkup := &gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{Text: "Google", Url: "https://google.com"},
				{Text: "Callback", CallbackData: "cb"},
			},
			{
				{Text: "Single", Url: "https://single.com"},
			},
		},
	}

	buttons := inlineKeyboardToButtonV2(replyMarkup)
	// Callback button has no URL, so it is filtered out.
	if len(buttons) != 2 {
		t.Fatalf("expected 2 buttons (callback filtered), got %d", len(buttons))
	}
	if buttons[0].Name != "Google" || buttons[0].SameLine != false {
		t.Fatal("first button mismatch")
	}
	if buttons[1].Name != "Single" || buttons[1].SameLine != false {
		t.Fatal("second button mismatch")
	}
}
