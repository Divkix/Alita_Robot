package helpers

import (
	"testing"
)

func TestIsChannelID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		chatID int64
		want   bool
	}{
		{
			name:   "typical channel ID is below threshold",
			chatID: -1001234567890,
			want:   true,
		},
		{
			name:   "exactly at boundary is channel",
			chatID: -1000000000001,
			want:   true,
		},
		{
			name:   "exactly at boundary is NOT channel",
			chatID: -1000000000000,
			want:   false,
		},
		{
			name:   "regular group ID (negative not channel)",
			chatID: -100,
			want:   false,
		},
		{
			name:   "positive user ID is not a channel",
			chatID: 123456789,
			want:   false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := IsChannelID(tc.chatID)
			if got != tc.want {
				t.Errorf("IsChannelID(%d) = %v, want %v", tc.chatID, got, tc.want)
			}
		})
	}
}
