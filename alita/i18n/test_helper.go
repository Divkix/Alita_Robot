package i18n

import (
	"github.com/spf13/viper"
)

// NewTestTranslator creates a Translator backed by inline YAML content for tests
// in other packages that need a functioning translator without initializing the
// full embedded locale filesystem. This helper lives in a regular (non _test) file
// so that it is available when packages outside i18n are tested.
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

