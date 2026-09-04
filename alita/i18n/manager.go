package i18n

import (
	"embed"
	"fmt"
	"sync"
)

var (
	managerInstance *LocaleManager
	managerOnce     sync.Once
)

func GetManager() *LocaleManager {
	managerOnce.Do(func() {
		managerInstance = &LocaleManager{
			localeMaps:  make(map[string]map[string]any),
			defaultLang: "en",
		}
	})
	return managerInstance
}

func (lm *LocaleManager) Initialize(fs *embed.FS, localePath string, config ManagerConfig) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	if lm.localeFS != nil {
		return fmt.Errorf("locale manager already initialized")
	}

	lm.localeFS = fs
	lm.localePath = localePath
	lm.defaultLang = config.Loader.DefaultLanguage

	if err := lm.loadLocaleFiles(); err != nil {
		if config.Loader.StrictMode {
			return NewI18nError("initialize", "", "", "failed to load locale files", err)
		}
		fmt.Printf("Warning: failed to load some locale files: %v\n", err)
	}

	if _, exists := lm.localeMaps[lm.defaultLang]; !exists {
		return NewI18nError("initialize", lm.defaultLang, "", "default language not found", ErrLocaleNotFound)
	}

	return nil
}

func (lm *LocaleManager) GetTranslator(langCode string) (*Translator, error) {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	if lm.localeFS == nil {
		return nil, NewI18nError("get_translator", langCode, "", "manager not initialized", ErrManagerNotInit)
	}

	targetLang := langCode
	data, exists := lm.localeMaps[langCode]
	if !exists {
		targetLang = lm.defaultLang
		data = lm.localeMaps[lm.defaultLang]
		if data == nil {
			return nil, NewI18nError("get_translator", langCode, "", "default language data not found", ErrLocaleNotFound)
		}
	}

	return &Translator{
		langCode: targetLang,
		manager:  lm,
		data:     data,
	}, nil
}

func (lm *LocaleManager) GetAvailableLanguages() []string {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	languages := make([]string, 0, len(lm.localeMaps))
	for langCode := range lm.localeMaps {
		languages = append(languages, langCode)
	}
	return languages
}
