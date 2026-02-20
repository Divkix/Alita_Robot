package cache

import "testing"

func TestSanitizeCacheKey(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{
			name:     "standard 3-segment key",
			key:      "alita:chat_settings:123",
			expected: "alita:chat_settings",
		},
		{
			name:     "4-segment key",
			key:      "alita:lock:123:flood",
			expected: "alita:lock",
		},
		{
			name:     "6-segment deep key",
			key:      "alita:captcha:refresh:cooldown:123:456",
			expected: "alita:captcha",
		},
		{
			name:     "admin cache key",
			key:      "alita:adminCache:123",
			expected: "alita:adminCache",
		},
		{
			name:     "empty string",
			key:      "",
			expected: "",
		},
		{
			name:     "no colons",
			key:      "plainkey",
			expected: "plainkey",
		},
		{
			name:     "single colon only prefix",
			key:      "alita:",
			expected: "alita:",
		},
		{
			name:     "exactly two segments",
			key:      "alita:module",
			expected: "alita:module",
		},
		{
			name:     "trailing colon after module",
			key:      "alita:module:",
			expected: "alita:module",
		},
		{
			name:     "leading colon",
			key:      ":module:123",
			expected: ":module",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeCacheKey(tt.key)
			if got != tt.expected {
				t.Errorf("SanitizeCacheKey(%q) = %q, want %q", tt.key, got, tt.expected)
			}
		})
	}
}

func TestCacheKeySegmentCount(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected int
	}{
		{
			name:     "standard 3-segment key",
			key:      "alita:chat_settings:123",
			expected: 3,
		},
		{
			name:     "4-segment key",
			key:      "alita:lock:123:flood",
			expected: 4,
		},
		{
			name:     "6-segment deep key",
			key:      "alita:captcha:refresh:cooldown:123:456",
			expected: 6,
		},
		{
			name:     "empty string",
			key:      "",
			expected: 1,
		},
		{
			name:     "no colons",
			key:      "plainkey",
			expected: 1,
		},
		{
			name:     "exactly two segments",
			key:      "alita:module",
			expected: 2,
		},
		{
			name:     "trailing colon",
			key:      "alita:module:",
			expected: 3,
		},
		{
			name:     "leading colon",
			key:      ":module:123",
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CacheKeySegmentCount(tt.key)
			if got != tt.expected {
				t.Errorf("CacheKeySegmentCount(%q) = %d, want %d", tt.key, got, tt.expected)
			}
		})
	}
}
