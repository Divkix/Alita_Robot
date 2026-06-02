//go:build testtools

package modules

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"testing"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/i18n"
)

const mockMessageTypesYAML = `
message_type_text: "text"
message_type_sticker: "sticker"
message_type_document: "document"
message_type_photo: "photo"
message_type_audio: "audio"
message_type_voice: "voice"
message_type_video: "video"
message_type_video_note: "video note"
message_type_unknown: "unknown"
`

// ---- isPermanentTelegramError ----

func TestIsPermanentTelegramError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"unrelated error", errors.New("network timeout"), false},
		{"message to delete not found", errors.New("message to delete not found"), true},
		{"message can't be deleted", errors.New("message can't be deleted"), true},
		{"bot was kicked", errors.New("bot was kicked"), true},
		{"chat not found", errors.New("chat not found"), true},
		{"group chat was deactivated", errors.New("group chat was deactivated"), true},
		{"bot is not a member", errors.New("bot is not a member"), true},
		{"CHAT_NOT_FOUND", errors.New("CHAT_NOT_FOUND"), true},
		{"PEER_ID_INVALID", errors.New("PEER_ID_INVALID"), true},
		{"partial match", errors.New("something CHAT_NOT_FOUND something"), true},
		{"false positive", errors.New("chat found successfully"), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isPermanentTelegramError(tc.err)
			if got != tc.want {
				t.Fatalf("isPermanentTelegramError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

// ---- isPermanentUnmuteError ----

func TestIsPermanentUnmuteError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"unrelated error", errors.New("rate limit exceeded"), false},
		{"user not found", errors.New("user not found"), true},
		{"USER_NOT_PARTICIPANT", errors.New("USER_NOT_PARTICIPANT"), true},
		{"bot was kicked", errors.New("bot was kicked"), true},
		{"chat not found", errors.New("chat not found"), true},
		{"group chat was deactivated", errors.New("group chat was deactivated"), true},
		{"bot is not a member", errors.New("bot is not a member"), true},
		{"CHAT_NOT_FOUND", errors.New("CHAT_NOT_FOUND"), true},
		{"PEER_ID_INVALID", errors.New("PEER_ID_INVALID"), true},
		{"user is an administrator", errors.New("user is an administrator"), true},
		{"not enough rights", errors.New("not enough rights"), true},
		{"partial match", errors.New("error: PEER_ID_INVALID occurred"), true},
		{"false positive", errors.New("user found and active"), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isPermanentUnmuteError(tc.err)
			if got != tc.want {
				t.Fatalf("isPermanentUnmuteError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

// ---- messageTypeToString ----

func TestMessageTypeToString(t *testing.T) {
	tr, err := i18n.NewTestTranslator(mockMessageTypesYAML)
	if err != nil {
		t.Fatalf("NewTestTranslator() error = %v", err)
	}

	tests := []struct {
		msgType int
		want    string
	}{
		{db.TEXT, "text"},
		{db.STICKER, "sticker"},
		{db.DOCUMENT, "document"},
		{db.PHOTO, "photo"},
		{db.AUDIO, "audio"},
		{db.VOICE, "voice"},
		{db.VIDEO, "video"},
		{db.VIDEO_NOTE, "video note"},
		{999, "unknown"},
		{0, "unknown"},
		{-1, "unknown"},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("type_%d", tc.msgType), func(t *testing.T) {
			got := messageTypeToString(tr, tc.msgType)
			if got != tc.want {
				t.Fatalf("messageTypeToString(%d) = %q, want %q", tc.msgType, got, tc.want)
			}
		})
	}
}

// ---- secureIntn ----

func TestSecureIntn(t *testing.T) {
	t.Run("max zero returns zero", func(t *testing.T) {
		if got := secureIntn(0); got != 0 {
			t.Fatalf("secureIntn(0) = %d, want 0", got)
		}
	})

	t.Run("negative max returns zero", func(t *testing.T) {
		if got := secureIntn(-5); got != 0 {
			t.Fatalf("secureIntn(-5) = %d, want 0", got)
		}
	})

	t.Run("positive max returns value in range", func(t *testing.T) {
		const max = 100
		for i := 0; i < 50; i++ {
			got := secureIntn(max)
			if got < 0 || got >= max {
				t.Fatalf("secureIntn(%d) = %d, want [0, %d)", max, got, max)
			}
		}
	})

	t.Run("max 1 always returns 0", func(t *testing.T) {
		for i := 0; i < 20; i++ {
			if got := secureIntn(1); got != 0 {
				t.Fatalf("secureIntn(1) = %d, want 0", got)
			}
		}
	})
}

// ---- secureShuffleStrings ----

func TestSecureShuffleStrings(t *testing.T) {
	t.Run("empty slice", func(t *testing.T) {
		empty := []string{}
		secureShuffleStrings(empty)
		if len(empty) != 0 {
			t.Fatalf("expected empty slice, got %v", empty)
		}
	})

	t.Run("single element", func(t *testing.T) {
		single := []string{"only"}
		secureShuffleStrings(single)
		if len(single) != 1 || single[0] != "only" {
			t.Fatalf("expected [only], got %v", single)
		}
	})

	t.Run("multiple elements preserves set", func(t *testing.T) {
		original := []string{"a", "b", "c", "d", "e"}
		for i := 0; i < 20; i++ {
			copySlice := slices.Clone(original)
			secureShuffleStrings(copySlice)
			if len(copySlice) != len(original) {
				t.Fatalf("length changed: %d vs %d", len(copySlice), len(original))
			}
			for _, v := range original {
				if !slices.Contains(copySlice, v) {
					t.Fatalf("element %q missing after shuffle: %v", v, copySlice)
				}
			}
		}
	})
}

// ---- generateMathCaptcha ----

func TestGenerateMathCaptcha(t *testing.T) {
	for i := 0; i < 20; i++ {
		question, answer, options := generateMathCaptcha()

		if question == "" {
			t.Fatal("expected non-empty question")
		}

		if answer == "" {
			t.Fatal("expected non-empty answer")
		}

		if _, err := strconv.Atoi(answer); err != nil {
			t.Fatalf("answer %q is not a valid integer: %v", answer, err)
		}

		if len(options) != 4 {
			t.Fatalf("expected 4 options, got %d: %v", len(options), options)
		}

		if !slices.Contains(options, answer) {
			t.Fatalf("answer %q not found in options %v", answer, options)
		}

		for _, opt := range options {
			if _, err := strconv.Atoi(opt); err != nil {
				t.Fatalf("option %q is not a valid integer: %v", opt, err)
			}
		}
	}
}

func TestGenerateTextCaptcha(t *testing.T) {
	answer, imageBytes, options, err := generateTextCaptcha()
	if err != nil {
		t.Fatalf("generateTextCaptcha() error = %v", err)
	}
	assertCaptchaChallenge(t, answer, imageBytes, options, false)
}

func TestGenerateMathImageCaptcha(t *testing.T) {
	answer, imageBytes, options, err := generateMathImageCaptcha()
	if err != nil {
		t.Fatalf("generateMathImageCaptcha() error = %v", err)
	}
	assertCaptchaChallenge(t, answer, imageBytes, options, true)
}

func assertCaptchaChallenge(t *testing.T, answer string, imageBytes []byte, options []string, numeric bool) {
	t.Helper()

	if answer == "" {
		t.Fatal("expected non-empty answer")
	}
	if len(imageBytes) == 0 {
		t.Fatal("expected non-empty image bytes")
	}
	if len(imageBytes) < 8 || string(imageBytes[:8]) != "\x89PNG\r\n\x1a\n" {
		t.Fatalf("expected PNG image bytes, got prefix %q", string(imageBytes[:min(len(imageBytes), 8)]))
	}
	if len(options) != 4 {
		t.Fatalf("expected exactly 4 answer options, got %d: %v", len(options), options)
	}
	if !slices.Contains(options, answer) {
		t.Fatalf("answer %q not present in options %v", answer, options)
	}

	seen := make(map[string]struct{}, len(options))
	for _, option := range options {
		if option == "" {
			t.Fatalf("empty option in %v", options)
		}
		if _, ok := seen[option]; ok {
			t.Fatalf("duplicate option %q in %v", option, options)
		}
		seen[option] = struct{}{}
		if numeric {
			if _, err := strconv.Atoi(option); err != nil {
				t.Fatalf("numeric captcha option %q is not an integer: %v", option, err)
			}
		}
	}
}

// ---- noopCaptchaStore ----

func TestNoopCaptchaStore(t *testing.T) {
	store := noopCaptchaStore{}

	t.Run("Set returns nil", func(t *testing.T) {
		if err := store.Set("id", "value"); err != nil {
			t.Fatalf("Set() = %v, want nil", err)
		}
	})

	t.Run("Get returns empty string", func(t *testing.T) {
		if got := store.Get("id", true); got != "" {
			t.Fatalf("Get() = %q, want empty string", got)
		}
		if got := store.Get("id", false); got != "" {
			t.Fatalf("Get(clear=false) = %q, want empty string", got)
		}
	})

	t.Run("Verify returns false", func(t *testing.T) {
		if got := store.Verify("id", "answer", true); got != false {
			t.Fatalf("Verify() = %v, want false", got)
		}
		if got := store.Verify("id", "answer", false); got != false {
			t.Fatalf("Verify(clear=false) = %v, want false", got)
		}
	})
}

// ---- newMathImageCaptchaDriver ----

func TestNewMathImageCaptchaDriver(t *testing.T) {
	const question = "12 + 34"

	driver := newMathImageCaptchaDriver(question)

	if driver.content != question {
		t.Fatalf("expected content %q, got %q", question, driver.content)
	}

	if driver.ShowLineOptions != 0 {
		t.Fatalf("expected ShowLineOptions=0, got %d", driver.ShowLineOptions)
	}

	if driver.Height != 80 {
		t.Fatalf("expected Height=80, got %d", driver.Height)
	}

	if driver.Width != 240 {
		t.Fatalf("expected Width=240, got %d", driver.Width)
	}
}

// ---- formatMathQuestion ----

func TestFormatMathQuestion(t *testing.T) {
	tests := []struct {
		a, b int
		op   string
		want string
	}{
		{5, 3, "+", "5 + 3"},
		{10, 4, "-", "10 - 4"},
		{7, 8, "*", "7 x 8"},
		{0, 0, "+", "0 + 0"},
		{99, 1, "-", "99 - 1"},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("%d_%s_%d", tc.a, tc.op, tc.b), func(t *testing.T) {
			got := formatMathQuestion(tc.a, tc.b, tc.op)
			if got != tc.want {
				t.Fatalf("formatMathQuestion(%d, %d, %q) = %q, want %q", tc.a, tc.b, tc.op, got, tc.want)
			}
		})
	}
}

// ---- fixedStringCaptchaDriver.GenerateIdQuestionAnswer ----

func TestFixedStringCaptchaDriverGenerateIdQuestionAnswer(t *testing.T) {
	const question = "12 + 34"

	driver := newMathImageCaptchaDriver(question)

	id, q, a := driver.GenerateIdQuestionAnswer()

	if id == "" {
		t.Fatal("expected non-empty id")
	}

	if q != question {
		t.Fatalf("expected question %q, got %q", question, q)
	}

	if a != question {
		t.Fatalf("expected answer %q, got %q", question, a)
	}
}

// TestFixedStringCaptchaDriverReturnsExactQuestion is kept for backward compatibility.
func TestFixedStringCaptchaDriverReturnsExactQuestion(t *testing.T) {
	const question = "12 + 34"

	driver := newMathImageCaptchaDriver(question)

	_, gotQuestion, gotAnswer := driver.GenerateIdQuestionAnswer()

	if gotQuestion != question {
		t.Fatalf("expected generated question %q, got %q", question, gotQuestion)
	}

	if gotAnswer != question {
		t.Fatalf("expected generated answer payload %q, got %q", question, gotAnswer)
	}
}

func TestFormatMathQuestionUsesASCIIForMultiplication(t *testing.T) {
	got := formatMathQuestion(10, 3, "*")
	want := "10 x 3"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestMathCaptchaDriverDisablesLineNoise(t *testing.T) {
	const question = "10 x 3"

	driver := newMathImageCaptchaDriver(question)

	if driver.ShowLineOptions != 0 {
		t.Fatalf("expected show line options to be disabled, got %d", driver.ShowLineOptions)
	}
}

// TestParseInt64ForLargeUserIDs verifies that ParseInt with 64-bit can handle
// user IDs exceeding 32-bit integer range (2^31). This prevents integer overflow
// on 32-bit systems when handling Telegram user IDs.
func TestParseInt64ForLargeUserIDs(t *testing.T) {
	testCases := []struct {
		name    string
		userID  string
		wantErr bool
	}{
		{
			name:    "normal user ID",
			userID:  "123456789",
			wantErr: false,
		},
		{
			name:    "large user ID exceeding 32-bit max",
			userID:  "2147483648", // 2^31
			wantErr: false,
		},
		{
			name:    "very large user ID near 64-bit range",
			userID:  "9223372036854775807", // math.MaxInt64
			wantErr: false,
		},
		{
			name:    "negative user ID (channel)",
			userID:  "-1001234567890",
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// This mimics how the captcha module parses user IDs from callback data
			parsedID, err := strconv.ParseInt(tc.userID, 10, 64)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for userID %s, got none", tc.userID)
				}
				return
			}
			if err != nil {
				t.Fatalf("failed to parse userID %s: %v", tc.userID, err)
			}

			// Verify the parsed value matches the original
			if strconv.FormatInt(parsedID, 10) != tc.userID {
				t.Fatalf("round-trip failed: got %d, want %s", parsedID, tc.userID)
			}
		})
	}
}
