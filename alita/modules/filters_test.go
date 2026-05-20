package modules

import (
	"testing"

	"github.com/divkix/Alita_Robot/alita/db"
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
	withNilCacheMarshal(t)

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

func TestFilterOverwriteCacheRoundTripsCurrentAndLegacyData(t *testing.T) {
	if cache.Marshal == nil {
		t.Skip("requires cache marshal")
	}

	current := overwriteFilter{overwriteBase: overwriteBase{
		ChatID:   -100123,
		ItemName: "hello",
		Text:     "current",
		DataType: db.TEXT,
	}}
	if err := setFilterOverwriteCache("token-current", current); err != nil {
		t.Fatalf("setFilterOverwriteCache() error = %v", err)
	}
	got, err := getFilterOverwriteCache("token-current")
	if err != nil {
		t.Fatalf("getFilterOverwriteCache() error = %v", err)
	}
	if got.ChatID != current.ChatID || got.ItemName != current.ItemName || got.Text != current.Text {
		t.Fatalf("getFilterOverwriteCache() = %+v, want %+v", got, current)
	}
	deleteFilterOverwriteCache("token-current")
	if _, err := getFilterOverwriteCache("token-current"); err == nil {
		t.Fatal("getFilterOverwriteCache(deleted) error = nil, want cache miss")
	}

	legacy := overwriteFilter{overwriteBase: overwriteBase{
		ChatID:   -100123,
		ItemName: "legacy",
		Text:     "legacy text",
		DataType: db.TEXT,
	}}
	if err := cache.Marshal.Set(cache.Context, legacyFilterOverwriteCacheKey("legacy", -100123), legacy); err != nil {
		t.Fatalf("legacy cache set error = %v", err)
	}
	gotLegacy, err := getLegacyFilterOverwriteCache("legacy", -100123)
	if err != nil {
		t.Fatalf("getLegacyFilterOverwriteCache() error = %v", err)
	}
	if gotLegacy.ItemName != legacy.ItemName || gotLegacy.Text != legacy.Text {
		t.Fatalf("getLegacyFilterOverwriteCache() = %+v, want %+v", gotLegacy, legacy)
	}
	deleteLegacyFilterOverwriteCache("legacy", -100123)
	if _, err := getLegacyFilterOverwriteCache("legacy", -100123); err == nil {
		t.Fatal("getLegacyFilterOverwriteCache(deleted) error = nil, want cache miss")
	}
}
