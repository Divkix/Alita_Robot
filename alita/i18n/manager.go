package i18n

import (
	"embed"
	"fmt"
	"sync"

	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/spf13/viper"
)

var (
	managerInstance *LocaleManager
	managerOnce     sync.Once
)

// GetManager returns the singleton LocaleManager instance.
func GetManager() *LocaleManager {
	managerOnce.Do(func() {
		managerInstance = &LocaleManager{
			viperCache:  make(map[string]*viper.Viper),
			localeData:  make(map[string][]byte),
			defaultLang: "en",
		}
	})
	return managerInstance
}

// Initialize initializes the LocaleManager with the provided configuration.
func (lm *LocaleManager) Initialize(fs *embed.FS, localePath string, config ManagerConfig) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	// Prevent re-initialization
	if lm.localeFS != nil {
		return fmt.Errorf("locale manager already initialized")
	}

	lm.localeFS = fs
	lm.localePath = localePath
	lm.defaultLang = config.Loader.DefaultLanguage

	// Initialize cache if available
	if config.Cache.EnableCache && cache.Manager != nil {
		lm.cacheClient = cache.Manager
	}

	// Load all locale files
	if err := lm.loadLocaleFiles(); err != nil {
		if config.Loader.StrictMode {
			return NewI18nError("initialize", "", "", "failed to load locale files", err)
		}
		// In non-strict mode, log error but continue
		fmt.Printf("Warning: failed to load some locale files: %v\n", err)
	}

	// Validate default language exists
	if _, exists := lm.localeData[lm.defaultLang]; !exists {
		return NewI18nError("initialize", lm.defaultLang, "", "default language not found", ErrLocaleNotFound)
	}

	return nil
}

// GetTranslator returns a translator for the specified language.
func (lm *LocaleManager) GetTranslator(langCode string) (*Translator, error) {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	if lm.localeFS == nil {
		return nil, NewI18nError("get_translator", langCode, "", "manager not initialized", ErrManagerNotInit)
	}

	// Check if language exists, fallback to default if not
	targetLang := langCode
	viperInstance, exists := lm.viperCache[langCode]
	if !exists {
		// Fallback to default language
		targetLang = lm.defaultLang
		viperInstance = lm.viperCache[lm.defaultLang]
		if viperInstance == nil {
			return nil, NewI18nError("get_translator", langCode, "", "default language viper not found", ErrLocaleNotFound)
		}
	}

	return &Translator{
		langCode:    targetLang,
		manager:     lm,
		viper:       viperInstance,
		cachePrefix: fmt.Sprintf("i18n:%s:", targetLang),
	}, nil
}

// GetAvailableLanguages returns a slice of all available language codes.
func (lm *LocaleManager) GetAvailableLanguages() []string {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	languages := make([]string, 0, len(lm.localeData))
	for langCode := range lm.localeData {
		languages = append(languages, langCode)
	}
	return languages
}

// IsLanguageSupported checks if a language is supported.
func (lm *LocaleManager) IsLanguageSupported(langCode string) bool {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	_, exists := lm.localeData[langCode]
	return exists
}

// GetDefaultLanguage returns the default language code.
func (lm *LocaleManager) GetDefaultLanguage() string {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	return lm.defaultLang
}

// ReloadLocales reloads all locale files (useful for development).
func (lm *LocaleManager) ReloadLocales() error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	if lm.localeFS == nil {
		return NewI18nError("reload", "", "", "manager not initialized", ErrManagerNotInit)
	}

	// Clear existing caches
	lm.viperCache = make(map[string]*viper.Viper)
	lm.localeData = make(map[string][]byte)

	// Clear external cache if available
	// Note: This would clear all cache, not just i18n entries
	// In production, you might want to implement selective clearing
	// if lm.cacheClient != nil {
	//     // TODO: Implement selective cache clearing
	// }

	return lm.loadLocaleFiles()
}

// GetStats returns statistics about the locale manager.
func (lm *LocaleManager) GetStats() map[string]any {
	languages := lm.GetAvailableLanguages()

	lm.mu.RLock()
	defer lm.mu.RUnlock()

	stats := map[string]any{
		"total_languages":  len(lm.localeData),
		"default_language": lm.defaultLang,
		"cache_enabled":    lm.cacheClient != nil,
		"languages":        languages,
	}

	totalSize := 0
	for _, data := range lm.localeData {
		totalSize += len(data)
	}
	stats["total_locale_data_size"] = totalSize

	return stats
}
