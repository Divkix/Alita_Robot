package i18n

import (
	"context"
	"fmt"
	"regexp"
	"strconv"

	log "github.com/sirupsen/logrus"
)

var (
	// Regex for parameter interpolation {key} style
	paramRegex = regexp.MustCompile(`\{([^}]+)\}`)
	// Regex for legacy parameter interpolation %s, %d, etc.
	legacyParamRegex = regexp.MustCompile(`%[sdvfbtoxX]`)
)

// GetString retrieves a translated string with optional parameter interpolation.
func (t *Translator) GetString(key string, params ...TranslationParams) (string, error) {
	if t.manager == nil {
		return "", NewI18nError("get_string", t.langCode, key, "manager not initialized", ErrManagerNotInit)
	}

	// Create cache key if caching is enabled
	cacheKey := ""
	if t.manager.cacheClient != nil && len(params) == 0 {
		// Only cache non-parameterized strings
		cacheKey = t.cachePrefix + key

		// Try to get from cache first
		if cached, err := t.manager.cacheClient.Get(context.Background(), cacheKey); err == nil {
			if cachedStr, ok := cached.(string); ok {
				return cachedStr, nil
			}
		}
	}

	// Get string from viper
	result := t.viper.GetString(key)

	// Check if key exists
	if result == "" || result == "<nil>" {
		// Try fallback to default language if not already using it
		if t.langCode != t.manager.defaultLang {
			defaultTranslator, err := t.manager.GetTranslator(t.manager.defaultLang)
			if err != nil {
				return "", NewI18nError("get_string", t.langCode, key, "fallback failed", err)
			}
			// Prevent infinite recursion
			if defaultTranslator.langCode == t.langCode {
				return "", NewI18nError("get_string", t.langCode, key, "recursive fallback detected", ErrRecursiveFallback)
			}
			return defaultTranslator.GetString(key, params...)
		}
		return "", NewI18nError("get_string", t.langCode, key, "translation not found", ErrKeyNotFound)
	}

	// Apply parameter interpolation if params provided
	if len(params) > 0 {
		var err error
		result, err = t.interpolateParams(result, params[0])
		if err != nil {
			return result, NewI18nError("get_string", t.langCode, key, "parameter interpolation failed", err)
		}
	} else {
		// Cache non-parameterized results
		if cacheKey != "" && t.manager.cacheClient != nil {
			_ = t.manager.cacheClient.Set(context.Background(), cacheKey, result)
		}
	}

	return result, nil
}

// GetStringSlice retrieves a translated string slice.
func (t *Translator) GetStringSlice(key string) ([]string, error) {
	if t.manager == nil {
		return nil, NewI18nError("get_string_slice", t.langCode, key, "manager not initialized", ErrManagerNotInit)
	}

	// Create cache key
	cacheKey := ""
	if t.manager.cacheClient != nil {
		cacheKey = t.cachePrefix + "slice:" + key

		// Try to get from cache first
		if cached, err := t.manager.cacheClient.Get(context.Background(), cacheKey); err == nil {
			if cachedSlice, ok := cached.([]string); ok {
				return cachedSlice, nil
			}
		}
	}

	result := t.viper.GetStringSlice(key)

	// Check if key exists
	if len(result) == 0 {
		// Try fallback to default language
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

	// Cache the result
	if cacheKey != "" && t.manager.cacheClient != nil {
		_ = t.manager.cacheClient.Set(context.Background(), cacheKey, result)
	}

	return result, nil
}

// GetInt retrieves a translated integer value.
func (t *Translator) GetInt(key string) (int, error) {
	if t.manager == nil {
		return 0, NewI18nError("get_int", t.langCode, key, "manager not initialized", ErrManagerNotInit)
	}

	result := t.viper.GetInt(key)
	if !t.viper.IsSet(key) {
		if t.langCode != t.manager.defaultLang {
			defaultTranslator, err := t.manager.GetTranslator(t.manager.defaultLang)
			if err != nil {
				return 0, NewI18nError("get_int", t.langCode, key, "fallback failed", err)
			}
			return defaultTranslator.GetInt(key)
		}
		return 0, NewI18nError("get_int", t.langCode, key, "translation not found", ErrKeyNotFound)
	}
	return result, nil
}

// GetBool retrieves a translated boolean value.
func (t *Translator) GetBool(key string) (bool, error) {
	if t.manager == nil {
		return false, NewI18nError("get_bool", t.langCode, key, "manager not initialized", ErrManagerNotInit)
	}

	result := t.viper.GetBool(key)
	if !t.viper.IsSet(key) {
		if t.langCode != t.manager.defaultLang {
			defaultTranslator, err := t.manager.GetTranslator(t.manager.defaultLang)
			if err != nil {
				return false, NewI18nError("get_bool", t.langCode, key, "fallback failed", err)
			}
			return defaultTranslator.GetBool(key)
		}
		return false, NewI18nError("get_bool", t.langCode, key, "translation not found", ErrKeyNotFound)
	}
	return result, nil
}

// GetFloat retrieves a translated float value.
func (t *Translator) GetFloat(key string) (float64, error) {
	if t.manager == nil {
		return 0.0, NewI18nError("get_float", t.langCode, key, "manager not initialized", ErrManagerNotInit)
	}

	result := t.viper.GetFloat64(key)
	if !t.viper.IsSet(key) {
		if t.langCode != t.manager.defaultLang {
			defaultTranslator, err := t.manager.GetTranslator(t.manager.defaultLang)
			if err != nil {
				return 0.0, NewI18nError("get_float", t.langCode, key, "fallback failed", err)
			}
			return defaultTranslator.GetFloat(key)
		}
		return 0.0, NewI18nError("get_float", t.langCode, key, "translation not found", ErrKeyNotFound)
	}
	return result, nil
}

// GetPlural retrieves a pluralized string based on count.
func (t *Translator) GetPlural(key string, count int, params ...TranslationParams) (string, error) {
	if t.manager == nil {
		return "", NewI18nError("get_plural", t.langCode, key, "manager not initialized", ErrManagerNotInit)
	}

	// Try to get plural forms
	pluralRule := PluralRule{
		Zero:  t.viper.GetString(key + ".zero"),
		One:   t.viper.GetString(key + ".one"),
		Two:   t.viper.GetString(key + ".two"),
		Few:   t.viper.GetString(key + ".few"),
		Many:  t.viper.GetString(key + ".many"),
		Other: t.viper.GetString(key + ".other"),
	}

	// If no plural forms found, try fallback
	if pluralRule.Other == "" && pluralRule.One == "" {
		// Try simple key without plural forms
		simple, err := t.GetString(key, params...)
		if err == nil && simple != "" {
			return simple, nil
		}

		// Try fallback to default language
		if t.langCode != t.manager.defaultLang {
			defaultTranslator, err := t.manager.GetTranslator(t.manager.defaultLang)
			if err != nil {
				return "", NewI18nError("get_plural", t.langCode, key, "fallback failed", err)
			}
			return defaultTranslator.GetPlural(key, count, params...)
		}
		return "", NewI18nError("get_plural", t.langCode, key, "plural translation not found", ErrKeyNotFound)
	}

	// Select appropriate plural form
	selectedForm := t.selectPluralForm(pluralRule, count)
	if selectedForm == "" {
		return "", NewI18nError("get_plural", t.langCode, key, "no appropriate plural form found", ErrKeyNotFound)
	}

	// Apply parameter interpolation if params provided
	if len(params) > 0 {
		// Add count to parameters if not already present
		enhancedParams := params[0]
		if enhancedParams == nil {
			enhancedParams = make(TranslationParams)
		}
		if _, exists := enhancedParams["count"]; !exists {
			enhancedParams["count"] = count
		}

		var err error
		selectedForm, err = t.interpolateParams(selectedForm, enhancedParams)
		if err != nil {
			return selectedForm, NewI18nError("get_plural", t.langCode, key, "parameter interpolation failed", err)
		}
	}

	return selectedForm, nil
}

// interpolateParams performs parameter interpolation on a string.
func (t *Translator) interpolateParams(text string, params TranslationParams) (string, error) {
	if params == nil {
		return text, nil
	}

	result := text

	// Handle {key} style parameters
	result = paramRegex.ReplaceAllStringFunc(result, func(match string) string {
		// Extract key name (remove { and })
		keyName := match[1 : len(match)-1]
		if value, exists := params[keyName]; exists {
			return fmt.Sprintf("%v", value)
		}
		return match // Keep original if no replacement found
	})

	// Handle legacy %s style parameters (for backward compatibility)
	// This is more complex as we need to maintain order
	if legacyParamRegex.MatchString(result) {
		// For legacy support, try to find numbered parameters or use order
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

// selectPluralForm selects the appropriate plural form based on language rules.
func (t *Translator) selectPluralForm(rule PluralRule, count int) string {
	// Implement basic English plural rules
	// For more languages, this would need language-specific logic

	switch {
	case count == 0 && rule.Zero != "":
		return rule.Zero
	case count == 1 && rule.One != "":
		return rule.One
	case count == 2 && rule.Two != "":
		return rule.Two
	case rule.Other != "":
		return rule.Other
	case rule.Many != "":
		return rule.Many
	case rule.Few != "":
		return rule.Few
	case rule.One != "":
		return rule.One
	default:
		return ""
	}
}

// extractOrderedValues extracts values from params in a predictable order for legacy sprintf.
func extractOrderedValues(params TranslationParams) []any {
	if params == nil {
		return nil
	}

	var values []any

	// Try common numbered keys first (0, 1, 2, etc.)
	for i := 0; i < 10; i++ {
		key := strconv.Itoa(i)
		if value, exists := params[key]; exists {
			values = append(values, value)
		} else {
			break
		}
	}

	// If no numbered keys found, try a predefined order of common keys
	if len(values) == 0 {
		// Extended list of common keys in a predictable order
		// This covers the current usage patterns in the codebase
		commonKeys := []string{
			// Most common patterns from the codebase
			"first", "second", "third", "fourth", "fifth",
			// Question/answer patterns (before number to match captcha_welcome_math_text order)
			"question", "answer",
			// Common numeric-related keys
			"number", "count", "value",
			// User/name patterns
			"name", "user", "username",
			// Generic argument patterns
			"arg1", "arg2", "arg3",
			// Single letter keys
			"s", "d", "v", "f",
		}

		for _, key := range commonKeys {
			if value, exists := params[key]; exists {
				values = append(values, value)
			}
		}
	}

	return values
}

// GetLanguageCode returns the language code for this translator.
func (t *Translator) GetLanguageCode() string {
	return t.langCode
}

// IsDefaultLanguage checks if this translator uses the default language.
func (t *Translator) IsDefaultLanguage() bool {
	return t.langCode == t.manager.defaultLang
}
