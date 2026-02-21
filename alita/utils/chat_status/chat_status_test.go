package chat_status

import (
	"math"
	"testing"
)

func TestIsValidUserId(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		id   int64
		want bool
	}{
		{"positive user id", 12345678, true},
		{"max int64", math.MaxInt64, true},
		{"one", 1, true},
		{"zero is invalid", 0, false},
		{"negative group id", -100000, false},
		{"channel id boundary", -1000000000000, false},
		{"channel id below boundary", -1000000000001, false},
		{"min int64", math.MinInt64, false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := IsValidUserId(tc.id); got != tc.want {
				t.Errorf("IsValidUserId(%d) = %v, want %v", tc.id, got, tc.want)
			}
		})
	}
}

func TestIsChannelId(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		id   int64
		want bool
	}{
		{"channel id just below boundary", -1000000000001, true},
		{"large channel id", -1000000000123, true},
		{"min int64", math.MinInt64, true},
		{"boundary value not channel", -1000000000000, false},
		{"regular group id", -100000, false},
		{"positive user id", 12345678, false},
		{"zero", 0, false},
		{"max int64", math.MaxInt64, false},
		{"minus one", -1, false},
		{"just above boundary", -999999999999, false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := IsChannelId(tc.id); got != tc.want {
				t.Errorf("IsChannelId(%d) = %v, want %v", tc.id, got, tc.want)
			}
		})
	}
}
