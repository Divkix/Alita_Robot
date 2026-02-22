package db

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Cache Key Generators
// ---------------------------------------------------------------------------

func TestCacheKeyGenerators(t *testing.T) {
	t.Parallel()

	type keyCase struct {
		id   int64
		want string
	}

	type generatorTest struct {
		name string
		fn   func(int64) string
		// Expected key format prefix (everything before the ID)
		prefix string
	}

	generators := []generatorTest{
		{name: "chatSettingsCacheKey", fn: chatSettingsCacheKey, prefix: "alita:chat_settings:"},
		{name: "userLanguageCacheKey", fn: userLanguageCacheKey, prefix: "alita:user_lang:"},
		{name: "chatLanguageCacheKey", fn: chatLanguageCacheKey, prefix: "alita:chat_lang:"},
		{name: "filterListCacheKey", fn: filterListCacheKey, prefix: "alita:filter_list:"},
		{name: "blacklistCacheKey", fn: blacklistCacheKey, prefix: "alita:blacklist:"},
		{name: "warnSettingsCacheKey", fn: warnSettingsCacheKey, prefix: "alita:warn_settings:"},
		{name: "disabledCommandsCacheKey", fn: disabledCommandsCacheKey, prefix: "alita:disabled_cmds:"},
		{name: "captchaSettingsCacheKey", fn: captchaSettingsCacheKey, prefix: "alita:captcha_settings:"},
	}

	ids := []keyCase{
		{id: 12345, want: "12345"},
		{id: 0, want: "0"},
		{id: -1001234567890, want: "-1001234567890"},
	}

	for _, gen := range generators {
		t.Run(gen.name, func(t *testing.T) {
			t.Parallel()

			for _, ic := range ids {
				got := gen.fn(ic.id)

				// Must start with "alita:" prefix
				if !strings.HasPrefix(got, "alita:") {
					t.Errorf("%s(%d) = %q: missing 'alita:' prefix", gen.name, ic.id, got)
				}

				// Must start with the expected key prefix
				if !strings.HasPrefix(got, gen.prefix) {
					t.Errorf("%s(%d) = %q: want prefix %q", gen.name, ic.id, got, gen.prefix)
				}

				// Must end with the expected ID string
				if !strings.HasSuffix(got, ic.want) {
					t.Errorf("%s(%d) = %q: want suffix %q", gen.name, ic.id, got, ic.want)
				}
			}
		})
	}
}

// TestCacheKeyGeneratorsUnique verifies that all 8 key generators produce
// distinct keys for the same input ID to prevent cache collisions.
func TestCacheKeyGeneratorsUnique(t *testing.T) {
	t.Parallel()

	const id = int64(12345)

	keys := []string{
		chatSettingsCacheKey(id),
		userLanguageCacheKey(id),
		chatLanguageCacheKey(id),
		filterListCacheKey(id),
		blacklistCacheKey(id),
		warnSettingsCacheKey(id),
		disabledCommandsCacheKey(id),
		captchaSettingsCacheKey(id),
	}

	seen := make(map[string]bool, len(keys))
	for _, k := range keys {
		if seen[k] {
			t.Fatalf("duplicate cache key detected: %q", k)
		}
		seen[k] = true
	}
}
