package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type TranslationKey struct {
	Key  string
	File string
	Line int
}

type MissingTranslation struct {
	Key   string
	Usage []string
}

func main() {
	fmt.Println("🔍 Checking translations...")
	fmt.Println()

	// Step 1: Extract all translation keys from Go files
	keys, err := extractTranslationKeys("../../alita")
	if err != nil {
		fmt.Printf("❌ Error extracting translation keys: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("📊 Found %d translation keys in codebase\n", len(keys))

	// Step 2: Load locale files
	locales, err := loadLocaleFiles("../../locales")
	if err != nil {
		fmt.Printf("❌ Error loading locale files: %v\n", err)
		os.Exit(1)
	}

	// Step 3: Check each locale for missing keys
	totalMissing := 0
	for localeName, locale := range locales {
		fmt.Printf("\n📁 Checking locale: %s\n", localeName)
		missing := checkMissingKeys(keys, locale)

		if len(missing) > 0 {
			fmt.Printf("  ⚠️  Missing %d translations:\n", len(missing))
			for _, m := range missing {
				fmt.Printf("    • %s\n", m.Key)
				for _, usage := range m.Usage {
					fmt.Printf("      └─ used in: %s\n", usage)
				}
			}
			totalMissing += len(missing)
		} else {
			fmt.Printf("  ✅ All translations present\n")
		}
	}

	// Step 4: Summary
	fmt.Printf("\n" + strings.Repeat("─", 50) + "\n")
	if totalMissing > 0 {
		fmt.Printf("❌ Summary: Found %d missing translations\n", totalMissing)
		os.Exit(1)
	} else {
		fmt.Printf("✅ Summary: All translations are present!\n")
	}
}

func extractTranslationKeys(rootDir string) ([]TranslationKey, error) {
	var keys []TranslationKey

	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		fileKeys, err := extractKeysFromFile(path)
		if err != nil {
			return fmt.Errorf("extract keys from %s: %w", path, err)
		}
		keys = append(keys, fileKeys...)

		return nil
	})

	// Note: alt_names are configuration keys, not translation keys
	// They are loaded from config.yml, not from translation files
	// So we don't need to check for them

	return keys, err
}

func extractKeysFromFile(filePath string) ([]TranslationKey, error) {
	var keys []TranslationKey

	// Resolve to absolute path to handle legitimate relative paths (e.g., ../../alita/...)
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, fmt.Errorf("could not resolve path %s: %w", filePath, err)
	}
	filePath = absPath

	// Read file content
	content, err := os.ReadFile(filePath) // #nosec G304 - path validation performed above
	if err != nil {
		return nil, err
	}

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, content, parser.AllErrors)
	if err != nil {
		return nil, err
	}

	ast.Inspect(node, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || (sel.Sel.Name != "GetString" && sel.Sel.Name != "GetStringSlice") || len(call.Args) == 0 {
			return true
		}
		lit, ok := call.Args[0].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		key, err := strconv.Unquote(lit.Value)
		if err != nil {
			return true
		}
		pos := fset.Position(lit.Pos())
		keys = append(keys, TranslationKey{Key: key, File: filePath, Line: pos.Line})

		return true
	})

	return keys, nil
}

func loadLocaleFiles(localesDir string) (map[string]map[string]any, error) {
	locales := make(map[string]map[string]any)

	entries, err := os.ReadDir(localesDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()
		if !strings.HasSuffix(filename, ".yml") && !strings.HasSuffix(filename, ".yaml") {
			continue
		}

		// Skip config.yml as it's not a translation file
		if filename == "config.yml" {
			continue
		}

		filePath := filepath.Join(localesDir, filename)

		// Resolve to absolute path to handle legitimate relative paths
		absFilePath, err := filepath.Abs(filePath)
		if err != nil {
			fmt.Printf("  Warning: Could not resolve path %s, skipping: %v\n", filename, err)
			continue
		}
		filePath = absFilePath

		// Ensure the resolved path is still within the locales directory
		absLocalesDir, _ := filepath.Abs(localesDir)
		if !strings.HasPrefix(filePath, absLocalesDir) {
			fmt.Printf("  Warning: File path %s is outside locales directory, skipping\n", filename)
			continue
		}

		data, err := os.ReadFile(filePath) // #nosec G304 - path validation performed above
		if err != nil {
			fmt.Printf("  ⚠️  Warning: Could not read %s: %v\n", filename, err)
			continue
		}

		var locale map[string]any
		if err := yaml.Unmarshal(data, &locale); err != nil {
			fmt.Printf("  ⚠️  Warning: Could not parse %s: %v\n", filename, err)
			continue
		}

		locales[filename] = locale
	}

	return locales, nil
}

func checkMissingKeys(keys []TranslationKey, locale map[string]any) []MissingTranslation {
	missing := make(map[string][]string)

	for _, key := range keys {
		// Skip alt_names keys as they're configuration, not translations
		if strings.HasPrefix(key.Key, "alt_names.") {
			continue
		}

		if !keyExists(key.Key, locale) {
			usage := fmt.Sprintf("%s:%d", filepath.Base(key.File), key.Line)
			missing[key.Key] = append(missing[key.Key], usage)
		}
	}

	// Convert to sorted list
	var result []MissingTranslation
	for key, usages := range missing {
		result = append(result, MissingTranslation{
			Key:   key,
			Usage: usages,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Key < result[j].Key
	})

	return result
}

func keyExists(key string, data map[string]any) bool {
	parts := strings.Split(key, ".")
	current := data

	for i, part := range parts {
		if i == len(parts)-1 {
			// Last part - check if key exists
			_, exists := current[part]
			return exists
		}

		// Navigate deeper
		if next, ok := current[part].(map[string]any); ok {
			current = next
		} else {
			return false
		}
	}

	return false
}
