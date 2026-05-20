package keyboard

import (
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db"
)

func TestBuildKeyboardGroupsSameLineButtons(t *testing.T) {
	t.Parallel()

	buttons := []db.Button{
		{Name: "one", Url: "https://one.example"},
		{Name: "two", Url: "https://two.example", SameLine: true},
		{Name: "three", Url: "https://three.example"},
	}

	got := BuildKeyboard(buttons)
	if len(got) != 2 {
		t.Fatalf("row count = %d, want 2", len(got))
	}
	if len(got[0]) != 2 {
		t.Fatalf("first row button count = %d, want 2", len(got[0]))
	}
	if got[0][0].Text != "one" || got[0][1].Text != "two" {
		t.Fatalf("first row texts = %#v, want one and two", got[0])
	}
	if got[1][0].Text != "three" {
		t.Fatalf("second row first text = %q, want three", got[1][0].Text)
	}
}

func TestChunkKeyboardSlices(t *testing.T) {
	t.Parallel()

	buttons := []gotgbot.InlineKeyboardButton{
		{Text: "one"},
		{Text: "two"},
		{Text: "three"},
		{Text: "four"},
		{Text: "five"},
	}

	got := ChunkKeyboardSlices(buttons, 2)
	if len(got) != 3 {
		t.Fatalf("chunk count = %d, want 3", len(got))
	}
	if len(got[0]) != 2 || len(got[1]) != 2 || len(got[2]) != 1 {
		t.Fatalf("chunk sizes = %d,%d,%d; want 2,2,1", len(got[0]), len(got[1]), len(got[2]))
	}
	if got[2][0].Text != "five" {
		t.Fatalf("last button text = %q, want five", got[2][0].Text)
	}
	if got := ChunkKeyboardSlices(buttons, 0); got != nil {
		t.Fatalf("chunkSize 0 = %#v, want nil", got)
	}
	if got := ChunkKeyboardSlices(buttons, -1); got != nil {
		t.Fatalf("negative chunkSize = %#v, want nil", got)
	}
}

func TestMakeLanguageKeyboardSkipsUnavailableLanguages(t *testing.T) {
	originalCodes := config.AppConfig.ValidLangCodes
	config.AppConfig.ValidLangCodes = []string{"bad-code"}
	defer func() { config.AppConfig.ValidLangCodes = originalCodes }()

	got := MakeLanguageKeyboard()
	if got != nil {
		t.Fatalf("keyboard for unavailable language = %#v, want nil", got)
	}
}
