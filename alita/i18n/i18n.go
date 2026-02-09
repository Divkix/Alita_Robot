package i18n

import (
	log "github.com/sirupsen/logrus"
)

// NewTranslator creates a new Translator instance using the modern LocaleManager.
// This is the recommended way to handle translations in new code.
func NewTranslator(langCode string) (*Translator, error) {
	manager := GetManager()
	return manager.GetTranslator(langCode)
}

// MustNewTranslator creates a new Translator instance with safe fallback.
// Falls back to English on error instead of panicking.
func MustNewTranslator(langCode string) *Translator {
	translator, err := NewTranslator(langCode)
	if err != nil {
		log.Warnf("Failed to create translator for %s: %v, falling back to English", langCode, err)
		translator, err = NewTranslator("en")
		if err != nil {
			log.Errorf("Failed to create English fallback translator: %v", err)
			return &Translator{langCode: "en"}
		}
	}
	return translator
}
