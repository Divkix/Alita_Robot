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
