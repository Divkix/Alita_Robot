package modules

import (
	"testing"

	"github.com/divkix/Alita_Robot/alita/i18n"
)

// The shared lock and pin toggle handlers read their replies from config
// structs, which check-translations cannot see: every reply must resolve.
func TestToggleRepliesResolveInEveryLocale(t *testing.T) {
	useRealLocales(t)

	keys := []string{
		lockKeys.whatKey, lockKeys.failedKey, lockKeys.doneKey,
		unlockKeys.whatKey, unlockKeys.failedKey, unlockKeys.doneKey,
	}
	for _, cfg := range []pinToggleConfig{antiChannelPinToggle, cleanLinkedToggle} {
		keys = append(keys, cfg.enabledKey, cfg.disabledKey, cfg.currentEnabledKey, cfg.currentDisabledKey)
	}
	for _, code := range localeCodes {
		tr := i18n.MustNewTranslator(code)
		for _, key := range keys {
			if text, err := tr.GetString(key); err != nil || text == "" {
				t.Errorf("%s: GetString(%q) = %q, %v", code, key, text, err)
			}
		}
	}
}
