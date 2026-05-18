package modules

import (
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/divkix/Alita_Robot/alita/db"
)

func TestGetPinType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		msg          *gotgbot.Message
		wantFileID   string
		wantText     string
		wantDataType int
		wantButtons  int
	}{
		{
			name: "text message with args no reply",
			msg: &gotgbot.Message{
				Text: "/permapin Hello world",
			},
			wantFileID:   "",
			wantText:     "Hello world",
			wantDataType: db.TEXT,
			wantButtons:  0,
		},
		{
			name: "reply to text message",
			msg: &gotgbot.Message{
				Text: "/permapin",
				ReplyToMessage: &gotgbot.Message{
					Text: "reply text",
				},
			},
			wantFileID:   "",
			wantText:     "reply text",
			wantDataType: db.TEXT,
			wantButtons:  0,
		},
		{
			name: "reply to sticker",
			msg: &gotgbot.Message{
				Text: "/permapin",
				ReplyToMessage: &gotgbot.Message{
					Sticker: &gotgbot.Sticker{FileId: "sticker_file_id"},
				},
			},
			wantFileID:   "sticker_file_id",
			wantText:     "",
			wantDataType: db.STICKER,
			wantButtons:  0,
		},
		{
			name: "reply to photo",
			msg: &gotgbot.Message{
				Text: "/permapin",
				ReplyToMessage: &gotgbot.Message{
					Photo: []gotgbot.PhotoSize{
						{FileId: "photo_low"},
						{FileId: "photo_high"},
					},
				},
			},
			wantFileID:   "photo_high",
			wantText:     "",
			wantDataType: db.PHOTO,
			wantButtons:  0,
		},
		{
			name: "reply to video",
			msg: &gotgbot.Message{
				Text: "/permapin",
				ReplyToMessage: &gotgbot.Message{
					Video: &gotgbot.Video{FileId: "video_file_id"},
				},
			},
			wantFileID:   "video_file_id",
			wantText:     "",
			wantDataType: db.VIDEO,
			wantButtons:  0,
		},
		{
			name: "reply to audio",
			msg: &gotgbot.Message{
				Text: "/permapin",
				ReplyToMessage: &gotgbot.Message{
					Audio: &gotgbot.Audio{FileId: "audio_file_id"},
				},
			},
			wantFileID:   "audio_file_id",
			wantText:     "",
			wantDataType: db.AUDIO,
			wantButtons:  0,
		},
		{
			name: "reply to voice",
			msg: &gotgbot.Message{
				Text: "/permapin",
				ReplyToMessage: &gotgbot.Message{
					Voice: &gotgbot.Voice{FileId: "voice_file_id"},
				},
			},
			wantFileID:   "voice_file_id",
			wantText:     "",
			wantDataType: db.VOICE,
			wantButtons:  0,
		},
		{
			name: "reply to video note",
			msg: &gotgbot.Message{
				Text: "/permapin",
				ReplyToMessage: &gotgbot.Message{
					VideoNote: &gotgbot.VideoNote{FileId: "videonote_file_id"},
				},
			},
			wantFileID:   "videonote_file_id",
			wantText:     "",
			wantDataType: db.VideoNote,
			wantButtons:  0,
		},
		{
			name: "reply to document",
			msg: &gotgbot.Message{
				Text: "/permapin",
				ReplyToMessage: &gotgbot.Message{
					Document: &gotgbot.Document{FileId: "doc_file_id"},
				},
			},
			wantFileID:   "doc_file_id",
			wantText:     "",
			wantDataType: db.DOCUMENT,
			wantButtons:  0,
		},
		{
			name: "reply to unsupported returns -1",
			msg: &gotgbot.Message{
				Text: "/permapin",
				ReplyToMessage: &gotgbot.Message{
					Game: &gotgbot.Game{Title: "Game"},
				},
			},
			wantFileID:   "",
			wantText:     "",
			wantDataType: -1,
			wantButtons:  0,
		},
		{
			name: "no reply no args returns -1",
			msg: &gotgbot.Message{
				Text: "/permapin",
			},
			wantFileID:   "",
			wantText:     "",
			wantDataType: -1,
			wantButtons:  0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var m moduleStruct
			fileID, text, dataType, buttons := m.GetPinType(tc.msg)
			if fileID != tc.wantFileID {
				t.Errorf("fileID = %q, want %q", fileID, tc.wantFileID)
			}
			if text != tc.wantText {
				t.Errorf("text = %q, want %q", text, tc.wantText)
			}
			if dataType != tc.wantDataType {
				t.Errorf("dataType = %d, want %d", dataType, tc.wantDataType)
			}
			if len(buttons) != tc.wantButtons {
				t.Errorf("buttons len = %d, want %d", len(buttons), tc.wantButtons)
			}
		})
	}
}
