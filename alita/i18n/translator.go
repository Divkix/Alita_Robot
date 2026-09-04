package i18n

import (
	"fmt"
	"regexp"
	"strconv"

	log "github.com/sirupsen/logrus"
)

var (
	paramRegex       = regexp.MustCompile(`\{([^}]+)\}`)
	legacyParamRegex = regexp.MustCompile(`%[sdvfbtoxX]`)
)

func (t *Translator) GetString(key string, params ...TranslationParams) (string, error) {
	if t.manager == nil {
		return "", NewI18nError("get_string", t.langCode, key, "manager not initialized", ErrManagerNotInit)
	}

	result := lookupString(t.data, key)

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

func (t *Translator) GetStringSlice(key string) ([]string, error) {
	if t.manager == nil {
		return nil, NewI18nError("get_string_slice", t.langCode, key, "manager not initialized", ErrManagerNotInit)
	}

	result := lookupStringSlice(t.data, key)

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

	return result, nil
}

func (t *Translator) interpolateParams(text string, params TranslationParams) (string, error) {
	if params == nil {
		return text, nil
	}

	result := text

	result = paramRegex.ReplaceAllStringFunc(result, func(match string) string {
		keyName := match[1 : len(match)-1]
		if value, exists := params[keyName]; exists {
			return fmt.Sprintf("%v", value)
		}
		return match
	})

	if legacyParamRegex.MatchString(result) {
		if orderedValues := extractOrderedValues(params); len(orderedValues) > 0 {
			specCount := len(legacyParamRegex.FindAllString(result, -1))
			if specCount <= len(orderedValues) {
				result = fmt.Sprintf(result, orderedValues[:specCount]...)
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
