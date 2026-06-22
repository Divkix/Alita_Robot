//go:build testtools

package i18n

import (
	"embed"
)

// NewTestTranslator creates a Translator backed by inline YAML content for tests.
// It is guarded by the `testtools` build tag so it is excluded from production builds.
func NewTestTranslator(yamlContent string) (*Translator, error) {
	data, err := parseYAML([]byte(yamlContent))
	if err != nil {
		return nil, err
	}
	lm := &LocaleManager{
		defaultLang: "en",
		localeMaps:  map[string]map[string]any{"en": data},
		localeData:  map[string][]byte{"en": []byte(yamlContent)},
	}
	return &Translator{
		langCode:    "en",
		manager:     lm,
		data:        data,
		cachePrefix: "i18n:en:",
	}, nil
}

// OverrideManagerForTest replaces the global LocaleManager singleton with one backed
// by the provided inline YAML content. It returns a restore function that reverts
// managerInstance to its prior value; call it via t.Cleanup. This is intentionally
// NOT goroutine-safe across parallel sub-tests that also call this function — run
// the parent test without t.Parallel() or ensure sub-tests share the same override.
//
// It is guarded by the testtools build tag and is never compiled into production.
func OverrideManagerForTest(yamlContent string) (restore func(), err error) {
	data, err := parseYAML([]byte(yamlContent))
	if err != nil {
		return nil, err
	}
	// A non-nil *embed.FS is required to pass GetTranslator's localeFS nil-guard.
	// The value is never used for file I/O because localeMaps is already populated.
	var dummyFS embed.FS
	lm := &LocaleManager{
		defaultLang: "en",
		localeMaps:  map[string]map[string]any{"en": data},
		localeData:  map[string][]byte{"en": []byte(yamlContent)},
		localeFS:    &dummyFS,
	}
	// Ensure managerOnce.Do has already fired before we overwrite managerInstance.
	// If we set managerInstance first and Then GetManager() is called, Once.Do would
	// fire and overwrite our value with the empty default singleton.
	_ = GetManager()
	prev := managerInstance
	managerInstance = lm
	return func() { managerInstance = prev }, nil
}
