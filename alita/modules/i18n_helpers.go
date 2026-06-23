package modules

import "github.com/divkix/Alita_Robot/alita/i18n"

// trS returns the translation for key, discarding the lookup error (which only
// signals a missing key; GetString already falls back to English). It replaces
// the repeated `func() string { t, _ := tr.GetString(key); return t }()` inline
// closures used to set single button labels.
func trS(tr *i18n.Translator, key string) string {
	s, _ := tr.GetString(key)
	return s
}
