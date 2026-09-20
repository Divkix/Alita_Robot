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
	// Derived state is rebuilt from the locale files below; drop anything a
	// pre-populated manager carried.
	lm.localeIndex = make(map[string]*lookupIndex)
	lm.translators.Clear()

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
	// Built translators are immutable, so a cache hit needs no locking.
	if cached, ok := lm.translators.Load(langCode); ok {
		return cached.(*Translator), nil
	}

	lm.mu.RLock()
	if lm.localeFS == nil {
		lm.mu.RUnlock()
		return nil, NewI18nError("get_translator", langCode, "", "manager not initialized", ErrManagerNotInit)
	}

	targetLang := langCode
	data, exists := lm.localeMaps[langCode]
	if !exists {
		targetLang = lm.defaultLang
		data = lm.localeMaps[lm.defaultLang]
		if data == nil {
			lm.mu.RUnlock()
			return nil, NewI18nError("get_translator", langCode, "", "default language data not found", ErrLocaleNotFound)
		}
	}
	if cached, ok := lm.translators.Load(targetLang); ok {
		lm.mu.RUnlock()
		return cached.(*Translator), nil
	}
	lm.mu.RUnlock()

	lm.mu.Lock()
	defer lm.mu.Unlock()

	if cached, ok := lm.translators.Load(targetLang); ok {
		return cached.(*Translator), nil
	}

	index := lm.localeIndex[targetLang]
	if index == nil && data != nil {
		index = buildLookupIndex(data)
		if lm.localeIndex == nil {
			lm.localeIndex = make(map[string]*lookupIndex)
		}
		lm.localeIndex[targetLang] = index
	}

	translator := &Translator{
		langCode: targetLang,
		manager:  lm,
		data:     data,
		index:    index,
	}
	lm.translators.Store(targetLang, translator)
	return translator, nil
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
