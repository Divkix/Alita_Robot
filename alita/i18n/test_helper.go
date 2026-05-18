//go:build testtools

package i18n

import (
	"github.com/spf13/viper"
)

// NewTestTranslator creates a Translator backed by inline YAML content for tests.
// It is guarded by the `testtools` build tag so it is excluded from production builds.
func NewTestTranslator(yamlContent string) (*Translator, error) {
	vi, err := compileViper([]byte(yamlContent))
	if err != nil {
		return nil, err
	}
	lm := &LocaleManager{
		defaultLang: "en",
		viperCache:  map[string]*viper.Viper{"en": vi},
		localeData:  map[string][]byte{"en": []byte(yamlContent)},
	}
	return &Translator{
		langCode:    "en",
		manager:     lm,
		viper:       vi,
		cachePrefix: "i18n:en:",
	}, nil
}
