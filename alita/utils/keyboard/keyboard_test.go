package keyboard

import (
	"reflect"
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

	tests := []struct {
		name       string
		chunkSize  int
		wantCounts []int
		wantNil    bool
	}{
		{name: "chunks buttons by valid size", chunkSize: 2, wantCounts: []int{2, 2, 1}},
		{name: "zero chunk size returns nil", chunkSize: 0, wantNil: true},
		{name: "negative chunk size returns nil", chunkSize: -1, wantNil: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := ChunkKeyboardSlices(buttons, tc.chunkSize)
			if tc.wantNil {
				if got != nil {
					t.Fatalf("ChunkKeyboardSlices() = %#v, want nil", got)
				}
				return
			}

			var counts []int
			for _, row := range got {
				counts = append(counts, len(row))
			}
			if !reflect.DeepEqual(counts, tc.wantCounts) {
				t.Fatalf("chunk sizes = %v, want %v", counts, tc.wantCounts)
			}
			if got[len(got)-1][0].Text != "five" {
				t.Fatalf("last button text = %q, want five", got[len(got)-1][0].Text)
			}
		})
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
