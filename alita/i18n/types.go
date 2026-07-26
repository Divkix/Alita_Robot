package i18n

import (
	"embed"
	"sync"
)

// TranslationParams represents parameters for translation interpolation
type TranslationParams map[string]any

// LocaleManager manages all locales with thread-safe operations
type LocaleManager struct {
	mu          sync.RWMutex
	localeMaps  map[string]map[string]any // Parsed YAML maps per language
	defaultLang string
	localeFS    *embed.FS
	localePath  string
}

// Translator provides translation methods for a specific language
type Translator struct {
	langCode string
	manager  *LocaleManager
	data     map[string]any // Parsed YAML map for this language
}

// LoaderConfig defines configuration for locale loading
type LoaderConfig struct {
	DefaultLanguage string
	StrictMode      bool // Fail if any locale file has errors
}

// ManagerConfig combines all configuration options
type ManagerConfig struct {
	Loader LoaderConfig
}

// DefaultManagerConfig returns sensible defaults for ManagerConfig.
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		Loader: LoaderConfig{
			DefaultLanguage: "en",
			StrictMode:      false,
		},
	}
}
