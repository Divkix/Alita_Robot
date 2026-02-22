package extraction

import (
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

func TestExtractQuotes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		sentence     string
		matchQuotes  bool
		matchWord    bool
		wantInQuotes string
		wantAfter    string
	}{
		{
			name:         "quoted text extraction",
			sentence:     `"hello world" remaining`,
			matchQuotes:  true,
			matchWord:    false,
			wantInQuotes: "hello world",
			wantAfter:    "remaining",
		},
		{
			name:         "word extraction from sentence",
			sentence:     "firstword rest of text",
			matchQuotes:  false,
			matchWord:    true,
			wantInQuotes: "firstword",
			wantAfter:    "rest of text",
		},
		{
			name:         "empty string both flags true",
			sentence:     "",
			matchQuotes:  true,
			matchWord:    true,
			wantInQuotes: "",
			wantAfter:    "",
		},
		{
			name:         "unmatched opening quote returns empty",
			sentence:     `"hello`,
			matchQuotes:  true,
			matchWord:    false,
			wantInQuotes: "",
			wantAfter:    "",
		},
		{
			name:         "both flags false returns empty",
			sentence:     "some text here",
			matchQuotes:  false,
			matchWord:    false,
			wantInQuotes: "",
			wantAfter:    "",
		},
		{
			name:         "special characters in quotes preserved",
			sentence:     `"hello & <world>" rest`,
			matchQuotes:  true,
			matchWord:    false,
			wantInQuotes: "hello & <world>",
			wantAfter:    "rest",
		},
		{
			name:         "multiline content in quotes via (?s) flag",
			sentence:     "\"hello\nworld\" after",
			matchQuotes:  true,
			matchWord:    false,
			wantInQuotes: "hello\nworld",
			wantAfter:    "after",
		},
		{
			name:         "word with special chars hyphen underscore digits",
			sentence:     "hello-world_123 rest",
			matchQuotes:  false,
			matchWord:    true,
			wantInQuotes: "hello-world_123",
			wantAfter:    "rest",
		},
		{
			name:         "single word no remainder",
			sentence:     "onlyword",
			matchQuotes:  false,
			matchWord:    true,
			wantInQuotes: "onlyword",
			wantAfter:    "",
		},
		{
			name:         "quoted text no remainder",
			sentence:     `"onlythis"`,
			matchQuotes:  true,
			matchWord:    false,
			wantInQuotes: "onlythis",
			wantAfter:    "",
		},
		{
			name:         "matchQuotes takes priority over matchWord when quoted",
			sentence:     `"quoted content" after`,
			matchQuotes:  true,
			matchWord:    true,
			wantInQuotes: "quoted content",
			wantAfter:    "after",
		},
		{
			name:         "matchWord false with unquoted content returns empty",
			sentence:     "some word here",
			matchQuotes:  true,
			matchWord:    false,
			wantInQuotes: "",
			wantAfter:    "",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotIn, gotAfter := ExtractQuotes(tc.sentence, tc.matchQuotes, tc.matchWord)
			if gotIn != tc.wantInQuotes {
				t.Errorf("inQuotes: got %q, want %q", gotIn, tc.wantInQuotes)
			}
			if gotAfter != tc.wantAfter {
				t.Errorf("afterWord: got %q, want %q", gotAfter, tc.wantAfter)
			}
		})
	}
}

func TestIdFromReply(t *testing.T) {
	t.Parallel()

	t.Run("nil ReplyToMessage returns zero and empty string", func(t *testing.T) {
		t.Parallel()
		msg := &gotgbot.Message{
			Text:           "/cmd reason text",
			ReplyToMessage: nil,
		}
		gotId, gotText := IdFromReply(msg)
		if gotId != 0 {
			t.Errorf("userId: got %d, want 0", gotId)
		}
		if gotText != "" {
			t.Errorf("text: got %q, want \"\"", gotText)
		}
	})

	t.Run("valid reply with From set returns sender ID and text", func(t *testing.T) {
		t.Parallel()
		msg := &gotgbot.Message{
			Text: "/cmd reason text",
			ReplyToMessage: &gotgbot.Message{
				From: &gotgbot.User{Id: 42},
			},
		}
		gotId, gotText := IdFromReply(msg)
		if gotId != 42 {
			t.Errorf("userId: got %d, want 42", gotId)
		}
		if gotText != "reason text" {
			t.Errorf("text: got %q, want \"reason text\"", gotText)
		}
	})

	t.Run("reply with no spaces in command text returns sender ID with empty text", func(t *testing.T) {
		t.Parallel()
		msg := &gotgbot.Message{
			Text: "/cmd",
			ReplyToMessage: &gotgbot.Message{
				From: &gotgbot.User{Id: 99},
			},
		}
		gotId, gotText := IdFromReply(msg)
		if gotId != 99 {
			t.Errorf("userId: got %d, want 99", gotId)
		}
		if gotText != "" {
			t.Errorf("text: got %q, want \"\"", gotText)
		}
	})

	t.Run("reply from SenderChat channel returns channel ID", func(t *testing.T) {
		t.Parallel()
		const channelId int64 = -1001234567890
		msg := &gotgbot.Message{
			Text: "/ban spam",
			ReplyToMessage: &gotgbot.Message{
				SenderChat: &gotgbot.Chat{Id: channelId},
			},
		}
		gotId, gotText := IdFromReply(msg)
		if gotId != channelId {
			t.Errorf("userId: got %d, want %d", gotId, channelId)
		}
		if gotText != "spam" {
			t.Errorf("text: got %q, want \"spam\"", gotText)
		}
	})

	t.Run("reply with multiple spaces preserves full remaining text", func(t *testing.T) {
		t.Parallel()
		msg := &gotgbot.Message{
			Text: "/kick this is the reason",
			ReplyToMessage: &gotgbot.Message{
				From: &gotgbot.User{Id: 77},
			},
		}
		gotId, gotText := IdFromReply(msg)
		if gotId != 77 {
			t.Errorf("userId: got %d, want 77", gotId)
		}
		// SplitN with n=2 means second part is everything after first space
		if gotText != "this is the reason" {
			t.Errorf("text: got %q, want \"this is the reason\"", gotText)
		}
	})

	t.Run("empty text with valid reply returns sender ID and empty text", func(t *testing.T) {
		t.Parallel()
		msg := &gotgbot.Message{
			Text: "",
			ReplyToMessage: &gotgbot.Message{
				From: &gotgbot.User{Id: 55},
			},
		}
		gotId, gotText := IdFromReply(msg)
		if gotId != 55 {
			t.Errorf("userId: got %d, want 55", gotId)
		}
		if gotText != "" {
			t.Errorf("text: got %q, want \"\"", gotText)
		}
	})
}

//nolint:dupl // Intentionally similar table-driven test pattern for different quote scenarios
func TestExtractQuotes_AdditionalEdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		sentence     string
		matchQuotes  bool
		matchWord    bool
		wantInQuotes string
		wantAfter    string
	}{
		{
			name:         "single word with matchWord=true extracts word with no after",
			sentence:     "hello",
			matchQuotes:  false,
			matchWord:    true,
			wantInQuotes: "hello",
			wantAfter:    "",
		},
		{
			name:         "quoted with trailing spaces trimmed",
			sentence:     `"hello"   `,
			matchQuotes:  true,
			matchWord:    false,
			wantInQuotes: "hello",
			wantAfter:    "",
		},
		{
			name:         "multiple quotes only first extracted",
			sentence:     `"first" "second"`,
			matchQuotes:  true,
			matchWord:    false,
			wantInQuotes: "first",
			wantAfter:    `"second"`,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotIn, gotAfter := ExtractQuotes(tc.sentence, tc.matchQuotes, tc.matchWord)
			if gotIn != tc.wantInQuotes {
				t.Errorf("inQuotes: got %q, want %q", gotIn, tc.wantInQuotes)
			}
			if gotAfter != tc.wantAfter {
				t.Errorf("afterWord: got %q, want %q", gotAfter, tc.wantAfter)
			}
		})
	}
}

func TestIdFromReply_NilReply(t *testing.T) {
	t.Parallel()

	msg := &gotgbot.Message{
		ReplyToMessage: nil,
	}
	// Should not panic, should return zero values
	gotId, gotText := IdFromReply(msg)
	if gotId != 0 {
		t.Errorf("expected userId=0 for nil reply, got %d", gotId)
	}
	if gotText != "" {
		t.Errorf("expected empty text for nil reply, got %q", gotText)
	}
}

//nolint:dupl // Intentionally similar table-driven test pattern for different quote scenarios
func TestExtractQuotes_UnicodeContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		sentence     string
		matchQuotes  bool
		matchWord    bool
		wantInQuotes string
		wantAfter    string
	}{
		{
			name:         "unicode word extracted correctly",
			sentence:     `"café" remaining`,
			matchQuotes:  true,
			matchWord:    false,
			wantInQuotes: "café",
			wantAfter:    "remaining",
		},
		{
			name:         "unicode content in quotes fully extracted",
			sentence:     `"日本語テスト" after`,
			matchQuotes:  true,
			matchWord:    false,
			wantInQuotes: "日本語テスト",
			wantAfter:    "after",
		},
		{
			name:         "emoji in quotes extracted correctly",
			sentence:     `"hello 🌍" world`,
			matchQuotes:  true,
			matchWord:    false,
			wantInQuotes: "hello 🌍",
			wantAfter:    "world",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotIn, gotAfter := ExtractQuotes(tc.sentence, tc.matchQuotes, tc.matchWord)
			if gotIn != tc.wantInQuotes {
				t.Errorf("inQuotes: got %q, want %q", gotIn, tc.wantInQuotes)
			}
			if gotAfter != tc.wantAfter {
				t.Errorf("afterWord: got %q, want %q", gotAfter, tc.wantAfter)
			}
		})
	}
}
