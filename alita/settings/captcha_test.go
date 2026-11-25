//go:build testtools

package settings

import "testing"

// Both number declarations share captchaNumber; each keeps its own bounds and
// message, and max attempts mirrors the migration's default of 2.
func TestCaptchaNumberBounds(t *testing.T) {
	if CaptchaMaxAttempts.Default != 2 {
		t.Fatalf("CaptchaMaxAttempts.Default = %v, want 2", CaptchaMaxAttempts.Default)
	}
	for _, value := range []int{1, 2, 3} {
		if err := CaptchaMaxAttempts.Validate(value); err != nil {
			t.Fatalf("CaptchaMaxAttempts.Validate(%d) = %v, want nil", value, err)
		}
	}
	for _, value := range []int{0, 4, -1, 11} {
		if err := CaptchaMaxAttempts.Validate(value); err == nil {
			t.Fatalf("CaptchaMaxAttempts.Validate(%d) = nil, want error", value)
		}
	}
	for _, value := range []int{1, 5, 10} {
		if err := CaptchaTimeout.Validate(value); err != nil {
			t.Fatalf("CaptchaTimeout.Validate(%d) = %v, want nil", value, err)
		}
	}
	for _, value := range []int{0, 11, -1} {
		if err := CaptchaTimeout.Validate(value); err == nil {
			t.Fatalf("CaptchaTimeout.Validate(%d) = nil, want error", value)
		}
	}
}
