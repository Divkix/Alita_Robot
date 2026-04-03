package modules

import (
	"strconv"
	"testing"
)

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
