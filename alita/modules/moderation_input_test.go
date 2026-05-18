package modules

import (
	"reflect"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

func TestBuildModerationMatchTextNilMessage(t *testing.T) {
	t.Parallel()

	if got := buildModerationMatchText(nil); got != "" {
		t.Fatalf("expected empty output, got %q", got)
	}
}

func TestBuildModerationMatchTextIncludesTextCaptionAndEntityURLs(t *testing.T) {
	t.Parallel()

	msg := &gotgbot.Message{
		Text:    "visit https://a.example now",
		Caption: "caption text",
		Entities: []gotgbot.MessageEntity{
			{Type: "url", Offset: 6, Length: 17}, // https://a.example
		},
		CaptionEntities: []gotgbot.MessageEntity{
			{Type: "text_link", Url: "https://b.example"},
			{Type: "text_link", Url: "https://b.example"}, // duplicate should dedupe
		},
	}

	got := buildModerationMatchText(msg)
	lines := strings.Split(got, "\n")
	want := []string{
		"visit https://a.example now",
		"caption text",
		"https://a.example",
		"https://b.example",
	}

	if !reflect.DeepEqual(lines, want) {
		t.Fatalf("unexpected moderation text lines:\nwant=%v\ngot=%v", want, lines)
	}
}

func TestExtractEntityURLs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		source   string
		entities []gotgbot.MessageEntity
		want     []string
	}{
		{
			name:     "nil entities",
			source:   "hello",
			entities: nil,
			want:     nil,
		},
		{
			name:   "empty entities",
			source: "hello",
			entities: []gotgbot.MessageEntity{},
			want:     nil,
		},
		{
			name:   "url entity from text",
			source: "visit https://example.com now",
			entities: []gotgbot.MessageEntity{
				{Type: "url", Offset: 6, Length: 19},
			},
			want: []string{"https://example.com"},
		},
		{
			name:   "text_link with explicit url",
			source: "click here",
			entities: []gotgbot.MessageEntity{
				{Type: "text_link", Offset: 0, Length: 4, Url: "https://hidden.example"},
			},
			want: []string{"https://hidden.example"},
		},
		{
			name:   "mixed url and text_link",
			source: "see https://a.example and click",
			entities: []gotgbot.MessageEntity{
				{Type: "url", Offset: 4, Length: 17},
				{Type: "text_link", Offset: 24, Length: 5, Url: "https://b.example"},
			},
			want: []string{"https://a.example", "https://b.example"},
		},
		{
			name:   "non-url entity skipped",
			source: "hello world",
			entities: []gotgbot.MessageEntity{
				{Type: "bold", Offset: 0, Length: 5},
			},
			want: []string{},
		},
		{
			name:   "url entity with empty extracted text skipped",
			source: "",
			entities: []gotgbot.MessageEntity{
				{Type: "url", Offset: 0, Length: 5},
			},
			want: []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := extractEntityURLs(tc.source, tc.entities)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("extractEntityURLs(%q, %+v) = %v, want %v", tc.source, tc.entities, got, tc.want)
			}
		})
	}
}

func TestExtractEntityText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source string
		offset int64
		length int64
		want   string
	}{
		{
			name:   "empty source",
			source: "",
			offset: 0,
			length: 5,
			want:   "",
		},
		{
			name:   "negative offset",
			source: "hello",
			offset: -1,
			length: 3,
			want:   "",
		},
		{
			name:   "zero length",
			source: "hello",
			offset: 0,
			length: 0,
			want:   "",
		},
		{
			name:   "offset past end",
			source: "hello",
			offset: 10,
			length: 2,
			want:   "",
		},
		{
			name:   "end past rune length",
			source: "hello",
			offset: 2,
			length: 10,
			want:   "",
		},
		{
			name:   "start equals end",
			source: "hello",
			offset: 2,
			length: 0,
			want:   "",
		},
		{
			name:   "start greater than end (defensive)",
			source: "hello",
			offset: 3,
			length: -1,
			want:   "",
		},
		{
			name:   "valid extraction",
			source: "hello world",
			offset: 0,
			length: 5,
			want:   "hello",
		},
		{
			name:   "valid extraction mid string",
			source: "hello world",
			offset: 6,
			length: 5,
			want:   "world",
		},
		{
			name:   "unicode runes",
			source: "привет мир",
			offset: 0,
			length: 6,
			want:   "привет",
		},
		{
			name:   "unicode extraction mid",
			source: "привет мир",
			offset: 7,
			length: 3,
			want:   "мир",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := extractEntityText(tc.source, tc.offset, tc.length)
			if got != tc.want {
				t.Fatalf("extractEntityText(%q, %d, %d) = %q, want %q", tc.source, tc.offset, tc.length, got, tc.want)
			}
		})
	}
}
