package i18n

import (
	"fmt"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// loadLocaleFiles loads all locale files from the embedded filesystem.
func (lm *LocaleManager) loadLocaleFiles() error {
	if lm.localeFS == nil || lm.localePath == "" {
		return NewI18nError("load_files", "", "", "filesystem or path not set", fmt.Errorf("invalid configuration"))
	}

	entries, err := lm.localeFS.ReadDir(lm.localePath)
	if err != nil {
		return NewI18nError("load_files", "", "", "failed to read locale directory", err)
	}

	var loadErrors []error

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Only process YAML files
		fileName := entry.Name()
		if !isYAMLFile(fileName) {
			continue
		}

		filePath := filepath.Join(lm.localePath, fileName)
		langCode := extractLangCode(fileName)

		if err := lm.loadSingleLocaleFile(filePath, langCode); err != nil {
			loadErrors = append(loadErrors, err)
			// Continue loading other files even if one fails
			continue
		}
	}

	if len(loadErrors) > 0 {
		return fmt.Errorf("failed to load %d locale files: %v", len(loadErrors), loadErrors)
	}

	return nil
}

// loadSingleLocaleFile loads and validates a single locale file.
func (lm *LocaleManager) loadSingleLocaleFile(filePath, langCode string) error {
	// Read file content
	content, err := lm.localeFS.ReadFile(filePath)
	if err != nil {
		return NewI18nError("load_file", langCode, "", "failed to read file", err)
	}

	parsed, err := parseYAML(content)
	if err != nil {
		return NewI18nError("load_file", langCode, "", "invalid YAML structure", err)
	}

	lm.localeMaps[langCode] = parsed

	return nil
}

// parseYAML unmarshals a YAML mapping for key lookups. yaml.v3 decodes nested
// mappings with string keys as map[string]any, so dot-path descent is clean.
func parseYAML(content []byte) (map[string]any, error) {
	var data any
	if err := yaml.Unmarshal(content, &data); err != nil {
		return nil, NewI18nError("validate_yaml", "", "", "YAML parsing failed", err)
	}

	parsed, ok := data.(map[string]any)
	if !ok {
		return nil, NewI18nError("validate_yaml", "", "", "root element must be a map", ErrInvalidYAML)
	}

	return parsed, nil
}

// extractLangCode extracts the language code from a filename.
func extractLangCode(fileName string) string {
	langCode := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	// Handle common YAML extensions
	langCode = strings.TrimSuffix(langCode, ".yml")
	langCode = strings.TrimSuffix(langCode, ".yaml")
	return langCode
}

// lookup descends a parsed YAML map by a dot-separated key path and returns the
// leaf value if present. Path segments are matched case-insensitively to replicate
// viper's case-insensitive key behavior (e.g. "alt_names.Admin" against a config
// where keys may differ in case).
func lookup(data map[string]any, key string) (any, bool) {
	if data == nil {
		return nil, false
	}

	segments := strings.Split(key, ".")
	var current any = data

	for _, seg := range segments {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		value, found := lookupSegment(m, seg)
		if !found {
			return nil, false
		}
		current = value
	}

	return current, true
}

// lookupSegment resolves a single map key, preferring an exact match and falling
// back to a case-insensitive match.
func lookupSegment(m map[string]any, seg string) (any, bool) {
	if value, ok := m[seg]; ok {
		return value, true
	}
	for k, v := range m {
		if strings.EqualFold(k, seg) {
			return v, true
		}
	}
	return nil, false
}

// lookupString resolves a dot-path key to its scalar value, coercing the leaf to a
// string via fmt.Sprint (mirroring viper.GetString). Missing keys yield "".
func lookupString(data map[string]any, key string) string {
	value, found := lookup(data, key)
	if !found || value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

// lookupStringSlice resolves a dot-path key to a []string, coercing each element of
// a YAML sequence via fmt.Sprint and splitting a scalar string on whitespace
// (mirroring viper.GetStringSlice). Missing keys yield an empty slice.
func lookupStringSlice(data map[string]any, key string) []string {
	value, found := lookup(data, key)
	if !found || value == nil {
		return nil
	}

	switch v := value.(type) {
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, elem := range v {
			out = append(out, fmt.Sprint(elem))
		}
		return out
	case string:
		return strings.Fields(v)
	default:
		return strings.Fields(fmt.Sprint(v))
	}
}

// isYAMLFile checks if a filename has a YAML extension.
func isYAMLFile(fileName string) bool {
	ext := strings.ToLower(filepath.Ext(fileName))
	return ext == ".yml" || ext == ".yaml"
}
