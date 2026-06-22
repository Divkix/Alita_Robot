package i18n

import (
	"embed"
	"sync"
	"time"

	"github.com/eko/gocache/lib/v4/cache"
)

// TranslationParams represents parameters for translation interpolation
type TranslationParams map[string]any

// LocaleManager manages all locales with thread-safe operations
type LocaleManager struct {
	mu          sync.RWMutex
	localeMaps  map[string]map[string]any // Parsed YAML maps per language
	localeData  map[string][]byte         // Raw YAML data
	cacheClient *cache.Cache[any]         // External cache for translations
	defaultLang string
	localeFS    *embed.FS
	localePath  string
}

// Translator provides translation methods for a specific language
type Translator struct {
	langCode    string
	manager     *LocaleManager
	data        map[string]any // Parsed YAML map for this language
	cachePrefix string         // Cache key prefix for this language
}

// CacheConfig defines cache configuration for translations
type CacheConfig struct {
	TTL               time.Duration
	EnableCache       bool
	CacheKeyPrefix    string
	MaxCacheSize      int64
	InvalidateOnError bool
}

// LoaderConfig defines configuration for locale loading
type LoaderConfig struct {
	DefaultLanguage string
	ValidateYAML    bool
	StrictMode      bool // Fail if any locale file has errors
}

// ManagerConfig combines all configuration options
type ManagerConfig struct {
	Cache  CacheConfig
	Loader LoaderConfig
}

// DefaultManagerConfig returns sensible defaults for ManagerConfig.
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		Cache: CacheConfig{
			TTL:               30 * time.Minute,
			EnableCache:       true,
			CacheKeyPrefix:    "i18n:",
			MaxCacheSize:      1000,
			InvalidateOnError: false,
		},
		Loader: LoaderConfig{
			DefaultLanguage: "en",
			ValidateYAML:    true,
			StrictMode:      false,
		},
	}
}
