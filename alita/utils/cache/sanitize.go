package cache

import "strings"

// SanitizeCacheKey extracts the namespace prefix from a cache key,
// stripping high-cardinality identifiers (user/chat IDs) to produce
// low-cardinality span attributes suitable for tracing.
//
// Example: "alita:chat_settings:123456" → "alita:chat_settings"
func SanitizeCacheKey(key string) string {
	parts := strings.SplitN(key, ":", 3)
	if len(parts) <= 2 {
		return key
	}
	return parts[0] + ":" + parts[1]
}

// CacheKeySegmentCount returns the number of colon-separated segments
// in a cache key. This provides structural context for debugging without
// adding cardinality to trace attributes.
func CacheKeySegmentCount(key string) int {
	return strings.Count(key, ":") + 1
}
