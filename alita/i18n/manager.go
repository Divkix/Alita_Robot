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
