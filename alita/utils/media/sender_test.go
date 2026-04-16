package media

import (
	"strings"
	"testing"

	"github.com/divkix/Alita_Robot/alita/db"
)
func TestTypeConstantsMatchDB(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		got      int
		expected int
	}{
		{"TypeText", TypeText, db.TEXT},
		{"TypeSticker", TypeSticker, db.STICKER},
		{"TypeDocument", TypeDocument, db.DOCUMENT},
		{"TypePhoto", TypePhoto, db.PHOTO},
		{"TypeAudio", TypeAudio, db.AUDIO},
		{"TypeVoice", TypeVoice, db.VOICE},
		{"TypeVideo", TypeVideo, db.VIDEO},
		{"TypeVideoNote", TypeVideoNote, db.VideoNote},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.got != tc.expected {
				t.Errorf("%s = %d, want %d", tc.name, tc.got, tc.expected)
			}
		})
	}
}

func TestParseModeConstants(t *testing.T) {
	t.Parallel()

	if HTML != "HTML" {
		t.Errorf("HTML = %q, want \"HTML\"", HTML)
	}
	if None != "" {
		t.Errorf("None = %q, want \"\"", None)
	}
}

func TestErrNoPermissionNotNil(t *testing.T) {
	t.Parallel()

	if ErrNoPermission == nil {
		t.Fatal("ErrNoPermission should not be nil")
	}
	if !strings.Contains(ErrNoPermission.Error(), "permission") {
		t.Errorf("ErrNoPermission message %q does not contain \"permission\"", ErrNoPermission.Error())
	}
}

func TestContentStruct(t *testing.T) {
	t.Parallel()

	c := Content{
		Text:    "hello world",
		FileID:  "file-abc-123",
		MsgType: TypePhoto,
		Name:    "test-note",
	}

	if c.Text != "hello world" {
		t.Errorf("Text = %q, want \"hello world\"", c.Text)
	}
	if c.FileID != "file-abc-123" {
		t.Errorf("FileID = %q, want \"file-abc-123\"", c.FileID)
	}
	if c.MsgType != TypePhoto {
		t.Errorf("MsgType = %d, want %d", c.MsgType, TypePhoto)
	}
	if c.Name != "test-note" {
		t.Errorf("Name = %q, want \"test-note\"", c.Name)
	}
}

func TestOptionsDefaults(t *testing.T) {
	t.Parallel()

	var opts Options

	if opts.ChatID != 0 {
		t.Errorf("ChatID = %d, want 0", opts.ChatID)
	}
	if opts.ReplyMsgID != 0 {
		t.Errorf("ReplyMsgID = %d, want 0", opts.ReplyMsgID)
	}
	if opts.ThreadID != 0 {
		t.Errorf("ThreadID = %d, want 0", opts.ThreadID)
	}
	if opts.Keyboard != nil {
		t.Error("Keyboard should be nil by default")
	}
	if opts.NoFormat != false {
		t.Error("NoFormat should be false by default")
	}
	if opts.NoNotif != false {
		t.Error("NoNotif should be false by default")
	}
	if opts.WebPreview != false {
		t.Error("WebPreview should be false by default")
	}
	if opts.IsProtected != false {
		t.Error("IsProtected should be false by default")
	}
	if opts.AllowWithoutReply != false {
		t.Error("AllowWithoutReply should be false by default")
	}
}

func TestIsPermissionError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		errStr   string
		expected bool
	}{
		{"not enough rights to send text messages", true},
		{"have no rights to send a message", true},
		{"Bad Request: CHAT_WRITE_FORBIDDEN", true},
		{"Forbidden: CHAT_RESTRICTED", true},
		{"need administrator rights in the channel chat", true},
		{"some other error", false},
		{"Bad Request: message is not modified", false},
		{"", false},
	}

	for _, tc := range cases {
		t.Run(tc.errStr, func(t *testing.T) {
			t.Parallel()
			got := isPermissionError(tc.errStr)
			if got != tc.expected {
				t.Errorf("isPermissionError(%q) = %v, want %v", tc.errStr, got, tc.expected)
			}
		})
	}
}
