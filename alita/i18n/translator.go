package i18n

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"sync"

	log "github.com/sirupsen/logrus"
)

var (
	paramRegex       = regexp.MustCompile(`\{([^}]+)\}`)
	legacyParamRegex = regexp.MustCompile(`%[sdvfbtoxX]`)
)

// reportedMisses dedupes String's miss logging: one Warn per key per process,
// so a typo'd key is greppable in telemetry without flooding it on every
// reply. GetString callers that branch on the error do not touch it.
var reportedMisses sync.Map

func (t *Translator) GetString(key string, params ...TranslationParams) (string, error) {
	if t.manager == nil {
		return "", NewI18nError("get_string", t.langCode, key, "manager not initialized", ErrManagerNotInit)
	}

	result := t.lookupString(key)

	if result == "" || result == "<nil>" {
		if t.langCode != t.manager.defaultLang {
			defaultTranslator, err := t.manager.GetTranslator(t.manager.defaultLang)
			if err != nil {
				return "", NewI18nError("get_string", t.langCode, key, "fallback failed", err)
			}
			if defaultTranslator.langCode == t.langCode {
				return "", NewI18nError("get_string", t.langCode, key, "recursive fallback detected", ErrRecursiveFallback)
			}
			return defaultTranslator.GetString(key, params...)
		}
		return "", NewI18nError("get_string", t.langCode, key, "translation not found", ErrKeyNotFound)
	}

	if len(params) > 0 {
		var err error
		result, err = t.interpolateParams(result, params[0])
		if err != nil {
			return result, NewI18nError("get_string", t.langCode, key, "parameter interpolation failed", err)
		}
	}

	return result, nil
}

// String is the never-error form of GetString for reply text: it resolves in
// the translator's language (the caller builds it from the chat's language),
// interpolates params, falls back to the default language, and on a total
// miss logs the key once and returns the key itself — a visible, greppable
// miss instead of a silently empty message. Callers that must detect a miss
// keep using GetString.
func (t *Translator) String(key string, params ...TranslationParams) string {
	if t == nil {
		return key
	}
	result, err := t.GetString(key, params...)
	if err == nil || result != "" {
		// A non-empty result with an error is interpolation's partially
		// filled text, still the best sendable value.
		return result
	}
	if t.manager != nil && t.langCode != t.manager.defaultLang {
		// GetString already fell back to the default language; a miss there
		// is a total miss. Let that translator's String log it under its own
		// language, so the Warn names the locale that lacks the key.
		if defaultTranslator, derr := t.manager.GetTranslator(t.manager.defaultLang); derr == nil && defaultTranslator != t {
			return defaultTranslator.String(key, params...)
		}
	}
	if _, reported := reportedMisses.LoadOrStore(key, struct{}{}); !reported {
		log.Warnf("[i18n] missing translation key=%q lang=%q; returning the key", key, t.langCode)
	}
	return key
}

func (t *Translator) GetStringSlice(key string) ([]string, error) {
	if t.manager == nil {
		return nil, NewI18nError("get_string_slice", t.langCode, key, "manager not initialized", ErrManagerNotInit)
	}

	result := t.lookupStringSlice(key)

	if len(result) == 0 {
		if t.langCode != t.manager.defaultLang {
			defaultTranslator, err := t.manager.GetTranslator(t.manager.defaultLang)
			if err != nil {
				return nil, NewI18nError("get_string_slice", t.langCode, key, "fallback failed", err)
			}
			if defaultTranslator.langCode == t.langCode {
				return nil, NewI18nError("get_string_slice", t.langCode, key, "recursive fallback detected", ErrRecursiveFallback)
			}
			return defaultTranslator.GetStringSlice(key)
		}
		return nil, NewI18nError("get_string_slice", t.langCode, key, "translation not found", ErrKeyNotFound)
	}

	// The lookup may hand back storage the index or the parsed map still owns, so callers
	// get a copy they can mutate.
	return slices.Clone(result), nil
}

func (t *Translator) interpolateParams(text string, params TranslationParams) (string, error) {
	if params == nil {
		return text, nil
	}

	substituted := false
	result := paramRegex.ReplaceAllStringFunc(text, func(match string) string {
		keyName := match[1 : len(match)-1]
		if value, exists := params[keyName]; exists {
			substituted = true
			return fmt.Sprintf("%v", value)
		}
		return match
	})
	// A filled {name} template is done: a '%' in a substituted value is user
	// text, not a verb, so it must never reach Sprintf.
	if substituted {
		return result, nil
	}

	if legacyParamRegex.MatchString(text) {
		if orderedValues := extractOrderedValues(params); len(orderedValues) > 0 {
			specCount := len(legacyParamRegex.FindAllString(text, -1))
			if specCount <= len(orderedValues) {
				result = fmt.Sprintf(text, orderedValues[:specCount]...)
			} else {
				log.Warnf("Translation specifier count mismatch: %d specifiers, %d values for key in lang %s", specCount, len(orderedValues), t.langCode)
			}
		}
	}

	return result, nil
}

func extractOrderedValues(params TranslationParams) []any {
	if params == nil {
		return nil
	}

	var values []any

	for i := 0; i < 10; i++ {
		key := strconv.Itoa(i)
		if value, exists := params[key]; exists {
			values = append(values, value)
		} else {
			break
		}
	}

	if len(values) == 0 {
		commonKeys := []string{
			"first", "second", "third", "fourth", "fifth",
			"question", "answer",
			"number", "count", "value",
			"name", "user", "username",
			"arg1", "arg2", "arg3",
			"s", "d", "v", "f",
			"duration", "threshold", "error", "user_id", "expires_in", "raid_time", "action_time", "auto_threshold", "mode", "reason", "limit",
		}
		for _, key := range commonKeys {
			if value, exists := params[key]; exists {
				values = append(values, value)
			}
		}
	}

	return values
}
