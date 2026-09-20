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
	localeIndex map[string]*lookupIndex
	// translators caches one immutable *Translator per language code, written once and
	// read from every request, so reads stay lock-free.
	translators sync.Map
	defaultLang string
	localeFS    *embed.FS
	localePath  string
}

type Translator struct {
	langCode string
	manager  *LocaleManager
	data     map[string]any
	// index is nil for hand-constructed translators; lookups then fall back to the
	// generic dot-path walk over data.
	index *lookupIndex
}

// lookupIndex is a locale's flat lookup table: every dot-path lowered to its scalar
// string form, plus the resolved []string for sequence leaves. A nil index means the
// table was never built, and callers fall back to the generic dot-path walk.
type lookupIndex struct {
	scalars map[string]string
	slices  map[string][]string
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
