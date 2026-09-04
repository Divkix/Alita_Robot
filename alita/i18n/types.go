package i18n

import (
	"embed"
	"sync"
)

type TranslationParams map[string]any

// LocaleManager manages all locales with thread-safe operations
type LocaleManager struct {
	mu          sync.RWMutex
	localeMaps  map[string]map[string]any
	defaultLang string
	localeFS    *embed.FS
	localePath  string
}

type Translator struct {
	langCode string
	manager  *LocaleManager
	data     map[string]any
}

type LoaderConfig struct {
	DefaultLanguage string
	StrictMode      bool // Fail if any locale file has errors
}

type ManagerConfig struct {
	Loader LoaderConfig
}

func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		Loader: LoaderConfig{
			DefaultLanguage: "en",
			StrictMode:      false,
		},
	}
}
