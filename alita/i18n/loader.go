package i18n

import (
	"bytes"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
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

		// Skip config files - they contain module configuration, not translations
		if fileName == "config.yml" || fileName == "config.yaml" {
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

	// Validate YAML structure
	if err := validateYAMLStructure(content); err != nil {
		return NewI18nError("load_file", langCode, "", "invalid YAML structure", err)
	}

	// Store raw data
	lm.localeData[langCode] = content

	// Pre-compile viper instance
	viperInstance, err := compileViper(content)
	if err != nil {
		return NewI18nError("load_file", langCode, "", "failed to compile viper", err)
	}

	lm.viperCache[langCode] = viperInstance

	return nil
}

// validateYAMLStructure validates that the YAML content is well-formed.
func validateYAMLStructure(content []byte) error {
	var data any
	if err := yaml.Unmarshal(content, &data); err != nil {
		return NewI18nError("validate_yaml", "", "", "YAML parsing failed", err)
	}

	// Check if it's a map structure (required for translations)
	if _, ok := data.(map[string]any); !ok {
		return NewI18nError("validate_yaml", "", "", "root element must be a map", ErrInvalidYAML)
	}

	return nil
}

// extractLangCode extracts the language code from a filename.
func extractLangCode(fileName string) string {
	langCode := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	// Handle common YAML extensions
	langCode = strings.TrimSuffix(langCode, ".yml")
	langCode = strings.TrimSuffix(langCode, ".yaml")
	return langCode
}

// compileViper creates and configures a viper instance from YAML content.
func compileViper(content []byte) (*viper.Viper, error) {
	vi := viper.New()
	vi.SetConfigType("yaml")

	if err := vi.ReadConfig(bytes.NewBuffer(content)); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	return vi, nil
}

// isYAMLFile checks if a filename has a YAML extension.
func isYAMLFile(fileName string) bool {
	ext := strings.ToLower(filepath.Ext(fileName))
	return ext == ".yml" || ext == ".yaml"
}
