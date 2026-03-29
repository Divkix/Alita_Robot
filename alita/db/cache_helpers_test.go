package db

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Cache Key Generator
// ---------------------------------------------------------------------------

func TestCacheKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		module   string
		ids      []any
		expected string
	}{
		{
			name:     "single int64 ID",
			module:   "chat_settings",
			ids:      []any{int64(12345)},
			expected: "alita:chat_settings:12345",
		},
		{
			name:     "single string ID",
			module:   "user",
			ids:      []any{"abc123"},
			expected: "alita:user:abc123",
		},
		{
			name:     "multiple IDs - int64 and string",
			module:   "lock",
			ids:      []any{int64(123), "photos"},
			expected: "alita:lock:123:photos",
		},
		{
			name:     "zero ID",
			module:   "chat",
			ids:      []any{int64(0)},
			expected: "alita:chat:0",
		},
		{
			name:     "negative chat ID",
			module:   "chat_lang",
			ids:      []any{int64(-1001234567890)},
			expected: "alita:chat_lang:-1001234567890",
		},
		{
			name:     "no IDs",
			module:   "stats",
			ids:      []any{},
			expected: "alita:stats",
		},
		{
			name:     "multiple int64 IDs",
			module:   "user_chat",
			ids:      []any{int64(123), int64(456)},
			expected: "alita:user_chat:123:456",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := CacheKey(tc.module, tc.ids...)

			// Verify the result
			if got != tc.expected {
				t.Errorf("CacheKey(%q, %v) = %q, want %q", tc.module, tc.ids, got, tc.expected)
			}

			// Must start with "alita:" prefix
			if !strings.HasPrefix(got, "alita:") {
				t.Errorf("CacheKey(%q, %v) = %q: missing 'alita:' prefix", tc.module, tc.ids, got)
			}

			// Must contain the module name
			if !strings.Contains(got, tc.module) {
				t.Errorf("CacheKey(%q, %v) = %q: missing module %q", tc.module, tc.ids, got, tc.module)
			}
		})
	}
}

// TestCacheKeyUnique verifies that different module/ID combinations produce
// distinct keys to prevent cache collisions.
func TestCacheKeyUnique(t *testing.T) {
	t.Parallel()

	const id = int64(12345)

	keys := []string{
		CacheKey("chat_settings", id),
		CacheKey("user_lang", id),
		CacheKey("chat_lang", id),
		CacheKey("filter_list", id),
		CacheKey("blacklist", id),
		CacheKey("warn_settings", id),
		CacheKey("disabled_cmds", id),
		CacheKey("captcha_settings", id),
	}

	seen := make(map[string]bool, len(keys))
	for _, k := range keys {
		if seen[k] {
			t.Fatalf("duplicate cache key detected: %q", k)
		}
		seen[k] = true
	}
}

// TestCacheKeyConsistency verifies that calling CacheKey multiple times
// with the same arguments produces the same result.
func TestCacheKeyConsistency(t *testing.T) {
	t.Parallel()

	// Call multiple times with same args
	key1 := CacheKey("test", int64(123), "abc")
	key2 := CacheKey("test", int64(123), "abc")
	key3 := CacheKey("test", int64(123), "abc")

	if key1 != key2 || key2 != key3 {
		t.Errorf("CacheKey not consistent: %q, %q, %q", key1, key2, key3)
	}
}
