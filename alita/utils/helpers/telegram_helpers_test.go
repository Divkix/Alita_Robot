package helpers

import (
	"errors"
	"testing"
)

func TestIsExpectedTelegramError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		errMsg  string
		wantNil bool // true means err is nil
		want    bool
	}{
		{
			name:    "nil error returns false",
			wantNil: true,
			want:    false,
		},
		{
			name:   "bot kicked error",
			errMsg: "bot was kicked from the supergroup chat",
			want:   true,
		},
		{
			name:   "bot blocked by user",
			errMsg: "bot was blocked by the user",
			want:   true,
		},
		{
			name:   "chat not found",
			errMsg: "chat not found",
			want:   true,
		},
		{
			name:   "message thread not found",
			errMsg: "message thread not found",
			want:   true,
		},
		{
			name:   "context deadline exceeded",
			errMsg: "context deadline exceeded",
			want:   true,
		},
		{
			name:   "not enough rights to restrict",
			errMsg: "not enough rights to restrict/unrestrict chat member",
			want:   true,
		},
		{
			name:   "message can't be deleted",
			errMsg: "message can't be deleted",
			want:   true,
		},
		{
			name:   "message to delete not found",
			errMsg: "message to delete not found",
			want:   true,
		},
		{
			name:   "CHAT_RESTRICTED",
			errMsg: "CHAT_RESTRICTED",
			want:   true,
		},
		{
			name:   "unknown unrelated error returns false",
			errMsg: "internal server error: something went wrong",
			want:   false,
		},
		{
			name:   "empty error message returns false",
			errMsg: "",
			want:   false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var err error
			if !tc.wantNil {
				err = errors.New(tc.errMsg)
			}
			got := IsExpectedTelegramError(err)
			if got != tc.want {
				t.Errorf("IsExpectedTelegramError(%q) = %v, want %v", tc.errMsg, got, tc.want)
			}
		})
	}
}
