package modules

import (
	"testing"

	"github.com/divkix/Alita_Robot/alita/utils/cache"
)

func TestFilterOverwriteCacheKeysAndToken(t *testing.T) {
	if got := filterOverwriteCacheKey("abc123"); got != "alita:filter_overwrite:abc123" {
		t.Fatalf("filterOverwriteCacheKey() = %q", got)
	}
	if got := legacyFilterOverwriteCacheKey("Hello World", -100123); got != "alita:filter_overwrite:Hello World:-100123" {
		t.Fatalf("legacyFilterOverwriteCacheKey() = %q", got)
	}

	token, err := newOverwriteToken()
	if err != nil {
		t.Fatalf("newOverwriteToken() error = %v", err)
	}
	if len(token) != 16 {
		t.Fatalf("newOverwriteToken() len = %d, want 16 hex chars", len(token))
	}
	for _, ch := range token {
		if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') {
			t.Fatalf("newOverwriteToken() contains non-hex character %q in %q", ch, token)
		}
	}
}

func TestFilterOverwriteCacheNoCacheFallbacks(t *testing.T) {
	originalMarshal := cache.Marshal
	cache.Marshal = nil
	t.Cleanup(func() {
		cache.Marshal = originalMarshal
	})

	data := overwriteFilter{overwriteBase: overwriteBase{
		ChatID:   -100123,
		ItemName: "hello",
		Text:     "world",
		DataType: 1,
	}}

	if err := setFilterOverwriteCache("token", data); err == nil {
		t.Fatal("setFilterOverwriteCache() error = nil, want cache not initialized")
	}
	if _, err := getFilterOverwriteCache("token"); err == nil {
		t.Fatal("getFilterOverwriteCache() error = nil, want cache not initialized")
	}
	if _, err := getLegacyFilterOverwriteCache("hello", -100123); err == nil {
		t.Fatal("getLegacyFilterOverwriteCache() error = nil, want cache not initialized")
	}

	deleteFilterOverwriteCache("token")
	deleteLegacyFilterOverwriteCache("hello", -100123)
}
