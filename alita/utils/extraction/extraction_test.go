package extraction

import (
	"errors"
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
		{
			name:         "no quotes but matchQuotes=true returns empty",
			sentence:     "hello world",
			matchQuotes:  true,
			matchWord:    false,
			wantInQuotes: "",
			wantAfter:    "",
		},
		{
			name:         "matchWord=true with numbers only",
			sentence:     "12345 rest",
			matchQuotes:  false,
			matchWord:    true,
			wantInQuotes: "12345",
			wantAfter:    "rest",
		},
		{
			name:         "matchWord=true with special chars only",
			sentence:     "+=-_{}[]( ) rest",
			matchQuotes:  false,
			matchWord:    true,
			wantInQuotes: "+=-_{}[](",
			wantAfter:    ") rest",
		},
		{
			// The word regex does not include '@', so it stops at the @ character.
			name:         "matchWord=true email-like splits at at-sign",
			sentence:     "test@example.com",
			matchQuotes:  false,
			matchWord:    true,
			wantInQuotes: "test",
			wantAfter:    "@example.com",
		},
		{
			name:         "matchQuotes=true starting with space then quote",
			sentence:     ` "hello" after`,
			matchQuotes:  true,
			matchWord:    false,
			wantInQuotes: "",
			wantAfter:    "",
		},
		{
			name:         "empty string matchQuotes only does not panic",
			sentence:     "",
			matchQuotes:  true,
			matchWord:    false,
			wantInQuotes: "",
			wantAfter:    "",
		},
		{
			name:         "empty string matchWord only does not panic",
			sentence:     "",
			matchQuotes:  false,
			matchWord:    true,
			wantInQuotes: "",
			wantAfter:    "",
		},
		{
			name:         "leading whitespace with matchWord strips spaces",
			sentence:     "   word rest",
			matchQuotes:  false,
			matchWord:    true,
			wantInQuotes: "word",
			wantAfter:    "rest",
		},
		{
			name:         "nested quotes extracted as-is",
			sentence:     `"say 'hello'" after`,
			matchQuotes:  true,
			matchWord:    false,
			wantInQuotes: "say 'hello'",
			wantAfter:    "after",
		},
		{
			// The regex \s? only consumes one trailing space; multiple trailing spaces leave
			// the remainder in afterWord.
			name:         "quote with no after and multiple trailing spaces",
			sentence:     `"onlythis"   `,
			matchQuotes:  true,
			matchWord:    false,
			wantInQuotes: "onlythis",
			wantAfter:    "  ",
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
			// The regex \s? only consumes one trailing space; with exactly one trailing
			// space the afterWord captures empty string.
			name:         "quoted with exactly one trailing space yields empty afterWord",
			sentence:     "\"hello\" ",
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

func TestIdFromReply_NilSender(t *testing.T) {
	t.Parallel()

	// gotgbot Message.GetSender() never returns nil; it always returns &Sender{}.
	// When both From and SenderChat are nil, Sender.Id() returns 0.
	// The function still extracts text from the parent message.
	msg := &gotgbot.Message{
		Text: "/cmd reason",
		ReplyToMessage: &gotgbot.Message{
			From:       nil,
			SenderChat: nil,
		},
	}
	gotId, gotText := IdFromReply(msg)
	if gotId != 0 {
		t.Errorf("expected userId=0 for nil sender, got %d", gotId)
	}
	if gotText != "reason" {
		t.Errorf("expected text 'reason' for nil sender, got %q", gotText)
	}
}

func TestIdFromReply_TextSplitVariations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		text       string
		wantUserId int64
		wantText   string
	}{
		{
			name:       "text with single space split",
			text:       "/cmd reason",
			wantUserId: 42,
			wantText:   "reason",
		},
		{
			// strings.SplitN splits on the literal " " character; tabs are not treated as delimiters.
			// The string contains a space between "reason" and "here", so it splits there.
			name:       "text with tab character split",
			text:       "/cmd\treason here",
			wantUserId: 42,
			wantText:   "here",
		},
		{
			name:       "text with multiple consecutive spaces",
			text:       "/cmd  reason  here",
			wantUserId: 42,
			wantText:   " reason  here",
		},
		{
			// strings.SplitN(" cmd reason", " ", 2) => ["", "cmd reason"]; len(res) >= 2 so res[1] == "cmd reason".
			name:       "text starting with space no command prefix",
			text:       " cmd reason",
			wantUserId: 42,
			wantText:   "cmd reason",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			msg := &gotgbot.Message{
				Text: tc.text,
				ReplyToMessage: &gotgbot.Message{
					From: &gotgbot.User{Id: tc.wantUserId},
				},
			}
			gotId, gotText := IdFromReply(msg)
			if gotId != tc.wantUserId {
				t.Errorf("userId: got %d, want %d", gotId, tc.wantUserId)
			}
			if gotText != tc.wantText {
				t.Errorf("text: got %q, want %q", gotText, tc.wantText)
			}
		})
	}
}

//nolint:dupl // Test functions intentionally similar for clarity
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

func TestParseTemporaryDuration(t *testing.T) {
	t.Parallel()

	const now int64 = 1_700_000_000

	tests := []struct {
		name        string
		input       string
		wantBanTime int64
		wantTimeStr string
		wantReason  string
		wantErr     error
	}{
		{
			name:        "minutes with reason",
			input:       "15m repeated spam",
			wantBanTime: now + 15*60,
			wantTimeStr: "15 minutes",
			wantReason:  "repeated spam",
		},
		{
			name:        "hours without reason",
			input:       "2h",
			wantBanTime: now + 2*60*60,
			wantTimeStr: "2 hours",
		},
		{
			name:        "days joins multi word reason",
			input:       "3d raid cleanup needed",
			wantBanTime: now + 3*24*60*60,
			wantTimeStr: "3 days",
			wantReason:  "raid cleanup needed",
		},
		{
			name:        "weeks",
			input:       "4w long cooldown",
			wantBanTime: now + 4*7*24*60*60,
			wantTimeStr: "4 weeks",
			wantReason:  "long cooldown",
		},
		{
			name:    "empty input",
			input:   " \t\n ",
			wantErr: errNoTimeSpecified,
		},
		{
			name:    "invalid amount",
			input:   "xw bad amount",
			wantErr: errInvalidTimeAmount,
		},
		{
			name:    "invalid type",
			input:   "10y bad unit",
			wantErr: errInvalidTimeType,
		},
		{
			name:    "one year exactly exceeds limit",
			input:   "365d too long",
			wantErr: errTimeLimitExceeded,
		},
		{
			name:        "negative amount preserves existing behavior",
			input:       "-1h already elapsed",
			wantBanTime: now - 60*60,
			wantTimeStr: "-1 hours",
			wantReason:  "already elapsed",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotBanTime, gotTimeStr, gotReason, gotErr := parseTemporaryDuration(tc.input, now)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("parseTemporaryDuration() err = %v, want %v", gotErr, tc.wantErr)
			}
			if tc.wantErr != nil {
				return
			}
			if gotBanTime != tc.wantBanTime {
				t.Fatalf("banTime = %d, want %d", gotBanTime, tc.wantBanTime)
			}
			if gotTimeStr != tc.wantTimeStr {
				t.Fatalf("timeStr = %q, want %q", gotTimeStr, tc.wantTimeStr)
			}
			if gotReason != tc.wantReason {
				t.Fatalf("reason = %q, want %q", gotReason, tc.wantReason)
			}
		})
	}
}
