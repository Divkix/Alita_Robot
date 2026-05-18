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

func TestErrNoPermissionNotNil(t *testing.T) {
	t.Parallel()

	if ErrNoPermission == nil {
		t.Fatal("ErrNoPermission should not be nil")
	}
	if !strings.Contains(ErrNoPermission.Error(), "permission") {
		t.Errorf("ErrNoPermission message %q does not contain \"permission\"", ErrNoPermission.Error())
	}
}

func TestContentZeroValue(t *testing.T) {
	t.Parallel()

	var c Content

	if c.Text != "" {
		t.Errorf("Text = %q, want zero value", c.Text)
	}
	if c.FileID != "" {
		t.Errorf("FileID = %q, want zero value", c.FileID)
	}
	if c.MsgType != 0 {
		t.Errorf("MsgType = %d, want 0", c.MsgType)
	}
	if c.Name != "" {
		t.Errorf("Name = %q, want zero value", c.Name)
	}
}

func TestOptionsZeroValue(t *testing.T) {
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
		{"CHAT_WRITE_FORBIDDEN", true},
		{"random error", false},
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
