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
		{name: "positive user ID", id: 123456789, want: true},
		{name: "minimum valid (1)", id: 1, want: true},
		{name: "zero", id: 0, want: false},
		{name: "negative one", id: -1, want: false},
		{name: "channel ID -1001234567890", id: -1001234567890, want: false},
		{name: "MaxInt64", id: math.MaxInt64, want: true},
		{name: "MinInt64", id: math.MinInt64, want: false},
		{name: "Group Anonymous Bot 1087968824", id: 1087968824, want: true},
		{name: "Telegram 777000", id: 777000, want: true},
	}

	for _, tc := range tests {

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := IsValidUserId(tc.id)
			if got != tc.want {
				t.Fatalf("IsValidUserId(%d) = %v, want %v", tc.id, got, tc.want)
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
		{name: "typical channel ID -1001234567890", id: -1001234567890, want: true},
		{name: "boundary first channel ID -1000000000001", id: -1000000000001, want: true},
		{name: "boundary not channel -1000000000000", id: -1000000000000, want: false},
		{name: "positive user ID 123456789", id: 123456789, want: false},
		{name: "zero", id: 0, want: false},
		{name: "MaxInt64", id: math.MaxInt64, want: false},
		{name: "MinInt64", id: math.MinInt64, want: true},
	}

	for _, tc := range tests {

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := IsChannelId(tc.id)
			if got != tc.want {
				t.Fatalf("IsChannelId(%d) = %v, want %v", tc.id, got, tc.want)
			}
		})
	}
}
